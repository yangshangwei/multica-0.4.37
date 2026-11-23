package service

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

// These tests add stage-enabled combinations to the existing M1 writers. The
// complete submission/self/R38 matrix belongs to the completion suites.
func stageExitServiceM1(t *testing.T, f *collaborationStageFixture, mode string) {
	t.Helper()
	f.fx.InsertNoID(t, "project_delivery_policy", testutil.Cols{
		"workspace_id": f.workspace, "project_id": f.project, "revision": 1,
		"policy_version": DeliveryEvidencePolicy, "mode": mode, "stage": DeliveryPolicyStage,
		"reason": "Stage-enabled exit compatibility", "created_by": f.creator,
	}, "workspace_id=$1 AND project_id=$2", f.workspace, f.project)
}

func stageExitServiceSnapshot(t *testing.T, f *collaborationStageFixture) string {
	t.Helper()
	var state string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
 'issues',(SELECT jsonb_agg(jsonb_build_object('id',id,'status',status,'revision',revision,'project',project_id,'parent',parent_issue_id) ORDER BY id) FROM issue WHERE workspace_id=$1),
 'session',(SELECT jsonb_build_object('queue',queue_state,'control',control_version,'seq',event_seq) FROM collaboration_session WHERE issue_id=$2),
 'collaboration',(SELECT jsonb_build_object('revision',revision,'lifecycle',lifecycle_epoch,'authority',authority_epoch) FROM issue_collaboration WHERE issue_id=$2),
 'tasks',(SELECT jsonb_agg(jsonb_build_object('id',id,'status',status,'completed',completed_at) ORDER BY id) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)),
 'events',(SELECT count(*) FROM collaboration_event WHERE issue_id=$2),
 'handoffs',(SELECT count(*) FROM issue_handoff WHERE issue_id=$2),
 'submissions',(SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$2),
 'attachments',(SELECT jsonb_agg(jsonb_build_object('id',id,'issue',issue_id,'delivered',delivered_at) ORDER BY id) FROM collaboration_attachment WHERE source_workspace_id=$1),
 'adjustments',(SELECT count(*) FROM delivery_project_change WHERE workspace_id=$1)
)::text`, f.workspace, f.issue.Issue.ID).Scan(&state)
	return state
}

func TestStageExitContinuationRootMoveClearM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		for _, action := range []string{"move", "clear"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				f := newCollaborationStageFixture(t)
				stageExitServiceM1(t, f, mode)
				f.revise(t, true)
				target := pgtype.UUID{}
				if action == "move" {
					target = util.MustParseUUID(f.fx.Project(t, "Unmanaged destination"))
				}
				p := CollaborationEditParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ProjectID: &target, ProjectChangeReason: "Explicit attempted project exit"}
				before := stageExitServiceSnapshot(t, f)
				receipt, err := f.svc.EditIssue(f.ctx, p)
				if stageGateCode(err) != "stage_policy_exit_required" || receipt.Code != "stage_policy_exit_required" {
					t.Fatalf("%s %s escaped the enabled stage policy: receipt=%+v err=%v", mode, action, receipt, err)
				}
				if after := stageExitServiceSnapshot(t, f); after != before {
					t.Fatalf("refused project exit changed issue, queue, submission, attachment or adjustment facts: before=%s after=%s", before, after)
				}
				stored, err := f.svc.GetWorkflowOperation(f.ctx, p.IssueID, p.RequestID)
				if err != nil || stored.Code != receipt.Code || stored.Status != 409 {
					t.Fatalf("original exit refusal did not recover: %+v %v", stored, err)
				}
				f.revise(t, false)
				if replay, err := f.svc.EditIssue(f.ctx, p); stageGateCode(err) != "stage_policy_exit_required" || !reflect.DeepEqual(replay, receipt) {
					t.Fatalf("closing the policy rewrote the original refusal: %+v %v", replay, err)
				}
				if stageExitServiceSnapshot(t, f) != before {
					t.Fatal("original refusal replay produced new effects")
				}
				p.CollaborationWorkflowRequest = f.req(t, f.ctx)
				if applied, err := f.svc.EditIssue(f.ctx, p); err != nil || applied.Code != "issue_edited" {
					t.Fatalf("fresh explicit exit after policy close failed: %+v %v", applied, err)
				}
				if n := f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id IS NOT DISTINCT FROM $2::uuid`, p.IssueID, target); n != 1 {
					t.Fatal("fresh exit did not preserve the root identity at the chosen destination")
				}
			})
		}
	}
}

