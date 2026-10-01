package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/multica-ai/multica/server/internal/auth"
	"net/http"

	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/util"
)

// rpcResponseCapture is a minimal in-memory http.ResponseWriter so a WS RPC can
// reuse an existing HTTP handler without a network round trip.
type rpcResponseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *rpcResponseCapture) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *rpcResponseCapture) WriteHeader(code int) { w.status = code }

func (w *rpcResponseCapture) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(b)
}

// DaemonRPCHandler is the daemonws.RPCHandler wired into the WS hub (MUL-4257).
// It dispatches a generic daemon:rpc_request to the matching HTTP handler,
// reusing all of its auth / payload-building / finalization logic by driving it
// with a synthetic in-process request carrying the WS connection's identity.
// The connection is already authenticated and pinned to its daemon + runtime
// set at connect, so the reused handler's per-runtime daemon/workspace checks
// see the same scope as the HTTP path.
func (h *Handler) DaemonRPCHandler(ctx context.Context, identity daemonws.ClientIdentity, method string, body json.RawMessage) (int, json.RawMessage, error) {
	if err := h.AuthorizeDaemonConnection(ctx, identity); err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, daemonws.ErrRuntimeScope) {
			status = http.StatusForbidden
		} else if errors.Is(err, auth.ErrPasswordSession) {
			status = http.StatusUnauthorized
		}
		return status, nil, err
	}
	switch method {
	case "tasks.claim":
		return h.rpcClaimTasks(ctx, identity, body)
	default:
		return http.StatusNotFound, nil, fmt.Errorf("unknown rpc method %q", method)
	}
}

func (h *Handler) rpcClaimTasks(ctx context.Context, identity daemonws.ClientIdentity, body json.RawMessage) (int, json.RawMessage, error) {
	if len(body) == 0 {
		body = json.RawMessage("{}")
	}
	if source, ok := auth.PasswordSessionFromContext(ctx); ok && source.BindingID != "" {
		var requested struct {
			RuntimeIDs []string `json:"runtime_ids"`
		}
		if err := json.Unmarshal(body, &requested); err != nil {
			return http.StatusBadRequest, nil, err
		}
		allowed := map[string]bool{}
		for _, id := range identity.RuntimeIDs {
			if parsed, err := util.ParseUUID(id); err == nil {
				id = uuidToString(parsed)
			}
			allowed[id] = true
		}
		for _, id := range requested.RuntimeIDs {
			if parsed, err := util.ParseUUID(id); err == nil {
				id = uuidToString(parsed)
			}
			if !allowed[id] {
				return http.StatusForbidden, nil, daemonws.ErrRuntimeScope
			}
		}
	}
	reqCtx := ctx
	// A daemon-token connection is workspace-scoped: pin the daemon context so
	// the reused handler's daemon_id + workspace checks behave exactly like the
	// HTTP path. A PAT/cloud connection (no daemon id) authorizes per-workspace
	// via membership from X-User-ID instead, so we must NOT set a single-
	// workspace daemon context there (it would reject the daemon's other
	// workspaces).
	if identity.DaemonID != "" {
		reqCtx = middleware.WithDaemonContext(reqCtx, identity.PrimaryWorkspaceID(), identity.DaemonID)
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "/api/daemon/tasks/claim", bytes.NewReader(body))
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if identity.UserID != "" {
		req.Header.Set("X-User-ID", identity.UserID)
	}
	if identity.Capabilities != "" {
		req.Header.Set("X-Client-Capabilities", identity.Capabilities)
	}
	if identity.ClientVersion != "" {
		req.Header.Set("X-Client-Version", identity.ClientVersion)
	}

	rec := &rpcResponseCapture{}
	h.ClaimTasksByRuntime(rec, req)
	status := rec.status
	if status == 0 {
		status = http.StatusOK
	}
	return status, json.RawMessage(rec.body.Bytes()), nil
}
