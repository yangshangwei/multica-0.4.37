# Integration design

The prior read-only diagnosis is accepted. Use branch fix/upstream-041-043-selective in /Volumes/artisan/code/2026/multica-upstream-fixes-041-043, based on 8e123db4801a9867a1f81e68cc87410e040aa1b6. The original main worktree is read-only throughout.

Port each requested patch atomically and preserve its tests. Git history is snapshot-based; do not merge upstream branches. Use source SHA trailers and the Lore commit protocol. These nine changes do not require schema or package-lock changes. Preserve existing modes and local code around each hunk.

Use the repository's existing pnpm store and frozen lockfile, and dedicated per-worktree output folders. A PostgreSQL container already listens on localhost:5432; allocate two new uniquely named databases after proving endpoint identity. Never reset or run tests against the user's normal database. Launch only this worktree's API/web processes with explicit environment and record ownership. Use an isolated Redis test instance only where needed.

Business boundaries: transient task lookup failures must not change cancellation semantics for truly missing tasks; failed Codex reuse must enter the existing fresh-prepare path while successful reuse remains unchanged; preview truncation must not affect stored/full tool or comment content; classification must not introduce retries; hidden runtime lookup must not bypass server-side authorization; keyboard changes must preserve unrelated text/table shortcuts; Inbox compact navigation must remain usable.

The integration owner performs git writes and service lifecycle actions. Independent native agents may audit coverage, add explicitly owned regression tests, or review the result, using this worktree only. Broad test runners are resource bounded on a 16 GB machine. Test data/outputs are isolated; source tests and durable verification notes are tracked.
