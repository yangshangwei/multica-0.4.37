package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
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

func TestManagedConnectionDetachFailsPendingRPCAndPreservesOtherTransport(t *testing.T) {
	received := make(chan struct{})
	release := make(chan struct{})
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var msg protocol.Message
			if conn.ReadJSON(&msg) != nil {
				return
			}
			if msg.Type == protocol.EventDaemonRPCRequest {
				close(received)
				<-release
				return
			}
		}
	}))
	defer server.Close()
	d := New(Config{ServerBaseURL: server.URL, HeartbeatInterval: time.Hour, WorkspacesRoot: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	originalGeneration := d.wsRPC.attach(func([]byte) (*wsOutbound, error) { return &wsOutbound{}, nil })
	d.wsRPC.markRPCV1Supported(originalGeneration)
	scoped := newWSRPCClient(0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connectionDone := make(chan struct{})
	go func() {
		defer close(connectionDone)
		d.runTaskWakeupConnectionUsingTransport(ctx, d.client, scoped, &atomic.Bool{}, "workspace", nil, make(chan taskWakeup, 1), nil)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for scoped.currentGeneration() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	callDone := make(chan error, 1)
	go func() { _, err := scoped.Call(ctx, "tasks.claim", time.Minute, nil, nil); callDone <- err }()
	select {
	case <-received:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("RPC never reached server")
	}
	close(release)
	select {
	case <-connectionDone:
	case <-time.After(2 * time.Second):
		t.Fatal("connection did not terminate")
	}
	select {
	case err := <-callDone:
		if !errors.Is(err, errWSRPCUncertain) {
			t.Errorf("sent claim error = %v, want uncertain", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("pending scoped claim did not fail immediately on detach")
	}
	if scoped.supportsRPCV1() {
		t.Error("disconnected scope retained capability")
	}
	scoped.mu.Lock()
	attached := scoped.sendFrame != nil
	scoped.mu.Unlock()
	if attached {
		t.Error("disconnected scope retained sender")
	}
	if d.wsRPC.currentGeneration() != originalGeneration || !d.wsRPC.supportsRPCV1() {
		t.Error("scoped teardown detached unrelated default transport")
	}
}

func TestTaskWakeupDiagnosticsExcludeEndpointAndPeerSecrets(t *testing.T) {
	const secret = "private-credential-marker"
	tests := []struct {
		name  string
		err   error
		class string
		code  int
	}{
		{"url", &url.Error{Op: "dial", URL: "wss://user:" + secret + "@host/path?token=" + secret, Err: context.DeadlineExceeded}, "deadline", 0},
		{"peer", fmt.Errorf("wrapped: %w", &websocket.CloseError{Code: 1008, Text: secret}), "peer_close", 1008},
		{"network", &net.OpError{Op: "read", Err: context.DeadlineExceeded}, "deadline", 0},
		{"untrusted", errors.New(secret), "other", 0},
		{"closed", net.ErrClosed, "network_closed", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class, code := taskWakeupErrorClassification(tt.err)
			if class != tt.class || code != tt.code {
				t.Fatalf("classification=(%s,%d)", class, code)
			}
			var out bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&out, nil))
			logger.Info("ended", "error_class", class, "close_code", code)
			if strings.Contains(out.String(), secret) {
				t.Fatal("diagnostic leaked secret")
			}
		})
	}
}
