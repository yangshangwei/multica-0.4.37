package collaborationrun

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func prepareProofRecovery(t *testing.T, count int) (*Ledger, []RetainedRun) {
	t.Helper()
	l := newTestLedger(t)
	for range count {
		id := testIdentity()
		id.RunID = uuid.NewString()
		local, err := l.Lock(id.DeploymentID, id.InstallationID, id.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if err := local.Register(id, "private-recovery-token"); err != nil {
			t.Fatal(err)
		}
		if err := local.Close(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := l.Retained()
	if err != nil {
		t.Fatal(err)
	}
	return l, entries
}

func TestProofRecoveryCursorPersistsBeforeRequestAndWrapsAfterAcknowledgement(t *testing.T) {
	l, entries := prepareProofRecovery(t, 9)
	round, err := l.BeginProofRecovery()
	if err != nil {
		t.Fatal(err)
	}
	first, err := round.Next()
	if err != nil || first == nil || first.Identity != entries[0].Identity {
		t.Fatalf("first recovery selection: %+v, %v", first, err)
	}
	if err := round.Close(); err != nil {
		t.Fatal(err)
	}
	// Model death immediately after selection: no proof HTTP request occurred.
	l, err = OpenLedger(l.dir)
	if err != nil {
		t.Fatal(err)
	}
	round, err = l.BeginProofRecovery()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(entries); i++ {
		entry, err := round.Next()
		if err != nil || entry == nil || entry.Identity != entries[i].Identity {
			t.Fatalf("restart lost durable position at index %d: %+v, %v", i, entry, err)
		}
	}
	if entry, err := round.Next(); err != nil || entry != nil {
		t.Fatal("one recovery round exceeded eight attempts")
	}
	if err := round.Close(); err != nil {
		t.Fatal(err)
	}
	last := entries[len(entries)-1].Identity
	local, err := l.Lock(last.DeploymentID, last.InstallationID, last.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := local.NeverLaunched(); err != nil {
		t.Fatal(err)
	}
	proof, err := local.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Acknowledge(proof.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	round, err = l.BeginProofRecovery()
	if err != nil {
		t.Fatal(err)
	}
	defer round.Close()
	wrapped, err := round.Next()
	if err != nil || wrapped == nil || wrapped.Identity != first.Identity {
		t.Fatalf("acknowledged cursor boundary did not wrap to a skipped attempt: %+v, %v", wrapped, err)
	}
	retained, err := l.Retained()
	if err != nil || len(retained) != 8 {
		t.Fatalf("cursor files were mistaken for run JSON or deleted a tombstone: %d, %v", len(retained), err)
	}
}

func TestProofRecoveryCursorLockExcludesAnotherProcess(t *testing.T) {
	const childEnv = "MULTICA_TEST_RECOVERY_CURSOR_DIRECTORY"
	if dir := os.Getenv(childEnv); dir != "" {
		l, err := OpenLedger(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := l.BeginProofRecovery(); !errors.Is(err, ErrBusy) {
			t.Fatalf("peer cursor lock was not exclusive: %v", err)
		}
		return
	}
	l, _ := prepareProofRecovery(t, 1)
	round, err := l.BeginProofRecovery()
	if err != nil {
		t.Fatal(err)
	}
	defer round.Close()
	if _, err := round.Next(); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(round.cursor.path)
	if err != nil {
		t.Fatal(err)
	}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-test.run=^TestProofRecoveryCursorLockExcludesAnotherProcess$")
	cmd.Env = append(os.Environ(), childEnv+"="+l.dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("peer recovery: %v\n%s", err, out)
	}
	after, err := os.ReadFile(round.cursor.path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("peer changed a locked cursor")
	}
}

func TestProofRecoveryCursorDamageFailsClosed(t *testing.T) {
	for _, damage := range []string{"invalid JSON", "duplicate field", "future version", "invalid key", "oversized", "permissions", "symlink", "hardlink"} {
		t.Run(damage, func(t *testing.T) {
			l, entries := prepareProofRecovery(t, 1)
			round, err := l.BeginProofRecovery()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := round.Next(); err != nil {
				t.Fatal(err)
			}
			path := round.cursor.path
			if err := round.Close(); err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "invalid JSON":
				err = os.WriteFile(path, []byte("{invalid"), 0600)
			case "duplicate field":
				err = os.WriteFile(path, []byte(`{"version":1,"version":1,"after":"a"}`), 0600)
			case "future version":
				err = os.WriteFile(path, []byte(`{"version":2,"after":"a"}`), 0600)
			case "invalid key":
				err = os.WriteFile(path, []byte(`{"version":1,"after":"../other-run"}`), 0600)
			case "oversized":
				err = os.WriteFile(path, bytes.Repeat([]byte(" "), 1025), 0600)
			case "permissions":
				err = os.Chmod(path, 0644)
			case "symlink":
				copy := filepath.Join(t.TempDir(), "cursor")
				if err := os.Rename(path, copy); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(copy, path)
			case "hardlink":
				err = os.Link(path, filepath.Join(t.TempDir(), "cursor"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := l.BeginProofRecovery(); !errors.Is(err, ErrUnconfirmed) {
				t.Fatalf("damaged cursor silently reset scheduling: %v", err)
			}
			retained, err := l.Retained()
			if err != nil || len(retained) != 1 || retained[0] != entries[0] {
				t.Fatal("cursor refusal changed a run or its recovery classification")
			}
		})
	}
}

func TestProofRecoveryCursorWriteFailurePreventsAttempt(t *testing.T) {
	l, _ := prepareProofRecovery(t, 1)
	round, err := l.BeginProofRecovery()
	if err != nil {
		t.Fatal(err)
	}
	defer round.Close()
	if err := os.Mkdir(round.cursor.path, 0700); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if entry, err := round.Next(); entry != nil || !errors.Is(err, ErrUnconfirmed) {
			t.Fatalf("an undurable cursor authorized a recovery attempt: %+v, %v", entry, err)
		}
	}
}

func TestProofRecoveryBusyCursorDoesNotHideCorruptInventory(t *testing.T) {
	l, entries := prepareProofRecovery(t, 1)
	round, err := l.BeginProofRecovery()
	if err != nil {
		t.Fatal(err)
	}
	defer round.Close()
	id := entries[0].Identity
	local, err := l.Lock(id.DeploymentID, id.InstallationID, id.RunID)
	if err != nil {
		t.Fatal(err)
	}
	path := local.path
	if err := local.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.BeginProofRecovery(); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("busy peer cursor hid a corrupt ledger: %v", err)
	}
}
