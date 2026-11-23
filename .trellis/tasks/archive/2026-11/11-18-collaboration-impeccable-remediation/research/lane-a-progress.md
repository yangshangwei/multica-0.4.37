# Lane A: picker and initial-field remediation

Owner: `/root/fix_fields`, resumed by `/root/resume_fields`. Scope: C01/C02/C03/C06/C09 only.

## Change boundary

The current custom-property search shell does not filter its real choices;
reversible multi-select rows do not expose their state; popup value inputs lose
their property label; and creation combines stage-schema errors with unavailable
catalog values. The repair stays at those existing owners.

Expected production paths are `property-picker.tsx`,
`custom-property-picker.tsx`, collaboration `field-inputs.tsx` and
`creation-form.tsx`, plus only the existing `creation.rejectionReasons.invalidStage`
key in the English and Simplified Chinese collaboration dictionaries. Adjacent
component regressions will cover the observable behavior. No Core schema, mutable
draft, dependency, global store, UI primitive, API, route or write workflow changes
are planned.

Shared picker changes will keep `optionId` listbox state and clear-command
semantics. Filtering will use the shell's existing search/reset contract. Stage
feedback will come from existing schema issue paths, preserving current catalog
checks and unfinished draft values. Coarse target floors and wrapping remain
local to the existing controls; browser geometry is owned by main.

## Preflight

- Read the complete saved Trellis hook context, task PRD/design/implementation
  plan, field-remediation research, applicable Views/UI guidance, locale
  conventions and Impeccable craft floor.
- Impeccable context loaded for the existing custom-property component. It
  permits scoped repairs using incumbent code; no automatic detector is active.
- Every owned source/test/locale path matched the saved working baseline before
  edits. The two collaboration dictionaries already have unrelated dirty hunks.
- Other lanes own knowledge, context and workflow. No unowned edits, task-state
  changes, commits, worktrees, stash/reset or live API writes will be performed.

## Regression and verification plan

1. Add real-component search-to-commit, no-match, reopen, state and label failures.
2. Add schema-stage attribution/correction and catalog-failure regressions.
3. Run the regressions before production changes and retain the failure log.
4. Apply minimal local repairs; run the lane and shared-picker compatibility
   suites with `--maxWorkers=2`.
5. Run scoped ESLint and an appropriate type check, self-review against the saved
   baseline, and give main concrete behavioral and browser-check boundaries.

Red/green evidence and exact changed paths will be added below as work completes.

## Pre-fix behavioral evidence

Command:

```sh
pnpm --dir packages/views exec vitest run issues/components/pickers/property-picker.test.tsx issues/components/pickers/custom-property-picker.test.tsx collaboration/field-inputs.test.tsx collaboration/creation-form.test.tsx --maxWorkers=2
```

Result: **4 failed files; 20 failed / 50 passed tests**, 15.36 seconds. Complete
output is retained in `research/lane-a-red.log` before production changes.

- Exact/partial search retained nonmatching Choice 1 and Choice 2 rows.
- No-match Enter wrote `choice-1`; multi-select added that nonmatch to the
  retained `choice-2`; label search Enter incorrectly cleared the selection.
- Native pressed-state queries could not find the selected label/custom rows.
- Real text/number/URL inputs could not be found by their property names.
- Stage input had no `aria-invalid` attribute or field-error association in
  English/Chinese and simultaneous-catalog-error regressions.

The passing checks include existing picker keyboard/IME behavior and the
unavailable-value command boundaries. These are behavioral failures rather than
test harness, import or environment failures. C09 retains the original compiled
Chromium 32px evidence as its red baseline; this lane makes no jsdom geometry
claim. Main owns the combined mechanical detector and real-browser verification.

## Final implementation and resume evidence

The resumed lane retained the existing production changes and original red
regressions. The only additional production edits were the existing stage
editor's visibility condition (`parentIssueId || stageError`) and `role="status"`
on `PickerEmpty`.

| Finding | Implemented behavior | Component evidence |
| --- | --- | --- |
| C01 | Custom single/multi-select uses the shell's local search and close/reset contract, filters real configured choices, preserves hidden selections, and announces no matches. Typed no-match Enter does not write; explicit clear still works. | Exact/partial/case-insensitive search, no-match Enter/status, reopen, retained selections and changed-definition regressions in the actual custom-property component. |
| C02 | Explicit `toggle` exposes native pressed state only for reversible custom multi-select and available collaboration label rows. Account `optionId` state takes precedence; clear, ordinary selection and unavailable-label removal remain commands. | Initial/add/remove/Space state, command exclusions and account option semantics in picker/label/account suites. |
| C03 | Text/number/URL popup inputs have an instance-specific ID and visible wrapping `property.name` label. Existing parsing, trimming, clearing and URL-open behavior remain. | Real input names, separate same-property editor IDs, text/URL Enter, decimal input/form submission and URL-open/clear checks. |
| C06 | Creation uses the existing schema issue paths for localized stage feedback, preserving catalog errors and submit validity. A malformed saved stage without a parent renders the same editable stage field; no value is silently discarded. | English/Chinese associated field error, zero/fraction/upper-bound wiring, correction/blank, retained recipient/file/draft, simultaneous catalog error and restored no-parent clearing regressions. |
| C09 | Coarse-pointer floors cover existing shared rows/search and text-value inputs plus the URL icon action; popup width and label/choice wrapping remain local. | Original compiled Chromium 32px red evidence is retained. Main owns final compiled 44px/narrow/theme geometry; this lane makes no jsdom geometry claim. |

