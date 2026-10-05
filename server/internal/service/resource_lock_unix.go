//go:build unix

package service

import (
	"golang.org/x/sys/unix"
	"os"
)

func resourceLock(file *os.File) (bool, error) {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == unix.EWOULDBLOCK || err == unix.EAGAIN {
		return false, nil
	}
	return err == nil, err
}
func resourceUnlock(file *os.File) error { return unix.Flock(int(file.Fd()), unix.LOCK_UN) }
func resourceWriteFlags() int            { return unix.O_NONBLOCK | unix.O_NOFOLLOW }

// All store files are direct children of a confined directory. openat enforces
// NOFOLLOW at the file open, including while index.json is atomically replaced.
func resourceOpenFile(root *os.Root, name string, flags int, mode os.FileMode) (*os.File, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	fd, err := unix.Openat(int(directory.Fd()), name, flags|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode.Perm()))
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
