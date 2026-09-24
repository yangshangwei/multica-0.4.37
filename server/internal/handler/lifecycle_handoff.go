package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// lifecycleHandoffRequest is intentionally one API shape for the five
// evidence handoffs. The discriminator keeps the write path auditable while
// allowing clients to submit exactly one kind of evidence per request.
type lifecycleHandoffRequest struct {
	Kind                string   `json:"kind"`
	Route               string   `json:"route,omitempty"`
	CauseState          string   `json:"cause_state,omitempty"`
	Reason              string   `json:"reason,omitempty"`
	MitigationComplete  bool     `json:"mitigation_complete,omitempty"`
	SeparateFollowUp    bool     `json:"separate_follow_up,omitempty"`
	FollowUpIssueID     string   `json:"follow_up_issue_id,omitempty"`
	FollowUpTitle       string   `json:"follow_up_title,omitempty"`
	FollowUpDescription string   `json:"follow_up_description,omitempty"`
	FollowUpPriority    string   `json:"follow_up_priority,omitempty"`
	AssigneeType        string   `json:"assignee_type,omitempty"`
	AssigneeID          string   `json:"assignee_id,omitempty"`
	HandoffNote         string   `json:"handoff_note,omitempty"`
	DiagnosisRef        string   `json:"diagnosis_ref,omitempty"`
	RegressionTest      string   `json:"regression_test,omitempty"`
	Conclusion          string   `json:"conclusion,omitempty"`
	Evidence            []string `json:"evidence,omitempty"`

	Facts                   []string                         `json:"facts,omitempty"`
	Inferences              []string                         `json:"inferences,omitempty"`
	Unknowns                []string                         `json:"unknowns,omitempty"`
	ExistingPreventionTasks []string                         `json:"existing_prevention_tasks,omitempty"`
	Prevention              *lifecyclePreventionTaskRequest  `json:"prevention,omitempty"`
	Rollout                 *lifecycleRolloutEvidenceRequest `json:"rollout,omitempty"`
	AgentEvaluation         *lifecycleAgentEvaluationRequest `json:"agent_evaluation,omitempty"`
	Governance              *lifecycleGovernanceRequest      `json:"governance,omitempty"`
}

type lifecyclePreventionTaskRequest struct {
	Issue            string `json:"issue,omitempty"`
	Title            string `json:"title"`
	Owner            string `json:"owner"`
	Priority         string `json:"priority"`
	AcceptanceSignal string `json:"acceptance_signal"`
	RelatedIncident  string `json:"related_incident,omitempty"`
}

type lifecycleRolloutEvidenceRequest struct {
	ApprovedDigest    string                            `json:"approved_digest"`
	ArtifactDigest    string                            `json:"artifact_digest"`
	Baseline          map[string]float64                `json:"baseline"`
	ObservationWindow lifecycleObservationWindowRequest `json:"observation_window"`
	WindowComplete    bool                              `json:"window_complete"`
	Signals           []lifecycleRolloutSignalRequest   `json:"signals"`
	RollbackApproved  bool                              `json:"rollback_approved"`
	RollbackExecuted  bool                              `json:"rollback_executed"`
}

type lifecycleAgentEvaluationRequest struct {
	ArtifactDigest   string                                `json:"artifact_digest"`
	BaselineVersion  string                                `json:"baseline_version"`
	CandidateVersion string                                `json:"candidate_version"`
	SkillVersion     string                                `json:"skill_version"`
	MCPVersion       string                                `json:"mcp_version"`
	Cases            []lifecycleAgentEvaluationCaseRequest `json:"cases"`
}

type lifecycleObservationWindowRequest struct {
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at"`
	Complete  bool   `json:"complete"`
}

type lifecycleRolloutSignalRequest struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}

type lifecycleAgentEvaluationCaseRequest struct {
	Category   string   `json:"category"`
	Trace      []string `json:"trace"`
	StopReason string   `json:"stop_reason"`
	Result     string   `json:"result"`
}

type lifecycleGovernanceRequest struct {
	Capability string                             `json:"capability"`
	Signals    []lifecycleGovernanceSignalRequest `json:"signals"`
}

type lifecycleGovernanceSignalRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type lifecycleHandoffResponse struct {
	Kind            string         `json:"kind"`
	Decision        string         `json:"decision"`
	SourceIssueID   string         `json:"source_issue_id"`
	FollowUpIssueID string         `json:"follow_up_issue_id,omitempty"`
	FollowUpCreated bool           `json:"follow_up_created,omitempty"`
	FollowUpReused  bool           `json:"follow_up_reused,omitempty"`
	QueuedTaskID    string         `json:"queued_task_id,omitempty"`
	AuditCommentID  string         `json:"audit_comment_id,omitempty"`
	Metadata        map[string]any `json:"metadata"`
}

