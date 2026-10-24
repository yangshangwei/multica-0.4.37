package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAdminCollaborationRechecksAuthorityAndResponsibilityAfterLockWait(t *testing.T) {
	for _, scenario := range []struct{ name, code string }{
		{"admin_role", "admin_forbidden"}, {"admin_password_version", "session_invalid"}, {"admin_disabled", "session_invalid"},
		{"creator_restored", "creator_not_disabled"}, {"candidate_disabled", "successor_unavailable"},
		{"candidate_left", "successor_unavailable"}, {"candidate_password_restricted", "successor_unavailable"},
		{"handler_changed", "stale_revision"}, {"source_scope_removed", "not_found"}, {"organization_inactive", "admin_unavailable"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			f.closeAsCreator(t)
			successor := util.MustParseUUID(f.outsider)
			f.enrollCandidate(t, successor)
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
			p := f.params(t, "reopen")
			p.SuccessorUserID = &successor
			beforeControl := controlStateSnapshot(t, f.collaborationQueueFixture)
			ctx, cancel := context.WithTimeout(f.adminCtx, 10*time.Second)
			defer cancel()
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			q := db.New(tx)
			switch scenario.name {
			case "admin_role":
				err = LockPlatformAdminMutation(ctx, tx, f.admin)
			case "admin_password_version", "admin_disabled":
				_, err = q.LockPasswordUser(ctx, f.admin)
			case "creator_restored":
				_, err = q.LockPasswordUser(ctx, util.MustParseUUID(f.creator))
			case "candidate_disabled", "candidate_left", "candidate_password_restricted":
				_, err = q.LockPasswordUser(ctx, successor)
			case "handler_changed":
				_, err = q.LockCollaborationIssue(ctx, p.IssueID)
			case "source_scope_removed":
				_, err = tx.Exec(ctx, `SELECT workspace_id FROM organization_workspace WHERE workspace_id=$1 FOR UPDATE`, f.workspace)
			case "organization_inactive":
				_, err = tx.Exec(ctx, `SELECT id FROM organization WHERE id=$1 FOR UPDATE`, f.org)
				t.Cleanup(func() { f.fx.Exec(t, `UPDATE organization SET state='active' WHERE id=$1`, f.org) })
			}
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, err := f.actions.ChangeLifecycle(ctx, p); done <- err }()
			waitWorkflowBlocked(t, f.collaborationQueueFixture, int(tx.Conn().PgConn().PID()), done)
			switch scenario.name {
			case "admin_role":
				err = q.SetPlatformRole(ctx, db.SetPlatformRoleParams{UserID: f.admin, Role: PlatformRoleObserver})
			case "admin_password_version":
				_, err = tx.Exec(ctx, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.admin)
			case "admin_disabled":
				_, err = tx.Exec(ctx, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.admin)
			case "creator_restored":
				_, err = tx.Exec(ctx, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.creator)
			case "candidate_disabled":
				_, err = tx.Exec(ctx, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, successor)
			case "candidate_left":
				_, err = tx.Exec(ctx, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, p.IssueID, successor)
			case "candidate_password_restricted":
				_, err = tx.Exec(ctx, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, successor)
			case "handler_changed":
				if _, err = q.UpdateCollaborationHandler(ctx, db.UpdateCollaborationHandlerParams{IssueID: p.IssueID, HandlerID: successor}); err == nil {
					_, err = q.AdvanceCollaborationWorkflowRevision(ctx, p.IssueID)
				}
			case "source_scope_removed":
				_, err = tx.Exec(ctx, `DELETE FROM organization_workspace WHERE workspace_id=$1`, f.workspace)
			case "organization_inactive":
				_, err = tx.Exec(ctx, `UPDATE organization SET state='inactive' WHERE id=$1`, f.org)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			assertAdminCollaborationError(t, <-done, scenario.code)
			if controlStateSnapshot(t, f.collaborationQueueFixture) != beforeControl ||
				f.fx.Count(t, `SELECT count(*) FROM admin_operation WHERE target_id=$1`, p.IssueID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM admin_audit_event WHERE target_id=$1`, p.IssueID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='admin_reopen'`, p.IssueID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done' AND creator_id=$2`, p.IssueID, f.creator) != 1 {
				t.Fatal("a lock-time refusal partially reopened, reassigned or changed the retained control ledger")
			}
		})
	}
}

