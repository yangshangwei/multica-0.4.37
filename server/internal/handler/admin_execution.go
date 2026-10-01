package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

func adminTimestamp(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
func adminOptionalTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}
func adminFilterID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	return parseUUID(s)
}
func adminOptionalText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func adminTaskParams(identity service.PlatformAdminIdentity, p AdminListQuery) db.ListAdminTasksParams {
	return db.ListAdminTasksParams{ActorID: identity.UserID, OrganizationID: identity.OrganizationID, TimeFrom: adminTimestamp(p.From), TimeTo: adminTimestamp(p.To), AsOf: adminTimestamp(p.AsOf), WorkspaceID: adminFilterID(p.Filters["workspace_id"]), TaskID: adminFilterID(p.Filters["task_id"]), IssueID: adminFilterID(p.Filters["issue_id"]), RuntimeID: adminFilterID(p.Filters["runtime_id"]), UserID: adminFilterID(p.Filters["user_id"]), InstallationID: adminFilterID(p.Filters["installation_id"]), Status: p.Filters["status"], Source: p.Filters["source"], Search: p.Filters["q"], AfterTime: adminOptionalTimestamp(p.AfterTime), AfterID: adminFilterID(p.AfterID), PageLimit: p.Limit + 1}
}

func (h *Handler) AdminTasks(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	p, err := parseAdminQuery(r, "tasks", uuidToString(identity.UserID), uuidToString(identity.OrganizationID), time.Now().UTC())
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminTasks(ctx, adminTaskParams(identity, p))
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	var cursor *string
	if len(rows) > int(p.Limit) {
		rows = rows[:p.Limit]
		last := rows[len(rows)-1]
		token, err := p.Cursor(last.CreatedAt.Time, uuidToString(last.ID))
		if err != nil {
			adminServiceError(w, r, err)
			return
		}
		cursor = &token
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, adminExecutionResponse(row))
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": cursor, "as_of": p.AsOf.Format(time.RFC3339Nano), "scope": uuidToString(identity.OrganizationID), "time_from": p.From.Format(time.RFC3339Nano), "time_to": p.To.Format(time.RFC3339Nano), "timezone": p.Timezone, "data_quality": map[string]any{"usage": "reported_only", "state": "live"}})
}

func (h *Handler) AdminTask(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "task id")
	if !ok {
		return
	}
	now := time.Now().UTC()
	p := AdminListQuery{Filters: map[string]string{"task_id": uuidToString(id)}, From: time.Time{}, To: now, AsOf: now, Limit: 1}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminTasks(ctx, adminTaskParams(identity, p))
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	if len(rows) == 0 {
		adminError(w, r, 404, "admin_not_found", "The execution was not found")
		return
	}
	writeJSON(w, 200, adminExecutionResponse(rows[0]))
}

func adminExecutionResponse(row db.ListAdminTasksRow) map[string]any {
	var usage any
	if row.ReportedModels > 0 {
		usage = map[string]int64{"input_tokens": row.InputTokens, "output_tokens": row.OutputTokens, "cache_read_tokens": row.CacheReadTokens, "cache_write_tokens": row.CacheWriteTokens}
	}
	var failure *string
	if row.Status == "failed" || row.FailureReason.Valid {
		code := "unknown"
		for _, known := range taskfailure.AllReasons() {
			if string(known) == row.FailureReason.String {
				code = string(known)
				break
			}
		}
		failure = &code
	}
	var title *string
	if row.ContentUrl != "" {
		title = adminOptionalText(row.Title)
	}
	return map[string]any{
		"id": uuidToString(row.ID), "workspace_id": uuidToString(row.WorkspaceID), "agent_id": uuidToString(row.AgentID), "issue_id": uuidToPtr(row.IssueID), "runtime_id": uuidToPtr(row.RuntimeID), "chat_session_id": uuidToPtr(row.ChatSessionID), "autopilot_run_id": uuidToPtr(row.AutopilotRunID),
		"source": row.Source, "status": row.Status, "attempt": row.Attempt, "parent_task_id": uuidToPtr(row.ParentTaskID), "retry_of_task_id": uuidToPtr(row.RetryOfTaskID), "rerun_of_task_id": uuidToPtr(row.RerunOfTaskID), "accountable_user_id": uuidToPtr(row.AccountableUserID),
		"submitted_installation_id": uuidToPtr(row.SubmittedInstallationID), "execution_installation_id": uuidToPtr(row.ExecutionInstallationID), "created_at": timestampToString(row.CreatedAt), "dispatched_at": timestampToPtr(row.DispatchedAt), "started_at": timestampToPtr(row.StartedAt), "completed_at": timestampToPtr(row.CompletedAt),
		"provider": adminOptionalText(row.Provider), "model": adminOptionalText(row.Model), "failure_code": failure, "usage": usage, "title": title, "content_access": row.ContentUrl != "", "content_url": adminOptionalText(row.ContentUrl), "allowed_actions": []string{},
	}
}

func (h *Handler) AdminIssues(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	p, err := parseAdminQuery(r, "issues", uuidToString(identity.UserID), uuidToString(identity.OrganizationID), time.Now().UTC())
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminIssues(ctx, db.ListAdminIssuesParams{ActorID: identity.UserID, OrganizationID: identity.OrganizationID, TimeFrom: adminTimestamp(p.From), TimeTo: adminTimestamp(p.To), AsOf: adminTimestamp(p.AsOf), WorkspaceID: adminFilterID(p.Filters["workspace_id"]), IssueID: adminFilterID(p.Filters["issue_id"]), Status: p.Filters["status"], Search: p.Filters["q"], AfterTime: adminOptionalTimestamp(p.AfterTime), AfterID: adminFilterID(p.AfterID), PageLimit: p.Limit + 1})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	var cursor *string
	if len(rows) > int(p.Limit) {
		rows = rows[:p.Limit]
		last := rows[len(rows)-1]
		token, err := p.Cursor(last.CreatedAt.Time, uuidToString(last.ID))
		if err != nil {
			adminServiceError(w, r, err)
			return
		}
		cursor = &token
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": uuidToString(row.ID), "workspace_id": uuidToString(row.WorkspaceID), "number": row.Number, "identifier": fmt.Sprintf("%s-%d", row.IssuePrefix, row.Number), "status": row.Status, "created_at": timestampToString(row.CreatedAt), "updated_at": timestampToString(row.UpdatedAt), "execution_count": row.ExecutionCount, "title": adminOptionalText(row.Title), "content_access": row.ContentUrl != "", "content_url": adminOptionalText(row.ContentUrl)})
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": cursor, "as_of": p.AsOf.Format(time.RFC3339Nano), "scope": uuidToString(identity.OrganizationID), "time_from": p.From.Format(time.RFC3339Nano), "time_to": p.To.Format(time.RFC3339Nano), "timezone": p.Timezone, "data_quality": map[string]any{"state": "live"}})
}
