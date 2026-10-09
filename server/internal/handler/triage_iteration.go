package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The T1 owner already holds its workspace/settings/member/catalog/I1 fences.
// Resolve the optional target before locking the pending issue so acceptance
// and its new membership use the same global iteration -> issue order.
func (h *Handler) lockTriageIterationTarget(ctx context.Context, tx pgx.Tx, ws pgtype.UUID, in TriageActionInput) (db.Iteration, error) {
	raw, present := in.Fields["current_iteration_id"]
	if !present {
		return db.Iteration{}, nil
	}
	if in.Action != "accept" && in.Action != "accept_and_execute" {
		return db.Iteration{}, triageErr(400, "iteration assignment is only supported when accepting")
	}
	var target *string
	if err := json.Unmarshal(raw, &target); err != nil {
		return db.Iteration{}, triageErr(400, "invalid current_iteration_id")
	}
	if target == nil {
		return db.Iteration{}, nil
	}
	id, err := triageUUID(*target, "current_iteration_id")
	if err != nil {
		return db.Iteration{}, err
	}
	q := h.Queries.WithTx(tx)
	settings, err := q.LockIterationSettings(ctx, ws)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !settings.Enabled {
		return db.Iteration{}, triageErr(409, "iterations are disabled")
	}
	if err != nil {
		return db.Iteration{}, err
	}
	rows, err := q.LockIterations(ctx, db.LockIterationsParams{WorkspaceID: ws, Column2: []pgtype.UUID{id}})
	if err != nil {
		return db.Iteration{}, err
	}
	if len(rows) != 1 || rows[0].Status != "planned" && rows[0].Status != "active" {
		return db.Iteration{}, triageErr(409, "iteration target is unavailable")
	}
	return rows[0], nil
}

func (h *Handler) recordTriageIterationMembership(ctx context.Context, tx pgx.Tx, r *http.Request, change *iteration.MembershipChange, actionID pgtype.UUID, sample time.Time) error {
	actor, err := json.Marshal(map[string]any{"type": "member", "id": requestUserID(r), "user_id": requestUserID(r), "source": "triage_accept"})
	if err != nil {
		return err
	}
	_, err = iteration.CommitMembershipChange(ctx, tx, change, actor, actionID, sample)
	return triageIterationError(err)
}

func triageIterationError(err error) error {
	var problem *iteration.OperationError
	if errors.As(err, &problem) {
		return triageErr(problem.Status, problem.Message)
	}
	return err
}
func (h *Handler) triageIterationAssignmentSupported(ctx context.Context, q *db.Queries, ws pgtype.UUID) (bool, error) {
	settings, err := q.GetIterationSettings(ctx, ws)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return settings.Enabled, nil
}
