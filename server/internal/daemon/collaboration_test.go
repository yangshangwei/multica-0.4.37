package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const collaborationPollerFakeArg = "__multica_collaboration_poller_fake"

func init() {
	if len(os.Args) != 2 {
		return
	}
	if os.Args[1] == collaborationrun.NamespaceHelperArg {
		os.Exit(collaborationrun.NamespaceHelperMain())
	}
	if os.Args[1] == collaborationPollerFakeArg {
		var input collaborationrun.ProviderInput
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil || len(input.Inputs) != 1 {
			os.Exit(20)
		}
		if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"outcome": protocol.CollaborationBusinessOutcomeSuccess, "body": input.Inputs[0].Body}); err != nil {
			os.Exit(21)
		}
		os.Exit(0)
	}
}

func TestCollaborationRunnerConfigRequiresExplicitLocalPaths(t *testing.T) {
	if cfg, err := parseCollaborationRunnerConfig(""); err != nil || cfg != nil {
		t.Fatal("default must stay disabled")
	}
	if _, err := parseCollaborationRunnerConfig(`{"runtime_id":"00000000-0000-4000-8000-000000000001","executable":"ambient-command","ledger_directory":"/private/ledger","sandbox_directory":"/private/sandbox"}`); err == nil {
		t.Fatal("accepted provider PATH lookup")
	}
	if _, err := parseCollaborationRunnerConfig(`{"runtime_id":"00000000-0000-4000-8000-000000000001","executable":"/private/fake","ledger_directory":"/private/ledger","sandbox_directory":"/private/sandbox","token":"ordinary-token"}`); err == nil {
		t.Fatal("config accepted an arbitrary credential")
	}
}

