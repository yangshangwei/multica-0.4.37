# Shared Views check — 2026-11-03

## Boundary and local fix plan

The added execution-history discovery query starts even when current channels,
targets, commands, groups or runs already establish AI activity. Its propagated
denial also repeats an alert already owned by an open transcript. The fix belongs
in `packages/views/collaboration/execution-panel.tsx`: query history only when
the execution snapshot does not establish that activity, and show its discovery
error only in that case. Adjacent execution/history suites will route mock
responses by endpoint, assert transcript/handoff laziness, exercise the outer
disclosure, and retain exact cancellation/original-input assertions.

The review covers task-owned collaboration Views and the two collaboration
locales. Core, Server, e2e, specs, dependencies and public interface/design
decisions remain with their owners. This reviewer does not commit or launch
browser/services. The root owns final visual captures and the design detector.

The root also requested the missing R12 current-viewer context next to actual
Participants. `participants.tsx` will show labelled current roles and relevant
capabilities. Acceptance/self-completion/return/submission readiness comes from
`deriveCollaborationDetailState`; no additional workflow model, directory query,
or participant API change is needed. Both locales and the existing happy-path
workflow tests will cover that wiring, plus one Chinese keyboard disclosure case.

The auxiliary audit found a concrete nested-disclosure gap: the attachment
delivery form opens its own disclosure for unfinished operations, but the new
outer file disclosure stays closed. Existing stored knowledge requests likewise
sit under the new closed ancestor. Local effects in those components will open
their actual ancestor details when recovery needs attention; no state, stores,
public props or automatic requests are added. Real remount tests will protect
the original keys and absence of replacement writes.

The root confirmed the user's retained R38 management completion exception and
updated PRD/design. Completed status plus a submitted result with no linked
decision will explicitly say that no initiator acceptance is recorded in both
locales, without inventing an administrator or reason. A direct DeliveryPanel
suite will also check exact prior-result/evidence wiring, legacy/unavailable
copy, and history absence only after pagination is exhausted.

## Baseline evidence

- Full native hook context and CLAUDE/task PRD/design/implementation artifacts read.
- Source snapshots of 67 assigned files retained at
  `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-views-check-4cw30nss`.
- `pnpm --filter @multica/views exec vitest run collaboration/execution-panel.test.tsx collaboration/command-history.test.tsx --maxWorkers=2`:
  8 passed, 4 failed. Failures reproduce the unnecessary `/commands` discovery,
  duplicate denial alert, request-indexed cancellation mock and old
  all-history-network-lazy assertion.
- After the execution fix, both focused suites pass all 15 tests, including
  three retained/empty/unavailable history discovery regressions. The three new
  discovery cases also passed before the optimization.
- Impeccable context and craft-floor read for the participant refinement. Its
  context command permits scoped fixes to existing code without PRODUCT.md;
  the root retains ownership of the one final detector/capture pass.

## Findings (fixed)

- **File:** `packages/views/collaboration/execution-panel.tsx` and its adjacent
  execution/history tests.
  **Issue:** unconditional command-history discovery ran alongside already
  established execution facts and repeated the transcript's propagated denial.
  Ordered cancellation mocks also consumed unrelated discovery responses.
  **Fix:** discover history only for empty/unknown execution snapshots, and
  render its discovery error only in that case. Tests route by endpoint, prove
  transcript/handoff/body laziness through the outer disclosure, and preserve
  cancellation's original key, author/content/file retention and no-Stop checks.
  Three new regressions retain historical inputs behind an empty manual snapshot,
  omit an actually empty manual console, and keep unavailable discovery visible.

- **File:** `packages/views/collaboration/participants.tsx`, both collaboration
  locales and `workflow.test.tsx`.
  **Issue:** the new current-permissions label appeared only in comment-author
  disclosures, leaving the actual Participants control without viewer context.
  **Fix:** add compact labelled current role/permission context beside that
  control. Read/discussion use current capabilities; editing, information return
  and submission use the Core-derived action set; exact acceptance and self-result
  completion use `canAccept`/`canSelfComplete`. Preserve the existing lazy
  participant list, names/creator/current-handler/external roles and confirmed
  removal. Existing happy paths assert this context; the new Chinese case opens
  Participants by keyboard and checks only its scoped read occurs.

