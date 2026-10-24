package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type adminCollaborationHTTPFixture struct {
	admin, creator, handler LoginResponse
	workspace, issue        string
	router                  *chi.Mux
}

func newAdminCollaborationHTTPFixture(t *testing.T) *adminCollaborationHTTPFixture {
	t.Helper()
	f := &adminCollaborationHTTPFixture{admin: adminHandlerSetup(t)}
	f.creator = passwordRegister(t, fmt.Sprintf("collabcreator%d", time.Now().UnixNano()))
	f.handler = passwordRegister(t, fmt.Sprintf("collabhandler%d", time.Now().UnixNano()))
	f.workspace = dbfx.Workspace(t, "Scoped administration source", "s5-"+uuid.NewString())
	dbfx.Member(t, f.workspace, f.creator.User.ID, "owner")
	org, err := testHandler.Queries.GetInternalOrganization(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dbfx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": org.ID, "workspace_id": f.workspace}, "workspace_id=$1", f.workspace)
	if err = testHandler.Queries.SeedIssueStatusEntries(context.Background(), parseUUID(f.workspace)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
	f.issue = dbfx.Issue(t, "PRIVATE ADMINISTRATION TITLE", testutil.Cols{"workspace_id": f.workspace, "description": "PRIVATE ADMINISTRATION BODY",
		"creator_type": "member", "creator_id": f.creator.User.ID, "assignee_type": "member", "assignee_id": f.handler.User.ID})
	dbfx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": f.issue}, "issue_id=$1", f.issue)
	dbfx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": f.issue})
	for _, user := range []string{f.creator.User.ID, f.handler.User.ID} {
		dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue, "user_id": user}, "issue_id=$1 AND user_id=$2", f.issue, user)
		dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": user}, "user_id=$1", user)
	}
	for _, table := range []string{"issue_handoff", "collaboration_event", "collaboration_notification"} {
		dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", f.issue)
	}
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_id=$1`, f.issue)
	for _, user := range []string{f.creator.User.ID, f.handler.User.ID, f.admin.User.ID} {
		dbfx.Cleanup(t, `DELETE FROM collaboration_account_invalidation WHERE user_id=$1`, user)
	}
	r := chi.NewRouter()
	r.Use(PlatformAdminNoStore)
	r.Use(middleware.Auth(testHandler.Queries, nil, nil))
	r.Get("/api/admin/collaboration/issues", testHandler.AdminCollaborationIssues)
	r.Get("/api/admin/collaboration/issues/{id}", testHandler.AdminCollaborationIssue)
	r.Get("/api/admin/collaboration/issues/{id}/reopen-candidates", testHandler.AdminCollaborationReopenCandidates)
	r.Post("/api/admin/collaboration/issues/{id}/lifecycle", testHandler.AdminCollaborationLifecycle)
	r.Get("/api/admin/collaboration/issues/{id}/operations/{requestId}", testHandler.AdminCollaborationOperation)
	r.Get("/api/collaboration/issues/{id}", testHandler.GetCollaborationIssue)
	r.Get("/api/collaboration/issues/{id}/comments", testHandler.ListCollaborationComments)
	r.Get("/api/collaboration/issues/{id}/resources", testHandler.GetCollaborationResources)
	f.router = r
	return f
}

func (f *adminCollaborationHTTPFixture) call(t *testing.T, method, path string, body any, key string) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+f.admin.Token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	// Administration must select the source from the scoped object, not an
	// ambient workspace left behind by a desktop account-tab transition.
	req.Header.Set("X-Workspace-ID", uuid.NewString())
	return testutil.Call(t, f.router.ServeHTTP, req)
}

func adminCollaborationHTTPBody(detail service.AdminCollaborationDetail, action string) map[string]any {
	for _, options := range detail.Actions {
		if options.Action == action {
			return map[string]any{"action": action, "expected_collaboration_revision": fmt.Sprint(detail.CollaborationRevision),
				"expected_status": detail.Status, "target_status": options.DefaultStatus, "status_options_version": options.StatusOptionsVersion,
				"reason": "Explicitly resolve the disabled creator's collaboration"}
		}
	}
	return nil
}

func TestAdminCollaborationHTTPMinimalReadsLifecycleAndOriginalRecovery(t *testing.T) {
	f := newAdminCollaborationHTTPFixture(t)
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator.User.ID)
	path := "/api/admin/collaboration/issues/" + f.issue
	var detail service.AdminCollaborationDetail
	res := f.call(t, "GET", path, nil, "").Want(200).JSON(&detail)
	if len(detail.Actions) != 2 || detail.SuccessorRequired || detail.Creator.UserID != parseUUID(f.creator.User.ID) || !detail.Creator.Disabled {
		t.Fatalf("unexpected administration detail: %+v", detail)
	}
	var page service.AdminCollaborationPage
	listed := f.call(t, "GET", "/api/admin/collaboration/issues?limit=1", nil, "").Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0].IssueID != parseUUID(f.issue) || page.NextCursor.Valid {
		t.Fatalf("scoped management list: %+v", page)
	}
	for _, response := range []*testutil.Response{res, listed} {
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("management response is cacheable")
		}
		for _, private := range []string{"PRIVATE ADMINISTRATION", "description", "password_hash", "payload_hash", "run_id", "session_id"} {
			if strings.Contains(response.Text(), private) {
				t.Fatalf("management response exposed %q", private)
			}
		}
	}
	body := adminCollaborationHTTPBody(detail, "complete")
	key := uuid.NewString()
	var result struct {
		Operation map[string]any                    `json:"operation"`
		Result    service.AdminCollaborationReceipt `json:"result"`
	}
	posted := f.call(t, "POST", path+"/lifecycle", body, key).Want(200).JSON(&result)
	if result.Operation["actor_id"] != f.admin.User.ID || result.Operation["target_kind"] != "collaboration_issue" || result.Result.Code != "completed" ||
		result.Result.CreatorUserID != parseUUID(f.creator.User.ID) || result.Result.HandlerUserID != parseUUID(f.handler.User.ID) || result.Result.QueueState != "closed" || result.Result.RequestID != key {
		t.Fatalf("administrative completion response = %+v", result)
	}
	var recovered struct {
		Operation map[string]any                    `json:"operation"`
		Result    service.AdminCollaborationReceipt `json:"result"`
	}
	f.call(t, "GET", path+"/operations/"+key, nil, "").Want(200).JSON(&recovered)
	if !reflect.DeepEqual(result, recovered) || posted.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("GET recovery did not retain the exact original result")
	}
	for _, ordinary := range []string{"", "/comments", "/resources"} {
		response := f.call(t, "GET", "/api/collaboration/issues/"+f.issue+ordinary, nil, "").Want(404)
		if strings.Contains(response.Text(), "PRIVATE") {
			t.Fatal("ordinary endpoint disclosed content to the administrator")
		}
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, f.issue, f.admin.User.ID) != 0 ||
		dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.admin.User.ID) != 0 {
		t.Fatal("administration granted ordinary membership")
	}
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.creator.User.ID)
	f.call(t, "GET", path+"/operations/"+key, nil, "").Want(200)
	f.call(t, "POST", path+"/lifecycle", body, uuid.NewString()).Want(409)
	if dbfx.Count(t, `SELECT count(*) FROM admin_operation WHERE target_id=$1`, f.issue) != 1 {
		t.Fatal("restored creator permitted another administration operation")
	}
}

func TestAdminCollaborationHTTPExplicitSuccessorAndPausedReopen(t *testing.T) {
	f := newAdminCollaborationHTTPFixture(t)
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator.User.ID)
	path := "/api/admin/collaboration/issues/" + f.issue
	var detail service.AdminCollaborationDetail
	f.call(t, "GET", path, nil, "").Want(200).JSON(&detail)
	f.call(t, "POST", path+"/lifecycle", adminCollaborationHTTPBody(detail, "cancel"), uuid.NewString()).Want(200)
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler.User.ID)
	candidate := passwordRegister(t, fmt.Sprintf("s5candidate%d", time.Now().UnixNano()))
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue, "user_id": candidate.User.ID}, "issue_id=$1 AND user_id=$2", f.issue, candidate.User.ID)
	f.call(t, "GET", path, nil, "").Want(200).JSON(&detail)
	if !detail.SuccessorRequired {
		t.Fatal("double-disabled reopen did not require a successor")
	}
	var candidates service.AdminCollaborationCandidatePage
	f.call(t, "GET", path+"/reopen-candidates", nil, "").Want(200).JSON(&candidates)
	if len(candidates.Items) != 1 || candidates.Items[0].UserID != parseUUID(candidate.User.ID) || !candidates.Items[0].External {
		t.Fatalf("incorrect candidate projection: %+v", candidates)
	}
	body := adminCollaborationHTTPBody(detail, "reopen")
	f.call(t, "POST", path+"/lifecycle", body, uuid.NewString()).Want(409)
	body["successor_user_id"] = candidate.User.ID
	key := uuid.NewString()
	var result struct {
		Result service.AdminCollaborationReceipt `json:"result"`
	}
	f.call(t, "POST", path+"/lifecycle", body, key).Want(200).JSON(&result)
	if result.Result.HandlerUserID != parseUUID(candidate.User.ID) || result.Result.PreviousHandlerUserID != parseUUID(f.handler.User.ID) || result.Result.QueueState != "paused" || result.Result.Code != "reopened" {
		t.Fatalf("incorrect R40 result: %+v", result)
	}
	f.call(t, "POST", path+"/lifecycle", body, key).Want(200)
	if dbfx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='admin_reopen' AND actor_user_id=$2 AND to_user_id=$3`, f.issue, f.admin.User.ID, candidate.User.ID) != 1 {
		t.Fatal("HTTP replay duplicated the successor handoff")
	}
}

