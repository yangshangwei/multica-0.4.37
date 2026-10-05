package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// RunProjectUpdateNotifications recovers persisted notifications immediately
// after startup. Each candidate has its own bounded transaction and commits
// independently; a slow recipient cannot retain an entire locked queue batch.
func (h *Handler) RunProjectUpdateNotifications(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if e := h.deliverProjectUpdateNotifications(ctx); e != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "project update notification scan deferred", "code", projectNotificationErrorCode(e))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (h *Handler) deliverProjectUpdateNotifications(ctx context.Context) error {
	// Candidate discovery MUST remain lock-free. All locking starts in
	// projectNotificationTransaction, in the same order as deletion/revocation.
	candidates, e := h.Queries.ListDueProjectUpdateNotifications(ctx)
	if e != nil {
		return e
	}
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		single, cancel := context.WithTimeout(ctx, 5*time.Second)
		e = h.deliverProjectUpdateNotification(single, candidate)
		cancel()
		if e != nil && ctx.Err() == nil {
			retryCtx, retryCancel := context.WithTimeout(ctx, 5*time.Second)
			recordErr := h.retryProjectUpdateNotification(retryCtx, candidate, projectNotificationErrorCode(e))
			retryCancel()
			if recordErr != nil {
				slog.WarnContext(ctx, "project update notification retry deferred", "notification_id", uuidToString(candidate.ID), "code", projectNotificationErrorCode(recordErr))
			}
		}
	}
	return nil
}

