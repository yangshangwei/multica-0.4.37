package collaborationrun

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// recordingServer keeps the server rules that decide whether recorded content
// survives: exact original keys are idempotent, new content needs the current
// attempt's capability and the next contiguous offset, nothing new follows an
// outcome or seal, and an outcome's tail is deduplicated by offset.
type recordingServer struct {
	t            *testing.T
	mu           sync.Mutex
	run          string
	tokens       map[string]string
	authorized   string
	keys         map[int]protocol.CollaborationRecordingCheckpoint
	messages     []protocol.CollaborationRecordedMessage
	outcome      *protocol.CollaborationRunOutcome
	seals        []protocol.CollaborationRecordingSeal
	checkpoints  []protocol.CollaborationRecordingCheckpoint
	down         bool
	refuseStatus int
	refuseCode   string
}

func newRecordingServer(t *testing.T, local *RunLedger) (*recordingServer, *Runner) {
	t.Helper()
	s := &recordingServer{t: t, run: local.runID, tokens: map[string]string{}, keys: map[int]protocol.CollaborationRecordingCheckpoint{}}
	s.issue(local)
	server := httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, &Runner{Client: client}
}

// issue mirrors a server permit: the newest authorized attempt and its token
// become current, and older attempts' tokens stay valid only for replay.
func (s *recordingServer) issue(local *RunLedger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	launch, found := lastAuthorizedRecordingLaunch(local.record.Host)
	if !found {
		s.t.Fatal("fixture has no authorized launch")
	}
	s.authorized = launch.AttemptSeq
	s.tokens[launch.AttemptSeq] = local.record.Host.BusinessToken
}

func (s *recordingServer) set(apply func(*recordingServer)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	apply(s)
}

func (s *recordingServer) snapshot() (committed int, checkpoints []protocol.CollaborationRecordingCheckpoint, seals []protocol.CollaborationRecordingSeal, outcome *protocol.CollaborationRunOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages), append([]protocol.CollaborationRecordingCheckpoint(nil), s.checkpoints...), append([]protocol.CollaborationRecordingSeal(nil), s.seals...), s.outcome
}

func refuse(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
}

func (s *recordingServer) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	switch {
	case r.URL.Path == runPath(s.run, "checkpoints"):
		s.checkpoint(w, r, token)
	case r.URL.Path == runPath(s.run, "outcome"):
		s.recordOutcome(w, r, token)
	case r.URL.Path == runPath(s.run, "recording/seal"):
		var p protocol.CollaborationRecordingSeal
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || protocol.ValidateCollaborationRecordingSeal(p) != nil {
			s.t.Errorf("invalid seal: %v", err)
			refuse(w, http.StatusBadRequest, "invalid_recording_seal")
			return
		}
		s.seals = append(s.seals, p)
		observed := p.ObservedThrough
		if observed != nil && *observed < len(s.messages) {
			n := len(s.messages)
			observed = &n
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: p.StreamID, RetainedThrough: len(s.messages), ObservedThrough: observed, Sealed: true, Reason: p.Reason})
	case strings.HasSuffix(r.URL.Path, "/termination"):
		var p protocol.CollaborationLaunchTermination
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			s.t.Error(err)
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationLaunchTerminationReceipt{ProtocolVersion: 3, Identity: p.Identity, LaunchID: p.LaunchID, EventID: p.EventID, Accepted: true, Authorized: true})
	case r.URL.Path == runPath(s.run, "proof"):
		var p protocol.CollaborationRunProof
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			s.t.Error(err)
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ProtocolVersion: 3, Identity: p.Identity, ReceiptID: p.ReceiptID, Accepted: true})
	default:
		s.t.Errorf("unexpected request: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *recordingServer) checkpoint(w http.ResponseWriter, r *http.Request, token string) {
	var p protocol.CollaborationRecordingCheckpoint
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || protocol.ValidateCollaborationRecordingCheckpoint(p) != nil {
		s.t.Errorf("invalid checkpoint: %v", err)
		refuse(w, http.StatusBadRequest, "invalid_recording_checkpoint")
		return
	}
	s.checkpoints = append(s.checkpoints, p)
	if s.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if s.tokens[p.AttemptSeq] != token {
		refuse(w, http.StatusForbidden, "business_capability_required")
		return
	}
	accept := func() {
		_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: p.Identity, StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: p.MessageOffset + len(p.Messages), Accepted: true})
	}
	if saved, ok := s.keys[p.Sequence]; ok {
		if !reflect.DeepEqual(saved, p) {
			refuse(w, http.StatusConflict, "checkpoint_conflict")
			return
		}
		accept()
		return
	}
	if s.refuseStatus != 0 {
		refuse(w, s.refuseStatus, s.refuseCode)
		return
	}
	if p.AttemptSeq != s.authorized {
		refuse(w, http.StatusForbidden, "business_capability_required")
		return
	}
	if len(s.seals) != 0 || s.outcome != nil || p.Sequence != len(s.keys)+1 || p.MessageOffset != len(s.messages) {
		refuse(w, http.StatusConflict, "checkpoint_out_of_order")
		return
	}
	s.keys[p.Sequence] = p
	s.messages = append(s.messages, p.Messages...)
	accept()
}

