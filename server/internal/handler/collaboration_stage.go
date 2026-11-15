package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

// stagePolicyActivationAvailable gates enabling a project stage policy over
// HTTP. It stays false until every completion, submission, project-detach and
// delete writer consumes the final stage guard: an intermediate build must not
// be switchable on in a real project. Disabling is never gated.
const stagePolicyActivationAvailable = false

type collaborationStageRolesBody struct {
	collaborationWorkflowBody
	ExpectedPolicyRevision string `json:"expected_policy_revision"`
	ExpectedRoleRevision   string `json:"expected_role_revision"`
	Roles                  *struct {
		BusinessOwner   string `json:"business_owner_user_id"`
		Implementer     string `json:"implementer_user_id"`
		ProductApprover string `json:"product_approver_user_id"`
		CodeReviewer    string `json:"code_reviewer_user_id"`
		QAApprover      string `json:"qa_approver_user_id"`
		ReleaseApprover string `json:"release_approver_user_id"`
	} `json:"roles"`
}

type collaborationStageContentBody struct {
	Summary  *string `json:"summary"`
	Children []struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Priority    *string `json:"priority"`
	} `json:"children"`
}

type collaborationStageSourceBody struct {
	Kind            string  `json:"kind"`
	CommentID       *string `json:"comment_id"`
	CommentRevision *string `json:"comment_revision"`
	RunID           *string `json:"run_id"`
}

type collaborationStageCandidateBody struct {
	CandidateID   string `json:"candidate_id"`
	Version       string `json:"version"`
	ContentDigest string `json:"content_digest"`
}

type collaborationStageProposalBody struct {
	collaborationWorkflowBody
	ExpectedPolicyRevision string                           `json:"expected_policy_revision"`
	ExpectedCandidate      *collaborationStageCandidateBody `json:"expected_candidate"`
	Content                *collaborationStageContentBody   `json:"content"`
	Source                 *collaborationStageSourceBody    `json:"source"`
}

type collaborationStageDecisionBody struct {
	collaborationWorkflowBody
	ExpectedPolicyRevision string                           `json:"expected_policy_revision"`
	ExpectedRoleRevision   string                           `json:"expected_role_revision"`
	Candidate              *collaborationStageCandidateBody `json:"candidate"`
	Decision               string                           `json:"decision"`
	Reason                 *string                          `json:"reason"`
}

type collaborationStageScopeBody struct {
	collaborationWorkflowBody
	ExpectedPolicyRevision string `json:"expected_policy_revision"`
	ExpectedRoleRevision   string `json:"expected_role_revision"`
	ExpectedScopeRevision  string `json:"expected_scope_revision"`
	Children               *[]struct {
		IssueID       string `json:"issue_id"`
		ContentDigest string `json:"content_digest"`
		// Code is required: explicit null, or one pull request reference.
		Code json.RawMessage `json:"code"`
	} `json:"children"`
	Reason *string `json:"reason"`
}

type stagePolicyBody struct {
	ExpectedPolicyRevision string  `json:"expected_policy_revision"`
	Enabled                *bool   `json:"enabled"`
	Reason                 *string `json:"reason"`
}

func writeStageInvalid(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_stage_request", "error": "invalid_stage_request"})
}

// stageVersionField parses a canonical decimal stage version: no sign, no
// leading zero, signed 64-bit. Zero is accepted only where "none yet" is legal.
func stageVersionField(value string, allowZero bool) (int64, bool) {
	if allowZero && value == "0" {
		return 0, true
	}
	return collaborationPositiveVersion(value)
}

// stageUUIDField accepts only a canonical, non-zero UUID string.
func stageUUIDField(value string) (pgtype.UUID, bool) {
	id, err := util.ParseUUID(value)
	if err != nil || !id.Valid || id.Bytes == [16]byte{} || util.UUIDToString(id) != value {
		return pgtype.UUID{}, false
	}
	return id, true
}

