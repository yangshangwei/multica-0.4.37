package service

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type accountReconciliationFixture struct {
	*platformAdminFixture
	worker               *CollaborationAccountReconciler
	queue                *CollaborationService
	admin, target, other pgtype.UUID
	first                pgtype.UUID
	targets              map[pgtype.UUID]pgtype.UUID
}

func newAccountReconciliationFixture(t *testing.T) *accountReconciliationFixture {
	t.Helper()
	f := &accountReconciliationFixture{platformAdminFixture: newPlatformAdminFixture(t)}
	f.admin, f.target, f.other = f.user(t, PlatformRoleSuperAdmin), f.user(t, ""), f.user(t, "")
	collaborationAccountFence(t, f.platformAdminFixture, f.admin)
	collaborationAccountFence(t, f.platformAdminFixture, f.target)
	collaborationAccountFence(t, f.platformAdminFixture, f.other)
	f.first, f.queue = collaborationAccountIssue(t, f.platformAdminFixture, f.target)
	f.targets = make(map[pgtype.UUID]pgtype.UUID)
	for _, participant := range []pgtype.UUID{f.other, f.admin} {
		f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.first, "user_id": participant}, "issue_id=$1 AND user_id=$2", f.first, participant)
	}
	f.worker = NewCollaborationAccountReconciler(f.svc.Queries, f.pool)
	return f
}

func (f *accountReconciliationFixture) issue(t *testing.T) pgtype.UUID {
	t.Helper()
	id := f.fx.Issue(t, "Retained collaboration", testutil.Cols{"assignee_type": "member", "assignee_id": f.target, "admission_status": "not_required"})
	f.fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": id}, "issue_id=$1", id)
	f.fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": id})
	for _, user := range []pgtype.UUID{f.target, f.other, f.admin} {
		f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": id, "user_id": user}, "issue_id=$1 AND user_id=$2", id, user)
	}
	return util.MustParseUUID(id)
}

