package handler

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"strings"

	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/middleware"
)

func (h *Handler) DaemonWebSocket(w http.ResponseWriter, r *http.Request) {
	if h.DaemonHub == nil {
		writeError(w, http.StatusServiceUnavailable, "daemon websocket unavailable")
		return
	}

	runtimeIDs := parseRuntimeIDs(r)
	userID := requestUserID(r)
	if len(runtimeIDs) == 0 && userID == "" {
		writeError(w, http.StatusBadRequest, "runtime_ids or user identity required")
		return
	}

	workspaceIDs := make([]string, 0, len(runtimeIDs))
	seenWorkspaceIDs := make(map[string]struct{}, len(runtimeIDs))
	for _, runtimeID := range runtimeIDs {
		rt, ok := h.requireDaemonRuntimeAccess(w, r, runtimeID)
		if !ok {
			return
		}
		if daemonID := middleware.DaemonIDFromContext(r.Context()); daemonID != "" && rt.DaemonID.Valid && rt.DaemonID.String != daemonID {
			writeError(w, http.StatusNotFound, "runtime not found")
			return
		}
		workspaceID := uuidToString(rt.WorkspaceID)
		if workspaceID != "" {
			if _, ok := seenWorkspaceIDs[workspaceID]; !ok {
				seenWorkspaceIDs[workspaceID] = struct{}{}
				workspaceIDs = append(workspaceIDs, workspaceID)
			}
		}
	}

	if source, ok := auth.PasswordSessionFromContext(r.Context()); ok && source.Kind == "daemon_token" && source.WorkspaceID != "" {
		for _, workspaceID := range workspaceIDs {
			if workspaceID != source.WorkspaceID {
				writeError(w, 403, "bound daemon workspace mismatch")
				return
			}
		}
		workspaceIDs = []string{source.WorkspaceID}
	}
	primaryWorkspaceID := ""
	if len(workspaceIDs) > 0 {
		primaryWorkspaceID = workspaceIDs[0]
	}

	h.DaemonHub.HandleWebSocket(w, r, daemonws.ClientIdentity{
		DaemonID:      middleware.DaemonIDFromContext(r.Context()),
		UserID:        userID,
		WorkspaceID:   primaryWorkspaceID,
		WorkspaceIDs:  workspaceIDs,
		RuntimeIDs:    runtimeIDs,
		ClientVersion: r.Header.Get("X-Client-Version"),
		Capabilities:  r.Header.Get("X-Client-Capabilities"),
	})
}

func parseRuntimeIDs(r *http.Request) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			id := strings.TrimSpace(part)
			if parsed, err := util.ParseUUID(id); err == nil {
				id = uuidToString(parsed)
			}
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	for _, raw := range r.URL.Query()["runtime_id"] {
		add(raw)
	}
	for _, raw := range r.URL.Query()["runtime_ids"] {
		add(raw)
	}
	return out
}

// AuthorizeDaemonConnection runs for upgrade and every inbound/outbound frame.
// Current database rows, not cached runtime snapshots or claimed headers, own
// the connection's managed scope.
func (h *Handler) AuthorizeDaemonConnection(ctx context.Context, identity daemonws.ClientIdentity) error {
	var managedBinding db.InstallationDaemonBinding
	source, hasSource := auth.PasswordSessionFromContext(ctx)
	if hasSource && identity.UserID != "" && identity.UserID != source.UserID {
		return daemonws.ErrRuntimeScope
	}
	if hasSource && source.Kind == "daemon_token" && (identity.DaemonID != source.DaemonID || identity.PrimaryWorkspaceID() != source.WorkspaceID) {
		return daemonws.ErrRuntimeScope
	}
	if hasSource && source.BindingID != "" {
		if identity.UserID != source.UserID || identity.DaemonID != source.DaemonID || identity.PrimaryWorkspaceID() != source.WorkspaceID {
			return daemonws.ErrRuntimeScope
		}
		for _, workspace := range identity.AuthorizedWorkspaceIDs() {
			if workspace != source.WorkspaceID {
				return daemonws.ErrRuntimeScope
			}
		}
		workspace, err := util.ParseUUID(source.WorkspaceID)
		if err != nil {
			return daemonws.ErrRuntimeScope
		}
		if managedBinding, err = service.ValidateManagedNamespace(ctx, h.Queries, workspace, source.DaemonID); err != nil {
			if errors.Is(err, service.ErrManagedRuntimeSource) {
				return daemonws.ErrRuntimeScope
			}
			return err
		}
	}
	for _, id := range identity.RuntimeIDs {
		runtimeID, err := util.ParseUUID(id)
		if err != nil {
			return daemonws.ErrRuntimeScope
		}
		runtime, err := h.getAgentRuntime(ctx, obsmetrics.RuntimeLookupSourceDaemonAPI, runtimeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return daemonws.ErrRuntimeScope
		}
		if err != nil {
			return err
		}
		if !identity.AllowsWorkspace(uuidToString(runtime.WorkspaceID)) {
			return daemonws.ErrRuntimeScope
		}
		if managedBinding.ID.Valid {
			// The namespace is checked afresh above for this authorization call.
			// Each runtime is then read from the database and checked against
			// that binding without repeating the same namespace/account reads.
			if runtime.OwnerID != managedBinding.PrincipalUserID || runtime.WorkspaceID != managedBinding.WorkspaceID || runtime.DaemonID.String != managedBinding.DaemonID {
				return daemonws.ErrRuntimeScope
			}
			continue
		}
		if _, err = service.ValidateManagedRuntime(ctx, h.runtimeLookup(obsmetrics.RuntimeLookupSourceDaemonAPI), runtime); err != nil {
			if errors.Is(err, service.ErrManagedRuntimeSource) {
				return daemonws.ErrRuntimeScope
			}
			return err
		}
	}
	return nil
}
