package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/inboxstate"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type InboxSnoozeReceipt struct {
	WorkspaceID    string  `json:"workspace_id"`
	GroupKind      string  `json:"group_kind"`
	GroupID        string  `json:"group_id"`
	Revision       int64   `json:"revision"`
	SnoozedUntil   *string `json:"snoozed_until"`
	RemindedAt     *string `json:"reminded_at"`
	ReminderReason *string `json:"reminder_reason"`
	UndoToken      *string `json:"undo_token"`
}

func inboxTimePtr(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	text := value.Time.UTC().Format(time.RFC3339Nano)
	return &text
}

func inboxReasonPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func inboxSnoozeReceipt(state inboxstate.State) InboxSnoozeReceipt {
	return InboxSnoozeReceipt{WorkspaceID: uuidToString(state.WorkspaceID), GroupKind: state.GroupKind, GroupID: uuidToString(state.GroupID), Revision: state.Revision, SnoozedUntil: inboxTimePtr(state.SnoozedUntil), RemindedAt: inboxTimePtr(state.RemindedAt), ReminderReason: textToPtr(state.ReminderReason), UndoToken: uuidToPtr(state.UndoToken)}
}

func (h *Handler) publishInboxState(state inboxstate.State) {
	h.publish(protocol.EventInboxUpdated, uuidToString(state.WorkspaceID), "member", uuidToString(state.RecipientID), inboxstate.Payload(state))
}

