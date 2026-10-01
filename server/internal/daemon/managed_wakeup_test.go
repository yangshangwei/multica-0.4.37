package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestManagedWorkspaceWebSocketsAndRPCRemainIsolated(t *testing.T) {
	workspaces := []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}
	runtimes := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}
	taskIDs := []string{"33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"}
	tokens := []string{"mdt_workspace_a", "mdt_workspace_b"}
	var mu sync.Mutex
	observed := make(map[string]string)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := -1
		for i, token := range tokens {
			if r.Header.Get("Authorization") == "Bearer "+token {
				index = i
			}
		}
		if index < 0 || r.URL.Query().Get("runtime_ids") != runtimes[index] {
			t.Error("websocket crossed managed workspace credentials")
			w.WriteHeader(403)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		mu.Lock()
		observed[tokens[index]] = r.URL.Query().Get("runtime_ids")
		mu.Unlock()
		payload, _ := json.Marshal(HeartbeatResponse{RuntimeID: runtimes[index], ServerCapabilities: []string{protocol.DaemonCapabilityRPCV1}})
		if err = conn.WriteJSON(protocol.Message{Type: protocol.EventDaemonHeartbeatAck, Payload: payload}); err != nil {
			return
		}
		for {
			var message protocol.Message
			if err = conn.ReadJSON(&message); err != nil {
				return
			}
			if message.Type != protocol.EventDaemonRPCRequest {
				continue
			}
			var request protocol.RPCRequestPayload
			if err = json.Unmarshal(message.Payload, &request); err != nil {
				t.Error(err)
				return
			}
			var body struct {
				RuntimeIDs []string `json:"runtime_ids"`
			}
			if err = json.Unmarshal(request.Body, &body); err != nil {
				t.Error(err)
				return
			}
			if len(body.RuntimeIDs) != 1 || body.RuntimeIDs[0] != runtimes[index] {
				t.Error("RPC carried another workspace runtime")
			}
			responseBody, _ := json.Marshal(map[string]any{"tasks": []Task{{ID: taskIDs[index], RuntimeID: runtimes[index], WorkspaceID: workspaces[index]}}})
			payload, _ = json.Marshal(protocol.RPCResponsePayload{RequestID: request.RequestID, Status: 200, Body: responseBody})
			if err = conn.WriteJSON(protocol.Message{Type: protocol.EventDaemonRPCResponse, Payload: payload}); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	daemon := New(Config{ServerBaseURL: server.URL, HeartbeatInterval: time.Hour, WorkspacesRoot: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	daemon.client.SetToken("mul_discovery_only")
	daemon.client.enableManagedTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for i, workspace := range workspaces {
		err := daemon.client.installManagedCredential(workspace, ManagedDaemonCredential{BindingID: "55555555-5555-4555-8555-555555555555", BindingEpoch: "1", CapabilityVersion: "1", DaemonToken: tokens[i], ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), PrincipalUserID: "66666666-6666-4666-8666-666666666666", AuthVersion: "1"})
		if err != nil {
			t.Fatal(err)
		}
		if err = daemon.client.recordManagedRegistration(workspace, []Runtime{{ID: runtimes[i]}}); err != nil {
			t.Fatal(err)
		}
		scoped := daemon.client.managed.workspaces[workspace]
		ids := []string{runtimes[i]}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = daemon.runTaskWakeupConnectionUsingTransport(ctx, scoped.client, scoped.rpc, &scoped.batchUnsupported, scoped.workspaceID, ids, make(chan taskWakeup, 8), make(chan struct{}))
		}()
	}
	ready := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ready = true
		for _, workspace := range workspaces {
			if !daemon.client.managed.workspaces[workspace].rpc.supportsRPCV1() {
				ready = false
			}
		}
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		cancel()
		wg.Wait()
		t.Fatal("managed websockets did not negotiate RPC independently")
	}
	tasks, err := daemon.ClaimTasksWSFirst(ctx, "managed-daemon", runtimes, 2)
	if err != nil {
		cancel()
		wg.Wait()
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("claimed %d tasks, want 2", len(tasks))
	}
	mu.Lock()
	if len(observed) != 2 {
		t.Error("both scoped websocket handshakes were not observed")
	}
	mu.Unlock()
	cancel()
	wg.Wait()
}

func TestManagedHeartbeatAckRejectsAnotherWorkspace(t *testing.T) {
	daemon := New(Config{WorkspacesRoot: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	daemon.client.enableManagedTransport()
	daemon.client.managed.runtimes["runtime-a"] = "workspace-a"
	rpc := newWSRPCClient(time.Second)
	generation := rpc.attach(func(frame []byte) (*wsOutbound, error) { return &wsOutbound{data: frame}, nil })
	daemon.handleWSHeartbeatAckUsingTransport(context.Background(), &HeartbeatResponse{RuntimeID: "runtime-a", ServerCapabilities: []string{protocol.DaemonCapabilityRPCV1}}, rpc, generation, "workspace-b")
	if rpc.supportsRPCV1() || daemon.wsHeartbeatRecentlyAcked("runtime-a") {
		t.Fatal("foreign workspace acknowledged the connection")
	}
}
