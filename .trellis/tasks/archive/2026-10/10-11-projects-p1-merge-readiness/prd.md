# Complete projects-p1 merge readiness and integration

## Goal

Integrate `codex/projects-p1` into `main` after correcting the three verified
readiness issues and passing current validation. The user requested execution
of the assessment recommendations, then explicitly approved Trellis task creation.

## Background

- Assessed source: `7ab8d038963dd920a611047997ac1904ee69a1eb`.
- Target: `origin/main` at `501f4b55f1675e59e46b86d094131d44bff1efd8`.
- The source contains the target and is 81 commits ahead; merge-tree is clean.
- The source includes projects, iterations, triage, desktop changes, administrator
  initialization, and archived verification material. Preserve that approved scope.

## Requirements

- R1: Resolve valid task references with alphanumeric workspace prefixes,
  including digit-first and entirely numeric prefixes, in iteration selection
  and backend identifier search. Preserve UUIDs, text search, positive task
  numbers, deduplication and the existing batch limit. Evidence:
  `packages/core/iterations/candidates.ts:62`,
  `server/internal/handler/issue.go:610`,
  `server/internal/handler/workspace.go:30`.
- R2: Administrator credential publication must remain recoverable if the
  process exits immediately after publication. Keep private permissions,
  symlink/hardlink rejection, no-overwrite semantics, durability ordering and
  existing administrator preservation. Evidence:
  `server/cmd/server/platform_admin_credentials.go:148` and
  `server/cmd/server/platform_admin_credentials_unix.go:39`.
- R3: Bring the 13 reported screenshots within the 300 KiB bitmap budget while
  retaining readable visual evidence and existing paths/references.
- R4: Verify the final candidate, open/update a PR into `main`, wait for green
  CI, merge and verify the resulting main commit. Include migration 567–570,
  feature defaults and planning-timezone implications in the PR description.

## Acceptance criteria

- AC1/R1: Single and batch numeric/mixed-prefix references resolve via exact
  lookup; ordinary text still uses search. Relevant TS and Go regressions pass.
- AC2/R2: A subprocess interrupted just after publication can retry with the
  same credentials; existing-file races never overwrite the winner. Tests
  continue to reject unsafe files, and supported-platform code compiles.
- AC3/R3: `node scripts/check-image-budget.mjs --base origin/main` passes without
  an exemption. Compressed evidence remains readable at its recorded dimensions.
- AC4/R4: Current lint, type checks, applicable tests/builds and CI pass. Any
  environment-limited validation is explicitly identified, never called passed.
- AC5/R4: The PR is merged, main includes the approved changes, and the working
  tree is clean. Preserve source branch history rather than squash 81 commits.
- AC6/R4: Input received before the editor's deferred `create` event survives
  autosave and opted-in unmount delivery exactly once, with the original edit
  baseline. Default cancellation behavior stays unchanged. Deterministic tests
  and the unchanged P1-L14/P1-P05 production browser cases verify the repair.

## Verification follow-up — 2026-10-12

Current `origin/main` at `9929aa9a3` already includes the assessed feature history.
The remaining integration concerns the corrective commits and the failures found
while completing R4; the original feature history must still be preserved.

P1-P05 reproduced a real draft-loss race in 4/10 unchanged browser executions.
Lifecycle evidence showed input `update` before deferred `create`, which marked
the unsent text as already emitted. The bounded R4 repair includes the shared
editor and its regression suite; it does not weaken the browser assertions or
redesign the feature. See `research/p1-editor-draft-recovery.md` and
`continuation.md` for evidence and the refreshed integration baseline.

## Out of scope

No new dependencies, feature redesign, unrelated cleanup, schema changes,
production deployment, release tag, or unrelated branch deletion. This is a
branch integration, not a new version publication.

## Decisions and approval

The user approved the concrete assessment recommendation with “请按照上述建议帮我处理”.
The subsequent task-creation choice confirmed the recordkeeping workflow.
There are no unresolved product or scope decisions; routine implementation and
verification choices remain with the agent under the workspace autonomy contract.
