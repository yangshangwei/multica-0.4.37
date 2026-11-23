package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) ListCollaborationDeliveryCandidates(w http.ResponseWriter, r *http.Request) {
	h.collaborationCompletionRead(w, r, false, false)
}
func (h *Handler) GetCollaborationCompletionPreview(w http.ResponseWriter, r *http.Request) {
	h.collaborationCompletionRead(w, r, true, false)
}
func (h *Handler) ListAdminCollaborationDeliveryCandidates(w http.ResponseWriter, r *http.Request) {
	h.collaborationCompletionRead(w, r, false, true)
}
func (h *Handler) GetAdminCollaborationCompletionPreview(w http.ResponseWriter, r *http.Request) {
	h.collaborationCompletionRead(w, r, true, true)
}
func (h *Handler) collaborationCompletionRead(w http.ResponseWriter, r *http.Request, preview, admin bool) {
	prepareCollaborationContentRequest(w, r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	root, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	var org pgtype.UUID
	if admin {
		identity, ok := h.requirePlatformAccess(w, r, true)
		if !ok {
			return
		}
		org = identity.OrganizationID
	}
	credential, err := h.completionAccess(r)
	if err != nil {
		writeCollaborationCompletionReadError(w, err)
		return
	}
	var request service.CollaborationCompletionPreviewRequest
	var source pgtype.UUID
	var limit int32
	var cursor string
	invalid := service.DeliveryCompletionError{Status: 400, Code: "invalid_delivery_completion_request"}
	if preview {
		if r.URL.RawQuery != "" {
			writeCollaborationCompletionReadError(w, invalid)
			return
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
		if e != nil {
			writeCollaborationCompletionReadError(w, invalid)
			return
		}
		request, err = service.ParseCollaborationCompletionPreview(raw)
	} else {
		var values url.Values
		values, err = url.ParseQuery(r.URL.RawQuery)
		if err == nil && len(values["source_issue_id"]) == 1 {
			source, ok = stageUUIDField(values.Get("source_issue_id"))
			if !ok {
				err = invalid
			}
			values.Del("source_issue_id")
			if err == nil {
				limit, cursor, err = deliveryCandidatesQuery(values.Encode())
			}
		} else {
			err = invalid
		}
	}
	if err != nil {
		writeCollaborationCompletionReadError(w, err)
		return
	}
	tx, _, err := h.deliveryPolicyTx(ctx)
	if err != nil {
		writeCollaborationCompletionReadError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var out any
	if preview {
		out, err = h.stageService().GetCollaborationCompletionPreviewInTx(ctx, tx, root, org, credential.Credential, request)
	} else {
		out, err = h.stageService().ListCollaborationDeliveryCandidatesInTx(ctx, tx, root, source, org, credential.Credential, limit, cursor)
	}
	if err == nil {
		err = service.CheckArtifactCredentialInTx(ctx, tx, credential.Credential)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeCollaborationCompletionReadError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func writeCollaborationCompletionReadError(w http.ResponseWriter, err error) {
	var domain *service.CollaborationError
	var delivery service.DeliveryCompletionError
	var artifact service.ArtifactError
	switch {
	case errors.Is(err, pgx.ErrNoRows), errors.As(err, &domain) && domain.Status == 404, errors.As(err, &delivery) && delivery.Status == 404, errors.As(err, &artifact) && artifact.Status == 403:
		writeJSON(w, 404, map[string]string{"code": "not_found", "error": "not_found"})
	case errors.As(err, &domain):
		writeCollaborationError(w, err)
	default:
		writeDeliveryCompletionError(w, err, false)
	}
}
