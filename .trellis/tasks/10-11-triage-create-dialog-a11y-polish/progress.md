# Progress — 2026-10-11

## Resume scope

The user requested continuation of this in-progress task and then explicitly selected this session to take over all remaining changes. The existing implementation and tests are included in the reviewed task scope. The unrelated `10-11-admin-auto-initialization` task remains outside this work.

## Browser verification

Reused the existing component harness under `.omx/qa/triage-create-dialog/`, mounting the real form, editors, pickers, styles and Query providers with fixture API responses. No user workspace data was created or modified by the harness.

- Chinese desktop: 1280 × 900.
- Chinese touch: 390 × 844.
- English compact touch: 320 × 568.
- English dark: 1280 × 900.
- All four cases: no page overflow and no JavaScript page errors; footer stays within the viewport.
- Touch picker triggers and close button measure at least 44px. Desktop picker triggers measure 32px.
- Real keyboard interaction: all four pickers open with Enter, Escape returns focus to the trigger.
- Blank-title shortcut, invalid source URL and inverted dates do not invoke the mutation. Errors remain associated with fields; the first invalid field receives focus.
- Candidate project and assignee retain their accessible selected-value descriptions and existing payload names.
- A long description scrolls within the form body while the footer remains visible.
- Simulated server failure preserves the draft; an unchanged retry keeps its request ID and closes only after success.

Evidence: `verified-report.json`, `interaction-report.json`, `*-verified.png` and `long-content-scrolled.png` under the harness directory. Visual verdict: `.omx/state/triage-create-dialog-a11y-polish/ralph-progress.json` (94/pass), compared with the original supplied screenshot and previous capture. These ignored artifacts are local evidence, not committed product files.

## Final automated verification

- Targeted creation/validation/PropertyPicker regression: 3 files, 60 tests passed.
- Related triage/editor/picker/locale suites: 99 files, 1,620 tests passed (`DEBUG_PRINT_LIMIT=800 pnpm --dir packages/views exec vitest run triage editor issues/components/pickers locales/parity.test.ts --maxWorkers=1`).
- Views and UI TypeScript checks passed (`pnpm --dir packages/views typecheck`, `pnpm --dir packages/ui typecheck`).
- Scoped ESLint over the changed UI/views code and tests passed.
- Impeccable detector over changed UI source returned `[]`; `git diff --check` and task-context validation passed.
- Independent review reproduced and fixed two new-prop paths: deferred custom triggers omitted their initial field name, and custom render children displaced the selected-value description. Four new regressions failed before correction. The final focused run passed 6 files / 43 tests, including both new paths and creation/project/assignee behavior.
- After the final picker changes: views typecheck and affected-file ESLint passed; the unchanged UI package retained its passing typecheck. The detector was rerun for the final modified source and returned `[]`.
- The browser interaction script was rerun after the final fixes, including exact selected-assignee description; all checks passed without page errors.

## Resolution

All acceptance criteria are satisfied within the approved scope. Independent
Trellis review reports no remaining findings. The form reuses shared picker
trigger preparation for eager/deferred paths and retains existing business APIs,
request identity, upload flow and acceptance wording. No dependencies were added.

Work is ready for the code/spec commit, task archive and journal record.

## Verification limits

The browser checks use a component harness with fixture API responses. They do not certify real screen-reader speech, physical touch/IME behavior, all native operating systems, or the complete product in dark mode.

## Changed files

- `.trellis/spec/views/frontend/index.md`
- `.trellis/spec/views/frontend/triage-create-form.md`
- `packages/ui/components/common/file-upload-button.tsx`
- `packages/ui/components/ui/dialog.tsx`
- `packages/views/editor/content-editor-accessibility.test.tsx`
- `packages/views/editor/content-editor.tsx`
- `packages/views/editor/title-editor.shortcut.test.tsx`
- `packages/views/editor/title-editor.tsx`
- `packages/views/issues/components/pickers/assignee-picker.tsx`
- `packages/views/issues/components/pickers/deferred-trigger.test.tsx`
- `packages/views/issues/components/pickers/label-picker.tsx`
- `packages/views/issues/components/pickers/priority-picker.tsx`
- `packages/views/issues/components/pickers/property-picker.test.tsx`
- `packages/views/issues/components/pickers/property-picker.tsx`
- `packages/views/locales/en/triage.json`
- `packages/views/locales/zh-Hans/triage.json`
- `packages/views/projects/components/project-picker.tsx`
- `packages/views/triage/triage-create-dialog.test.tsx`
- `packages/views/triage/triage-create-dialog.tsx`
- `packages/views/triage/triage-create-dialog.validation.test.ts`
- `packages/views/triage/triage-fields.tsx`
