package agent

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

// The test binary is the provider fixture, selected by an explicit child-only
// environment marker before test argument parsing. No installed CLI is used.
func init() {
	if os.Getenv("AGENT_PRIVATE_STDERR_FIXTURE") != "1" {
		return
	}
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		os.Exit(41)
	}
	_, _ = fmt.Fprintln(os.Stderr, "old-diagnostic-not-retained", strings.Repeat("x", 4096), input, "api_key=fixture-secret-for-redaction", "retained-stderr-tail")
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"result","is_error":true,"result":"provider rejected the turn"}`)
	os.Exit(1)
}

func TestClaudePrivateStderrPolicyIsExecutionScoped(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	for _, private := range []bool{true, false, true} {
		logs.Reset()
		backend, err := ResolveBackend("claude", Config{ExecutablePath: executable, Logger: logger, PrivateInput: private, Env: map[string]string{"AGENT_PRIVATE_STDERR_FIXTURE": "1"}})
		if err != nil {
			t.Fatal(err)
		}
		session, err := backend.Execute(context.Background(), "private-stderr-policy-sentinel", ExecOptions{Timeout: 5 * time.Second, Cwd: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		for range session.Messages {
		}
		result := <-session.Result
		if result.Status != "failed" || result.Output != "" || !strings.Contains(result.Error, "private-stderr-policy-sentinel") || !strings.Contains(result.Error, "retained-stderr-tail") {
			t.Fatal("stderr policy changed failed evidence")
		}
		if strings.Contains(result.Error, "old-diagnostic-not-retained") || len(result.Error) > agentStderrTailBytes+1024 {
			t.Fatal("stderr diagnostic is not bounded")
		}
		if private && strings.Contains(result.Error, "fixture-secret-for-redaction") {
			t.Fatal("private diagnostic skipped the existing secret sanitizer")
		}
		if strings.Contains(logs.String(), "private-stderr-policy-sentinel") == private {
			t.Fatalf("private=%v: raw stderr logging policy was not per execution", private)
		}
		if !strings.Contains(logs.String(), "claude started") || !strings.Contains(logs.String(), "claude finished") {
			t.Fatal("stderr policy discarded lifecycle metadata")
		}
		logger.Debug("shared-logger-still-enabled")
		if !strings.Contains(logs.String(), "shared-logger-still-enabled") {
			t.Fatal("private execution mutated shared logger")
		}
	}
}
