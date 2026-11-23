# Final scope review of the nine-commit integration candidate

Reviewed: 2026-11-08, during full-check-05.

Candidate: `7038918a7793d722f47124a19b976a446b3c7f3e` in `/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108`; baseline: `e541cfb436fed9c6733174479718b651e2cf030d`.

**Code-scope verdict: no new confirmed issue in the existing nine-commit, 64-path delta. Landing remains blocked by the failed daemon watchdog test and the incomplete full gate.** The separate watchdog repair was outside this review and needs its own review plus a new complete fixed-SHA gate. This report does not grant acceptance by combining earlier partial runs.

## Findings (fixed)

None. This dispatch was read-only for product code, task metadata, source worktrees, indexes, refs and services. Only this report was written. No tests, builds, browser or database environments were started.

## Findings (not fixed)

### Completion evidence still identifies an earlier candidate

`research/source-disposition-summary.md` still names `18e43dc93` as its fixed candidate and correctly says full verification is pending. The current candidate has five subsequent verification repairs, and full-check-05 has now exposed another failure. Before closure, update that summary and the task's acceptance/progress/tracker records to the actual passing candidate, preserving the earlier outcomes. Do not mark the complete gate or local landing complete from this static review.

This is a required closure action already assigned to the main session, not a request to redesign or repeat the completed source audit. The pending watchdog repair can change the final commit/path counts.

### Final candidate review is still required for the watchdog increment

The current failure is `TestExecuteAndDrain_IdleWatchdog_DoesNotFireDuringInFlightToolCall` in `server/internal/daemon`. The approved nine commits do not modify `daemon.go` or `daemon_test.go`; the repair underway in the diagnostic worktree introduces a new production lifecycle boundary. Review that delta independently, including real timeout behavior and final transcript preservation, before incorporating it into a newly frozen full-gate candidate. This report deliberately does not read or approve partially written scratch files.

## Review coverage

The source audit and preservation were already completed and were not repeated. Reviewed the task PRD/design/implementation plan, final source-disposition records, current diff and previous independent reports. Consulted the server, UI, views, Web and Desktop spec indexes plus the relevant capability, context-experiment, input-style, collaboration-detail, database-isolation and E2E environment contracts. Several generic frontend index entries remain placeholders; authoritative rules come from CLAUDE.md and the active domain contracts.

| Commits | Distinct responsibility | Existing independent evidence and final review |
| --- | --- | --- |
| `296543527`, `285dfdcab` | Input boundary/fill split; existing project text-role tokens; restricted-resource placeholders; collaboration batch refusal and success-only feedback | `integration-independent-review.md`, `recipient-ui-integration.md`, `visual-verification.json`; inspected final behavior diff and synchronized contracts |
| `7647d6eb8`, `18e43dc93` | Context canary stops remaining trials; retained Knowledge continuation accounting and canonical namespace protection | Original integration review and its explicit follow-up approval; `guard-tests-integration.md`, `context-fixed-sha-result.json`; final continuation implementation/control hashes match the approved hashes |
| `c52eeb53f` | Instrumented runtime lookup bound to the exact authority snapshot | `runtime-lookup-independent-review.md`; all eight approved hashes match current candidate files |
| `2b4803f01`, `7038918a7` | Scope lifecycle Refresh to the Overview section; register denial waiters before clicks | Final L09/L16 review and earlier L14/environment diagnosis; final lifecycle file matches approved SHA256 |
| `00e99892f` | Build and serve the allocated Desktop renderer; bind complete-check API/WS/source paths; owned cleanup | `check-environment-independent-review.md` follow-up; all five final approved hashes match. Earlier mismatching hashes in that report belong to its preserved pre-fix review, not current drift |
| `d4af1d6d3` | Observe actual successful recovery when a Refresh request is replaced | Final logical-refresh section of `navigation-recovery-independent-review.md`; current file matches its approved SHA256, including the superseding repair after the failed finite-set attempt |

The first four commits account for 49 distinct paths; the later five add 15 distinct paths, for 64 total. No changed dependency manifest, database migration, ordinary Agent provider implementation or unrelated source-worktree overlay enters this candidate.

### Behavior and boundary findings

