package execenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	markers, err := stagedConflictMarkerPaths(repo)
	if err != nil {
		t.Fatal(err)
	}
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

	markers, err := stagedConflictMarkerPaths(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 0 {
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

	markers, err := stagedConflictMarkerPaths(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 0 {
		t.Fatalf("whitespace errors must not be reported as conflict markers, got %v", markers)
	}
}

func TestStagedConflictMarkerPathsChecksOnlyStagedRegularText(t *testing.T) {
	auditIsolateGit(t)
	repo := newTestRepo(t)
	name := "conflicted file [one].txt"
	if runtime.GOOS != "windows" {
		name = "conflicted:file\n[one].txt"
	}
	body := "<<<<<<< HEAD\nagent side\n=======\nuser side\n>>>>>>> user change\n"
	writeFile(t, filepath.Join(repo, name), body)
	writeFile(t, filepath.Join(repo, "binary.bin"), "binary\x00header\n"+body)
	gitRun(t, repo, "--literal-pathspecs", "add", "--", name, "binary.bin")

	// Populate non-regular index entries directly, without requiring OS symlink
	// privileges or cloning a submodule. Their marker-like payload is not source.
	blob := gitRun(t, repo, "rev-parse", ":"+name)
	gitRun(t, repo, "update-index", "--add", "--cacheinfo", "120000,"+blob+",marker-link")
	head := gitRun(t, repo, "rev-parse", "HEAD")
	gitRun(t, repo, "update-index", "--add", "--cacheinfo", "160000,"+head+",submodule")
	gitRun(t, repo, "rm", "keep.txt")
	// Inspection must use the staged blob, even if the file on disk has changed.
	writeFile(t, filepath.Join(repo, name), "unstaged resolution\n")

	markers, err := stagedConflictMarkerPaths(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 1 || markers[0] != name {
		t.Fatalf("only the staged regular text file should be reported, got %q", markers)
	}
}

func TestStagedConflictMarkerPathsUsesConfiguredWidth(t *testing.T) {
	for _, tt := range []struct {
		name  string
		width string
		body  string
		want  bool
	}{
		{"short_markers", "3", "<<< ours\nleft\n===\nright\n>>> theirs\n", true},
		{"other_width_is_prose", "9", "<<< ours\nleft\n===\nright\n>>> theirs\n", false},
		{"wide_markers", "9", "<<<<<<<<< ours\nleft\n=========\nright\n>>>>>>>>> theirs\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auditIsolateGit(t)
			repo := newTestRepo(t)
			writeFile(t, filepath.Join(repo, ".gitattributes"), "conflicted.txt conflict-marker-size="+tt.width+"\n")
			writeFile(t, filepath.Join(repo, "conflicted.txt"), tt.body)
			gitRun(t, repo, "add", ".gitattributes", "conflicted.txt")
			markers, err := stagedConflictMarkerPaths(repo)
			if err != nil {
				t.Fatal(err)
			}
			if (len(markers) > 0) != tt.want {
				t.Fatalf("conflict-marker-size=%s: got %q, want conflict=%v", tt.width, markers, tt.want)
			}
		})
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

func TestFinalizeCommitsResolutionOverPreviouslyStagedMarkers(t *testing.T) {
	auditIsolateGit(t)
	repo := newTestRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "user A\n")
	first := auditPrepareTurn(t, repo, turnOneTask)
	writeFile(t, filepath.Join(first.Path, "tracked.txt"), "agent B\n")
	finalizeOK(t, first)

	writeFile(t, filepath.Join(repo, "tracked.txt"), "user C\n")
	second := auditPrepareTurn(t, repo, turnTwoTask)
	if len(second.ReplayConflicts) == 0 {
		t.Fatal("fixture: expected a replay conflict")
	}
	gitRun(t, second.Path, "add", "tracked.txt")
	// Finalize owns the final staging step, including edits after the agent's
	// last git add. The old staged markers are no longer the delivered content.
	writeFile(t, filepath.Join(second.Path, "tracked.txt"), "resolved B and C\n")
	outcome := finalizeOK(t, second)
	if got := gitRun(t, repo, "show", outcome.Branch+":tracked.txt"); got != "resolved B and C" {
		t.Errorf("delivered content = %q, want the working-tree resolution", got)
	}
	if got := readFile(t, filepath.Join(repo, "tracked.txt")); got != "user C\n" {
		t.Errorf("the user's working tree changed: %q", got)
	}
}

func TestFinalizeAllowsNonConflictMarkerText(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"setext_heading", "Heading\n=======\nA valid Markdown heading.\n"},
		{"diagnostic_text", "Fix leftover conflict markers in another file.  \n"},
		{"short_prose", "< example\n=\n> example\n"},
		{"indented_example", "  <<<<<<< ours\nleft\n=======\nright\n>>>>>>> theirs\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auditIsolateGit(t)
			repo := newTestRepo(t)
			wt := auditPrepareTurn(t, repo, turnOneTask)
			writeFile(t, filepath.Join(wt.Path, "README.md"), tt.body)
			gitRun(t, wt.Path, "add", "README.md")
			outcome := finalizeOK(t, wt)
			got, err := runGitStdout(repo, "show", outcome.Branch+":README.md")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.body {
				t.Errorf("delivered Markdown = %q, want %q", got, tt.body)
			}
		})
	}
}