func (s *recordingServer) recordOutcome(w http.ResponseWriter, r *http.Request, token string) {
	var p protocol.CollaborationRunOutcome
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		s.t.Error(err)
		return
	}
	if s.outcome != nil {
		if !reflect.DeepEqual(*s.outcome, p) {
			refuse(w, http.StatusConflict, "outcome_conflict")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if s.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if p.AttemptSeq != s.authorized || s.tokens[p.AttemptSeq] != token {
		refuse(w, http.StatusConflict, "stale_execution")
		return
	}
	if p.Record == nil || p.Record.Version != 2 || p.Record.MessageOffset > len(s.messages) {
		refuse(w, http.StatusConflict, "recording_stream_conflict")
		return
	}
	for i, m := range p.Record.Messages {
		if n := p.Record.MessageOffset + i; n < len(s.messages) && !reflect.DeepEqual(s.messages[n], m) {
			refuse(w, http.StatusConflict, "recording_stream_conflict")
			return
		}
	}
	for i, m := range p.Record.Messages {
		if p.Record.MessageOffset+i >= len(s.messages) {
			s.messages = append(s.messages, m)
		}
	}
	s.outcome = &p
	w.WriteHeader(http.StatusNoContent)
}

func recordingText(i int) protocol.CollaborationRecordedMessage {
	return protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: fmt.Sprintf("ordinal-%d", i)}
}

func waitForCommitted(t *testing.T, s *recordingServer, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if committed, _, _, _ := s.snapshot(); committed == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	committed, _, _, _ := s.snapshot()
	t.Fatalf("server retained %d messages during the run, want %d", committed, want)
}

// REC-1: one transient failure must not stop uploads for the rest of the run.
// The original chunk is retried with its exact key before later chunks.
func TestHostCheckpointTransientFailureRetriesDuringRun(t *testing.T) {
	ledger := newHostTestLedger(t)
	local := registerHostTestRun(t, ledger)
	defer local.Close()
	hostTestPermit(t, local)
	server, runner := newRecordingServer(t, local)
	server.set(func(s *recordingServer) { s.down = true })
	record := &hostRecording{}
	sender, err := startHostCheckpoints(t.Context(), &hostSupervisor{runner: runner, local: local}, record, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 16 {
		record.add(recordingText(i))
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, checkpoints, _, _ := server.snapshot(); len(checkpoints) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first checkpoint was never sent")
		}
		time.Sleep(10 * time.Millisecond)
	}
	server.set(func(s *recordingServer) { s.down = false })
	for i := 16; i < 32; i++ {
		record.add(recordingText(i))
	}
	// Both chunks must reach the server while the Provider is still running,
	// not only in the bounded post-run recovery.
	waitForCommitted(t, server, 32)
	sender.finish()
	if sender.failed.Load() || hostRecordingChunksPending(local.record.Host.Recording) {
		t.Fatal("a repaired transient failure left the recording failed or pending")
	}
	_, checkpoints, _, _ := server.snapshot()
	if len(checkpoints) < 3 || !reflect.DeepEqual(checkpoints[0], checkpoints[1]) {
		t.Fatal("the failed chunk was not retried first with its original key and payload")
	}
	outcome, ok := record.outcome(local.record.Identity, local.record.AttemptSeq, local.record.AttemptNonce, agent.Result{Status: "completed"})
	if !ok || outcome.Record.Completeness != "complete" || outcome.Record.MessageOffset != 32 || len(outcome.Record.Messages) != 0 {
		t.Fatalf("delivered recording was not reported complete: %+v", outcome.Record)
	}
}

