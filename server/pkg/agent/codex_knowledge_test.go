package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const codexKnowledgeThread = "019c0000-0000-7000-8000-000000000001"

func init() {
	if path := os.Getenv("MULTICA_TEST_CODEX_KNOWLEDGE"); path != "" {
		runCodexKnowledgeFake(path)
		os.Exit(0)
	}
}

// This is the test executable itself, not an installed CLI or account.
func runCodexKnowledgeFake(path string) {
	mode := os.Getenv("MULTICA_TEST_CODEX_KNOWLEDGE_MODE")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(90)
	}
	defer f.Close()
	fmt.Fprintln(f, "started")
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for {
		var request map[string]any
		if decoder.Decode(&request) != nil {
			return
		}
		raw, _ := json.Marshal(request)
		fmt.Fprintln(f, string(raw))
		method, _ := request["method"].(string)
		id, ok := request["id"]
		if !ok {
			continue
		}
		result := map[string]any{}
		switch method {
		case "initialize":
			if mode == "initialize_timeout" {
				continue
			}
		case "thread/start", "thread/resume":
			if mode == "resume_rejected" {
				encoder.Encode(map[string]any{"id": id, "error": map[string]any{"code": -1, "message": "private-sentinel"}})
				continue
			}
			params, _ := request["params"].(map[string]any)
			ephemeral, _ := params["ephemeral"].(bool)
			thread := map[string]any{"id": codexKnowledgeThread, "ephemeral": ephemeral, "path": nil}
			if mode == "missing_id" {
				delete(thread, "id")
			}
			if mode == "missing_ephemeral" {
				delete(thread, "ephemeral")
			}
			if mode == "persisted_host" {
				thread["ephemeral"] = false
			}
			if mode == "host_path" {
				thread["path"] = "persisted.jsonl"
			}
			result["thread"] = thread
		case "turn/start":
			if strings.HasPrefix(mode, "binding_") {
				runCodexKnowledgeBindingFake(encoder, id, mode)
				continue
			}
			result["turn"] = map[string]any{"id": "turn-1"}
			encoder.Encode(map[string]any{"id": id, "result": result})
			encoder.Encode(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": codexKnowledgeThread, "turn": map[string]any{"id": "turn-1"}}})
			status := "completed"
			turn := map[string]any{"id": "turn-1", "status": status}
			if mode == "private_error" {
				fmt.Fprintln(os.Stderr, "private-sentinel")
				encoder.Encode(map[string]any{"method": "error", "params": map[string]any{"threadId": codexKnowledgeThread, "turnId": "turn-1", "error": map[string]any{"message": "private-sentinel"}, "willRetry": false}})
				turn["status"] = "failed"
				turn["error"] = map[string]any{"message": "private-sentinel"}
			}
			encoder.Encode(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": codexKnowledgeThread, "turnId": "turn-1", "item": map[string]any{"type": "agentMessage", "id": "answer", "text": "private-answer", "phase": "final_answer"}}})
			encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": codexKnowledgeThread, "turn": turn}})
			continue
		}
		encoder.Encode(map[string]any{"id": id, "result": result})
	}
}

func codexKnowledgeFixture(t *testing.T, mode string) (Config, ExecOptions, string, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(dir, "rpc.jsonl")
	logs := &bytes.Buffer{}
	cfg := Config{ExecutablePath: exe, BuiltinRuntime: true, CLIVersion: "fake-codex", Logger: slog.New(slog.NewTextHandler(logs, nil)), Env: map[string]string{
		"CODEX_HOME": dir, "MULTICA_TEST_CODEX_KNOWLEDGE": record, "MULTICA_TEST_CODEX_KNOWLEDGE_MODE": mode,
	}}
	opts := ExecOptions{Cwd: dir, Model: "test-model", Timeout: 3 * time.Second, HandshakeTimeout: 100 * time.Millisecond,
		Knowledge: &KnowledgeLaunchIntent{ConfigurationID: "codex-config", Session: KnowledgeSessionIntent{Kind: KnowledgeSessionFresh, Generation: "generation"}}}
	return cfg, opts, record, logs
}

func TestKnowledgeCodexThreadAdmission(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		resume, ackFail, deny bool
		turns, observations   int
		reason                string
	}{
		{name: "fresh", turns: 1, observations: 1},
		{name: "resume", resume: true, turns: 1, observations: 1},
		{name: "ack_refused", ackFail: true, observations: 1, reason: KnowledgeAuthorizationDenied},
		{name: "resume_rejected", resume: true, reason: KnowledgeContextChanged},
		{name: "wrong_resume", resume: true, observations: 1, reason: KnowledgeContextChanged},
		{name: "missing_id", reason: KnowledgeContextChanged},
		{name: "missing_ephemeral", reason: KnowledgeContextChanged},
		{name: "private_error", turns: 1, observations: 1, reason: KnowledgeUnavailable},
		{name: "initialize_timeout", reason: KnowledgeUnavailable},
		{name: "denied", deny: true, reason: KnowledgeAuthorizationDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, opts, record, logs := codexKnowledgeFixture(t, tc.name)
			if tc.resume {
				opts.ResumeSessionID = codexKnowledgeThread
				opts.Knowledge.Session.Kind = KnowledgeSessionResume
				opts.Knowledge.Session.NativeSessionID = codexKnowledgeThread
			}
			if tc.name == "wrong_resume" {
				opts.ResumeSessionID = "019c0000-0000-7000-8000-000000000002"
				opts.Knowledge.Session.NativeSessionID = opts.ResumeSessionID
			}
			observations := 0
			recorder := &lifecycleRecorder{}
			opts.Lifecycle = NewOrdinaryLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
				if tc.deny {
					return LaunchCallbacks{}, &KnowledgeLaunchError{Reason: KnowledgeAuthorizationDenied}
				}
				callbacks, err := recorder.observer().BeforeLaunch(ctx, info)
				callbacks.NativeSessionObserved = func(observation NativeSessionObservation) error {
					observations++
					raw, _ := os.ReadFile(record)
					if strings.Contains(string(raw), `"method":"turn/start"`) {
						t.Error("knowledge input sent before synchronous ACK")
					}
					if observation.Source != "system" {
						t.Error("unexpected native source")
					}
					if tc.ackFail {
						return &KnowledgeLaunchError{Reason: KnowledgeAuthorizationDenied}
					}
					return nil
				}
				return callbacks, err
			}))
			digest, err := KnowledgeInvocationFingerprint("codex", cfg, opts)
			if err != nil {
				t.Fatal(err)
			}
			opts.Knowledge.InvocationFingerprint = digest
			backend, err := New("codex", cfg)
			if err != nil {
				t.Fatal(err)
			}
			session, err := backend.Execute(t.Context(), "exact admitted knowledge", opts)
			if tc.deny {
				if KnowledgeFailureReason(err) != tc.reason {
					t.Fatalf("denial=%v", err)
				}
				if _, e := os.Stat(record); !os.IsNotExist(e) {
					t.Fatal("denied process started")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			result := knowledgeResult(t, session)
			if result.KnowledgeFailure != tc.reason {
				t.Fatalf("reason=%q want=%q result=%+v", result.KnowledgeFailure, tc.reason, result)
			}
			if observations != tc.observations {
				t.Fatalf("observations=%d want%d", observations, tc.observations)
			}
			raw, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(raw), "started\n") != 1 || strings.Count(string(raw), `"method":"turn/start"`) != tc.turns {
				t.Fatalf("unexpected process/turn starts: %s", raw)
			}
			if tc.turns > 0 && !strings.Contains(string(raw), `"text":"exact admitted knowledge"`) {
				t.Fatal("admitted prompt was changed")
			}
			if tc.reason == "" && (result.Status != "completed" || result.SessionID != codexKnowledgeThread) {
				t.Fatalf("bad success: %+v", result)
			}
			if strings.Contains(logs.String(), "private-sentinel") || strings.Contains(result.Error, "private-sentinel") {
				t.Fatal("private error leaked")
			}
			if tc.name == "private_error" && !strings.Contains(result.KnowledgeOutput, "private-sentinel") {
				t.Fatal("private diagnostic lost")
			}
		})
	}
}

