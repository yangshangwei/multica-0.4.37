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
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type adminCancellationFixture struct {
	*platformAdminFixture
	actor, runtime, agent pgtype.UUID
	control               *AdminOperationService
}

func newAdminCancellationFixture(t *testing.T) *adminCancellationFixture {
	t.Helper()
	f := newPlatformAdminFixture(t)
	for _, table := range []string{"workspace", "member", "agent", "issue", "chat_session", "chat_message", "task_message", "managed_installation", "installation_daemon_binding"} {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	// LIKE INCLUDING ALL does not copy user triggers. Use the actual migration
	// definition so queue ABA and cancellation fences are exercised here too.
	var trigger string
	f.fx.QueryRow(t, "SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgrelid='public.agent_task_queue'::regclass AND NOT tgisinternal AND tgname='agent_task_queue_state_version' ").Scan(&trigger)
	if trigger == "" {
		t.Fatal("task state-version migration is required")
	}
	// The source definition names public.agent_task_queue; replace only that
	// qualified target, keeping the trigger function and conditions unchanged.
	trigger = strings.Replace(trigger, " ON public.agent_task_queue ", " ON agent_task_queue ", 1)
	f.fx.Exec(t, trigger)
	actor := f.user(t, PlatformRoleSuperAdmin)
	workspace := f.fx.Workspace(t, "Cancellation scope", "cancel-"+uuid.NewString())
	f.fx.Member(t, workspace, util.UUIDToString(actor), "owner")
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": f.org, "workspace_id": workspace}, "workspace_id=$1", workspace)
	runtime := util.MustParseUUID(f.fx.Runtime(t, "Cancellation runtime", testutil.Cols{"workspace_id": workspace, "owner_id": actor, "runtime_mode": "local"}))
	fx := testutil.New(f.pool, workspace, util.UUIDToString(actor))
	agent := util.MustParseUUID(fx.Agent(t, "Cancellation agent", util.UUIDToString(runtime)))
	tasks := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	return &adminCancellationFixture{platformAdminFixture: f, actor: actor, runtime: runtime, agent: agent, control: NewAdminOperationService(f.svc.Queries, f.pool, tasks)}
}

func (f *adminCancellationFixture) task(t *testing.T, status string) db.AgentTaskQueue {
	t.Helper()
	fields := testutil.Cols{"runtime_id": f.runtime, "status": status}
	if status == "running" || status == "dispatched" || status == "waiting_local_directory" {
		fields["dispatched_at"] = time.Now().UTC().Truncate(time.Microsecond)
	}
	id := f.fx.Task(t, util.UUIDToString(f.agent), fields)
	task, err := f.svc.Queries.GetAgentTask(t.Context(), util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func (f *adminCancellationFixture) params(task db.AgentTaskQueue) AdminCancelTaskParams {
	return AdminCancelTaskParams{OrganizationID: f.org, TaskID: task.ID, IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Fence: AdminExecutionFence{RuntimeID: task.RuntimeID, DispatchedAt: task.DispatchedAt, StateVersion: task.StateVersion}, Reason: "Cancel isolated acceptance execution", RequestID: uuid.NewString()}
}

func TestAdminCancellationQueuedIsAtomicAndRecoverableByOriginalKey(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task := f.task(t, "queued")
	p := f.params(task)
	ctx := adminTestContext(f.actor, 1)
	result, err := f.control.CancelTask(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation.State != "succeeded" || result.Operation.ResultCode != "cancelled_before_dispatch" || result.Operation.Confirmation != "not_required" {
		t.Fatalf("receipt=%+v", result.Operation)
	}
	changed, err := f.svc.Queries.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Status != "cancelled" || changed.StateVersion != task.StateVersion+1 {
		t.Fatalf("task status/version=%s/%d", changed.Status, changed.StateVersion)
	}
	replay, err := f.control.CancelTask(ctx, p)
	if err != nil || replay.Operation.ID != result.Operation.ID || !replay.Replayed {
		t.Fatalf("replay=%+v, %v", replay, err)
	}
	found, err := f.svc.FindOperationByKey(ctx, f.org, p.IdempotencyKey)
	if err != nil || found.ID != result.Operation.ID {
		t.Fatalf("lost response lookup=%+v %v", found, err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND phase='applied'", result.Operation.ID); n != 1 {
		t.Fatalf("applied audits=%d", n)
	}
	p.Reason = "A different request"
	_, err = f.control.CancelTask(ctx, p)
	assertPlatformAdminError(t, err, "idempotency_conflict")
}

func TestAdminCancellationAuditFailureRollsBackTaskAndOperation(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task := f.task(t, "queued")
	f.control.TxStarter = platformAdminFailAuditStarter{f.pool}
	_, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
	if err == nil {
		t.Fatal("audit failure accepted cancellation")
	}
	after, err := f.svc.Queries.GetAgentTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != task.Status || after.StateVersion != task.StateVersion {
		t.Fatal("failed audit changed task")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE target_id=$1", task.ID); n != 0 {
		t.Fatalf("failed cancellation left %d operations", n)
	}
}

func TestAdminCancellationRejectsStaleFenceAndObserver(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task := f.task(t, "queued")
	p := f.params(task)
	p.Fence.StateVersion++
	_, err := f.control.CancelTask(adminTestContext(f.actor, 1), p)
	assertPlatformAdminError(t, err, "execution_fence_conflict")
	observer := f.user(t, PlatformRoleObserver)
	_, err = f.control.CancelTask(adminTestContext(observer, 1), f.params(task))
	if err == nil {
		t.Fatal("observer cancelled execution")
	}
	_, err = f.svc.Queries.GetAgentTask(context.Background(), task.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("rejected cancellation removed task")
	}
}

func (f *adminCancellationFixture) managed(t *testing.T, task db.AgentTaskQueue) (db.AgentTaskQueue, context.Context) {
	t.Helper()
	const deployment = "00000000-0000-4000-8000-000000000009"
	t.Setenv("MULTICA_DEPLOYMENT_ID", deployment)
	runtime, err := f.svc.Queries.GetAgentRuntime(t.Context(), f.runtime)
	if err != nil {
		t.Fatal(err)
	}
	daemon := uuid.NewString()
	f.fx.Exec(t, "UPDATE agent_runtime SET daemon_id=$2,metadata=metadata||'{\"capabilities\":[\"admin_cancel_ack_v1\"]}'::jsonb WHERE id=$1", f.runtime, daemon)
	installation := f.fx.Insert(t, "managed_installation", testutil.Cols{"organization_id": f.org, "deployment_id": deployment, "public_key": make([]byte, 32), "key_fingerprint": auth.HashToken(uuid.NewString()), "responsible_user_id": f.actor})
	binding := f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": installation, "workspace_id": runtime.WorkspaceID, "daemon_id": daemon, "principal_user_id": f.actor, "auth_version": int64(1), "binding_epoch": int64(1)})
	f.fx.Exec(t, "UPDATE agent_task_queue SET execution_installation_id=$2,execution_binding_id=$3,execution_binding_epoch=1 WHERE id=$1", task.ID, installation, binding)
	task, err = f.svc.Queries.GetAgentTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(f.actor), Version: 1, Kind: "daemon_token", WorkspaceID: util.UUIDToString(runtime.WorkspaceID), DaemonID: daemon, BindingID: binding, BindingEpoch: 1})
	return task, ctx
}
func cancellationAck(op db.AdminOperation, outcome string) TaskCancellationAck {
	return TaskCancellationAck{OperationID: op.ID, BindingEpoch: op.BindingEpoch.Int64, Fence: AdminExecutionFence{RuntimeID: op.ExecutionRuntimeID, DispatchedAt: op.ExecutionDispatchedAt}, Outcome: outcome}
}

func TestAdminCancellationManagedAckRequiresStoppedEvidenceAndPreservesLegacyFields(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task, daemonCtx := f.managed(t, f.task(t, "running"))
	result, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation.State != "applied" || result.Operation.Confirmation != "pending" {
		t.Fatalf("premature confirmation: %+v", result.Operation)
	}
	if err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, TaskCancellationAck{BranchName: "saved-branch"}); err != nil {
		t.Fatal(err)
	}
	legacy, err := f.svc.GetOperation(adminTestContext(f.actor, 1), f.org, result.Operation.ID)
	if err != nil || legacy.Confirmation != "pending" {
		t.Fatalf("legacy cleanup confirmed execution: %+v %v", legacy, err)
	}
	ack := cancellationAck(result.Operation, "not_observed")
	if err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, ack); err != nil {
		t.Fatal(err)
	}
	missing, _ := f.svc.GetOperation(adminTestContext(f.actor, 1), f.org, result.Operation.ID)
	if missing.State != "applied" || missing.ConfirmedAt.Valid {
		t.Fatal("missing process was presented as stopped")
	}
	ack.Outcome = "stopped"
	ack.BranchName = "must-not-replace-saved-branch"
	ack.ErrorMessage = "private preserved path /secret/work"
	if err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, ack); err != nil {
		t.Fatal(err)
	}
	if err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, ack); err != nil {
		t.Fatal(err)
	}
	confirmed, _ := f.svc.GetOperation(adminTestContext(f.actor, 1), f.org, result.Operation.ID)
	if confirmed.State != "succeeded" || confirmed.Confirmation != "confirmed" || !confirmed.ConfirmedAt.Valid {
		t.Fatalf("ACK not recorded: %+v", confirmed)
	}
	after, _ := f.svc.Queries.GetAgentTask(t.Context(), task.ID)
	if after.BranchName.String != "saved-branch" {
		t.Fatal("replayed ACK overwrote existing branch")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND phase='daemon_stopped'", confirmed.ID); n != 1 {
		t.Fatalf("daemon evidence audits=%d", n)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND after_state::text LIKE '%secret%'", confirmed.ID); n != 0 {
		t.Fatal("private diagnostic entered admin audit")
	}
}

