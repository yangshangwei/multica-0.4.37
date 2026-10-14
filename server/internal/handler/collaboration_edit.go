package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationEditBody struct {
	collaborationWorkflowBody
	// Keep raw presence through union validation: null is still an explicit
	// field and cannot hide metadata inside a status request.
	Title                json.RawMessage `json:"title"`
	Description          json.RawMessage `json:"description"`
	StartDate            json.RawMessage `json:"start_date"`
	DueDate              json.RawMessage `json:"due_date"`
	Priority             json.RawMessage `json:"priority"`
	LabelIDs             json.RawMessage `json:"label_ids"`
	Properties           json.RawMessage `json:"properties"`
	Status               json.RawMessage `json:"status"`
	StatusOptionsVersion json.RawMessage `json:"status_options_version"`
}

func (h *Handler) UpdateCollaborationIssue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	var body collaborationEditBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	metadata := len(body.Title) > 0 || len(body.Description) > 0 || len(body.StartDate) > 0 || len(body.DueDate) > 0 ||
		len(body.Priority) > 0 || len(body.LabelIDs) > 0 || len(body.Properties) > 0
	status := len(body.Status) > 0 || len(body.StatusOptionsVersion) > 0
	if metadata && status {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "error": "Status and content edits require separate requests"})
		return
	}
	params := service.CollaborationEditParams{CollaborationWorkflowRequest: request, SetStartDate: len(body.StartDate) > 0, SetDueDate: len(body.DueDate) > 0}
	var targetStatus, statusVersion string
	var labels *[]string
	for _, field := range []struct {
		raw    json.RawMessage
		target any
	}{
		{body.Title, &params.Title}, {body.Description, &params.Description},
		{body.StartDate, &params.StartDate}, {body.DueDate, &params.DueDate},
		{body.Priority, &params.Priority}, {body.LabelIDs, &labels}, {body.Properties, &params.Properties},
		{body.Status, &targetStatus}, {body.StatusOptionsVersion, &statusVersion},
	} {
		if len(field.raw) > 0 && json.Unmarshal(field.raw, field.target) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "error": "Invalid request"})
			return
		}
	}
	if labels != nil {
		ids := make([]pgtype.UUID, 0, len(*labels))
		for _, label := range *labels {
			id, ok := parseUUIDOrBadRequest(w, label, "label_id")
			if !ok {
				return
			}
			ids = append(ids, id)
		}
		params.LabelIDs = &ids
	}
	if params.Properties != nil {
		properties := make(map[string]json.RawMessage, len(params.Properties))
		for key, value := range params.Properties {
			id, ok := parseUUIDOrBadRequest(w, key, "property_id")
			if !ok {
				return
			}
			key = uuidToString(id)
			if _, duplicate := properties[key]; duplicate {
				writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "error": "Duplicate property identity"})
				return
			}
			properties[key] = value
		}
		params.Properties = properties
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	var receipt service.CollaborationWorkflowReceipt
	var err error
	if status {
		receipt, err = svc.ChangeStatus(r.Context(), service.CollaborationStatusParams{
			CollaborationWorkflowRequest: request, TargetStatus: targetStatus, StatusOptionsVersion: statusVersion,
		})
	} else {
		receipt, err = svc.EditIssue(r.Context(), params)
	}
	writeCollaborationWorkflowResponse(w, receipt, err)
}
