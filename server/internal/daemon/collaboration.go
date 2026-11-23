package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// CollaborationRunnerConfig is an explicit single-runtime opt-in. Host mode
// selects an already registered ordinary runtime; legacy bundle settings remain
// separate and never become an implicit fallback for host execution.
type CollaborationRunnerConfig struct {
	RuntimeID                  string                                 `json:"runtime_id"`
	Executable                 string                                 `json:"executable"`
	CLIExecutable              string                                 `json:"cli_executable,omitempty"`
	Arguments                  []string                               `json:"arguments"`
	LedgerDirectory            string                                 `json:"ledger_directory"`
	SandboxDirectory           string                                 `json:"sandbox_directory"`
	ExpectedRuntimeConfigEpoch string                                 `json:"expected_runtime_config_epoch,omitempty"`
	Bundle                     *collaborationrun.ProviderBundleConfig `json:"bundle,omitempty"`
	Host                       *collaborationrun.HostConfig           `json:"host,omitempty"`
	WorkspaceDirectory         string                                 `json:"workspace_directory,omitempty"`
}

func parseCollaborationRunnerConfig(raw string) (*CollaborationRunnerConfig, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 65536 {
		return nil, errors.New("collaboration runner configuration is too large")
	}
	var cfg CollaborationRunnerConfig
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&cfg) != nil || d.Decode(new(any)) != io.EOF || !managedUUID.MatchString(cfg.RuntimeID) || !filepath.IsAbs(cfg.LedgerDirectory) {
		return nil, errors.New("invalid collaboration runner configuration")
	}
	if cfg.Host != nil {
		epoch, err := strconv.ParseInt(cfg.ExpectedRuntimeConfigEpoch, 10, 64)
		if cfg.Host.Validate() != nil || !filepath.IsAbs(cfg.WorkspaceDirectory) || cfg.SandboxDirectory != "" || cfg.Bundle != nil ||
			cfg.Executable != "" || len(cfg.Arguments) != 0 || err != nil || epoch < 1 || strconv.FormatInt(epoch, 10) != cfg.ExpectedRuntimeConfigEpoch ||
			cfg.CLIExecutable != "" && !filepath.IsAbs(cfg.CLIExecutable) {
			return nil, errors.New("invalid collaboration host runtime configuration")
		}
		return &cfg, nil
	}
	if !filepath.IsAbs(cfg.SandboxDirectory) || cfg.WorkspaceDirectory != "" {
		return nil, errors.New("invalid legacy collaboration directory configuration")
	}
	if cfg.Bundle == nil {
		if !filepath.IsAbs(cfg.Executable) || cfg.ExpectedRuntimeConfigEpoch != "" {
			return nil, errors.New("invalid collaboration executable configuration")
		}
	} else {
		epoch, err := strconv.ParseInt(cfg.ExpectedRuntimeConfigEpoch, 10, 64)
		if err != nil || epoch < 1 || strconv.FormatInt(epoch, 10) != cfg.ExpectedRuntimeConfigEpoch || cfg.Bundle.Validate() != nil ||
			cfg.Executable != "" || cfg.CLIExecutable != "" || len(cfg.Arguments) != 0 {
			return nil, errors.New("invalid collaboration provider bundle configuration")
		}
	}
	if cfg.CLIExecutable != "" && !filepath.IsAbs(cfg.CLIExecutable) {
		return nil, errors.New("collaboration CLI executable must be an explicit absolute path")
	}
	return &cfg, nil
}

type collaborationPoller struct {
	daemon      *Daemon
	config      *CollaborationRunnerConfig
	ledger      *collaborationrun.Ledger
	controller  collaborationrun.Launcher
	host        *collaborationrun.HostExecutor
	proofClient *collaborationrun.Client
	enabled     bool
	active      atomic.Bool
	blocked     sync.Map
	running     sync.Map
}

