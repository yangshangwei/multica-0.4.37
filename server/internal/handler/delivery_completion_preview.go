package handler

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

// ListDeliveryEvidenceCandidates lists only metadata the current human may read.
func (h *Handler) ListDeliveryEvidenceCandidates(w http.ResponseWriter, r *http.Request) {
	h.deliveryCompletionPreviewRead(w, r, false)
}

// GetDeliveryCompletionPreview assembles a current, non-authorizing review.
func (h *Handler) GetDeliveryCompletionPreview(w http.ResponseWriter, r *http.Request) {
	h.deliveryCompletionPreviewRead(w, r, true)
}

func deliveryCandidatesQuery(raw string) (int32, string, error) {
	values, err := url.ParseQuery(raw)
	invalid := service.DeliveryCompletionError{Status: 400, Code: "invalid_delivery_completion_request"}
	if err != nil {
		return 0, "", invalid
	}
	limit := int64(20)
	for key, entries := range values {
		if (key != "limit" && key != "cursor") || len(entries) != 1 || entries[0] == "" {
			return 0, "", invalid
		}
	}
	if value := values.Get("limit"); value != "" {
		limit, err = strconv.ParseInt(value, 10, 32)
		if err != nil || limit < 1 || limit > 50 || strconv.FormatInt(limit, 10) != value {
			return 0, "", invalid
		}
	}
	if len(values.Get("cursor")) > 2048 {
		return 0, "", invalid
	}
	return int32(limit), values.Get("cursor"), nil
}

func (h *Handler) deliveryCompletionPreviewRead(w http.ResponseWriter, r *http.Request, selected bool) {
	w.Header().Set("Cache-Control", "no-store")
	// Bound candidate authority scans and their retained row locks as one read.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	access, err := h.completionAccess(r)
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	kind, _, err := access.Credential.Actor()
	if err != nil || kind != "human" {
		writeDeliveryCompletionError(w, service.DeliveryCompletionError{Status: 403, Code: "delivery_completion_denied"}, false)
		return
	}
	var request service.DeliveryCompletionPreviewRequest
	var limit int32
	var cursor string
	if selected {
		var raw []byte
		raw, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
		if err == nil {
			request, err = service.ParseDeliveryCompletionPreview(raw)
		} else {
			err = service.DeliveryCompletionError{Status: 400, Code: "invalid_delivery_completion_request"}
		}
	} else {
		limit, cursor, err = deliveryCandidatesQuery(r.URL.RawQuery)
	}
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	initial, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	tx, q, err := h.deliveryPolicyTx(ctx)
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	defer tx.Rollback(ctx)
	if err = completionWorkspaceSelection(r, q, initial.WorkspaceID); err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	access.RequestedWorkspaceID = uuidToString(initial.WorkspaceID)
	var out any
	if selected {
		out, err = service.GetDeliveryCompletionPreviewInTx(ctx, tx, access, initial, request)
	} else {
		out, err = service.ListDeliveryEvidenceCandidatesInTx(ctx, tx, access, initial, limit, cursor)
	}
	if err == nil {
		err = service.CheckArtifactCredentialInTx(ctx, tx, access.Credential)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeDeliveryCompletionError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
