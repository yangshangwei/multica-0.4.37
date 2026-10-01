package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type adminAlertFixture struct {
	*platformAdminFixture
	alerts                           *AdminAlertService
	actor, workspace, runtime, agent pgtype.UUID
}

func newAdminAlertFixture(t *testing.T) *adminAlertFixture {
	t.Helper()
	f := newPlatformAdminFixture(t)
	for _, table := range []string{"admin_alert", "admin_alert_detector_state", "workspace", "member", "agent", "issue", "managed_installation", "installation_daemon_binding"} {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	actor := f.user(t, PlatformRoleSuperAdmin)
	workspace := util.MustParseUUID(f.fx.Workspace(t, "Alert scope", "alerts-"+uuid.NewString()))
	f.fx.Member(t, util.UUIDToString(workspace), util.UUIDToString(actor), "owner")
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": f.org, "workspace_id": workspace}, "workspace_id=$1", workspace)
	fx := testutil.New(f.pool, util.UUIDToString(workspace), util.UUIDToString(actor))
	runtime := util.MustParseUUID(fx.Runtime(t, "Alert runtime", testutil.Cols{"owner_id": actor, "runtime_mode": "local"}))
	agent := util.MustParseUUID(fx.Agent(t, "Alert agent", util.UUIDToString(runtime)))
	return &adminAlertFixture{f, NewAdminAlertService(f.svc.Queries, f.pool, "00000000-0000-4000-8000-000000000009", nil), actor, workspace, runtime, agent}
}
func (f *adminAlertFixture) task(t *testing.T, status string) pgtype.UUID {
	t.Helper()
	fields := testutil.Cols{"runtime_id": f.runtime, "status": status}
	if status == "queued" {
		fields["queued_at"] = time.Now().Add(-10 * time.Minute)
		fields["queued_at_source"] = "transition"
	}
	if status == "failed" || status == "completed" {
		fields["completed_at"] = time.Now().Add(-time.Minute)
	}
	return util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), fields))
}
func (f *adminAlertFixture) observe(t *testing.T, rule string, subject pgtype.UUID, condition string, at time.Time) db.AdminAlert {
	t.Helper()
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	rows, err := f.alerts.applyObservations(t.Context(), db.New(tx), f.org, []alertObservation{{Rule: rule, SubjectID: subject, Condition: condition, ObservedAt: at, ResolutionCode: "queue_left"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		return db.AdminAlert{}
	}
	return rows[0]
}
func (f *adminAlertFixture) mutation(alert db.AdminAlert, action string) AdminAlertMutationParams {
	return AdminAlertMutationParams{OrganizationID: f.org, AlertID: alert.ID, IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Action: action, ExpectedVersion: alert.Version, Reason: "Review isolated alert", RequestID: uuid.NewString()}
}

func TestAdminAlertClosedFailureIsNotRecreatedByCompensation(t *testing.T) {
	f := newAdminAlertFixture(t)
	task := f.task(t, "failed")
	now := time.Now().UTC()
	first := f.observe(t, "execution_failed", task, "problem", now)
	duplicate := f.observe(t, "execution_failed", task, "problem", now.Add(time.Second))
	if first.ID != duplicate.ID || duplicate.OccurrenceCount != 1 || duplicate.Version != first.Version {
		t.Fatal("repeat observation counted another failure")
	}
	p := f.mutation(duplicate, "close")
	p.ResolutionCode = "handled"
	closed, err := f.alerts.Mutate(adminTestContext(f.actor, 1), p)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Target.Status != "closed" || !closed.Target.ConditionActive {
		t.Fatal("manual handling erased historical source fact")
	}
	again := f.observe(t, "execution_failed", task, "problem", now.Add(time.Minute))
	if again.ID != first.ID || again.Status != "closed" || f.fx.Count(t, "SELECT count(*) FROM admin_alert") != 1 {
		t.Fatal("compensation reopened closed failed task")
	}
	source, err := f.svc.Queries.GetAgentTask(t.Context(), task)
	if err != nil || source.Status != "failed" {
		t.Fatal("alert closure changed original task", err)
	}
}

func TestAdminAlertAcknowledgementRecoveryAndNewEpisodeStayDistinct(t *testing.T) {
	f := newAdminAlertFixture(t)
	task := f.task(t, "queued")
	now := time.Now().UTC()
	first := f.observe(t, "queue_timeout", task, "problem", now)
	ack, err := f.alerts.Mutate(adminTestContext(f.actor, 1), f.mutation(first, "acknowledge"))
	if err != nil {
		t.Fatal(err)
	}
	if ack.Target.Status != "acknowledged" || !ack.Target.ConditionActive || ack.Target.ResolvedAt.Valid {
		t.Fatal("acknowledgement claimed recovery")
	}
	recovered := f.observe(t, "queue_timeout", task, "healthy", now.Add(time.Minute))
	if recovered.Status != "resolved" || recovered.ConditionActive {
		t.Fatal("verified recovery did not resolve episode")
	}
	stale := f.observe(t, "queue_timeout", task, "problem", now.Add(time.Second))
	if stale.ID != first.ID || stale.Status != "resolved" {
		t.Fatal("out-of-order failure reopened newer recovery")
	}
	second := f.observe(t, "queue_timeout", task, "problem", now.Add(2*time.Minute))
	if second.ID == first.ID || second.Status != "open" || second.OccurrenceCount != 1 {
		t.Fatal("new fault did not get a fresh episode")
	}
}

func TestAdminAlertAssignConflictHasDurableFailedReceipt(t *testing.T) {
	f := newAdminAlertFixture(t)
	alert := f.observe(t, "queue_timeout", f.task(t, "queued"), "problem", time.Now())
	actors := []pgtype.UUID{f.actor, f.user(t, PlatformRoleSuperAdmin)}
	params := []AdminAlertMutationParams{f.mutation(alert, "assign"), f.mutation(alert, "assign")}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range actors {
		params[i].AssigneeID = actors[i]
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, errs[i] = f.alerts.Mutate(adminTestContext(actors[i], 1), params[i]) }(i)
	}
	wg.Wait()
	succeeded := 0
	for i, err := range errs {
		op, lookupErr := f.svc.FindOperationByKey(adminTestContext(actors[i], 1), f.org, params[i].IdempotencyKey)
		if lookupErr != nil {
			t.Fatal(lookupErr)
		}
		if err == nil {
			succeeded++
			if op.State != "succeeded" {
				t.Fatal("successful assignment receipt incorrect")
			}
		} else {
			assertPlatformAdminError(t, err, "alert_version_conflict")
			if op.State != "failed" || op.AppliedAt.Valid {
				t.Fatal("conflict lacks durable failed receipt")
			}
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent assignments succeeded=%d", succeeded)
	}
}

func TestAdminAlertObserverAndAuditFailureCannotMutate(t *testing.T) {
	f := newAdminAlertFixture(t)
	alert := f.observe(t, "queue_timeout", f.task(t, "queued"), "problem", time.Now())
	observer := f.user(t, PlatformRoleObserver)
	if _, err := f.alerts.Mutate(adminTestContext(observer, 1), f.mutation(alert, "acknowledge")); err == nil {
		t.Fatal("observer handled alert")
	}
	f.alerts.TxStarter = platformAdminFailAuditStarter{f.pool}
	if _, err := f.alerts.Mutate(adminTestContext(f.actor, 1), f.mutation(alert, "acknowledge")); err == nil {
		t.Fatal("failed audit accepted mutation")
	}
	current, err := f.svc.Queries.GetAdminAlert(t.Context(), db.GetAdminAlertParams{ID: alert.ID, OrganizationID: f.org})
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != "open" || current.Version != alert.Version || f.fx.Count(t, "SELECT count(*) FROM admin_operation") != 0 {
		t.Fatal("rejected mutation partially committed")
	}
}

func TestAdminAlertCloseRequiresRealResolutionEvidence(t *testing.T) {
	f := newAdminAlertFixture(t)
	failed := f.task(t, "failed")
	alert := f.observe(t, "execution_failed", failed, "problem", time.Now())
	unrelated := f.task(t, "completed")
	p := f.mutation(alert, "close")
	p.ResolutionCode = "retry_succeeded"
	p.RelatedTaskID = unrelated
	_, err := f.alerts.Mutate(adminTestContext(f.actor, 1), p)
	assertPlatformAdminError(t, err, "alert_resolution_invalid")
	retry := f.task(t, "completed")
	f.fx.Exec(t, "UPDATE agent_task_queue SET retry_of_task_id=$2 WHERE id=$1", retry, failed)
	p = f.mutation(alert, "close")
	p.ResolutionCode = "retry_succeeded"
	p.RelatedTaskID = retry
	result, err := f.alerts.Mutate(adminTestContext(f.actor, 1), p)
	if err != nil || result.Target.Status != "closed" {
		t.Fatal("valid completed retry was rejected", err)
	}
	queue := f.observe(t, "queue_timeout", f.task(t, "queued"), "problem", time.Now())
	_, err = f.alerts.Mutate(adminTestContext(f.actor, 1), f.mutation(queue, "close"))
	assertPlatformAdminError(t, err, "alert_state_conflict")
}

type alertTestLiveness struct {
	ok    bool
	alive map[string]bool
	check func(context.Context)
}

func (s *alertTestLiveness) Available() bool { return true }
func (s *alertTestLiveness) IsAliveBatch(ctx context.Context, _ []string) (map[string]bool, bool) {
	if s.check != nil {
		s.check(ctx)
	}
	return s.alive, s.ok
}

func (f *adminAlertFixture) installation(t *testing.T, inflight bool, now time.Time) (pgtype.UUID, pgtype.UUID) {
	t.Helper()
	installation := util.MustParseUUID(f.fx.Insert(t, "managed_installation", testutil.Cols{"organization_id": f.org, "deployment_id": f.alerts.DeploymentID, "public_key": make([]byte, 32), "key_fingerprint": auth.HashToken(uuid.NewString()), "responsible_user_id": f.actor, "created_at": now.Add(-time.Hour)}))
	daemon := uuid.NewString()
	f.fx.Exec(t, "UPDATE agent_runtime SET daemon_id=$2,last_seen_at=$3 WHERE id=$1", f.runtime, daemon, now.Add(-10*time.Minute))
	binding := f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": installation, "workspace_id": f.workspace, "daemon_id": daemon, "principal_user_id": f.actor, "auth_version": int64(1), "binding_epoch": int64(1), "last_seen_at": now.Add(-10 * time.Minute)})
	f.fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(uuid.NewString()), "workspace_id": f.workspace, "daemon_id": daemon, "user_id": f.actor, "auth_version": int64(1), "installation_binding_id": binding, "installation_binding_epoch": int64(1), "expires_at": now.Add(time.Hour)})
	task := f.task(t, "running")
	if inflight {
		f.fx.Exec(t, "UPDATE agent_task_queue SET execution_installation_id=$2,execution_binding_id=$3,execution_binding_epoch=1 WHERE id=$1", task, installation, binding)
	}
	return installation, task
}