func stageCandidateField(body *collaborationStageCandidateBody) (service.CollaborationStageCandidateRef, bool) {
	var out service.CollaborationStageCandidateRef
	if body == nil {
		return out, false
	}
	id, ok := stageUUIDField(body.CandidateID)
	version, valid := stageVersionField(body.Version, false)
	if !ok || !valid || len(body.ContentDigest) != 64 {
		return out, false
	}
	return service.CollaborationStageCandidateRef{CandidateID: id, Version: version, ContentDigest: body.ContentDigest}, true
}

// stageCodeField decodes the required per-child code version: an explicit null
// ("no code is carried forward") or exactly one pull request reference. An
// absent field is refused; nothing is inferred.
func stageCodeField(raw json.RawMessage) (*service.CollaborationStageCodeRef, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, true
	}
	var body struct {
		PullRequest *struct {
			Provider      string `json:"provider"`
			PullRequestID string `json:"pull_request_id"`
			HeadSHA       string `json:"head_sha"`
		} `json:"pull_request"`
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if iteration.ValidateObjectJSON(trimmed) != nil || decoder.Decode(&body) != nil || body.PullRequest == nil {
		return nil, false
	}
	id, ok := stageUUIDField(body.PullRequest.PullRequestID)
	if !ok {
		return nil, false
	}
	return &service.CollaborationStageCodeRef{Provider: body.PullRequest.Provider, PullRequestID: id, HeadSHA: body.PullRequest.HeadSHA}, true
}

func (h *Handler) stageService() *service.CollaborationService {
	s := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	s.StageExecutionSources = h.StageExecutionSources
	return s
}

