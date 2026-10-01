package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/multica-ai/multica/server/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/realtime"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type passwordWSMember struct{}

func (passwordWSMember) IsMember(context.Context, string, string) bool { return true }

type passwordWSAuthorizer struct{ calls atomic.Int64 }

func (a *passwordWSAuthorizer) AuthorizeScope(context.Context, string, string, string, string) (bool, error) {
	a.calls.Add(1)
	return true, nil
}

func passwordWSWait(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal("websocket did not become ready")
		}
		time.Sleep(time.Millisecond)
	}
}
func passwordWSRead(t *testing.T, c *websocket.Conn) []byte {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func passwordWSClosed(t *testing.T, c *websocket.Conn) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := c.ReadMessage()
	if err == nil {
		t.Fatalf("revoked connection received %s", raw)
	}
	if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
		t.Fatal("revoked socket stayed open")
	}
}

func TestPasswordRealtimeRevocationBeforeSendAndSubscribe(t *testing.T) {
	passwordTestSetup(t)
	for _, tc := range []struct {
		name               string
		incoming, disabled bool
	}{
		{"revoked outgoing", false, false},
		{"revoked incoming", true, false},
		{"disabled outgoing", false, true},
		{"disabled incoming", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := passwordRegister(t, fmt.Sprintf("ws%d", time.Now().UnixNano()))
			hub := realtime.NewHub()
			hub.SetPasswordQueries(testHandler.Queries)
			a := &passwordWSAuthorizer{}
			hub.SetAuthorizer(a)
			go hub.Run()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				realtime.HandleWebSocket(hub, passwordWSMember{}, nil, nil, w, r)
			}))
			defer srv.Close()
			headers := http.Header{"Cookie": {auth.AuthCookieName + "=" + user.Token}}
			c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"?workspace_id=ws-password", headers)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			passwordWSWait(t, func() bool { return hub.HasLocalSubscribers(realtime.ScopeWorkspace, "ws-password") })
			hub.BroadcastToWorkspace("ws-password", []byte(`{"type":"before"}`))
			passwordWSRead(t, c)
			if tc.disabled {
				dbfx.Exec(t, `UPDATE "user" SET disabled_at=now(), disabled_reason='test suspension' WHERE id=$1`, user.User.ID)
			} else {
				dbfx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, user.User.ID)
			}
			if tc.incoming {
				if err = c.WriteJSON(map[string]any{"type": "subscribe", "payload": map[string]string{"scope": "task", "id": "private-task"}}); err != nil {
					t.Fatal(err)
				}
			} else {
				hub.BroadcastToWorkspace("ws-password", []byte(`{"type":"secret-after-reset"}`))
			}
			passwordWSClosed(t, c)
			if a.calls.Load() != 0 {
				t.Fatal("revoked subscription reached scope handler")
			}
		})
	}
}

func TestPasswordDaemonRevocationAndContext(t *testing.T) {
	passwordTestSetup(t)
	for _, tc := range []struct {
		name               string
		incoming, disabled bool
	}{
		{"revoked outgoing", false, false},
		{"revoked incoming", true, false},
		{"disabled outgoing", false, true},
		{"disabled incoming", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := passwordRegister(t, fmt.Sprintf("dw%d", time.Now().UnixNano()))
			hub := daemonws.NewHub()
			hub.SetPasswordQueries(testHandler.Queries)
			var calls atomic.Int64
			hub.SetRPCHandler(func(ctx context.Context, _ daemonws.ClientIdentity, _ string, _ json.RawMessage) (int, json.RawMessage, error) {
				session, ok := auth.PasswordSessionFromContext(ctx)
				if !ok || session.UserID != user.User.ID || session.Version != 1 {
					return 500, nil, fmt.Errorf("password session lost in WS context")
				}
				calls.Add(1)
				return 200, json.RawMessage(`{}`), nil
			})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hub.HandleWebSocket(w, r, daemonws.ClientIdentity{UserID: user.User.ID, RuntimeIDs: []string{"runtime-password"}})
			}))
			defer srv.Close()
			c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), http.Header{"Authorization": {"Bearer " + user.Token}})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			passwordWSWait(t, func() bool { return hub.RuntimeConnectionCount("runtime-password") == 1 })
			frame := map[string]any{"type": "daemon:rpc_request", "payload": map[string]any{"request_id": "before", "method": "tasks.claim", "body": map[string]any{}}}
			if err = c.WriteJSON(frame); err != nil {
				t.Fatal(err)
			}
			passwordWSRead(t, c)
			if calls.Load() != 1 {
				t.Fatal("RPC lost authenticated context")
			}
			if tc.disabled {
				dbfx.Exec(t, `UPDATE "user" SET disabled_at=now(), disabled_reason='test suspension' WHERE id=$1`, user.User.ID)
			} else {
				dbfx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, user.User.ID)
			}
			if tc.incoming {
				if err = c.WriteJSON(frame); err != nil {
					t.Fatal(err)
				}
			} else {
				hub.NotifyTaskAvailable("runtime-password", "private-task")
			}
			passwordWSClosed(t, c)
			if calls.Load() != 1 {
				t.Fatal("revoked RPC performed side effects")
			}
		})
	}
}

