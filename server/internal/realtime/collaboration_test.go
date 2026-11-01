package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/auth"
)

type collaborationScopeAccess struct {
	mu      sync.Mutex
	allowed bool
	failed  error
}

func (a *collaborationScopeAccess) AuthorizeScope(ctx context.Context, user, workspace, scope, id string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := auth.AuthenticatedPrincipalFromContext(ctx)
	if !ok || p.UserID != user || p.Kind != auth.CredentialJWT {
		return false, nil
	}
	if scope == "account" {
		return id == user, nil
	}
	return scope == "issue" && a.allowed, a.failed
}

func (a *collaborationScopeAccess) revoke() {
	a.mu.Lock()
	a.allowed = false
	a.mu.Unlock()
}

type noWorkspaceAccess struct{ t *testing.T }

func (m noWorkspaceAccess) IsMember(context.Context, string, string) bool {
	m.t.Error("account connection attempted workspace membership")
	return false
}

func accountTestSocket(t *testing.T, authorizer ScopeAuthorizer) (*Hub, *websocket.Conn) {
	t.Helper()
	hub := NewHub()
	hub.SetAuthorizer(authorizer)
	return hub, accountTestSocketOnHub(t, hub)
}

// accountTestSocketOnHub lets a test configure the hub before Run and before
// the client's pumps start.
func accountTestSocketOnHub(t *testing.T, hub *Hub) *websocket.Conn {
	t.Helper()
	go hub.Run()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, noWorkspaceAccess{t}, nil, nil, w, r)
	}))
	t.Cleanup(server.Close)
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?scope=account", nil)
	if err != nil {
		if response != nil {
			t.Fatalf("account connection rejected: %s: %v", response.Status, err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.WriteJSON(map[string]any{"type": "auth", "payload": map[string]string{"token": makeTestToken(t)}}); err != nil {
		t.Fatal(err)
	}
	if frame := readAccountFrame(t, conn); frame.Type != "auth_ack" {
		t.Fatalf("expected authenticated account, got %+v", frame)
	}
	return conn
}

type accountFrame struct {
	Type    string            `json:"type"`
	Payload map[string]string `json:"payload"`
}

func readAccountFrame(t *testing.T, conn *websocket.Conn) accountFrame {
	t.Helper()
	frame, err := tryReadAccountFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func tryReadAccountFrame(conn *websocket.Conn) (accountFrame, error) {
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return accountFrame{}, err
	}
	var frame accountFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return accountFrame{}, fmt.Errorf("invalid frame: %s: %w", data, err)
	}
	return frame, nil
}

// expectLiveSocket proves the connection survived: a ping must still be
// answered after the frame under test.
func expectLiveSocket(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
		t.Fatalf("socket closed after the frame: %v", err)
	}
	if frame, err := tryReadAccountFrame(conn); err != nil || frame.Type != "pong" {
		t.Fatalf("socket did not survive the frame: %+v %v", frame, err)
	}
}

func subscribeAccountScope(t *testing.T, conn *websocket.Conn, scope, id string) accountFrame {
	t.Helper()
	if err := conn.WriteJSON(map[string]any{"type": "subscribe", "payload": map[string]string{"scope": scope, "id": id}}); err != nil {
		t.Fatal(err)
	}
	return readAccountFrame(t, conn)
}

func TestAccountWebSocketDoesNotJoinWorkspaceRooms(t *testing.T) {
	hub, conn := accountTestSocket(t, &collaborationScopeAccess{allowed: true})
	if f := subscribeAccountScope(t, conn, "account", testUserID); f.Type != "subscribe_ack" {
		t.Fatalf("own account subscription failed: %+v", f)
	}
	if f := subscribeAccountScope(t, conn, "account", "another-user"); f.Type != "subscribe_error" || f.Payload["error"] != "forbidden" {
		t.Fatalf("another account was exposed: %+v", f)
	}
	if f := subscribeAccountScope(t, conn, "workspace", testWorkspaceID); f.Type != "subscribe_error" {
		t.Fatalf("account connection acquired a workspace: %+v", f)
	}
	if f := subscribeAccountScope(t, conn, "issue", "00000000-0000-4000-8000-000000000001"); f.Type != "subscribe_ack" {
		t.Fatalf("shared issue subscription failed: %+v", f)
	}
	if hub.HasLocalSubscribers("workspace", testWorkspaceID) || hub.HasLocalSubscribers("workspace", "") {
		t.Fatal("account client joined a workspace room")
	}
}

