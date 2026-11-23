package collaborationrun

import (
	"archive/tar"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitTestBlob(data string) string {
	h := sha1.New()
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(data))
	_, _ = h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func gitTestArchive(t *testing.T, headers []*tar.Header, bodies []string) []byte {
	t.Helper()
	var data bytes.Buffer
	w := tar.NewWriter(&data)
	for i, header := range headers {
		if err := w.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(bodies[i])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestPreparationGitArchiveRejectsEscapesLinksObjectDriftAndLimits(t *testing.T) {
	for _, kind := range []string{"valid", "git_commit_header", "wrong_commit_header", "global_path_header", "absolute", "parent", "symlink", "hardlink", "device", "duplicate", "object_mismatch", "export_ignore", "lfs_pointer", "file_limit", "entry_limit", "size_limit", "git_directory", "attachment_collision"} {
		t.Run(kind, func(t *testing.T) {
			data := "fixed committed content"
			file := gitTreeEntry{Path: "src/main.txt", Mode: "100644", OID: gitTestBlob(data), SizeBytes: int64(len(data))}
			limits := preparationTestCapabilities().Limits
			headers := []*tar.Header{{Name: file.Path, Mode: 0644, Typeflag: tar.TypeReg, Size: file.SizeBytes}}
			bodies := []string{data}
			switch kind {
			case "git_commit_header", "wrong_commit_header", "global_path_header":
				headers = append([]*tar.Header{{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, Format: tar.FormatPAX, PAXRecords: map[string]string{"comment": strings.Repeat("a", 40)}}}, headers...)
				bodies = append([]string{""}, bodies...)
				if kind == "wrong_commit_header" {
					headers[0].PAXRecords["comment"] = strings.Repeat("b", 40)
				}
				if kind == "global_path_header" {
					headers[0].PAXRecords["path"] = "../outside"
				}
			case "absolute":
				headers[0].Name = "/outside"
			case "parent":
				headers[0].Name = "../outside"
			case "symlink", "hardlink", "device":
				headers[0].Typeflag = map[string]byte{"symlink": tar.TypeSymlink, "hardlink": tar.TypeLink, "device": tar.TypeChar}[kind]
				headers[0].Linkname, headers[0].Size, bodies[0] = "/outside", 0, ""
			case "duplicate":
				headers, bodies = append(headers, headers[0]), append(bodies, data)
			case "object_mismatch":
				file.OID = strings.Repeat("f", 40)
			case "export_ignore":
				headers, bodies = nil, nil
			case "lfs_pointer":
				data = "version https://git-lfs.github.com/spec/v1\noid sha256:" + strings.Repeat("a", 64) + "\nsize 1234\n"
				file.SizeBytes, file.OID = int64(len(data)), gitTestBlob(data)
				headers[0].Size, bodies[0] = file.SizeBytes, data
			case "file_limit":
				limits.PerFileBytes = 1
			case "entry_limit":
				limits.EntryCount = 1
				headers = append([]*tar.Header{{Name: "src/", Mode: 0700, Typeflag: tar.TypeDir}}, headers...)
				bodies = append([]string{""}, bodies...)
			case "size_limit":
				limits.GitExpandedBytes = 1
			case "git_directory":
				file.Path, headers[0].Name = ".git/config", ".git/config"
			case "attachment_collision":
				file.Path, headers[0].Name = "attachments/supplied", "attachments/supplied"
			}
			root := t.TempDir()
			err := extractGitArchive(t.Context(), bytes.NewReader(gitTestArchive(t, headers, bodies)), root, []gitTreeEntry{file}, "sha1", strings.Repeat("a", 40), limits)
			if (err == nil) != (kind == "valid" || kind == "git_commit_header") {
				t.Fatalf("archive kind=%s err=%v", kind, err)
			}
			if kind == "valid" {
				got, err := os.ReadFile(filepath.Join(root, file.Path))
				if err != nil || string(got) != data {
					t.Fatal("commit bytes were not materialized")
				}
			}
		})
	}
}

func TestPreparationGitTreeRequiresFullRegularBlobInventory(t *testing.T) {
	for _, kind := range []string{"valid", "submodule", "symlink", "short_oid", "duplicate", "unknown_mode", "missing_size", "escape"} {
		t.Run(kind, func(t *testing.T) {
			line := "100644 blob " + gitTestBlob("content") + "       7\tfile.txt\x00"
			switch kind {
			case "submodule":
				line = strings.Replace(line, "100644 blob", "160000 commit", 1)
			case "symlink":
				line = strings.Replace(line, "100644", "120000", 1)
			case "short_oid":
				line = strings.Replace(line, gitTestBlob("content"), "abc123", 1)
			case "duplicate":
				line += line
			case "unknown_mode":
				line = strings.Replace(line, "100644", "100600", 1)
			case "missing_size":
				line = strings.Replace(line, "       7", "       -", 1)
			case "escape":
				line = strings.Replace(line, "file.txt", "../file.txt", 1)
			}
			files, err := parseGitTree([]byte(line), "sha1", preparationTestCapabilities().Limits)
			if (err == nil) != (kind == "valid") || kind == "valid" && (len(files) != 1 || files[0].SizeBytes != 7) {
				t.Fatalf("tree kind=%s count=%d err=%v", kind, len(files), err)
			}
		})
	}
}
