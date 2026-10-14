package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const collaborationReferencePredicate = "EXISTS (SELECT 1 FROM issue_collaboration reference WHERE reference.issue_id = i.id)"

func collaborationMachineRequest(r *http.Request) bool {
	principal, ok := collaboration.PrincipalFromContext(r.Context())
	return !ok || !principal.Human()
}

// A source-side machine sees identity only, never a normal executable Issue DTO.
type collaborationIssueReference struct {
	ID                     string `json:"id"`
	WorkspaceID            string `json:"workspace_id"`
	Number                 int32  `json:"number"`
	Identifier             string `json:"identifier"`
	Title                  string `json:"title"`
	CollaborationReference bool   `json:"collaboration_reference"`
	CanOpen                bool   `json:"can_open"`
}

func collaborationReferenceResponse(id, workspaceID pgtype.UUID, number int32, title, prefix string) collaborationIssueReference {
	return collaborationIssueReference{ID: uuidToString(id), WorkspaceID: uuidToString(workspaceID), Number: number,
		Identifier: fmt.Sprintf("%s-%d", prefix, number), Title: title, CollaborationReference: true}
}

// Only identity filters may include collaboration references. Private filters
// must exclude them before the SQL window/count, even if their values are hidden.
func collaborationReferenceFiltersAllowed(r *http.Request) bool {
	for key, values := range r.URL.Query() {
		switch key {
		case "q", "limit", "offset", "sort", "direction", "open_only":
			continue
		}
		for _, value := range values {
			if value != "" {
				return false
			}
		}
	}
	return true
}

func writeUnsupportedCollaborationAction(w http.ResponseWriter) {
	writeJSON(w, http.StatusConflict, map[string]string{"error": "This action is not available for a collaboration task.", "code": "unsupported_collaboration_action"})
}

func (h *Handler) rejectLegacyCollaborationAccess(w http.ResponseWriter, r *http.Request, issue db.Issue) bool {
	shared, err := h.Queries.IsCollaborationIssue(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "collaboration access unavailable")
		return true
	}
	if !shared {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeUnsupportedCollaborationAction(w)
		return true
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	if _, err := svc.ResolveIssueAccess(r.Context(), nil, issue.ID); err != nil {
		var e *service.CollaborationError
		if errors.As(err, &e) {
			writeJSON(w, e.Status, map[string]string{"error": e.Code, "code": e.Code})
		} else {
			writeError(w, http.StatusServiceUnavailable, "collaboration access unavailable")
		}
		return true
	}
	return false
}

func (h *Handler) rejectLegacyCollaborationWrite(w http.ResponseWriter, r *http.Request, issueID pgtype.UUID) bool {
	shared, err := h.Queries.IsCollaborationIssue(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "collaboration access unavailable")
		return true
	}
	if shared {
		writeUnsupportedCollaborationAction(w)
		return true
	}
	return false
}

func (h *Handler) rejectLegacyCollaborationBatch(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, rawIDs []string) bool {
	ids := make([]pgtype.UUID, 0, len(rawIDs))
	for _, id := range rawIDs {
		if parsed, err := util.ParseUUID(id); err == nil {
			ids = append(ids, parsed)
		}
	}
	shared, err := h.Queries.AnyCollaborationIssuesInWorkspace(r.Context(), db.AnyCollaborationIssuesInWorkspaceParams{WorkspaceID: workspaceID, IssueIds: ids})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "collaboration access unavailable")
		return true
	}
	if shared {
		writeUnsupportedCollaborationAction(w)
		return true
	}
	return false
}

// handleLegacyCollaborationUpdate keeps the explicitly permitted R58 layout
// operation. All other writes must use the collaboration domain endpoints.
func (h *Handler) handleLegacyCollaborationUpdate(w http.ResponseWriter, r *http.Request, issue db.Issue, body []byte, req UpdateIssueRequest) bool {
	shared, err := h.Queries.IsCollaborationIssue(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "collaboration access unavailable")
		return true
	}
	if !shared {
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return true
	}
	for name := range fields {
		if name = strings.ToLower(name); name != "position" && name != "expected_revision" {
			writeUnsupportedCollaborationAction(w)
			return true
		}
	}
	if req.Position == nil {
		writeUnsupportedCollaborationAction(w)
		return true
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	updated, err := svc.Reorder(r.Context(), issue.ID, *req.Position, req.ExpectedRevision)
	if err != nil {
		var e *service.CollaborationError
		if errors.As(err, &e) {
			writeJSON(w, e.Status, map[string]string{"error": e.Code, "code": e.Code})
		} else {
			writeError(w, http.StatusInternalServerError, "failed to reorder collaboration task")
		}
		return true
	}
	response := issueToResponse(updated, h.getIssuePrefix(r.Context(), updated.WorkspaceID))
	h.newStatusCategoryFiller(r.Context(), updated.WorkspaceID)(&response)
	writeJSON(w, http.StatusOK, response)
	return true
}
