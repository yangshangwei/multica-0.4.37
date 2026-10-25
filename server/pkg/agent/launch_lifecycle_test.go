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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type lifecycleObserverFunc func(context.Context, LaunchInfo) (LaunchCallbacks, error)

func (f lifecycleObserverFunc) BeforeLaunch(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
	return f(ctx, info)
}

type lifecycleRecorder struct {
	mu         sync.Mutex
	events     []string
	started    []ProcessObservation
	terminated []LaunchTermination
}

func (r *lifecycleRecorder) observer() LaunchObserver {
	return lifecycleObserverFunc(func(_ context.Context, info LaunchInfo) (LaunchCallbacks, error) {
		r.mu.Lock()
		r.events = append(r.events, "permit:"+info.Kind)
		r.mu.Unlock()
		return LaunchCallbacks{
			Started: func(p ProcessObservation) error {
				r.mu.Lock()
				defer r.mu.Unlock()
				r.events = append(r.events, "started")
				r.started = append(r.started, p)
				return nil
			},
			Terminated: func(end LaunchTermination) error {
				r.mu.Lock()
				defer r.mu.Unlock()
				r.events = append(r.events, "terminated:"+end.Kind)
				r.terminated = append(r.terminated, end)
				return nil
			},
		}, nil
	})
}

func lifecycleHelper(t *testing.T, lifecycle *LaunchLifecycle, mode string) (*os.File, string) {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "processes"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file, path
}

func lifecycleTestEnv(mode, record string) []string {
	return buildEnv(map[string]string{
		"AGENT_LIFECYCLE_FAKE":   mode,
		"AGENT_LIFECYCLE_RECORD": record,
	})
}

func TestHostLifecycleObservesRealWaitAndSerialLaunches(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "exit")
	for range 2 {
		cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
		cmd.Env = lifecycleTestEnv("exit", file.Name())
		if err := lifecycle.Run(cmd); err != nil {
			t.Fatal(err)
		}
	}
	if err := lifecycle.Seal(); err != nil {
		t.Fatal(err)
	}
	want := "permit:preparation,started,terminated:managed_launch_ended,permit:preparation,started,terminated:managed_launch_ended"
	if got := strings.Join(recorder.events, ","); got != want {
		t.Fatalf("launch order = %s, want %s", got, want)
	}
	if len(recorder.started) != 2 || recorder.started[0].PID == 0 || recorder.started[0].InstanceID == "" ||
		recorder.started[0].InstanceID == recorder.started[1].InstanceID {
		t.Fatalf("missing or reused native process identity: %+v", recorder.started)
	}
	for _, end := range recorder.terminated {
		if end.Kind != LaunchManagedEnded || !end.WaitObserved || end.CleanupScope == "" {
			t.Fatalf("invalid actual wait observation: %+v", end)
		}
	}
}

func TestHostLifecycleStopWinsBetweenPermitAndSpawn(t *testing.T) {
	entered, permit := make(chan struct{}), make(chan struct{})
	ended := make(chan LaunchTermination, 1)
	lifecycle := NewLaunchLifecycle(lifecycleObserverFunc(func(context.Context, LaunchInfo) (LaunchCallbacks, error) {
		close(entered)
		<-permit
		return LaunchCallbacks{
			Started:    func(ProcessObservation) error { t.Error("stopped launch started"); return nil },
			Terminated: func(end LaunchTermination) error { ended <- end; return nil },
		}, nil
	}))
	file, exe := lifecycleHelper(t, lifecycle, "exit")
	cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = lifecycleTestEnv("exit", file.Name())
	done := make(chan error, 1)
	go func() { done <- lifecycle.Run(cmd) }()
	<-entered
	lifecycle.Stop()
	close(permit)
	if err := <-done; !errors.Is(err, ErrLaunchSealed) {
		t.Fatalf("start error = %v, want sealed", err)
	}
	if end := <-ended; end.Kind != LaunchNotStarted || end.WaitObserved {
		t.Fatalf("unused permit = %+v", end)
	}
	if body, _ := os.ReadFile(file.Name()); len(body) != 0 {
		t.Fatal("stop lost its fence: helper executed")
	}
}

