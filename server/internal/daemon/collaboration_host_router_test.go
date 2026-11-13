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
	AgentTimeoutMS     json.RawMessage `json:"agent_timeout_ms,omitempty"`
	HoldReceiptACKs    bool            `json:"hold_receipt_acks,omitempty"`
}

func (input collaborationHostRouterInput) executionTimeout() (time.Duration, error) {
	if len(input.AgentTimeoutMS) == 0 {
		return 3 * time.Minute, nil
	}
	var milliseconds int64
	if err := json.Unmarshal(input.AgentTimeoutMS, &milliseconds); err != nil || milliseconds < 1000 || milliseconds > 180000 {
		return 0, fmt.Errorf("host fixture agent_timeout_ms must be an integer from 1000 to 180000")
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
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
	Operation           string `json:"operation"`
	RunID               string `json:"run_id"`
	ProtocolVersion     int    `json:"protocol_version"`
	Containment         string `json:"containment"`
	AttemptSeq          string `json:"attempt_seq,omitempty"`
	LaunchHash          string `json:"launch_hash,omitempty"`
	Kind                string `json:"kind,omitempty"`
	State               string `json:"state,omitempty"`
	Status              int    `json:"status"`
	EventID             string `json:"event_id,omitempty"`
	ReceiptID           string `json:"receipt_id,omitempty"`
	RequestSHA256       string `json:"request_sha256,omitempty"`
	ResponseSHA256      string `json:"response_sha256,omitempty"`
	AcknowledgementHeld bool   `json:"acknowledgement_held,omitempty"`
}

type collaborationHostTransport struct {
	capacity        *collaborationCapacityObserver
	mu              sync.Mutex
	facts           []collaborationHostHTTPFact
	holdReceiptACKs bool
}

func (o *collaborationHostTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	fact := collaborationHostHTTPFact{Operation: filepath.Base(r.URL.Path)}
	observe := r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/collaboration/runtime/runs/") &&
		(fact.Operation == "claim" || fact.Operation == "permit" || fact.Operation == "termination" || fact.Operation == "proof" || fact.Operation == "outcome")
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
			EventID         string                            `json:"event_id"`
			ReceiptID       string                            `json:"receipt_id"`
		}
		wire, err := io.ReadAll(io.LimitReader(body, protocol.CollaborationOutcomeJSONLimit+1))
		_ = body.Close()
		if err != nil || len(wire) > protocol.CollaborationOutcomeJSONLimit || json.Unmarshal(wire, &request) != nil {
			return nil, fmt.Errorf("host fixture request was not bounded JSON")
		}
		digest := sha256.Sum256(wire)
		fact.RequestSHA256 = hex.EncodeToString(digest[:])
		fact.RunID = strings.Split(strings.TrimPrefix(r.URL.Path, "/api/collaboration/runtime/runs/"), "/")[0]
		fact.ProtocolVersion, fact.Containment = request.ProtocolVersion, request.Containment
		fact.AttemptSeq, fact.Kind, fact.State = request.AttemptSeq, request.Kind, request.State
		fact.EventID, fact.ReceiptID = request.EventID, request.ReceiptID
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
		if o.holdReceiptACKs && (fact.Operation == "outcome" || fact.Operation == "proof") &&
			err == nil && response != nil && response.StatusCode >= 200 && response.StatusCode < 300 {
			// The real API answered after its transaction. Hold only that ACK,
			// on every replay until this poller exits; never forge a result.
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
			_ = response.Body.Close()
			response = nil
			if readErr != nil {
				err = fmt.Errorf("host fixture could not inspect its committed receipt")
			} else {
				digest := sha256.Sum256(body)
				fact.ResponseSHA256 = hex.EncodeToString(digest[:])
				fact.AcknowledgementHeld = true
				err = fmt.Errorf("host fixture held a committed receipt acknowledgement")
			}
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
	agentTimeout, err := input.executionTimeout()
	if err != nil {
		return err
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
		AgentTimeout: agentTimeout, CollaborationRunner: config}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.setAgentVersion("claude", "fixture-1")
	d.managed = &managedDaemonLifecycle{homeDirectory: input.Directory, store: &ManagedCredentialStore{DeploymentID: input.DeploymentID, InstallationID: input.InstallationID}}
	sem := newTaskSlotSemaphore(d.cfg.MaxConcurrentTasks)
	transport := &collaborationHostTransport{capacity: &collaborationCapacityObserver{daemon: d, slots: sem}, holdReceiptACKs: input.HoldReceiptACKs}
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
		"runtime_config_epoch": registration.RuntimeConfigEpoch, "max_concurrent_tasks": 1, "os": runtime.GOOS, "arch": runtime.GOARCH,
		"agent_timeout_ms": agentTimeout.Milliseconds(), "hold_receipt_acks": input.HoldReceiptACKs}); err != nil {
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
