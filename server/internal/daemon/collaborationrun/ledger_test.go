package collaborationrun

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func testIdentity() protocol.CollaborationRunIdentity {
	return protocol.CollaborationRunIdentity{
		DeploymentID: "00000000-0000-4000-8000-000000000001", InstallationID: "00000000-0000-4000-8000-000000000002",
		IssueID: "00000000-0000-4000-8000-000000000003", SessionID: "00000000-0000-4000-8000-000000000004",
		TargetID: "00000000-0000-4000-8000-000000000005", ChannelID: "00000000-0000-4000-8000-000000000006",
		BatchID: "00000000-0000-4000-8000-000000000007", GroupID: "00000000-0000-4000-8000-000000000008",
		RunID: "00000000-0000-4000-8000-000000000009", Fence: "1", ClaimGeneration: "1", LaunchNonce: "00000000-0000-4000-8000-000000000010",
	}
}

func newTestLedger(t *testing.T) *Ledger {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("private POSIX ledger requires Linux or macOS")
	}
	l, err := OpenLedger(filepath.Join(t.TempDir(), "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func registerTestRun(t *testing.T, l *Ledger) *RunLedger {
	t.Helper()
	r, err := l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Register(testIdentity(), "supervisor-private-token"); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestNonceBurnSurvivesRestartAndCannotLaunchAgain(t *testing.T) {
	l := newTestLedger(t)
	r := registerTestRun(t, l)
	if err := r.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); err != nil {
		t.Fatal(err)
	}
	if err := r.Authorize(); err != nil {
		t.Fatal(err)
	}
	if err := r.Burn(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Burn(); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("replaying a burned attempt must not authorize a fork: %v", err)
	}
	if r.record.Phase != phaseBurned {
		t.Fatalf("lost durable burn: %s", r.record.Phase)
	}
	if r.record.Proof != nil {
		t.Fatal("crash window fabricated a proof")
	}
}

func TestSealAndProofSurviveRestartAndRefuseLateAttempt(t *testing.T) {
	l := newTestLedger(t)
	r := registerTestRun(t, l)
	if err := r.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); err != nil {
		t.Fatal(err)
	}
	if err := r.Authorize(); err != nil {
		t.Fatal(err)
	}
	if err := r.Burn(); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordProcess("pid:[1234]"); err != nil {
		t.Fatal(err)
	}
	if err := r.Reap(0); err != nil {
		t.Fatal(err)
	}
	proof, err := r.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if proof.State != "exited_proven" || proof.SealedAttemptSeq != "1" {
		t.Fatalf("invalid proof: %+v", proof)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r, err = l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	replayed, err := r.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ReceiptID != proof.ReceiptID {
		t.Fatal("restart generated a different proof receipt")
	}
	if err := r.RequestPermit("2", "00000000-0000-4000-8000-000000000012"); !errors.Is(err, ErrSealed) {
		t.Fatalf("sealed run admitted a late fallback: %v", err)
	}
	if err := r.Acknowledge(proof.ReceiptID); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerLockExcludesAnotherProcess(t *testing.T) {
	if os.Getenv("MULTICA_TEST_COLLAB_LOCK") != "" {
		l, err := OpenLedger(os.Getenv("MULTICA_TEST_COLLAB_LOCK"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("peer lock was not exclusive: %v", err)
		}
		return
	}
	l := newTestLedger(t)
	r := registerTestRun(t, l)
	defer r.Close()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-test.run=^TestLedgerLockExcludesAnotherProcess$")
	cmd.Env = append(os.Environ(), "MULTICA_TEST_COLLAB_LOCK="+l.dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("peer: %v\n%s", err, out)
	}
}

func TestLedgerCorruptionNeverCreatesFreshRun(t *testing.T) {
	l := newTestLedger(t)
	r := registerTestRun(t, l)
	path := r.path
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("corrupt ledger did not preserve barrier: %v", err)
	}
}

func TestLedgerRejectsSharedPermissionsAndSymlinks(t *testing.T) {
	l := newTestLedger(t)
	if err := os.Chmod(l.dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLedger(l.dir); err == nil {
		t.Fatal("accepted public ledger directory")
	}
	if err := os.Chmod(l.dir, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(l.dir, link); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLedger(link); err == nil {
		t.Fatal("accepted symlink ledger root")
	}
}

func TestUncertainPermitCannotBecomeExitProof(t *testing.T) {
	l := newTestLedger(t)
	r := registerTestRun(t, l)
	defer r.Close()
	if err := r.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Seal(); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("sealed uncertain permit: %v", err)
	}
	if err := r.Reap(0); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("reaped absent containment: %v", err)
	}
}

func TestRetainedInventoryPreservesCapacityAndProofRecovery(t *testing.T) {
	l := newTestLedger(t)
	r := registerTestRun(t, l)
	if err := r.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := l.Retained()
	if err != nil {
		t.Fatal(err)
	}
	// Nothing can fork before Burn: a pending permit is recoverable, not capacity.
	if len(entries) != 1 || entries[0].Unconfirmed || !entries[0].CanProve {
		t.Fatalf("pre-burn permit retained capacity: %+v", entries)
	}
	r, err = l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Authorize(); err != nil {
		t.Fatal(err)
	}
	if err := r.Burn(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err = l.Retained()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].Unconfirmed || entries[0].CanProve {
		t.Fatalf("burned attempt lost capacity: %+v", entries)
	}
	r, err = l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	// A burned record can never become a nonlaunch tombstone or a new permit.
	if err := r.NeverLaunched(); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("burned attempt became never-launched: %v", err)
	}
	if err := r.Authorize(); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("burned attempt returned to authorization: %v", err)
	}
	if err := r.RequestPermit("1", "00000000-0000-4000-8000-000000000012"); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("burned attempt requested another permit: %v", err)
	}
}

func TestUnrequestedRegistrationSealsAttemptZeroTombstone(t *testing.T) {
	l := newTestLedger(t)
	r := registerTestRun(t, l)
	if err := r.NeverLaunched(); err != nil {
		t.Fatal(err)
	}
	proof, err := r.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if proof.State != "never_launched_proven" || proof.SealedAttemptSeq != "0" || proof.LastAuthorizedAttemptSeq != "0" || proof.AttemptNonce != "" || proof.ExitCode != nil {
		t.Fatalf("unrequested registration proof: %+v", proof)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatalf("attempt-zero tombstone did not reload: %v", err)
	}
	defer reloaded.Close()
	if err := reloaded.RequestPermit("1", "00000000-0000-4000-8000-000000000011"); !errors.Is(err, ErrSealed) {
		t.Fatalf("sealed registration requested a permit: %v", err)
	}
	again, err := reloaded.Seal()
	if err != nil || !reflect.DeepEqual(again, proof) {
		t.Fatalf("sealed proof replay changed: %+v %v", again, err)
	}
}