- **File:** `packages/views/collaboration/attachment-panel.tsx`,
  `knowledge-panel.tsx` and their tests.
  **Issue:** the new outer auxiliary disclosures hid saved recovery controls:
  opening the attachment form's own details did not open its ancestor, and stored
  knowledge requests remained behind a closed ancestor after remount.
  **Fix:** reuse the existing anchor-opening pattern in local attention effects
  to open the actual ancestor details. No new state/store/public props or
  automatic operation is introduced. Two saved upload/delivery cases preserve
  the original request and prove no write; the existing knowledge remount case
  now includes the collapsed outer disclosure and still proves original-key GET
  recovery with exactly one original approval POST.

- **File:** `packages/views/collaboration/delivery-panel.tsx` and the new
  `delivery-panel.test.tsx`.
  **Issue:** a current-only first history page claimed there were no earlier
  results even when a next page existed. The new panel's actual history/evidence
  wiring also lacked a direct Views test.
  **Fix:** show absence only after pagination is exhausted. The new suite covers
  exact current/earlier submission identities, frozen result/basis/author/time,
  revision decision/reason, selected protected evidence, source-comment anchor,
  endpoint-lazy history, read retry and legacy unavailable facts.

- **File:** `packages/views/collaboration/delivery-panel.tsx`, both collaboration
  locales and its new suite.
  **Issue:** the user-retained R38 administrative completion exception can leave
  completed status plus a result without a linked initiator decision.
  **Fix:** display “No initiator acceptance is recorded for this result.” /
  “此结果未记录发起人验收。” in that factual case. Tests cover both locales and
  withhold an Accepted label. No administrator, decision actor or reason is
  inferred from status; the administrative production boundary was not edited.

## Findings (not fixed)

No remaining task-local product-code issues were found after the fixes and
successful lint/type-check/test gate.

Full Views lint reports 27 warnings in existing files outside collaboration.
They are outside this lane's ownership and were left unchanged. All paths below
are relative to `packages/views/`:

| File | Lines | Warnings |
| --- | --- | --- |
| `agents/agents-i18n-parity.test.ts` | 150, 166 | 2 unused disables |
| `agents/components/tabs/mcp-config-tab.tsx` | 112 | 2 hook dependencies |
| `attachments/use-html-preview-scroll-restore.ts` | 211 | 1 hook dependency |
| `common/single-row-fit.ts` | 64 | 1 unused disable |
| `editor/zoom-canvas.tsx` | 161 | 1 hook dependency |
| `issues/components/execution-log-section.tsx` | 250 | 1 hook dependency |
| `issues/components/issue-usage-dialog.tsx` | 72, 85, 231 | 3 hook dependencies |
| `issues/components/swimlane-view.tsx` | 964 | 1 hook dependency |
| `issues/components/table-view.tsx` | 1848 | 1 hook dependency |
| `issues/surface/use-issue-group-branches.ts` | 331 | 1 hook dependency |
| `onboarding/onboarding-flow.tsx` | 235 | 1 hook dependency |
| `projects/components/project-detail.tsx` | 153, 261 | 2 hook dependencies |
| `runtimes/components/charts/activity-heatmap.tsx` | 204 | 1 hook dependency |
| `runtimes/components/usage-section.tsx` | 373, 383, 699, 703 | 4 hook dependencies |
| `runtimes/utils.test.ts` | 1079, 1089, 1101, 1123, 1133 | 5 unused disables |

Spec synchronization remains with the root because this lane was explicitly
excluded from `.trellis/spec/` writes. Recommend recording facts-first command
history discovery/lazy bodies in
`.trellis/spec/core/frontend/collaboration-workbench.md`, and recovery opening
all enclosing auxiliary disclosures in
`.trellis/spec/views/frontend/collaboration-knowledge.md`. Completed status
without a linked decision must remain distinct from recorded initiator acceptance
when documenting the retained R38 exception.

Production Web/Electron journeys, narrow/intermediate/wide screenshots, actual
scroll/focus acceptance and the one Impeccable detector pass belong to the root.
This component check does not claim their runtime acceptance.

## Verification

