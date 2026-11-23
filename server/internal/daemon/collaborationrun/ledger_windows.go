//go:build windows

package collaborationrun

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ledgerUserSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return user.User.Sid, nil
}

func ledgerSecurityDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := ledgerUserSID()
	if err != nil {
		return nil, err
	}
	return windows.SecurityDescriptorFromString("O:" + user.String() + "D:P(A;OICI;FA;;;" + user.String() + ")(A;OICI;FA;;;SY)")
}

func ledgerWindowsPath(path string) (*uint16, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(absolute, `\\?\`) {
		if strings.HasPrefix(absolute, `\\`) {
			absolute = `\\?\UNC\` + absolute[2:]
		} else {
			absolute = `\\?\` + absolute
		}
	}
	return windows.UTF16PtrFromString(absolute)
}

func createPrivateDirectory(path string) error {
	name, err := ledgerWindowsPath(path)
	if err != nil {
		return err
	}
	sd, err := ledgerSecurityDescriptor()
	if err != nil {
		return err
	}
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	return windows.CreateDirectory(name, &attributes)
}

// POSIX mode/uid do not describe a Windows ACL. Every filesystem entry is
// instead checked through its native security descriptor before being used.
func ownedByCurrentUser(os.FileInfo) bool { return false }

func privateDirectoryOwner(path string, _ os.FileInfo) bool {
	name, err := ledgerWindowsPath(path)
	if err != nil {
		return false
	}
	sd, err := windows.GetNamedSecurityInfo(windows.UTF16PtrToString(name), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	return err == nil && privateWindowsDescriptor(sd)
}

func privateWindowsDescriptor(sd *windows.SECURITY_DESCRIPTOR) bool {
	user, err := ledgerUserSID()
	if err != nil {
		return false
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !owner.Equals(user) {
		return false
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount == 0 || dacl.AceCount > 8 {
		return false
	}
	hasUser := false
	for i := uint16(0); i < dacl.AceCount; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(dacl, uint32(i), &ace) != nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return false
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if sid.Equals(user) {
			hasUser = true
		} else if !sid.IsWellKnown(windows.WinLocalSystemSid) {
			return false
		}
	}
	return hasUser
}

func protectNewPrivateFile(f *os.File) error {
	sd, err := ledgerSecurityDescriptor()
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	name, err := ledgerWindowsPath(f.Name())
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(windows.UTF16PtrToString(name), windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, owner, nil, dacl, nil)
}

func windowsPrivateFile(path string, create bool) (*os.File, error) {
	name, err := ledgerWindowsPath(path)
	if err != nil {
		return nil, err
	}
	disposition, access := uint32(windows.OPEN_EXISTING), uint32(windows.GENERIC_READ)
	var attributes *windows.SecurityAttributes
	if create {
		sd, err := ledgerSecurityDescriptor()
		if err != nil {
			return nil, err
		}
		attributes = &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
		disposition, access = windows.OPEN_ALWAYS, windows.GENERIC_READ|windows.GENERIC_WRITE
	}
	handle, err := windows.CreateFile(name, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		attributes, disposition, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(handle), path)
	var info windows.ByHandleFileInformation
	sd, securityErr := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if windows.GetFileInformationByHandle(handle, &info) != nil || info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 ||
		info.NumberOfLinks != 1 || securityErr != nil || !privateWindowsDescriptor(sd) {
		_ = f.Close()
		return nil, errors.New("collaboration ledger file is not private")
	}
	return f, nil
}

func openPrivateFile(path string) (*os.File, error) { return windowsPrivateFile(path, false) }

func lockPrivateFile(path string) (*os.File, error) {
	f, err := windowsPrivateFile(path, true)
	if err != nil {
		return nil, err
	}
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return f, nil
}

func unlockPrivateFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &windows.Overlapped{})
}

func replacePrivateFile(from, to string) error {
	source, err := ledgerWindowsPath(from)
	if err != nil {
		return err
	}
	destination, err := ledgerWindowsPath(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(source, destination, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

// File data is flushed before the same-directory write-through rename above.
// Windows does not provide the Unix directory-fsync operation.
func syncDirectory(string) error { return nil }
