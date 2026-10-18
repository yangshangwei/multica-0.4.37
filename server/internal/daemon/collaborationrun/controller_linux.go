//go:build linux

package collaborationrun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/multica-ai/multica/server/internal/cli"
	"golang.org/x/sys/unix"
)

func namespaceSupported() bool { return true }

func configureNamespace(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:                 unix.CLONE_NEWUSER | unix.CLONE_NEWPID | unix.CLONE_NEWNS | unix.CLONE_NEWIPC | unix.CLONE_NEWUTS,
		UidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Geteuid(), Size: 1}},
		GidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getegid(), Size: 1}},
		GidMappingsEnableSetgroups: false,
	}
}

// NamespaceHelperMain must be called by the CLI before ordinary command setup.
// The helper cannot run in the host PID namespace and never receives a daemon or
// termination credential. Its death makes Linux kill every namespace member.
func NamespaceHelperMain() int {
	if os.Getpid() != 1 {
		return 125
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	config := os.NewFile(3, "collaboration-config")
	ready := os.NewFile(4, "collaboration-ready")
	liveness := os.NewFile(5, "collaboration-parent")
	if config == nil || ready == nil || liveness == nil {
		return 125
	}
	unix.CloseOnExec(5)
	fail := func(code string) int {
		_ = json.NewEncoder(ready).Encode(namespaceReady{ErrorCode: code})
		_ = ready.Close()
		return 125
	}
	raw, err := io.ReadAll(io.LimitReader(config, 2*1024*1024+1))
	_ = config.Close()
	var request namespaceRequest
	if err != nil || len(raw) > 2*1024*1024 || decodeStrict(raw, &request) != nil || !filepath.IsAbs(request.Root) {
		return fail("invalid_helper_request")
	}
	// The writer is held only outside the namespace. The untrusted child cannot
	// keep it open; ptrace and /proc/1/fd access are disabled before child start.
	go func() { var b [1]byte; _, _ = liveness.Read(b[:]); os.Exit(137) }()
	if err := isolateRoot(request.Root); err != nil {
		return fail(err.Error())
	}
	namespaceID, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return fail("namespace_identity")
	}
	if err := dropNamespacePrivileges(); err != nil {
		return fail("drop_privileges")
	}
	if err := json.NewEncoder(ready).Encode(namespaceReady{NamespaceID: namespaceID}); err != nil {
		return 125
	}
	_ = ready.Close()
	if request.Probe {
		return 0
	}
	cmd := exec.Command("/agent", request.Arguments...)
	cmd.Dir = "/work"
	var input ProviderInput
	if decodeStrict(request.Input, &input) != nil {
		return 125
	}
	runEnvironment, err := cli.CollaborationEnvironment(input.Identity, request.ServerURL, request.BusinessToken)
	if err != nil {
		return 125
	}
	cmd.Env = append([]string{"LANG=C", "HOME=/work", "TMPDIR=/tmp", "PATH=/"}, runEnvironment...)
	cmd.Stdin = bytes.NewReader(request.Input)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return 125
	}
	if err := cmd.Wait(); err != nil {
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() < 0 {
			return 137
		}
		return cmd.ProcessState.ExitCode()
	}
	// Returning from PID 1 destroys descendants, including ones which double-
	// forked, changed process group/session or already closed their stdio.
	return 0
}

func isolateRoot(root string) error {
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("private_mount: %w", err)
	}
	if err := unix.Mount(root, root, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("bind_root: %w", err)
	}
	if err := unix.Mount("/dev/null", filepath.Join(root, "dev/null"), "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind_null: %w", err)
	}
	// Linux checks the inherited proc visibility when mounting proc from a user
	// namespace. Mount the new PID view before detaching the old root.
	if err := unix.Mount("proc", filepath.Join(root, "proc"), "proc", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("mount_proc: %w", err)
	}
	if err := unix.PivotRoot(root, filepath.Join(root, ".oldroot")); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("root_chdir: %w", err)
	}
	if err := unix.Unmount("/.oldroot", unix.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount_old_root: %w", err)
	}
	if err := os.Remove("/.oldroot"); err != nil {
		return fmt.Errorf("remove_old_root: %w", err)
	}
	for _, dir := range []string{"/work", "/tmp"} {
		if err := unix.Mount("tmpfs", dir, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV, "size=64m,mode=0700"); err != nil {
			return err
		}
	}
	return unix.Mount("", "/", "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV, "")
}

func dropNamespacePrivileges() error {
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		return err
	}
	// Lock out exec's special root-capability behavior before clearing all sets.
	if err := unix.Prctl(unix.PR_SET_SECUREBITS, 1|2|4|8, 0, 0, 0); err != nil {
		return err
	}
	for capability := uintptr(0); capability < 64; capability++ {
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, capability, 0, 0, 0); err != nil {
			if err == unix.EINVAL {
				break
			}
			return err
		}
	}
	var empty [2]unix.CapUserData
	return unix.Capset(&unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}, &empty[0])
}
