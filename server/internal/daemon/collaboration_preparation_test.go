package daemon

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func pollerTestBundle(t *testing.T) *collaborationrun.ProviderBundleConfig {
	t.Helper()
	return &collaborationrun.ProviderBundleConfig{Version: 1, Provider: "claude", ProtocolFamily: "claude", AdapterID: "static_json_v1", Entrypoint: "/agent",
		Files: []collaborationrun.ProviderBundleFile{{Source: filepath.Join(t.TempDir(), "explicit-unused-fixture"), Destination: "/agent", SHA256: strings.Repeat("a", 64), SizeBytes: 1, Mode: "executable"}},
		Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20,
			GitExpandedBytes: 256 << 20, EntryCount: 65536, TimeoutSeconds: 600}}
}

func TestCollaborationPreparedConfigRequiresExplicitEpochAndExclusiveBundle(t *testing.T) {
	for _, kind := range []string{"valid", "missing_epoch", "rounded_epoch", "executable_override", "argument_override", "incomplete_limits"} {
		t.Run(kind, func(t *testing.T) {
			cfg := CollaborationRunnerConfig{RuntimeID: uuid.NewString(), ExpectedRuntimeConfigEpoch: "9007199254740993", Bundle: pollerTestBundle(t),
				LedgerDirectory: filepath.Join(t.TempDir(), "ledger"), SandboxDirectory: filepath.Join(t.TempDir(), "sandboxes")}
			switch kind {
			case "missing_epoch":
				cfg.ExpectedRuntimeConfigEpoch = ""
			case "rounded_epoch":
				cfg.ExpectedRuntimeConfigEpoch = "9.007199254740993e15"
			case "executable_override":
				cfg.Executable = filepath.Join(t.TempDir(), "another-executable")
			case "argument_override":
				cfg.Arguments = []string{"--another-model"}
			case "incomplete_limits":
				cfg.Bundle.Limits.GitExpandedBytes = 0
			}
			raw, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := parseCollaborationRunnerConfig(string(raw))
			if (err == nil) != (kind == "valid") || kind == "valid" && parsed.ExpectedRuntimeConfigEpoch != "9007199254740993" {
				t.Fatalf("config kind=%s err=%v", kind, err)
			}
		})
	}
}

func TestCollaborationPreparedPollerRegistersBeforeCandidateAdmission(t *testing.T) {
	workspaceID, runtimeID := uuid.NewString(), uuid.NewString()
	var expected protocol.CollaborationRunClaimRequest
	var registration *protocol.CollaborationRuntimeCapabilityRegistration
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mdt_poller_fixture" || r.Header.Get("Cookie") != "" {
			t.Error("poller lost managed credential scope")
		}
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/daemon/register":
			_ = json.NewEncoder(w).Encode(RegisterResponse{Runtimes: []Runtime{{ID: runtimeID, Provider: "claude"}}})
		case "/api/collaboration/runtime/capabilities":
			if r.Method == http.MethodPost {
				var request protocol.CollaborationRuntimeCapabilitiesRequest
				if json.NewDecoder(r.Body).Decode(&request) != nil || request.ExpectedRuntimeConfigEpoch != "4" || request.DeploymentID != expected.DeploymentID || request.BindingID != expected.BindingID {
					t.Error("poller changed registration authority")
				}
				registration = &protocol.CollaborationRuntimeCapabilityRegistration{DeploymentID: request.DeploymentID, InstallationID: request.InstallationID,
					BindingID: request.BindingID, BindingEpoch: request.BindingEpoch, RuntimeConfigEpoch: "5", Capabilities: request.Capabilities}
			}
			epoch := "4"
			if registration != nil {
				epoch = "5"
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRuntimeCapabilitiesResponse{RuntimeID: runtimeID, RuntimeConfigEpoch: epoch, Registration: registration})
		case "/api/collaboration/runtime/runs":
			if registration == nil {
				t.Error("candidate admission ran before capabilities were registered")
			}
			_, _ = io.WriteString(w, `{"run_ids":[]}`)
		default:
			t.Error("unexpected poller route")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	d, identity := collaborationManagedPollerFixture(t, server.URL, workspaceID, runtimeID)
	expected = identity
	if _, err := d.client.Register(t.Context(), map[string]any{"workspace_id": workspaceID}); err != nil {
		t.Fatal(err)
	}
	calls = nil
	d.cfg.CollaborationRunner.Bundle, d.cfg.CollaborationRunner.Executable, d.cfg.CollaborationRunner.ExpectedRuntimeConfigEpoch = pollerTestBundle(t), "", "4"
	poller := &collaborationPoller{daemon: d, config: d.cfg.CollaborationRunner, enabled: true}
	sem := newTaskSlotSemaphore(1)
	slot := <-sem
	var pending sync.WaitGroup
	if poller.dispatch(t.Context(), t.Context(), slot, sem, make(chan struct{}, 1), &pending) {
		t.Fatal("empty candidates dispatched a run")
	}
	want := []string{"GET /api/collaboration/runtime/capabilities", "POST /api/collaboration/runtime/capabilities", "GET /api/collaboration/runtime/runs"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected registration/admission order: %v", calls)
	}
}