// REC-1/REC-4: chunks that are still unacknowledged when the Provider ends are
// carried in the outcome. A later replay or the outcome itself retains them,
// so a transient outage leaves neither a lost tail nor a permanent partial.
func TestHostOutcomeCarriesUndeliveredTail(t *testing.T) {
	for _, tc := range []struct {
		name         string
		outcomeFirst bool
	}{
		{name: "replay_before_outcome"},
		{name: "outcome_committed_before_replay", outcomeFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := newHostTestLedger(t)
			local := registerHostTestRun(t, ledger)
			defer local.Close()
			hostTestPermit(t, local)
			server, runner := newRecordingServer(t, local)
			runner.Ledger = ledger
			server.set(func(s *recordingServer) { s.down = true })
			record := &hostRecording{}
			sender, err := startHostCheckpoints(t.Context(), &hostSupervisor{runner: runner, local: local}, record, 1)
			if err != nil {
				t.Fatal(err)
			}
			for i := range 20 {
				record.add(recordingText(i))
			}
			sender.finish()
			outcome, ok := record.outcome(local.record.Identity, local.record.AttemptSeq, local.record.AttemptNonce, agent.Result{Status: "completed", Output: "done"})
			if !ok || outcome.Record.Completeness != "complete" || outcome.Record.MessageOffset != 0 || len(outcome.Record.Messages) != 20 {
				t.Fatalf("outcome dropped the undelivered tail or called a pending upload a loss: %+v", outcome.Record)
			}
			if err := local.recordHostOutcome(outcome); err != nil {
				t.Fatal(err)
			}
			server.set(func(s *recordingServer) {
				s.down = false
				if tc.outcomeFirst {
					// The outcome committed but its ACK was lost before a restart.
					s.outcome, s.messages = &outcome, append([]protocol.CollaborationRecordedMessage(nil), outcome.Record.Messages...)
				}
			})
			if err := runner.recoverHost(t.Context(), local); err != nil {
				t.Fatal(err)
			}
			committed, _, seals, saved := server.snapshot()
			if committed != 20 || saved == nil || !local.record.Host.OutcomeRecorded || hostRecordingPending(local.record.Host) {
				t.Fatalf("recovery lost recorded content: committed=%d outcome=%v pending=%v", committed, saved != nil, hostRecordingPending(local.record.Host))
			}
			if len(seals) == 0 {
				t.Fatal("recording was never sealed")
			}
			seal := seals[len(seals)-1]
			if seal.Reason != "completed" || seal.AcknowledgedThrough != 20 || seal.ObservedThrough == nil || *seal.ObservedThrough != 20 {
				t.Fatalf("fully retained recording sealed as %q acknowledged=%d", seal.Reason, seal.AcknowledgedThrough)
			}
			for _, c := range local.record.Host.Recording.Chunks {
				if !c.Accepted {
					t.Fatalf("chunk %d stayed unresolved or was refused although its content is retained", c.Sequence)
				}
			}
		})
	}
}

