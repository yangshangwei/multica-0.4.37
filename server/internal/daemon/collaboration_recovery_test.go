package daemon

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func collaborationRecoveryLedger(t *testing.T, count int, phase string) (*collaborationrun.Ledger, string, []collaborationrun.RetainedRun, map[string]protocol.CollaborationRunProof) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("private POSIX ledger requires Linux or macOS")
	}
	dir := filepath.Join(t.TempDir(), "ledger")
	ledger, err := collaborationrun.OpenLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	proofs := make(map[string]protocol.CollaborationRunProof)
	deploymentID, installationID := uuid.NewString(), uuid.NewString()
	for range count {
		id := protocol.CollaborationRunIdentity{DeploymentID: deploymentID, InstallationID: installationID,
			IssueID: uuid.NewString(), SessionID: uuid.NewString(), TargetID: uuid.NewString(), ChannelID: uuid.NewString(),
			BatchID: uuid.NewString(), GroupID: uuid.NewString(), RunID: uuid.NewString(), LaunchNonce: uuid.NewString(),
			Fence: "1", ClaimGeneration: "1"}
		local, err := ledger.Lock(id.DeploymentID, id.InstallationID, id.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if err := local.Register(id, "private-recovery-termination"); err != nil {
			t.Fatal(err)
		}
		if phase != "registered" {
			if err := local.RequestPermit("1", uuid.NewString()); err != nil {
				t.Fatal(err)
			}
		}
		if phase == "authorized" || phase == "burned" {
			if err := local.Authorize(); err != nil {
				t.Fatal(err)
			}
		}
		if phase == "burned" {
			if err := local.Burn(); err != nil {
				t.Fatal(err)
			}
		}
		if phase == "proof" {
			if err := local.NeverLaunched(); err != nil {
				t.Fatal(err)
			}
			proof, err := local.Seal()
			if err != nil {
				t.Fatal(err)
			}
			proofs[id.RunID] = proof
		}
		if err := local.Close(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ledger.Retained()
	if err != nil || len(entries) != count {
		t.Fatalf("retained fixtures: %d, %v", len(entries), err)
	}
	return ledger, dir, entries, proofs
}

func collaborationRecoveryPoller(t *testing.T, ledger *collaborationrun.Ledger, url string, transport http.RoundTripper) *collaborationPoller {
	t.Helper()
	client, err := collaborationrun.NewClient(url, "", transport)
	if err != nil {
		t.Fatal(err)
	}
	return &collaborationPoller{ledger: ledger, proofClient: client,
		daemon: &Daemon{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
}

func collaborationRecoveryRecordPath(dir string, id protocol.CollaborationRunIdentity) string {
	digest := sha256.Sum256([]byte(id.DeploymentID + "/" + id.InstallationID + "/" + id.RunID))
	return filepath.Join(dir, fmt.Sprintf("%x.json", digest))
}

func TestCollaborationProofRecoveryRotatesHTTPFailuresAcrossRestart(t *testing.T) {
	ledger, dir, entries, proofs := collaborationRecoveryLedger(t, 9, "proof")
	denied := make(map[string]bool)
	original := make(map[string][]byte)
	for _, entry := range entries[:8] {
		id := entry.Identity.RunID
		denied[id] = true
		var err error
		original[id], err = os.ReadFile(collaborationRecoveryRecordPath(dir, entry.Identity))
		if err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	requests := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		var proof protocol.CollaborationRunProof
		if err != nil || json.Unmarshal(raw, &proof) != nil {
			t.Error("invalid recovery request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		expected, err := json.Marshal(proofs[proof.Identity.RunID])
		if err != nil || !bytes.Equal(raw, expected) {
			t.Error("recovery replaced original proof, receipt or attempt bytes")
		}
		if r.URL.Path != "/api/collaboration/runtime/runs/"+proof.Identity.RunID+"/proof" || r.Header.Get("Authorization") != "Bearer private-recovery-termination" {
			t.Error("recovery attempted a non-proof action or used a different credential")
		}
		mu.Lock()
		requests[proof.Identity.RunID]++
		calls := requests[proof.Identity.RunID]
		mu.Unlock()
		if denied[proof.Identity.RunID] {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if calls == 1 {
			// The server accepted the proof but the response was lost. The next
			// round must replay exactly the sealed receipt already checked above.
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{Identity: proof.Identity, ReceiptID: proof.ReceiptID, Accepted: true})
	}))
	defer server.Close()
	for round := range 3 {
		// Reconstruct both Ledger and poller, retaining no in-memory position.
		ledger, err := collaborationrun.OpenLedger(dir)
		if err != nil {
			t.Fatal(err)
		}
		poller := collaborationRecoveryPoller(t, ledger, server.URL, nil)
		if err := poller.recoverProofs(t.Context()); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		count := 0
		for _, n := range requests {
			count += n
		}
		mu.Unlock()
		if count > 8*(round+1) {
			t.Fatalf("recovery exceeded eight attempts per round: %d", count)
		}
	}
	mu.Lock()
	lastCalls := requests[entries[8].Identity.RunID]
	mu.Unlock()
	if lastCalls != 2 {
		t.Fatalf("ninth proof starved behind eight persistent failures across restart: %d calls", lastCalls)
	}
	retained, err := ledger.Retained()
	if err != nil || len(retained) != 8 {
		t.Fatalf("only the reachable ninth proof should be acknowledged: %d, %v", len(retained), err)
	}
	for _, entry := range entries[:8] {
		raw, err := os.ReadFile(collaborationRecoveryRecordPath(dir, entry.Identity))
		if err != nil || !bytes.Equal(raw, original[entry.Identity.RunID]) {
			t.Fatal("failed recovery changed a retained sealed record")
		}
	}
}

func TestCollaborationProofRecoveryRotatesBusyPrefixAcrossProcesses(t *testing.T) {
	const childEnv = "MULTICA_TEST_RECOVERY_LOCK_DIRECTORY"
	if dir := os.Getenv(childEnv); dir != "" {
		ledger, err := collaborationrun.OpenLedger(dir)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := ledger.Retained()
		if err != nil || len(entries) != 9 {
			t.Fatalf("child inventory: %d, %v", len(entries), err)
		}
		for _, entry := range entries[:8] {
			local, err := ledger.Lock(entry.Identity.DeploymentID, entry.Identity.InstallationID, entry.Identity.RunID)
			if err != nil {
				t.Fatal(err)
			}
			defer local.Close()
		}
		_, _ = fmt.Fprintln(os.Stdout, "recovery locks ready")
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}
	_, dir, entries, _ := collaborationRecoveryLedger(t, 9, "proof")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-test.run=^TestCollaborationProofRecoveryRotatesBusyPrefixAcrossProcesses$")
	cmd.Env = append(os.Environ(), childEnv+"="+dir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("lock holder: %v, %s", err, stderr.String())
		}
	})
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		ready <- line
	}()
	select {
	case line := <-ready:
		if line != "recovery locks ready\n" {
			t.Fatalf("lock holder did not start: %q", line)
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("lock holder failed to synchronize")
	}
	var mu sync.Mutex
	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var proof protocol.CollaborationRunProof
		if err := json.NewDecoder(r.Body).Decode(&proof); err != nil {
			t.Error(err)
		}
		mu.Lock()
		received = append(received, proof.Identity.RunID)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{Identity: proof.Identity, ReceiptID: proof.ReceiptID, Accepted: true})
	}))
	defer server.Close()
	for range 2 {
		ledger, err := collaborationrun.OpenLedger(dir)
		if err != nil {
			t.Fatal(err)
		}
		poller := collaborationRecoveryPoller(t, ledger, server.URL, nil)
		if err := poller.recoverProofs(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0] != entries[8].Identity.RunID {
		t.Fatalf("eight peer locks starved the available ninth proof: %v", received)
	}
}

