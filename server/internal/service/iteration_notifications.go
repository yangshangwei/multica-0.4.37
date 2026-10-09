package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// EnqueueIterationNotifications borrows the business transaction. Recipient
// references must already be authorized and locked by the lifecycle owner.
func EnqueueIterationNotifications(ctx context.Context, q *db.Queries, workspaceID, iterationID, operationID pgtype.UUID, kind string, recipients []string) error {
	switch kind {
	case "start", "end", "cancel", "dates_changed", "disable":
	default:
		return errors.New("unsupported iteration notification kind")
	}
	for _, recipient := range recipients {
		id, err := util.ParseUUID(recipient)
		if err != nil {
			return err
		}
		if err = q.CreateIterationNotification(ctx, db.CreateIterationNotificationParams{WorkspaceID: workspaceID, IterationID: iterationID, OperationID: operationID, RecipientUserID: id, Kind: kind}); err != nil {
			return err
		}
	}
	return nil
}

// RunIterationNotifications recovers persisted work after restart. It never
// invokes agents or sends external messages, and never changes lifecycle state.
func (s *IterationService) RunIterationNotifications(ctx context.Context, onDelivered func(db.InboxItem)) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	nextReminderScan := time.Time{}
	for {
		if ctx.Err() != nil {
			return
		}
		if now := time.Now().UTC(); !now.Before(nextReminderScan) {
			nextReminderScan = now.Add(time.Minute)
			if err := s.EnqueueIterationOverdueReminders(ctx, now); err != nil && ctx.Err() == nil {
				slog.WarnContext(ctx, "iteration reminder scan deferred")
			}
			if counts, err := s.IterationNotificationCounts(ctx); err == nil {
				slog.InfoContext(ctx, "iteration notification queue", "pending", counts.Pending, "dead_letter", counts.DeadLetter)
			}
		}
		if err := s.DeliverIterationNotifications(ctx, onDelivered); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "iteration notification scan deferred")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *IterationService) notificationTransaction(ctx context.Context, ws, recipient, iid pgtype.UUID, fn func(*db.Queries, db.Iteration, bool) error) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED, READ WRITE"); err != nil {
		return err
	}
	q := db.New(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, ws); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: recipient}); err != nil {
		return err
	}
	_, err = q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: ws, UserID: recipient})
	valid := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	rows, err := q.LockIterations(ctx, db.LockIterationsParams{WorkspaceID: ws, Column2: []pgtype.UUID{iid}})
	if err != nil {
		return err
	}
	var row db.Iteration
	if len(rows) == 1 {
		row = rows[0]
	}
	if err = fn(q, row, valid); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *IterationService) DeliverIterationNotifications(ctx context.Context, onDelivered func(db.InboxItem)) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	candidates, err := db.New(tx).ListDueIterationNotifications(ctx)
	_ = tx.Rollback(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		var inbox *db.InboxItem
		single, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = s.notificationTransaction(single, candidate.WorkspaceID, candidate.RecipientUserID, candidate.IterationID, func(q *db.Queries, i db.Iteration, valid bool) error {
			row, err := q.LockIterationNotification(single, db.LockIterationNotificationParams{WorkspaceID: candidate.WorkspaceID, ID: candidate.ID})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			if row.RecipientUserID != candidate.RecipientUserID || row.IterationID != candidate.IterationID {
				return nil
			}
			if !valid {
				return q.DeleteIterationNotificationsForMember(single, db.DeleteIterationNotificationsForMemberParams{WorkspaceID: row.WorkspaceID, UserID: row.RecipientUserID})
			}
			obsolete := !i.ID.Valid && row.Kind != "disable"
			if row.Kind == "overdue" && !obsolete {
				zone, err := time.LoadLocation(i.Timezone)
				if err != nil {
					return err
				}
				settings, err := q.GetIterationSettings(single, i.WorkspaceID)
				if err != nil {
					return err
				}
				obsolete = !settings.Enabled || i.Status != "active" || reminderRecipient(i) != row.RecipientUserID || time.Now().In(zone).Format(time.DateOnly) <= i.EndDate.Time.Format(time.DateOnly)
			}
			if obsolete {
				return q.CompleteIterationNotification(single, db.CompleteIterationNotificationParams{WorkspaceID: row.WorkspaceID, ID: row.ID, Status: "suppressed", LastErrorCode: pgtype.Text{String: "reminder_obsolete", Valid: true}})
			}
			title := i.Name
			payload := map[string]string{"kind": row.Kind}
			if row.Kind == "disable" {
				title = "Iterations disabled"
			} else {
				payload["iteration_id"] = util.UUIDToString(i.ID)
			}
			details, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			item, err := q.CreateIterationInbox(single, db.CreateIterationInboxParams{ID: row.ID, WorkspaceID: row.WorkspaceID, RecipientID: row.RecipientUserID, Title: title, Details: details})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil {
				inbox = &item
			}
			return q.CompleteIterationNotification(single, db.CompleteIterationNotificationParams{WorkspaceID: row.WorkspaceID, ID: row.ID, Status: "delivered"})
		})
		cancel()
		if err == nil && inbox != nil && onDelivered != nil {
			onDelivered(*inbox)
		}
		if err != nil {
			retryCtx, retryCancel := context.WithTimeout(ctx, 5*time.Second)
			retryErr := s.notificationTransaction(retryCtx, candidate.WorkspaceID, candidate.RecipientUserID, candidate.IterationID, func(q *db.Queries, _ db.Iteration, valid bool) error {
				if !valid {
					return q.DeleteIterationNotificationsForMember(retryCtx, db.DeleteIterationNotificationsForMemberParams{WorkspaceID: candidate.WorkspaceID, UserID: candidate.RecipientUserID})
				}
				return q.RetryIterationNotification(retryCtx, db.RetryIterationNotificationParams{WorkspaceID: candidate.WorkspaceID, ID: candidate.ID})
			})
			retryCancel()
			if retryErr != nil {
				return retryErr
			}
		}
	}
	return nil
}

