//go:build linux || darwin

package collaborationrun

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Getuid()
}

func privateFile(path string, flags int) (*os.File, error) {
	fd, err := unix.Open(path, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || !ownedByCurrentUser(info) || !ok || stat.Nlink != 1 {
		_ = f.Close()
		return nil, errors.New("collaboration ledger file is not private")
	}
	return f, nil
}

func openPrivateFile(path string) (*os.File, error) { return privateFile(path, unix.O_RDONLY) }

func lockPrivateFile(path string) (*os.File, error) {
	f, err := privateFile(path, unix.O_RDWR|unix.O_CREAT)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return f, nil
}

func unlockPrivateFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
