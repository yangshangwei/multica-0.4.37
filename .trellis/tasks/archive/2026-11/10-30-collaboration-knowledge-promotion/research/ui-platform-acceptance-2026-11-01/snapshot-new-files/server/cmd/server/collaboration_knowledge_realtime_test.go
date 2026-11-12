package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/redis/go-redis/v9"
)

type knowledgeFinalSendGate struct {
	realtime.ScopeAuthorizer
	mu               sync.Mutex
	entered, release chan struct{}
	patHash          string
}

func (g *knowledgeFinalSendGate) AuthorizeScope(ctx context.Context, user, workspace, scope, id string) (bool, error) {
	if principal, ok := auth.AuthenticatedPrincipalFromContext(ctx); ok {
		g.mu.Lock()
		g.patHash = principal.PATTokenHash
		g.mu.Unlock()
	}
	allowed, err := g.ScopeAuthorizer.AuthorizeScope(ctx, user, workspace, scope, id)
	if scope == realtime.ScopeWorkspace && allowed && err == nil {
		g.mu.Lock()
		entered, release := g.entered, g.release
		g.entered, g.release = nil, nil
		g.mu.Unlock()
		if entered != nil {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}
	}
	return allowed, err
}

type knowledgeSocketMembership struct{ q *db.Queries }

func (m knowledgeSocketMembership) IsMember(ctx context.Context, user, workspace string) bool {
	_, err := m.q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: util.MustParseUUID(user), WorkspaceID: util.MustParseUUID(workspace)})
	return err == nil
}

func knowledgeSocket(t *testing.T, token string, q *db.Queries, resolver realtime.PATResolver, gate *knowledgeFinalSendGate) (*realtime.Hub, *websocket.Conn) {
	t.Helper()
	hub := realtime.NewHub()
	hub.SetPasswordQueries(q)
	hub.SetAuthorizer(gate)
	go hub.Run()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realtime.HandleWebSocket(hub, knowledgeSocketMembership{q}, resolver, nil, w, r)
	}))
	t.Cleanup(server.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?workspace_id="+testWorkspaceID, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err = conn.WriteJSON(map[string]any{"type": "auth", "payload": map[string]string{"token": token}}); err != nil {
		t.Fatal(err)
	}
	var ack map[string]json.RawMessage
	if err = json.Unmarshal(readKnowledgeSocket(t, conn), &ack); err != nil || string(ack["type"]) != `"auth_ack"` {
		t.Fatal("knowledge socket did not authenticate")
	}
	if err = conn.WriteJSON(map[string]any{"type": "subscribe", "payload": map[string]string{"scope": "workspace", "id": testWorkspaceID}}); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(readKnowledgeSocket(t, conn), &ack); err != nil || string(ack["type"]) != `"subscribe_ack"` {
		t.Fatal("knowledge socket did not subscribe")
	}
	return hub, conn
}