func reminderRecipient(i db.Iteration) pgtype.UUID {
	if i.CoordinatorUserID.Valid {
		return i.CoordinatorUserID
	}
	return i.StartedBy
}

func (s *IterationService) EnqueueIterationOverdueReminders(ctx context.Context, now time.Time) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	candidates, err := db.New(tx).ListActiveIterationsForReminder(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	_ = tx.Rollback(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		recipient := reminderRecipient(candidate)
		if !recipient.Valid {
			continue
		}
		err = s.notificationTransaction(ctx, candidate.WorkspaceID, recipient, candidate.ID, func(q *db.Queries, i db.Iteration, valid bool) error {
			if !valid || i.Status != "active" || reminderRecipient(i) != recipient {
				return nil
			}
			zone, err := time.LoadLocation(i.Timezone)
			if err != nil {
				return err
			}
			local := now.In(zone).Format(time.DateOnly)
			if local <= i.EndDate.Time.Format(time.DateOnly) {
				return nil
			}
			date, err := time.Parse(time.DateOnly, local)
			if err != nil {
				return err
			}
			return q.CreateIterationNotification(ctx, db.CreateIterationNotificationParams{WorkspaceID: i.WorkspaceID, IterationID: i.ID, OperationID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, RecipientUserID: recipient, Kind: "overdue", LocalDate: pgtype.Date{Time: date, Valid: true}})
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// IterationNotificationCounts exposes bounded, content-free outbox health.
func (s *IterationService) IterationNotificationCounts(ctx context.Context) (db.GetIterationNotificationCountsRow, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return db.GetIterationNotificationCountsRow{}, err
	}
	defer tx.Rollback(ctx)
	return db.New(tx).GetIterationNotificationCounts(ctx)
}
