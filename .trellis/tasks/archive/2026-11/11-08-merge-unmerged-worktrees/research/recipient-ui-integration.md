# Recipient/UI source integration

Integration checkout: `/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108`.
Source: `mc-recipient-integration-20261030-072838-a3fe`, source HEAD `2612465a016aa606cc7a2538e7890f1f4b76d995`; source archive and per-path hashes are recorded in `recipient-takeover-decisions.json` and `takeover-preservation-manifest.json`.
Target baseline: `e541cfb436fed9c6733174479718b651e2cf030d`.

## Boundary and changes

Only the missing reviewed UI increments were applied. Backend/Go tests, source worktrees, root task metadata, Git index and commits were not touched by this implementer. No dependencies added. Source reaction label removals were rejected, retaining current localized action names and count descriptions.

- The 33-file `recipient-safe-source-candidates.patch` passed `git apply --check` against the current integration tree, then was applied without staging. It coordinates `input` boundary versus `input-fill` tone across shared primitives/pickers, fixes four project heading role tokens, retains visible restricted-resource placeholders with bilingual labels, and disables ordinary batch operations when collaboration items are selected.
- Four source workflow paths were adapted, not copied: `CollaborationWorkflow` now invokes optional `onApplied` only for already-accepted successful receipts. Source bridge uses that signal for a persistent success toast; `onDismiss` still owns closing/cancellation, and existing `embedded`, busy locking, request recovery and focus contracts remain intact.
- Preserved existing test assertions and added behavioral checks: unnamed restricted images/files remain visibly withheld without resolver/network resource rendering; all batch operation triggers advertise disabled state, direct callbacks cannot mutate, and clear selection remains enabled with the reason associated; confirmed workflow actions signal success once, uncertain/refused responses and ordinary dismissal do not.

## Verification

- Focused Views command: `pnpm --filter @multica/views exec vitest run collaboration/source-bridge.test.tsx collaboration/field-editing.test.tsx collaboration/workflow.test.tsx rich-content/rich-content-parity.test.tsx issues/components/batch-action-toolbar.confirm.test.tsx --maxWorkers=2`: **5 files, 54 tests passed**. Log `/tmp/recipient-ui-views-tests-20261108.log`.
- Web stylesheet checks: `pnpm --filter @multica/web exec vitest run app/brand-variant-cascade.test.ts app/text-contrast.test.ts --maxWorkers=2`: **2 files, 37 tests passed**. Log `/tmp/recipient-ui-web-tests-20261108.log`.
- Locale parity: `pnpm --filter @multica/views exec vitest run locales/parity.test.ts --maxWorkers=2`: **1 file, 66 tests passed**. Log `/tmp/recipient-ui-locale-tests-20261108.log`.
- `pnpm exec turbo run lint typecheck --filter=@multica/ui --filter=@multica/views --concurrency=2`: UI lint/typecheck and Views lint passed; Views lint reports 27 existing warnings and zero errors. The initial Views typecheck found an unsupported Testing Library `exact` option in two new assertions; these were corrected. The second assertion also had an ambiguous mocked `label` selector in the first focused run; it now uses the button's actual aria-label.
- Final `pnpm --filter @multica/views typecheck`: **passed**, exit 0 after correcting the two test assertions. Static logs: `/tmp/recipient-ui-static-20261108.log`, rerun `/tmp/recipient-ui-typecheck-rerun-20261108.log`.
- `git diff --check` passed.

## Main-session validation still required

No browser or screenshot verdict was performed by this implementer. Main session owns full quality gate and visual checks: light/dark input borders and focus (including half-opacity rings), preserved read-only/disabled fills, active brand chip behavior, batch toolbar at narrow viewport, project headings, and restricted image/link/file placeholders. The existing text-contrast test excludes input/ring: its pass must not be reported as proof of the new boundary/focus contrast ratios. No product data/API semantics changed.

## Changed-file manifest

- `apps/web/app/brand-variant-cascade.test.ts`
- `packages/ui/components/ui/button.tsx`
- `packages/ui/components/ui/checkbox.tsx`
- `packages/ui/components/ui/command.tsx`
- `packages/ui/components/ui/input-group.tsx`
- `packages/ui/components/ui/input-otp.tsx`
- `packages/ui/components/ui/input.tsx`
- `packages/ui/components/ui/select.tsx`
- `packages/ui/components/ui/switch.tsx`
- `packages/ui/components/ui/tabs.tsx`
- `packages/ui/components/ui/textarea.tsx`
- `packages/ui/components/ui/time-input.tsx`
- `packages/ui/styles/base.css`
- `packages/ui/styles/tokens.css`
- `packages/views/admin/filter-feedback.tsx`
- `packages/views/agents/components/inspector/model-picker.tsx`
- `packages/views/agents/components/inspector/runtime-picker.tsx`
- `packages/views/agents/components/inspector/service-tier-setting-field.tsx`
- `packages/views/agents/components/inspector/thinking-picker.tsx`
- `packages/views/autopilots/components/pickers/timezone-picker.tsx`
- `packages/views/autopilots/components/schedule-editor/schedule-editor.tsx`
- `packages/views/collaboration/field-editing.test.tsx`
- `packages/views/collaboration/source-bridge.test.tsx`
- `packages/views/collaboration/source-bridge.tsx`
- `packages/views/collaboration/workflow.tsx`
- `packages/views/issues/components/batch-action-toolbar.confirm.test.tsx`
- `packages/views/issues/components/batch-action-toolbar.tsx`
- `packages/views/issues/components/issue-usage-dialog.tsx`
- `packages/views/locales/en/editor.json`
- `packages/views/locales/zh-Hans/editor.json`
- `packages/views/projects/components/project-detail.tsx`
- `packages/views/projects/components/project-overview.tsx`
- `packages/views/projects/components/project-progress.tsx`
- `packages/views/projects/components/project-risk-issues.tsx`
- `packages/views/rich-content/rich-content-parity.test.tsx`
- `packages/views/rich-content/rich-content.tsx`
- `packages/views/skills/components/skill-presentation-fields.tsx`