func readKnowledgeSocket(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func knowledgeRelayFrame(t *testing.T) []byte {
	t.Helper()
	frame, err := json.Marshal(struct {
		Type    string                                    `json:"type"`
		Payload protocol.CollaborationKnowledgeTargetHint `json:"payload"`
	}{protocol.EventCollaborationKnowledgeChanged, protocol.CollaborationKnowledgeTargetHint{TargetKind: "workspace", TargetID: testWorkspaceID, KnowledgeRevision: "7"}})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func TestKnowledgeQueuedFrameRejectsStalePATCacheAfterRevocationOrExpiry(t *testing.T) {
	if testPool == nil {
		t.Fatal("isolated database required")
	}
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("requires isolated REDIS_TEST_URL")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	if err = rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal("isolated Redis unavailable")
	}
	for _, mutation := range []string{"revoke", "expire", "subject_mismatch"} {
		t.Run(mutation, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			fx := testutil.New(testPool, testWorkspaceID, testUserID)
			q := db.New(testPool)
			raw, err := auth.GeneratePATToken()
			if err != nil {
				t.Fatal(err)
			}
			hash := auth.HashToken(raw)
			id := fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": testUserID, "name": "knowledge freshness", "token_hash": hash, "token_prefix": raw[:12], "expires_at": time.Now().Add(time.Hour)})
			cache := auth.NewPATCache(rdb)
			t.Cleanup(func() { cache.Invalidate(context.Background(), hash) })
			cache.Set(context.Background(), hash, testUserID, auth.AuthCacheTTL)
			resolver := &patResolver{queries: q, cache: cache}
			gate := &knowledgeFinalSendGate{ScopeAuthorizer: &collaborationScopeAuthorizer{workspace: newScopeAuthorizer(q), service: service.NewCollaborationService(q, testPool, nil)}}
			hub, conn := knowledgeSocket(t, raw, q, resolver, gate)
			gate.mu.Lock()
			boundHash := gate.patHash
			gate.mu.Unlock()
			if boundHash != hash {
				t.Fatal("socket lost the original authenticated PAT identity")
			}
			frame := knowledgeRelayFrame(t)
			if err = hub.PublishWithID(realtime.ScopeWorkspace, testWorkspaceID, "", frame, uuid.NewString()+":target"); err != nil {
				t.Fatal(err)
			}
			var got map[string]json.RawMessage
			if err = json.Unmarshal(readKnowledgeSocket(t, conn), &got); err != nil || string(got["type"]) != `"collaboration:knowledge_changed"` {
				t.Fatal("valid original PAT did not receive the safe relay hint")
			}
			entered, release := make(chan struct{}), make(chan struct{})
			gate.mu.Lock()
			gate.entered, gate.release = entered, release
			gate.mu.Unlock()
			if err = hub.PublishWithID(realtime.ScopeWorkspace, testWorkspaceID, "", frame, uuid.NewString()+":target"); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(4 * time.Second):
				t.Fatal("knowledge frame did not reach final-send barrier")
			}
			if mutation == "revoke" {
				if _, err = q.RevokePersonalAccessToken(context.Background(), db.RevokePersonalAccessTokenParams{ID: util.MustParseUUID(id), UserID: util.MustParseUUID(testUserID)}); err != nil {
					t.Fatal(err)
				}
			} else if mutation == "expire" {
				fx.Exec(t, `UPDATE personal_access_token SET expires_at=now()-interval '1 second' WHERE id=$1`, id)
			} else {
				other := fx.User(t, "Other PAT owner", "knowledge-other-"+uuid.NewString()+"@example.test")
				fx.Exec(t, `UPDATE personal_access_token SET user_id=$2 WHERE id=$1`, id, other)
			}
			// Simulate the real race: eviction succeeds, then an older resolver
			// fills the cache with its pre-revocation result. Never depend on TTL.
			cache.Invalidate(context.Background(), hash)
			cache.Set(context.Background(), hash, testUserID, auth.AuthCacheTTL)
			if user, ok := resolver.ResolveToken(context.Background(), raw); !ok || user != testUserID {
				t.Fatal("test did not retain stale cached authority")
			}
			pat, lookupErr := q.GetPersonalAccessTokenByHash(context.Background(), hash)
			if mutation == "subject_mismatch" {
				if lookupErr != nil || util.UUIDToString(pat.UserID) == testUserID {
					t.Fatal("test did not change original credential subject")
				}
			} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
				t.Fatal("database still accepts revoked/expired PAT")
			}
			if fx.Count(t, `SELECT count(*) FROM "user" u JOIN member m ON m.user_id=u.id WHERE u.id=$1 AND u.disabled_at IS NULL AND m.workspace_id=$2`, testUserID, testWorkspaceID) != 1 {
				t.Fatal("test accidentally revoked account or membership")
			}
			close(release)
			marker := []byte(`{"type":"knowledge-freshness-marker","payload":{}}`)
			hub.BroadcastToWorkspace(testWorkspaceID, marker)
			if string(readKnowledgeSocket(t, conn)) != string(marker) {
				t.Fatal("stale PAT received a queued knowledge frame after committed revocation/expiry")
			}
			// Unrelated traffic above still follows its existing cached behavior.
			fresh, err := auth.GeneratePATToken()
			if err != nil {
				t.Fatal(err)
			}
			fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": testUserID, "name": "reconnected knowledge", "token_hash": auth.HashToken(fresh), "token_prefix": fresh[:12], "expires_at": time.Now().Add(time.Hour)})
			t.Cleanup(func() { cache.Invalidate(context.Background(), auth.HashToken(fresh)) })
			freshGate := &knowledgeFinalSendGate{ScopeAuthorizer: gate.ScopeAuthorizer}
			freshHub, freshConn := knowledgeSocket(t, fresh, q, resolver, freshGate)
			if err = freshHub.PublishWithID(realtime.ScopeWorkspace, testWorkspaceID, "", frame, uuid.NewString()+":target"); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(readKnowledgeSocket(t, freshConn), &got); err != nil || string(got["type"]) != `"collaboration:knowledge_changed"` {
				t.Fatal("reconnect did not bind the replacement credential")
			}
		})
	}
}