func TestCollaborationWebSocketRevocationBeforeSend(t *testing.T) {
	access := &collaborationScopeAccess{allowed: true}
	hub, conn := accountTestSocket(t, access)
	issue := "00000000-0000-4000-8000-000000000001"
	if f := subscribeAccountScope(t, conn, "issue", issue); f.Type != "subscribe_ack" {
		t.Fatalf("subscribe failed: %+v", f)
	}
	access.revoke()
	hub.BroadcastToScope("issue", issue, []byte(`{"type":"collaboration:changed","payload":{"private":"must-not-be-sent"}}`))
	f := readAccountFrame(t, conn)
	if f.Type != "collaboration:access_changed" || f.Payload["issue_id"] != issue || len(f.Payload) != 1 {
		t.Fatalf("revocation must send only a scoped invalidation: %+v", f)
	}
	if hub.HasLocalSubscribers("issue", issue) {
		t.Fatal("revoked issue subscription retained")
	}
	if f := subscribeAccountScope(t, conn, "issue", issue); f.Type != "subscribe_error" || f.Payload["error"] != "forbidden" {
		t.Fatalf("revoked issue was subscribed again: %+v", f)
	}
	// Losing one issue does not tear down the account or other lawful scopes.
	if f := subscribeAccountScope(t, conn, "account", testUserID); f.Type != "subscribe_ack" {
		t.Fatalf("valid account was lost with issue access: %+v", f)
	}
}

func TestAccountWebSocketRequiresAnAuthorizer(t *testing.T) {
	hub := NewHub()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, noWorkspaceAccess{t}, nil, nil, w, r)
	}))
	defer server.Close()
	headers := http.Header{"Cookie": {auth.AuthCookieName + "=" + makeTestToken(t)}}
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?scope=account", headers)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("account without an authorizer must fail closed: %v %v", response, err)
	}
}

func TestAccountWebSocketUnknownFrameDoesNotLogPrivateInput(t *testing.T) {
	var logs bytes.Buffer
	var mu sync.Mutex
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&lockedWriter{w: &logs, mu: &mu}, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	_, conn := accountTestSocket(t, &collaborationScopeAccess{allowed: true})
	if err := conn.WriteJSON(map[string]string{"type": "private-attachment-capability-sentinel"}); err != nil {
		t.Fatal(err)
	}
	// A following pong proves that the unknown frame was processed first.
	if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
		t.Fatal(err)
	}
	if frame := readAccountFrame(t, conn); frame.Type != "pong" {
		t.Fatalf("unexpected frame after ignored input: %s", frame.Type)
	}
	mu.Lock()
	captured := logs.String()
	mu.Unlock()
	if strings.Contains(captured, "private-attachment-capability-sentinel") {
		t.Fatal("unknown account frame copied private input into diagnostics")
	}
}

// slowIssueAccess delays the next issue authorization by a chosen duration.
// Account and subscribe checks stay fast.
type slowIssueAccess struct {
	collaborationScopeAccess
	gate  sync.Mutex
	delay time.Duration
}

func (a *slowIssueAccess) slowNext(delay time.Duration) {
	a.gate.Lock()
	a.delay = delay
	a.gate.Unlock()
}

func (a *slowIssueAccess) AuthorizeScope(ctx context.Context, user, workspace, scope, id string) (bool, error) {
	if scope == ScopeIssue {
		a.gate.Lock()
		delay := a.delay
		a.delay = 0
		a.gate.Unlock()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}
	}
	return a.collaborationScopeAccess.AuthorizeScope(ctx, user, workspace, scope, id)
}

// The access hint written from outboundScopeCurrent is a physical write of its
// own. A revocation check slower than the write budget must not spend it.
func TestCollaborationSlowRevocationCheckStillSendsAccessHint(t *testing.T) {
	const writeBudget = 200 * time.Millisecond
	access := &slowIssueAccess{collaborationScopeAccess: collaborationScopeAccess{allowed: true}}
	hub := NewHub()
	hub.SetAuthorizer(access)
	hub.writeTimeout = writeBudget
	conn := accountTestSocketOnHub(t, hub)
	issue := "00000000-0000-4000-8000-000000000001"
	if f := subscribeAccountScope(t, conn, "issue", issue); f.Type != "subscribe_ack" {
		t.Fatalf("subscribe failed: %+v", f)
	}
	access.revoke()
	access.slowNext(3 * writeBudget)
	hub.BroadcastToScope("issue", issue, []byte(`{"type":"collaboration:changed","payload":{"private":"must-not-be-sent"}}`))
	f, err := tryReadAccountFrame(conn)
	if err != nil {
		t.Fatalf("access hint lost behind a revocation check slower than the %v write budget: %v", writeBudget, err)
	}
	if f.Type != "collaboration:access_changed" || f.Payload["issue_id"] != issue || len(f.Payload) != 1 {
		t.Fatalf("revocation must send only a scoped invalidation: %+v", f)
	}
	expectLiveSocket(t, conn)
}
