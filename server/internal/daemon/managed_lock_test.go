package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestManagedLockSubprocess(t *testing.T) {
	mode := os.Getenv("MULTICA_LOCK_TEST_MODE")
	if mode == "" {
		return
	}
	directory := os.Getenv("MULTICA_LOCK_TEST_DIR")
	name := os.Getenv("MULTICA_LOCK_TEST_NAME")
	ready := func() {
		if err := os.WriteFile(filepath.Join(directory, name+".ready"), nil, 0600); err != nil {
			panic(err)
		}
	}
	if mode == "choosing" {
		owner, err := managementProcessIdentity()
		if err != nil {
			panic(err)
		}
		locks := filepath.Join(directory, ".installation-locks")
		if err := managedEnsureDirectory(locks, true); err != nil {
			panic(err)
		}
		nonce := uuid.NewString()
		if err := writeManagementJSON(filepath.Join(locks, nonce+".json"), managementLockTicket{Version: 1, PID: os.Getpid(), OwnerNonce: nonce, HostID: owner.hostID, BootID: owner.bootID}); err != nil {
			panic(err)
		}
		ready()
		for {
			time.Sleep(time.Second)
		}
	}
	err := withManagementFileLock(directory, func() error {
		guard := filepath.Join(directory, "critical-guard")
		f, err := os.OpenFile(guard, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		f.Close()
		defer os.Remove(guard)
		ready()
		if mode == "hold" {
			for {
				time.Sleep(time.Second)
			}
		}
		counter := filepath.Join(directory, "counter")
		raw, _ := os.ReadFile(counter)
		n, _ := strconv.Atoi(string(raw))
		time.Sleep(50 * time.Millisecond)
		return os.WriteFile(counter, []byte(strconv.Itoa(n+1)), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func managedLockChild(t *testing.T, directory, name, mode string) (*exec.Cmd, <-chan error) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-test.run=^TestManagedLockSubprocess$")
	command.Env = append(os.Environ(), "MULTICA_LOCK_TEST_MODE="+mode, "MULTICA_LOCK_TEST_DIR="+directory, "MULTICA_LOCK_TEST_NAME="+name)
	var output strings.Builder
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		err := command.Wait()
		if err != nil {
			err = fmt.Errorf("%w: %s", err, output.String())
		}
		done <- err
	}()
	t.Cleanup(func() { _ = command.Process.Kill() })
	return command, done
}
func waitManagedLockFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("subprocess did not publish %s", path)
}

func TestManagedLockSerializesRealProcesses(t *testing.T) {
	directory := t.TempDir()
	_, first := managedLockChild(t, directory, "first", "increment")
	_, second := managedLockChild(t, directory, "second", "increment")
	for _, done := range []<-chan error{first, second} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("lock contender timed out")
		}
	}
	value, err := os.ReadFile(filepath.Join(directory, "counter"))
	if err != nil || string(value) != "2" {
		t.Fatalf("lost serialized update: %s %v", value, err)
	}
}

func TestManagedLockRecoversKilledOwnerAndChoosingRecord(t *testing.T) {
	for _, mode := range []string{"hold", "choosing"} {
		t.Run(mode, func(t *testing.T) {
			identity, err := LoadManagedInstallation(t.TempDir(), "11111111-1111-4111-8111-111111111111")
			if err != nil {
				t.Fatal(err)
			}
			directory := identity.directory
			child, done := managedLockChild(t, directory, "holder", mode)
			waitManagedLockFile(t, filepath.Join(directory, "holder.ready"))
			entered := make(chan error, 1)
			go func() { entered <- withManagementFileLock(directory, func() error { return nil }) }()
			select {
			case err := <-entered:
				t.Fatalf("live owner stolen: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			if err := child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			<-done
			select {
			case err := <-entered:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("dead owner not recovered")
			}
			// The lock metadata can be recovered without replacing the identity file.
			recovered, err := LoadManagedInstallation(filepath.Dir(filepath.Dir(filepath.Dir(directory))), identity.DeploymentID())
			if err != nil || recovered.PublicKeyBase64() != identity.PublicKeyBase64() {
				t.Fatalf("installation key changed after crash: %v", err)
			}
		})
	}
}

func TestManagedLockKeepsForeignOwnerAndRecoversOlderBoot(t *testing.T) {
	current, err := managementProcessIdentity()
	if err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []bool{false, true} {
		t.Run(strconv.FormatBool(foreign), func(t *testing.T) {
			directory := t.TempDir()
			nonce := uuid.NewString()
			ticket := "1"
			host := current.hostID
			if foreign {
				host = strings.Repeat("f", 64)
			}
			path := filepath.Join(directory, nonce+".json")
			if err := writeManagementJSON(path, managementLockTicket{Version: 1, PID: os.Getpid(), OwnerNonce: nonce, HostID: host, BootID: strings.Repeat("0", 64), Ticket: &ticket}); err != nil {
				t.Fatal(err)
			}
			records, err := managementTickets(directory)
			if err != nil {
				t.Fatal(err)
			}
			if foreign && len(records) != 1 {
				t.Fatal("foreign host owner incorrectly removed")
			}
			if !foreign && len(records) != 0 {
				t.Fatal("old boot not recovered despite reused live PID")
			}
		})
	}
}

func TestManagedProcessExitRequiresDefiniteEvidence(t *testing.T) {
	for _, pid := range []int{-1, 0, os.Getpid()} {
		if ManagementProcessExited(pid) {
			t.Fatalf("invalid or live PID %d accepted as exited", pid)
		}
	}
}
