package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationResourceRequestBody struct {
	ExpectedLifecycleEpoch   string `json:"expected_lifecycle_epoch"`
	ExpectedParticipantEpoch string `json:"expected_participant_epoch"`
	ExpectedAccountEpoch     string `json:"expected_account_epoch"`
}

func collaborationResourceRequest(w http.ResponseWriter, r *http.Request, body collaborationResourceRequestBody) (service.CollaborationQueueRequest, bool) {
	var result service.CollaborationQueueRequest
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return result, false
	}
	result.IssueID, result.RequestID = id, r.Header.Get("Idempotency-Key")
	if len(result.RequestID) == 0 || len(result.RequestID) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request_id"})
		return result, false
	}
	for _, value := range []struct {
		raw    string
		target *int64
	}{
		{body.ExpectedLifecycleEpoch, &result.ExpectedLifecycleEpoch}, {body.ExpectedParticipantEpoch, &result.ExpectedParticipantEpoch}, {body.ExpectedAccountEpoch, &result.ExpectedAccountEpoch},
	} {
		parsed, valid := collaborationPositiveVersion(value.raw)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
			return result, false
		}
		*value.target = parsed
	}
	return result, true
}

func writeCollaborationResourceResponse(w http.ResponseWriter, result service.CollaborationQueueReceipt, err error) {
	if err != nil && result.Code == "" {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, result.Status, result)
}

func (h *Handler) RegisterCollaborationTarget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct {
		collaborationResourceRequestBody
		Kind                     collaboration.TargetKind       `json:"kind"`
		ResourceID               string                         `json:"resource_id"`
		ExpectedTargetSetVersion string                         `json:"expected_target_set_version"`
		Roster                   []collaboration.TargetResource `json:"roster"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	resource, ok := parseUUIDOrBadRequest(w, body.ResourceID, "resource_id")
	if !ok {
		return
	}
	version, ok := collaborationPositiveVersion(body.ExpectedTargetSetVersion)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.RegisterTarget(r.Context(), service.CollaborationRegisterTargetParams{CollaborationQueueRequest: request,
		Kind: body.Kind, ResourceID: resource, ExpectedTargetSetVersion: version, Roster: body.Roster})
	writeCollaborationResourceResponse(w, result, err)
}

func (h *Handler) ConsentCollaborationTarget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct {
		collaborationResourceRequestBody
		AgentID               string `json:"agent_id"`
		ExpectedTargetEpoch   string `json:"expected_target_epoch"`
		ExpectedConfigVersion string `json:"expected_config_version"`
		Identity              struct {
			Kind           string `json:"kind"`
			InstallationID string `json:"installation_id"`
			BindingID      string `json:"binding_id"`
			BindingEpoch   string `json:"binding_epoch"`
		} `json:"identity"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	p := service.CollaborationConsentParams{CollaborationQueueRequest: request, Identity: service.CollaborationResourceIdentity{Kind: body.Identity.Kind}}
	for _, value := range []struct {
		raw, name string
		target    *pgtype.UUID
	}{
		{chi.URLParam(r, "targetId"), "target_id", &p.TargetID}, {body.AgentID, "agent_id", &p.AgentID},
		{body.Identity.InstallationID, "installation_id", &p.Identity.InstallationID}, {body.Identity.BindingID, "binding_id", &p.Identity.BindingID},
	} {
		id, valid := parseUUIDOrBadRequest(w, value.raw, value.name)
		if !valid {
			return
		}
		*value.target = id
	}
	for _, value := range []struct {
		raw    string
		target *int64
	}{
		{body.ExpectedTargetEpoch, &p.ExpectedTargetEpoch}, {body.ExpectedConfigVersion, &p.ExpectedConfigVersion}, {body.Identity.BindingEpoch, &p.Identity.BindingEpoch},
	} {
		version, valid := collaborationPositiveVersion(value.raw)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
			return
		}
		*value.target = version
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.ConsentTargetResource(r.Context(), p)
	writeCollaborationResourceResponse(w, result, err)
}

func (h *Handler) ReconfigureCollaborationTarget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct {
		collaborationResourceRequestBody
		Control service.CollaborationTargetControl `json:"control"`
		Roster  []collaboration.TargetResource     `json:"roster"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	target, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "targetId"), "target_id")
	if !ok {
		return
	}
	if body.Control.TargetID != target {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_target_configuration"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.ReconfigureTarget(r.Context(), service.CollaborationReconfigureTargetParams{CollaborationQueueRequest: request, Control: body.Control, Roster: body.Roster})
	writeCollaborationResourceResponse(w, result, err)
}

func (h *Handler) RevokeCollaborationGrant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct {
		collaborationResourceRequestBody
		ExpectedGrantEpoch string `json:"expected_grant_epoch"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	grant, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "grantId"), "grant_id")
	if !ok {
		return
	}
	epoch, ok := collaborationPositiveVersion(body.ExpectedGrantEpoch)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.RevokeGrant(r.Context(), service.CollaborationRevokeGrantParams{CollaborationQueueRequest: request, GrantID: grant, ExpectedGrantEpoch: epoch})
	writeCollaborationResourceResponse(w, result, err)
}
