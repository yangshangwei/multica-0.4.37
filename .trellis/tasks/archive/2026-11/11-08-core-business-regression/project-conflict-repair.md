# P1-P10 acceptance description conflict repair

Product fix: `packages/views/projects/components/project-progress.tsx`; canonical regression coverage: `project-management.test.tsx`. No core mutation/API/server behavior changed. Browser environment was not rebuilt or exercised by this lane; root owns production Web/Desktop regression.

## Failure and cause

The full browser run returned `409 project_description_conflict`, retained the draft, but never displayed the latest-description comparison/adoption control. The failure screenshot shows the sidebar already has the new description. API logs show the conflict at 23:22:15.085, overlapping project reads at 15.121 and 15.230, and brief cancelled overview/resources requests followed by successful replacements.

`handleConflict` awaited one imperative `fetchQuery(...).catch(() => undefined)` and copied its one-time result into local state. Real-time project changes can invalidate the same active query more than once (project-management and generic project refresh responders). The installed Query version piggybacks one silent cancellation onto its replacement promise, so a single cancellation probe passed. A second replacement cancels that piggybacked promise: the catch returns undefined while the Query cache eventually receives the newest project. No later callback repopulates the local comparison state.

The canonical component regression uses the real QueryClient and active project observer, a conflict response, two controlled abortable detail reads, then a successful replacement. Before the fix it reproduced the missing comparison while preserving draft/cache state. It does not use timing thresholds, mock Query internals or weaken browser assertions.

## Repair

The composer observes the existing protected project-detail query while a description-conflict response needs review. Its comparison comes from current Query data instead of a copied local server object. Same/absent revisions are not offered; a newer cached revision cannot be adopted while the query is fetching. Read failures expose Retry without repeating the preview mutation. The existing access guard handles protected-read denial and clears drafts. Adoption still explicitly changes the expected description revision before a new preview/publication.

## Verification

- Original single-replacement probe: passed; retained as diagnostic evidence, not a failure claim.
- Original repeated-replacement regression: **FAIL** at missing latest-description comparison.
- All project views: **23 files / 206 tests PASS**, including four new regressions for repeated replacement, read-error retry, cached-description pending guard, and access denial.
- Changed-file lint and views typecheck: PASS. A test-only unsupported Testing Library `exact` option was caught by the first typecheck and removed; the second typecheck passed without product changes.
- Full views suite: **599 files / 7291 tests PASS**, 0 failures, 0 skipped, exit0. Raw `views-after.log` and JSON retain all names/results.

Private evidence: `/Users/artisan/.multica/qa/core-20261108/project-conflict/` contains original request order, diagnostic red/green logs, full project JSON, patch, lint/typecheck logs. Root should rebind source identity and rebuild both clients before final browser verification.