func TestStageExitContinuationExplicitClosePreservesM1AndSourceAuthority(t *testing.T) {
	for _, control := range []string{"reason", "done", "source_member"} {
		t.Run(control, func(t *testing.T) {
			f := newCollaborationStageFixture(t)
			stageExitServiceM1(t, f, "enforce")
			f.revise(t, true)
			f.revise(t, false)
			target := pgtype.UUID{}
			p := CollaborationEditParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ProjectID: &target, ProjectChangeReason: "Explicit M1 adjustment"}
			want := "delivery_project_change_reason_required"
			switch control {
			case "reason":
				p.ProjectChangeReason = ""
			case "done":
				f.fx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, p.IssueID)
				p.CollaborationWorkflowRequest = f.req(t, f.ctx)
				want = "delivery_project_change_conflict"
			case "source_member":
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
				p.CollaborationWorkflowRequest = f.req(t, f.ctx)
				want = "forbidden"
			}
			before := stageExitServiceSnapshot(t, f)
			if _, err := f.svc.EditIssue(f.ctx, p); stageGateCode(err) != want {
				t.Fatalf("policy close weakened %s: want=%s err=%v", control, want, err)
			}
			if stageExitServiceSnapshot(t, f) != before {
				t.Fatal("an existing M1/source refusal changed root or execution facts")
			}
		})
	}
}

type stageExitServiceCommitStarter struct {
	TxStarter
	reached chan struct{}
	release <-chan struct{}
	once    *sync.Once
	pid     *atomic.Uint32
}

func (s stageExitServiceCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageExitServiceCommitTx{Tx: tx, owner: s}, nil
}

type stageExitServiceCommitTx struct {
	pgx.Tx
	owner stageExitServiceCommitStarter
}

func (tx *stageExitServiceCommitTx) Commit(ctx context.Context) error {
	tx.owner.once.Do(func() {
		tx.owner.pid.Store(tx.Conn().PgConn().PID())
		close(tx.owner.reached)
	})
	select {
	case <-tx.owner.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return tx.Tx.Commit(ctx)
}

func stageExitServiceBarrier(t *testing.T, svc *CollaborationService) (*CollaborationService, <-chan struct{}, *atomic.Uint32, func()) {
	t.Helper()
	reached, release := make(chan struct{}), make(chan struct{})
	var once, releaseOnce sync.Once
	pid := &atomic.Uint32{}
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	copy := *svc
	copy.TxStarter = stageExitServiceCommitStarter{TxStarter: svc.TxStarter, reached: reached, release: release, once: &once, pid: pid}
	return &copy, reached, pid, unblock
}

func stageExitServiceWait(t *testing.T, reached <-chan struct{}) {
	t.Helper()
	select {
	case <-reached:
	case <-time.After(10 * time.Second):
		t.Fatal("actual writer never reached its commit barrier")
	}
}

func stageExitServiceWaiter(t *testing.T, fx *testutil.Fixture, blocker uint32) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		fx.QueryRow(t, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
 AND wait_event_type='Lock' AND $1::integer=ANY(pg_blocking_pids(pid)))`, int64(blocker)).Scan(&waiting)
		if waiting {
			t.Logf("actual second backend waits on first writer PID %d before its commit", blocker)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("second actual writer never waited on the held first transaction")
}

func stageExitSelfParams(t *testing.T, f *stageExecutionFixture) CollaborationLifecycleParams {
	t.Helper()
	_, err := f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: f.stage.req(t, f.ctx), TargetUserID: util.MustParseUUID(f.creator)}})
	if err != nil {
		t.Fatal(err)
	}
	options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "complete")
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationLifecycleParams{CollaborationWorkflowRequest: f.stage.req(t, f.ctx), Action: "complete", TargetStatus: "done",
		StatusOptionsVersion: options.StatusOptionsVersion, Note: "Exact synthetic self-result", Basis: "Existing controlled stage facts"}
}

type stageExitServiceResult struct {
	receipt CollaborationWorkflowReceipt
	err     error
}

func TestStageExitContinuationPolicyCompletionCommitOrders(t *testing.T) {
	for _, order := range []string{"completion_first", "policy_first"} {
		t.Run(order, func(t *testing.T) {
			f := newStageCompletionFixture(t)
			p := stageExitSelfParams(t, f)
			policy := StagePolicyRevisionParams{WorkspaceID: util.MustParseUUID(f.workspace), ProjectID: util.MustParseUUID(f.stage.project),
				RequestID: uuid.NewString(), ExpectedRevision: 1, Enabled: true, Reason: "Explicit stage policy replacement", Credential: completionRequestCredential(f.ctx)}
			first, reached, pid, release := stageExitServiceBarrier(t, f.svc)
			finished := make(chan error, 1)
			if order == "completion_first" {
				go func() { _, err := first.ChangeLifecycle(f.ctx, p); finished <- err }()
				stageExitServiceWait(t, reached)
				// This policy writer uses its existing NOWAIT authority locks.
				// Retain that refusal, then retry the same unstored intent only
				// after the real completion has committed.
				if _, err := f.svc.ReviseStagePolicy(f.ctx, policy); stageGateCode(err) != "stage_policy_busy" {
					t.Fatalf("policy replacement crossed the actual completion transaction: %v", err)
				}
				release()
				if err := <-finished; err != nil {
					t.Fatalf("completion-first writer failed: %v", err)
				}
				if _, err := f.svc.ReviseStagePolicy(f.ctx, policy); err != nil {
					t.Fatalf("policy could not commit after completion released its fences: %v", err)
				}
				if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, p.IssueID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1`, p.IssueID) != 1 {
					t.Fatal("completion-first did not commit exactly one result")
				}
			} else {
				go func() { _, err := first.ReviseStagePolicy(f.ctx, policy); finished <- err }()
				stageExitServiceWait(t, reached)
				before := stageExitServiceSnapshot(t, f.stage.collaborationStageFixture)
				completed := make(chan error, 1)
				go func() { _, err := f.svc.ChangeLifecycle(f.ctx, p); completed <- err }()
				stageExitServiceWaiter(t, f.fx, pid.Load())
				release()
				if err := <-finished; err != nil {
					t.Fatal(err)
				}
				if err := <-completed; stageGateCode(err) != "stage_incomplete" {
					t.Fatalf("committed new policy revived earlier stage approval: %v", err)
				}
				if stageExitServiceSnapshot(t, f.stage.collaborationStageFixture) != before {
					t.Fatal("policy-first completion changed status, result, attachments or queue")
				}
			}
		})
	}
}

