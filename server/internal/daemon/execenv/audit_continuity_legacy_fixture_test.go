package execenv

import (
	"fmt"
	"log/slog"
	"strings"
)

// Audit-only legacy code copied mechanically from 41281e4326e506ea40638706577cbc94cec61a42
// server/internal/daemon/execenv/local_worktree.go. Only type/function names
// are changed to coexist with main; git execution helpers are unchanged.

type auditLegacyLocalWorktree struct {
	// GitRoot is the user's repository root — the repo that owns the branch.
	GitRoot string
	// Path is the worktree root inside the env root.
	Path string
	// WorkDir is the agent's cwd: Path, plus the offset of LocalPath inside
	// the repo when the user pointed the resource at a subdirectory.
	WorkDir string
	// Branch is the branch created for this task, in the user's repo.
	Branch string
	// BaseCommit is the commit the worktree started from. Finalize compares
	// the branch tip against it to decide whether the task produced anything.
	BaseCommit string
	// DirtyBaseCaptured records that the user had uncommitted tracked edits
	// which were replayed into the worktree.
	DirtyBaseCaptured bool
	// aborted, when set, makes Finalize refuse to commit or remove anything.
	// Set by the daemon when a pre-commit step failed in a way that would make
	// the committed branch wrong (see AbortWithReason).
	aborted error
	// UntrackedCopied / UntrackedSkipped report the untracked-file replay.
	// A non-zero skip count means the bounds below were hit and the agent is
	// looking at less than the user has on disk; it is logged at warn level so
	// the gap is findable rather than invisible.
	UntrackedCopied  int
	UntrackedSkipped int
}

func (w *auditLegacyLocalWorktree) Finalize(logger *slog.Logger) (LocalWorktreeOutcome, error) {
	if w == nil {
		return LocalWorktreeOutcome{}, nil
	}
	outcome := LocalWorktreeOutcome{Branch: w.Branch}

	unlock, err := lockGitRoot(w.GitRoot, logger)
	if err != nil {
		// Nothing has been committed or removed yet, so the agent's work is
		// still sitting in the worktree. Report it as preserved rather than
		// naming a branch that does not carry it.
		outcome.Branch = ""
		outcome.PreservedPath = w.Path
		return outcome, fmt.Errorf("could not lock %q to finalize branch %s: %w; "+
			"the work is preserved in the worktree at %s", w.GitRoot, w.Branch, err, w.Path)
	}
	defer unlock()

	// Something before the commit went wrong in a way that would make the
	// delivered branch misleading. Commit nothing and keep the worktree: the
	// agent's work is still in it, and so is whatever the caller could not
	// clean up, which a human can now look at directly.
	if w.aborted != nil {
		// Report NO branch. One exists in the user's repo, but nothing was
		// committed to it, so naming it as this task's result would point them
		// at a branch that is missing the very work they are looking for. The
		// preserved worktree path below is the honest pointer.
		outcome.Branch = ""
		outcome.PreservedPath = w.Path
		if logger != nil {
			logger.Error("execenv: worktree finalize aborted; nothing committed, worktree kept for inspection",
				"path", w.Path, "branch", w.Branch, "git_root", w.GitRoot, "error", w.aborted)
		}
		return outcome, fmt.Errorf(
			"refusing to deliver branch %s: %w; the task worktree is preserved at %s (listed by `git worktree list` in %s)",
			w.Branch, w.aborted, w.Path, w.GitRoot)
	}

	// Treat "can't tell" like "dirty": committing costs an empty commit at
	// worst, while assuming clean risks deleting the agent's edits.
	dirty, statusErr := worktreeIsDirty(w.Path)
	if statusErr != nil {
		if logger != nil {
			logger.Warn("execenv: inspect worktree status failed; committing defensively",
				"path", w.Path, "error", statusErr)
		}
		dirty = true
	}
	if dirty {
		committed, err := w.commitAll(logger)
		if err != nil {
			outcome.PreservedPath = w.Path
			if logger != nil {
				logger.Error("execenv: could not commit the agent's changes; keeping the worktree so the work is recoverable",
					"path", w.Path, "branch", w.Branch, "git_root", w.GitRoot, "error", err)
			}
			return outcome, fmt.Errorf(
				"could not commit the agent's changes to branch %s: %w; the work is preserved in the worktree at %s (listed by `git worktree list` in %s) — recover it before that directory is reclaimed",
				w.Branch, err, w.Path, w.GitRoot)
		}
		outcome.AutoCommitted = committed
	}

	// A branch still sitting exactly on its base commit means the task changed
	// nothing — the read-only case. Delete it so the user's branch list only
	// ever grows for tasks that actually produced work.
	tip, err := runGitTrimmed(w.Path, "rev-parse", "--verify", "HEAD")
	producedWork := err != nil || tip != w.BaseCommit

	if removeErr := removeLocalWorktreeDir(w.GitRoot, w.Path, logger); removeErr != nil {
		outcome.PreservedPath = w.Path
		return outcome, fmt.Errorf(
			"could not remove finalized worktree for branch %s: %w; the task worktree remains at %s",
			w.Branch, removeErr, w.Path)
	}

	if !producedWork {
		deleteBranch(w.GitRoot, w.Branch, logger)
		outcome.Branch = ""
	}

	if logger != nil {
		logger.Info("execenv: local worktree finalized",
			"git_root", w.GitRoot,
			"branch", outcome.Branch,
			"auto_committed", outcome.AutoCommitted,
			"produced_work", producedWork,
		)
	}
	return outcome, nil
}

func (w *auditLegacyLocalWorktree) commitAll(logger *slog.Logger) (bool, error) {
	return auditLegacyCommitEverything(w.Path, "chore(agent): uncommitted changes from task")
}

func auditLegacyCommitEverything(worktreePath, message string) (bool, error) {
	if out, err := runGit(worktreePath, "add", "-A"); err != nil {
		return false, fmt.Errorf("git add: %s: %w", strings.TrimSpace(out), err)
	}
	// --no-verify: the user's commit hooks are written for the user's own
	// workflow (interactive linters, test suites, signing prompts) and a hook
	// failure here would mean losing the agent's work to save a lint run. Note
	// it does NOT disable commit.gpgSign, which is why the caller has to treat
	// a commit failure as "keep the worktree" rather than a warning.
	args := append(commitIdentityArgs(worktreePath), "commit", "--no-verify", "-m", message)
	if out, err := runGit(worktreePath, args...); err != nil {
		if strings.Contains(out, "nothing to commit") {
			return false, nil
		}
		return false, fmt.Errorf("git commit: %s: %w", strings.TrimSpace(out), err)
	}
	return true, nil
}
