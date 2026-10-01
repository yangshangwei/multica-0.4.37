package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestManagedAdmissionHTTPAndWebSocketCannotClaimStoppedInstallation(t *testing.T) {
	f := managedDaemonSetup(t)
	fx := testutil.New(testPool, f.workspace, f.owner)
	agent := fx.Agent(t, "Stopped transport fixture", f.runtime)
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": f.runtime})
	fx.Exec(t, "UPDATE managed_installation SET admission='stopped',admission_version=2 WHERE id=(SELECT installation_id FROM installation_daemon_binding WHERE id=$1)", f.binding)
	router := chi.NewRouter()
	router.Use(middleware.DaemonAuth(f.h.Queries, nil, nil, nil))
	router.Post("/api/daemon/runtimes/{runtimeId}/tasks/claim", f.h.ClaimTaskByRuntime)
	router.Post("/api/daemon/tasks/claim", f.h.ClaimTasksByRuntime)
	router.Post("/api/daemon/claim", f.h.ClaimTasksByRuntime)
	body := map[string]any{"daemon_id": f.daemon, "runtime_ids": []string{f.runtime}, "max_tasks": 1}
	for _, path := range []string{"/api/daemon/runtimes/" + f.runtime + "/tasks/claim", "/api/daemon/tasks/claim", "/api/daemon/claim"} {
		req := testutil.JSONRequest("POST", path, body)
		req.Header.Set("Authorization", "Bearer "+f.token)
		var result struct {
			Task  *AgentTaskResponse  `json:"task"`
			Tasks []AgentTaskResponse `json:"tasks"`
		}
		testutil.Call(t, router.ServeHTTP, req).Want(200).JSON(&result)
		if result.Task != nil || len(result.Tasks) != 0 {
			t.Fatalf("%s bypassed stopped admission", path)
		}
	}
	server := httptest.NewServer(middleware.DaemonAuth(f.h.Queries, nil, nil, nil)(http.HandlerFunc(f.h.DaemonWebSocket)))
	defer server.Close()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?runtime_id="+f.runtime, http.Header{"Authorization": {"Bearer " + f.token}})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err = connection.WriteJSON(map[string]any{"type": "daemon:rpc_request", "payload": map[string]any{"request_id": "stopped-claim", "method": "tasks.claim", "body": body}}); err != nil {
		t.Fatal(err)
	}
	if err = connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		var message struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err = connection.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		if message.Type != "daemon:rpc_response" {
			continue
		}
		var result struct {
			RequestID string `json:"request_id"`
			Status    int    `json:"status"`
			Body      struct {
				Tasks []AgentTaskResponse `json:"tasks"`
			} `json:"body"`
		}
		if err = json.Unmarshal(message.Payload, &result); err != nil {
			t.Fatal(err)
		}
		if result.RequestID != "stopped-claim" || result.Status != 200 || len(result.Body.Tasks) != 0 {
			t.Fatalf("WS bypassed stopped admission: %s", message.Payload)
		}
		break
	}
	if fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'", task) != 1 {
		t.Fatal("a transport dispatched the stopped task")
	}
}
