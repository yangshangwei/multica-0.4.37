# R3/R4 client implementation evidence

Implemented on 2026-10-07. No commits, dependency changes, API changes, store migrations, or platform routing changes were made by this lane.

## Changes and ownership

- `packages/views/iterations/iteration-form.tsx`: baseline resource/revision and editable values share one state lifetime. Clean input adopts a newer snapshot; dirty input retains its original revision. Requests contain only changed, currently editable fields. Conflict reads use the existing protected detail query; the existing comparison component offers server adoption or explicit merging of changed fields, followed by a separate Save action. The command hook remains responsible for persisted request identity and recovery.
- Known successful edits lock inputs through an authoritative detail refresh. A failed refresh retains the input and offers read-only retry without replaying the committed write. Permission errors hide the form, including the case where imperative `fetchQuery` rejects as cancelled while the existing protected choices observer receives the access-denied error.
- `packages/views/iterations/iteration-page.tsx`: transient failures with retained successful data keep the editor mounted; definitive client errors/missing data still hide protected/deleted content. Independent review subsequently included 408/429 among transient errors; see final-review.md. Capability/settings guards require this same adjustment because command invalidation refreshes the whole workspace iteration prefix. Existing query access/session fences are reused.
- `packages/views/iterations/iteration-form.test.tsx`: canonical draft/revision, explicit resolution, changed-field, successful refresh, failed refresh, continuous input lock, exact recovery, status restriction, session-change, and access-denial regressions.
- `packages/views/iterations/iteration-navigation.test.tsx`: dirty editor retention during both detail-only and workspace-wide transient failure; definitive resource deletion remains hidden. Existing revocation/navigation coverage retained.
- `packages/views/projects/components/project-detail.tsx`: key the overview by workspace/project identity.
- `packages/views/projects/components/project-detail.test.tsx`: real Query cache and actual detail → overview → progress boundary, with A and B already cached. A's text is still waiting in the editor debounce when navigation occurs; unmount flush writes it to A, B restores only its own text, B's preview uses only B's payload, and returning to A clears B's preview and restores A's text. Existing narrow mocks remain for unrelated sidebar/platform surfaces; the editor double reproduces its mount-only/debounce/unmount-emission contract.
- `packages/views/locales/{en,zh-Hans}/projects.json`: conflict/refresh copy. English button uses “Keep my changes,” with an explicit review-then-save hint; Chinese uses “合并修改”. Comparison labels display fields and member names rather than request JSON/revisions/UUID-only coordinator values.

## Red evidence

Before product edits:

1. `pnpm --filter @multica/views exec vitest run iterations/iteration-form.test.tsx --maxWorkers 2`: 6 new tests failed, 4 existing tests passed. The dirty draft sent `expected_revision: 3` instead of 2; clean inputs stayed stale; conflict choices and post-success refresh were missing.
2. `pnpm --filter @multica/views exec vitest run projects/components/project-detail.test.tsx -t 'isolates cached' --maxWorkers 2`: failed because B still rendered A's textarea and text after a cached ancestor rerender.
3. `pnpm --filter @multica/views exec vitest run iterations/iteration-navigation.test.tsx -t 'transient detail|definitive resource' --maxWorkers 2`: transient-error test failed because the Name field was unmounted; deletion control passed.

During boundary verification, added regressions also exposed and repaired stale status restrictions after successful refresh, workspace-wide transient invalidation unmount, and imperative-read cancellation during revocation.

## Verification

- `pnpm --filter @multica/views exec vitest run iterations/iteration-form.test.tsx iterations/iteration-navigation.test.tsx projects/components/project-detail.test.tsx --maxWorkers 2`: 40/40 passed at the initial integrated checkpoint, including the cached A/B regression and existing project detail behavior.
- `pnpm --filter @multica/views exec vitest run iterations projects/components/project-detail.test.tsx projects/components/project-management.test.tsx projects/components/project-review-regressions.test.tsx projects/components/project-overview-lifecycle.test.tsx locales/parity.test.ts --maxWorkers 2`: 174/175 passed at the expanded checkpoint; its only failure was the new cancellation-on-revocation form case described above, subsequently fixed. All other canonical suites passed.
- After that fix and final locale changes: `pnpm --filter @multica/views exec vitest run iterations/iteration-form.test.tsx iterations/iteration-navigation.test.tsx locales/parity.test.ts --maxWorkers 2`: **97/97 passed**.
- After the final changed-field simplification: `pnpm --filter @multica/views exec vitest run iterations/iteration-form.test.tsx iterations/iteration-navigation.test.tsx --maxWorkers 2`: **33/33 passed**.
- `pnpm --filter @multica/core exec vitest run iterations/command.test.tsx iterations/access.test.ts projects/progress-draft-store.test.ts --maxWorkers 2`: **20/20 passed**.
- `pnpm --filter @multica/views typecheck`: passed after correcting the new test's unsupported `getByRole` option. Main owns the final root typecheck after the final localized edits.
- `pnpm --filter @multica/views lint`: passed with 27 existing warnings and no errors. Final changed-file ESLint passed with only the two pre-existing `project-detail.tsx` hook dependency warnings.
- `git diff --check -- packages/views`: passed.

## Remaining verification / limits

Main owns the running full-repository checks, independent review, spec updates and commits. No new native browser/desktop UI run was performed in this lane; route lifetime is exercised through the actual shared detail/overview/progress component boundary and real Query cache, with a focused editor test double. Native editor debounce semantics remain owned by existing editor tests. No known unresolved functional failure remains in this slice.
