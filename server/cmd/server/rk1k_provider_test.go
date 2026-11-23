//go:build agentintegration && !windows

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
)

const (
	rk1kProviderName = "rk1k-provider"
	rk1kStandInName  = "rk1k-sim-claude"
	rk1kPlanDirEnv   = "RK1K_PROVIDER_PLAN_DIR"
	rk1kFakeVersion  = "2.1.316"
)

var (
	rk1kToolFrames       func(args []string, shown string) error
	rk1kCLIEvidenceError = errors.New("private CLI evidence publication failed")
)

type rk1kInjectedWriter struct {
	io.Writer
	fault string
}

func (w rk1kInjectedWriter) Write(raw []byte) (int, error) {
	written, err := w.Writer.Write(raw)
	return written, errors.Join(err, rk1InjectedEvidenceError("stdout_write", w.fault))
}

func init() {
	switch strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") {
	case rk1kProviderName:
		dir := os.Getenv(rk1kPlanDirEnv)
		if dir == "" || !filepath.IsAbs(dir) {
			os.Exit(26)
		}
		os.Exit(rk1kProviderMain(dir))
	case rk1kStandInName:
		os.Exit(rk1kStandInMain(filepath.Dir(os.Args[0])))
	}
}

func rk1kProviderIdentity() (rk1kIdentity, error) {
	task, run := os.Getenv("MULTICA_TASK_ID"), os.Getenv("MULTICA_COLLABORATION_RUN_ID")
	if task != "" && run != "" {
		return rk1kIdentity{}, errors.New("ambiguous Provider identity")
	}
	if task != "" {
		id := rk1kIdentity{Kind: "task", TaskID: task, Workspace: os.Getenv("MULTICA_WORKSPACE_ID"),
			Agent: os.Getenv("MULTICA_AGENT_ID")}
		token := os.Getenv("MULTICA_TOKEN")
		if !id.valid() || !strings.HasPrefix(token, "mat_") || len(token) < 12 {
			return rk1kIdentity{}, errors.New("invalid ordinary task identity")
		}
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if strings.HasPrefix(key, "MULTICA_COLLABORATION_") {
				return rk1kIdentity{}, errors.New("ordinary task has collaboration environment")
			}
		}
		return id, nil
	}
	if run != "" {
		ctx, err := cli.ReadCollaborationContext()
		if err != nil || ctx == nil || !strings.HasPrefix(ctx.Token, "mcb_") {
			return rk1kIdentity{}, errors.New("invalid collaboration business identity")
		}
		id := rk1kIdentity{Kind: "run", RunID: run, Host: &ctx.Identity}
		if id.valid() {
			return id, nil
		}
	}
	return rk1kIdentity{}, errors.New("missing Provider identity")
}

func rk1kChildEnv() []string {
	var out []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !rk1RemovedKey(key) && !strings.HasPrefix(key, "RK1K_") &&
			key != "MULTICA_RUN_REAL_AGENT_SMOKE" {
			out = append(out, entry)
		}
	}
	return out
}

