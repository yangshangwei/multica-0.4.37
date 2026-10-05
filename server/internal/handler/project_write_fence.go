package handler

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var errProjectResponseWritten = errors.New("project response already written")

type projectAPIError struct {
	Status      int
	Code        string
	Message     string
	Current     any
	FieldErrors []map[string]string
}

func (e *projectAPIError) Error() string { return e.Message }
func projectErr(status int, code, message string) error {
	return &projectAPIError{Status: status, Code: code, Message: message}
}
func writeProjectAPIError(w http.ResponseWriter, err error) {
	var api *projectAPIError
	if !errors.As(err, &api) {
		api = &projectAPIError{Status: 503, Code: "project_write_retry_exhausted", Message: "project operation unavailable; retry the same request"}
	}
	body := map[string]any{"error": api.Message, "code": api.Code}
	if api.Current != nil {
		body["current"] = api.Current
	}
	if len(api.FieldErrors) > 0 {
		body["field_errors"] = api.FieldErrors
	}
	if api.Status == 503 {
		body["retryable"] = true
		w.Header().Set("Retry-After", "1")
	}
	writeJSON(w, api.Status, body)
}
func (h *Handler) projectHumanActor(r *http.Request, workspaceID string) (pgtype.UUID, error) {
	user := requestUserID(r)
	if user == "" {
		return pgtype.UUID{}, projectErr(401, "unauthenticated", "user not authenticated")
	}
	actor, _ := h.resolveActor(r, user, workspaceID)
	if isMachineCredentialActor(r) || actor != "member" {
		return pgtype.UUID{}, projectErr(403, "forbidden", "this operation requires a human member")
	}
	id, err := util.ParseUUID(user)
	if err != nil {
		return id, projectErr(400, "invalid_request", "invalid actor id")
	}
	return id, nil
}
func retryableProjectTransaction(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01" || pgErr.Code == "55P03")
}

// runProjectTransaction is the shared P1 transaction boundary. The isolation
// statement MUST precede every query. Locking membership reads after the
// revocation fence prevent an old RR snapshot from restoring revoked access.
// The callback may run again only after the entire prior transaction rolls back;
// it must not publish events or perform external effects before this returns.
func (h *Handler) runProjectTransaction(ctx context.Context, workspaceID, actorID pgtype.UUID, fn func(pgx.Tx, *db.Queries) error) error {
	return h.runProjectTransactionAtIsolation(ctx, workspaceID, actorID, pgx.RepeatableRead, fn)
}

// Deletion waits for association writers before sweeping their children. A
// snapshot taken before that wait can miss a child whose creator held only a
// shared project lock (and therefore never changed the project tuple version).
// READ COMMITTED sees every such committed child after the exclusive lock is
// acquired; the lock prevents any later association from entering the sweep.
func (h *Handler) runProjectDeleteTransaction(ctx context.Context, workspaceID, actorID pgtype.UUID, fn func(pgx.Tx, *db.Queries) error) error {
	return h.runProjectTransactionAtIsolation(ctx, workspaceID, actorID, pgx.ReadCommitted, fn)
}
func (h *Handler) runProjectTransactionAtIsolation(ctx context.Context, workspaceID, actorID pgtype.UUID, isolation pgx.TxIsoLevel, fn func(pgx.Tx, *db.Queries) error) error {
	for attempt := 0; attempt < 3; attempt++ {
		err := h.projectTransactionOnce(ctx, workspaceID, actorID, isolation, fn)
		if !retryableProjectTransaction(err) {
			return err
		}
		if attempt == 2 {
			break
		}
		delay := 25 * time.Millisecond
		if attempt == 1 {
			delay = 75 * time.Millisecond
		}
		timer := time.NewTimer(delay + time.Duration(rand.IntN(26))*time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return projectErr(503, "project_write_retry_exhausted", "project operation conflicted; retry the same request")
}
func (h *Handler) projectTransactionOnce(ctx context.Context, workspaceID, actorID pgtype.UUID, isolation pgx.TxIsoLevel, fn func(pgx.Tx, *db.Queries) error) error {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	isolationSQL := "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ WRITE"
	if isolation == pgx.ReadCommitted {
		isolationSQL = "SET TRANSACTION ISOLATION LEVEL READ COMMITTED, READ WRITE"
	}
	if _, err = tx.Exec(ctx, isolationSQL); err != nil {
		return err
	}
	q := h.Queries.WithTx(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return projectErr(404, "project_not_found", "workspace not found")
		}
		return err
	}
	if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: workspaceID, UserID: actorID}); err != nil {
		return err
	}
	if _, err = q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: workspaceID, UserID: actorID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return projectErr(403, "forbidden", "workspace membership required")
		}
		return err
	}
	if err = fn(tx, q); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
