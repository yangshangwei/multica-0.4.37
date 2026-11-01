package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestOrdinaryKnowledgeLifecyclePreservesTaskCredentials(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewOrdinaryLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "environment")
	cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = append(lifecycleTestEnv("environment", file.Name()),
		"MULTICA_TOKEN=mat_task_only", "MULTICA_TASK_ID=task_identity", "MULTICA_CONTEXT=task_context",
		"ANTHROPIC_API_KEY=provider_fixture", "ORDINARY_PREF=unchanged")
	want := cmd.Environ()
	if err := lifecycle.Run(cmd); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(file.Name() + ".env")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(string(body), "\n")
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Error("ordinary launch changed the already assembled environment")
	}
}

// Dispatch before TestMain parses the real provider's argv. The child is this
// test-owned Go executable; no installed CLI, account or shell is involved.
func init() {
	if spec := os.Getenv("AGENT_KNOWLEDGE_LEAF_FAKE"); spec != "" {
		runKnowledgeLeafFake(spec)
		os.Exit(0)
	}
}

type knowledgeFakeSpec struct {
	Record string
	Events []json.RawMessage
	Flood  int
	Block  bool
}

func runKnowledgeLeafFake(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		os.Exit(90)
	}
	var spec knowledgeFakeSpec
	if json.Unmarshal(raw, &spec) != nil {
		os.Exit(91)
	}
	f, err := os.OpenFile(spec.Record+".starts", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		os.Exit(92)
	}
	_, _ = fmt.Fprintln(f, os.Getpid())
	_ = f.Close()
	argv, _ := json.Marshal(os.Args[1:])
	_ = os.WriteFile(spec.Record+".args", argv, 0o600)
	_ = os.WriteFile(spec.Record+".env", []byte(strings.Join(os.Environ(), "\n")), 0o600)
	line, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		os.Exit(93)
	}
	_ = os.WriteFile(spec.Record+".stdin", line, 0o600)
	for range spec.Flood {
		fmt.Println(`{"type":"log","log":{"level":"info","message":"fill public event channel"}}`)
	}
	for _, event := range spec.Events {
		fmt.Println(string(event))
	}
	if spec.Block {
		for {
			time.Sleep(time.Second)
		}
	}
}

func knowledgeFixture(t *testing.T, events ...string) (Config, ExecOptions, knowledgeFakeSpec) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	spec := knowledgeFakeSpec{Record: filepath.Join(dir, "record")}
	for _, event := range events {
		spec.Events = append(spec.Events, json.RawMessage(event))
	}
	cfg := Config{ExecutablePath: exe, CLIVersion: "fake-claude", BuiltinRuntime: true,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Env: map[string]string{
			"AGENT_KNOWLEDGE_LEAF_FAKE": filepath.Join(dir, "spec.json"),
			"MULTICA_TOKEN":             "mat_task_only", "MULTICA_TASK_ID": "task-1", "IS_SANDBOX": "1",
		}}
	writeKnowledgeSpec(t, cfg, spec)
	opts := ExecOptions{Cwd: dir, Model: "fake-model", Timeout: 5 * time.Second,
		Knowledge: &KnowledgeLaunchIntent{ConfigurationID: "runtime-config-1",
			Session: KnowledgeSessionIntent{Kind: KnowledgeSessionFresh, Generation: "scope-generation-1"}},
	}
	setKnowledgeFingerprint(t, cfg, &opts)
	return cfg, opts, spec
}

func writeKnowledgeSpec(t *testing.T, cfg Config, spec knowledgeFakeSpec) {
	t.Helper()
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Env["AGENT_KNOWLEDGE_LEAF_FAKE"], raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func setKnowledgeFingerprint(t *testing.T, cfg Config, opts *ExecOptions) {
	t.Helper()
	digest, err := KnowledgeInvocationFingerprint("claude", cfg, *opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Knowledge.InvocationFingerprint = digest
}

func knowledgeObserver(recorder *lifecycleRecorder, observed func(NativeSessionObservation) error) LaunchObserver {
	return lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
		callbacks, err := recorder.observer().BeforeLaunch(ctx, info)
		callbacks.NativeSessionObserved = observed
		return callbacks, err
	})
}

