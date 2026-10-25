package collaborationrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// HostConfig selects the already configured ordinary runtime. Executable and
// options are populated from the daemon's registered runtime, never a task's
// text. AccountSource identifies the selected account configuration, not a key.
type HostConfig struct {
	Provider        string                                  `json:"provider"`
	ProviderVersion string                                  `json:"provider_version"`
	ProviderMode    string                                  `json:"provider_mode"`
	AccountSource   string                                  `json:"account_source"`
	Git             *HostGitConfig                          `json:"git,omitempty"`
	Limits          protocol.CollaborationPreparationLimits `json:"limits"`
	Executable      string                                  `json:"-"`
	ExtraArgs       []string                                `json:"-"`
	DefaultModel    string                                  `json:"-"`
}

type HostGitConfig struct {
	Executable   string `json:"executable"`
	BundleFile   string `json:"bundle_file"`
	BundleDigest string `json:"bundle_digest"`
	SizeBytes    int64  `json:"size_bytes"`
	ObjectFormat string `json:"object_format"`
	CommitOID    string `json:"commit_oid"`
}

func (c *HostConfig) Validate() error {
	if c == nil || !agent.HostLifecycleSupported(c.Provider, c.ProviderMode) || c.ProviderMode != "local" ||
		c.ProviderVersion == "" || len(c.AccountSource) == 0 || len(c.AccountSource) > 256 || strings.ContainsAny(c.AccountSource, "\x00\r\n") {
		return ErrUnsupported
	}
	if _, valid := agent.NormalizeHostProviderVersion(c.Provider, c.ProviderVersion); !valid {
		return ErrUnsupported
	}
	if c.Provider == "opencode" && len(c.ExtraArgs) != 0 || !slices.Equal(c.ExtraArgs, agent.FilterLaunchPrefix(c.Provider, c.ExtraArgs, slog.New(slog.NewTextHandler(io.Discard, nil)))) {
		return ErrUnsupported
	}
	for _, arg := range c.ExtraArgs {
		if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
			return ErrUnsupported
		}
		name, _, _ := strings.Cut(arg, "=")
		switch name {
		case "--resume", "-r", "--continue", "--session", "--session-id", "--fork-session", "--attach", "--server", "--url", "--cwd", "--cd", "-C", "--directory", "--workdir", "--config-dir", "--model", "-m":
			return ErrUnsupported
		}
		if strings.HasPrefix(arg, "-r") && !strings.HasPrefix(arg, "--") || c.Provider == "claude" && arg == "-c" {
			return ErrUnsupported
		}
	}
	if len(c.ExtraArgs) > 64 {
		return ErrUnsupported
	}
	if g := c.Git; g != nil && (!filepath.IsAbs(g.Executable) || !filepath.IsAbs(g.BundleFile) || !validDigest(g.BundleDigest) ||
		g.SizeBytes < 1 || g.SizeBytes > maxProviderFileBytes || !gitObjectID(g.CommitOID, g.ObjectFormat)) {
		return ErrUnsupported
	}
	// Capability validation checks exact limits, provider identity and platform.
	caps := c.capabilityShape(strings.Repeat("0", 64))
	if err := protocol.ValidateCollaborationRuntimeCapabilities(caps); err != nil {
		return ErrUnsupported
	}
	return nil
}