func TestHostLifecycleStopKeepsConstructedCommandFenced(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "exit")
	cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = lifecycleTestEnv("exit", file.Name())
	lifecycle.Stop()
	// Providers use this shared start entry after building argv and pipes.
	// Removing a constructed command's observation binding would bypass Stop.
	if err := startOwnedProcessTree(cmd, slog.Default()); !errors.Is(err, ErrLaunchSealed) {
		if err == nil {
			_ = cmd.Wait()
			releaseProcessGroup(cmd)
		}
		t.Fatalf("constructed command lost its stop fence: %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatal("stopped command requested a permit")
	}
}

func TestHostLifecycleRepeatedStartKeepsActiveLaunch(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "block")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := lifecycle.CommandContext(ctx, LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = lifecycleTestEnv("block", file.Name())
	t.Cleanup(func() {
		if cmd.Process != nil && cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = waitOwnedProcessTree(cmd)
		}
		releaseProcessGroup(cmd)
	})
	if err := lifecycle.Start(cmd); err != nil {
		t.Fatal(err)
	}
	scope := processSupervisionScope(cmd)
	if err := lifecycle.Start(cmd); !errors.Is(err, ErrLaunchUnconfirmed) {
		t.Fatalf("repeated start = %v, want unconfirmed", err)
	}
	if got := processSupervisionScope(cmd); got != scope {
		t.Fatalf("repeated start released process ownership: %s -> %s", scope, got)
	}
	if err := lifecycle.Seal(); !errors.Is(err, ErrLaunchUnconfirmed) {
		t.Fatalf("active launch lost its terminal barrier: %v", err)
	}
	lifecycle.Stop()
	_ = lifecycle.Wait(cmd)
	if cmd.ProcessState == nil || len(recorder.terminated) != 1 || !recorder.terminated[0].WaitObserved {
		t.Fatalf("original wait was lost after repeated start: %+v", recorder.terminated)
	}
	if err := lifecycle.Seal(); err != nil {
		t.Fatalf("waited launch could not seal: %v", err)
	}
}

func TestHostLifecycleRejectedCommandCannotBypassGate(t *testing.T) {
	for _, reason := range []string{"stopped before start", "stopped after permit", "permit failure"} {
		t.Run(reason, func(t *testing.T) {
			recorder := &lifecycleRecorder{}
			delegate := recorder.observer()
			failure := errors.New("permit unavailable")
			var lifecycle *LaunchLifecycle
			lifecycle = NewLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
				if reason == "permit failure" {
					return LaunchCallbacks{}, failure
				}
				if reason == "stopped after permit" {
					lifecycle.Stop()
				}
				return delegate.BeforeLaunch(ctx, info)
			}))
			file, exe := lifecycleHelper(t, lifecycle, "exit")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := lifecycle.CommandContext(ctx, LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
			cmd.Env = lifecycleTestEnv("exit", file.Name())
			t.Cleanup(func() {
				if cmd.Process != nil && cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
				releaseProcessGroup(cmd)
			})
			want := ErrLaunchSealed
			if reason == "stopped before start" {
				lifecycle.Stop()
			} else if reason == "permit failure" {
				want = failure
			}
			for attempt := 1; attempt <= 2; attempt++ {
				if err := startOwnedProcessTree(cmd, slog.Default()); !errors.Is(err, want) {
					t.Fatalf("start %d bypassed the original rejection: %v", attempt, err)
				}
			}
			if cmd.Process != nil {
				t.Fatal("rejected command reached native start")
			}
		})
	}
}

