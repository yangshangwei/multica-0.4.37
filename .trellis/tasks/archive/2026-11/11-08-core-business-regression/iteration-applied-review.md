# Review of current iteration receipt callback and merged Leave helper

Reviewed current HEAD `6201a58102668ce19e5f2959ea7dfb807d932a43`, its parent iteration change `2f222f65706052f5a2dc0c8380ca67e5ea650ded`, and the working-tree merged `leaveThroughUI`. Read-only review: no test, API, browser, build or source edit was performed by this lane. Root owns targeted unit session32252 and subsequent current-candidate rebuild/full acceptance.

## Verdict

No source-proven blocking regression found in these narrow changes. No product fix requested from this review. The intended callback boundary is correct for the present consumer: close the assignment dialog after a confirmed server receipt and retirement of its exact pending key, while retained Query invalidations still finish asynchronously.

## Receipt, session and access semantics

- `onApplied` is after `getIterationOperation`/`sendIterationCommand` resolves, a second `assertCurrentContext`, `defaultStorage.removeItem(key)`, pending notification and local pending clear. It is not optimistic UI and does not run on missing receipt, rejected write or unknown network outcome.
- `assertCurrentContext` verifies actor, current auth store actor, API session scope, workspace read access and access epoch before this callback. A replaced session or revoked/regranted scope cannot reuse the old callback to confirm an old intent. Existing assertions after each refresh remain unchanged.
- Recovery still reads the original request ID and reuses the exact payload after a documented missing operation. The callback fires after that recovered or newly returned receipt, not after the recovery lookup merely starts.
- `IterationAddExisting` passes only `onAdded`, whose current production parent implementation is `setAssignmentOpen(false)`. The dialog is explicitly `keepMounted`; closing does not cancel the hook or its invalidations. The form remains locked by `apply.isPending` if reopened before refresh settles, so the later `reset()` cannot erase newly editable input during that interval.
- Navigating away fully unmounts the old page; the captured parent state setter cannot close a new page's dialog. The asynchronous command retains server/cache cleanup as before. This hook callback is not itself a generic unmount-cancellation mechanism, so new consumers must not add unrelated global side effects without defining their lifetime.
- A late session/access change during refresh can still reject `mutateAsync` after the UI closed, because the server write was already confirmed for the old context. Exact pending storage has already retired, so it is not replayed as unknown work. The existing remaining refresh fences protect the new context.

## Callback and test caveats

The optional callback currently executes inside the mutation try block. A throwing future callback would skip subsequent invalidations and reject a confirmed command. The only current consumer is a synchronous React close setter, so this is not a demonstrated regression; keep `onApplied` callbacks nonthrowing UI notifications or explicitly isolate their errors if broader consumers are added.

The new hook regression proves `onApplied` runs once before an intentionally blocked refresh settles. Existing actor/session/access tests prove the fences, but do not pass an `onApplied` spy themselves; adding negative callback assertions to those canonical tests would make this newly named contract more explicit. This review did not add duplicate tests or re-run root's active unit command.

## Merged Leave helper

The merged helper waits for `Leave issue` or `More actions` to become visible before sampling which branch is available. It then waits for the chosen control to be enabled, opens the real disclosure when needed and clicks the real Leave menu item. The actual confirmation POST, HTTP200, idempotency key and input/receipt checks remain unchanged. This fixes the immediate empty-action-bar sample without raising any response budget or bypassing interaction.

`getByRole` defaults to accessible elements, so a hidden menu entry does not replace the direct-button readiness assertion. A later permanent capability change still fails rather than fabricating authority. The helper assumes callers begin with no modal covering the action bar, matching existing disclosure contracts; its change does not hide such a prerequisite failure.

## Current-state evidence

The reviewed files are SHA-256-bound in the adjacent private review record. Current product differs from the frozen725 candidate at the iteration hook/UI files in addition to earlier task repairs, so historical725 build/package results cannot be promoted to620 completion. Rebuild and exact package source/proof rebinding remain required after the namespace lane releases its environment.
