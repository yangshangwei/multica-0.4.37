//go:build linux || darwin

package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type rootNode struct {
	name string
	file *os.File
	info os.FileInfo
}
type secureRoot struct {
	path     string
	root     *os.Root
	file     *os.File
	info     os.FileInfo
	identity string
	device   uint64
	mode     os.FileMode
	chain    []rootNode
}

func (r *secureRoot) close() {
	if r.root != nil {
		_ = r.root.Close()
	}
	for _, n := range r.chain {
		_ = n.file.Close()
	}
}

func statFile(file *os.File) (unix.Stat_t, error) {
	var st unix.Stat_t
	err := unix.Fstat(int(file.Fd()), &st)
	return st, err
}
func sameSnapshot(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func openSecureRoot(path string, mode os.FileMode) (r *secureRoot, err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return nil, ErrInvalid
	}
	r = &secureRoot{path: path, mode: mode}
	defer func() {
		if err != nil {
			r.close()
		}
	}()
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return r, ErrInvalid
	}
	top := os.NewFile(uintptr(fd), "/")
	topInfo, e := top.Stat()
	if e != nil {
		_ = top.Close()
		return r, ErrInvalid
	}
	r.chain = append(r.chain, rootNode{"/", top, topInfo})
	parent := top
	absolute := ""
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		absolute += "/" + part
		before, e := os.Lstat(absolute)
		if e != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return r, ErrInvalid
		}
		fd, e = unix.Openat(int(parent.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return r, ErrInvalid
		}
		file := os.NewFile(uintptr(fd), absolute)
		info, e := file.Stat()
		if e != nil {
			_ = file.Close()
			return r, ErrInvalid
		}
		r.chain = append(r.chain, rootNode{absolute, file, info})
		st, e := statFile(file)
		if e != nil || !os.SameFile(before, info) || (int(st.Uid) != os.Geteuid() && st.Uid != 0) || st.Mode&0022 != 0 && (st.Uid != 0 || st.Mode&unix.S_ISVTX == 0) {
			return r, ErrInvalid
		}
		parent = file
	}
	r.file = parent
	r.info = r.chain[len(r.chain)-1].info
	st, e := statFile(parent)
	if e != nil || int(st.Uid) != os.Geteuid() || r.info.Mode().Perm() != mode {
		return r, ErrInvalid
	}
	r.device = uint64(st.Dev)
	r.root, e = os.OpenRoot(path)
	if e != nil {
		return r, ErrInvalid
	}
	check, e := r.root.Open(".")
	if e != nil {
		return r, ErrInvalid
	}
	checkInfo, e := check.Stat()
	_ = check.Close()
	if e != nil || !os.SameFile(checkInfo, r.info) {
		return r, ErrInvalid
	}
	var identity strings.Builder
	for _, n := range r.chain {
		st, e := statFile(n.file)
		if e != nil {
			return r, ErrInvalid
		}
		fmt.Fprintf(&identity, "%d:%d;", st.Dev, st.Ino)
	}
	r.identity = identity.String()
	return r, nil
}

func (r *secureRoot) recheck() error {
	for _, n := range r.chain {
		current, err := os.Lstat(n.name)
		if err != nil || !os.SameFile(current, n.info) || current.Mode() != n.info.Mode() {
			return ErrInvalid
		}
		opened, err := n.file.Stat()
		if err != nil || !os.SameFile(opened, n.info) || opened.Mode() != n.info.Mode() {
			return ErrInvalid
		}
	}
	return nil
}

func (r *secureRoot) read(path string, limit int64, fault func(string) error) ([]byte, error) {
	if limit < 0 || limit > MaxFileBytes {
		return nil, ErrInvalid
	}
	parts := strings.Split(path, "/")
	parent := r.file
	var opened []rootNode
	defer func() {
		for _, n := range opened {
			_ = n.file.Close()
		}
	}()
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, ErrInvalid
		}
		prefix := strings.Join(parts[:i+1], "/")
		before, err := r.root.Lstat(prefix)
		if err != nil || before.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalid
		}
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		fd, err := unix.Openat(int(parent.Fd()), part, flags, 0)
		if err != nil {
			return nil, ErrInvalid
		}
		file := os.NewFile(uintptr(fd), prefix)
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return nil, ErrInvalid
		}
		opened = append(opened, rootNode{prefix, file, info})
		st, err := statFile(file)
		if err != nil || uint64(st.Dev) != r.device || int(st.Uid) != os.Geteuid() || !sameSnapshot(before, info) {
			return nil, ErrInvalid
		}
		if i < len(parts)-1 {
			if !info.IsDir() || info.Mode().Perm() != 0500 {
				return nil, ErrInvalid
			}
		} else if !info.Mode().IsRegular() || info.Mode().Perm() != 0400 || st.Nlink != 1 || info.Size() > limit {
			return nil, ErrInvalid
		}
		parent = file
	}
	bytes, err := io.ReadAll(io.LimitReader(parent, limit+1))
	if err != nil || int64(len(bytes)) > limit {
		return nil, ErrInvalid
	}
	if fault != nil {
		if err := fault("read:" + path); err != nil {
			return nil, err
		}
	}
	for _, n := range opened {
		current, err := r.root.Lstat(n.name)
		if err != nil || !sameSnapshot(current, n.info) {
			return nil, ErrInvalid
		}
		live, err := n.file.Stat()
		if err != nil || !sameSnapshot(live, n.info) {
			return nil, ErrInvalid
		}
		st, err := statFile(n.file)
		if err != nil || uint64(st.Dev) != r.device || int(st.Uid) != os.Geteuid() || live.Mode().IsRegular() && st.Nlink != 1 {
			return nil, ErrInvalid
		}
	}
	if err := r.recheck(); err != nil {
		return nil, err
	}
	return bytes, nil
}

