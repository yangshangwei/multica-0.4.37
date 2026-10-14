package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationAccountSummary struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	External  bool   `json:"external"`
}

type CollaborationWriteContext struct {
	ParticipantEpoch string `json:"participant_epoch"`
	AccountEpoch     string `json:"account_epoch"`
}

type CollaborationIssueResponse struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Status          string  `json:"status"`
	StatusCategory  string  `json:"status_category"`
	Priority        string  `json:"priority"`
	StartDate       *string `json:"start_date"`
	DueDate         *string `json:"due_date"`
	SourceWorkspace struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Identifier string `json:"identifier"`
	} `json:"source_workspace"`
	Creator        CollaborationAccountSummary `json:"creator"`
	Handler        CollaborationAccountSummary `json:"handler"`
	Revision       string                      `json:"collaboration_revision"`
	LifecycleEpoch string                      `json:"lifecycle_epoch"`
	WriteContext   *CollaborationWriteContext  `json:"write_context,omitempty"`
	Session        struct {
		ID             string `json:"id"`
		QueueState     string `json:"queue_state"`
		ControlVersion string `json:"control_version"`
	} `json:"session"`
	CurrentInformationRequest *CollaborationInformationRequestResponse `json:"current_information_request"`
	Capabilities              collaboration.Capabilities               `json:"capabilities"`
}

// CollaborationInformationRequestResponse names the open request and where a
// return lands. Clients display ReturnStatus and send it back unchanged.
type CollaborationInformationRequestResponse struct {
	ID                   string                      `json:"id"`
	Requester            CollaborationAccountSummary `json:"requester"`
	Recipient            CollaborationAccountSummary `json:"recipient"`
	Note                 string                      `json:"note"`
	ReturnStatus         *string                     `json:"return_status"`
	ReturnStatusFallback bool                        `json:"return_status_fallback"`
	CreatedAt            string                      `json:"created_at"`
}

func writeCollaborationError(w http.ResponseWriter, err error) {
	var domain *service.CollaborationError
	if errors.As(err, &domain) {
		writeJSON(w, domain.Status, map[string]string{"error": domain.Code, "code": domain.Code})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Collaboration data is unavailable.", "code": "collaboration_unavailable"})
}

func (h *Handler) collaborationAccountSummary(ctx context.Context, userID, workspaceID pgtype.UUID) (CollaborationAccountSummary, error) {
	u, err := h.Queries.GetUser(ctx, userID)
	if err != nil {
		return CollaborationAccountSummary{}, err
	}
	_, err = h.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: userID, WorkspaceID: workspaceID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationAccountSummary{}, err
	}
	return CollaborationAccountSummary{UserID: uuidToString(u.ID), Name: u.Name, AvatarURL: u.AvatarUrl.String, External: errors.Is(err, pgx.ErrNoRows)}, nil
}

func (h *Handler) collaborationIssueResponse(ctx context.Context, access service.CollaborationIssueAccess) (CollaborationIssueResponse, error) {
	var out CollaborationIssueResponse
	i := access.Issue
	if i.CreatorType != "member" || !i.AssigneeType.Valid || i.AssigneeType.String != "member" || !i.AssigneeID.Valid {
		return out, errors.New("collaboration has invalid human identity")
	}
	workspace, err := h.Queries.GetWorkspace(ctx, i.WorkspaceID)
	if err != nil {
		return out, err
	}
	out.Creator, err = h.collaborationAccountSummary(ctx, i.CreatorID, i.WorkspaceID)
	if err != nil {
		return out, err
	}
	out.Handler, err = h.collaborationAccountSummary(ctx, i.AssigneeID, i.WorkspaceID)
	if err != nil {
		return out, err
	}
	out.ID, out.Title, out.Description = uuidToString(i.ID), i.Title, i.Description.String
	out.Status, out.StatusCategory, out.Priority = i.Status, access.StatusCategory, i.Priority
	out.StartDate, out.DueDate = dateToPtr(i.StartDate), dateToPtr(i.DueDate)
	out.SourceWorkspace.ID, out.SourceWorkspace.Name = uuidToString(workspace.ID), workspace.Name
	out.SourceWorkspace.Identifier = fmt.Sprintf("%s-%d", h.getIssuePrefix(ctx, workspace.ID), i.Number)
	out.Revision = strconv.FormatInt(access.Revision, 10)
	out.LifecycleEpoch = strconv.FormatInt(access.LifecycleEpoch, 10)
	if access.ParticipantEpoch.Valid && access.ParticipantEpoch.Int64 > 0 && access.AccountEpoch.Valid && access.AccountEpoch.Int64 > 0 {
		out.WriteContext = &CollaborationWriteContext{ParticipantEpoch: strconv.FormatInt(access.ParticipantEpoch.Int64, 10), AccountEpoch: strconv.FormatInt(access.AccountEpoch.Int64, 10)}
	}
	out.Session.ID, out.Session.QueueState = uuidToString(access.SessionID), access.QueueState
	out.Session.ControlVersion = strconv.FormatInt(access.ControlVersion, 10)
	out.Capabilities = access.Capabilities
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	request, err := svc.InformationRequestView(ctx, access)
	if err != nil || request == nil {
		return out, err
	}
	view := &CollaborationInformationRequestResponse{ID: uuidToString(request.ID), Note: request.Note,
		ReturnStatusFallback: request.ReturnStatusFallback, CreatedAt: timestampToString(request.CreatedAt)}
	if view.Requester, err = h.collaborationAccountSummary(ctx, request.RequesterID, i.WorkspaceID); err != nil {
		return out, err
	}
	if view.Recipient, err = h.collaborationAccountSummary(ctx, request.RecipientID, i.WorkspaceID); err != nil {
		return out, err
	}
	if request.ReturnStatus != "" {
		view.ReturnStatus = &request.ReturnStatus
	}
	out.CurrentInformationRequest = view
	return out, nil
}

func (h *Handler) GetCollaborationIssue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue id")
	if !ok {
		return
	}
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	access, err := svc.ResolveIssueAccess(r.Context(), nil, id)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	out, err := h.collaborationIssueResponse(r.Context(), access)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetCollaborationCreation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	source, ok := parseUUIDOrBadRequest(w, r.URL.Query().Get("source_workspace_id"), "source workspace id")
	if !ok {
		return
	}
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	access, err := svc.GetCreation(r.Context(), source, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	out, err := h.collaborationIssueResponse(r.Context(), access)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
