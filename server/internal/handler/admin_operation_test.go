package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestAdminCancellationHTTPMetadataPrivacyReceiptAndDaemonAck(t *testing.T) {
	f := managedDaemonSetup(t)
	admin := adminHandlerSetup(t)
	f.h.cfg.PlatformAdminEnabled = true
	dbfx.Exec(t, "UPDATE agent_runtime SET metadata=metadata||'{\"capabilities\":[\"admin_cancel_ack_v1\"]}'::jsonb WHERE id=$1", f.runtime)
	fx := testutil.New(testPool, f.workspace, f.owner)
	agent := fx.Agent(t, "PRIVATE AGENT", f.runtime)
	issue := fx.Issue(t, "PRIVATE TITLE", testutil.Cols{"description": "PRIVATE BODY"})
	var installation string
	dbfx.QueryRow(t, "SELECT installation_id::text FROM installation_daemon_binding WHERE id=$1", f.binding).Scan(&installation)
	dispatched := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": f.runtime, "issue_id": issue, "status": "dispatched", "dispatched_at": dispatched, "execution_installation_id": installation, "execution_binding_id": f.binding, "execution_binding_epoch": int64(1), "work_dir": "/private/working/directory", "context": testutil.Raw(`'{"prompt":"PRIVATE PROMPT"}'::jsonb`)})
	router := chi.NewRouter()
	router.Use(middleware.Auth(f.h.Queries, nil, nil))
	router.Get("/api/admin/tasks/{id}", f.h.AdminTask)
	router.Post("/api/admin/tasks/{id}/cancel", f.h.AdminCancelTask)
	router.Get("/api/admin/operations/{id}", f.h.AdminOperation)
	router.Get("/api/admin/operations", f.h.AdminOperations)
	call := func(method, path string, body any, key string) *testutil.Response {
		req := testutil.JSONRequest(method, path, body)
		req.Header.Set("Authorization", "Bearer "+admin.Token)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		return testutil.Call(t, router.ServeHTTP, req)
	}
	var detail map[string]any
	call("GET", "/api/admin/tasks/"+task, nil, "").Want(200).JSON(&detail)
	raw, _ := json.Marshal(detail)
	if strings.Contains(string(raw), "PRIVATE") || strings.Contains(string(raw), "/private/") {
		t.Fatal("admin detail exposed private task content")
	}
	if detail["content_access"] != false {
		t.Fatal("platform role granted workspace content access")
	}
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='running' WHERE id=$1", task)
	fence := detail["execution_fence"].(map[string]any)
	delete(fence, "target_version")
	body := map[string]any{"expected_execution_fence": fence, "reason": "Stop isolated test execution"}
	key := uuid.NewString()
	var response struct {
		Operation map[string]any `json:"operation"`
		Target    map[string]any `json:"target"`
	}
	call("POST", "/api/admin/tasks/"+task+"/cancel", body, key).Want(http.StatusAccepted).JSON(&response)
	if response.Operation["state"] != "applied" || response.Operation["confirmation"] != "pending" || response.Target["status"] != "cancelled" {
		t.Fatalf("incorrect cancellation receipt: %+v", response)
	}
	opID := response.Operation["id"].(string)
	call("POST", "/api/admin/tasks/"+task+"/cancel", body, key).Want(http.StatusAccepted).JSON(&response)
	if response.Operation["id"] != opID {
		t.Fatal("idempotent retry changed operation")
	}
	var found struct {
		Items []map[string]any `json:"items"`
	}
	call("GET", "/api/admin/operations?idempotency_key="+key, nil, "").Want(200).JSON(&found)
	if len(found.Items) != 1 || found.Items[0]["id"] != opID {
		t.Fatal("original key did not recover cancellation")
	}
	daemonRouter := chi.NewRouter()
	daemonRouter.Use(middleware.DaemonAuth(f.h.Queries, nil, nil, nil))
	daemonRouter.Get("/api/daemon/tasks/{taskId}/status", f.h.GetTaskStatus)
	daemonRouter.Post("/api/daemon/tasks/{taskId}/cancel-ack", f.h.AckTaskCancelled)
	daemonCall := func(method, path string, body any) *testutil.Response {
		req := testutil.JSONRequest(method, path, body)
		req.Header.Set("Authorization", "Bearer "+f.token)
		return testutil.Call(t, daemonRouter.ServeHTTP, req)
	}
	var status struct {
		Status       string         `json:"status"`
		Cancellation map[string]any `json:"cancellation"`
	}
	daemonCall("GET", "/api/daemon/tasks/"+task+"/status", nil).Want(200).JSON(&status)
	if status.Status != "cancelled" || status.Cancellation["operation_id"] != opID || status.Cancellation["binding_epoch"] != "1" {
		t.Fatalf("missing durable cancellation metadata: %+v", status)
	}
	ack := map[string]any{"operation_id": opID, "binding_epoch": "1", "execution_fence": status.Cancellation["execution_fence"], "outcome": "not_observed"}
	daemonCall("POST", "/api/daemon/tasks/"+task+"/cancel-ack", ack).Want(200)
	var operation map[string]any
	call("GET", "/api/admin/operations/"+opID, nil, "").Want(200).JSON(&operation)
	if operation["state"] != "applied" || operation["confirmed_at"] != nil {
		t.Fatal("not_observed claimed process exit")
	}
	ack["outcome"] = "stopped"
	ack["branch_name"] = 42
	daemonCall("POST", "/api/daemon/tasks/"+task+"/cancel-ack", ack).Want(400)
	call("GET", "/api/admin/operations/"+opID, nil, "").Want(200).JSON(&operation)
	if operation["state"] != "applied" || operation["confirmed_at"] != nil {
		t.Fatal("malformed receipt confirmed a cancellation")
	}
	delete(ack, "branch_name")
	encodedAck, err := json.Marshal(ack)
	if err != nil {
		t.Fatal(err)
	}
	trailing, err := http.NewRequest("POST", "/api/daemon/tasks/"+task+"/cancel-ack", strings.NewReader(string(encodedAck)+` {}`))
	if err != nil {
		t.Fatal(err)
	}
	trailing.Header.Set("Authorization", "Bearer "+f.token)
	trailing.Header.Set("Content-Type", "application/json")
	testutil.Call(t, daemonRouter.ServeHTTP, trailing).Want(400)
	daemonCall("POST", "/api/daemon/tasks/"+task+"/cancel-ack", ack).Want(200)
	call("GET", "/api/admin/operations/"+opID, nil, "").Want(200).JSON(&operation)
	if operation["state"] != "succeeded" || operation["confirmation"] != "confirmed" {
		t.Fatalf("stopped receipt was not recorded: %v", operation)
	}
	ack["binding_epoch"] = "2"
	daemonCall("POST", "/api/daemon/tasks/"+task+"/cancel-ack", ack).Want(409)
}