func TestAdminCollaborationHTTPRejectsMalformedAndUnauthorizedRequests(t *testing.T) {
	f := newAdminCollaborationHTTPFixture(t)
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator.User.ID)
	path := "/api/admin/collaboration/issues/" + f.issue
	var detail service.AdminCollaborationDetail
	f.call(t, "GET", path, nil, "").Want(200).JSON(&detail)
	body := adminCollaborationHTTPBody(detail, "complete")
	for _, malformed := range []json.RawMessage{
		json.RawMessage(`{"action":"complete","action":"cancel"}`),
		json.RawMessage(`{"action":"complete","unexpected":true}`),
		json.RawMessage(`null`),
	} {
		f.call(t, "POST", path+"/lifecycle", malformed, uuid.NewString()).Want(400)
	}
	for _, successor := range []any{nil, "", 7, f.handler.User.ID} {
		body["successor_user_id"] = successor
		response := f.call(t, "POST", path+"/lifecycle", body, uuid.NewString()).Want(400)
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("invalid successor response is cacheable")
		}
	}
	delete(body, "successor_user_id")
	body["expected_collaboration_revision"] = "01"
	f.call(t, "POST", path+"/lifecycle", body, uuid.NewString()).Want(400)
	body["expected_collaboration_revision"] = "1"
	f.call(t, "GET", "/api/admin/collaboration/issues?limit=0", nil, "").Want(400)
	f.call(t, "GET", "/api/admin/collaboration/issues?cursor=unknown", nil, "").Want(400)
	for _, featureDisabled := range []bool{false, true} {
		if featureDisabled {
			testHandler.cfg.PlatformAdminEnabled = false
		} else {
			dbfx.Exec(t, `UPDATE platform_role_binding SET role='platform_observer' WHERE user_id=$1`, f.admin.User.ID)
		}
		for _, route := range []string{"/api/admin/collaboration/issues", path, path + "/reopen-candidates", path + "/operations/" + uuid.NewString()} {
			f.call(t, "GET", route, nil, "").Want(403)
		}
		f.call(t, "POST", path+"/lifecycle", body, uuid.NewString()).Want(403)
	}
	req := testutil.JSONRequest("GET", "/api/admin/collaboration/issues", nil)
	unauthenticated := testutil.Call(t, f.router.ServeHTTP, req).Want(http.StatusUnauthorized)
	if unauthenticated.Header().Get("Cache-Control") != "no-store" || dbfx.Count(t, `SELECT count(*) FROM admin_operation WHERE target_id=$1`, f.issue) != 0 {
		t.Fatal("invalid requests leaked caching authority or persisted an operation")
	}
}
