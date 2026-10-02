//go:build !windows

package daemon

import "os"

func openManagementLockFile(path string) (*os.File, error) {
	return os.Open(path)
}

func renameManagementLockFile(from, to string) error {
	return os.Rename(from, to)
}

func retryManagementLockIO(operation func() error) error {
	return operation()
}