type collaborationRecoveryTransport func(*http.Request) (*http.Response, error)

func (f collaborationRecoveryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCollaborationProofRecoverySharesRoundDeadline(t *testing.T) {
	ledger, _, _, _ := collaborationRecoveryLedger(t, 9, "proof")
	var deadline time.Time
	calls := 0
	transport := collaborationRecoveryTransport(func(r *http.Request) (*http.Response, error) {
		got, ok := r.Context().Deadline()
		if !ok || time.Until(got) > 10*time.Second {
			t.Error("proof request has no bounded deadline")
		}
		if calls == 0 {
			deadline = got
		} else if !got.Equal(deadline) {
			t.Error("each proof obtained another ten-second budget instead of sharing one round deadline")
		}
		calls++
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	poller := collaborationRecoveryPoller(t, ledger, "http://recovery.invalid", transport)
	if err := poller.recoverProofs(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 8 {
		t.Fatalf("expected eight bounded attempts: %d", calls)
	}
}

func TestCollaborationProofRecoveryHonorsCallerCancellation(t *testing.T) {
	for _, phase := range []string{"registered", "permit_pending", "authorized", "proof"} {
		t.Run(phase, func(t *testing.T) {
			ledger, _, _, _ := collaborationRecoveryLedger(t, 2, phase)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			deadline, _ := ctx.Deadline()
			calls := 0
			transport := collaborationRecoveryTransport(func(r *http.Request) (*http.Response, error) {
				got, ok := r.Context().Deadline()
				if !ok || !got.Equal(deadline) {
					t.Error("retained pre-burn proof detached from the recovery context")
				}
				calls++
				cancel()
				return nil, context.Canceled
			})
			poller := collaborationRecoveryPoller(t, ledger, "http://recovery.invalid", transport)
			if err := poller.recoverProofs(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("recovery ignored caller cancellation: %v", err)
			}
			if calls != 1 {
				t.Fatalf("cancelled recovery sent another proof: %d", calls)
			}
		})
	}
}

func TestCollaborationProofRecoveryRetainsUnknownAndDamagedLedgerBarriers(t *testing.T) {
	ledger, dir, entries, _ := collaborationRecoveryLedger(t, 1, "burned")
	id := entries[0].Identity
	path := collaborationRecoveryRecordPath(dir, id)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	transport := collaborationRecoveryTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("unknown run must never contact a proof or business endpoint")
	})
	poller := collaborationRecoveryPoller(t, ledger, "http://recovery.invalid", transport)
	poller.blocked.Store(id.RunID, true)
	poller.daemon.activeTasks.Add(1)
	sem := newTaskSlotSemaphore(1)
	reserveCollaborationSlots(sem, 1)
	if err := poller.recoverProofs(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("recovery changed an unconfirmed attempt")
	}
	if err := os.WriteFile(path, []byte("{damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := poller.recoverProofs(t.Context()); !errors.Is(err, collaborationrun.ErrUnconfirmed) {
		t.Fatalf("corrupt inventory did not stop further machine claims: %v", err)
	}
	if _, blocked := poller.blocked.Load(id.RunID); !blocked || len(sem) != 0 || poller.daemon.activeTasks.Load() != 1 || poller.daemon.trySetClaimBarrier() {
		t.Fatal("recovery erased unknown capacity, blocked identity or the update barrier")
	}
	if requests != 0 {
		t.Fatalf("unknown/damaged recovery sent %d unauthorized requests", requests)
	}
}
