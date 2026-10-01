package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// TaskCancellationEffects carries only work that must run after the caller's
// transaction commits. It is never executed on a rolled-back cancellation.
type TaskCancellationEffects struct {
	Result  CancelTaskResult
	Options CancelTaskOptions
	Changed bool
}

var ErrClaimSuperseded = errors.New("the claimed task delivery has changed")

func lockClaimedTask(ctx context.Context, q *db.Queries, expected db.AgentTaskQueue) error {
	if err := lockChatSessionForTaskWrite(ctx, q, expected.ID); err != nil {
		return err
	}
	current, err := q.GetCancellationTaskForUpdate(ctx, expected.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrClaimSuperseded
	}
	if err != nil {
		return err
	}
	if !expected.RuntimeID.Valid || !expected.DispatchedAt.Valid || current.Status != "dispatched" || current.StartedAt.Valid || current.RuntimeID != expected.RuntimeID || !sameCancellationTime(current.DispatchedAt, expected.DispatchedAt) || current.ClaimGeneration != expected.ClaimGeneration {
		return ErrClaimSuperseded
	}
	return nil
}

func (s *TaskService) CancelClaimedTask(ctx context.Context, task db.AgentTaskQueue) (*db.AgentTaskQueue, error) {
	return s.CancelClaimedTaskWithReason(ctx, task, "", "")
}

func (s *TaskService) CancelClaimedTaskWithReason(ctx context.Context, task db.AgentTaskQueue, errorMessage, failureReason string) (*db.AgentTaskQueue, error) {
	if s.TxStarter == nil {
		return nil, errors.New("claim rejection requires a transaction")
	}
	result, err := s.CancelTaskWithResult(ctx, task.ID, CancelTaskOptions{ClientSupportsDraftRestore: true, ErrorMessage: errorMessage, FailureReason: failureReason, expectedClaim: &task})
	if err != nil {
		return nil, err
	}
	return &result.Task, nil
}

func (s *TaskService) FailClaimedTask(ctx context.Context, task db.AgentTaskQueue, errorMessage, failureReason string) (*db.AgentTaskQueue, error) {
	if s.TxStarter == nil {
		return nil, errors.New("claim rejection requires a transaction")
	}
	return s.failTask(ctx, task.ID, errorMessage, "", "", "", failureReason, false, "", "", &task)
}

// CancelTaskInTx preserves the shared chat-session -> task order and existing
// explicit-user cancellation semantics without opening or committing a tx.
func (s *TaskService) CancelTaskInTx(ctx context.Context, q *db.Queries, taskID pgtype.UUID, opts CancelTaskOptions) (*TaskCancellationEffects, error) {
	opts.ErrorMessage = util.SanitizeTextForPostgres(opts.ErrorMessage)
	opts.FailureReason = util.SanitizeTextForPostgres(opts.FailureReason)
	if opts.UserInitiated && (opts.ErrorMessage != "" || opts.FailureReason != "") {
		return nil, errors.New("user-initiated cancellation cannot carry a server failure reason")
	}
	if opts.expectedClaim != nil {
		if opts.expectedClaim.ID != taskID {
			return nil, ErrClaimSuperseded
		}
		if err := lockClaimedTask(ctx, q, *opts.expectedClaim); err != nil {
			return nil, err
		}
	}
	effects := &TaskCancellationEffects{Options: opts}
	var task db.AgentTaskQueue
	var err error
	if opts.QueuedOnly {
		if opts.QueueAction != "edit" && opts.QueueAction != "remove" {
			return nil, errors.New("queue action must be edit or remove")
		}
		if _, err = q.LockChatSessionForTask(ctx, taskID); err != nil {
			return nil, fmt.Errorf("lock queued chat session: %w", err)
		}
		task, err = q.CancelQueuedAgentTask(ctx, db.CancelQueuedAgentTaskParams{ID: taskID, ChatSessionID: opts.ExpectedChatSession})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNoLongerQueued
		}
		if err != nil {
			return nil, fmt.Errorf("cancel queued task: %w", err)
		}
		effects.Result.CancelledChatMessage, err = s.settleQueuedChatInput(ctx, q, task, opts.QueueAction)
		if err != nil {
			return nil, err
		}
	} else {
		if err = lockChatSessionForTaskWrite(ctx, q, taskID); err != nil {
			return nil, err
		}
		switch {
		case opts.UserInitiated:
			task, err = q.CancelAgentTaskByUser(ctx, taskID)
		case opts.ErrorMessage != "" || opts.FailureReason != "":
			task, err = q.CancelAgentTaskWithReason(ctx, db.CancelAgentTaskWithReasonParams{ID: taskID, Error: pgtype.Text{String: opts.ErrorMessage, Valid: opts.ErrorMessage != ""}, FailureReason: pgtype.Text{String: opts.FailureReason, Valid: opts.FailureReason != ""}})
		default:
			task, err = q.CancelAgentTask(ctx, taskID)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			task, err = q.GetAgentTask(ctx, taskID)
			if err != nil {
				return nil, fmt.Errorf("cancel task: %w", err)
			}
			effects.Result.Task = task
			return effects, nil
		}
		if err != nil {
			return nil, fmt.Errorf("cancel task: %w", err)
		}
		if task.ChatSessionID.Valid {
			if err = q.AdvanceCancelledChatSessionPointer(ctx, task.ID); err != nil {
				return nil, err
			}
		}
	}
	effects.Result.Task = task
	effects.Changed = true
	return effects, nil
}

// ApplyCancellationEffects keeps the legacy best-effort post-commit contract.
func (s *TaskService) ApplyCancellationEffects(ctx context.Context, effects *TaskCancellationEffects) *CancelTaskResult {
	if !effects.Changed {
		return &effects.Result
	}
	task := effects.Result.Task
	slog.Info("task cancelled", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	s.captureTaskCancelled(ctx, task)
	if !effects.Options.QueuedOnly {
		effects.Result.CancelledChatMessage = s.finalizeCancelledChatMessage(ctx, task, effects.Options)
	}
	s.ReconcileAgentStatus(ctx, task.AgentID)
	s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)
	s.NotifyTaskFinished(task)
	return &effects.Result
}
