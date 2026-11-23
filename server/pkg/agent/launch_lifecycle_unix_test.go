//go:build unix

package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestHostLifecycleEscapedDescendantIsOutsideCleanupScope(t *testing.T) {
	dir := t.TempDir()
	source, executable, pidPath := filepath.Join(dir, "helper.go"), filepath.Join(dir, "helper"), filepath.Join(dir, "child.pid")
	const program = `package main
import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)
func main() {
	if len(os.Args) > 1 { time.Sleep(time.Minute); return }
	child := exec.Command(os.Args[0], "detached")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil { panic(err) }
	if err := os.WriteFile(os.Getenv("DETACHED_PID_FILE"), []byte(fmt.Sprint(child.Process.Pid)), 0600); err != nil { panic(err) }
}
`
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("go", "build", "-o", executable, source).CombinedOutput(); err != nil {
		t.Fatalf("build test helper: %v: %s", err, output)
	}
	t.Cleanup(func() {
		if body, err := os.ReadFile(pidPath); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(body))); err == nil && pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	recorder := &lifecycleRecorder{}
	lifecycle := NewLaunchLifecycle(recorder.observer())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := lifecycle.CommandContext(ctx, LaunchInfo{Provider: "git", Kind: LaunchKindPreparation}, executable)
	cmd.Env = buildEnv(map[string]string{"DETACHED_PID_FILE": pidPath})
	if err := lifecycle.Run(cmd); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid detached child identity: %v", err)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatal("fixture did not leave a live escaped descendant")
	}
	group, err := syscall.Getpgid(pid)
	if err != nil || group == cmd.Process.Pid {
		t.Fatalf("descendant did not escape the original process group: %v", err)
	}
	if len(recorder.terminated) != 1 || recorder.terminated[0].Kind != LaunchManagedEnded || !recorder.terminated[0].WaitObserved ||
		recorder.terminated[0].CleanupScope != SupervisionProcessGroup {
		t.Fatalf("cleanup escaped its declared process-group scope: %+v", recorder.terminated)
	}
	if err := lifecycle.Seal(); err != nil {
		t.Fatalf("an escaped descendant blocked the ended direct launch: %v", err)
	}
}
