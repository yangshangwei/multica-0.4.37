package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestManagedPIDReservationRejectsLiveAndUnknownPredecessor(t *testing.T) {
	childRoot := t.TempDir()
	child, done := managedLockChild(t, childRoot, "live", "hold")
	waitManagedLockFile(t, filepath.Join(childRoot, "live.ready"))
	defer func() { _ = child.Process.Kill(); <-done }()
	for _, pid := range []string{strconv.Itoa(child.Process.Pid), "invalid"} {
		t.Run(pid, func(t *testing.T) {
			home := t.TempDir()
			profile := "pid-test"
			directory, _ := managedProfileDirectory(home, profile)
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "daemon.pid")
			if err := os.WriteFile(path, []byte(pid), 0600); err != nil {
				t.Fatal(err)
			}
			called := false
			err := ReserveManagedProcess(home, profile, os.Getpid(), func() (int, error) { called = true; return os.Getpid(), nil })
			if err == nil || called {
				t.Fatal("duplicate foreground start took ownership")
			}
			_ = ReleaseManagedProcess(home, profile, os.Getpid())
			got, _ := os.ReadFile(path)
			if string(got) != pid {
				t.Fatal("failed duplicate erased predecessor evidence")
			}
		})
	}
}

func TestManagedPIDSuccessorSurvivesPredecessorCleanup(t *testing.T) {
	home := t.TempDir()
	profile := "successor-test"
	if err := os.MkdirAll(filepath.Join(home, ".multica", "profiles", profile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ReserveManagedProcess(home, profile, 0, func() (int, error) { return os.Getpid(), nil }); err != nil {
		t.Fatal(err)
	}
	childRoot := t.TempDir()
	child, done := managedLockChild(t, childRoot, "successor", "hold")
	waitManagedLockFile(t, filepath.Join(childRoot, "successor.ready"))
	defer func() { _ = child.Process.Kill(); <-done }()
	if err := ReserveManagedProcess(home, profile, os.Getpid(), func() (int, error) { return child.Process.Pid, nil }); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseManagedProcess(home, profile, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	directory, _ := managedProfileDirectory(home, profile)
	pid, err := readManagedPID(filepath.Join(directory, "daemon.pid"))
	if err != nil || pid != child.Process.Pid {
		t.Fatal("predecessor removed successor PID")
	}
	if err := CheckManagedProcessAbsent(home, profile, 0); err == nil {
		t.Fatal("live successor treated as absent")
	}
	if err := CheckManagedProcessAbsent(home, profile, child.Process.Pid); err != nil {
		t.Fatal("reserved successor could not enter foreground")
	}
}

func TestManagedPIDKeepsLiveEvidenceAndFailedSpawnDoesNotClaim(t *testing.T) {
	home := t.TempDir()
	profile := "failed-test"
	if err := os.MkdirAll(filepath.Join(home, ".multica", "profiles", profile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ReserveManagedProcess(home, profile, 0, func() (int, error) { return 0, errors.New("fake spawn failed") }); err == nil {
		t.Fatal("failed spawn accepted")
	}
	if err := CheckManagedProcessAbsent(home, profile, 0); err != nil {
		t.Fatal(err)
	}
	if err := ReserveManagedProcess(home, profile, 0, func() (int, error) { return os.Getpid(), nil }); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseManagedProcess(home, profile, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if err := CheckManagedProcessAbsent(home, profile, 0); err == nil {
		t.Fatal("live process cleanup erased its last PID proof")
	}
}
