# Desktop core audit remediation — 2026-10-09

User approved rechecking and fixing every P1/P2 finding in `docs/audits/2026-10-09-desktop-core-impeccable.md`. No Trellis task: the user's earlier preference continues. No new dependencies, broad redesign, server changes, commits or unrelated cleanup.

## Change boundary

The behavior gap is in shared page interaction/accessibility and desktop shell wiring. Preserve product styling, route/query semantics, cached content, drafts, all task dates and rows, and existing working-copy edits. Use current source and a failing regression to confirm a finding before modifying its implementation. Old audit screenshots are evidence of the earlier revision, not a substitute for rechecking.

Expected files are the audited components and colocated tests: chat window/fab and focus hooks, inbox page, projects page/detail, issue list row/list view and Gantt, triage pagination, agent list/detail, skill detail, desktop tab bar/layout. Locale changes belong only to the corresponding English/Chinese namespaces. Shared UI/core helpers are changed only when an existing primitive's contract requires it, with the change explicitly recorded.

## Acceptance checklist

- [ ] P1-01: Closed chat content is inert and absent from the accessibility tree; open/close focus behavior works without losing drafts or stealing unrelated focus.
- [ ] P1-02: Inbox/project cold-query errors show retry instead of an empty or missing result. Refetch failure retains cached data and drafts. Confirm genuine 404 and permission behavior.
- [ ] P1-03: Task/agent/project row and group selections have descriptive names, keyboard access, visible focus, correct selected/indeterminate states and no accidental row navigation.
- [ ] P1-04: Project title is a real shared AppLink; keyboard, ordinary click and modifier click open the correct destination once.
- [ ] P1-05: All Gantt status-bar text/background combinations meet 4.5:1 in light/dark themes, including neutral/transparent statuses; retain status meaning.
- [ ] P1-06: Desktop tabs can move left/right without dragging, by keyboard and pointer; pinned boundaries and persistence remain valid.
- [ ] P2-07: Enabled triage pagination does not overlap the chat launcher at 900px and enlarged text; use existing clearance token.
- [ ] P2-08: Desktop exposes the current tab; agent and skill detail tabs implement roving focus, arrows/Home/End and associated panels while preserving URL/deep-link/editor behavior.
- [ ] P2-09: Reduced-motion preference stops desktop layout interpolation without breaking normal motion or final geometry.
- [ ] P2-10: Gantt axis/grid and rows render bounded visible work for long spans/large lists. Dates/tasks remain accessible by scrolling; sorting, today alignment and zoom still work.

## Ownership

1. Recovery and project access: chat/inbox/projects components, tests and their locales.
2. Task selection and Gantt: issue list/Gantt, triage pagination, their tests/locales and narrowly required Gantt tokens.
3. Desktop and resource navigation: tab bar/layout, agent list/detail, skill detail, their tests/locales.
4. Root: integration, browser/electron verification, independent review, evidence and final report.

Implementers are not alone in this worktree. Do not revert other edits or recursively delegate. Recheck each assigned issue first; report an invalidated finding instead of applying an obsolete fix.

## Verification

Run targeted regressions while implementing; use real Base UI controls for keyboard contracts. Validate affected packages with lint/typecheck and meaningful tests, then inspect the integrated diff independently. Reuse the isolated Electron audit harness with fresh profile copy, real local API and business writes blocked; fault injection is explicit. Capture one batched final view/interaction pass and at most one corrective confirmation pass. Check light/dark contrast, 900px/200% layout, keyboard selection and focus, error/retry and cached failure, and large-span/large-row Gantt behavior. Retain evidence and report material limits without claiming unrun checks.

## Reuse/simplification

Prefer existing Checkbox, AppLink, Tabs, error-state components and installed virtualization utilities. Replace mouse-only/nested selection controls with a single semantic control. Do not introduce compatibility shims, generic frameworks, new product features or data truncation to satisfy a UI test.
