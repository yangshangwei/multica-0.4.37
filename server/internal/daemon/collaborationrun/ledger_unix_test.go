//go:build linux || darwin

package collaborationrun

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPrivateFileReopensAtomicallyReplacedRead(t *testing.T) {
	dir := newTestLedger(t).dir
	path := filepath.Join(dir, "state.json")
	if err := atomicPrivateWrite(path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	opens := 0
	f, err := privateFileWithOpen(path, unix.O_RDONLY, func(path string, flags int, mode uint32) (int, error) {
		fd, err := unix.Open(path, flags, mode)
		if err != nil {
			return fd, err
		}
		opens++
		if opens == 1 {
			if err := atomicPrivateWrite(path, []byte("new")); err != nil {
				t.Fatal(err)
			}
			var stat unix.Stat_t
			if err := unix.Fstat(fd, &stat); err != nil || stat.Nlink != 0 {
				t.Fatalf("atomic replacement did not unlink the opened inode: %v, links=%d", err, stat.Nlink)
			}
		}
		return fd, nil
	})
	if err != nil {
		t.Fatalf("valid atomic replacement rejected: %v", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil || string(raw) != "new" || opens != 2 {
		t.Fatalf("read did not reopen the current private file: content=%q opens=%d error=%v", raw, opens, err)
	}
}

func TestPrivateFileReplacementRevalidatesTheNewPath(t *testing.T) {
	for _, replacement := range []string{"hardlink", "symlink", "shared", "directory", "missing"} {
		t.Run(replacement, func(t *testing.T) {
			dir := newTestLedger(t).dir
			path := filepath.Join(dir, "state.json")
			if err := atomicPrivateWrite(path, []byte("old")); err != nil {
				t.Fatal(err)
			}
			opens := 0
			f, err := privateFileWithOpen(path, unix.O_RDONLY, func(path string, flags int, mode uint32) (int, error) {
				opens++
				fd, err := unix.Open(path, flags, mode)
				if err != nil || opens != 1 {
					return fd, err
				}
				if err := atomicPrivateWrite(path, []byte("new")); err != nil {
					t.Fatal(err)
				}
				switch replacement {
				case "hardlink":
					err = os.Link(path, filepath.Join(dir, "alias"))
				case "symlink":
					target := filepath.Join(dir, "target")
					if err = os.Rename(path, target); err == nil {
						err = os.Symlink(target, path)
					}
				case "shared":
					err = os.Chmod(path, 0644)
				case "directory", "missing":
					if err = os.Remove(path); err == nil && replacement == "directory" {
						err = os.Mkdir(path, 0700)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				return fd, nil
			})
			if f != nil {
				_ = f.Close()
			}
			if err == nil || f != nil || opens != 2 {
				t.Fatalf("replacement %s was not independently rejected: opens=%d error=%v", replacement, opens, err)
			}
			if replacement == "missing" && (!errors.Is(err, ErrUnconfirmed) || errors.Is(err, os.ErrNotExist)) {
				t.Fatalf("disappeared existing state must not be treated as a fresh record: %v", err)
			}
		})
	}
}

func TestPrivateFileInitiallyMissingRemainsNewState(t *testing.T) {
	f, err := openPrivateFile(filepath.Join(newTestLedger(t).dir, "missing.json"))
	if f != nil {
		_ = f.Close()
	}
	if f != nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("initially absent state no longer permits registration: %v", err)
	}
}

func TestPrivateFileReplacementNeverRetriesLocksOrUnsafeOriginals(t *testing.T) {
	for _, kind := range []string{"lock", "shared_original", "hardlinked_original"} {
		t.Run(kind, func(t *testing.T) {
			dir := newTestLedger(t).dir
			path := filepath.Join(dir, "state")
			if err := atomicPrivateWrite(path, []byte("old")); err != nil {
				t.Fatal(err)
			}
			flags := unix.O_RDONLY
			if kind == "lock" {
				flags = unix.O_RDWR | unix.O_CREAT
			}
			opens := 0
			f, err := privateFileWithOpen(path, flags, func(path string, flags int, mode uint32) (int, error) {
				opens++
				fd, err := unix.Open(path, flags, mode)
				if err != nil {
					return fd, err
				}
				if kind == "hardlinked_original" {
					err = os.Link(path, filepath.Join(dir, "alias"))
				} else {
					if kind == "shared_original" {
						err = unix.Fchmod(fd, 0644)
					}
					if err == nil {
						err = atomicPrivateWrite(path, []byte("new"))
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				return fd, nil
			})
			if f != nil {
				_ = f.Close()
			}
			if err == nil || f != nil || opens != 1 {
				t.Fatalf("unsafe original or lock retried: opens=%d error=%v", opens, err)
			}
		})
	}
}

func TestPrivateFileReplacementRetriesAreBoundedAndCloseDescriptors(t *testing.T) {
	dir := newTestLedger(t).dir
	path := filepath.Join(dir, "state.json")
	if err := atomicPrivateWrite(path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	var descriptors []int
	f, err := privateFileWithOpen(path, unix.O_RDONLY, func(path string, flags int, mode uint32) (int, error) {
		fd, err := unix.Open(path, flags, mode)
		if err != nil {
			return fd, err
		}
		descriptors = append(descriptors, fd)
		if err := atomicPrivateWrite(path, []byte("new")); err != nil {
			t.Fatal(err)
		}
		return fd, nil
	})
	if f != nil {
		_ = f.Close()
	}
	if err == nil || f != nil || len(descriptors) != 3 {
		t.Fatalf("continuous replacement was not bounded: opens=%d error=%v", len(descriptors), err)
	}
	for _, fd := range descriptors {
		var stat unix.Stat_t
		if err := unix.Fstat(fd, &stat); !errors.Is(err, unix.EBADF) {
			t.Fatalf("rejected descriptor remains open: %v", err)
		}
	}
}
