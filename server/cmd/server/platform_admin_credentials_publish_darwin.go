package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// Publishing must not expose a second link or overwrite an existing file.
func publishInitializationFile(directory *os.File, temp, name string) error {
	return unix.RenameatxNp(int(directory.Fd()), temp, int(directory.Fd()), name, unix.RENAME_EXCL)
}
