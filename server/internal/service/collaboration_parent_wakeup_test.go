package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func collaborationLifecycleParent(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	parent := f.fx.Issue(t, "Ordinary parent", testutil.Cols{"status": "in_progress"})
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=$2, stage=2 WHERE id=$1`, f.issue.Issue.ID, parent)
	return parent
}

func TestCollaborationLifecycleParentWakeupIsCommittedAndTransient(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	parent := collaborationLifecycleParent(t, f)
	p := lifecycleParams(t, f, "complete", "done", 1, 1)
	// R36 preserves the creator's lifecycle authority after source departure.
	// The resulting parent effect is internal, never a parent read capability.
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	receipt, err := f.svc.ChangeLifecycle(f.ctx, p)
	if err != nil || receipt.ParentWakeup == nil {
		t.Fatalf("committed close did not carry parent transition: %+v %v", receipt, err)
	}
	wake := receipt.ParentWakeup
	if wake.Before.ID != f.issue.Issue.ID || wake.After.ID != wake.Before.ID || wake.Before.Status != "todo" || wake.After.Status != "done" ||
		wake.After.ParentIssueID != util.MustParseUUID(parent) || wake.After.Stage.Int32 != 2 || !wake.After.Stage.Valid {
		t.Fatal("parent transition did not preserve the locked before/after identity and stage")
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2 AND completed_at IS NOT NULL`, f.issue.Issue.ID, p.RequestID) != 1 {
		t.Fatal("parent effect escaped before the lifecycle and receipt committed")
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), parent) || strings.Contains(string(encoded), "ParentWakeup") || strings.Contains(string(encoded), f.issue.Issue.Title) {
		t.Fatal("internal parent transition entered receipt JSON")
	}
	op, err := f.svc.Queries.GetCollaborationOperation(f.ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID,
		ActorUserID: util.MustParseUUID(f.creator), RequestID: p.RequestID})
	if err != nil || strings.Contains(string(op.Result), parent) {
		t.Fatalf("persisted operation contains parent data: %v", err)
	}
	decoded, err := decodeWorkflowReceipt(op)
	if err != nil || decoded.ParentWakeup != nil {
		t.Fatalf("decoding a stored receipt reconstructed its effect: %v", err)
	}
	replayed, err := f.svc.ChangeLifecycle(f.ctx, p)
	if err != nil || replayed.ParentWakeup != nil || replayed.Code != receipt.Code {
		t.Fatalf("lifecycle replay repeated its parent effect: %+v %v", replayed, err)
	}
	// Reopen is not a terminal transition; a fresh later close is.
	reopened, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", 2, 2))
	if err != nil || reopened.ParentWakeup != nil {
		t.Fatalf("reopen generated a parent wake: %+v %v", reopened, err)
	}
	closedAgain, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "cancel", "cancelled", 3, 3))
	if err != nil || closedAgain.ParentWakeup == nil || closedAgain.ParentWakeup.After.Status != "cancelled" {
		t.Fatalf("fresh cancellation did not generate its own terminal transition: %+v %v", closedAgain, err)
	}
}

func TestCollaborationLifecycleParentWakeupAbsentOnRefusalAndFailure(t *testing.T) {
	for _, scenario := range []string{"forbidden", "stale_revision", "event_failure", "receipt_failure", "commit_failure", "unknown_commit"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			collaborationLifecycleParent(t, f)
			p := lifecycleParams(t, f, "complete", "done", 1, 1)
			ctx := f.ctx
			switch scenario {
			case "forbidden":
				ctx = collaborationContext(f.handler)
			case "stale_revision":
				p.ExpectedCollaborationRevision = 2
			case "event_failure":
				f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: "INSERT INTO collaboration_event"}
			case "receipt_failure":
				f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: "UPDATE collaboration_operation"}
			case "commit_failure":
				f.svc.TxStarter = &queueFaultStarter{TxStarter: f.pool, commitFailure: true}
			case "unknown_commit":
				f.svc.TxStarter = &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
			}
			receipt, err := f.svc.ChangeLifecycle(ctx, p)
			if err == nil || receipt.ParentWakeup != nil {
				t.Fatalf("unsuccessful lifecycle exposed parent effect: %+v %v", receipt, err)
			}
			f.svc.TxStarter = f.pool
			status := "todo"
			if scenario == "unknown_commit" {
				status = "done"
				replay, err := f.svc.ChangeLifecycle(ctx, p)
				if err != nil || replay.Code != "completed" || replay.ParentWakeup != nil {
					t.Fatalf("unknown commit recovery recreated a parent effect: %+v %v", replay, err)
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status=$2`, f.issue.Issue.ID, status) != 1 {
				t.Fatal("failed lifecycle changed the committed child state")
			}
		})
	}
}

func TestCollaborationLifecycleConcurrentReplayReturnsOneParentWakeup(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	collaborationLifecycleParent(t, f)
	p := lifecycleParams(t, f, "complete", "done", 1, 1)
	type result struct {
		receipt CollaborationWorkflowReceipt
		err     error
	}
	start, results := make(chan struct{}), make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			receipt, err := f.svc.ChangeLifecycle(f.ctx, p)
			results <- result{receipt: receipt, err: err}
		}()
	}
	close(start)
	wakes := 0
	for range 2 {
		r := <-results
		if r.err != nil || r.receipt.Code != "completed" {
			t.Fatalf("concurrent completion failed: %+v %v", r.receipt, r.err)
		}
		if r.receipt.ParentWakeup != nil {
			wakes++
		}
	}
	if wakes != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='completed'`, f.issue.Issue.ID) != 1 {
		t.Fatal("concurrent lifecycle replay did not collapse to one committed parent effect")
	}
}