func TestHostLifecycleConcurrentWaitKeepsOriginalWait(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "block")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := lifecycle.CommandContext(ctx, LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = lifecycleTestEnv("block", file.Name())
	if err := lifecycle.Start(cmd); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	remaining := 2
	t.Cleanup(func() {
		lifecycle.Stop()
		for ; remaining > 0; remaining-- {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("native waiter failed to finish")
				return
			}
		}
	})
	scope := processSupervisionScope(cmd)
	go func() { done <- lifecycle.Wait(cmd) }()
	go func() { done <- lifecycle.Wait(cmd) }()
	select {
	case err := <-done:
		remaining--
		if !errors.Is(err, ErrLaunchUnconfirmed) {
			t.Fatalf("concurrent wait = %v, want unconfirmed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent waiter was not rejected")
	}
	if got := processSupervisionScope(cmd); got != scope {
		t.Fatalf("concurrent wait released process ownership: %s -> %s", scope, got)
	}
	if err := lifecycle.Seal(); !errors.Is(err, ErrLaunchUnconfirmed) {
		t.Fatalf("original wait lost its terminal barrier: %v", err)
	}
	lifecycle.Stop()
	<-done
	remaining--
	if len(recorder.terminated) != 1 || !recorder.terminated[0].WaitObserved {
		t.Fatalf("original wait was not observed exactly once: %+v", recorder.terminated)
	}
	if err := lifecycle.Seal(); err != nil {
		t.Fatal(err)
	}
}

func TestHostLifecycleStopDuringStartedObservation(t *testing.T) {
	observing, release := make(chan struct{}), make(chan struct{})
	recorder := &lifecycleRecorder{}
	delegate := recorder.observer()
	lifecycle := NewLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
		callbacks, err := delegate.BeforeLaunch(ctx, info)
		started := callbacks.Started
		callbacks.Started = func(process ProcessObservation) error {
			close(observing)
			<-release
			return started(process)
		}
		return callbacks, err
	}))
	file, exe := lifecycleHelper(t, lifecycle, "block")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := lifecycle.CommandContext(ctx, LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = lifecycleTestEnv("block", file.Name())
	started := make(chan error, 1)
	go func() { started <- lifecycle.Start(cmd) }()
	<-observing
	stopped := make(chan struct{})
	go func() { lifecycle.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Error("Stop blocked behind the Started observation")
	}
	close(release)
	<-stopped
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	_ = lifecycle.Wait(cmd)
	if len(recorder.terminated) != 1 || !recorder.terminated[0].WaitObserved {
		t.Fatalf("stopped launch did not retain its actual wait: %+v", recorder.terminated)
	}
}

func TestHostLifecycleGateFailureNeverFallsBack(t *testing.T) {
	failure := errors.New("permit unavailable")
	calls := 0
	lifecycle := NewLaunchLifecycle(lifecycleObserverFunc(func(context.Context, LaunchInfo) (LaunchCallbacks, error) {
		calls++
		return LaunchCallbacks{}, failure
	}))
	file, exe := lifecycleHelper(t, lifecycle, "exit")
	for range 2 {
		cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
		cmd.Env = lifecycleTestEnv("exit", file.Name())
		if err := lifecycle.Run(cmd); !errors.Is(err, failure) {
			t.Fatalf("gate failure lost: %v", err)
		}
	}
	if calls != 1 || !errors.Is(lifecycle.Err(), failure) {
		t.Fatalf("failure permitted a fallback: calls=%d, error=%v", calls, lifecycle.Err())
	}
	if body, _ := os.ReadFile(file.Name()); len(body) != 0 {
		t.Fatal("failed gate spawned a process")
	}
}