func executeKnowledgeFixture(t *testing.T, cfg Config, opts ExecOptions) (*Session, error) {
	t.Helper()
	backend, err := New("claude", cfg)
	if err != nil {
		t.Fatal(err)
	}
	return backend.Execute(t.Context(), "exact application input", opts)
}

func knowledgeResult(t *testing.T, session *Session) Result {
	t.Helper()
	// Deliberately do not drain Messages: native observation must survive a
	// full/drop-prone public channel, just as process observation does.
	select {
	case result := <-session.Result:
		return result
	case <-time.After(8 * time.Second):
		t.Fatal("fake command did not finish")
		return Result{}
	}
}

func knowledgeStartCount(t *testing.T, spec knowledgeFakeSpec) int {
	t.Helper()
	raw, err := os.ReadFile(spec.Record + ".starts")
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(raw), "\n")
}

func TestKnowledgeClaudeNativeObservationIsBoundToPhysicalAttempt(t *testing.T) {
	cfg, opts, spec := knowledgeFixture(t,
		`{"type":"system","session_id":"native-one"}`,
		`{"type":"system","session_id":"native-one"}`,
		`{"type":"result","session_id":"native-one","result":"done"}`)
	spec.Flood = 400
	writeKnowledgeSpec(t, cfg, spec)
	var infos []LaunchInfo
	var observed []NativeSessionObservation
	recorder := &lifecycleRecorder{}
	delegate := knowledgeObserver(recorder, func(observation NativeSessionObservation) error {
		observed = append(observed, observation)
		return nil
	})
	opts.Lifecycle = NewOrdinaryLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
		infos = append(infos, info)
		// Mutating the caller's intent after command creation cannot relabel
		// the command or the direct callback to its native observation.
		opts.Knowledge.Session.Generation = "stale-outer-options"
		return delegate.BeforeLaunch(ctx, info)
	}))
	t.Cleanup(opts.Lifecycle.Stop)
	for attempt := 1; attempt <= 2; attempt++ {
		opts.Knowledge.Session.Generation = fmt.Sprintf("generation-%d", attempt)
		session, err := executeKnowledgeFixture(t, cfg, opts)
		if err != nil {
			t.Fatal(err)
		}
		result := knowledgeResult(t, session)
		if result.Status != "completed" || result.SessionID != "native-one" {
			t.Fatalf("native result lost: %+v", result)
		}
	}
	if len(infos) != 2 || len(observed) != 2 || len(recorder.started) != 2 || len(recorder.terminated) != 2 {
		t.Fatalf("lost attempt or duplicate observation: admissions=%d native=%d starts=%d ends=%d", len(infos), len(observed), len(recorder.started), len(recorder.terminated))
	}
	for i, observation := range observed {
		if observation.Attempt != infos[i].Attempt || observation.Attempt.Sequence != int64(i+1) || observation.Attempt.Nonce == "" ||
			observation.Knowledge != infos[i].Knowledge || observation.Knowledge.Session.Generation != fmt.Sprintf("generation-%d", i+1) ||
			observation.Source != "system" || !observation.MatchesIntent {
			t.Fatalf("observation was not bound to the physical command: %+v", observation)
		}
	}
	if observed[0].Attempt.Nonce == observed[1].Attempt.Nonce {
		t.Fatal("physical commands shared an admission nonce")
	}
	stdin, err := os.ReadFile(spec.Record + ".stdin")
	if err != nil {
		t.Fatal(err)
	}
	want, err := buildClaudeInput("exact application input")
	if err != nil || string(stdin) != string(want) || knowledgeStartCount(t, spec) != 2 {
		t.Fatal("actual fake input/start count differs from observed launches")
	}
}

func TestKnowledgeClaudeResultOnlyObservationPrecedesPointer(t *testing.T) {
	cfg, opts, _ := knowledgeFixture(t, `{"type":"result","session_id":"native-result","result":"done"}`)
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	observed := make(chan NativeSessionObservation, 1)
	recorder := &lifecycleRecorder{}
	opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(recorder, func(observation NativeSessionObservation) error {
		observed <- observation
		close(entered)
		<-release
		return nil
	}))
	t.Cleanup(opts.Lifecycle.Stop)
	session, err := executeKnowledgeFixture(t, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("raw result ID was not directly observed")
	}
	select {
	case result := <-session.Result:
		t.Fatalf("pointer escaped before binding completed: %+v", result)
	default:
	}
	if observation := <-observed; observation.Source != "result" || observation.NativeSessionID != "native-result" || !observation.MatchesIntent {
		t.Fatalf("wrong result observation: %+v", observation)
	}
	release <- struct{}{}
	if result := knowledgeResult(t, session); result.SessionID != "native-result" || result.Status != "completed" {
		t.Fatalf("accepted native observation was not published after binding: %+v", result)
	}
}

