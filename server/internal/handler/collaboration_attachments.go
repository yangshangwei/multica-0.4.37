package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/storage"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// File plaintext/ciphertext buffers are each bounded to 100 MiB. Acquire before
// reading multipart data or decrypting and retain until the response completes.
// This limits simultaneous private byte operations without an unbounded queue.
var collaborationAttachmentByteSlots = make(chan struct{}, 2)

func acquireCollaborationAttachmentBytes(w http.ResponseWriter, r *http.Request) (func(), bool) {
	if r.Context().Err() != nil {
		writeJSON(w, http.StatusRequestTimeout, map[string]string{"code": "request_cancelled", "error": "request_cancelled"})
		return nil, false
	}
	select {
	case collaborationAttachmentByteSlots <- struct{}{}:
		return func() { <-collaborationAttachmentByteSlots }, true
	default:
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "attachment_busy", "error": "attachment_busy"})
		return nil, false
	}
}

func (h *Handler) collaborationAttachmentService() *service.CollaborationService {
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	svc.Storage = h.Storage
	return svc
}

func collaborationAttachmentSelection(w http.ResponseWriter, rawDraft *string, rawIDs []string) (*pgtype.UUID, []pgtype.UUID, bool) {
	if len(rawIDs) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_selection", "error": "invalid_attachment_selection"})
		return nil, nil, false
	}
	var draft *pgtype.UUID
	if rawDraft != nil {
		id, ok := parseUUIDOrBadRequest(w, *rawDraft, "draft_id")
		if !ok {
			return nil, nil, false
		}
		draft = &id
	}
	ids := make([]pgtype.UUID, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, ok := parseUUIDOrBadRequest(w, raw, "attachment_id")
		if !ok {
			return nil, nil, false
		}
		ids = append(ids, id)
	}
	return draft, ids, true
}

func (h *Handler) UploadCollaborationAttachmentDraft(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	p, ok := collaboration.PrincipalFromContext(r.Context())
	if !ok || !p.Human() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "forbidden", "error": "forbidden"})
		return
	}
	release, ok := acquireCollaborationAttachmentBytes(w, r)
	if !ok {
		return
	}
	defer release()
	r.Body = http.MaxBytesReader(w, r.Body, service.CollaborationAttachmentMaxBytes+(64<<10))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": "attachment_too_large", "error": "attachment_too_large"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_upload", "error": "invalid_attachment_upload"})
		}
		return
	}
	defer r.MultipartForm.RemoveAll()
	for key, values := range r.MultipartForm.Value {
		if (key != "draft_id" && key != "source_workspace_id" && key != "issue_id") || len(values) != 1 || len(values[0]) > 128 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_upload", "error": "invalid_attachment_upload"})
			return
		}
	}
	files := r.MultipartForm.File["file"]
	if len(files) != 1 || len(r.MultipartForm.File) != 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_upload", "error": "invalid_attachment_upload"})
		return
	}
	if files[0].Size > service.CollaborationAttachmentMaxBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": "attachment_too_large", "error": "attachment_too_large"})
		return
	}
	input := service.CollaborationAttachmentUpload{RequestID: r.Header.Get("Idempotency-Key")}
	input.DraftID, ok = parseUUIDOrBadRequest(w, r.FormValue("draft_id"), "draft_id")
	if !ok {
		return
	}
	for _, field := range []struct {
		name string
		id   *pgtype.UUID
	}{{"source_workspace_id", &input.SourceWorkspaceID}, {"issue_id", &input.IssueID}} {
		if values, present := r.MultipartForm.Value[field.name]; present {
			id, valid := parseUUIDOrBadRequest(w, values[0], field.name)
			if !valid {
				return
			}
			*field.id = id
		}
	}
	file, err := files[0].Open()
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	defer file.Close()
	input.Data = make([]byte, int(files[0].Size))
	if _, err = io.ReadFull(file, input.Data); err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": "attachment_too_large", "error": "attachment_too_large"})
		return
	}
	input.Name = path.Base(strings.ReplaceAll(files[0].Filename, "\\", "/"))
	input.ContentType = http.DetectContentType(input.Data)
	if contentType := extContentTypes[strings.ToLower(path.Ext(input.Name))]; contentType != "" {
		input.ContentType = contentType
	}
	out, err := h.collaborationAttachmentService().UploadAttachmentDraft(r.Context(), input)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) ListCollaborationDraftAttachments(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	draft, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "draftId"), "draft_id")
	if !ok {
		return
	}
	out, err := h.collaborationAttachmentService().ListDraftAttachments(r.Context(), draft)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"draft_id": draft, "attachments": out})
}

