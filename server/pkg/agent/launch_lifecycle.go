package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

const (
	LaunchKindProvider    = "provider"
	LaunchKindPreparation = "preparation"
	LaunchNotStarted      = "not_started"
	LaunchManagedEnded    = "managed_launch_ended"
	LaunchUnknown         = "unknown"

	SupervisionProcessGroup = "process_group"
	SupervisionWindowsJob   = "windows_job"
	SupervisionDirect       = "direct_process"

	CleanupTrackedScopeComplete = "tracked_scope_complete"
	CleanupBestEffort           = "best_effort"
	CleanupUnverified           = "unverified"
)

var (
	ErrLaunchSealed      = errors.New("managed launches are sealed")
	ErrLaunchUnconfirmed = errors.New("managed launch termination is unconfirmed")
	ErrLaunchUnsupported = errors.New("provider mode has no verified managed lifecycle")
	errUnresumedWrapper  = fmt.Errorf("host wrapper requires process-tree ownership: %w", ErrLaunchUnsupported)
)

// LaunchInfo contains only safe adapter identity, never argv or environment.
type LaunchInfo struct {
	Provider string
	Kind     string
}

// ProcessObservation identifies the actual process instance while its native
// handle belongs to this execution. InstanceID is not restart recovery proof.
// A handle identity is used if the platform start-time query was unavailable;
// only waiting that original handle can then establish an end observation.
type ProcessObservation struct {
	PID              int
	InstanceID       string
	StartedAt        time.Time
	SupervisionScope string
}

// LaunchTermination is independent of the provider's business result. A real
// Wait can end a failed execution; output EOF or a successful result cannot.
// Cleanup describes only the recorded process group/Job/direct-process scope.
type LaunchTermination struct {
	Kind             string
	Process          ProcessObservation
	WaitObserved     bool
	EndedAt          time.Time
	Cleanup          string
	CleanupScope     string
	CleanupAttempted bool
}

// LaunchCallbacks belong to one durable launch registration. Both callbacks
// must be provided. A callback failure permanently prevents further launches
// through this lifecycle; recovery must replay the original durable record.
type LaunchCallbacks struct {
	Started    func(ProcessObservation) error
	Terminated func(LaunchTermination) error
	// Environment refreshes attempt-scoped credentials and temporary paths
	// after permission is granted, including on a provider's internal retry.
	// Ordinary task/daemon Multica identities are never accepted here.
	Environment map[string]string
}

// LaunchObserver persists registration and obtains permission synchronously
// before any irreversible native start. The returned closures retain that
// launch's identity. It must not authorize a new identity on uncertain replay.
type LaunchObserver interface {
	BeforeLaunch(context.Context, LaunchInfo) (LaunchCallbacks, error)
}

// LaunchLifecycle gates every physical provider/helper start in one logical
// execution, including provider-internal retries. Stop and native Start share
// a mutex. Network permission work does not hold that mutex, so Stop can win
// while an authorization request is outstanding.
type LaunchLifecycle struct {
	mu       sync.Mutex
	starting sync.Mutex
	observer LaunchObserver
	sealed   bool
	failure  error
	commands map[*exec.Cmd]*observedLaunch
}

type observedLaunch struct {
	lifecycle *LaunchLifecycle
	ctx       context.Context
	info      LaunchInfo
	cmd       *exec.Cmd
	native    *os.Process
	needsTree bool
	callbacks LaunchCallbacks
	process   ProcessObservation
	phase     string
	unwatch   func() bool
}

type executionLifecycleKey struct{}

type executionLifecycleContext struct {
	lifecycle *LaunchLifecycle
	info      LaunchInfo
	mode      string
}

var observedRuntimeCommands sync.Map // *exec.Cmd -> *observedLaunch

func NewLaunchLifecycle(observer LaunchObserver) *LaunchLifecycle {
	return &LaunchLifecycle{observer: observer, commands: make(map[*exec.Cmd]*observedLaunch)}
}