func (f *accountReconciliationFixture) targetFor(t *testing.T, issue pgtype.UUID) (db.CollaborationSession, db.CollaborationTarget) {
	t.Helper()
	session, err := f.queue.Queries.GetCollaborationSession(t.Context(), issue)
	if err != nil {
		t.Fatal(err)
	}
	if !f.targets[issue].Valid {
		target, _ := seedCollaborationTarget(t, f.fx, issue, session.ID, f.admin, 1)
		f.targets[issue] = target.ID
	}
	targets, err := f.queue.Queries.ListCollaborationTargets(t.Context(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		if target.ID == f.targets[issue] {
			return session, target
		}
	}
	t.Fatal("default synthetic target missing")
	return session, db.CollaborationTarget{}
}

func (f *accountReconciliationFixture) submit(t *testing.T, issue, author pgtype.UUID, epoch int64) CollaborationQueueReceipt {
	t.Helper()
	_, target := f.targetFor(t, issue)
	r, err := f.queue.SubmitCommand(collaborationPasswordContext(author, epoch), CollaborationSubmitCommandParams{
		CollaborationQueueRequest: CollaborationQueueRequest{IssueID: issue, RequestID: uuid.NewString(),
			ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: epoch},
		TargetID: target.ID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		Content: "Immutable delivered command",
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *accountReconciliationFixture) grant(t *testing.T, issue, owner pgtype.UUID, epoch int64, active bool) pgtype.UUID {
	t.Helper()
	session, target := f.targetFor(t, issue)
	if !active {
		_, grant := seedCollaborationTarget(t, f.fx, issue, session.ID, owner, epoch)
		return grant.ID
	}
	return rebindCollaborationTestTarget(t, f.fx, issue, session.ID, target.ID, owner, epoch).ID
}

// Runs remain explicit DB fixtures, not evidence of native launch or exit.
func (f *accountReconciliationFixture) run(t *testing.T, issue pgtype.UUID, command CollaborationQueueReceipt, grant pgtype.UUID, attempts int64) db.CollaborationRun {
	t.Helper()
	session, _ := f.targetFor(t, issue)
	run := seedCollaborationGroupRun(t, f.fx, session.ID, command, attempts)
	if run.GrantID != grant {
		t.Fatal("run fixture did not bind the requested current grant")
	}
	f.fx.Insert(t, "collaboration_termination", testutil.Cols{
		"issue_id": issue, "run_id": run.ID, "fence": run.Fence, "launch_nonce": run.LaunchNonce, "deployment_id": uuid.NewString(),
		"installation_id": run.InstallationID, "identity_kind": "enrolled", "public_key": []byte("fixture-supervisor-key"), "token_hash": uuid.NewString(),
	})
	return run
}

func (f *accountReconciliationFixture) change(t *testing.T, target pgtype.UUID, action string, version int64) {
	t.Helper()
	if _, err := f.svc.ChangeAccount(adminTestContext(f.admin, 1), collaborationAccountChange(f.platformAdminFixture, target, action, version)); err != nil {
		t.Fatal(err)
	}
}

func (f *accountReconciliationFixture) drain(t *testing.T) {
	t.Helper()
	for range 10 {
		n, err := f.worker.Reconcile(t.Context(), 32)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
	}
	t.Fatal("reconciliation did not exhaust this bounded fixture")
}

func TestCollaborationAccountReconciliationDisableRestore(t *testing.T) {
	f := newAccountReconciliationFixture(t)
	owned, fresh, unaffected := f.issue(t), f.issue(t), f.issue(t)
	oldAuthor := f.run(t, f.first, f.submit(t, f.first, f.target, 1), f.grant(t, f.first, f.other, 1, true), 1)
	oldGrant := f.grant(t, owned, f.target, 1, true)
	oldOwner := f.run(t, owned, f.submit(t, owned, f.other, 1), oldGrant, 1)
	unusedGrant := f.grant(t, fresh, f.target, 1, false)
	var obsolete, retained []pgtype.UUID
	for _, issue := range []pgtype.UUID{f.first, owned, fresh} {
		obsolete = append(obsolete, f.submit(t, issue, f.target, 1).CommandID)
		retained = append(retained, f.submit(t, issue, f.other, 1).CommandID)
	}
	untouched := f.run(t, unaffected, f.submit(t, unaffected, f.other, 1), f.grant(t, unaffected, f.other, 1, true), 1)
	f.change(t, f.target, "disable", 1)
	f.change(t, f.target, "restore", 2)
	// Current authority is valid again before the old intent is consumed.
	retained = append(retained, f.submit(t, f.first, f.target, 3).CommandID)
	newGrant := f.grant(t, fresh, f.target, 3, true)
	newRun := f.run(t, fresh, f.submit(t, fresh, f.target, 3), newGrant, 1)
	retained = append(retained, f.submit(t, fresh, f.target, 3).CommandID)

	if n, err := f.worker.Reconcile(t.Context(), 1); err != nil || n != 1 {
		t.Fatalf("bounded first step: %d, %v", n, err)
	}
	ids := []string{util.UUIDToString(f.first), util.UUIDToString(owned), util.UUIDToString(fresh)}
	sort.Strings(ids)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=2 AND last_issue_id=$2 AND reconciled_at IS NULL`, f.target, ids[0]) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE kind='account_credentials_revoked'`) != 1 {
		t.Fatal("first source effects and ordered progress did not commit together")
	}
	// Restart with no in-memory progress or previous transaction state.
	f.worker = NewCollaborationAccountReconciler(f.svc.Queries, f.pool)
	f.drain(t)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND reconciled_at IS NOT NULL`, f.target) != 2 {
		t.Fatal("disable/restore intents were lost or not completed")
	}
	for _, id := range obsolete {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND reason='credentials_revoked' AND author_auth_epoch=1 AND content='Immutable delivered command'`, id) != 1 {
			t.Fatal("old author work was not cancelled with immutable attribution/content")
		}
	}
	for _, id := range retained {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND reason IS NULL`, id) != 1 {
			t.Fatal("another author's or restored-epoch command was cancelled")
		}
	}
	for _, run := range []db.CollaborationRun{oldAuthor, oldOwner} {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND fence=$2 AND launch_nonce=$3 AND claim_generation=2 AND authorized_attempt_seq=1 AND progress_state='ending' AND forced_disposition='stopped' AND disposition_reason='credentials_revoked' AND proof_state='unknown' AND settled_at IS NULL`, run.ID, run.Fence, run.LaunchNonce) != 1 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND active_group_id=$2 AND queue_state='paused' AND control_version=2`, run.TargetID, run.GroupID) != 1 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND fence=$2 AND launch_nonce=$3 AND stop_requested_at IS NOT NULL AND confirmed_at IS NULL AND proof IS NULL`, run.ID, run.Fence, run.LaunchNonce) != 1 {
			t.Fatal("obsolete running work lost its barrier, stop intent or historical identity")
		}
	}
	for _, id := range []pgtype.UUID{oldGrant, unusedGrant} {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND epoch=2 AND revoked_at IS NOT NULL AND revocation_reason='credentials_revoked'`, id) != 1 {
			t.Fatal("obsolete owner grant was not revoked exactly once")
		}
	}
	for _, run := range []db.CollaborationRun{newRun, untouched} {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND claim_generation=1 AND forced_disposition IS NULL AND settled_at IS NULL`, run.ID) != 1 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='active' AND control_version=1 AND active_group_id=$2`, run.TargetID, run.GroupID) != 1 {
			t.Fatal("old invalidation changed unrelated/current execution")
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND owner_auth_epoch=3 AND revoked_at IS NULL`, newGrant) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE creator_id=$1 AND assignee_id=$1`, f.target) != 4 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE epoch=1 AND revoked_at IS NULL`) != 12 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE kind='account_credentials_revoked' AND actor_user_id IS NOT NULL`) != 0 {
		t.Fatal("fanout changed responsibility/participation, revived a grant, or impersonated the account")
	}
	if n, err := f.worker.Reconcile(t.Context(), 32); n != 0 || err != nil {
		t.Fatalf("completed scan is not inert: %d, %v", n, err)
	}
}

func TestCollaborationOldInvalidationPreservesFreshGrantOnSameTypedTarget(t *testing.T) {
	f := newAccountReconciliationFixture(t)
	oldGrant := f.grant(t, f.first, f.target, 1, true)
	_, before := f.targetFor(t, f.first)
	oldInput := f.submit(t, f.first, f.target, 1)
	f.change(t, f.target, "disable", 1)
	f.change(t, f.target, "restore", 2)
	newGrant := f.grant(t, f.first, f.target, 3, true)
	freshInput := f.submit(t, f.first, f.target, 3)
	freshRun := f.run(t, f.first, freshInput, newGrant, 1)
	_, after := f.targetFor(t, f.first)
	if before.ID != after.ID || before.ResourceID != after.ResourceID || after.ConfigVersion <= before.ConfigVersion {
		t.Fatal("fixture must replace consent on the same typed target, not evade invalidation with another target")
	}
	f.drain(t)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND author_auth_epoch=1`, oldInput.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL`, oldGrant) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND target_id=$2 AND owner_auth_epoch=3 AND revoked_at IS NULL`, newGrant, before.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='active' AND control_version=1 AND active_group_id=$2`, before.ID, freshRun.GroupID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND claim_generation=1 AND settled_at IS NULL`, freshRun.ID) != 1 {
		t.Fatal("an old epoch paused fresh consent/work on the same typed target or retained obsolete input")
	}
}

