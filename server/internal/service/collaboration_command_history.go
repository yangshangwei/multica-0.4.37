package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationCommandHistoryTarget struct {
	Kind       string      `json:"kind"`
	ResourceID pgtype.UUID `json:"resource_id"`
}

type CollaborationCommandCancellation struct {
	EventID     *string                      `json:"event_id"`
	EventSeq    *string                      `json:"event_seq"`
	ActorUserID *string                      `json:"actor_user_id"`
	Actor       *CollaborationContentAccount `json:"actor"`
	CreatedAt   *time.Time                   `json:"created_at"`
	Kind        *string                      `json:"kind"`
}

type CollaborationCommandHistoryEntry struct {
	ID            pgtype.UUID                       `json:"id"`
	SessionID     pgtype.UUID                       `json:"session_id"`
	TargetID      pgtype.UUID                       `json:"target_id"`
	Target        CollaborationCommandHistoryTarget `json:"target"`
	InputEventID  pgtype.UUID                       `json:"input_event_id"`
	InputEventSeq *string                           `json:"input_event_seq"`
	CommentID     *string                           `json:"comment_id"`
	ChannelID     pgtype.UUID                       `json:"channel_id"`
	BatchID       pgtype.UUID                       `json:"batch_id"`
	GroupID       pgtype.UUID                       `json:"group_id"`
	Seq           int64                             `json:"seq,string"`
	AuthorUserID  pgtype.UUID                       `json:"author_user_id"`
	Author        *CollaborationContentAccount      `json:"author"`
	Content       string                            `json:"content"`
	AttachmentIDs []pgtype.UUID                     `json:"attachment_ids"`
	State         string                            `json:"state"`
	Reason        *string                           `json:"reason"`
	CreatedAt     time.Time                         `json:"created_at"`
	Cancellation  *CollaborationCommandCancellation `json:"cancellation"`
}

type CollaborationCommandHistoryPage struct {
	ProtocolVersion int                                `json:"protocol_version"`
	IssueID         pgtype.UUID                        `json:"issue_id"`
	Items           []CollaborationCommandHistoryEntry `json:"items"`
	NextCursor      *string                            `json:"next_cursor"`
}

type CollaborationCommandHistoryQuery struct {
	Limit  int
	Cursor string
}

type collaborationCommandHistoryCursor struct {
	IssueID   pgtype.UUID `json:"issue_id"`
	CommandID pgtype.UUID `json:"command_id"`
	CreatedAt string      `json:"created_at"`
}

// CommandHistory includes commands that never acquired a run. Every page uses
// the same independent source-member/participant read bases as private content;
// this reader neither computes nor grants cancellation or execution authority.
func (s *CollaborationService) CommandHistory(ctx context.Context, issueID pgtype.UUID, p CollaborationCommandHistoryQuery) (CollaborationCommandHistoryPage, error) {
	out := CollaborationCommandHistoryPage{ProtocolVersion: 1, IssueID: issueID, Items: []CollaborationCommandHistoryEntry{}}
	invalidQuery := collaborationError(http.StatusBadRequest, "invalid_command_history_query")
	if !issueID.Valid || p.Limit < 1 || p.Limit > 100 || len(p.Cursor) > 1024 {
		return out, invalidQuery
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		query := db.ListCollaborationCommandHistoryParams{IssueID: issueID, SessionID: access.SessionID, RowLimit: int32(p.Limit + 1)}
		if p.Cursor != "" {
			raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
			var cursor collaborationCommandHistoryCursor
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err != nil || iteration.ValidateObjectJSON(raw) != nil || decoder.Decode(&cursor) != nil || cursor.IssueID != issueID || !cursor.CommandID.Valid {
				return invalidQuery
			}
			created, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt)
			if err != nil || created.IsZero() || created.UTC().Format(time.RFC3339Nano) != cursor.CreatedAt {
				return invalidQuery
			}
			query.BeforeCreatedAt, query.BeforeID = pgtype.Timestamptz{Time: created, Valid: true}, cursor.CommandID
			exists, err := q.CollaborationCommandHistoryCursorExists(ctx, db.CollaborationCommandHistoryCursorExistsParams{
				SessionID: access.SessionID, CommandID: cursor.CommandID, CreatedAt: query.BeforeCreatedAt,
			})
			if err != nil {
				return err
			}
			if !exists {
				return invalidQuery
			}
		}
		rows, err := q.ListCollaborationCommandHistory(ctx, query)
		if err != nil {
			return err
		}
		hasMore := len(rows) > p.Limit
		if hasMore {
			rows = rows[:p.Limit]
		}
		users := make([]pgtype.UUID, 0, len(rows)*2)
		for _, row := range rows {
			users = append(users, row.CollaborationCommand.AuthorUserID, row.CancellationActorID)
		}
		accounts, err := collaborationContentAccounts(ctx, q, access.Issue.WorkspaceID, users)
		if err != nil {
			return err
		}
		for _, row := range rows {
			entry, err := collaborationCommandHistoryEntry(row, accounts)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, entry)
		}
		if hasMore {
			last := rows[len(rows)-1].CollaborationCommand
			raw, err := json.Marshal(collaborationCommandHistoryCursor{IssueID: issueID, CommandID: last.ID, CreatedAt: last.CreatedAt.Time.UTC().Format(time.RFC3339Nano)})
			if err != nil {
				return err
			}
			cursor := base64.RawURLEncoding.EncodeToString(raw)
			out.NextCursor = &cursor
		}
		return nil
	})
	return out, err
}

