package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) ProposeCollaborationTargetSwitch(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		collaborationResourceRequestBody
		ExpectedTargetEpoch   string                         `json:"expected_target_epoch"`
		ExpectedConfigVersion string                         `json:"expected_config_version"`
		Roster                []collaboration.TargetResource `json:"roster"`
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
	p := service.CollaborationProposeSwitchParams{CollaborationQueueRequest: request, TargetID: target, Roster: body.Roster}
	if !collaborationSwitchVersions(w, []collaborationSwitchVersion{{body.ExpectedTargetEpoch, &p.ExpectedTargetEpoch}, {body.ExpectedConfigVersion, &p.ExpectedConfigVersion}}) {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.ProposeTargetSwitch(r.Context(), p)
	writeCollaborationResourceResponse(w, receipt, err)
}

type collaborationSwitchVersion struct {
	raw string
	out *int64
}

func collaborationSwitchVersions(w http.ResponseWriter, versions []collaborationSwitchVersion) bool {
	for _, value := range versions {
		parsed, ok := collaborationPositiveVersion(value.raw)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
			return false
		}
		*value.out = parsed
	}
	return true
}

func collaborationSwitchPath(w http.ResponseWriter, r *http.Request) (target, proposal pgtype.UUID, ok bool) {
	target, ok = parseUUIDOrBadRequest(w, chi.URLParam(r, "targetId"), "target_id")
	if !ok {
		return
	}
	proposal, ok = parseUUIDOrBadRequest(w, chi.URLParam(r, "proposalId"), "proposal_id")
	return
}

func (h *Handler) ConsentCollaborationTargetSwitch(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		collaborationResourceRequestBody
		AgentID  string `json:"agent_id"`
		Identity struct {
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
	target, proposal, ok := collaborationSwitchPath(w, r)
	if !ok {
		return
	}
	p := service.CollaborationSwitchConsentParams{CollaborationQueueRequest: request, TargetID: target, ProposalID: proposal,
		Identity: service.CollaborationResourceIdentity{Kind: body.Identity.Kind}}
	for _, value := range []struct {
		raw, name string
		out       *pgtype.UUID
	}{{body.AgentID, "agent_id", &p.AgentID}, {body.Identity.InstallationID, "installation_id", &p.Identity.InstallationID}, {body.Identity.BindingID, "binding_id", &p.Identity.BindingID}} {
		id, valid := parseUUIDOrBadRequest(w, value.raw, value.name)
		if !valid {
			return
		}
		*value.out = id
	}
	if !collaborationSwitchVersions(w, []collaborationSwitchVersion{{body.Identity.BindingEpoch, &p.Identity.BindingEpoch}}) {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.ConsentTargetSwitchResource(r.Context(), p)
	writeCollaborationResourceResponse(w, receipt, err)
}

func (h *Handler) RevokeCollaborationTargetSwitchConsent(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationResourceRequestBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body)
	if !ok {
		return
	}
	target, proposal, ok := collaborationSwitchPath(w, r)
	if !ok {
		return
	}
	agent, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "agentId"), "agent_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.RevokeTargetSwitchConsent(r.Context(), service.CollaborationRevokeSwitchConsentParams{CollaborationQueueRequest: request, TargetID: target, ProposalID: proposal, AgentID: agent})
	writeCollaborationResourceResponse(w, receipt, err)
}

func (h *Handler) CommitCollaborationTargetSwitch(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		collaborationResourceRequestBody
		Control struct {
			TargetID                string          `json:"target_id"`
			ExpectedTargetEpoch     string          `json:"expected_target_epoch"`
			ExpectedConfigVersion   string          `json:"expected_config_version"`
			ExpectedHoldVersion     string          `json:"expected_hold_version"`
			ExpectedControlVersion  string          `json:"expected_control_version"`
			ObservedGroupID         json.RawMessage `json:"observed_group_id"`
			ObservedGroupGeneration string          `json:"observed_group_generation"`
		} `json:"control"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	target, proposal, ok := collaborationSwitchPath(w, r)
	if !ok {
		return
	}
	observedTarget, ok := parseUUIDOrBadRequest(w, body.Control.TargetID, "target_id")
	if !ok {
		return
	}
	if target != observedTarget {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_target_switch"})
		return
	}
	p := service.CollaborationCommitSwitchParams{CollaborationQueueRequest: request, ProposalID: proposal, Control: service.CollaborationTargetControl{TargetID: target}}
	if !collaborationSwitchVersions(w, []collaborationSwitchVersion{{body.Control.ExpectedTargetEpoch, &p.Control.ExpectedTargetEpoch},
		{body.Control.ExpectedConfigVersion, &p.Control.ExpectedConfigVersion}, {body.Control.ExpectedHoldVersion, &p.Control.ExpectedHoldVersion},
		{body.Control.ExpectedControlVersion, &p.Control.ExpectedControlVersion}}) {
		return
	}
	if body.Control.ObservedGroupGeneration != "0" && !collaborationSwitchVersions(w, []collaborationSwitchVersion{{body.Control.ObservedGroupGeneration, &p.Control.ObservedGroupGeneration}}) {
		return
	}
	var groupID *string
	if len(body.Control.ObservedGroupID) == 0 || json.Unmarshal(body.Control.ObservedGroupID, &groupID) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_target_switch"})
		return
	}
	if groupID != nil {
		p.Control.ObservedGroupID, ok = parseUUIDOrBadRequest(w, *groupID, "observed_group_id")
		if !ok {
			return
		}
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.CommitTargetSwitch(r.Context(), p)
	writeCollaborationResourceResponse(w, receipt, err)
}

func (h *Handler) GetCollaborationTargetSwitchProposal(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	target, proposal, ok := collaborationSwitchPath(w, r)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	view, err := svc.GetTargetSwitchProposal(r.Context(), issue, target, proposal)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) ListCollaborationTargetSwitchProposals(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	target, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "targetId"), "target_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	views, err := svc.ListTargetSwitchProposals(r.Context(), issue, target)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func (h *Handler) GetCollaborationSwitchOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.GetSwitchOperation(r.Context(), issue, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	if receipt.Redacted {
		writeJSON(w, http.StatusOK, struct {
			Redacted        bool        `json:"redacted"`
			ProtocolVersion int         `json:"protocol_version"`
			IssueID         pgtype.UUID `json:"issue_id"`
			RequestID       string      `json:"request_id"`
			Action          string      `json:"action"`
			Status          int         `json:"status"`
			Code            string      `json:"code"`
		}{true, receipt.ProtocolVersion, receipt.IssueID, receipt.RequestID, receipt.Action, receipt.Status, receipt.Code})
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