func TestKnowledgeUnknownFreshSessionCannotBePinnedWithoutObservation(t *testing.T) {
	for _, resume := range []string{"", "requested-but-never-observed"} {
		t.Run(resume, func(t *testing.T) {
			cfg, opts, _ := knowledgeFixture(t, `{"type":"result","result":"done"}`)
			if resume != "" {
				opts.ResumeSessionID = resume
				opts.Knowledge.Session.Kind = KnowledgeSessionResume
				opts.Knowledge.Session.NativeSessionID = resume
			}
			recorder := &lifecycleRecorder{}
			opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(recorder, func(NativeSessionObservation) error {
				t.Error("fabricated a native observation")
				return nil
			}))
			session, err := executeKnowledgeFixture(t, cfg, opts)
			if err != nil {
				t.Fatal(err)
			}
			result := knowledgeResult(t, session)
			if result.SessionID != "" || !result.ProcessExited || len(recorder.started) != 1 || len(recorder.terminated) != 1 {
				t.Fatalf("process facts fabricated a reusable native session: %+v", result)
			}
		})
	}
}

func TestKnowledgeChangedNativeSessionDoesNotRelabelHistory(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(fmt.Sprint(resume), func(t *testing.T) {
			cfg, opts, spec := knowledgeFixture(t,
				`{"type":"system","session_id":"native-one"}`,
				`{"type":"result","session_id":"native-two","result":"work already done","modelUsage":{"fake-model":{"inputTokens":12,"outputTokens":7}}}`)
			if resume {
				opts.ResumeSessionID = "requested-history"
				opts.Knowledge.Session = KnowledgeSessionIntent{Kind: KnowledgeSessionResume, Generation: "original-generation", NativeSessionID: "requested-history"}
			}
			var observations []NativeSessionObservation
			recorder := &lifecycleRecorder{}
			opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(recorder, func(observation NativeSessionObservation) error {
				observations = append(observations, observation)
				return nil // Even a permissive observer cannot accept a mismatch.
			}))
			session, err := executeKnowledgeFixture(t, cfg, opts)
			if err != nil {
				t.Fatal(err)
			}
			result := knowledgeResult(t, session)
			if result.Status != "failed" || result.KnowledgeFailure != KnowledgeContextChanged || result.Error != KnowledgeContextChanged || result.ResumeRejected || !result.ProcessExited {
				t.Fatalf("changed native ID was not a terminal refusal: %+v", result)
			}
			if len(observations) == 0 || observations[len(observations)-1].MatchesIntent || result.SessionID == "native-two" {
				t.Fatal("unapproved session replaced the admitted history")
			}
			if !resume && (len(observations) != 2 || observations[1].PreviousNativeSessionID != "native-one" || result.Output != "" || result.KnowledgeOutput != "work already done" || result.Usage["fake-model"].InputTokens != 12) {
				t.Fatalf("refusal lost prior work or the changed-ID fact: %+v", result)
			}
			if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeContextChanged || knowledgeStartCount(t, spec) != 1 {
				t.Fatalf("changed ID allowed an automatic replay: %v", err)
			}
			if len(recorder.terminated) != 1 || !recorder.terminated[0].WaitObserved {
				t.Fatal("refusal lost actual process termination")
			}
		})
	}
}

func TestKnowledgeNativeObservationFailureStopsProcessAndSeals(t *testing.T) {
	cfg, opts, spec := knowledgeFixture(t, `{"type":"system","session_id":"private-native-id"}`)
	spec.Block = true
	writeKnowledgeSpec(t, cfg, spec)
	recorder := &lifecycleRecorder{}
	failure := errors.New("private callback cause must not enter result text")
	opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(recorder, func(NativeSessionObservation) error { return failure }))
	t.Cleanup(opts.Lifecycle.Stop)
	session, err := executeKnowledgeFixture(t, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	result := knowledgeResult(t, session)
	if result.KnowledgeFailure != KnowledgeUnavailable || result.Error != KnowledgeUnavailable || result.SessionID != "" || !result.ProcessExited || !errors.Is(opts.Lifecycle.Err(), failure) {
		t.Fatalf("uncertain observation was not sealed without a pointer: %+v", result)
	}
	if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeUnavailable || knowledgeStartCount(t, spec) != 1 {
		t.Fatalf("callback failure permitted a replay: %v", err)
	}
}

