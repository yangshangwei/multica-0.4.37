package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeScopeAccess struct {
	mu      sync.Mutex
	allowed bool
	entered chan struct{}
	release chan struct{}
}

func (a *knowledgeScopeAccess) AuthorizeScope(_ context.Context, _, workspace, scope, id string) (bool, error) {
	if scope == ScopeAccount {
		return true, nil
	}
	if scope != ScopeWorkspace || workspace != id {
		return false, nil
	}
	a.mu.Lock()
	entered, release := a.entered, a.release
	a.entered, a.release = nil, nil
	a.mu.Unlock()
	if entered != nil {
		close(entered)
		<-release
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.allowed, nil
}

func TestKnowledgeQueuedTargetHintReauthorizesAtPhysicalSend(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	const workspace = "b736820a-476b-4e30-bbbf-e8f046c54562"
	const deletedProject = "ebd2b8d2-fc55-43a3-a939-972a956b83b4"
	access := &knowledgeScopeAccess{allowed: true}
	hub := NewHub()
	hub.SetAuthorizer(access)
	go hub.Run()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, &mockMembershipChecker{}, nil, nil, w, r)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?workspace_id="+workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = conn.WriteJSON(map[string]any{"type": "auth", "payload": map[string]string{"token": makeTestToken(t)}}); err != nil {
		t.Fatal(err)
	}
	if got := readAccountFrame(t, conn); got.Type != "auth_ack" {
		t.Fatalf("auth: %+v", got)
	}
	// Acknowledged subscription orders publication after registration.
	if err = conn.WriteJSON(map[string]any{"type": "subscribe", "payload": map[string]string{"scope": "workspace", "id": workspace}}); err != nil {
		t.Fatal(err)
	}
	if got := readAccountFrame(t, conn); got.Type != "subscribe_ack" {
		t.Fatalf("subscribe: %+v", got)
	}
	frame, _ := json.Marshal(map[string]any{"type": protocol.EventCollaborationKnowledgeChanged, "payload": protocol.CollaborationKnowledgeTargetHint{TargetKind: "project", TargetID: deletedProject, KnowledgeRevision: "9"}})
	if err = hub.PublishWithID(ScopeWorkspace, workspace, "", frame, "d09b945c-366a-40f4-87a0-1728f53c301a:target"); err != nil {
		t.Fatal(err)
	}
	if got := readAccountFrame(t, conn); got.Type != protocol.EventCollaborationKnowledgeChanged || len(got.Payload) != 3 || got.Payload["target_id"] != deletedProject {
		t.Fatalf("authorized retained target: %+v", got)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	access.mu.Lock()
	access.entered, access.release = entered, release
	access.mu.Unlock()
	if err = hub.PublishWithID(ScopeWorkspace, workspace, "", frame, "56a1c0b5-c7ce-468e-83f7-e071c9f74ce5:target"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("queued frame bypassed final-send authorizer")
	}
	access.mu.Lock()
	access.allowed = false
	access.mu.Unlock()
	close(release)
	// The ordinary marker is behind the refused frame on the same socket. It
	// proves no private target tick leaked without relying on read timeouts.
	hub.BroadcastToWorkspace(workspace, []byte(`{"type":"knowledge-test-marker","payload":{}}`))
	if got := readAccountFrame(t, conn); got.Type != "knowledge-test-marker" {
		t.Fatalf("revoked member received queued hint: %+v", got)
	}
}

func TestKnowledgeOutboundPredicateRejectsPrivateFieldsAndWrongRoom(t *testing.T) {
	const workspace = "b736820a-476b-4e30-bbbf-e8f046c54562"
	hub := NewHub()
	hub.SetAuthorizer(&knowledgeScopeAccess{allowed: true})
	client := &Client{hub: hub, workspaceID: workspace, userID: "viewer", ctx: context.Background()}
	for _, tc := range []struct {
		scope scopeKey
		body  string
	}{
		{scopeKey{ScopeWorkspace, workspace}, `{"type":"collaboration:knowledge_changed","payload":{"target_kind":"workspace","target_id":"b736820a-476b-4e30-bbbf-e8f046c54562","knowledge_revision":"1","source_id":"PRIVATE_SOURCE"}}`},
		{scopeKey{ScopeAccount, "viewer"}, `{"type":"collaboration:knowledge_changed","payload":{"target_kind":"workspace","target_id":"b736820a-476b-4e30-bbbf-e8f046c54562","knowledge_revision":"1"}}`},
		{scopeKey{ScopeWorkspace, workspace}, `{"type":"collaboration:knowledge_changed","payload":{"target_kind":"workspace","target_id":"ebd2b8d2-fc55-43a3-a939-972a956b83b4","knowledge_revision":"1"}}`},
	} {
		if _, allowed := client.outboundKnowledgeFrame(outboundMessage{scope: tc.scope, data: []byte(tc.body)}); allowed {
			t.Fatal("malformed/private or incorrectly routed hint was permitted")
		}
	}
	if _, allowed := client.outboundKnowledgeFrame(outboundMessage{data: []byte(`{"type":"collaboration:workbench_changed","payload":{}}`)}); !allowed {
		t.Fatal("ordinary content-free workbench behavior changed")
	}
}