func TestKnowledgeCodexEphemeralThread(t *testing.T) {
	for _, mode := range []string{"ephemeral", "missing_ephemeral", "persisted_host", "host_path"} {
		t.Run(mode, func(t *testing.T) {
			cfg, opts, record, _ := codexKnowledgeFixture(t, mode)
			opts.Knowledge = nil
			opts.CodexEphemeral = true
			backend, _ := New("codex", cfg)
			session, err := backend.Execute(t.Context(), "private host input", opts)
			if err != nil {
				t.Fatal(err)
			}
			result := knowledgeResult(t, session)
			raw, _ := os.ReadFile(record)
			if !strings.Contains(string(raw), `"ephemeral":true`) {
				t.Fatal("missing nonpersistent RPC policy")
			}
			if mode == "ephemeral" {
				if result.Status != "completed" {
					t.Fatalf("result=%+v", result)
				}
			} else if result.KnowledgeFailure != KnowledgeContextChanged || strings.Contains(string(raw), `"method":"turn/start"`) {
				t.Fatalf("unsupported persistence admitted: %+v", result)
			}
		})
	}
}

func TestKnowledgeCodexFingerprint(t *testing.T) {
	cfg, opts, _, _ := codexKnowledgeFixture(t, "fingerprint")
	opts.McpConfig = json.RawMessage(`{}`)
	before, err := KnowledgeInvocationFingerprint("codex", cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureCodexMcpConfig(filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"), opts.McpConfig, cfg.Logger); err != nil {
		t.Fatal(err)
	}
	after, err := KnowledgeInvocationFingerprint("codex", cfg, opts)
	if err != nil || before != after {
		t.Fatal("materialization changed effective invocation")
	}
	opts.ServiceTier = "priority"
	changed, _ := KnowledgeInvocationFingerprint("codex", cfg, opts)
	if changed == after {
		t.Fatal("tier omitted from invocation")
	}
	opts.ExtraArgs = []string{"--profile", "other"}
	if _, err := KnowledgeInvocationFingerprint("codex", cfg, opts); err == nil {
		t.Fatal("uncontrolled profile admitted")
	}
}

func TestKnowledgeCodexRejectsChangedInputsBeforeLaunch(t *testing.T) {
	for _, change := range []string{"model", "tier", "config", "instructions", "arguments", "continuity", "ephemeral"} {
		t.Run(change, func(t *testing.T) {
			cfg, opts, record, _ := codexKnowledgeFixture(t, "unused")
			opts.Lifecycle = NewOrdinaryLaunchLifecycle((&lifecycleRecorder{}).observer())
			digest, err := KnowledgeInvocationFingerprint("codex", cfg, opts)
			if err != nil {
				t.Fatal(err)
			}
			opts.Knowledge.InvocationFingerprint = digest
			switch change {
			case "model":
				opts.Model = "different"
			case "tier":
				opts.ServiceTier = "priority"
			case "config":
				if err := os.WriteFile(filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"), []byte("model_provider = 'different'\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "instructions":
				if err := os.WriteFile(filepath.Join(opts.Cwd, "AGENTS.md"), []byte("different source"), 0600); err != nil {
					t.Fatal(err)
				}
			case "arguments":
				opts.CustomArgs = []string{"--profile", "other"}
			case "continuity":
				opts.ResumeExpected = true
				opts.ResumeContinuityNotice = "unprepared notice"
			case "ephemeral":
				opts.CodexEphemeral = true
			}
			backend, err := New("codex", cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := backend.Execute(t.Context(), "frozen prompt", opts); KnowledgeFailureReason(err) == "" {
				t.Fatalf("changed %s admitted: %v", change, err)
			}
			if _, err := os.Stat(record); !os.IsNotExist(err) {
				t.Fatal("changed invocation started a process")
			}
		})
	}
}

func TestKnowledgeCodexTerminationRefusalRetainsPrivateOutput(t *testing.T) {
	cfg, opts, record, _ := codexKnowledgeFixture(t, "fresh")
	recorder := &lifecycleRecorder{}
	opts.Lifecycle = NewOrdinaryLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
		callbacks, err := recorder.observer().BeforeLaunch(ctx, info)
		callbacks.NativeSessionObserved = func(NativeSessionObservation) error { return nil }
		terminated := callbacks.Terminated
		callbacks.Terminated = func(termination LaunchTermination) error {
			if err := terminated(termination); err != nil {
				return err
			}
			return &KnowledgeLaunchError{Reason: KnowledgeContextChanged}
		}
		return callbacks, err
	}))
	digest, err := KnowledgeInvocationFingerprint("codex", cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Knowledge.InvocationFingerprint = digest
	backend, err := New("codex", cfg)
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.Execute(t.Context(), "frozen prompt", opts)
	if err != nil {
		t.Fatal(err)
	}
	result := knowledgeResult(t, session)
	if result.Status != "failed" || result.KnowledgeFailure != KnowledgeContextChanged || result.Output != "" || !strings.Contains(result.KnowledgeOutput, "private-answer") {
		t.Fatalf("private evidence lost: %+v", result)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "started\n") != 1 {
		t.Fatal("termination refusal replayed input")
	}
}

func TestKnowledgeCodexSessionSelectors(t *testing.T) {
	for _, selector := range []string{"--config", "--config=profile='other'", "-c", "-cprofile='other'", "-vc", "--profile", "--profile=other", "-pother", "--cd=/other", "-C/other", "--ignore-user-config", "--worktree", "--", "resume", "fork", "exec", "app-server", "'--profile=other'"} {
		if !KnowledgeSessionSelectorPresent("codex", []string{selector}) {
			t.Errorf("selector %q could bypass native history", selector)
		}
	}
	for _, args := range [][]string{nil, {"--verbose"}, {"--model", "test-model"}, {"--color=never"}} {
		if KnowledgeSessionSelectorPresent("codex", args) {
			t.Errorf("nonselector classified as selector: %q", args)
		}
	}
}

func TestKnowledgeCodexConfigurationInstructionBinding(t *testing.T) {
	cfg, _, _, _ := codexKnowledgeFixture(t, "unused")
	home := cfg.Env["CODEX_HOME"]
	instructions := filepath.Join(home, "custom-instructions.md")
	if err := os.WriteFile(instructions, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("model_instructions_file = 'custom-instructions.md'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := KnowledgeCodexConfigFingerprint(cfg.Env)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(instructions, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := KnowledgeCodexConfigFingerprint(cfg.Env)
	if err != nil || before == after {
		t.Fatal("referenced instruction bytes were not bound")
	}
	if err := os.Remove(instructions); err != nil {
		t.Fatal(err)
	}
	if _, err := KnowledgeCodexConfigFingerprint(cfg.Env); err == nil {
		t.Fatal("unreadable instructions accepted")
	}
}

func TestKnowledgeCodexConfiguredCredentialEnvironmentBinding(t *testing.T) {
	cfg, _, _, _ := codexKnowledgeFixture(t, "unused")
	config := "model_provider = 'custom'\n[model_providers.custom]\nenv_key = 'TEST_PROVIDER_CREDENTIAL'\n[model_providers.custom.env_http_headers]\nAuthorization = 'TEST_PROVIDER_HEADER'\n"
	if err := os.WriteFile(filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_PROVIDER_CREDENTIAL", "inherited-first")
	t.Setenv("TEST_PROVIDER_HEADER", "header-first")
	before, err := KnowledgeCodexConfigFingerprint(cfg.Env)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_PROVIDER_CREDENTIAL", "inherited-second")
	afterKey, err := KnowledgeCodexConfigFingerprint(cfg.Env)
	if err != nil || before == afterKey {
		t.Fatal("configured credential env_key omitted")
	}
	cfg.Env["TEST_PROVIDER_HEADER"] = "explicit-header"
	afterHeader, err := KnowledgeCodexConfigFingerprint(cfg.Env)
	if err != nil || afterKey == afterHeader {
		t.Fatal("configured env_http_headers omitted")
	}
	t.Setenv("TEST_PROVIDER_HEADER", "ignored-inherited-header")
	shadowed, err := KnowledgeCodexConfigFingerprint(cfg.Env)
	if err != nil || shadowed != afterHeader {
		t.Fatal("explicit environment did not override inherited credential")
	}
	if strings.Contains(shadowed, "explicit") {
		t.Fatal("credential bytes leaked from hash helper")
	}
}

func runCodexKnowledgeBindingFake(encoder *json.Encoder, id any, mode string) {
	emit := func(method string, params map[string]any) {
		_ = encoder.Encode(map[string]any{"method": method, "params": params})
	}
	events := func(turn, answer, status string) {
		emit("turn/started", map[string]any{"threadId": codexKnowledgeThread, "turn": map[string]any{"id": turn}})
		emit("item/completed", map[string]any{"threadId": codexKnowledgeThread, "turnId": turn, "item": map[string]any{"type": "agentMessage", "id": "answer", "text": answer, "phase": "final_answer"}})
		emit("turn/completed", map[string]any{"threadId": codexKnowledgeThread, "turn": map[string]any{"id": turn, "status": status}})
	}
	response := func(turn any) {
		_ = encoder.Encode(map[string]any{"id": id, "result": map[string]any{"turn": map[string]any{"id": turn}}})
	}
	switch mode {
	case "binding_private_turn_success":
		response("private-sentinel-turn")
		events("private-sentinel-turn", "current-answer", "completed")
	case "binding_private_turn_timeout":
		response("private-sentinel-turn")
		emit("turn/started", map[string]any{"threadId": codexKnowledgeThread, "turn": map[string]any{"id": "private-sentinel-turn"}})
		emit("item/completed", map[string]any{"threadId": codexKnowledgeThread, "turnId": "private-sentinel-turn", "item": map[string]any{"type": "agentMessage", "id": "answer", "text": "current-answer", "phase": "final_answer"}})
	case "binding_correct_early":
		events("turn-new", "current-answer", "completed")
		response("turn-new")
	case "binding_old_early":
		events("turn-old", "old-answer", "completed")
		response("turn-new")
	case "binding_old_late":
		response("turn-new")
		events("turn-old", "old-answer", "completed")
	case "binding_old_then_current":
		events("turn-old", "old-answer", "completed")
		response("turn-new")
		events("turn-new", "current-answer", "completed")
	case "binding_missing_id":
		events("turn-new", "old-answer", "completed")
		response(nil)
	case "binding_invalid_id":
		events("turn-new", "old-answer", "completed")
		response(map[string]any{"not": "an-id"})
	case "binding_rpc_error":
		events("turn-new", "old-answer", "completed")
		_ = encoder.Encode(map[string]any{"id": id, "error": map[string]any{"code": -1, "message": "private-sentinel"}})
	case "binding_wrong_rpc_id":
		events("turn-new", "old-answer", "completed")
		_ = encoder.Encode(map[string]any{"id": 999, "result": map[string]any{"turn": map[string]any{"id": "turn-new"}}})
	case "binding_missing_rpc_id":
		events("turn-new", "old-answer", "completed")
		_ = encoder.Encode(map[string]any{"result": map[string]any{"turn": map[string]any{"id": "turn-new"}}})
	case "binding_pending_overflow":
		for i := 0; i <= codexKnowledgePendingEvents; i++ {
			emit("item/completed", map[string]any{"threadId": codexKnowledgeThread, "turnId": "turn-new", "item": map[string]any{"type": "agentMessage", "id": "answer", "text": "old-answer", "phase": "final_answer"}})
		}
		response("turn-new")
		events("turn-new", "current-answer", "completed")
	case "binding_unknown_status":
		response("turn-new")
		events("turn-new", "current-answer", "private-sentinel")
	case "binding_provenance_noise":
		emit("turn/started", map[string]any{"turn": map[string]any{"id": "poison-turn"}})
		emit("turn/started", map[string]any{"threadId": "other-thread", "turn": map[string]any{"id": "poison-turn"}})
		response("turn-new")
		emit("error", map[string]any{"threadId": codexKnowledgeThread, "turnId": "turn-old", "error": map[string]any{"message": "private-sentinel"}, "willRetry": false})
		events("turn-old", "old-answer", "failed")
		events("turn-new", "current-answer", "completed")
	}
}

func TestKnowledgeCodexTurnResponseBinding(t *testing.T) {
	for _, tc := range []struct {
		mode    string
		success bool
	}{
		{"correct_early", true}, {"old_early", false}, {"old_late", false}, {"old_then_current", true},
		{"missing_id", false}, {"invalid_id", false}, {"rpc_error", false}, {"unknown_status", false}, {"provenance_noise", true},
		{"wrong_rpc_id", false}, {"missing_rpc_id", false}, {"pending_overflow", false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			cfg, opts, _, logs := codexKnowledgeFixture(t, "binding_"+tc.mode)
			opts.Timeout = 750 * time.Millisecond
			recorder := &lifecycleRecorder{}
			opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(recorder, func(NativeSessionObservation) error { return nil }))
			digest, err := KnowledgeInvocationFingerprint("codex", cfg, opts)
			if err != nil {
				t.Fatal(err)
			}
			opts.Knowledge.InvocationFingerprint = digest
			backend, err := New("codex", cfg)
			if err != nil {
				t.Fatal(err)
			}
			session, err := backend.Execute(t.Context(), "frozen knowledge input", opts)
			if err != nil {
				t.Fatal(err)
			}
			result := knowledgeResult(t, session)
			if tc.success {
				if result.Status != "completed" || result.Output != "current-answer" {
					t.Fatalf("current turn lost: %+v", result)
				}
			} else if result.Status == "completed" || result.Output != "" {
				t.Fatalf("uncorrelated/invalid turn accepted: %+v", result)
			}
			if strings.Contains(result.Output, "old-answer") || strings.Contains(result.KnowledgeOutput, "old-answer") {
				t.Fatal("unrelated turn contaminated evidence")
			}
			if strings.Contains(logs.String(), "private-sentinel") || strings.Contains(result.Error, "private-sentinel") {
				t.Fatal("private malformed status/error leaked into generic output")
			}
			if tc.mode == "unknown_status" && !strings.Contains(result.KnowledgeOutput, "private-sentinel") {
				t.Fatal("unsupported status diagnostic lost from private evidence")
			}
		})
	}
}

func TestKnowledgeCodexPendingNotificationByteBound(t *testing.T) {
	c := &codexClient{strictKnowledge: true}
	c.knowledgeTurn.arm(codexKnowledgeThread)
	params := map[string]any{"threadId": codexKnowledgeThread, "turnId": "turn-new", "item": map[string]any{"type": "agentMessage", "text": strings.Repeat("x", codexKnowledgePendingBytes)}}
	c.handleKnowledgeNotification("item/completed", params)
	if !c.knowledgeTurn.failed || len(c.knowledgeTurn.pending) != 0 {
		t.Fatal("oversized pending input was retained")
	}
	if err := c.bindKnowledgeTurn(json.RawMessage(`{"turn":{"id":"turn-new"}}`), nil); KnowledgeFailureReason(err) != KnowledgeContextChanged {
		t.Fatalf("overflow did not refuse authoritative turn: %v", err)
	}
}

func TestKnowledgeCodexPrivateTurnIdentityLogs(t *testing.T) {
	for _, mode := range []string{"success", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			cfg, opts, _, logs := codexKnowledgeFixture(t, "binding_private_turn_"+mode)
			opts.SemanticInactivityTimeout = 60 * time.Millisecond
			opts.FirstTurnNoProgressTimeout = 200 * time.Millisecond
			recorder := &lifecycleRecorder{}
			opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(recorder, func(NativeSessionObservation) error { return nil }))
			digest, err := KnowledgeInvocationFingerprint("codex", cfg, opts)
			if err != nil {
				t.Fatal(err)
			}
			opts.Knowledge.InvocationFingerprint = digest
			backend, err := New("codex", cfg)
			if err != nil {
				t.Fatal(err)
			}
			session, err := backend.Execute(t.Context(), "frozen knowledge input", opts)
			if err != nil {
				t.Fatal(err)
			}
			result := knowledgeResult(t, session)
			if mode == "success" {
				if result.Status != "completed" || result.Output != "current-answer" {
					t.Fatalf("completed input failed: %+v", result)
				}
			} else if result.Status != "failed" || result.KnowledgeFailure != KnowledgeUnavailable {
				t.Fatalf("timeout did not retain refusal: %+v", result)
			}
			if !strings.Contains(logs.String(), "first_item_wait") {
				t.Fatal("first item telemetry was not exercised")
			}
			if mode == "timeout" && !strings.Contains(logs.String(), CodexSemanticInactivityMarker) {
				t.Fatal("inactivity telemetry was not exercised")
			}
			if strings.Contains(logs.String(), "private-sentinel-turn") || strings.Contains(result.Error, "private-sentinel-turn") {
				t.Fatal("private native turn ID leaked into generic logs/error")
			}
			if !strings.Contains(logs.String(), codexDiagnosticID(true, "private-sentinel-turn")) {
				t.Fatal("diagnostic identity hash missing")
			}
		})
	}
}
