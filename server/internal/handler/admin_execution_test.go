package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestAdminExecutionsProjectionPaginationAndIssueCounts(t *testing.T) {
	login := adminHandlerSetup(t)
	ws := dbfx.Workspace(t, "S03 workspace", "s03-"+uuid.NewString())
	if _, err := testHandler.Queries.AssignWorkspaceOrganization(context.Background(), parseUUID(ws)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	runtimeID := fx.Runtime(t, "PRIVATE RUNTIME")
	agent := fx.Agent(t, "PRIVATE AGENT", runtimeID)
	issue := fx.Issue(t, "PRIVATE TITLE", testutil.Cols{"description": "PRIVATE BODY"})
	created := time.Now().UTC().Add(-time.Hour)
	first := fx.Task(t, agent, testutil.Cols{"runtime_id": runtimeID, "issue_id": issue, "status": "failed", "created_at": created, "attempt": 1, "error": "PRIVATE ERROR /home/secret", "failure_reason": "PRIVATE FAILURE", "work_dir": "/home/secret", "context": testutil.Raw(`'{"prompt":"PRIVATE PROMPT"}'::jsonb`)})
	second := fx.Task(t, agent, testutil.Cols{"runtime_id": runtimeID, "issue_id": issue, "status": "completed", "created_at": created, "attempt": 2, "retry_of_task_id": first, "parent_task_id": first})
	fx.Task(t, agent, testutil.Cols{"runtime_id": runtimeID, "issue_id": issue, "status": "completed", "created_at": created.Add(-60 * 24 * time.Hour), "attempt": 1})
	fx.Insert(t, "task_usage", testutil.Cols{"task_id": second, "provider": "openai", "model": "model", "input_tokens": 10, "output_tokens": 3, "cache_read_tokens": 0, "cache_write_tokens": 0})
	router := chi.NewRouter()
	router.Use(middleware.Auth(testHandler.Queries, nil, nil))
	router.Get("/api/admin/tasks", testHandler.AdminTasks)
	router.Get("/api/admin/tasks/{id}", testHandler.AdminTask)
	router.Get("/api/admin/issues", testHandler.AdminIssues)
	call := func(path string) *testutil.Response {
		req := testutil.JSONRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+login.Token)
		return testutil.Call(t, router.ServeHTTP, req)
	}
	base := "/api/admin/tasks?workspace_id=" + ws + "&limit=1"
	var page struct {
		Items  []map[string]any `json:"items"`
		Cursor *string          `json:"next_cursor"`
		AsOf   string           `json:"as_of"`
	}
	call(base).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Cursor == nil {
		t.Fatalf("page: %+v", page)
	}
	all := append([]map[string]any{}, page.Items...)
	snapshot := page.AsOf
	fx.Task(t, agent, testutil.Cols{"runtime_id": runtimeID, "status": "queued"})
	page.Items = nil
	call(base + "&cursor=" + url.QueryEscape(*page.Cursor)).Want(200).JSON(&page)
	all = append(all, page.Items...)
	if len(all) != 2 || all[0]["id"] == all[1]["id"] || page.AsOf != snapshot || page.Cursor != nil {
		t.Fatalf("unstable pagination: %+v %+v", all, page)
	}
	for _, item := range all {
		b, _ := json.Marshal(item)
		if strings.Contains(string(b), "PRIVATE") || strings.Contains(string(b), "/home/") {
			t.Fatalf("private data leaked: %s", b)
		}
		if item["title"] != nil || item["content_access"] != false {
			t.Fatalf("nonmember content access: %v", item)
		}
	}
	var detail map[string]any
	call("/api/admin/tasks/" + first).Want(200).JSON(&detail)
	if detail["usage"] != nil || detail["started_at"] != nil || detail["failure_code"] != "unknown" {
		t.Fatalf("unknown metadata invented: %v", detail)
	}
	call("/api/admin/tasks/" + second).Want(200).JSON(&detail)
	if detail["retry_of_task_id"] != first || detail["attempt"] != float64(2) || detail["usage"] == nil || detail["model"] != "model" {
		t.Fatalf("lost execution lineage/usage: %v", detail)
	}
	var issues struct {
		Items []map[string]any `json:"items"`
	}
	call("/api/admin/issues?workspace_id=" + ws).Want(200).JSON(&issues)
	if len(issues.Items) != 1 || issues.Items[0]["execution_count"] != float64(2) || issues.Items[0]["title"] != nil {
		t.Fatalf("issue/execution counts: %+v", issues)
	}
	fx.Member(t, ws, login.User.ID, "member")
	call("/api/admin/issues?workspace_id=" + ws).Want(200).JSON(&issues)
	if issues.Items[0]["title"] != "PRIVATE TITLE" || issues.Items[0]["content_access"] != true {
		t.Fatalf("original membership not honored: %+v", issues)
	}
	call("/api/admin/tasks?workspace_id=" + ws + "&status=failed&source=issue&issue_id=" + issue).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] != first {
		t.Fatalf("combined filters: %+v", page)
	}
	call("/api/admin/tasks/" + uuid.NewString()).Want(http.StatusNotFound)
	ordinary := passwordRegister(t, fmt.Sprintf("s03ordinary%d", time.Now().UnixNano()))
	req := testutil.JSONRequest("GET", base, nil)
	req.Header.Set("Authorization", "Bearer "+ordinary.Token)
	testutil.Call(t, router.ServeHTTP, req).Want(403)
}