func TestOrdinaryKnowledgeObserverCannotSubstituteEnvironment(t *testing.T) {
	for _, key := range []string{"MULTICA_TOKEN", "MULTICA_DAEMON_TOKEN", "ANTHROPIC_API_KEY", "PATH", "UNRELATED_NEW_ENTRY"} {
		t.Run(key, func(t *testing.T) {
			cfg, opts, spec := knowledgeFixture(t)
			recorder := &lifecycleRecorder{}
			delegate := knowledgeObserver(recorder, func(NativeSessionObservation) error { return nil })
			opts.Lifecycle = NewOrdinaryLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
				callbacks, err := delegate.BeforeLaunch(ctx, info)
				callbacks.Environment = map[string]string{key: "broader-identity"}
				return callbacks, err
			}))
			if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeInvocationUnsupported {
				t.Fatalf("environment substitution was accepted: %v", err)
			}
			if knowledgeStartCount(t, spec) != 0 || len(recorder.terminated) != 1 || recorder.terminated[0].Kind != LaunchNotStarted {
				t.Fatal("environment refusal started work or lost its unused admission")
			}
		})
	}
}

func TestCollaborationLifecycleStillRejectsOrdinaryCredentials(t *testing.T) {
	cfg, opts, spec := knowledgeFixture(t, `{"type":"result","result":"done"}`)
	opts.Knowledge = nil
	cfg.Env["MULTICA_COLLABORATION_TOKEN"] = "mcb_collaboration_only"
	recorder := &lifecycleRecorder{}
	opts.Lifecycle = NewLaunchLifecycle(recorder.observer())
	session, err := executeKnowledgeFixture(t, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	_ = knowledgeResult(t, session)
	env, err := os.ReadFile(spec.Record + ".env")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(env), "MULTICA_TOKEN=") || strings.Contains(string(env), "MULTICA_TASK_ID=") || !strings.Contains(string(env), "MULTICA_COLLABORATION_TOKEN=mcb_collaboration_only") {
		t.Fatal("ordinary policy changed collaboration credential filtering")
	}
}

func TestKnowledgeEffectiveArgsCannotOverrideSessionSelection(t *testing.T) {
	for _, arg := range []string{"--resume", "--resume=other", "-r", "-rother", "--continue", "-c", "-cr", "--session", "--session-id=other", "--fork-session", "--attach", "--server", "--url", "--cwd", "--cd", "-C/tmp", "--directory", "--workdir", "--config-dir", "remote-control", "--", "--unknown-vendor-flag"} {
		for _, source := range []string{"defaults", "custom"} {
			t.Run(source+arg, func(t *testing.T) {
				cfg, opts, spec := knowledgeFixture(t)
				if source == "defaults" {
					opts.ExtraArgs = []string{arg}
				} else {
					opts.CustomArgs = []string{arg}
				}
				if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeInvocationUnsupported || knowledgeStartCount(t, spec) != 0 {
					t.Fatalf("selector override reached native execution: %v", err)
				}
			})
		}
	}
}

func TestKnowledgeInvocationFingerprintRejectsDrift(t *testing.T) {
	for _, change := range []string{"model", "cwd", "safe-args", "executable", "mcp", "thinking", "settings"} {
		t.Run(change, func(t *testing.T) {
			cfg, opts, spec := knowledgeFixture(t)
			switch change {
			case "model":
				opts.Model = "different-model"
			case "cwd":
				opts.Cwd = t.TempDir()
			case "safe-args":
				opts.CustomArgs = []string{"--verbose"}
			case "executable":
				cfg.ExecutablePath = filepath.Join(t.TempDir(), "missing-fake")
			case "mcp":
				opts.McpConfig = json.RawMessage(`{"mcpServers":{}}`)
			case "thinking":
				opts.ThinkingLevel = "high"
			case "settings":
				opts.ClaudeSettingsPath = filepath.Join(t.TempDir(), "settings")
			}
			if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeContextChanged || knowledgeStartCount(t, spec) != 0 {
				t.Fatalf("configuration drift reached native execution: %v", err)
			}
		})
	}
}

