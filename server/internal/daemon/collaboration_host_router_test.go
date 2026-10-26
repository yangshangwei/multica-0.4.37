package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Playwright owns this native subprocess, its ordinary fake Claude executable,
// and its server-issued managed credential. Only the credential crosses stdin.
const collaborationHostRouterHarnessArg = "__multica_collaboration_host_router_poller"

type collaborationHostRouterInput struct {
	collaborationRouterHarnessInput
	RuntimeConfigEpoch string
}

func init() {
	if len(os.Args) == 2 && os.Args[1] == collaborationHostRouterHarnessArg {
		if err := runCollaborationHostRouterHarness(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// Keep observations content-free. The real transport still sends/receives the
// original request and response; no fixture supplies permits or terminal facts.
type collaborationHostHTTPFact struct {
	Operation       string `json:"operation"`
	RunID           string `json:"run_id"`
	ProtocolVersion int    `json:"protocol_version"`
	Containment     string `json:"containment"`
	AttemptSeq      string `json:"attempt_seq,omitempty"`
	LaunchHash      string `json:"launch_hash,omitempty"`
	Kind            string `json:"kind,omitempty"`
	State           string `json:"state,omitempty"`
	Status          int    `json:"status"`
}

type collaborationHostTransport struct {
	capacity *collaborationCapacityObserver
	mu       sync.Mutex
	facts    []collaborationHostHTTPFact
}

func (o *collaborationHostTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	fact := collaborationHostHTTPFact{Operation: filepath.Base(r.URL.Path)}
	observe := r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/collaboration/runtime/runs/") &&
		(fact.Operation == "claim" || fact.Operation == "permit" || fact.Operation == "termination" || fact.Operation == "proof")
	if observe && r.GetBody != nil {
		body, err := r.GetBody()
		if err != nil {
			return nil, fmt.Errorf("host fixture could not inspect its request")
		}
		var request struct {
			ProtocolVersion int                               `json:"protocol_version"`
			Containment     string                            `json:"containment"`
			Identity        protocol.CollaborationRunIdentity `json:"identity"`
			AttemptSeq      string                            `json:"attempt_seq"`
			AttemptNonce    string                            `json:"attempt_nonce"`
			Kind            string                            `json:"kind"`
			State           string                            `json:"state"`
		}
		err = json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&request)
		_ = body.Close()
		if err != nil {
			return nil, fmt.Errorf("host fixture request was not bounded JSON")
		}
		fact.RunID = strings.Split(strings.TrimPrefix(r.URL.Path, "/api/collaboration/runtime/runs/"), "/")[0]
		fact.ProtocolVersion, fact.Containment = request.ProtocolVersion, request.Containment
		fact.AttemptSeq, fact.Kind, fact.State = request.AttemptSeq, request.Kind, request.State
		if request.AttemptNonce != "" {
			sum := sha256.Sum256([]byte(request.AttemptNonce))
			fact.LaunchHash = hex.EncodeToString(sum[:])
		}
	}
	response, err := o.capacity.RoundTrip(r)
	if observe {
		if response != nil {
			fact.Status = response.StatusCode
		}
		o.mu.Lock()
		o.facts = append(o.facts, fact)
		o.mu.Unlock()
	}
	return response, err
}

func runCollaborationHostRouterHarness() error {
	var input collaborationHostRouterInput
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 64<<10)).Decode(&input); err != nil {
		return fmt.Errorf("host fixture input was not valid JSON")
	}
	for _, id := range []string{input.DeploymentID, input.InstallationID, input.WorkspaceID, input.RuntimeID, input.DaemonID} {
		if !managedUUID.MatchString(id) {
			return fmt.Errorf("host fixture requires issued runtime identities")
		}
	}
	for _, path := range []string{input.Directory, input.Executable, input.CLIExecutable} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("host fixture requires explicit private directories and binaries")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	parent := filepath.Join(input.Directory, ".multica", "management", input.DeploymentID)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return fmt.Errorf("host fixture could not prepare its private management directory")
	}
	config := &CollaborationRunnerConfig{
		RuntimeID: input.RuntimeID, CLIExecutable: input.CLIExecutable,
		LedgerDirectory:    filepath.Join(parent, "collaboration-runs", input.InstallationID),
		WorkspaceDirectory: filepath.Join(input.Directory, "collaboration-work"), ExpectedRuntimeConfigEpoch: input.RuntimeConfigEpoch,
		Host: &collaborationrun.HostConfig{Provider: "claude", ProviderVersion: "fixture-1", ProviderMode: "local", AccountSource: "test-owned-empty-account",
			Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20,
				WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 65536, TimeoutSeconds: 600}},
	}
	d := New(Config{ServerBaseURL: input.ServerURL, DaemonID: input.DaemonID, CLIVersion: "0.4.40-test", WorkspacesRoot: filepath.Join(input.Directory, "ordinary-work"),
		Agents: map[string]AgentEntry{"claude": {Path: input.Executable}}, MaxConcurrentTasks: 1, PollInterval: 50 * time.Millisecond,
		AgentTimeout: 3 * time.Minute, CollaborationRunner: config}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.setAgentVersion("claude", "fixture-1")
	d.managed = &managedDaemonLifecycle{homeDirectory: input.Directory, store: &ManagedCredentialStore{DeploymentID: input.DeploymentID, InstallationID: input.InstallationID}}
	sem := newTaskSlotSemaphore(d.cfg.MaxConcurrentTasks)
	transport := &collaborationHostTransport{capacity: &collaborationCapacityObserver{daemon: d, slots: sem}}
	d.client.client.Transport = transport
	d.client.enableManagedTransport()
	if err := d.client.installManagedCredential(input.WorkspaceID, input.Credential); err != nil {
		return fmt.Errorf("host fixture could not install its issued credential")
	}
	registered, err := d.client.Register(ctx, map[string]any{"workspace_id": input.WorkspaceID, "daemon_id": input.DaemonID,
		"runtimes": []map[string]string{{"name": "Test-owned host Claude", "type": "claude", "version": "fixture-1", "status": "online"}}})
	if err != nil || len(registered.Runtimes) != 1 || registered.Runtimes[0].ID != input.RuntimeID {
		return fmt.Errorf("host fixture could not register its exact ordinary runtime")
	}
	d.workspaces[input.WorkspaceID] = &workspaceState{workspaceID: input.WorkspaceID, runtimeIDs: []string{input.RuntimeID}}
	d.collaboration, _, err = d.initializeCollaborationPoller(ctx)
	if err != nil || d.collaboration == nil || !d.collaboration.enabled || d.collaboration.host == nil || d.collaboration.controller != nil {
		return fmt.Errorf("host fixture ordinary runtime was not admitted")
	}
	client, identity, err := d.collaboration.managedClient()
	if err != nil {
		return fmt.Errorf("host fixture has no managed collaboration identity")
	}
	capabilities, err := d.collaboration.host.Config.Capabilities()
	if err != nil {
		return fmt.Errorf("host fixture capabilities are unavailable")
	}
	// This is the same registration used by dispatch. Consent must be captured
	// only after it has committed the new host configuration epoch.
	registration, err := client.EnsureCapabilities(ctx, identity, input.RuntimeConfigEpoch, capabilities)
	if err != nil {
		return fmt.Errorf("host fixture capability registration was refused: %w", err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"ready": true, "protocol_version": identity.ProtocolVersion,
		"execution_mode": identity.Containment, "preparation_version": capabilities.PreparationVersion,
		"runtime_config_epoch": registration.RuntimeConfigEpoch, "max_concurrent_tasks": 1, "os": runtime.GOOS, "arch": runtime.GOARCH}); err != nil {
		return err
	}
	if os.Getenv("COLLABORATION_HOST_READY_ONLY") == "1" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"claims": 0, "invalid_capacity": false, "available_slots": 1, "active_tasks": 0, "http": []collaborationHostHTTPFact{}})
	}
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	var work sync.WaitGroup
	d.runBatchPoller(ctx, ctx, sem, make(chan struct{}, 1), &work)
	work.Wait()
	transport.mu.Lock()
	facts := append([]collaborationHostHTTPFact(nil), transport.facts...)
	transport.mu.Unlock()
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"claims": transport.capacity.claims.Load(), "invalid_capacity": transport.capacity.bad.Load(),
		"available_slots": len(sem), "active_tasks": d.activeTasks.Load(), "http": facts})
}
