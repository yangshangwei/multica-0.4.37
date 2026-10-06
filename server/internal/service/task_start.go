package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// startTaskAtomically owns the former autocommit start without adding retries.
// Even an issue with no iteration must take the workspace fence before its row
// lock, so a join committed while waiting is included in the recorded facts.
func (s *TaskService) startTaskAtomically(ctx context.Context, current db.AgentTaskQueue) (db.AgentTaskQueue, error) {
	if s.TxStarter == nil {
		return db.AgentTaskQueue{}, errors.New("task start requires a transaction starter")
	}
	operationID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED"); err != nil {
		return db.AgentTaskQueue{}, err
	}
	q := s.Queries.WithTx(tx)
	var issue db.Issue
	var lockedIteration db.Iteration
	if current.IssueID.Valid {
		// This read only resolves the workspace; facts are read after the fence.
		issue, err = q.GetIssue(ctx, current.IssueID)
		if err != nil {
			return db.AgentTaskQueue{}, err
		}
		if _, err = q.LockWorkspaceForChatSessionCreate(ctx, issue.WorkspaceID); err != nil {
			return db.AgentTaskQueue{}, err
		}
		if err = q.LockIssueStatusCatalogShared(ctx, issue.WorkspaceID); err != nil {
			return db.AgentTaskQueue{}, err
		}
		if err = iteration.LockWorkspace(ctx, tx, issue.WorkspaceID); err != nil {
			return db.AgentTaskQueue{}, err
		}
		lockedIteration, err = iteration.LockIssueIteration(ctx, tx, issue.WorkspaceID, issue.ID)
		if err != nil {
			return db.AgentTaskQueue{}, err
		}
		issue, err = q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if err != nil {
			return db.AgentTaskQueue{}, err
		}
	}
	// Quick-create completion can attach an issue to an initially unassociated
	// task. Refuse changed ownership rather than start outside that issue's fence.
	lockedTask, err := q.LockAgentTaskForStart(ctx, current.ID)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	if lockedTask.IssueID != current.IssueID {
		return db.AgentTaskQueue{}, errors.New("task issue changed before start; retry task start")
	}
	if err = s.checkIssueExecution(ctx, q, lockedTask.IssueID, lockedTask.TriggerCommentID, lockedTask.CoalescedCommentIds); err != nil {
		return db.AgentTaskQueue{}, err
	}
	record, err := iteration.PrepareIssueRecord(ctx, tx, issue, lockedIteration)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	task, err := q.StartAgentTask(ctx, current.ID)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	if err = iteration.RecordExecutionStart(ctx, tx, record, operationID); err != nil {
		return db.AgentTaskQueue{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("commit task start: %w", err)
	}
	return task, nil
}
