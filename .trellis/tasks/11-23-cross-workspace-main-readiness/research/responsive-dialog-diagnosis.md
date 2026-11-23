# Responsive collaboration-dialog diagnosis

2026-11-23. Read-only diagnosis against private source `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`. No product/tests were changed or executed during the frozen full browser run.

## Confirmed failure and owning cause

The original `e2e/collaboration-recipient-picker-web.spec.ts` successfully opens the creation dialog from the account sidebar, fills the title, searches recipients and verifies that Escape closes only the picker. It then crosses from the large viewport to 375px at line 122. The next Recipient click times out because the creation dialog is no longer mounted.

Raw evidence:

- `.omx/private/qa-work/main-readiness-20261123/core/final-default/playwright.log:250` records the original case's timeout at line 123, locating Recipient inside the missing named dialog.
- `core/final-default/results/collaboration-recipient-pi-e1741--the-outer-dialog-on-Escape-chromium/error-context.md` shows only the account page and its compact sidebar opener; no creation dialog or sidebar navigation is present. No separate screenshot was found in that failed result directory during this inspection.

The smallest causal chain is in the shared layout:

1. `packages/views/layout/app-sidebar.tsx:567` owns `collaborationOpen`; `:635` renders the keyed `CollaborationCreationEntry` **inside** `<Sidebar>` (`:611`).
2. `packages/ui/components/ui/sidebar.tsx:142` derives compact layout from `useIsCompact()` (1024px); `:143` initializes independent `openMobile=false`. At `:319`, crossing the breakpoint changes the returned desktop `<div>` subtree into a `<Sheet open={openMobile}>`. The entry is part of that subtree's `children`.
3. `packages/ui/components/ui/sheet.tsx:55` places all children under `SheetPortal` without `keepMounted`. Installed Base UI `dialog/portal/DialogPortal.js:25–36` defaults `keepMounted=false` and returns null when not mounted. A newly entered compact Sheet is closed, so the creation entry and its portalled form are removed along with the desktop navigation subtree.
4. AppSidebar itself and its `collaborationOpen` state remain mounted above this switch. The problem is the modal component's placement under a disposable responsive subtree. A body portal does not preserve React ownership across parent unmount.

The reverse crossing also replaces ancestors, so an open compact Sheet can remount/reset descendants when returning to the desktop branch. Existing delivery-form storage may restore persisted fields after a later reopen (`packages/core/collaboration/delivery-form.ts` saves changes); the observed failure establishes **unexpected dialog disappearance and loss of mounted interaction state**, not proof that every durable draft byte was erased.

## Other closure paths considered

- The pre-resize Escape assertion already confirms the outer dialog is visible. The failure is subsequent to the viewport transition, not a reproduced Escape propagation defect.
- `CollaborationCreationEntry` has explicit close/cancel and on-created navigation paths; it has no viewport-driven close effect (`creation-form.tsx:29–86`).
- AppSidebar's full-path effect (`:579`) closes the navigation Sheet, not `collaborationOpen`; the test does not change route between the successful outer-dialog assertion and resize.
- The creation key is API scope + user + displayed workspace ID. `useCollaborationScope` depends on API/user identity, not viewport; current workspace selection is unchanged by this resize.
- The inspected account shell does not add a breakpoint-dependent `PopupVisibilityProvider`; the generic popup visibility code therefore does not supply a competing intentional close explanation.

## Smallest repair and verification plan

Keep ownership in the existing shared `AppSidebar`, but return a fragment with `<Sidebar>…navigation and rail…</Sidebar>` followed by the **same keyed `CollaborationCreationEntry` as a stable sibling**. Preserve scope/source/open callbacks, return-focus refs and explicit closure. No new store, dependency, platform duplicate or `Sidebar` primitive change is required. Both Web account/workspace shells and Desktop use this shared component.

The current `app-sidebar.test.tsx` substitutes a fragment for `Sidebar` and a stateless placeholder for the creation entry, so its compact tests cannot expose this React-tree loss. Add a bounded real-primitive test mode using the actual `SidebarProvider` and responsive `Sidebar` implementation. Drive actual `matchMedia` listeners/innerWidth across 1024px; use a small stateful draft entry stub whose instance identity and entered text prove preservation, without mocking the unmount behavior itself.

Required regressions:

- Open from the large sidebar, enter text, cross into a **closed** compact Sheet and back; same draft instance remains visible and unchanged.
- Start inside an **open** compact Sheet and cross back to large, then compact; preserve the same draft instance and text independently of navigation-sheet visibility.
- Explicit close remains effective; scope/workspace key change still resets the scoped entry rather than carrying data across identities.

Run that new regression red before the sibling move, green after, then the owning sidebar/creation suites and scoped lint/typecheck. After the frozen full run ends, replay the unchanged original recipient-picker journey against rebuilt production Web, plus the relevant shared/native creation path. Retain the original failure and build/source binding; do not solve it by reopening the dialog inside the test or shrinking the viewport before the original assertions.
