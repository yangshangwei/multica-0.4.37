# Review and implementation evidence

## User decision

Initial request: review the supplied triage history screenshot with Impeccable and suggest timeline improvements. The independent review recommended a compact date-grouped timeline and visible filters. The user then explicitly chose time-based tracing and the timeline + filter-feedback scope, and requested task creation and progress. This is the accepted scope; do not reopen side-panel/table choices.

Prior review archive: `.impeccable/critique/2026-10-09T16-13-58Z__packages-views-triage-triage-history-tsx.md`.
Reference screenshot: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/codex-clipboard-4amCZV.png`.

## Current source map

- `packages/views/triage/triage-page.tsx`: history query at ~126, toolbar at ~678, global history and title navigation at ~734; Pagination at ~875. `useTriageRoute` is already the authority for latest intended query values.
- `packages/views/triage/triage-history.tsx`: shared event formatting and differences; global record use plus single-item history, execution status and duplicate reference components. Keep those other consumers stable.
- `packages/views/triage/triage-filters.tsx`: existing history fields and date inputs; no new filter capability needed.
- `packages/views/triage/use-triage-route.ts`: serializes asynchronous platform acknowledgements while retaining newer input; clear through this hook.
- `packages/core/types/triage.ts`: global entries already have action, time, actor, snapshots and CSV counts. Only per-item actions have round/execution status.
- `packages/views/common/format-in-time-zone.ts`: existing safe formatter supports locale and explicit/implicit timezone; invalid input returns the original string.
- `packages/views/triage/triage-page.test.tsx`: existing deep-link, queue, delayed-navigation and pagination contracts. Current useT mock omits interpolation; update test support meaningfully if new translated summaries require it.
- `packages/views/triage/triage-ui.test.ts`: canonical snapshot-change tests; avoid copying that matrix into DOM tests.

## Design evidence

Independent assessment A covered reading order, groups, density, narrow wrapping and keyboard semantics before detector findings. Independent B ran the full detector on history/page/filters once and returned `[]` with exit 0. Thus the prior full scan also supplies pre-edit mechanical evidence for the narrower layout work; a final changed-target scan remains required.

Use neutral tokens, real Lucide icons, localized absolute day headings and compact event metadata. Metadata is subordinate; the event action and task/file identity lead. Do not display arrows/glyphs as the product icon system.

## Environment and safety

The critique's CUA attempt failed with `CUA_REPL_ENABLED_SURFACES is required`. No live surface was inspected. This checkout's environment ledger then showed services stopped. Use an actual rendered component harness or an available matching dev environment for visual proof, and clearly identify fixture data. Do not edit user tabs or start authenticated/paid agent executions.

At task start, unrelated edits existed in iteration core/views and `apps/web/next-env.d.ts`, plus another Trellis iteration task. Preserve them. Specialist model presets previously failed at the provider; Trellis roles have commented-out model overrides and can inherit the working parent model.
