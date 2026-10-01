package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const adminCancellationAckWindow = 10 * time.Minute
const adminCancellationScanInterval = 15 * time.Second

type AdminExecutionFence struct {
	RuntimeID    pgtype.UUID
	DispatchedAt pgtype.Timestamptz
	StateVersion int64
}
type AdminCancelTaskParams struct {
	OrganizationID, TaskID, IdempotencyKey pgtype.UUID
	Fence                                  AdminExecutionFence
	Reason, RequestID                      string
}
type AdminCancelTaskResult struct {
	Operation db.AdminOperation
	Task      db.AgentTaskQueue
	Replayed  bool
}
type AdminOperationService struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Tasks     *TaskService
	Now       func() time.Time
}

func NewAdminOperationService(q *db.Queries, tx TxStarter, tasks *TaskService) *AdminOperationService {
	return &AdminOperationService{Queries: q, TxStarter: tx, Tasks: tasks, Now: time.Now}
}
func cancellationTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}
func sameCancellationTime(a, b pgtype.Timestamptz) bool {
	return a.Valid == b.Valid && (!a.Valid || a.Time.Equal(b.Time))
}
func cancellationFenceMatches(task db.AgentTaskQueue, fence AdminExecutionFence) bool {
	return task.RuntimeID == fence.RuntimeID && sameCancellationTime(task.DispatchedAt, fence.DispatchedAt) && (task.DispatchedAt.Valid || task.StateVersion == fence.StateVersion)
}
func cancellationHash(p AdminCancelTaskParams) (string, error) {
	// Once dispatched, ordinary progress changes the task state version without
	// creating another execution. Its immutable runtime/time tuple is the fence.
	if p.Fence.DispatchedAt.Valid {
		p.Fence.StateVersion = 0
	}
	payload, err := json.Marshal(struct {
		TaskID pgtype.UUID
		Fence  AdminExecutionFence
		Reason string
	}{p.TaskID, p.Fence, strings.TrimSpace(p.Reason)})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func (s *AdminOperationService) CancelTask(ctx context.Context, p AdminCancelTaskParams) (AdminCancelTaskResult, error) {
	var result AdminCancelTaskResult
	if !p.OrganizationID.Valid || !p.TaskID.Valid || !p.IdempotencyKey.Valid || (!p.Fence.DispatchedAt.Valid && p.Fence.StateVersion < 1) || (p.Fence.DispatchedAt.Valid && !p.Fence.RuntimeID.Valid) {
		return result, platformError(400, "invalid_request", "A task, idempotency key and execution fence are required")
	}
	if err := validateAdminReason(p.Reason, p.RequestID); err != nil {
		return result, err
	}
	hash, err := cancellationHash(p)
	if err != nil {
		return result, err
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	op, replayed, err := NewPlatformAdminService(s.Queries, s.TxStarter).CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{OrganizationID: p.OrganizationID, TargetKind: "task", TargetID: p.TaskID, Kind: "task.cancel", IdempotencyKey: p.IdempotencyKey, PayloadHash: hash, Reason: strings.TrimSpace(p.Reason), State: "applied", ResultCode: "cancellation_accepted"})
	if err != nil {
		return result, err
	}
	if replayed {
		result.Operation, err = effectiveAdminOperation(ctx, q, op)
		if err != nil {
			return result, err
		}
		result.Task, err = q.GetAgentTask(ctx, p.TaskID)
		if err != nil {
			return result, err
		}
		result.Replayed = true
		return result, nil
	}
	if err = lockChatSessionForTaskWrite(ctx, q, p.TaskID); err != nil {
		return result, err
	}
	task, err := q.GetAdminCancellationTaskForUpdate(ctx, db.GetAdminCancellationTaskForUpdateParams{TaskID: p.TaskID, OrganizationID: p.OrganizationID})
	if err != nil {
		return result, err
	}
	now := cancellationTimestamp(s.Now())
	metadata := db.SetAdminCancellationOperationParams{ID: op.ID, OrganizationID: p.OrganizationID, TaskID: task.ID, RuntimeID: task.RuntimeID, DispatchedAt: task.DispatchedAt, ExecutionFence: task.StateVersion, InstallationID: task.ExecutionInstallationID, BindingID: task.ExecutionBindingID, BindingEpoch: task.ExecutionBindingEpoch, State: "applied", ResultCode: "confirmation_unavailable", Confirmation: "unavailable", ReconciliationState: "complete", AppliedAt: now, NextReconcileAt: now}
	changed := false
	var definitiveError error
	switch task.Status {
	case "cancelled":
		root, findErr := q.FindAdminCancellationRoot(ctx, db.FindAdminCancellationRootParams{OrganizationID: p.OrganizationID, TaskID: task.ID, RuntimeID: task.RuntimeID, DispatchedAt: task.DispatchedAt})
		if findErr == nil {
			if task.RuntimeID != p.Fence.RuntimeID || !sameCancellationTime(task.DispatchedAt, p.Fence.DispatchedAt) || (!task.DispatchedAt.Valid && p.Fence.StateVersion != task.StateVersion && p.Fence.StateVersion != root.ExecutionFence.Int64) {
				definitiveError = platformError(409, "execution_fence_conflict", "The execution changed; refresh its state")
				break
			}
			metadata.RootOperationID = root.ID
			metadata.ExecutionFence = root.ExecutionFence.Int64
			metadata.InstallationID, metadata.BindingID, metadata.BindingEpoch = root.TargetInstallationID, root.BindingID, root.BindingEpoch
			metadata.State, metadata.ResultCode, metadata.Confirmation, metadata.ReconciliationState = root.State, root.ResultCode, root.Confirmation, root.ReconciliationState
			metadata.AppliedAt, metadata.ConfirmedAt, metadata.AckDeadline = root.AppliedAt, root.ConfirmedAt, root.AckDeadline
			metadata.NextReconcileAt = pgtype.Timestamptz{}
		} else if errors.Is(findErr, pgx.ErrNoRows) {
			metadata.State = "succeeded"
			metadata.ResultCode = "already_cancelled_unverified"
		} else {
			return result, findErr
		}
		metadata.EffectsCompletedAt = now
	case "completed", "failed":
		metadata.State, metadata.ResultCode, metadata.Confirmation = "succeeded", "already_terminal", "not_required"
		metadata.EffectsCompletedAt = now
	default:
		if !cancellationFenceMatches(task, p.Fence) {
			definitiveError = platformError(409, "execution_fence_conflict", "The execution changed; refresh its state")
			break
		}
		if task.Status != "queued" && task.Status != "deferred" && task.Status != "dispatched" && task.Status != "running" && task.Status != "waiting_local_directory" {
			return result, platformError(409, "execution_not_cancellable", "This execution state cannot be cancelled")
		}
		if (task.Status == "queued" || task.Status == "deferred") && !task.DispatchedAt.Valid {
			metadata.State, metadata.ResultCode, metadata.Confirmation = "succeeded", "cancelled_before_dispatch", "not_required"
		} else {
			canConfirm, err := cancellationConfirmationSupported(ctx, q, task)
			if err != nil {
				return result, err
			}
			if canConfirm {
				metadata.ResultCode, metadata.Confirmation, metadata.ReconciliationState = "awaiting_daemon_confirmation", "pending", "pending"
				metadata.AckDeadline = cancellationTimestamp(now.Time.Add(adminCancellationAckWindow))
			}
		}
		effects, cancelErr := s.Tasks.CancelTaskInTx(ctx, q, task.ID, CancelTaskOptions{UserInitiated: true, ClientSupportsDraftRestore: true})
		if cancelErr != nil {
			return result, cancelErr
		}
		task, changed = effects.Result.Task, effects.Changed
	}
	if definitiveError != nil {
		metadata.State, metadata.ResultCode, metadata.Confirmation, metadata.ReconciliationState = "failed", "execution_fence_conflict", "not_required", "complete"
		metadata.AppliedAt, metadata.NextReconcileAt = pgtype.Timestamptz{}, pgtype.Timestamptz{}
		metadata.EffectsCompletedAt = now
	}
	op, err = q.SetAdminCancellationOperation(ctx, metadata)
	if err != nil {
		return result, err
	}
	phases := []string{"request", "applied"}
	if definitiveError != nil {
		phases = []string{"request", "failed"}
	}
	for _, phase := range phases {
		if err = recordCancellationPhase(ctx, q, op, phase, p.RequestID); err != nil {
			return result, err
		}
	}
	if op.State == "succeeded" {
		if err = recordCancellationPhase(ctx, q, op, "succeeded", p.RequestID); err != nil {
			return result, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Operation, result.Task = op, task
	if definitiveError != nil {
		return result, definitiveError
	}
	if changed {
		if err = s.completeCancellationEffects(ctx, op); err != nil {
			slog.Warn("admin cancellation effects deferred", "operation_id", util.UUIDToString(op.ID), "error", err)
		}
	}
	return result, nil
}

func cancellationConfirmationSupported(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) (bool, error) {
	if !task.ExecutionBindingID.Valid || !task.ExecutionBindingEpoch.Valid || !task.RuntimeID.Valid || !task.DispatchedAt.Valid {
		return false, nil
	}
	runtime, err := q.GetAgentRuntime(ctx, task.RuntimeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var metadata struct {
		Capabilities []string `json:"capabilities"`
	}
	if json.Unmarshal(runtime.Metadata, &metadata) != nil {
		return false, nil
	}
	supported := false
	for _, capability := range metadata.Capabilities {
		if capability == protocol.DaemonCapabilityAdminCancelAckV1 {
			supported = true
			break
		}
	}
	if !supported {
		return false, nil
	}
	binding, err := q.GetInstallationBinding(ctx, task.ExecutionBindingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return binding.State == "active" && binding.BindingEpoch == task.ExecutionBindingEpoch.Int64 && binding.InstallationID == task.ExecutionInstallationID && binding.WorkspaceID == runtime.WorkspaceID && binding.DaemonID == runtime.DaemonID.String && binding.PrincipalUserID == runtime.OwnerID, nil
}

func recordCancellationPhase(ctx context.Context, q *db.Queries, op db.AdminOperation, phase, requestID string) error {
	after, err := json.Marshal(map[string]any{"state": op.State, "result_code": op.ResultCode, "confirmation": op.Confirmation, "reconciliation_state": op.ReconciliationState})
	if err != nil {
		return err
	}
	return q.RecordAdminCancellationPhase(ctx, db.RecordAdminCancellationPhaseParams{OperationID: op.ID, OrganizationID: op.OrganizationID, ActorKind: op.ActorKind, ActorUserID: op.ActorID, TargetKind: op.TargetKind, TargetID: op.TargetID, Action: op.Kind, Phase: phase, RequestID: requestID, Reason: op.Reason, BeforeState: []byte(`{}`), AfterState: after, ResultCode: op.ResultCode})
}

func effectiveAdminOperation(ctx context.Context, q *db.Queries, op db.AdminOperation) (db.AdminOperation, error) {
	if !op.RootOperationID.Valid {
		return op, nil
	}
	root, err := q.GetAdminOperationRoot(ctx, op.ID)
	if err != nil {
		return db.AdminOperation{}, fmt.Errorf("read operation root: %w", err)
	}
	op.State, op.ResultCode, op.Confirmation, op.ReconciliationState = root.State, root.ResultCode, root.Confirmation, root.ReconciliationState
	op.ConfirmedAt, op.AckDeadline = root.ConfirmedAt, root.AckDeadline
	return op, nil
}

// completeCancellationEffects makes the durable chat changes and their marker
// one commit. Publishing after that commit remains a best-effort acceleration;
// clients and daemons can recover the authoritative cancelled row by polling.
func (s *AdminOperationService) completeCancellationEffects(ctx context.Context, op db.AdminOperation) error {
	if op.RootOperationID.Valid || op.EffectsCompletedAt.Valid {
		return nil
	}
	task, err := s.Queries.GetAgentTask(ctx, op.TargetTaskID)
	if err != nil {
		return err
	}
	if _, err = s.Queries.RefreshAgentStatusFromTasks(ctx, task.AgentID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if err = lockChatSessionForTaskWrite(ctx, q, task.ID); err != nil {
		return err
	}
	task, err = q.GetCancellationTaskForUpdate(ctx, task.ID)
	if err != nil {
		return err
	}
	current, err := q.GetAdminCancellationRootForUpdate(ctx, op.ID)
	if err != nil {
		return err
	}
	if current.EffectsCompletedAt.Valid {
		return nil
	}
	if task.Status != "cancelled" {
		return errors.New("cancellation effects require the cancelled target")
	}
	if _, err = s.Tasks.finalizeCancelledChatMessageInTx(ctx, q, task, CancelTaskOptions{UserInitiated: true, ClientSupportsDraftRestore: true}); err != nil {
		return err
	}
	if err = q.DeleteTaskTokensByTask(ctx, task.ID); err != nil {
		return err
	}
	if err = q.MarkAdminCancellationEffectsComplete(ctx, db.MarkAdminCancellationEffectsCompleteParams{ID: op.ID, CompletedAt: cancellationTimestamp(s.Now())}); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.Tasks.captureTaskCancelled(ctx, task)
	s.Tasks.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)
	s.Tasks.NotifyTaskFinished(task)
	return nil
}

// Reconcile performs bounded database work. Leases are committed before any
// task finalization to avoid holding an operation lock while waiting on tasks.
func (s *AdminOperationService) Reconcile(ctx context.Context, limit int32) error {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	now := s.Now().UTC()
	rows, err := s.Queries.LeaseDueAdminCancellations(ctx, db.LeaseDueAdminCancellationsParams{AsOf: cancellationTimestamp(now), LeaseUntil: cancellationTimestamp(now.Add(adminCancellationScanInterval)), BatchLimit: limit})
	if err != nil {
		return err
	}
	var failures []error
	for _, op := range rows {
		if err = s.completeCancellationEffects(ctx, op); err != nil {
			failures = append(failures, err)
			continue
		}
		if err = s.reconcileCancellationRoot(ctx, op.ID, now, limit); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
func (s *AdminOperationService) reconcileCancellationRoot(ctx context.Context, id pgtype.UUID, now time.Time, limit int32) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	root, err := q.GetAdminCancellationRootForUpdate(ctx, id)
	if err != nil {
		return err
	}
	if root.State == "applied" && root.Confirmation == "pending" && root.AckDeadline.Valid && !root.AckDeadline.Time.After(now) {
		root, err = q.MarkAdminCancellationUnconfirmed(ctx, db.MarkAdminCancellationUnconfirmedParams{ID: id, AsOf: cancellationTimestamp(now)})
		if err != nil {
			return err
		}
		if err = recordCancellationPhase(ctx, q, root, "unconfirmed", "coordinator"); err != nil {
			return err
		}
	}
	followers, err := q.ListAdminCancellationFollowers(ctx, db.ListAdminCancellationFollowersParams{RootID: id, BatchLimit: limit})
	if err != nil {
		return err
	}
	for _, follower := range followers {
		updated, err := q.SyncAdminCancellationFollower(ctx, db.SyncAdminCancellationFollowerParams{ID: follower.ID, ExpectedVersion: follower.Version})
		if err != nil {
			return err
		}
		phase := "reconciled_" + updated.Confirmation
		if err = recordCancellationPhase(ctx, q, updated, phase, "coordinator"); err != nil {
			return err
		}
	}
	remaining, err := q.HasUnreconciledAdminCancellationFollowers(ctx, id)
	if err != nil {
		return err
	}
	next := pgtype.Timestamptz{}
	if remaining || !root.EffectsCompletedAt.Valid {
		next = cancellationTimestamp(now.Add(adminCancellationScanInterval))
	} else if root.State == "applied" && root.Confirmation == "pending" {
		next = root.AckDeadline
	}
	if err = q.ScheduleAdminCancellationReconcile(ctx, db.ScheduleAdminCancellationReconcileParams{ID: id, NextReconcileAt: next}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *AdminOperationService) Run(ctx context.Context, observers ...func(error)) {
	ticker := time.NewTicker(adminCancellationScanInterval)
	defer ticker.Stop()
	for {
		err := s.Reconcile(ctx, 100)
		if ctx.Err() == nil {
			for _, observe := range observers {
				if observe != nil {
					observe(err)
				}
			}
		}
		if err != nil && ctx.Err() == nil {
			slog.Warn("admin cancellation reconciliation failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type AdminCancellationMetadata struct {
	OperationID    string                     `json:"operation_id"`
	BindingEpoch   string                     `json:"binding_epoch"`
	ExecutionFence AdminCancellationWireFence `json:"execution_fence"`
	AckDeadline    string                     `json:"ack_deadline"`
}
type AdminCancellationWireFence struct {
	RuntimeID    string `json:"runtime_id"`
	DispatchedAt string `json:"dispatched_at"`
}

func (s *AdminOperationService) CancellationMetadata(ctx context.Context, task db.AgentTaskQueue) (*AdminCancellationMetadata, error) {
	if task.Status != "cancelled" {
		return nil, nil
	}
	op, err := s.Queries.GetTaskCancellationRoot(ctx, task.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !op.BindingID.Valid || !op.BindingEpoch.Valid || !op.ExecutionRuntimeID.Valid || !op.ExecutionDispatchedAt.Valid || !op.AckDeadline.Valid {
		return nil, nil
	}
	source, ok := auth.PasswordSessionFromContext(ctx)
	if !ok || source.Kind != "daemon_token" || source.BindingID != util.UUIDToString(op.BindingID) || source.BindingEpoch != op.BindingEpoch.Int64 {
		return nil, ErrManagedRuntimeSource
	}
	return &AdminCancellationMetadata{OperationID: util.UUIDToString(op.ID), BindingEpoch: strconv.FormatInt(op.BindingEpoch.Int64, 10), ExecutionFence: AdminCancellationWireFence{RuntimeID: util.UUIDToString(op.ExecutionRuntimeID), DispatchedAt: op.ExecutionDispatchedAt.Time.UTC().Format(time.RFC3339Nano)}, AckDeadline: op.AckDeadline.Time.UTC().Format(time.RFC3339Nano)}, nil
}

type TaskCancellationAck struct {
	OperationID                                             pgtype.UUID
	BindingEpoch                                            int64
	Fence                                                   AdminExecutionFence
	Outcome                                                 string
	BranchName, DurableWorkDir, ErrorMessage, FailureReason string
}

func (s *AdminOperationService) AcknowledgeCancellation(ctx context.Context, taskID pgtype.UUID, ack TaskCancellationAck) error {
	managed := ack.OperationID.Valid || ack.BindingEpoch != 0 || ack.Fence.RuntimeID.Valid || ack.Fence.DispatchedAt.Valid || ack.Outcome != ""
	if managed && (!ack.OperationID.Valid || ack.BindingEpoch < 1 || !ack.Fence.RuntimeID.Valid || !ack.Fence.DispatchedAt.Valid || (ack.Outcome != "stopped" && ack.Outcome != "not_observed")) {
		return platformError(400, "invalid_cancel_ack", "Complete cancellation evidence is required")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	preview, err := q.GetAgentTask(ctx, taskID)
	if err != nil {
		return err
	}
	if preview.RuntimeID.Valid {
		runtime, err := q.GetAgentRuntime(ctx, preview.RuntimeID)
		if err != nil {
			return err
		}
		if _, err = LockManagedRuntime(ctx, q, runtime); err != nil {
			return err
		}
	}
	if err = lockChatSessionForTaskWrite(ctx, q, taskID); err != nil {
		return err
	}
	task, err := q.GetCancellationTaskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	if managed {
		root, err := q.GetAdminCancellationRootForUpdate(ctx, ack.OperationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return platformError(409, "cancellation_ack_conflict", "The cancellation receipt no longer matches this execution")
		}
		if err != nil {
			return err
		}
		source, ok := auth.PasswordSessionFromContext(ctx)
		if !ok || source.Kind != "daemon_token" || source.BindingID != util.UUIDToString(root.BindingID) || source.BindingEpoch != ack.BindingEpoch || root.BindingEpoch.Int64 != ack.BindingEpoch || root.TargetTaskID != task.ID || root.ExecutionRuntimeID != ack.Fence.RuntimeID || !sameCancellationTime(root.ExecutionDispatchedAt, ack.Fence.DispatchedAt) || task.RuntimeID != ack.Fence.RuntimeID || !sameCancellationTime(task.DispatchedAt, ack.Fence.DispatchedAt) || task.ExecutionBindingID != root.BindingID || task.ExecutionBindingEpoch != root.BindingEpoch {
			return platformError(409, "cancellation_ack_conflict", "The cancellation receipt no longer matches this execution")
		}
		if task.Status != "cancelled" {
			return platformError(409, "cancellation_ack_conflict", "The execution was not cancelled by this operation")
		}
		if ack.Outcome == "stopped" && root.State == "applied" {
			root, err = q.ConfirmAdminCancellationRoot(ctx, db.ConfirmAdminCancellationRootParams{ID: root.ID, ExpectedVersion: root.Version, ConfirmedAt: cancellationTimestamp(s.Now())})
			if err != nil {
				return err
			}
		}
		if err = recordCancellationPhase(ctx, q, root, "daemon_"+ack.Outcome, "daemon-receipt"); err != nil {
			return err
		}
	}
	if v := strings.TrimSpace(util.SanitizeTextForPostgres(ack.DurableWorkDir)); v != "" {
		if err = q.SetAgentTaskDurableWorkDir(ctx, db.SetAgentTaskDurableWorkDirParams{ID: taskID, DurableWorkDir: pgtype.Text{String: v, Valid: true}}); err != nil {
			return err
		}
	}
	if v := strings.TrimSpace(util.SanitizeTextForPostgres(ack.BranchName)); v != "" {
		if err = q.SetAgentTaskBranchName(ctx, db.SetAgentTaskBranchNameParams{ID: taskID, BranchName: pgtype.Text{String: v, Valid: true}}); err != nil {
			return err
		}
	}
	if v := strings.TrimSpace(util.SanitizeTextForPostgres(ack.ErrorMessage)); v != "" {
		reason := strings.TrimSpace(util.SanitizeTextForPostgres(ack.FailureReason))
		if err = q.SetAgentTaskErrorIfEmpty(ctx, db.SetAgentTaskErrorIfEmptyParams{ID: taskID, Error: pgtype.Text{String: v, Valid: true}, FailureReason: pgtype.Text{String: reason, Valid: reason != ""}}); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.Tasks.RebroadcastCancelledTask(ctx, taskID)
	s.Tasks.FinalizeDeferredCancelledChat(ctx, taskID)
	return nil
}