- Shared UI and views remain inside their existing package boundaries. The input token split preserves muted fills while strengthening boundaries; project headings use the pre-existing semantic scale. Existing localized reaction labels were retained.
- Restricted images/files render inert React text/SVG placeholders without resource URLs or resolver requests. Batch action disabling keeps the existing handler guard and allows clearing the selection. The success callback is reached only through the existing success receipt/status predicate; cancellation continues to use dismissal separately.
- Capability callers use the existing RuntimeLookup wrapper with the handler snapshot or service claim transaction. The final diff does not change downstream identity, membership, permission or error handling predicates. The bound function avoids importing service into auth.
- Context/Knowledge additions remain test harnesses with their explicit real-provider opt-in. The continuation checks canonical output namespace overlap, existing inode aliases and launch containment while preserving original registration strings and attempt accounting. This integration is not authority to execute a paid provider.
- Check infrastructure reuses registry port allocation, existing process ownership and existing electron-vite/Vite installation. It builds Desktop before serving its renderer/preload, validates served index bytes, and keeps production Web builds. Stale source renderer-port rebinding and source-env preservation have a targeted regression. No new skip or real-provider switch was added.
- Both E2E repair files preserve the real user actions and substantive assertions. Navigation keeps the held request barrier, actual 200/completed response, error disappearance and exact scroll positions; it no longer assumes a canceled request must supply that response. Lifecycle keeps actual revocation, the 20-second request budget and renderer responsiveness assertions. Scoping uses the Overview heading, not ordinal or forced clicks.

## Selective archive/commit guidance

Read-only snapshot of this task directory found 97 files, 3,551,978 bytes, all text: 41 Markdown, 32 JSON, 12 JSONL, nine Python, one MJS, one patch and one log. The largest files are decision ledgers (about 1.06 MB and 0.74 MB), not media/build artifacts. Concurrent task additions may increase these counts.

A bounded scan found no credential-bearing PostgreSQL/Redis URL, long Bearer value, JWT, common API token/private-key pattern, or quoted literal secret assignment in these task files. This is a scoped pattern scan, not a guarantee that arbitrary sensitive prose could never exist. No secret values were printed. External verification `.env` files were not opened and must remain outside Git.

Recommended explicit inclusion in the task-record commit:

- Task `prd.md`, `design.md`, `implement.md`, `progress.md`, `task.json`, context manifests and handoffs, with current completion statements updated only after their evidence exists.
- Curated research Markdown: source decisions, implementation explanations, diagnoses, independent reviews and preservation summaries, including historical failed-review sections.
- JSON decision ledgers, provenance/preservation manifests, visual/result summaries and verification tracker. The large Runtime and remaining-test JSON ledgers are useful item-level evidence and can be deliberately included despite size; do not replace them with unsupported aggregate claims.
- Small `*-meta.json`, source-hash/preflight summaries and final before/after/root-preservation reports, where their contents are credential-free. A new independent session journal file can preserve this task's record without staging the already-dirty shared journal/index.

Keep the following raw or superseded material external or untracked after task archival unless there is a deliberate need to version it:

- `research/recipient-safe-source-candidates.patch`: raw source snapshot, not an additional implementation to apply or a necessary task narrative.
- `research/guard-tests-integration/*.jsonl` and the task's raw `.log` file: retained execution output; committed summaries can link to a hash-bound external copy.
- `research/takeover-review-20261108/merge_runtime.py`, `merge_runtime2.py`, and `write_runtime_ledger.py`: superseded takeover scripts, not reusable approved operations. Retain their historical review and manifests.
- `research/handoff-tools/` and `research/handoff-tools-1544/`: machine-specific operational copies. Preserve externally; if retained in Git, explicitly label their historical candidate/time assumptions and avoid implying that they are general safe commands for future task state.
- Any new full check traces, test-results directories, screenshots, copied environment files or database dumps: keep in the task's external verification archive with hashes and concise committed pointers.

Do not delete excluded material. Before moving the task, preserve a complete task-directory snapshot if needed and verify its manifest; after archive, stage an explicit allowlist. Do not use blanket staging of the root `.trellis/workspace/` or the whole dirty repository. `task.py archive --no-commit` and a selective Lore commit avoid accidentally capturing the original journal/index edits. This recommendation concerns durable-record packaging only; it does not relax preservation requirements.

## Landing conditions retained

The existing `landing-gate-review.md` and `land-verified.py` cover the required safety boundary. Main must still collect the new complete exit0, full-log failure scan, source identity, API/Web/renderer provenance and cleanup, reconcile only the exact known Next generated import with its executable mode preserved, verify a clean unchanged integration candidate, and prove path disjointness before fast-forwarding without autostash. Then compare the original tracked patches, all 740 unrelated untracked contents/modes and staged state, plus the landed product bytes/modes. Preserve all source branches/worktrees and backup refs. A later docs-only commit must be linked to the passing product SHA and must not change product trees.

## Verification

- Static scope/diff and prior exact-hash approvals: reviewed as above.
- `git diff --check e541cfb43..7038918a7`: PASS, no output.
- Lint/typecheck: not rerun in this read-only dispatch. Existing full-check-05 evidence reports the static stage passed; that stage is not a full-gate pass.
- Tests: not rerun by this reviewer; the main session owns the already-running complete check. Full-check-05 has a confirmed daemon failure, and no passing complete run is claimed.
- Product code fixes: none in this report. Watchdog repair and final post-pass evidence remain pending.
