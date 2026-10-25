//go:build windows

package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestHostLifecycleWindowsProcessIdentityUsesOriginalHandle(t *testing.T) {
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	file, exe := lifecycleHelper(t, lifecycle, "exit")
	cmd := lifecycle.CommandContext(context.Background(), LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, exe)
	cmd.Env = lifecycleTestEnv("exit", file.Name())
	if err := lifecycle.Start(cmd); err != nil {
		t.Fatal(err)
	}
	instance, err := nativeProcessInstanceID(cmd.Process)
	if err != nil || !strings.HasPrefix(instance, "windows:") || instance != recorder.started[0].InstanceID {
		t.Fatalf("original process handle did not identify the launch: %q, %v", instance, err)
	}
	if err := lifecycle.Wait(cmd); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeProcessInstanceID(cmd.Process); err == nil {
		t.Fatal("released process handle was replaced by a PID lookup")
	}
}

func TestHostLifecycleWindowsWrapperRequiresJobBeforeResume(t *testing.T) {
	originalAttach := attachProcessTree
	attachProcessTree = func(*exec.Cmd) error { return errors.New("test job attachment failure") }
	t.Cleanup(func() { attachProcessTree = originalAttach })
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "cmd.exe")
	if err := os.WriteFile(wrapper, contents, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		path    string
		host    bool
		refused bool
	}{
		{"host wrapper", wrapper, true, true},
		{"host native executable", exe, true, false},
		{"ordinary wrapper", wrapper, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &lifecycleRecorder{}
			lifecycle := NewLaunchLifecycle(recorder.observer())
			file, _ := lifecycleHelper(t, lifecycle, "exit")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var cmd *exec.Cmd
			if test.host {
				cmd = lifecycle.CommandContext(ctx, LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, test.path)
			} else {
				cmd = NewCommand(test.path, nil).exec(ctx)
			}
			cmd.Env = lifecycleTestEnv("exit", file.Name())
			if test.host {
				err = lifecycle.Run(cmd)
			} else {
				err = runOwned(cmd, nil)
			}
			if test.refused {
				if !errors.Is(err, ErrLaunchUnsupported) {
					t.Fatalf("unowned host wrapper was not refused: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			if test.refused != (len(body) == 0) {
				t.Fatalf("unexpected wrapper execution: refused=%v, recorded launches=%d", test.refused, strings.Count(string(body), "\n"))
			}
			if test.host {
				if len(recorder.started) != 1 || recorder.started[0].SupervisionScope != SupervisionDirect ||
					len(recorder.terminated) != 1 || recorder.terminated[0].Kind != LaunchManagedEnded || !recorder.terminated[0].WaitObserved ||
					recorder.terminated[0].CleanupScope != SupervisionDirect || recorder.terminated[0].Cleanup == CleanupTrackedScopeComplete {
					t.Fatalf("Job failure was not recorded as an actually waited direct process: %+v", recorder.terminated)
				}
				if err := lifecycle.Seal(); err != nil {
					t.Fatalf("waited failed launch retained an unknown barrier: %v", err)
				}
			}
		})
	}
}

func TestHostLifecycleWindowsWrapperWaitRequiresManagedChildEnd(t *testing.T) {
	exe, pidPath := buildDescendantSpawner(t)
	wrapper := filepath.Join(filepath.Dir(exe), "cmd.exe")
	if err := os.Rename(exe, wrapper); err != nil {
		t.Fatal(err)
	}
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := lifecycle.CommandContext(ctx, LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, wrapper, "spawn-and-exit")
	cmd.Env = buildEnv(map[string]string{"DESCENDANT_PID_FILE": pidPath})
	if err := lifecycle.Start(cmd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		lifecycle.Stop()
		if cmd.ProcessState == nil {
			_ = lifecycle.Wait(cmd)
		}
		releaseProcessGroup(cmd)
	})
	tree, ok := lookupProcessTree(cmd)
	if !ok {
		t.Fatal("wrapper was not attached to its required Job")
	}
	child := waitForDescendantPid(t, pidPath)
	// Keep a full-access handle for test cleanup, but give the supervisor only
	// query access. Termination then fails while live managed work is visible.
	var readOnly windows.Handle
	const jobObjectQueryAccess = 0x0004 // JOB_OBJECT_QUERY is not exported by x/sys/windows.
	if err := windows.DuplicateHandle(windows.CurrentProcess(), tree.job, windows.CurrentProcess(), &readOnly, jobObjectQueryAccess, false, 0); err != nil {
		t.Fatal(err)
	}
	ownedProcessTreesMu.Lock()
	ownedProcessTrees[cmd] = ownedProcessTree{job: readOnly}
	ownedProcessTreesMu.Unlock()
	t.Cleanup(func() {
		_ = windows.TerminateJobObject(tree.job, 1)
		_ = windows.CloseHandle(tree.job)
	})
	if err := lifecycle.Wait(cmd); !errors.Is(err, ErrLaunchUnconfirmed) {
		t.Fatalf("live provider child was accepted as ended: %v", err)
	}
	if !processStillRunning(child) {
		t.Fatal("fixture did not retain live managed work")
	}
	if len(recorder.terminated) != 1 || recorder.terminated[0].Kind != LaunchUnknown ||
		!recorder.terminated[0].WaitObserved || recorder.terminated[0].CleanupScope != SupervisionWindowsJob {
		t.Fatalf("wrapper wait lost the unknown managed-child observation: %+v", recorder.terminated)
	}
	if !errors.Is(lifecycle.Seal(), ErrLaunchUnconfirmed) {
		t.Fatal("live managed child did not retain the run barrier")
	}
}