func (h *Handler) ConfigureCollaborationStageRoles(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationStageRolesBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	policy, validPolicy := stageVersionField(body.ExpectedPolicyRevision, false)
	role, validRole := stageVersionField(body.ExpectedRoleRevision, true)
	if !validPolicy || !validRole || body.Roles == nil {
		writeStageInvalid(w)
		return
	}
	var set service.CollaborationStageRoleSet
	for _, field := range []struct {
		raw string
		out *pgtype.UUID
	}{{body.Roles.BusinessOwner, &set.BusinessOwner}, {body.Roles.Implementer, &set.Implementer}, {body.Roles.ProductApprover, &set.ProductApprover},
		{body.Roles.CodeReviewer, &set.CodeReviewer}, {body.Roles.QAApprover, &set.QAApprover}, {body.Roles.ReleaseApprover, &set.ReleaseApprover}} {
		id, valid := stageUUIDField(field.raw)
		if !valid {
			writeStageInvalid(w)
			return
		}
		*field.out = id
	}
	receipt, err := h.stageService().ConfigureStageRoles(r.Context(), service.CollaborationStageRolesParams{CollaborationWorkflowRequest: request,
		ExpectedPolicyRevision: policy, ExpectedRoleRevision: role, Roles: set})
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func stageProposalFields(body collaborationStageProposalBody) (collaboration.StageProposalContent, service.CollaborationStageProposalSource, bool) {
	var content collaboration.StageProposalContent
	var source service.CollaborationStageProposalSource
	if body.Content == nil || body.Content.Summary == nil || body.Content.Children == nil || body.Source == nil {
		return content, source, false
	}
	content.Summary = *body.Content.Summary
	for _, child := range body.Content.Children {
		if child.Title == nil || child.Description == nil || child.Priority == nil {
			return content, source, false
		}
		content.Children = append(content.Children, collaboration.StageProposalChild{Title: *child.Title, Description: *child.Description, Priority: *child.Priority})
	}
	source.Kind = body.Source.Kind
	switch source.Kind {
	case "manual":
		if body.Source.CommentID != nil || body.Source.CommentRevision != nil || body.Source.RunID != nil {
			return content, source, false
		}
	case "comment":
		if body.Source.CommentID == nil || body.Source.CommentRevision == nil || body.Source.RunID != nil {
			return content, source, false
		}
		id, ok := stageUUIDField(*body.Source.CommentID)
		revision, valid := stageVersionField(*body.Source.CommentRevision, false)
		if !ok || !valid {
			return content, source, false
		}
		source.CommentID, source.CommentRevision = id, revision
	case "run":
		if body.Source.RunID == nil || body.Source.CommentID != nil || body.Source.CommentRevision != nil {
			return content, source, false
		}
		id, ok := stageUUIDField(*body.Source.RunID)
		if !ok {
			return content, source, false
		}
		source.RunID = id
	default:
		return content, source, false
	}
	return content, source, true
}

func (h *Handler) CreateCollaborationStageProposal(w http.ResponseWriter, r *http.Request) {
	h.writeCollaborationStageProposal(w, r, false)
}

func (h *Handler) ReviseCollaborationStageProposal(w http.ResponseWriter, r *http.Request) {
	h.writeCollaborationStageProposal(w, r, true)
}

func (h *Handler) writeCollaborationStageProposal(w http.ResponseWriter, r *http.Request, revise bool) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationStageProposalBody
	// A legal 512 KiB proposal must still fit when its text is JSON-escaped.
	if !decodeCollaborationWorkflowBodyLimit(w, r, &body, 2<<20) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	policy, validPolicy := stageVersionField(body.ExpectedPolicyRevision, false)
	content, source, validContent := stageProposalFields(body)
	if !validPolicy || !validContent || (body.ExpectedCandidate != nil) != revise {
		writeStageInvalid(w)
		return
	}
	params := service.CollaborationStageProposalParams{CollaborationWorkflowRequest: request, ExpectedPolicyRevision: policy, Content: content, Source: source}
	var receipt service.CollaborationWorkflowReceipt
	var err error
	if revise {
		proposal, validProposal := stageUUIDField(chi.URLParam(r, "proposalId"))
		expected, validExpected := stageCandidateField(body.ExpectedCandidate)
		if !validProposal || !validExpected {
			writeStageInvalid(w)
			return
		}
		params.ProposalID, params.ExpectedCandidate = proposal, &expected
		receipt, err = h.stageService().ReviseStageProposal(r.Context(), params)
	} else {
		receipt, err = h.stageService().CreateStageProposal(r.Context(), params)
	}
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func (h *Handler) DecideCollaborationStageProposal(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationStageDecisionBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	policy, validPolicy := stageVersionField(body.ExpectedPolicyRevision, false)
	role, validRole := stageVersionField(body.ExpectedRoleRevision, true)
	proposal, validProposal := stageUUIDField(chi.URLParam(r, "proposalId"))
	candidate, validCandidate := stageCandidateField(body.Candidate)
	if !validPolicy || !validRole || !validProposal || !validCandidate || body.Reason == nil {
		writeStageInvalid(w)
		return
	}
	receipt, err := h.stageService().DecideStageProposal(r.Context(), service.CollaborationStageDecisionParams{CollaborationWorkflowRequest: request,
		ExpectedPolicyRevision: policy, ExpectedRoleRevision: role, ProposalID: proposal, Candidate: candidate, Decision: body.Decision, Reason: *body.Reason})
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func (h *Handler) ConfirmCollaborationStageScope(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationStageScopeBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	policy, validPolicy := stageVersionField(body.ExpectedPolicyRevision, false)
	role, validRole := stageVersionField(body.ExpectedRoleRevision, true)
	scope, validScope := stageVersionField(body.ExpectedScopeRevision, true)
	if !validPolicy || !validRole || !validScope || body.Children == nil || body.Reason == nil {
		writeStageInvalid(w)
		return
	}
	children := make([]service.CollaborationStageScopeChild, 0, len(*body.Children))
	for _, child := range *body.Children {
		id, valid := stageUUIDField(child.IssueID)
		code, validCode := stageCodeField(child.Code)
		if !valid || !validCode {
			writeStageInvalid(w)
			return
		}
		children = append(children, service.CollaborationStageScopeChild{IssueID: id, ContentDigest: child.ContentDigest, Code: code})
	}
	receipt, err := h.stageService().ConfirmStageScope(r.Context(), service.CollaborationStageScopeParams{CollaborationWorkflowRequest: request,
		ExpectedPolicyRevision: policy, ExpectedRoleRevision: role, ExpectedScopeRevision: scope, Children: children, Reason: *body.Reason})
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func (h *Handler) GetCollaborationStage(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := stageUUIDField(chi.URLParam(r, "id"))
	if !ok {
		writeStageInvalid(w)
		return
	}
	view, err := h.stageService().StageView(r.Context(), id)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) ListCollaborationStageProposals(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := stageUUIDField(chi.URLParam(r, "id"))
	query := service.CollaborationStageProposalQuery{}
	for key, values := range r.URL.Query() {
		if (key != "limit" && key != "cursor") || len(values) != 1 {
			ok = false
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || strconv.Itoa(value) != raw {
			ok = false
		}
		query.Limit = value
	} else if _, present := r.URL.Query()["limit"]; present {
		ok = false
	}
	query.Cursor = r.URL.Query().Get("cursor")
	if !ok {
		writeStageInvalid(w)
		return
	}
	page, err := h.stageService().ListStageProposals(r.Context(), id, query)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) GetCollaborationStageProposal(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := stageUUIDField(chi.URLParam(r, "id"))
	proposal, valid := stageUUIDField(chi.URLParam(r, "proposalId"))
	if !ok || !valid {
		writeStageInvalid(w)
		return
	}
	view, err := h.stageService().GetStageProposal(r.Context(), id, proposal)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// stagePolicyRequest resolves the workspace project and the verified human
// credential shared by the three project stage policy routes.
func (h *Handler) stagePolicyRequest(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, service.ArtifactCredentialSnapshot, bool) {
	w.Header().Set("Cache-Control", "no-store")
	principal, kind, _, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "stage_policy_denied", "error": "stage_policy_denied"})
		return pgtype.UUID{}, pgtype.UUID{}, service.ArtifactCredentialSnapshot{}, false
	}
	workspace, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, service.ArtifactCredentialSnapshot{}, false
	}
	project, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project_id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, service.ArtifactCredentialSnapshot{}, false
	}
	return workspace, project, artifactCredentialSnapshot(r.Context(), principal), true
}

