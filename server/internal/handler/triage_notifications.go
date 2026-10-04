package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// Stable event/recipient identity is persisted with the decision. Delivery can
// fail or restart independently without replaying admission or duplicating inbox.
func (h *Handler) queueTriageNotification(ctx context.Context, tx pgx.Tx, ws, recipient, issue, batch pgtype.UUID, eventKey, title string, details map[string]string, due time.Time) error {
	if !recipient.Valid {
		return nil
	}
	if details == nil {
		details = map[string]string{}
	}
	details["triage_event"] = eventKey
	if issue.Valid {
		details["issue_id"] = uuidToString(issue)
	}
	if batch.Valid {
		details["batch_id"] = uuidToString(batch)
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	// Immediate notices use the database transaction clock. A Go timestamp
	// can be slightly ahead of PostgreSQL and miss the first delivery scan.
	deadline := pgtype.Timestamptz{}
	if due.After(time.Now()) {
		deadline = pgtype.Timestamptz{Time: due, Valid: true}
	}
	_, err = tx.Exec(ctx, `INSERT INTO triage_notification(id,workspace_id,recipient_id,event_key,issue_id,batch_id,title,details,due_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,COALESCE($9::timestamptz,now())) ON CONFLICT(workspace_id,recipient_id,event_key) DO UPDATE SET title=EXCLUDED.title,details=EXCLUDED.details,delivered_at=CASE WHEN triage_notification.details IS DISTINCT FROM EXCLUDED.details OR triage_notification.title IS DISTINCT FROM EXCLUDED.title THEN NULL ELSE triage_notification.delivered_at END`, dbid.NewV7(), ws, recipient, eventKey, issue, batch, title, raw, deadline)
	return err
}
func (h *Handler) triageActionNotifications(ctx context.Context, tx pgx.Tx, ws, actor pgtype.UUID, s db.WorkspaceTriageSetting, before TriageItem, out TriageActionResult) error {
	issue := parseUUID(out.Item.Issue.ID)
	action := out.Action
	recipients := map[pgtype.UUID]bool{}
	title := "Triage: " + action.Action + " — " + out.Item.Issue.Title
	due := time.Now()
	key := "action:" + action.ID
	details := map[string]string{"action": action.Action, "actor_id": uuidToString(actor)}
	switch action.Action {
	case "accept", "accept_and_execute", "reject", "duplicate":
		if out.Item.Issue.CreatorType == "member" {
			recipients[parseUUID(out.Item.Issue.CreatorID)] = true
		}
		rows, err := tx.Query(ctx, `SELECT user_id FROM issue_subscriber WHERE issue_id=$1 AND user_type='member' AND unsubscribed_at IS NULL AND reason<>'delegated'`, issue)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id pgtype.UUID
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			recipients[id] = true
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return err
		}
	case "reopen":
		if s.ResponsibilityMode != "none" && s.ResponsibilityMemberID.Valid {
			recipients[s.ResponsibilityMemberID] = true
		}
		key = fmt.Sprintf("entered:%s:%d", out.Item.Issue.ID, out.Item.Round)
	case "assign_reviewer":
		if out.Item.ReviewerID != nil && (before.ReviewerID == nil || *out.Item.ReviewerID != *before.ReviewerID) {
			recipients[parseUUID(*out.Item.ReviewerID)] = true
		}
	case "snooze":
		if out.Item.ReviewerID != nil && out.Item.SnoozedUntil != nil {
			recipients[parseUUID(*out.Item.ReviewerID)] = true
			due, _ = time.Parse(time.RFC3339Nano, *out.Item.SnoozedUntil)
			details["snoozed_until"] = *out.Item.SnoozedUntil
			key = "snooze:" + action.ID
			title = "Triage item ready for review — " + out.Item.Issue.Title
		}
	}
	if action.Action == "assign_reviewer" && out.Item.ReviewerID != nil && out.Item.SnoozedUntil != nil && (before.ReviewerID == nil || *before.ReviewerID != *out.Item.ReviewerID) {
		snoozedUntil, err := time.Parse(time.RFC3339Nano, *out.Item.SnoozedUntil)
		if err == nil && snoozedUntil.After(time.Now()) {
			var snoozeID pgtype.UUID
			err = tx.QueryRow(ctx, `SELECT id FROM triage_action WHERE issue_id=$1 AND action='snooze' ORDER BY created_at DESC,id DESC LIMIT 1`, issue).Scan(&snoozeID)
			if err != nil && err != pgx.ErrNoRows {
				return err
			}
			if err == nil {
				if err = h.queueTriageNotification(ctx, tx, ws, parseUUID(*out.Item.ReviewerID), issue, pgtype.UUID{}, "snooze:"+uuidToString(snoozeID), "Triage item ready for review — "+out.Item.Issue.Title, map[string]string{"snoozed_until": *out.Item.SnoozedUntil}, snoozedUntil); err != nil {
					return err
				}
			}
		}
	}
	for recipient := range recipients {
		if recipient == actor && action.Action != "snooze" {
			continue
		}
		if err := h.queueTriageNotification(ctx, tx, ws, recipient, issue, pgtype.UUID{}, key, title, details, due); err != nil {
			return err
		}
	}
	return nil
}
func (h *Handler) deliverTriageNotifications(ctx context.Context, ws pgtype.UUID) {
	if err := h.deliverTriageNotificationsOnce(ctx, ws); err != nil {
		slog.WarnContext(ctx, "triage notification delivery deferred", "workspace_id", uuidToString(ws), "error", err)
	}
}
func (h *Handler) deliverTriageNotificationsOnce(ctx context.Context, ws pgtype.UUID) error {
	if !ws.Valid {
		rows, err := h.DB.Query(ctx, `SELECT DISTINCT workspace_id FROM triage_notification WHERE delivered_at IS NULL AND due_at<=now() LIMIT 100`)
		if err != nil {
			return err
		}
		ids := []pgtype.UUID{}
		for rows.Next() {
			var id pgtype.UUID
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			if err = h.deliverTriageNotificationsOnce(ctx, id); err != nil {
				return err
			}
		}
		return nil
	}

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	if ws.Valid {
		if _, err = q.LockWorkspaceForChatSessionCreate(ctx, ws); err != nil {
			return err
		}
	}
	rows, err := tx.Query(ctx, `SELECT id,workspace_id,recipient_id,issue_id,batch_id,event_key,title,details FROM triage_notification WHERE delivered_at IS NULL AND due_at<=now() AND ($1::uuid IS NULL OR workspace_id=$1) ORDER BY due_at,id LIMIT 100 FOR UPDATE SKIP LOCKED`, ws)
	if err != nil {
		return err
	}
	type pending struct {
		id, workspace, recipient, issue, batch pgtype.UUID
		key, title                             string
		details                                []byte
	}
	items := []pending{}
	for rows.Next() {
		var i pending
		if err = rows.Scan(&i.id, &i.workspace, &i.recipient, &i.issue, &i.batch, &i.key, &i.title, &i.details); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	delivered := []db.InboxItem{}
	for _, i := range items {
		if !ws.Valid {
			if _, err = q.LockWorkspaceForChatSessionCreate(ctx, i.workspace); err != nil {
				return err
			}
		}
		var valid bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member WHERE workspace_id=$1 AND user_id=$2)`, i.workspace, i.recipient).Scan(&valid)
		if err != nil {
			return err
		}
		if i.issue.Valid {
			var available bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM issue WHERE id=$1 AND workspace_id=$2)`, i.issue, i.workspace).Scan(&available)
			if err != nil {
				return err
			}
			valid = valid && available
		}
		if strings.HasPrefix(i.key, "snooze:") {
			var details map[string]string
			if err = json.Unmarshal(i.details, &details); err != nil {
				return err
			}
			var current bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM issue_triage t JOIN issue iss ON iss.id=t.issue_id WHERE t.issue_id=$1 AND iss.admission_status='pending' AND t.reviewer_id=$2 AND t.snoozed_until=$3::timestamptz)`, i.issue, i.recipient, details["snoozed_until"]).Scan(&current)
			if err != nil {
				return err
			}
			valid = valid && current
		}
		if valid {
			// Same membership write fence as revocation; delivery cannot recreate a
			// removed member's notification after membership cleanup committed.
			// A batch retry takes its actor guard before updating this outbox
			// row. Never wait for that guard while holding the row ourselves.
			var acquired bool
			if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtext(($1::uuid)::text),hashtext(($2::uuid)::text))`, i.workspace, i.recipient).Scan(&acquired); err != nil {
				return err
			}
			if !acquired {
				continue
			}
			_, err = q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: i.workspace, UserID: i.recipient})
			if err == pgx.ErrNoRows {
				valid = false
			} else if err != nil {
				return err
			}
		}
		if valid {
			_, err = tx.Exec(ctx, `INSERT INTO inbox_item(id,workspace_id,recipient_type,recipient_id,type,severity,issue_id,title,body,actor_type,details) VALUES($1,$2,'member',$3,'triage','info',$4,$5,'','system',$6) ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title,details=EXCLUDED.details`, i.id, i.workspace, i.recipient, i.issue, i.title, i.details)
			if err != nil {
				return err
			}
		}
		if valid {
			item, e := q.GetInboxItem(ctx, i.id)
			if e != nil {
				return e
			}
			delivered = append(delivered, item)
		}
		if _, err = tx.Exec(ctx, `UPDATE triage_notification SET delivered_at=now() WHERE id=$1`, i.id); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	for _, item := range delivered {
		raw, e := json.Marshal(inboxToResponse(item))
		if e != nil {
			continue
		}
		var data map[string]any
		if json.Unmarshal(raw, &data) != nil {
			continue
		}
		h.publish(protocol.EventInboxNew, uuidToString(item.WorkspaceID), "system", "", map[string]any{"item": data})
	}
	return nil
}

// RunTriageNotifications recovers due and previously failed deliveries after a
// process restart. Queue visibility itself is always a read-time predicate.
func (h *Handler) RunTriageNotifications(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		h.deliverTriageNotifications(ctx, pgtype.UUID{})
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Batch summaries share the stable request-key set, so a transport retry updates
// the existing inbox row instead of creating a second summary.
func (h *Handler) triageBatchSummary(ctx context.Context, r *http.Request, key string, success, failed int) {
	tx, ws, actor, _, err := h.beginTriageWrite(ctx, r, true)
	if err != nil {
		slog.WarnContext(ctx, "triage batch summary deferred", "error", err)
		return
	}
	defer tx.Rollback(ctx)
	details := map[string]string{"success_count": strconv.Itoa(success), "failed_count": strconv.Itoa(failed)}
	err = h.queueTriageNotification(ctx, tx, ws, actor, pgtype.UUID{}, pgtype.UUID{}, "review-batch:"+key, "Triage batch completed", details, time.Now())
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		slog.WarnContext(ctx, "triage batch summary deferred", "error", err)
		return
	}
	h.deliverTriageNotifications(ctx, ws)
}
