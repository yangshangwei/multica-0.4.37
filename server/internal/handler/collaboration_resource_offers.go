package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func collaborationOfferInventoryVisible(member db.Member, agent db.Agent, runtime db.AgentRuntime, targets []db.AgentInvocationTarget) bool {
	return memberAllowedToViewAgent(agent, targets, uuidToString(member.UserID), member.Role) &&
		(canUseRuntimeForAgent(member, runtime) || roleAllowed(member.Role, "owner", "admin"))
}

func (h *Handler) GetCollaborationResourceOfferOptions(w http.ResponseWriter, r *http.Request) {
	issue, ok := collaborationResourceRead(w, r)
	if !ok {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query) != 1 || len(query["execution_workspace_id"]) != 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_resource_scope"})
		return
	}
	workspace, ok := parseUUIDOrBadRequest(w, query.Get("execution_workspace_id"), "execution_workspace_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.ResourceOfferOptions(r.Context(), issue, workspace, collaborationOfferInventoryVisible)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type collaborationResourceOfferCursor struct {
	IssueID         pgtype.UUID `json:"issue_id"`
	ViewerID        pgtype.UUID `json:"viewer_id"`
	BeforeCreatedAt time.Time   `json:"before_created_at"`
	BeforeID        pgtype.UUID `json:"before_id"`
}

func (h *Handler) ListCollaborationResourceOffers(w http.ResponseWriter, r *http.Request) {
	issue, ok := collaborationResourceRead(w, r)
	if !ok {
		return
	}
	principal, _ := collaboration.PrincipalFromContext(r.Context())
	viewer, ok := parseUUIDOrBadRequest(w, principal.UserID, "viewer_id")
	if !ok {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_resource_offer_query"})
		return
	}
	for key, values := range query {
		if len(values) != 1 || key != "limit" && key != "cursor" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_resource_offer_query"})
			return
		}
	}
	p := service.CollaborationResourceOfferQuery{Limit: 50}
	if query.Has("limit") {
		limit, err := strconv.Atoi(query.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_limit"})
			return
		}
		p.Limit = limit
	}
	if query.Has("cursor") {
		raw := query.Get("cursor")
		if len(raw) == 0 || len(raw) > 512 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_cursor"})
			return
		}
		data, err := base64.RawURLEncoding.Strict().DecodeString(raw)
		var cursor collaborationResourceOfferCursor
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err != nil || base64.RawURLEncoding.EncodeToString(data) != raw || iteration.ValidateObjectJSON(data) != nil || decoder.Decode(&cursor) != nil ||
			cursor.IssueID != issue || cursor.ViewerID != viewer || !cursor.BeforeID.Valid || cursor.BeforeCreatedAt.IsZero() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_cursor"})
			return
		}
		p.BeforeCreatedAt, p.BeforeID = pgtype.Timestamptz{Time: cursor.BeforeCreatedAt, Valid: true}, cursor.BeforeID
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.ListResourceOffers(r.Context(), issue, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		data, err := json.Marshal(collaborationResourceOfferCursor{IssueID: issue, ViewerID: viewer, BeforeCreatedAt: last.CreatedAt, BeforeID: last.OfferID})
		if err != nil {
			writeCollaborationError(w, err)
			return
		}
		cursor := base64.RawURLEncoding.EncodeToString(data)
		page.NextCursor = &cursor
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) GetCollaborationResourceOffer(w http.ResponseWriter, r *http.Request) {
	issue, ok := collaborationResourceRead(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_resource_scope"})
		return
	}
	offer, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "offerId"), "offer_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.GetResourceOffer(r.Context(), issue, offer)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func prepareCollaborationResourceOfferWrite(w http.ResponseWriter, r *http.Request) bool {
	prepareCollaborationContentRequest(w, r)
	principal, ok := collaboration.PrincipalFromContext(r.Context())
	if !ok || !principal.Human() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "forbidden"})
		return false
	}
	if r.URL.RawQuery != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_resource_scope"})
		return false
	}
	if len(r.Header.Values("Idempotency-Key")) != 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request_id"})
		return false
	}
	return true
}

func (h *Handler) OfferCollaborationResource(w http.ResponseWriter, r *http.Request) {
	if !prepareCollaborationResourceOfferWrite(w, r) {
		return
	}
	var body struct {
		collaborationResourceRequestBody
		ExecutionWorkspaceID string                         `json:"execution_workspace_id"`
		Kind                 collaboration.TargetKind       `json:"kind"`
		ResourceID           string                         `json:"resource_id"`
		Roster               []collaboration.TargetResource `json:"roster"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	workspace, ok := parseUUIDOrBadRequest(w, body.ExecutionWorkspaceID, "execution_workspace_id")
	if !ok {
		return
	}
	resource, ok := parseUUIDOrBadRequest(w, body.ResourceID, "resource_id")
	if !ok {
		return
	}
	for i := range body.Roster {
		for _, value := range []struct {
			name   string
			target *string
		}{
			{"agent_id", &body.Roster[i].AgentID}, {"runtime_id", &body.Roster[i].RuntimeID},
			{"execution_workspace_id", &body.Roster[i].ExecutionWorkspaceID}, {"owner_user_id", &body.Roster[i].OwnerUserID},
		} {
			id, ok := parseUUIDOrBadRequest(w, *value.target, value.name)
			if !ok {
				return
			}
			*value.target = uuidToString(id)
		}
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.OfferResource(r.Context(), service.CollaborationOfferResourceParams{CollaborationQueueRequest: request,
		ExecutionWorkspaceID: workspace, Kind: body.Kind, ResourceID: resource, Roster: body.Roster}, collaborationOfferInventoryVisible)
	writeCollaborationResourceResponse(w, result, err)
}

func (h *Handler) WithdrawCollaborationResourceOffer(w http.ResponseWriter, r *http.Request) {
	if !prepareCollaborationResourceOfferWrite(w, r) {
		return
	}
	var body struct {
		collaborationResourceRequestBody
		ExpectedOfferVersion string `json:"expected_offer_version"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	offer, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "offerId"), "offer_id")
	if !ok {
		return
	}
	version, ok := collaborationPositiveVersion(body.ExpectedOfferVersion)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.WithdrawResourceOffer(r.Context(), service.CollaborationWithdrawResourceOfferParams{CollaborationQueueRequest: request,
		OfferID: offer, ExpectedOfferVersion: version})
	writeCollaborationResourceResponse(w, result, err)
}

func (h *Handler) AdoptCollaborationResourceOffer(w http.ResponseWriter, r *http.Request) {
	if !prepareCollaborationResourceOfferWrite(w, r) {
		return
	}
	var body struct {
		collaborationResourceRequestBody
		ExpectedOfferVersion     string `json:"expected_offer_version"`
		ExpectedTargetSetVersion string `json:"expected_target_set_version"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationResourceRequest(w, r, body.collaborationResourceRequestBody)
	if !ok {
		return
	}
	offer, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "offerId"), "offer_id")
	if !ok {
		return
	}
	version, ok := collaborationPositiveVersion(body.ExpectedOfferVersion)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
		return
	}
	targetSet, ok := collaborationPositiveVersion(body.ExpectedTargetSetVersion)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.AdoptResourceOffer(r.Context(), service.CollaborationAdoptResourceOfferParams{CollaborationQueueRequest: request,
		OfferID: offer, ExpectedOfferVersion: version, ExpectedTargetSetVersion: targetSet})
	writeCollaborationResourceResponse(w, result, err)
}
