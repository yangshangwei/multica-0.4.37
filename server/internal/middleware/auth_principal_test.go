package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
)

func TestAuthStampsTrustedPrincipalInsteadOfClientActorHeaders(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	claims := validClaims()
	claims["sub"] = "00000000-0000-4000-8000-000000000081"
	var got auth.AuthenticatedPrincipal
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		got, ok = auth.AuthenticatedPrincipalFromContext(r.Context())
		if !ok {
			t.Error("successful authentication did not stamp principal")
		}
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodGet, "/api/me/workbench", nil)
	r.Header.Set("Authorization", "Bearer "+generateToken(claims, auth.JWTSecret()))
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-User-ID", "forged")
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: "stale", Kind: auth.CredentialCloud, AgentActor: true}))
	w := httptest.NewRecorder()
	Auth(nil, nil, nil)(next).ServeHTTP(w, r)
	if !called || w.Code != http.StatusNoContent || got.UserID != claims["sub"] || got.Kind != auth.CredentialJWT || got.Restricted || got.AgentActor {
		t.Fatalf("trusted principal = %+v, status=%d called=%v", got, w.Code, called)
	}
}

func TestAuthLegacyAgentHintsCannotAcquireCollaborationRights(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	const userID = "00000000-0000-4000-8000-000000000082"
	for _, credential := range []string{"jwt", "cookie", "cached_pat"} {
		for _, hints := range []struct {
			name, agent, task string
		}{
			{name: "human"},
			{name: "agent_and_task", agent: "00000000-0000-4000-8000-000000000083", task: "00000000-0000-4000-8000-000000000084"},
			{name: "agent_only", agent: "legacy-agent"},
			{name: "task_only", task: "legacy-task"},
			{name: "malformed_pair", agent: "invalid", task: "invalid"},
		} {
			t.Run(credential+"/"+hints.name, func(t *testing.T) {
				claims := validClaims()
				claims["sub"] = userID
				token, kind := generateToken(claims, auth.JWTSecret()), auth.CredentialJWT
				var cache *auth.PATCache
				if credential == "cached_pat" {
					token, kind = "mul_legacy_actor_test", auth.CredentialPAT
					rdb, mock := redismock.NewClientMock()
					t.Cleanup(func() { _ = rdb.Close() })
					mock.ExpectGet("mul:auth:pat:" + auth.HashToken(token)).SetVal(userID)
					t.Cleanup(func() {
						if err := mock.ExpectationsWereMet(); err != nil {
							t.Error(err)
						}
					})
					cache = auth.NewPATCache(rdb)
				}
				called := false
				next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					principal, ok := auth.AuthenticatedPrincipalFromContext(r.Context())
					if !ok || principal.UserID != userID || principal.Kind != kind {
						t.Fatalf("actor hints changed verified credential identity: %+v", principal)
					}
					actor, ok := collaboration.PrincipalFromContext(r.Context())
					wantHuman := hints.agent == "" && hints.task == ""
					if !ok || actor.Human() != wantHuman || principal.AgentActor == wantHuman {
						t.Errorf("legacy actor hints acquired human collaboration authority: %+v", actor)
					}
					if r.Header.Get("X-Agent-ID") != hints.agent || r.Header.Get("X-Task-ID") != hints.task {
						t.Error("collaboration classification changed ordinary actor headers")
					}
					w.WriteHeader(http.StatusNoContent)
				})
				r := httptest.NewRequest(http.MethodGet, "/api/me/workbench", nil)
				if credential == "cookie" {
					r.AddCookie(&http.Cookie{Name: auth.AuthCookieName, Value: token})
				} else {
					r.Header.Set("Authorization", "Bearer "+token)
				}
				r.Header.Set("X-Agent-ID", hints.agent)
				r.Header.Set("X-Task-ID", hints.task)
				w := httptest.NewRecorder()
				Auth(nil, cache, nil)(next).ServeHTTP(w, r)
				if !called || w.Code != http.StatusNoContent {
					t.Fatalf("valid credential did not authenticate: %d %s", w.Code, w.Body.String())
				}
			})
		}
	}
}