func TestKnowledgeSocketPreservesCurrentJWTAndPasswordGates(t *testing.T) {
	for _, mode := range []string{"legacy", "password", "password_pat"} {
		t.Run(mode, func(t *testing.T) {
			authMode := mode
			if mode == "password_pat" {
				authMode = "password"
			}
			t.Setenv("MULTICA_AUTH_MODE", authMode)
			fx := testutil.New(testPool, testWorkspaceID, testUserID)
			q := db.New(testPool)
			viewer := fx.User(t, "Knowledge JWT viewer", "knowledge-jwt-"+uuid.NewString()+"@example.test")
			fx.Member(t, testWorkspaceID, viewer, "admin")
			if authMode == "password" {
				fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": viewer, "username": "knowledge_" + uuid.NewString(), "password_hash": "unused-in-token-check"}, "user_id=$1", viewer)
			}
			gate := &knowledgeFinalSendGate{ScopeAuthorizer: &collaborationScopeAuthorizer{workspace: newScopeAuthorizer(q), service: service.NewCollaborationService(q, testPool, nil)}}
			token := platformRouterJWT(t, viewer, nil)
			if mode == "password_pat" {
				var err error
				token, err = auth.GeneratePATToken()
				if err != nil {
					t.Fatal(err)
				}
				fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": viewer, "name": "password knowledge", "token_hash": auth.HashToken(token), "token_prefix": token[:12], "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
			}
			hub, conn := knowledgeSocket(t, token, q, nil, gate)
			if err := hub.PublishWithID(realtime.ScopeWorkspace, testWorkspaceID, "", knowledgeRelayFrame(t), uuid.NewString()+":target"); err != nil {
				t.Fatal(err)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(readKnowledgeSocket(t, conn), &got); err != nil || string(got["type"]) != `"collaboration:knowledge_changed"` {
				t.Fatal("valid JWT/password identity was refused")
			}
			if authMode == "password" {
				fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, viewer)
			} else {
				fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, viewer)
			}
			if err := hub.PublishWithID(realtime.ScopeWorkspace, testWorkspaceID, "", knowledgeRelayFrame(t), uuid.NewString()+":target"); err != nil {
				t.Fatal(err)
			}
			if authMode == "password" {
				_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
				_, _, err := conn.ReadMessage()
				if !websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Fatal("stale password session was not closed before knowledge send")
				}
			} else {
				marker := []byte(`{"type":"knowledge-jwt-marker","payload":{}}`)
				hub.BroadcastToWorkspace(testWorkspaceID, marker)
				if string(readKnowledgeSocket(t, conn)) != string(marker) {
					t.Fatal("disabled JWT account received knowledge")
				}
			}
		})
	}
}

func TestKnowledgeSocketRejectsPrivateOrAmbiguousOuterEnvelope(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	q := db.New(testPool)
	gate := &knowledgeFinalSendGate{ScopeAuthorizer: &collaborationScopeAuthorizer{workspace: newScopeAuthorizer(q), service: service.NewCollaborationService(q, testPool, nil)}}
	hub, conn := knowledgeSocket(t, platformRouterJWT(t, testUserID, nil), q, nil, gate)
	payload := `{"target_kind":"workspace","target_id":"` + testWorkspaceID + `","knowledge_revision":"7"}`
	eventID := uuid.NewString() + ":target"
	for name, raw := range map[string]string{
		"outer_source":      `{"type":"collaboration:knowledge_changed","payload":` + payload + `,"source_id":"PRIVATE_SOURCE","event_id":"` + eventID + `"}`,
		"duplicate_payload": `{"type":"collaboration:knowledge_changed","payload":{"source_id":"PRIVATE_SOURCE"},"payload":` + payload + `,"event_id":"` + eventID + `"}`,
		"duplicate_type":    `{"type":"collaboration:knowledge_changed","type":"unrelated","payload":` + payload + `,"source_id":"PRIVATE_SOURCE","event_id":"` + eventID + `"}`,
		"malformed":         `{"type":"collaboration:knowledge_changed","payload":`,
	} {
		t.Run(name, func(t *testing.T) {
			gate := &knowledgeFinalSendGate{ScopeAuthorizer: &collaborationScopeAuthorizer{workspace: newScopeAuthorizer(q), service: service.NewCollaborationService(q, testPool, nil)}}
			hub, conn := knowledgeSocket(t, platformRouterJWT(t, testUserID, nil), q, nil, gate)
			if err := hub.PublishWithID(realtime.ScopeWorkspace, testWorkspaceID, "", []byte(raw), uuid.NewString()+":target"); err != nil {
				t.Fatal(err)
			}
			marker := []byte(`{"type":"knowledge-envelope-marker","payload":{}}`)
			hub.BroadcastToWorkspace(testWorkspaceID, marker)
			if string(readKnowledgeSocket(t, conn)) != string(marker) {
				t.Fatal("malformed/private knowledge envelope reached actual socket")
			}
		})
	}
	if err := hub.PublishWithID(realtime.ScopeWorkspace, testWorkspaceID, "", knowledgeRelayFrame(t), uuid.NewString()+":target"); err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(readKnowledgeSocket(t, conn), &got); err != nil || string(got["type"]) != `"collaboration:knowledge_changed"` || len(got) != 3 {
		t.Fatal("safe relay envelope failed positive control")
	}
}
