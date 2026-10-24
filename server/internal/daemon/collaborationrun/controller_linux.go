//go:build linux

package collaborationrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/protocol"
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
	facts := os.NewFile(6, "collaboration-preparation-facts")
	if config == nil || ready == nil || liveness == nil {
		return 125
	}
	unix.CloseOnExec(5)
	unix.CloseOnExec(6)
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
	var input ProviderInput
	workspaceBytes := int64(64 << 20)
	if request.Execution != nil {
		if decodeStrict(request.Input, &input) != nil || protocol.ValidateCollaborationPreparation(input.Preparation, input.Identity, input.Inputs) != nil {
			return fail("invalid_preparation")
		}
		workspaceBytes = input.Preparation.Limits.WorkspaceBytes
	}
	if request.Git != nil {
		workspaceBytes = request.Git.Limits.WorkspaceBytes
	}
	if err := isolateRoot(request.Root, workspaceBytes, request.Execution == nil && !request.ProbeGit, request.Devices); err != nil {
		return fail(namespaceSetupCode(err))
	}
	namespaceID, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return fail("namespace_identity")
	}
	// Protect the trusted init's in-memory request and control FDs before any
	// nested preparation. Remaining capabilities are dropped before the provider.
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		return fail("protect_supervisor")
	}
	if err := json.NewEncoder(ready).Encode(namespaceReady{NamespaceID: namespaceID}); err != nil {
		return 125
	}
	_ = ready.Close()
	if request.Probe {
		if request.ProbeGit {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := runNestedGit(ctx, namespaceRequest{Root: "/.preparation/git-root", Probe: true, Devices: request.ProbeGitDevices}, "")
			cancel()
			if err != nil {
				return 125
			}
		}
		if dropNamespacePrivileges() != nil {
			return 125
		}
		return 0
	}
	if request.Git != nil {
		if dropNamespacePrivileges() != nil {
			return 125
		}
		return executePreparedGit(*request.Git)
	}
	if request.Execution != nil {
		return executePreparedNamespace(request, input, facts)
	}
	if dropNamespacePrivileges() != nil {
		return 125
	}
	cmd := exec.Command("/agent", request.Arguments...)
	cmd.Dir = "/work"
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

func isolateRoot(root string, workspaceBytes int64, readOnly bool, devices []string) error {
	if workspaceBytes < 1 || workspaceBytes > 512<<20 || !validBundleDevices(devices) {
		return ErrUnsupported
	}
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("private_mount: %w", err)
	}
	if err := unix.Mount(root, root, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("bind_root: %w", err)
	}
	if err := unix.Mount("/dev/null", filepath.Join(root, "dev/null"), "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind_null: %w", err)
	}
	for _, device := range devices {
		if err := bindNamespaceEntropy(root, device); err != nil {
			return err
		}
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
	for dir, size := range map[string]int64{"/work": workspaceBytes, "/tmp": 64 << 20, "/home": 16 << 20} {
		// Legacy roots predate the separate provider home directory.
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create_%s: %w", filepath.Base(dir), err)
		}
		if err := unix.Mount("tmpfs", dir, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV, fmt.Sprintf("size=%d,mode=0700", size)); err != nil {
			return fmt.Errorf("mount_%s: %w", filepath.Base(dir), err)
		}
	}
	if !readOnly {
		return nil
	}
	return makeNamespaceRootReadOnly()
}

func makeNamespaceRootReadOnly() error {
	if err := unix.Mount("", "/", "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV, ""); err != nil {
		return fmt.Errorf("readonly_root: %w", err)
	}
	return nil
}

func namespaceSetupCode(err error) string {
	stage, _, _ := strings.Cut(err.Error(), ":")
	switch stage {
	case "private_mount", "bind_root", "bind_null", "mount_proc", "pivot_root", "root_chdir", "unmount_old_root", "remove_old_root",
		"create_work", "create_tmp", "create_home", "mount_work", "mount_tmp", "mount_home", "readonly_root", "bind_urandom", "entropy_device":
	default:
		stage = "isolate_root"
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return fmt.Sprintf("%s_errno_%d", stage, int(errno))
	}
	return stage
}

// Entropy is an explicit device capability, separate from readonly file data.
// MNT_READONLY does not deny writes to a character device; callers must not
// describe this bind as a readonly descriptor or expose it without opt-in.
func bindNamespaceEntropy(root, device string) error {
	if device != "/dev/urandom" {
		return ErrUnsupported
	}
	fd, err := unix.Open(device, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("entropy_device: %w", err)
	}
	defer unix.Close(fd)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil || st.Mode&unix.S_IFMT != unix.S_IFCHR || unix.Major(uint64(st.Rdev)) != 1 || unix.Minor(uint64(st.Rdev)) != 9 {
		return fmt.Errorf("entropy_device: %w", ErrUnsupported)
	}
	// Bind the checked descriptor, never reopen a replaceable pathname.
	if err := unix.Mount(fmt.Sprintf("/proc/self/fd/%d", fd), filepath.Join(root, "dev/urandom"), "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind_urandom: %w", err)
	}
	// Protect the source inode's metadata (not character-device I/O) from a
	// provider that happens to have the same numeric owner as the source.
	if err := unix.Mount("", filepath.Join(root, "dev/urandom"), "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("bind_urandom: %w", err)
	}
	return nil
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
