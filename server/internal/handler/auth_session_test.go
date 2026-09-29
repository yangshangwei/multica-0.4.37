package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestGetMeMissingUserExpiresSession(t *testing.T) {
	userID := dbfx.User(t, "Deleted Session User", "deleted-session@multica.ai")
	dbfx.Exec(t, `DELETE FROM "user" WHERE id = $1`, userID)
	req := testutil.WithHeaders(
		testutil.JSONRequest(http.MethodGet, "/api/me", nil),
		"X-User-ID", userID,
	)

	response := testutil.Call(t, testHandler.GetMe, req).Want(http.StatusUnauthorized)
	if got := response.Map()["error"]; got != "user not found" {
		t.Fatalf("missing user error = %v, want user not found", got)
	}
	// A late response for an old session must not erase a newer login cookie.
	if cookies := response.Header().Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("missing-user response must not mutate cookies: %v", cookies)
	}
}

func TestGetMeLookupFailurePreservesSession(t *testing.T) {
	userID := dbfx.User(t, "Retryable Session User", "retryable-session@multica.ai")
	fault := &lookupFaultPool{DBTX: testPool, query: "GetUser"}
	h := &Handler{Queries: db.New(fault)}
	req := testutil.WithHeaders(
		testutil.JSONRequest(http.MethodGet, "/api/me", nil),
		"X-User-ID", userID,
	)

	response := testutil.Call(t, h.GetMe, req).Want(http.StatusInternalServerError)
	if !fault.called {
		t.Fatal("user lookup fault was not exercised")
	}
	if got := response.Map()["error"]; got != "failed to lookup user" {
		t.Fatalf("lookup failure error = %v, want failed to lookup user", got)
	}
	if cookies := response.Header().Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("retryable lookup failure must not mutate cookies: %v", cookies)
	}

	var user UserResponse
	testutil.Call(t, testHandler.GetMe, req).Want(http.StatusOK).JSON(&user)
	if user.ID != userID {
		t.Fatalf("recovered session user = %q, want %q", user.ID, userID)
	}
}
