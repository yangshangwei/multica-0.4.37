package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// prepareRestartRecord leaves a durable record at phase, as a crash would.
func prepareRestartRecord(t *testing.T, l *Ledger, phase string) {
	t.Helper()
	r := registerTestRun(t, l)
	if phase != phaseRegistered {
		if err := r.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); err != nil {
			t.Fatal(err)
		}
	}
	if phase != phaseRegistered && phase != phasePermitPending {
		if err := r.Authorize(); err != nil {
			t.Fatal(err)
		}
	}
	if phase == phaseBurned || phase == phaseRecorded {
		if err := r.Burn(); err != nil {
			t.Fatal(err)
		}
	}
	if phase == phaseRecorded {
		if err := r.RecordProcess("pid:[123]"); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}

// Before the burn nothing can have forked, so restart proves nonlaunch through
// the termination capability only. It never claims, permits or reads content.
func TestRunnerRestartConvergesPreBurnRecordsToNeverLaunched(t *testing.T) {
	for _, phase := range []string{phaseRegistered, phasePermitPending, phaseAuthorized} {
		t.Run(phase, func(t *testing.T) {
			l := newTestLedger(t)
			prepareRestartRecord(t, l, phase)
			entries, err := l.Retained()
			if err != nil || len(entries) != 1 || entries[0].Unconfirmed || !entries[0].CanProve {
				t.Fatalf("pre-burn record must not retain capacity: %+v %v", entries, err)
			}
			wantSeq, wantNonce := "1", "00000000-0000-4000-8000-000000000011"
			if phase == phaseRegistered {
				wantSeq, wantNonce = "0", ""
			}
			proofs := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != runPath(testIdentity().RunID, "proof") || r.Header.Get("Authorization") != "Bearer supervisor-private-token" {
					t.Errorf("restart used %s with a non-termination credential", r.URL.Path)
					w.WriteHeader(http.StatusTeapot)
					return
				}
				var proof protocol.CollaborationRunProof
				if err := json.NewDecoder(r.Body).Decode(&proof); err != nil {
					t.Error(err)
				}
				proofs++
				if proof.State != "never_launched_proven" || proof.ExitCode != nil || proof.LastAuthorizedAttemptSeq != wantSeq || proof.SealedAttemptSeq != wantSeq || proof.AttemptNonce != wantNonce {
					t.Errorf("pre-burn restart proof: %+v", proof)
				}
				_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ReceiptID: proof.ReceiptID, Identity: proof.Identity, Accepted: true})
			}))
			defer s.Close()
			client, err := NewClient(s.URL, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			// Proof recovery runs without a launcher, exactly as recoverProofs does.
			runner := &Runner{Ledger: l, Client: client}
			if err := runner.Run(context.Background(), testIdentity().RunID, testClaimRequest()); err != nil {
				t.Fatalf("pre-burn restart did not converge: %v", err)
			}
			if proofs != 1 {
				t.Fatalf("expected one never-launched proof, got %d", proofs)
			}
			if entries, err = l.Retained(); err != nil || len(entries) != 0 {
				t.Fatalf("acknowledged tombstone still retained: %+v %v", entries, err)
			}
		})
	}
}

func TestRunnerRestartNeverRepeatsUncertainPermitOrFork(t *testing.T) {
	for _, phase := range []string{phaseBurned, phaseRecorded} {
		t.Run(phase, func(t *testing.T) {
			l := newTestLedger(t)
			prepareRestartRecord(t, l, phase)
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("restart read business data or requested a new attempt")
			}))
			defer s.Close()
			client, err := NewClient(s.URL, "mdt_test", nil)
			if err != nil {
				t.Fatal(err)
			}
			runner := &Runner{Ledger: l, Client: client, Controller: &NamespaceController{}}
			if err := runner.Run(context.Background(), testIdentity().RunID, testClaimRequest()); !errors.Is(err, ErrUnconfirmed) {
				t.Fatalf("restart erased barrier: %v", err)
			}
		})
	}
}

func TestRunnerReplaysSameSealedProofAfterLostHTTPResponse(t *testing.T) {
	l := newTestLedger(t)
	r := registerTestRun(t, l)
	if err := r.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); err != nil {
		t.Fatal(err)
	}
	if err := r.Authorize(); err != nil {
		t.Fatal(err)
	}
	if err := r.NeverLaunched(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Seal(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	var previous protocol.CollaborationRunProof
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != runPath(testIdentity().RunID, "proof") || r.Header.Get("Authorization") != "Bearer supervisor-private-token" {
			t.Error("proof used non-termination route or credential")
		}
		var proof protocol.CollaborationRunProof
		if err := json.NewDecoder(r.Body).Decode(&proof); err != nil {
			t.Error(err)
		}
		calls++
		if calls == 1 {
			previous = proof
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		if proof.ReceiptID != previous.ReceiptID || proof.Identity != previous.Identity || proof.AttemptNonce != previous.AttemptNonce {
			t.Error("proof replay changed identity")
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ReceiptID: proof.ReceiptID, Identity: proof.Identity, Accepted: true})
	}))
	defer s.Close()
	client, err := NewClient(s.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{Ledger: l, Client: client}
	if err := runner.Run(context.Background(), testIdentity().RunID, testClaimRequest()); !errors.Is(err, ErrProofPending) {
		t.Fatalf("lost ack: %v", err)
	}
	if err := runner.Run(context.Background(), testIdentity().RunID, testClaimRequest()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected original proof replay, got %d requests", calls)
	}
}
