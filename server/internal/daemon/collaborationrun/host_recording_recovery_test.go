package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func retainedHostRecordingFixture(t *testing.T, count int) (*Ledger, *RunLedger) {
	t.Helper()
	ledger := newHostTestLedger(t)
	local := registerHostTestRun(t, ledger)
	permit := hostTestPermit(t, local)
	stream := uuid.NewString()
	if err := local.startHostRecording(stream); err != nil {
		t.Fatal(err)
	}
	for i := range count {
		p := protocol.CollaborationRecordingCheckpoint{Version: 1, Identity: permit.Identity, AttemptSeq: permit.AttemptSeq,
			AttemptNonce: permit.AttemptNonce, StreamID: stream, Sequence: i + 1, MessageOffset: i,
			Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "retained original"}}}
		if err := local.stageHostCheckpoint(p, local.record.Host.BusinessToken); err != nil {
			t.Fatal(err)
		}
	}
	if err := local.stopHost(); err != nil {
		t.Fatal(err)
	}
	if err := local.observeHostTermination(permit.AttemptNonce, "not_started", protocol.CollaborationCleanupObservation{State: "unverified", Scope: "direct_process"}, nil); err != nil {
		t.Fatal(err)
	}
	hostTestAcknowledge(t, local, true)
	if _, err := local.sealHost(); err != nil {
		t.Fatal(err)
	}
	return ledger, local
}

func TestHostRecordingRecoverySharesBacklogDeadlineAndPrioritizesProof(t *testing.T) {
	ledger, local := retainedHostRecordingFixture(t, 32)
	defer local.Close()
	var mu sync.Mutex
	var paths []string
	var requests []protocol.CollaborationRecordingCheckpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case runPath(local.runID, "proof"):
			var p protocol.CollaborationRunProof
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				t.Error(err)
				return
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ProtocolVersion: 3, Identity: p.Identity, ReceiptID: p.ReceiptID, Accepted: true})
		case runPath(local.runID, "checkpoints"):
			var p protocol.CollaborationRecordingCheckpoint
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				t.Error(err)
				return
			}
			if r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat("d", 64) {
				t.Error("checkpoint replay did not use its original business credential")
			}
			mu.Lock()
			requests = append(requests, p)
			mu.Unlock()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(150 * time.Millisecond):
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: p.Identity, StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: p.MessageOffset + len(p.Messages), Accepted: true})
		default:
			t.Errorf("unexpected recovery request: %s", r.URL.Path)
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{Ledger: ledger, Client: client}
	started := time.Now()
	err = runner.recoverHost(t.Context(), local)
	if err == nil || errors.Is(err, ErrProofPending) || errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("unfinished telemetry must remain pending independently of proof: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2500*time.Millisecond {
		t.Fatalf("telemetry deadline multiplied by backlog size: %s", elapsed)
	}
	if local.record.Phase != phaseAcknowledged || !hostRecordingPending(local.record.Host) {
		t.Fatal("telemetry budget exhaustion lost proof ACK or unresolved recording facts")
	}
	if rec := local.record.Host.Recording; rec.Seal == nil || rec.SealAccepted || rec.Chunks[31].Payload == nil {
		t.Fatal("deadline exhaustion did not retain the unsent seal and original checkpoint")
	}
	retained, err := ledger.Retained()
	if err != nil || len(retained) != 1 || retained[0].Unconfirmed || !retained[0].CanProve {
		t.Fatalf("recording backlog must remain recoverable without an execution barrier: %+v, %v", retained, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) < 2 || paths[0] != runPath(local.runID, "proof") || len(requests) == 0 || len(requests) == 32 {
		t.Fatalf("proof priority or bounded progress missing: paths=%v, checkpoints=%d", paths, len(requests))
	}
}

