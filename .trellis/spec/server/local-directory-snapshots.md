# Local directory snapshot safety

A private Git index is a performance optimization, not merely a bag of blob IDs. Git uses the index file timestamp to decide when same-size, same-timestamp working-tree entries may be racy-clean and need their content rechecked.

- Read seed index bytes and its mtime from the same opened file descriptor. Git can atomically replace the index while capture runs.
- Preserve that mtime on the private copy. A fresh copy timestamp can incorrectly make stale cached file stats look trustworthy and omit a user's rapid edit.
- On copy/close/timestamp failure, discard the private index and rebuild from HEAD before staging. An empty index can lose tracked files that match ignore rules.
- Cleanup or rebuilding failures stop capture; do not continue with a potentially unsafe seed.
- Never acquire the real index lock or change the user's index bytes/mtime, refs or working files while taking a snapshot. Keep the existing EnvRoot lock and Finalize conflict guards.
- Keep this behavior local to snapshot seeding; generic file-copy helpers should not silently acquire new timestamp semantics.

Canonical regressions live in `server/internal/daemon/execenv/snapshot_index_stat_test.go`, `local_worktree_staged_conflict_test.go`, and `gitroot_lock_test.go`. The timestamp regression controls file/index mtimes with real Git and no sleep, and verifies both captured content and source-state preservation. Include race checks for worktree/snapshot changes.
