package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestIterationSettingsRoutesKeepRolloutClosed(t *testing.T) {
	request := func(method, path, token string, body any, want int) map[string]any {
		raw, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(t.Context(), method, testServer.URL+"/api/workspaces/"+testWorkspaceID+"/"+path, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s =%d want%d: %s", method, path, response.StatusCode, want, data)
		}
		var decoded map[string]any
		_ = json.Unmarshal(data, &decoded)
		return decoded
	}
	request("GET", "iteration-settings", "", nil, 401)
	settings := request("GET", "iteration-settings", testToken, nil, 200)
	if settings["enabled"] != false || settings["workspace_id"] != testWorkspaceID {
		t.Fatalf("settings=%v", settings)
	}
	caps := request("GET", "iteration-capabilities", testToken, nil, 200)
	if caps["supported"] != false || caps["atomic_handoff"] != false || caps["enabled"] != false {
		t.Fatalf("premature capabilities=%v", caps)
	}
	body := map[string]any{"request_id": uuid.NewString(), "expected_revision": 1, "confirmed_timezone": "UTC"}
	request("POST", "iteration-settings/enable", testToken, body, 422)
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	email := uuid.NewString() + "@example.invalid"
	member := fx.User(t, "Settings route member", email)
	fx.Member(t, testWorkspaceID, member, "member")
	token, err := generateTestJWT(member, email, "Settings route member")
	if err != nil {
		t.Fatal(err)
	}
	request("POST", "iteration-settings/enable", token, body, 403)
	if n := fx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
		t.Fatal("closed route created settings")
	}
}