func TestCollaborationAccountReconciliationStopEvidence(t *testing.T) {
	for _, scenario := range []string{"never_launched_author", "never_launched_owner", "success_without_permit", "started_without_permit", "first_forced_root", "closed"} {
		t.Run(scenario, func(t *testing.T) {
			f := newAccountReconciliationFixture(t)
			author, owner := f.target, f.other
			if scenario == "never_launched_owner" {
				author, owner = f.other, f.target
			}
			run := f.run(t, f.first, f.submit(t, f.first, author, 1), f.grant(t, f.first, owner, 1, true), 0)
			switch scenario {
			case "success_without_permit":
				f.fx.Exec(t, `UPDATE collaboration_run SET business_outcome='success' WHERE id=$1`, run.ID)
			case "started_without_permit":
				f.fx.Exec(t, `UPDATE collaboration_run SET started_at=now() WHERE id=$1`, run.ID)
			case "first_forced_root", "closed":
				f.fx.Exec(t, `UPDATE collaboration_run SET authorized_attempt_seq=1, claim_generation=7, progress_state='ending', forced_disposition='stopped', disposition_reason='participant_stop' WHERE id=$1`, run.ID)
				f.fx.Exec(t, `UPDATE collaboration_target SET queue_state='paused', pause_reason='participant_stop' WHERE id=$1`, run.TargetID)
				if scenario == "closed" {
					f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed', pause_reason='issue_completed' WHERE id=$1`, run.SessionID)
				}
			}
			f.change(t, f.target, "disable", 1)
			f.drain(t)
			if strings.HasPrefix(scenario, "never_launched_") {
				disposition := "cancelled"
				if scenario == "never_launched_owner" {
					disposition = "stopped"
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='never_launched_proven' AND settled_at IS NOT NULL AND forced_disposition=$2`, run.ID, disposition) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state=$2`, run.CommandID, disposition) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='paused' AND active_group_id IS NULL`, run.TargetID) != 1 {
					t.Fatal("server-proven never-launched work did not settle with the correct disposition")
				}
			} else {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='unknown' AND settled_at IS NULL AND forced_disposition='stopped' AND fence=$2 AND launch_nonce=$3`, run.ID, run.Fence, run.LaunchNonce) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND active_group_id=$2 AND queue_state<>'active'`, run.TargetID, run.GroupID) != 1 {
					t.Fatal("inconsistent/possibly live work lost its barrier or historical proof identity")
				}
				if scenario == "first_forced_root" || scenario == "closed" {
					if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND claim_generation=7 AND disposition_reason='participant_stop'`, run.ID) != 1 ||
						f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NOT NULL AND confirmed_at IS NULL`, run.ID) != 1 {
						t.Fatal("later invalidation overwrote the first forced root or lost supervisor stop intent")
					}
				}
				if scenario == "closed" && f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='closed'`, run.SessionID) != 1 {
					t.Fatal("invalidation reopened a closed queue")
				}
			}
		})
	}
}

