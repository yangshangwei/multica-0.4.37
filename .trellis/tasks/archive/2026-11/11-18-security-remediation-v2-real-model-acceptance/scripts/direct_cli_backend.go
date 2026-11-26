//go:build agentintegration && !windows

package agent

// This test-only backend executes the official CLI directly. It is selected
// only by the task's reviewed overlay; production ResolveBackend is unchanged.

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

type securityDirectCLIConfig struct {
	Mode                string `json:"mode"`
	ExecutionMode       string `json:"execution_mode"`
	OutputDir           string `json:"output_dir"`
	Model               string `json:"model"`
	ThinkingLevel       string `json:"thinking_level"`
	Scenario            string `json:"scenario"`
	FixtureManifestPath string `json:"fixture_manifest_path"`
}

type securityDirectCLIBackend struct {
	cfg          Config
	registration securityDirectCLIConfig
}

func securityDirectCLIEnabled() bool { return os.Getenv("SECREM_DIRECT_CLI") == "1" }

func newSecurityDirectCLIBackend(cfg Config) (Backend, error) {
	if os.Getenv("SECREM_DIRECT_CLI") != "1" {
		return nil, errors.New("direct CLI test seam is disabled")
	}
	path := os.Getenv("SECREM_HARNESS_CONFIG")
	info, err := os.Lstat(path)
	if err != nil || !filepath.IsAbs(path) || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("direct CLI private registration required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != os.Getenv("SECREM_HARNESS_CONFIG_SHA256") {
		return nil, errors.New("direct CLI registration hash differs")
	}
	var registration securityDirectCLIConfig
	if json.Unmarshal(raw, &registration) != nil || registration.ExecutionMode != "official-cli-direct" || !filepath.IsAbs(registration.OutputDir) {
		return nil, errors.New("official CLI mode registration required")
	}
	if registration.Mode == "real" {
		if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" || os.Getenv("SECREM_REAL_EXECUTION_AUTHORIZED") != "1" {
			return nil, errors.New("direct CLI real entry is disabled")
		}
	} else if registration.Mode != "fake" || os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		return nil, errors.New("direct CLI fake/real gate mismatch")
	}
	if !filepath.IsAbs(cfg.ExecutablePath) || len(cfg.LaunchPrefix) != 0 {
		return nil, errors.New("direct CLI executable must be an explicit unprefixed path")
	}
	return &securityDirectCLIBackend{cfg: cfg, registration: registration}, nil
}

