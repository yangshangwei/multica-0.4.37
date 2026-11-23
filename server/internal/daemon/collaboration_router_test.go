package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The real-router test owns this subprocess and supplies a real server-issued
// binding over stdin. No test endpoint substitutes for admission or proof.
const collaborationRouterHarnessArg = "__multica_collaboration_router_poller"

type collaborationRouterHarnessInput struct {
	ServerURL      string
	WorkspaceID    string
	RuntimeID      string
	DaemonID       string
	DeploymentID   string
	InstallationID string
	Directory      string
	Executable     string
	CLIExecutable  string
	Arguments      []string
	Credential     ManagedDaemonCredential
}

func init() {
	if len(os.Args) == 2 && os.Args[1] == collaborationRouterHarnessArg {
		if err := runCollaborationRouterHarness(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

type collaborationCapacityObserver struct {
	daemon *Daemon
	slots  chan int
	claims atomic.Int64
	bad    atomic.Bool
}

func (o *collaborationCapacityObserver) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasPrefix(r.URL.Path, "/api/collaboration/runtime/runs/") && strings.HasSuffix(r.URL.Path, "/claim") {
		o.claims.Add(1)
		if len(o.slots) != 0 || o.daemon.activeTasks.Load() != 1 {
			o.bad.Store(true)
		}
	}
	return http.DefaultTransport.RoundTrip(r)
}

func runCollaborationRouterHarness() error {
	var input collaborationRouterHarnessInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	parent := filepath.Join(input.Directory, ".multica", "management", input.DeploymentID)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	d := New(Config{ServerBaseURL: input.ServerURL, DaemonID: input.DaemonID, WorkspacesRoot: filepath.Join(input.Directory, "workspaces"),
		MaxConcurrentTasks: 1, PollInterval: 50 * time.Millisecond, CollaborationRunner: &CollaborationRunnerConfig{
			RuntimeID: input.RuntimeID, Executable: input.Executable, CLIExecutable: input.CLIExecutable, Arguments: input.Arguments,
			LedgerDirectory: filepath.Join(parent, "collaboration-runs", input.InstallationID), SandboxDirectory: filepath.Join(input.Directory, "sandboxes"),
		}}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	d.managed = &managedDaemonLifecycle{homeDirectory: input.Directory, store: &ManagedCredentialStore{DeploymentID: input.DeploymentID, InstallationID: input.InstallationID}}
	sem := newTaskSlotSemaphore(d.cfg.MaxConcurrentTasks)
	observer := &collaborationCapacityObserver{daemon: d, slots: sem}
	d.client.client.Transport = observer
	d.client.enableManagedTransport()
	if err := d.client.installManagedCredential(input.WorkspaceID, input.Credential); err != nil {
		return err
	}
	registered, err := d.client.Register(ctx, map[string]any{"workspace_id": input.WorkspaceID, "daemon_id": input.DaemonID,
		"runtimes": []map[string]string{{"name": "Test-owned Linux provider", "type": "codex", "status": "online"}}})
	if err != nil {
		return fmt.Errorf("register test runtime: %w", err)
	}
	if len(registered.Runtimes) != 1 || registered.Runtimes[0].ID != input.RuntimeID {
		return fmt.Errorf("real registration changed the intended runtime")
	}
	d.workspaces[input.WorkspaceID] = &workspaceState{workspaceID: input.WorkspaceID, runtimeIDs: []string{input.RuntimeID}}
	d.collaboration, _, err = d.initializeCollaborationPoller(ctx)
	if err != nil {
		return err
	}
	if d.collaboration == nil || !d.collaboration.enabled {
		return fmt.Errorf("native collaboration poller was not admitted")
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"ready": true, "max_concurrent_tasks": d.cfg.MaxConcurrentTasks}); err != nil {
		return err
	}
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	var work sync.WaitGroup
	d.runBatchPoller(ctx, ctx, sem, make(chan struct{}, 1), &work)
	work.Wait()
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"claims": observer.claims.Load(), "invalid_capacity": observer.bad.Load(),
		"available_slots": len(sem), "active_tasks": d.activeTasks.Load()})
}
