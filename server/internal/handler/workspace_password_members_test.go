package handler

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	"net/http"
	"testing"
	"time"
)

func TestPasswordMembersKeepDistinctUsernamesWithSameDisplayName(t *testing.T) {
	usernames := []string{fmt.Sprintf("same_a_%d", time.Now().UnixNano()), fmt.Sprintf("same_b_%d", time.Now().UnixNano())}
	ids := make([]string, 2)
	for i, username := range usernames {
		ids[i] = dbfx.User(t, "Same Name", "", testutil.Cols{"email": nil})
		dbfx.Member(t, testWorkspaceID, ids[i], "member")
		dbfx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": ids[i], "username": username, "password_hash": "unused-test-hash"}, "user_id=$1", ids[i])
	}
	req := testutil.JSONRequest(http.MethodGet, "/api/workspaces/"+testWorkspaceID+"/members", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", testWorkspaceID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	var result []MemberWithUserResponse
	testutil.Call(t, testHandler.ListMembersWithUser, req).Want(http.StatusOK).JSON(&result)
	seen := make(map[string]MemberWithUserResponse)
	for _, member := range result {
		seen[member.UserID] = member
	}
	for i, id := range ids {
		member := seen[id]
		if member.Name != "Same Name" || member.Email != "" || member.Username != usernames[i] {
			t.Fatalf("account identity lost: %+v", member)
		}
	}
}