| Check | Command | Actual result |
| --- | --- | --- |
| Focused execution/history | `pnpm --filter @multica/views exec vitest run collaboration/execution-panel.test.tsx collaboration/command-history.test.tsx --maxWorkers=2` | 15 passed |
| Workflow/current viewer | `pnpm --filter @multica/views exec vitest run collaboration/workflow.test.tsx --maxWorkers=2` | 11 passed |
| Result/auxiliary recovery | `pnpm --filter @multica/views exec vitest run collaboration/delivery-panel.test.tsx collaboration/attachment-panel.test.tsx collaboration/knowledge-panel.test.tsx --maxWorkers=2` | 43 passed |
| Final collaboration + locale parity | `pnpm --filter @multica/views exec vitest run collaboration locales/parity.test.ts --maxWorkers=2` | **281 passed, 30 files**, exit 0; start 09:52:49, 56.93 s |
| Final type-check | `pnpm --filter @multica/views typecheck` | **Pass**, exit 0 |
| Final lint | `pnpm --filter @multica/views lint` | **Pass**, exit 0, zero errors; 27 unrelated warnings above |
| Scoped whitespace | `git diff --check -- packages/views/collaboration packages/views/locales/en/collaboration.json packages/views/locales/zh-Hans/collaboration.json` | **Pass**, exit 0 |

The final gate ran after the ancestor-opening, history pagination and completed
decision copy changes. Red checks first demonstrated the two ancestor attachment
cases, the knowledge remount case and premature empty-history message. The
completion fixture's required `has_history` flag was corrected before separately
confirming both missing-copy failures, then all three focused files passed.
Tests use `DEBUG_PRINT_LIMIT` only to bound failure diagnostics.

## Exact checker-owned file changes and preservation

The following paths contain this check lane's local delta relative to its entry
snapshot. This list does not claim ownership of every pre-existing hunk in them:

- `packages/views/collaboration/execution-panel.tsx`
- `packages/views/collaboration/execution-panel.test.tsx`
- `packages/views/collaboration/command-history.test.tsx`
- `packages/views/collaboration/participants.tsx`
- `packages/views/collaboration/workflow.test.tsx`
- `packages/views/collaboration/attachment-panel.tsx`
- `packages/views/collaboration/attachment-panel.test.tsx`
- `packages/views/collaboration/knowledge-panel.tsx`
- `packages/views/collaboration/knowledge-panel.test.tsx`
- `packages/views/collaboration/delivery-panel.tsx`
- `packages/views/collaboration/delivery-panel.test.tsx` (new test file)
- `packages/views/locales/en/collaboration.json`
- `packages/views/locales/zh-Hans/collaboration.json`
- This report, `.trellis/tasks/11-02-collaboration-detail-optimization/research/views-check.md`.

The entry snapshot audit shows 55 of the 67 existing assigned files unchanged;
12 existing files differ and only `delivery-panel.test.tsx` is new. Each locale
has exactly six added keys and zero changed/deleted original keys:
`delivery.noAcceptanceRecorded`, `participants.yourRole`, `participants.participant`,
`participants.reader`, `participants.view`, `participants.discuss`.
The root's pre-existing `data-submission-id` remains on result records.

The three source-picker paths raised by the root were already dirty when this
check lane entered and were never edited here. Their final SHA-256 values equal
their entry values; earlier authorship cannot be attributed to shared_views versus
the concurrent source-picker session from this dispatch context:

| Path under `packages/views/collaboration/` | Equal entry/final SHA-256 |
| --- | --- |
| `field-inputs.tsx` | `823c7a6d1898eea6b85f39ff0b103fdec32b8858d70f85200c3f811049967987` |
| `field-inputs.test.tsx` | `3c1398807c0ee3e4f6a9d48a553552038370803a6a6f49527d827fe348661794` |
| `field-editing.test.tsx` | `be59331b9060dda80bd0376c02a41ad76a05aec6bbfb1de92835bcf98d744674` |

No Core, Server, e2e, spec, dependency, commit or service/browser launch was
performed by this checker. UI source is ready for production compilation. There
were no production edits after the final readiness message; the subsequent
Desktop build handle 58139 does not need a rebuild for this lane's report-only
completion.