// fallbackToSecondAttempt performs the ledger steps of a fresh-session
// fallback: the first launch ends and the server issues attempt 2.
func fallbackToSecondAttempt(t *testing.T, local *RunLedger, first protocol.CollaborationRunPermitRequest) {
	t.Helper()
	if err := local.burnHost(first.AttemptNonce); err != nil {
		t.Fatal(err)
	}
	if err := local.recordHostProcess(first.AttemptNonce, "handle:"+uuid.NewString(), "process_group", 1234); err != nil {
		t.Fatal(err)
	}
	if err := local.observeHostTermination(first.AttemptNonce, "managed_launch_ended", protocol.CollaborationCleanupObservation{State: "unverified", Scope: "process_group"}, nil); err != nil {
		t.Fatal(err)
	}
	hostTestAcknowledge(t, local, true)
	p, err := local.requestHostPermit("provider")
	if err != nil {
		t.Fatal(err)
	}
	if err := local.authorizeHost(protocol.CollaborationRunPermitResponse{Identity: p.Identity, AttemptSeq: p.AttemptSeq, AttemptNonce: p.AttemptNonce, BusinessToken: "mcb_" + strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
}

// REC-2: a chunk staged under attempt 1 that the server never committed is
// rebound to the fallback attempt instead of closing the run-wide stream.
func TestHostCheckpointUncommittedChunkRebindsToFallbackAttempt(t *testing.T) {
	ledger := newHostTestLedger(t)
	local := registerHostTestRun(t, ledger)
	defer local.Close()
	first := hostTestPermit(t, local)
	server, runner := newRecordingServer(t, local)
	server.set(func(s *recordingServer) { s.down = true })
	supervisor := &hostSupervisor{runner: runner, local: local}
	record := &hostRecording{}
	sender, err := startHostCheckpoints(t.Context(), supervisor, record, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 16 {
		record.add(recordingText(i))
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, checkpoints, _, _ := server.snapshot(); len(checkpoints) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first checkpoint was never sent")
		}
		time.Sleep(10 * time.Millisecond)
	}
	func() {
		supervisor.mu.Lock()
		defer supervisor.mu.Unlock()
		fallbackToSecondAttempt(t, local, first)
	}()
	server.issue(local)
	server.set(func(s *recordingServer) { s.down = false })
	for i := 16; i < 32; i++ {
		record.add(recordingText(i))
	}
	waitForCommitted(t, server, 32)
	sender.finish()
	rec := local.record.Host.Recording
	if sender.failed.Load() || rec.Closed || rec.Reason != "" || hostRecordingChunksPending(rec) {
		t.Fatalf("fallback closed the recording: closed=%v reason=%q", rec.Closed, rec.Reason)
	}
	_, checkpoints, _, _ := server.snapshot()
	var rebound *protocol.CollaborationRecordingCheckpoint
	for i := range checkpoints {
		if checkpoints[i].Sequence == 1 && checkpoints[i].AttemptSeq == "2" {
			rebound = &checkpoints[i]
		}
	}
	if checkpoints[0].AttemptSeq != "1" || rebound == nil || rebound.StreamID != checkpoints[0].StreamID || rebound.MessageOffset != 0 || !reflect.DeepEqual(rebound.Messages, checkpoints[0].Messages) {
		t.Fatal("the uncommitted chunk was not resent under the fallback attempt with its original offset and messages")
	}
}

// REC-3: the seal reason names the refusal; not every definitive 4xx is a
// loss of authority.
func TestHostCheckpointRefusalNamesSealReason(t *testing.T) {
	for _, tc := range []struct {
		code   string
		status int
		want   string
	}{
		{"execution_stopped", http.StatusConflict, "stopped"},
		{"business_capability_required", http.StatusForbidden, "authority_lost"},
		{"issue_paused", http.StatusConflict, "authority_lost"},
		{"checkpoint_conflict", http.StatusConflict, "incomplete"},
		{"checkpoint_out_of_order", http.StatusConflict, "incomplete"},
		{"invalid_recording_checkpoint", http.StatusBadRequest, "incomplete"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			_, local := retainedHostRecordingFixture(t, 2)
			defer local.Close()
			server, runner := newRecordingServer(t, local)
			server.set(func(s *recordingServer) { s.refuseStatus, s.refuseCode = tc.status, tc.code })
			if err := runner.recoverHostRecording(t.Context(), local); err != nil {
				t.Fatal(err)
			}
			rec := local.record.Host.Recording
			if !rec.Chunks[0].Rejected || !rec.Chunks[1].Rejected || !rec.Closed || rec.Reason != tc.want {
				t.Fatalf("refusal %s: closed=%v reason=%q, want %q", tc.code, rec.Closed, rec.Reason, tc.want)
			}
		})
	}
}

// The carried tail is trimmed from its end to fit the outcome JSON bound, and
// a maximal claim, maximal pending chunks and that outcome still fit the ledger.
func TestHostOutcomeTailFitsRecordAndLedgerBounds(t *testing.T) {
	ledger := newHostTestLedger(t)
	local := registerHostTestRun(t, ledger)
	defer local.Close()
	hostTestPermit(t, local)
	next, err := local.recordingNext()
	if err != nil {
		t.Fatal(err)
	}
	claim := *next.Host.Claim
	claim.Inputs = append([]protocol.CollaborationRunInput{}, claim.Inputs...)
	claim.Inputs[0].Body = strings.Repeat("a", (2<<20)-(64<<10))
	next.Host.Claim = &claim
	if err := local.persist(next); err != nil {
		t.Fatal(err)
	}
	server, runner := newRecordingServer(t, local)
	server.set(func(s *recordingServer) { s.down = true })
	record := &hostRecording{}
	sender, err := startHostCheckpoints(t.Context(), &hostSupervisor{runner: runner, local: local}, record, 1)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("<", protocol.CollaborationRecordTextLimit)
	for range 8 {
		record.add(protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: body})
	}
	sender.finish()
	if len(local.record.Host.Recording.Chunks) != 8 {
		t.Fatalf("staged %d chunks, want 8", len(local.record.Host.Recording.Chunks))
	}
	outcome, ok := record.outcome(local.record.Identity, local.record.AttemptSeq, local.record.AttemptNonce, agent.Result{Status: "completed"})
	if !ok {
		t.Fatal("an oversized tail made the outcome unrecordable")
	}
	if n := len(outcome.Record.Messages); n == 0 || n == 8 || outcome.Record.Completeness != "partial" || outcome.Record.MessageOffset != 0 {
		t.Fatalf("tail was not trimmed to a partial contiguous prefix: messages=%d completeness=%s", n, outcome.Record.Completeness)
	}
	for _, m := range outcome.Record.Messages {
		if m.Body != body {
			t.Fatal("trimmed tail changed a carried message")
		}
	}
	if err := local.recordHostOutcome(outcome); err != nil {
		t.Fatalf("trimmed outcome did not fit the ledger: %v", err)
	}
	data, err := os.ReadFile(local.path)
	if err != nil || len(data) > maxLedgerBytes {
		t.Fatalf("ledger size %d exceeds %d: %v", len(data), maxLedgerBytes, err)
	}
}
