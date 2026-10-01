package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type adminInstallationAxis struct {
	State      string  `json:"state"`
	ObservedAt *string `json:"observed_at"`
	Source     string  `json:"source"`
	Freshness  string  `json:"freshness"`
	ReasonCode string  `json:"reason_code"`
}
type adminInstallationRuntimeEvidence struct {
	RuntimeID, BindingID, WorkspaceID, PrincipalID, Provider, Status, OfflineCode, Capability string
	Authorized                                                                                bool
	LastSeen, BindingSeen                                                                     pgtype.Timestamptz
	RunningTasks                                                                              int64
}

func installationAxis(state, source, freshness, reason string, observed pgtype.Timestamptz) adminInstallationAxis {
	return adminInstallationAxis{State: state, Source: source, Freshness: freshness, ReasonCode: reason, ObservedAt: timestampToPtr(observed)}
}
func installationRecent(now time.Time, seen pgtype.Timestamptz, age time.Duration) bool {
	return seen.Valid && seen.Time.After(now.Add(-age)) && !seen.Time.After(now.Add(time.Minute))
}
func installationLatest(a, b pgtype.Timestamptz) pgtype.Timestamptz {
	if !a.Valid || b.Valid && b.Time.After(a.Time) {
		return b
	}
	return a
}

// These are independent observed prerequisites, not a promise that an
// arbitrary task can claim a slot. Claim SQL still owns task-specific access,
// concurrency and the exact RuntimeClaimFreshnessSeconds DB heartbeat fence.
func installationAxes(now time.Time, lifecycle, admission string, clientSeen pgtype.Timestamptz, runtimes []adminInstallationRuntimeEvidence, alive map[string]bool, storeConfigured, storeOK bool) (adminInstallationAxis, adminInstallationAxis, adminInstallationAxis) {
	client := installationAxis("unknown", "client_report", "unknown", "client_report_missing", clientSeen)
	if clientSeen.Valid {
		client = installationAxis("inactive", "client_report", "stale", "client_report_stale", clientSeen)
		if installationRecent(now, clientSeen, 180*time.Second) {
			client = installationAxis("active", "client_report", "fresh", "client_report_recent", clientSeen)
		}
	}
	daemon := installationAxis("unknown", "none", "unknown", "binding_evidence_missing", pgtype.Timestamptz{})
	ready := installationAxis("unknown", "none", "unknown", "runtime_evidence_missing", pgtype.Timestamptz{})
	var latest, runtimeLatest pgtype.Timestamptz
	latestSource := "none"
	reachable, authorized, hasRuntime, canRun, knownOffline, knownStale, knownBroken := false, false, false, false, false, false, false
	source := "binding_heartbeat"
	for _, runtime := range runtimes {
		if !runtime.Authorized {
			continue
		}
		authorized = true
		if runtime.BindingSeen.Valid && (!latest.Valid || runtime.BindingSeen.Time.After(latest.Time)) {
			latest = runtime.BindingSeen
			latestSource = "binding_heartbeat"
		}
		if runtime.LastSeen.Valid && (!latest.Valid || runtime.LastSeen.Time.After(latest.Time)) {
			latest = runtime.LastSeen
			latestSource = "runtime_heartbeat"
		}
		if installationRecent(now, runtime.BindingSeen, time.Duration(service.RuntimeClaimFreshnessSeconds)*time.Second) {
			reachable = true
			source = "binding_heartbeat"
		}
		if installationRecent(now, runtime.LastSeen, time.Duration(service.RuntimeClaimFreshnessSeconds)*time.Second) {
			reachable = true
			source = "runtime_heartbeat"
		}
		if runtime.RuntimeID == "" {
			continue
		}
		hasRuntime = true
		runtimeLatest = installationLatest(runtimeLatest, runtime.LastSeen)
		if alive[runtime.RuntimeID] {
			reachable = true
			source = "liveness_store"
		}
		if runtime.Status == "online" && installationRecent(now, runtime.LastSeen, time.Duration(service.RuntimeClaimFreshnessSeconds)*time.Second) && runtime.Capability == "1" {
			canRun = true
		}
		if runtime.Status == "offline" {
			knownOffline = true
		}
		if runtime.OfflineCode == "not_executable" {
			knownBroken = true
		}
		if runtime.LastSeen.Valid && !installationRecent(now, runtime.LastSeen, time.Duration(service.RuntimeClaimFreshnessSeconds)*time.Second) {
			knownStale = true
		}
	}
	if latest.Valid {
		daemon = installationAxis("unreachable", latestSource, "stale", "heartbeat_stale", latest)
	}
	if reachable {
		observed := latest
		if source == "liveness_store" {
			observed = adminTimestamp(now)
		}
		daemon = installationAxis("reachable", source, "fresh", "heartbeat_recent", observed)
	}
	if len(runtimes) > 0 && !authorized {
		ready = installationAxis("no_permission", "credential_gate", "fresh", "credential_unavailable", adminTimestamp(now))
	}
	if hasRuntime && authorized {
		freshness := "unknown"
		if runtimeLatest.Valid {
			freshness = "stale"
			if installationRecent(now, runtimeLatest, time.Duration(service.RuntimeClaimFreshnessSeconds)*time.Second) {
				freshness = "fresh"
			}
		}
		switch {
		case canRun:
			ready = installationAxis("ready", "runtime_heartbeat", "fresh", "base_requirements_met", runtimeLatest)
		case knownBroken:
			ready = installationAxis("environment_unavailable", "runtime_heartbeat", freshness, "engine_not_executable", runtimeLatest)
		case knownOffline:
			ready = installationAxis("environment_unavailable", "runtime_heartbeat", freshness, "runtime_offline", runtimeLatest)
		case knownStale:
			ready = installationAxis("environment_unavailable", "runtime_heartbeat", "stale", "runtime_stale", runtimeLatest)
		}
	}
	if storeConfigured && !storeOK {
		daemon = installationAxis("unavailable", "liveness_store", "unavailable", "liveness_unavailable", latest)
		ready = installationAxis("unavailable", "liveness_store", "unavailable", "liveness_unavailable", latest)
	}
	if admission == "stopped" {
		ready = installationAxis("stopped", "admission", "fresh", "admission_stopped", adminTimestamp(now))
	}
	if lifecycle == "retired" {
		ready = installationAxis("stopped", "admission", "fresh", "installation_retired", adminTimestamp(now))
	} else if lifecycle != "active" || admission != "accepting" && admission != "stopped" {
		ready = installationAxis("unknown", "none", "unknown", "policy_unknown", pgtype.Timestamptz{})
	}
	return client, daemon, ready
}