func TestHostRecordingSealReplaysBeforeAdvancingAcknowledgement(t *testing.T) {
	_, local := retainedHostRecordingFixture(t, 1)
	defer local.Close()
	token := local.record.TerminationToken
	var mu sync.Mutex
	var seals []protocol.CollaborationRecordingSeal
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p protocol.CollaborationRecordingSeal
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path != runPath(local.runID, "recording/seal") || r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("seal changed its original endpoint or termination identity")
		}
		mu.Lock()
		seals = append(seals, p)
		count := len(seals)
		mu.Unlock()
		if count == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: p.StreamID, RetainedThrough: 1, ObservedThrough: p.ObservedThrough, Reason: p.Reason, Sealed: true})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{Client: client}
	if err := runner.sealHostRecording(t.Context(), local); err == nil {
		t.Fatal("fixture failed to lose the original seal acknowledgement")
	}
	if err := local.acknowledgeHostCheckpoint(1, false); err != nil {
		t.Fatal(err)
	}
	if err := runner.sealHostRecording(t.Context(), local); err != nil {
		t.Fatal(err)
	}
	if err := runner.sealHostRecording(t.Context(), local); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seals) != 3 || !reflect.DeepEqual(seals[0], seals[1]) {
		t.Fatalf("seal ACK recovery changed the original key/payload or failed to advance: %+v", seals)
	}
	advanced := seals[0]
	advanced.Sequence, advanced.AcknowledgedThrough = 2, 1
	if !reflect.DeepEqual(advanced, seals[2]) || !local.record.Host.Recording.SealAccepted || hostRecordingPending(local.record.Host) {
		t.Fatal("receipt recovery did not persist one monotonic metadata update")
	}
}

func TestHostRecordingRecoveredUnflushedTailRemainsUnknown(t *testing.T) {
	_, local := retainedHostRecordingFixture(t, 1)
	defer local.Close()
	if err := local.acknowledgeHostCheckpoint(1, false); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p protocol.CollaborationRecordingSeal
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
			return
		}
		if p.ObservedThrough != nil || p.Reason != "incomplete" || p.AcknowledgedThrough != 1 {
			t.Error("restart invented a fully observed tail from the persisted prefix")
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: p.StreamID, RetainedThrough: 1, ObservedThrough: p.ObservedThrough, Reason: p.Reason, Sealed: true})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Runner{Client: client}).sealHostRecording(t.Context(), local); err != nil {
		t.Fatal(err)
	}
}

func TestHostRecordingLedgerRejectsSealWithUnissuedNonce(t *testing.T) {
	_, local := retainedHostRecordingFixture(t, 0)
	defer local.Close()
	next, err := local.recordingNext()
	if err != nil {
		t.Fatal(err)
	}
	next.Host.Recording.Seal = &protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: next.Identity,
		AttemptSeq: "1", AttemptNonce: uuid.NewString(), StreamID: next.Host.Recording.StreamID, Reason: "incomplete"}
	if err := validateHostRecordingLedger(next); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("unissued recording seal nonce survived ledger validation: %v", err)
	}
}

func TestHostRecordingRecoveryCancelledBacklogKeepsUnresolvedPayloads(t *testing.T) {
	_, local := retainedHostRecordingFixture(t, 2)
	defer local.Close()
	if err := local.acknowledgeHostCheckpoint(1, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := (&Runner{}).recoverHostRecording(ctx, local); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled recovery kept processing the rejected backlog: %v", err)
	}
	if c := local.record.Host.Recording.Chunks[1]; c.Rejected || c.Payload == nil || c.BusinessToken == "" {
		t.Fatal("cancelled recovery discarded an unresolved original checkpoint")
	}
}