func TestCollaborationAccountReconciliationBusySourceKeepsCursor(t *testing.T) {
	for _, lock := range []string{"workspace", "issue", "session"} {
		t.Run(lock, func(t *testing.T) {
			f := newAccountReconciliationFixture(t)
			second := f.issue(t)
			f.submit(t, f.first, f.target, 1)
			f.submit(t, second, f.target, 1)
			f.change(t, f.target, "disable", 1)
			f.change(t, f.target, "restore", 2)
			ids := []string{util.UUIDToString(f.first), util.UUIDToString(second)}
			sort.Strings(ids)
			tx, err := f.pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			switch lock {
			case "workspace":
				_, err = tx.Exec(t.Context(), `SELECT id FROM workspace WHERE id=$1 FOR UPDATE`, f.fx.WorkspaceID)
			case "issue":
				_, err = tx.Exec(t.Context(), `SELECT id FROM issue WHERE id=$1 FOR NO KEY UPDATE`, ids[0])
			case "session":
				_, err = tx.Exec(t.Context(), `SELECT id FROM collaboration_session WHERE issue_id=$1 FOR UPDATE`, ids[0])
			}
			if err != nil {
				t.Fatal(err)
			}
			fault := &accountReconciliationFaultStarter{TxStarter: f.pool}
			f.worker.TxStarter = fault
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			if n, err := f.worker.Reconcile(ctx, 1); n != 0 || !collaborationErrorCode(err, "collaboration_busy") || ctx.Err() != nil || fault.begins.Load() != 3 {
				t.Fatalf("source conflict was not bounded/retried: %d, %v, attempts=%d", n, err, fault.begins.Load())
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE state='waiting'`) != 2 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE last_issue_id IS NULL AND reconciled_at IS NULL`) != 2 {
				t.Fatal("busy source advanced a cursor, skipped an issue or overtook the earlier epoch")
			}
			if err = tx.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			if n, err := f.worker.Reconcile(t.Context(), 1); n != 1 || err != nil {
				t.Fatalf("released source did not recover: %d, %v", n, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE account_epoch=2 AND last_issue_id=$1 AND reconciled_at IS NULL`, ids[0]) != 1 {
				t.Fatal("retry did not resume from the first ordered source")
			}
			f.drain(t)
		})
	}
}

func TestCollaborationAccountReconciliationMultipleConsumers(t *testing.T) {
	f := newAccountReconciliationFixture(t)
	issues := []pgtype.UUID{f.first}
	for range 5 {
		issues = append(issues, f.issue(t))
	}
	for _, issue := range issues {
		f.submit(t, issue, f.target, 1)
		f.submit(t, issue, f.other, 1)
	}
	f.change(t, f.target, "disable", 1)
	f.change(t, f.target, "restore", 2)
	f.change(t, f.other, "disable", 1)
	// Lock the first account's earlier epoch. Another consumer must take the
	// other account, never its restored epoch, even with SKIP LOCKED enabled.
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(t.Context(), `SELECT user_id FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=2 FOR UPDATE`, f.target); err != nil {
		t.Fatal(err)
	}
	if n, err := f.worker.Reconcile(t.Context(), 1); n != 1 || err != nil {
		t.Fatalf("independent account did not make progress: %d, %v", n, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND last_issue_id IS NULL AND reconciled_at IS NULL`, f.target) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND last_issue_id IS NOT NULL`, f.other) != 1 {
		t.Fatal("consumer overtook a claimed earlier account epoch")
	}
	if err = tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			worker := NewCollaborationAccountReconciler(f.svc.Queries, f.pool)
			for range 30 {
				n, err := worker.Reconcile(t.Context(), 1)
				if err != nil && !collaborationErrorCode(err, "collaboration_busy") {
					results <- err
					return
				}
				if n == 0 && err == nil {
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		t.Fatal(err)
	}
	f.drain(t)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE reconciled_at IS NOT NULL`) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE state='cancelled' AND reason='credentials_revoked'`) != 12 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE kind='account_credentials_revoked'`) != 12 {
		t.Fatal("competing consumers lost or repeated a committed issue effect")
	}
}

func TestCollaborationAccountReconciliationIndependentAccountAfterFailure(t *testing.T) {
	for _, failure := range []string{"busy", "missing_source"} {
		t.Run(failure, func(t *testing.T) {
			f := newAccountReconciliationFixture(t)
			blocked := f.submit(t, f.first, f.target, 1)
			independent := f.issue(t)
			run := f.run(t, independent, f.submit(t, independent, f.other, 1), f.grant(t, independent, f.other, 1, true), 1)
			waiting := f.submit(t, independent, f.other, 1)
			f.change(t, f.target, "disable", 1)
			f.change(t, f.target, "restore", 2)
			f.change(t, f.other, "disable", 1)
			tx, err := f.pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if failure == "busy" {
				if _, err = tx.Exec(t.Context(), `SELECT id FROM issue WHERE id=$1 FOR NO KEY UPDATE`, f.first); err != nil {
					t.Fatal(err)
				}
			} else {
				// Deliberate private-fixture corruption: retained dependencies must
				// not turn a missing source into successful reconciliation.
				f.fx.Exec(t, `DELETE FROM issue WHERE id=$1`, f.first)
			}
			fault := &accountReconciliationFaultStarter{TxStarter: f.pool}
			f.worker.TxStarter = fault
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			n, err := f.worker.Reconcile(ctx, 8)
			if n != 2 || err == nil || ctx.Err() != nil || fault.begins.Load() > 8*3 {
				t.Fatalf("independent account starved or failure hidden: %d, %v, attempts=%d", n, err, fault.begins.Load())
			}
			if failure == "busy" && !collaborationErrorCode(err, "collaboration_busy") ||
				failure == "missing_source" && !strings.Contains(err.Error(), "source is missing") {
				t.Fatalf("wrong deferred failure: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND last_issue_id IS NULL AND reconciled_at IS NULL`, f.target) != 2 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, blocked.CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND last_issue_id=$2 AND reconciled_at IS NOT NULL`, f.other, independent) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled'`, waiting.CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND proof_state='unknown' AND settled_at IS NULL`, run.ID) != 1 {
				t.Fatal("failed account advanced, or independent stop/cancellation did not converge")
			}
			if err = tx.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			if failure == "busy" {
				f.drain(t)
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE reconciled_at IS NOT NULL`) != 3 {
					t.Fatal("ephemeral exclusion survived into the next pass")
				}
			}
		})
	}
}

type accountReconciliationFaultStarter struct {
	TxStarter
	begins      atomic.Int32
	failure     string
	afterCommit func()
}

type accountReconciliationFaultTx struct {
	pgx.Tx
	starter *accountReconciliationFaultStarter
}

func (s *accountReconciliationFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	s.begins.Add(1)
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &accountReconciliationFaultTx{Tx: tx, starter: s}, nil
}

func (tx *accountReconciliationFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.starter.failure == "event" && strings.Contains(sql, "INSERT INTO collaboration_event") {
		return errRow{errors.New("injected reconciliation event failure")}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func (tx *accountReconciliationFaultTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.starter.failure == "cursor" && strings.Contains(sql, "SET last_issue_id") {
		return pgconn.CommandTag{}, errors.New("injected reconciliation cursor failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *accountReconciliationFaultTx) Commit(ctx context.Context) error {
	if tx.starter.failure == "commit" {
		return errors.New("injected reconciliation commit failure")
	}
	err := tx.Tx.Commit(ctx)
	if err == nil && tx.starter.afterCommit != nil {
		tx.starter.afterCommit()
	}
	if err == nil && tx.starter.failure == "unknown_commit" {
		return &pgconn.PgError{Code: "40001", Message: "injected reconciliation unknown commit"}
	}
	return err
}

func TestCollaborationAccountReconciliationAtomicFailures(t *testing.T) {
	for _, failure := range []string{"event", "cursor", "commit", "unknown_commit"} {
		t.Run(failure, func(t *testing.T) {
			f := newAccountReconciliationFixture(t)
			grant := f.grant(t, f.first, f.target, 1, true)
			run := f.run(t, f.first, f.submit(t, f.first, f.target, 1), grant, 1)
			command := f.submit(t, f.first, f.target, 1)
			f.change(t, f.target, "disable", 1)
			fault := &accountReconciliationFaultStarter{TxStarter: f.pool, failure: failure}
			f.worker.TxStarter = fault
			if n, err := f.worker.Reconcile(t.Context(), 1); n != 0 || err == nil || !strings.Contains(err.Error(), "injected reconciliation") || fault.begins.Load() != 1 {
				t.Fatalf("failed/unknown commit was accepted or replayed: %d, %v, attempts=%d", n, err, fault.begins.Load())
			}
			if failure != "unknown_commit" {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, command.CommandID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND epoch=1 AND revoked_at IS NULL`, grant) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND claim_generation=1 AND forced_disposition IS NULL`, run.ID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='active' AND control_version=1 AND active_group_id=$2`, run.TargetID, run.GroupID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NULL`, run.ID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE kind='account_credentials_revoked'`) != 0 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE last_issue_id IS NULL AND reconciled_at IS NULL`) != 1 {
					t.Fatal("failed transaction left partial cancellation/revocation/stop/event/progress")
				}
			}
			f.worker = NewCollaborationAccountReconciler(f.svc.Queries, f.pool)
			f.drain(t)
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND claim_generation=2 AND forced_disposition='stopped' AND proof_state='unknown' AND settled_at IS NULL`, run.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND epoch=2 AND revoked_at IS NOT NULL`, grant) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE kind='account_credentials_revoked'`) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE last_issue_id=$1 AND reconciled_at IS NOT NULL`, f.first) != 1 {
				t.Fatal("restart did not recover exactly once from durable intent/progress")
			}
		})
	}
}

func TestCollaborationAccountReconciliationStartupAndShutdown(t *testing.T) {
	f := newAccountReconciliationFixture(t)
	command := f.submit(t, f.first, f.target, 1)
	f.change(t, f.target, "disable", 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.worker.TxStarter = &accountReconciliationFaultStarter{TxStarter: f.pool, afterCommit: cancel}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.worker.Run(ctx)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reconciler did not process on startup and stop on cancellation")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled'`, command.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE last_issue_id=$1 AND reconciled_at IS NULL`, f.first) != 1 {
		t.Fatal("startup scan lost committed progress during shutdown")
	}
	f.worker = NewCollaborationAccountReconciler(f.svc.Queries, f.pool)
	f.drain(t)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE reconciled_at IS NOT NULL`) != 1 {
		t.Fatal("restart did not finish interrupted scan")
	}
}

func TestCollaborationAccountReconciliationProgressRollbackGuard(t *testing.T) {
	for _, migration := range []string{"617_collaboration_account_invalidation_progress", "618_collaboration_account_invalidation_attempt", "619_collaboration_account_invalidation_poll"} {
		t.Run(migration, func(t *testing.T) {
			sql, err := os.ReadFile("../../migrations/" + migration + ".down.sql")
			if err != nil {
				t.Fatal(err)
			}
			f := newPlatformAdminFixture(t)
			if migration == "619_collaboration_account_invalidation_poll" {
				up, err := os.ReadFile("../../migrations/" + migration + ".up.sql")
				if err != nil {
					t.Fatal(err)
				}
				f.fx.Exec(t, string(up))
				if f.fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND c.relname='idx_collaboration_account_invalidation_poll' AND i.indisvalid`) != 1 {
					t.Fatal("concurrent polling index is not valid")
				}
			}
			target := f.user(t, "")
			tx, err := f.pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(t.Context(), `INSERT INTO collaboration_account_invalidation (user_id, account_epoch, revocation_key, reason) VALUES ($1,2,'test-rollback','credentials_revoked')`, target); err != nil {
				t.Fatal(err)
			}
			if _, err = f.pool.Exec(t.Context(), string(sql)); err == nil {
				t.Fatal("progress rollback waited out an uncommitted invalidation writer")
			} else {
				var pgerr *pgconn.PgError
				if !errors.As(err, &pgerr) || pgerr.Code != "55P03" {
					t.Fatalf("expected NOWAIT guard, got %v", err)
				}
			}
			if err = tx.Commit(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err = f.pool.Exec(t.Context(), string(sql)); err == nil || !strings.Contains(err.Error(), "cannot roll back retained") {
				t.Fatalf("retained progress rollback was not refused: %v", err)
			}
			f.fx.Exec(t, `DELETE FROM collaboration_account_invalidation WHERE user_id=$1`, target)
			if _, err = f.pool.Exec(t.Context(), string(sql)); err != nil {
				t.Fatalf("empty private-schema maintenance rollback failed: %v", err)
			}
		})
	}
}