func rk1kProviderMain(dir string) int {
	if slices.Contains(os.Args[1:], "--version") {
		fmt.Println(rk1kFakeVersion + " (test-owned Claude stand-in)")
		return 0
	}
	id, err := rk1kProviderIdentity()
	if err != nil {
		return 27
	}
	plan, err := rk1ReadPlan(filepath.Join(dir, id.key()+".json"))
	if err != nil {
		return 28
	}
	// The only pass-through target currently admitted is this test binary's
	// stand-in. Real account admission is closed in the knowledge entry.
	if plan.Mode == "real" && (plan.Real != filepath.Join(dir, rk1kStandInName) || !rk1kTestExecutable(plan.Real)) {
		return 41
	}
	launch, err := rk1NextLaunchDir(filepath.Join(dir, id.key()))
	if err != nil {
		return 29
	}
	rk1LaunchDir = launch
	cwd, err := os.Getwd()
	if err != nil {
		return 30
	}
	purpose := "mcb"
	if id.Kind == "task" {
		purpose = "mat"
	}
	meta := rk1kLaunchMeta{rk1LaunchMeta: rk1LaunchMeta{Args: os.Args[1:], Cwd: cwd,
		Mode: plan.Mode, Behavior: plan.Behavior, Fault: plan.Fault, StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
		EvidenceState: "started"}, Identity: id, CredentialPurpose: purpose, Seed: plan.Params["seed"] == "true"}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if rk1RemovedKey(key) || strings.HasPrefix(key, "RK1K_") {
			meta.RemovedEnv = append(meta.RemovedEnv, key)
		}
	}
	slices.Sort(meta.RemovedEnv)
	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	frame, err := rk1ReadFrame(reader)
	meta.PromptBytes = len(frame)
	if err != nil || rk1WriteFile(filepath.Join(launch, "stdin-first.jsonl"), frame) != nil ||
		rk1WriteJSON(filepath.Join(launch, "launch.json"), meta) != nil {
		return 30
	}
	capture, err := os.OpenFile(filepath.Join(launch, "stdout.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 31
	}
	var output io.Writer = io.MultiWriter(os.Stdout, capture)
	if plan.Mode == "fake" {
		output = rk1kInjectedWriter{Writer: output, fault: plan.Fault}
	}
	code := 32
	if plan.Mode == "real" {
		var started bool
		code, started = rk1kExec(plan.Real, os.Args[1:], rk1kChildEnv(),
			io.MultiReader(bytes.NewReader(frame), reader), output)
		if started {
			meta.FakeStarts = 1
		}
	} else {
		meta.FakeStarts = 1
		code, _ = rk1kFake(plan, id, frame, output)
	}
	var finalizer rk1CaptureFinalizer = capture
	if plan.Mode == "fake" {
		finalizer = rk1InjectedEvidenceCapture{rk1CaptureFinalizer: capture, fault: plan.Fault}
	}
	if err := rk1FinalizeLaunchEvidence(finalizer, &meta.rk1LaunchMeta, code, func(final rk1LaunchMeta) error {
		meta.rk1LaunchMeta = final
		raw, err := os.ReadFile(filepath.Join(launch, "stdout.jsonl"))
		if err != nil {
			return err
		}
		meta.NativeID, meta.ResultSeen = rk1kObserveNativeResult(raw)
		return errors.Join(rk1WriteJSON(filepath.Join(launch, "launch.json"), meta),
			rk1InjectedEvidenceError("launch_record", plan.Fault))
	}); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, rk1EvidenceFailureMarker)
		return rk1EvidenceFailureExit(code)
	}
	return code
}

// The borrowed executor returns only an exit code. This sibling also retains
// whether Start actually succeeded so a refused stand-in is not counted.
func rk1kExec(path string, args, env []string, stdin io.Reader, stdout io.Writer) (int, bool) {
	return rk1kExecObserved(path, args, env, stdin, stdout, nil)
}

func rk1kExecObserved(path string, args, env []string, stdin io.Reader, stdout io.Writer, started func() error) (int, bool) {
	if !filepath.IsAbs(path) {
		return 40, false
	}
	cmd := exec.Command(path, args...)
	cmd.Env, cmd.Stdout, cmd.Stderr, cmd.WaitDelay = env, stdout, os.Stderr, rk1ChildWaitBudget
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return 41, false
	}
	if err := cmd.Start(); err != nil {
		_ = pipe.Close()
		return 41, false
	}
	if started != nil {
		if err := started(); err != nil {
			_ = pipe.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return 43, true
		}
	}
	go func() { _, _ = io.Copy(pipe, stdin); _ = pipe.Close() }()
	signals, done := make(chan os.Signal, 2), make(chan struct{})
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			select {
			case sig := <-signals:
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()
	err = cmd.Wait()
	signal.Stop(signals)
	close(done)
	if err == nil {
		return 0, true
	}
	if exit, ok := err.(*exec.ExitError); ok {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal()), true
		}
		return exit.ExitCode(), true
	}
	return 42, true
}