func TestAdminAlertOfflineSourceFailurePausesOpeningAndRecovery(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	installation, _ := f.installation(t, true, now)
	liveness := &alertTestLiveness{ok: false, alive: map[string]bool{}}
	liveness.check = func(ctx context.Context) {
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, "SELECT rule FROM admin_alert_detector_state WHERE organization_id=$1 AND rule=$2 FOR UPDATE NOWAIT", f.org, AdminAlertInstallationUnreachable); err != nil {
			t.Fatal("liveness was read while holding detector locks", err)
		}
	}
	f.alerts.Liveness = liveness
	f.alerts.Now = func() time.Time { return now }
	if err := f.alerts.ScanRule(t.Context(), AdminAlertInstallationUnreachable); err == nil {
		t.Fatal("failed liveness source was accepted")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert"); n != 0 {
		t.Fatal("source outage generated offline alert")
	}
	health, err := f.alerts.DetectorHealth(t.Context(), f.org)
	if err != nil {
		t.Fatal(err)
	}
	if health[0].SourceState != "unavailable" || health[0].LastSuccessfulAt != nil {
		t.Fatalf("failure reported healthy detector: %+v", health[0])
	}
	now = now.Add(time.Second)
	liveness.ok = true
	if err = f.alerts.ScanRule(t.Context(), AdminAlertInstallationUnreachable); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1 AND status='open'", installation); n != 1 {
		t.Fatal("known unreachable installation not detected")
	}
	now = now.Add(time.Second)
	liveness.ok = false
	liveness.alive[util.UUIDToString(f.runtime)] = true
	if err = f.alerts.ScanRule(t.Context(), AdminAlertInstallationUnreachable); err == nil {
		t.Fatal("failed source fabricated recovery")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1 AND condition_active", installation); n != 1 {
		t.Fatal("source outage resolved offline alert")
	}
	now = now.Add(time.Second)
	liveness.ok = true
	if err = f.alerts.ScanRule(t.Context(), AdminAlertInstallationUnreachable); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1 AND status='resolved' AND NOT condition_active", installation); n != 1 {
		t.Fatal("verified liveness did not resolve alert")
	}
}

