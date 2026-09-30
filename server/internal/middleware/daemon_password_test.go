package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDaemonPasswordDatabaseFailureIsUnavailable(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	for _, token := range []string{"mul_existing", "mdt_existing"} {
		t.Run(token, func(t *testing.T) {
			called := false
			h := DaemonAuth(nil, nil, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
			req := httptest.NewRequest("POST", "/api/daemon/heartbeat", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable || called {
				t.Fatalf("status=%d called=%v; want 503 and no business handler", rec.Code, called)
			}
		})
	}
}
