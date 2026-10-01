package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManagedTransportUsesScopedCredentialsAndAuthoritativeMaps(t *testing.T) {
	const wsA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const wsB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	const runtimeA = "11111111-1111-4111-8111-111111111111"
	const runtimeB = "22222222-2222-4222-8222-222222222222"
	const taskA = "33333333-3333-4333-8333-333333333333"
	const taskB = "44444444-4444-4444-8444-444444444444"
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		token := r.Header.Get("Authorization")
		workspace, runtimeID, taskID := wsA, runtimeA, taskA
		if token == "Bearer mdt_workspace_b" {
			workspace, runtimeID, taskID = wsB, runtimeB, taskB
		} else if token != "Bearer mdt_workspace_a" {
			t.Errorf("request used unscoped credential: %s", token)
			w.WriteHeader(403)
			return
		}
		switch r.URL.Path {
		case "/api/daemon/register":
			if body["workspace_id"] != workspace {
				t.Error("registration used another workspace token")
			}
			json.NewEncoder(w).Encode(map[string]any{"runtimes": []map[string]string{{"id": runtimeID, "provider": "fake"}}})
		case "/api/daemon/heartbeat":
			if body["runtime_id"] != runtimeID {
				t.Error("heartbeat crossed workspace")
			}
			json.NewEncoder(w).Encode(map[string]any{})
		case "/api/daemon/tasks/claim":
			ids := body["runtime_ids"].([]any)
			if len(ids) != 1 || ids[0] != runtimeID {
				t.Error("batch was not split by known workspace")
			}
			json.NewEncoder(w).Encode(map[string]any{"tasks": []map[string]string{{"id": taskID, "runtime_id": runtimeID, "workspace_id": workspace}}})
		default:
			if !strings.Contains(r.URL.Path, taskID) {
				t.Errorf("task routed through wrong credential: %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("mul_discovery_only")
	client.enableManagedTransport()
	for _, entry := range []struct{ workspace, token string }{{wsA, "mdt_workspace_a"}, {wsB, "mdt_workspace_b"}} {
		err := client.installManagedCredential(entry.workspace, ManagedDaemonCredential{BindingID: "55555555-5555-4555-8555-555555555555", BindingEpoch: "1", CapabilityVersion: "1", DaemonToken: entry.token, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), PrincipalUserID: "66666666-6666-4666-8666-666666666666", AuthVersion: "1"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client.Register(context.Background(), map[string]any{"workspace_id": entry.workspace}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{runtimeA, runtimeB} {
		if _, err := client.SendHeartbeat(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := client.ClaimTasks(context.Background(), "daemon", []string{runtimeA, runtimeB}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks", len(tasks))
	}
	for _, task := range tasks {
		if err = client.StartTask(context.Background(), task.ID); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	before := calls
	mu.Unlock()
	if _, err = client.SendHeartbeat(context.Background(), "unknown-runtime"); err == nil {
		t.Fatal("unknown managed runtime fell back to PAT")
	}
	if err = client.StartTask(context.Background(), "unknown-task"); err == nil {
		t.Fatal("unknown managed task fell back to PAT")
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != before {
		t.Fatal("unknown resource emitted an HTTP request")
	}
}