func TestAdminCollaborationAuthorityFencesRemainUntilCommit(t *testing.T) {
	for _, scenario := range []string{"role", "password", "source_scope", "organization", "successor_account", "successor_participation"} {
		t.Run(scenario, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			f.closeAsCreator(t)
			successor := util.MustParseUUID(f.outsider)
			f.enrollCandidate(t, successor)
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
			p := f.params(t, "reopen")
			p.SuccessorUserID = &successor
			ready, release := make(chan int, 1), make(chan struct{})
			releaseOnce := sync.OnceFunc(func() { close(release) })
			defer releaseOnce()
			f.actions.TxStarter = lifecycleCommitBarrierStarter{TxStarter: f.pool, ready: ready, release: release}
			ctx, cancel := context.WithTimeout(f.adminCtx, 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := f.actions.ChangeLifecycle(ctx, p); done <- err }()
			var blocker int
			select {
			case blocker = <-ready:
			case err := <-done:
				t.Fatalf("action did not reach commit: %v", err)
			case <-ctx.Done():
				t.Fatal("action did not reach commit barrier")
			}
			writerDone := make(chan error, 1)
			if scenario == "organization" {
				t.Cleanup(func() { f.fx.Exec(t, `UPDATE organization SET state='active' WHERE id=$1`, f.org) })
			}
			go func() {
				tx, err := f.pool.Begin(ctx)
				if err != nil {
					writerDone <- err
					return
				}
				defer tx.Rollback(context.Background())
				q := db.New(tx)
				switch scenario {
				case "role":
					if err = LockPlatformAdminMutation(ctx, tx, f.admin); err == nil {
						err = q.SetPlatformRole(ctx, db.SetPlatformRoleParams{UserID: f.admin, Role: PlatformRoleObserver})
					}
				case "password":
					if _, err = q.LockPasswordUser(ctx, f.admin); err == nil {
						_, err = tx.Exec(ctx, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.admin)
					}
				case "source_scope":
					_, err = tx.Exec(ctx, `DELETE FROM organization_workspace WHERE workspace_id=$1`, f.workspace)
				case "organization":
					_, err = tx.Exec(ctx, `UPDATE organization SET state='inactive' WHERE id=$1`, f.org)
				case "successor_account":
					_, err = tx.Exec(ctx, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, successor)
				case "successor_participation":
					_, err = tx.Exec(ctx, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, p.IssueID, successor)
				}
				if err == nil {
					err = tx.Commit(ctx)
				}
				writerDone <- err
			}()
			waitWorkflowBlocked(t, f.collaborationQueueFixture, blocker, writerDone)
			releaseOnce()
			if err := <-done; err != nil {
				t.Fatalf("fenced action commit: %v", err)
			}
			if err := <-writerDone; err != nil {
				t.Fatalf("later authority writer: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo' AND assignee_id=$2 AND creator_id=$3`, p.IssueID, successor, f.creator) != 1 {
				t.Fatal("the fenced action lost its atomic responsibility change")
			}
		})
	}
}

func adminCollaborationFixtureWithRetainedWork(t *testing.T) (*adminCollaborationFixture, db.CollaborationRun, CollaborationQueueReceipt) {
	t.Helper()
	var run db.CollaborationRun
	var waiting CollaborationQueueReceipt
	f := newAdminCollaborationFixture(t, func(f *collaborationQueueFixture) {
		run = f.run(t, f.submit(t, f.handler, "Immutable running input"), 1)
		waiting = f.submit(t, f.creator, "Immutable waiting input")
		f.fx.InsertNoID(t, "collaboration_runtime_reservation", testutil.Cols{"run_id": run.ID, "execution_workspace_id": run.ExecutionWorkspaceID,
			"agent_id": run.AgentID, "runtime_id": run.RuntimeID, "daemon_id": "admin-fixture-daemon", "installation_id": run.InstallationID,
			"binding_id": run.BindingID}, "run_id=$1", run.ID)
		f.fx.Insert(t, "collaboration_termination", testutil.Cols{"issue_id": f.issue.Issue.ID, "run_id": run.ID, "fence": run.Fence,
			"launch_nonce": run.LaunchNonce, "deployment_id": uuid.NewString(), "installation_id": run.InstallationID,
			"identity_kind": "enrolled", "public_key": []byte("fixture supervisor"), "token_hash": uuid.NewString()})
	})
	return f, run, waiting
}

