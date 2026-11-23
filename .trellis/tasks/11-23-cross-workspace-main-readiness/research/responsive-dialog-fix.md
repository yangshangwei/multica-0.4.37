# Responsive collaboration-dialog fix

2026-11-23. Bounded checker self-fix authorized by the parent after the original browser failure was observed and diagnosed. Source base: `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`. All edits and checks took place in the separate private checkout `.omx/private/qa-work/main-readiness-20261123/modal-fix`; the frozen `src` browser candidate and root product files were not modified. No staging, commits, new dependency or new store.

## Result and scope

The creation entry now remains a keyed sibling of the responsive `Sidebar`, so changing between its desktop column and compact Sheet preserves the open form instance and draft. Opening creation closes the compact navigation Sheet. Explicit cancellation, Escape and close-button dismissal restore that Sheet and its actual Start collaboration focus; successful creation closes/navigates without reopening navigation.

Five files changed:

- `packages/views/layout/app-sidebar.tsx`: stable sibling placement, close the mobile Sheet on creation entry, separate dismissal handling, consume one requested initial-focus target, and clear it on route/scope change. Existing API/user/workspace key and dynamic wide/More return refs remain intact.
- `packages/ui/components/ui/sidebar.tsx`: optional typed `compactInitialFocus` passthrough to the existing SheetContent's native `initialFocus`. No business logic or default behavior change. Native null focus resolution retains Base UI's default fallback.
- `packages/views/collaboration/creation-form.tsx`: optional `onDismiss` callback for actual user dismissal, shared by Dialog close/Escape and Cancel. The existing successful-create callback still closes and navigates separately; it does not notify dismissal. Existing callers omitting the callback retain their behavior.
- `packages/views/layout/app-sidebar.test.tsx`: **four new cases**, actual SidebarProvider/Sidebar/Sheet plus a stateful draft stub using the real Dialog. Existing lightweight query/domain mocks remain; the responsive unmount is never simulated by a mock.
- `packages/views/collaboration/creation-form.test.tsx`: extend three existing real-form tests with exact dismissal counts and no-dismiss-on-success assertions; no new case population here.

The extra narrow APIs were required by real failures during the first sibling-only fix. A retained open mobile Sheet remounted above the sibling dialog and made it inaccessible. Closing the Sheet on entry fixed stacking, but restoring it on every `onOpenChange(false)` also briefly mounted it during successful navigation. Explicit `onDismiss` removes that ambiguity. Sheet autofocus could override the returning form focus; the optional native initial-focus target solves this without DOM queries, forced focus timers or recovery loops. Intermediate failed outputs are retained below.

## Test-first and final evidence

Raw logs root `M`: `.omx/private/qa-work/main-readiness-20261123/modal-regression/`.

| Evidence | Result |
| --- | --- |
| `M/red.log` before any product edit | Original 72 sidebar cases passed; the three initial new responsive cases failed: disappeared closed-Sheet draft, remounted reverse-crossing draft, and missing scoped form. |
| `M/green.log` sibling-only intermediate | 74 passed / 1 failed; compact-open round-trip left the preserved draft inaccessible beneath a new Sheet. |
| `M/green-r2.log` intermediate | 74 passed / 2 failed; native reopened Sheet autofocus won over attempted form return focus. |
| `M/green-r3.log` intermediate | 75 passed / 1 failed; cancellation focus fixed, but completion could mount a transient navigation Sheet. |
| `M/dismiss-red.log` before `onDismiss` implementation | Real creation suite 30 passed / 3 failed at newly added missing-dismiss callback assertions. |
| `M/final-unit.log` | **4 files / 126 tests passed**: sidebar 76, real creation form 33, sidebar auto-collapse 9, page-header 8. One worker, no file parallelism. |
| `M/views-lint.log`, `M/ui-lint.log` | Scoped ESLint passed with max-warnings=0 for all five changed files. Only the existing pnpm configuration notice appeared. |
| `M/typecheck.log` | Core/UI/plugin-sdk typechecks passed; Views identified a test-only unsupported RTL `exact` option. That option was removed; RTL's string accessible-name match is already exact. No product change followed. |
| `M/creation-final.log`, `M/creation-final-lint.log` | After that single test typing correction: the complete real creation suite **33/33 passed** and scoped lint passed again. These replace existing results; they do not add 33 to the 126 total. |
| `M/views-typecheck-confirmed.log`, `M/views-typecheck.exit` | Final complete Views typecheck **exit 0**. UI success above still applies: UI source was unchanged after its successful check. |
| `git diff --check` in modal-fix | Passed. |

Final unit command:

```sh
pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx collaboration/creation-form.test.tsx layout/sidebar-auto-collapse.test.tsx layout/page-header.test.tsx --maxWorkers=1 --no-file-parallelism --reporter=verbose
```

The responsive cases prove stable DOM/input instance and entered value through large→closed compact→large and compact-open→large→compact. They also verify source-key reset, explicit Cancel/Escape focus, and no navigation reopening on successful completion. The real creation suite verifies controlled and uncontrolled dismissal once per action, all Escape/Cancel/Close paths, retained cancelled draft, and successful native Enter submission without `onDismiss`.

## Integration and remaining validation

Ready for parent integration **after** the frozen full suite and planned original rail red evidence. Copy only these five changes; do not copy this checkout wholesale. Parent reviewed the three product diffs independently. No new review agent was spawned.

The root-owned navigation spec needs a narrow addition: dialogs with active drafts must be outside the responsive Sidebar subtree; compact entry closes the Sheet, explicit dismissal restores the original navigation action through native initial focus, creation success navigates without restoration. Record the optional `onDismiss` distinction and typed primitive passthrough. No editorial overhaul is needed.

Actual production-browser validation of the original recipient journey and the relevant Web/Electron rail creation/resize/Escape paths remains pending rebuilt integration. The jsdom result proves ownership and callback/focus behavior, not painted mobile geometry, native popup focus or packaged Main compatibility. No new native success-path claim is made.

## Final SHA-256 fingerprints

```text
34a3b45cba6427eca6c3c2f8abb39c057a52fd833cd830a0f3602dcf52de7ff8  packages/ui/components/ui/sidebar.tsx
68e91a053600cfbdde4383a15e75d7bd723cac2890b48abe9f3483e647a4d33c  packages/views/collaboration/creation-form.test.tsx
f9c7a6c3f7d1b7db5a5e7a8d85717a4e7adadd317bac26420a88b33c8dddbc79  packages/views/collaboration/creation-form.tsx
604639f29eb5714341e8b3b0bdd7bf494f27f7d915e5f9329fd2debed628bf12  packages/views/layout/app-sidebar.test.tsx
b18233ef94415de56efb20fc1aaaa2ae93b023ff83794891098a13d99fe75c20  packages/views/layout/app-sidebar.tsx
```
