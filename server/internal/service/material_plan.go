package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type materialExecutionKey struct{}
type materialExecutionPermit struct{ issueID, taskID pgtype.UUID }

// Claims already hold task/agent locks. Refuse a busy project instead of waiting
// on a configuration writer that may be cancelling the same task. Finalization
// repeats this fence before disclosing the payload or minting a task token.
func checkMaterialClaim(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) error {
	if !task.IssueID.Valid {
		return nil
	}
	plan, err := q.GetMaterialPlanForIssue(ctx, task.IssueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = q.LockMaterialProjectForClaim(ctx, db.LockMaterialProjectForClaimParams{ID: plan.ProjectID, WorkspaceID: plan.WorkspaceID}); err != nil {
		return err
	}
	return checkMaterialExecution(context.WithValue(ctx, materialExecutionKey{}, materialExecutionPermit{issueID: task.IssueID, taskID: task.ID}), q, task.IssueID)
}

// A permanently refused unstarted plan must not remain the agent's queue head.
// Keep its terminal row for explicit human recovery; transient lock/DB failures
// still abort the transaction and can be retried without losing the attempt.
func settleMaterialClaim(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) (bool, error) {
	err := checkMaterialClaim(ctx, q, task)
	if err == nil {
		return true, nil
	}
	var denied MaterialError
	if !errors.As(err, &denied) {
		return false, err
	}
	if task.Status != "dispatched" || task.StartedAt.Valid {
		return false, err
	}
	_, err = q.CancelAgentTaskWithReason(ctx, db.CancelAgentTaskWithReasonParams{ID: task.ID, Error: materialText(denied.Code), FailureReason: materialText(denied.Code)})
	return false, err
}

// checkMaterialExecution prevents every ordinary enqueue/rerun/mention from
// starting plan work. Only the durable plan job may enqueue, and only its exact
// recorded task may cross queued-to-started. This remains true after approval:
// a plan issue is never an implicit code-execution authorization.
func checkMaterialExecution(ctx context.Context, q *db.Queries, issueID pgtype.UUID) error {
	if !issueID.Valid {
		return nil
	}
	_, err := q.GetMaterialPlanForIssue(ctx, issueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	binding, err := q.MaterialExecutionBinding(ctx, issueID)
	if err != nil {
		return err
	}
	permit, ok := ctx.Value(materialExecutionKey{}).(materialExecutionPermit)
	if !ok || permit.issueID != issueID || !binding.Requested {
		return materialError(409, "material_plan_waiting_manual")
	}
	if binding.Status != "queued" && binding.Status != "dispatching" && binding.Status != "running" {
		return materialError(409, "material_plan_not_executable")
	}
	if permit.taskID.Valid {
		if binding.TaskID != permit.taskID {
			return materialError(409, "material_plan_task_changed")
		}
	} else if binding.TaskID.Valid {
		return materialError(409, "material_plan_already_started")
	}
	base, err := materialParse[material.Baseline](binding.Baseline)
	if err != nil || base.ID != util.UUIDToString(binding.BaselineID) || binding.CurrentRevisionID != binding.BaselineID {
		return materialError(409, "material_baseline_changed")
	}
	if err = checkMaterialRunAuthority(ctx, q, binding); err != nil {
		return err
	}
	if permit.taskID.Valid {
		task, e := q.GetAgentTask(ctx, permit.taskID)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if e != nil || !materialTaskMatches(binding, task) {
			return materialError(409, "material_plan_task_changed")
		}
		row, e := q.GetGitMaterial(ctx, db.GetGitMaterialParams{WorkspaceID: binding.WorkspaceID, ProjectID: binding.ProjectID, ID: binding.MaterialID})
		if e != nil {
			return e
		}
		revision, e := q.GetMaterialRevision(ctx, binding.BaselineID)
		if e != nil {
			return e
		}
		baseline, payload, e := materialPlanBaseline(row, revision)
		if e != nil || !task.HandoffNote.Valid || task.HandoffNote.String != materialPlanPrompt(row, baseline, payload) {
			return materialError(409, "material_baseline_changed")
		}
	}
	return nil
}

func materialTaskMatches(binding db.MaterialExecutionBindingRow, task db.AgentTaskQueue) bool {
	return binding.Requested && task.ID == binding.TaskID && task.IssueID == binding.IssueID && task.AgentID == binding.AgentID &&
		task.OriginatorUserID == binding.AuthorizedBy && task.TriggerEvidenceKind.String == "material_baseline" && task.TriggerEvidenceRefID == binding.BaselineID
}

func AuthorizeMaterialPlanTask(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) error {
	binding, err := q.MaterialExecutionBinding(ctx, task.IssueID)
	if err != nil {
		return err
	}
	if task.Status != "running" || !materialTaskMatches(binding, task) {
		return materialError(403, "material_plan_task_denied")
	}
	active := binding.CurrentRevisionID == binding.BaselineID && (binding.Status == "queued" || binding.Status == "running")
	submitted := binding.RevisionID.Valid && binding.CurrentRevisionID == binding.RevisionID && (binding.Status == "awaiting_review" || binding.Status == "approved")
	base, err := materialParse[material.Baseline](binding.Baseline)
	if err != nil || base.ID != util.UUIDToString(binding.BaselineID) || (!active && !submitted) {
		return materialError(403, "material_plan_task_denied")
	}
	return checkMaterialRunAuthority(ctx, q, binding)
}

func checkMaterialRunAuthority(ctx context.Context, q *db.Queries, binding db.MaterialExecutionBindingRow) error {
	config, err := materialParse[materialBinding](binding.Settings)
	if err != nil || !config.Enabled {
		return materialError(409, "material_disabled")
	}
	if err = materialCurrentMember(ctx, q, binding.WorkspaceID, binding.AuthorizedBy, binding.MemberID); err != nil {
		return err
	}
	if _, err = currentMaterialManager(ctx, q, binding.WorkspaceID, binding.ConfigAuthorizedBy, binding.ConfigMemberID); err != nil {
		return err
	}
	if config.PlanAgentID == nil || *config.PlanAgentID != util.UUIDToString(binding.AgentID) {
		return materialError(409, "material_plan_context_changed")
	}
	revision, err := q.GetMaterialRevision(ctx, binding.BaselineID)
	if err != nil {
		return err
	}
	if err = checkMaterialBaselineTarget(revision, config); err != nil {
		return err
	}
	if _, err = checkMaterialBinding(ctx, q, binding.WorkspaceID, binding.ProjectID, config, false); err != nil {
		return err
	}
	issue, err := q.GetIssue(ctx, binding.IssueID)
	if err != nil {
		return err
	}
	if issue.ProjectID != binding.ProjectID || issue.WorkspaceID != binding.WorkspaceID {
		return materialError(409, "material_plan_project_changed")
	}
	agent, err := q.LockAgentForAutopilotDispatch(ctx, db.LockAgentForAutopilotDispatchParams{ID: binding.AgentID, WorkspaceID: binding.WorkspaceID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	allowed, invokeErr := materialInvoke(ctx, q, agent, binding.AuthorizedBy)
	if invokeErr != nil {
		return invokeErr
	}
	if err != nil || agent.WorkspaceID != binding.WorkspaceID || !allowed {
		return materialError(403, "material_plan_invocation_denied")
	}
	return nil
}

func checkMaterialBaselineTarget(revision db.GitMaterialRevision, current materialBinding) error {
	frozen, err := materialParse[materialBinding](revision.Settings)
	if err != nil || frozen.Provider != current.Provider || frozen.Repository != current.Repository || frozen.ResourceID != current.ResourceID ||
		frozen.ResourceDigest != current.ResourceDigest || frozen.BaseBranch != current.BaseBranch || frozen.RootPath != current.RootPath ||
		frozen.ConnectionID == nil || current.ConnectionID == nil || *frozen.ConnectionID != *current.ConnectionID || frozen.ConnectionRevision != current.ConnectionRevision {
		return materialError(409, "material_configuration_changed")
	}
	return nil
}

func (s *MaterialService) processMaterialPlan(ctx context.Context, job db.GitMaterialJob) error {
	if s.Issues == nil || s.Tasks == nil {
		return materialError(503, "material_plan_service_unavailable")
	}
	// Expensive preparation happens before the writer transaction. The shared
	// enqueue helper rechecks the exact agent/runtime and prepared overlay.
	var prepared PreparedIssueTaskEnqueue
	var attr attribution.Result
	var preparedOK bool
	config, err := s.Queries.GetMaterialSettings(ctx, db.GetMaterialSettingsParams{ProjectID: job.ProjectID, WorkspaceID: job.WorkspaceID})
	if err != nil {
		return err
	}
	b, err := materialParse[materialBinding](config.Settings)
	if err != nil {
		return err
	}
	principal := config.AuthorizedBy
	agentID := pgtype.UUID{}
	if b.PlanAgentID != nil {
		agentID, err = materialUUID(*b.PlanAgentID)
		if err != nil {
			return err
		}
	}
	if plan, e := s.Queries.GetMaterialPlan(ctx, job.MaterialID); e == nil {
		principal = plan.AuthorizedBy
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	if agentID.Valid {
		attr = attribution.DirectHumanRun(principal, attribution.EvidenceKind("material_baseline"), job.RevisionID)
		prepared, err = s.Tasks.PrepareIssueTaskEnqueue(ctx, agentID, attr)
		preparedOK = err == nil
	}
	policy := ResolveIssueCountPolicy(ctx, s.Issues.Entitlements, job.WorkspaceID)
	t, row, revision, binding, err := s.workerTx(ctx, job)
	if err != nil {
		return err
	}
	defer t.tx.Rollback(ctx)
	finish := func(state, code string) error {
		if code != "" {
			if _, e := updateMaterial(ctx, t.q, row, "blocked", code, row.CurrentRevisionID, row.Baseline); e != nil {
				return e
			}
		}
		if e := t.q.FinishMaterialJob(ctx, db.FinishMaterialJobParams{ID: job.ID, LeaseToken: job.LeaseToken, State: state, Blocker: materialText(code)}); e != nil {
			return e
		}
		return t.tx.Commit(ctx)
	}
	if row.CurrentRevisionID != revision.ID {
		return finish("done", "")
	}
	base, rev, err := materialPlanBaseline(row, revision)
	if err != nil {
		return finish("blocked", "material_baseline_changed")
	}
	if !binding.Enabled {
		return finish("blocked", "material_disabled")
	}
	if err = checkMaterialBaselineTarget(revision, binding); err != nil {
		return finish("blocked", materialCode(err))
	}
	if _, err = checkMaterialBinding(ctx, t.q, row.WorkspaceID, row.ProjectID, binding, false); err != nil {
		return finish("blocked", materialCode(err))
	}
	p, err := t.q.GetMaterialPlan(ctx, row.ID)
	var created *db.Issue
	if errors.Is(err, pgx.ErrNoRows) {
		// The immutable material/baseline identity is the deduplication key. A new
		// baseline must retain its own plan even while an older plan has this title.
		result, e := s.Issues.CreateInTx(ctx, t.tx, IssueCreateParams{WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, Title: "Plan: " + row.Title, Description: materialText(materialPlanPrompt(row, base, rev)), Status: "todo", Priority: "none", CreatorType: "member", CreatorID: t.actor, AllowDuplicate: true}, policy)
		if e != nil {
			return e
		}
		created = &result.Issue
		status := "waiting_manual"
		frozen, e := materialParse[materialBinding](revision.Settings)
		if e != nil {
			return e
		}
		auto := frozen.AutoStartPlan && binding.AutoStartPlan
		if auto {
			status = "blocked"
		}
		err = t.q.CreateMaterialPlan(ctx, db.CreateMaterialPlanParams{MaterialID: row.ID, WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, BaselineID: revision.ID, IssueID: result.Issue.ID, Status: status, Requested: auto, AuthorizedBy: t.actor, MemberID: t.member.ID, AgentID: agentID})
		if err != nil {
			return err
		}
		p, err = t.q.GetMaterialPlan(ctx, row.ID)
	}
	if err != nil {
		return err
	}
	if !p.Requested {
		err = finish("done", "")
		if err == nil && created != nil {
			s.Issues.publishStageMaterialized(ctx, *created)
		}
		return err
	}
	if p.TaskID.Valid {
		return finish("done", "")
	}
	block := func(code string) error {
		p.Status = "blocked"
		p.Blocker = materialText(code)
		if e := saveMaterialPlan(ctx, t.q, p); e != nil {
			return e
		}
		e := finish("blocked", code)
		if e == nil && created != nil {
			s.Issues.publishStageMaterialized(ctx, *created)
		}
		return e
	}
	if !preparedOK || !p.AgentID.Valid {
		return block("material_plan_agent_unavailable")
	}
	if binding.PlanAgentID == nil || *binding.PlanAgentID != util.UUIDToString(agentID) || p.AgentID != agentID || p.AuthorizedBy != principal {
		return block("material_plan_context_changed")
	}
	if err = materialCurrentMember(ctx, t.q, row.WorkspaceID, p.AuthorizedBy, p.MemberID); err != nil {
		return block(materialCode(err))
	}
	agent, err := t.q.GetAgent(ctx, p.AgentID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	allowed, invokeErr := materialInvoke(ctx, t.q, agent, p.AuthorizedBy)
	if invokeErr != nil {
		return invokeErr
	}
	if err != nil || agent.WorkspaceID != row.WorkspaceID || !allowed {
		return block("material_plan_invocation_denied")
	}
	if s.ResolveProvider == nil {
		return block("material_provider_unavailable")
	}
	provider, cleanup, err := s.ResolveProvider(ctx, t.q, row.WorkspaceID, binding.Settings)
	if err != nil {
		return block("material_connection_unavailable")
	}
	if cleanup != nil {
		defer cleanup()
	}
	if err = materialVerifyFiles(ctx, provider, row, rev, base.Commit); err != nil {
		return block("material_baseline_unreadable")
	}
	issue, err := t.q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: p.IssueID, WorkspaceID: row.WorkspaceID})
	if err != nil {
		return err
	}
	if issue.ProjectID != row.ProjectID {
		return block("material_plan_project_changed")
	}
	p.Status = "dispatching"
	p.Blocker = pgtype.Text{}
	if err = saveMaterialPlan(ctx, t.q, p); err != nil {
		return err
	}
	dispatchCtx := context.WithValue(ctx, materialExecutionKey{}, materialExecutionPermit{issueID: issue.ID})
	task, reused, err := s.Tasks.EnqueuePreparedIssueTaskInTx(dispatchCtx, t.tx, issue, p.AgentID, pgtype.UUID{}, materialPlanPrompt(row, base, rev), attr, prepared)
	if errors.Is(err, ErrIssueTaskUnavailable) {
		return block("material_plan_agent_unavailable")
	}
	if errors.Is(err, ErrIssueTaskContextChanged) {
		return block("material_plan_context_changed")
	}
	if err != nil {
		return err
	}
	p.Status = "queued"
	p.TaskID = task.ID
	if err = saveMaterialPlan(ctx, t.q, p); err != nil {
		return err
	}
	if _, err = updateMaterial(ctx, t.q, row, "planning", "", row.CurrentRevisionID, row.Baseline); err != nil {
		return err
	}
	if err = finish("done", ""); err != nil {
		return err
	}
	if created != nil {
		s.Issues.publishStageMaterialized(ctx, *created)
	}
	if !reused {
		s.Tasks.FinalizeIssueTaskEnqueue(ctx, task)
	}
	return nil
}
