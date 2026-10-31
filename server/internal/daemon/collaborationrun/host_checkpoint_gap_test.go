package collaborationrun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// These cases exercise the producer and sender together: a local rejected batch
// loses data, while a failed HTTP receipt still has an exact durable payload.
func TestHostCheckpointLocalFailureCannotRebaseQueuedMessages(t *testing.T) {
	for _, tc := range []struct {
		name        string
		prefix      bool
		loseACK     bool
		invalid     bool
		wantChunks  int
		wantThrough int
	}{
		{name: "invalid_first_batch", invalid: true},
		{name: "accepted_prefix_invalid_middle", prefix: true, invalid: true, wantChunks: 1, wantThrough: 16},
		{name: "uncertain_prefix_invalid_middle", prefix: true, loseACK: true, invalid: true, wantChunks: 1, wantThrough: 16},
		{name: "uncertain_prefix_valid_suffix", prefix: true, loseACK: true, wantChunks: 3, wantThrough: 48},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := newHostTestLedger(t)
			local := registerHostTestRun(t, ledger)
			defer local.Close()
			permit := hostTestPermit(t, local)
			businessToken, terminationToken := local.record.Host.BusinessToken, local.record.TerminationToken
			var mu sync.Mutex
			var checkpoints []protocol.CollaborationRecordingCheckpoint
			var seals []protocol.CollaborationRecordingSeal
			proofs, retained := 0, 0
			prefixStarted, releasePrefix := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(releasePrefix) }) }
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case runPath(permit.Identity.RunID, "checkpoints"):
					var p protocol.CollaborationRecordingCheckpoint
					if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
						t.Error(err)
						return
					}
					if protocol.ValidateCollaborationRecordingCheckpoint(p) != nil || p.Identity != permit.Identity || p.AttemptSeq != permit.AttemptSeq || p.AttemptNonce != permit.AttemptNonce || r.Header.Get("Authorization") != "Bearer "+businessToken {
						t.Error("checkpoint changed its original issued identity or credential")
					}
					mu.Lock()
					checkpoints = append(checkpoints, p)
					first := len(checkpoints) == 1
					retained = max(retained, p.MessageOffset+len(p.Messages))
					mu.Unlock()
					if tc.prefix && first {
						close(prefixStarted)
						<-releasePrefix
						if tc.loseACK {
							w.WriteHeader(http.StatusServiceUnavailable)
							return
						}
					}
					_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: p.Identity, StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: p.MessageOffset + len(p.Messages), Accepted: true})
				case runPath(permit.Identity.RunID, "launches/"+permit.AttemptNonce+"/termination"):
					var p protocol.CollaborationLaunchTermination
					if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
						t.Error(err)
						return
					}
					_ = json.NewEncoder(w).Encode(protocol.CollaborationLaunchTerminationReceipt{ProtocolVersion: 3, Identity: p.Identity, LaunchID: p.LaunchID, EventID: p.EventID, Accepted: true, Authorized: true})
				case runPath(permit.Identity.RunID, "proof"):
					var p protocol.CollaborationRunProof
					if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
						t.Error(err)
						return
					}
					mu.Lock()
					proofs++
					mu.Unlock()
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ProtocolVersion: 3, Identity: p.Identity, ReceiptID: p.ReceiptID, Accepted: true})
				case runPath(permit.Identity.RunID, "recording/seal"):
					var p protocol.CollaborationRecordingSeal
					if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
						t.Error(err)
						return
					}
					if p.Identity != permit.Identity || p.AttemptSeq != permit.AttemptSeq || p.AttemptNonce != permit.AttemptNonce || r.Header.Get("Authorization") != "Bearer "+terminationToken {
						t.Error("metadata seal changed its original termination identity")
					}
					mu.Lock()
					seals = append(seals, p)
					through := retained
					mu.Unlock()
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: p.StreamID, RetainedThrough: through, ObservedThrough: p.ObservedThrough, Sealed: true, Reason: p.Reason})
				default:
					t.Errorf("unexpected sender recovery request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			defer unblock()
			client, err := NewClient(server.URL, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			runner := &Runner{Client: client, Ledger: ledger}
			supervisor := &hostSupervisor{runner: runner, local: local}
			record := &hostRecording{}
			sender, err := startHostCheckpoints(t.Context(), supervisor, record, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer sender.finish()
			activity := &HostActivity{}
			message := func(i int) {
				record.message(agent.Message{Type: agent.MessageText, Content: fmt.Sprintf("original-ordinal-%d", i)}, activity)
			}
			start := 0
			if tc.prefix {
				for i := range 16 {
					message(i)
				}
				select {
				case <-prefixStarted:
				case <-time.After(2 * time.Second):
					t.Fatal("prefix was not staged and sent")
				}
				start = 16
			}
			// Keep the full later batch queued when validation rejects the bad
			// batch; otherwise scheduling could hide the offset-rebase defect.
			supervisor.mu.Lock()
			for i := start; i < start+32; i++ {
				if tc.invalid && i == start {
					record.message(agent.Message{Type: agent.MessageToolUse, CallID: "call", Tool: strings.Repeat("n", 129), Input: map[string]any{}}, activity)
				} else {
					message(i)
				}
			}
			supervisor.mu.Unlock()
			unblock()
			drained := make(chan struct{})
			go func() { sender.finish(); close(drained) }()
			select {
			case <-drained:
			case <-time.After(2 * time.Second):
				t.Fatal("local staging failure blocked the sender drain")
			}
			rec := local.record.Host.Recording
			if len(rec.Chunks) != tc.wantChunks {
				t.Errorf("staged %d chunks, want %d: queued messages were rebased after a rejected batch", len(rec.Chunks), tc.wantChunks)
			}
			if tc.prefix && tc.loseACK && (len(rec.Chunks) == 0 || rec.Chunks[0].Payload == nil || rec.Chunks[0].Accepted) {
				t.Fatal("lost prefix receipt discarded its original durable payload")
			}
			if tc.invalid && (!rec.Closed || !sender.failed.Load()) {
				t.Error("local validation failure did not mark the recording incomplete")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			if err := runner.recoverHost(ctx, local); err != nil {
				t.Fatal(err)
			}
			if ctx.Err() != nil || local.record.Phase != phaseAcknowledged || !local.record.Host.Stopped || hostRecordingPending(local.record.Host) {
				t.Fatal("recording gap held original Stop/proof or left settled recovery pending")
			}
			outcome, ok := record.outcome(permit.Identity, permit.AttemptSeq, permit.AttemptNonce, agent.Result{Status: "completed"})
			if !ok || outcome.Record.MessageOffset != tc.wantThrough || len(outcome.Record.Messages) != 0 || tc.invalid && outcome.Record.Completeness != "partial" {
				t.Error("final outcome hid the recording gap or reintroduced its missing suffix")
			}
			mu.Lock()
			defer mu.Unlock()
			for _, p := range checkpoints {
				for i, m := range p.Messages {
					ordinal := p.MessageOffset + i
					if ordinal >= tc.wantThrough || m.Body != fmt.Sprintf("original-ordinal-%d", ordinal) {
						t.Errorf("uploaded rebased/post-gap message: offset=%d body=%q", ordinal, m.Body)
					}
				}
			}
			if tc.loseACK && (len(checkpoints) < 2 || !reflect.DeepEqual(checkpoints[0], checkpoints[1])) {
				t.Error("uncertain prefix did not replay its exact original key and payload")
			}
			if proofs != 1 || len(seals) != 1 || retained != tc.wantThrough {
				t.Errorf("wrong recovered prefix/proof/seal counts: through=%d proofs=%d seals=%d", retained, proofs, len(seals))
			} else if seal := seals[0]; seal.AcknowledgedThrough != tc.wantThrough || tc.invalid && (seal.ObservedThrough != nil || seal.Reason == "completed") || !tc.invalid && (seal.ObservedThrough == nil || *seal.ObservedThrough != tc.wantThrough) {
				t.Error("seal invented a complete tail or lost the known drained prefix")
			}
		})
	}
}
