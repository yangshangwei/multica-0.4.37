package collaborationrun

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

const maxGitIndexBytes = 16 << 20

type gitPreparationRequest struct {
	Executable string                                  `json:"executable"`
	Git        protocol.CollaborationPreparationGit    `json:"git"`
	Limits     protocol.CollaborationPreparationLimits `json:"limits"`
	Deadline   time.Time                               `json:"deadline"`
}

type gitTreeEntry struct {
	Path      string `json:"path"`
	Mode      string `json:"mode"`
	OID       string `json:"oid"`
	SizeBytes int64  `json:"size_bytes"`
}

func gitObjectID(value, format string) bool {
	length := 40
	if format == "sha256" {
		length = 64
	} else if format != "sha1" {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && len(value) == length && value == strings.ToLower(value)
}

func validGitWorkPath(value string) bool {
	if value == "" || value == "." || path.IsAbs(value) || path.Clean(value) != value || len(value) > 4096 || strings.ContainsAny(value, "\x00\\") {
		return false
	}
	parts := strings.Split(value, "/")
	if len(parts) > 128 || parts[0] == "attachments" {
		return false
	}
	for _, part := range parts {
		if part == "." || part == ".." || part == ".git" {
			return false
		}
	}
	return true
}

func validateGitTree(entries []gitTreeEntry, format string, limits protocol.CollaborationPreparationLimits) error {
	if len(entries) > limits.EntryCount || limits.EntryCount < 1 || limits.EntryCount > 65536 {
		return ErrProtocol
	}
	seen, total := make(map[string]bool, len(entries)), int64(0)
	for _, entry := range entries {
		if !validGitWorkPath(entry.Path) || seen[entry.Path] || (entry.Mode != "100644" && entry.Mode != "100755") ||
			!gitObjectID(entry.OID, format) || entry.SizeBytes < 0 || entry.SizeBytes > limits.PerFileBytes || entry.SizeBytes > 100<<20 {
			return ErrProtocol
		}
		seen[entry.Path] = true
		total += entry.SizeBytes
	}
	for name := range seen {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if seen[parent] {
				return ErrProtocol
			}
		}
	}
	if total > limits.GitExpandedBytes || limits.GitExpandedBytes > 256<<20 {
		return ErrProtocol
	}
	return nil
}

// ls-tree is machine-readable and includes gitlinks. Checking it before archive
// prevents submodules and export-ignore attributes from silently disappearing.
func parseGitTree(raw []byte, format string, limits protocol.CollaborationPreparationLimits) ([]gitTreeEntry, error) {
	if len(raw) > maxGitIndexBytes || len(raw) > 0 && raw[len(raw)-1] != 0 {
		return nil, ErrProtocol
	}
	entries := []gitTreeEntry{}
	for _, line := range bytes.Split(raw, []byte{0}) {
		if len(line) == 0 {
			continue
		}
		header, name, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(header)
		if !ok || len(fields) != 4 || fields[1] != "blob" {
			return nil, ErrProtocol
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || size < 0 {
			return nil, ErrProtocol
		}
		entries = append(entries, gitTreeEntry{Path: name, Mode: fields[0], OID: fields[2], SizeBytes: size})
	}
	return entries, validateGitTree(entries, format, limits)
}

type prefixCapture struct{ bytes []byte }

func (p *prefixCapture) Write(data []byte) (int, error) {
	n := len(data)
	remaining := 1024 - len(p.bytes)
	if remaining > 0 {
		p.bytes = append(p.bytes, data[:min(remaining, len(data))]...)
	}
	return n, nil
}

func extractGitArchive(ctx context.Context, source io.Reader, work string, entries []gitTreeEntry, format, commit string, limits protocol.CollaborationPreparationLimits) error {
	if !gitObjectID(commit, format) {
		return ErrProtocol
	}
	if err := validateGitTree(entries, format, limits); err != nil {
		return err
	}
	expected := make(map[string]gitTreeEntry, len(entries))
	directories := make(map[string]bool)
	for _, entry := range entries {
		expected[entry.Path] = entry
		for parent := path.Dir(entry.Path); parent != "."; parent = path.Dir(parent) {
			directories[parent] = true
		}
	}
	// Bound tar framing too; many empty/PAX entries cannot bypass byte limits.
	bounded := &io.LimitedReader{R: contextReader{ctx, source}, N: limits.GitExpandedBytes + int64(limits.EntryCount)*2048 + 1<<20}
	tarReader := tar.NewReader(bounded)
	seen, extracted, count := map[string]bool{}, 0, 0
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ErrProtocol
		}
		count++
		// git archive emits a PAX global comment when exporting a commit.
		// Only that exact identity is metadata; global path/size overrides and
		// other header types must not change what is materialized.
		if header.Typeflag == tar.TypeXGlobalHeader {
			if count != 1 || count > limits.EntryCount || header.Name != "pax_global_header" || len(header.PAXRecords) != 1 || header.PAXRecords["comment"] != commit {
				return ErrProtocol
			}
			continue
		}
		name := header.Name
		if header.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if count > limits.EntryCount || !validGitWorkPath(name) || seen[name] {
			return ErrProtocol
		}
		seen[name] = true
		destination := filepath.Join(work, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if _, conflict := expected[name]; conflict || header.Size != 0 || !directories[name] {
				return ErrProtocol
			}
			if err := os.MkdirAll(destination, 0700); err != nil {
				return ErrProtocol
			}
		case tar.TypeReg, tar.TypeRegA:
			entry, ok := expected[name]
			if !ok || header.Size != entry.SizeBytes || (header.Mode&0111 != 0) != (entry.Mode == "100755") {
				return ErrProtocol
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
				return ErrProtocol
			}
			mode := os.FileMode(0600)
			if entry.Mode == "100755" {
				mode = 0700
			}
			f, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return ErrProtocol
			}
			var digest hash.Hash = sha1.New()
			if format == "sha256" {
				digest = sha256.New()
			}
			_, _ = fmt.Fprintf(digest, "blob %d\x00", entry.SizeBytes)
			var prefix prefixCapture
			n, copyErr := io.Copy(io.MultiWriter(f, digest, &prefix), contextReader{ctx, tarReader})
			closeErr := f.Close()
			if copyErr != nil || closeErr != nil || n != entry.SizeBytes || hex.EncodeToString(digest.Sum(nil)) != entry.OID ||
				bytes.HasPrefix(prefix.bytes, []byte("version https://git-lfs.github.com/spec/v1\n")) {
				return ErrProtocol
			}
			extracted++
		default:
			return ErrProtocol
		}
	}
	if extracted != len(expected) {
		return ErrProtocol
	}
	if _, err := io.Copy(io.Discard, bounded); err != nil || bounded.N == 0 {
		return ErrProtocol
	}
	return nil
}
