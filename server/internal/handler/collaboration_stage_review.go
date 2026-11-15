package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

type stageReviewObservations struct {
	collaborationWorkflowBody
	ExpectedPolicyRevision string `json:"expected_policy_revision"`
	ExpectedRoleRevision   string `json:"expected_role_revision"`
}

// Stage action bodies have no arbitrary object-key namespaces. Require exact
// lowercase wire names before encoding/json's case-folded struct matching.
func stageActionKeys(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			for _, c := range key {
				if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
					return false
				}
			}
			if !stageActionKeys(child) {
				return false
			}
		}
	case []any:
		for _, child := range v {
			if !stageActionKeys(child) {
				return false
			}
		}
	}
	return true
}

func decodeStageAction(w http.ResponseWriter, r *http.Request, target any, limit int64) bool {
	var object map[string]any
	if !decodeCollaborationWorkflowBodyLimit(w, r, &object, limit) {
		return false
	}
	if !stageActionKeys(object) {
		writeStageInvalid(w)
		return false
	}
	raw, err := json.Marshal(object)
	if err != nil {
		writeStageInvalid(w)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		writeStageInvalid(w)
		return false
	}
	return true
}

func stageNullableObject(raw json.RawMessage, target any) (present, valid bool) {
	if len(raw) == 0 {
		return false, false
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, true
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return false, false
	}
	return true, true
}

func stageReviewRequest(w http.ResponseWriter, r *http.Request, body stageReviewObservations) (service.CollaborationWorkflowRequest, int64, int64, bool) {
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return request, 0, 0, false
	}
	policy, pok := stageVersionField(body.ExpectedPolicyRevision, false)
	role, rok := stageVersionField(body.ExpectedRoleRevision, false)
	if !pok || !rok {
		writeStageInvalid(w)
		return request, 0, 0, false
	}
	return request, policy, role, true
}