func TestAdminCancellationConcurrentActorsRecoverOwnReceiptsAndRepairFollowers(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task, daemonCtx := f.managed(t, f.task(t, "running"))
	other := f.user(t, PlatformRoleSuperAdmin)
	actors := []pgtype.UUID{f.actor, other}
	params := []AdminCancelTaskParams{f.params(task), f.params(task)}
	results := make([]AdminCancelTaskResult, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range actors {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = f.control.CancelTask(adminTestContext(actors[i], 1), params[i])
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if results[0].Operation.ID == results[1].Operation.ID {
		t.Fatal("different actors lost separate receipts")
	}
	var root db.AdminOperation
	for i, result := range results {
		if !result.Operation.RootOperationID.Valid {
			root = result.Operation
		}
		found, err := f.svc.FindOperationByKey(adminTestContext(actors[i], 1), f.org, params[i].IdempotencyKey)
		if err != nil || found.ID != result.Operation.ID {
			t.Fatal("lost original actor/key receipt", err)
		}
	}
	if !root.ID.Valid || f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE target_task_id=$1 AND root_operation_id IS NULL", task.ID) != 1 {
		t.Fatal("concurrent cancellation did not elect one root")
	}
	if err := f.control.AcknowledgeCancellation(daemonCtx, task.ID, cancellationAck(root, "stopped")); err != nil {
		t.Fatal(err)
	}
	// Root commit is deliberately observed before the follower scanner runs.
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE root_operation_id=$1 AND state='applied'", root.ID); n != 1 {
		t.Fatal("fixture did not expose interrupted follower synchronization")
	}
	for i := range actors {
		found, err := f.svc.FindOperationByKey(adminTestContext(actors[i], 1), f.org, params[i].IdempotencyKey)
		if err != nil || found.State != "succeeded" || found.Confirmation != "confirmed" {
			t.Fatalf("effective root read contradicted confirmation: %+v %v", found, err)
		}
	}
	if err := f.control.Reconcile(t.Context(), 100); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE target_task_id=$1 AND state='succeeded'", task.ID); n != 2 {
		t.Fatalf("unrepaired receipts=%d", 2-n)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND state_version=$2", task.ID, task.StateVersion+1); n != 1 {
		t.Fatal("followers applied cancellation twice")
	}
}

