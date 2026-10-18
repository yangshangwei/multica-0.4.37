package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationCreateIssueBody struct {
	SourceWorkspaceID string   `json:"source_workspace_id"`
	HandlerUserID     *string  `json:"handler_user_id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	ProjectID         *string  `json:"project_id"`
	ParentIssueID     *string  `json:"parent_issue_id"`
	DraftID           *string  `json:"draft_id"`
	AttachmentIDs     []string `json:"attachment_ids"`
}

func (h *Handler) CreateCollaborationIssue(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	principal, human := collaboration.PrincipalFromContext(r.Context())
	if !human || !principal.Human() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "forbidden", "error": "forbidden"})
		return
	}
	var body collaborationCreateIssueBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	p := service.CollaborationCreateParams{Title: body.Title, Description: body.Description, RequestID: r.Header.Get("Idempotency-Key")}
	handler := principal.UserID
	if body.HandlerUserID != nil {
		handler = *body.HandlerUserID
	}
	for _, field := range []struct {
		name string
		in   *string
		out  *pgtype.UUID
	}{
		{"source_workspace_id", &body.SourceWorkspaceID, &p.SourceWorkspaceID},
		{"handler_user_id", &handler, &p.HandlerID},
		{"project_id", body.ProjectID, &p.ProjectID},
		{"parent_issue_id", body.ParentIssueID, &p.ParentIssueID},
	} {
		if field.in == nil {
			continue
		}
		parsed, ok := parseUUIDOrBadRequest(w, *field.in, field.name)
		if !ok {
			return
		}
		*field.out = parsed
	}
	var ok bool
	p.DraftID, p.AttachmentIDs, ok = collaborationAttachmentSelection(w, body.DraftID, body.AttachmentIDs)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	access, err := svc.Create(r.Context(), p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	out, err := h.collaborationIssueResponse(r.Context(), access)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) GetCollaborationCreationOptions(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var source pgtype.UUID
	if values, present := r.URL.Query()["source_workspace_id"]; present {
		if len(values) != 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "error": "invalid_request"})
			return
		}
		var ok bool
		source, ok = parseUUIDOrBadRequest(w, values[0], "source_workspace_id")
		if !ok {
			return
		}
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.CreationOptions(r.Context(), source)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetCollaborationCreationResult(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	values := r.URL.Query()["source_workspace_id"]
	if len(values) != 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_request", "error": "invalid_request"})
		return
	}
	source, ok := parseUUIDOrBadRequest(w, values[0], "source_workspace_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.GetCreationResult(r.Context(), source, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