func TestHostLifecycleStartedFailureStillWaitsAndBlocksFallback(t *testing.T) {
	failure := errors.New("started observation not durable")
	ended := make(chan LaunchTermination, 1)
	calls := 0
	lifecycle := NewLaunchLifecycle(lifecycleObserverFunc(func(context.Context, LaunchInfo) (LaunchCallbacks, error) {
		calls++
		return LaunchCallbacks{
			Started:    func(ProcessObservation) error { return failure },
			Terminated: func(end LaunchTermination) error { ended <- end; return nil },
		}, nil
	}))
	file, exe := lifecycleHelper(t, lifecycle, "block")
	cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = lifecycleTestEnv("block", file.Name())
	if err := lifecycle.Run(cmd); !errors.Is(err, failure) {
		t.Fatalf("lost started observation error: %v", err)
	}
	select {
	case end := <-ended:
		if end.Kind != LaunchManagedEnded || !end.WaitObserved || cmd.ProcessState == nil {
			t.Fatalf("spawned process was not actually reaped: %+v", end)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("started persistence failure leaked its process")
	}
	other := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	if err := lifecycle.Run(other); !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("observer failure allowed retry: calls=%d error=%v", calls, err)
	}
}

func TestHostLifecycleTerminationFailureBlocksNextPermit(t *testing.T) {
	failure := errors.New("termination receipt unavailable")
	calls := 0
	lifecycle := NewLaunchLifecycle(lifecycleObserverFunc(func(context.Context, LaunchInfo) (LaunchCallbacks, error) {
		calls++
		return LaunchCallbacks{
			Started:    func(ProcessObservation) error { return nil },
			Terminated: func(LaunchTermination) error { return failure },
		}, nil
	}))
	file, exe := lifecycleHelper(t, lifecycle, "exit")
	for range 2 {
		cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
		cmd.Env = lifecycleTestEnv("exit", file.Name())
		if err := lifecycle.Run(cmd); !errors.Is(err, failure) {
			t.Fatalf("termination persistence error lost: %v", err)
		}
	}
	if calls != 1 {
		t.Fatal("new permit issued before previous termination was accepted")
	}
}

func TestHostLifecycleMissingExecutableIsNotStarted(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, filepath.Join(t.TempDir(), "missing-helper"))
	if err := lifecycle.Run(cmd); err == nil {
		t.Fatal("missing executable unexpectedly started")
	}
	if len(recorder.started) != 0 || len(recorder.terminated) != 1 || recorder.terminated[0].Kind != LaunchNotStarted {
		t.Fatalf("failed native start was not accounted for: %+v", recorder)
	}
}

func TestHostLifecycleFiltersExplicitOrdinaryCredentials(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "environment")
	cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = append(lifecycleTestEnv("environment", file.Name()),
		"MULTICA_TOKEN=ordinary", "multica_daemon_token=daemon", "MULTICA_COLLABORATION_TERMINATION_TOKEN=supervisor",
		"MULTICA_COLLABORATION_TOKEN=mcb_test", "MULTICA_COLLABORATION_SERVER=https://collaboration.example", "MULTICA_COLLABORATION_CONTEXT=bound-context",
		"MULTICA_COLLABORATION_ISSUE_ID=bound-issue", "MULTICA_COLLABORATION_RUN_ID=bound-run", "ANTHROPIC_API_KEY=ordinary-provider-account")
	if err := lifecycle.Run(cmd); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(file.Name() + ".env")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"MULTICA_TOKEN=", "multica_daemon_token=", "MULTICA_COLLABORATION_TERMINATION_TOKEN="} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("host launch leaked %s", forbidden)
		}
	}
	for _, kept := range []string{
		"MULTICA_COLLABORATION_TOKEN=mcb_test", "MULTICA_COLLABORATION_SERVER=https://collaboration.example", "MULTICA_COLLABORATION_CONTEXT=bound-context",
		"MULTICA_COLLABORATION_ISSUE_ID=bound-issue", "MULTICA_COLLABORATION_RUN_ID=bound-run", "ANTHROPIC_API_KEY=ordinary-provider-account",
	} {
		if !strings.Contains(string(body), kept) {
			t.Fatalf("host launch dropped selected execution configuration %s", kept)
		}
	}
}