func TestAdminAlertOfflineDoesNotInferUnassociatedOrMissingEvidence(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC()
	installation, task := f.installation(t, false, now)
	f.alerts.Now = func() time.Time { return now }
	if err := f.alerts.ScanRule(t.Context(), AdminAlertInstallationUnreachable); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM admin_alert") != 0 {
		t.Fatal("historical runtime-only task became an installation alert")
	}
	f.fx.Exec(t, "UPDATE agent_task_queue SET execution_installation_id=$2,execution_binding_id=(SELECT id FROM installation_daemon_binding WHERE installation_id=$2),execution_binding_epoch=1 WHERE id=$1", task, installation)
	f.fx.Exec(t, "UPDATE installation_daemon_binding SET last_seen_at=NULL WHERE installation_id=$1", installation)
	f.fx.Exec(t, "UPDATE agent_runtime SET last_seen_at=NULL WHERE id=$1", f.runtime)
	now = now.Add(time.Second)
	if err := f.alerts.ScanRule(t.Context(), AdminAlertInstallationUnreachable); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM admin_alert") != 0 {
		t.Fatal("missing heartbeat evidence became offline")
	}
}

func TestAdminAlertCyclicFailureScanFindsLateCommitsAndOldCreatedTasks(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	f.alerts.Now = func() time.Time { return now }
	for i := 0; i < 201; i++ {
		id := f.task(t, "failed")
		f.fx.Exec(t, "UPDATE agent_task_queue SET created_at=$2,completed_at=$3 WHERE id=$1", id, now.Add(-90*24*time.Hour), now.Add(-time.Hour).Add(time.Duration(i)*time.Second))
	}
	if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert"); n != 200 {
		t.Fatalf("first batch=%d", n)
	}
	late := f.task(t, "failed")
	f.fx.Exec(t, "UPDATE agent_task_queue SET completed_at=$2 WHERE id=$1", late, now.Add(-2*time.Hour))
	if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert"); n != 201 {
		t.Fatalf("remaining batch=%d", n)
	}
	if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1", late); n != 1 {
		t.Fatal("completed_at watermark permanently skipped late commit")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE occurrence_count<>1"); n != 0 {
		t.Fatal("compensation counted old failures again")
	}
}

