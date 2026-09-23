package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// lifecycleHandoffRequest is intentionally one API shape for the four
// evidence handoffs. The discriminator keeps the write path auditable while
// allowing clients to submit exactly one kind of evidence per request.
type lifecycleHandoffRequest struct {
	Kind                string `json:"kind"`
	Route               string `json:"route,omitempty"`
	CauseState          string `json:"cause_state,omitempty"`
	Reason              string `json:"reason,omitempty"`
	MitigationComplete  bool   `json:"mitigation_complete,omitempty"`
	SeparateFollowUp    bool   `json:"separate_follow_up,omitempty"`
	FollowUpIssueID     string `json:"follow_up_issue_id,omitempty"`
	FollowUpTitle       string `json:"follow_up_title,omitempty"`
	FollowUpDescription string `json:"follow_up_description,omitempty"`
	FollowUpPriority    string `json:"follow_up_priority,omitempty"`
	AssigneeType        string `json:"assignee_type,omitempty"`
	AssigneeID          string `json:"assignee_id,omitempty"`
	HandoffNote         string `json:"handoff_note,omitempty"`

	Facts                   []string                         `json:"facts,omitempty"`
	Inferences              []string                         `json:"inferences,omitempty"`
	Unknowns                []string                         `json:"unknowns,omitempty"`
	ExistingPreventionTasks []string                         `json:"existing_prevention_tasks,omitempty"`
	Prevention              *lifecyclePreventionTaskRequest  `json:"prevention,omitempty"`
	Rollout                 *lifecycleRolloutEvidenceRequest `json:"rollout,omitempty"`
	AgentEvaluation         *lifecycleAgentEvaluationRequest `json:"agent_evaluation,omitempty"`
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
	ApprovedDigest   string                          `json:"approved_digest"`
	ArtifactDigest   string                          `json:"artifact_digest"`
	Baseline         map[string]float64              `json:"baseline"`
	WindowComplete   bool                            `json:"window_complete"`
	Signals          []lifecycleRolloutSignalRequest `json:"signals"`
	RollbackApproved bool                            `json:"rollback_approved"`
	RollbackExecuted bool                            `json:"rollback_executed"`
}

