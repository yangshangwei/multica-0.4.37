package handler

import (
	"net/http"
	"time"
)

// AdminReadConfig contains only the deployment settings this read API exposes.
// Nil retention values mean unconfigured; no value schedules deletion.
type AdminReadConfig struct {
	Configured                  bool
	AuthMode                    string
	RegistrationEnabled         bool
	RegistrationPolicy          string
	ManagedInstallationsEnabled bool
	WorkspaceCreationEnabled    bool
	ConfirmedOperationsDays     *int
	AlertsDays                  *int
	AuditDays                   *int
}

func positiveAdminDays(value *int) *int {
	if value == nil || *value < 1 {
		return nil
	}
	return value
}
func (h *Handler) AdminSettings(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		adminServiceError(w, r, adminQueryError())
		return
	}
	config := h.AdminReadConfig
	configuration := map[string]any{"auth_mode": nil, "registration_enabled": nil, "registration_policy": nil, "managed_installations_enabled": nil, "workspace_creation_enabled": nil, "read_only": true, "source": "unknown"}
	quality := "unknown"
	if config.Configured {
		configuration = map[string]any{"auth_mode": config.AuthMode, "registration_enabled": config.RegistrationEnabled, "registration_policy": config.RegistrationPolicy, "managed_installations_enabled": config.ManagedInstallationsEnabled, "workspace_creation_enabled": config.WorkspaceCreationEnabled, "read_only": true, "source": "deployment"}
		quality = "complete"
	}
	operations, alerts, audit := positiveAdminDays(config.ConfirmedOperationsDays), positiveAdminDays(config.AlertsDays), positiveAdminDays(config.AuditDays)
	policy := "configured"
	if operations == nil || alerts == nil || audit == nil {
		policy = "unknown"
		if quality == "complete" {
			quality = "partial"
		}
	}
	writeJSON(w, 200, map[string]any{"scope": uuidToString(identity.OrganizationID), "as_of": time.Now().UTC().Format(time.RFC3339Nano), "data_quality": quality, "configuration": configuration,
		"retention": map[string]any{"confirmed_operations_days": operations, "alerts_days": alerts, "audit_days": audit, "automatic_deletion_enabled": false, "policy_source": policy}, "refresh_intervals_seconds": map[string]int{"list": 15, "detail": 5}})
}
