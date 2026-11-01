package collaborationrun

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

type handoffCheckpointLifecycleFixture struct {
	*launchHandoffFixture
	mu          sync.Mutex
	mode        string
	release     chan struct{}
	started     chan struct{}
	proof       chan struct{}
	startOnce   sync.Once
	proofOnce   sync.Once
	releaseOnce sync.Once
	checkpoints []protocol.CollaborationRecordingCheckpoint
	seals       []protocol.CollaborationRecordingSeal
	claims      int
	snapshots   int
	retained    int
}

func newHandoffCheckpointLifecycleFixture(t *testing.T, mode string) *handoffCheckpointLifecycleFixture {
	t.Helper()
	h := newLaunchHandoffFixture(t, "result")
	h.claim.RecordingVersion = protocol.CollaborationCheckpointVersion
	f := &handoffCheckpointLifecycleFixture{launchHandoffFixture: h, mode: mode,
		release: make(chan struct{}), started: make(chan struct{}), proof: make(chan struct{})}
	// Registered after the borrowed fixture's cleanup: blocked handlers are
	// explicitly released before httptest.Server.Close waits for them.
	t.Cleanup(f.unblock)
	original := h.server.Config.Handler
	h.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkpoint := r.URL.Path == runPath(h.claim.Identity.RunID, "checkpoints")
		seal := r.URL.Path == runPath(h.claim.Identity.RunID, "recording/seal")
		if !checkpoint && !seal {
			f.mu.Lock()
			// An outage covers the outcome as well. Otherwise the outcome's
			// carried tail retains the chunk and no checkpoint stays pending
			// for the restart path these tests exercise.
			if f.mode == "unavailable" && strings.HasSuffix(r.URL.Path, "/outcome") {
				f.mu.Unlock()
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/claim") {
				f.claims++
			}
			if strings.HasSuffix(r.URL.Path, "/handoff/snapshots") {
				f.snapshots++
			}
			f.mu.Unlock()
			original.ServeHTTP(w, r)
			if strings.HasSuffix(r.URL.Path, "/proof") {
				f.proofOnce.Do(func() { close(f.proof) })
			}
			return
		}
		if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Workspace-ID") != "" {
			t.Error("recording transport escaped its narrow endpoint")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var identity protocol.CollaborationRunIdentity
		var sequence, nonce string
		var p protocol.CollaborationRecordingCheckpoint
		var s protocol.CollaborationRecordingSeal
		if checkpoint {
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil || protocol.ValidateCollaborationRecordingCheckpoint(p) != nil {
				t.Errorf("invalid checkpoint: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			identity, sequence, nonce = p.Identity, p.AttemptSeq, p.AttemptNonce
		} else {
			if err := json.NewDecoder(r.Body).Decode(&s); err != nil || protocol.ValidateCollaborationRecordingSeal(s) != nil {
				t.Errorf("invalid seal: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			identity, sequence, nonce = s.Identity, s.AttemptSeq, s.AttemptNonce
		}
		h.hostRunFixture.mu.Lock()
		permit, issued := h.permits[sequence]
		h.hostRunFixture.mu.Unlock()
		credential := h.claim.TerminationToken
		if checkpoint {
			credential = "mcb_" + strings.Repeat(sequence, 64)
		}
		if !issued || identity != permit.Identity || sequence != permit.AttemptSeq || nonce != permit.AttemptNonce || r.Header.Get("Authorization") != "Bearer "+credential {
			t.Error("recording request changed the actual issued identity, attempt, nonce or credential")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		f.mu.Lock()
		mode := f.mode
		if checkpoint {
			f.checkpoints = append(f.checkpoints, p)
		} else {
			f.seals = append(f.seals, s)
		}
		if mode == "available" && checkpoint {
			f.retained = max(f.retained, p.MessageOffset+len(p.Messages))
		}
		retained := f.retained
		f.mu.Unlock()
		f.startOnce.Do(func() { close(f.started) })
		if mode == "blocked" {
			<-f.release
		}
		if mode != "available" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if checkpoint {
			_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: identity, StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: retained, Accepted: true})
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: s.StreamID, RetainedThrough: retained, ObservedThrough: s.ObservedThrough, Sealed: true, Reason: s.Reason})
	})
	return f
}

func (f *handoffCheckpointLifecycleFixture) unblock() {
	f.releaseOnce.Do(func() { close(f.release) })
}

