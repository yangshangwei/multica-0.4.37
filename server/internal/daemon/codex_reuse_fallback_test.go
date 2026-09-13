package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunTaskCodexReuseFallsBackToFreshPreparation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script app-server fixture is POSIX-only")
	}

	for _, tc := range []struct {
		name             string
		preparationFails bool
	}{
		{"fresh preparation completes the task", false},
		{"fresh preparation failure never launches an agent", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sharedCodexHome := t.TempDir()
			t.Setenv("CODEX_HOME", sharedCodexHome)
			launchLog := filepath.Join(t.TempDir(), "codex-launch.txt")
			t.Setenv("UPSTREAM_CODEX_LAUNCH_LOG", launchLog)
			if tc.preparationFails {
				// The missing referenced catalog affects a fresh home too, unlike
				// the damage to the prior task's home below.
				if err := os.WriteFile(filepath.Join(sharedCodexHome, "config.toml"),
					[]byte("model_catalog_json = \"missing-catalog.json\"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			d, _, cleanup := newLeaderReuseTestDaemon(t)
			defer cleanup()
			fakeCodex := filepath.Join(t.TempDir(), "codex")
			// This is the same minimal app-server handshake used by the agent
			// package's subprocess tests. It records the environment actually
			// launched and the thread setup request, then responds to the
			// handshake and exits when the backend closes stdin after receiving
			// turn/completed. The assertion below checks start versus resume.
			script := `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then printf '%s\n' 'codex-cli 0.144.5'; exit 0; fi
printf '%s\n' "$PWD" "$CODEX_HOME" > "$UPSTREAM_CODEX_LAUNCH_LOG"
IFS= read -r _
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{}}'
IFS= read -r _
IFS= read -r setup_request
printf '%s\n' "$setup_request" >> "$UPSTREAM_CODEX_LAUNCH_LOG"
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thread-reuse-fallback"}}}'
IFS= read -r _
printf '%s\n' '{"jsonrpc":"2.0","id":3,"result":{}}'
printf '%s\n' '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-reuse-fallback","turn":{"id":"turn-1","status":"completed"}}}'
while IFS= read -r _; do :; done
`
			if err := os.WriteFile(fakeCodex, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			d.cfg.Agents = map[string]AgentEntry{"codex": {Path: fakeCodex}}
			d.activeStores = make(map[string]int)
			d.cfg.CodexHandshakeTimeout = time.Second
			d.cfg.CodexThreadHandshakeTimeout = time.Second
			d.runtimeIndex["rt-leader"] = Runtime{ID: "rt-leader", Provider: "codex"}

			task := leaderReuseTestTask("task-codex-fallback")
			// The claimed task still points at a former session whose home is
			// unusable. A fresh environment must not receive that stale resume.
			task.PriorSessionID = "thread-prior-codex"
			task.PriorWorkDir = filepath.Join(d.cfg.WorkspacesRoot, task.WorkspaceID, "0123456789ab", "workdir")
			writeLeaderTaskMarker(t, task.PriorWorkDir, task.AgentID, task.IssueID)
			writeLeaderManagedEnvProvenance(t, task.PriorWorkDir, task.WorkspaceID, task.IssueID, task.AgentID)
			blockedHome := filepath.Join(filepath.Dir(task.PriorWorkDir), "codex-home")
			if err := os.WriteFile(blockedHome, []byte("damaged prior home"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, ok := shouldReusePriorWorkdir(task, nil, d.cfg.WorkspacesRoot); !ok {
				t.Fatal("fixture must reach home preparation with a valid reuse candidate")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result, err := d.runTask(ctx, task, "codex", 0, d.logger)
			if tc.preparationFails {
				if err == nil || !strings.Contains(err.Error(), "prepare execution environment") || !strings.Contains(err.Error(), "missing-catalog.json") {
					t.Fatalf("runTask error = %v, want the fresh preparation's missing-catalog failure", err)
				}
				if _, statErr := os.Stat(launchLog); !os.IsNotExist(statErr) {
					t.Fatalf("Codex launched despite incomplete preparation: launch marker stat error = %v", statErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("runTask did not recover with fresh preparation: %v", err)
			}
			if result.Status != "completed" || result.WorkDir == "" {
				t.Fatalf("task did not complete in a prepared environment: %+v", result)
			}
			if sameDir(t, result.WorkDir, task.PriorWorkDir) {
				t.Fatal("Codex ran in the damaged reused environment")
			}
			launched, err := os.ReadFile(launchLog)
			if err != nil {
				t.Fatalf("read fake Codex launch evidence: %v", err)
			}
			lines := strings.Split(strings.TrimSpace(string(launched)), "\n")
			if len(lines) != 3 || !sameDir(t, lines[0], result.WorkDir) {
				t.Fatalf("launch evidence = %q, want workdir, prepared Codex home, and thread setup request", launched)
			}
			var setupRequest struct {
				Method string `json:"method"`
			}
			if err := json.Unmarshal([]byte(lines[2]), &setupRequest); err != nil {
				t.Fatalf("decode thread setup request: %v", err)
			}
			if setupRequest.Method != "thread/start" || strings.Contains(lines[2], task.PriorSessionID) {
				t.Fatalf("fresh environment must start a new thread without stale session %q: %s", task.PriorSessionID, lines[2])
			}
			wantHome := filepath.Join(filepath.Dir(result.WorkDir), "codex-home")
			if !sameDir(t, lines[1], wantHome) {
				t.Fatalf("launched CODEX_HOME = %q, want fresh task home %q", lines[1], wantHome)
			}
			if _, err := os.Stat(filepath.Join(wantHome, "config.toml")); err != nil {
				t.Fatalf("fresh Codex home lacks its prepared config: %v", err)
			}
			if body, err := os.ReadFile(blockedHome); err != nil || string(body) != "damaged prior home" {
				t.Fatalf("fallback changed the rejected prior home: body=%q err=%v", body, err)
			}
		})
	}
}
