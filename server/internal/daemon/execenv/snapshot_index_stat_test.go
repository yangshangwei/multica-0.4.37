package execenv

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCaptureUserSnapshotRechecksRacyCleanIndex(t *testing.T) {
	auditIsolateGit(t)
	repo := newTestRepo(t)
	// Match the real same-size/same-timestamp window without sleeping. Ignoring
	// ctime is a supported Git setting and makes the cached stat data stable
	// when this fixture restores the working file's mtime after editing it.
	gitRun(t, repo, "config", "core.trustctime", "false")
	file := filepath.Join(repo, "tracked.txt")
	index := filepath.Join(repo, ".git", "index")
	stamp := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(file, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "tracked.txt")
	if err := os.Chtimes(index, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	// "original\n" and "modified\n" have the same byte length. Git must
	// re-read this file because its cached mtime equals the index timestamp.
	writeFile(t, file, "modified\n")
	if err := os.Chtimes(file, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	indexBefore, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	refsBefore := gitRun(t, repo, "show-ref", "--head")
	head := gitRun(t, repo, "rev-parse", "HEAD")
	if got := gitRun(t, repo, "--no-optional-locks", "diff", "--name-only", "--", "tracked.txt"); got != "tracked.txt" {
		t.Fatalf("fixture: the original index must detect the edit as racy-clean, got %q", got)
	}

	snapshot, err := captureUserSnapshot(repo, t.TempDir(), head, worktreeTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if got := gitRun(t, repo, "show", snapshot+":tracked.txt"); got != "modified" {
		t.Errorf("snapshot reused stale index content %q, want the current working-tree content", got)
	}
	indexAfter, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(indexBefore, indexAfter) {
		t.Error("snapshot capture changed the user's real index bytes")
	}
	if info, err := os.Stat(index); err != nil || !info.ModTime().Equal(stamp) {
		t.Errorf("snapshot capture changed the user's index timestamp: info=%v err=%v", info, err)
	}
	if refsAfter := gitRun(t, repo, "show-ref", "--head"); refsAfter != refsBefore {
		t.Errorf("snapshot capture changed the user's refs: before=%q after=%q", refsBefore, refsAfter)
	}
	if got := readFile(t, file); got != "modified\n" {
		t.Errorf("snapshot capture changed the user's working file: %q", got)
	}
}

func TestCaptureUserSnapshotRebuildsUnavailableIndexFromHEAD(t *testing.T) {
	for _, name := range []string{"missing index", "index is a directory"} {
		t.Run(name, func(t *testing.T) {
			auditIsolateGit(t)
			repo := newTestRepo(t)
			// A cold rebuild must retain tracked files even when an ignore rule
			// would exclude them from a brand-new empty index.
			writeFile(t, filepath.Join(repo, ".gitignore"), "tracked.txt\n")
			gitRun(t, repo, "add", ".gitignore")
			gitRun(t, repo, "commit", "-m", "ignore rule for tracked fixture")
			index := filepath.Join(repo, ".git", "index")
			savedIndex := filepath.Join(repo, ".git", "saved-index")
			indexBefore, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(index, savedIndex); err != nil {
				t.Fatal(err)
			}
			if name == "index is a directory" {
				if err := os.Mkdir(index, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			writeFile(t, filepath.Join(repo, "tracked.txt"), "modified\n")
			writeFile(t, filepath.Join(repo, "new.txt"), "new user file\n")
			refsBefore := gitRun(t, repo, "show-ref", "--head")
			head := gitRun(t, repo, "rev-parse", "HEAD")

			snapshot, err := captureUserSnapshot(repo, t.TempDir(), head, worktreeTestLogger())
			if err != nil {
				t.Fatal(err)
			}
			if got := gitRun(t, repo, "show", snapshot+":tracked.txt"); got != "modified" {
				t.Errorf("rebuilt snapshot lost the tracked file's edit: %q", got)
			}
			if got := gitRun(t, repo, "show", snapshot+":new.txt"); got != "new user file" {
				t.Errorf("rebuilt snapshot lost the untracked file: %q", got)
			}
			if refsAfter := gitRun(t, repo, "show-ref", "--head"); refsAfter != refsBefore {
				t.Error("rebuilding the private index changed the user's refs")
			}
			if saved, err := os.ReadFile(savedIndex); err != nil || !bytes.Equal(saved, indexBefore) {
				t.Errorf("rebuilding the private index changed the saved user index: %v", err)
			}
			info, err := os.Stat(index)
			if name == "missing index" {
				if !os.IsNotExist(err) {
					t.Errorf("capture created the user's missing index: info=%v err=%v", info, err)
				}
			} else if err != nil || !info.IsDir() {
				t.Errorf("capture replaced the user's unusable index: info=%v err=%v", info, err)
			}
		})
	}
}
