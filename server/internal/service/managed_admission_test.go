package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestManagedAdmissionStopsEveryClaimPrimitive(t *testing.T) {
	for _, path := range []string{"agent", "runtime", "batch", "sql"} {
		t.Run(path, func(t *testing.T) {
			f, source, runtimeID := managedRuntimeFixture(t)
			f.fx.Exec(t, "CREATE TABLE agent (LIKE public.agent INCLUDING ALL)")
			fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
			agentID := mustManagedUUID(t, fx.Agent(t, "Admission fixture", util.UUIDToString(runtimeID)))
			taskID := fx.Task(t, util.UUIDToString(agentID), testutil.Cols{"runtime_id": runtimeID})
			f.fx.Exec(t, "UPDATE managed_installation SET admission='stopped',admission_version=2 WHERE id=(SELECT installation_id FROM installation_daemon_binding WHERE id=$1)", source.BindingID)
			ctx := auth.WithPasswordSession(t.Context(), source)
			svc := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
			var claimed bool
			var err error
			switch path {
			case "agent":
				var task *db.AgentTaskQueue
				task, err = svc.ClaimTask(ctx, agentID)
				claimed = task != nil
			case "runtime":
				var task *db.AgentTaskQueue
				task, err = svc.ClaimTaskForRuntime(ctx, runtimeID)
				claimed = task != nil
			case "batch":
				var tasks []db.AgentTaskQueue
				tasks, err = svc.ClaimTasksForRuntimes(ctx, []pgtype.UUID{runtimeID}, 1)
				claimed = len(tasks) != 0
			case "sql":
				_, err = f.svc.Queries.ClaimAgentTask(ctx, db.ClaimAgentTaskParams{AgentID: agentID, RuntimeID: runtimeID, PrepareLeaseSecs: 60, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
				claimed = err == nil
				if errors.Is(err, pgx.ErrNoRows) {
					err = nil
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if claimed || fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'", taskID) != 1 {
				t.Fatal("stopped installation admitted a new execution")
			}
		})
	}
}

type admissionReadSignalStarter struct {
	pool    *pgxpool.Pool
	reached chan struct{}
}

func (s admissionReadSignalStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	return admissionReadSignalTx{Tx: tx, reached: s.reached}, err
}

type admissionReadSignalTx struct {
	pgx.Tx
	reached chan struct{}
}

func (tx admissionReadSignalTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "-- name: LockManagedInstallationForAdmissionRead") {
		select {
		case tx.reached <- struct{}{}:
		default:
		}
	}
	return tx.Tx.QueryRow(ctx, query, args...)
}

func TestManagedAdmissionStopSerializesBeforeClaim(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	f.fx.Exec(t, "CREATE TABLE agent (LIKE public.agent INCLUDING ALL)")
	fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
	agentID := mustManagedUUID(t, fx.Agent(t, "Concurrent admission", util.UUIDToString(runtimeID)))
	taskID := fx.Task(t, util.UUIDToString(agentID), testutil.Cols{"runtime_id": runtimeID})
	ctx := auth.WithPasswordSession(t.Context(), source)
	binding, err := f.svc.Queries.GetInstallationBinding(ctx, mustManagedUUID(t, source.BindingID))
	if err != nil {
		t.Fatal(err)
	}
	stop, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stop.Rollback(context.Background())
	if _, err = db.New(stop).LockManagedInstallation(ctx, binding.InstallationID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.New(stop).UpdateManagedInstallationAdmission(ctx, db.UpdateManagedInstallationAdmissionParams{ID: binding.InstallationID, AdmissionVersion: 1, Admission: "stopped"}); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{}, 1)
	svc := NewTaskService(f.svc.Queries, admissionReadSignalStarter{f.pool, reached}, nil, events.New())
	type result struct {
		task *db.AgentTaskQueue
		err  error
	}
	done := make(chan result, 1)
	go func() { task, err := svc.ClaimTask(ctx, agentID); done <- result{task, err} }()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("claim did not reach admission fence")
	}
	select {
	case result := <-done:
		t.Fatalf("claim passed uncommitted stop: %+v", result)
	case <-time.After(30 * time.Millisecond):
	}
	if err = stop.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || result.task != nil {
			t.Fatalf("claim ignored committed stop: %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("claim did not finish after stop committed")
	}
	if fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'", taskID) != 1 {
		t.Fatal("concurrent claim escaped admission")
	}
}

func TestManagedAdmissionOperationCASReplayAndAuditRollback(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	actor := f.platformAdminFixture.user(t, PlatformRoleSuperAdmin)
	ctx := adminTestContext(actor, 1)
	binding, err := f.svc.Queries.GetInstallationBinding(ctx, mustManagedUUID(t, source.BindingID))
	if err != nil {
		t.Fatal(err)
	}
	p := AdminAdmissionParams{OrganizationID: f.org, InstallationID: binding.InstallationID, IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, ExpectedVersion: 1, Admission: "stopped", Reason: "Pause new claims during maintenance", RequestID: "admission-test"}
	beforeAudit := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event")
	f.management.TxStarter = platformAdminFailAuditStarter{f.pool}
	if _, err = f.management.ChangeAdmission(ctx, p, nil); err == nil {
		t.Fatal("audit failure committed admission")
	}
	if f.fx.Count(t, "SELECT count(*) FROM managed_installation WHERE id=$1 AND admission='accepting' AND admission_version=1", binding.InstallationID) != 1 || f.fx.Count(t, "SELECT count(*) FROM admin_operation") != 0 {
		t.Fatal("failed admission partially committed")
	}
	f.management.TxStarter = f.pool
	first, err := f.management.ChangeAdmission(ctx, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Operation.State != "succeeded" || first.Operation.ResultCode != "admission_stopped" || first.Installation.AdmissionVersion != 2 {
		t.Fatalf("invalid admission result: %+v", first)
	}
	replay, err := f.management.ChangeAdmission(ctx, p, nil)
	if err != nil || !replay.Replayed || replay.Operation.ID != first.Operation.ID || f.fx.Count(t, "SELECT count(*) FROM admin_audit_event") != beforeAudit+3 {
		t.Fatalf("replay duplicated policy/audit: %v", err)
	}
	p.Admission = "accepting"
	if _, err = f.management.ChangeAdmission(ctx, p, nil); err == nil {
		t.Fatal("same key changed its intended policy")
	}
	p.IdempotencyKey = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if _, err = f.management.ChangeAdmission(ctx, p, nil); err == nil {
		t.Fatal("stale version replaced newer policy")
	}
	p.ExpectedVersion = 2
	observer := f.platformAdminFixture.user(t, PlatformRoleObserver)
	if _, err = f.management.ChangeAdmission(adminTestContext(observer, 1), p, nil); err == nil {
		t.Fatal("observer changed admission")
	}
	resumed, err := f.management.ChangeAdmission(ctx, p, NewTaskService(f.svc.Queries, f.pool, nil, events.New()))
	if err != nil || resumed.Installation.AdmissionVersion != 3 || resumed.Installation.Admission != "accepting" {
		t.Fatalf("resume failed: %v", err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM agent_runtime WHERE id=$1", runtimeID) != 1 {
		t.Fatal("admission changed runtime ownership/lifecycle")
	}
}

func TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(strconv.FormatBool(batch), func(t *testing.T) {
			f, source, runtimeID := managedRuntimeFixture(t)
			f.fx.Exec(t, "CREATE TABLE agent (LIKE public.agent INCLUDING ALL)")
			fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
			agentID := fx.Agent(t, "Recovery admission", util.UUIDToString(runtimeID))
			taskID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID})
			ctx := auth.WithPasswordSession(t.Context(), source)
			svc := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
			first, err := svc.ClaimTaskForRuntime(ctx, runtimeID)
			if err != nil || first == nil || !first.ExecutionAdmissionVersion.Valid || first.ExecutionAdmissionVersion.Int64 != 1 {
				t.Fatalf("first claim omitted admission: %v %+v", err, first)
			}
			fence := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
			fx.Exec(t, "UPDATE agent_task_queue SET dispatched_at=$2,prepare_lease_expires_at=$2 WHERE id=$1", taskID, fence)
			fx.Exec(t, "UPDATE managed_installation SET admission='stopped',admission_version=2 WHERE id=$1", first.ExecutionInstallationID)
			var reclaimed []db.AgentTaskQueue
			if batch {
				reclaimed, err = svc.ClaimTasksForRuntimes(ctx, []pgtype.UUID{runtimeID}, 1)
			} else {
				var task *db.AgentTaskQueue
				task, err = svc.ClaimTaskForRuntime(ctx, runtimeID)
				if task != nil {
					reclaimed = append(reclaimed, *task)
				}
			}
			if err != nil || len(reclaimed) != 1 || !reclaimed[0].DispatchedAt.Time.Equal(fence) || reclaimed[0].ExecutionAdmissionVersion.Int64 != 1 {
				t.Fatalf("admitted fence changed or lost during stop: %v %+v", err, reclaimed)
			}
			fx.Exec(t, "UPDATE agent_task_queue SET execution_admission_version=NULL,prepare_lease_expires_at=$2 WHERE id=$1", taskID, fence)
			if batch {
				reclaimed, err = svc.ClaimTasksForRuntimes(ctx, []pgtype.UUID{runtimeID}, 1)
			} else {
				var task *db.AgentTaskQueue
				task, err = svc.ClaimTaskForRuntime(ctx, runtimeID)
				reclaimed = nil
				if task != nil {
					reclaimed = append(reclaimed, *task)
				}
			}
			if err != nil || len(reclaimed) != 0 {
				t.Fatalf("unproven recovery bypassed stop: %v %+v", err, reclaimed)
			}
			fx.Exec(t, "UPDATE managed_installation SET admission='accepting',admission_version=3 WHERE id=$1", first.ExecutionInstallationID)
			recovered, recoveryErr := svc.ClaimTaskForRuntime(ctx, runtimeID)
			if recoveryErr != nil || recovered == nil || recovered.ExecutionAdmissionVersion.Valid || !recovered.DispatchedAt.Time.Equal(fence) {
				t.Fatalf("pre-control managed delivery lost compatibility or fabricated admission proof: %v %+v", recoveryErr, recovered)
			}
		})
	}
}