func TestAdminCollaborationCloseReopenPreservesProofAndCancellationHistory(t *testing.T) {
	f, originalRun, waiting := adminCollaborationFixtureWithRetainedWork(t)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false,creation_enabled=false WHERE id=1`)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	closed, err := f.actions.ChangeLifecycle(f.adminCtx, f.params(t, "cancel"))
	if err != nil || closed.Result.UnsettledRuns != 1 || closed.Result.QueueState != "closed" {
		t.Fatalf("close with unproven work = %+v, %v", closed, err)
	}
	beforeReopen, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: originalRun.ID})
	if err != nil {
		t.Fatal(err)
	}
	if beforeReopen.ProofState != "unknown" || beforeReopen.SettledAt.Valid || beforeReopen.LaunchNonce != originalRun.LaunchNonce || beforeReopen.Fence != originalRun.Fence {
		t.Fatal("administrative close fabricated termination or replaced the original run identity")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command c JOIN collaboration_event e ON e.id=c.cancellation_event_id
		WHERE c.id=$1 AND c.state='cancelled' AND c.content='Immutable waiting input' AND c.reason='issue_cancelled'
		AND e.actor_user_id=$2 AND e.payload->>'cancellation_kind'='automatic' AND e.payload->>'command_id'=c.id::text`, waiting.CommandID, f.admin) != 1 {
		t.Fatal("close lost exact automatic cancellation provenance or its real administrator")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NOT NULL AND proof IS NULL AND confirmed_at IS NULL`, originalRun.ID) != 1 {
		t.Fatal("close did not retain unconfirmed supervisor stop intent")
	}
	reopened, err := f.actions.ChangeLifecycle(f.adminCtx, f.params(t, "reopen"))
	if err != nil || reopened.Result.QueueState != "paused" || reopened.Result.UnsettledRuns != 1 {
		t.Fatalf("reopen with retained proof barrier = %+v, %v", reopened, err)
	}
	afterReopen, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: originalRun.ID})
	if err != nil || !reflect.DeepEqual(beforeReopen, afterReopen) {
		t.Fatalf("reopen changed the old run/barrier: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND content='Immutable waiting input'`, waiting.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, originalRun.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE session_id=$1 AND queue_state<>'paused'`, f.issue.SessionID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("reopen revived terminal input, a channel, or ordinary AI execution")
	}
}

func TestAdminCollaborationIterationFailureRollsBackSuccessorAndReopen(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	iterationID := f.fx.Insert(t, "iteration", testutil.Cols{"workspace_id": f.workspace, "name": "Administration iteration",
		"timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": f.creator})
	f.fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": f.workspace, "iteration_id": iterationID,
		"issue_id": f.issue.Issue.ID, "first_joined_at": time.Now(), "current_joined_at": time.Now()}, "iteration_id=$1 AND issue_id=$2", iterationID, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iterationID)
	f.fx.Exec(t, `UPDATE issue SET current_iteration_id=$2 WHERE id=$1`, f.issue.Issue.ID, iterationID)
	f.closeAsCreator(t)
	successor := util.MustParseUUID(f.outsider)
	f.enrollCandidate(t, successor)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
	p := f.params(t, "reopen")
	p.SuccessorUserID = &successor
	before := adminCollaborationStateSnapshot(t, f)
	beforeEvents := f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID)
	f.actions.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: "INSERT INTO iteration_event"}
	if _, err := f.actions.ChangeLifecycle(f.adminCtx, p); err == nil {
		t.Fatal("failed iteration fact was ignored")
	}
	if adminCollaborationStateSnapshot(t, f) != before || f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID) != beforeEvents {
		t.Fatal("iteration failure retained half a reopen, handoff or audit")
	}
	f.actions.TxStarter = f.pool
	if _, err := f.actions.ChangeLifecycle(f.adminCtx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_event e JOIN collaboration_operation o ON o.id=e.operation_id
		WHERE e.iteration_id=$1 AND o.scope_id=$2 AND o.request_id=$3 AND e.actor->>'id'=$4`, iterationID, p.IssueID,
		util.UUIDToString(p.IdempotencyKey), util.UUIDToString(f.admin)) == 0 {
		t.Fatal("iteration facts lost the real administrator and original operation identity")
	}
}

