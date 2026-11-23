package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func collaborationMemoryRequest(method, issue, suffix, actor, key string, body any) *http.Request {
	path := "/api/collaboration/issues/" + issue + "/memory" + suffix
	var r *http.Request
	if raw, ok := body.(string); ok {
		r = testutil.JSONRequest(method, path, raw)
	} else {
		r = newRequest(method, path, body)
	}
	r = withURLParam(r, "id", issue)
	if version, ok := strings.CutPrefix(suffix, "/versions/"); ok {
		version, _, _ = strings.Cut(version, "/")
		r = withURLParams(r, "version", version)
	}
	if requestID, ok := strings.CutPrefix(suffix, "/operations/"); ok {
		r = withURLParams(r, "requestId", requestID)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	r.Header.Set("X-Workspace-ID", "unrelated-ambient-workspace")
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
}

func collaborationMemoryIssue(t *testing.T) string {
	t.Helper()
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	return issue
}

func TestCollaborationMemoryHTTPWireShapes(t *testing.T) {
	issue := collaborationMemoryIssue(t)
	external := dbfx.User(t, "External context author", "memory-author-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)

	response := testutil.Call(t, testHandler.GetCollaborationMemory, collaborationMemoryRequest(http.MethodGet, issue, "", testUserID, "", nil)).Want(http.StatusOK)
	empty := response.Map()
	if empty["version"] != "0" || empty["content"] != "" || empty["author"] != nil || empty["created_at"] != nil || empty["bytes"] != float64(0) ||
		empty["max_bytes"] != float64(service.CollaborationMemoryDefaultMaxBytes) || empty["can_edit"] != true || empty["can_purge"] != true ||
		empty["restored_from_version"] != nil || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("empty shared context wire: %v", empty)
	}

	key := uuid.NewString()
	write := map[string]any{"expected_version": "0", "content": "External finding"}
	created := testutil.Call(t, testHandler.UpdateCollaborationMemory, collaborationMemoryRequest(http.MethodPut, issue, "", external, key, write)).Want(http.StatusOK)
	receipt := created.Map()
	if receipt["code"] != "memory_updated" || receipt["action"] != "memory_update" || receipt["request_id"] != key || receipt["version"] != "1" ||
		receipt["current_version"] != "1" || receipt["bytes"] != float64(len("External finding")) || receipt["status"] != float64(200) ||
		receipt["protocol_version"] != float64(1) || receipt["issue_id"] != issue || receipt["event_seq"] == nil {
		t.Fatalf("update receipt wire: %v", receipt)
	}
	replay := testutil.Call(t, testHandler.UpdateCollaborationMemory, collaborationMemoryRequest(http.MethodPut, issue, "", external, key, write)).Want(http.StatusOK)
	if replay.Text() != created.Text() {
		t.Fatalf("replayed receipt changed: %s vs %s", replay.Text(), created.Text())
	}

	current := testutil.Call(t, testHandler.GetCollaborationMemory, collaborationMemoryRequest(http.MethodGet, issue, "", testUserID, "", nil)).Want(http.StatusOK)
	view := current.Map()
	author, _ := view["author"].(map[string]any)
	member, _ := author["member"].(map[string]any)
	if view["version"] != "1" || view["content"] != "External finding" || author["kind"] != "member" || author["agent"] != nil ||
		member["user_id"] != external || member["external"] != true || member["name"] != "External context author" {
		t.Fatalf("current version wire: %v", view)
	}
	if strings.Contains(current.Text(), "memory-author-") || strings.Contains(current.Text(), `"email"`) {
		t.Fatal("shared context leaked the author's private identity")
	}

	stale := testutil.Call(t, testHandler.UpdateCollaborationMemory, collaborationMemoryRequest(http.MethodPut, issue, "", testUserID, uuid.NewString(), write)).Want(http.StatusConflict).Map()
	if stale["code"] != "stale_memory_version" || stale["current_version"] != "1" || stale["version"] != nil {
		t.Fatalf("stale refusal wire: %v", stale)
	}

	page := testutil.Call(t, testHandler.ListCollaborationMemoryVersions, collaborationMemoryRequest(http.MethodGet, issue, "/versions", testUserID, "", nil)).Want(http.StatusOK).Map()
	items, _ := page["items"].([]any)
	if page["current_version"] != "1" || page["next_cursor"] != nil || len(items) != 1 || items[0].(map[string]any)["version"] != "1" ||
		items[0].(map[string]any)["purged"] != false || strings.Contains(testutil.Call(t, testHandler.ListCollaborationMemoryVersions,
		collaborationMemoryRequest(http.MethodGet, issue, "/versions", testUserID, "", nil)).Text(), "External finding") {
		t.Fatalf("history page wire: %v", page)
	}
	detail := testutil.Call(t, testHandler.GetCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodGet, issue, "/versions/1", testUserID, "", nil)).Want(http.StatusOK).Map()
	if detail["version"] != "1" || detail["content"] != "External finding" || detail["current_version"] != "1" || detail["purged"] != false {
		t.Fatalf("version detail wire: %v", detail)
	}
	missing := testutil.Call(t, testHandler.GetCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodGet, issue, "/versions/2", testUserID, "", nil)).Want(http.StatusNotFound).Map()
	if missing["code"] != "memory_version_not_found" {
		t.Fatalf("a missing version must not look like issue denial: %v", missing)
	}

	second := map[string]any{"expected_version": "1", "content": "Creator rewrite"}
	testutil.Call(t, testHandler.UpdateCollaborationMemory, collaborationMemoryRequest(http.MethodPut, issue, "", testUserID, uuid.NewString(), second)).Want(http.StatusOK)
	restored := testutil.Call(t, testHandler.RestoreCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodPost, issue, "/versions/1/restore", external, uuid.NewString(),
		map[string]any{"expected_version": "2"})).Want(http.StatusOK).Map()
	if restored["code"] != "memory_restored" || restored["version"] != "3" {
		t.Fatalf("restore receipt wire: %v", restored)
	}
	testutil.Call(t, testHandler.PurgeCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodPost, issue, "/versions/1/purge", external, uuid.NewString(),
		map[string]any{"reason": "credential"})).Want(http.StatusForbidden)
	currentPurge := testutil.Call(t, testHandler.PurgeCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodPost, issue, "/versions/3/purge", testUserID, uuid.NewString(),
		map[string]any{"reason": "credential"})).Want(http.StatusConflict).Map()
	if currentPurge["code"] != "memory_version_current" {
		t.Fatalf("current version purge wire: %v", currentPurge)
	}
	purgeKey := uuid.NewString()
	purged := testutil.Call(t, testHandler.PurgeCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodPost, issue, "/versions/2/purge", testUserID, purgeKey,
		map[string]any{"reason": "credential"})).Want(http.StatusOK).Map()
	if purged["code"] != "memory_purged" || purged["version"] != "2" || purged["current_version"] != "3" {
		t.Fatalf("purge receipt wire: %v", purged)
	}
	purgedDetail := testutil.Call(t, testHandler.GetCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodGet, issue, "/versions/2", external, "", nil)).Want(http.StatusOK).Map()
	if purgedDetail["content"] != nil || purgedDetail["purged"] != true || purgedDetail["purged_at"] == nil || purgedDetail["purged_by_admin"] != false ||
		purgedDetail["purged_by"].(map[string]any)["user_id"] != testUserID || purgedDetail["bytes"] != float64(len("Creator rewrite")) {
		t.Fatalf("purged version wire: %v", purgedDetail)
	}
	recovered := testutil.Call(t, testHandler.GetCollaborationMemoryOperation, collaborationMemoryRequest(http.MethodGet, issue, "/operations/"+purgeKey, testUserID, "", nil)).Want(http.StatusOK).Map()
	if recovered["code"] != "memory_purged" || recovered["request_id"] != purgeKey {
		t.Fatalf("operation recovery wire: %v", recovered)
	}
	unknown := testutil.Call(t, testHandler.GetCollaborationMemoryOperation, collaborationMemoryRequest(http.MethodGet, issue, "/operations/"+uuid.NewString(), testUserID, "", nil)).Want(http.StatusNotFound).Map()
	if unknown["code"] != "operation_not_found" {
		t.Fatalf("missing operation must not look like issue denial: %v", unknown)
	}
}

