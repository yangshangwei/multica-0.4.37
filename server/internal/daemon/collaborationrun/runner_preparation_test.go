package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

type preparationTestLauncher struct {
	root           string
	preparationErr error
	starts         int
	preparations   int
}

func (l *preparationTestLauncher) Probe(context.Context) error { return nil }
func (l *preparationTestLauncher) Prepare(context.Context, ProviderInput, *Client) (*PreparedRun, error) {
	l.preparations++
	if l.preparationErr != nil {
		return nil, l.preparationErr
	}
	return &PreparedRun{root: l.root}, nil
}
func (l *preparationTestLauncher) Start(context.Context, ProviderInput) (*Process, error) {
	l.starts++
	return nil, ErrUnsupported
}

func TestRunnerPreparationFailureKeepsNonceProofAndBurnBoundary(t *testing.T) {
	for _, kind := range []string{"preparation_error", "epoch_changed", "revalidation_error", "unknown_start"} {
		t.Run(kind, func(t *testing.T) {
			request, claim := preparationTestClaim(t)
			proofs, outcomes, claims := 0, 0, 0
			var permit protocol.CollaborationRunPermitRequest
			var proof protocol.CollaborationRunProof
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/claim"):
					claims++
					_ = json.NewEncoder(w).Encode(claim)
				case strings.HasSuffix(r.URL.Path, "/permit"):
					_ = json.NewDecoder(r.Body).Decode(&permit)
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunPermitResponse{Identity: permit.Identity, AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, BusinessToken: "mcb_fixture"})
				case strings.HasSuffix(r.URL.Path, "/preparation"):
					if kind == "revalidation_error" {
						w.WriteHeader(403)
						return
					}
					_ = json.NewEncoder(w).Encode(claim.Preparation)
				case strings.HasSuffix(r.URL.Path, "/stop-intent"):
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunStopIntent{Identity: claim.Identity})
				case strings.HasSuffix(r.URL.Path, "/observations"):
					var fact protocol.CollaborationPreparationObservation
					if json.NewDecoder(r.Body).Decode(&fact) != nil || fact.State != "failed" || fact.AttemptNonce != permit.AttemptNonce || fact.Identity != claim.Identity {
						t.Error("preparation fact changed original attempt identity")
					}
					_ = json.NewEncoder(w).Encode(protocol.CollaborationPreparationObservationReceipt{Identity: fact.Identity, EventID: fact.EventID, ManifestDigest: fact.ManifestDigest, Recorded: true})
				case strings.HasSuffix(r.URL.Path, "/proof"):
					proofs++
					_ = json.NewDecoder(r.Body).Decode(&proof)
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{Identity: proof.Identity, ReceiptID: proof.ReceiptID, Accepted: true})
				case strings.HasSuffix(r.URL.Path, "/outcome"):
					outcomes++
					w.WriteHeader(204)
				default:
					t.Error("unexpected preparation request")
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			root := filepath.Join(t.TempDir(), "private-staging")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			launcher := &preparationTestLauncher{root: root}
			if kind == "preparation_error" {
				launcher.preparationErr = preparationFailure("attachments", "attachment_unavailable", ErrTransport)
			}
			client, err := NewClient(server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			runner := &Runner{Client: client, Ledger: newTestLedger(t), Controller: launcher, RuntimeConfigEpoch: "5"}
			if kind == "epoch_changed" {
				runner.RuntimeConfigEpoch = "6"
			}
			err = runner.Run(t.Context(), claim.Identity.RunID, request)
			if err == nil || outcomes != 0 || claims != 1 {
				t.Fatalf("failure was hidden or fabricated an outcome: %v", err)
			}
			if kind == "unknown_start" {
				if !errors.Is(err, ErrUnconfirmed) || proofs != 0 || launcher.starts != 1 {
					t.Fatal("post-burn start uncertainty lost its barrier")
				}
				if _, err := os.Stat(root); err != nil {
					t.Fatal("unknown live scope lost its staging")
				}
				entries, err := runner.Ledger.Retained()
				if err != nil || len(entries) != 1 || !entries[0].Unconfirmed {
					t.Fatal("burned scope stopped retaining capacity")
				}
			} else if proofs != 1 || launcher.starts != 0 || proof.State != "never_launched_proven" || proof.AttemptNonce != permit.AttemptNonce || proof.SealedAttemptSeq != "1" {
				t.Fatalf("preburn failure changed original proof or started a helper: proofs=%d starts=%d proof=%+v", proofs, launcher.starts, proof)
			}
			_ = runner.Run(t.Context(), claim.Identity.RunID, request)
			if claims != 1 || launcher.starts > 1 || launcher.preparations > 1 {
				t.Fatal("restart retried business preparation or a burned attempt")
			}
		})
	}
}