func (d *Daemon) initializeCollaborationPoller(ctx context.Context) (*collaborationPoller, int, error) {
	config := d.cfg.CollaborationRunner
	if config == nil {
		return nil, 0, nil
	}
	if d.managed != nil {
		d.managed.mu.Lock()
		deploymentID, installationID := "", ""
		if d.managed.store != nil {
			deploymentID, installationID = d.managed.store.DeploymentID, d.managed.store.InstallationID
		}
		homeDirectory := d.managed.homeDirectory
		d.managed.mu.Unlock()
		if !managedUUID.MatchString(deploymentID) || !managedUUID.MatchString(installationID) || !filepath.IsAbs(homeDirectory) {
			return nil, 0, collaborationrun.ErrUnsupported
		}
		parent := filepath.Join(homeDirectory, ".multica", "management", deploymentID, "collaboration-runs")
		expected := filepath.Join(parent, installationID)
		// Every profile/daemon for this installation must use the same lock and
		// nonce tombstones. A per-profile directory would allow a second fork.
		if filepath.Clean(config.LedgerDirectory) != expected {
			return nil, 0, errors.New("collaboration ledger must use the shared installation directory")
		}
		if err := managedEnsureDirectory(parent, true); err != nil {
			return nil, 0, err
		}
		if runtime.GOOS != "windows" {
			f, err := os.Open(filepath.Dir(parent))
			if err != nil {
				return nil, 0, err
			}
			if err := errors.Join(f.Sync(), f.Close()); err != nil {
				return nil, 0, err
			}
		}
	}
	ledger, err := collaborationrun.OpenLedger(config.LedgerDirectory)
	if err != nil {
		return nil, 0, err
	}
	retained, err := ledger.Retained()
	if err != nil {
		return nil, 0, err
	}
	proofClient, err := collaborationrun.NewClient(d.cfg.ServerBaseURL, "", d.client.client.Transport)
	if err != nil {
		return nil, 0, err
	}
	p := &collaborationPoller{daemon: d, config: config, ledger: ledger, controller: &collaborationrun.NamespaceController{Executable: config.Executable, CLIExecutable: config.CLIExecutable, Directory: config.SandboxDirectory, Arguments: config.Arguments, Bundle: config.Bundle}, proofClient: proofClient}
	var hostErr error
	if config.Host != nil {
		p.host, hostErr = d.hostCollaborationExecutor(config)
		p.controller = nil
	}
	reserved := 0
	for _, entry := range retained {
		if entry.Unconfirmed {
			reserved++
			p.blocked.Store(entry.Identity.RunID, true)
		}
	}
	available := false
	if config.Host != nil {
		available = hostErr == nil && p.host != nil && p.host.Probe(ctx) == nil
	} else {
		available = p.controller.Probe(ctx) == nil
	}
	if d.managed != nil && d.client.managed != nil && available {
		p.enabled = true
	} else {
		d.logger.Warn("collaboration runtime unavailable", "reason", "managed_identity_or_execution_capability_required")
	}
	return p, reserved, nil
}

func reserveCollaborationSlots(sem chan int, count int) {
	for range count {
		select {
		case <-sem:
		default:
			return
		}
	}
}

func (p *collaborationPoller) recoverProofs(ctx context.Context) (err error) {
	roundCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	recovery, err := p.ledger.BeginProofRecovery()
	if errors.Is(err, collaborationrun.ErrBusy) {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := recovery.Close(); closeErr != nil {
			err = errors.Join(err, collaborationrun.ErrUnconfirmed)
		}
	}()
	runner := &collaborationrun.Runner{Ledger: p.ledger, Client: p.proofClient}
	for roundCtx.Err() == nil {
		entry, err := recovery.Next()
		if err != nil {
			return err
		}
		if entry == nil || roundCtx.Err() != nil {
			break
		}
		request := protocol.CollaborationRunClaimRequest{DeploymentID: entry.Identity.DeploymentID, InstallationID: entry.Identity.InstallationID}
		if err := runner.Run(roundCtx, entry.Identity.RunID, request); errors.Is(err, collaborationrun.ErrUnconfirmed) {
			return err
		} else if err != nil && !errors.Is(err, collaborationrun.ErrBusy) {
			p.daemon.logger.Warn("collaboration proof remains pending", "run_id", entry.Identity.RunID)
		}
	}
	// Exhausting this round's budget postpones proof work without stopping the
	// machine poller. Caller cancellation still ends the poll loop promptly.
	return ctx.Err()
}