// HostLifecycleSupported is an adapter allowlist, not platform/version rollout
// approval. The daemon additionally requires its admitted capability matrix.
// Windows shell/batch wrappers require Job ownership and completed Job work;
// direct native executables may use the declared direct-process fallback.
func HostLifecycleSupported(provider, mode string) bool {
	if mode != "" && mode != "local" {
		return false
	}
	switch provider {
	case "claude", "codex", "opencode":
		return true
	default:
		return false
	}
}

func withExecutionLifecycle(ctx context.Context, opts ExecOptions, provider string) (context.Context, error) {
	if opts.Lifecycle == nil {
		return ctx, nil
	}
	if !HostLifecycleSupported(provider, opts.OpenclawMode) {
		return nil, ErrLaunchUnsupported
	}
	return context.WithValue(ctx, executionLifecycleKey{}, executionLifecycleContext{
		lifecycle: opts.Lifecycle,
		info:      LaunchInfo{Provider: provider, Kind: LaunchKindProvider},
		mode:      opts.OpenclawMode,
	}), nil
}

func bindCommandLifecycle(ctx context.Context, cmd *exec.Cmd) *exec.Cmd {
	value, ok := ctx.Value(executionLifecycleKey{}).(executionLifecycleContext)
	if !ok || value.lifecycle == nil {
		return cmd
	}
	l := value.lifecycle
	state := &observedLaunch{lifecycle: l, ctx: ctx, info: value.info, cmd: cmd, phase: "created"}
	l.mu.Lock()
	l.commands[cmd] = state
	l.mu.Unlock()
	observedRuntimeCommands.Store(cmd, state)
	state.unwatch = context.AfterFunc(ctx, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if state.phase == "created" {
			// With the command context already cancelled, os/exec itself also
			// forbids a later native start. Until then retain the stop tombstone.
			delete(l.commands, cmd)
			observedRuntimeCommands.Delete(cmd)
		}
	})
	return cmd
}

func (l *LaunchLifecycle) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.failure
}

// Seal prevents new launches. It succeeds only after every admitted launch's
// terminal callback completed. It does not turn a stop or EOF into evidence.
func (l *LaunchLifecycle) Seal() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sealed = true
	if l.failure != nil {
		return l.failure
	}
	for _, state := range l.commands {
		if state.phase != "created" {
			return ErrLaunchUnconfirmed
		}
	}
	return nil
}

// Stop seals first, then cancels every known live native process. The caller
// still waits for each original terminal observation before settling its run.
func (l *LaunchLifecycle) Stop() {
	l.mu.Lock()
	l.sealed = true
	var commands []*exec.Cmd
	for cmd, state := range l.commands {
		switch state.phase {
		case "observing_start", "started", "waiting":
			commands = append(commands, cmd)
		}
	}
	l.mu.Unlock()
	for _, cmd := range commands {
		signalProcessGroup(cmd, syscall.SIGKILL)
	}
}

func (l *LaunchLifecycle) fail(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failure == nil {
		l.failure = err
	}
	l.sealed = true
}

func (s *observedLaunch) forget() {
	l := s.lifecycle
	if s.unwatch != nil {
		s.unwatch()
	}
	l.mu.Lock()
	if s.cmd.Process == nil && s.cmd.Err == nil {
		// A rejected or never-started command must remain unusable after its
		// binding is released. Otherwise a repeated shared Start would fall
		// through to the ordinary path while its context is still live.
		s.cmd.Err = ErrLaunchSealed
		if l.failure != nil {
			s.cmd.Err = l.failure
		}
	}
	delete(l.commands, s.cmd)
	l.mu.Unlock()
	observedRuntimeCommands.Delete(s.cmd)
}

func (s *observedLaunch) beginWait() error {
	l := s.lifecycle
	l.mu.Lock()
	defer l.mu.Unlock()
	if s.phase != "started" {
		return ErrLaunchUnconfirmed
	}
	s.phase = "waiting"
	return nil
}