func (h *Handler) ListSnoozedInbox(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ws, ok := parseUUIDOrBadRequest(w, ctxWorkspaceID(r.Context()), "workspace id")
	if !ok {
		return
	}
	items, err := h.Queries.ListSnoozedInboxItems(r.Context(), db.ListSnoozedInboxItemsParams{WorkspaceID: ws, RecipientType: "member", RecipientID: parseUUID(userID)})
	if err != nil {
		writeError(w, 500, "failed to list snoozed inbox")
		return
	}
	response := make([]InboxItemResponse, len(items))
	for index, item := range items {
		response[index] = inboxRowToResponse(db.ListInboxItemsRow(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) SnoozeInboxItem(w http.ResponseWriter, r *http.Request) {
	h.changeInboxSnooze(w, r, "snooze")
}
func (h *Handler) UnsnoozeInboxItem(w http.ResponseWriter, r *http.Request) {
	h.changeInboxSnooze(w, r, "unsnooze")
}
func (h *Handler) UndoInboxSnooze(w http.ResponseWriter, r *http.Request) {
	h.changeInboxSnooze(w, r, "undo")
}

func (h *Handler) changeInboxSnooze(w http.ResponseWriter, r *http.Request, action string) {
	previous, ok := h.loadInboxItemForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var request struct {
		Until     string `json:"snoozed_until"`
		Revision  *int64 `json:"expected_revision"`
		UndoToken string `json:"undo_token"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var trailing any
	if decoder.Decode(&request) != nil || decoder.Decode(&trailing) != io.EOF {
		writeError(w, 400, "invalid inbox snooze request")
		return
	}
	var until time.Time
	var token pgtype.UUID
	var err error
	if action == "undo" {
		if request.Revision != nil || request.Until != "" {
			writeError(w, 400, "invalid undo request")
			return
		}
		token, ok = parseUUIDOrBadRequest(w, request.UndoToken, "undo token")
		if !ok {
			return
		}
	} else {
		if request.Revision == nil || *request.Revision < 0 || request.UndoToken != "" {
			writeError(w, 400, "expected_revision is required")
			return
		}
		if action == "snooze" {
			until, err = time.Parse(time.RFC3339Nano, request.Until)
			if err != nil {
				writeError(w, 400, "invalid snoozed_until")
				return
			}
		} else if request.Until != "" {
			writeError(w, 400, "invalid unsnooze request")
			return
		}
	}
	if previous.Archived {
		writeError(w, 409, "archived notification cannot be snoozed")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to update inbox snooze")
		return
	}
	defer tx.Rollback(r.Context())
	state, err := inboxstate.Lock(r.Context(), tx, inboxstate.ForItem(previous))
	if err == nil {
		current, loadErr := db.New(tx).GetInboxItemInWorkspace(r.Context(), db.GetInboxItemInWorkspaceParams{ID: previous.ID, WorkspaceID: previous.WorkspaceID})
		if loadErr != nil {
			err = loadErr
		} else if current.RecipientID != previous.RecipientID || current.RecipientType != previous.RecipientType || current.Archived {
			err = pgx.ErrNoRows
		}
	}
	if err == nil {
		switch action {
		case "snooze":
			err = inboxstate.Snooze(r.Context(), tx, &state, until, *request.Revision)
		case "unsnooze":
			err = inboxstate.Unsnooze(r.Context(), tx, &state, *request.Revision)
		case "undo":
			err = inboxstate.Undo(r.Context(), tx, &state, token)
		}
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		switch {
		case errors.Is(err, inboxstate.ErrConflict):
			writeError(w, 409, "inbox snooze changed; refresh and retry")
		case errors.Is(err, inboxstate.ErrInvalidTime):
			writeError(w, 400, "snoozed_until must be in the future")
		case errors.Is(err, pgx.ErrNoRows):
			writeError(w, 404, "inbox item not found")
		default:
			writeError(w, 500, "failed to update inbox snooze")
		}
		return
	}
	h.publishInboxState(state)
	writeJSON(w, http.StatusOK, inboxSnoozeReceipt(state))
}

func (h *Handler) expireInboxSnoozesOnce(ctx context.Context) error {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	if err = inboxstate.CleanupUnavailable(ctx, tx); err != nil {
		tx.Rollback(ctx)
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	states, err := h.Queries.ListDueInboxSnoozes(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range states {
		tx, err := h.TxStarter.Begin(ctx)
		if err != nil {
			return err
		}
		state, err := inboxstate.Lock(ctx, tx, inboxstate.ForState(candidate))
		if errors.Is(err, pgx.ErrNoRows) {
			tx.Rollback(ctx)
			continue
		}
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		now, err := inboxstate.Now(ctx, tx)
		var changed bool
		if err == nil {
			changed, err = inboxstate.Expire(ctx, tx, &state, now)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if err != nil {
			return err
		}
		if changed {
			h.publishInboxState(state)
		}
	}
	return nil
}

func (h *Handler) RunInboxSnoozes(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		if err := h.expireInboxSnoozesOnce(ctx); err != nil {
			slog.WarnContext(ctx, "inbox snooze expiry deferred", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) changeInboxItemState(ctx context.Context, previous db.InboxItem, action string) (db.InboxItem, inboxstate.State, error) {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return db.InboxItem{}, inboxstate.State{}, err
	}
	defer tx.Rollback(ctx)
	state, err := inboxstate.Lock(ctx, tx, inboxstate.ForItem(previous))
	if err != nil {
		return db.InboxItem{}, state, err
	}
	q := h.Queries.WithTx(tx)
	current, err := q.GetInboxItemInWorkspace(ctx, db.GetInboxItemInWorkspaceParams{ID: previous.ID, WorkspaceID: previous.WorkspaceID})
	if err != nil {
		return db.InboxItem{}, state, err
	}
	if current.RecipientType != previous.RecipientType || current.RecipientID != previous.RecipientID {
		return db.InboxItem{}, state, pgx.ErrNoRows
	}
	now, err := inboxstate.Now(ctx, tx)
	if err != nil {
		return db.InboxItem{}, state, err
	}
	if _, err = inboxstate.Expire(ctx, tx, &state, now); err != nil {
		return db.InboxItem{}, state, err
	}
	var item db.InboxItem
	switch action {
	case "read", "unread":
		read := action == "read"
		if current.Read != read && state.UndoToken.Valid {
			inboxstate.InvalidateUndo(&state)
		}
		if read {
			item, err = q.MarkInboxRead(ctx, current.ID)
		} else {
			item, err = q.MarkInboxUnread(ctx, current.ID)
		}
	case "archive":
		item, err = q.ArchiveInboxItem(ctx, current.ID)
		if err == nil && item.IssueID.Valid {
			_, err = q.ArchiveInboxByIssue(ctx, db.ArchiveInboxByIssueParams{WorkspaceID: item.WorkspaceID, RecipientType: item.RecipientType, RecipientID: item.RecipientID, IssueID: item.IssueID})
		}
		inboxstate.Clear(&state)
	case "unarchive":
		item, err = q.UnarchiveInboxItem(ctx, current.ID)
		if err == nil && item.IssueID.Valid {
			_, err = q.UnarchiveInboxByIssue(ctx, db.UnarchiveInboxByIssueParams{WorkspaceID: item.WorkspaceID, RecipientType: item.RecipientType, RecipientID: item.RecipientID, IssueID: item.IssueID})
		}
		inboxstate.Clear(&state)
	}
	if err == nil {
		err = inboxstate.Save(ctx, tx, state)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return item, state, err
}

func (h *Handler) runInboxBulk(ctx context.Context, workspaceID, recipientID pgtype.UUID, change func(*db.Queries) (int64, error)) (int64, error) {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err = inboxstate.LockScope(ctx, tx, workspaceID, recipientID, "member"); err != nil {
		return 0, err
	}
	count, err := change(h.Queries.WithTx(tx))
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}
