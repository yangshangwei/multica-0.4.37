//go:build linux

package collaborationrun

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestLinuxRunnerHTTPOutcomeStopAndRevokedBusinessProof(t *testing.T) {
	for _, mode := range []string{"success", "failure", "exit", "stop_before_fork", "revoked_business", "legacy_outcome"} {
		t.Run(mode, func(t *testing.T) {
			outcomes, proofs := 0, 0
			var attempt protocol.CollaborationRunPermitRequest
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/claim"):
					_ = json.NewEncoder(w).Encode(testClaim())
				case strings.HasSuffix(r.URL.Path, "/permit"):
					if err := json.NewDecoder(r.Body).Decode(&attempt); err != nil {
						t.Error(err)
					}
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunPermitResponse{Identity: attempt.Identity, AttemptSeq: attempt.AttemptSeq, AttemptNonce: attempt.AttemptNonce, BusinessToken: "business-only"})
				case strings.HasSuffix(r.URL.Path, "/stop-intent"):
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunStopIntent{Identity: testIdentity(), StopRequested: mode == "stop_before_fork"})
				case strings.HasSuffix(r.URL.Path, "/outcome"):
					outcomes++
					if r.Header.Get("Authorization") != "Bearer business-only" {
						t.Error("outcome credential")
					}
					if mode == "revoked_business" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					var outcome protocol.CollaborationRunOutcome
					if err := json.NewDecoder(r.Body).Decode(&outcome); err != nil {
						t.Error(err)
					}
					wantOutcome := "success"
					if mode == "failure" {
						wantOutcome = "failure"
					}
					if outcome.Body != "two immutable inputs" || outcome.Outcome != wantOutcome || outcome.Identity != attempt.Identity || outcome.AttemptNonce != attempt.AttemptNonce || outcome.AttemptSeq != attempt.AttemptSeq {
						t.Error("missing explicit business result or attempt identity")
					}
					w.WriteHeader(204)
				case strings.HasSuffix(r.URL.Path, "/proof"):
					proofs++
					if r.Header.Get("Authorization") != "Bearer termination-only" {
						t.Error("proof credential")
					}
					var proof protocol.CollaborationRunProof
					if err := json.NewDecoder(r.Body).Decode(&proof); err != nil {
						t.Error(err)
					}
					if proof.Identity != testIdentity() || proof.AttemptNonce != attempt.AttemptNonce || proof.LastAuthorizedAttemptSeq != "1" || proof.SealedAttemptSeq != "1" {
						t.Error("wrong sealed attempt")
					}
					if mode == "stop_before_fork" && proof.State != "never_launched_proven" {
						t.Error("stop before fork was not durably sealed")
					}
					if mode != "stop_before_fork" && proof.State != "exited_proven" {
						t.Error("missing namespace proof")
					}
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ReceiptID: proof.ReceiptID, Identity: proof.Identity, Accepted: true})
				default:
					t.Error("unexpected runtime path")
					w.WriteHeader(404)
				}
			}))
			defer s.Close()
			providerMode := "success"
			if mode == "exit" || mode == "failure" || mode == "legacy_outcome" {
				providerMode = mode
			}
			c := testController(t, providerMode)
			client, err := NewClient(s.URL, "mdt_test", nil)
			if err != nil {
				t.Fatal(err)
			}
			runner := &Runner{Ledger: newTestLedger(t), Client: client, Controller: c}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err = runner.Run(ctx, testIdentity().RunID, testClaimRequest())
			if mode == "revoked_business" {
				var denied *HTTPError
				if !errors.As(err, &denied) || denied.Status != 401 {
					t.Fatalf("expected revoked result, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if proofs != 1 {
				t.Fatalf("proof count %d", proofs)
			}
			if (mode == "exit" || mode == "stop_before_fork" || mode == "legacy_outcome") && outcomes != 0 {
				t.Fatal("OS exit, accepted Stop or a command terminal fabricated a business result")
			}
			if (mode == "success" || mode == "failure" || mode == "revoked_business") && outcomes != 1 {
				t.Fatalf("outcome count %d", outcomes)
			}
		})
	}
}

func TestLinuxSupervisorCrashClosesNamespaceButKeepsUnknown(t *testing.T) {
	if root := os.Getenv("MULTICA_TEST_COLLAB_CRASH_ROOT"); root != "" {
		ledger, err := OpenLedger(filepath.Join(root, "ledger"))
		if err != nil {
			t.Fatal(err)
		}
		local := registerTestRun(t, ledger)
		if err := local.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); err != nil {
			t.Fatal(err)
		}
		if err := local.Authorize(); err != nil {
			t.Fatal(err)
		}
		if err := local.Burn(); err != nil {
			t.Fatal(err)
		}
		c := &NamespaceController{Executable: os.Getenv("MULTICA_TEST_COLLAB_FAKE"), Directory: filepath.Join(root, "sandboxes"), Arguments: []string{"--collaboration-test-agent", "tree", os.Getenv("MULTICA_TEST_COLLAB_ADDRESS")}}
		process, err := c.Start(context.Background(), ProviderInput{})
		if err != nil {
			t.Fatal(err)
		}
		if os.Getenv("MULTICA_TEST_COLLAB_CRASH_STAGE") == "recorded" {
			if err := local.RecordProcess(process.NamespaceID()); err != nil {
				t.Fatal(err)
			}
		}
		var b [1]byte
		_, _ = os.Stdin.Read(b[:])
		os.Exit(23)
	}
	for _, stage := range []string{"forked", "recorded"} {
		t.Run(stage, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			c := testController(t, "exit")
			root := t.TempDir()
			bin, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, "-test.run=^TestLinuxSupervisorCrashClosesNamespaceButKeepsUnknown$")
			cmd.Env = append(os.Environ(), "MULTICA_TEST_COLLAB_CRASH_ROOT="+root, "MULTICA_TEST_COLLAB_FAKE="+c.Executable, "MULTICA_TEST_COLLAB_ADDRESS="+listener.Addr().String(), "MULTICA_TEST_COLLAB_CRASH_STAGE="+stage)
			input, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			var peers []net.Conn
			for range 2 {
				_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second))
				peer, err := listener.Accept()
				if err != nil {
					t.Fatal(err)
				}
				defer peer.Close()
				_ = peer.SetDeadline(time.Now().Add(5 * time.Second))
				if _, err := bufio.NewReader(peer).ReadString('\n'); err != nil {
					t.Fatal(err)
				}
				peers = append(peers, peer)
			}
			if _, err := input.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
			_ = input.Close()
			if err := cmd.Wait(); err == nil {
				t.Fatal("supervisor did not crash")
			}
			for _, peer := range peers {
				var b [1]byte
				if _, err := peer.Read(b[:]); err != io.EOF {
					t.Fatalf("descendant survived supervisor loss: %v", err)
				}
			}
			ledger, err := OpenLedger(filepath.Join(root, "ledger"))
			if err != nil {
				t.Fatal(err)
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unknown restart called server") }))
			defer s.Close()
			client, err := NewClient(s.URL, "mdt_test", nil)
			if err != nil {
				t.Fatal(err)
			}
			runner := &Runner{Ledger: ledger, Client: client, Controller: c}
			if err := runner.Run(context.Background(), testIdentity().RunID, testClaimRequest()); !errors.Is(err, ErrUnconfirmed) {
				t.Fatalf("crash window must retain unknown despite observed sockets closing: %v", err)
			}
		})
	}
}
