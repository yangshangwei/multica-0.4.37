# Branch/worktree audit — 2026-11-08

## Repository state

- Current branch: `codex/cross-workspace-collaboration`
- Current HEAD: `e541cfb436fed9c6733174479718b651e2cf030d`
- Current branch vs `origin/codex/cross-workspace-collaboration`: `0 0`
- Current root dirty entries: 315 (53 tracked, 262 untracked)
- `HEAD...origin/main`: `558 0` (current branch is 558 commits ahead)
- `HEAD...upstream/main`: `1291 5809` (long-running divergent history; excluded)

## Worktrees

- Registered worktrees: 86
- Dirty worktrees: 53
- Worktrees with committed tip commits outside current HEAD: 10 entries
- One registration is `prunable` because its gitdir points to a missing location.

## Local/origin branches not directly merged

The following branch tips have commits outside current HEAD; their commit counts are
`git rev-list --count HEAD..<branch>`:

| Branch | Count |
| --- | ---: |
| `agent/agent/task1` | 1 |
| `codex/acceptance-evidence-822c15785-f417` | 2 |
| `codex/collab-recipient-picker-integration-20261030` | 6 |
| `codex/collaboration-context-experiments` | 1 |
| `codex/collaboration-context-resume-20261031` | 5 |
| `codex/collaboration-final-owned-20261103` | 1 |
| `codex/collaboration-fixture-repair-20261103` | 1 |
| `codex/knowledge-advance-notes-20261102` | 1 |
| `codex/knowledge-codex-20261104` | 3 |
| `codex/projects-p1-merge-readiness` | 2 |
| `codex/runtime-acceptance-20261104` | 17 |
| `codex/verify-collaboration-host-runtime-20261026` | 1 |

The 41 branch references contain 40 unique commit objects. `git log --cherry-mark`
marks most recipient-picker, knowledge and runtime-acceptance implementation commits
as patch-equivalent to commits already present in current HEAD.

## High-risk dirty source worktrees

Counts below are source files whose current working-tree content differs from the
current HEAD, not merely files changed relative to that worktree's own old HEAD.

| Worktree | Status entries | Source files differing from current HEAD |
| --- | ---: | ---: |
| `mc-knowledge-ui-qa-20261101` (detached) | 145 | 55 |
| `mc-runtime-recording-recovery-20261101` | 94 | 48 |
| `mc-recipient-integration-20261030-072838-a3fe` | 49 | 39 |
| `mc-handoff-cursor-20261101` | 67 | 37 |
| `mc-handoff-launch-20261031` | 28 | 16 |
| `mc-core-e2e-1a32ca36c-20261104` (detached) | 16 | 10 |

The handoff/recording worktrees overlap heavily (23–60 paths between pairs), so they
must be reconciled as one batch. Knowledge UI overlaps the handoff batch on 10 paths;
recipient integration overlaps recording on 4 paths; core E2E overlaps Knowledge on 1 path.

## Decision

Use a clean integration worktree from current HEAD, archive all source worktrees before
mutation, skip patch-equivalent implementation commits, selectively apply non-equivalent
tests/CI/spec changes, then merge verified logical commits back while preserving the
root worktree's existing dirty state.
