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

func TestIterationLifecycleRoutesRespectScopeAndClosedRelease(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	period := fx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "JWT period", "timezone": "UTC", "start_date": "2026-10-06", "end_date": "2026-10-19", "created_by": testUserID})
	other := fx.Workspace(t, "Foreign period space", uuid.NewString())
	foreign := testutil.New(testPool, other, testUserID)
	otherPeriod := foreign.Insert(t, "iteration", testutil.Cols{"workspace_id": other, "name": "Hidden period", "timezone": "UTC", "start_date": "2026-10-06", "end_date": "2026-10-19", "created_by": testUserID})
	call := func(method, path, token string, body any, want int) map[string]any {
		encoded, _ := json.Marshal(body)
		request, err := http.NewRequestWithContext(t.Context(), method, testServer.URL+"/api/workspaces/"+testWorkspaceID+"/"+path, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s=%d want%d: %s", method, path, response.StatusCode, want, raw)
		}
		var result map[string]any
		_ = json.Unmarshal(raw, &result)
		return result
	}
	call("GET", "iterations", "", nil, 401)
	page := call("GET", "iterations", testToken, nil, 200)
	if page["workspace_id"] != testWorkspaceID {
		t.Fatal("list workspace drift")
	}
	for _, suffix := range []string{"", "/issues", "/events"} {
		call("GET", "iterations/"+period+suffix, testToken, nil, 200)
		call("GET", "iterations/"+otherPeriod+suffix, testToken, nil, 404)
	}
	call("POST", "iterations", testToken, map[string]any{"request_id": uuid.NewString(), "name": "Must stay closed", "start_date": "2026-10-06", "end_date": "2026-10-19", "confirmed_timezone": "UTC"}, 422)
	call("PUT", "iterations/"+period, testToken, map[string]any{"request_id": uuid.NewString(), "expected_revision": 1, "fields": map[string]any{"name": "Must not change"}}, 422)
	draft := map[string]any{"operation": "start", "iteration_id": period, "expected_iteration_revision": 1, "expected_scope_revision": 1, "expected_settings_revision": 1, "moves": []any{}, "start": map[string]any{"target_id": period, "mode": "scheduled", "terminal_choices": []any{}}}
	call("POST", "iteration-previews", testToken, draft, 422)
	call("POST", "iteration-operations", testToken, map[string]any{"request_id": uuid.NewString(), "draft": draft, "preview_hash": "0000000000000000000000000000000000000000000000000000000000000000"}, 422)
	if n := fx.Count(t, `SELECT count(*) FROM iteration WHERE id=$1 AND name='JWT period' AND status='planned' AND revision=1`, period); n != 1 {
		t.Fatal("closed release mutated period")
	}
}
