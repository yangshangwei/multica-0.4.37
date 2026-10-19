package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationWorkflowBody struct {
	ExpectedCollaborationRevision string `json:"expected_collaboration_revision"`
	ExpectedLifecycleEpoch        string `json:"expected_lifecycle_epoch"`
	ExpectedParticipantEpoch      string `json:"expected_participant_epoch"`
	ExpectedAccountEpoch          string `json:"expected_account_epoch"`
}

type collaborationParticipantBody struct {
	collaborationWorkflowBody
	TargetUserID string `json:"target_user_id"`
}

type collaborationHandoffBody struct {
	collaborationParticipantBody
	// Purpose is required: an ordinary transfer and an information request
	// have different responsibility and status effects.
	Purpose string `json:"purpose"`
	Note    string `json:"note"`
	// Raw fields distinguish absent values from explicit null on transfers.
	ExpectedStatus       json.RawMessage `json:"expected_status"`
	StatusOptionsVersion json.RawMessage `json:"status_options_version"`
}

type collaborationInformationReturnBody struct {
	collaborationWorkflowBody
	InformationRequestID string `json:"information_request_id"`
	ExpectedStatus       string `json:"expected_status"`
	StatusOptionsVersion string `json:"status_options_version"`
	Note                 string `json:"note"`
}

type collaborationAcceptanceBody struct {
	collaborationWorkflowBody
	ExpectedStatus       string `json:"expected_status"`
	StatusOptionsVersion string `json:"status_options_version"`
	Note                 string `json:"note"`
}

type collaborationOperationResponse struct {
	Code                  string                              `json:"code"`
	CollaborationRevision string                              `json:"collaboration_revision"`
	StatusOptions         *service.CollaborationStatusOptions `json:"status_options,omitempty"`
}

func collaborationPositiveVersion(value string) (int64, bool) {
	if len(value) == 0 || len(value) > 19 || value[0] < '1' || value[0] > '9' {
		return 0, false
	}
	for _, digit := range []byte(value) {
		if digit < '0' || digit > '9' {
			return 0, false
		}
	}
	version, err := strconv.ParseInt(value, 10, 64)
	return version, err == nil
}

func decodeCollaborationWorkflowBody(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeCollaborationWorkflowBodyLimit(w, r, target, 64<<10)
}

func decodeCollaborationWorkflowBodyLimit(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"code": "json_required", "error": "JSON request required"})
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": "request_too_large", "error": "Request too large"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "error": "Invalid request"})
		}
		return false
	}
	// Reuse the strict object validator: duplicate keys and trailing JSON must
	// not give the version/idempotency digest a different meaning to a caller.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if iteration.ValidateObjectJSON(raw) != nil || decoder.Decode(target) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "error": "Invalid request"})
		return false
	}
	return true
}

func collaborationWorkflowRequest(w http.ResponseWriter, r *http.Request, body collaborationWorkflowBody) (service.CollaborationWorkflowRequest, bool) {
	var out service.CollaborationWorkflowRequest
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return out, false
	}
	out.IssueID, out.RequestID = id, r.Header.Get("Idempotency-Key")
	values := []struct {
		raw string
		out *int64
	}{
		{body.ExpectedCollaborationRevision, &out.ExpectedCollaborationRevision},
		{body.ExpectedLifecycleEpoch, &out.ExpectedLifecycleEpoch},
		{body.ExpectedParticipantEpoch, &out.ExpectedParticipantEpoch},
		{body.ExpectedAccountEpoch, &out.ExpectedAccountEpoch},
	}
	for _, value := range values {
		parsed, valid := collaborationPositiveVersion(value.raw)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid collaboration version"})
			return out, false
		}
		*value.out = parsed
	}
	if len(out.RequestID) == 0 || len(out.RequestID) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request_id", "error": "Idempotency key required"})
		return out, false
	}
	return out, true
}

func (h *Handler) InviteCollaborationParticipant(w http.ResponseWriter, r *http.Request) {
	h.mutateCollaborationParticipants(w, r, "invite")
}

func (h *Handler) TransferCollaborationIssue(w http.ResponseWriter, r *http.Request) {
	h.mutateCollaborationParticipants(w, r, "transfer")
}

func (h *Handler) RemoveCollaborationParticipant(w http.ResponseWriter, r *http.Request) {
	h.mutateCollaborationParticipants(w, r, "remove")
}

func (h *Handler) LeaveCollaborationIssue(w http.ResponseWriter, r *http.Request) {
	h.mutateCollaborationParticipants(w, r, "leave")
}

