//go:build agentintegration && !windows

package daemon

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// This no-model probe establishes the production claim seam only. It does not
// execute the fake path or substitute for an actual platform/server claim.
func TestSecurityRemediationOrdinaryDriverPreparation(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		t.Fatal("preparation refuses the real-agent gate")
	}
	dir := t.TempDir()
	provider := filepath.Join(dir, "missing-test-provider")
	cli := filepath.Join(dir, "missing-test-cli")
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/daemon/runtimes/synthetic-runtime/tasks/claim" || r.Header.Get("Authorization") != "Bearer synthetic-daemon-token" {
			t.Errorf("unexpected claim method/path/identity")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"task":{"id":"synthetic-task","agent_id":"synthetic-leader","runtime_id":"synthetic-runtime","issue_id":"synthetic-issue","workspace_id":"synthetic-workspace","is_leader_task":true,"leader_role_resolved":true,"agent":{"instructions":"synthetic briefing"}}}`)
	}))
	defer server.Close()
	driver, err := NewRK1KnowledgeDriver(RK1KnowledgeDriverOptions{
		Provider: "codex",
		Config: Config{ServerBaseURL: server.URL, DaemonID: "synthetic-daemon", WorkspacesRoot: filepath.Join(dir, "workspaces"),
			Agents: map[string]AgentEntry{"codex": {Path: provider, Model: "synthetic-model"}}},
		Token: "synthetic-daemon-token", RuntimeID: "synthetic-runtime", CLIPath: cli,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	task, err := driver.Claim(t.Context())
	if err != nil || task == nil || task.ID != "synthetic-task" || task.AgentID != "synthetic-leader" || !task.IsLeaderTask || !task.LeaderRoleResolved || calls != 1 {
		t.Fatalf("claim contract mismatch: error=%v calls=%d", err, calls)
	}
	if _, err := os.Stat(provider); !os.IsNotExist(err) {
		t.Fatal("test provider path unexpectedly exists")
	}
}

func TestSecurityRemediationRealModelEntry(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("real-agent gate absent; no executable or account access")
	}
	t.Fatal("preparation entry cannot run: isolated tool broker, actual platform evidence driver and reviewed registration are required")
}
