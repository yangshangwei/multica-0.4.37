package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func readManagedPID(path string) (int, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64 || !managedOwnedByCurrentUser(info) {
		return 0, errors.New("invalid managed daemon PID evidence")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(raw))
	pid, err := strconv.ParseUint(value, 10, 32)
	if err != nil || pid == 0 || strconv.FormatUint(pid, 10) != value {
		return 0, errors.New("invalid managed daemon PID evidence")
	}
	return int(pid), nil
}

func withManagedPIDLock(homeDirectory, profile string, operation func(string) error) error {
	directory, err := managedProfileDirectory(homeDirectory, profile)
	if err != nil {
		return err
	}
	if err := managedEnsureDirectory(directory, false); err != nil {
		return err
	}
	return withManagementFileLock(directory, func() error { return operation(directory) })
}

// CheckManagedProcessAbsent requires every available PID proof to be absent.
// permittedPID is used only for a foreground child's reserved PID or its own
// drained predecessor handing over to a successor while holding this lock.
func checkManagedProcessAbsent(homeDirectory, profile, directory string, permittedPID int) error {
	pid, err := readManagedPID(filepath.Join(directory, "daemon.pid"))
	if err != nil {
		return err
	}
	if permittedPID > 0 && pid == permittedPID {
		return nil
	}
	if pid > 0 && !ManagementProcessExited(pid) {
		return errors.New("managed daemon process exit is not confirmed")
	}
	control, err := readManagementControlFile(homeDirectory, profile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if control.PID != permittedPID && !ManagementProcessExited(control.PID) {
		return errors.New("managed control process exit is not confirmed")
	}
	return nil
}

func CheckManagedProcessAbsent(homeDirectory, profile string, permittedPID int) error {
	return withManagedPIDLock(homeDirectory, profile, func(directory string) error {
		return checkManagedProcessAbsent(homeDirectory, profile, directory, permittedPID)
	})
}

// ReserveManagedProcess serializes check, spawn and atomic PID publication.
// A foreground successor cannot run until its creator releases this lock.
func ReserveManagedProcess(homeDirectory, profile string, permittedPID int, start func() (int, error)) error {
	return withManagedPIDLock(homeDirectory, profile, func(directory string) error {
		if err := checkManagedProcessAbsent(homeDirectory, profile, directory, permittedPID); err != nil {
			return err
		}
		pid, err := start()
		if err != nil {
			return err
		}
		if pid <= 0 {
			return errors.New("invalid managed process reservation")
		}
		return writeManagementJSON(filepath.Join(directory, "daemon.pid"), pid)
	})
}

// ReleaseManagedProcess leaves successor, live, and unknown/corrupt evidence
// untouched. A live foreground process retains its tombstone until an OS probe
// proves exit; the next reservation safely replaces it after that point.
func ReleaseManagedProcess(homeDirectory, profile string, pid int) error {
	return withManagedPIDLock(homeDirectory, profile, func(directory string) error {
		path := filepath.Join(directory, "daemon.pid")
		current, err := readManagedPID(path)
		if err != nil {
			return err
		}
		if current != pid || !ManagementProcessExited(pid) {
			return nil
		}
		return os.Remove(path)
	})
}
