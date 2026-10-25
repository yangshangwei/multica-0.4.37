package daemon

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationHostVersionMatchesDetectedProviderBanner(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ provider, configured, detected, version string }{
		{"claude", "2.1.5", "2.1.5 (Claude Code)", "2.1.5"},
		{"claude", "2.1.5 (Claude Code)", "2.1.5", "2.1.5"},
		{"codex", "0.118.0-alpha.1+build.5", "codex-cli 0.118.0-alpha.1+build.5", "0.118.0-alpha.1+build.5"},
		{"opencode", "v1.17.7", "1.17.7", "1.17.7"},
	} {
		t.Run(test.provider+"/"+test.configured, func(t *testing.T) {
			config := CollaborationRunnerConfig{RuntimeID: uuid.NewString(), LedgerDirectory: filepath.Join(t.TempDir(), "ledger"),
				WorkspaceDirectory: filepath.Join(t.TempDir(), "workspaces"), CLIExecutable: executable, ExpectedRuntimeConfigEpoch: "1",
				Host: &collaborationrun.HostConfig{Provider: test.provider, ProviderVersion: test.configured, ProviderMode: "local", AccountSource: "fixture-account",
					Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 65536, TimeoutSeconds: 600}}}
			raw, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := parseCollaborationRunnerConfig(string(raw))
			if err != nil {
				t.Fatal(err)
			}
			d := &Daemon{cfg: Config{Agents: map[string]AgentEntry{test.provider: {Path: executable}}},
				agentVersions: map[string]string{test.provider: test.detected}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			host, err := d.hostCollaborationExecutor(parsed)
			if err != nil {
				t.Fatalf("detected banner refused the configured Provider version: %v", err)
			}
			capabilities, err := host.Config.Capabilities()
			if err != nil || capabilities.Host.ProviderVersion != test.version {
				t.Fatalf("capability did not retain the canonical Provider version: %v", err)
			}
			alternate := *host.Config
			alternate.ProviderVersion = test.detected
			formatted, err := alternate.Capabilities()
			if err != nil || !reflect.DeepEqual(formatted, capabilities) {
				t.Fatalf("banner formatting changed the managed configuration fingerprint: %v", err)
			}
			if d.agentVersion(test.provider) != test.detected {
				t.Fatal("host admission rewrote the ordinary diagnostic version")
			}
			d.agentVersions[test.provider] = test.version + "-different"
			if _, err := d.hostCollaborationExecutor(parsed); err == nil {
				t.Fatal("a different prerelease identity was admitted")
			}
		})
	}
}
