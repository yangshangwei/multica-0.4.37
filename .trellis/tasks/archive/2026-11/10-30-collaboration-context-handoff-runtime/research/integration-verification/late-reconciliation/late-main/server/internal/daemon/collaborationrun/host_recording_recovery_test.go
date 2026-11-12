package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type hostRecordingRecoveryRequest struct {
	path          string
	authorization string
	body          []byte
}

type hostRecordingRecoveryFixture struct {
	*hostRunFixture
	mu          sync.Mutex
	mode        string
	requests    []hostRecordingRecoveryRequest
	retained    map[int][]byte
	through     int
	started     chan struct{}
	terminated  chan struct{}
	proved      chan struct{}
	release     chan struct{}
	startOnce   sync.Once
	endOnce     sync.Once
	proofOnce   sync.Once
	releaseOnce sync.Once
}

func newHostRecordingRecoveryFixture(t *testing.T, providerMode, mode string) *hostRecordingRecoveryFixture {
	t.Helper()
	host := newHostRunFixture(t, providerMode)
	host.claim.RecordingVersion = protocol.CollaborationCheckpointVersion
	f := &hostRecordingRecoveryFixture{hostRunFixture: host, mode: mode, retained: make(map[int][]byte),
		started: make(chan struct{}), terminated: make(chan struct{}), proved: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(f.unblock)
	original := host.server.Config.Handler
	host.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkpoint := r.URL.Path == runPath(host.claim.Identity.RunID, "checkpoints")
		seal := r.URL.Path == runPath(host.claim.Identity.RunID, "recording/seal")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		f.mu.Lock()
		f.requests = append(f.requests, hostRecordingRecoveryRequest{path: r.URL.Path,
			authorization: r.Header.Get("Authorization"), body: body})
		mode := f.mode
		f.mu.Unlock()
		if !checkpoint && !seal {
			original.ServeHTTP(w, r)
			if strings.HasSuffix(r.URL.Path, "/termination") {
				f.endOnce.Do(func() { close(f.terminated) })
			}
			if strings.HasSuffix(r.URL.Path, "/proof") {
				f.proofOnce.Do(func() { close(f.proved) })
			}
			return
		}
		if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Workspace-ID") != "" {
			t.Error("recording request escaped its narrow transport")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var p protocol.CollaborationRecordingCheckpoint
		var s protocol.CollaborationRecordingSeal
		var identity protocol.CollaborationRunIdentity
		var sequence, nonce string
		if checkpoint {
			if err := json.Unmarshal(body, &p); err != nil || protocol.ValidateCollaborationRecordingCheckpoint(p) != nil {
				t.Errorf("invalid checkpoint: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			identity, sequence, nonce = p.Identity, p.AttemptSeq, p.AttemptNonce
		} else {
			if err := json.Unmarshal(body, &s); err != nil || protocol.ValidateCollaborationRecordingSeal(s) != nil {
				t.Errorf("invalid seal: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			identity, sequence, nonce = s.Identity, s.AttemptSeq, s.AttemptNonce
		}
		host.mu.Lock()
		permit, issued := host.permits[sequence]
		host.mu.Unlock()
		credential := host.claim.TerminationToken
		if checkpoint {
			credential = "mcb_" + strings.Repeat(sequence, 64)
		}
		if !issued || identity != permit.Identity || sequence != permit.AttemptSeq || nonce != permit.AttemptNonce || r.Header.Get("Authorization") != "Bearer "+credential {
			t.Error("recording request changed the issued identity, attempt or credential")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		f.startOnce.Do(func() { close(f.started) })
		if mode == "blocked" {
			<-f.release
		}
		f.mu.Lock()
		if checkpoint {
			prior, accepted := f.retained[p.Sequence]
			if accepted && !reflect.DeepEqual(prior, body) {
				f.mu.Unlock()
				t.Error("checkpoint replay changed its original bytes")
				w.WriteHeader(http.StatusConflict)
				return
			}
			if !accepted && (mode == "available" || mode == "malformed_seal") {
				// Recovery may ACK only a request accepted before the lost reply.
				f.mu.Unlock()
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if mode == "lost_ack" {
				f.retained[p.Sequence] = append([]byte(nil), body...)
				f.through = max(f.through, p.MessageOffset+len(p.Messages))
			}
		}
		through := f.through
		f.mu.Unlock()
		if mode != "available" && mode != "malformed_seal" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if checkpoint {
			_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: p.Identity,
				StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: through, Accepted: true})
			return
		}
		observed := s.ObservedThrough
		if mode == "malformed_seal" {
			negative := -1
			observed = &negative
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: s.StreamID,
			RetainedThrough: through, ObservedThrough: observed, Sealed: true, Reason: s.Reason})
	})
	return f
}

func (f *hostRecordingRecoveryFixture) unblock() {
	f.releaseOnce.Do(func() { close(f.release) })
}

func (f *hostRecordingRecoveryFixture) restart(t *testing.T, transport http.RoundTripper) *Runner {
	t.Helper()
	ledger, err := OpenLedger(f.runner.Ledger.dir)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(f.server.URL, "", transport)
	if err != nil {
		t.Fatal(err)
	}
	return &Runner{Ledger: ledger, Client: client}
}

func (f *hostRecordingRecoveryFixture) recoveryRequest() protocol.CollaborationRunClaimRequest {
	return protocol.CollaborationRunClaimRequest{DeploymentID: f.request.DeploymentID, InstallationID: f.request.InstallationID}
}

func TestHostRecordingRecoveryRestartReplaysOnlyOriginalRequests(t *testing.T) {
	f := newHostRecordingRecoveryFixture(t, "result", "lost_ack")
	_ = f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
	local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	recording := local.record.Host.Recording
	if local.record.Phase != phaseAcknowledged || recording == nil || len(recording.Chunks) != 1 || recording.Chunks[0].Payload == nil {
		_ = local.Close()
		t.Fatal("fixture did not retain an uncertain checkpoint after proof ACK")
	}
	original := *recording.Chunks[0].Payload
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	requestCount := len(f.requests)
	f.mode = "available"
	f.mu.Unlock()
	restarted := f.restart(t, nil)
	if err := restarted.Run(t.Context(), f.claim.Identity.RunID, f.recoveryRequest()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	replayed := append([]hostRecordingRecoveryRequest(nil), f.requests[requestCount:]...)
	f.mu.Unlock()
	checkpointCount := 0
	for _, request := range replayed {
		switch request.path {
		case runPath(f.claim.Identity.RunID, "checkpoints"):
			var replay protocol.CollaborationRecordingCheckpoint
			if err := json.Unmarshal(request.body, &replay); err != nil || !reflect.DeepEqual(replay, original) {
				t.Fatal("restart changed the original checkpoint request")
			}
			checkpointCount++
		case runPath(f.claim.Identity.RunID, "recording/seal"):
		default:
			t.Fatalf("restart invoked a nonrecording operation: %s", request.path)
		}
	}
	if checkpointCount != 1 {
		t.Fatalf("restart did not replay exactly one original checkpoint: %d", checkpointCount)
	}
	local, err = restarted.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	recording = local.record.Host.Recording
	if local.record.Phase != phaseAcknowledged || !recording.Chunks[0].Accepted || recording.Chunks[0].Payload != nil || !recording.SealAccepted {
		t.Fatal("restart did not persist original checkpoint/seal ACKs")
	}
	f.hostRunFixture.mu.Lock()
	defer f.hostRunFixture.mu.Unlock()
	if len(f.proofs) != 1 || len(f.permits) != 1 || len(f.starts(t)) != 1 {
		t.Fatal("recording recovery repeated proof, permit or provider execution")
	}
}

func TestHostRecordingRecoveryBlockedUploadCannotPreventStopAndProof(t *testing.T) {
	f := newHostRecordingRecoveryFixture(t, "wait", "blocked")
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
	// The retained continuity message flushes while the fake provider waits.
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.runner.Run(ctx, f.claim.Identity.RunID, f.request) }()
	t.Cleanup(func() {
		cancel()
		f.unblock()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("Runner did not exit during bounded fixture teardown")
		}
	})
	f.waitForStart(t)
	select {
	case <-f.started:
	case <-time.After(6 * time.Second):
		t.Fatal("recording upload did not begin while the provider waited")
	}
	select {
	case err := <-done:
		t.Fatalf("fixture ended before an explicit stop: %v", err)
	default:
	}
	f.hostRunFixture.mu.Lock()
	f.stop = true
	f.hostRunFixture.mu.Unlock()
	select {
	case <-f.terminated:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked upload prevented the managed provider termination")
	}
	select {
	case <-f.proved:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked upload prevented the original proof before release")
	}
	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("recording recovery prevented bounded return after proof")
	}
	// Keep a completion for cleanup without a second wait on the consumed value.
	done <- runErr
	if errors.Is(runErr, ErrProofPending) || errors.Is(runErr, ErrUnconfirmed) {
		t.Fatalf("recording failure became unconfirmed termination: %v", runErr)
	}
	select {
	case <-f.release:
		t.Fatal("recording was released before Stop/proof were verified")
	default:
	}
	local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	if local.record.Phase != phaseAcknowledged || !local.record.Host.Stopped {
		t.Fatal("blocked recording prevented durable Stop/proof ACK")
	}
	if local.record.Host.Recording.Seal == nil || !hostRecordingPending(local.record.Host) {
		t.Fatal("blocked recording did not retain its metadata-only seal for recovery")
	}
}

