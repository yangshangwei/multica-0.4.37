# Design

## Spatial thesis

The primary reading path is date → time → action and task → actor/source → reason → optional field changes. Date headings separate days; a quiet 1px rail and small Lucide action nodes connect events inside each group. The action and task belong on one wrapping primary line, with actor/source in a compact supporting line. CSV entries use the same timeline structure but their filename and outcome counts carry the event content.

Preserve the incumbent neutral product styling and role-named type scale. Use a readable constrained content width with a small time gutter instead of remote right-aligned timestamps. Narrow containers move the time into the event header or shrink the gutter while allowing long identifiers and names to wrap; preserve DOM reading order and practical touch targets. The rail is decorative and hidden from assistive technology. There is no decorative entrance animation.

## Boundaries and composition

All production changes belong to `packages/views/triage/` and its existing `en` / `zh-Hans` locale files. A triage-local timeline component is appropriate; do not introduce a generic timeline framework or business-aware primitive in `packages/ui`.

Keep `TriageHistoryRecord`'s existing single-item consumers and execution/retry behavior working. Reuse the snapshot-difference formatting and member/project lookups. If a small extraction is needed to share event content with the compact global layout, first protect its observable behavior and keep the extraction local. Do not duplicate the before/after formatting logic.

`TriagePage` retains data fetching, `useTriageRoute`, task selection, import opening, scroll restoration, and 50-row pagination. The new global timeline receives existing entries and destination callbacks. Existing routes and query factories stay unchanged.

## Dates and audit identity

Group adjacent entries by a stable display-calendar date using the same timezone as their displayed time. Preserve API order rather than sorting only the current page. Use a localized absolute date heading including the year, avoiding a new midnight refresh requirement for relative labels. Compact time is hour/minute; retain a full localized timestamp with timezone in accessible detail/label text, not a hover-only affordance.

The existing `common/format-in-time-zone.ts` is available; reuse it where it meets the display shape. Invalid values must have a safe localized fallback or retain readable raw evidence without an Intl exception. Keep the boundary logic independently testable with deterministic timezone inputs if a helper is extracted.

Do not combine equal-time events, infer a review batch from import provenance, or label the number of loaded entries as an all-day total. No current-state badge replaces a historical action.

## Filter feedback

Recognize exactly the existing global-history fields: `q`, `source`, `result`, `processed_by`, `processed_after`, `processed_before`. Derive summary and active state from the intended `params` returned by `useTriageRoute`. Resolve known result/source values with current translations and processor IDs with the existing member list; keep unknown/unavailable values intelligible. Render date-only filter values without converting their timezone or implying a changed server bound.

Use a wrapping semantic summary row with a clear-all button near the existing toolbar. Avoid a second independent filter state. Clearing uses one `changeParams` call removing those fields and offset; it preserves unrelated query keys and the selected history view. Reuse that same clear action for filtered no-match feedback. Queue-only filtering and intake tab behavior stay untouched.

Before claiming no matches or no history from an empty entries page, inspect the authoritative total. A positive total means an empty page, not an empty result set. Display page-specific copy with a first-page button that changes only offset. This local recovery corrects the independent review finding and satisfies R4's truthful feedback without adding a new filtering or API capability.

## Compatibility and scope

No server response or request changes, new dependency, schema migration, or platform router change. Both clients consume the shared view. Unknown event actions and malformed timestamps retain safe fallback presentation. English and Simplified Chinese remain in parity.

The user selected the first increment: no persistent split-pane details and no new retry workflow in this change. Existing item detail and CSV modal navigation are preserved.

## Verification and rollback

Pure date/filter transformations get canonical node tests only if introduced. Component tests cover semantic grouping, disclosure/navigation wiring, applied-filter clearing and empty-state differences. Preserve the existing single-item history, import, route-latency, selection, execution-status, and pagination tests.

Review real rendered components at wide and narrow sizes, English and Chinese, with long content. Native CUA was unavailable during critique, so a local Playwright/browser harness using actual shared components and existing dependencies is an acceptable fallback; document whether data is fixture data. Stop task-owned processes after captures. Run one batched inspection, fix its concrete findings, and at most one confirmation round. Persist visual-verdict evidence under the task's `.omx/state` scope.

Rollback consists of reverting this task's scoped frontend and translation changes; no persisted data requires migration.