func TestAdminCancellationHTTPDefinitiveFenceFailureIsRecoverable(t *testing.T) {
	admin := adminHandlerSetup(t)
	workspace := dbfx.Workspace(t, "Rejected cancellation", "cancel-rejected-"+uuid.NewString())
	if _, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(workspace)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", workspace)
	fx := testutil.New(testPool, workspace, admin.User.ID)
	runtime := fx.Runtime(t, "Rejected runtime")
	agent := fx.Agent(t, "Rejected agent", runtime)
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime})
	router := chi.NewRouter()
	router.Use(middleware.Auth(testHandler.Queries, nil, nil))
	router.Post("/api/admin/tasks/{id}/cancel", testHandler.AdminCancelTask)
	router.Get("/api/admin/operations", testHandler.AdminOperations)
	key := uuid.NewString()
	req := testutil.JSONRequest("POST", "/api/admin/tasks/"+task+"/cancel", map[string]any{"reason": "Reject stale queue view", "expected_execution_fence": map[string]any{"runtime_id": runtime, "dispatched_at": nil, "target_version": "2"}})
	req.Header.Set("Authorization", "Bearer "+admin.Token)
	req.Header.Set("Idempotency-Key", key)
	testutil.Call(t, router.ServeHTTP, req).Want(409)
	lookup := testutil.JSONRequest("GET", "/api/admin/operations?idempotency_key="+key, nil)
	lookup.Header.Set("Authorization", "Bearer "+admin.Token)
	var response struct {
		Items []map[string]any `json:"items"`
	}
	testutil.Call(t, router.ServeHTTP, lookup).Want(200).JSON(&response)
	if len(response.Items) != 1 || response.Items[0]["state"] != "failed" || response.Items[0]["result_code"] != "execution_fence_conflict" || response.Items[0]["applied_at"] != nil {
		t.Fatalf("missing definitive failed receipt: %+v", response)
	}
	if n := fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'", task); n != 1 {
		t.Fatal("rejected cancellation modified task")
	}
}
