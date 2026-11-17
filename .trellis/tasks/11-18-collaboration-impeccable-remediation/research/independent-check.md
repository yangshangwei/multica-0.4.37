# Independent C01–C09 review

Date: 2026-11-18. Reviewer: `/root/quality_review` (`trellis-check`).

The task-scope review is complete. Three mechanical assertion options were
removed, the affected regressions passed, and the final root lint and type
checks passed. No additional task-scope behavior defect was found.

The review loaded the full saved native hook, all 13 check-manifest entries,
PRD/design/implementation artifacts, root `CLAUDE.md` and relevant package
indexes. Production changes were compared with the working snapshots identified
by `research/source-baseline.json`, not the checkout's unrelated branch diff.
The saved baseline hashes were verified before comparison. The two collaboration
locale diffs contain only the bounded-stage message beyond their existing dirty
baseline. Core lease/cache implementation and the ordinary issue dictionaries
remain identical to the task baseline.

## Findings (fixed)

- File: `packages/views/collaboration/creation-form.test.tsx:833`
  - Issue: A new unavailable-property assertion supplied `exact: true` to
    Testing Library's `getByRole`, which is not a supported `ByRoleOptions`
    member. Root type checking reported TS2769.
  - Fix: Removed that option. The string accessible name `Clear`, row scope,
    interaction and subsequent draft/eligibility assertions are unchanged;
    role queries match a string name exactly by default.
- File: `packages/views/collaboration/field-inputs.test.tsx:186` and `:209`
  - Issue: Two new clear-command assertions had the same unsupported option
    and TS2769 failure.
  - Fix: Removed both options while preserving the `None` accessible name and
    all selected-state/no-match assertions.

Main released Lane A's test ownership before these edits. No production file,
permission, schema, test expectation, task status, plan, spec or dependency was
changed by the reviewer. The only simplification is deletion of the three
invalid options.

## Findings (not fixed)

No unresolved C01–C09 code finding remains.

Root lint reports **31 warnings outside this task**: 27 in Views, three in the
UI stepper, and one in Desktop. They are existing hook-dependency or unused
disable-directive diagnostics. They were left alone because those files and
behaviors belong to other work. The complete final diagnostics are preserved
in `research/independent-lint.log`; the affected locations are:

| File | Warning locations | Count |
| --- | --- | ---: |
| `apps/desktop/src/renderer/src/components/tab-content.tsx` | 54 | 1 |
| `packages/ui/components/ui/stepper.tsx` | 144, 249, 255 | 3 |
| `packages/views/agents/agents-i18n-parity.test.ts` | 150, 166 | 2 |
| `packages/views/agents/components/tabs/mcp-config-tab.tsx` | 112 (two dependency diagnostics) | 2 |
| `packages/views/attachments/use-html-preview-scroll-restore.ts` | 211 | 1 |
| `packages/views/common/single-row-fit.ts` | 64 | 1 |
| `packages/views/editor/zoom-canvas.tsx` | 161 | 1 |
| `packages/views/issues/components/execution-log-section.tsx` | 250 | 1 |
| `packages/views/issues/components/issue-usage-dialog.tsx` | 72, 85, 231 | 3 |
| `packages/views/issues/components/swimlane-view.tsx` | 964 | 1 |
| `packages/views/issues/components/table-view.tsx` | 1848 | 1 |
| `packages/views/issues/surface/use-issue-group-branches.ts` | 331 | 1 |
| `packages/views/onboarding/onboarding-flow.tsx` | 243 | 1 |
| `packages/views/projects/components/project-detail.tsx` | 154, 262 | 2 |
| `packages/views/runtimes/components/charts/activity-heatmap.tsx` | 204 | 1 |
| `packages/views/runtimes/components/usage-section.tsx` | 373, 383, 699, 703 | 4 |
| `packages/views/runtimes/utils.test.ts` | 1079, 1089, 1101, 1123, 1133 | 5 |

The commands also emit the existing pnpm advisory about ignored root
`pnpm.onlyBuiltDependencies`/`pnpm.overrides`. Configuration migration is outside
this remediation. An earlier progress message counted only 28 lint warnings
from a tail excerpt; this final count includes the three cached UI warnings.

Browser evidence remains main-owned. The reviewed regression result has 17
scenarios and no failures, page errors or blocked requests. All production and
CSS source hashes match. Its manifest also records `creation-form.test.tsx`,
whose assertion-only change above occurred after the build; that test file is
listed for provenance and is not imported by the browser fixture. Main was
notified to record that exact post-build difference while retaining the original
manifest. No product/CSS difference or new browser defect was found.