func (h *Handler) installationReadScope(w http.ResponseWriter, r *http.Request) (service.PlatformAdminIdentity, pgtype.UUID, bool) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return identity, pgtype.UUID{}, false
	}
	deployment := auth.ManagedDeploymentID()
	if deployment == "" {
		adminError(w, r, 503, "installation_unavailable", "The deployment identity is not configured")
		return identity, pgtype.UUID{}, false
	}
	return identity, parseUUID(deployment), true
}

func installationReadQuery(r *http.Request, resource string, identity service.PlatformAdminIdentity, deployment pgtype.UUID, now time.Time, filters []string) (AdminListQuery, error) {
	p, err := ParseAdminListQuery(r, AdminListScope{Resource: resource + ":" + uuidToString(deployment), ActorID: uuidToString(identity.UserID), OrganizationID: uuidToString(identity.OrganizationID)}, filters, now)
	if err != nil {
		return p, err
	}
	for _, value := range p.Filters {
		if len(value) > 128 || strings.ContainsRune(value, 0) {
			return p, adminQueryError()
		}
	}
	if p.Filters["lifecycle"] != "" && !adminContains([]string{"active", "retired"}, p.Filters["lifecycle"]) {
		return p, adminQueryError()
	}
	if p.Filters["client_state"] != "" && !adminContains([]string{"active", "inactive", "unknown"}, p.Filters["client_state"]) {
		return p, adminQueryError()
	}
	return p, nil
}