func TestAdminCollaborationRequiredWritesRollbackEverything(t *testing.T) {
	for _, action := range []string{"complete", "reopen"} {
		needles := []string{"INSERT INTO admin_operation", "INSERT INTO collaboration_operation", "UPDATE collaboration_session SET queue_state",
			"UPDATE issue AS i SET", "UPDATE issue_handoff SET resolution", "UPDATE issue_collaboration SET active_information_request_id",
			"UPDATE issue_collaboration SET revision", "INSERT INTO collaboration_event", "INSERT INTO admin_audit_event", "UPDATE collaboration_operation"}
		if action == "complete" {
			needles = append(needles, "UPDATE collaboration_command SET state", "UPDATE collaboration_run", "UPDATE collaboration_termination SET")
		} else {
			needles = append(needles, "UPDATE issue SET assignee_type", "INSERT INTO issue_handoff")
		}
		for _, needle := range needles {
			t.Run(action+"/"+needle, func(t *testing.T) {
				f, _, _ := adminCollaborationFixtureWithRetainedWork(t)
				if action == "reopen" {
					f.closeAsCreator(t)
				}
				successor := util.MustParseUUID(f.outsider)
				f.enrollCandidate(t, successor)
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
				p := f.params(t, action)
				if action == "reopen" {
					p.SuccessorUserID = &successor
				}
				before := adminCollaborationStateSnapshot(t, f)
				f.actions.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
				if _, err := f.actions.ChangeLifecycle(f.adminCtx, p); err == nil {
					t.Fatal("required write failure was ignored")
				}
				if after := adminCollaborationStateSnapshot(t, f); after != before {
					t.Fatal("required failure committed half an operation, handoff, control effect, history or audit")
				}
			})
		}
	}
}

func TestAdminCollaborationCommitFailureAndOriginalKeyRecovery(t *testing.T) {
	for _, lostReply := range []bool{false, true} {
		name := "rollback"
		if lostReply {
			name = "lost_commit_reply"
		}
		t.Run(name, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			f.closeAsCreator(t)
			successor := util.MustParseUUID(f.outsider)
			f.enrollCandidate(t, successor)
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
			p := f.params(t, "reopen")
			p.SuccessorUserID = &successor
			before := adminCollaborationStateSnapshot(t, f)
			fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: lostReply, commitFailure: !lostReply}
			f.actions.TxStarter = fault
			result, err := f.actions.ChangeLifecycle(f.adminCtx, p)
			if err == nil || fault.begins.Load() != 1 || result.ParentWakeup != nil || result.Operation.ID.Valid {
				t.Fatalf("uncertain commit retried or exposed an uncommitted result: %+v, %v, begins=%d", result, err, fault.begins.Load())
			}
			f.actions.TxStarter = f.pool
			if !lostReply {
				if adminCollaborationStateSnapshot(t, f) != before {
					t.Fatal("definite commit failure retained partial administration")
				}
				_, err = f.actions.GetOperation(f.adminCtx, f.org, p.IssueID, p.IdempotencyKey)
				if !errors.Is(err, pgx.ErrNoRows) {
					t.Fatalf("rolled-back operation appeared successful: %v", err)
				}
				return
			}
			beforeRead := adminCollaborationStateSnapshot(t, f)
			recovered, err := f.actions.GetOperation(f.adminCtx, f.org, p.IssueID, p.IdempotencyKey)
			if err != nil || recovered.Result.HandlerUserID != successor || recovered.Result.Code != "reopened" || recovered.Result.QueueState != "paused" || recovered.ParentWakeup != nil {
				t.Fatalf("original-key recovery = %+v, %v", recovered, err)
			}
			if adminCollaborationStateSnapshot(t, f) != beforeRead ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='admin_reopen'`, p.IssueID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM admin_operation WHERE target_id=$1`, p.IssueID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM admin_audit_event WHERE target_id=$1`, p.IssueID) != 1 {
				t.Fatal("GET recovery changed the durable result or duplicated administration")
			}
		})
	}
}

func TestAdminCollaborationConcurrentOriginalKeyHasOneHandoff(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	f.closeAsCreator(t)
	successor := util.MustParseUUID(f.outsider)
	f.enrollCandidate(t, successor)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
	p := f.params(t, "reopen")
	p.SuccessorUserID = &successor
	start := make(chan struct{})
	type result struct {
		r   AdminCollaborationResult
		err error
	}
	done := make(chan result, 2)
	for range 2 {
		go func() { <-start; r, err := f.actions.ChangeLifecycle(f.adminCtx, p); done <- result{r, err} }()
	}
	close(start)
	a, b := <-done, <-done
	if a.err != nil || b.err != nil || a.r.Operation.ID != b.r.Operation.ID || !reflect.DeepEqual(a.r.Result, b.r.Result) || a.r.Replayed == b.r.Replayed {
		t.Fatalf("concurrent original requests diverged: %+v, %+v", a, b)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='admin_reopen'`, p.IssueID) != 1 {
		t.Fatal("concurrent replay created multiple handoffs")
	}
}