func TestAdminExecutionsSourceScopeAndChatPrivacy(t *testing.T) {
	login := adminHandlerSetup(t)
	ws := dbfx.Workspace(t, "Sources", "sources-"+uuid.NewString())
	if _, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(ws)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	runtimeID := fx.Runtime(t, "Runtime")
	agent := fx.Agent(t, "Agent", runtimeID)
	owner := fx.User(t, "Conversation owner", "s03-owner-"+uuid.NewString()+"@example.invalid")
	chat := fx.Insert(t, "chat_session", testutil.Cols{"workspace_id": ws, "agent_id": agent, "creator_id": owner, "title": "PRIVATE CHAT", "status": "active"})
	issue := fx.Issue(t, "Automation issue", testutil.Cols{"origin_type": "autopilot"})
	autopilotID := fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": ws, "title": "Automation", "assignee_id": agent, "status": "paused", "created_by_type": "member", "created_by_id": login.User.ID})
	runID := fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": autopilotID, "source": "manual", "status": "completed"})
	for _, source := range []string{"autopilot_issue", "chat", "autopilot", "quick_create", "unknown"} {
		fields := testutil.Cols{"runtime_id": runtimeID, "status": "completed", "accountable_user_id": login.User.ID}
		switch source {
		case "autopilot_issue":
			fields["issue_id"] = issue
		case "chat":
			fields["chat_session_id"] = chat
		case "autopilot":
			fields["autopilot_run_id"] = runID
		case "quick_create":
			fields["context"] = testutil.Raw(`'{"type":"quick_create","prompt":"PRIVATE PROMPT"}'::jsonb`)
		}
		id := fx.Task(t, agent, fields)
		var list struct {
			Items []map[string]any `json:"items"`
		}
		path := "/api/admin/tasks?workspace_id=" + ws + "&source=" + source + "&runtime_id=" + runtimeID + "&user_id=" + login.User.ID
		passwordCall(t, "GET", path, nil, login.Token, testHandler.AdminTasks).Want(200).JSON(&list)
		if len(list.Items) != 1 || list.Items[0]["id"] != id || list.Items[0]["source"] != source {
			t.Fatalf("source filter %s: %+v", source, list)
		}
		if source == "chat" {
			fx.Member(t, ws, login.User.ID, "owner")
			passwordCall(t, "GET", path, nil, login.Token, testHandler.AdminTasks).Want(200).JSON(&list)
			if list.Items[0]["content_access"] != false || list.Items[0]["title"] != nil {
				t.Fatalf("workspace owner saw private conversation: %v", list)
			}
		}
	}
	hidden := dbfx.Workspace(t, "Unassigned", "unassigned-"+uuid.NewString())
	foreign := testutil.New(testPool, hidden, login.User.ID)
	rt := foreign.Runtime(t, "Other")
	otherAgent := foreign.Agent(t, "Other", rt)
	id := foreign.Task(t, otherAgent, testutil.Cols{"runtime_id": rt})
	var out struct {
		Items []map[string]any `json:"items"`
	}
	passwordCall(t, "GET", "/api/admin/tasks?task_id="+id, nil, login.Token, testHandler.AdminTasks).Want(200).JSON(&out)
	if len(out.Items) != 0 {
		t.Fatal("execution outside organization scope was returned")
	}
	router := chi.NewRouter()
	router.Use(middleware.Auth(testHandler.Queries, nil, nil))
	router.Get("/api/admin/tasks/{id}", testHandler.AdminTask)
	req := testutil.JSONRequest("GET", "/api/admin/tasks/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	testutil.Call(t, router.ServeHTTP, req).Want(404)
	passwordCall(t, "GET", "/api/admin/tasks?organization_id="+uuid.NewString(), nil, login.Token, testHandler.AdminTasks).Want(400)
}

func TestAdminExecutionChatContentMatchesOriginalVisibility(t *testing.T) {
	login := adminHandlerSetup(t)
	ws := dbfx.Workspace(t, "Chat boundaries", "chat-boundary-"+uuid.NewString())
	if _, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(ws)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	fx.Member(t, ws, login.User.ID, "member")
	other := fx.User(t, "Other agent owner", "s03-chat-"+uuid.NewString()+"@example.invalid")
	runtimeID := fx.Runtime(t, "Chat runtime")
	agent := fx.Agent(t, "Private agent", runtimeID, testutil.Cols{"owner_id": other})
	chat := fx.Insert(t, "chat_session", testutil.Cols{"workspace_id": ws, "agent_id": agent, "creator_id": login.User.ID, "title": "Private conversation", "status": "active", "explicitly_created_at": time.Now()})
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtimeID, "chat_session_id": chat, "status": "completed"})
	router := chi.NewRouter()
	router.Use(middleware.Auth(testHandler.Queries, nil, nil))
	router.Get("/api/admin/tasks/{id}", testHandler.AdminTask)
	router.Group(func(r chi.Router) {
		r.Use(middleware.RequireWorkspaceMember(testHandler.Queries))
		r.Get("/api/chat/{sessionId}", testHandler.GetChatSession)
	})
	check := func(access bool, originalStatus int) {
		t.Helper()
		req := testutil.JSONRequest("GET", "/api/admin/tasks/"+task, nil)
		req.Header.Set("Authorization", "Bearer "+login.Token)
		var detail map[string]any
		testutil.Call(t, router.ServeHTTP, req).Want(200).JSON(&detail)
		if detail["content_access"] != access {
			t.Fatalf("metadata content access %v, want %v", detail, access)
		}
		req = testutil.JSONRequest("GET", "/api/chat/"+chat, nil)
		req.Header.Set("Authorization", "Bearer "+login.Token)
		req.Header.Set("X-Workspace-ID", ws)
		testutil.Call(t, router.ServeHTTP, req).Want(originalStatus)
	}
	check(false, 403)
	fx.Exec(t, "UPDATE agent SET owner_id=$2 WHERE id=$1", agent, login.User.ID)
	check(true, 200)
	fx.Exec(t, "UPDATE chat_session SET explicitly_created_at=NULL WHERE id=$1", chat)
	check(false, 404)
	fx.Exec(t, "UPDATE chat_session SET explicitly_created_at=now() WHERE id=$1", chat)
	fx.Exec(t, "UPDATE agent SET owner_id=$2,permission_mode='public_to' WHERE id=$1", agent, other)
	fx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": agent, "target_type": "member", "target_id": login.User.ID})
	check(true, 200)
}

func TestAdminExecutionWaitingLocalDirectoryFilter(t *testing.T) {
	login := adminHandlerSetup(t)
	ws := dbfx.Workspace(t, "Waiting execution", "waiting-"+uuid.NewString())
	if _, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(ws)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	runtimeID := fx.Runtime(t, "Waiting runtime")
	agent := fx.Agent(t, "Waiting agent", runtimeID)
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtimeID, "status": "waiting_local_directory", "wait_reason": "PRIVATE DIRECTORY /home/private"})
	var result struct {
		Items []map[string]any `json:"items"`
	}
	response := passwordCall(t, "GET", "/api/admin/tasks?workspace_id="+ws+"&status=waiting_local_directory", nil, login.Token, testHandler.AdminTasks).Want(200)
	response.JSON(&result)
	if len(result.Items) != 1 || result.Items[0]["id"] != task || result.Items[0]["status"] != "waiting_local_directory" {
		t.Fatalf("lost waiting execution: %v", result)
	}
	if strings.Contains(response.Text(), "PRIVATE DIRECTORY") || strings.Contains(response.Text(), "/home/private") {
		t.Fatal("private wait reason leaked")
	}
}