func TestAdminAlertQueueClockAndUnknownCarryAcrossBatches(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	f.alerts.Now = func() time.Time { return now }
	unknown := f.task(t, "queued")
	f.fx.Exec(t, "UPDATE agent_task_queue SET queued_at=NULL,queued_at_source=NULL,created_at=$2 WHERE id=$1", unknown, now.Add(-time.Hour))
	for i := 0; i < 200; i++ {
		id := f.task(t, "queued")
		f.fx.Exec(t, "UPDATE agent_task_queue SET queued_at=$2,queued_at_source='observation',created_at=$3 WHERE id=$1", id, now.Add(-time.Minute), now.Add(-90*24*time.Hour))
	}
	if err := f.alerts.ScanRule(t.Context(), AdminAlertQueueTimeout); err != nil {
		t.Fatal(err)
	}
	if err := f.alerts.ScanRule(t.Context(), AdminAlertQueueTimeout); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM admin_alert") != 0 {
		t.Fatal("old creation time replaced conservative queue observation clock")
	}
	health, err := f.alerts.DetectorHealth(t.Context(), f.org)
	if err != nil {
		t.Fatal(err)
	}
	if health[1].SourceState != "unknown" || health[1].UnknownCount != 1 {
		t.Fatalf("last clean batch erased earlier unknown evidence: %+v", health[1])
	}
}

