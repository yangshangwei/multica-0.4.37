package execenv

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Continuity across turns, for the three ways a conversation's work could be
// stranded or corrupted.
//
// The first test drives current code and was a real defect: a preserved conflict
// kept the branch checked out, so the next turn forked an alt name, recorded
// nothing, and every later turn re-forked from the same frozen tip. Finalize now
// detaches a preserved worktree, and this test is the pin.
//
// The other two drive auditLegacyLocalWorktree — a mechanical copy of the parent
// as it existed at 41281e4326e5, kept because parent and helper are the same
// program but not necessarily the same build (see decodePreparationRequest: every
// upgrade path replaces the helper binary under a live daemon, and until it
// re-execs an older parent talks to a newer helper, with no version field on the
// wire to detect it). That older parent drops a branch on `!producedWork` with no
// createdBranch guard, and commits with a bare `git add -A` and no unmerged check.
// Current code has both guards, so these two assert the contrast: the hazard is
// real for a deployed old daemon, and current source does not share it.
func auditPrepareTurn(t *testing.T, repo, taskID string) *LocalWorktree {
	t.Helper()
	wt, err := PrepareLocalWorktree(LocalWorktreeParams{
		LocalPath: repo, EnvRoot: t.TempDir(), AgentName: "J", TaskID: taskID,
		ConversationKey: "MUL-6881", WorkspaceID: testBranchOwner.WorkspaceID,
		AgentID: testBranchOwner.AgentID, ConversationID: testBranchOwner.ConversationID,
	}, worktreeTestLogger())
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	return wt
}

func auditIsolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
}

func TestAuditContinuity_PreservedConflictRetryShouldBeInherited(t *testing.T) {
	auditIsolateGit(t)
	repo := newTestRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "user A\n")
	first := auditPrepareTurn(t, repo, turnOneTask)
	writeFile(t, filepath.Join(first.WorkDir, "tracked.txt"), "agent B\n")
	finalizeOK(t, first)

	writeFile(t, filepath.Join(repo, "tracked.txt"), "user C\n")
	failed := auditPrepareTurn(t, repo, turnTwoTask)
	if len(failed.ReplayConflicts) == 0 {
		t.Fatal("fixture: expected conflict")
	}
	outcome, err := failed.Finalize(worktreeTestLogger())
	if err == nil || outcome.PreservedPath != failed.Path {
		t.Fatalf("fixture: unresolved conflict was not preserved: outcome=%+v err=%v", outcome, err)
	}
	if _, err := os.Stat(failed.Path); err != nil {
		t.Fatalf("fixture: preserved worktree is missing: %v", err)
	}

	// Keep the failed worktree on disk, as the real finalizer explicitly does.
	retry := auditPrepareTurn(t, repo, turnThreeTask)
	writeFile(t, filepath.Join(retry.WorkDir, "tracked.txt"), "resolved B and C\n")
	writeFile(t, filepath.Join(retry.WorkDir, "retry-success.txt"), "delivered by successful retry\n")
	gitRun(t, retry.Path, "add", "tracked.txt", "retry-success.txt")
	retryOutcome := finalizeOK(t, retry)
	t.Logf("failed branch=%s retained=%s; successful retry branch=%s tracksState=%v", failed.Branch, failed.Path, retryOutcome.Branch, retry.tracksState)
	if got := gitRun(t, repo, "show", retryOutcome.Branch+":retry-success.txt"); got != "delivered by successful retry" {
		t.Fatalf("fixture: retry did not deliver: %q", got)
	}

	next := auditPrepareTurn(t, repo, "11112222-3333-4444-5555-dddddddddddd")
	t.Logf("next branch=%s replay_conflicts=%v tracksState=%v content=%q", next.Branch, next.ReplayConflicts, next.tracksState, readFile(t, filepath.Join(next.WorkDir, "tracked.txt")))
	if _, err := os.Stat(filepath.Join(next.WorkDir, "retry-success.txt")); err != nil {
		t.Errorf("successful retry result not inherited by next turn: branch %s omits retry-success.txt from %s (artifact absent=%v)", next.Branch, retryOutcome.Branch, os.IsNotExist(err))
	}
	if got := readFile(t, filepath.Join(next.WorkDir, "tracked.txt")); got != "resolved B and C\n" {
		t.Errorf("successful retry resolution not inherited: got %q", got)
	}
	if got := readFile(t, filepath.Join(repo, "tracked.txt")); got != "user C\n" {
		t.Errorf("user working directory changed: %q", got)
	}
}