func TestAdminCancellationAckAuditFailureRollsBackLegacyWrites(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task, daemonCtx := f.managed(t, f.task(t, "running"))
	result, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
	if err != nil {
		t.Fatal(err)
	}
	ack := cancellationAck(result.Operation, "stopped")
	ack.BranchName = "must-roll-back"
	f.control.TxStarter = platformAdminFailAuditStarter{f.pool}
	if err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, ack); err == nil {
		t.Fatal("failed ACK audit was accepted")
	}
	after, _ := f.svc.Queries.GetAgentTask(t.Context(), task.ID)
	op, _ := f.svc.GetOperation(adminTestContext(f.actor, 1), f.org, result.Operation.ID)
	if after.BranchName.Valid || op.State != "applied" || op.ConfirmedAt.Valid {
		t.Fatal("ACK audit failure partially committed")
	}
}

func TestAdminCancellationDeadlineAndOldBindingCannotConfirmNewExecution(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task, daemonCtx := f.managed(t, f.task(t, "running"))
	result, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
	if err != nil {
		t.Fatal(err)
	}
	f.control.Now = func() time.Time { return result.Operation.AckDeadline.Time.Add(time.Second) }
	if err = f.control.Reconcile(t.Context(), 100); err != nil {
		t.Fatal(err)
	}
	op, _ := f.svc.GetOperation(adminTestContext(f.actor, 1), f.org, result.Operation.ID)
	if op.State != "applied" || op.Confirmation != "unconfirmed" {
		t.Fatalf("deadline fabricated terminal result: %+v", op)
	}
	ack := cancellationAck(op, "stopped")
	ack.Fence.DispatchedAt.Time = ack.Fence.DispatchedAt.Time.Add(time.Second)
	err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, ack)
	assertPlatformAdminError(t, err, "cancellation_ack_conflict")
	f.fx.Exec(t, "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1", op.BindingID)
	if err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, cancellationAck(op, "stopped")); err == nil {
		t.Fatal("revoked binding confirmed cancellation")
	}
	op, _ = f.svc.GetOperation(adminTestContext(f.actor, 1), f.org, result.Operation.ID)
	if op.State != "applied" || op.ConfirmedAt.Valid {
		t.Fatal("invalid ACK modified root")
	}
}