func TestHostRecordingRecoveryPendingProofHasPriority(t *testing.T) {
	f := newHostRecordingRecoveryFixture(t, "result", "lost_ack")
	f.loseProofReply = true
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); !errors.Is(err, ErrProofPending) {
		t.Fatalf("fixture did not retain the original pending proof: %v", err)
	}
	var mu sync.Mutex
	paths := []string{}
	transport := hostReviewTransport(func(request *http.Request) (*http.Response, error) {
		mu.Lock()
		paths = append(paths, request.URL.Path)
		mu.Unlock()
		if strings.HasSuffix(request.URL.Path, "/checkpoints") || strings.HasSuffix(request.URL.Path, "/recording/seal") || strings.HasSuffix(request.URL.Path, "/outcome") {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}
		return http.DefaultTransport.RoundTrip(request)
	})
	restarted := f.restart(t, transport)
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	err := restarted.Run(ctx, f.claim.Identity.RunID, f.recoveryRequest())
	if errors.Is(err, ErrProofPending) || errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("telemetry consumed the original proof budget: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) < 2 || paths[0] != runPath(f.claim.Identity.RunID, "proof") || paths[1] != runPath(f.claim.Identity.RunID, "checkpoints") {
		t.Fatalf("pending proof was not followed by original recording recovery: %v", paths)
	}
	local, err := restarted.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	if local.record.Phase != phaseAcknowledged || !hostRecordingPending(local.record.Host) {
		t.Fatal("telemetry uncertainty weakened the accepted original proof")
	}
}

