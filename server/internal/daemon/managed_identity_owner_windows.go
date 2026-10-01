//go:build windows

package daemon

import (
	"errors"
	"os"
	"syscall"
)

// Windows applies the user's profile ACL. POSIX mode bits do not represent it;
// native ACL and installed-profile acceptance remains an explicit OS test.
func managedOwnedByCurrentUser(_ os.FileInfo) bool { return true }

func managementProcessDefinitelyDead(pid int) bool {
	handle, err := syscall.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		return errors.Is(err, syscall.Errno(87))
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	if err = syscall.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code != 259
}
