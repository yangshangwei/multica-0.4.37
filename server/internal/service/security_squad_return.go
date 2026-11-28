package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Security reviewer completion is a platform lifecycle return, never permission
// for an inert reviewer comment to dispatch. Immutable template keys establish
// the workflow; versions describe creation defaults, not current saved prompts.
func (s *TaskService) securityReviewerReturnAllowed(ctx context.Context, q *db.Queries, worker, source db.AgentTaskQueue, issue db.Issue, leader db.Agent) (bool, error) {
	if !source.IsLeaderTask || !source.SquadID.Valid || source.IssueID != worker.IssueID ||
		source.Status == "cancelled" ||
		!worker.TriggerCommentID.Valid || !worker.OriginatorUserID.Valid || worker.OriginatorUserID != source.OriginatorUserID ||
		!issue.AssigneeType.Valid || issue.AssigneeType.String != "squad" || issue.AssigneeID != source.SquadID ||
		leader.TemplateKey != "security-remediation-lead" || leader.RuntimeID != source.RuntimeID {
		return false, nil
	}
	squad, err := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: source.SquadID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return false, err
	}
	if squad.ArchivedAt.Valid || squad.TemplateKey != "security-remediation" || squad.LeaderID != source.AgentID {
		return false, nil
	}
	member, err := q.IsSquadMember(ctx, db.IsSquadMemberParams{SquadID: squad.ID, MemberType: "agent", MemberID: worker.AgentID})
	if err != nil || !member {
		return false, err
	}
	delegation, err := q.GetComment(ctx, worker.TriggerCommentID)
	if err != nil {
		return false, err
	}
	if delegation.IssueID != issue.ID || delegation.WorkspaceID != issue.WorkspaceID || delegation.AuthorType != "agent" ||
		delegation.AuthorID != source.AgentID || delegation.SourceTaskID != source.ID || !delegation.DispatchEligible {
		return false, nil
	}
	reviewer, err := q.GetAgent(ctx, worker.AgentID)
	if err != nil {
		return false, err
	}
	if auth.TaskCapabilityProfile(reviewer.TemplateKey, reviewer.TemplateVersion) != auth.TaskCapabilityReviewer ||
		reviewer.WorkspaceID != issue.WorkspaceID || reviewer.ArchivedAt.Valid || !worker.RuntimeID.Valid || reviewer.RuntimeID != worker.RuntimeID {
		return false, nil
	}
	for _, agent := range []db.Agent{leader, reviewer} {
		runtime, err := s.runtimeLookup(q).Get(ctx, agent.RuntimeID)
		if err != nil {
			return false, err
		}
		if runtime.WorkspaceID != issue.WorkspaceID {
			return false, nil
		}
		for _, principal := range []pgtype.UUID{worker.OriginatorUserID, runtime.OwnerID} {
			if !principal.Valid {
				return false, nil
			}
			if _, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: principal, WorkspaceID: issue.WorkspaceID}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return false, nil
				}
				return false, err
			}
			user, err := q.GetUser(ctx, principal)
			if err != nil {
				return false, err
			}
			if user.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(principal), user.Email.String) {
				return false, nil
			}
		}
		targets, err := q.ListAgentInvocationTargets(ctx, agent.ID)
		if err != nil {
			return false, err
		}
		if !auth.AgentInvocationAllowed(agent, targets, util.UUIDToString(worker.OriginatorUserID), true, false) {
			return false, nil
		}
	}
	return true, nil
}

