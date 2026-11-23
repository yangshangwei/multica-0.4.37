package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestKnowledgePATProvenanceSurvivesVerifiedCacheHit(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	const token = "mul_knowledge_cached_fixture"
	hash := auth.HashToken(token)
	user := uuid.NewString()
	rdb, mock := redismock.NewClientMock()
	t.Cleanup(func() { _ = rdb.Close() })
	mock.ExpectGet("mul:auth:pat:" + hash).SetVal(user)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.AuthenticatedPrincipalFromContext(r.Context())
		if !ok || p.UserID != user || p.Kind != auth.CredentialPAT || p.PATTokenHash != hash {
			t.Error("cached authentication lost original PAT provenance")
		}
		raw, _ := json.Marshal(p)
		if strings.Contains(string(raw), hash) {
			t.Error("credential provenance was publicly serializable")
		}
		w.WriteHeader(204)
	})
	r := testutil.JSONRequest("GET", "/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	testutil.Call(t, Auth(nil, auth.NewPATCache(rdb), nil)(next).ServeHTTP, r).Want(204)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgePATProvenanceSurvivesLegacyAndPasswordDatabaseAuth(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires isolated DATABASE_URL")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	user := fx.User(t, "Knowledge PAT owner", "knowledge-pat-"+uuid.NewString()+"@example.test")
	fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": user, "username": "knowledge-" + uuid.NewString(), "password_hash": "fixture-only", "session_version": 1, "must_change_password": false}, "user_id=$1", user)
	token, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken(token)
	fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": user, "name": "Knowledge auth fixture", "token_hash": hash, "token_prefix": "mul_test", "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", mode)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p, ok := auth.AuthenticatedPrincipalFromContext(r.Context())
				if !ok || p.UserID != user || p.Kind != auth.CredentialPAT || p.PATTokenHash != hash {
					t.Error("database authentication lost original PAT provenance")
				}
				w.WriteHeader(204)
			})
			r := testutil.JSONRequest("GET", "/api/me", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			testutil.Call(t, Auth(db.New(pool), nil, nil)(next).ServeHTTP, r).Want(204)
		})
	}
}