type artifactOutput struct {
	parent       *secureRoot
	name         string
	directory    *os.File
	info         os.FileInfo
	artifactInfo os.FileInfo
	file         *os.File
	published    bool
}

func (o *artifactOutput) close() {
	if o.file != nil {
		_ = o.file.Close()
	}
	if o.directory != nil {
		_ = o.directory.Close()
	}
}
func (r *secureRoot) createOutput(id string) (*artifactOutput, error) {
	if err := unix.Mkdirat(int(r.file.Fd()), id, 0700); err != nil {
		return nil, err
	}
	o := &artifactOutput{parent: r, name: id}
	fd, err := unix.Openat(int(r.file.Fd()), id, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return o, err
	}
	o.directory = os.NewFile(uintptr(fd), id)
	o.info, err = o.directory.Stat()
	if err != nil {
		return o, err
	}
	st, err := statFile(o.directory)
	if err != nil || uint64(st.Dev) != r.device || int(st.Uid) != os.Geteuid() || o.info.Mode().Perm() != 0700 {
		return o, ErrInvalid
	}
	if err = o.check(); err != nil {
		return o, err
	}
	fd, err = unix.Openat(int(o.directory.Fd()), "artifact.partial", unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return o, err
	}
	o.file = os.NewFile(uintptr(fd), "artifact.partial")
	o.artifactInfo, err = o.file.Stat()
	if err != nil {
		return o, err
	}
	return o, nil
}
func (o *artifactOutput) check() error {
	if err := o.parent.recheck(); err != nil {
		return err
	}
	info, err := o.parent.root.Lstat(o.name)
	if err != nil || !os.SameFile(info, o.info) || info.Mode().Perm() != 0700 || !info.IsDir() {
		return ErrInvalid
	}
	return nil
}
func (o *artifactOutput) digest(name string) (string, int64, error) {
	if err := o.check(); err != nil {
		return "", 0, err
	}
	fd, err := unix.Openat(int(o.directory.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", 0, err
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}
	st, err := statFile(file)
	if err != nil || !os.SameFile(info, o.artifactInfo) || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || st.Nlink != 1 || uint64(st.Dev) != o.parent.device || int(st.Uid) != os.Geteuid() || info.Size() > MaxTotalBytes+MaxFiles*1024+1024 {
		return "", 0, ErrInvalid
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, info.Size()+1))
	if err != nil || n != info.Size() {
		return "", 0, ErrInvalid
	}
	final, err := file.Stat()
	if err != nil || !sameSnapshot(info, final) {
		return "", 0, ErrInvalid
	}
	st, err = statFile(file)
	if err != nil || st.Nlink != 1 {
		return "", 0, ErrInvalid
	}
	named, err := o.parent.root.Lstat(o.name + "/" + name)
	if err != nil || !sameSnapshot(info, named) {
		return "", 0, ErrInvalid
	}
	return hex.EncodeToString(hash.Sum(nil)), n, nil
}
func (o *artifactOutput) publish() error {
	if err := o.check(); err != nil {
		return err
	}
	// linkat creates the final name exclusively; it never replaces a collision.
	// The temporary link is removed before digest validation requires nlink=1.
	if err := unix.Linkat(int(o.directory.Fd()), "artifact.partial", int(o.directory.Fd()), "artifact.tar", 0); err != nil {
		return err
	}
	o.published = true
	return unix.Unlinkat(int(o.directory.Fd()), "artifact.partial", 0)
}
func (o *artifactOutput) sync() error {
	if err := o.check(); err != nil {
		return err
	}
	if err := o.directory.Sync(); err != nil {
		return err
	}
	return o.parent.file.Sync()
}
func (o *artifactOutput) cleanup() error {
	if o.published {
		return ErrInvalid
	}
	if err := o.check(); err != nil {
		return err
	}
	partial, err := o.parent.root.Lstat(o.name + "/artifact.partial")
	if err != nil || !os.SameFile(partial, o.artifactInfo) || !partial.Mode().IsRegular() {
		return ErrInvalid
	}
	if o.file != nil {
		if err := o.file.Close(); err != nil {
			return err
		}
		o.file = nil
	}
	if err := unix.Unlinkat(int(o.directory.Fd()), "artifact.partial", 0); err != nil {
		return err
	}
	if err := o.directory.Sync(); err != nil {
		return err
	}
	if err := unix.Unlinkat(int(o.parent.file.Fd()), o.name, unix.AT_REMOVEDIR); err != nil {
		return err
	}
	return o.parent.file.Sync()
}
