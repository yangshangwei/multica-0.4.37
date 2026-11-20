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

func createPrivateDirectory(path string) error { return os.Mkdir(path, 0700) }

func privateDirectoryOwner(_ string, info os.FileInfo) bool {
	return info.Mode().Perm()&0077 == 0 && ownedByCurrentUser(info)
}

func protectNewPrivateFile(*os.File) error { return nil }

func replacePrivateFile(from, to string) error { return os.Rename(from, to) }

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

func privateFile(path string, flags int) (*os.File, error) {
	return privateFileWithOpen(path, flags, unix.Open)
}

func privateFileWithOpen(path string, flags int, open func(string, int, uint32) (int, error)) (*os.File, error) {
	for attempt := 0; attempt < 3; attempt++ {
		fd, err := open(path, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
		if err != nil {
			if attempt > 0 {
				// A previously opened record existed. Losing its replacement
				// must not become the initial-absence path used by registration.
				return nil, ErrUnconfirmed
			}
			return nil, err
		}
		f := os.NewFile(uintptr(fd), path)
		info, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || !ownedByCurrentUser(info) || !ok {
			_ = f.Close()
			return nil, errors.New("collaboration ledger file is not private")
		}
		if stat.Nlink == 1 {
			return f, nil
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		// Atomic publication can unlink this inode between Open and Stat.
		// Reopen and validate the current path; never accept the stale inode,
		// retry a lock file, or treat multiple links as ordinary replacement.
		if flags != unix.O_RDONLY || stat.Nlink != 0 {
			return nil, errors.New("collaboration ledger file is not private")
		}
	}
	return nil, ErrUnconfirmed
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
