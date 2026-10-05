# P1 frontend final bounded re-review

Verdict: **APPROVE**.

Scope: RR-01, RR-02, RR-03 from frontend-review-2.md and core handling of the new workspace scope404. Reviewed fix commit `97e3f9124` against the frozen current HEAD `61ba556e4` on 2026-10-05. The relevant access/session/progress/description source has no subsequent diff from the reviewed fix and no working-tree changes. No source or tests were modified, no commit was created; only this report was written.

## Findings closed

### RR-01 — CLOSED: session cleanup, sensitive recovery and late work

Inspected `packages/core/platform/session-cleanup.ts` and `packages/core/projects/access.ts`. Actual session teardown now calls resetProjectAccessSession before resetting registered drafts. It clears denied state, copy-only deleted text and old deletion/editor registrations. A module-owned monotonically increasing session generation participates in request epochs and is not reset with the store.

Requests captured before teardown reject with project_session_changed and do not revoke a new session. Mounted progress and description editors capture their generation; an old unmount flush cannot write previous-account text into the new session. Old cleanup callbacks cannot remove the new session's pending deletion or editor registration. Existing auth expiry/logout and Desktop server-reset call sites use this shared teardown.

Independently reran tests using the actual cleanup helper, real QueryClient and Zustand: new account requests reach the network; copy-only text and denial state clear; old requests and optimistic mutations cannot restore or roll back the new account's protected cache; old editor cleanup/flush ownership stays isolated.

### RR-02 — CLOSED: preview survives pending editor flush

Inspected ProjectProgress's patch and preview transition. The patch now reads the current durable draft and compares the proposed content before resetting preview. An identical pending editor flush is an acknowledgement and leaves the successful preview mounted. A real content change still invalidates preview and its consent.

The updated editor substitute models delayed emission and unmount flush. Independently reran both the immediate-success case (before the300ms body debounce) and the delayed-success case (after emission): Publish remains present and the body remains reviewable. Earlier cancellation/navigation preservation and revocation/session teardown no-repersist cases also pass.

### RR-03 — CLOSED: explicit server adoption survives stale controlled props

Inspected ProjectDescription's adoptedServer description/revision baseline, conflict action and controlled editor input. The selected authorized server version remains the controlled value until canonical props reach its revision. The action checks current access/session, cancels the older detail read, updates an existing detail cache's adopted description fields and explicitly removes the rejected draft.

The regression's editor substitute now follows the real clean controlled-value synchronization and updates its clean baseline during adoptContent. Independently reran the stale-props/failed-GET case: selectingv2 immediately retainsv2, removes the rejected durable draft and makes the next edit send thev2 description revision. The original explicit-discard/remount regression also remains passing.

### Workspace scope404 — CLOSED

Inspected the middleware's actual `{code:"workspace_access_denied"}`404 producer and the core API/access path. Only that scope404 is treated as access loss. The protected request wrapper retains its denial code after sanitizing the error, allowing shared access guards to erase protected content. Project-not-found retains its separate copy-only deletion path; other resource404, operation/source403 and health503 are not incorrectly upgraded to workspace revocation.

Independently reran the real ApiClient response-error parsing regression plus the classification matrix. The real fetch/ApiError boundary preserves workspace_access_denied and clears the protected cache.

## Independent verification

Working directory for every command: `/Volumes/artisan/code/2026/multica-projects-p1`.

- `pnpm -C packages/core exec vitest run projects/access-lifecycle.test.ts projects/mutations.test.tsx platform/session-cleanup.test.ts` — **3 files /22 tests passed**, exit0.
- `pnpm -C packages/views exec vitest run projects/components/project-review-regressions.test.tsx` — **1 file /16 tests passed**, exit0.
- Read the faithful editor test implementation and verified its controlled-value/adopt/unmount behavior against the real ContentEditor paths used by the two previous reproductions.
- Confirmed no relevant source diff after97e3f9124 and no dirty relevant product files. No additional types/lint/full-suite claim is made by this review; those are separately recorded by the implementation/parent gates.

There are **zero remaining must-fix findings in this requested scope**. RR-01–RR-03 are closed. Earlier C pagination approval and measured performance are not re-audited or re-claimed here. This frontend approval does not replace the parent-owned final production API/Web/Electron build and eight E2E scenarios. No new dependency, unrelated refactor or product simplification was introduced by this read-only review.
