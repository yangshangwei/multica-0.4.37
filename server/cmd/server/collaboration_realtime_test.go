package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRealtimeHTTPAndWebSocketShareCurrentAuthorization(t *testing.T) {
	if testPool == nil {
		t.Fatal("isolated database is required")
	}
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", mode)
			fx := testutil.New(testPool, testWorkspaceID, testUserID)
			viewer := fx.User(t, "Realtime viewer", "realtime-"+uuid.NewString()+"@example.test")
			if mode == "password" {
				fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": viewer, "username": "realtime_" + uuid.NewString(), "password_hash": "test-not-used-for-login"}, "user_id=$1", viewer)
			}
			issue := fx.Issue(t, "Realtime authorization", testutil.Cols{"description": "PRIVATE_SHARED_BODY", "assignee_type": "member", "assignee_id": viewer})
			fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
			fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
			fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": viewer}, "issue_id=$1 AND user_id=$2", issue, viewer)
			fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": viewer}, "user_id=$1", viewer)
			token := platformRouterJWT(t, viewer, nil)
			q := db.New(testPool)
			hub := realtime.NewHub()
			hub.SetPasswordQueries(q)
			hub.SetAuthorizer(&collaborationScopeAuthorizer{workspace: newScopeAuthorizer(q), service: service.NewCollaborationService(q, testPool, nil)})
			go hub.Run()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { realtime.HandleWebSocket(hub, nil, nil, nil, w, r) }))
			defer server.Close()
			for _, hint := range []string{"X-Agent-ID", "X-Task-ID"} {
				headers := http.Header{"Cookie": {auth.AuthCookieName + "=" + token}, hint: {uuid.NewString()}}
				machine, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?scope=account", headers)
				if machine != nil {
					_ = machine.Close()
				}
				if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
					t.Fatalf("legacy agent context acquired account content through %s: %v %v", hint, response, err)
				}
			}
			connect := func() *websocket.Conn {
				t.Helper()
				conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?scope=account", nil)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = conn.Close() })
				if err = conn.WriteJSON(map[string]any{"type": "auth", "payload": map[string]string{"token": token}}); err != nil {
					t.Fatal(err)
				}
				var ack struct {
					Type string `json:"type"`
				}
				_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				if err = conn.ReadJSON(&ack); err != nil || ack.Type != "auth_ack" {
					t.Fatalf("account auth: %+v %v", ack, err)
				}
				return conn
			}
			read := func(conn *websocket.Conn) map[string]json.RawMessage {
				t.Helper()
				var frame map[string]json.RawMessage
				_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				if err := conn.ReadJSON(&frame); err != nil {
					t.Fatal(err)
				}
				return frame
			}
			subscribe := func(conn *websocket.Conn, expected string) {
				t.Helper()
				if err := conn.WriteJSON(map[string]any{"type": "subscribe", "payload": map[string]string{"scope": "issue", "id": issue}}); err != nil {
					t.Fatal(err)
				}
				if frame := read(conn); string(frame["type"]) != `"`+expected+`"` {
					t.Fatalf("subscription: %s", frame)
				}
			}
			get := func(expected int) map[string]any {
				t.Helper()
				request := testutil.JSONRequest(http.MethodGet, "/api/collaboration/issues/"+issue, nil)
				request.Header.Set("Authorization", "Bearer "+token)
				request.Header.Set("X-Workspace-ID", uuid.NewString())
				return testutil.Call(t, testServer.Config.Handler.ServeHTTP, request).Want(expected).Map()
			}
			conn := connect()
			subscribe(conn, "subscribe_ack")
			detail := get(http.StatusOK)
			if detail["description"] != "PRIVATE_SHARED_BODY" {
				t.Fatal("authorized HTTP detail was lost")
			}
			cursor, ok := detail["realtime"].(map[string]any)
			if !ok || cursor["event_seq"] != "0" || cursor["authority_epoch"] != "1" {
				t.Fatalf("snapshot cursor was not exact: %v", detail)
			}
			fx.Member(t, testWorkspaceID, viewer, "member")
			fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue, viewer)
			hub.BroadcastToScope(realtime.ScopeIssue, issue, []byte(`{"type":"collaboration:changed","payload":{}}`))
			if frame := read(conn); string(frame["type"]) != `"collaboration:changed"` {
				t.Fatalf("source member lost its independent room: %s", frame)
			}
			caps := get(http.StatusOK)["capabilities"].(map[string]any)
			if caps["control_ai"] != false || caps["read"] != true {
				t.Fatalf("source-only authority was wrong: %v", caps)
			}
			fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, viewer)
			hub.BroadcastToScope(realtime.ScopeIssue, issue, []byte(`{"type":"collaboration:changed","payload":{"secret":"MUST_NOT_SEND"}}`))
			frame := read(conn)
			if string(frame["type"]) != `"collaboration:access_changed"` || strings.Contains(string(frame["payload"]), "MUST_NOT_SEND") {
				t.Fatalf("last read basis leaked queued content: %s", frame)
			}
			get(http.StatusNotFound)
			subscribe(connect(), "subscribe_error")
		})
	}
}

func TestCollaborationSourceMembershipInvalidationUsesAccountScope(t *testing.T) {
	bus := events.New()
	b := &fakeBroadcaster{}
	registerListeners(bus, b)
	bus.Publish(events.Event{Type: protocol.EventMemberRemoved, WorkspaceID: "source", Payload: map[string]any{"user_id": "viewer", "workspace_id": "source", "private": "MUST_NOT_COPY"}})
	if len(b.scopeCalls) != 1 || b.scopeCalls[0].scopeType != realtime.ScopeAccount || b.scopeCalls[0].scopeID != "viewer" {
		t.Fatalf("membership invalidation did not reach its account: %+v", b.scopeCalls)
	}
	if strings.Contains(string(b.scopeCalls[0].msg), "MUST_NOT_COPY") {
		t.Fatal("account invalidation copied unrelated payload")
	}
}

func TestCollaborationNotificationStateInvalidatesOnlyItsViewer(t *testing.T) {
	bus := events.New()
	b := &fakeBroadcaster{}
	registerListeners(bus, b)
	bus.Publish(events.Event{Type: "collaboration:workbench_changed", WorkspaceID: "ignored", ActorID: "viewer", Payload: map[string]any{"private": "MUST_NOT_COPY"}})
	if len(b.scopeCalls) != 1 || b.scopeCalls[0].scopeType != realtime.ScopeAccount || b.scopeCalls[0].scopeID != "viewer" || len(b.workspaceCalls) != 0 || len(b.userCalls) != 0 || b.broadcastCalled != 0 {
		t.Fatalf("notification state reached another audience: %+v", b)
	}
	if string(b.scopeCalls[0].msg) != `{"type":"collaboration:workbench_changed","payload":{}}` {
		t.Fatal("notification invalidation copied content")
	}
}
