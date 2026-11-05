# Completed-work merge implementation plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Integrate only independently completed, verified changes that are missing from `codex/cross-workspace-collaboration`.

**Architecture:** Preserve all other sessions' worktrees, indexes, stashes, unfinished code and frozen experiment evidence. Inspect immutable candidate snapshots in a separate integration worktree, whitelist each completed change, validate against current target code, then integrate only that reviewed patch. Commit identity alone is not evidence of missing code; archive status alone is not evidence of completed development.

**Tech Stack:** Git worktrees/patch comparison, the repository's pnpm/Vitest/TypeScript tools, Go fake-provider tests where applicable.

## Authority and boundaries

The user requested a merge plan and execution, then explicitly narrowed scope: do not merge other sessions' unfinished work; merge only changes confirmed complete and forgotten. The earlier choice to create no Trellis task persists. This document is an execution record, not a new Trellis task.

Target snapshot: `39167cdca78f0c1a3d0d6ce6e557e2dd9930546c`. Integration branch: `codex/merge-unintegrated-20261105`. Execution worktree: `/Volumes/artisan/code/2026/mc-merge-unintegrated-20261105`.

Never apply the entire staged 45-file snapshot, pop a stash, reset another checkout, update an active task's completion state, or consume real model calls for this merge. No remote push, production release or database operation is part of this task.

## Admission rule

A change enters the merge whitelist only when all are true:

1. A bounded implementation is explicitly finished, not merely archived or staged.
2. Verification/review evidence binds that implementation; known blockers are resolved.
3. Its effective change is absent from current HEAD and is not an intentionally discarded predecessor.
4. It is independent of unfinished work in another session and preserves current fixes/contracts.
5. Current scoped checks and independent review pass before integration.

If no candidate meets these conditions, record an empty merge whitelist rather than fabricate integration by promoting unfinished work.

## Candidate disposition

| Candidate | Evidence required | Initial disposition |
|---|---|---|
| 45 staged UI/test/docs paths in recipient integration worktree | Completed change boundaries and source-bound review/acceptance | EXCLUDE: no completed scope with verified source binding established; staging/preservation snapshots are insufficient, and some changes remove existing accessible names |
| `docs/multica-introduction.html` and its archived task | Finished browser/content/visual review | EXCLUDE: archive note explicitly says archived as-is, verification unfinished and no commit requested |
| `PRODUCT.md` | Its own completed scope/verification | EXCLUDE: no independent completed/verified deliverable established |
| E5 canary stop guard and its three Go test files | Historical successor hashes, fake tests and independent review; independence from live unfinished experiments | EXCLUDE: locally verified stop guard was intentionally confined to frozen experiment overlays; active real-series work remains separately owned, with no forgotten-merge declaration |
| execution-panel handoff mock | Finished, reviewed test repair and current failing-case evidence | EXCLUDE: superseded by current independent /handoff request routing; old null mock removes exercised behavior |
| Recipient E2E `4aa77e08e` environment fallback | Finished independent acceptance plus consistency with current explicit environment gate | ADMIT: original isolated production acceptance 1/1 passed; scripts/check.sh sets NEXT_PUBLIC_API_URL, TestApiClient uses it, and existing loopback/database/same-origin guards remain |
| Old acceptance/planning/research snapshots | Finished deliverable, no current task-state regression | EXCLUDE whole snapshots: acceptance NOT PASSED, planning/in_progress records or superseded materials |
| Knowledge/Host/runtime/dialog-wait committed fixes | Patch/tree comparison | ALREADY INTEGRATED; do not replay |

## Task 1: Establish an evidence-backed whitelist

- Preserve source diffs and file bytes outside the repository; record source HEAD/path and hashes.
- Inspect task completion notes and actual verification records, not only `status=completed`.
- Independently review the UI/test candidates and committed E2E candidates in parallel, with no source edits.
- Write the final disposition and exact admitted files below before implementation.

## Task 2: Integrate admitted changes in isolation

- Run each candidate's existing focused baseline checks.
- Apply only the completed patch, adapting conflict context without importing unrelated old code.
- For a behavioral fix, retain the original meaningful regression test and verify its failure on baseline where safe; do not add implementation-mirroring tests.
- For test-only changes, run the actual relevant suite with fake executables, private outputs and no real-provider account use.
- Record exact commands, counts, failures and resolved causes.

Admitted-source checks:

