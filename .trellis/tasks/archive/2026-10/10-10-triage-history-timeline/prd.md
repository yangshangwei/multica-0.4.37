# Triage history timeline and applied-filter feedback

## Goal

Help Web and Desktop users trace who did what, when, and why across triage history without repeatedly matching distant timestamps and fragmented metadata.

## Approved direction

After the Impeccable review and its concrete timeline example, the user selected “按时间追溯经过” and “时间轴＋筛选反馈”, then explicitly requested “请新建trellis任务 推进”. This task implements that selected first increment. The preceding review is the scope/design decision record; no additional product decision is pending.

## Background

- Global history currently renders a flat list with full timestamps at the right edge and separate title/action/actor/source rows (`packages/views/triage/triage-page.tsx:734`, `triage-history.tsx:148`).
- Search, source, result, processor, and processed-date bounds already exist. The filter panel can be collapsed without an applied-condition summary (`triage-filters.tsx:80`).
- The page uses one empty message for first-use history and a filtered query with no matches. URL updates already preserve the user's latest intent through delayed platform navigation.
- The reviewer and deterministic assessments were independent. The three-file detector returned no rule findings; visual hierarchy and traceability remain the diagnosed gaps.

## Requirements

### R1 — Chronological reading

Present global history in date groups with a compact timeline. Keep the server's event order and every distinct record, including equal timestamps. Date headings, adjacent time labels, action/task identity, actor/source metadata, reasons, and expandable field changes form a consistent reading order.

### R2 — Honest event semantics

Each node describes the historical event, not the task's current state. CSV imports foreground the filename and the existing created/skipped/failed counts without a repeated action heading. Keep full timestamps accessible, retain immutable snapshot differences, and keep existing task/import navigation. Do not invent batch identity or day totals from a paginated result.

### R3 — Applied-filter feedback

Show the active history search/filter conditions even while the filter editor is collapsed, using readable localized names and a clear-all action. Feedback derives from the current intended query. Clearing removes only the history search/filter fields and resets pagination, preserving the history view and unrelated navigation context.

### R4 — Empty results

Distinguish genuinely empty history from no matches under active conditions. A no-match state offers a direct way to clear those conditions. When a page is empty but the server reports matching records (`total > 0`), explain that this page has no records and offer a return to the first page while preserving filters; do not claim history or matches are absent.

### R5 — Shared accessible presentation

Use shared Web/Desktop components, existing design tokens and icon primitives, and English/Simplified Chinese messages. Keep semantic date headings, lists, time elements, keyboard-operable disclosures, readable long content, and narrow-screen wrapping.

## Acceptance criteria

- [x] AC1 / R1: Multiple events on one display-calendar date share a heading; a date boundary creates another group; events retain incoming order and identity.
- [x] AC2 / R1–R2: Times are adjacent to events; full date/time including the display timezone remains available accessibly. Invalid timestamps cannot crash the history screen.
- [x] AC3 / R2: Task/import actions still open their existing destinations; reasons and before/after disclosures retain their data and keyboard behavior. Single-item execution/retry history does not regress.
- [x] AC4 / R3: Every effective history condition is visible in a summary when the editor is closed. Clearing resets the history fields and offset while preserving view/unrelated parameters, including through delayed route acknowledgements.
- [x] AC5 / R4: Unfiltered empty history and filtered no-match history have different messages; the latter provides clear filters. An empty out-of-range page with a positive total offers first-page recovery without clearing conditions.
- [x] AC6 / R5: English and Simplified Chinese, wide and narrow layouts, and long titles/reasons/filter values remain readable with no horizontal overflow caused by the new UI.
- [x] AC7: Relevant behavior tests, localization parity, views typecheck, scoped lint/static checks and a rendered visual review have recorded results.

## Out of scope

Side-panel task details, a table/timeline switch, cross-task batch grouping, per-task round redesign, new filters or filter-date semantics, backend/API/database changes, mobile-native UI, broad error-recovery work, and changes to other active iteration tasks.

## Constraints

No new dependencies. Preserve current query/pagination and permission semantics. Group and display event timestamps on the same browser-local clock as the existing history; do not reinterpret date-filter values. Application code remains in the shared views package. Unrelated pre-existing edits must remain intact.