type auditLegacyPreparationResponse struct {
	Environment *struct {
		RootDir       string
		WorkDir       string
		LocalWorktree *auditLegacyLocalWorktree
	} `json:"environment"`
	Error string `json:"error"`
}

// This invokes the real current helper in a subprocess and decodes its output
// with the pre-35e71ce4f parent's wire type and mechanically copied Finalize.
func auditPrepareWithLegacyParent(t *testing.T, repo, taskID string) *auditLegacyLocalWorktree {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// Every key below existed in the old parent's request. In particular, the
	// LocalWorktree object carries no newer ownership or finalizer-contract key.
	payload, err := json.Marshal(map[string]any{
		"action": preparationActionPrepare,
		"prepare": map[string]any{
			"WorkspacesRoot": t.TempDir(), "WorkspaceID": testBranchOwner.WorkspaceID,
			"TaskID": taskID, "IssueIdentifier": "MUL-6881", "AgentName": "J", "Provider": "claude",
			"Task":          map[string]any{"IssueID": testBranchOwner.ConversationID, "AgentID": testBranchOwner.AgentID},
			"LocalWorktree": map[string]any{"LocalPath": repo},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	command := preparationHelperTestCommand()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	response, err := cmd.Output()
	if err != nil {
		t.Fatalf("real current helper: %v: %s", err, stderr.String())
	}
	var decoded auditLegacyPreparationResponse
	if err := json.Unmarshal(response, &decoded); err != nil {
		t.Fatalf("legacy parent decode: %v", err)
	}
	if decoded.Error != "" || decoded.Environment == nil || decoded.Environment.LocalWorktree == nil {
		t.Fatalf("legacy parent response: %+v", decoded)
	}
	var currentWire preparationResponse
	if err := json.Unmarshal(response, &currentWire); err != nil {
		t.Fatalf("inspect current helper wire: %v", err)
	}
	env := decoded.Environment
	// The parent cleans sidecars before finalization in production.
	if err := CleanupRuntimeConfig(env.WorkDir, "claude"); err != nil {
		t.Fatal(err)
	}
	if err := CleanupSidecars(env.RootDir); err != nil {
		t.Fatal(err)
	}
	t.Logf("current helper wire: continued=%v tracksState=%v replayConflicts=%v; legacy parent accepted branch=%s base=%s",
		currentWire.Environment.LocalWorktree.Continued, currentWire.Environment.LocalWorktree.tracksState, currentWire.Environment.LocalWorktree.ReplayConflicts,
		env.LocalWorktree.Branch, env.LocalWorktree.BaseCommit)
	return env.LocalWorktree
}

func TestAuditContinuity_LegacyParentMustNotDeleteContinuedBranch(t *testing.T) {
	auditIsolateGit(t)
	repo := newTestRepo(t)
	first := auditPrepareTurn(t, repo, turnOneTask)
	writeFile(t, filepath.Join(first.WorkDir, "prior-work.txt"), "all prior turns\n")
	finalizeOK(t, first)
	before := gitRun(t, repo, "rev-parse", "refs/heads/"+first.Branch)

	legacy := auditPrepareWithLegacyParent(t, repo, turnTwoTask)
	if legacy.Branch != first.Branch || legacy.BaseCommit != before {
		t.Fatalf("fixture: helper did not continue branch: %+v", legacy)
	}
	outcome, err := legacy.Finalize(worktreeTestLogger())
	if err != nil {
		t.Fatalf("legacy finalizer: %v", err)
	}
	t.Logf("legacy finalizer returned success: outcome=%+v; prior delivery=%s", outcome, before)

	// The hazard: that older parent had no createdBranch guard, so a read-only
	// turn on a CONTINUED branch deleted it, taking every earlier turn's work.
	_, legacyErr := gitTry(t, repo, "rev-parse", "--verify", "refs/heads/"+first.Branch)
	legacyDroppedBranch := legacyErr != nil
	if !legacyDroppedBranch {
		t.Skip("legacy parent retained the branch here; nothing to contrast against")
	}
	t.Logf("legacy parent dropped continued branch %s (the upgrade hazard)", first.Branch)

	// Current code must not: `dropped := !producedWork && w.createdBranch`, and a
	// continued branch was not created by this prepare. Same scenario, current
	// Finalize, branch survives.
	gitRun(t, repo, "branch", first.Branch, before)
	third := auditPrepareTurn(t, repo, "mul-6881-turn-three")
	if third.Branch != first.Branch {
		t.Fatalf("fixture: current prepare did not continue the branch, got %s", third.Branch)
	}
	if _, err := third.Finalize(worktreeTestLogger()); err != nil {
		t.Fatalf("current finalize on a read-only continued turn: %v", err)
	}
	if _, err := gitTry(t, repo, "rev-parse", "--verify", "refs/heads/"+first.Branch); err != nil {
		t.Errorf("current Finalize deleted continued branch %s on a read-only turn; the createdBranch guard regressed", first.Branch)
	}
}

func TestAuditContinuity_LegacyParentMustNotCommitConflictMarkers(t *testing.T) {
	auditIsolateGit(t)
	repo := newTestRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "user A\n")
	first := auditPrepareTurn(t, repo, turnOneTask)
	writeFile(t, filepath.Join(first.WorkDir, "tracked.txt"), "agent B\n")
	finalizeOK(t, first)

	writeFile(t, filepath.Join(repo, "tracked.txt"), "user C\n")
	legacy := auditPrepareWithLegacyParent(t, repo, turnTwoTask)
	unmerged, err := unmergedPaths(legacy.Path)
	if err != nil || len(unmerged) == 0 {
		t.Fatalf("fixture: expected unresolved index: %v, %v", unmerged, err)
	}
	outcome, err := legacy.Finalize(worktreeTestLogger())
	if err != nil {
		t.Logf("legacy finalizer rejected unresolved conflict: %v", err)
		return
	}
	committed := gitRun(t, repo, "show", outcome.Branch+":tracked.txt")
	t.Logf("legacy finalizer returned success: outcome=%+v committed content=%q", outcome, committed)
	if !strings.Contains(committed, "<<<<<<<") || !strings.Contains(committed, ">>>>>>>") {
		t.Skip("legacy parent did not commit markers here; nothing to contrast against")
	}
	t.Logf("legacy parent committed conflict markers to %s (the upgrade hazard)", outcome.Branch)

	// Current code must refuse. Same conflicted worktree shape, current Finalize:
	// unmergedPaths catches the unstaged case, stagedConflictMarkerPaths catches
	// the `git add`-ed one, and neither may deliver.
	writeFile(t, filepath.Join(repo, "tracked.txt"), "user D\n")
	current := auditPrepareTurn(t, repo, "mul-6881-turn-three")
	if unmerged, _ := unmergedPaths(current.Path); len(unmerged) == 0 {
		t.Fatalf("fixture: expected the replay to conflict for the current-code leg")
	}
	// Stage it, which is what defeated the index-only check.
	gitRun(t, current.Path, "add", "tracked.txt")
	currentOutcome, currentErr := current.Finalize(worktreeTestLogger())
	if currentErr == nil {
		t.Error("current Finalize delivered a staged conflict; the marker check regressed")
	}
	if currentOutcome.Branch != "" {
		t.Errorf("current Finalize named branch %q as delivered despite a conflict", currentOutcome.Branch)
	}
	if currentOutcome.PreservedPath == "" {
		t.Error("current Finalize must preserve the worktree so the conflict is recoverable")
	}
}
