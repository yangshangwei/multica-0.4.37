//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// prepareDaemonChildFds marks descriptors inherited from the launching
// process close-on-exec before a detached daemon is started. The daemon's
// intended stdin/stdout/stderr are mapped explicitly by os/exec, which clears
// close-on-exec on those child mappings. This prevents an Electron controller's
// debug sockets, Chromium files, and profile database handles from keeping the
// controller's child-process pipe open after Electron exits.
func prepareDaemonChildFds() error {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return fmt.Errorf("enumerate inherited descriptors: %w", err)
	}
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil || fd < 3 {
			continue
		}
		flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
		if err != nil {
			// A descriptor can close between readdir and fcntl. That is not an
			// inherited descriptor that needs protection.
			if errors.Is(err, unix.EBADF) {
				continue
			}
			return fmt.Errorf("read descriptor %d flags: %w", fd, err)
		}
		if flags&unix.FD_CLOEXEC != 0 {
			continue
		}
		if _, err := unix.FcntlInt(uintptr(fd), unix.F_SETFD, flags|unix.FD_CLOEXEC); err != nil {
			if errors.Is(err, unix.EBADF) {
				continue
			}
			return fmt.Errorf("mark descriptor %d close-on-exec: %w", fd, err)
		}
	}
	return nil
}
