package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// CollaborationRunnerConfig is an explicit single-runtime opt-in. It never
// reuses provider discovery, private account directories or ordinary task args.
type CollaborationRunnerConfig struct {
	RuntimeID        string   `json:"runtime_id"`
	Executable       string   `json:"executable"`
	Arguments        []string `json:"arguments"`
	LedgerDirectory  string   `json:"ledger_directory"`
	SandboxDirectory string   `json:"sandbox_directory"`
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
	if d.Decode(&cfg) != nil || d.Decode(new(any)) != io.EOF || !managedUUID.MatchString(cfg.RuntimeID) || !filepath.IsAbs(cfg.Executable) || !filepath.IsAbs(cfg.LedgerDirectory) || !filepath.IsAbs(cfg.SandboxDirectory) {
		return nil, errors.New("invalid collaboration runner configuration")
	}
	return &cfg, nil
}

type collaborationPoller struct {
	daemon      *Daemon
	config      *CollaborationRunnerConfig
	ledger      *collaborationrun.Ledger
	controller  *collaborationrun.NamespaceController
	proofClient *collaborationrun.Client
	enabled     bool
	active      atomic.Bool
	blocked     sync.Map
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
		f, err := os.Open(filepath.Dir(parent))
		if err != nil {
			return nil, 0, err
		}
		if err := errors.Join(f.Sync(), f.Close()); err != nil {
			return nil, 0, err
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
	p := &collaborationPoller{daemon: d, config: config, ledger: ledger, controller: &collaborationrun.NamespaceController{Executable: config.Executable, Directory: config.SandboxDirectory, Arguments: config.Arguments}, proofClient: proofClient}
	reserved := 0
	for _, entry := range retained {
		if entry.Unconfirmed {
			reserved++
			p.blocked.Store(entry.Identity.RunID, true)
		}
	}
	if d.managed != nil && d.client.managed != nil && p.controller.Probe(ctx) == nil {
		p.enabled = true
	} else {
		d.logger.Warn("collaboration runtime unavailable", "reason", "managed_identity_or_containment_required")
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

func (p *collaborationPoller) recoverProofs(ctx context.Context) error {
	entries, err := p.ledger.Retained()
	if err != nil {
		return err
	}
	recovered := 0
	for _, entry := range entries {
		if !entry.CanProve {
			continue
		}
		if recovered >= 8 {
			break
		}
		recovered++
		runner := &collaborationrun.Runner{Ledger: p.ledger, Client: p.proofClient}
		request := protocol.CollaborationRunClaimRequest{DeploymentID: entry.Identity.DeploymentID, InstallationID: entry.Identity.InstallationID}
		if err := runner.Run(ctx, entry.Identity.RunID, request); err != nil && !errors.Is(err, collaborationrun.ErrBusy) {
			p.daemon.logger.Warn("collaboration proof remains pending", "run_id", entry.Identity.RunID)
		}
	}
	return nil
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
	candidates, err := client.List(pollCtx, request.RuntimeID)
	if err != nil {
		return false
	}
	runID := ""
	for _, id := range candidates.RunIDs {
		if _, blocked := p.blocked.Load(id); !blocked {
			runID = id
			break
		}
	}
	if runID == "" {
		return false
	}
	dispatched = true
	wg.Add(1)
	p.daemon.activeTasks.Add(1)
	go func() {
		defer wg.Done()
		runner := &collaborationrun.Runner{Ledger: p.ledger, Client: client, Controller: p.controller}
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
		p.active.Store(false)
		signalPollerWakeup(wakeup)
	}()
	return true
}
