//go:build windows

package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func openManagementLockFile(path string) (*os.File, error) {
	openPath, err := managementLockWindowsPath(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	name, err := syscall.UTF16PtrFromString(openPath)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	// os.Open omits FILE_SHARE_DELETE, conflicting with peer ticket publication
	// and release. Match Node's sharing mode without changing ticket ownership.
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

// CreateFile does not perform os.Open's long-path conversion. Resolve relative
// paths before applying the extended prefix, which disables Win32 normalization.
func managementLockWindowsPath(path string) (string, error) {
	path = filepath.FromSlash(path)
	if strings.HasPrefix(path, extendedLengthPrefix) || strings.HasPrefix(path, `\\.\`) || strings.HasPrefix(path, `\??\`) {
		return path, nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if fitsWithoutExtendedLengthPrefix(absolute) {
		return absolute, nil
	}
	if strings.HasPrefix(absolute, `\\`) {
		return extendedLengthUNCPrefix + absolute[2:], nil
	}
	return extendedLengthPrefix + absolute, nil
}

func renameManagementLockFile(from, to string) error {
	return renameManagementLockFileUntil(from, to, time.Now().Add(time.Second))
}

func renameManagementLockFileUntil(from, to string, deadline time.Time) error {
	for {
		err := os.Rename(from, to)
		// MoveFileEx cannot replace an open destination even if its readers share
		// DELETE access. Keep the choosing ticket visible until short reads finish;
		// never unlink the destination or restart the protected identity operation.
		if err == nil || (!errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, syscall.Errno(32))) {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return err
		}
		time.Sleep(min(10*time.Millisecond, remaining))
	}
}
