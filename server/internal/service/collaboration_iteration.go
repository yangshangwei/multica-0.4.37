package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationIterationParams struct {
	CollaborationWorkflowRequest
	CurrentIterationID        pgtype.UUID
	ExpectedIssueRevision     int64
	ExpectedIterationRevision int64
	Reason                    string
	AllowCompleted            bool `json:",omitempty"`
}

func (t *collaborationWorkflowTx) lockIterationAssignment(ctx context.Context, workspace, issueID, target pgtype.UUID) error {
	if err := t.q.EnsureIterationSettings(ctx, workspace); err != nil {
		return err
	}
	settings, err := t.q.LockIterationSettings(ctx, workspace)
	if err != nil {
		return err
	}
	t.iterationsEnabled = settings.Enabled
	ids, err := t.q.ListIssueDeleteIterationIDs(ctx, db.ListIssueDeleteIterationIDsParams{WorkspaceID: workspace, IssueIds: []pgtype.UUID{issueID}})
	if err != nil {
		return err
	}
	if target.Valid && (len(ids) == 0 || ids[0] != target) {
		ids = append(ids, target)
	}
	rows, err := t.q.LockIterations(ctx, db.LockIterationsParams{WorkspaceID: workspace, Column2: ids})
	if err != nil {
		return err
	}
	t.iterations = make(map[pgtype.UUID]db.Iteration, len(rows))
	for _, row := range rows {
		t.iterations[row.ID] = row
	}
	return nil
}

func (s *CollaborationService) ChangeIteration(ctx context.Context, p CollaborationIterationParams) (CollaborationWorkflowReceipt, error) {
	p.targetIteration = p.CurrentIterationID
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, "change_iteration", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		before := t.access.Issue
		if before.Revision != p.ExpectedIssueRevision {
			return t.result("stale_revision", 409), nil
		}
		if !t.iterationsEnabled {
			return t.result("iteration_disabled", 409), nil
		}
		target := t.iterations[p.CurrentIterationID]
		if p.CurrentIterationID.Valid {
			if !target.ID.Valid {
				return t.result("iteration_not_found", 409), nil
			}
			if p.ExpectedIterationRevision < 1 || target.Revision != p.ExpectedIterationRevision {
				return t.result("iteration_revision_conflict", 409), nil
			}
		} else if p.ExpectedIterationRevision != 0 {
			return t.result("invalid_request", 400), nil
		}
		if before.CurrentIterationID == p.CurrentIterationID {
			return t.result("unchanged", 200), nil
		}
		reason := strings.TrimSpace(strings.ReplaceAll(p.Reason, "\r\n", "\n"))
		if len(reason) > 2000 || before.CurrentIterationID.Valid && reason == "" {
			return t.result("iteration_validation_failed", 422), nil
		}
		change, err := iteration.PrepareMembershipChange(ctx, t.tx, before, t.iterations[before.CurrentIterationID], target, p.AllowCompleted)
		if err != nil {
			var refusal *iteration.OperationError
			if errors.As(err, &refusal) {
				return t.result(refusal.Code, refusal.Status), nil
			}
			return CollaborationWorkflowReceipt{}, err
		}
		if reason != "" {
			change.Reason = &reason
		}
		actor, err := json.Marshal(map[string]any{"type": "member", "id": t.actor, "user_id": t.actor})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		sampled, err := iteration.SampleBusinessTime(ctx, t.tx, nil)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		after, err := iteration.CommitMembershipChange(ctx, t.tx, change, actor, t.operation.ID, sampled)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.access.Issue = after
		t.access.Revision, err = t.q.AdvanceCollaborationEditRevision(ctx, before.ID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		receipt := t.result("iteration_changed", 200)
		seq, err := t.q.AdvanceCollaborationEventSequence(ctx, before.ID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		payload, err := json.Marshal(map[string]any{"before": map[string]any{"current_iteration_id": before.CurrentIterationID},
			"after": map[string]any{"current_iteration_id": after.CurrentIterationID}, "reason": reason, "receipt": receipt, "request_id": t.operation.RequestID})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		_, err = t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: before.ID, EventSeq: seq, ActorUserID: t.actor, Kind: "iteration_changed", Payload: payload})
		return receipt, err
	})
}