func (h *Handler) installationEvidence(ctx context.Context, identity service.PlatformAdminIdentity, deployment pgtype.UUID, ids []pgtype.UUID) (map[string][]adminInstallationRuntimeEvidence, map[string]bool, bool, bool, bool, error) {
	rows, err := h.Queries.ListAdminInstallationEvidence(ctx, db.ListAdminInstallationEvidenceParams{OrganizationID: identity.OrganizationID, DeploymentID: deployment, InstallationIds: ids})
	if err != nil {
		return nil, nil, false, false, false, err
	}
	truncated := len(rows) > 2000
	if truncated {
		rows = rows[:2000]
	}
	grouped := map[string][]adminInstallationRuntimeEvidence{}
	runtimeIDs := []string{}
	for _, row := range rows {
		item := adminInstallationRuntimeEvidence{RuntimeID: uuidToString(row.RuntimeID), BindingID: uuidToString(row.BindingID), WorkspaceID: uuidToString(row.WorkspaceID), PrincipalID: uuidToString(row.PrincipalUserID), Provider: row.Provider, Status: row.RuntimeStatus, OfflineCode: row.OfflineCode, Capability: row.CapabilityVersion, Authorized: row.Authorized && !auth.IsTemporarilyDisabledUser(uuidToString(row.PrincipalUserID), row.PrincipalEmail), LastSeen: row.LastSeenAt, BindingSeen: row.BindingSeenAt, RunningTasks: row.RunningTasks}
		grouped[uuidToString(row.InstallationID)] = append(grouped[uuidToString(row.InstallationID)], item)
		if row.RuntimeID.Valid {
			runtimeIDs = append(runtimeIDs, item.RuntimeID)
		}
	}
	configured := h.LivenessStore != nil && h.LivenessStore.Available()
	alive := map[string]bool{}
	storeOK := true
	if configured && len(runtimeIDs) > 0 {
		alive, storeOK = h.LivenessStore.IsAliveBatch(ctx, runtimeIDs)
	}
	return grouped, alive, configured, storeOK, truncated, nil
}

func installationReadResponse(row db.ListAdminInstallationsRow, now time.Time, evidence []adminInstallationRuntimeEvidence, alive map[string]bool, configured, storeOK, truncated, canWrite bool) map[string]any {
	client, daemon, ready := installationAxes(now, row.Lifecycle, row.Admission, row.ClientSeenAt, evidence, alive, configured, storeOK)
	if truncated {
		daemon = installationAxis("unknown", "none", "unknown", "evidence_truncated", pgtype.Timestamptz{})
		if ready.State != "stopped" {
			ready = installationAxis("unknown", "none", "unknown", "evidence_truncated", pgtype.Timestamptz{})
		}
	}
	return map[string]any{"id": uuidToString(row.ID), "deployment_id": uuidToString(row.DeploymentID), "organization_id": uuidToString(row.OrganizationID), "lifecycle": row.Lifecycle, "responsible_user_id": uuidToPtr(row.ResponsibleUserID), "display_name": row.DisplayName, "groups": row.Groups, "desktop_version": textToPtr(row.DesktopVersion), "os": textToPtr(row.Os), "admission": row.Admission, "admission_version": strconv.FormatInt(row.AdmissionVersion, 10), "created_at": timestampToString(row.CreatedAt), "updated_at": timestampToString(row.UpdatedAt), "runtime_count": row.RuntimeCount, "binding_count": row.BindingCount, "client_activity": client, "daemon_reachability": daemon, "execution_readiness": ready, "allowed_actions": installationAdmissionActions(row, canWrite)}
}

func installationListParams(identity service.PlatformAdminIdentity, deployment pgtype.UUID, p AdminListQuery, now time.Time) db.ListAdminInstallationsParams {
	return db.ListAdminInstallationsParams{OrganizationID: identity.OrganizationID, DeploymentID: deployment, AsOf: adminTimestamp(p.AsOf), TimeFrom: adminTimestamp(p.From), TimeTo: adminTimestamp(p.To), ObservedAt: adminTimestamp(now), Lifecycle: p.Filters["lifecycle"], Version: p.Filters["version"], Os: p.Filters["os"], GroupName: p.Filters["group"], Search: p.Filters["q"], ClientState: p.Filters["client_state"], AfterTime: adminOptionalTimestamp(p.AfterTime), AfterID: adminFilterID(p.AfterID), PageLimit: p.Limit + 1}
}

func (h *Handler) AdminInstallations(w http.ResponseWriter, r *http.Request) {
	identity, deployment, ok := h.installationReadScope(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	p, err := installationReadQuery(r, "installations", identity, deployment, now, []string{"q", "lifecycle", "version", "os", "group", "client_state"})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminInstallations(ctx, installationListParams(identity, deployment, p, now))
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	var next *string
	if len(rows) > int(p.Limit) {
		rows = rows[:p.Limit]
		last := rows[len(rows)-1]
		token, err := p.Cursor(last.CreatedAt.Time, uuidToString(last.ID))
		if err != nil {
			adminServiceError(w, r, err)
			return
		}
		next = &token
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	evidence, alive, configured, storeOK, truncated, err := h.installationEvidence(ctx, identity, deployment, ids)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, installationReadResponse(row, now, evidence[uuidToString(row.ID)], alive, configured, storeOK, truncated, identity.Role == service.PlatformRoleSuperAdmin))
	}
	quality := "complete"
	if truncated {
		quality = "partial"
	}
	if configured && !storeOK {
		quality = "unavailable"
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next, "as_of": p.AsOf.Format(time.RFC3339Nano), "scope": uuidToString(identity.OrganizationID), "data_quality": quality})
}

