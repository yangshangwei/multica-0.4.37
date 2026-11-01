package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
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

func TestCollaborationKnowledgeRuntimeProductionManagedRoutesRemainDefaultClosed(t *testing.T) {
	f := newCollaborationRouteFixture(t)
	run := uuid.NewString()
	p := protocol.CollaborationRunKnowledgePrepare{ProtocolVersion: 1, Identity: protocol.CollaborationRunIdentity{RunID: run}, RequestID: uuid.NewString(), HandoffID: uuid.NewString(), InvocationFingerprint: strings.Repeat("a", 64)}
	for _, tc := range []struct {
		action string
		body   any
	}{
		{"prepare", p},
		{"authorize", protocol.CollaborationRunKnowledgeAuthorization{CollaborationRunKnowledgePrepare: p, ManifestSHA256: strings.Repeat("b", 64), PromptSHA256: strings.Repeat("c", 64), AttemptSeq: "1", AttemptNonce: uuid.NewString()}},
	} {
		response := f.managed.request(t, f.binding.DaemonToken, false, "POST", "/api/collaboration/runtime/runs/"+run+"/knowledge-context/"+tc.action, tc.body, nil).Want(409)
		if response.Map()["code"] != "knowledge_invocation_unsupported" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("managed route did not enforce instance gate")
		}
	}
}
