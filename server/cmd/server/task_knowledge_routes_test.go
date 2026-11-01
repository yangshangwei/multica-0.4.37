package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestTaskKnowledgeProductionRoutesKeepMachineScopeAndClosedGate(t *testing.T) {
	f := newPlatformAdminRouterFixture(t)
	user := f.user(t, "")
	workspace := f.fx.Workspace(t, "Knowledge route scope", "knowledge-router-"+uuid.NewString())
	f.fx.WorkspaceID, f.fx.UserID = workspace, user
	f.fx.Member(t, workspace, user, "owner")
	var org string
	f.fx.QueryRow(t, `SELECT id FROM organization LIMIT 1`).Scan(&org)
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": org, "workspace_id": workspace}, "workspace_id=$1", workspace)
	daemon := "knowledge-router-" + uuid.NewString()
	runtime := f.fx.Runtime(t, "Knowledge route runtime", testutil.Cols{"daemon_id": daemon, "provider": "claude", "runtime_mode": "local"})
	agentID := f.fx.Agent(t, "Knowledge route agent", runtime)
	contextBody, err := json.Marshal(service.QuickCreateContext{Type: service.QuickCreateContextType, WorkspaceID: workspace, RequesterID: user, Prompt: "Route scope fixture"})
	if err != nil {
		t.Fatal(err)
	}
	id := f.fx.Task(t, agentID, testutil.Cols{"runtime_id": runtime, "status": "dispatched", "claim_generation": 1, "dispatched_at": time.Now(), "context": contextBody})
	task, err := db.New(f.pool).GetAgentTask(t.Context(), util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	token := "mdt_" + uuid.NewString()
	f.fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(token), "workspace_id": workspace, "daemon_id": daemon, "user_id": user, "auth_version": 1, "expires_at": testutil.Raw("now()+interval '1 hour'")})
	session := protocol.TaskKnowledgeSessionBinding{Provider: "claude", RuntimeID: runtime, AgentID: agentID, ConfigurationID: "route-config", ScopeKind: "quick_create", ScopeID: id, Generation: uuid.NewString()}
	base := protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: "1", DispatchedAt: task.DispatchedAt.Time.UTC().Format(time.RFC3339Nano), Session: session, InvocationFingerprint: strings.Repeat("a", 64)}
	cases := []struct {
		path, code string
		body       any
	}{
		{"prepare", "knowledge_invocation_unsupported", base},
		{"authorize", "knowledge_invocation_unsupported", protocol.TaskKnowledgeAuthorizeRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: uuid.NewString(), ClaimGeneration: "1", DispatchedAt: base.DispatchedAt, Session: session, InvocationFingerprint: base.InvocationFingerprint, ManifestSHA256: strings.Repeat("b", 64), PromptSHA256: strings.Repeat("c", 64), AttemptSequence: 1, AttemptNonce: uuid.NewString()}},
		{"observations", "knowledge_context_changed", protocol.TaskKnowledgeObservation{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: uuid.NewString(), ClaimGeneration: "1", DispatchedAt: base.DispatchedAt, AttemptSequence: 1, AttemptNonce: uuid.NewString(), Kind: "started", ProcessID: 42, ProcessInstanceID: "fixture-process"}},
	}
	human := platformRouterJWT(t, user, nil)
	for _, tc := range cases {
		path := "/api/daemon/tasks/" + id + "/knowledge-context/" + tc.path
		denied := f.request(t, human, false, "POST", path, tc.body, nil).Want(403)
		if !strings.Contains(denied.Text(), "knowledge_authorization_denied") {
			t.Fatalf("human credential did not reach the narrow boundary: %s", denied.Text())
		}
		response := f.request(t, token, false, "POST", path, tc.body, nil).Want(409)
		if !strings.Contains(response.Text(), tc.code) || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Text(), "contributions") {
			t.Fatalf("production route lost its scoped/gated contract: %s", response.Text())
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND knowledge_delivery IS NOT NULL`, id) != 0 {
		t.Fatal("closed gate or unadmitted observation wrote knowledge metadata")
	}
}