func TestHostLifecycleFiltersAmbientOrdinaryCredentials(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "environment")
	t.Setenv("AGENT_LIFECYCLE_FAKE", "environment")
	t.Setenv("AGENT_LIFECYCLE_RECORD", file.Name())
	t.Setenv("MULTICA_TOKEN", "ordinary-owner")
	t.Setenv("MULTICA_DAEMON_TOKEN", "daemon-owner")
	t.Setenv("MULTICA_COLLABORATION_TOKEN", "mcb_test")
	cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	// Leaving Env nil exercises os/exec's inherited-environment path, without
	// the provider's buildEnv prefilter masking a launch-gate regression.
	if err := lifecycle.Run(cmd); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(file.Name() + ".env")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "MULTICA_TOKEN=") || strings.Contains(string(body), "MULTICA_DAEMON_TOKEN=") {
		t.Fatal("host launch inherited ordinary Multica credentials")
	}
	if !strings.Contains(string(body), "MULTICA_COLLABORATION_TOKEN=mcb_test") {
		t.Fatal("host launch dropped its collaboration identity")
	}
}

func TestHostLifecycleRejectsInvalidEnvironmentRefresh(t *testing.T) {
	for _, entry := range []struct{ key, value string }{
		{"MULTICA_TOKEN", "ordinary-owner"},
		{"multica_daemon_token", "daemon-owner"},
		{"MULTICA_COLLABORATION_TERMINATION_TOKEN", "supervisor"},
		{"", "value"},
		{"INVALID=KEY", "value"},
		{"INVALID\x00KEY", "value"},
		{"VALID_KEY", "invalid\x00value"},
	} {
		t.Run(strconv.Quote(entry.key), func(t *testing.T) {
			recorder := &lifecycleRecorder{}
			delegate := recorder.observer()
			lifecycle := NewLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
				callbacks, err := delegate.BeforeLaunch(ctx, info)
				callbacks.Environment = map[string]string{entry.key: entry.value}
				return callbacks, err
			}))
			file, exe := lifecycleHelper(t, lifecycle, "exit")
			cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
			cmd.Env = lifecycleTestEnv("exit", file.Name())
			if err := lifecycle.Run(cmd); !errors.Is(err, ErrLaunchUnsupported) {
				t.Fatalf("invalid refresh was not rejected: %v", err)
			}
			if cmd.Process != nil || len(recorder.terminated) != 1 || recorder.terminated[0].Kind != LaunchNotStarted {
				t.Fatalf("invalid refresh reached native start or lost its unused permit: %+v", recorder.terminated)
			}
			if !errors.Is(lifecycle.Err(), ErrLaunchUnsupported) {
				t.Fatal("invalid refresh failed to fence later launches")
			}
		})
	}
}