func TestAdminAlertDetectorWriteFailureRollsBackAndReportsUnavailable(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC()
	f.alerts.Now = func() time.Time { return now }
	f.task(t, "failed")
	if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err != nil {
		t.Fatal(err)
	}
	late := f.task(t, "failed")
	now = now.Add(time.Second)
	f.alerts.TxStarter = &failNamedExecTxStarter{pool: f.pool, queryName: "CompleteAdminAlertDetectorBatch", err: errors.New("checkpoint unavailable")}
	if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err == nil {
		t.Fatal("failed checkpoint accepted detector changes")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1", late); n != 0 {
		t.Fatal("alert committed without its detector checkpoint")
	}
	health, err := f.alerts.DetectorHealth(t.Context(), f.org)
	if err != nil {
		t.Fatal(err)
	}
	if health[2].SourceState != "unavailable" || health[2].LastErrorCode == nil {
		t.Fatalf("write failure left detector healthy: %+v", health[2])
	}
	f.alerts.TxStarter = f.pool
	now = now.Add(time.Second)
	if err = f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1", late); n != 1 {
		t.Fatal("failed detector batch was not retried")
	}
}

func TestAdminAlertMissingAssigneeHasDurableFailedReceipt(t *testing.T) {
	f := newAdminAlertFixture(t)
	alert := f.observe(t, AdminAlertQueueTimeout, f.task(t, "queued"), "problem", time.Now())
	p := f.mutation(alert, "assign")
	p.AssigneeID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	_, err := f.alerts.Mutate(adminTestContext(f.actor, 1), p)
	assertPlatformAdminError(t, err, "alert_assignee_unavailable")
	op, err := f.svc.FindOperationByKey(adminTestContext(f.actor, 1), f.org, p.IdempotencyKey)
	if err != nil || op.State != "failed" {
		t.Fatal("missing assignee left uncertain original key", err)
	}
}

func TestAdminAlertAuditDoesNotCopyUncontrolledResolutionText(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC()
	task := f.task(t, "queued")
	f.observe(t, AdminAlertQueueTimeout, task, "problem", now)
	resolved := f.observe(t, AdminAlertQueueTimeout, task, "healthy", now.Add(time.Second))
	f.fx.Exec(t, "UPDATE admin_alert SET resolution_code='PRIVATE /home/secret' WHERE id=$1", resolved.ID)
	if _, err := f.alerts.Mutate(adminTestContext(f.actor, 1), f.mutation(resolved, "close")); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE target_id=$1 AND (before_state::text LIKE '%PRIVATE%' OR after_state::text LIKE '%PRIVATE%')", resolved.ID); n != 0 {
		t.Fatal("uncontrolled diagnostic escaped through audit snapshots")
	}
}

func TestAdminAlertSupersededDetectorCannotApplyStaleOfflineObservation(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	f.installation(t, true, now)
	entered, release := make(chan struct{}), make(chan struct{})
	f.alerts.Now = func() time.Time { return now }
	f.alerts.Liveness = &alertTestLiveness{ok: true, alive: map[string]bool{}, check: func(ctx context.Context) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
		}
	}}
	done := make(chan error, 1)
	go func() { done <- f.alerts.ScanRule(context.Background(), AdminAlertInstallationUnreachable) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first detector did not reach liveness")
	}
	newer := NewAdminAlertService(f.svc.Queries, f.pool, f.alerts.DeploymentID, &alertTestLiveness{ok: true, alive: map[string]bool{util.UUIDToString(f.runtime): true}})
	newer.Now = func() time.Time { return now.Add(31 * time.Second) }
	if err := newer.ScanRule(t.Context(), AdminAlertInstallationUnreachable); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert"); n != 0 {
		t.Fatal("expired detector lease applied old offline evidence after recovery")
	}
}

func TestAdminAlertFailureHintsAndUnknownCommitDoNotDuplicateEpisodes(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC()
	f.alerts.Now = func() time.Time { return now }
	task := f.task(t, "failed")
	if err := f.alerts.processFailureHints(t.Context(), []pgtype.UUID{task, task}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	f.alerts.TxStarter = platformAdminUncertainCommitStarter{f.pool}
	if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err == nil {
		t.Fatal("uncertain commit reported a certain outcome")
	}
	f.alerts.TxStarter = f.pool
	now = now.Add(time.Second)
	if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1 AND occurrence_count=1", task); n != 1 {
		t.Fatal("event hint plus scan created duplicate failure episodes")
	}
}

