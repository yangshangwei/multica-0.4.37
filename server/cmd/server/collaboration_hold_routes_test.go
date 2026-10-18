package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Reuse the real router, source issue and human participants while enrolling an
// independent installation/daemon namespace. Each lane must own its capacity;
// reusing the first runtime would only test shared-resource waiting.
func secondCollaborationRouteTarget(t *testing.T, first *collaborationRouteFixture) *collaborationRouteFixture {
	t.Helper()
	f := *first
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.RawURLEncoding.EncodeToString(public)
	challenge := func(body any) installation.Proof {
		t.Helper()
		var value installation.Challenge
		f.managed.request(t, f.ownerToken, false, http.MethodPost, "/api/installations/challenges", body, nil).Want(http.StatusOK).JSON(&value)
		canonical, err := base64.RawURLEncoding.DecodeString(value.SignaturePayload)
		if err != nil {
			t.Fatal(err)
		}
		return installation.Proof{ChallengeID: value.ChallengeID, SignaturePayload: value.SignaturePayload, BodyPayload: value.BodyPayload,
			Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, canonical))}
	}
	var enrolled service.InstallationEnrollment
	f.managed.request(t, f.ownerToken, false, http.MethodPost, "/api/installations/enroll", challenge(map[string]any{
		"purpose": "enroll", "public_key": key, "desktop_version": "0.4.40", "os": "linux",
	}), nil).Want(http.StatusOK).JSON(&enrolled)
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	f.managed.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.owner, "name": "Second fixture installation", "token_hash": auth.HashToken(pat), "token_prefix": "mul_", "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	daemonID, workspace := uuid.NewString(), f.managed.fx.WorkspaceID
	f.managed.request(t, pat, false, http.MethodPost, "/api/daemon/installation-bindings", challenge(map[string]any{
		"purpose": "bind", "public_key": key, "installation_id": enrolled.InstallationID, "workspace_id": workspace,
		"daemon_id": daemonID, "expected_binding_epoch": nil,
	}), nil).Want(http.StatusOK).JSON(&f.binding)
	f.installationID = enrolled.InstallationID
	f.runtime = f.managed.fx.Runtime(t, "Independent test-owned Linux provider", testutil.Cols{"owner_id": f.owner, "daemon_id": daemonID,
		"runtime_mode": "local", "provider": "codex", "status": "online", "last_seen_at": time.Now()})
	f.agent = f.managed.fx.Agent(t, "Second direct execution lane", f.runtime, testutil.Cols{"owner_id": f.owner, "kind": "user", "max_concurrent_tasks": 1})
	session, err := db.New(f.managed.pool).GetCollaborationSession(t.Context(), util.MustParseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	var registered service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodPost, "/api/collaboration/issues/"+f.issue+"/targets", map[string]any{
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
		"expected_target_set_version": strconv.FormatInt(session.TargetSetVersion, 10), "kind": "agent", "resource_id": f.agent,
		"roster": []collaboration.TargetResource{{Role: "agent", AgentID: f.agent, RuntimeID: f.runtime, ExecutionWorkspaceID: workspace, OwnerUserID: f.owner}},
	}, nil).Want(http.StatusCreated).JSON(&registered)
	f.target = util.UUIDToString(registered.TargetID)
	consentCollaborationRouteTarget(t, &f)
	if f.target == first.target || f.binding.BindingID == first.binding.BindingID || f.installationID == first.installationID {
		t.Fatal("second lane reused the first resource identity")
	}
	return &f
}

func consentCollaborationRouteTarget(t *testing.T, f *collaborationRouteFixture) {
	t.Helper()
	f.managed.request(t, f.ownerToken, false, http.MethodPost, "/api/collaboration/issues/"+f.issue+"/targets/"+f.target+"/consents", map[string]any{
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
		"agent_id": f.agent, "expected_target_epoch": "1", "expected_config_version": "1",
		"identity": map[string]any{"kind": "managed", "installation_id": f.installationID, "binding_id": f.binding.BindingID, "binding_epoch": f.binding.BindingEpoch},
	}, nil).Want(http.StatusCreated)
}