func TestKnowledgeCustomProtocolFamilyIsNotBuiltinSupport(t *testing.T) {
	for _, mode := range []string{"custom", "prefix", "filtered-prefix", "remote", "codex", "opencode"} {
		t.Run(mode, func(t *testing.T) {
			cfg, opts, spec := knowledgeFixture(t)
			provider := "claude"
			switch mode {
			case "custom":
				cfg.BuiltinRuntime = false
			case "prefix":
				cfg.LaunchPrefix = []string{"wrapper", "start"}
			case "filtered-prefix":
				cfg.LaunchPrefix = []string{"--output-format", "text"}
			case "remote":
				opts.OpenclawMode = "gateway"
			default:
				provider = mode
			}
			opts.Lifecycle = NewOrdinaryLaunchLifecycle((&lifecycleRecorder{}).observer())
			backend, err := New(provider, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := backend.Execute(t.Context(), "input", opts); KnowledgeFailureReason(err) != KnowledgeInvocationUnsupported || knowledgeStartCount(t, spec) != 0 {
				t.Fatalf("unsupported invocation started with knowledge: %v", err)
			}
		})
	}
}

func TestKnowledgeAmbiguousLegacyContinueRefused(t *testing.T) {
	cfg, opts, spec := knowledgeFixture(t, `{"type":"result","session_id":"legacy","result":"ordinary done"}`)
	opts.CustomArgs = []string{"--continue", "--verbose"}
	if err := ValidateKnowledgeSessionArgs("claude", opts.CustomArgs); KnowledgeFailureReason(err) != KnowledgeInvocationUnsupported {
		t.Fatalf("ambiguous history selection was declared controlled: %v", err)
	}
	// Proven legacy/no-managed-history callers still retain their old behavior.
	// The leaf must not silently strip their explicit session/user intent.
	opts.Knowledge = nil
	session, err := executeKnowledgeFixture(t, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result := knowledgeResult(t, session); result.Status != "completed" || result.SessionID != "legacy" {
		t.Fatalf("ordinary legacy execution changed: %+v", result)
	}
	raw, err := os.ReadFile(spec.Record + ".args")
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if json.Unmarshal(raw, &args) != nil || !slices.Contains(args, "--continue") || !slices.Contains(args, "--verbose") {
		t.Fatal("ordinary arguments were silently stripped")
	}
}

func TestKnowledgeLeafLaunchRefusalDoesNotReplayInput(t *testing.T) {
	for _, refusalAt := range []int64{1, 2} {
		t.Run(fmt.Sprint(refusalAt), func(t *testing.T) {
			cfg, opts, spec := knowledgeFixture(t, `{"type":"result","session_id":"native","result":"prior output","modelUsage":{"fake-model":{"inputTokens":9,"outputTokens":3}}}`)
			recorder := &lifecycleRecorder{}
			delegate := knowledgeObserver(recorder, func(NativeSessionObservation) error { return nil })
			permits := 0
			opts.Lifecycle = NewOrdinaryLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
				permits++
				if info.Attempt.Sequence == refusalAt {
					return LaunchCallbacks{}, &KnowledgeLaunchError{Reason: KnowledgeAuthorizationDenied}
				}
				return delegate.BeforeLaunch(ctx, info)
			}))
			if refusalAt == 2 {
				session, err := executeKnowledgeFixture(t, cfg, opts)
				if err != nil {
					t.Fatal(err)
				}
				result := knowledgeResult(t, session)
				if result.Output != "prior output" || result.Usage["fake-model"].InputTokens != 9 || !result.ProcessExited {
					t.Fatalf("initial work facts lost: %+v", result)
				}
			}
			for range 2 {
				if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeAuthorizationDenied {
					t.Fatalf("knowledge refusal lost its type: %v", err)
				}
			}
			if permits != int(refusalAt) || knowledgeStartCount(t, spec) != int(refusalAt-1) || len(recorder.terminated) != int(refusalAt-1) {
				t.Fatal("refusal replayed input or discarded an earlier termination")
			}
		})
	}
}

