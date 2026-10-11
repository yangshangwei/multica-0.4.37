//go:build !windows

package main

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func openInitializationDirectory(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil || int(stat.Uid) != os.Geteuid() || stat.Mode&0022 != 0 {
		_ = file.Close()
		return nil, errors.New("credentials_directory_unsafe")
	}
	return file, nil
}

func openInitializationFile(directory *os.File, name string, create bool) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	if create {
		flags = unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_NOFOLLOW | unix.O_CLOEXEC
	}
	fd, err := unix.Openat(int(directory.Fd()), name, flags, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
func checkInitializationFile(file *os.File) error {
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&07777 != 0600 || int(stat.Uid) != os.Geteuid() || stat.Nlink != 1 {
		return errors.New("credentials_file_unsafe")
	}
	return nil
}
func removeInitializationFile(directory *os.File, name string) error {
	return unix.Unlinkat(int(directory.Fd()), name, 0)
}
