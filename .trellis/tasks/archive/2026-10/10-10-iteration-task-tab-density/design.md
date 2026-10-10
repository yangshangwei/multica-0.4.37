# Technical design

## Status and ownership

Implemented design based on the four-point review and subsequent explicit implementation approval. One coherent shared-view change owns the task toolbar, phase context and row presentation; no backend lane is needed.

Production scope:
- `packages/views/iterations/iteration-page.tsx`: existing phase and whole-period summary.
- `packages/views/iterations/iteration-issue-list.tsx`: controls, summaries, result states and rows.
- `packages/views/locales/en/projects.json` and `packages/views/locales/zh-Hans/projects.json`: localized copy.

Reuse existing iteration presentation helpers, historical category labels and shared primitives. Add an iteration-local component only when it simplifies the existing code. No generic filter framework, new store or dependency.

## Layout

Keep the header, main action and three tabs. Planned content is: one compact planning row, a toolbar, conditional filter summaries/result feedback, then tasks.

Search should be approximately 240-320 CSS px on desktop and adapt to content-container width. Wrap controls and metadata in narrow panels. Do not gain density by shrinking typography below tokens, touch targets below 44px, or hiding long titles.

Use an existing accessible popup/entry pattern for the five filters, retaining portal, keyboard and focus-return behavior. Keep grouping and applied summaries outside it.

## Phase and query flow

Reuse the phase already computed with `iterationScopePhase`; pass that phase or a minimally derived presentation prop to the list. Do not duplicate lifecycle inference.

| Phase | Presentation |
| --- | --- |
| Planned / cancelled before start | Planned source; no original-commitment switch |
| Active, including zero baseline | Current/original switch |
| Completed / cancelled after start | Existing frozen sources, labels and scope semantics |
| Unknown | Neutral handling; no fabricated start or commitment |

Retain paginated `iterationIssuesOptions`; use `iterationGroupedIssuesOptions` only for explicit grouping. Filter opening must not traverse all pages. Choices continue to use complete selected-source response metadata.

Whole-period counters come from detail/snapshot statistics; matches come from the list query. Avoid a matches/total fraction when the matching unfiltered selected-source total is not established.

## Applied conditions

Derive counts and summaries from existing state, without parallel state. Assignee type plus ID represent one condition. Search stays visible in its input; scope and grouping remain explicit controls.

Removing a chip removes only its field and resets pagination. Reuse the existing full reset contract in both non-empty and empty states: clear search and field filters, restore current scope, reset cursors, retain grouping. Copy must accurately describe that reset.

Retain missing-selection and unavailable-metadata handling. Do not replace saved historical names with today's entity names. Preserve mounted tab state and workspace/iteration identity boundaries.

## Task rows

Title is primary, identifier secondary; status and actor have stable positions. Blocked status receives semantic emphasis plus text or an appropriate icon.

Build meaningful metadata segments instead of unconditional separators. Null actor identity can mean unassigned; non-null identity with missing saved name remains unknown. Historical categories continue using fixed historical labels.

Keep positive rollovers and the existing review threshold; do not change data, status, navigation or frozen/current comparison behavior.

## Trade-offs, risks and rollback

Search stays visible even for small lists to preserve discovery. Move explanatory text beside its subject without removing its meaning. Base responsiveness on panel space rather than assuming viewport width equals content width.

Popup focus, scope availability, reset semantics, historical names and overflow require real interaction checks. Pure spacing changes use visual evidence rather than CSS-string tests.

No API, migration or dependency changes. Rollback only task-owned view, locale and regression-test hunks; no data cleanup. Preserve existing unrelated work in this checkout.