func TestCollaborationRestartReservesUnknownMachineCapacity(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("POSIX ledger")
	}
	root := t.TempDir()
	ledgerDir := filepath.Join(root, "ledger")
	ledger, err := collaborationrun.OpenLedger(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}
	id := protocol.CollaborationRunIdentity{DeploymentID: "00000000-0000-4000-8000-000000000001", InstallationID: "00000000-0000-4000-8000-000000000002", IssueID: "00000000-0000-4000-8000-000000000003", SessionID: "00000000-0000-4000-8000-000000000004", TargetID: "00000000-0000-4000-8000-000000000005", ChannelID: "00000000-0000-4000-8000-000000000006", BatchID: "00000000-0000-4000-8000-000000000007", GroupID: "00000000-0000-4000-8000-000000000008", RunID: "00000000-0000-4000-8000-000000000009", LaunchNonce: "00000000-0000-4000-8000-000000000010", Fence: "1", ClaimGeneration: "1"}
	local, err := ledger.Lock(id.DeploymentID, id.InstallationID, id.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Register(id, "private-termination"); err != nil {
		t.Fatal(err)
	}
	if err := local.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); err != nil {
		t.Fatal(err)
	}
	if err := local.Authorize(); err != nil {
		t.Fatal(err)
	}
	if err := local.Burn(); err != nil {
		t.Fatal(err)
	}
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	d := &Daemon{cfg: Config{ServerBaseURL: "http://127.0.0.1:1", MaxConcurrentTasks: 1, CollaborationRunner: &CollaborationRunnerConfig{RuntimeID: id.TargetID, Executable: "/explicit/missing/fake", LedgerDirectory: ledgerDir, SandboxDirectory: filepath.Join(root, "sandboxes")}}, client: NewClient("http://127.0.0.1:1"), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	poller, reserved, err := d.initializeCollaborationPoller(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if poller == nil || reserved != 1 || poller.enabled {
		t.Fatalf("lost unknown slot or allowed legacy launch: reserved=%d enabled=%v", reserved, poller.enabled)
	}
	sem := newTaskSlotSemaphore(1)
	reserveCollaborationSlots(sem, reserved)
	select {
	case <-sem:
		t.Fatal("ordinary work acquired unknown collaboration capacity")
	default:
	}
}

func TestManagedCollaborationProfilesShareOneLedgerDirectory(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("POSIX ledger")
	}
	homeDirectory := t.TempDir()
	deploymentID := "00000000-0000-4000-8000-000000000001"
	installationID := "00000000-0000-4000-8000-000000000002"
	parent := filepath.Join(homeDirectory, ".multica", "management", deploymentID)
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := &CollaborationRunnerConfig{RuntimeID: deploymentID, Executable: "/explicit/missing/fake", LedgerDirectory: filepath.Join(homeDirectory, "per-profile-ledger"), SandboxDirectory: filepath.Join(homeDirectory, "sandboxes")}
	d := &Daemon{cfg: Config{ServerBaseURL: "http://127.0.0.1:1", CollaborationRunner: cfg}, managed: &managedDaemonLifecycle{homeDirectory: homeDirectory, store: &ManagedCredentialStore{DeploymentID: deploymentID, InstallationID: installationID}}, client: NewClient("http://127.0.0.1:1"), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if _, _, err := d.initializeCollaborationPoller(context.Background()); err == nil {
		t.Fatal("per-profile ledger could admit the same nonce twice")
	}
	cfg.LedgerDirectory = filepath.Join(parent, "collaboration-runs", installationID)
	if _, reserved, err := d.initializeCollaborationPoller(context.Background()); err != nil || reserved != 0 {
		t.Fatalf("shared installation ledger: reserved=%d err=%v", reserved, err)
	}
}

func collaborationManagedPollerFixture(t *testing.T, serverURL, workspaceID, runtimeID string) (*Daemon, protocol.CollaborationRunClaimRequest) {
	t.Helper()
	root := t.TempDir()
	deploymentID, installationID := uuid.NewString(), uuid.NewString()
	parent := filepath.Join(root, ".multica", "management", deploymentID)
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	config := &CollaborationRunnerConfig{RuntimeID: runtimeID, Executable: filepath.Join(root, "fake-provider"),
		LedgerDirectory: filepath.Join(parent, "collaboration-runs", installationID), SandboxDirectory: filepath.Join(root, "sandboxes")}
	d := New(Config{ServerBaseURL: serverURL, WorkspacesRoot: filepath.Join(root, "workspaces"),
		MaxConcurrentTasks: 1, PollInterval: time.Hour, CollaborationRunner: config}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.managed = &managedDaemonLifecycle{homeDirectory: root, store: &ManagedCredentialStore{DeploymentID: deploymentID, InstallationID: installationID}}
	d.client.SetToken("mul_discovery_only")
	d.client.enableManagedTransport()
	credential := ManagedDaemonCredential{BindingID: uuid.NewString(), BindingEpoch: "9", CapabilityVersion: "1", DaemonToken: "mdt_poller_fixture",
		ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), PrincipalUserID: uuid.NewString(), AuthVersion: "1"}
	if err := d.client.installManagedCredential(workspaceID, credential); err != nil {
		t.Fatal(err)
	}
	request := protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion,
		DeploymentID: deploymentID, InstallationID: installationID, RuntimeID: runtimeID,
		BindingID: credential.BindingID, BindingEpoch: credential.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	return d, request
}

func TestCollaborationManagedPollerRequiresRegisteredRuntime(t *testing.T) {
	workspaceID, runtimeID := uuid.NewString(), uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mdt_poller_fixture" {
			t.Error("collaboration reused an ambient credential")
		}
		switch r.URL.Path {
		case "/api/daemon/register":
			_ = json.NewEncoder(w).Encode(RegisterResponse{Runtimes: []Runtime{{ID: runtimeID, Provider: "fake"}}})
		case "/api/collaboration/runtime/runs":
			if r.URL.Query().Get("runtime_id") != runtimeID {
				t.Error("collaboration listed a different managed runtime")
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunListResponse{RunIDs: []string{}})
		default:
			t.Error("unexpected managed request")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	d, expected := collaborationManagedPollerFixture(t, server.URL, workspaceID, runtimeID)
	poller := &collaborationPoller{daemon: d, config: d.cfg.CollaborationRunner}
	if _, _, err := poller.managedClient(); err == nil {
		t.Fatal("an unregistered runtime acquired collaboration authority")
	}
	if _, err := d.client.Register(t.Context(), map[string]any{"workspace_id": workspaceID}); err != nil {
		t.Fatal(err)
	}
	client, request, err := poller.managedClient()
	if err != nil || request != expected {
		t.Fatalf("registration did not carry the installation/binding/runtime identity: %v", err)
	}
	if _, err := client.List(t.Context(), request.RuntimeID); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationBatchPollerSlotsAndRunnerLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires real Linux kernel containment")
	}
	workspaceID, runtimeID, runID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	sem := newTaskSlotSemaphore(1)
	heldSlot := <-sem
	listed := make(chan struct{}, 1)
	outcomes := make(chan protocol.CollaborationRunOutcome, 1)
	proofs := make(chan protocol.CollaborationRunProof, 1)
	proofRelease := make(chan struct{})
	var proofOnce sync.Once
	openProof := func() { proofOnce.Do(func() { close(proofRelease) }) }
	var expected protocol.CollaborationRunClaimRequest
	var claim protocol.CollaborationRunClaimResponse
	var attempt protocol.CollaborationRunPermitRequest
	var d *Daemon
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/daemon/register":
			_ = json.NewEncoder(w).Encode(RegisterResponse{Runtimes: []Runtime{{ID: runtimeID, Provider: "fake"}}})
		case "/api/collaboration/runtime/runs":
			if r.Header.Get("Authorization") != "Bearer mdt_poller_fixture" || r.URL.Query().Get("runtime_id") != runtimeID || len(sem) != 0 {
				t.Error("candidate polling lost its registered credential or machine slot")
			}
			select {
			case listed <- struct{}{}:
			default:
				t.Error("poller listed another candidate without released capacity")
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunListResponse{RunIDs: []string{runID}})
		case "/api/collaboration/runtime/runs/" + runID + "/claim":
			var request protocol.CollaborationRunClaimRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request != expected || r.Header.Get("Authorization") != "Bearer mdt_poller_fixture" {
				t.Error("Runner claim lost its managed registration identity")
			}
			if len(sem) != 0 || d.activeTasks.Load() != 1 {
				t.Error("Runner claimed without owning machine/update capacity")
			}
			_ = json.NewEncoder(w).Encode(claim)
		case "/api/collaboration/runtime/runs/" + runID + "/permit":
			if err := json.NewDecoder(r.Body).Decode(&attempt); err != nil || attempt.Identity != claim.Identity || r.Header.Get("Authorization") != "Bearer mdt_poller_fixture" {
				t.Error("Runner permit changed the admitted identity")
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunPermitResponse{Identity: attempt.Identity, AttemptSeq: attempt.AttemptSeq, AttemptNonce: attempt.AttemptNonce, BusinessToken: "business-only"})
		case "/api/collaboration/runtime/runs/" + runID + "/stop-intent":
			if r.Header.Get("Authorization") != "Bearer termination-only" {
				t.Error("Runner stop observation lost its narrow supervisor credential")
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunStopIntent{Identity: claim.Identity})
		case "/api/collaboration/runtime/runs/" + runID + "/outcome":
			var outcome protocol.CollaborationRunOutcome
			if err := json.NewDecoder(r.Body).Decode(&outcome); err != nil || outcome.Outcome != "success" || outcome.Body != claim.Inputs[0].Body || r.Header.Get("Authorization") != "Bearer business-only" {
				t.Error("test-owned executable did not return its explicit input result")
			}
			select {
			case outcomes <- outcome:
			default:
				t.Error("Runner sent a duplicate business result")
			}
			w.WriteHeader(http.StatusNoContent)
		case "/api/collaboration/runtime/runs/" + runID + "/proof":
			var proof protocol.CollaborationRunProof
			if err := json.NewDecoder(r.Body).Decode(&proof); err != nil || r.Header.Get("Authorization") != "Bearer termination-only" {
				t.Error("Runner proof lost its narrow supervisor credential")
			}
			select {
			case proofs <- proof:
			default:
				t.Error("Runner sent a duplicate terminal proof")
			}
			select {
			case <-proofRelease:
			case <-r.Context().Done():
				return
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ReceiptID: proof.ReceiptID, Identity: proof.Identity, Accepted: true})
		default:
			t.Error("collaboration consumed the ordinary task claim path")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	d, expected = collaborationManagedPollerFixture(t, server.URL, workspaceID, runtimeID)
	claim = protocol.CollaborationRunClaimResponse{Identity: protocol.CollaborationRunIdentity{
		DeploymentID: expected.DeploymentID, InstallationID: expected.InstallationID, RunID: runID,
		IssueID: uuid.NewString(), SessionID: uuid.NewString(), TargetID: uuid.NewString(), ChannelID: uuid.NewString(),
		BatchID: uuid.NewString(), GroupID: uuid.NewString(), LaunchNonce: uuid.NewString(), Fence: "1", ClaimGeneration: "1"},
		AgentID: uuid.NewString(), ResourceOwnerID: uuid.NewString(), ExecutionWorkspaceID: workspaceID, TerminationToken: "termination-only",
		Inputs: []protocol.CollaborationRunInput{{CommandID: uuid.NewString(), AuthorUserID: uuid.NewString(), ParticipantEpoch: "1", AccountEpoch: "1", LifecycleEpoch: "1", Body: "immutable poller input"}}}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.cfg.CollaborationRunner.Executable, content, 0700); err != nil {
		t.Fatal(err)
	}
	d.cfg.CollaborationRunner.Arguments = []string{collaborationPollerFakeArg}
	if _, err := d.client.Register(t.Context(), map[string]any{"workspace_id": workspaceID}); err != nil {
		t.Fatal(err)
	}
	d.workspaces[workspaceID] = &workspaceState{workspaceID: workspaceID, runtimeIDs: []string{runtimeID}}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	poller, reserved, err := d.initializeCollaborationPoller(ctx)
	if err != nil || reserved != 0 {
		t.Fatalf("initialize test-owned collaboration runner: reserved=%d err=%v", reserved, err)
	}
	if !poller.enabled {
		if os.Getenv("MULTICA_REQUIRE_COLLAB_NAMESPACE") == "1" {
			t.Fatal("required Linux kernel/static fixture containment unavailable")
		}
		t.Skip("Linux kernel/static fixture containment unavailable")
	}
	d.collaboration = poller
	pollCtx, cancelPoll := context.WithCancel(ctx)
	wakeup, pollDone := make(chan struct{}), make(chan struct{})
	var taskWG sync.WaitGroup
	go func() { defer close(pollDone); d.runBatchPoller(pollCtx, ctx, sem, wakeup, &taskWG) }()
	t.Cleanup(func() {
		cancelPoll()
		cancel()
		openProof()
		<-pollDone
		done := make(chan struct{})
		go func() { taskWG.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("collaboration Runner did not drain")
		}
	})
	// An unbuffered wakeup synchronizes with the blocked slot wait. No sleeps
	// or mock controller results stand in for the pre-claim capacity gate.
	select {
	case wakeup <- struct{}{}:
	case <-listed:
		t.Fatal("collaboration polled before machine capacity became available")
	case <-ctx.Done():
		t.Fatal("poller did not reach its capacity wait")
	}
	sem <- heldSlot
	var proof protocol.CollaborationRunProof
	select {
	case proof = <-proofs:
	case <-ctx.Done():
		t.Fatal("managed poller did not dispatch the real Runner")
	}
	if proof.Identity != claim.Identity || proof.State != "exited_proven" || proof.Containment != protocol.CollaborationContainmentLinuxPIDNamespace || proof.AttemptNonce != attempt.AttemptNonce || proof.SealedAttemptSeq != "1" {
		t.Fatal("Runner did not seal the real namespace's admitted attempt")
	}
	select {
	case outcome := <-outcomes:
		if outcome.Identity != claim.Identity || outcome.AttemptNonce != proof.AttemptNonce || outcome.AttemptSeq != proof.SealedAttemptSeq {
			t.Fatal("business result and terminal proof refer to different attempts")
		}
	default:
		t.Fatal("Runner produced proof without the fake provider's business result")
	}
	if len(sem) != 0 || d.activeTasks.Load() != 1 {
		t.Fatal("poller released capacity while Runner still awaited proof acknowledgement")
	}
	cancelPoll()
	<-pollDone
	openProof()
	taskWG.Wait()
	if len(sem) != 1 || d.activeTasks.Load() != 0 || poller.active.Load() {
		t.Fatal("completed Runner did not return its machine/update capacity")
	}
}
