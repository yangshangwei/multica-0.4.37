package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

func informationBodyFields(raw json.RawMessage, fields ...string) (map[string]json.RawMessage, bool) {
	if iteration.ValidateObjectJSON(raw) != nil {
		return nil, false
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil || len(value) != len(fields) {
		return nil, false
	}
	for _, field := range fields {
		if _, present := value[field]; !present {
			return nil, false
		}
	}
	return value, true
}

func informationBodyUUID(raw json.RawMessage) (pgtype.UUID, bool) {
	var text string
	if json.Unmarshal(raw, &text) != nil || text == "" {
		return pgtype.UUID{}, false
	}
	id, err := util.ParseUUID(text)
	return id, err == nil && id.Valid && id.Bytes != [16]byte{} && util.UUIDToString(id) == text
}

func decodeInformationRepairSelection(raw json.RawMessage) (service.CollaborationInformationRepairSelection, bool) {
	var selection service.CollaborationInformationRepairSelection
	fields, valid := informationBodyFields(raw, "context_version", "active_request_id", "basis")
	if !valid || json.Unmarshal(fields["context_version"], &selection.ContextVersion) != nil || len(selection.ContextVersion) != 64 {
		return selection, false
	}
	for _, digit := range selection.ContextVersion {
		if digit < '0' || digit > '9' && digit < 'a' || digit > 'f' {
			return selection, false
		}
	}
	if !bytes.Equal(bytes.TrimSpace(fields["active_request_id"]), []byte("null")) {
		selection.ActiveRequestID, valid = informationBodyUUID(fields["active_request_id"])
		if !valid {
			return selection, false
		}
	}
	var kind struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(fields["basis"], &kind) != nil {
		return selection, false
	}
	selection.Basis.Kind = kind.Kind
	var basis map[string]json.RawMessage
	switch kind.Kind {
	case "human_selected":
		_, valid = informationBodyFields(fields["basis"], "kind")
	case "historical_information":
		basis, valid = informationBodyFields(fields["basis"], "kind", "origin_request_id", "origin_event_id")
		if valid {
			request, okRequest := informationBodyUUID(basis["origin_request_id"])
			event, okEvent := informationBodyUUID(basis["origin_event_id"])
			selection.Basis.OriginRequestID, selection.Basis.OriginEventID = &request, &event
			valid = okRequest && okEvent
		}
	case "pending_submission":
		basis, valid = informationBodyFields(fields["basis"], "kind", "submission_id", "submission_event_id")
		if valid {
			submission, okSubmission := informationBodyUUID(basis["submission_id"])
			event, okEvent := informationBodyUUID(basis["submission_event_id"])
			selection.Basis.SubmissionID, selection.Basis.SubmissionEventID = &submission, &event
			valid = okSubmission && okEvent
		}
	default:
		valid = false
	}
	return selection, valid
}

func informationMetadataSelection(w http.ResponseWriter, body collaborationHandoffBody) (*string, *string, bool) {
	if len(body.UnblockCondition) == 0 && len(body.EstimatedResolutionAt) == 0 {
		return nil, nil, true
	}
	var condition, estimate string
	if len(body.UnblockCondition) == 0 || len(body.EstimatedResolutionAt) == 0 ||
		bytes.Equal(bytes.TrimSpace(body.UnblockCondition), []byte("null")) || bytes.Equal(bytes.TrimSpace(body.EstimatedResolutionAt), []byte("null")) ||
		json.Unmarshal(body.UnblockCondition, &condition) != nil || json.Unmarshal(body.EstimatedResolutionAt, &estimate) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "information_metadata_invalid", "error": "Invalid information metadata"})
		return nil, nil, false
	}
	return &condition, &estimate, true
}

