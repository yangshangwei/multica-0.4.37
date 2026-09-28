package main

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestBuiltinAgentAvatarRouteIsPublic(t *testing.T) {
	// Exercise the production router: this static path must win over the
	// signed /api/avatars/{sig}/* route without requiring authentication.
	req := testutil.JSONRequest(http.MethodGet, "/api/avatars/builtin/afu-seal-v1.png", nil)
	res := testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(http.StatusOK)
	if got := res.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("public avatar Content-Type = %q, want image/png", got)
	}
}
