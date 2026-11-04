package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/pelletier/go-toml/v2"
)

// The stdout reader owns all fields except armed. The lifecycle goroutine sets
// threadID before arming; the atomic store publishes that identity to the reader.
// Until the turn/start response supplies its ID, no notification may affect the
// output, usage, errors, or completion of the admitted input.
type codexKnowledgeTurn struct {
	armed        atomic.Bool
	threadID     string
	turnID       string
	failed       bool
	pendingBytes int
	pending      []codexKnowledgeNotification
}

type codexKnowledgeNotification struct {
	method string
	params map[string]any
}

const (
	codexKnowledgePendingEvents = 256
	codexKnowledgePendingBytes  = 4 << 20
)

func (g *codexKnowledgeTurn) arm(threadID string) {
	g.threadID = threadID
	g.armed.Store(true)
}

func codexKnowledgeTurnID(method string, params map[string]any) string {
	if method == "turn/started" || method == "turn/completed" {
		return extractNestedString(params, "turn", "id")
	}
	id, _ := params["turnId"].(string)
	return id
}

func (c *codexClient) handleKnowledgeNotification(method string, params map[string]any) {
	g := &c.knowledgeTurn
	if !g.armed.Load() || g.failed || c.turnCompleted {
		return
	}
	threadID, _ := params["threadId"].(string)
	turnID := codexKnowledgeTurnID(method, params)
	if threadID != g.threadID || !validKnowledgeIdentity(turnID) {
		return
	}
	if method != "turn/started" && method != "turn/completed" && method != "error" && !strings.HasPrefix(method, "item/") {
		return
	}
	if g.turnID == "" {
		raw, err := json.Marshal(params)
		if err != nil || len(g.pending) >= codexKnowledgePendingEvents || g.pendingBytes+len(raw) > codexKnowledgePendingBytes {
			g.failed, g.pending = true, nil
			return
		}
		g.pendingBytes += len(raw)
		g.pending = append(g.pending, codexKnowledgeNotification{method, params})
		return
	}
	if turnID == g.turnID {
		c.handleRawNotification(method, params)
	}
}

// Called by the stdout reader before publishing the RPC result to its waiter.
// This preserves ordering for notifications arriving on either side of the reply.
func (c *codexClient) bindKnowledgeTurn(result json.RawMessage, requestErr error) error {
	g := &c.knowledgeTurn
	var response struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if requestErr != nil || !g.armed.Load() || g.failed || json.Unmarshal(result, &response) != nil || !validKnowledgeIdentity(response.Turn.ID) {
		g.failed, g.pending = true, nil
		return &KnowledgeLaunchError{Reason: KnowledgeContextChanged, Err: requestErr}
	}
	g.turnID, c.turnID = response.Turn.ID, response.Turn.ID
	pending := g.pending
	g.pending = nil
	for _, notification := range pending {
		c.handleKnowledgeNotification(notification.method, notification.params)
	}
	return nil
}