func TestHostHandoffCheckpointBlockedRecordingCannotHoldProof(t *testing.T) {
	f := newHandoffCheckpointLifecycleFixture(t, "blocked")
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- f.runner.Run(ctx, f.claim.Identity.RunID, f.request)
	}()
	t.Cleanup(func() {
		cancel()
		f.unblock()
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			t.Error("Runner did not exit after bounded fixture teardown")
		}
	})
	select {
	case <-f.started:
	case <-time.After(3 * time.Second):
		t.Fatal("the negotiated recording sender was never exercised")
	}
	select {
	case <-f.proof:
	case <-time.After(5 * time.Second):
		t.Fatal("blocked recording prevented the original proof request before release")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("recording recovery prevented bounded return after proof")
	}
	select {
	case <-f.release:
		t.Fatal("fixture released recording before verifying the proof ACK")
	default:
	}
	local, err := f.runner.Ledger.Lock(f.claim.Identity.DeploymentID, f.claim.Identity.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	if local.record.Phase != phaseAcknowledged || !local.record.Host.Stopped {
		t.Fatal("recording outage prevented durable Stop/proof ACK before fixture release")
	}
	f.hostRunFixture.mu.Lock()
	defer f.hostRunFixture.mu.Unlock()
	permit, termination := f.permits["1"], f.terminations["1"]
	if len(f.proofs) != 1 || termination.Identity != permit.Identity || termination.AttemptSeq != permit.AttemptSeq || termination.AttemptNonce != permit.AttemptNonce {
		t.Fatal("blocked recording changed the original termination identity")
	}
}

func TestHostHandoffCheckpointRestartRecoversWithoutExecution(t *testing.T) {
	f := newHandoffCheckpointLifecycleFixture(t, "unavailable")
	_ = f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
	local, err := f.runner.Ledger.Lock(f.claim.Identity.DeploymentID, f.claim.Identity.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	recording := local.record.Host.Recording
	if local.record.Phase != phaseAcknowledged || recording == nil || len(recording.Chunks) != 1 || recording.Chunks[0].Payload == nil || recording.Chunks[0].Accepted {
		_ = local.Close()
		t.Fatal("fixture did not persist an original uncertain checkpoint and proof ACK")
	}
	original := *recording.Chunks[0].Payload
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	claims, snapshots, requests := f.claims, f.snapshots, len(f.checkpoints)
	f.mode = "available"
	f.mu.Unlock()
	starts := len(f.starts(t))
	f.hostRunFixture.mu.Lock()
	permit, termination := f.permits["1"], f.terminations["1"]
	f.hostRunFixture.mu.Unlock()
	reopened, err := OpenLedger(f.runner.Ledger.dir)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(f.server.URL, "mdt_fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	// A fresh Runner with no executor must recover only the retained receipts.
	restarted := &Runner{Ledger: reopened, Client: client}
	if err := restarted.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claims != claims || f.snapshots != snapshots || len(f.starts(t)) != starts {
		t.Fatal("checkpoint recovery repeated claim, snapshot or provider execution")
	}
	if len(f.checkpoints) <= requests {
		t.Fatal("new Runner did not replay the pending original checkpoint")
	}
	for _, replay := range f.checkpoints[requests:] {
		if !reflect.DeepEqual(replay, original) {
			t.Fatal("restart changed the original checkpoint identity/key/payload")
		}
	}
	if len(f.seals) == 0 {
		t.Fatal("restart did not seal the recording after receipt recovery")
	}
	for _, seal := range f.seals {
		if seal.Identity != permit.Identity || seal.AttemptSeq != permit.AttemptSeq || seal.AttemptNonce != permit.AttemptNonce || seal.StreamID != original.StreamID {
			t.Fatal("recording seal changed the actual issued attempt identity")
		}
	}
	f.hostRunFixture.mu.Lock()
	if !reflect.DeepEqual(f.terminations["1"], termination) || termination.Identity != permit.Identity || termination.AttemptSeq != permit.AttemptSeq || termination.AttemptNonce != permit.AttemptNonce {
		t.Error("restart replaced the original termination identity")
	}
	f.hostRunFixture.mu.Unlock()
	local, err = reopened.Lock(f.claim.Identity.DeploymentID, f.claim.Identity.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	recording = local.record.Host.Recording
	if local.record.Phase != phaseAcknowledged || recording == nil || len(recording.Chunks) != 1 || !recording.SealAccepted || !recording.Chunks[0].Accepted || recording.Chunks[0].Payload != nil {
		t.Fatal("restart did not persist checkpoint/seal ACKs without retaining the body")
	}
}