func TestCollaborationMemoryHTTPRejectsMalformedRequests(t *testing.T) {
	issue := collaborationMemoryIssue(t)
	for _, raw := range []string{
		`{"expected_version":0,"content":"x"}`,
		`{"expected_version":"01","content":"x"}`,
		`{"expected_version":"-1","content":"x"}`,
		`{"expected_version":"","content":"x"}`,
		`{"expected_version":"0"}`,
		`{"content":"x"}`,
		`{"expected_version":"0","content":"x","content":"y"}`,
		`{"expected_version":"0","content":"x","author":"agent"}`,
		`{"expected_version":"0","content":"x"} {}`,
	} {
		testutil.Call(t, testHandler.UpdateCollaborationMemory, collaborationMemoryRequest(http.MethodPut, issue, "", testUserID, uuid.NewString(), raw)).Want(http.StatusBadRequest)
	}
	missingKey := testutil.Call(t, testHandler.UpdateCollaborationMemory, collaborationMemoryRequest(http.MethodPut, issue, "", testUserID, "",
		map[string]any{"expected_version": "0", "content": "x"})).Want(http.StatusBadRequest).Map()
	if missingKey["code"] != "invalid_request_id" {
		t.Fatalf("missing Idempotency-Key: %v", missingKey)
	}
	limit := service.CollaborationMemoryReadLimit(service.CollaborationMemoryDefaultMaxBytes)
	huge, _ := json.Marshal(map[string]any{"expected_version": "0", "content": strings.Repeat("a", int(limit))})
	tooLarge := testutil.Call(t, testHandler.UpdateCollaborationMemory, collaborationMemoryRequest(http.MethodPut, issue, "", testUserID, uuid.NewString(), string(huge))).Want(http.StatusRequestEntityTooLarge).Map()
	if tooLarge["code"] != "request_too_large" {
		t.Fatalf("oversized body: %v", tooLarge)
	}
	overLimit := testutil.Call(t, testHandler.UpdateCollaborationMemory, collaborationMemoryRequest(http.MethodPut, issue, "", testUserID, uuid.NewString(),
		map[string]any{"expected_version": "0", "content": strings.Repeat("界", service.CollaborationMemoryDefaultMaxBytes/3+1)})).Want(http.StatusRequestEntityTooLarge).Map()
	if overLimit["code"] != "memory_too_large" || overLimit["max_bytes"] != float64(service.CollaborationMemoryDefaultMaxBytes) ||
		overLimit["bytes"] != float64(3*(service.CollaborationMemoryDefaultMaxBytes/3+1)) {
		t.Fatalf("decoded content over the limit: %v", overLimit)
	}
	for _, suffix := range []string{"/versions/0", "/versions/01", "/versions/abc"} {
		testutil.Call(t, testHandler.GetCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodGet, issue, suffix, testUserID, "", nil)).Want(http.StatusBadRequest)
	}
	for _, query := range []string{"limit=0", "limit=101", "cursor=0", "cursor=01"} {
		r := collaborationMemoryRequest(http.MethodGet, issue, "/versions", testUserID, "", nil)
		r.URL.RawQuery = query
		testutil.Call(t, testHandler.ListCollaborationMemoryVersions, r).Want(http.StatusBadRequest)
	}
	testutil.Call(t, testHandler.PurgeCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodPost, issue, "/versions/1/purge", testUserID, uuid.NewString(), `{}`)).Want(http.StatusBadRequest)
	testutil.Call(t, testHandler.RestoreCollaborationMemoryVersion, collaborationMemoryRequest(http.MethodPost, issue, "/versions/1/restore", testUserID, uuid.NewString(), `{}`)).Want(http.StatusBadRequest)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("a malformed request created a shared-context version")
	}
	// Unauthorized readers and machine principals see the ordinary issue denial.
	outsider := dbfx.User(t, "Unrelated account", "memory-outsider-"+uuid.NewString()+"@example.test")
	testutil.Call(t, testHandler.GetCollaborationMemory, collaborationMemoryRequest(http.MethodGet, issue, "", outsider, "", nil)).Want(http.StatusNotFound)
	machine := collaborationMemoryRequest(http.MethodGet, issue, "", testUserID, "", nil)
	machine = machine.WithContext(auth.WithAuthenticatedPrincipal(machine.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, testHandler.GetCollaborationMemory, machine).Want(http.StatusNotFound)
}