func TestPasswordDaemonTokenKindsAndRevocation(t *testing.T) {
	passwordTestSetup(t)
	user := passwordRegister(t, fmt.Sprintf("tokens%d", time.Now().UnixNano()))
	for _, kind := range []string{"mul_", "mdt_"} {
		t.Run(kind, func(t *testing.T) {
			raw := kind + fmt.Sprintf("%d", time.Now().UnixNano())
			table := "personal_access_token"
			cols := testutil.Cols{"user_id": user.User.ID, "auth_version": int64(1), "token_hash": auth.HashToken(raw), "name": "password test", "token_prefix": kind}
			if kind == "mdt_" {
				table = "daemon_token"
				cols = testutil.Cols{"user_id": user.User.ID, "auth_version": int64(1), "token_hash": auth.HashToken(raw), "workspace_id": testWorkspaceID, "daemon_id": "password-daemon", "expires_at": time.Now().Add(time.Hour)}
			}
			id := dbfx.Insert(t, table, cols)
			identity, err := auth.CheckPasswordToken(context.Background(), testHandler.Queries, raw, true)
			if err != nil || identity.Session.UserID != user.User.ID || identity.Session.Version != 1 {
				t.Fatalf("identity=%+v err=%v", identity, err)
			}
			if kind == "mdt_" {
				if _, err = auth.CheckPasswordToken(context.Background(), testHandler.Queries, raw, false); !errors.Is(err, auth.ErrPasswordSession) {
					t.Fatal("daemon token entered human websocket")
				}
			}
			dbfx.Exec(t, "UPDATE "+table+" SET auth_version=0 WHERE id=$1", id)
			if _, err = auth.CheckPasswordToken(context.Background(), testHandler.Queries, raw, true); !errors.Is(err, auth.ErrPasswordSession) {
				t.Fatal("legacy version gained password access")
			}
			dbfx.Exec(t, "UPDATE "+table+" SET auth_version=1 WHERE id=$1", id)
			dbfx.Exec(t, "DELETE FROM "+table+" WHERE id=$1", id)
			if _, err = auth.CheckPasswordToken(context.Background(), testHandler.Queries, raw, true); !errors.Is(err, auth.ErrPasswordSession) {
				t.Fatal("deleted credential retained access")
			}
		})
	}
	if _, err := auth.CheckPasswordToken(context.Background(), testHandler.Queries, "mcn_cloud", true); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatal("cloud credential gained password access")
	}
}

func TestPasswordDaemonHeartbeatRejectsRevocationAndDatabaseFailure(t *testing.T) {
	passwordTestSetup(t)
	for _, failure := range []string{"revoked", "database"} {
		t.Run(failure, func(t *testing.T) {
			user := passwordRegister(t, fmt.Sprintf("heartbeat%d", time.Now().UnixNano()))
			pool, err := pgxpool.New(t.Context(), testPool.Config().ConnString())
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			hub := daemonws.NewHub()
			hub.SetPasswordQueries(db.New(pool))
			pending := make(chan string, 2)
			pending <- "before"
			var consumed atomic.Int64
			hub.SetHeartbeatHandler(func(_ context.Context, _ daemonws.ClientIdentity, runtime string, _ bool) (*protocol.DaemonHeartbeatAckPayload, error) {
				select {
				case <-pending:
					consumed.Add(1)
				default:
				}
				return &protocol.DaemonHeartbeatAckPayload{RuntimeID: runtime, Status: "ok"}, nil
			})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hub.HandleWebSocket(w, r, daemonws.ClientIdentity{UserID: user.User.ID, RuntimeIDs: []string{"heartbeat-runtime"}})
			}))
			defer srv.Close()
			c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), http.Header{"Authorization": {"Bearer " + user.Token}})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			frame := map[string]any{"type": "daemon:heartbeat", "payload": map[string]string{"runtime_id": "heartbeat-runtime"}}
			if err = c.WriteJSON(frame); err != nil {
				t.Fatal(err)
			}
			passwordWSRead(t, c)
			if consumed.Load() != 1 {
				t.Fatal("authorized heartbeat did not consume pending action")
			}
			pending <- "after"
			if failure == "database" {
				pool.Close()
			} else {
				dbfx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, user.User.ID)
			}
			if err = c.WriteJSON(frame); err != nil {
				t.Fatal(err)
			}
			passwordWSClosed(t, c)
			if consumed.Load() != 1 || len(pending) != 1 {
				t.Fatal("invalid heartbeat consumed pending action before rejection")
			}
		})
	}
}

func TestPasswordRealtimeDatabaseFailureStopsProtectedEvents(t *testing.T) {
	passwordTestSetup(t)
	user := passwordRegister(t, fmt.Sprintf("dbfault%d", time.Now().UnixNano()))
	pool, err := pgxpool.New(t.Context(), testPool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	hub := realtime.NewHub()
	hub.SetPasswordQueries(db.New(pool))
	go hub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realtime.HandleWebSocket(hub, passwordWSMember{}, nil, nil, w, r)
	}))
	defer srv.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"?workspace_id=dbfault-workspace", http.Header{"Cookie": {auth.AuthCookieName + "=" + user.Token}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	passwordWSWait(t, func() bool { return hub.HasLocalSubscribers(realtime.ScopeWorkspace, "dbfault-workspace") })
	hub.BroadcastToWorkspace("dbfault-workspace", []byte(`{"type":"before"}`))
	passwordWSRead(t, c)
	pool.Close()
	hub.BroadcastToWorkspace("dbfault-workspace", []byte(`{"type":"secret-after-database-failure"}`))
	passwordWSClosed(t, c)
}
