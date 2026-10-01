package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) adminAlertService() *service.AdminAlertService {
	return service.NewAdminAlertService(h.Queries, h.TxStarter, auth.ManagedDeploymentID(), h.LivenessStore)
}
func adminAlertResponse(alert db.AdminAlert, canWrite bool) map[string]any {
	actions := []string{}
	if canWrite && adminContains([]string{"open", "acknowledged", "resolved"}, alert.Status) {
		if alert.Status == "open" {
			actions = append(actions, "acknowledge")
		}
		actions = append(actions, "assign")
		if alert.Rule == service.AdminAlertExecutionFailed || alert.Status == "resolved" && !alert.ConditionActive {
			actions = append(actions, "close")
		}
	}
	resolution := textToPtr(alert.ResolutionCode)
	if resolution != nil {
		code := service.AdminAlertResolutionCode(*resolution)
		resolution = &code
	}
	return map[string]any{"id": uuidToString(alert.ID), "organization_id": uuidToString(alert.OrganizationID), "rule": alert.Rule, "severity": alert.Severity, "subject_kind": alert.SubjectKind, "subject_id": uuidToString(alert.SubjectID), "status": alert.Status, "condition_active": alert.ConditionActive, "first_seen_at": timestampToString(alert.FirstSeenAt), "last_seen_at": timestampToString(alert.LastSeenAt), "last_observed_at": timestampToString(alert.LastObservedAt), "occurrence_count": alert.OccurrenceCount, "assignee_id": uuidToPtr(alert.AssigneeID), "acknowledged_at": timestampToPtr(alert.AcknowledgedAt), "resolved_at": timestampToPtr(alert.ResolvedAt), "closed_at": timestampToPtr(alert.ClosedAt), "version": strconv.FormatInt(alert.Version, 10), "resolution_code": resolution, "related_task_id": uuidToPtr(alert.RelatedTaskID), "operation_id": uuidToPtr(alert.OperationID), "allowed_actions": actions}
}
func parseAdminAlertQuery(r *http.Request, actor service.PlatformAdminIdentity, now time.Time) (AdminListQuery, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return AdminListQuery{}, adminQueryError()
	}
	status := values.Get("status")
	if items, present := values["status"]; present && len(items) != 1 {
		return AdminListQuery{}, adminQueryError()
	}
	if status == "" {
		status = "active"
		values.Set("status", status)
	}
	if !adminContains([]string{"active", "all", "open", "acknowledged", "resolved", "closed"}, status) {
		return AdminListQuery{}, adminQueryError()
	}
	window := 31 * 24 * time.Hour
	if status == "active" || status == "open" || status == "acknowledged" {
		window = 0
	}
	copyRequest := r.Clone(r.Context())
	copyURL := *r.URL
	copyURL.RawQuery = values.Encode()
	copyRequest.URL = &copyURL
	p, err := ParseAdminListQuery(copyRequest, AdminListScope{Resource: "alerts", ActorID: uuidToString(actor.UserID), OrganizationID: uuidToString(actor.OrganizationID), Window: window}, []string{"q", "status", "rule", "severity", "subject_kind", "subject_id", "assignee_id"}, now)
	if err != nil {
		return p, err
	}
	if values.Has("time_from") && p.To.Sub(p.From) > 31*24*time.Hour {
		return p, adminQueryError()
	}
	if p.Filters["rule"] != "" && !adminContains([]string{service.AdminAlertInstallationUnreachable, service.AdminAlertQueueTimeout, service.AdminAlertExecutionFailed}, p.Filters["rule"]) {
		return p, adminQueryError()
	}
	if p.Filters["severity"] != "" && !adminContains([]string{"warning", "critical"}, p.Filters["severity"]) {
		return p, adminQueryError()
	}
	if p.Filters["subject_kind"] != "" && !adminContains([]string{"installation", "task"}, p.Filters["subject_kind"]) {
		return p, adminQueryError()
	}
	for _, key := range []string{"subject_id", "assignee_id"} {
		if value := p.Filters[key]; value != "" {
			if _, err := util.ParseUUID(value); err != nil {
				return p, adminQueryError()
			}
		}
	}
	if len(p.Filters["q"]) > 128 || strings.ContainsRune(p.Filters["q"], 0) {
		return p, adminQueryError()
	}
	return p, nil
}
func (h *Handler) AdminAlerts(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	p, err := parseAdminAlertQuery(r, actor, time.Now().UTC())
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminAlerts(ctx, db.ListAdminAlertsParams{OrganizationID: actor.OrganizationID, AsOf: adminTimestamp(p.AsOf), TimeFrom: adminOptionalTime(p.From), TimeTo: adminTimestamp(p.To), Status: p.Filters["status"], Rule: p.Filters["rule"], Severity: p.Filters["severity"], SubjectKind: p.Filters["subject_kind"], SubjectID: adminFilterID(p.Filters["subject_id"]), AssigneeID: adminFilterID(p.Filters["assignee_id"]), Search: p.Filters["q"], AfterTime: adminOptionalTime(p.AfterTime), AfterID: adminFilterID(p.AfterID), PageLimit: p.Limit + 1})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	var next *string
	if len(rows) > int(p.Limit) {
		rows = rows[:p.Limit]
		last := rows[len(rows)-1]
		value, e := p.Cursor(last.FirstSeenAt.Time, uuidToString(last.ID))
		if e != nil {
			adminServiceError(w, r, e)
			return
		}
		next = &value
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, adminAlertResponse(row, actor.Role == service.PlatformRoleSuperAdmin))
	}
	health, err := h.adminAlertService().DetectorHealth(ctx, actor.OrganizationID)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	quality, known, stale := "complete", 0, false
	for _, detector := range health {
		if detector.SourceState != "healthy" || !detector.ScanComplete || detector.UnknownCount > 0 {
			quality = "partial"
		}
		if detector.LastSuccessfulAt == nil {
			quality = "partial"
			continue
		}
		known++
		observed, e := time.Parse(time.RFC3339Nano, *detector.LastSuccessfulAt)
		if e != nil || observed.Before(time.Now().Add(-time.Minute)) || observed.After(time.Now().Add(time.Minute)) {
			stale = true
		}
	}
	if known == 0 {
		quality = "unknown"
	} else if quality == "complete" && stale {
		quality = "stale"
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next, "scope": uuidToString(actor.OrganizationID), "as_of": p.AsOf.Format(time.RFC3339Nano), "time_from": timestampToPtr(adminOptionalTime(p.From)), "time_to": p.To.Format(time.RFC3339Nano), "timezone": p.Timezone, "data_quality": quality, "detector_health": health})
}
func (h *Handler) AdminAlert(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "alert id")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	alert, err := h.Queries.GetAdminAlert(ctx, db.GetAdminAlertParams{ID: id, OrganizationID: actor.OrganizationID})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, adminAlertResponse(alert, actor.Role == service.PlatformRoleSuperAdmin))
}
func (h *Handler) AdminAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	h.mutateAdminAlert(w, r, "acknowledge")
}
func (h *Handler) AdminAssignAlert(w http.ResponseWriter, r *http.Request) {
	h.mutateAdminAlert(w, r, "assign")
}
func (h *Handler) AdminCloseAlert(w http.ResponseWriter, r *http.Request) {
	h.mutateAdminAlert(w, r, "close")
}
func (h *Handler) mutateAdminAlert(w http.ResponseWriter, r *http.Request, action string) {
	actor, ok := h.requirePlatformAccess(w, r, true)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "alert id")
	if !ok {
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "idempotency key")
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion string          `json:"expected_version"`
		Reason          string          `json:"reason"`
		AssigneeID      json.RawMessage `json:"assignee_id"`
		ResolutionCode  string          `json:"resolution_code"`
		RelatedTaskID   *string         `json:"related_task_id"`
	}
	if !passwordDecode(w, r, &body) {
		return
	}
	version, err := strconv.ParseInt(body.ExpectedVersion, 10, 64)
	if err != nil || version < 1 {
		adminError(w, r, 400, "invalid_request", "A valid alert version is required")
		return
	}
	p := service.AdminAlertMutationParams{OrganizationID: actor.OrganizationID, AlertID: id, IdempotencyKey: key, Action: action, ExpectedVersion: version, Reason: body.Reason, RequestID: adminRequestID(r), ResolutionCode: body.ResolutionCode}
	if action == "assign" {
		var selected *string
		if len(body.AssigneeID) == 0 || json.Unmarshal(body.AssigneeID, &selected) != nil {
			adminError(w, r, 400, "invalid_request", "assignee_id must be a user ID or null")
			return
		}
		if selected != nil {
			p.AssigneeID, err = util.ParseUUID(*selected)
			if err != nil {
				adminError(w, r, 400, "invalid_request", "The assignee ID is invalid")
				return
			}
		}
	} else if len(body.AssigneeID) > 0 {
		adminError(w, r, 400, "invalid_request", "This action does not assign an alert")
		return
	}
	if body.RelatedTaskID != nil {
		p.RelatedTaskID, err = util.ParseUUID(*body.RelatedTaskID)
		if err != nil {
			adminError(w, r, 400, "invalid_request", "The related task ID is invalid")
			return
		}
	}
	result, err := h.adminAlertService().Mutate(r.Context(), p)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"operation": adminOperationResponse(result.Operation), "target": adminAlertResponse(result.Target, true)})
}