func (h *Handler) CreateCollaborationStageCodeCandidate(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		stageReviewObservations
		ChildIssueID string          `json:"child_issue_id"`
		Supersedes   json.RawMessage `json:"supersedes"`
		PullRequest  *struct {
			Provider      string `json:"provider"`
			PullRequestID string `json:"pull_request_id"`
			HeadSHA       string `json:"head_sha"`
		} `json:"pull_request"`
		Artifact json.RawMessage `json:"artifact"`
	}
	if !decodeStageAction(w, r, &body, 256<<10) {
		return
	}
	request, policy, role, ok := stageReviewRequest(w, r, body.stageReviewObservations)
	if !ok {
		return
	}
	child, valid := stageUUIDField(body.ChildIssueID)
	if !valid || body.PullRequest == nil {
		writeStageInvalid(w)
		return
	}
	pr, valid := stageUUIDField(body.PullRequest.PullRequestID)
	if !valid {
		writeStageInvalid(w)
		return
	}
	p := service.CollaborationStageCodeParams{CollaborationWorkflowRequest: request, ExpectedPolicyRevision: policy, ExpectedRoleRevision: role, ChildIssueID: child,
		PullRequest: service.CollaborationStageCodeRef{Provider: body.PullRequest.Provider, PullRequestID: pr, HeadSHA: body.PullRequest.HeadSHA}}
	var prior collaborationStageCandidateBody
	if present, valid := stageNullableObject(body.Supersedes, &prior); !valid {
		writeStageInvalid(w)
		return
	} else if present {
		ref, valid := stageCandidateField(&prior)
		if !valid {
			writeStageInvalid(w)
			return
		}
		p.Supersedes = &ref
	}
	var artifact struct {
		ApprovalID string `json:"approval_id"`
	}
	if present, valid := stageNullableObject(body.Artifact, &artifact); !valid {
		writeStageInvalid(w)
		return
	} else if present {
		id, valid := stageUUIDField(artifact.ApprovalID)
		if !valid {
			writeStageInvalid(w)
			return
		}
		p.Artifact = &service.CollaborationStageArtifactSelection{ApprovalID: id}
	}
	receipt, err := h.stageService().CreateStageCodeCandidate(r.Context(), p)
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func stageDecisionExpiry(raw json.RawMessage) (*time.Time, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true
	}
	var value string
	if json.Unmarshal(raw, &value) != nil || !strings.HasSuffix(value, "Z") {
		return nil, false
	}
	if dot := strings.IndexByte(value, '.'); dot >= 0 && len(value)-dot-2 > 6 {
		return nil, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func (h *Handler) DecideCollaborationStage(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		stageReviewObservations
		ExpectedStageRevision string                           `json:"expected_stage_revision"`
		Step                  string                           `json:"step"`
		Decision              string                           `json:"decision"`
		Candidate             *collaborationStageCandidateBody `json:"candidate"`
		Evidence              json.RawMessage                  `json:"evidence"`
		DependsOn             *[]string                        `json:"depends_on"`
		Reason                *string                          `json:"reason"`
		ExpiresAt             json.RawMessage                  `json:"expires_at"`
	}
	if !decodeStageAction(w, r, &body, 256<<10) {
		return
	}
	request, policy, role, ok := stageReviewRequest(w, r, body.stageReviewObservations)
	if !ok {
		return
	}
	stage, validStage := stageVersionField(body.ExpectedStageRevision, false)
	candidate, validCandidate := stageCandidateField(body.Candidate)
	expires, validExpiry := stageDecisionExpiry(body.ExpiresAt)
	var evidence service.CollaborationStageReviewEvidence
	var fields map[string]json.RawMessage
	if !validStage || !validCandidate || !validExpiry || body.DependsOn == nil || body.Reason == nil || json.Unmarshal(body.Evidence, &fields) != nil || (len(fields) != 5 && len(fields) != 6) {
		writeStageInvalid(w)
		return
	}
	for _, key := range []string{"ci_digest", "test_report_id", "build", "deployment_id", "rollback_target_digest"} {
		if len(fields[key]) == 0 {
			writeStageInvalid(w)
			return
		}
	}
	if body.Step == "code_review" || body.Step == "qa" {
		var build map[string]json.RawMessage
		if json.Unmarshal(fields["build"], &build) == nil {
			if _, present := build["target_digest"]; present {
				writeStageInvalid(w)
				return
			}
		}
	}
	if present, valid := stageNullableObject(body.Evidence, &evidence); !valid || !present {
		writeStageInvalid(w)
		return
	}
	receipt, err := h.stageService().DecideStage(r.Context(), service.CollaborationStageReviewParams{CollaborationWorkflowRequest: request, ExpectedPolicyRevision: policy, ExpectedRoleRevision: role,
		ExpectedStageRevision: stage, Step: body.Step, Decision: body.Decision, Candidate: candidate, Evidence: evidence, DependsOn: *body.DependsOn, Reason: *body.Reason, ExpiresAt: expires})
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func (h *Handler) RevokeCollaborationStageDecision(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		stageReviewObservations
		Reason *string `json:"reason"`
	}
	if !decodeStageAction(w, r, &body, 64<<10) {
		return
	}
	request, policy, role, ok := stageReviewRequest(w, r, body.stageReviewObservations)
	if !ok {
		return
	}
	id, valid := stageUUIDField(chi.URLParam(r, "decisionId"))
	if !valid || body.Reason == nil {
		writeStageInvalid(w)
		return
	}
	receipt, err := h.stageService().RevokeStageDecision(r.Context(), service.CollaborationStageRevocationParams{CollaborationWorkflowRequest: request, ExpectedPolicyRevision: policy, ExpectedRoleRevision: role, DecisionID: id, Reason: *body.Reason})
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func (h *Handler) AdvanceCollaborationStage(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		stageReviewObservations
		ExpectedStageRevision string                           `json:"expected_stage_revision"`
		TargetStage           string                           `json:"target_stage"`
		Candidate             *collaborationStageCandidateBody `json:"candidate"`
		DecisionIDs           *[]string                        `json:"decision_ids"`
	}
	if !decodeStageAction(w, r, &body, 64<<10) {
		return
	}
	request, policy, role, ok := stageReviewRequest(w, r, body.stageReviewObservations)
	if !ok {
		return
	}
	stage, validStage := stageVersionField(body.ExpectedStageRevision, false)
	candidate, validCandidate := stageCandidateField(body.Candidate)
	if !validStage || !validCandidate || body.DecisionIDs == nil {
		writeStageInvalid(w)
		return
	}
	receipt, err := h.stageService().AdvanceStage(r.Context(), service.CollaborationStageAdvanceParams{CollaborationWorkflowRequest: request, ExpectedPolicyRevision: policy, ExpectedRoleRevision: role,
		ExpectedStageRevision: stage, TargetStage: body.TargetStage, Candidate: candidate, DecisionIDs: *body.DecisionIDs})
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func (h *Handler) GetCollaborationStageCodeEvidence(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, iok := stageUUIDField(chi.URLParam(r, "id"))
	candidate, cok := stageUUIDField(chi.URLParam(r, "candidateId"))
	if !iok || !cok {
		writeStageInvalid(w)
		return
	}
	view, err := h.stageService().StageCodeEvidence(r.Context(), issue, candidate)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