func TestStageExitContinuationRootMoveCompletionCommitOrders(t *testing.T) {
	for _, order := range []string{"completion_first", "move_first"} {
		t.Run(order, func(t *testing.T) {
			f := newStageCompletionFixture(t)
			p := stageExitSelfParams(t, f)
			clear := pgtype.UUID{}
			move := CollaborationEditParams{CollaborationWorkflowRequest: f.stage.req(t, f.ctx), ProjectID: &clear, ProjectChangeReason: "Attempted clear while stage enabled"}
			first, reached, pid, release := stageExitServiceBarrier(t, f.svc)
			finished := make(chan stageExitServiceResult, 1)
			second := make(chan stageExitServiceResult, 1)
			if order == "completion_first" {
				go func() { r, err := first.ChangeLifecycle(f.ctx, p); finished <- stageExitServiceResult{r, err} }()
				stageExitServiceWait(t, reached)
				go func() { r, err := f.svc.EditIssue(f.ctx, move); second <- stageExitServiceResult{r, err} }()
				stageExitServiceWaiter(t, f.fx, pid.Load())
				release()
				if result := <-finished; result.err != nil || result.receipt.Code != "completed" {
					t.Fatalf("completion-first result: %+v %v", result.receipt, result.err)
				}
				if result := <-second; stageGateCode(result.err) != "stale_authority" {
					t.Fatalf("waiting root exit ignored the committed completion lifecycle: %+v %v", result.receipt, result.err)
				}
			} else {
				go func() { r, err := first.EditIssue(f.ctx, move); finished <- stageExitServiceResult{r, err} }()
				stageExitServiceWait(t, reached)
				go func() { r, err := f.svc.ChangeLifecycle(f.ctx, p); second <- stageExitServiceResult{r, err} }()
				stageExitServiceWaiter(t, f.fx, pid.Load())
				release()
				if result := <-finished; stageGateCode(result.err) != "stage_policy_exit_required" {
					t.Fatalf("move-first escaped stage: %+v %v", result.receipt, result.err)
				}
				if result := <-second; result.err != nil || result.receipt.Code != "completed" {
					t.Fatalf("waiting completion failed after the refused move committed: %+v %v", result.receipt, result.err)
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id=$2 AND status='done'`, p.IssueID, f.stage.project) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1`, p.IssueID) != 1 {
				t.Fatal("root identity/project or one-result completion changed across the lock order")
			}
		})
	}
}