func holdCollaborationRouteTarget(t *testing.T, f *collaborationRouteFixture) {
	t.Helper()
	var grant string
	var epoch int64
	f.managed.fx.QueryRow(t, `SELECT id::text,epoch FROM collaboration_grant WHERE target_id=$1 AND revoked_at IS NULL`, f.target).Scan(&grant, &epoch)
	f.managed.request(t, f.ownerToken, false, http.MethodPost, "/api/collaboration/issues/"+f.issue+"/grants/"+grant+"/revoke", map[string]any{
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1", "expected_grant_epoch": strconv.FormatInt(epoch, 10),
	}, nil).Want(http.StatusOK)
	consentCollaborationRouteTarget(t, f)
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='grant_required' AND queue_state='paused'`, f.target) != 1 {
		t.Fatal("new owner consent released a target hold or resumed its stopped lane")
	}
}

func collaborationRouteTargetResumeBody(t *testing.T, f *collaborationRouteFixture) map[string]any {
	t.Helper()
	body := f.controlBody(t, "resume", "target")
	for _, target := range body["targets"].([]service.CollaborationTargetControl) {
		if util.UUIDToString(target.TargetID) == f.target {
			body["targets"] = []service.CollaborationTargetControl{target}
			return body
		}
	}
	t.Fatal("selected target missing from the source control snapshot")
	return nil
}

func collaborationRouteControlState(t *testing.T, f *collaborationRouteFixture) string {
	t.Helper()
	var snapshot string
	f.managed.fx.QueryRow(t, `SELECT jsonb_build_object(
		'session', (SELECT to_jsonb(s) FROM collaboration_session s WHERE s.id=$1),
		'channels', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_channel c WHERE c.session_id=$1),
		'targets', (SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM collaboration_target t WHERE t.session_id=$1),
		'commands', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$1),
		'groups', (SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_execution_group g WHERE g.session_id=$1),
		'runs', (SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.session_id=$1),
		'reservations', (SELECT jsonb_agg(to_jsonb(v) ORDER BY v.run_id) FROM collaboration_runtime_reservation v JOIN collaboration_run r ON r.id=v.run_id WHERE r.session_id=$1),
		'events', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM collaboration_event e WHERE e.issue_id=$2)
	)::text`, f.session, f.issue).Scan(&snapshot)
	return snapshot
}

func recoverCollaborationRouteControl(t *testing.T, f *collaborationRouteFixture, key string, want service.CollaborationQueueReceipt) {
	t.Helper()
	var receipt service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodGet, "/api/collaboration/issues/"+f.issue+"/control/operations/"+key, nil, nil).Want(http.StatusOK).JSON(&receipt)
	if !reflect.DeepEqual(receipt, want) {
		t.Fatal("operation recovery replaced the original control identity or decision")
	}
}

// Both supervisors use the test-owned static provider and Linux containment.
// The delayed proof is real and sealed in its local ledger; no SQL proof or
// process-state mock can satisfy this test's resumption and capacity assertions.
func TestCollaborationPartialResumeRoutesLinux(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("MULTICA_RUN_COLLAB_RUNTIME_E2E") != "1" {
		t.Skip("requires explicit Linux namespace integration environment")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 75*time.Second)
	t.Cleanup(cancel)
	a := newCollaborationRouteFixture(t)
	b := secondCollaborationRouteTarget(t, a)
	barrier := &collaborationProofBarrier{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(barrier.open)
	clientA, err := collaborationrun.NewClient(a.server.URL, a.binding.DaemonToken, barrier)
	if err != nil {
		t.Fatal(err)
	}
	clientB, err := collaborationrun.NewClient(b.server.URL, b.binding.DaemonToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	newRunner := func(f *collaborationRouteFixture, client *collaborationrun.Client) (*collaborationrun.Runner, net.Listener, protocol.CollaborationRunClaimRequest) {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		controller := collaborationRouteController(t, "block", listener.Addr().String())
		ledger, err := collaborationrun.OpenLedger(filepath.Join(t.TempDir(), "ledger"))
		if err != nil {
			t.Fatal(err)
		}
		request := protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion,
			DeploymentID: f.deployment, InstallationID: f.installationID, RuntimeID: f.runtime,
			BindingID: f.binding.BindingID, BindingEpoch: f.binding.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
		return &collaborationrun.Runner{Ledger: ledger, Client: client, Controller: controller, StopPollInterval: 10 * time.Millisecond}, listener, request
	}
	runnerA, listenerA, requestA := newRunner(a, clientA)
	runnerB, listenerB, requestB := newRunner(b, clientB)
	firstA, firstB := a.comment(t, false, "first A input"), b.comment(t, true, "first B input")
	initial := a.control(t, uuid.NewString(), a.controlBody(t, "resume", "issue"), http.StatusOK)
	if initial.Code != "resumed" || len(initial.Channels) != 2 || len(initial.TargetHolds) != 2 {
		t.Fatal("initial Resume did not release both independently consented targets")
	}
	listA, err := clientA.List(ctx, a.runtime)
	if err != nil || len(listA.RunIDs) != 1 {
		t.Fatalf("A did not promote exactly one input: %v", err)
	}
	listB, err := clientB.List(ctx, b.runtime)
	if err != nil || len(listB.RunIDs) != 1 {
		t.Fatalf("B was blocked by A's independent namespace: %v", err)
	}
	runA, runB := listA.RunIDs[0], listB.RunIDs[0]
	finishedA, finishedB := make(chan error, 1), make(chan error, 1)
	go func() { finishedA <- runnerA.Run(ctx, runA, requestA); close(finishedA) }()
	go func() { finishedB <- runnerB.Run(ctx, runB, requestB); close(finishedB) }()
	t.Cleanup(func() {
		cancel()
		barrier.open()
		for _, finished := range []<-chan error{finishedA, finishedB} {
			select {
			case <-finished:
			case <-time.After(10 * time.Second):
				t.Error("two-lane supervisor did not finish fixture cleanup")
			}
		}
	})
	_, observedA := collaborationRouteObservation(t, listenerA)
	_, observedB := collaborationRouteObservation(t, listenerB)
	if observedA.Identity.RunID != runA || observedB.Identity.RunID != runB || observedA.Identity.ChannelID == observedB.Identity.ChannelID ||
		len(observedA.Inputs) != 1 || observedA.Inputs[0].CommandID != util.UUIDToString(firstA.Targets[0].CommandID) || observedA.Inputs[0].AuthorUserID != a.owner ||
		len(observedB.Inputs) != 1 || observedB.Inputs[0].CommandID != util.UUIDToString(firstB.Targets[0].CommandID) || observedB.Inputs[0].AuthorUserID != b.contributor {
		t.Fatal("real parallel providers lost their lane, input or author identity")
	}
	nextA := a.comment(t, true, "A waits for an explicitly renewed Resume")
	a.control(t, uuid.NewString(), a.controlBody(t, "stop", "issue"), http.StatusOK)
	select {
	case <-barrier.entered:
	case <-ctx.Done():
		t.Fatal("A did not reach real sealed-proof delivery after Stop")
	}
	select {
	case err = <-finishedB:
		if err != nil {
			t.Fatalf("B's real sealed proof did not commit: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("B did not finish independently of A's proof delivery")
	}
	holdCollaborationRouteTarget(t, a)
	holdCollaborationRouteTarget(t, b)
	partialKey, partialBody := uuid.NewString(), a.controlBody(t, "resume", "issue")
	partial := a.control(t, partialKey, partialBody, http.StatusOK)
	if partial.Code != "partially_resumed" || partial.QueueState != "active" || len(partial.Channels) != 2 || len(partial.TargetHolds) != 2 {
		t.Fatal("Resume All did not expose separate channel and target-hold decisions")
	}
	channels := make(map[string]service.CollaborationChannelControlResult)
	for _, value := range partial.Channels {
		id := util.UUIDToString(value.ChannelID)
		if _, exists := channels[id]; exists {
			t.Fatal("partial Resume duplicated a channel result")
		}
		channels[id] = value
	}
	holds := make(map[string]service.CollaborationTargetHoldResult)
	for _, value := range partial.TargetHolds {
		id := util.UUIDToString(value.TargetID)
		if _, exists := holds[id]; exists {
			t.Fatal("partial Resume duplicated a target-hold result")
		}
		holds[id] = value
	}
	if channels[observedA.Identity.ChannelID].Code != "stop_unconfirmed" || channels[observedA.Identity.ChannelID].QueueState != "paused" ||
		channels[observedB.Identity.ChannelID].Code != "resumed" || channels[observedB.Identity.ChannelID].QueueState != "active" ||
		holds[a.target].Code != "stop_unconfirmed" || holds[a.target].HoldReason == nil || *holds[a.target].HoldReason != collaboration.HoldGrantRequired ||
		holds[b.target].Code != "hold_released" || holds[b.target].HoldReason != nil {
		t.Fatal("proof-blocked A prevented B's lane/hold recovery or released its own hold")
	}
	if a.managed.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runA) != 1 ||
		a.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NULL`, runA) != 1 ||
		a.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='exited_proven' AND settled_at IS NOT NULL`, runB) != 1 {
		t.Fatal("partial Resume used process observation or another lane's proof to clear A's reservation")
	}
	nextB := b.comment(t, true, "B continues while A's proof is withheld")
	listB, err = clientB.List(ctx, b.runtime)
	if err != nil || len(listB.RunIDs) != 1 || listB.RunIDs[0] == runB {
		t.Fatalf("resumed B could not advance independently of A: %v", err)
	}
	runnerB.Controller.Arguments = []string{collaborationRouteFakeArg, "succeed", listenerB.Addr().String()}
	if err = runnerB.Run(ctx, listB.RunIDs[0], requestB); err != nil {
		t.Fatalf("B's subsequent real provider failed: %v", err)
	}
	_, subsequentB := collaborationRouteObservation(t, listenerB)
	if len(subsequentB.Inputs) != 1 || subsequentB.Inputs[0].CommandID != util.UUIDToString(nextB.Targets[0].CommandID) {
		t.Fatal("B replayed stopped input instead of its new batch")
	}
	listA, err = clientA.List(ctx, a.runtime)
	if err != nil || len(listA.RunIDs) != 0 {
		t.Fatalf("A produced work before its explicit proof-ready Resume: %v", err)
	}
	refusedKey, refusedBody := uuid.NewString(), collaborationRouteTargetResumeBody(t, a)
	before := collaborationRouteControlState(t, a)
	refused := a.control(t, refusedKey, refusedBody, http.StatusConflict)
	if refused.Code != "stop_unconfirmed" || before != collaborationRouteControlState(t, a) {
		t.Fatal("target Resume with missing proof changed control, hold, input or retained execution state")
	}
	recoverCollaborationRouteControl(t, a, partialKey, partial)
	recoverCollaborationRouteControl(t, a, refusedKey, refused)
	barrier.open()
	select {
	case err = <-finishedA:
		// Proof delivery has its own bounded deadline. If the barrier outlived
		// that HTTP request, retry the same sealed local record, never a launch.
		if errors.Is(err, collaborationrun.ErrProofPending) {
			err = runnerA.Run(ctx, runA, requestA)
		}
		if err != nil {
			t.Fatalf("A's original sealed proof could not finish after delivery resumed: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("A did not complete proof delivery after barrier release")
	}
	if a.managed.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='paused' AND hold_reason='grant_required'`, a.target) != 1 ||
		a.managed.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, runA) != 1 {
		t.Fatal("late proof implicitly released A's hold/control or failed to free its proven reservation")
	}
	before = collaborationRouteControlState(t, a)
	if replay := a.control(t, refusedKey, refusedBody, http.StatusConflict); !reflect.DeepEqual(replay, refused) || before != collaborationRouteControlState(t, a) {
		t.Fatal("the rejected Resume key became a deferred recovery after proof arrived")
	}
	resumed := a.control(t, uuid.NewString(), collaborationRouteTargetResumeBody(t, a), http.StatusOK)
	if resumed.Code != "resumed" || resumed.QueueState != "active" || len(resumed.Channels) != 1 || len(resumed.TargetHolds) != 1 || resumed.TargetHolds[0].Code != "hold_released" {
		t.Fatal("fresh target Resume did not release precisely A's lane and hold")
	}
	listA, err = clientA.List(ctx, a.runtime)
	if err != nil || len(listA.RunIDs) != 1 || listA.RunIDs[0] == runA {
		t.Fatalf("fresh A Resume did not promote its retained waiting input: %v", err)
	}
	runnerA.Controller.Arguments = []string{collaborationRouteFakeArg, "succeed", listenerA.Addr().String()}
	if err = runnerA.Run(ctx, listA.RunIDs[0], requestA); err != nil {
		t.Fatalf("A's fresh real provider failed: %v", err)
	}
	_, subsequentA := collaborationRouteObservation(t, listenerA)
	if len(subsequentA.Inputs) != 1 || subsequentA.Inputs[0].CommandID != util.UUIDToString(nextA.Targets[0].CommandID) || subsequentA.Inputs[0].AuthorUserID != a.contributor ||
		a.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='stopped'`, []string{util.UUIDToString(firstA.Targets[0].CommandID), util.UUIDToString(firstB.Targets[0].CommandID)}) != 2 ||
		a.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='succeeded'`, []string{util.UUIDToString(nextA.Targets[0].CommandID), util.UUIDToString(nextB.Targets[0].CommandID)}) != 2 {
		t.Fatal("independent resumption replayed stopped inputs or lost the waiting authors/results")
	}
	if replay := a.control(t, partialKey, partialBody, http.StatusOK); !reflect.DeepEqual(replay, partial) {
		t.Fatal("old partial Resume changed after both lanes progressed")
	}
	recoverCollaborationRouteControl(t, a, partialKey, partial)
	recoverCollaborationRouteControl(t, a, refusedKey, refused)
}
