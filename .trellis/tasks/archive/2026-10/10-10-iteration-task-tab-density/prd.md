# Iteration task tab: compact controls and readable task rows

## Goal

Make tasks the primary content of the iteration detail Tasks tab by reducing the space occupied by statistics, explanations and controls, while keeping search and refinement discoverable.

## Request and evidence

The user requested an impeccable review of the supplied screenshot, then explicitly requested a Trellis task for the four proposed improvements. The review identified four P2 usability issues and no confirmed P0/P1 blocker. In the 2048 x 1088 reference image, the first task starts near y=790, about 500 image pixels below the tab divider.

See [review evidence](research/review-findings.md), [reference screenshot](research/task-tab-before.png), and [full review](../../../../../.impeccable/critique/2026-10-10T11-37-46Z__packages-views-iterations-iteration-page-tsx.md). Implementation and verification are complete; see [verification](verification.md).

## Requirements

### R1 — Compact, discoverable controls [P2]

Keep search visible regardless of task count. Place bounded-width search, filters and grouping in a compact toolbar; wrap with available panel space. Replace the full-width bordered filter disclosure with a lightweight entry point. Show the grouping mode outside the filter panel.

Evidence: `packages/views/iterations/iteration-issue-list.tsx:140,147`.

### R2 — Compact, phase-correct context [P2]

Remove the repeated Tasks heading within the Tasks tab. Combine planned count and commitment timing into one concise row. Avoid a separate unfiltered matching-count row; provide result feedback when search or filtering is active. Keep statistical explanations available near their subject.

Offer current/original comparison only when actual lifecycle facts establish that the iteration started. Planned and cancelled-before-start periods have no formed original commitment; a started iteration with an empty original commitment is valid.

Whole-period statistics remain independent of list refinements. Preserve frozen wording and values. Never use current-scope totals as denominators for original-scope matches.

Evidence: `iteration-page.tsx:125-129`, `iteration-issue-list.tsx:108,138-139,161`, and the phase-aware scope contract in `.trellis/spec/core/frontend/iteration-operations.md`.

### R3 — Visible applied conditions and recovery [P2]

Show an active-condition count and removable summaries even after filters close. Provide the existing clear/reset behavior for both non-empty and empty results. Preserve grouping when clearing filters; removing one condition preserves the others.

Preserve selected missing/historical values, metadata-unavailable behavior and refinements across detail-tab switches. Query-affecting changes reset pagination.

Evidence: `packages/views/iterations/iteration-issue-list.tsx:147-158,172`.

### R4 — Readable task rows [P2]

Prioritize titles over identifiers. Give status and assignee a readable, stable place; distinguish blocked work with text and semantic emphasis. Omit empty metadata segments and orphan separators. Truly unassigned actors remain distinct from unknown historical identities.

Keep positive rollovers and the existing review threshold visible; zero rollover need not appear in the default row. Preserve historical category labels, long-title access, navigation and historical comparison.

Evidence: `packages/views/iterations/iteration-issue-list.tsx:164-170`.

## Acceptance criteria

- [x] AC1 / R1-R2: At the reference capture size, theme and zoom, the first task in the planned two-task example begins in the image's upper half. Search stays visible without spanning nearly all content width.
- [x] AC2 / R1: Search, all five existing filter types and six grouping modes remain available. Grouping is accessible without opening filters. A narrow desktop panel has no clipped controls.
- [x] AC3 / R2: Planned and cancelled-before-start states show planning semantics without original-scope selection. Actually started zero-baseline periods still allow scope comparison. Closed snapshots remain frozen; unknown phases are not guessed from counts.
- [x] AC4 / R2-R3: Search/scope/filter changes update matching tasks without changing whole-period counters. Result feedback has no incorrect denominator or confused scope.
- [x] AC5 / R3: Two applied field conditions remain identifiable after closing filters with non-empty results. Each can be removed independently. Visible reset restores the existing reset state and retains grouping; zero-result recovery still works.
- [x] AC6 / R3: Tab switches retain refinements; query-affecting changes reset cursor history. Opening filters alone does not fetch all task pages.
- [x] AC7 / R4: Blocked and todo work are distinguishable without color alone. Empty fields produce no stray separators. Unknown identities are not shown as unassigned; positive rollovers and review warnings remain visible.
- [x] AC8 / R1-R4: English and Simplified Chinese retain accessible names, keyboard navigation, popup focus return, visible focus and 44px coarse-pointer targets. Long titles and values remain accessible in narrow panels.
- [x] AC9 / R1-R4: Loading, permission loss, read errors, empty plans, zero matches, incomplete metadata and frozen-task comparison preserve existing recovery behavior.

## Out of scope

- Iteration overview, Progress / Planning adjustments content and workspace settings redesign.
- Server/API/database changes, statistics recalculation, new filter types, bulk editing, inline mutations, new routes or native mobile UI.
- Global design-system changes, new dependencies or page-specific changes to shared primitive defaults.

## Constraints and deferred verification

Use existing shared Web/Desktop components and tokens. Preserve lifecycle/query contracts and unrelated working-tree changes.

The review used screenshot and source evidence. Native UI access failed with `CUA_REPL_ENABLED_SURFACES is required`; service status showed ownership mismatch. Keyboard, responsive and contrast checks remain pending for an environment whose checkout ownership is verified.