func (h *Handler) GetCollaborationAttachmentUpload(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	draft, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "draftId"), "draft_id")
	if !ok {
		return
	}
	out, err := h.collaborationAttachmentService().GetAttachmentUpload(r.Context(), draft, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func collaborationAttachmentPath(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, pgtype.UUID, bool) {
	var issue, draft pgtype.UUID
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "attachmentId"), "attachment_id")
	if !ok {
		return issue, draft, id, false
	}
	if raw := chi.URLParam(r, "id"); raw != "" {
		issue, ok = parseUUIDOrBadRequest(w, raw, "issue_id")
	} else {
		draft, ok = parseUUIDOrBadRequest(w, chi.URLParam(r, "draftId"), "draft_id")
	}
	return issue, draft, id, ok
}

func (h *Handler) GetCollaborationAttachment(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, draft, id, ok := collaborationAttachmentPath(w, r)
	if !ok {
		return
	}
	row, err := h.collaborationAttachmentService().GetAttachment(r.Context(), issue, draft, id)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service.CollaborationAttachmentMetadata(row))
}

func (h *Handler) ListCollaborationAttachments(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	out, err := h.collaborationAttachmentService().ListAttachments(r.Context(), issue)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issue_id": issue, "attachments": out})
}

func (h *Handler) DiscardCollaborationAttachmentDraft(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	_, draft, id, ok := collaborationAttachmentPath(w, r)
	if !ok {
		return
	}
	if err := h.collaborationAttachmentService().DiscardAttachmentDraft(r.Context(), draft, id); err != nil {
		writeCollaborationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeliverCollaborationAttachments(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		DraftID       *string  `json:"draft_id"`
		AttachmentIDs []string `json:"attachment_ids"`
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	draft, ids, ok := collaborationAttachmentSelection(w, body.DraftID, body.AttachmentIDs)
	if !ok {
		return
	}
	receipt, err := h.collaborationAttachmentService().DeliverAttachments(r.Context(), service.CollaborationAttachmentDelivery{IssueID: issue,
		DraftID: draft, AttachmentIDs: ids, RequestID: r.Header.Get("Idempotency-Key")})
	writeCollaborationCommentReceipt(w, receipt, err)
}

func (h *Handler) GetCollaborationAttachmentDeliveryOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	receipt, err := h.collaborationAttachmentService().GetAttachmentDeliveryOperation(r.Context(), issue, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

func (h *Handler) GetCollaborationAttachmentContent(w http.ResponseWriter, r *http.Request) {
	h.serveCollaborationAttachment(w, r, false)
}

func (h *Handler) DownloadCollaborationAttachment(w http.ResponseWriter, r *http.Request) {
	h.serveCollaborationAttachment(w, r, true)
}

func (h *Handler) serveCollaborationAttachment(w http.ResponseWriter, r *http.Request, download bool) {
	prepareCollaborationContentRequest(w, r)
	issue, draft, id, ok := collaborationAttachmentPath(w, r)
	if !ok {
		return
	}
	svc := h.collaborationAttachmentService()
	row, err := svc.GetAttachment(r.Context(), issue, draft, id)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	h.serveCollaborationAttachmentBytes(w, r, svc, row, download, func(ctx context.Context) error {
		_, err := svc.GetAttachment(ctx, issue, draft, id)
		return err
	})
}

func (h *Handler) serveCollaborationAttachmentBytes(w http.ResponseWriter, r *http.Request, svc *service.CollaborationService, row db.CollaborationAttachment, download bool, reauthorize func(context.Context) error) {
	if !download && isTextPreviewable(row.ContentType, row.Name) && row.SizeBytes > maxPreviewTextSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": "preview_too_large", "error": "preview_too_large"})
		return
	}
	release, ok := acquireCollaborationAttachmentBytes(w, r)
	if !ok {
		return
	}
	defer release()
	data, err := svc.ReadAttachmentBytes(r.Context(), row)
	if err == nil {
		// A large/slow object read may cross revocation. Check again immediately
		// before sending the first byte rather than reuse the earlier snapshot.
		err = reauthorize(r.Context())
	}
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	h.setAttachmentPreviewSecurityHeaders(w)
	w.Header().Set("Content-Security-Policy", w.Header().Get("Content-Security-Policy")+"; sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", row.ContentType)
	if download {
		w.Header().Set("Content-Disposition", storage.AttachmentContentDisposition(row.Name))
	} else {
		w.Header().Set("Content-Disposition", storage.ContentDisposition(row.ContentType, row.Name))
	}
	http.ServeContent(collaborationAttachmentNoStoreWriter{w}, r, row.Name, time.Time{}, bytes.NewReader(data))
}

type collaborationAttachmentNoStoreWriter struct{ http.ResponseWriter }

func (w collaborationAttachmentNoStoreWriter) WriteHeader(status int) {
	// ServeContent removes cache headers on Range errors; private errors are
	// also uncacheable, including 416 after a previous successful response.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.ResponseWriter.WriteHeader(status)
}
