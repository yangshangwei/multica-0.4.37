package daemonws

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/auth"
	"io"
	"net"
	"testing"
)

func TestConnectionErrorClassDoesNotExposePeerOrCredentialDetails(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		class string
		code  int
	}{
		{"authorization deadline", fmt.Errorf("query: %w", context.DeadlineExceeded), "timeout", 0},
		{"cancelled", context.Canceled, "cancelled", 0},
		{"session revoked", auth.ErrPasswordSession, "session_invalid", 0},
		{"runtime moved", ErrRuntimeScope, "runtime_scope_invalid", 0},
		{"closed socket", net.ErrClosed, "closed", 0},
		{"end of stream", io.EOF, "eof", 0},
		{"peer text", &websocket.CloseError{Code: 1008, Text: "secret-peer-text"}, "peer_close", 1008},
		{"opaque failure", errors.New("https://user:secret@example.invalid/?token=private"), "unavailable", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class, code := connectionErrorClass(tt.err)
			if class != tt.class || code != tt.code {
				t.Fatalf("classification = %q/%d, want %q/%d", class, code, tt.class, tt.code)
			}
		})
	}
}
