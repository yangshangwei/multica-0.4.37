package handler

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var adminCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_.:-]{0,127}$`)
var adminDecimalPattern = regexp.MustCompile(`^[0-9]{1,19}$`)
var adminResourceKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

// Only safe historical decision fields survive projection; never emit raw
// config, task diagnostics, credentials or arbitrary nested snapshots.
func adminAuditSnapshot(raw []byte) map[string]any {
	result := map[string]any{}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return result
	}
	for key, item := range value {
		switch key {
		case "role":
			if item == nil || item == "super_admin" || item == "platform_observer" {
				result[key] = item
			}
		case "disabled", "access_revoked", "requires_password_change", "condition_active":
			if flag, ok := item.(bool); ok {
				result[key] = flag
			}
		case "auth_version", "version", "admission_version", "binding_epoch":
			text := ""
			switch number := item.(type) {
			case json.Number:
				text = number.String()
			case string:
				text = number
			}
			if adminDecimalPattern.MatchString(text) {
				result[key] = text
			}
		case "resolution_code":
			if item == nil {
				result[key] = nil
			} else if text, ok := item.(string); ok {
				result[key] = service.AdminAlertResolutionCode(text)
			}
		case "state", "status", "admission", "confirmation", "reconciliation_state", "rule", "severity":
			allowed := map[string][]string{
				"state":     {"applied", "succeeded", "failed", "active", "retired"},
				"status":    {"open", "acknowledged", "resolved", "closed", "queued", "preparing", "dispatched", "running", "waiting_local_directory", "completed", "failed", "cancelled", "deferred", "active", "disabled"},
				"admission": {"accepting", "stopped"}, "confirmation": {"pending", "confirmed", "unconfirmed", "unavailable", "not_required"}, "reconciliation_state": {"pending", "complete", "unconfirmed"},
				"rule": {"installation_unreachable", "queue_timeout", "execution_failed"}, "severity": {"warning", "critical"},
			}
			if text, ok := item.(string); ok {
				if !adminContains(allowed[key], text) {
					text = "unknown"
				}
				result[key] = text
			}
		case "result_code":
			if text, ok := item.(string); ok && adminCodePattern.MatchString(text) {
				result[key] = text
			}
		case "assignee_id", "assigned_to", "responsible_user_id", "related_task_id":
			if item == nil {
				result[key] = nil
			} else if text, ok := item.(string); ok {
				if _, err := uuid.Parse(text); err == nil {
					result[key] = text
				}
			}
		}
	}
	return result
}

// Resource versions are UUID revisions, unlike numeric administrative versions.
// Their separate projection cannot broaden historical snapshots for other targets.
func adminAuditTargetSnapshot(raw []byte, targetKind string) map[string]any {
	if targetKind != "resource" {
		return adminAuditSnapshot(raw)
	}
	result := map[string]any{}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return result
	}
	for key, item := range value {
		switch key {
		case "kind":
			if item == "skill" || item == "mcp" {
				result[key] = item
			}
		case "key":
			if text, ok := item.(string); ok && adminResourceKeyPattern.MatchString(text) {
				result[key] = text
			}
		case "state":
			if item == "published" || item == "withdrawn" {
				result[key] = item
			}
		case "version", "expected_version", "operation_id":
			if text, ok := item.(string); ok {
				if text == "" && key == "expected_version" {
					result[key] = text
					continue
				}
				if id, err := uuid.Parse(text); err == nil && id != uuid.Nil && id.String() == text {
					result[key] = text
				}
			}
		case "content_digest":
			if text, ok := item.(string); ok && len(text) == 71 && strings.HasPrefix(text, "sha256:") {
				if _, err := hex.DecodeString(text[7:]); err == nil {
					result[key] = text
				}
			}
		case "file_count", "byte_count":
			if number, ok := item.(json.Number); ok {
				maximum := int64(9 << 20)
				if key == "file_count" {
					maximum = 257
				}
				if count, err := number.Int64(); err == nil && count >= 0 && count <= maximum {
					result[key] = count
				}
			}
		}
	}
	return result
}
func (h *Handler) AdminAudit(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	p, err := ParseAdminListQuery(r, AdminListScope{Resource: "audit", ActorID: uuidToString(identity.UserID), OrganizationID: uuidToString(identity.OrganizationID), Window: 90 * 24 * time.Hour}, []string{"action", "actor_user_id", "target_kind", "target_id", "phase"}, time.Now().UTC())
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	for _, name := range []string{"actor_user_id", "target_id"} {
		if value := p.Filters[name]; value != "" {
			if _, err := uuid.Parse(value); err != nil {
				adminServiceError(w, r, adminQueryError())
				return
			}
		}
	}
	for _, name := range []string{"action", "target_kind", "phase"} {
		value := p.Filters[name]
		if len(value) > 128 || strings.ContainsRune(value, 0) {
			adminServiceError(w, r, adminQueryError())
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminAuditEvents(ctx, db.ListAdminAuditEventsParams{OrganizationID: identity.OrganizationID, TimeFrom: adminTimestamp(p.From), TimeTo: adminTimestamp(p.To), AsOf: adminTimestamp(p.AsOf), Action: p.Filters["action"], ActorUserID: adminFilterID(p.Filters["actor_user_id"]), TargetKind: p.Filters["target_kind"], TargetID: adminFilterID(p.Filters["target_id"]), Phase: p.Filters["phase"], AfterTime: adminOptionalTimestamp(p.AfterTime), AfterID: adminFilterID(p.AfterID), PageLimit: p.Limit + 1})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	var cursor *string
	if len(rows) > int(p.Limit) {
		rows = rows[:p.Limit]
		last := rows[len(rows)-1]
		value, err := p.Cursor(last.CreatedAt.Time, uuidToString(last.ID))
		if err != nil {
			adminServiceError(w, r, err)
			return
		}
		cursor = &value
	}
	quality := "complete"
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		actorQuality := "captured"
		var actorName *string
		if row.ActorDisplayName.Valid {
			actorName = &row.ActorDisplayName.String
		} else {
			actorQuality = "unknown"
			quality = "partial"
		}
		items = append(items, map[string]any{"id": uuidToString(row.ID), "operation_id": uuidToPtr(row.OperationID), "actor_kind": row.ActorKind, "actor_user_id": uuidToPtr(row.ActorUserID), "actor_display_name": actorName, "actor_snapshot_quality": actorQuality, "target_kind": row.TargetKind, "target_id": uuidToString(row.TargetID), "action": row.Action, "phase": row.Phase, "result_code": row.ResultCode, "request_id": row.RequestID, "reason": row.Reason, "before_state": adminAuditTargetSnapshot(row.BeforeState, row.TargetKind), "after_state": adminAuditTargetSnapshot(row.AfterState, row.TargetKind), "created_at": timestampToString(row.CreatedAt)})
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": cursor, "scope": uuidToString(identity.OrganizationID), "as_of": p.AsOf.Format(time.RFC3339Nano), "window": adminWindow(p), "data_quality": quality})
}
