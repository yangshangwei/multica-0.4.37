//go:build linux || darwin

package collaborationrun

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Open every component relative to its pinned parent descriptor. Neither a
// final symlink nor a parent replacement may turn an explicit source into an
// unrelated account directory, pipe, device or file outside the selection.
func openBundleSource(source string) (*os.File, error) {
	if !filepath.IsAbs(source) || filepath.Clean(source) != source || strings.ContainsRune(source, 0) {
		return nil, ErrUnsupported
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, ErrUnsupported
	}
	parts := strings.Split(strings.TrimPrefix(source, "/"), "/")
	for i, part := range parts {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(fd, part, flags, 0)
		_ = unix.Close(fd)
		if openErr != nil {
			return nil, ErrUnsupported
		}
		fd = next
		var st unix.Stat_t
		if unix.Fstat(fd, &st) != nil || (int(st.Uid) != os.Geteuid() && st.Uid != 0) ||
			st.Mode&0022 != 0 && (i == len(parts)-1 || st.Mode&unix.S_ISVTX == 0 || st.Uid != 0) {
			_ = unix.Close(fd)
			return nil, ErrUnsupported
		}
		if i == len(parts)-1 && st.Mode&unix.S_IFMT != unix.S_IFREG {
			_ = unix.Close(fd)
			return nil, ErrUnsupported
		}
	}
	return os.NewFile(uintptr(fd), source), nil
}