func (s *observedLaunch) start(logger *slog.Logger) error {
	l := s.lifecycle
	l.starting.Lock()
	defer l.starting.Unlock()
	l.mu.Lock()
	var blocked error
	switch {
	case l.failure != nil:
		blocked = l.failure
	case l.sealed:
		blocked = ErrLaunchSealed
	case s.ctx.Err() != nil:
		blocked = s.ctx.Err()
	case s.phase != "created":
		blocked = ErrLaunchUnconfirmed
	case l.observer == nil:
		blocked = errors.New("managed launch observer is required")
	case s.info.Kind == LaunchKindProvider && !HostLifecycleSupported(s.info.Provider, ""):
		blocked = ErrLaunchUnsupported
	case s.info.Kind == LaunchKindPreparation && s.info.Provider != "git":
		blocked = ErrLaunchUnsupported
	case s.info.Kind != LaunchKindPreparation && s.info.Kind != LaunchKindProvider:
		blocked = ErrLaunchUnsupported
	}
	if blocked == nil {
		for _, other := range l.commands {
			if other != s && other.phase != "created" {
				blocked = ErrLaunchUnconfirmed
				break
			}
		}
	}
	if blocked != nil {
		unstarted := s.phase == "created"
		if unstarted && s.cmd.Err == nil {
			s.cmd.Err = blocked
		}
		l.mu.Unlock()
		if unstarted {
			s.forget()
		}
		return blocked
	}
	s.phase = "authorizing"
	l.mu.Unlock()
	callbacks, err := l.observer.BeforeLaunch(s.ctx, s.info)
	if err != nil {
		l.fail(err)
		s.forget()
		return err
	}
	s.callbacks = callbacks
	if callbacks.Started == nil || callbacks.Terminated == nil {
		err := errors.New("managed launch callbacks are incomplete")
		l.fail(err)
		// The registration may have consumed a permit. Keep the state unknown
		// rather than inventing a terminal receipt without its callback.
		return err
	}
	l.mu.Lock()
	if l.failure != nil {
		err = l.failure
	} else if l.sealed {
		err = ErrLaunchSealed
	} else {
		err = s.ctx.Err()
	}
	if err != nil {
		l.mu.Unlock()
		return errors.Join(err, s.finish(LaunchTermination{Kind: LaunchNotStarted, EndedAt: time.Now().UTC(), Cleanup: CleanupUnverified}))
	}
	s.phase = "starting"
	s.cmd.Env = collaborationLaunchEnv(s.cmd.Environ())
	for key, value := range callbacks.Environment {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, 0) ||
			strings.HasPrefix(strings.ToUpper(key), "MULTICA_") && len(collaborationLaunchEnv([]string{key + "=" + value})) == 0 {
			l.mu.Unlock()
			err := ErrLaunchUnsupported
			l.fail(err)
			return errors.Join(err, s.finish(LaunchTermination{Kind: LaunchNotStarted, EndedAt: time.Now().UTC(), Cleanup: CleanupUnverified}))
		}
		filtered := s.cmd.Env[:0]
		for _, entry := range s.cmd.Env {
			name, _, _ := strings.Cut(entry, "=")
			if !strings.EqualFold(name, key) {
				filtered = append(filtered, entry)
			}
		}
		s.cmd.Env = append(filtered, key+"="+value)
	}
	instanceID := "handle:" + uuid.NewString()
	s.needsTree = managedProcessTreeRequired(s.cmd)
	err = startNativeProcessTree(s.cmd, logger)
	if errors.Is(err, errUnresumedWrapper) {
		// This suspended wrapper never ran, so it could not create provider
		// children. Its own real Wait can still end this failed attempt.
		s.needsTree = false
		l.sealed = true
	}
	if s.cmd.Process != nil {
		s.native = s.cmd.Process
		// A failed Windows resume may already have waited the original
		// process. Never query that released process's potentially reused PID.
		if s.cmd.ProcessState == nil {
			if nativeID, identityErr := nativeProcessInstanceID(s.native); identityErr == nil {
				instanceID = nativeID
			}
		}
		s.process = ProcessObservation{
			PID: s.cmd.Process.Pid, InstanceID: instanceID,
			StartedAt: time.Now().UTC(), SupervisionScope: processSupervisionScope(s.cmd),
		}
		s.phase = "observing_start"
	}
	l.mu.Unlock()
	if s.cmd.Process != nil {
		if observedErr := callbacks.Started(s.process); observedErr != nil {
			l.fail(observedErr)
			err = errors.Join(err, observedErr)
		}
		l.mu.Lock()
		s.phase = "started"
		l.mu.Unlock()
	}
	if err == nil {
		return nil
	}
	if s.cmd.Process == nil {
		return errors.Join(err, s.finish(LaunchTermination{Kind: LaunchNotStarted, EndedAt: time.Now().UTC(), Cleanup: CleanupUnverified}))
	}
	// Windows can create a suspended child, then fail to resume it after an
	// actual Wait. Every post-start error keeps that native start in the ledger.
	if s.cmd.ProcessState == nil {
		signalProcessGroup(s.cmd, syscall.SIGKILL)
		if s.cmd.WaitDelay == 0 {
			s.cmd.WaitDelay = probeWaitDelay
		}
		err = errors.Join(err, waitOwnedProcessTree(s.cmd))
	} else {
		err = errors.Join(err, s.waited())
	}
	releaseProcessGroup(s.cmd)
	return err
}

