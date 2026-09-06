package execenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The index-based unmerged check is cleared by staging, so an agent that runs
// `git add` over a conflicted file — the command the replay instructions hand it —
// used to deliver conflict markers on the branch. These pin the content check that
// closes it.

func TestStagedConflictMarkerPathsFindsStagedMarkers(t *testing.T) {
	repo := newTestRepo(t)
	path := filepath.Join(repo, "conflicted.txt")
	body := "line one\n<<<<<<< HEAD\nagent side\n=======\nuser side\n>>>>>>> abc1234 (their change)\nline last\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "conflicted.txt")

	markers := stagedConflictMarkerPaths(repo)
	if len(markers) != 1 || markers[0] != "conflicted.txt" {
		t.Fatalf("expected conflicted.txt to be reported, got %v", markers)
	}
}

func TestStagedConflictMarkerPathsIgnoresCleanStagedContent(t *testing.T) {
	repo := newTestRepo(t)
	path := filepath.Join(repo, "clean.txt")
	if err := os.WriteFile(path, []byte("a resolved file\nwith two lines\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "clean.txt")

	if markers := stagedConflictMarkerPaths(repo); len(markers) != 0 {
		t.Fatalf("clean content must not be reported, got %v", markers)
	}
}

// `git diff --cached --check` exits non-zero for whitespace errors too. Refusing
// to deliver a branch over a trailing space would be its own defect, so only the
// marker diagnostic may trip this.
func TestStagedConflictMarkerPathsIgnoresWhitespaceErrors(t *testing.T) {
	repo := newTestRepo(t)
	path := filepath.Join(repo, "whitespace.txt")
	if err := os.WriteFile(path, []byte("trailing space here \nand a tab\tthere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "whitespace.txt")

	if markers := stagedConflictMarkerPaths(repo); len(markers) != 0 {
		t.Fatalf("whitespace errors must not be reported as conflict markers, got %v", markers)
	}
}

// The end-to-end shape: a real conflict, "resolved" by staging it as-is, must not
// reach the delivered branch.
func TestFinalizeRefusesStagedConflictMarkers(t *testing.T) {
	repo := newTestRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "user A\n")
	gitRun(t, repo, "add", "tracked.txt")
	gitRun(t, repo, "-c", "user.email=t@e.st", "-c", "user.name=t", "commit", "-m", "base")

	first := auditPrepareTurn(t, repo, turnOneTask)
	writeFile(t, filepath.Join(first.WorkDir, "tracked.txt"), "agent B\n")
	finalizeOK(t, first)

	// A divergent local edit makes the next turn's replay conflict.
	writeFile(t, filepath.Join(repo, "tracked.txt"), "user C\n")
	second := auditPrepareTurn(t, repo, turnTwoTask)

	unmerged, err := unmergedPaths(second.Path)
	if err != nil || len(unmerged) == 0 {
		t.Fatalf("fixture: expected an unresolved index, got %v (%v)", unmerged, err)
	}

	// The agent "resolves" by staging the conflicted file untouched — exactly what
	// `git add <file>` does, and what its instructions tell it marks a file done.
	gitRun(t, second.Path, "add", "tracked.txt")
	if stillUnmerged, _ := unmergedPaths(second.Path); len(stillUnmerged) != 0 {
		t.Fatalf("fixture: staging should have cleared the unmerged entry, got %v", stillUnmerged)
	}

	outcome, err := second.Finalize(worktreeTestLogger())
	if err == nil {
		t.Fatal("Finalize must refuse a staged conflict rather than deliver markers")
	}
	if outcome.Branch != "" {
		t.Errorf("no branch may be reported as delivered, got %q", outcome.Branch)
	}
	if outcome.PreservedPath == "" {
		t.Error("the worktree must be preserved so the work is recoverable")
	}
	if !strings.Contains(err.Error(), "conflict markers") {
		t.Errorf("the error should name the cause, got: %v", err)
	}
	// And nothing with markers may be on the branch.
	if out, showErr := runGit(repo, "show", second.Branch+":tracked.txt"); showErr == nil {
		if strings.Contains(out, "<<<<<<<") {
			t.Errorf("branch %s carries conflict markers: %q", second.Branch, out)
		}
	}
}
