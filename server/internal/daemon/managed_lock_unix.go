//go:build !windows

package daemon

import "os"

func openManagementLockFile(path string) (*os.File, error) {
	return os.Open(path)
}
