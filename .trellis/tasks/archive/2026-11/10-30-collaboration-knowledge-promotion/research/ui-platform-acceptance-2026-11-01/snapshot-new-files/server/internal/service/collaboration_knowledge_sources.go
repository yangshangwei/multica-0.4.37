package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// KnowledgeSourceAvailability borrows an authorized transaction. It grants no
// read authority and takes no hidden locks. Callers must stabilize the source
// before publishing or disclosing a derived body. Missing/purged/unrestorable
// references are unavailable; infrastructure errors never become empty success.
func (s *CollaborationService) KnowledgeSourceAvailability(ctx context.Context, q *db.Queries, issueID pgtype.UUID, required []protocol.CollaborationKnowledgeSource) (bool, error) {
	if q == nil {
		return false, knowledgeFailure(503, "knowledge_unavailable")
	}
	if !issueID.Valid || len(required) == 0 || len(required) > protocol.CollaborationKnowledgeMaxReferences {
		return false, nil
	}
	issue, err := q.GetIssue(ctx, issueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	shared, err := q.IsCollaborationIssue(ctx, issueID)
	if err != nil || !shared {
		return false, err
	}
	for _, ref := range required {
		if ref.IssueID != util.UUIDToString(issueID) {
			return false, nil
		}
		body, available, err := knowledgeSourceText(ctx, q, issue, ref)
		if err != nil {
			return false, err
		}
		if !available || protocol.CollaborationKnowledgeHash(body) != ref.SHA256 {
			return false, nil
		}
	}
	return true, nil
}

func knowledgeSourceText(ctx context.Context, q *db.Queries, issue db.Issue, ref protocol.CollaborationKnowledgeSource) (string, bool, error) {
	switch ref.Kind {
	case "memory":
		version, err := strconv.ParseInt(ref.Version, 10, 64)
		if err != nil || version < 1 {
			return "", false, nil
		}
		row, err := q.GetCollaborationMemoryVersion(ctx, db.GetCollaborationMemoryVersionParams{IssueID: issue.ID, Version: version})
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		return row.Content, !row.PurgedAt.Valid, nil
	case "comment":
		id, err := util.ParseUUID(ref.CommentID)
		if err != nil {
			return "", false, nil
		}
		version, err := strconv.ParseInt(ref.Version, 10, 64)
		if err != nil || version < 1 {
			return "", false, nil
		}
		comment, err := q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{CommentID: id, IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		if comment.DeletedAt.Valid {
			return "", false, nil
		}
		if comment.Revision == version {
			return comment.Content, true, nil
		}
		if version > comment.Revision {
			return "", false, nil
		}
		// Historical edits retain their exact after/before version in the
		// source event. A tombstone above suppresses all of those old copies.
		next := ""
		if version < 9223372036854775807 {
			next = strconv.FormatInt(version+1, 10)
		}
		events, err := q.GetKnowledgeCommentVersionEvents(ctx, db.GetKnowledgeCommentVersionEventsParams{IssueID: issue.ID, CommentID: ref.CommentID, Version: ref.Version, NextVersion: next})
		if err != nil {
			return "", false, err
		}
		if len(events) > 2 {
			return "", false, nil
		}
		var found *string
		for _, event := range events {
			var p collaborationCommentHistoryPayload
			if json.Unmarshal(event.Payload, &p) != nil || p.CommentID != id {
				return "", false, nil
			}
			var body *string
			if p.CommentRevision == version {
				body = &p.After
			} else if p.CommentRevision == version+1 {
				body = p.Before
			}
			if body != nil {
				if found != nil && *found != *body {
					return "", false, nil
				}
				value := *body
				found = &value
			}
		}
		if found != nil {
			return *found, true, nil
		}
		return "", false, nil
	case "run_record":
		return knowledgeRunRecordText(ctx, q, issue.ID, ref)
	default:
		return "", false, nil
	}
}

func knowledgeRunRecordText(ctx context.Context, q *db.Queries, issueID pgtype.UUID, ref protocol.CollaborationKnowledgeSource) (string, bool, error) {
	runID, err := util.ParseUUID(ref.RunID)
	if err != nil {
		return "", false, nil
	}
	run, err := q.GetCollaborationRecordedRun(ctx, db.GetCollaborationRecordedRunParams{IssueID: issueID, RunID: runID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if strings.HasPrefix(ref.RecordID, "input:") {
		var inputs []protocol.CollaborationRunInput
		if len(run.PromptSnapshot) > 4<<20 || json.Unmarshal(run.PromptSnapshot, &inputs) != nil || len(inputs) > 1024 {
			return "", false, nil
		}
		for _, input := range inputs {
			if "input:"+input.CommandID == ref.RecordID {
				return input.Body, true, nil
			}
		}
		return "", false, nil
	}
	var events []db.CollaborationRunEvent
	parts := strings.Split(ref.RecordID, ":")
	if len(parts) != 3 {
		return "", false, nil
	}
	switch parts[0] {
	case "event":
		id, err := util.ParseUUID(parts[1])
		if err != nil {
			return "", false, nil
		}
		event, err := q.GetCollaborationHandoffSourceEvent(ctx, db.GetCollaborationHandoffSourceEventParams{IssueID: issueID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		if event.RunID != runID {
			return "", false, nil
		}
		events = []db.CollaborationRunEvent{event}
	case "record":
		through, err := q.GetCollaborationRunRecordSequence(ctx, db.GetCollaborationRunRecordSequenceParams{IssueID: issueID, RunID: runID})
		if err != nil {
			return "", false, err
		}
		events, err = q.ListCollaborationRecordingEvents(ctx, db.ListCollaborationRecordingEventsParams{IssueID: issueID, RunID: runID, ThroughSeq: through})
		if err != nil {
			return "", false, err
		}
	default:
		return "", false, nil
	}
	var found *string
	for _, event := range events {
		if event.Kind == "recording_seal" {
			continue
		}
		items, err := collaborationRecordEventItems(event)
		if err != nil {
			return "", false, nil
		}
		for _, item := range items {
			if item.ID == ref.RecordID {
				if found != nil && *found != item.Body {
					return "", false, nil
				}
				value := item.Body
				found = &value
			}
		}
	}
	if found != nil {
		return *found, true, nil
	}
	return "", false, nil
}