func TestAdminCancellationUncertainCommitRecoversEffectsWithoutAnotherTaskTransition(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task := f.task(t, "queued")
	agent, err := f.svc.Queries.GetAgent(t.Context(), f.agent)
	if err != nil {
		t.Fatal(err)
	}
	issue := testutil.New(f.pool, util.UUIDToString(agent.WorkspaceID), util.UUIDToString(f.actor)).Issue(t, "Notification recovery")
	f.fx.Exec(t, "UPDATE agent_task_queue SET issue_id=$2 WHERE id=$1", task.ID, issue)
	task, err = f.svc.Queries.GetAgentTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	p := f.params(task)
	ctx := adminTestContext(f.actor, 1)
	eventsSeen := 0
	f.control.Tasks.Bus.Subscribe(protocol.EventTaskCancelled, func(events.Event) { eventsSeen++ })
	f.control.TxStarter = platformAdminUncertainCommitStarter{f.pool}
	if _, err := f.control.CancelTask(ctx, p); err == nil {
		t.Fatal("lost commit response was reported as certain")
	}
	op, err := f.svc.FindOperationByKey(ctx, f.org, p.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if eventsSeen != 0 || op.EffectsCompletedAt.Valid {
		t.Fatal("effects ran before commit outcome was known")
	}
	f.control.TxStarter = f.pool
	if err = f.control.Reconcile(t.Context(), 100); err != nil {
		t.Fatal(err)
	}
	if eventsSeen != 1 {
		t.Fatalf("recovered cancellation events=%d", eventsSeen)
	}
	replay, err := f.control.CancelTask(ctx, p)
	if err != nil || !replay.Replayed || replay.Operation.ID != op.ID {
		t.Fatal("lost-response replay created a different operation", err)
	}
	after, _ := f.svc.Queries.GetAgentTask(t.Context(), task.ID)
	if after.StateVersion != task.StateVersion+1 {
		t.Fatal("recovery repeated task transition")
	}
}

func TestAdminCancellationChatEffectsRollbackWithTheirMarkerAndRetryOnce(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task := f.task(t, "running")
	agent, err := f.svc.Queries.GetAgent(t.Context(), f.agent)
	if err != nil {
		t.Fatal(err)
	}
	session := f.fx.Insert(t, "chat_session", testutil.Cols{"workspace_id": agent.WorkspaceID, "agent_id": f.agent, "creator_id": f.actor})
	f.fx.Exec(t, "UPDATE agent_task_queue SET chat_session_id=$2 WHERE id=$1", task.ID, session)
	f.fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "user", "content": "Keep my original input", "task_id": task.ID})
	f.fx.Insert(t, "task_message", testutil.Cols{"task_id": task.ID, "seq": 1, "type": "text", "content": "Partial output"})
	task, err = f.svc.Queries.GetAgentTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.control.TxStarter = &failNamedExecTxStarter{pool: f.pool, queryName: "MarkAdminCancellationEffectsComplete", err: errors.New("effects marker unavailable")}
	result, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
	if err != nil {
		t.Fatal("postcommit effect failure changed accepted response", err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM chat_message WHERE task_id=$1 AND role='assistant'", task.ID); n != 0 {
		t.Fatal("chat effects committed without their completion marker")
	}
	f.control.TxStarter = f.pool
	if err = f.control.Reconcile(t.Context(), 100); err != nil {
		t.Fatal(err)
	}
	if err = f.control.Reconcile(t.Context(), 100); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM chat_message WHERE task_id=$1 AND role='assistant'", task.ID); n != 1 {
		t.Fatalf("recovery created %d chat outcomes", n)
	}
	op, err := f.svc.GetOperation(adminTestContext(f.actor, 1), f.org, result.Operation.ID)
	if err != nil || !op.EffectsCompletedAt.Valid {
		t.Fatal("durable effect marker missing", err)
	}
}