func (h *Handler) AdminInstallation(w http.ResponseWriter, r *http.Request) {
	identity, deployment, ok := h.installationReadScope(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "installation id")
	if !ok {
		return
	}
	now := time.Now().UTC()
	p := AdminListQuery{Filters: map[string]string{}, From: time.Time{}, To: now, AsOf: now, Limit: 1}
	args := installationListParams(identity, deployment, p, now)
	args.InstallationID = id
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminInstallations(ctx, args)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	if len(rows) == 0 {
		adminError(w, r, 404, "admin_not_found", "The installation was not found")
		return
	}
	evidence, alive, configured, storeOK, truncated, err := h.installationEvidence(ctx, identity, deployment, []pgtype.UUID{id})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	bindings, err := h.Queries.ListAdminInstallationBindings(ctx, db.ListAdminInstallationBindingsParams{ID: id, OrganizationID: identity.OrganizationID, DeploymentID: deployment})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	users, err := h.Queries.ListAdminInstallationUsers(ctx, db.ListAdminInstallationUsersParams{ID: id, OrganizationID: identity.OrganizationID, DeploymentID: deployment})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	more := truncated || len(bindings) > 100 || len(users) > 100
	if len(bindings) > 100 {
		bindings = bindings[:100]
	}
	if len(users) > 100 {
		users = users[:100]
	}
	runtimeItems := []map[string]any{}
	for _, item := range evidence[uuidToString(id)] {
		if item.RuntimeID == "" {
			continue
		}
		runtimeItems = append(runtimeItems, map[string]any{"id": item.RuntimeID, "binding_id": item.BindingID, "workspace_id": item.WorkspaceID, "principal_user_id": item.PrincipalID, "provider": item.Provider, "status": item.Status, "last_seen_at": timestampToPtr(item.LastSeen), "running_tasks": item.RunningTasks})
	}
	writeJSON(w, 200, map[string]any{"installation": installationReadResponse(rows[0], now, evidence[uuidToString(id)], alive, configured, storeOK, truncated, identity.Role == service.PlatformRoleSuperAdmin), "bindings": bindings, "users": users, "runtimes": runtimeItems, "details_truncated": more, "as_of": now.Format(time.RFC3339Nano), "scope": uuidToString(identity.OrganizationID)})
}

func (h *Handler) AdminUnassociatedRuntimes(w http.ResponseWriter, r *http.Request) {
	identity, deployment, ok := h.installationReadScope(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	p, err := installationReadQuery(r, "unassociated-runtimes", identity, deployment, now, []string{"workspace_id"})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	var workspace pgtype.UUID
	if p.Filters["workspace_id"] != "" {
		var ok bool
		workspace, ok = parseUUIDOrBadRequest(w, p.Filters["workspace_id"], "workspace id")
		if !ok {
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminUnassociatedRuntimes(ctx, db.ListAdminUnassociatedRuntimesParams{OrganizationID: identity.OrganizationID, DeploymentID: deployment, AsOf: adminTimestamp(p.AsOf), TimeFrom: adminTimestamp(p.From), TimeTo: adminTimestamp(p.To), WorkspaceID: workspace, AfterTime: adminOptionalTimestamp(p.AfterTime), AfterID: adminFilterID(p.AfterID), PageLimit: p.Limit + 1})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	var next *string
	if len(rows) > int(p.Limit) {
		rows = rows[:p.Limit]
		last := rows[len(rows)-1]
		token, err := p.Cursor(last.CreatedAt.Time, uuidToString(last.ID))
		if err != nil {
			adminServiceError(w, r, err)
			return
		}
		next = &token
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": uuidToString(row.ID), "workspace_id": uuidToString(row.WorkspaceID), "owner_id": uuidToPtr(row.OwnerID), "provider": row.Provider, "status": row.Status, "last_seen_at": timestampToPtr(row.LastSeenAt), "created_at": timestampToString(row.CreatedAt), "association": "unassociated", "allowed_actions": []string{}})
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next, "as_of": p.AsOf.Format(time.RFC3339Nano), "scope": uuidToString(identity.OrganizationID), "data_quality": "complete"})
}

func installationAdmissionActions(row db.ListAdminInstallationsRow, canWrite bool) []string {
	if !canWrite || row.Lifecycle != "active" {
		return []string{}
	}
	if row.Admission == "accepting" {
		return []string{"stop_admission"}
	}
	if row.Admission == "stopped" {
		return []string{"resume_admission"}
	}
	return []string{}
}
