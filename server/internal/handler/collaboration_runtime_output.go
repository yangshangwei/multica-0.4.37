package handler

import (
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/multica-ai/multica/server/internal/service"
)

// One file and the original comment UUID are a single run-bound operation.
// The mcb resolves Agent/run/attempt; no human or draft fields are accepted.
func (h *Handler) CreateCollaborationRuntimeCommentAttachment(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	if run.Bytes == [16]byte{} {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_upload", "error": "invalid_attachment_upload"})
		return
	}
	release, ok := acquireCollaborationAttachmentBytes(w, r)
	if !ok {
		return
	}
	defer release()
	invalid := func() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_upload", "error": "invalid_attachment_upload"})
	}
	tooLarge := func() {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": "attachment_too_large", "error": "attachment_too_large"})
	}
	r.Body = http.MaxBytesReader(w, r.Body, service.CollaborationAttachmentMaxBytes+service.CollaborationCommentMaxBytes+(64<<10))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			tooLarge()
		} else {
			invalid()
		}
		return
	}
	defer r.MultipartForm.RemoveAll()
	for key, values := range r.MultipartForm.Value {
		limit := 128
		if key == "content" {
			limit = service.CollaborationCommentMaxBytes
		}
		if (key != "comment_id" && key != "content" && key != "parent_id") || len(values) != 1 || len(values[0]) > limit {
			invalid()
			return
		}
	}
	if len(r.MultipartForm.Value["comment_id"]) != 1 || len(r.MultipartForm.Value["content"]) != 1 || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		invalid()
		return
	}
	input := service.CollaborationRuntimeCommentAttachmentParams{CollaborationRuntimeCommentParams: service.CollaborationRuntimeCommentParams{Content: r.MultipartForm.Value["content"][0]}}
	input.CommentID, ok = parseUUIDOrBadRequest(w, r.MultipartForm.Value["comment_id"][0], "comment_id")
	if !ok {
		return
	}
	if input.CommentID.Bytes == [16]byte{} {
		invalid()
		return
	}
	if values, present := r.MultipartForm.Value["parent_id"]; present {
		input.ParentID, ok = parseUUIDOrBadRequest(w, values[0], "parent_id")
		if !ok {
			return
		}
		if input.ParentID.Bytes == [16]byte{} {
			invalid()
			return
		}
	}
	fileHeader := r.MultipartForm.File["file"][0]
	if fileHeader.Size > service.CollaborationAttachmentMaxBytes {
		tooLarge()
		return
	}
	input.Name = path.Base(strings.ReplaceAll(fileHeader.Filename, "\\", "/"))
	file, err := fileHeader.Open()
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	defer file.Close()
	input.Data, err = io.ReadAll(io.LimitReader(file, service.CollaborationAttachmentMaxBytes+1))
	if len(input.Data) > service.CollaborationAttachmentMaxBytes {
		tooLarge()
		return
	}
	if err != nil || int64(len(input.Data)) != fileHeader.Size {
		invalid()
		return
	}
	result, err := h.collaborationAttachmentService().CreateRuntimeCommentAttachment(r.Context(), run, token, input)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