func (h *Handler) GetStagePolicy(w http.ResponseWriter, r *http.Request) {
	workspace, project, credential, ok := h.stagePolicyRequest(w, r)
	if !ok {
		return
	}
	policy, err := h.stageService().GetStagePolicy(r.Context(), workspace, project, credential)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	policy.ActivationAvailable = stagePolicyActivationAvailable
	writeJSON(w, http.StatusOK, policy)
}

func (h *Handler) UpdateStagePolicy(w http.ResponseWriter, r *http.Request) {
	workspace, project, credential, ok := h.stagePolicyRequest(w, r)
	if !ok {
		return
	}
	var body stagePolicyBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	expected, valid := stageVersionField(body.ExpectedPolicyRevision, true)
	if !valid || body.Enabled == nil || body.Reason == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_stage_policy_request", "error": "invalid_stage_policy_request"})
		return
	}
	requestID := r.Header.Get("Idempotency-Key")
	if len(requestID) == 0 || len(requestID) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request_id", "error": "Idempotency key required"})
		return
	}
	if *body.Enabled && !stagePolicyActivationAvailable {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "stage_policy_unavailable", "error": "stage_policy_unavailable"})
		return
	}
	receipt, err := h.stageService().ReviseStagePolicy(r.Context(), service.StagePolicyRevisionParams{WorkspaceID: workspace, ProjectID: project,
		RequestID: requestID, ExpectedRevision: expected, Enabled: *body.Enabled, Reason: *body.Reason, Credential: credential})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

func (h *Handler) GetStagePolicyOperation(w http.ResponseWriter, r *http.Request) {
	workspace, project, credential, ok := h.stagePolicyRequest(w, r)
	if !ok {
		return
	}
	receipt, err := h.stageService().GetStagePolicyOperation(r.Context(), workspace, project, credential, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