func rk1kTestExecutable(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return false
	}
	digest := func(name string) ([]byte, error) {
		file, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		_, readErr := io.Copy(h, file)
		return h.Sum(nil), errors.Join(readErr, file.Close())
	}
	a, firstErr := digest(path)
	b, secondErr := digest(self)
	return firstErr == nil && secondErr == nil && bytes.Equal(a, b)
}

func rk1kStandInMain(dir string) int {
	if slices.Contains(os.Args[1:], "--version") {
		fmt.Println(rk1kFakeVersion + " (test-owned Claude stand-in)")
		return 0
	}
	id, err := rk1kProviderIdentity()
	if err != nil {
		return 27
	}
	plan, err := rk1ReadPlan(filepath.Join(dir, id.key()+".json"))
	if err != nil || plan.Mode != "real" {
		return 28
	}
	var keys []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if rk1WriteJSON(filepath.Join(dir, id.key()+".sim-env.json"), keys) != nil ||
		rk1WriteFile(filepath.Join(dir, id.key()+".sim-pid"), []byte(strconv.Itoa(os.Getpid()))) != nil {
		return 29
	}
	frame, err := rk1ReadFrame(bufio.NewReaderSize(os.Stdin, 1<<20))
	if err != nil {
		return 30
	}
	plan.Mode = "fake"
	code, _ := rk1kFake(plan, id, frame, os.Stdout)
	return code
}

// CLI tools use one explicitly frozen executable. They never look for an
// installed Multica binary or substitute human/PAT credentials.
func rk1kCLI(path string, args ...string) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Base(path) != "multica" {
		return nil, errors.New("missing test-owned Multica CLI")
	}
	cmd := exec.Command(path, args...)
	cmd.Env = rk1kChildEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if recordErr := rk1kRecordCLI(args, out); recordErr != nil {
		_, _ = fmt.Fprintln(os.Stderr, rk1EvidenceFailureMarker)
		return out, errors.Join(rk1kCLIEvidenceError, recordErr)
	}
	if rk1kToolFrames != nil {
		if frameErr := rk1kToolFrames(args, string(out)); frameErr != nil {
			return out, errors.Join(rk1kCLIEvidenceError, frameErr)
		}
	}
	if err != nil {
		return nil, errors.New("task-scoped CLI request failed")
	}
	return out, nil
}

