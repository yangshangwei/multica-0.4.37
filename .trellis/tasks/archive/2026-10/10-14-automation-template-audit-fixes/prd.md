# Automation template audit fixes

## Goal

Make automation template discovery clear and reliable with keyboard and touch, resolving A01–A07 from the approved impeccable audit.

## Approval and evidence

After receiving the concrete seven-point audit and recommended fixes, the user explicitly requested: “请根据修改建议 建个trellis任务 修改验证”. This authorizes creating this task and implementing/verifying that scope. No user-owned product decision remains.

Baseline: `.impeccable/audit/2026-10-14-automation/report.md` and adjacent screenshots/measurements. Scoped product files were clean at task creation; unrelated workspace changes must be preserved.

## Requirements and acceptance

| ID | Requirement and source | Observable acceptance |
| --- | --- | --- |
| A01 / P1 | Light card focus is 2.62:1; catalog line 202, shared ring token line 206. | Gallery cards/controls have visible keyboard focus >=3:1 in light/dark. Reuse semantic tokens without broad global styling changes. |
| A02 / P2 | Back preserves group/scroll but loses card focus; catalog line 194, template page line 118. | In-flow/native Back restore group, scroll and original card focus once content is ready. Removed targets fall back to the active filter, or blank action if the registry is empty. Refetches never reclaim focus. Confirmed creation clears the originating pending focus so a fresh visit does not restore a stale card; failed/unreadable creation preserves the return handoff. |
| A03 / P2 | Recommendation is a quiet footer row, inflating first-row cards to 222px versus 194px; catalog line 217. | Recommendation is recognizable near the title without its old dedicated footer row; preserve compact identity and template order. |
| A04 / P2 | Default-plan/timezone explanation absent; catalog line 221. | Before card selection, explain default schedules, timezone selection next, and schedule editing after creation; preserve the server-owned template contract. |
| A05 / P2 | “按需跟进” obscures task output; catalog line 183, locale line 474. | Chinese/English labels distinguish unconditional task creation from conditional follow-up truthfully; unknown modes remain conservative. |
| A06 / P2 | Executor/editability guidance is after all ten cards at y=1144; catalog line 238. | Concise prerequisite/editability guidance is above cards without a duplicate footer or excessive explanation blocks. |
| A07 / P2 | Blank creation becomes a 32px plus; list line 674, shared header line 143. | Empty-list and creation-gallery paths have a readable blank action with a >=44px hit height on narrow/coarse input, without competing duplicate empty-state actions. |

## Preserved behavior

- Classification, counts, backend order, unknown templates and loading/error/empty states remain correct.
- Selection only opens configuration. It does not create or enable an automation.
- Prompt, cron, output mode and provenance remain server-owned.
- Project/assignee defaults, return origin, explicit All and zero scroll round-trip.
- Populated automation management retains its current operations.
- Chinese/English keys remain in parity; long text and narrow layouts remain usable.

## Verification

Meaningful focus round-trip/fallback/delayed-data and blank-wiring regressions; existing template/list/locale tests; scoped lint and views typecheck; live wide/compact/narrow, light/dark, keyboard/touch and native zoom checks; before/after verdicts and measured contrast; one post-change impeccable scan; independent check-agent review.

## Out of scope

Global theme redesign, backend changes, independent Expo UI, dependencies, new discovery features, production deployment and real automation execution.
