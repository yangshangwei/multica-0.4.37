package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationContentCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func prepareCollaborationContentRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
}

func collaborationContentPagination(w http.ResponseWriter, r *http.Request, defaultLimit int) (int, pgtype.Timestamptz, pgtype.UUID, bool) {
	limit := defaultLimit
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_limit", "error": "Invalid page limit"})
			return 0, pgtype.Timestamptz{}, pgtype.UUID{}, false
		}
		limit = parsed
	}
	var before pgtype.Timestamptz
	var id pgtype.UUID
	if value := r.URL.Query().Get("cursor"); value != "" {
		if len(value) > 512 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_cursor", "error": "Invalid page cursor"})
			return 0, before, id, false
		}
		data, err := base64.RawURLEncoding.DecodeString(value)
		var cursor collaborationContentCursor
		if err != nil || iteration.ValidateObjectJSON(data) != nil || json.Unmarshal(data, &cursor) != nil || cursor.CreatedAt.IsZero() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_cursor", "error": "Invalid page cursor"})
			return 0, before, id, false
		}
		var ok bool
		id, ok = parseUUIDOrBadRequest(w, cursor.ID, "cursor_id")
		if !ok {
			return 0, before, id, false
		}
		before = pgtype.Timestamptz{Time: cursor.CreatedAt, Valid: true}
	}
	return limit, before, id, true
}

func collaborationNextContentCursor(at time.Time, id pgtype.UUID) (*string, error) {
	data, err := json.Marshal(collaborationContentCursor{CreatedAt: at, ID: uuidToString(id)})
	if err != nil {
		return nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(data)
	return &value, nil
}

func (h *Handler) ListCollaborationComments(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	limit, before, beforeID, ok := collaborationContentPagination(w, r, 100)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.ListComments(r.Context(), id, service.CollaborationCommentsQuery{Limit: limit, BeforeAt: before, BeforeID: beforeID})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	var next *string
	if page.HasMore && len(page.Items) > 0 {
		next, err = collaborationNextContentCursor(page.Items[0].CreatedAt, page.Items[0].ID)
		if err != nil {
			writeCollaborationError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"issue_id": id, "items": page.Items, "next_cursor": next})
}

func (h *Handler) GetCollaborationComment(w http.ResponseWriter, r *http.Request) {
	h.readCollaborationComment(w, r, false)
}

func (h *Handler) GetCollaborationCommentHistory(w http.ResponseWriter, r *http.Request) {
	h.readCollaborationComment(w, r, true)
}

func (h *Handler) readCollaborationComment(w http.ResponseWriter, r *http.Request, history bool) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	comment, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "commentId"), "comment_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	if history {
		items, err := svc.CommentHistory(r.Context(), issue, comment)
		if err != nil {
			writeCollaborationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"issue_id": issue, "comment_id": comment, "items": items})
		return
	}
	item, err := svc.GetComment(r.Context(), issue, comment)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) EditCollaborationComment(w http.ResponseWriter, r *http.Request) {
	h.changeCollaborationComment(w, r, false)
}

func (h *Handler) DeleteCollaborationComment(w http.ResponseWriter, r *http.Request) {
	h.changeCollaborationComment(w, r, true)
}

func (h *Handler) changeCollaborationComment(w http.ResponseWriter, r *http.Request, deleting bool) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		Content                 *string   `json:"content"`
		ExpectedCommentRevision string    `json:"expected_comment_revision"`
		DraftID                 *string   `json:"draft_id"`
		AttachmentIDs           *[]string `json:"attachment_ids"`
	}
	maxBodyBytes := int64(64 << 10)
	if !deleting {
		// Allow the JSON envelope and worst-case string escaping, including
		// original drafts from clients that counted 64K JavaScript code units.
		// The service still limits accepted comment text to 64KiB of UTF-8.
		maxBodyBytes = 512 << 10
	}
	if !decodeCollaborationWorkflowBodyLimit(w, r, &body, maxBodyBytes) {
		return
	}
	if (deleting && (body.Content != nil || body.DraftID != nil || body.AttachmentIDs != nil)) || (!deleting && body.Content == nil) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_comment_request", "error": "Invalid comment request"})
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	comment, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "commentId"), "comment_id")
	if !ok {
		return
	}
	version, ok := collaborationPositiveVersion(body.ExpectedCommentRevision)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid comment version"})
		return
	}
	p := service.CollaborationEditCommentParams{IssueID: issue, CommentID: comment, RequestID: r.Header.Get("Idempotency-Key"), ExpectedCommentRevision: version}
	if body.Content != nil {
		p.Content = *body.Content
	}
	if body.AttachmentIDs != nil {
		draft, ids, ok := collaborationAttachmentSelection(w, body.DraftID, *body.AttachmentIDs)
		if !ok {
			return
		}
		p.DraftID, p.AttachmentIDs = draft, &ids
	} else if body.DraftID != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_selection", "error": "invalid_attachment_selection"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	var receipt service.CollaborationCommentReceipt
	var err error
	if deleting {
		receipt, err = svc.DeleteComment(r.Context(), p)
	} else {
		receipt, err = svc.EditComment(r.Context(), p)
	}
	writeCollaborationCommentReceipt(w, receipt, err)
}

func writeCollaborationCommentReceipt(w http.ResponseWriter, receipt service.CollaborationCommentReceipt, err error) {
	if err != nil && receipt.Code == "" {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, receipt.Status, receipt)
}
