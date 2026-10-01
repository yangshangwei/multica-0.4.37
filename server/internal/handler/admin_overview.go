package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func adminWindow(p AdminListQuery) map[string]string {
	return map[string]string{"time_from": p.From.Format(time.RFC3339Nano), "time_to": p.To.Format(time.RFC3339Nano), "timezone": p.Timezone}
}
func adminRatio(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}
func adminDuration(value float64) *float64 {
	if value < 0 {
		return nil
	}
	return &value
}
func adminReportedCount(value, reported int64) *int64 {
	if reported == 0 {
		return nil
	}
	return &value
}

func adminOverviewExecutionMetrics(row db.GetAdminOverviewExecutionsRow) (map[string]any, map[string]any, string) {
	unknownQueue := row.FinishedCount - row.QueueSamples - row.QueueLowerBoundSamples
	quality := "complete"
	if row.UnknownStatus > 0 || row.MissingTasks > 0 || unknownQueue > 0 || row.QueueLowerBoundSamples > 0 || row.Unfinished != row.Queued+row.Dispatched+row.Running+row.WaitingLocalDirectory+row.Deferred {
		quality = "partial"
	}
	executions := map[string]any{"completed": row.Completed, "failed": row.Failed, "cancelled": row.Cancelled, "unfinished": row.Unfinished, "queued": row.Queued, "dispatched": row.Dispatched, "running": row.Running, "waiting_local_directory": row.WaitingLocalDirectory, "deferred": row.Deferred, "success_rate": adminRatio(row.Completed, row.Completed+row.Failed), "snapshot": "current", "time_basis": "finished",
		"queue_seconds": map[string]any{"p50": adminDuration(row.QueueP50), "p95": adminDuration(row.QueueP95), "samples": row.QueueSamples, "lower_bound_samples": row.QueueLowerBoundSamples, "unknown_samples": unknownQueue},
		"run_seconds":   map[string]any{"p50": adminDuration(row.RunP50), "p95": adminDuration(row.RunP95), "samples": row.RunSamples}}
	usageQuality := "complete"
	if row.MissingTasks > 0 {
		usageQuality = "partial"
	}
	if row.ReportedTasks == 0 {
		usageQuality = "unknown"
	}
	usage := map[string]any{"input_tokens": adminReportedCount(row.InputTokens, row.ReportedTasks), "output_tokens": adminReportedCount(row.OutputTokens, row.ReportedTasks), "cache_read_tokens": adminReportedCount(row.CacheReadTokens, row.ReportedTasks), "cache_write_tokens": adminReportedCount(row.CacheWriteTokens, row.ReportedTasks), "total_tokens": adminReportedCount(row.InputTokens+row.OutputTokens+row.CacheReadTokens+row.CacheWriteTokens, row.ReportedTasks), "missing_tasks": row.MissingTasks, "unpriced_tasks": row.UnpricedTasks, "quality": usageQuality, "billing": "tokens_only"}
	return executions, usage, quality
}