func (p *collaborationPoller) managedClient() (*collaborationrun.Client, protocol.CollaborationRunClaimRequest, error) {
	d := p.daemon
	var request protocol.CollaborationRunClaimRequest
	if d.managed == nil || d.client.managed == nil {
		return nil, request, collaborationrun.ErrUnsupported
	}
	d.managed.mu.Lock()
	if d.managed.store != nil {
		request.DeploymentID = d.managed.store.DeploymentID
		request.InstallationID = d.managed.store.InstallationID
	}
	d.managed.mu.Unlock()
	registry := d.client.managed
	registry.mu.RLock()
	scoped, err := registry.workspaceLocked(registry.runtimes[p.config.RuntimeID])
	var credential ManagedDaemonCredential
	if err == nil {
		credential = scoped.credential
	}
	registry.mu.RUnlock()
	if err != nil {
		return nil, request, err
	}
	request.ProtocolVersion = protocol.CollaborationRuntimeProtocolVersion
	request.RuntimeID = p.config.RuntimeID
	request.BindingID = credential.BindingID
	request.BindingEpoch = credential.BindingEpoch
	request.Containment = protocol.CollaborationContainmentLinuxPIDNamespace
	if p.config.Host != nil {
		request.ProtocolVersion = protocol.CollaborationHostRuntimeProtocolVersion
		request.Containment = protocol.CollaborationContainmentHostProcessSupervision
	}
	client, err := collaborationrun.NewClient(d.cfg.ServerBaseURL, credential.DaemonToken, d.client.client.Transport)
	return client, request, err
}

// dispatch owns an existing machine slot before the claim. An unknown run keeps
// that slot and the update barrier; only a known, reaped scope may release it.
func (p *collaborationPoller) dispatch(pollCtx, runCtx context.Context, slot int, sem chan int, wakeup chan struct{}, wg *sync.WaitGroup) bool {
	if !p.enabled || !p.active.CompareAndSwap(false, true) {
		return false
	}
	dispatched := false
	defer func() {
		if !dispatched {
			p.active.Store(false)
		}
	}()
	client, request, err := p.managedClient()
	if err != nil {
		return false
	}
	runtimeConfigEpoch := ""
	if p.host != nil {
		entry, found := p.daemon.agents()[p.host.Config.Provider]
		version, valid := agent.NormalizeHostProviderVersion(p.host.Config.Provider, p.daemon.agentVersion(p.host.Config.Provider))
		if !found || !valid || entry.Path != p.host.Config.Executable || version != p.host.Config.ProviderVersion {
			return false
		}
		capabilities, err := p.host.Config.Capabilities()
		if err != nil {
			return false
		}
		registration, err := client.EnsureCapabilities(pollCtx, request, p.config.ExpectedRuntimeConfigEpoch, capabilities)
		if err != nil {
			return false
		}
		runtimeConfigEpoch = registration.RuntimeConfigEpoch
		request.PreparationVersion = protocol.CollaborationHostPreparationVersion
		request.CapabilityDigest, err = protocol.CollaborationRuntimeCapabilityDigest(capabilities)
		if err != nil {
			return false
		}
	} else if p.config.Bundle != nil {
		capabilities, err := p.config.Bundle.Capabilities()
		if err != nil {
			return false
		}
		registration, err := client.EnsureCapabilities(pollCtx, request, p.config.ExpectedRuntimeConfigEpoch, capabilities)
		if err != nil {
			return false
		}
		runtimeConfigEpoch = registration.RuntimeConfigEpoch
		request.PreparationVersion = protocol.CollaborationPreparationVersion
		request.BundleDigest = capabilities.Provider.BundleDigest
	}
	candidates, err := client.List(pollCtx, request.RuntimeID)
	if err != nil {
		return false
	}
	runID := ""
	for _, id := range candidates.RunIDs {
		_, busy := p.running.Load(id)
		if _, blocked := p.blocked.Load(id); !blocked && !busy {
			runID = id
			break
		}
	}
	if runID == "" && p.host != nil {
		retained, err := p.ledger.Retained()
		if err != nil {
			return false
		}
		for _, entry := range retained {
			_, busy := p.running.Load(entry.Identity.RunID)
			if entry.Waiting && entry.RuntimeID == request.RuntimeID && entry.Identity.DeploymentID == request.DeploymentID && entry.Identity.InstallationID == request.InstallationID && !busy {
				runID = entry.Identity.RunID
				break
			}
		}
	}
	if runID == "" {
		return false
	}
	dispatched = true
	p.running.Store(runID, true)
	wg.Add(1)
	p.daemon.activeTasks.Add(1)
	go func() {
		defer wg.Done()
		runner := &collaborationrun.Runner{Ledger: p.ledger, Client: client, Controller: p.controller, Host: p.host, RuntimeConfigEpoch: runtimeConfigEpoch}
		err := runner.Run(runCtx, runID, request)
		if errors.Is(err, collaborationrun.ErrUnconfirmed) {
			p.blocked.Store(runID, true)
			p.daemon.logger.Warn("collaboration run retains capacity until termination is proven", "run_id", runID)
		} else {
			p.daemon.activeTasks.Add(-1)
			sem <- slot
			if err != nil {
				p.daemon.logger.Warn("collaboration run ended with pending recording or proof", "run_id", runID)
			}
		}
		p.running.Delete(runID)
		if p.host == nil {
			p.active.Store(false)
		}
		signalPollerWakeup(wakeup)
	}()
	if p.host != nil {
		p.active.Store(false)
	}
	return true
}

