package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// startTaskAtomically owns the former autocommit start without adding retries.
// Even an issue with no iteration must take the workspace fence before its row
// lock, so a join committed while waiting is included in the recorded facts.
func (s *TaskService) startTaskAtomically(ctx context.Context, current db.AgentTaskQueue, knowledgeFences ...*protocol.TaskKnowledgeFence) (db.AgentTaskQueue, error) {
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
	var stageRootFence pgtype.UUID
	if current.IssueID.Valid {
		// This read only resolves the workspace; facts are read after the fence.
		issue, err = q.GetIssue(ctx, current.IssueID)
		if err != nil {
			return db.AgentTaskQueue{}, err
		}
		if _, err = q.LockWorkspaceForChatSessionCreate(ctx, issue.WorkspaceID); err != nil {
			return db.AgentTaskQueue{}, err
		}
		if plan, e := q.GetMaterialPlanForIssue(ctx, issue.ID); e == nil {
			if _, err = q.LockProjectForExecutionSquad(ctx, db.LockProjectForExecutionSquadParams{ID: plan.ProjectID, WorkspaceID: plan.WorkspaceID}); err != nil {
				return db.AgentTaskQueue{}, err
			}
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return db.AgentTaskQueue{}, e
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
		// Root before child, the same order as proposal approval: a stage-gated
		// child fences its root lifecycle before taking its own row lock.
		if stageRootFence, err = lockStageChildRoot(ctx, q, issue.ID); err != nil {
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
	if err = validateTaskKnowledgeFences(lockedTask, knowledgeFences); err != nil {
		return db.AgentTaskQueue{}, err
	}
	if lockedTask.IssueID != current.IssueID {
		return db.AgentTaskQueue{}, errors.New("task issue changed before start; retry task start")
	}
	materialCtx := context.WithValue(ctx, materialExecutionKey{}, materialExecutionPermit{issueID: lockedTask.IssueID, taskID: lockedTask.ID})
	if err = s.checkIssueExecution(materialCtx, q, lockedTask.IssueID, lockedTask.TriggerCommentID, lockedTask.CoalescedCommentIds); err != nil {
		return db.AgentTaskQueue{}, err
	}
	if lockedTask.TriggerEvidenceKind.String == "material_operation" || lockedTask.TriggerEvidenceKind.String == "material_baseline" {
		op, d, e := AuthorizeMaterialOperation(ctx, q, lockedTask)
		if e != nil {
			return db.AgentTaskQueue{}, e
		}
		if op.ClaimGeneration != lockedTask.ClaimGeneration || op.ChallengeHash == "" {
			return db.AgentTaskQueue{}, materialError(409, "material_claim_required")
		}
		if d.Action == "verify_baseline" && op.State != "succeeded" {
			return db.AgentTaskQueue{}, materialError(409, "material_baseline_unverified")
		}
		if d.TargetKind != "repository_initialization" && d.RepositoryExecution != nil && len(d.RepositoryExecution.ReadyProof) > 0 && d.Action != "verify_baseline" && op.State != "prepared" {
			return db.AgentTaskQueue{}, materialError(409, "material_repository_unverified")
		}
		if d.TargetKind != "repository_initialization" && d.RepositoryExecution != nil {
			receipt, err := materialParse[protocol.MaterialObservation](op.Receipt)
			if err != nil || time.Since(receipt.ObservedAt) > time.Minute {
				return db.AgentTaskQueue{}, materialError(409, "material_repository_unverified")
			}
		}
	}
	// The real queued-to-started boundary: decide under the root lifecycle fence.
	if err = checkStageChildExecutionLocked(ctx, q, lockedTask.IssueID, stageRootFence); err != nil {
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