func (c *HostConfig) capabilityShape(digest string) protocol.CollaborationRuntimeCapabilities {
	version, _ := agent.NormalizeHostProviderVersion(c.Provider, c.ProviderVersion)
	scopes := []string{agent.SupervisionProcessGroup}
	if runtime.GOOS == "windows" {
		scopes = []string{agent.SupervisionWindowsJob, agent.SupervisionDirect}
	}
	var git *protocol.CollaborationPreparationGit
	if c.Git != nil {
		git = &protocol.CollaborationPreparationGit{ObjectFormat: c.Git.ObjectFormat, CommitOID: c.Git.CommitOID, BundleDigest: c.Git.BundleDigest}
	}
	return protocol.CollaborationRuntimeCapabilities{Version: protocol.CollaborationHostCapabilityVersion, PreparationVersion: protocol.CollaborationHostPreparationVersion,
		OS: runtime.GOOS, Arch: runtime.GOARCH, Containment: protocol.CollaborationContainmentHostProcessSupervision, AttachmentFormat: protocol.CollaborationAttachmentFormat,
		Provider: protocol.CollaborationPreparationProvider{Provider: c.Provider, ProtocolFamily: c.Provider, AdapterID: "ordinary_backend_v1", AdapterVersion: 1,
			AccountSource: "runtime_configuration", Resume: "isolated", Fallback: true}, Git: git, Limits: c.Limits,
		Host: &protocol.CollaborationHostCapability{ProviderVersion: version, ProviderMode: c.ProviderMode, ConfigurationDigest: digest, SupervisionScopes: scopes}}
}

func (c *HostConfig) Capabilities() (protocol.CollaborationRuntimeCapabilities, error) {
	if c.Validate() != nil || !filepath.IsAbs(c.Executable) {
		return protocol.CollaborationRuntimeCapabilities{}, ErrUnsupported
	}
	info, err := os.Stat(c.Executable)
	if err != nil || !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return protocol.CollaborationRuntimeCapabilities{}, ErrUnsupported
	}
	// Token refresh is intentionally absent. Detectable managed invocation and
	// executable changes require a new capability and renewed issue consent.
	configuration := *c
	configuration.ProviderVersion, _ = agent.NormalizeHostProviderVersion(c.Provider, c.ProviderVersion)
	raw, err := json.Marshal(struct {
		Config       *HostConfig `json:"config"`
		Executable   string      `json:"executable"`
		Size         int64       `json:"size"`
		Modified     int64       `json:"modified"`
		ExtraArgs    []string    `json:"extra_args"`
		DefaultModel string      `json:"default_model"`
	}{&configuration, c.Executable, info.Size(), info.ModTime().UnixNano(), c.ExtraArgs, c.DefaultModel})
	if err != nil {
		return protocol.CollaborationRuntimeCapabilities{}, err
	}
	sum := sha256.Sum256(raw)
	return c.capabilityShape(hex.EncodeToString(sum[:])), nil
}

type HostActivity struct {
	LastActivity      atomic.Int64
	InFlightTools     atomic.Int32
	WatchdogFired     atomic.Bool
	WatchdogThreshold atomic.Int64
}

// HostExecutor reuses the ordinary Backend/ExecOptions/Session leaf. It never
// invokes ordinary issue/task admission, credentials, prompts or callbacks.
type HostExecutor struct {
	Config        *HostConfig
	Directory     string
	CLIExecutable string
	Options       agent.ExecOptions
	Logger        *slog.Logger
	Watchdog      func(context.Context, *HostActivity, context.CancelFunc, <-chan agent.Message)
}

func (h *HostExecutor) Probe(context.Context) error {
	if h == nil || h.Config == nil || !filepath.IsAbs(h.Directory) || !filepath.IsAbs(h.CLIExecutable) {
		return ErrUnsupported
	}
	opts := h.Options
	if opts.Cwd != "" || opts.Model != "" || opts.SystemPrompt != "" || opts.ThreadName != "" || opts.MaxTurns != 0 || opts.ResumeSessionID != "" ||
		opts.ResumeExpected || opts.ResumeContinuityNotice != "" || len(opts.ExtraArgs) != 0 || len(opts.CustomArgs) != 0 || len(opts.McpConfig) != 0 ||
		opts.ThinkingLevel != "" || opts.ServiceTier != "" || opts.OpenclawMode != "" || opts.ClaudeSettingsPath != "" || opts.QwenpawWorkspace != "" || opts.Lifecycle != nil {
		return ErrUnsupported
	}
	if _, err := h.Config.Capabilities(); err != nil {
		return err
	}
	info, err := os.Stat(h.CLIExecutable)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxProviderFileBytes ||
		runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return ErrUnsupported
	}
	return privateDirectory(h.Directory)
}

func (h *HostExecutor) logger() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

const hostStopGrace = 15 * time.Second