Physical devices, spoken assistive technology, installed Electron Main and live
collaboration writes remain separate verification surfaces. This review does
not claim those checks.

## Verification

### Commands run by the independent reviewer

| Check | Result | Evidence |
| --- | --- | --- |
| Initial `pnpm typecheck` | Exit 2; exactly the three TS2769 errors above | `research/independent-typecheck-initial.log` |
| Final `pnpm typecheck` | **Pass, exit 0; 9/9 successful tasks, six cached; 3m12.018s** | `research/independent-typecheck.log` |
| Initial `pnpm lint` | Pass, exit 0; 6/6 successful tasks; 19.77s | `research/independent-lint-initial.log` |
| Final `pnpm lint` | **Pass, exit 0; 6/6 successful tasks, five cached; 20.95s; zero errors, 31 warnings outside task** | `research/independent-lint.log` |
| Affected component suites | **Pass, exit 0; 2 files / 44 tests; 18.43s; no failures or skips** | `research/independent-tests.log` |
| Task-owned tracked `git diff --check` | Pass, exit 0; no output | Explicit task-path command |
| New component regressions and this report | Pass, exit 0; no trailing whitespace and final newlines present | Explicit three-file read check |

The final test command was:

```sh
pnpm --dir packages/views exec vitest run collaboration/field-inputs.test.tsx collaboration/creation-form.test.tsx --maxWorkers=2
```

Root scripts exclude Mobile as configured by the repository. The final type
check includes Web and both Desktop node and renderer checks. No full test run
or static detector was repeated by this reviewer; main owns the combined suite
and detector, and lanes had already run their focused compatibility checks.

### Code-path and regression review

| Finding | Review conclusion |
| --- | --- |
| C01 | Custom single/multi-select filters configured choices at its existing owner. Total choice count keeps search mounted; typed no-match Enter cannot choose the fixed empty row or a hidden option. Selection, explicit clear, reopen/reset and definition-change tests exercise the actual primitive. |
| C02 | `toggle` is opt-in for reversible choices; `optionId` retains option/selected/disabled/tab-order semantics and `emptyValue` remains a command. Available labels toggle, unavailable selected labels remain one-way removal commands. |
| C03 | Text/number/URL inputs have visible property-name labels and instance-specific IDs. Trimming, decimal parsing/step/input mode, URL action and native form submission remain at the existing owner. |
| C04 | The reason container has a stable key and position across same-tuple absence; typed or focused client input keeps the native node enabled unless a real write lock applies. Current candidate grants alone render actions, absence/inactivity blocks submission, denial removes the decision, and selecting another candidate immediately remounts the review. No body, quoted server reason or capability was copied to client state. |
| C05 | Only the local context confirmation changed: viewport-bound flex popup, nonshrinking title/footer, scrolling middle and bounded textarea growth. Existing reason, purge permission and snapshot-pinned restore guards remain. Semantic tests retain/send the exact legal reason and require explicit confirmation/revalidation. Painted geometry is supplied by main's compiled-CSS scenarios. |
| C06 | The existing immutable schema supplies stage error paths; catalog failures stay distinct. An invalid saved stage without a parent exposes its recovery field; correction/blank restores normal eligibility without discarding the draft. The mutable draft schema is unchanged. |
| C07 | Retry calls the existing status-options `refetch()` in the same form; loading is visible and the existing success/current-option/not-fetching guard still blocks submit. No retry creates an action request. |
| C08 | `active` reaches memory history/selected-version, selected knowledge review and retained proposal discovery observers, including the delivery observer sharing the candidate key. Disabled readers retain mounted editors and source selections; lifecycle/lease timers and required root status reads remain. No hide effect cancels another visible owner. |
| C09 | Floors and wrapping are local to the shared picker rows/search and text-value/URL actions. Fine-pointer density remains intact. Main's real compiled geometry supplies 44px/narrow/theme evidence; component assertions do not stand in for painted geometry. |

The reviewer read the actual passing lane logs: Lane A 163 tests plus 45 shared
picker compatibility tests, Lane B 62 Views tests plus 25 canonical Core tests,
and Lane C 48 component/compatibility tests. These are lane-performed checks,
not additional executions by this reviewer. Their commands and red/green
evidence remain in the lane progress files and referenced logs.

The source-picker signature recommendation was completed by main. Current
creation/source-picker, knowledge/lease and detail specs now describe the
optional stage error, no-parent recovery, client editor continuity, optional
candidate enabled argument and hidden-reader/root-state boundary. No platform
configuration/template touch point is implicated by this task.