func TestHostRecordingRecoveryAdvancesSealAfterOriginalACK(t *testing.T) {
	f := newHostRecordingRecoveryFixture(t, "result", "lost_ack")
	_ = f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
	local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	recording := local.record.Host.Recording
	if recording == nil || recording.Seal == nil || recording.SealAccepted || recording.Seal.Sequence != 1 || recording.Seal.AcknowledgedThrough != 0 {
		_ = local.Close()
		t.Fatal("fixture did not retain an uncertain original seal")
	}
	original := *recording.Seal
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	requestCount := len(f.requests)
	f.mode = "available"
	f.mu.Unlock()
	restarted := f.restart(t, nil)
	if err := restarted.Run(t.Context(), f.claim.Identity.RunID, f.recoveryRequest()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	replayed := append([]hostRecordingRecoveryRequest(nil), f.requests[requestCount:]...)
	f.mu.Unlock()
	seals := 0
	for _, request := range replayed {
		if request.path != runPath(f.claim.Identity.RunID, "recording/seal") {
			continue
		}
		var seal protocol.CollaborationRecordingSeal
		if err := json.Unmarshal(request.body, &seal); err != nil || !reflect.DeepEqual(seal, original) {
			t.Fatal("learning a checkpoint ACK rewrote the uncertain original seal")
		}
		seals++
	}
	if seals != 1 {
		t.Fatalf("original seal was not ACKed exactly once on restart: %d", seals)
	}
	local, err = restarted.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	through := hostRecordingAcknowledged(local.record.Host.Recording)
	if through == 0 || !local.record.Host.Recording.SealAccepted || !hostRecordingPending(local.record.Host) {
		_ = local.Close()
		t.Fatal("newly learned checkpoint ACK did not retain a pending metadata advance")
	}
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Run(t.Context(), f.claim.Identity.RunID, f.recoveryRequest()); err != nil {
		t.Fatal(err)
	}
	local, err = restarted.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	seal := *local.record.Host.Recording.Seal
	expected := original
	expected.Sequence = original.Sequence + 1
	expected.AcknowledgedThrough = through
	if !reflect.DeepEqual(seal, expected) || !local.record.Host.Recording.SealAccepted || hostRecordingPending(local.record.Host) {
		t.Fatal("confirmed original seal did not advance monotonically without changing its reason/identity")
	}
	f.mu.Lock()
	requestCount = len(f.requests)
	f.mu.Unlock()
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Run(t.Context(), f.claim.Identity.RunID, f.recoveryRequest()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) != requestCount {
		t.Fatal("settled recording metadata was uploaded again")
	}
}

func TestHostRecordingRecoveryMalformedSealACKKeepsOriginalPending(t *testing.T) {
	f := newHostRecordingRecoveryFixture(t, "result", "lost_ack")
	_ = f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
	local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if local.record.Host.Recording == nil || local.record.Host.Recording.Seal == nil {
		_ = local.Close()
		t.Fatal("fixture did not retain an original seal")
	}
	original := *local.record.Host.Recording.Seal
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.mode = "malformed_seal"
	requestCount := len(f.requests)
	f.mu.Unlock()
	restarted := f.restart(t, nil)
	if err := restarted.Run(t.Context(), f.claim.Identity.RunID, f.recoveryRequest()); !errors.Is(err, ErrProtocol) {
		t.Fatalf("malformed seal response was not refused: %v", err)
	}
	local, err = restarted.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	recording := local.record.Host.Recording
	if local.record.Phase != phaseAcknowledged || recording.SealAccepted || !hostRecordingPending(local.record.Host) || !reflect.DeepEqual(*recording.Seal, original) {
		t.Fatal("malformed ACK changed the accepted proof or original pending seal")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, request := range f.requests[requestCount:] {
		if request.path != runPath(f.claim.Identity.RunID, "checkpoints") && request.path != runPath(f.claim.Identity.RunID, "recording/seal") {
			t.Fatalf("malformed receipt recovery performed a new operation: %s", request.path)
		}
	}
}

func TestHostRecordingRecoveryBoundsTheWholeCheckpointPass(t *testing.T) {
	ledger := newHostTestLedger(t)
	local := registerHostTestRun(t, ledger)
	defer local.Close()
	permit := hostTestPermit(t, local)
	stream := uuid.NewString()
	if err := local.startHostRecording(stream); err != nil {
		t.Fatal(err)
	}
	identity, token := local.record.Identity, local.record.Host.BusinessToken
	for sequence := 1; sequence <= 64; sequence++ {
		p := protocol.CollaborationRecordingCheckpoint{Version: 1, Identity: identity, AttemptSeq: permit.AttemptSeq,
			AttemptNonce: permit.AttemptNonce, StreamID: stream, Sequence: sequence, MessageOffset: sequence - 1,
			Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "retained message"}}}
		if err := local.stageHostCheckpoint(p, token); err != nil {
			t.Fatal(err)
		}
	}
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != runPath(identity.RunID, "checkpoints") || r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("prefix recovery invoked a different operation or credential")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var p protocol.CollaborationRecordingCheckpoint
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests.Add(1)
		timer := time.NewTimer(40 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: p.Identity,
			StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: p.MessageOffset + len(p.Messages), Accepted: true})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	err = (&Runner{Client: client}).recoverHostRecording(ctx, local)
	if !errors.Is(err, ErrTransport) || requests.Load() == 0 || requests.Load() >= 64 || ctx.Err() != nil {
		t.Fatalf("checkpoint pass did not leave a bounded original prefix pending: requests=%d, error=%v", requests.Load(), err)
	}
	through := hostRecordingAcknowledged(local.record.Host.Recording)
	if through == 0 || through >= 64 || local.record.Host.Recording.Chunks[through].Payload == nil {
		t.Fatal("bounded replay lost the first still-uncertain original checkpoint")
	}
}