func TestAdminAlertFailureCycleHorizonWrapsDespiteSustainedNewTail(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	origin := now
	f.alerts.Now = func() time.Time { return now }
	for i := 0; i < 201; i++ {
		id := f.task(t, "failed")
		f.fx.Exec(t, "UPDATE agent_task_queue SET completed_at=$2 WHERE id=$1", id, origin.Add(-time.Hour).Add(time.Duration(i)*time.Second))
	}
	if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err != nil {
		t.Fatal(err)
	}
	late := f.task(t, "failed")
	f.fx.Exec(t, "UPDATE agent_task_queue SET completed_at=$2 WHERE id=$1", late, origin.Add(-2*time.Hour))
	for round := 0; round < 3; round++ {
		start := now
		now = now.Add(10 * time.Minute)
		for i := 0; i < 200; i++ {
			id := f.task(t, "failed")
			f.fx.Exec(t, "UPDATE agent_task_queue SET completed_at=$2 WHERE id=$1", id, start.Add(time.Duration(i+1)*time.Second))
		}
		if err := f.alerts.ScanRule(t.Context(), AdminAlertExecutionFailed); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1", late); n != 1 {
		t.Fatal("moving scan horizon kept late pre-cursor failure invisible under sustained load")
	}
}

type alertQueueSourceFailure struct{ *pgxpool.Pool }

func (s alertQueueSourceFailure) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "ListAdminAlertQueueCandidates") {
		return nil, errors.New("queue source unavailable")
	}
	return s.Pool.Query(ctx, query, args...)
}

func TestAdminAlertRemovedQueueTaskRequiresSuccessfulAbsenceObservation(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC()
	f.alerts.Now = func() time.Time { return now }
	task := f.task(t, "queued")
	if err := f.alerts.ScanRule(t.Context(), AdminAlertQueueTimeout); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, "DELETE FROM agent_task_queue WHERE id=$1", task)
	f.alerts.Queries = db.New(alertQueueSourceFailure{f.pool})
	now = now.Add(time.Second)
	if err := f.alerts.ScanRule(t.Context(), AdminAlertQueueTimeout); err == nil {
		t.Fatal("source error was treated as confirmed absence")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1 AND condition_active", task); n != 1 {
		t.Fatal("failed source query resolved an alert")
	}
	f.alerts.Queries = f.svc.Queries
	now = now.Add(time.Second)
	if err := f.alerts.ScanRule(t.Context(), AdminAlertQueueTimeout); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1 AND status='resolved' AND NOT condition_active AND resolution_code='task_removed' AND version=2", task); n != 1 {
		t.Fatal("confirmed deleted task left an unclosable queue alert")
	}
	now = now.Add(time.Second)
	if err := f.alerts.ScanRule(t.Context(), AdminAlertQueueTimeout); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1", task); n != 1 {
		t.Fatal("removed task recreated an episode")
	}
}

func TestAdminAlertUnmappedQueueSubjectIsUnknownNotRemoved(t *testing.T) {
	f := newAdminAlertFixture(t)
	now := time.Now().UTC()
	f.alerts.Now = func() time.Time { return now }
	task := f.task(t, "queued")
	if err := f.alerts.ScanRule(t.Context(), AdminAlertQueueTimeout); err != nil {
		t.Fatal(err)
	}
	workspace := f.fx.Workspace(t, "Foreign alert scope", "foreign-alert-"+uuid.NewString())
	fx := testutil.New(f.pool, workspace, util.UUIDToString(f.actor))
	runtime := fx.Runtime(t, "Foreign runtime")
	agent := fx.Agent(t, "Foreign agent", runtime)
	f.fx.Exec(t, "UPDATE agent_task_queue SET agent_id=$2,runtime_id=$3 WHERE id=$1", task, agent, runtime)
	now = now.Add(time.Second)
	if err := f.alerts.ScanRule(t.Context(), AdminAlertQueueTimeout); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_alert WHERE subject_id=$1 AND condition_active AND status='open'", task); n != 1 {
		t.Fatal("out-of-scope task was misreported as deleted/recovered")
	}
}
