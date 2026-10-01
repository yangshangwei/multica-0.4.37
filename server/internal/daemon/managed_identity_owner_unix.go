//go:build !windows

package daemon

import (
	"errors"
	"os"
	"syscall"
)

func managedOwnedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Getuid()
}

func managementProcessDefinitelyDead(pid int) bool {
	return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}