// CreateLifecycleHandoff records a lifecycle decision and creates or reuses
// the next issue in the loop. It deliberately delegates issue/task semantics
// to the existing services so permissions, workspace checks and deduplication
// remain centralized.
func (h *Handler) CreateLifecycleHandoff(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	if !h.requireAgentAutonomy(w, r, uuidToString(issue.WorkspaceID), service.AutonomyContributor, "create lifecycle follow-up work") {
		return
	}

	var req lifecycleHandoffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.FollowUpTitle = util.SanitizeTextForPostgres(req.FollowUpTitle)
	req.Kind = strings.TrimSpace(req.Kind)
	if req.Kind == "" {
		req.Kind = "rca"
	}

	actorType, actorID := h.resolveActor(r, userID, uuidToString(issue.WorkspaceID))
	decision := service.LifecycleDecisionUnknown
	needsFollowUp := false
	var evidence map[string]any

	switch req.Kind {
	case "rca":
		downstream := strings.TrimSpace(req.FollowUpIssueID)
		if downstream == "" {
			downstream = strings.TrimSpace(req.FollowUpTitle)
		}
		var err error
		decision, err = service.ValidateRCARoute(service.RCARouteInput{
			Route: req.Route, CauseState: req.CauseState, Reason: req.Reason,
			MitigationComplete: req.MitigationComplete, SeparateFollowUp: req.SeparateFollowUp,
			DownstreamRepairIssue: downstream,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := service.ValidateRCAArtifact(service.RCAArtifact{
			DiagnosisRef: req.DiagnosisRef, RegressionTest: req.RegressionTest,
			Conclusion: req.Conclusion, Evidence: req.Evidence, Unknowns: req.Unknowns,
		}); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		needsFollowUp = true
		evidence = map[string]any{
			"route": req.Route, "cause_state": req.CauseState, "reason": req.Reason,
			"mitigation_complete": req.MitigationComplete, "separate_follow_up": req.SeparateFollowUp,
			"diagnosis_ref": req.DiagnosisRef, "regression_test": req.RegressionTest,
			"conclusion": req.Conclusion, "evidence": req.Evidence, "unknowns": req.Unknowns,
		}

	case "incident-learning":
		if req.Prevention == nil && len(req.ExistingPreventionTasks) == 0 {
			writeError(w, http.StatusBadRequest, "incident-learning requires prevention task evidence")
			return
		}
		prevention := req.Prevention
		if err := h.validateExistingLifecyclePreventionTasks(r, issue, req.ExistingPreventionTasks); err != nil {
			h.writeLifecycleError(w, err)
			return
		}

		validation := service.IncidentLearningEvidence{
			SourceIssue: uuidToString(issue.ID), Facts: req.Facts, Inferences: req.Inferences,
			Unknowns: req.Unknowns, ExistingPreventionTasks: req.ExistingPreventionTasks,
		}
		if prevention != nil {
			if strings.TrimSpace(prevention.Issue) == "" {
				prevention.Issue = prevention.Title
			}
			related := prevention.RelatedIncident
			if related == "" {
				related = uuidToString(issue.ID)
			}
			validation.PreventionTasks = []service.PreventionTaskEvidence{{
				Issue: prevention.Issue, Title: prevention.Title, Owner: prevention.Owner,
				Priority: prevention.Priority, AcceptanceSignal: prevention.AcceptanceSignal,
				RelatedIncident: related,
			}}
		}
		// The source issue identifier is not needed for the contract's linkage
		// check beyond being stable and non-empty; UUID is always available.
		validation.SourceIssue = uuidToString(issue.ID)
		if err := service.ValidateIncidentLearning(validation); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		needsFollowUp = prevention != nil
		decision = service.LifecycleDecisionContinue
		evidence = map[string]any{
			"facts": req.Facts, "inferences": req.Inferences, "unknowns": req.Unknowns,
			"existing_prevention_tasks": req.ExistingPreventionTasks,
			"prevention": map[string]any{
				"owner": lifecyclePreventionOwner(prevention), "priority": lifecyclePreventionPriority(prevention),
				"acceptance_signal": lifecyclePreventionAcceptanceSignal(prevention),
			},
		}
	case "rollout":
		if req.Rollout == nil {
			writeError(w, http.StatusBadRequest, "rollout evidence is required")
			return
		}
		rollout := service.RolloutEvidence{
			ApprovedDigest: req.Rollout.ApprovedDigest, ArtifactDigest: req.Rollout.ArtifactDigest,
			Baseline: req.Rollout.Baseline, WindowComplete: req.Rollout.ObservationWindow.Complete,
			ObservationWindowStart: req.Rollout.ObservationWindow.StartedAt,
			ObservationWindowEnd:   req.Rollout.ObservationWindow.EndedAt,
			Signals:                lifecycleRolloutSignals(req.Rollout.Signals), RollbackApproved: req.Rollout.RollbackApproved,
			RollbackExecuted: req.Rollout.RollbackExecuted,
		}
		decision = service.EvaluateRolloutEvidence(rollout)
		evidence = map[string]any{"approved_digest": rollout.ApprovedDigest, "artifact_digest": rollout.ArtifactDigest,
			"baseline": rollout.Baseline, "observation_window": req.Rollout.ObservationWindow,
			"window_complete": rollout.WindowComplete, "signals": rollout.Signals,
			"rollback_approved": rollout.RollbackApproved, "rollback_executed": rollout.RollbackExecuted}
	case "agent-evaluation":
		if req.AgentEvaluation == nil {
			writeError(w, http.StatusBadRequest, "agent evaluation evidence is required")
			return
		}
		eval := service.AgentEvaluationEvidence{
			ArtifactDigest:  req.AgentEvaluation.ArtifactDigest,
			BaselineVersion: req.AgentEvaluation.BaselineVersion, CandidateVersion: req.AgentEvaluation.CandidateVersion,
			SkillVersion: req.AgentEvaluation.SkillVersion, MCPVersion: req.AgentEvaluation.MCPVersion,
			Cases: lifecycleAgentEvaluationCases(req.AgentEvaluation.Cases),
		}
		decision = service.ValidateAgentEvaluation(eval)
		evidence = map[string]any{"artifact_digest": eval.ArtifactDigest, "baseline_version": eval.BaselineVersion, "candidate_version": eval.CandidateVersion,
			"skill_version": eval.SkillVersion, "mcp_version": eval.MCPVersion, "cases": eval.Cases}
		if strings.TrimSpace(req.FollowUpIssueID) != "" || strings.TrimSpace(req.FollowUpTitle) != "" {
			needsFollowUp = true
		}
	case "governance":
		if req.Governance == nil {
			writeError(w, http.StatusBadRequest, "governance evidence is required")
			return
		}
		governance := service.GovernanceEvidence{
			Capability: req.Governance.Capability,
			Signals:    lifecycleGovernanceSignals(req.Governance.Signals),
		}
		decision = service.ValidateGovernanceEvidence(governance)
		evidence = map[string]any{
			"capability": governance.Capability,
			"signals":    governance.Signals,
		}
		if strings.TrimSpace(req.FollowUpIssueID) != "" || strings.TrimSpace(req.FollowUpTitle) != "" {
			needsFollowUp = true
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported lifecycle handoff kind")
		return
	}

	result, err := h.persistLifecycleHandoff(r, issue, req, actorType, actorID, decision, evidence, needsFollowUp)
	if err != nil {
		h.writeLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// authorizeLifecycleFollowUp checks the assignee on the issue that will be
// reused. A request's proposed assignee is only relevant for a newly-created
// issue; duplicate and explicit-ID paths must authorize the actual persisted
// target before any metadata, audit, or queue side effects occur.
func (h *Handler) authorizeLifecycleFollowUp(r *http.Request, followUp db.Issue) error {
	if status, message := h.validateAssigneePair(r.Context(), r, uuidToString(followUp.WorkspaceID), followUp.AssigneeType, followUp.AssigneeID); status != 0 {
		return lifecycleAssigneeError{status: status, message: message}
	}
	return nil
}

type lifecycleAssigneeError struct {
	status  int
	message string
}

func (e lifecycleAssigneeError) Error() string { return e.message }

func lifecycleAssignee(req lifecycleHandoffRequest) (pgtype.Text, pgtype.UUID, error) {
	if req.AssigneeType == "" && req.AssigneeID == "" {
		return pgtype.Text{}, pgtype.UUID{}, nil
	}
	if req.AssigneeType == "" || req.AssigneeID == "" {
		return pgtype.Text{}, pgtype.UUID{}, errors.New("assignee_type and assignee_id must be provided together")
	}
	id, err := util.ParseUUID(req.AssigneeID)
	if err != nil {
		return pgtype.Text{}, pgtype.UUID{}, errors.New("invalid assignee_id")
	}
	return pgtype.Text{String: req.AssigneeType, Valid: true}, id, nil
}

func (h *Handler) resolveLifecycleIssue(r *http.Request, raw string, workspaceID, parentIssueID pgtype.UUID) (db.Issue, error) {
	workspace := uuidToString(workspaceID)
	if issue, ok := h.resolveIssueByIdentifier(r.Context(), raw, workspace); ok {
		if !sameLifecycleParent(issue, parentIssueID) {
			return db.Issue{}, errors.New("follow-up issue is not a child of the source issue")
		}
		return issue, nil
	}
	id, err := util.ParseUUID(raw)
	if err != nil {
		return db.Issue{}, errors.New("follow-up issue not found")
	}
	issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		return db.Issue{}, errors.New("follow-up issue not found in this workspace")
	}
	if !sameLifecycleParent(issue, parentIssueID) {
		return db.Issue{}, errors.New("follow-up issue is not a child of the source issue")
	}
	return issue, nil
}

func sameLifecycleParent(issue db.Issue, parentID pgtype.UUID) bool {
	return issue.ParentIssueID.Valid && parentID.Valid && uuidToString(issue.ParentIssueID) == uuidToString(parentID)
}

func (h *Handler) validateExistingLifecyclePreventionTasks(r *http.Request, source db.Issue, identifiers []string) error {
	for _, raw := range identifiers {
		if strings.TrimSpace(raw) == "" {
			return errors.New("existing_prevention_tasks contains an empty issue identifier")
		}
		prevention, err := h.resolveLifecycleIssueWithoutParent(r, raw, source.WorkspaceID)
		if err != nil {
			return fmt.Errorf("existing prevention task %q: %w", raw, err)
		}
		if uuidToString(prevention.ID) == uuidToString(source.ID) {
			return errors.New("existing prevention task cannot be the source issue")
		}
	}
	return nil
}

func (h *Handler) resolveLifecycleIssueWithoutParent(r *http.Request, raw string, workspaceID pgtype.UUID) (db.Issue, error) {
	workspace := uuidToString(workspaceID)
	if issue, ok := h.resolveIssueByIdentifier(r.Context(), raw, workspace); ok {
		return issue, nil
	}
	id, err := util.ParseUUID(raw)
	if err != nil {
		return db.Issue{}, errors.New("issue not found")
	}
	issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		return db.Issue{}, errors.New("issue not found in this workspace")
	}
	return issue, nil
}

func lifecyclePreventionOwner(task *lifecyclePreventionTaskRequest) string {
	if task == nil {
		return ""
	}
	return task.Owner
}

func lifecyclePreventionPriority(task *lifecyclePreventionTaskRequest) string {
	if task == nil {
		return ""
	}
	return task.Priority
}

func lifecyclePreventionAcceptanceSignal(task *lifecyclePreventionTaskRequest) string {
	if task == nil {
		return ""
	}
	return task.AcceptanceSignal
}

func (h *Handler) writeLifecycleError(w http.ResponseWriter, err error) {
	if errors.Is(err, errLifecycleMetadataTooLarge) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, service.ErrIssueTaskContextChanged) || errors.Is(err, service.ErrIssueTaskUnavailable) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, service.ErrAttributionFailClosed) {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	var assigneeErr lifecycleAssigneeError
	if errors.As(err, &assigneeErr) {
		writeError(w, assigneeErr.status, assigneeErr.message)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not a child") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	slog.Error("lifecycle handoff failed", "error", err)
	writeError(w, http.StatusInternalServerError, "failed to persist lifecycle handoff")
}

func lifecycleRolloutSignals(items []lifecycleRolloutSignalRequest) []service.RolloutSignalEvidence {
	result := make([]service.RolloutSignalEvidence, len(items))
	for i, item := range items {
		result[i] = service.RolloutSignalEvidence{Name: item.Name, Value: item.Value, Threshold: item.Threshold}
	}
	return result
}

func lifecycleAgentEvaluationCases(items []lifecycleAgentEvaluationCaseRequest) []service.AgentEvaluationCaseEvidence {
	result := make([]service.AgentEvaluationCaseEvidence, len(items))
	for i, item := range items {
		result[i] = service.AgentEvaluationCaseEvidence{Category: item.Category, Trace: item.Trace, StopReason: item.StopReason, Result: item.Result}
	}
	return result
}

func lifecycleGovernanceSignals(items []lifecycleGovernanceSignalRequest) []service.GovernanceSignalEvidence {
	result := make([]service.GovernanceSignalEvidence, len(items))
	for i, item := range items {
		result[i] = service.GovernanceSignalEvidence{Name: item.Name, Status: item.Status, Detail: item.Detail}
	}
	return result
}

func lifecycleHandoffHistory(raw []byte) []map[string]any {
	metadata := util.JSONObjectOrEmpty(raw)
	history := make([]map[string]any, 0, 20)
	if values, ok := metadata["lifecycle_handoff_history"].([]any); ok {
		for _, value := range values {
			if entry, ok := value.(map[string]any); ok {
				history = append(history, entry)
			}
		}
	}
	if len(history) == 0 {
		if latest, ok := metadata["lifecycle_handoff"].(map[string]any); ok {
			history = append(history, latest)
		}
	}
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	return history
}
