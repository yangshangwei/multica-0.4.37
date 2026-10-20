package service

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type collaborationMentionRef struct {
	Kind       collaboration.TargetKind
	ResourceID pgtype.UUID
}

type collaborationMentionSet struct {
	People  map[pgtype.UUID]bool
	All     bool
	Targets []collaborationMentionRef
}

func parseCollaborationMentions(content string) collaborationMentionSet {
	result := collaborationMentionSet{People: make(map[pgtype.UUID]bool), Targets: make([]collaborationMentionRef, 0)}
	seen := make(map[collaborationMentionRef]bool)
	for _, mention := range util.ParseMentions(util.SanitizeTextForPostgres(content)) {
		if mention.Type == "all" && mention.ID == "all" {
			result.All = true
			continue
		}
		id, err := util.ParseUUID(mention.ID)
		if err != nil {
			continue
		}
		switch mention.Type {
		case "member":
			result.People[id] = true
		case "agent", "squad":
			ref := collaborationMentionRef{Kind: collaboration.TargetKind(mention.Type), ResourceID: id}
			if !seen[ref] {
				seen[ref] = true
				result.Targets = append(result.Targets, ref)
			}
		}
	}
	return result
}

type CollaborationMentionCatalogue struct {
	ProtocolVersion   int                           `json:"protocol_version"`
	IssueID           pgtype.UUID                   `json:"issue_id"`
	Participants      []CollaborationContentAccount `json:"participants"`
	Targets           []CollaborationMentionTarget  `json:"targets"`
	CanSubmitCommands bool                          `json:"can_submit_commands"`
}

type CollaborationCommentWriteContext struct {
	ParticipantEpoch int64 `json:"participant_epoch,string"`
	AccountEpoch     int64 `json:"account_epoch,string"`
}

type CollaborationCommentPreview struct {
	ProtocolVersion int                                 `json:"protocol_version"`
	IssueID         pgtype.UUID                         `json:"issue_id"`
	LifecycleEpoch  int64                               `json:"lifecycle_epoch,string"`
	WriteContext    *CollaborationCommentWriteContext   `json:"write_context,omitempty"`
	Targets         []CollaborationCommentTargetPreview `json:"targets"`
	Recipients      []CollaborationCommentRecipient     `json:"recipients"`
}

func (s *CollaborationService) MentionTargets(ctx context.Context, issueID pgtype.UUID) (CollaborationMentionCatalogue, error) {
	out := CollaborationMentionCatalogue{ProtocolVersion: CollaborationContentProtocolVersion, IssueID: issueID}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		var err error
		out.Participants, err = collaborationMentionParticipants(ctx, q, access)
		if err != nil {
			return err
		}
		out.Targets, err = s.AuthorizedMentionTargets(ctx, q, access)
		out.CanSubmitCommands = access.Capabilities.SubmitCommand
		return err
	})
	return out, err
}

func validateCollaborationCommentInput(p CollaborationCommentInput) error {
	if (strings.TrimSpace(util.SanitizeTextForPostgres(p.Content)) == "" && len(p.AttachmentIDs) == 0) || len(p.Content) > 64<<10 || len(p.SuppressedTargetIDs) > 256 {
		return collaborationError(http.StatusBadRequest, "invalid_comment_request")
	}
	if err := validateCollaborationAttachmentIDs(p.AttachmentIDs); err != nil {
		return err
	}
	for _, id := range p.SuppressedTargetIDs {
		if !id.Valid {
			return collaborationError(http.StatusBadRequest, "invalid_comment_request")
		}
	}
	if len(parseCollaborationMentions(p.Content).Targets) > 256 {
		return collaborationError(http.StatusBadRequest, "too_many_comment_targets")
	}
	return nil
}

func validateCollaborationCommentParent(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, parent pgtype.UUID) error {
	if !parent.Valid {
		return nil
	}
	_, err := q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{IssueID: access.Issue.ID, WorkspaceID: access.Issue.WorkspaceID, CommentID: parent})
	if errors.Is(err, pgx.ErrNoRows) {
		return collaborationError(http.StatusNotFound, "parent_comment_not_found")
	}
	return err
}

func (s *CollaborationService) PreviewComment(ctx context.Context, issueID pgtype.UUID, p CollaborationCommentInput) (CollaborationCommentPreview, error) {
	var out CollaborationCommentPreview
	if err := validateCollaborationCommentInput(p); err != nil {
		return out, err
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if err := validateCollaborationCommentParent(ctx, q, access, p.ParentID); err != nil {
			return err
		}
		var catalogue []CollaborationMentionTarget
		if len(parseCollaborationMentions(p.Content).Targets) > 0 {
			var err error
			catalogue, err = s.AuthorizedMentionTargets(ctx, q, access)
			if err != nil {
				return err
			}
		}
		participants, err := collaborationMentionParticipants(ctx, q, access)
		if err != nil {
			return err
		}
		principal, _ := collaboration.PrincipalFromContext(ctx)
		actor, err := util.ParseUUID(principal.UserID)
		if err != nil {
			return err
		}
		targets, err := s.previewCommentGrouping(ctx, q, access, actor, p, catalogue)
		if err != nil {
			return err
		}
		out = CollaborationCommentPreview{ProtocolVersion: CollaborationContentProtocolVersion, IssueID: issueID, LifecycleEpoch: access.LifecycleEpoch,
			Targets: targets, Recipients: collaborationCommentRecipients(participants, actor, parseCollaborationMentions(p.Content))}
		if access.ParticipantEpoch.Valid && access.AccountEpoch.Valid {
			out.WriteContext = &CollaborationCommentWriteContext{ParticipantEpoch: access.ParticipantEpoch.Int64, AccountEpoch: access.AccountEpoch.Int64}
		}
		return nil
	})
	return out, err
}

func collaborationCommentTargetPreview(access CollaborationIssueAccess, input CollaborationCommentInput, catalogue []CollaborationMentionTarget) []CollaborationCommentTargetResult {
	byResource := make(map[collaborationMentionRef]CollaborationMentionTarget, len(catalogue))
	for _, target := range catalogue {
		byResource[collaborationMentionRef{Kind: target.Kind, ResourceID: target.ResourceID}] = target
	}
	suppressed := make(map[pgtype.UUID]bool, len(input.SuppressedTargetIDs))
	for _, id := range input.SuppressedTargetIDs {
		suppressed[id] = true
	}
	result := make([]CollaborationCommentTargetResult, 0)
	for _, ref := range parseCollaborationMentions(input.Content).Targets {
		item := CollaborationCommentTargetResult{Kind: string(ref.Kind), ResourceID: ref.ResourceID, State: "blocked", Code: "target_unavailable"}
		target, ok := byResource[ref]
		if !ok {
			result = append(result, item)
			continue
		}
		item.TargetID, item.TargetEpoch, item.ConfigVersion = target.TargetID, target.TargetEpoch, target.ConfigVersion
		item.HoldReason = target.HoldReason
		holdVersion := target.HoldVersion
		item.HoldVersion = &holdVersion
		switch {
		case suppressed[target.TargetID]:
			item.State, item.Code = "suppressed", "target_suppressed"
		case !access.Capabilities.ControlAI:
			item.Code = "not_participant"
		case collaboration.TerminalCategory(access.StatusCategory) || access.QueueState == "closed":
			item.Code = "issue_closed"
		case !access.Capabilities.SubmitCommand:
			item.Code = "command_unavailable"
		default:
			item.State, item.Code, item.WaitingReason = "waiting", "command_queued", target.WaitingReason
		}
		result = append(result, item)
	}
	return result
}
