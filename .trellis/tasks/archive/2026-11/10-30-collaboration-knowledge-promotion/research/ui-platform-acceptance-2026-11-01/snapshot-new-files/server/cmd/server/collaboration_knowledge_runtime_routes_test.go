package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationKnowledgeRuntimeProductionRoutesRejectBusinessCredential(t *testing.T) {
	if testServer == nil || testPool == nil {
		t.Fatal("explicit isolated router fixture required")
	}
	for _, action := range []string{"prepare", "authorize", "status"} {
		method, want := "POST", http.StatusUnauthorized
		if action == "status" {
			method = "GET"
		}
		path := "/api/collaboration/runtime/runs/" + uuid.NewString() + "/knowledge-context/" + action
		for _, token := range []string{testToken, "mcb_" + strings.Repeat("a", 64)} {
			r := testutil.JSONRequest(method, path, map[string]any{"content": "PRIVATE_ROUTER_C4"})
			r.Header.Set("Authorization", "Bearer "+token)
			response := testutil.Call(t, testServer.Config.Handler.ServeHTTP, r).Want(want)
			if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Text(), "PRIVATE_ROUTER_C4") {
				t.Fatal("runtime route lost content-free narrow auth")
			}
		}
	}
}
