//go:build unix

package service

import (
	"os"
	"syscall"
)

func mcpManifestOpenFlags() int {
	// NONBLOCK prevents a regular-file-to-FIFO swap from hanging a request.
	// Root confines symlink resolution; Lstat and inode checks reject changed
	// file identities before any content is read.
	return os.O_RDONLY | syscall.O_NONBLOCK | syscall.O_NOFOLLOW
}