func TestFinalizeChecksMarkersIntroducedAfterStaging(t *testing.T) {
	auditIsolateGit(t)
	repo := newTestRepo(t)
	wt := auditPrepareTurn(t, repo, turnOneTask)
	path := filepath.Join(wt.Path, "tracked.txt")
	writeFile(t, path, "clean staged content\n")
	gitRun(t, wt.Path, "add", "tracked.txt")
	body := "<<<<<<< HEAD\nagent side\n=======\nuser side\n>>>>>>> user change\n"
	writeFile(t, path, body)

	outcome, err := wt.Finalize(worktreeTestLogger())
	if err == nil {
		t.Fatal("Finalize delivered markers introduced after the agent's last git add")
	}
	if outcome.Branch != "" || outcome.AutoCommitted || outcome.PreservedPath != wt.Path {
		t.Errorf("unresolved work must remain recoverable, got %+v", outcome)
	}
	if got := readFile(t, path); got != body {
		t.Errorf("preserved conflict changed: %q", got)
	}
	if got := gitRun(t, repo, "rev-parse", wt.Branch); got != wt.BaseCommit {
		t.Errorf("branch advanced despite the conflict: %s", got)
	}
}

func TestFinalizeKeepsWorktreeWhenStagedInspectionFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Git fault-injection shim requires a POSIX shell")
	}
	auditIsolateGit(t)
	repo := newTestRepo(t)
	wt := auditPrepareTurn(t, repo, turnOneTask)
	writeFile(t, filepath.Join(wt.Path, "agent-output.txt"), "irreplaceable work\n")
	gitRun(t, wt.Path, "add", "agent-output.txt")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	shim := `#!/bin/sh
if [ "$3" = "diff" ] && [ "$4" = "--cached" ]; then
  echo "fixture: staged inspection unavailable" >&2
  exit 128
fi
exec "$MULTICA_TEST_REAL_GIT" "$@"
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTICA_TEST_REAL_GIT", realGit)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	outcome, err := wt.Finalize(worktreeTestLogger())
	if err == nil || !strings.Contains(err.Error(), "staged inspection unavailable") {
		t.Fatalf("inspection failure must be explicit, got outcome=%+v err=%v", outcome, err)
	}
	if outcome.Branch != "" || outcome.AutoCommitted || outcome.PreservedPath != wt.Path {
		t.Errorf("uninspected work must remain recoverable, got %+v", outcome)
	}
	if got := readFile(t, filepath.Join(wt.Path, "agent-output.txt")); got != "irreplaceable work\n" {
		t.Errorf("inspection failure lost the agent's work: %q", got)
	}
}

func TestFinalizeAllowsUnchangedCommittedConflictExamples(t *testing.T) {
	for _, name := range []string{"unrelated_edit", "mode_change", "rename"} {
		t.Run(name, func(t *testing.T) {
			auditIsolateGit(t)
			repo := newTestRepo(t)
			body := "Example\n<<<<<<< ours\nleft\n=======\nright\n>>>>>>> theirs\n"
			writeFile(t, filepath.Join(repo, "example.txt"), body)
			gitRun(t, repo, "add", "example.txt")
			gitRun(t, repo, "commit", "-m", "add a conflict example fixture")
			wt := auditPrepareTurn(t, repo, turnOneTask)
			fileName := "example.txt"
			switch name {
			case "mode_change":
				// Keep the staged mode even on filesystems without executable bits.
				gitRun(t, repo, "config", "core.filemode", "false")
				gitRun(t, wt.Path, "update-index", "--chmod=+x", "example.txt")
			case "rename":
				fileName = "renamed.txt"
				gitRun(t, wt.Path, "mv", "example.txt", fileName)
			default:
				body = strings.Replace(body, "Example", "Documented example", 1)
				writeFile(t, filepath.Join(wt.Path, "example.txt"), body)
			}

			outcome := finalizeOK(t, wt)
			if got := gitRun(t, repo, "show", outcome.Branch+":"+fileName); got != strings.TrimSpace(body) {
				t.Errorf("the committed example changed: %q", got)
			}
		})
	}
}

func TestFinalizeAllowsTextAfterCommittedConflictExampleAtEOF(t *testing.T) {
	for _, tt := range []struct {
		name   string
		suffix string
	}{
		{"newline", "\n"},
		{"crlf", "\r\n"},
		{"paragraph", "\nA paragraph below the unchanged example.\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auditIsolateGit(t)
			repo := newTestRepo(t)
			body := "Example\n<<<<<<< ours\nleft\n=======\nright\n>>>>>>> theirs"
			writeFile(t, filepath.Join(repo, "example.txt"), body)
			gitRun(t, repo, "add", "example.txt")
			gitRun(t, repo, "commit", "-m", "add a conflict example without a final newline")
			wt := auditPrepareTurn(t, repo, turnOneTask)
			writeFile(t, filepath.Join(wt.Path, "example.txt"), body+tt.suffix)

			outcome := finalizeOK(t, wt)
			got, err := runGitStdout(repo, "show", outcome.Branch+":example.txt")
			if err != nil {
				t.Fatal(err)
			}
			if got != body+tt.suffix {
				t.Errorf("delivered content = %q, want %q", got, body+tt.suffix)
			}
		})
	}
}

func TestFinalizeRefusesNewGroupsBesideCommittedConflictExamples(t *testing.T) {
	for _, extra := range []string{
		"<<<<<<< agent\nnew left\n=======\nnew right\n>>>>>>> local\n",
		"<<<<<<< ours\nleft\n=======\nright\n>>>>>>> theirs\n",
	} {
		t.Run(strings.Fields(extra)[1], func(t *testing.T) {
			auditIsolateGit(t)
			repo := newTestRepo(t)
			body := "<<<<<<< ours\nleft\n=======\nright\n>>>>>>> theirs\n"
			writeFile(t, filepath.Join(repo, "example.txt"), body)
			gitRun(t, repo, "add", "example.txt")
			gitRun(t, repo, "commit", "-m", "add a conflict example fixture")
			wt := auditPrepareTurn(t, repo, turnOneTask)
			writeFile(t, filepath.Join(wt.Path, "example.txt"), body+"\n"+extra)

			outcome, err := wt.Finalize(worktreeTestLogger())
			if err == nil || outcome.Branch != "" || outcome.AutoCommitted || outcome.PreservedPath != wt.Path {
				t.Fatalf("an existing example must not exempt new groups: outcome=%+v err=%v", outcome, err)
			}
			if got := readFile(t, filepath.Join(wt.Path, "example.txt")); got != body+"\n"+extra {
				t.Errorf("the new conflict was not preserved: %q", got)
			}
		})
	}
}
