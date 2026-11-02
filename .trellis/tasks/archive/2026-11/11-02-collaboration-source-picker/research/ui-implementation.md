# Shared source-picker implementation

## Result and ownership

- `packages/views/collaboration/field-inputs.tsx`: replace disconnected search/native parent select with the existing searchable property picker. Opening shows recent source candidates; search results are visible inline. Selection keeps the parent UUID and chosen display label; reopening clears search. Loading, empty workspace, no matches, truncation and retry are distinct. Both candidate and clear rows retain 44px minimum targets. Optional identifiers from older responses do not render as `undefined`.
- `packages/views/collaboration/creation-form.tsx`: forward pending/placeholder/error/retry state, retain current source Query data on a transient search error, and keep the form mounted. Authoritative source denial never reuses retained data.
- `packages/views/collaboration/workflow.tsx`: forward the same picker state for source-field editing; a failed field read blocks source submission while allowing retry in the retained form.
- `packages/views/issues/components/pickers/property-picker.tsx`: add optional `searchMaxLength` and pass it to the existing input; the parent approved this small additional scope. The source picker passes 200; other callers retain the default.
- Both collaboration locale JSONs explain the optional parent relationship and localize each feedback state.
- `field-inputs.test.tsx`, `creation-form.test.tsx`, and the existing `field-editing.test.tsx` cover the shared UI and caller wiring through actual primitives.

No backend/core-query code, app build, `next-env.d.ts`, other task records, or pre-existing unrelated edits were changed by this lane. No commit was made.

## Verification

1. Before implementation, `pnpm --filter @multica/views test collaboration/field-inputs.test.tsx` failed all six initial regressions against the original UI: the accessible Parent issue button/task list did not exist.
2. `pnpm --filter @multica/views test collaboration/field-inputs.test.tsx collaboration/field-editing.test.tsx collaboration/creation-form.test.tsx issues/components/pickers/property-picker.test.tsx locales/parity.test.ts` passed **95 tests in 5 files**.
3. `pnpm --filter @multica/views typecheck` passed, including the final nested-dialog and source-denial regressions.
4. `pnpm --filter @multica/views exec eslint collaboration/field-inputs.tsx collaboration/field-inputs.test.tsx collaboration/creation-form.tsx collaboration/creation-form.test.tsx collaboration/workflow.tsx collaboration/field-editing.test.tsx issues/components/pickers/property-picker.tsx` passed.
5. Task-owned `git diff --check` passed.
6. After the final clear-row target-height adjustment, the narrow field-inputs suite passed **7 tests**, and its file-level ESLint passed.

The tests cover click-without-search, selection/clear, reopening, result visibility, pending/failed candidate suppression, retry focus, nested Escape without closing the outer dialog, IME/arrow/Enter behavior, selected-label recovery, delayed old-query rejection, mounted creation draft retention and authoritative denial. Backend identifier matching and production browser/capture validation remain with the parent/server lane.

## Check corrections

The initial new Query fixtures omitted the required status `category`; they were corrected to the real creation-options schema. Testing Library role queries use exact string names by default and do not accept Playwright's `exact` option; the new role assertions were corrected before the final type check. Neither issue required production compatibility bypasses.