- Run `node <artifact-dir>/e2e-admission-controls.cjs baseline` **before applying the patch**, then `node <artifact-dir>/e2e-admission-controls.cjs candidate` after the patch. The runner reads the current file in both modes, so a later baseline invocation is not a historical replay. Preserve the original baseline result and hash. The recorded baseline demonstrates the otherwise-valid fallback case skips; the restored patch enters the fixture, preserving eight other guard outcomes.
- `pnpm exec tsc --noEmit --skipLibCheck --strict --target ES2022 --module ESNext --moduleResolution bundler --esModuleInterop --types node e2e/collaboration-recipient-picker-web.spec.ts`.
- Run ESLint with the existing shared base rules through a temporary configuration that includes this root E2E file; an ignored-file warning is not a pass.
- `pnpm exec playwright test e2e/collaboration-recipient-picker-web.spec.ts --list`; discovery must retain the single current expanded recipient journey.
- `git diff --check` and independent review must confirm the only source hunk is the original environment fallback.

No Go or UI checks are needed after those candidates were excluded. The browser journey's prior successful evidence is retained separately from this fresh admission verification.

## Task 3: Independent review and local integration

- Review requirement compliance first: whitelist only, no unfinished session work, no superseded regressions.
- Review implementation quality and test evidence; fix only admitted-scope issues.
- Create a scoped Lore-format commit. Record exact source provenance and excluded work in the message/plan.
- Re-read target HEAD, current staged/unstaged paths and admitted source bytes immediately before integration.
- If target advances, incorporate the new target into the isolated branch and rerun checks affected by actual overlap. Never overwrite a concurrent file based on an old snapshot.
- Merge/cherry-pick only the verified commit into the current target while preserving unrelated dirty work. If Git would touch another session's dirty overlapping files, hold that candidate and finish other independent admitted changes.
- Confirm the applied commit is reachable, exact intended content is present, and unrelated user files are unchanged by this operation.

## Results

Whitelist: exactly one source file, `e2e/collaboration-recipient-picker-web.spec.ts`, restoring the completed `4aa77e08e` API-address fallback hunk. Preserve all newer selectors, assertions and explicit locality/database/origin gates. No UI, application runtime, experimental harness, task metadata or unrelated current working files are admitted.

Readiness review finished: all 45 staged frontend/test/docs files are excluded for absent completion proof (the introduction archive explicitly says incomplete); the verified E5 guard belongs to intentionally isolated experiment overlays; the old null handoff mock is superseded by URL-specific response routing. The completed branch audit found no other eligible product-source omission. Reports: `readiness-audit.md` and `completion-candidates.md` under the execution artifact directory.

Source completion evidence: original `4aa77e08e` commit body and `d6a70ed29` integration record report isolated production Web/API recipient acceptance 1/1; full-suite unrelated failures remain disclosed. Current dependency evidence: `scripts/check.sh:100` sets `NEXT_PUBLIC_API_URL`, `e2e/fixtures.ts:13` consumes it; no dedicated-variable opt-in contract was found.

Verification plan: execute the actual spec's admission code in a temporary instrumented VM, intercepting `TestApiClient` construction before any fixture/network activity. Compare baseline and restored patch for dedicated URL, valid fallback, missing configuration, remote URLs and differing origins. Then run TypeScript syntax diagnostics, lint that actually covers the root E2E path, Playwright discovery, and independent spec/quality review. This proves environment admission and preserved guards, not a new full browser acceptance run.

The isolated candidate now contains only the admitted one-line fallback plus this plan. Fresh verification results:

| Check | Result |
|---|---|
| Baseline actual-spec admission | 8/9 expected outcomes; valid fallback incorrectly skipped, demonstrating the omission |
| Candidate actual-spec admission | 9/9 pass; valid fallback reaches fixture construction, all eight other guards preserved |
| TypeScript syntax diagnostics | No errors |
| Explicit strict TypeScript check of spec/imports | Exit 0 |
| Scoped ESLint with shared base rules and confirmed file coverage | Exit 0, 0 errors, 0 warnings |
| Playwright discovery | Exit 0, the single current expanded recipient journey retained |

No fixture, database, API, browser or Provider was launched by the admission harness. No fresh production-browser pass is claimed; prior 1/1 browser evidence remains source-specific. Independent scope and quality reviews both passed with no blockers. Review artifacts are `scope-review.md` and `quality-review.md` in the execution artifact directory. Local integration is authorized for this exact candidate after a final clean-index/path-overlap check; Git history and the captured pre/post integration record identify the applied commit.
