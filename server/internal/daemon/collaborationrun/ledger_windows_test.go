//go:build windows

package collaborationrun

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestHostWindowsLedgerLockPeer(t *testing.T) {
	path := os.Getenv("COLLAB_HOST_LEDGER_LOCK_PEER")
	if path == "" {
		return
	}
	f, err := lockPrivateFile(path)
	if errors.Is(err, ErrBusy) {
		os.Exit(42)
	}
	if err != nil {
		os.Exit(43)
	}
	_ = unlockPrivateFile(f)
	_ = f.Close()
	os.Exit(0)
}

func TestHostWindowsLedgerProtectsFilesAndLocksAcrossProcesses(t *testing.T) {
	l := newHostTestLedger(t)
	path := filepath.Join(l.dir, "cross-process.lock")
	lock, err := lockPrivateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	peer := func() error {
		cmd := exec.Command(executable, "-test.run=^TestHostWindowsLedgerLockPeer$")
		cmd.Env = append(os.Environ(), "COLLAB_HOST_LEDGER_LOCK_PEER="+path)
		return cmd.Run()
	}
	var exit *exec.ExitError
	if err := peer(); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("peer bypassed LockFileEx: %v", err)
	}
	_ = unlockPrivateFile(lock)
	_ = lock.Close()
	if err := peer(); err != nil {
		t.Fatalf("closed owner did not release native lock: %v", err)
	}
	state := filepath.Join(l.dir, "state")
	if err := atomicPrivateWrite(state, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := atomicPrivateWrite(state, []byte("new")); err != nil {
		t.Fatal(err)
	}
	f, err := openPrivateFile(state)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	link := filepath.Join(l.dir, "hardlink")
	if err := os.Link(state, link); err != nil {
		t.Fatal(err)
	}
	if f, err := openPrivateFile(state); err == nil {
		_ = f.Close()
		t.Fatal("hardlinked ledger identity accepted")
	}
}

func TestHostWindowsLedgerRejectsSharedACL(t *testing.T) {
	l := newHostTestLedger(t)
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(l.dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLedger(l.dir); err == nil {
		t.Fatal("world-readable collaboration ledger accepted")
	}
}

func TestHostWindowsLedgerSupportsLongPaths(t *testing.T) {
	parent := filepath.Join(t.TempDir(), strings.Repeat("long-", 25), strings.Repeat("path-", 25))
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	l, err := OpenLedger(filepath.Join(parent, "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(l.dir, "record")
	if err := atomicPrivateWrite(state, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := atomicPrivateWrite(state, []byte("second")); err != nil {
		t.Fatal(err)
	}
	f, err := openPrivateFile(state)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}