// RC and strict lock order are deliberate. In particular, never acquire a
// recipient fence or project lock after locking an outbox row: deletion and
// revocation acquire those parents before removing inbox/outbox children.
func (h *Handler) projectNotificationTransaction(ctx context.Context, candidate db.ProjectUpdateNotification, fn func(*db.Queries, db.ProjectUpdateNotification, db.Project, bool) error) error {
	tx, e := h.TxStarter.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED, READ WRITE"); e != nil {
		return e
	}
	q := h.Queries.WithTx(tx)
	if _, e = q.LockWorkspaceForChatSessionCreate(ctx, candidate.WorkspaceID); errors.Is(e, pgx.ErrNoRows) {
		return nil
	} else if e != nil {
		return e
	}
	if e = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: candidate.WorkspaceID, UserID: candidate.RecipientUserID}); e != nil {
		return e
	}
	_, e = q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: candidate.WorkspaceID, UserID: candidate.RecipientUserID})
	valid := e == nil
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	p, e := q.LockProjectForAssociation(ctx, db.LockProjectForAssociationParams{ID: candidate.ProjectID, WorkspaceID: candidate.WorkspaceID})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	row, e := q.LockProjectUpdateNotification(ctx, db.LockProjectUpdateNotificationParams{ID: candidate.ID, WorkspaceID: candidate.WorkspaceID, ProjectID: candidate.ProjectID})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	// Candidate identity is untrusted after discovery; never deliver under the
	// locks of another recipient even if a future maintenance tool changes it.
	if row.RecipientUserID != candidate.RecipientUserID || row.UpdateID != candidate.UpdateID {
		return nil
	}
	if e = fn(q, row, p, valid); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (h *Handler) deliverProjectUpdateNotification(ctx context.Context, candidate db.ProjectUpdateNotification) error {
	var inbox *db.InboxItem
	e := h.projectNotificationTransaction(ctx, candidate, func(q *db.Queries, row db.ProjectUpdateNotification, p db.Project, valid bool) error {
		cancelReason := "recipient_unavailable"
		var revision db.ProjectUpdateRevision
		if valid {
			var e error
			revision, e = q.GetProjectUpdateRevision(ctx, db.GetProjectUpdateRevisionParams{WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, UpdateID: row.UpdateID, Revision: row.SourceRevision})
			if errors.Is(e, pgx.ErrNoRows) {
				valid = false
				cancelReason = "source_unavailable"
			} else if e != nil {
				return e
			}
		}
		if valid {
			valid = false
			for _, mention := range util.ParseMentions(revision.Body) {
				if mention.Type == "member" {
					id, e := util.ParseUUID(mention.ID)
					if e == nil && id == row.RecipientUserID {
						valid = true
						break
					}
				}
			}
			if !valid {
				cancelReason = "recipient_not_mentioned"
			}
		}
		if !valid {
			return q.CompleteProjectUpdateNotification(ctx, db.CompleteProjectUpdateNotificationParams{ID: row.ID, WorkspaceID: row.WorkspaceID, Status: "cancelled", LastErrorCode: pgtype.Text{String: cancelReason, Valid: true}})
		}
		details, e := json.Marshal(map[string]any{"project_id": uuidToString(row.ProjectID), "update_id": uuidToString(row.UpdateID), "revision": row.SourceRevision})
		if e != nil {
			return e
		}
		item, e := q.CreateProjectUpdateInbox(ctx, db.CreateProjectUpdateInboxParams{ID: row.ID, WorkspaceID: row.WorkspaceID, RecipientID: row.RecipientUserID, Title: "Project update — " + p.Title, ActorID: revision.EditorUserID, Details: details})
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if e == nil {
			inbox = &item
		}
		return q.CompleteProjectUpdateNotification(ctx, db.CompleteProjectUpdateNotificationParams{ID: row.ID, WorkspaceID: row.WorkspaceID, Status: "delivered"})
	})
	if e != nil {
		return e
	}
	// Publish only after the inbox and outbox have committed together. This is
	// the existing inbox event, never a comment/issue/task execution event.
	if inbox != nil {
		raw, e := json.Marshal(inboxToResponse(*inbox))
		if e != nil {
			return e
		}
		var data map[string]any
		if e = json.Unmarshal(raw, &data); e != nil {
			return e
		}
		h.publish(protocol.EventInboxNew, uuidToString(inbox.WorkspaceID), "member", uuidToString(inbox.ActorID), map[string]any{"item": data})
	}
	return nil
}
func projectNotificationBackoff(attempt int32, jitter time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 5 * time.Second
	for i := int32(1); i < attempt && delay < 15*time.Minute; i++ {
		delay *= 2
	}
	if delay > 15*time.Minute {
		delay = 15 * time.Minute
	}
	return delay + jitter
}
func projectNotificationErrorCode(e error) string {
	if errors.Is(e, context.DeadlineExceeded) {
		return "delivery_timeout"
	}
	if errors.Is(e, context.Canceled) {
		return "delivery_cancelled"
	}
	var pg *pgconn.PgError
	if errors.As(e, &pg) {
		return "database_" + pg.Code
	}
	return "delivery_failed"
}
func (h *Handler) retryProjectUpdateNotification(ctx context.Context, candidate db.ProjectUpdateNotification, code string) error {
	return h.projectNotificationTransaction(ctx, candidate, func(q *db.Queries, row db.ProjectUpdateNotification, _ db.Project, valid bool) error {
		if !valid {
			return q.CompleteProjectUpdateNotification(ctx, db.CompleteProjectUpdateNotificationParams{ID: row.ID, WorkspaceID: row.WorkspaceID, Status: "cancelled", LastErrorCode: pgtype.Text{String: "recipient_unavailable", Valid: true}})
		}
		next := time.Now().UTC().Add(projectNotificationBackoff(row.Attempts+1, time.Duration(rand.IntN(1001))*time.Millisecond))
		if e := q.RetryProjectUpdateNotification(ctx, db.RetryProjectUpdateNotificationParams{ID: row.ID, WorkspaceID: row.WorkspaceID, NextAttemptAt: pgtype.Timestamptz{Time: next, Valid: true}, LastErrorCode: pgtype.Text{String: code, Valid: true}}); e != nil {
			return e
		}
		if row.Attempts+1 >= 12 {
			slog.ErrorContext(ctx, "project update notification dead letter", "notification_id", uuidToString(row.ID), "attempts", row.Attempts+1, "code", code)
		}
		return nil
	})
}