func TestAdminCancellationTerminalResultsAndRetryAreNeverOverwritten(t *testing.T) {
	for _, status := range []string{"completed", "failed", "cancelled", "deferred"} {
		t.Run(status, func(t *testing.T) {
			f := newAdminCancellationFixture(t)
			task := f.task(t, status)
			result, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
			if err != nil {
				t.Fatal(err)
			}
			want := status
			code := "already_terminal"
			if status == "cancelled" {
				code = "already_cancelled_unverified"
			}
			if status == "deferred" {
				want = "cancelled"
				code = "cancelled_before_dispatch"
			}
			if result.Task.Status != want || result.Operation.ResultCode != code {
				t.Fatalf("terminal handling=%s/%s", result.Task.Status, result.Operation.ResultCode)
			}
			if status == "completed" || status == "failed" {
				if err = f.control.AcknowledgeCancellation(t.Context(), task.ID, TaskCancellationAck{BranchName: "stale branch", ErrorMessage: "stale error"}); err != nil {
					t.Fatal(err)
				}
				after, _ := f.svc.Queries.GetAgentTask(t.Context(), task.ID)
				if after.Status != status || after.BranchName.Valid || after.Error.Valid {
					t.Fatal("late ACK rewrote terminal result")
				}
			}
		})
	}
}