func (h *Handler) mutateCollaborationParticipants(w http.ResponseWriter, r *http.Request, action string) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	var base collaborationWorkflowBody
	var target, note, purpose, expectedStatus, statusOptionsVersion string
	switch action {
	case "invite":
		var body collaborationParticipantBody
		if !decodeCollaborationWorkflowBody(w, r, &body) {
			return
		}
		base, target = body.collaborationWorkflowBody, body.TargetUserID
	case "transfer":
		var body collaborationHandoffBody
		if !decodeCollaborationWorkflowBody(w, r, &body) {
			return
		}
		if body.Purpose != "transfer" && body.Purpose != "information" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_handoff_purpose", "error": "Invalid handoff purpose"})
			return
		}
		if body.Purpose == "transfer" && (len(body.ExpectedStatus) > 0 || len(body.StatusOptionsVersion) > 0) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "error": "Transfer does not accept status fields"})
			return
		}
		if (len(body.ExpectedStatus) > 0 && json.Unmarshal(body.ExpectedStatus, &expectedStatus) != nil) ||
			(len(body.StatusOptionsVersion) > 0 && json.Unmarshal(body.StatusOptionsVersion, &statusOptionsVersion) != nil) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_status_request", "error": "Invalid status confirmation"})
			return
		}
		base, target, note, purpose = body.collaborationWorkflowBody, body.TargetUserID, body.Note, body.Purpose
	case "remove", "leave":
		if !decodeCollaborationWorkflowBody(w, r, &base) {
			return
		}
		if action == "remove" {
			target = chi.URLParam(r, "userID")
		}
	}
	request, ok := collaborationWorkflowRequest(w, r, base)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	var receipt service.CollaborationWorkflowReceipt
	var err error
	if action == "leave" {
		receipt, err = svc.Leave(r.Context(), request)
	} else {
		userID, ok := parseUUIDOrBadRequest(w, target, "target_user_id")
		if !ok {
			return
		}
		params := service.CollaborationParticipantParams{CollaborationWorkflowRequest: request, TargetUserID: userID}
		switch action {
		case "invite":
			receipt, err = svc.InviteParticipant(r.Context(), params)
		case "transfer":
			handoff := service.CollaborationTransferParams{CollaborationParticipantParams: params, Note: note}
			if purpose == "information" {
				receipt, err = svc.RequestInformation(r.Context(), service.CollaborationInformationRequestParams{
					CollaborationTransferParams: handoff, ExpectedStatus: expectedStatus, StatusOptionsVersion: statusOptionsVersion})
			} else {
				receipt, err = svc.Transfer(r.Context(), handoff)
			}
		case "remove":
			receipt, err = svc.RemoveParticipant(r.Context(), params)
		}
	}
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func (h *Handler) ReturnCollaborationInformation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	var body collaborationInformationReturnBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	requestID, ok := parseUUIDOrBadRequest(w, body.InformationRequestID, "information_request_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.ReturnInformation(r.Context(), service.CollaborationInformationReturnParams{CollaborationWorkflowRequest: request,
		InformationRequestID: requestID, ExpectedStatus: body.ExpectedStatus, StatusOptionsVersion: body.StatusOptionsVersion, Note: body.Note})
	writeCollaborationWorkflowResponse(w, receipt, err)
}

func (h *Handler) SubmitCollaborationAcceptance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	var body collaborationAcceptanceBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.SubmitAcceptance(r.Context(), service.CollaborationAcceptanceParams{CollaborationWorkflowRequest: request,
		ExpectedStatus: body.ExpectedStatus, StatusOptionsVersion: body.StatusOptionsVersion, Note: body.Note})
	writeCollaborationWorkflowResponse(w, receipt, err)
}

// Existing workflow replies stay minimal; status mutations may additionally
// supply current, action-authorized choices after a catalog conflict.
func writeCollaborationWorkflowResponse(w http.ResponseWriter, receipt service.CollaborationWorkflowReceipt, err error) {
	if err != nil {
		if receipt.Code != "" && receipt.CollaborationRevision > 0 && receipt.Status >= 400 {
			body := map[string]any{"code": receipt.Code, "error": receipt.Code,
				"collaboration_revision": strconv.FormatInt(receipt.CollaborationRevision, 10)}
			if receipt.StatusOptions != nil {
				body["status_options"] = receipt.StatusOptions
			}
			writeJSON(w, receipt.Status, body)
		} else {
			writeCollaborationError(w, err)
		}
		return
	}
	writeJSON(w, receipt.Status, collaborationOperationResponse{Code: receipt.Code, CollaborationRevision: strconv.FormatInt(receipt.CollaborationRevision, 10), StatusOptions: receipt.StatusOptions})
}

func (h *Handler) GetCollaborationOperation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.GetWorkflowOperation(r.Context(), id, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collaborationOperationResponse{Code: receipt.Code, CollaborationRevision: strconv.FormatInt(receipt.CollaborationRevision, 10)})
}
