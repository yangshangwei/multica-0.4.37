package daemonws

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"

	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/auth"
)

// Classify failures without retaining credential-bearing URLs or peer close text.
func connectionErrorClass(err error) (string, int) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout", 0
	case errors.Is(err, context.Canceled):
		return "cancelled", 0
	case errors.Is(err, auth.ErrPasswordSession):
		return "session_invalid", 0
	case errors.Is(err, ErrRuntimeScope):
		return "runtime_scope_invalid", 0
	case errors.Is(err, net.ErrClosed):
		return "closed", 0
	case errors.Is(err, io.EOF):
		return "eof", 0
	}
	var closed *websocket.CloseError
	if errors.As(err, &closed) {
		return "peer_close", closed.Code
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return "timeout", 0
	}
	return "unavailable", 0
}

func (c *client) logConnectionFailure(phase string, err error) {
	class, code := connectionErrorClass(err)
	slog.Info("daemon websocket connection ended", "daemon_id", c.identity.DaemonID,
		"workspace_id", c.identity.PrimaryWorkspaceID(), "phase", phase,
		"error_class", class, "close_code", code, "runtimes", len(c.runtimes))
}
