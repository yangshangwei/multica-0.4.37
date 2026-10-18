package service

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CreateComment is the sole human comment/input transaction. A domain refusal
// belongs to its target; a database failure rolls back the entire operation.
func (s *CollaborationService) CreateComment(ctx context.Context, p CollaborationCreateCommentParams) (CollaborationCommentReceipt, error) {
	if err := validateCollaborationCommentInput(p.CollaborationCommentInput); err != nil {
		return CollaborationCommentReceipt{}, err
	}
	if p.ExpectedLifecycleEpoch < 1 || p.ExpectedParticipantEpoch < 0 || p.ExpectedAccountEpoch < 0 || len(p.TargetVersions) > 256 {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusBadRequest, "invalid_comment_request")
	}
	versions := make(map[pgtype.UUID]CollaborationCommentTargetVersion, len(p.TargetVersions))
	for _, version := range p.TargetVersions {
		if !version.TargetID.Valid || version.ExpectedTargetEpoch < 1 || version.ExpectedConfigVersion < 1 {
			return CollaborationCommentReceipt{}, collaborationError(http.StatusBadRequest, "invalid_target_version")
		}
		if prior, exists := versions[version.TargetID]; exists && prior != version {
			return CollaborationCommentReceipt{}, collaborationError(http.StatusBadRequest, "invalid_target_version")
		}
		versions[version.TargetID] = version
	}
	return s.mutateCollaborationContent(ctx, p.IssueID, p.RequestID, "create_comment", p, func(t *collaborationContentTx) (CollaborationCommentReceipt, error) {
		if err := validateCollaborationCommentParent(ctx, t.q, t.access, p.ParentID); err != nil {
			return CollaborationCommentReceipt{}, err
		}
		var catalogue []CollaborationMentionTarget
		if len(parseCollaborationMentions(p.Content).Targets) > 0 {
			var err error
			catalogue, err = s.AuthorizedMentionTargets(ctx, t.q, t.access)
			if err != nil {
				return CollaborationCommentReceipt{}, err
			}
		}
		targets := collaborationCommentTargetPreview(t.access, p.CollaborationCommentInput, catalogue)
		positions := make(map[pgtype.UUID]int)
		for i := range targets {
			item := &targets[i]
			if item.State != "waiting" {
				continue
			}
			version, exists := versions[item.TargetID]
			if !exists || version.ExpectedTargetEpoch != item.TargetEpoch || version.ExpectedConfigVersion != item.ConfigVersion {
				item.State, item.Code, item.WaitingReason = "blocked", "stale_target", ""
				continue
			}
			positions[item.TargetID] = i
		}
		// Acquire all selected resource locks before any batch/command writes.
		// The append primitive still revalidates, on the same transaction.
		for _, target := range t.targets {
			i, selected := positions[target.ID]
			if !selected {
				continue
			}
			code, err := s.AuthorizeTargetInputInTx(ctx, t.tx, p.IssueID, target)
			if err != nil {
				return CollaborationCommentReceipt{}, err
			}
			if code != "" {
				targets[i].State, targets[i].Code, targets[i].WaitingReason = "blocked", code, ""
			}
		}
		comment, err := t.q.CreateCollaborationComment(ctx, db.CreateCollaborationCommentParams{
			CommentID: util.MustParseUUID(uuid.NewString()), IssueID: p.IssueID, WorkspaceID: t.access.Issue.WorkspaceID,
			AuthorID: t.actor, ParentID: p.ParentID, Content: util.SanitizeTextForPostgres(p.Content)})
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		receipt := t.result("comment_created", http.StatusCreated)
		receipt.CommentID, receipt.CommentRevision, receipt.EventID = comment.ID, comment.Revision, util.MustParseUUID(uuid.NewString())
		for i := range targets {
			item := &targets[i]
			if item.State != "waiting" {
				continue
			}
			version := versions[item.TargetID]
			queued, err := s.AppendCollaborationInputInTx(ctx, t.tx, CollaborationSubmitCommandParams{
				CollaborationQueueRequest: p.CollaborationQueueRequest, TargetID: item.TargetID, InputEventID: receipt.EventID,
				ExpectedTargetEpoch: version.ExpectedTargetEpoch, ExpectedConfigVersion: version.ExpectedConfigVersion, Content: comment.Content})
			if err != nil {
				return CollaborationCommentReceipt{}, err
			}
			if queued.Status >= 400 {
				item.State, item.Code, item.WaitingReason = "blocked", queued.Code, ""
				continue
			}
			if !queued.CommandID.Valid || queued.CommandSequence < 1 || !queued.ChannelID.Valid || !queued.BatchID.Valid {
				return CollaborationCommentReceipt{}, invalidCollaborationLedger("comment append returned no input or batch")
			}
			item.Code, item.CommandID, item.CommandSequence = queued.Code, queued.CommandID, queued.CommandSequence
			item.ChannelID, item.BatchID = queued.ChannelID, queued.BatchID
		}
		receipt.Targets = targets
		if err = t.recordCommentEvent(ctx, &receipt, nil, comment.Content); err != nil {
			return CollaborationCommentReceipt{}, err
		}
		participants, err := collaborationMentionParticipants(ctx, t.q, t.access)
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		recipients := collaborationCommentRecipients(participants, t.actor, parseCollaborationMentions(comment.Content))
		if err = writeCollaborationCommentNotifications(ctx, t.q, p.IssueID, comment.ID, receipt.EventSeq, "comment_created", "comment", recipients); err != nil {
			return CollaborationCommentReceipt{}, err
		}
		return receipt, nil
	})
}
