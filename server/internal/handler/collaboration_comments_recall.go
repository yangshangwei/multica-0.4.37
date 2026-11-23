package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func collaborationRecallBytes(w http.ResponseWriter, r *http.Request) (int, bool) {
	value := protocol.CollaborationRecallDefaultBytes
	if raw, ok := r.URL.Query()["max_bytes"]; ok {
		var err error
		if len(raw) != 1 {
			err = strconv.ErrSyntax
		} else {
			value, err = strconv.Atoi(raw[0])
		}
		if err != nil || value < protocol.CollaborationRecallMinBytes || value > protocol.CollaborationRecallMaxBytes {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_max_bytes", "error": "Invalid byte budget"})
			return 0, false
		}
	}
	return value, true
}

func collaborationCommentsRecallQuery(w http.ResponseWriter, r *http.Request) (service.CollaborationCommentsRecallQuery, bool) {
	p := service.CollaborationCommentsRecallQuery{Limit: protocol.CollaborationRecallDefaultLimit, Cursor: r.URL.Query().Get("cursor")}
	var ok bool
	p.MaxBytes, ok = collaborationRecallBytes(w, r)
	if !ok {
		return p, false
	}
	if raw, exists := r.URL.Query()["limit"]; exists {
		var err error
		if len(raw) != 1 {
			err = strconv.ErrSyntax
		} else {
			p.Limit, err = strconv.Atoi(raw[0])
		}
		if err != nil || p.Limit < 1 || p.Limit > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_limit", "error": "Invalid page limit"})
			return p, false
		}
	}
	if len(r.URL.Query()["cursor"]) > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_cursor", "error": "Invalid page cursor"})
		return p, false
	}
	return p, true
}

func (h *Handler) ListCollaborationRuntimeComments(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	query, ok := collaborationCommentsRecallQuery(w, r)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.RecallRuntimeComments(r.Context(), run, token, query)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) GetCollaborationRuntimeCommentContent(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	comment, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "commentId"), "comment_id")
	if !ok {
		return
	}
	budget, ok := collaborationRecallBytes(w, r)
	if !ok {
		return
	}
	if len(r.URL.Query()["revision"]) != 1 || len(r.URL.Query()["cursor"]) > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_comment_query", "error": "Invalid comment content query"})
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.RuntimeCommentContent(r.Context(), run, token, comment, service.CollaborationCommentContentQuery{Revision: r.URL.Query().Get("revision"), Cursor: r.URL.Query().Get("cursor"), MaxBytes: budget})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