func TestAdminCancellationLegacyManagedRuntimeDoesNotPromiseConfirmation(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task, daemonCtx := f.managed(t, f.task(t, "running"))
	f.fx.Exec(t, "UPDATE agent_runtime SET metadata='{}'::jsonb WHERE id=$1", f.runtime)
	result, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation.ResultCode != "confirmation_unavailable" || result.Operation.Confirmation != "unavailable" {
		t.Fatalf("legacy client promised ACK: %+v", result.Operation)
	}
	metadata, err := f.control.CancellationMetadata(daemonCtx, result.Task)
	if err != nil || metadata != nil {
		t.Fatal("unsupported daemon received actionable confirmation metadata", err)
	}
}

func TestAdminCancellationRacesCompletionAndNeverTargetsRetry(t *testing.T) {
	f := newAdminCancellationFixture(t)
	for i := 0; i < 12; i++ {
		task := f.task(t, "running")
		p := f.params(task)
		start := make(chan struct{})
		var cancelled AdminCancelTaskResult
		var cancelErr, completeErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			cancelled, cancelErr = f.control.CancelTask(adminTestContext(f.actor, 1), p)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, completeErr = f.control.Tasks.CompleteTask(t.Context(), task.ID, []byte(`{"text":"finished"}`), "", "", "", false, "", "")
		}()
		close(start)
		wg.Wait()
		if cancelErr != nil || completeErr != nil {
			t.Fatalf("terminal race: cancel=%v complete=%v", cancelErr, completeErr)
		}
		after, err := f.svc.Queries.GetAgentTask(t.Context(), task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.Status == "completed" {
			if cancelled.Operation.ResultCode != "already_terminal" {
				t.Fatal("late cancel rewrote completed outcome")
			}
		} else if after.Status != "cancelled" {
			t.Fatalf("unexpected terminal status %s", after.Status)
		}
		retry := f.task(t, "queued")
		f.fx.Exec(t, "UPDATE agent_task_queue SET retry_of_task_id=$2 WHERE id=$1", retry.ID, task.ID)
		if _, err = f.control.CancelTask(adminTestContext(f.actor, 1), p); err != nil {
			t.Fatal(err)
		}
		unchanged, _ := f.svc.Queries.GetAgentTask(t.Context(), retry.ID)
		if unchanged.Status != "queued" || unchanged.StateVersion != retry.StateVersion {
			t.Fatal("old cancellation modified new retry")
		}
		f.fx.Exec(t, "DELETE FROM agent_task_queue WHERE id=$1", retry.ID)
	}
}

func TestAdminCancellationQueuedABAInvalidatesStateVersion(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task := f.task(t, "queued")
	params := f.params(task)
	f.fx.Exec(t, "UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1", task.ID)
	f.fx.Exec(t, "UPDATE agent_task_queue SET status='queued',dispatched_at=NULL WHERE id=$1", task.ID)
	_, err := f.control.CancelTask(adminTestContext(f.actor, 1), params)
	assertPlatformAdminError(t, err, "execution_fence_conflict")
	current, err := f.svc.Queries.GetAgentTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.StateVersion <= task.StateVersion || current.Status != "queued" {
		t.Fatal("task transition version did not protect queue ABA")
	}
}

func TestAdminCancellationTwoScannersDeduplicateFollowerAudit(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task, daemonCtx := f.managed(t, f.task(t, "running"))
	root, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
	if err != nil {
		t.Fatal(err)
	}
	other := f.user(t, PlatformRoleSuperAdmin)
	follower, err := f.control.CancelTask(adminTestContext(other, 1), f.params(task))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, cancellationAck(root.Operation, "stopped")); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { errs <- f.control.Reconcile(context.Background(), 1) }()
	}
	for i := 0; i < 2; i++ {
		if err = <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND phase='reconciled_confirmed'", follower.Operation.ID); n != 1 {
		t.Fatalf("follower confirmation audit count=%d", n)
	}
}