func codexKnowledgeFingerprint(cfg Config, opts ExecOptions) (string, error) {
	home := cfg.Env["CODEX_HOME"]
	if !filepath.IsAbs(home) || opts.Knowledge != nil && opts.CodexEphemeral {
		return "", knowledgeUnsupported()
	}
	sourceHash, err := codexKnowledgeConfigFingerprint(cfg.Env, opts.McpConfig, true)
	if err != nil {
		return "", err
	}
	workspaceSources, err := codexKnowledgeWorkspaceFingerprint(opts.Cwd, cfg.Env)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(struct {
		Version                                           int
		Executable, CLI, Cwd, Model, Thinking, Tier, Home string
		Args                                              []string
		MCP                                               json.RawMessage
		Ephemeral                                         bool
		Sources, WorkspaceSources                         string
	}{1, cfg.ExecutablePath, cfg.CLIVersion, opts.Cwd, opts.Model, opts.ThinkingLevel, opts.ServiceTier, home,
		buildCodexArgs(opts, slog.New(slog.NewTextHandler(io.Discard, nil))), opts.McpConfig, opts.CodexEphemeral, sourceHash, workspaceSources})
	if err != nil {
		return "", knowledgeUnsupported()
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func withKnowledgeCodexLifecycle(ctx context.Context, cfg Config, opts ExecOptions) (result context.Context, err error) {
	defer func() {
		if err != nil && opts.Knowledge != nil && opts.Lifecycle != nil {
			opts.Lifecycle.fail(err)
		}
	}()
	if opts.CodexEphemeral {
		if !KnowledgeLifecycleSupported("codex", opts.OpenclawMode, cfg.BuiltinRuntime) || opts.ResumeSessionID != "" ||
			opts.Knowledge != nil || len(opts.ExtraArgs) != 0 || len(opts.CustomArgs) != 0 || len(cfg.LaunchPrefix) != 0 {
			return nil, knowledgeUnsupported()
		}
	}
	if opts.Knowledge != nil {
		if err := validateKnowledgeIntent(opts); err != nil {
			return nil, err
		}
		// A continuity notice must be finalized by the daemon before prepare.
		if opts.ResumeExpected && opts.ResumeSessionID == "" && opts.ResumeContinuityNotice != "" {
			return nil, &KnowledgeLaunchError{Reason: KnowledgeContextChanged}
		}
		fingerprint, err := KnowledgeInvocationFingerprint("codex", cfg, opts)
		if err != nil {
			return nil, err
		}
		if fingerprint != opts.Knowledge.InvocationFingerprint {
			return nil, &KnowledgeLaunchError{Reason: KnowledgeContextChanged}
		}
	}
	return withExecutionLifecycle(ctx, opts, "codex")
}

func codexPrivateThreadResult(raw json.RawMessage, opts ExecOptions) error {
	if opts.Knowledge == nil && !opts.CodexEphemeral {
		return nil
	}
	var result struct {
		Thread struct {
			ID        string  `json:"id"`
			Ephemeral *bool   `json:"ephemeral"`
			Path      *string `json:"path"`
		} `json:"thread"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return &KnowledgeLaunchError{Reason: KnowledgeContextChanged}
	}
	id, err := uuid.Parse(result.Thread.ID)
	if err != nil || id.String() != result.Thread.ID || result.Thread.Ephemeral == nil ||
		*result.Thread.Ephemeral != opts.CodexEphemeral || opts.CodexEphemeral && result.Thread.Path != nil {
		return &KnowledgeLaunchError{Reason: KnowledgeContextChanged}
	}
	return nil
}

// KnowledgeCodexConfigFingerprint binds account/config sources without disclosing credentials.
func KnowledgeCodexConfigFingerprint(env map[string]string) (string, error) {
	return codexKnowledgeConfigFingerprint(env, nil, false)
}

func codexKnowledgeConfigFingerprint(env map[string]string, mcp json.RawMessage, normalize bool) (string, error) {
	home := env["CODEX_HOME"]
	if !filepath.IsAbs(home) {
		return "", knowledgeUnsupported()
	}
	// Only digests leave this function. The effective task home can contain
	// API credentials, and those bytes never belong in logs or wire metadata.
	sources := map[string]string{}
	for _, name := range []string{"config.toml", "config.json", "auth.json", "instructions.md", "AGENTS.md", "AGENTS.override.md"} {
		path := filepath.Join(home, name)
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			err = nil
			raw = nil
		}
		if err != nil {
			return "", knowledgeUnsupported()
		}
		if normalize && name == "config.toml" {
			content, err := codexMcpConfigContents(string(raw), mcp)
			if err != nil {
				return "", knowledgeUnsupported()
			}
			raw = []byte(content)
		}
		if name == "config.toml" {
			if err := codexKnowledgeInstructionSources(raw, home, env, sources); err != nil {
				return "", err
			}
		}
		digest := sha256.Sum256(raw)
		sources[path] = hex.EncodeToString(digest[:])
	}
	for _, key := range []string{"OPENAI_API_KEY", "CODEX_API_KEY", "OPENAI_BASE_URL"} {
		value, ok := env[key]
		if !ok {
			value = os.Getenv(key)
		}
		digest := sha256.Sum256([]byte(value))
		sources[key] = hex.EncodeToString(digest[:])
	}
	raw, err := json.Marshal(sources)
	if err != nil {
		return "", knowledgeUnsupported()
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

// Bind project-layer configuration and instruction bytes as well as their paths.
// Trusted operator instructions remain allowed; a changed source needs a new prepare.
func codexKnowledgeWorkspaceFingerprint(cwd string, env map[string]string) (string, error) {
	if !filepath.IsAbs(cwd) {
		return "", knowledgeUnsupported()
	}
	sources := map[string]string{}
	for dir := filepath.Clean(cwd); ; dir = filepath.Dir(dir) {
		for _, name := range []string{"AGENTS.md", "AGENTS.override.md", filepath.Join(".codex", "config.toml")} {
			path := filepath.Join(dir, name)
			raw, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				raw = nil
				err = nil
			}
			if err != nil {
				return "", knowledgeUnsupported()
			}
			if name == filepath.Join(".codex", "config.toml") {
				if err := codexKnowledgeInstructionSources(raw, filepath.Dir(path), env, sources); err != nil {
					return "", err
				}
			}
			digest := sha256.Sum256(raw)
			sources[path] = hex.EncodeToString(digest[:])
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	raw, err := json.Marshal(sources)
	if err != nil {
		return "", knowledgeUnsupported()
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func codexKnowledgeInstructionSources(raw []byte, base string, env map[string]string, sources map[string]string) error {
	var config map[string]any
	if err := toml.Unmarshal(raw, &config); err != nil {
		return knowledgeUnsupported()
	}
	var visit func(map[string]any) error
	visit = func(values map[string]any) error {
		for key, value := range values {
			if key == "env_key" {
				name, ok := value.(string)
				if !ok || name == "" || strings.ContainsAny(name, "=\x00") {
					return knowledgeUnsupported()
				}
				resolved, exists := env[name]
				if !exists {
					resolved = os.Getenv(name)
				}
				digest := sha256.Sum256([]byte(resolved))
				sources["env:"+name] = hex.EncodeToString(digest[:])
			}
			if key == "env_http_headers" {
				headers, ok := value.(map[string]any)
				if !ok {
					return knowledgeUnsupported()
				}
				for _, binding := range headers {
					name, ok := binding.(string)
					if !ok || name == "" || strings.ContainsAny(name, "=\x00") {
						return knowledgeUnsupported()
					}
					resolved, exists := env[name]
					if !exists {
						resolved = os.Getenv(name)
					}
					digest := sha256.Sum256([]byte(resolved))
					sources["env:"+name] = hex.EncodeToString(digest[:])
				}
			}
			if child, ok := value.(map[string]any); ok {
				if err := visit(child); err != nil {
					return err
				}
			}
			if key != "model_instructions_file" && key != "experimental_instructions_file" {
				continue
			}
			path, ok := value.(string)
			if !ok || path == "" {
				return knowledgeUnsupported()
			}
			if strings.HasPrefix(path, "~/") {
				home := env["HOME"]
				if home == "" {
					home, _ = os.UserHomeDir()
				}
				path = filepath.Join(home, path[2:])
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(base, path)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return knowledgeUnsupported()
			}
			digest := sha256.Sum256(content)
			sources[path] = hex.EncodeToString(digest[:])
		}
		return nil
	}
	return visit(config)
}

func codexDiagnosticID(private bool, id string) string {
	if !private || id == "" {
		return id
	}
	digest := sha256.Sum256([]byte(id))
	return hex.EncodeToString(digest[:])
}
