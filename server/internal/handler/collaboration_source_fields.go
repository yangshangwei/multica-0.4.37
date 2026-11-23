package handler

import (
	"encoding/json"
	"net/http"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

func collaborationNullableUUID(w http.ResponseWriter, raw json.RawMessage, field string) (*pgtype.UUID, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	var value *string
	if json.Unmarshal(raw, &value) != nil {
		writeJSON(w, 400, map[string]string{"code": "invalid_request"})
		return nil, false
	}
	if value == nil {
		return &pgtype.UUID{}, true
	}
	id, ok := parseUUIDOrBadRequest(w, *value, field)
	if !ok {
		return nil, false
	}
	return &id, true
}

func collaborationParentSearchQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	values, exists := r.URL.Query()["parent_search"]
	if !exists {
		return "", true
	}
	if len(values) != 1 || utf8.RuneCountInString(values[0]) > 200 {
		writeJSON(w, 400, map[string]string{"code": "invalid_request"})
		return "", false
	}
	return values[0], true
}

func (h *Handler) UpdateCollaborationIteration(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		collaborationWorkflowBody
		IterationID               json.RawMessage `json:"current_iteration_id"`
		ExpectedRevision          string          `json:"expected_revision"`
		ExpectedIterationRevision string          `json:"expected_iteration_revision"`
		Reason                    string          `json:"iteration_reason"`
		AllowCompleted            json.RawMessage `json:"allow_completed"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	target, ok := collaborationNullableUUID(w, body.IterationID, "current_iteration_id")
	if !ok {
		return
	}
	issueRevision, valid := collaborationPositiveVersion(body.ExpectedRevision)
	if target == nil || !valid {
		writeJSON(w, 400, map[string]string{"code": "invalid_request"})
		return
	}
	p := service.CollaborationIterationParams{CollaborationWorkflowRequest: request, CurrentIterationID: *target, ExpectedIssueRevision: issueRevision, Reason: body.Reason}
	if len(body.AllowCompleted) > 0 && (string(body.AllowCompleted) == "null" || json.Unmarshal(body.AllowCompleted, &p.AllowCompleted) != nil) {
		writeJSON(w, 400, map[string]string{"code": "invalid_request"})
		return
	}
	if target.Valid {
		p.ExpectedIterationRevision, valid = collaborationPositiveVersion(body.ExpectedIterationRevision)
		if !valid {
			writeJSON(w, 400, map[string]string{"code": "invalid_request"})
			return
		}
	} else if body.ExpectedIterationRevision != "" {
		writeJSON(w, 400, map[string]string{"code": "invalid_request"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.ChangeIteration(r.Context(), p)
	writeCollaborationWorkflowResponse(w, receipt, err)
}