type lifecycleAgentEvaluationRequest struct {
	BaselineVersion  string                                `json:"baseline_version"`
	CandidateVersion string                                `json:"candidate_version"`
	SkillVersion     string                                `json:"skill_version"`
	MCPVersion       string                                `json:"mcp_version"`
	Cases            []lifecycleAgentEvaluationCaseRequest `json:"cases"`
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
	req.Kind = strings.TrimSpace(req.Kind)
	if req.Kind == "" {
		req.Kind = "rca"
	}

	actorType, actorID := h.resolveActor(r, userID, uuidToString(issue.WorkspaceID))
	decision := service.LifecycleDecisionUnknown
	var followUp db.Issue
	created := false
	reused := false
	createdTaskID := ""
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
		followUp, created, reused, createdTaskID, err = h.resolveOrCreateLifecycleFollowUp(r, issue, req, actorType, actorID)
		if err != nil {
			h.writeLifecycleError(w, err)
			return
		}
		evidence = map[string]any{
			"route": req.Route, "cause_state": req.CauseState, "reason": req.Reason,
			"mitigation_complete": req.MitigationComplete, "separate_follow_up": req.SeparateFollowUp,
		}
	case "incident-learning":
		if req.Prevention == nil {
			writeError(w, http.StatusBadRequest, "incident-learning requires prevention task evidence")
			return
		}
		prevention := req.Prevention
		if strings.TrimSpace(req.FollowUpIssueID) == "" {
			if saved := parseIssueMetadata(issue.Metadata); saved != nil {
				if savedHandoff, ok := saved["lifecycle_handoff"].(map[string]any); ok {
					if preventionID, ok := savedHandoff["prevention_issue_id"].(string); ok {
						req.FollowUpIssueID = preventionID
					}
				}
			}
		}
		if strings.TrimSpace(prevention.Issue) == "" {
			prevention.Issue = prevention.Title
		}
		related := prevention.RelatedIncident
		if related == "" {
			related = uuidToString(issue.ID)
		}
		validation := service.IncidentLearningEvidence{
			SourceIssue: uuidToString(issue.ID), Facts: req.Facts, Inferences: req.Inferences,
			Unknowns: req.Unknowns, ExistingPreventionTasks: req.ExistingPreventionTasks,
			PreventionTasks: []service.PreventionTaskEvidence{{
				Issue: prevention.Issue, Title: prevention.Title, Owner: prevention.Owner,
				Priority: prevention.Priority, AcceptanceSignal: prevention.AcceptanceSignal,
				RelatedIncident: related,
			}},
		}
		// The source issue identifier is not needed for the contract's linkage
		// check beyond being stable and non-empty; UUID is always available.
		validation.SourceIssue = uuidToString(issue.ID)
		if err := service.ValidateIncidentLearning(validation); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var err error
		followUp, created, reused, createdTaskID, err = h.resolveOrCreateLifecycleFollowUp(r, issue, req, actorType, actorID)
		if err != nil {
			h.writeLifecycleError(w, err)
			return
		}
		if err := h.persistPreventionMetadata(r, followUp, issue, *prevention); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to persist prevention evidence")
			return
		}
		decision = service.LifecycleDecisionContinue
		evidence = map[string]any{
			"facts": req.Facts, "inferences": req.Inferences, "unknowns": req.Unknowns,
			"prevention": map[string]any{
				"owner": prevention.Owner, "priority": prevention.Priority,
				"acceptance_signal": prevention.AcceptanceSignal,
			},
		}
	case "rollout":
		if req.Rollout == nil {
			writeError(w, http.StatusBadRequest, "rollout evidence is required")
			return
		}
		rollout := service.RolloutEvidence{
			ApprovedDigest: req.Rollout.ApprovedDigest, ArtifactDigest: req.Rollout.ArtifactDigest,
			Baseline: req.Rollout.Baseline, WindowComplete: req.Rollout.WindowComplete,
			Signals: lifecycleRolloutSignals(req.Rollout.Signals), RollbackApproved: req.Rollout.RollbackApproved,
			RollbackExecuted: req.Rollout.RollbackExecuted,
		}
		decision = service.EvaluateRolloutEvidence(rollout)
		evidence = map[string]any{"approved_digest": rollout.ApprovedDigest, "artifact_digest": rollout.ArtifactDigest,
			"baseline": rollout.Baseline, "window_complete": rollout.WindowComplete, "signals": rollout.Signals,
			"rollback_approved": rollout.RollbackApproved, "rollback_executed": rollout.RollbackExecuted}
	case "agent-evaluation":
		if req.AgentEvaluation == nil {
			writeError(w, http.StatusBadRequest, "agent evaluation evidence is required")
			return
		}
		eval := service.AgentEvaluationEvidence{
			BaselineVersion: req.AgentEvaluation.BaselineVersion, CandidateVersion: req.AgentEvaluation.CandidateVersion,
			SkillVersion: req.AgentEvaluation.SkillVersion, MCPVersion: req.AgentEvaluation.MCPVersion,
			Cases: lifecycleAgentEvaluationCases(req.AgentEvaluation.Cases),
		}
		decision = service.ValidateAgentEvaluation(eval)
		evidence = map[string]any{"baseline_version": eval.BaselineVersion, "candidate_version": eval.CandidateVersion,
			"skill_version": eval.SkillVersion, "mcp_version": eval.MCPVersion, "cases": eval.Cases}
		if strings.TrimSpace(req.FollowUpIssueID) != "" || strings.TrimSpace(req.FollowUpTitle) != "" {
			var err error
			followUp, created, reused, createdTaskID, err = h.resolveOrCreateLifecycleFollowUp(r, issue, req, actorType, actorID)
			if err != nil {
				h.writeLifecycleError(w, err)
				return
			}
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported lifecycle handoff kind")
		return
	}

	metadata := map[string]any{
		"kind": req.Kind, "decision": string(decision), "source_issue_id": uuidToString(issue.ID),
		"evidence": evidence, "recorded_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if followUp.ID.Valid {
		metadata["follow_up_issue_id"] = uuidToString(followUp.ID)
	}
	if req.Kind == "incident-learning" && followUp.ID.Valid {
		metadata["prevention_issue_id"] = uuidToString(followUp.ID)
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil || len(metadataBytes) > 7000 {
		writeError(w, http.StatusBadRequest, "lifecycle evidence is too large")
		return
	}
	updated, err := h.Queries.SetIssueMetadataKey(r.Context(), db.SetIssueMetadataKeyParams{
		ID: issue.ID, WorkspaceID: issue.WorkspaceID, Key: "lifecycle_handoff", Value: metadataBytes,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist lifecycle evidence")
		return
	}

	queuedTaskID := ""
	if created {
		// IssueService already enqueued the assignment-triggered task.
		queuedTaskID = createdTaskID
	} else if followUp.ID.Valid {
		queuedTaskID = h.enqueueExistingLifecycleFollowUp(r, followUp, actorType, actorID, req.HandoffNote)
	}

	commentID := h.recordLifecycleAuditComment(r, issue, actorType, actorID, req.Kind, decision, evidence, followUp)
	writeJSON(w, http.StatusCreated, lifecycleHandoffResponse{
		Kind: req.Kind, Decision: string(decision), SourceIssueID: uuidToString(issue.ID),
		FollowUpIssueID: uuidToString(followUp.ID), FollowUpCreated: created, FollowUpReused: reused,
		QueuedTaskID: queuedTaskID, AuditCommentID: commentID, Metadata: parseIssueMetadata(updated.Metadata),
	})
}

func (h *Handler) resolveOrCreateLifecycleFollowUp(r *http.Request, source db.Issue, req lifecycleHandoffRequest, actorType, actorID string) (db.Issue, bool, bool, string, error) {
	if strings.TrimSpace(req.FollowUpIssueID) != "" {
		issue, err := h.resolveLifecycleIssue(r, req.FollowUpIssueID, source.WorkspaceID)
		if err != nil {
			return db.Issue{}, false, false, "", err
		}
		return issue, false, true, "", nil
	}
	if strings.TrimSpace(req.FollowUpTitle) == "" {
		return db.Issue{}, false, false, "", errors.New("follow_up_title is required when follow_up_issue_id is absent")
	}
	if h.IssueService == nil {
		return db.Issue{}, false, false, "", errors.New("issue service unavailable")
	}
	assigneeType, assigneeID, err := lifecycleAssignee(req)
	if err != nil {
		return db.Issue{}, false, false, "", err
	}
	if status, message := h.validateAssigneePair(r.Context(), r, uuidToString(source.WorkspaceID), assigneeType, assigneeID); status != 0 {
		return db.Issue{}, false, false, "", fmt.Errorf("%s", message)
	}
	creatorID, err := util.ParseUUID(actorID)
	if err != nil {
		return db.Issue{}, false, false, "", errors.New("invalid lifecycle actor")
	}
	priority := req.FollowUpPriority
	if priority == "" {
		priority = "none"
	}
	result, err := h.IssueService.Create(r.Context(), service.IssueCreateParams{
		WorkspaceID: source.WorkspaceID, Title: req.FollowUpTitle,
		Description: ptrToText(&req.FollowUpDescription), Status: "todo", Priority: priority,
		AssigneeType: assigneeType, AssigneeID: assigneeID, CreatorType: actorType,
		CreatorID: creatorID, ParentIssueID: source.ID,
	}, service.IssueCreateOpts{ActorID: actorID, AnalyticsAgentID: func() string {
		if assigneeType.Valid && assigneeType.String == "agent" {
			return uuidToString(assigneeID)
		}
		return ""
	}()})
	if errors.Is(err, service.ErrActiveDuplicate) && result.DuplicateIssue != nil {
		return *result.DuplicateIssue, false, true, "", nil
	}
	if err != nil {
		return db.Issue{}, false, false, "", err
	}
	return result.Issue, true, false, uuidToString(result.AssignedTaskID), nil
}

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

func (h *Handler) resolveLifecycleIssue(r *http.Request, raw string, workspaceID pgtype.UUID) (db.Issue, error) {
	workspace := uuidToString(workspaceID)
	if issue, ok := h.resolveIssueByIdentifier(r.Context(), raw, workspace); ok {
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
	return issue, nil
}

func (h *Handler) persistPreventionMetadata(r *http.Request, prevention db.Issue, source db.Issue, task lifecyclePreventionTaskRequest) error {
	values := map[string]string{
		"lifecycle_owner": task.Owner, "lifecycle_priority": task.Priority,
		"lifecycle_acceptance_signal": task.AcceptanceSignal, "lifecycle_source_issue": uuidToString(source.ID),
	}
	for key, value := range values {
		bytes, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := h.Queries.SetIssueMetadataKey(r.Context(), db.SetIssueMetadataKeyParams{
			ID: prevention.ID, WorkspaceID: prevention.WorkspaceID, Key: key, Value: bytes,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) enqueueExistingLifecycleFollowUp(r *http.Request, issue db.Issue, actorType, actorID, note string) string {
	if h.TaskService == nil || !issue.AssigneeID.Valid || !issue.AssigneeType.Valid {
		return ""
	}
	var task db.AgentTaskQueue
	var err error
	switch issue.AssigneeType.String {
	case "agent":
		task, err = h.TaskService.EnqueueTaskForIssueWithHandoff(r.Context(), issue, note, memberActorUserID(actorType, actorID))
	case "squad":
		if h.enqueueSquadLeaderTask(r.Context(), issue, pgtype.UUID{}, actorType, actorID, note) {
			return "queued"
		}
	}
	if err != nil || !task.ID.Valid {
		return ""
	}
	return uuidToString(task.ID)
}

func (h *Handler) recordLifecycleAuditComment(r *http.Request, issue db.Issue, actorType, actorID, kind string, decision service.LifecycleDecision, evidence map[string]any, followUp db.Issue) string {
	evidenceBytes, _ := json.Marshal(evidence)
	content := fmt.Sprintf("lifecycle-handoff kind=%s decision=%s follow_up=%s evidence=%s", kind, decision, uuidToString(followUp.ID), evidenceBytes)
	actorUUID, err := util.ParseUUID(actorID)
	if err != nil {
		return ""
	}
	var sourceTaskID pgtype.UUID
	if task, ok := h.taskFromRequestHeader(r); ok && actorType == "agent" {
		sourceTaskID = task.ID
	}
	created, err := h.Queries.CreateComment(r.Context(), db.CreateCommentParams{
		ID: dbid.NewV7(), IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
		AuthorType: actorType, AuthorID: actorUUID, Content: content, Type: "progress_update", SourceTaskID: sourceTaskID,
	})
	if err != nil {
		return ""
	}
	resp := commentToResponse(created.Comment(), nil, nil)
	resp.IssueRevision = created.IssueRevision
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), actorType, actorID, map[string]any{
		"comment": resp, "issue_title": issue.Title, "issue_revision": created.IssueRevision,
	})
	return uuidToString(created.ID)
}

func (h *Handler) writeLifecycleError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "not found") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
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
