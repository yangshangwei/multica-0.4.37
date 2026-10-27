package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestParseCollaborationMemoryMaxBytes(t *testing.T) {
	for raw, want := range map[string]int{
		"":         service.CollaborationMemoryDefaultMaxBytes,
		"   ":      service.CollaborationMemoryDefaultMaxBytes,
		"32768":    service.CollaborationMemoryMinMaxBytes,
		" 131072 ": service.CollaborationMemoryDefaultMaxBytes,
		"524288":   service.CollaborationMemoryHardMaxBytes,
	} {
		got, err := parseCollaborationMemoryMaxBytes(raw)
		if err != nil || got != want {
			t.Fatalf("parseCollaborationMemoryMaxBytes(%q) = %d, %v; want %d", raw, got, err, want)
		}
	}
	// Invalid values stop the boot instead of quietly restoring the default.
	for raw, message := range map[string]string{
		"32767":  "between 32768 and 524288",
		"524289": "between 32768 and 524288",
		"0":      "between 32768 and 524288",
		"-1":     "between 32768 and 524288",
		"128KB":  "must be an integer",
		"1.5e5":  "must be an integer",
	} {
		got, err := parseCollaborationMemoryMaxBytes(raw)
		if err == nil || got != 0 || !strings.Contains(err.Error(), message) || !strings.Contains(err.Error(), raw) {
			t.Fatalf("parseCollaborationMemoryMaxBytes(%q) = %d, %v; want a %q error naming the value", raw, got, err, message)
		}
	}
}

// The service and handler suites own the behavior matrices. Here an external
// participant reaches the shared-context routes through real authentication.
func TestCollaborationMemoryRoutesServeExternalParticipant(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router fixture required")
	}
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	email := "memory-router-" + uuid.NewString() + "@example.invalid"
	external := base.User(t, "External context editor", email)
	token, err := generateTestJWT(external, email, "External context editor")
	if err != nil {
		t.Fatal(err)
	}
	issue := base.Issue(t, "Shared context route", testutil.Cols{"assignee_type": "member", "assignee_id": external})
	base.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	base.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	for _, user := range []string{testUserID, external} {
		base.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": user}, "issue_id=$1 AND user_id=$2", issue, user)
	}
	base.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, issue)
	base.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	base.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	path := "/api/collaboration/issues/" + issue + "/memory"
	call := func(method, url, key string, body any) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, url, body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Workspace-ID", "unrelated-client-workspace")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}
	if empty := call(http.MethodGet, path, "", nil).Want(http.StatusOK).Map(); empty["version"] != "0" || empty["can_edit"] != true || empty["can_purge"] != false {
		t.Fatalf("external participant empty read: %v", empty)
	}
	key := uuid.NewString()
	written := call(http.MethodPut, path, key, map[string]any{"expected_version": "0", "content": "Decision: ship behind the gate"}).Want(http.StatusOK).Map()
	if written["version"] != "1" || written["code"] != "memory_updated" {
		t.Fatalf("external participant write: %v", written)
	}
	if recovered := call(http.MethodGet, path+"/operations/"+key, "", nil).Want(http.StatusOK).Map(); recovered["version"] != "1" {
		t.Fatalf("original-key recovery through the router: %v", recovered)
	}
	if versions := call(http.MethodGet, path+"/versions", "", nil).Want(http.StatusOK).Map(); versions["current_version"] != "1" {
		t.Fatalf("history through the router: %v", versions)
	}
	if detail := call(http.MethodGet, path+"/versions/1", "", nil).Want(http.StatusOK).Map(); detail["content"] != "Decision: ship behind the gate" {
		t.Fatalf("version through the router: %v", detail)
	}
	call(http.MethodPost, path+"/versions/1/restore", uuid.NewString(), map[string]any{"expected_version": "1"}).Want(http.StatusOK)
	call(http.MethodPost, path+"/versions/1/purge", uuid.NewString(), map[string]any{"reason": "not the initiator"}).Want(http.StatusForbidden)
	var revision int64
	base.QueryRow(t, `SELECT revision FROM issue_collaboration WHERE issue_id=$1`, issue).Scan(&revision)
	if revision != 1 || base.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, issue) != 2 {
		t.Fatalf("shared context advanced the issue revision (%d) or lost a version", revision)
	}
}
