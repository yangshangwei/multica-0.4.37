# Independent Trellis review

Reviewed the complete task change set in Core, UI, Views and Web against the PRD, audit F01-F11, core accessibility and settings recovery contracts. Existing unrelated working-tree changes were excluded. No commit was made.

## Findings fixed

1. **Permission loss retained an archive confirmation.** `squad-detail-page.tsx` now gates the pending archive dialog and clears both archive/add-member intent when management authority is lost. A fresh regression failed before the fix and proves authority recovery does not reopen the stale confirmation.
2. **Denied squad directory candidates could reappear after a failed retry.** `squadListOptions` now uses the existing401/403 exact-query access boundary. Detail/member404 handling remains separate; a catalog404 retains its previous compatibility behavior. Two failing tests proved401/403 stale data before the fix; the catalog404 case remains green.
3. **Project squad manager Sheet escaped a closed sidebar.** The actual project sidebar uses Sheet, so that wrapper now follows the same native Dialog close-action visibility scope. A real-primitives test proved the leak and now checks Sheet + nested Select disappearance and fresh reopening.
4. **Direct sidebar Select escaped the scope.** Project knowledge policy uses Select outside a Sheet. A scope-only open-state adapter now closes it without touching selected value/defaultValue or ordinary callers. Controlled/uncontrolled hide/show/open tests failed first and now pass. Native Select has only `unmount`, so its bounded adapter is documented as the exception to native close actions.
5. **Cancelled fresh trigger reopened a dismissed controlled popup.** `useScopedPopup` previously cleared its dismissal latch before the consumer could cancel the event. It now checks the native cancellation result first. A dedicated red/green regression proves cancellation remains authoritative.
6. **A late squad save could publish to the newly navigated squad key.** The added cache publication now derives workspace/squad identity from the accepted response instead of current route closure values. TanStack updates pending mutation options on a preserved observer; the response identity prevents cross-squad contamination.

## Verified

- Final sidebar real-primitives suite: **23/23** passed.
- Squad detail recovery/draft suite after archive fix: **14/14** passed.
- Core squad/detail/member/query-access suites: **24/24** passed.
- Full `packages/ui`, `packages/core`, and `packages/views` TypeScript checks: **passed**.
- Reviewer-edited UI/Core/Views ESLint: **0 errors,0 warnings**.
- `git diff --check` for reviewer changes: **passed**.
- Existing broader picker/locale review test run and main-session integrated test results remain owned by the main verification ledger; counts are not summed here to avoid duplication.

Logs and red proofs are in `evidence/core-ui-review-*`, `evidence/core-ui-archive-*`, `evidence/core-ui-directory-*`, `evidence/core-ui-sheet-red.log`, `evidence/core-ui-select-red.log`, `evidence/core-ui-popup-cancel-red.log`, and `evidence/review-lint.json`.

## Remaining limits

- Main session owns native Electron AX/keyboard evidence, rendered target sizing, detector output, and integrated verification.
- The native close-action scope assumes uncontrolled callers allow imperative closure. Actual audited sidebar callers do. A future uncontrolled caller cancelling the imperative close must also reconcile its open intent rather than expecting the scope to mutate native internals.
- No new dependency, backend API/schema change or broad refactor was introduced.