func TestManagedReclaimRejectsEarlierDeliveryRollback(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	f.fx.Exec(t, "CREATE TABLE agent (LIKE public.agent INCLUDING ALL)")
	fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
	agentID := fx.Agent(t, "Delivery fence", util.UUIDToString(runtimeID))
	taskID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID})
	ctx := auth.WithPasswordSession(t.Context(), source)
	svc := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	first, err := svc.ClaimTaskForRuntime(ctx, runtimeID)
	if err != nil || first == nil {
		t.Fatalf("claim A: %v", err)
	}
	fx.Exec(t, "UPDATE agent_task_queue SET prepare_lease_expires_at=now()-interval '1 minute' WHERE id=$1", taskID)
	second, err := f.svc.Queries.ReclaimStaleDispatchedTaskForRuntime(ctx, db.ReclaimStaleDispatchedTaskForRuntimeParams{RuntimeID: runtimeID, ClaimRecoverySecs: -1, PrepareLeaseSecs: 60, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
	if err != nil || !second.DispatchedAt.Time.Equal(first.DispatchedAt.Time) {
		t.Fatalf("reclaim B lost stable execution fence: %v", err)
	}
	token := db.CreateTaskTokenParams{TokenHash: auth.HashToken("mat_" + uuid.NewString()), TaskID: first.ID, AgentID: first.AgentID, WorkspaceID: mustManagedUUID(t, source.WorkspaceID), UserID: mustManagedUUID(t, source.UserID), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}
	if _, err = svc.FinalizeTaskClaim(ctx, *first, token, nil, false); err == nil {
		t.Fatal("stale delivery minted a task credential")
	}
	if fx.Count(t, "SELECT count(*) FROM task_token") != 0 {
		t.Fatal("stale finalization leaked credentials")
	}
	if _, err = f.svc.Queries.SetTaskDeliveredCommentIDs(ctx, db.SetTaskDeliveredCommentIDsParams{TaskID: first.ID, RuntimeID: first.RuntimeID, DispatchedAt: first.DispatchedAt, ClaimGeneration: first.ClaimGeneration, DeliveredCommentIds: []pgtype.UUID{}}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale delivery changed its comment receipt: %v", err)
	}
	if _, err = svc.FinalizeTaskClaim(ctx, second, token, nil, false); err != nil {
		t.Fatalf("current delivery could not finalize: %v", err)
	}
	if _, err = svc.RequeueTaskAfterClaimFailure(ctx, *first); err == nil {
		t.Fatal("delayed failure of delivery A rolled back reclaimed delivery B")
	}
	if fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='dispatched'", taskID) != 1 {
		t.Fatal("new delivery was returned to the queue")
	}
}