func (d *Daemon) hostCollaborationExecutor(config *CollaborationRunnerConfig) (*collaborationrun.HostExecutor, error) {
	if config.Host == nil || config.Host.Validate() != nil {
		return nil, collaborationrun.ErrUnsupported
	}
	host := *config.Host
	host.ProviderVersion, _ = agent.NormalizeHostProviderVersion(host.Provider, host.ProviderVersion)
	if _, custom := d.customProfileLaunchForRuntime(config.RuntimeID); custom {
		return nil, collaborationrun.ErrUnsupported
	}
	entry, found := d.agents()[host.Provider]
	version, valid := agent.NormalizeHostProviderVersion(host.Provider, d.agentVersion(host.Provider))
	if !found || !valid || !filepath.IsAbs(entry.Path) || version != host.ProviderVersion {
		return nil, collaborationrun.ErrUnsupported
	}
	host.Executable, host.DefaultModel = entry.Path, entry.Model
	switch host.Provider {
	case "claude":
		host.ExtraArgs = append([]string(nil), d.cfg.ClaudeArgs...)
	case "codex":
		host.ExtraArgs = append([]string(nil), d.cfg.CodexArgs...)
	}
	cliExecutable := config.CLIExecutable
	if cliExecutable == "" {
		var err error
		cliExecutable, err = resolveSelfExecutable()
		if err != nil {
			return nil, err
		}
	}
	h := &collaborationrun.HostExecutor{Config: &host, Directory: config.WorkspaceDirectory, CLIExecutable: cliExecutable, Logger: d.logger,
		Options: agent.ExecOptions{Timeout: d.cfg.AgentTimeout, SemanticInactivityTimeout: d.cfg.CodexSemanticInactivityTimeout,
			FirstTurnNoProgressTimeout: d.cfg.CodexFirstTurnNoProgressTimeout, HandshakeTimeout: d.cfg.CodexHandshakeTimeout,
			ThreadHandshakeTimeout: d.cfg.CodexThreadHandshakeTimeout}}
	if host.Provider == "opencode" {
		h.Options.IdleWatchdogTimeout = d.cfg.OpenCodeIdleWatchdog
	}
	h.Watchdog = func(ctx context.Context, activity *collaborationrun.HostActivity, cancel context.CancelFunc, messages <-chan agent.Message) {
		window := d.cfg.AgentIdleWatchdog
		if window <= 0 {
			return
		}
		if override := h.Options.IdleWatchdogTimeout; override > 0 && override < window {
			window = override
		}
		d.runIdleWatchdog(ctx, window, d.cfg.AgentToolWatchdog, &activity.LastActivity, &activity.InFlightTools, &activity.WatchdogFired,
			&activity.WatchdogThreshold, cancel, messages, d.logger, config.RuntimeID)
	}
	return h, nil
}