func TestHostRecordingRecoveryBlockedUploadCannotPreventStopAndProof(t *testing.T) {
	f := newHostRunFixture(t, "wait")
	f.claim.RecordingVersion = protocol.CollaborationCheckpointVersion
	key, err := hostSessionKey(f.claim)
	if err != nil {
		t.Fatal(err)
	}
	session, err := f.runner.Ledger.lockHostSession(key, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.save("", true); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	started, terminated, proved, release := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var startOnce, endOnce, proofOnce sync.Once
	original := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/checkpoints") || strings.HasSuffix(r.URL.Path, "/recording/seal") {
			startOnce.Do(func() { close(started) })
			<-release
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		original.ServeHTTP(w, r)
		if strings.HasSuffix(r.URL.Path, "/termination") {
			endOnce.Do(func() { close(terminated) })
		}
		if strings.HasSuffix(r.URL.Path, "/proof") {
			proofOnce.Do(func() { close(proved) })
		}
	})
	// Unlike the combined result fixture, this provider is still running when
	// its continuity message starts uploading and the user requests Stop.
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- f.runner.Run(ctx, f.claim.Identity.RunID, f.request)
	}()
	t.Cleanup(func() {
		cancel()
		close(release)
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			t.Error("Runner did not exit during bounded fixture teardown")
		}
	})
	f.waitForStart(t)
	select {
	case <-started:
	case <-time.After(6 * time.Second):
		t.Fatal("recording upload did not begin while the provider waited")
	}
	select {
	case err := <-done:
		t.Fatalf("fixture ended before an explicit Stop: %v", err)
	default:
	}
	f.mu.Lock()
	f.stop = true
	f.mu.Unlock()
	select {
	case <-terminated:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked upload prevented managed provider termination")
	}
	select {
	case <-proved:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked upload prevented the original proof before release")
	}
	select {
	case err := <-done:
		if errors.Is(err, ErrProofPending) || errors.Is(err, ErrUnconfirmed) {
			t.Fatalf("recording failure became unconfirmed termination: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("recording recovery prevented bounded return after proof")
	}
	local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	if local.record.Phase != phaseAcknowledged || !local.record.Host.Stopped || local.record.Host.Recording.Seal == nil || !hostRecordingPending(local.record.Host) {
		t.Fatal("blocked recording lost the accepted Stop/proof or pending metadata seal")
	}
	if local.record.Proof.State != "host_exit_observed" || len(f.starts(t)) != 1 {
		t.Fatal("Stop did not retain the original managed launch termination")
	}
}

func TestHostRecordingRecoveryMalformedSealACKKeepsOriginalPending(t *testing.T) {
	ledger, local := retainedHostRecordingFixture(t, 1)
	defer local.Close()
	runID, identity, proof := local.runID, local.record.Identity, *local.record.Proof
	var mu sync.Mutex
	var requests []string
	var seals []protocol.CollaborationRecordingSeal
	mode := "lost_ack"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case runPath(runID, "proof"):
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ProtocolVersion: 3, Identity: identity, ReceiptID: proof.ReceiptID, Accepted: true})
		case runPath(runID, "checkpoints"):
			var p protocol.CollaborationRecordingCheckpoint
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				t.Error(err)
				return
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: identity, StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: 1, Accepted: true})
		case runPath(runID, "recording/seal"):
			var seal protocol.CollaborationRecordingSeal
			if err := json.NewDecoder(r.Body).Decode(&seal); err != nil {
				t.Error(err)
				return
			}
			seals = append(seals, seal)
			if mode == "lost_ack" {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			observed := seal.ObservedThrough
			if mode == "malformed" {
				negative := -1
				observed = &negative
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: seal.StreamID,
				RetainedThrough: 1, ObservedThrough: observed, Sealed: true, Reason: seal.Reason})
		default:
			t.Errorf("recording recovery invoked another operation: %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Runner{Ledger: ledger, Client: client}).recoverHost(t.Context(), local); err == nil || local.record.Phase != phaseAcknowledged || local.record.Host.Recording.SealAccepted {
		t.Fatalf("fixture did not retain an uncertain seal after proof ACK: %v", err)
	}
	original := *local.record.Host.Recording.Seal
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	mode, requests = "malformed", nil
	mu.Unlock()
	reopened, err := OpenLedger(ledger.dir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &Runner{Ledger: reopened, Client: client}
	request := protocol.CollaborationRunClaimRequest{DeploymentID: identity.DeploymentID, InstallationID: identity.InstallationID}
	if err := restarted.Run(t.Context(), runID, request); !errors.Is(err, ErrProtocol) {
		t.Fatalf("malformed seal response was not refused: %v", err)
	}
	local, err = reopened.Lock(identity.DeploymentID, identity.InstallationID, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	recording := local.record.Host.Recording
	if local.record.Phase != phaseAcknowledged || recording.SealAccepted || !hostRecordingPending(local.record.Host) || !reflect.DeepEqual(*recording.Seal, original) || !reflect.DeepEqual(*local.record.Proof, proof) {
		t.Fatal("malformed ACK changed the accepted proof or original pending seal")
	}
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	mode = "available"
	mu.Unlock()
	if err := restarted.Run(t.Context(), runID, request); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Run(t.Context(), runID, request); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 || len(seals) != 3 {
		t.Fatalf("recovery repeated an acknowledged operation: requests=%v seals=%d", requests, len(seals))
	}
	for _, path := range requests {
		if path != runPath(runID, "recording/seal") {
			t.Fatalf("restart performed a nonrecording operation: %s", path)
		}
	}
	for _, seal := range seals {
		if !reflect.DeepEqual(seal, original) {
			t.Fatal("seal recovery changed its original identity, key or payload")
		}
	}
}