// Recheck the platform return at credential delivery and execution start. A
// queued return cannot retain authority after assignment, membership or runtime
// changes. Lock only genuine security-return sources; ordinary inert comments
// still follow their existing unconditional non-dispatch contract.
func (s *TaskService) validateSecurityReviewReturnTask(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) error {
	if task.TriggerEvidenceKind.String == "delegated_result" && !task.TriggerCommentID.Valid {
		return auth.ErrTaskCapability
	}
	ids := append(append([]pgtype.UUID{}, task.CoalescedCommentIds...), task.TriggerCommentID)
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		comment, err := q.GetComment(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) && !(task.TriggerEvidenceKind.String == "delegated_result" && id == task.TriggerCommentID) {
			// Ordinary planned inputs can be deleted before claim. The builder
			// records surviving inputs; do not turn this narrow return guard into
			// a new completeness requirement for every ordinary task.
			continue
		}
		if err != nil {
			return err
		}
		if !IsDelegatedFailureRecoveryComment(comment) {
			continue
		}
		worker, err := q.GetAgentTask(ctx, comment.SourceTaskID)
		if err != nil {
			return err
		}
		restricted, err := RestrictedAgentResult(ctx, q, util.UUIDToString(worker.AgentID))
		if err != nil {
			return err
		}
		if !restricted {
			continue
		}
		for _, principalTask := range []pgtype.UUID{worker.ID, worker.DelegatedFromTaskID} {
			if !principalTask.Valid {
				return auth.ErrTaskCapability
			}
			if _, err := q.LockTaskCapabilityPrincipals(ctx, principalTask); err != nil {
				return err
			}
		}
		source, err := q.GetAgentTask(ctx, worker.DelegatedFromTaskID)
		if err != nil {
			return err
		}
		if _, err := q.LockLifecycleSquad(ctx, db.LockLifecycleSquadParams{ID: source.SquadID, WorkspaceID: comment.WorkspaceID}); err != nil {
			return err
		}
		for _, agent := range []pgtype.UUID{source.AgentID, worker.AgentID} {
			if _, err := q.LockTaskCapabilityInvocationTargets(ctx, agent); err != nil {
				return err
			}
		}
		target, err := s.loadDelegatedFailureRecoveryTarget(ctx, q, worker)
		if err != nil {
			return err
		}
		if target == nil || !target.securityReturn || !task.IsLeaderTask || task.AgentID != source.AgentID || task.SquadID != source.SquadID ||
			task.OriginatorUserID != worker.OriginatorUserID || task.IssueID != comment.IssueID || task.RuntimeID != target.agent.RuntimeID {
			return auth.ErrTaskCapability
		}
	}
	return nil
}

func (s *TaskService) securityReviewerReturnContent(ctx context.Context, q *db.Queries, worker, source db.AgentTaskQueue) (string, error) {
	if worker.Status == "failed" {
		return fmt.Sprintf("Delegated reviewer task `%s` failed without an automatic retry pending. Required review evidence is missing: use the workflow's bounded redispatch or report the blocker for human handling. Never skip a required review or accept the issue from this failure. Source coordinator task: `%s`.", util.UUIDToString(worker.ID), util.UUIDToString(source.ID)), nil
	}
	hasResult, err := q.HasSecurityReviewerResult(ctx, db.HasSecurityReviewerResultParams{IssueID: worker.IssueID, AuthorID: worker.AgentID, SourceTaskID: worker.ID})
	if err != nil {
		return "", err
	}
	status := "Review evidence is available; read the reviewer's actual result and verify its scope and commit before deciding the next step."
	if !hasResult {
		status = "There is no review result (progress and failure notices are not results). Treat the evidence as missing; use the workflow's bounded retry or report the blocker."
	}
	return fmt.Sprintf("Delegated reviewer task `%s` completed. %s This platform return is not approval or issue acceptance. Source coordinator task: `%s`.", util.UUIDToString(worker.ID), status, util.UUIDToString(source.ID)), nil
}

func (s *TaskService) returnSecurityReviewerCompletion(ctx context.Context, task db.AgentTaskQueue) error {
	if task.Status != "completed" || !task.DelegatedFromTaskID.Valid {
		return nil
	}
	agent, err := s.Queries.GetAgent(ctx, task.AgentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion) != auth.TaskCapabilityReviewer {
		return nil
	}
	_, err = s.recoverDelegatedTaskFailure(ctx, task)
	return err
}

// A refused lifecycle return is permanently inert. A later human action must
// create a new authorized delegation, rather than reviving the old obligation.
func (s *TaskService) withdrawSecurityReviewReturn(ctx context.Context, comment db.Comment, worker db.AgentTaskQueue) error {
	restricted, err := RestrictedAgentResult(ctx, s.Queries, util.UUIDToString(worker.AgentID))
	if err != nil || !restricted {
		return err
	}
	return s.Queries.WithdrawSecurityReviewReturn(ctx, db.WithdrawSecurityReviewReturnParams{CommentID: comment.ID, SourceTaskID: worker.ID})
}
