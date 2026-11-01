//go:build agentintegration && !windows

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// RK1 Provider wrapper. HostExecutor launches a copy of this test binary named
// rk1-provider; init dispatches it before TestMain can open a database. The
// plan file for the run selects either a test-owned fake behavior or the real
// Claude CLI. Both modes capture argv, the exact first stdin frame and stdout.
const (
	rk1ProviderName    = "rk1-provider"
	rk1SimClaudeName   = "rk1-sim-claude"
	rk1PlanDirEnv      = "RK1_PROVIDER_PLAN_DIR"
	rk1FakeVersion     = "2.1.200"
	rk1MaxPromptFrame  = 8 << 20
	rk1ChildWaitBudget = 5 * time.Second
)

type rk1Plan struct {
	Mode     string            `json:"mode"` // fake | real
	Behavior string            `json:"behavior,omitempty"`
	Fault    string            `json:"fault,omitempty"`
	Real     string            `json:"real_executable,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
}

type rk1LaunchMeta struct {
	Args        []string `json:"args"`
	Cwd         string   `json:"cwd"`
	Mode        string   `json:"mode"`
	Behavior    string   `json:"behavior"`
	Fault       string   `json:"fault,omitempty"`
	RemovedEnv  []string `json:"removed_env"`
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at,omitempty"`
	ExitCode    *int     `json:"exit_code,omitempty"`
	PromptBytes int      `json:"prompt_frame_bytes"`
}

func init() {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if name == rk1SimClaudeName {
		// Test-owned stand-in for the real CLI, hard-linked into a trial's plan
		// directory so the wrapper's real pass-through path runs before any paid
		// launch. Its plan directory is its own: RK1_* never reaches it.
		os.Exit(rk1SimClaudeMain(filepath.Dir(os.Args[0])))
	}
	if name != rk1ProviderName {
		return
	}
	dir := os.Getenv(rk1PlanDirEnv)
	if dir == "" || !filepath.IsAbs(dir) {
		// A copied Provider without its marker must never enter TestMain.
		os.Exit(26)
	}
	os.Exit(rk1ProviderMain(dir))
}

func rk1ProviderMain(dir string) int {
	if slices.Contains(os.Args[1:], "--version") {
		// The real CLI is probed directly by the harness, never through here.
		fmt.Println(rk1FakeVersion + " (Claude Code)")
		return 0
	}
	runID := os.Getenv("MULTICA_COLLABORATION_RUN_ID")
	if parsed, err := uuid.Parse(runID); err != nil || parsed.String() != runID {
		return 27
	}
	plan, err := rk1ReadPlan(filepath.Join(dir, runID+".json"))
	if err != nil {
		return 28
	}
	launch, err := rk1NextLaunchDir(filepath.Join(dir, runID))
	if err != nil {
		return 29
	}
	rk1LaunchDir = launch
	cwd, _ := os.Getwd()
	meta := rk1LaunchMeta{Args: os.Args[1:], Cwd: cwd, Mode: plan.Mode, Behavior: plan.Behavior, Fault: plan.Fault,
		RemovedEnv: rk1RemovedEnv(), StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	frame, err := rk1ReadFrame(reader)
	meta.PromptBytes = len(frame)
	if err != nil || rk1WriteFile(filepath.Join(launch, "stdin-first.jsonl"), frame) != nil || rk1WriteJSON(filepath.Join(launch, "launch.json"), meta) != nil {
		return 30
	}
	capture, err := os.OpenFile(filepath.Join(launch, "stdout.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 31
	}
	stdout := io.MultiWriter(os.Stdout, capture)
	code := 32
	switch plan.Mode {
	case "real":
		code = rk1Exec(plan.Real, os.Args[1:], rk1ChildEnv(), io.MultiReader(strings.NewReader(string(frame)), reader), stdout)
	case "fake":
		code = rk1Fake(plan, frame, stdout)
	}
	_ = capture.Sync()
	_ = capture.Close()
	meta.FinishedAt, meta.ExitCode = time.Now().UTC().Format(time.RFC3339Nano), &code
	_ = rk1WriteJSON(filepath.Join(launch, "launch.json"), meta)
	return code
}

// rk1ChildEnv removes test-process state that production host mode would not
// carry. MULTICA_* is already removed by the backend before this process.
func rk1ChildEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !rk1RemovedKey(key) {
			env = append(env, entry)
		}
	}
	return env
}

func rk1RemovedKey(key string) bool {
	return key == "DATABASE_URL" || strings.HasPrefix(key, "RK1_") || strings.HasPrefix(key, "PG")
}

func rk1RemovedEnv() []string {
	var keys []string
	for _, entry := range os.Environ() {
		if key, _, _ := strings.Cut(entry, "="); rk1RemovedKey(key) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

func rk1Exec(path string, args, env []string, stdin io.Reader, stdout io.Writer) int {
	if !filepath.IsAbs(path) {
		return 40
	}
	cmd := exec.Command(path, args...)
	cmd.Env, cmd.Stdout, cmd.Stderr, cmd.WaitDelay = env, stdout, os.Stderr, rk1ChildWaitBudget
	// An explicit pipe: Wait must not wait for a copier blocked on the
	// backend's stdin, or a child that dies early holds the launch to timeout.
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return 41
	}
	if err := cmd.Start(); err != nil {
		return 41
	}
	go func() { _, _ = io.Copy(pipe, stdin); _ = pipe.Close() }()
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for sig := range signals {
			_ = cmd.Process.Signal(sig)
		}
	}()
	err = cmd.Wait()
	signal.Stop(signals)
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exit.ExitCode()
	}
	return 42
}

// rk1SimClaudeMain behaves like the real CLI for the framework's purposes: it
// reads the stream-json frame the wrapper forwards, records which environment
// keys reached it, and runs the run's fake behavior/fault.
func rk1SimClaudeMain(dir string) int {
	if slices.Contains(os.Args[1:], "--version") {
		fmt.Println(rk1FakeVersion + " (Claude Code)")
		return 0
	}
	runID := os.Getenv("MULTICA_COLLABORATION_RUN_ID")
	plan, err := rk1ReadPlan(filepath.Join(dir, runID+".json"))
	if err != nil || plan.Mode != "real" {
		return 28
	}
	var keys []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		keys = append(keys, key)
	}
	slices.Sort(keys)
	// The pid lets the proof check that Stop's group kill reached this process.
	if rk1WriteJSON(filepath.Join(dir, runID+".sim-env.json"), keys) != nil || rk1WriteFile(filepath.Join(dir, runID+".sim-pid"), []byte(strconv.Itoa(os.Getpid()))) != nil {
		return 29
	}
	frame, err := rk1ReadFrame(bufio.NewReaderSize(os.Stdin, 1<<20))
	if err != nil {
		return 30
	}
	return rk1Fake(rk1Plan{Mode: "fake", Behavior: plan.Behavior, Fault: plan.Fault, Params: plan.Params}, frame, os.Stdout)
}