The additional resume regression command was:

```sh
pnpm --dir packages/views exec vitest run issues/components/pickers/custom-property-picker.test.tsx collaboration/creation-form.test.tsx -t '(no-match|restored invalid stage without a parent)' --maxWorkers=2
```

Before those two production edits, **3 tests failed / 43 skipped**: the restored
invalid stage had no spinbutton, and both select modes had no accessible status
for the visible no-match result. Output is retained in
`research/lane-a-resume-red.log`.

The first complete lane run passed **162 tests** and exposed one new numeric
Enter test-harness failure, retained in `research/lane-a-green-first.log`.
Isolation/probe logs are `research/lane-a-number-isolation.log` and
`research/lane-a-number-probe.log`. The probe verified the actual numeric value
`1.25`, focus and native validity before Enter. Installed `user-event`'s
`event/behavior/keypress.js` excludes `number` from
`SubmitSingleInputOnEnter` (lines 40–49), so it does not dispatch the implicit
submission for a single number input. The numeric DOM case now types the same
decimal, asserts its raw value/step/inputMode/validity, and submits the real
form. Production submission remains native. Text/URL DOM cases retain Enter;
main owns actual numeric Enter evidence in Chromium.

## Final checks

```sh
pnpm --dir packages/views exec vitest run issues/components/pickers/property-picker.test.tsx issues/components/pickers/custom-property-picker.test.ts issues/components/pickers/custom-property-picker.test.tsx collaboration/field-inputs.test.tsx collaboration/creation-form.test.tsx collaboration/field-editing.test.tsx collaboration/account-picker.test.tsx locales/parity.test.ts --maxWorkers=2
```

Result: **8 files / 163 tests passed**, 18.75 seconds, exit 0.
Output: `research/lane-a-green.log`.

```sh
pnpm --dir packages/views exec vitest run issues/components/pickers/assignee-picker.keyboard.test.tsx issues/components/pickers/actor-property-picker.test.ts issues/components/pickers/status-picker.test.tsx issues/components/pickers/stage-picker.test.tsx projects/components/project-picker.test.tsx modals/quick-create-actor-picker.test.tsx --maxWorkers=2
```

Result: **6 files / 45 tests passed**, 7.68 seconds, exit 0.
Output: `research/lane-a-compatibility.log`.

```sh
pnpm --dir packages/views exec eslint issues/components/pickers/property-picker.tsx issues/components/pickers/property-picker.test.tsx issues/components/pickers/custom-property-picker.tsx issues/components/pickers/custom-property-picker.test.tsx collaboration/field-inputs.tsx collaboration/field-inputs.test.tsx collaboration/creation-form.tsx collaboration/creation-form.test.tsx
```

Result: **exit 0**, no ESLint findings. Output: `research/lane-a-eslint.log`.
These pnpm commands emit the existing root configuration warning about ignored
`pnpm.onlyBuiltDependencies`/`pnpm.overrides`; no package configuration changed.
The owned tracked diff passed `git diff --check`; the new component suite has
no trailing whitespace. No relevant test or lint failure remains in this lane.

## Exact owned changes

Production:

- `packages/views/issues/components/pickers/property-picker.tsx`
- `packages/views/issues/components/pickers/custom-property-picker.tsx`
- `packages/views/collaboration/field-inputs.tsx`
- `packages/views/collaboration/creation-form.tsx`
- `packages/views/locales/en/collaboration.json`: existing `creation.rejectionReasons.invalidStage` only.
- `packages/views/locales/zh-Hans/collaboration.json`: existing `creation.rejectionReasons.invalidStage` only.

Regressions:

- `packages/views/issues/components/pickers/property-picker.test.tsx`
- `packages/views/issues/components/pickers/custom-property-picker.test.tsx` (new real-component suite).
- `packages/views/collaboration/field-inputs.test.tsx`
- `packages/views/collaboration/creation-form.test.tsx`

The existing pure `custom-property-picker.test.ts` is unchanged and was run.
Compared the production diff against `research/source-baseline.json` snapshots;
both locale diffs contain only the named stage string beyond their original
dirty baseline. Other lanes' files and unrelated locale hunks were preserved.
No spec, task status, plan, commit, reset/stash, browser or live-data operation
was performed by the resumed lane.

## Integration handoff

Lane A source and regression work is ready. Main owns combined typecheck/lint,
static detector, final current-source browser geometry/interaction, independent
review, specs and audit closure. The Chromium fixture should verify numeric
and URL Enter, no-match announcements, real field names, 44px coarse row/search/
input/URL-action targets and 320px wrapping in both themes. Physical devices,
spoken assistive technology and installed Electron Main were not exercised by
this lane.