func TestKnowledgeClaudeExactResumeAndSafeArgsReachCommand(t *testing.T) {
	cfg, opts, spec := knowledgeFixture(t,
		`{"type":"system","session_id":"authorized-native"}`,
		`{"type":"result","session_id":"authorized-native","result":"done"}`)
	opts.ResumeSessionID = "authorized-native"
	opts.Knowledge.Session = KnowledgeSessionIntent{Kind: KnowledgeSessionResume, Generation: "scope-existing", NativeSessionID: "authorized-native"}
	opts.ExtraArgs = []string{"--max-turns", "2"}
	opts.CustomArgs = []string{"--verbose", "--max-budget-usd=1"}
	setKnowledgeFingerprint(t, cfg, &opts)
	observations := 0
	opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(&lifecycleRecorder{}, func(observation NativeSessionObservation) error {
		observations++
		if observation.Knowledge.Session != opts.Knowledge.Session || !observation.MatchesIntent {
			t.Error("exact resume did not retain its admitted intent")
		}
		return nil
	}))
	session, err := executeKnowledgeFixture(t, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result := knowledgeResult(t, session); result.Status != "completed" || result.SessionID != "authorized-native" || observations != 1 {
		t.Fatalf("exact resume failed: %+v, observations=%d", result, observations)
	}
	raw, err := os.ReadFile(spec.Record + ".args")
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if json.Unmarshal(raw, &args) != nil || !slices.Equal(args, buildClaudeArgs(opts, cfg.Logger)) {
		t.Fatal("effective command differs from the fingerprinted options")
	}
	resume := slices.Index(args, "--resume")
	if resume < 0 || args[resume+1] != "authorized-native" || slices.Contains(args, "--session-id") {
		t.Fatal("adapter changed the explicit native selection")
	}
	env, err := os.ReadFile(spec.Record + ".env")
	if err != nil || !strings.Contains(string(env), "MULTICA_TOKEN=mat_task_only") || !strings.Contains(string(env), "MULTICA_TASK_ID=task-1") {
		t.Fatal("managed Claude lost its ordinary task credentials")
	}
}

func TestKnowledgeInvalidSessionIntentCannotLaunch(t *testing.T) {
	for _, invalid := range []string{"fresh-with-native", "fresh-with-resume-option", "resume-mismatch", "resume-missing-native", "generation", "kind", "collaboration-policy", "missing-observer"} {
		t.Run(invalid, func(t *testing.T) {
			cfg, opts, spec := knowledgeFixture(t)
			opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(&lifecycleRecorder{}, func(NativeSessionObservation) error { return nil }))
			switch invalid {
			case "fresh-with-native":
				opts.Knowledge.Session.NativeSessionID = "not-fresh"
			case "fresh-with-resume-option":
				opts.ResumeSessionID = "not-fresh"
			case "resume-mismatch":
				opts.Knowledge.Session.Kind = KnowledgeSessionResume
				opts.Knowledge.Session.NativeSessionID = "admitted"
				opts.ResumeSessionID = "different"
			case "resume-missing-native":
				opts.Knowledge.Session.Kind = KnowledgeSessionResume
			case "generation":
				opts.Knowledge.Session.Generation = ""
			case "kind":
				opts.Knowledge.Session.Kind = "continue"
			case "collaboration-policy":
				opts.Lifecycle = NewLaunchLifecycle((&lifecycleRecorder{}).observer())
			case "missing-observer":
				opts.Lifecycle = nil
			}
			if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeInvocationUnsupported || knowledgeStartCount(t, spec) != 0 {
				t.Fatalf("invalid intent was executed: %v", err)
			}
		})
	}
}

func TestKnowledgeMissingNativeCallbackRetainsUnusedAdmission(t *testing.T) {
	cfg, opts, spec := knowledgeFixture(t)
	recorder := &lifecycleRecorder{}
	opts.Lifecycle = NewOrdinaryLaunchLifecycle(recorder.observer())
	if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeUnavailable {
		t.Fatalf("missing native callback accepted: %v", err)
	}
	if knowledgeStartCount(t, spec) != 0 || len(recorder.terminated) != 1 || recorder.terminated[0].Kind != LaunchNotStarted {
		t.Fatal("unused admission lost its separate process fact")
	}
}

