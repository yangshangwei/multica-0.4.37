package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationHostConfigurationUsesRegisteredOrdinaryRuntime(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := CollaborationRunnerConfig{RuntimeID: uuid.NewString(), LedgerDirectory: filepath.Join(t.TempDir(), "ledger"),
		WorkspaceDirectory: filepath.Join(t.TempDir(), "workspaces"), CLIExecutable: executable, ExpectedRuntimeConfigEpoch: "4",
		Host: &collaborationrun.HostConfig{Provider: "claude", ProviderVersion: "fixture-1", ProviderMode: "local", AccountSource: "fixture-owner",
			Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 65536, TimeoutSeconds: 600}}}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseCollaborationRunnerConfig(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	d := &Daemon{cfg: Config{Agents: map[string]AgentEntry{"claude": {Path: executable, Model: "ordinary-default"}}}, agentVersions: map[string]string{"claude": "fixture-1"}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	host, err := d.hostCollaborationExecutor(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if host.Config.Executable != executable || host.Config.DefaultModel != "ordinary-default" {
		t.Fatal("collaboration bypassed registered ordinary runtime settings")
	}
	caps, err := host.Config.Capabilities()
	if err != nil || caps.Containment != protocol.CollaborationContainmentHostProcessSupervision || caps.PreparationVersion != 2 {
		t.Fatalf("native platform kept namespace admission: %v", err)
	}
	d.agentVersions["claude"] = "different-version"
	if _, err := d.hostCollaborationExecutor(parsed); err == nil {
		t.Fatal("unconfirmed provider version silently replaced the authorized runtime")
	}
	for _, kind := range []string{"legacy_directory", "legacy_executable", "legacy_arguments", "no_epoch", "gateway"} {
		t.Run(kind, func(t *testing.T) {
			changed := cfg
			host := *cfg.Host
			changed.Host = &host
			switch kind {
			case "legacy_directory":
				changed.SandboxDirectory = t.TempDir()
			case "legacy_executable":
				changed.Executable = executable
			case "legacy_arguments":
				changed.Arguments = []string{"--ambient"}
			case "no_epoch":
				changed.ExpectedRuntimeConfigEpoch = ""
			case "gateway":
				changed.Host.ProviderMode = "gateway"
			}
			raw, _ := json.Marshal(changed)
			if _, err := parseCollaborationRunnerConfig(string(raw)); err == nil {
				t.Fatal("mixed/unverified runtime configuration was accepted")
			}
		})
	}
}

func TestCollaborationHostPollerUsesNativeModeAndSharedLedger(t *testing.T) {
	runtimeID, workspaceID := uuid.NewString(), uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/daemon/register" {
			t.Error("unexpected request during host initialization")
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(RegisterResponse{Runtimes: []Runtime{{ID: runtimeID, Provider: "claude"}}})
	}))
	defer server.Close()
	d, identity := collaborationManagedPollerFixture(t, server.URL, workspaceID, runtimeID)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d.cfg.Agents = map[string]AgentEntry{"claude": {Path: executable}}
	available := d.cfg.Agents
	d.agentsAvailable.Store(&available)
	d.agentVersions["claude"] = "fixture-1"
	if _, err := d.client.Register(t.Context(), map[string]any{"workspace_id": workspaceID}); err != nil {
		t.Fatal(err)
	}
	cfg := d.cfg.CollaborationRunner
	cfg.Executable, cfg.SandboxDirectory = "", ""
	cfg.WorkspaceDirectory, cfg.CLIExecutable, cfg.ExpectedRuntimeConfigEpoch = filepath.Join(t.TempDir(), "host-work"), executable, "1"
	cfg.Host = &collaborationrun.HostConfig{Provider: "claude", ProviderVersion: "fixture-1", ProviderMode: "local", AccountSource: "fixture-account",
		Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 65536, TimeoutSeconds: 600}}
	poller, reserved, err := d.initializeCollaborationPoller(context.Background())
	if err != nil || reserved != 0 || poller == nil || !poller.enabled || poller.host == nil || poller.controller != nil {
		t.Fatalf("native host poller unavailable: reserved=%d, err=%v", reserved, err)
	}
	_, claim, err := poller.managedClient()
	if err != nil || claim.ProtocolVersion != 3 || claim.Containment != protocol.CollaborationContainmentHostProcessSupervision || claim.DeploymentID != identity.DeploymentID || claim.InstallationID != identity.InstallationID {
		t.Fatalf("host poller changed mode or installed identity: %v", err)
	}
}
