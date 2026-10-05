# Frontend review 2: completed remediation

2026-10-05. Scope: RR-01–RR-03 in the independent frontend-review-2.md, the real workspace middleware404, and the G2/G3 evidence gaps. C pagination and all earlier FR semantics are unchanged. The parent separately owns the TriageFilters timing regression and final full-suite/browser verification.

## RR-01: authenticated session boundaries

Before implementation, the new `projects/access-lifecycle.test.ts` produced **3 failures / 5 passes**: the real workspace denial classification failed, actual session cleanup left denied/copy-only state intact, and a previous-session request resolved after cleanup. Two further cleanup ownership regressions were also observed RED: an old deletion cleanup removed a new session's pending deletion, and an old editor cleanup removed its successor's flusher.

`clearClientSessionData` now invokes project access teardown before resetting durable drafts. Teardown increments a module-owned monotonic generation, clears denied/epoch/copy-only state and registered old editor/deletion pointers, and scrubs protected project mutation state. The generation is never reset to zero. Captured old requests reject with a session-change error that cannot revoke the newly authenticated session. Old mounted editors carry their generation so their unmount flush cannot restore previous-account text. Cleanup callbacks only release their own generation/registration.

The same production cleanup function is used by logout/session expiry and Desktop runtime/server configuration reset (`apps/desktop/src/renderer/src/App.tsx`, AppContent reset and pre-bootstrap reset paths). Tests invoke this actual helper with real Zustand and QueryClient; they do not claim a separate browser logout/server-switch run.

GREEN evidence: **11 access lifecycle cases**, the existing session-cleanup suite, an old optimistic mutation settling after a new account's cache was populated, and both progress/description pending-unmount cases. New account reads reach the network; old account late responses remain rejected and cannot restore copy-only text or cached descriptions.

## Real middleware404

`isProjectAccessLost` recognizes only404 carrying `code=workspace_access_denied` as a scope denial. A real ApiClient fetch-error response test proves that this code survives the request boundary and clears protected content. Other resource404, operation/source403 and DB503 remain non-revocation failures. `project_not_found` retains the distinct copy-only deletion behavior. The parent owns the server middleware and mobile changes.

## RR-02: preview transition and pending editor flush

The debounce-faithful editor regression failed before the fix: fast Preview success briefly exposed Publish but the pending unmount flush reopened the editor. The draft patch now compares against the current durable intent and only invalidates preview on an actual change. A flush of the exact already-previewed body is an acknowledgement, so review stays mounted. Fast (<300ms) and deferred response cases pass, alongside the prior cancel/navigation-within-debounce and revocation-no-repersist cases.

## RR-03: authorized server version versus stale controlled props

The enhanced editor fake follows the real controlled-value clean guard and updates that clean baseline when adoptContent is called. Before the fix, selecting serverv2 while props remainedv1 renderedv1 again. The component now retains the authorized adopted description/revision until canonical props catch up; it also cancels the older detail read and updates an existing detail cache with the adopted text. The action checks its current session/access before changing state. The rejected local draft is still explicitly removed.

GREEN regression keeps stalev1 props after the action, verifies immediatev2 text and no durable rejected draft, then verifies the next edit sends description_revisionv2. Existing explicit-discard/remount and concurrent autosave cases remain passing.

## G2 / G3 acceptance evidence

`project-overview-lifecycle.test.tsx` adds **3 real-component / real-QueryClient cases** without changing product behavior:

- G2: fake planning clock crosses UTC midnight; the mounted60-second timer causes a second real Query request and new displayed reference date. Advancing the clock without ticking the timer and firing window focus causes a third request and updated date.
- G3: all-completed and all-cancelled scopes each display100% closure while both null acceptance summaries still display “No acceptance recorded”; no Passed conclusion is synthesized.

## Final executed checks

- Core targeted projects/API/session/auth-initializer/core-provider/draft/realtime suite: **16 files / 188 tests passed**.
- Shared project/review/day-boundary/pagination/inbox suite: **9 files / 113 tests passed**.
- `pnpm --filter @multica/core typecheck` and `lint`: passed.
- `pnpm --filter @multica/views typecheck`: passed.
- Scoped ESLint for the changed description/progress components and new/modified tests: passed.
- `git diff --check`: passed.

Remaining parent gates: complete frontend re-review, final production Web/Desktop rebuild and real browser evidence. This document does not substitute earlier browser results for the current commit.
