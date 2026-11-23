package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// This predicate is applied before pagination/counts. An external participant
// remains in the filter after handing the issue back to a source member.
const externalCollaboratorsPredicate = `EXISTS (
 SELECT 1 FROM issue_collaboration ec
 JOIN issue_participant p ON p.issue_id=ec.issue_id AND p.revoked_at IS NULL
 JOIN "user" u ON u.id=p.user_id AND u.disabled_at IS NULL
 WHERE ec.issue_id=i.id
 AND NOT EXISTS (SELECT 1 FROM member m WHERE m.workspace_id=i.workspace_id AND m.user_id=p.user_id)
)`

type sourceIssueCollaboration struct {
	Creator                  CollaborationAccountSummary `json:"creator"`
	Handler                  CollaborationAccountSummary `json:"handler"`
	HasExternalCollaborators bool                        `json:"has_external_collaborators"`
}

// The relationship is authority-critical: never silently return ordinary
// issue affordances when the projection query fails.
func (h *Handler) sourceIssueCollaborations(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, ids []pgtype.UUID) (map[string]*sourceIssueCollaboration, bool) {
	out := make(map[string]*sourceIssueCollaboration)
	if len(ids) == 0 {
		return out, true
	}
	rows, err := h.Queries.ListSourceIssueCollaborations(r.Context(), db.ListSourceIssueCollaborationsParams{WorkspaceID: workspaceID, IssueIds: ids})
	if err != nil {
		writeCollaborationError(w, err)
		return nil, false
	}
	for _, row := range rows {
		if !row.CreatorUserID.Valid || !row.HandlerUserID.Valid {
			writeError(w, http.StatusServiceUnavailable, "collaboration identity unavailable")
			return nil, false
		}
		out[uuidToString(row.IssueID)] = &sourceIssueCollaboration{
			Creator:                  CollaborationAccountSummary{UserID: uuidToString(row.CreatorUserID), Name: row.CreatorName.String, AvatarURL: row.CreatorAvatarUrl.String, External: row.CreatorExternal},
			Handler:                  CollaborationAccountSummary{UserID: uuidToString(row.HandlerUserID), Name: row.HandlerName.String, AvatarURL: row.HandlerAvatarUrl.String, External: row.HandlerExternal},
			HasExternalCollaborators: row.HasExternalCollaborators,
		}
	}
	if len(out) > 0 {
		w.Header().Set("Cache-Control", "no-store")
	}
	return out, true
}

func (h *Handler) fillSourceIssueCollaboration(w http.ResponseWriter, r *http.Request, resp *IssueResponse) bool {
	projections, ok := h.sourceIssueCollaborations(w, r, parseUUID(resp.WorkspaceID), []pgtype.UUID{parseUUID(resp.ID)})
	if ok {
		resp.Collaboration = projections[resp.ID]
	}
	return ok
}

func (h *Handler) projectSourceChildIssues(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, rows []IssueResponse) ([]any, bool) {
	ids := make([]pgtype.UUID, len(rows))
	for index, row := range rows {
		ids[index] = parseUUID(row.ID)
	}
	projections, ok := h.sourceIssueCollaborations(w, r, workspaceID, ids)
	if !ok {
		return nil, false
	}
	out := make([]any, len(rows))
	for index, row := range rows {
		row.Collaboration = projections[row.ID]
		if row.Collaboration != nil && collaborationMachineRequest(r) {
			out[index] = collaborationIssueReference{ID: row.ID, WorkspaceID: row.WorkspaceID, Number: row.Number, Identifier: row.Identifier, Title: row.Title, CollaborationReference: true}
		} else {
			out[index] = row
		}
	}
	return out, true
}

func (h *Handler) fillSourceCollaborationGroups(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, groups []issueTableGroupDescriptorResponse) bool {
	ids := make([]pgtype.UUID, 0, len(groups))
	for _, group := range groups {
		if group.Value.Kind == "assignee" && group.Value.Actor != nil && group.Value.Actor.Type == "member" {
			ids = append(ids, parseUUID(group.Value.Actor.ID))
		}
	}
	if len(ids) == 0 {
		return true
	}
	rows, err := h.Queries.ListSourceCollaborationAccounts(r.Context(), db.ListSourceCollaborationAccountsParams{WorkspaceID: workspaceID, UserIds: ids})
	if err != nil {
		writeCollaborationError(w, err)
		return false
	}
	accounts := make(map[string]CollaborationAccountSummary, len(rows))
	for _, row := range rows {
		accounts[uuidToString(row.UserID)] = CollaborationAccountSummary{UserID: uuidToString(row.UserID), Name: row.Name, AvatarURL: row.AvatarUrl.String, External: row.External}
	}
	for index := range groups {
		actor := groups[index].Value.Actor
		if actor == nil || actor.Type != "member" {
			continue
		}
		if account, found := accounts[actor.ID]; found {
			groups[index].Value.Account = &account
		}
	}
	return true
}