func (h *Handler) overviewInstallationReadiness(ctx context.Context, identity service.PlatformAdminIdentity, deployment pgtype.UUID, asOf time.Time) (*int64, *int64, error) {
	installations, err := h.Queries.ListAdminOverviewInstallations(ctx, db.ListAdminOverviewInstallationsParams{OrganizationID: identity.OrganizationID, DeploymentID: deployment})
	if err != nil {
		return nil, nil, err
	}
	evidence, err := h.Queries.ListAdminOverviewEvidence(ctx, db.ListAdminOverviewEvidenceParams{OrganizationID: identity.OrganizationID, DeploymentID: deployment, AsOf: adminTimestamp(asOf)})
	if err != nil {
		return nil, nil, err
	}
	if len(installations) > 10000 || len(evidence) > 10000 {
		return nil, nil, nil
	}
	grouped := map[string][]adminInstallationRuntimeEvidence{}
	ids := []string{}
	for _, row := range evidence {
		key := uuidToString(row.InstallationID)
		grouped[key] = append(grouped[key], adminInstallationRuntimeEvidence{RuntimeID: uuidToString(row.RuntimeID), BindingID: uuidToString(row.BindingID), WorkspaceID: uuidToString(row.WorkspaceID), PrincipalID: uuidToString(row.PrincipalUserID), Provider: row.Provider, Status: row.RuntimeStatus, OfflineCode: row.OfflineCode, Capability: row.CapabilityVersion, Authorized: row.Authorized && !auth.IsTemporarilyDisabledUser(uuidToString(row.PrincipalUserID), row.PrincipalEmail), LastSeen: row.LastSeenAt, BindingSeen: row.BindingSeenAt})
		if row.RuntimeID.Valid {
			ids = append(ids, uuidToString(row.RuntimeID))
		}
	}
	configured := h.LivenessStore != nil && h.LivenessStore.Available()
	storeOK := !configured
	alive := map[string]bool{}
	if configured {
		alive, storeOK = h.LivenessStore.IsAliveBatch(ctx, ids)
	}
	reachable, ready := int64(0), int64(0)
	reachableKnown, readyKnown := true, true
	for _, installation := range installations {
		if installation.Lifecycle != "active" {
			continue
		}
		_, daemon, execution := installationAxes(asOf, installation.Lifecycle, installation.Admission, installation.ClientSeenAt, grouped[uuidToString(installation.ID)], alive, configured, storeOK)
		if daemon.State == "reachable" {
			reachable++
		}
		if execution.State == "ready" {
			ready++
		}
		if daemon.State == "unknown" || daemon.State == "unavailable" {
			reachableKnown = false
		}
		if execution.State == "unknown" || execution.State == "unavailable" {
			readyKnown = false
		}
	}
	var reachableResult, readyResult *int64
	if reachableKnown {
		reachableResult = &reachable
	}
	if readyKnown {
		readyResult = &ready
	}
	return reachableResult, readyResult, nil
}

func (h *Handler) AdminOverview(w http.ResponseWriter, r *http.Request) {
	identity, deployment, ok := h.installationReadScope(w, r)
	if !ok {
		return
	}
	p, err := ParseAdminListQuery(r, AdminListScope{Resource: "overview", ActorID: uuidToString(identity.UserID), OrganizationID: uuidToString(identity.OrganizationID), Window: 31 * 24 * time.Hour}, nil, time.Now().UTC())
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	if p.AfterID != "" {
		adminServiceError(w, r, adminQueryError())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	outcomes, err := h.Queries.GetAdminOverviewExecutions(ctx, db.GetAdminOverviewExecutionsParams{OrganizationID: identity.OrganizationID, TimeFrom: adminTimestamp(p.From), TimeTo: adminTimestamp(p.To), AsOf: adminTimestamp(p.AsOf)})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	counts, err := h.Queries.GetAdminOverviewCounts(ctx, db.GetAdminOverviewCountsParams{TimeFrom: adminTimestamp(p.From), TimeTo: adminTimestamp(p.To), OrganizationID: identity.OrganizationID, DeploymentID: deployment, AsOf: adminTimestamp(p.AsOf)})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	reachable, ready, err := h.overviewInstallationReadiness(ctx, identity, deployment, p.AsOf)
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	executions, usage, quality := adminOverviewExecutionMetrics(outcomes)
	if reachable == nil || ready == nil {
		quality = "partial"
	}
	writeJSON(w, 200, map[string]any{"scope": uuidToString(identity.OrganizationID), "as_of": p.AsOf.Format(time.RFC3339Nano), "window": adminWindow(p), "rule_version": "1", "data_quality": quality,
		"installations": map[string]any{"total": counts.Installations, "retired": counts.Retired, "client_active": counts.ClientActive, "daemon_reachable": reachable, "ready": ready, "unassociated": counts.Unassociated}, "executions": executions, "usage": usage,
		"alerts": map[string]int64{"open": counts.AlertsOpen, "acknowledged": counts.AlertsAcknowledged, "resolved": counts.AlertsResolved, "closed": counts.AlertsClosed}})
}
