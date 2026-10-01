package handler

import (
	"context"
	"net/http"
	"time"
)

type AdminHealthSource struct {
	Name      string     `json:"name"`
	State     string     `json:"state"`
	CheckedAt *time.Time `json:"checked_at"`
	Code      string     `json:"code"`
}

type AdminHealthSnapshot struct{ Sources []AdminHealthSource }

type adminDatabasePinger interface{ Ping(context.Context) error }

func (h *Handler) AdminHealth(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		adminServiceError(w, r, adminQueryError())
		return
	}
	now := time.Now().UTC()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	sources := map[string]AdminHealthSource{}
	for _, name := range []string{"api", "database", "liveness", "task_coordinator", "cancellation_coordinator", "alert_detector"} {
		sources[name] = AdminHealthSource{Name: name, State: "unknown", Code: "not_observed"}
	}
	if h.AdminHealthSnapshot != nil {
		for _, source := range h.AdminHealthSnapshot(ctx).Sources {
			if _, known := sources[source.Name]; !known || source.Name == "api" || source.Name == "database" {
				continue
			}
			if !adminContains([]string{"healthy", "degraded", "unavailable", "stale", "unknown"}, source.State) {
				source.State = "unknown"
			}
			if source.CheckedAt != nil && source.CheckedAt.After(now.Add(time.Minute)) {
				source.CheckedAt = nil
				source.State = "unknown"
			}
			if source.CheckedAt == nil && source.State == "healthy" {
				source.State = "unknown"
			}
			if !adminCodePattern.MatchString(source.Code) {
				source.Code = "unknown"
			}
			sources[source.Name] = source
		}
	}
	sources["api"] = AdminHealthSource{Name: "api", State: "healthy", CheckedAt: &now, Code: "request_served"}
	if database, ok := h.TxStarter.(adminDatabasePinger); ok {
		source := AdminHealthSource{Name: "database", State: "healthy", CheckedAt: &now, Code: "query_succeeded"}
		if err := database.Ping(ctx); err != nil {
			source.State = "unavailable"
			source.Code = "database_unavailable"
		}
		sources["database"] = source
	}
	if source := sources["liveness"]; source.Code == "database_fallback" {
		database := sources["database"]
		source.State, source.CheckedAt = database.State, database.CheckedAt
		sources["liveness"] = source
	}
	list := make([]AdminHealthSource, 0, len(sources))
	quality := "complete"
	for _, name := range []string{"api", "database", "liveness", "task_coordinator", "cancellation_coordinator", "alert_detector"} {
		source := sources[name]
		if source.State != "healthy" {
			quality = "partial"
		}
		list = append(list, source)
	}
	writeJSON(w, 200, map[string]any{"scope": uuidToString(identity.OrganizationID), "as_of": now.Format(time.RFC3339Nano), "data_quality": quality, "sources": list, "detector_state": sources["alert_detector"].State})
}
