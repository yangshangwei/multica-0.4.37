# Iteration task-list presentation

## Scope and ownership

Use this contract when changing the iteration detail Tasks tab, its controls,
summary or task rows. The shared implementation is
`packages/views/iterations/iteration-page.tsx` and
`iteration-issue-list.tsx`; Web and Desktop consume the same view.

The source and historical rules in
[Iteration Operations](../../core/frontend/iteration-operations.md) remain
authoritative. This document owns presentation and interaction details, not a
second statistics or lifecycle model.

## Phase and statistics

`IterationIssueList({ wsId, id, historical, phase, emptyState })` receives the
existing `IterationScopePhase` computed by `iterationScopePhase` in its parent.
Do not infer a formed commitment from dates, a positive original count, or the
presence of a snapshot alone.

| Phase | Task scope control |
| --- | --- |
| planned / cancelledBeforeStart | Current planning source; no original switch |
| active, including an empty original baseline | Current/original switch |
| completed / cancelledAfterStart | Current/original sources with frozen labels |
| unknown | Neutral explanation; no original switch |

If a mounted view loses permission to present original scope, restore current
scope and reset pagination without clearing the user's search or field filters.
Switching detail tabs must retain refinements; changing workspace/iteration
identity must reset them.

Whole-period counters remain sourced from detail/snapshot statistics. Show
query matching count only with refinements and never replace those counters.
Do not construct a matching/total fraction using the current total for an
original-scope query. Keep statistical help keyboard-accessible near the summary.

## Controls and applied conditions

- Search stays visible, including empty plans, and has a bounded desktop width.
  Controls wrap according to available panel space; use shared type and spacing
  tokens and retain 44px coarse-pointer targets.
- Grouping is independent from filters and displays its selected mode.
- The filter button opens the shared named Popover. Select options are portalled:
  tests must locate the popup/options from the page rather than assuming they are
  descendants of the task panel. Escape closes the popup and returns focus.
- Derive active field count and removable conditions from the existing state.
  Assignee type plus ID is one field condition, not two.
- Removing one condition leaves the others intact and resets cursor history.
- The full reset contract is search empty, field filters empty, current scope,
  first page, and **unchanged grouping**. Expose the same reset in both non-empty
  and zero-match states; its accessible explanation must describe this behavior.
- Use response `filter_options`, preserving missing selections and unknown
  historical names. Opening filters must not fetch the complete issue set.
  The complete grouped query remains reserved for explicit grouping.

## Task rows

Title is the primary task link/comparison trigger; identifier remains separate
visible metadata. Status and assignee occupy stable positions on wide panels and
wrap on narrow ones. Blocked work uses category text plus semantic emphasis, not
color alone.

Construct metadata from meaningful fields instead of unconditional separators.
Null assignee identity is Unassigned; a non-null historical identity with no
saved name is Unknown. Historical category text must not silently resolve to
today's custom status names. Show positive rollover counts and retain the
existing review warning at three or more; do not change the stored count.

## Examples and regression ownership

Good: an active iteration with original count zero still offers original scope.
Two field conditions remain visible after the popup closes; reset restores all
tasks while preserving grouping by status.

Base: an empty plan still offers search and the existing empty-state action.
Missing filter metadata disables only the affected selectors.

Bad: hiding original scope because `statistics.original === 0`, counting an
assignee's type and ID as separate conditions, or rendering a missing saved name
as Unassigned.

```tsx
// Wrong: a zero-task commitment can still be a real commitment.
const canCompareScopes = statistics.original > 0;

// Correct: reuse the parent's established phase.
const canCompareScopes =
  phase === "active" || phase === "completed" || phase === "cancelledAfterStart";
```

- `iteration-details.test.tsx`: actual controls/popups, condition removal/reset,
  cursor behavior, metadata compatibility, rows and unknown-phase fallback.
- `iteration-navigation.test.tsx`: parent phase wiring, zero baselines,
  independent counters, tab retention and empty-plan search.
- `e2e/iterations-audit-desktop.spec.ts`: real-API non-empty filtering/reset,
  popup focus, bilingual planned layouts, bounded search, narrow/coarse targets.
- `e2e/fixtures/iteration-scope-business.ts`: retained refinements through real
  lifecycle events and frozen history on Web/Desktop. Its capture helper keeps
  Playwright viewport and Chromium pointer emulation synchronized.
