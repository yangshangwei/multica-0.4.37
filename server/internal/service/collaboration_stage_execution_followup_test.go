package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// The independent2c review first reproduced these cases in its private source.
// Keep the canonical regressions beside the real service execution fixture.
type stageFollowupObservationFailure struct {
	StageExecutionSources
	afterExecute func()
}

func (s stageFollowupObservationFailure) Execute(ctx context.Context, inv StageExecutionInvocation) (StageExecutionProof, error) {
	proof, err := s.StageExecutionSources.Execute(ctx, inv)
	if s.afterExecute != nil {
		s.afterExecute()
	}
	return proof, err
}
func (s stageFollowupObservationFailure) ExecutionObservation(context.Context, pgx.Tx, StageExecutionInvocation, StageExecutionProof) (StageExecutionObservation, error) {
	return StageExecutionObservation{}, errors.New("controlled observation authentication unavailable")
}

type stageFinalizationDeadlineStarter struct {
	TxStarter
	deadlines    []time.Time
	failFallback bool
}

func (s *stageFinalizationDeadlineStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	if deadline, ok := ctx.Deadline(); ok {
		s.deadlines = append(s.deadlines, deadline)
		if s.failFallback && len(s.deadlines) == 2 {
			return nil, context.DeadlineExceeded
		}
	}
	return s.TxStarter.Begin(ctx)
}

func TestStageExecutionFinalizationSharesOneDeadline(t *testing.T) {
	for _, kind := range []string{"unknown_fallback", "caller_cancelled", "fallback_unavailable"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageExecutionFixture(t)
			request, _ := f.release(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			source := stageFollowupObservationFailure{StageExecutionSources: f.source}
			if kind == "caller_cancelled" {
				source.afterExecute = cancel
			}
			starter := &stageFinalizationDeadlineStarter{TxStarter: f.pool, failFallback: kind == "fallback_unavailable"}
			f.svc.TxStarter = starter
			f.svc.StageExecutionSources = source
			receipt, err := f.svc.PrepareStageExecution(ctx, request, f.source.consumer(request))
			f.svc.TxStarter = f.pool
			if len(starter.deadlines) != 2 {
				t.Fatalf("expected Record and unknown fallback contexts, got %d", len(starter.deadlines))
			}
			if !starter.deadlines[0].Equal(starter.deadlines[1]) {
				t.Errorf("unknown fallback renewed the five-second finalization budget by %s", starter.deadlines[1].Sub(starter.deadlines[0]))
			}
			if f.source.calls != 1 {
				t.Fatalf("finalization repeated callback: %d", f.source.calls)
			}
			if kind == "fallback_unavailable" {
				if !errors.Is(err, context.DeadlineExceeded) || receipt.FactID != nil || receipt.IntentID != "" {
					t.Fatalf("failed bounded finalization manufactured persistence: %+v %v", receipt, err)
				}
				history, err := f.svc.StageExecutionReceipt(context.Background(), request, f.source.consumer(request))
				if err != nil || history.State != "pending" || history.FactID != nil || f.source.calls != 1 {
					t.Fatalf("unconfirmed finalization did not preserve original unresolved history: %+v %v", history, err)
				}
			} else if err != nil || receipt.State != "unknown" || receipt.FactID != nil {
				t.Fatalf("post-entry refusal lost uncertainty: %+v %v", receipt, err)
			}
		})
	}
}

func TestStageExecutionR4ReflectsTransitiveArtifactAuthority(t *testing.T) {
	for _, step := range []string{"release", "rollback"} {
		t.Run(step, func(t *testing.T) {
			f := newStageExecutionFixture(t)
			request, release := f.release(t)
			if step == "rollback" {
				deployed, err := f.svc.PrepareStageExecution(context.Background(), request, f.source.consumer(request))
				if err != nil || deployed.FactID == nil {
					t.Fatalf("deployment prerequisite: %+v %v", deployed, err)
				}
				if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "verification", release)); err != nil {
					t.Fatal(err)
				}
				metric := f.metric(t, *deployed.FactID, 1, "anomaly")
				request = f.rollback(t, release, *deployed.FactID, metric)
			}
			before := f.stage.evidence(t, f.stage.releaseCtx, f.candidate)
			expected := map[string]bool{"code_review": false, "qa": false, "release": false}
			if step == "rollback" {
				expected["rollback"] = false
			}
			for _, d := range before.Decisions {
				if _, ok := expected[d.Step]; ok && d.Decision == "approved" && d.State == "valid" {
					expected[d.Step] = true
				}
			}
			for kind, found := range expected {
				if !found {
					t.Fatalf("missing current %s approval before revocation", kind)
				}
			}
			calls := f.source.calls
			f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.stage.reviewer)
			view := f.stage.evidence(t, f.stage.releaseCtx, f.candidate)
			if view.Candidate.Baseline == nil || view.BlockedEvidence != 0 {
				t.Fatal("current release reader lost its own artifact authority")
			}
			seen := map[string]bool{}
			for _, d := range view.Decisions {
				if _, ok := expected[d.Step]; ok && d.Decision == "approved" {
					seen[d.Step] = true
					if d.State == "valid" {
						t.Errorf("R4 kept %s valid after its original code reviewer lost artifact authority", d.Step)
					}
				}
			}
			for kind := range expected {
				if !seen[kind] {
					t.Errorf("R4 discarded retained %s history", kind)
				}
			}
			for _, hint := range view.Signable {
				if hint.Step == step && hint.Allowed {
					t.Errorf("R4 advertised %s with a revoked transitive issuer", step)
				}
			}
			if _, err := f.svc.PrepareStageExecution(context.Background(), request, f.source.consumer(request)); !collaborationErrorCode(err, "stage_signer_changed") || f.source.calls != calls {
				t.Fatalf("actual execution refusal changed: %v calls=%d prior=%d", err, f.source.calls, calls)
			}
		})
	}
}