type securityDirectCLIEvent struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Item     struct {
		ID               string `json:"id"`
		Type             string `json:"type"`
		Text             string `json:"text"`
		Command          string `json:"command"`
		AggregatedOutput string `json:"aggregated_output"`
		ExitCode         *int   `json:"exit_code"`
	} `json:"item"`
	Usage *struct {
		Input  int64 `json:"input_tokens"`
		Cached int64 `json:"cached_input_tokens"`
		Output int64 `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

func (b *securityDirectCLIBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	if b.registration.Model != "" && opts.Model != b.registration.Model {
		return nil, errors.New("direct CLI model differs from registration")
	}
	if b.registration.ThinkingLevel != "" && opts.ThinkingLevel != "" && opts.ThinkingLevel != b.registration.ThinkingLevel {
		return nil, errors.New("direct CLI effort differs from registration")
	}
	if opts.SystemPrompt != "" || len(opts.McpConfig) != 0 && string(opts.McpConfig) != "null" && string(opts.McpConfig) != "{}" || len(opts.ExtraArgs) != 0 || len(opts.CustomArgs) != 0 || opts.Knowledge != nil || opts.Lifecycle != nil || opts.ServiceTier != "" {
		return nil, errors.New("unsupported direct CLI option; no prompt or capability projection allowed")
	}
	if !filepath.IsAbs(opts.Cwd) {
		return nil, errors.New("direct CLI requires actual task cwd")
	}
	var fixturePath string
	var fixtureHash [32]byte
	if b.registration.Scenario == "P1" {
		manifestPath := b.registration.FixtureManifestPath
		if !filepath.IsAbs(manifestPath) {
			return nil, errors.New("direct CLI frozen fixture manifest is missing")
		}
		manifestRaw, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			return nil, errors.New("frozen fixture manifest unavailable")
		}
		var frozen map[string]string
		if json.Unmarshal(manifestRaw, &frozen) != nil {
			return nil, errors.New("frozen fixture manifest malformed")
		}
		const relative = "fixtures/repos/production/drill.json"
		source := filepath.Join(filepath.Dir(filepath.Dir(manifestPath)), relative)
		resolved, pathErr := filepath.EvalSymlinks(source)
		if pathErr != nil || resolved != source {
			return nil, errors.New("frozen fixture path must not traverse symlinks")
		}
		material, readErr := os.ReadFile(source)
		if readErr != nil {
			return nil, errors.New("frozen P1 material unavailable")
		}
		fixtureHash = sha256.Sum256(material)
		if hex.EncodeToString(fixtureHash[:]) != frozen[relative] {
			return nil, errors.New("frozen P1 material hash differs")
		}
		fixturePath = filepath.Join(opts.Cwd, "drill.json")
		file, writeErr := os.OpenFile(fixturePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0444)
		if writeErr != nil {
			return nil, errors.New("P1 material destination already exists or is unavailable")
		}
		_, writeErr = file.Write(material)
		writeErr = errors.Join(writeErr, file.Sync(), file.Close())
		if writeErr != nil {
			return nil, errors.New("P1 material could not be delivered")
		}
	}
	agents, err := os.ReadFile(filepath.Join(opts.Cwd, "AGENTS.md"))
	if err != nil {
		return nil, errors.New("complete generated AGENTS missing")
	}
	args := []string{"exec", "--json", "--skip-git-repo-check", "-"}
	if opts.ResumeSessionID != "" {
		args = []string{"exec", "resume", "--json", "--skip-git-repo-check", opts.ResumeSessionID, "-"}
	}
	runCtx, cancel := runContext(ctx, opts.Timeout)
	cmd := exec.CommandContext(runCtx, b.cfg.ExecutablePath, args...)
	cmd.Dir = opts.Cwd
	cmd.Env = buildEnv(b.cfg.Env)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second
	outputDir := filepath.Join(b.registration.OutputDir, "direct-cli", b.cfg.TaskID)
	if err = os.MkdirAll(outputDir, 0700); err != nil {
		cancel()
		return nil, err
	}
	privateFile := func(name string) (*os.File, error) {
		return os.OpenFile(filepath.Join(outputDir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	}
	stderr, err := privateFile("stderr.txt")
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stderr.Close()
		cancel()
		return nil, err
	}
	rawOutput, err := privateFile("events.jsonl")
	if err != nil {
		stderr.Close()
		cancel()
		return nil, err
	}
	promptHash := sha256.Sum256([]byte(prompt))
	agentsHash := sha256.Sum256(agents)
	start := time.Now()
	if err = cmd.Start(); err != nil {
		rawOutput.Close()
		stderr.Close()
		cancel()
		return nil, err
	}
	messages := make(chan Message, 32)
	result := make(chan Result, 1)
	go func() {
		defer cancel()
		defer close(result)
		final := Result{Status: "failed", BusinessOutcomeMissing: true, Usage: map[string]TokenUsage{}}
		var terminal, streamFailure, usageObserved bool
		threadEvents := 0
		var lastText, threadID, streamReason string
		scanner := bufio.NewScanner(io.TeeReader(stdout, rawOutput))
		scanner.Buffer(make([]byte, 64<<10), 16<<20)
		emit := func(m Message) {
			select {
			case messages <- m:
			case <-runCtx.Done():
			}
		}
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(strings.TrimSpace(string(line))) == 0 {
				continue
			}
			var event securityDirectCLIEvent
			if json.Unmarshal(line, &event) != nil || event.Type == "" {
				streamFailure = true
				streamReason = "malformed native CLI event"
				continue
			}
			switch event.Type {
			case "thread.started":
				threadEvents++
				if _, parseErr := uuid.Parse(event.ThreadID); parseErr != nil || threadEvents != 1 {
					streamFailure = true
					streamReason = "conflicting native CLI thread identity"
					continue
				}
				threadID = event.ThreadID
				emit(Message{Type: MessageStatus, Status: "running", SessionID: threadID})
			case "turn.started":
				emit(Message{Type: MessageStatus, Status: "running"})
			case "item.started":
				if event.Item.Type == "command_execution" {
					emit(Message{Type: MessageToolUse, Tool: "shell", CallID: event.Item.ID, Input: map[string]any{"command": event.Item.Command}})
				}
			case "item.completed":
				switch event.Item.Type {
				case "agent_message":
					lastText = event.Item.Text
					emit(Message{Type: MessageText, Content: event.Item.Text})
				case "reasoning":
					emit(Message{Type: MessageThinking, Content: event.Item.Text})
				case "command_execution":
					emit(Message{Type: MessageToolResult, Tool: "shell", CallID: event.Item.ID, Output: event.Item.AggregatedOutput})
				}
			case "turn.completed":
				if terminal {
					streamFailure = true
					streamReason = "multiple native CLI terminal events"
				}
				terminal = true
				final.Status = "completed"
				final.BusinessOutcomeMissing = false
				if event.Usage != nil {
					usageObserved = true
					final.Usage[opts.Model] = TokenUsage{InputTokens: event.Usage.Input, CacheReadTokens: event.Usage.Cached, OutputTokens: event.Usage.Output}
				}
			case "turn.failed":
				if terminal {
					streamFailure = true
					streamReason = "multiple native CLI terminal events"
				}
				terminal = true
				final.Status = "failed"
				final.BusinessOutcomeMissing = false
				if event.Error != nil {
					final.Error = event.Error.Message
				} else {
					final.Error = "native CLI turn failed"
				}
			case "error":
				streamFailure = true
				streamReason = event.Message
				if streamReason == "" {
					streamReason = "native CLI error"
				}
			}
		}
		if scanner.Err() != nil {
			streamFailure = true
			streamReason = "native CLI stream read failed"
		}
		waitErr := cmd.Wait()
		final.ProcessExited = cmd.ProcessState != nil
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		close(messages)
		if runCtx.Err() != nil {
			final.Status = "timeout"
			final.Error = "direct CLI watchdog or cancellation"
		} else if streamFailure {
			final.Status = "failed"
			final.Error = streamReason
		} else if waitErr != nil {
			final.Status = "failed"
			final.Error = "native CLI process exited unsuccessfully"
		} else if !terminal || threadID == "" {
			final.Status = "failed"
			final.Error = "native CLI terminal or thread identity missing"
		}
		final.Output = lastText
		final.SessionID = threadID
		final.DurationMs = time.Since(start).Milliseconds()
		retentionErr := errors.Join(rawOutput.Sync(), rawOutput.Close(), stderr.Sync(), stderr.Close())
		after, readErr := os.ReadFile(filepath.Join(opts.Cwd, "AGENTS.md"))
		afterHash := sha256.Sum256(after)
		if retentionErr != nil || readErr != nil || afterHash != agentsHash {
			final.Status = "failed"
			final.Error = "direct CLI evidence retention or instruction identity failed"
		}
		fixtureUnchanged := true
		if fixturePath != "" {
			current, fixtureErr := os.ReadFile(fixturePath)
			fixtureUnchanged = fixtureErr == nil && sha256.Sum256(current) == fixtureHash
			if !fixtureUnchanged {
				final.Status = "failed"
				final.Error = "read-only scenario material changed"
			}
		}
		metadata := map[string]any{"execution_mode": "official-cli-direct", "prompt_sha256": hex.EncodeToString(promptHash[:]), "prompt_bytes": len([]byte(prompt)), "agents_sha256": hex.EncodeToString(agentsHash[:]), "agents_bytes": len(agents), "agents_unchanged": readErr == nil && afterHash == agentsHash, "model_requested": opts.Model, "reasoning_effort_requested": opts.ThinkingLevel, "session_id": threadID, "resume_requested": opts.ResumeSessionID, "usage_observed": usageObserved, "terminal_observed": terminal, "process_exited": final.ProcessExited, "status": final.Status, "duration_ms": final.DurationMs, "argv": args, "fixture_sha256": hex.EncodeToString(fixtureHash[:]), "fixture_unchanged": fixtureUnchanged}
		data, _ := json.MarshalIndent(metadata, "", "  ")
		meta, metaErr := privateFile("metadata.json")
		if metaErr == nil {
			_, metaErr = meta.Write(append(data, '\n'))
			metaErr = errors.Join(metaErr, meta.Sync(), meta.Close())
		}
		if retentionErr != nil || metaErr != nil || readErr != nil || afterHash != agentsHash {
			final.Status = "failed"
			final.Error = "direct CLI evidence retention or instruction identity failed"
		}
		result <- final
	}()
	return &Session{Messages: messages, Result: result}, nil
}

var _ Backend = (*securityDirectCLIBackend)(nil)