func rk1kRecordCLI(args []string, out []byte) error {
	if rk1LaunchDir == "" {
		return nil
	}
	name := args
	if len(name) > 3 {
		name = name[:3]
	}
	raw, err := json.Marshal(map[string]any{"command": strings.Join(name, " "), "stdout_bytes": len(out), "stdout_chars": utf8.RuneCount(out),
		"next_cursor_char_offset": bytes.Index(out, []byte(`"next_cursor"`)), "next_offset_char_offset": bytes.Index(out, []byte(`"next_offset"`))})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(rk1LaunchDir, "cli-calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(append(raw, '\n'))
	return errors.Join(err, file.Sync(), file.Close())
}

func rk1kFake(plan rk1Plan, id rk1kIdentity, frame []byte, stdout io.Writer) (int, string) {
	if plan.Fault == "crash" {
		return 3, ""
	}
	prompt, err := rk1kPrompt(frame)
	if err != nil {
		return 4, ""
	}
	items, err := rk1kContributions(prompt)
	if err != nil {
		return 4, ""
	}
	native := plan.Params["native_id"]
	if native == "" {
		native = uuid.NewString()
	}
	if !rk1kUUID(native) {
		return 4, ""
	}
	if plan.Fault == "changed_native" {
		native = uuid.NewString()
	}
	if id.Kind == "task" && plan.Behavior != "guard" {
		store := plan.Params["native_store"]
		if !filepath.IsAbs(store) {
			return 4, ""
		}
		if err := os.MkdirAll(store, 0700); err != nil {
			return 4, ""
		}
		path := filepath.Join(store, native+".jsonl")
		var history []byte
		resume := ""
		for i, arg := range os.Args[1:] {
			if arg == "--resume" && i+2 < len(os.Args) {
				resume = os.Args[i+2]
			}
		}
		if resume != "" {
			if resume != native {
				return 4, ""
			}
			history, err = os.ReadFile(path)
			if err != nil || len(history) == 0 {
				return 4, ""
			}
		} else if _, err := os.Lstat(path); err == nil {
			return 4, ""
		}
		if rk1WriteFile(path, append(history, frame...)) != nil {
			return 4, ""
		}
	}
	emit := func(value any) error {
		raw, err := json.Marshal(value)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, rk1EvidenceFailureMarker)
			return err
		}
		_, err = stdout.Write(append(raw, '\n'))
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, rk1EvidenceFailureMarker)
		}
		return err
	}
	text := func(value string) error {
		return emit(map[string]any{"type": "assistant", "message": map[string]any{"model": "rk1k-fake",
			"content": []map[string]string{{"type": "text", "text": value}}}})
	}
	if plan.Fault != "missing_native" {
		if emit(map[string]any{"type": "system", "subtype": "init", "session_id": native, "model": "rk1k-fake"}) != nil {
			return rk1EvidenceFailureExit(4), native
		}
	}
	calls := 0
	priorFrames := rk1kToolFrames
	defer func() { rk1kToolFrames = priorFrames }()
	rk1kToolFrames = func(args []string, shown string) error {
		calls++
		toolID := fmt.Sprintf("toolu_rk1k_%d", calls)
		if err := emit(map[string]any{"type": "assistant", "message": map[string]any{"model": "rk1k-fake",
			"content": []map[string]any{{"type": "tool_use", "id": toolID, "name": "Bash",
				"input": map[string]string{"command": "multica " + strings.Join(args, " ")}}}}}); err != nil {
			return err
		}
		return emit(map[string]any{"type": "user", "message": map[string]any{"role": "user",
			"content": []map[string]any{{"type": "tool_result", "tool_use_id": toolID, "content": shown}}}})
	}
	useCLI := func(args ...string) int {
		_, err := rk1kCLI(plan.Params["cli"], args...)
		if errors.Is(err, rk1kCLIEvidenceError) {
			return rk1EvidenceFailureExit(4)
		}
		if err != nil {
			return 4
		}
		return 0
	}
	if plan.Behavior != "guard" {
		if id.Kind == "task" {
			if code := useCLI("project", "get", plan.Params["project"], "--output", "json"); code != 0 {
				return code, native
			}
			if plan.Behavior == "quick_create" {
				if os.Getenv("MULTICA_QUICK_CREATE_TASK_ID") != id.TaskID {
					return 4, native
				}
				if code := useCLI("issue", "create", "--title", "RK1K "+id.TaskID,
					"--description", "Created by the actual quick-create task", "--project", plan.Params["project"],
					"--status", "backlog", "--output", "json"); code != 0 {
					return code, native
				}
			}
		} else {
			issue := os.Getenv("MULTICA_COLLABORATION_ISSUE_ID")
			for _, args := range [][]string{{"collaboration", "memory", "show", issue, "--output", "json"}, {"collaboration", "handoff", "show", issue, "--output", "json"}} {
				if code := useCLI(args...); code != 0 {
					return code, native
				}
			}
			if plan.Behavior == "delegate" {
				if code := useCLI("issue", "comment", "add", os.Getenv("MULTICA_COLLABORATION_ISSUE_ID"),
					"--content", plan.Params["comment"], "--request-id", uuid.NewString(), "--output", "json"); code != 0 {
					return code, native
				}
			}
		}
	}
	answer := "NONE"
	var material []string
	for _, item := range items {
		material = append(material, item.Content)
	}
	if len(material) > 0 {
		answer = strings.Join(material, "\n")
	}
	if plan.Fault == "wrong" {
		answer = "WELL_FORMED_WRONG"
	}
	if text(answer) != nil {
		return rk1EvidenceFailureExit(4), native
	}
	if plan.Fault == "missing_result" {
		return 0, native
	}
	result := map[string]any{"type": "result", "is_error": plan.Fault == "provider_error", "result": answer}
	if plan.Fault != "missing_native" {
		result["session_id"] = native
	}
	if emit(result) != nil {
		return rk1EvidenceFailureExit(4), native
	}
	return 0, native
}