func TestHostLifecycleProviderWaitDoesNotUseEOFOrProcessExited(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "closed-output")
	backend, err := New("opencode", Config{ExecutablePath: exe, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Env: map[string]string{
		"AGENT_LIFECYCLE_FAKE": "closed-output", "AGENT_LIFECYCLE_RECORD": file.Name(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.Execute(context.Background(), "prompt", ExecOptions{Cwd: t.TempDir(), Lifecycle: lifecycle, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lifecycle.Stop)
	go func() {
		for range session.Messages {
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(file.Name() + ".closed"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fake never closed output")
		}
		time.Sleep(10 * time.Millisecond)
	}
	recorder.mu.Lock()
	count := len(recorder.terminated)
	recorder.mu.Unlock()
	if count != 0 {
		t.Fatal("output EOF was mistaken for actual process exit")
	}
	select {
	case result := <-session.Result:
		t.Fatalf("provider completed before actual wait: %+v", result)
	default:
	}
	if err := os.WriteFile(file.Name()+".release", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-session.Result:
		if result.Status != "completed" {
			t.Fatalf("fake result: %+v", result)
		}
		if result.ProcessExited {
			t.Fatal("fixture no longer covers a provider omitting ProcessExited")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("provider failed to finish after native exit")
	}
	if len(recorder.terminated) != 1 || !recorder.terminated[0].WaitObserved {
		t.Fatalf("actual native wait was not recorded: %+v", recorder.terminated)
	}
}

func TestHostLifecycleUnsupportedProviderFailsBeforeSpawn(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "exit")
	backend, err := New("openclaw", Config{ExecutablePath: exe, Env: map[string]string{
		"AGENT_LIFECYCLE_FAKE": "exit", "AGENT_LIFECYCLE_RECORD": file.Name(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Execute(context.Background(), "prompt", ExecOptions{Lifecycle: lifecycle, OpenclawMode: "gateway"}); !errors.Is(err, ErrLaunchUnsupported) {
		t.Fatalf("unverified gateway was accepted: %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatal("unverified provider obtained a launch permit")
	}
	if body, _ := os.ReadFile(file.Name()); len(body) != 0 {
		t.Fatal("unverified provider started a helper or agent")
	}
}

func TestHostLifecycleBusinessResultIndependentOfReceiptFailure(t *testing.T) {
	failure := errors.New("receipt transport unavailable")
	lifecycle := NewLaunchLifecycle(lifecycleObserverFunc(func(context.Context, LaunchInfo) (LaunchCallbacks, error) {
		return LaunchCallbacks{
			Started:    func(ProcessObservation) error { return nil },
			Terminated: func(LaunchTermination) error { return failure },
		}, nil
	}))
	file, exe := lifecycleHelper(t, lifecycle, "opencode")
	backend, err := New("opencode", Config{ExecutablePath: exe, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Env: map[string]string{
		"AGENT_LIFECYCLE_FAKE": "opencode", "AGENT_LIFECYCLE_RECORD": file.Name(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.Execute(context.Background(), "prompt", ExecOptions{Cwd: t.TempDir(), Lifecycle: lifecycle, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for range session.Messages {
	}
	result := <-session.Result
	if result.Status != "completed" || result.Output != "done" || result.BusinessOutcomeMissing {
		t.Fatalf("receipt failure changed business result: %+v", result)
	}
	if !errors.Is(lifecycle.Err(), failure) {
		t.Fatalf("receipt failure was not retained independently: %v", lifecycle.Err())
	}
}

func TestOpenCodeMissingBusinessOutcomePreservesOrdinaryStatus(t *testing.T) {
	file, exe := lifecycleHelper(t, nil, "empty-opencode")
	backend, err := New("opencode", Config{ExecutablePath: exe, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Env: map[string]string{
		"AGENT_LIFECYCLE_FAKE": "empty-opencode", "AGENT_LIFECYCLE_RECORD": file.Name(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.Execute(t.Context(), "prompt", ExecOptions{Cwd: t.TempDir(), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for range session.Messages {
	}
	result := <-session.Result
	if result.Status != "completed" || !result.BusinessOutcomeMissing {
		t.Fatalf("missing result changed the ordinary status or lost its separate marker: %+v", result)
	}
	raw, err := json.Marshal(result)
	if err != nil || strings.Contains(string(raw), "BusinessOutcomeMissing") {
		t.Fatal("internal result evidence leaked into the ordinary wire shape")
	}
}

func TestHostLifecycleCodexTimeoutDoesNotLaunchUnpermittedDiagnostic(t *testing.T) {
	recorder := &lifecycleRecorder{}
	delegate := recorder.observer()
	var lifecycle *LaunchLifecycle
	lifecycle = NewLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
		callbacks, err := delegate.BeforeLaunch(ctx, info)
		terminated := callbacks.Terminated
		callbacks.Terminated = func(end LaunchTermination) error {
			lifecycle.Stop()
			return terminated(end)
		}
		return callbacks, err
	}))
	file, exe := lifecycleHelper(t, lifecycle, "codex-timeout")
	backend, err := New("codex", Config{ExecutablePath: exe, CodexVersion: "0.118.0", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Env: map[string]string{
		"AGENT_LIFECYCLE_FAKE": "codex-timeout", "AGENT_LIFECYCLE_RECORD": file.Name(), "CODEX_HOME": t.TempDir(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.Execute(t.Context(), "prompt", ExecOptions{Cwd: t.TempDir(), Lifecycle: lifecycle, Timeout: 5 * time.Second,
		SemanticInactivityTimeout: 100 * time.Millisecond, FirstTurnNoProgressTimeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	for range session.Messages {
	}
	result := <-session.Result
	if result.Status != "timeout" {
		t.Fatalf("fixture did not reach turn timeout: %+v", result)
	}
	started, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(started), "\n") != 1 || len(recorder.started) != 1 || len(recorder.terminated) != 1 {
		t.Fatal("timeout diagnostics spawned outside the sealed launch lifecycle")
	}
	if !strings.Contains(result.Error, `codex_version="0.118.0"`) {
		t.Fatal("host diagnostic lost the frozen provider version")
	}
}

func TestHostLifecycleCodexInternalRetryUsesNewGateAfterWait(t *testing.T) {
	for _, test := range []struct {
		name          string
		stopAfterWait bool
		rejectReceipt bool
		wantLaunches  int
	}{
		{"permitted retry", false, false, 2},
		{"stop after first wait", true, false, 1},
		{"unacknowledged first wait", false, true, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &lifecycleRecorder{}
			delegate := recorder.observer()
			attemptTemps := []string{t.TempDir(), t.TempDir()}
			permits := 0
			var lifecycle *LaunchLifecycle
			lifecycle = NewLaunchLifecycle(lifecycleObserverFunc(func(ctx context.Context, info LaunchInfo) (LaunchCallbacks, error) {
				callbacks, err := delegate.BeforeLaunch(ctx, info)
				permits++
				callbacks.Environment = map[string]string{
					"MULTICA_COLLABORATION_TOKEN": fmt.Sprintf("mcb_attempt_%d", permits),
					"TMPDIR":                      attemptTemps[permits-1],
				}
				original := callbacks.Terminated
				callbacks.Terminated = func(end LaunchTermination) error {
					if err := original(end); err != nil {
						return err
					}
					if test.stopAfterWait {
						lifecycle.Stop()
					}
					if test.rejectReceipt {
						return errors.New("first receipt not acknowledged")
					}
					return nil
				}
				return callbacks, err
			}))
			file, exe := lifecycleHelper(t, lifecycle, "codex-retry")
			backend, err := New("codex", Config{ExecutablePath: exe, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Env: map[string]string{
				"AGENT_LIFECYCLE_FAKE": "codex-retry", "AGENT_LIFECYCLE_RECORD": file.Name(),
				"CODEX_HOME": t.TempDir(), "MULTICA_COLLABORATION_TOKEN": "mcb_stale", "TMPDIR": t.TempDir(),
			}})
			if err != nil {
				t.Fatal(err)
			}
			session, err := backend.Execute(context.Background(), "prompt", ExecOptions{Cwd: t.TempDir(), Lifecycle: lifecycle, Timeout: 10 * time.Second, HandshakeTimeout: 250 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			for range session.Messages {
			}
			result := <-session.Result
			if test.wantLaunches == 2 && result.Status != "completed" {
				t.Fatalf("permitted retry failed: %+v", result)
			}
			if test.wantLaunches == 1 && result.Status == "completed" {
				t.Fatalf("blocked retry succeeded: %+v", result)
			}
			if len(recorder.started) != test.wantLaunches || len(recorder.terminated) != test.wantLaunches {
				t.Fatalf("unexpected physical launches: starts=%d ends=%d events=%v result=%+v", len(recorder.started), len(recorder.terminated), recorder.events, result)
			}
			body, _ := os.ReadFile(file.Name())
			if strings.Count(string(body), "\n") != test.wantLaunches {
				t.Fatalf("native launches differ from observations: %q", body)
			}
			if test.wantLaunches == 2 && strings.Join(recorder.events, ",") != "permit:provider,started,terminated:managed_launch_ended,permit:provider,started,terminated:managed_launch_ended" {
				t.Fatalf("retry did not wait for previous termination: %v", recorder.events)
			}
			environment, err := os.ReadFile(file.Name() + ".attempt-env")
			if err != nil {
				t.Fatal(err)
			}
			var wantEnvironment strings.Builder
			for attempt := 1; attempt <= test.wantLaunches; attempt++ {
				fmt.Fprintf(&wantEnvironment, "mcb_attempt_%d|%s\n", attempt, attemptTemps[attempt-1])
			}
			if string(environment) != wantEnvironment.String() {
				t.Fatal("a physical Codex launch inherited stale attempt credentials or temporary directory")
			}
		})
	}
}

// The same native executable is used by Windows and Unix. TestMain dispatches
// before the test flag parser, so provider-owned argv cannot select a test.
func runLifecycleTestHelper(mode string) {
	record := os.Getenv("AGENT_LIFECYCLE_RECORD")
	f, err := os.OpenFile(record, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		os.Exit(91)
	}
	_, _ = fmt.Fprintln(f, os.Getpid())
	_ = f.Close()
	switch mode {
	case "exit":
		return
	case "empty-opencode":
		_, _ = io.Copy(io.Discard, os.Stdin)
	case "environment":
		if err := os.WriteFile(record+".env", []byte(strings.Join(os.Environ(), "\n")), 0o600); err != nil {
			os.Exit(92)
		}
	case "block":
		for {
			time.Sleep(time.Second)
		}
	case "closed-output", "opencode":
		_, _ = io.ReadAll(os.Stdin)
		fmt.Println(`{"type":"text","sessionID":"collaboration-native-test","part":{"type":"text","text":"done"}}`)
		fmt.Println(`{"type":"step_finish","sessionID":"collaboration-native-test","part":{"type":"step-finish"}}`)
		if mode == "closed-output" {
			_ = os.Stdout.Close()
			_ = os.Stderr.Close()
			_ = os.WriteFile(record+".closed", nil, 0o600)
			for {
				if _, err := os.Stat(record + ".release"); err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	case "claude":
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Println(`{"type":"result","subtype":"success","is_error":false,"session_id":"collaboration-native-test","result":"done"}`)
	case "codex-retry", "codex-timeout":
		environment, err := os.OpenFile(record+".attempt-env", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			os.Exit(94)
		}
		_, _ = fmt.Fprintf(environment, "%s|%s\n", os.Getenv("MULTICA_COLLABORATION_TOKEN"), os.Getenv("TMPDIR"))
		_ = environment.Close()
		data, _ := os.ReadFile(record)
		if mode == "codex-retry" && strings.Count(string(data), "\n") == 1 {
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			time.Sleep(4 * time.Second)
			return
		}
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			var req struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			if json.Unmarshal(scanner.Bytes(), &req) != nil || len(req.ID) == 0 {
				continue
			}
			payload := `{}`
			if strings.HasPrefix(req.Method, "thread/") {
				payload = `{"thread":{"id":"collaboration-native-test"}}`
			}
			fmt.Printf("{\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":%s}\n", req.ID, payload)
			if req.Method == "turn/start" {
				if mode == "codex-timeout" {
					fmt.Println(`{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"collaboration-native-test","turn":{"id":"turn-1","status":"inProgress"}}}`)
				} else {
					fmt.Println(`{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"collaboration-native-test","turn":{"id":"turn-1","status":"completed"}}}`)
				}
			}
		}
	default:
		_, _ = fmt.Fprintln(os.Stderr, "unexpected lifecycle test mode "+strconv.Quote(mode))
		os.Exit(93)
	}
}
