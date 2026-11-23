package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// refusingLauncher stands in for containment. These tests never reach a fork,
// so no agent CLI is resolved or executed.
type refusingLauncher struct{ starts int }

func (l *refusingLauncher) Probe(context.Context) error { return nil }
func (l *refusingLauncher) Start(context.Context, ProviderInput) (*Process, error) {
	l.starts++
	return nil, errors.New("test launcher must not start a provider")
}

type permitServer struct {
	t       *testing.T
	mu      sync.Mutex
	permit  func(call int, w http.ResponseWriter, in protocol.CollaborationRunPermitRequest)
	permits []protocol.CollaborationRunPermitRequest
	proofs  []protocol.CollaborationRunProof
	stops   int
}

func (p *permitServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case strings.HasSuffix(r.URL.Path, "/claim"):
		_ = json.NewEncoder(w).Encode(testClaim())
	case strings.HasSuffix(r.URL.Path, "/permit"):
		var in protocol.CollaborationRunPermitRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			p.t.Error(err)
		}
		p.permits = append(p.permits, in)
		p.permit(len(p.permits), w, in)
	case strings.HasSuffix(r.URL.Path, "/stop-intent"):
		p.stops++
		_ = json.NewEncoder(w).Encode(map[string]any{"identity": testIdentity(), "stop_requested": true})
	case strings.HasSuffix(r.URL.Path, "/proof"):
		if r.Header.Get("Authorization") != "Bearer termination-only" {
			p.t.Error("proof did not use the termination capability")
		}
		var proof protocol.CollaborationRunProof
		if err := json.NewDecoder(r.Body).Decode(&proof); err != nil {
			p.t.Error(err)
		}
		p.proofs = append(p.proofs, proof)
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ReceiptID: proof.ReceiptID, Identity: proof.Identity, Accepted: true})
	default:
		p.t.Errorf("unexpected collaboration request %s", r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}
}

func runPermitScenario(t *testing.T, permit func(call int, w http.ResponseWriter, in protocol.CollaborationRunPermitRequest)) (*permitServer, *refusingLauncher, *Ledger, error) {
	t.Helper()
	l := newTestLedger(t)
	server := &permitServer{t: t, permit: permit}
	s := httptest.NewServer(server)
	t.Cleanup(s.Close)
	client, err := NewClient(s.URL, "mdt_test", nil)
	if err != nil {
		t.Fatal(err)
	}
	launcher := &refusingLauncher{}
	runner := &Runner{Ledger: l, Client: client, Controller: launcher, PermitAttempts: 3, PermitBackoff: time.Millisecond}
	err = runner.Run(context.Background(), testIdentity().RunID, testClaimRequest())
	return server, launcher, l, err
}

// assertNeverLaunchedConverged proves the slot-return contract: the error is
// not ErrUnconfirmed (dispatch returns the machine slot) and nothing is retained.
func assertNeverLaunchedConverged(t *testing.T, server *permitServer, launcher *refusingLauncher, l *Ledger, err error) {
	t.Helper()
	if errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("pre-burn permit failure kept capacity: %v", err)
	}
	if launcher.starts != 0 {
		t.Fatal("provider started without a permit")
	}
	if len(server.proofs) != 1 {
		t.Fatalf("expected one never-launched proof, got %d", len(server.proofs))
	}
	proof := server.proofs[0]
	if proof.State != "never_launched_proven" || proof.SealedAttemptSeq != "1" || proof.LastAuthorizedAttemptSeq != "1" || proof.AttemptNonce != server.permits[0].AttemptNonce || proof.ExitCode != nil {
		t.Fatalf("never-launched proof: %+v", proof)
	}
	if entries, err := l.Retained(); err != nil || len(entries) != 0 {
		t.Fatalf("permit failure left a retained record: %+v %v", entries, err)
	}
}

func TestRunnerExplicitPermitRefusalProvesNeverLaunched(t *testing.T) {
	// A Stop that settles the claimed run before Permit yields this 409.
	server, launcher, l, err := runPermitScenario(t, func(_ int, w http.ResponseWriter, _ protocol.CollaborationRunPermitRequest) {
		w.WriteHeader(http.StatusConflict)
	})
	var status *HTTPError
	if !errors.As(err, &status) || status.Status != http.StatusConflict {
		t.Fatalf("refusal was not reported: %v", err)
	}
	if len(server.permits) != 1 {
		t.Fatalf("explicit refusal was replayed %d times", len(server.permits))
	}
	if server.stops != 0 {
		t.Fatal("refused permit consulted stop intent as if authorized")
	}
	assertNeverLaunchedConverged(t, server, launcher, l, err)
}

func TestRunnerReplaysLostPermitWithSameAttempt(t *testing.T) {
	server, launcher, l, err := runPermitScenario(t, func(call int, w http.ResponseWriter, in protocol.CollaborationRunPermitRequest) {
		switch call {
		case 1:
			// The server may have committed the permit; the response is lost.
			conn, _, hijackErr := w.(http.Hijacker).Hijack()
			if hijackErr == nil {
				_ = conn.Close()
			}
		case 2:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunPermitResponse{Identity: in.Identity, AttemptSeq: in.AttemptSeq, AttemptNonce: in.AttemptNonce, BusinessToken: "business-token"})
		}
	})
	if err != nil {
		t.Fatalf("replayed permit followed by Stop: %v", err)
	}
	if len(server.permits) != 3 {
		t.Fatalf("expected two replays, got %d permit requests", len(server.permits))
	}
	for _, p := range server.permits[1:] {
		if p != server.permits[0] {
			t.Fatalf("replay changed the attempt: %+v vs %+v", p, server.permits[0])
		}
	}
	// The replayed permit succeeded, so the runner reached the authorized
	// stop check; the fake Stop proves nonlaunch before any burn or fork.
	if server.stops != 1 {
		t.Fatalf("authorized attempt skipped stop intent: %d", server.stops)
	}
	assertNeverLaunchedConverged(t, server, launcher, l, err)
}

func TestRunnerExhaustedPermitReplayProvesNeverLaunched(t *testing.T) {
	server, launcher, l, err := runPermitScenario(t, func(_ int, w http.ResponseWriter, _ protocol.CollaborationRunPermitRequest) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	var status *HTTPError
	if !errors.As(err, &status) || status.Status != http.StatusServiceUnavailable {
		t.Fatalf("exhausted replay was not reported: %v", err)
	}
	if len(server.permits) != 3 {
		t.Fatalf("expected bounded replay, got %d permit requests", len(server.permits))
	}
	for _, p := range server.permits[1:] {
		if p != server.permits[0] {
			t.Fatal("replay changed the attempt seq or nonce")
		}
	}
	assertNeverLaunchedConverged(t, server, launcher, l, err)
}