func TestKnowledgeTerminationCallbackFailureKeepsUsageAndRefusal(t *testing.T) {
	cfg, opts, spec := knowledgeFixture(t, `{"type":"result","session_id":"native","result":"completed work","modelUsage":{"fake-model":{"inputTokens":9}}}`)
	recorder := &lifecycleRecorder{}
	delegate := knowledgeObserver(recorder, func(NativeSessionObservation) error { return nil })
	opts.Lifecycle = NewOrdinaryLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
		callbacks, err := delegate.BeforeLaunch(ctx, info)
		terminated := callbacks.Terminated
		callbacks.Terminated = func(end LaunchTermination) error {
			if err := terminated(end); err != nil {
				return err
			}
			return errors.New("receipt not durable")
		}
		return callbacks, err
	}))
	session, err := executeKnowledgeFixture(t, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	result := knowledgeResult(t, session)
	if result.KnowledgeFailure != KnowledgeUnavailable || result.Status != "failed" || result.Output != "" || result.KnowledgeOutput != "completed work" || result.Usage["fake-model"].InputTokens != 9 || !result.ProcessExited {
		t.Fatalf("terminal observation failure lost work or failure class: %+v", result)
	}
	if _, err := executeKnowledgeFixture(t, cfg, opts); KnowledgeFailureReason(err) != KnowledgeUnavailable || knowledgeStartCount(t, spec) != 1 {
		t.Fatalf("terminal receipt failure allowed replay: %v", err)
	}
}

func TestKnowledgeUnsupportedBackendRetainsOrdinaryBaseExecution(t *testing.T) {
	file, exe := lifecycleHelper(t, nil, "opencode")
	backend, err := New("opencode", Config{ExecutablePath: exe, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Env: map[string]string{
		"AGENT_LIFECYCLE_FAKE": "opencode", "AGENT_LIFECYCLE_RECORD": file.Name(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if KnowledgeLifecycleSupported("opencode", "local", true) {
		t.Fatal("unsupported adapter advertised knowledge")
	}
	session, err := backend.Execute(t.Context(), "ordinary base input", ExecOptions{Cwd: t.TempDir(), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if result := knowledgeResult(t, session); result.Status != "completed" || result.Output != "done" || result.KnowledgeFailure != "" {
		t.Fatalf("unsupported knowledge adapter lost base execution: %+v", result)
	}
}

func TestKnowledgeInvalidNativeIDCannotBecomeObservation(t *testing.T) {
	for _, nativeID := range []string{" spaced-id", "control\x00id", strings.Repeat("x", 4097)} {
		raw, err := json.Marshal(map[string]string{"type": "result", "session_id": nativeID, "result": "done"})
		if err != nil {
			t.Fatal(err)
		}
		cfg, opts, _ := knowledgeFixture(t, string(raw))
		opts.Lifecycle = NewOrdinaryLaunchLifecycle(knowledgeObserver(&lifecycleRecorder{}, func(NativeSessionObservation) error {
			t.Error("invalid stream bytes crossed the native-identity boundary")
			return nil
		}))
		session, err := executeKnowledgeFixture(t, cfg, opts)
		if err != nil {
			t.Fatal(err)
		}
		result := knowledgeResult(t, session)
		if result.KnowledgeFailure != KnowledgeContextChanged || result.SessionID != "" || result.Error != KnowledgeContextChanged || !result.ProcessExited {
			t.Fatal("malformed native identity was not refused with process facts retained")
		}
	}
}

func TestKnowledgeNoSessionPersistenceFlagHasNoReenableForm(t *testing.T) {
	if err := ValidateKnowledgeSessionArgs("claude", []string{"--no-session-persistence"}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--no-session-persistence=false"}, {"--no-session-persistence=true"},
		{"--no-session-persistence", "false"}, {"--session-persistence"},
		{"--no-no-session-persistence"}, {"--persist-session"},
	} {
		if ValidateKnowledgeSessionArgs("claude", args) == nil {
			t.Fatalf("unproven persistence override accepted: %v", args)
		}
	}
	if ValidateKnowledgeSessionArgs("codex", []string{"--no-session-persistence"}) == nil {
		t.Fatal("Claude-only persistence control admitted another provider")
	}
}
