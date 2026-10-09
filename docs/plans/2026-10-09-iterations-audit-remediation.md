# Iteration audit remediation

## Goal and change boundary

Resolve the 19 findings in the user's iteration UI audit while retaining the current timeline, detail tabs, saved timezone, frozen history, and confirmed-operation model. The smallest gap is that the main surfaces already use the design system, but labels, focus, status text, secondary controls and confirmation content do not consistently meet the same standard.

The scope includes shared iteration views, the shared tab panel focus treatment, two readable semantic status text tokens, bilingual iteration copy, and the existing iteration-issues response/query contract needed for filter options. No new dependencies, database migrations, routes or lifecycle operations are needed. Preserve the existing working-tree edits. Do not stage complete premodified files or commit another session's changes.

## Confirmed evidence

- `packages/ui/components/ui/tabs.tsx` removes the outline on a keyboard-focusable Base UI panel.
- `iteration-page.tsx` uses `text-warning` for overdue text and `bg-chart-1/10 text-chart-1` for the active badge.
- `iteration-overview.tsx` uses the same badge pair, transparent neutral marks, a nested main landmark, native status select and a pagination label for local reveal-more.
- `iteration-issue-list.tsx` names the search field as task selection and invokes the complete grouped-task traversal when opening filters.
- `iteration-operation.tsx` uses native selects/checkboxes, an unlabeled mode selector, weak preview structure and a generic confirmation action.
- `iteration-current-comparison.tsx` puts historical titles inside a fixed-height, non-wrapping button.
- `ListIterationIssues` already obtains the complete original/current historical projection before selecting and paginating. Filter metadata can be derived from that projection without fetching all task pages in the browser or changing frozen facts.
- Baseline: `pnpm --filter @multica/views exec vitest run iterations` passed 12 files / 133 tests; `pnpm --filter @multica/core exec vitest run iterations` passed 11 files / 120 tests.
- Existing screenshots are references for incumbent layout only. Verification requires new captures from the changed implementation.

## Acceptance map

| Finding | Required outcome | Evidence |
| --- | --- | --- |
| 1 | Overdue text uses a semantic text tone with at least 4.5:1 contrast in both themes. | Computed foreground/background contrast on actual surfaces. |
| 2 | Active badges meet 4.5:1 contrast including their tinted background. | Actual badge colors, both themes. |
| 3 | Keyboard focus on a shared tab panel has a visible indicator. | Real Base UI keyboard interaction and browser focus styles. |
| 4 | Task search has a visible search label and matching accessible name. | Label association and search interaction regression. |
| 5 | Native selects, textareas and checkboxes in the audited flow use existing UI primitives. | Source review and real-control interaction tests. |
| 6 | Confirmations show readable change summaries, grouped task information and operation-specific confirmation labels; cancellation/deletion/disabling look destructive. | Preview and recovery regressions plus light/dark, wide/narrow dialog captures. |
| 7 | Historical titles and comparisons wrap without horizontal overflow. | Long Latin/CJK titles at narrow widths. |
| 8 | Opening filters causes no all-task page traversal; choices still cover the complete selected historical scope. | Request counts and API/filter-metadata regressions beyond the first task page. |
| 9 | Audited controls, including portal controls, have consistent 44px coarse-pointer targets. | Computed target bounds with coarse-pointer emulation. |
| 10 | Decorative neutral marks use the faint token; text retains readable foreground tones. | Source scan and actual contrast. |
| 11 | Create/start actions take priority; retry/pagination stay secondary. | Source review and screenshots. |
| 12 | Required reasons and disabled start/preview causes are visible and associated with controls. | Form/operation regressions. |
| 13 | Recovery actions identify their own operation while retaining original request identity. | Multiple-pending-command regression. |
| 14 | Iteration pages do not create a second main landmark inside the application shell. | Real DOM landmarks. |
| 15 | Scope date groups follow a valid heading hierarchy. | Heading inspection. |
| 16 | Static overview copy does not use an unnecessary live region. | DOM review. |
| 17 | Overview says load more; task/project pages can return to the previous page and reset cursor history on filter changes. | Paging and stale-cursor regressions. |
| 18 | Initial reads show appropriately shaped skeletons; background failures retain authorized content. | Loading and refresh/revocation regressions. |
| 19 | Project/history disclosures use a consistent accessible treatment; event detail prioritizes readable facts with raw stored evidence secondary. | Disclosure keyboard tests and visual review. |

## Technical decisions

- Preserve UI identity: keep the timeline, underline tabs and progressive disclosure. Improve operation structure with headings, definition lists, named sections and bounded task lists.
- Add text-specific status tokens; do not darken a fill/chart token globally to repair a text pairing.
- Fix focus in `TabsContent` once. Keep Base UI keyboard, selection and retained-panel behavior unchanged.
- Reuse `Select`, `Textarea`, `Checkbox`, `Skeleton` and the incumbent disclosure patterns. Use one coarse-pointer utility convention throughout this scope, including dialogs and select popups.
- Derive compact filter choices from the unfiltered current/original historical projection in the existing issues response. Extend its Zod schema compatibly. Missing metadata from an older backend must never cause an all-task fetch or invent historical choices.
- Keep the complete grouped query for explicit grouping, which needs every member. The filter UI must not invoke it merely to populate controls.
- Keep preview completeness, immutable history, baseline revisions, authorization eviction, and exact request recovery authoritative. Presentation must not infer a successful operation from current status or change saved draft payloads.

## Execution and verification

1. Obtain the required Trellis task-creation consent; transfer this plan into the task PRD/design/implementation artifacts and curate context manifests.
2. Capture current-rendering references and record a visual verdict before UI changes.
3. Implement bounded parallel lanes: page/token/focus work; forms/operations/recovery; filter metadata and task paging. Keep shared locale ownership with the coordinating session.
4. Finish comparison/project/settings/event presentation and integrate the lanes. Add meaningful behavioral regressions before behavioral changes; use browser inspection for visual-only edits.
5. Run the iteration suites, locale parity, shared package lint/typecheck, workspace typecheck, UI export check and scoped static analysis. Run Go handler/unit checks for changed response logic.
6. Use an isolated test profile and current application build for real browser verification. Inspect wide/narrow, light/dark, both locales, keyboard focus, dialog/select interactions, touch targets, long-title overflow and filter request counts. Preserve existing listeners/profiles.
7. Batch visual findings into one fix pass and one confirmation pass. Persist evidence and a fresh impeccable audit with explicit coverage limits.
8. Update the relevant iteration spec and session record. Commit only this task's changes if they can be isolated from the existing dirty worktree; otherwise leave them reviewable and record why.

## Known verification constraints

- A detector result of zero does not prove Tailwind colors, accessible names or layout pass.
- Existing Electron/web listeners are not automatically owned by this task and must not be terminated or rebuilt without establishing ownership.
- Source- or component-level evidence must not be reported as a live browser or native E2E pass.
