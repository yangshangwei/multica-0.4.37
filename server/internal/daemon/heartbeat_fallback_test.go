package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// This tests fallback policy through the native HTTP client, not the scheduler's
// elapsed recovery time or fleet capacity. It never starts the daemon Run loop.
func TestManagedHeartbeatHTTPFallbackUsesScopedCredentials(t *testing.T) {
	workspaces := []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}
	runtimes := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}
	tokens := []string{"mdt_fallback_a", "mdt_fallback_b"}
	calls := make(chan string, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/daemon/heartbeat" {
			t.Errorf("unexpected daemon side effect: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		var body struct {
			RuntimeID           string `json:"runtime_id"`
			SupportsBatchImport bool   `json:"supports_batch_import"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		index := -1
		for i, id := range runtimes {
			if body.RuntimeID == id {
				index = i
			}
		}
		if index < 0 || r.Header.Get("Authorization") != "Bearer "+tokens[index] || !body.SupportsBatchImport {
			t.Error("HTTP fallback lost its managed runtime credential or heartbeat payload")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		calls <- body.RuntimeID
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(HeartbeatResponse{RuntimeID: body.RuntimeID, Status: "ok"}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	d := New(Config{ServerBaseURL: server.URL, HeartbeatInterval: 15 * time.Second, WorkspacesRoot: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.client.SetToken("mul_discovery_only")
	d.client.enableManagedTransport()
	for i, workspace := range workspaces {
		err := d.client.installManagedCredential(workspace, ManagedDaemonCredential{BindingID: "55555555-5555-4555-8555-555555555555", BindingEpoch: "1", CapabilityVersion: "1", DaemonToken: tokens[i], ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), PrincipalUserID: "66666666-6666-4666-8666-666666666666", AuthVersion: "1"})
		if err != nil {
			t.Fatal(err)
		}
		if err := d.client.recordManagedRegistration(workspace, []Runtime{{ID: runtimes[i]}}); err != nil {
			t.Fatal(err)
		}
	}
	if got := d.wsHeartbeatFreshness(); got != 30*time.Second {
		t.Fatalf("default heartbeat freshness = %s, want 30s", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := func(t *testing.T, index int, wantHTTP bool) {
		t.Helper()
		if transient := d.runHeartbeatTick(ctx, runtimes[index]); transient {
			t.Fatal("unexpected HTTP heartbeat failure")
		}
		select {
		case got := <-calls:
			if !wantHTTP || got != runtimes[index] {
				t.Fatalf("unexpected fallback request for %s (want HTTP %v)", got, wantHTTP)
			}
		default:
			if wantHTTP {
				t.Fatal("expected native HTTP heartbeat request")
			}
		}
	}
	ack := func(index int) {
		scoped := d.client.managed.workspaces[workspaces[index]]
		d.handleWSHeartbeatAckUsingTransport(ctx, &HeartbeatResponse{RuntimeID: runtimes[index], Status: "ok"}, scoped.rpc, scoped.rpc.currentGeneration(), workspaces[index])
	}
	// Without ACKs, each runtime uses its own workspace credential.
	tick(t, 0, true)
	tick(t, 1, true)

	// Fresh ACKs suppress duplicate HTTP work for both runtimes.
	ack(0)
	ack(1)
	tick(t, 0, false)
	tick(t, 1, false)

	// Expiring one ACK resumes only that runtime's HTTP fallback.
	d.wsHBMu.Lock()
	d.wsHBLastAck[runtimes[0]] = time.Now().Add(-d.wsHeartbeatFreshness() - time.Second)
	d.wsHBMu.Unlock()
	tick(t, 0, true)
	tick(t, 1, false)

	// Disconnecting a workspace clears its ACK without affecting its sibling.
	ack(0)
	d.clearManagedWSHeartbeatAcks(workspaces[0], []string{runtimes[0]})
	tick(t, 0, true)
	tick(t, 1, false)
}
