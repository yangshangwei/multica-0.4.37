package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The launch matrix owns individual handoff refusals. These cases cover only
// their interaction with a negotiated recording stream and original proof.
func TestHostHandoffCheckpointFailureCleanup(t *testing.T) {
	for _, name := range []string{"snapshot_refused", "binding_refused", "fallback_snapshot_drift"} {
		t.Run(name, func(t *testing.T) {
			mode := "result"
			if name == "fallback_snapshot_drift" {
				mode = "reject_resume"
			}
			h := newLaunchHandoffFixture(t, mode)
			h.claim.RecordingVersion = protocol.CollaborationCheckpointVersion
			switch name {
			case "snapshot_refused":
				h.snapshotFailures = []HTTPError{{Status: http.StatusForbidden, Code: "business_capability_required"}}
			case "binding_refused":
				h.bindingFailures = []HTTPError{{Status: http.StatusConflict, Code: "handoff_source_unavailable"}}
			case "fallback_snapshot_drift":
				seedHostResume(t, h.hostRunFixture)
				h.mutateSnapshot = func(v *protocol.CollaborationHandoffEnvelope) {
					if len(h.snapshots) > 1 {
						v.Authority.HandlerID = uuid.NewString()
					}
				}
			}

			var mu sync.Mutex
			var checkpoints []protocol.CollaborationRecordingCheckpoint
			var seals []protocol.CollaborationRecordingSeal
			retained := 0
			original := h.server.Config.Handler
			h.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				checkpoint := r.URL.Path == runPath(h.claim.Identity.RunID, "checkpoints")
				seal := r.URL.Path == runPath(h.claim.Identity.RunID, "recording/seal")
				if !checkpoint && !seal {
					original.ServeHTTP(w, r)
					return
				}
				if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Workspace-ID") != "" {
					t.Error("recording request lost its narrow transport")
					w.WriteHeader(http.StatusForbidden)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if checkpoint {
					var p protocol.CollaborationRecordingCheckpoint
					if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					h.hostRunFixture.mu.Lock()
					permit, issued := h.permits[p.AttemptSeq]
					h.hostRunFixture.mu.Unlock()
					if protocol.ValidateCollaborationRecordingCheckpoint(p) != nil || p.Identity != h.claim.Identity || !issued || permit.AttemptNonce != p.AttemptNonce || r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat(p.AttemptSeq, 64) {
						t.Error("checkpoint lost its issued attempt or business credential")
						w.WriteHeader(http.StatusForbidden)
						return
					}
					checkpoints = append(checkpoints, p)
					retained = max(retained, p.MessageOffset+len(p.Messages))
					if name == "fallback_snapshot_drift" && len(checkpoints) == 1 {
						// The server retained the prefix; only its first ACK is lost.
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						if err := conn.Close(); err != nil {
							t.Error(err)
						}
						return
					}
					_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: p.Identity, StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: retained, Accepted: true})
					return
				}
				var p protocol.CollaborationRecordingSeal
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if protocol.ValidateCollaborationRecordingSeal(p) != nil || p.Identity != h.claim.Identity || r.Header.Get("Authorization") != "Bearer "+h.claim.TerminationToken {
					t.Error("recording seal lost its original termination authority")
					w.WriteHeader(http.StatusForbidden)
					return
				}
				seals = append(seals, p)
				_ = json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: p.StreamID, RetainedThrough: retained, ObservedThrough: p.ObservedThrough, Sealed: true, Reason: p.Reason})
			})

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			err := h.runner.Run(ctx, h.claim.Identity.RunID, h.request)
			if ctx.Err() != nil {
				t.Fatalf("handoff failure did not finish recording and proof within the deadline: %v", err)
			}
			wantStarts := 0
			if name == "fallback_snapshot_drift" {
				wantStarts = 1
				if !errors.Is(err, ErrProtocol) {
					t.Fatalf("fallback did not preserve its snapshot refusal: %v", err)
				}
			}
			if starts := len(h.starts(t)); starts != wantStarts {
				t.Fatalf("started %d providers after handoff failure, want %d", starts, wantStarts)
			}
			h.hostRunFixture.mu.Lock()
			proofs, terminations, permits := len(h.proofs), len(h.terminations), len(h.permits)
			h.hostRunFixture.mu.Unlock()
			if proofs != 1 || terminations != 1 || permits != 1 {
				t.Fatalf("original cleanup changed: proofs=%d terminations=%d permits=%d", proofs, terminations, permits)
			}
			h.hostRunFixture.mu.Lock()
			permit, termination, proof := h.permits["1"], h.terminations["1"], h.proofs[0]
			h.hostRunFixture.mu.Unlock()
			wantTermination, wantProof := "not_started", "never_launched_proven"
			if name == "fallback_snapshot_drift" {
				wantTermination, wantProof = "managed_launch_ended", "host_exit_observed"
			}
			if termination.Identity != permit.Identity || termination.AttemptSeq != permit.AttemptSeq || termination.AttemptNonce != permit.AttemptNonce || termination.Kind != wantTermination {
				t.Fatalf("handoff failure changed the original attempt or termination fact: %+v", termination)
			}
			if proof.Identity != permit.Identity || proof.State != wantProof || len(proof.Launches) != 1 || !reflect.DeepEqual(proof.Launches[0], termination) {
				t.Fatal("proof did not preserve the same original termination fact")
			}
			local, err := h.runner.Ledger.Lock(h.claim.Identity.DeploymentID, h.claim.Identity.InstallationID, h.claim.Identity.RunID)
			if err != nil {
				t.Fatal(err)
			}
			defer local.Close()
			if local.record.Phase != phaseAcknowledged || !local.record.Host.Stopped {
				t.Fatal("handoff failure lost its stopped state or original proof ACK")
			}
			mu.Lock()
			defer mu.Unlock()
			recording := local.record.Host.Recording
			if name == "snapshot_refused" {
				if recording != nil || len(checkpoints) != 0 || len(seals) != 0 {
					t.Fatal("snapshot refusal created a recording stream before startup validation")
				}
				return
			}
			if recording == nil || !recording.SealAccepted || len(seals) != 1 {
				t.Errorf("recording stream was not sealed after handoff failure: recording=%+v seal_requests=%d", recording, len(seals))
			}
			if name == "fallback_snapshot_drift" {
				if len(checkpoints) != 2 || !reflect.DeepEqual(checkpoints[0], checkpoints[1]) {
					t.Fatalf("lost checkpoint ACK was not recovered with the original payload: requests=%d", len(checkpoints))
				}
				if recording == nil || len(recording.Chunks) != 1 || !recording.Chunks[0].Accepted || recording.Chunks[0].Payload != nil {
					t.Fatal("checkpoint receipt recovery did not clear the acknowledged payload")
				}
			}
		})
	}
}