func validInformationHandoffRaw(raw []byte, body *collaborationHandoffBody) bool {
	if body.Purpose != "information_repair" && len(body.UnblockCondition) == 0 && len(body.EstimatedResolutionAt) == 0 && len(body.InformationRepair) == 0 {
		return true
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	allowed := map[string]bool{"purpose": true, "note": true, "target_user_id": true, "expected_collaboration_revision": true,
		"expected_lifecycle_epoch": true, "expected_participant_epoch": true, "expected_account_epoch": true,
		"expected_status": true, "status_options_version": true, "unblock_condition": true,
		"estimated_resolution_at": true, "information_repair": true}
	for field := range fields {
		if !allowed[field] {
			return false
		}
	}
	if body.Purpose == "information_repair" {
		_, valid := informationBodyFields(raw, "purpose", "note", "target_user_id", "expected_collaboration_revision",
			"expected_lifecycle_epoch", "expected_participant_epoch", "expected_account_epoch", "expected_status",
			"status_options_version", "information_repair")
		if !valid {
			return false
		}
		for _, name := range []string{"purpose", "note", "target_user_id", "expected_collaboration_revision", "expected_lifecycle_epoch",
			"expected_participant_epoch", "expected_account_epoch", "expected_status", "status_options_version"} {
			var text string
			if bytes.Equal(bytes.TrimSpace(fields[name]), []byte("null")) || json.Unmarshal(fields[name], &text) != nil {
				return false
			}
		}
		return true
	}
	return true
}

func (h *Handler) informationReadRequest(w http.ResponseWriter, r *http.Request) (context.Context, context.CancelFunc, pgtype.UUID, *service.CollaborationService, bool) {
	prepareCollaborationContentRequest(w, r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	id, valid := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !valid {
		cancel()
		return nil, nil, pgtype.UUID{}, nil, false
	}
	svc := h.stageService()
	if _, err := svc.ResolveIssueAccess(ctx, nil, id); err != nil {
		writeCollaborationError(w, err)
		cancel()
		return nil, nil, pgtype.UUID{}, nil, false
	}
	return ctx, cancel, id, svc, true
}

func informationHistoryQuery(w http.ResponseWriter, r *http.Request) (service.CollaborationInformationHistoryQuery, bool) {
	out := service.CollaborationInformationHistoryQuery{Limit: 50}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_information_query", "error": "Invalid information query"})
		return out, false
	}
	for name, values := range query {
		if (name != "limit" && name != "cursor") || len(values) != 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_information_query", "error": "Invalid information query"})
			return out, false
		}
	}
	if values, present := query["limit"]; present {
		value, err := strconv.Atoi(values[0])
		if err != nil || value < 1 || value > 50 || strconv.Itoa(value) != values[0] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_information_query", "error": "Invalid information query"})
			return out, false
		}
		out.Limit = value
	}
	out.Cursor = query.Get("cursor")
	if len(out.Cursor) > 2048 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_information_query", "error": "Invalid information query"})
		return out, false
	}
	return out, true
}

func (h *Handler) ListCollaborationInformationHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, id, svc, valid := h.informationReadRequest(w, r)
	if !valid {
		return
	}
	defer cancel()
	query, valid := informationHistoryQuery(w, r)
	if !valid {
		return
	}
	page, err := svc.ListInformationHistory(ctx, id, query)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) GetCollaborationInformationHandoff(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, id, svc, valid := h.informationReadRequest(w, r)
	if !valid {
		return
	}
	defer cancel()
	if query, err := url.ParseQuery(r.URL.RawQuery); err != nil || len(query) != 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_information_query", "error": "Invalid information query"})
		return
	}
	handoff, valid := parseUUIDOrBadRequest(w, chi.URLParam(r, "handoffID"), "handoff_id")
	if !valid {
		return
	}
	page, err := svc.InformationHistoryDetail(ctx, id, handoff)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) ListCollaborationInformationRepairAffected(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, id, svc, valid := h.informationReadRequest(w, r)
	if !valid {
		return
	}
	defer cancel()
	handoff, valid := parseUUIDOrBadRequest(w, chi.URLParam(r, "handoffID"), "handoff_id")
	if !valid {
		return
	}
	query, valid := informationHistoryQuery(w, r)
	if !valid {
		return
	}
	page, err := svc.ListInformationRepairAffected(ctx, id, handoff, query)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