func TestAdminCancellationInflightFenceSurvivesStatusAdvance(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task := f.task(t, "dispatched")
	p := f.params(task)
	f.fx.Exec(t, "UPDATE agent_task_queue SET status='running' WHERE id=$1", task.ID)
	result, err := f.control.CancelTask(adminTestContext(f.actor, 1), p)
	if err != nil || result.Task.Status != "cancelled" {
		t.Fatalf("same execution rejected after start: %+v %v", result, err)
	}
	p.Fence.StateVersion = 0 // Older clients send only the execution tuple.
	replay, err := f.control.CancelTask(adminTestContext(f.actor, 1), p)
	if err != nil || !replay.Replayed || replay.Operation.ID != result.Operation.ID {
		t.Fatal("irrelevant in-flight version changed idempotent identity", err)
	}
}

func TestAdminCancellationLockedFollowerRemainsScheduledForRepair(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task, daemonCtx := f.managed(t, f.task(t, "running"))
	root, err := f.control.CancelTask(adminTestContext(f.actor, 1), f.params(task))
	if err != nil {
		t.Fatal(err)
	}
	other := f.user(t, PlatformRoleSuperAdmin)
	follower, err := f.control.CancelTask(adminTestContext(other, 1), f.params(task))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.control.AcknowledgeCancellation(daemonCtx, task.ID, cancellationAck(root.Operation, "stopped")); err != nil {
		t.Fatal(err)
	}
	locked, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Rollback(t.Context())
	if _, err = locked.Exec(t.Context(), "SELECT id FROM admin_operation WHERE id=$1 FOR UPDATE", follower.Operation.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.control.Reconcile(t.Context(), 100); err != nil {
		t.Fatal(err)
	}
	var next *time.Time
	f.fx.QueryRow(t, "SELECT next_reconcile_at FROM admin_operation WHERE id=$1", root.Operation.ID).Scan(&next)
	if next == nil {
		t.Fatal("locked unsynchronized follower permanently unscheduled its root")
	}
	if err = locked.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.control.Now = func() time.Time { return next.Add(time.Second) }
	if err = f.control.Reconcile(t.Context(), 100); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND phase='reconciled_confirmed'", follower.Operation.ID); n != 1 {
		t.Fatalf("released follower was not repaired: audits=%d", n)
	}
}

func TestAdminCancellationDefinitiveFenceFailurePersistsAnOriginalKeyReceipt(t *testing.T) {
	f := newAdminCancellationFixture(t)
	task := f.task(t, "queued")
	p := f.params(task)
	p.Fence.StateVersion++
	ctx := adminTestContext(f.actor, 1)
	_, err := f.control.CancelTask(ctx, p)
	assertPlatformAdminError(t, err, "execution_fence_conflict")
	failed, err := f.svc.FindOperationByKey(ctx, f.org, p.IdempotencyKey)
	if err != nil {
		t.Fatalf("definitive rejection has no recoverable receipt: %v", err)
	}
	if failed.State != "failed" || failed.ResultCode != "execution_fence_conflict" || failed.AppliedAt.Valid || failed.NextReconcileAt.Valid {
		t.Fatalf("rejection receipt=%+v", failed)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND phase='failed'", failed.ID); n != 1 {
		t.Fatalf("failed audits=%d", n)
	}
	f.fx.Exec(t, "UPDATE agent_task_queue SET status='deferred' WHERE id=$1", task.ID)
	replay, err := f.control.CancelTask(ctx, p)
	if err != nil || !replay.Replayed || replay.Operation.ID != failed.ID || replay.Operation.State != "failed" {
		t.Fatal("rejected original key later applied", err)
	}
	current, err := f.svc.Queries.GetAgentTask(ctx, task.ID)
	if err != nil || current.Status != "deferred" {
		t.Fatal("failed receipt replay changed task", err)
	}
	accepted, err := f.control.CancelTask(ctx, f.params(current))
	if err != nil || accepted.Operation.RootOperationID.Valid || accepted.Operation.State != "succeeded" {
		t.Fatal("failed receipt became cancellation root", err)
	}
	follower, err := f.control.CancelTask(ctx, f.params(current))
	if err != nil || follower.Operation.RootOperationID != accepted.Operation.ID {
		t.Fatal("root lookup selected an earlier failed request", err)
	}
}

