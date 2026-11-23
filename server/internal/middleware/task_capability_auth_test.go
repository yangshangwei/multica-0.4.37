package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTaskCapabilityAuthenticationLookupFailure(t *testing.T) {
	for _, mode := range []string{"device", "password"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", mode)
			for _, tc := range []struct {
				err    error
				status int
			}{{errors.New("database unavailable"), 503}, {pgx.ErrNoRows, 401}} {
				req := httptest.NewRequest("GET", "/api/issues", nil)
				req.Header.Set("Authorization", "Bearer mat_synthetic_lookup")
				rec := httptest.NewRecorder()
				Auth(db.New(workspaceErrorDB{tc.err}), nil, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unknown authority reached handler") })).ServeHTTP(rec, req)
				if rec.Code != tc.status {
					t.Fatalf("got %d, want %d", rec.Code, tc.status)
				}
			}
		})
	}
}
