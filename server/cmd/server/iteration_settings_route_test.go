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

func TestIterationSettingsRoutesRequireWorkspaceOptIn(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	fx.Cleanup(t, `DELETE FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID)
	fx.Cleanup(t, `DELETE FROM iteration_operation WHERE workspace_id=$1`, testWorkspaceID)
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
	request("GET", "iteration-capabilities", "", nil, 401)
	settings := request("GET", "iteration-settings", testToken, nil, 200)
	if settings["enabled"] != false || settings["revision"] != float64(1) || settings["workspace_id"] != testWorkspaceID {
		t.Fatalf("settings=%v", settings)
	}
	caps := request("GET", "iteration-capabilities", testToken, nil, 200)
	if caps["supported"] != true || caps["manual"] != true || caps["atomic_handoff"] != true || caps["enabled"] != false || caps["schema_version"] != float64(1) || caps["workspace_id"] != testWorkspaceID {
		t.Fatalf("capabilities must separate service support from workspace opt-in: %v", caps)
	}
	body := map[string]any{"request_id": uuid.NewString(), "expected_revision": settings["revision"], "confirmed_timezone": settings["effective_timezone"]}
	request("POST", "iteration-settings/enable", "", body, 401)
	email := uuid.NewString() + "@example.invalid"
	member := fx.User(t, "Settings route member", email)
	fx.Member(t, testWorkspaceID, member, "member")
	token, err := generateTestJWT(member, email, "Settings route member")
	if err != nil {
		t.Fatal(err)
	}
	if readable := request("GET", "iteration-settings", token, nil, 200); readable["enabled"] != false {
		t.Fatalf("member read changed default settings: %v", readable)
	}
	request("GET", "iteration-capabilities", token, nil, 200)
	request("POST", "iteration-settings/enable", token, body, 403)
	if n := fx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
		t.Fatal("reads or unauthorized enable created settings")
	}
	if n := fx.Count(t, `SELECT count(*) FROM iteration_operation WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
		t.Fatal("unauthorized enable persisted an operation")
	}
	result := request("POST", "iteration-settings/enable", testToken, body, 200)
	if result["operation"] != "enable" || result["request_id"] != body["request_id"] {
		t.Fatalf("owner enable lost the confirmed request: %v", result)
	}
	settings = request("GET", "iteration-settings", token, nil, 200)
	if settings["enabled"] != true || settings["revision"] != float64(2) {
		t.Fatalf("owner opt-in was not readable by a member: %v", settings)
	}
	if caps = request("GET", "iteration-capabilities", token, nil, 200); caps["enabled"] != true {
		t.Fatalf("capabilities did not reflect workspace opt-in: %v", caps)
	}
}