func collaborationCommandHistoryEntry(row db.ListCollaborationCommandHistoryRow, accounts map[pgtype.UUID]*CollaborationContentAccount) (CollaborationCommandHistoryEntry, error) {
	c := row.CollaborationCommand
	entry := CollaborationCommandHistoryEntry{
		ID: c.ID, SessionID: c.SessionID, TargetID: c.TargetID,
		Target:       CollaborationCommandHistoryTarget{Kind: row.TargetKind.String, ResourceID: row.ResourceID},
		InputEventID: c.InputEventID, ChannelID: c.ChannelID, BatchID: c.BatchID, GroupID: c.GroupID, Seq: c.Seq,
		AuthorUserID: c.AuthorUserID, Author: accounts[c.AuthorUserID], Content: c.Content, AttachmentIDs: []pgtype.UUID{},
		State: c.State, Reason: collaborationCommandHistoryReason(c.Reason), CreatedAt: c.CreatedAt.Time,
	}
	invalid := func() (CollaborationCommandHistoryEntry, error) {
		return CollaborationCommandHistoryEntry{}, invalidCollaborationLedger("command history evidence invalid")
	}
	if !c.ID.Valid || !c.SessionID.Valid || !c.TargetID.Valid || !c.InputEventID.Valid || !c.ChannelID.Valid || !c.BatchID.Valid || !c.AuthorUserID.Valid || c.Seq < 1 || !c.CreatedAt.Valid || !row.ResourceID.Valid || (entry.Target.Kind != "agent" && entry.Target.Kind != "squad") {
		return invalid()
	}
	switch c.State {
	case "waiting", "in_flight", "succeeded", "failed", "stopped", "cancelled":
	default:
		return invalid()
	}
	if json.Unmarshal(c.Attachments, &entry.AttachmentIDs) != nil || entry.AttachmentIDs == nil || validateCollaborationAttachmentIDs(entry.AttachmentIDs) != nil {
		return invalid()
	}
	if row.InputEventSeq.Valid {
		var source collaborationCommentHistoryPayload
		if row.InputEventSeq.Int64 < 1 || json.Unmarshal(row.InputPayload, &source) != nil || !source.CommentID.Valid || source.After != c.Content || !sameCollaborationAttachmentIDs(source.AfterAttachmentIDs, entry.AttachmentIDs) {
			return invalid()
		}
		seq := strconv.FormatInt(row.InputEventSeq.Int64, 10)
		entry.InputEventSeq, entry.CommentID = &seq, util.UUIDToPtr(source.CommentID)
	}
	if c.State == "cancelled" {
		// Old bulk cancellations may have no exact event. Preserve the reason,
		// while actor, time and event identity explicitly remain unknown.
		entry.Cancellation = &CollaborationCommandCancellation{}
	}
	if c.CancellationEventID.Valid {
		var facts collaborationCommandCancellationEvent
		if c.State != "cancelled" || c.CancellationEventID != row.CancellationID || !row.CancellationTime.Valid || !row.CancellationSeq.Valid || row.CancellationSeq.Int64 < 1 || json.Unmarshal(row.CancellationPayload, &facts) != nil || facts.CommandID != c.ID || facts.TargetID != c.TargetID || facts.ChannelID != c.ChannelID || facts.BatchID != c.BatchID || facts.CommandSequence != c.Seq {
			return invalid()
		}
		kind := facts.CancellationKind
		if kind == "" && c.Reason.String == "cancelled_by_participant" && facts.Action == "cancel_command" && facts.Code == "command_cancelled" && row.CancellationActorID.Valid && facts.Reason == "" {
			// Migration 735 links only a singleton, fully correlated old manual
			// receipt. Its absence of a new kind/reason is not a generic fallback.
			kind = "manual"
		} else if (kind != "manual" && kind != "automatic") || facts.Reason != c.Reason.String || facts.GroupID != c.GroupID {
			return invalid()
		}
		if kind == "manual" && !row.CancellationActorID.Valid {
			return invalid()
		}
		seq := strconv.FormatInt(row.CancellationSeq.Int64, 10)
		entry.Cancellation = &CollaborationCommandCancellation{
			EventID: util.UUIDToPtr(row.CancellationID), EventSeq: &seq, ActorUserID: util.UUIDToPtr(row.CancellationActorID),
			Actor: accounts[row.CancellationActorID], CreatedAt: &row.CancellationTime.Time, Kind: &kind,
		}
	}
	return entry, nil
}
