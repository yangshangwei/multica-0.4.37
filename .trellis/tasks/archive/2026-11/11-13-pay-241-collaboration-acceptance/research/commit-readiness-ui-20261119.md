# UI commit readiness outside S2 — 2026-11-19

**Disposition: exclude the 23 quick-create paths and the live core UI audit
paths from this ready-only batch.** No additional uncommitted UI group in this
bounded review has complete final acceptance evidence. E2E1118 fixes #37/#38
are already accepted, committed and archived; they do not need another commit.
Main may proceed with its independently accepted S2 group and existing ready
commits without expanding this request into unfinished UI implementation.

Exact current/HEAD/snapshot file hashes, locale keys and excluded paths are in
[commit-readiness-ui-20261119.json](commit-readiness-ui-20261119.json), SHA-256
`395fe7a04975724ca01309a95390a8b4e55bc7dc7aa7c13f7972bbaf3ffd2457`.
This reviewer performed no test, DB/API, source edit, staging, commit or push.

## Quick-create ownership and readiness

The task `11-17-quick-create-blocked-feedback` remains `in_progress`, with all
acceptance checkboxes open and no task-local execution/independent final
verification report. The 21 code/test/locale paths below exactly match the
frozen 2026-11-18 E2E source; the two specs are also task-scoped but absent from
that source snapshot. Stable bytes establish ownership, not completed ACs.

The complete coherent group is:

```text
packages/core/package.json
packages/core/issues/quick-create-submit-state.ts
packages/core/issues/quick-create-submit-state.test.ts
packages/views/modals/creator-recommendations.tsx
packages/views/modals/creator-recommendations.test.tsx
packages/views/modals/issue-description-assist.tsx
packages/views/modals/issue-description-assist.test.tsx
packages/views/modals/quick-create-actor-picker.tsx
packages/views/modals/quick-create-actor-picker.test.tsx
packages/views/modals/quick-create-issue.tsx
packages/views/modals/quick-create-issue.test.tsx
packages/views/chat/components/chat-new-chat-button.test.tsx
packages/views/locales/en/modals.json
packages/views/locales/zh-Hans/modals.json
e2e/chat-actor-picker.spec.ts
e2e/core-business-agent-loops.spec.ts
e2e/issue-assist-flow.spec.ts
e2e/quick-create-actor-picker-phase2.spec.ts
e2e/quick-create-actor-picker.spec.ts
e2e/quick-create-autonomy.spec.ts
e2e/upstream-selected-fixes.spec.ts
.trellis/spec/views/frontend/issue-description-assist.md
.trellis/spec/views/frontend/quick-create-actor-picker.md
```

`packages/core/package.json` adds only the
`./issues/quick-create-submit-state` export. It must travel with its new helper,
test and the panel import; do not copy that export alone into a clean S2
candidate. No dependency addition is involved.

Both `modals.json` diffs are entirely within four branches:
`create_issue.agent`, `create_issue.ai_optimize`,
`create_issue.actor_picker`, and `create_issue.creator_recommendations`.
English has 24 changed leaf keys; Chinese has 33. Other modal branches are
unchanged. These are separate from S2's `collaboration.json` stage refusal
keys and core UI audit's `issues.json`/`squads.json` edits. Whole modal files
would be appropriate only when the quick-create group itself is ready.

The retained 11/18 raw reports provide partial actual evidence on those exact
code bytes:

| File | Recorded cases |
| --- | --- |
| chat actor picker | 3 PASS |
| quick-create phase 2 | 4 PASS |
| quick-create autonomy | 3 PASS |
| upstream selected fixes | 5 PASS |
| issue assist flow | 7 PASS / 1 setup timeout |
| quick-create actor picker | 7 PASS / 1 FAIL, original #51 |
| core business agent loops | 3 PASS / 1 FAIL, separate G6 payload behavior |
| settings preferences | 1 PASS / 1 FAIL, original #52 |

The aggregate files themselves contain other failures and skips; these counts
are only the selected original cases, not a whole-run acceptance. The current
#51 fixture still samples `textContent()` before directory/default selection
settles. The failure occurs before the first favorite action and second
workspace; it does not establish cross-workspace contamination. The current
#52 clean file still uses a full-page `Switch to Manual` locator that matches
two valid controls. Neither original case has a corrected complete acceptance
in the supplied records.

No new submit-state/quick-create owning-unit execution receipt or full
blocked-focus/shortcut/visual acceptance was located in the bounded task,
journal and supplied artifact review. The future task needs its helper matrix,
related Views/cache/assist/manual/collaboration/chat/dialog and parity checks,
plus actual focus/draft/shortcut/portal/IME/narrow-theme checks and #51/#52
corrections. Those checks are recommendations for that task; **do not start
them as an expansion of the present ready-only commit request**.

## E2E1118 already-ready work

The retained joint verification summary accepts #37/#38 only: 14 production
Web cases and 6 real Electron cases, zero retries, original-file/source
bindings retained. Their work commits are already ancestors of HEAD:

- #37: `22e0b202f60255f5dc9030139454ec87e4606270`.
- #38: `da19f01ddc722396327cda94227cb9a6d7036cc7`.

Both child tasks are archived and the first-batch remaining list is empty.
Current dirty `issue-detail`/picker test changes must not be swept into those
old fixes: they belong to the newer core UI audit. The seven dirty E2E files
listed in the quick-create group change its wording/selectors and are not a
new accepted E2E1118 repair batch.

## Core UI audit is still active

`11-19-core-ui-audit-remediation` is `in_progress`. Its evidence and context
manifest changed during this review at 14:11–14:13 UTC, and its verification
ledger has no final independent acceptance. A partial native check reports
F02/F04/F05/F07 outcomes, but is not the task's full acceptance. Do not treat
the presence of individual lint/typecheck/native logs as a finished task.

Exclude all **49** paths in its `changed-files.json` and the additionally dirty
`packages/ui/components/ui/sheet.tsx` (50 exact paths/hashes in companion JSON).
These include Web contrast CSS/tests, Core squad queries, shared Button and
popup primitives/tokens, issue pickers/editors/detail tests, sidebar/layout,
project and squad pages, and the `issues.json`/`squads.json` locales.

There is no direct path intersection with the 23 quick-create files, but
Button/Tooltip/Popover/Dialog/PropertyPicker/token dependencies are shared.
Current working-tree validation can consume this ongoing dependency graph;
it cannot be labelled validation of an isolated HEAD-plus-S2 commit. This
exclusion preserves the other session's work and does not ask it to stop.

## Clean S2 candidate guidance

Use HEAD plus the accepted S2 delta, excluding every path above. Retain HEAD's
Core export map when omitting the quick-create helper/panel changes. Retain
only the reviewed S2 `collaboration.json` stage-key delta, not modal or audit
locale branches. Validate that exact isolated candidate and its package links;
do not import unfinished quick-create or core UI dependencies merely because
they existed in an earlier dirty validation tree.

No new uncommitted UI commit is cleared by this report. Existing accepted
#37/#38 commits can remain in the pending push set, and Main owns S2 validation,
its directed commit, and the user-authorized push.
