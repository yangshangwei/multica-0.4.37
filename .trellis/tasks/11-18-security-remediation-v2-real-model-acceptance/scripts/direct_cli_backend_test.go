//go:build agentintegration && !windows

package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSecurityDirectCLIBackendNativeContract(t *testing.T) {
	for _, tc := range []struct {
		name, events string
		exit         int
		wantStatus   string
		missing      bool
	}{
		{"completed", `{"type":"thread.started","thread_id":"11111111-1111-4111-8111-111111111111"}
{"type":"item.completed","item":{"id":"m1","type":"agent_message","text":"Native result"}}
{"type":"turn.completed","usage":{"input_tokens":11,"cached_input_tokens":3,"output_tokens":4}}
`, 0, "completed", false},
		{"empty", "", 0, "failed", true},
		{"exit_after_complete", `{"type":"thread.started","thread_id":"11111111-1111-4111-8111-111111111111"}
{"type":"turn.completed"}
`, 7, "failed", false},
		{"conflicting_threads", `{"type":"thread.started","thread_id":"11111111-1111-4111-8111-111111111111"}
{"type":"thread.started","thread_id":"22222222-2222-4222-8222-222222222222"}
{"type":"turn.completed"}
`, 0, "failed", false},
		{"duplicate_thread", `{"type":"thread.started","thread_id":"11111111-1111-4111-8111-111111111111"}
{"type":"thread.started","thread_id":"11111111-1111-4111-8111-111111111111"}
{"type":"turn.completed"}
`, 0, "failed", false},
		{"malformed", `{"type":"thread.started","thread_id":"11111111-1111-4111-8111-111111111111"}
not-json
{"type":"turn.completed"}
`, 0, "failed", false},
		{"failed", `{"type":"thread.started","thread_id":"11111111-1111-4111-8111-111111111111"}
{"type":"turn.failed","error":{"message":"synthetic failure"}}
`, 1, "failed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			work := filepath.Join(root, "work")
			if err := os.Mkdir(work, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte("Complete synthetic identity\nEND\n"), 0600); err != nil {
				t.Fatal(err)
			}
			events := filepath.Join(root, "events")
			if err := os.WriteFile(events, []byte(tc.events), 0600); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(root, "fake-codex-exec")
			source := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SECREM_ARGS\"\ncat > \"$SECREM_STDIN\"\ncat \"$SECREM_EVENTS\"\nexit " + string(rune('0'+tc.exit)) + "\n"
			if err := os.WriteFile(fake, []byte(source), 0700); err != nil {
				t.Fatal(err)
			}
			cfg := Config{ExecutablePath: fake, TaskID: "synthetic-task", Env: map[string]string{"SECREM_ARGS": filepath.Join(root, "args"), "SECREM_STDIN": filepath.Join(root, "stdin"), "SECREM_EVENTS": events}}
			b := &securityDirectCLIBackend{cfg: cfg, registration: securityDirectCLIConfig{Mode: "fake", ExecutionMode: "official-cli-direct", OutputDir: root}}
			prompt := "Exact input with Unicode 中文 and trailing newline.\n"
			session, err := b.Execute(context.Background(), prompt, ExecOptions{Cwd: work, Model: "synthetic-model", Timeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			for range session.Messages {
			}
			result := <-session.Result
			if result.Status != tc.wantStatus || result.BusinessOutcomeMissing != tc.missing || !result.ProcessExited {
				t.Fatalf("result=%+v", result)
			}
			actual, err := os.ReadFile(filepath.Join(root, "stdin"))
			if err != nil || string(actual) != prompt {
				t.Fatal("exact prompt was not delivered")
			}
			args, _ := os.ReadFile(filepath.Join(root, "args"))
			if string(args) != "exec\n--json\n--skip-git-repo-check\n-\n" {
				t.Fatalf("not official CLI args: %q", args)
			}
			meta, _ := os.ReadFile(filepath.Join(root, "direct-cli", "synthetic-task", "metadata.json"))
			var observed map[string]any
			if json.Unmarshal(meta, &observed) != nil || observed["agents_unchanged"] != true {
				t.Fatal("instruction evidence missing")
			}
			if tc.name == "completed" && (result.Output != "Native result" || result.Usage["synthetic-model"].CacheReadTokens != 3) {
				t.Fatalf("native result/usage lost: %+v", result)
			}
		})
	}
}

func TestSecurityDirectCLIBackendGatesBeforeProvider(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "registration.json")
	payload := []byte(`{"mode":"real","execution_mode":"official-cli-direct","output_dir":"` + root + `"}`)
	if err := os.WriteFile(p, payload, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	t.Setenv("SECREM_HARNESS_CONFIG", p)
	t.Setenv("SECREM_HARNESS_CONFIG_SHA256", hex.EncodeToString(digest[:]))
	t.Setenv("SECREM_DIRECT_CLI", "1")
	t.Setenv("MULTICA_RUN_REAL_AGENT_SMOKE", "")
	t.Setenv("SECREM_REAL_EXECUTION_AUTHORIZED", "")
	if _, err := newSecurityDirectCLIBackend(Config{ExecutablePath: filepath.Join(root, "missing")}); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("real provider admitted without opt-in", err)
	}
	t.Setenv("MULTICA_RUN_REAL_AGENT_SMOKE", "1")
	t.Setenv("SECREM_REAL_EXECUTION_AUTHORIZED", "1")
	t.Setenv("SECREM_HARNESS_CONFIG_SHA256", strings.Repeat("0", 64))
	if _, err := newSecurityDirectCLIBackend(Config{ExecutablePath: filepath.Join(root, "missing")}); err == nil {
		t.Fatal("changed registration admitted")
	}
}