func (s *observedLaunch) finish(end LaunchTermination) error {
	l := s.lifecycle
	l.mu.Lock()
	s.phase = "reporting"
	l.mu.Unlock()
	err := s.callbacks.Terminated(end)
	if end.Kind == LaunchUnknown {
		err = errors.Join(err, ErrLaunchUnconfirmed)
	}
	if err != nil {
		l.fail(err)
	}
	s.forget()
	return err
}

func (s *observedLaunch) waited() error {
	end := LaunchTermination{
		Kind: LaunchUnknown, Process: s.process, EndedAt: time.Now().UTC(),
		Cleanup: CleanupUnverified, CleanupScope: s.process.SupervisionScope,
	}
	if s.cmd.ProcessState != nil && s.cmd.Process == s.native && s.cmd.ProcessState.Pid() == s.process.PID {
		end.Kind, end.WaitObserved = LaunchManagedEnded, true
		// Native executables manage their direct launch; descendant cleanup is
		// a separate bounded fact. A Windows shell wrapper also needs its Job
		// to end because the actual provider runs in a child of that wrapper.
		signalProcessGroup(s.cmd, syscall.SIGKILL)
		end.Cleanup, end.CleanupAttempted = CleanupBestEffort, true
		treeEnded := s.process.SupervisionScope != SupervisionDirect && waitProcessGroupGone(s.cmd, 250*time.Millisecond)
		if treeEnded {
			end.Cleanup = CleanupTrackedScopeComplete
		}
		if s.needsTree && !treeEnded {
			end.Kind = LaunchUnknown
		}
	}
	return s.finish(end)
}

// Start and Wait are the only supported way to run a command returned by
// CommandContext. Calling exec.Cmd.Start/Wait directly bypasses this contract.
func (l *LaunchLifecycle) Start(cmd *exec.Cmd) error {
	state, ok := observedRuntimeCommands.Load(cmd)
	if !ok || state.(*observedLaunch).lifecycle != l {
		return fmt.Errorf("managed command identity: %w", ErrLaunchUnsupported)
	}
	return state.(*observedLaunch).start(slog.Default())
}

func (l *LaunchLifecycle) Wait(cmd *exec.Cmd) error {
	state, ok := observedRuntimeCommands.Load(cmd)
	if !ok || state.(*observedLaunch).lifecycle != l {
		return fmt.Errorf("managed command identity: %w", ErrLaunchUnsupported)
	}
	launch := state.(*observedLaunch)
	if err := launch.beginWait(); err != nil {
		return err
	}
	err := waitObservedProcessTree(launch)
	// Only the admitted waiter may release ownership. A duplicate Wait must
	// leave the active process's Job intact for the original waiter.
	releaseProcessGroup(cmd)
	return errors.Join(err, l.Err())
}

func (l *LaunchLifecycle) Run(cmd *exec.Cmd) error {
	if err := l.Start(cmd); err != nil {
		return err
	}
	return l.Wait(cmd)
}

func collaborationLaunchEnv(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(key), "MULTICA_") {
			switch key {
			case "MULTICA_COLLABORATION_TOKEN", "MULTICA_COLLABORATION_SERVER", "MULTICA_COLLABORATION_CONTEXT", "MULTICA_COLLABORATION_ISSUE_ID", "MULTICA_COLLABORATION_RUN_ID":
			default:
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
