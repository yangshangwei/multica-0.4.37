//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const daemonFDProcessRole = "MULTICA_TEST_DAEMON_FD_ROLE"

// Re-execution provides real inheritable descriptors without changing the
// descriptors of the surrounding test process.
func TestDaemonDescriptorProcess(t *testing.T) {
	role := os.Getenv(daemonFDProcessRole)
	if role == "" {
		return
	}
	root := os.Getenv("MULTICA_TEST_DAEMON_FD_ROOT")
	if root == "" {
		t.Fatal("explicit fixture root required")
	}
	if role == "bridge" {
		// Put leaks above the intentional ExtraFiles mapping. Otherwise exec's
		// remapping of child fd3 could accidentally hide the defect.
		for source, target := range map[int]int{3: 64, 4: 65} {
			if _, err := unix.FcntlInt(uintptr(target), unix.F_GETFD, 0); err != unix.EBADF {
				t.Fatalf("fixture target descriptor %d is already used: %v", target, err)
			}
			if err := unix.Dup3(source, target, 0); err != nil {
				t.Fatal(err)
			}
			if err := unix.Close(source); err != nil {
				t.Fatal(err)
			}
		}
		pipeTarget, err := os.Readlink("/proc/self/fd/64")
		if err != nil {
			t.Fatal(err)
		}
		intentional, err := os.Open(filepath.Join(root, "intentional"))
		if err != nil {
			t.Fatal(err)
		}
		defer intentional.Close()
		stdout, err := os.Create(filepath.Join(root, "stdout"))
		if err != nil {
			t.Fatal(err)
		}
		defer stdout.Close()
		stderr, err := os.Create(filepath.Join(root, "stderr"))
		if err != nil {
			t.Fatal(err)
		}
		defer stderr.Close()
		child := exec.Command(os.Args[0], "-test.run=^TestDaemonDescriptorProcess$")
		child.Env = []string{daemonFDProcessRole + "=leaf", "MULTICA_TEST_DAEMON_FD_ROOT=" + root,
			"MULTICA_TEST_LEAKED_PIPE=" + pipeTarget}
		forwardManagedHandoff(child, []byte(`{"proof":"test-only-stdin-handoff"}`))
		child.Stdout, child.Stderr = stdout, stderr
		child.ExtraFiles = []*os.File{intentional}
		child.SysProcAttr = daemonSysProcAttr(true)
		if err := prepareDaemonChildFds(); err != nil {
			t.Fatal(err)
		}
		// CLOEXEC must not close or invalidate a live descriptor in the parent.
		if _, err := unix.Write(64, []byte("parent-still-open\n")); err != nil {
			t.Fatalf("parent descriptor was closed: %v", err)
		}
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		waited := make(chan error, 1)
		go func() { waited <- child.Wait() }()
		consumed := false
		defer func() {
			if !consumed {
				_ = child.Process.Kill()
				<-waited
			}
		}()
		if err := os.WriteFile(filepath.Join(root, "child-pid"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			t.Fatal(err)
		}
		// Wait drains the bytes.Reader used by forwardManagedHandoff before
		// this bridge exits. The leaf stays alive until the outer test releases it.
		for _, fd := range []int{64, 65} {
			if err := unix.Close(fd); err != nil {
				t.Fatal(err)
			}
		}
		select {
		case err := <-waited:
			consumed = true
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(15 * time.Second):
			_ = child.Process.Kill()
			<-waited
			consumed = true
			t.Fatal("fixture leaf did not release")
		}
		return
	}
	if role != "leaf" {
		t.Fatal("unknown fixture role")
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdin, []byte(`{"proof":"test-only-stdin-handoff"}`)) {
		t.Fatal("managed stdin bytes changed")
	}
	intentional := os.NewFile(3, "intentional")
	data, err := io.ReadAll(intentional)
	if err != nil || string(data) != "intentional-extra-file" {
		t.Fatalf("explicit ExtraFiles mapping lost: %q, %v", data, err)
	}
	if err := intentional.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	var leaks []string
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && (target == os.Getenv("MULTICA_TEST_LEAKED_PIPE") || target == filepath.Join(root, "unrelated")) {
			leaks = append(leaks, entry.Name())
		}
	}
	if _, err := fmt.Fprintln(os.Stdout, "stdout-survived"); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintln(os.Stderr, "stderr-survived"); err != nil {
		t.Fatal(err)
	}
	ready, err := json.Marshal(leaks)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ready"), ready, 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "release")); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("fixture release was not delivered")
}

func TestDaemonInheritedDescriptorsDoNotKeepParentPipeOpen(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"intentional": "intentional-extra-file", "unrelated": "unrelated-parent-resource"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readEnd.Close()
	defer writeEnd.Close()
	unrelated, err := os.Open(filepath.Join(root, "unrelated"))
	if err != nil {
		t.Fatal(err)
	}
	defer unrelated.Close()
	bridge := exec.Command(os.Args[0], "-test.run=^TestDaemonDescriptorProcess$")
	bridge.Env = []string{daemonFDProcessRole + "=bridge", "MULTICA_TEST_DAEMON_FD_ROOT=" + root}
	bridge.ExtraFiles = []*os.File{writeEnd, unrelated}
	var bridgeOutput bytes.Buffer
	bridge.Stdout, bridge.Stderr = &bridgeOutput, &bridgeOutput
	if err := bridge.Start(); err != nil {
		t.Fatal(err)
	}
	if err := writeEnd.Close(); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- bridge.Wait() }()
	defer func() {
		_ = os.WriteFile(filepath.Join(root, "release"), nil, 0o600)
		select {
		case err := <-finished:
			if err != nil {
				t.Errorf("fixture bridge: %v\n%s", err, bridgeOutput.String())
			}
		case <-time.After(3 * time.Second):
			_ = bridge.Process.Kill()
			<-finished
			if raw, err := os.ReadFile(filepath.Join(root, "child-pid")); err == nil {
				if pid, err := strconv.Atoi(string(raw)); err == nil {
					_ = unix.Kill(pid, unix.SIGKILL)
				}
			}
			t.Error("fixture bridge failed to exit")
		}
	}()
	var ready []byte
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ready, err = os.ReadFile(filepath.Join(root, "ready"))
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("leaf did not become ready: %v", err)
	}
	var leaks []string
	if err := json.Unmarshal(ready, &leaks); err != nil {
		t.Fatal(err)
	}
	if len(leaks) != 0 {
		t.Errorf("unrelated inherited descriptors survived exec: %v", leaks)
	}
	if err := readEnd.SetReadDeadline(time.Now().Add(750 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	read, err := io.ReadAll(readEnd)
	if err != nil {
		t.Errorf("parent pipe did not reach EOF while daemon child was alive: %v", err)
	}
	if string(read) != "parent-still-open\n" {
		t.Errorf("parent descriptor did not remain usable: %q", read)
	}
	pidBytes, err := os.ReadFile(filepath.Join(root, "child-pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Kill(pid, 0); err != nil {
		t.Fatalf("leaf exited before EOF proof: %v", err)
	}
	for name, expected := range map[string]string{"stdout": "stdout-survived\n", "stderr": "stderr-survived\n"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(b) != expected {
			t.Errorf("%s mapping changed: %q, %v", name, b, err)
		}
	}
}