func TestSecurityDirectCLIBackendResumeIsExplicit(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	os.Mkdir(work, 0700)
	os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte("complete"), 0600)
	fake := filepath.Join(root, "fake")
	os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SECREM_ARGS\"\ncat >/dev/null\nprintf '%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"33333333-3333-4333-8333-333333333333\"}' '{\"type\":\"turn.completed\"}'\n"), 0700)
	b := &securityDirectCLIBackend{cfg: Config{ExecutablePath: fake, TaskID: "resume", Env: map[string]string{"SECREM_ARGS": filepath.Join(root, "args")}}, registration: securityDirectCLIConfig{OutputDir: root}}
	s, err := b.Execute(context.Background(), "unaltered resumed input", ExecOptions{Cwd: work, ResumeSessionID: "33333333-3333-4333-8333-333333333333", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for range s.Messages {
	}
	result := <-s.Result
	if result.Status != "completed" {
		t.Fatal(result)
	}
	args, _ := os.ReadFile(filepath.Join(root, "args"))
	if string(args) != "exec\nresume\n--json\n--skip-git-repo-check\n33333333-3333-4333-8333-333333333333\n-\n" {
		t.Fatalf("resume discarded: %q", args)
	}
}

func TestSecurityDirectCLIDeliversFrozenP1Material(t *testing.T) {
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte("Complete synthetic identity\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtures := filepath.Join(root, "fixture-task", "fixtures")
	if err := os.MkdirAll(filepath.Join(fixtures, "repos", "production"), 0700); err != nil {
		t.Fatal(err)
	}
	material := []byte("{\"synthetic\":true,\"deployment_exists\":false}\n")
	hash := sha256.Sum256(material)
	if err := os.WriteFile(filepath.Join(fixtures, "repos", "production", "drill.json"), material, 0600); err != nil {
		t.Fatal(err)
	}
	inventory, _ := json.Marshal(map[string]string{"fixtures/repos/production/drill.json": hex.EncodeToString(hash[:])})
	manifest := filepath.Join(fixtures, "freeze.sha256.json")
	os.WriteFile(manifest, inventory, 0600)
	fake := filepath.Join(root, "fake")
	os.WriteFile(fake, []byte("#!/bin/sh\ncat >/dev/null\ncat drill.json > \"$SECREM_FIXTURE_CAPTURE\"\nprintf '%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"33333333-3333-4333-8333-333333333333\"}' '{\"type\":\"turn.completed\"}'\n"), 0700)
	b := &securityDirectCLIBackend{cfg: Config{ExecutablePath: fake, TaskID: "fixture", Env: map[string]string{"SECREM_FIXTURE_CAPTURE": filepath.Join(root, "observed")}}, registration: securityDirectCLIConfig{OutputDir: root, Scenario: "P1", FixtureManifestPath: manifest}}
	s, err := b.Execute(context.Background(), "unaltered input", ExecOptions{Cwd: work, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for range s.Messages {
	}
	result := <-s.Result
	if result.Status != "completed" {
		t.Fatal(result)
	}
	observed, _ := os.ReadFile(filepath.Join(root, "observed"))
	if string(observed) != string(material) {
		t.Fatal("frozen fixture changed")
	}
	if _, err := b.Execute(context.Background(), "second input", ExecOptions{Cwd: work}); err == nil {
		t.Fatal("existing fixture overwritten")
	}
}