func TestClaimRejectionCannotCancelOrFailAReclaimedDelivery(t *testing.T) {
	f := newAdminCancellationFixture(t)
	for _, action := range []string{"cancel", "reason", "fail"} {
		t.Run(action, func(t *testing.T) {
			task := f.task(t, "dispatched")
			f.fx.Exec(t, "UPDATE agent_task_queue SET claim_generation=claim_generation+1 WHERE id=$1", task.ID)
			var err error
			switch action {
			case "cancel":
				_, err = f.control.Tasks.CancelClaimedTask(t.Context(), task)
			case "reason":
				_, err = f.control.Tasks.CancelClaimedTaskWithReason(t.Context(), task, "old runtime lacks support", "local_directory_error")
			case "fail":
				_, err = f.control.Tasks.FailClaimedTask(t.Context(), task, "stale rejection", "agent_error")
			}
			if !errors.Is(err, ErrClaimSuperseded) {
				t.Fatalf("stale rejection error=%v", err)
			}
			current, e := f.svc.Queries.GetAgentTask(t.Context(), task.ID)
			if e != nil || current.Status != "dispatched" || current.Error.Valid {
				t.Fatalf("stale claim changed current delivery: %s %v", current.Status, e)
			}
			f.fx.Exec(t, "DELETE FROM agent_task_queue WHERE id=$1", task.ID)
		})
	}
}

func TestClaimRejectionCurrentDeliveryKeepsChatEffectsAndAutomaticRetry(t *testing.T) {
	for _, action := range []string{"cancel", "fail"} {
		t.Run(action, func(t *testing.T) {
			f := newAdminCancellationFixture(t)
			task := f.task(t, "dispatched")
			agent, err := f.svc.Queries.GetAgent(t.Context(), f.agent)
			if err != nil {
				t.Fatal(err)
			}
			session := f.fx.Insert(t, "chat_session", testutil.Cols{"workspace_id": agent.WorkspaceID, "agent_id": f.agent, "creator_id": f.actor})
			f.fx.Exec(t, "UPDATE agent_task_queue SET chat_session_id=$2,attempt=1,max_attempts=2 WHERE id=$1", task.ID, session)
			f.fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": session, "role": "user", "content": "Preserve this input", "task_id": task.ID})
			f.fx.Insert(t, "task_message", testutil.Cols{"task_id": task.ID, "seq": 1, "type": "text", "content": "Partial response"})
			task, err = f.svc.Queries.GetAgentTask(t.Context(), task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if action == "cancel" {
				result, err := f.control.Tasks.CancelClaimedTask(t.Context(), task)
				if err != nil || result.Status != "cancelled" {
					t.Fatal("current claim cancellation failed", err)
				}
				if n := f.fx.Count(t, "SELECT count(*) FROM chat_message WHERE task_id=$1 AND role='assistant'", task.ID); n != 1 {
					t.Fatalf("postcommit chat effect blocked by claim guard: %d", n)
				}
			} else {
				result, err := f.control.Tasks.FailClaimedTask(t.Context(), task, "runtime temporarily unavailable", "runtime_offline")
				if err != nil || result.Status != "failed" {
					t.Fatal("current claim failure failed", err)
				}
				if n := f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE retry_of_task_id=$1 AND status='deferred'", task.ID); n != 1 {
					t.Fatalf("claim guard changed automatic retry behavior: %d", n)
				}
			}
		})
	}
}
