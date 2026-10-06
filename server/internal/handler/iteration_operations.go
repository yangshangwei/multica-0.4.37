package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func iterationAPIError(status int, code, message string) error {
	return &iteration.OperationError{Status: status, Code: code, Message: message}
}

func writeIterationAPIError(w http.ResponseWriter, err error) {
	var api *iteration.OperationError
	if !errors.As(err, &api) {
		api = &iteration.OperationError{Status: 503, Code: "iteration_unavailable", Message: "Iteration operation unavailable; retry the original request", Retryable: true}
	}
	if api.Status == 503 {
		w.Header().Set("Retry-After", "1")
	}
	writeJSON(w, api.Status, api)
}

func iterationWorkspaceScope(r *http.Request) (pgtype.UUID, pgtype.UUID, error) {
	ws, err := util.ParseUUID(workspaceIDFromURL(r, "id"))
	if err != nil || ws.Bytes == [16]byte{} {
		return ws, pgtype.UUID{}, iterationAPIError(400, "invalid_request", "Invalid workspace ID")
	}
	user, err := util.ParseUUID(requestUserID(r))
	if err != nil || user.Bytes == [16]byte{} {
		return ws, user, iterationAPIError(401, "unauthenticated", "User authentication required")
	}
	if selected := r.Header.Get("X-Workspace-ID"); selected != "" {
		selectedID, e := util.ParseUUID(selected)
		if e != nil || selectedID != ws {
			return ws, user, iterationAPIError(403, "forbidden", "Workspace scope mismatch")
		}
	}
	return ws, user, nil
}

// withIterationRead keeps current membership valid through the protected read.
// It does not take the writer fence or manufacture a settings row. A fresh RC
// statement after a revocation wait cannot restore a departed member's access.
func (h *Handler) withIterationRead(ctx context.Context, ws, user pgtype.UUID, read func(pgx.Tx, *db.Queries) error) error {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED, READ WRITE"); err != nil {
		return err
	}
	q := h.Queries.WithTx(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, ws); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return iterationAPIError(404, "workspace_not_found", "Workspace not found")
		}
		return err
	}
	if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: user}); err != nil {
		return err
	}
	if _, err = q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: ws, UserID: user}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return iterationAPIError(403, "forbidden", "Workspace membership required")
		}
		return err
	}
	if err = read(tx, q); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h *Handler) GetIterationOperation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, err := iterationWorkspaceScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	requestID, err := util.ParseUUID(chi.URLParam(r, "requestID"))
	if err != nil || requestID.Bytes == [16]byte{} {
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid request ID"))
		return
	}
	var result iteration.WriteResult
	err = h.withIterationRead(r.Context(), ws, actor, func(tx pgx.Tx, _ *db.Queries) error {
		var e error
		result, e = iteration.ReadOperation(r.Context(), tx, ws, actor, requestID)
		return e
	})
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
