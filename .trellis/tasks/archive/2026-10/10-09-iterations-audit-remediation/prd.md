# Iteration audit remediation

## Goal

Make all audited iteration surfaces readable, keyboard accessible, consistent with the existing component library and usable on narrow/touch layouts. Resolve the user's 19 findings without changing iteration lifecycle or historical truth.

## Background and approval

The user supplied a 13/20 audit covering the overview, detail tabs, forms, lifecycle confirmations, assignment, settings and issue/project embeds. Task creation and implementation of docs/plans/2026-10-09-iterations-audit-remediation.md were approved.

## Scope and requirements

Shared iteration views, tab panel focus, readable semantic status text tokens, bilingual labels, and compact filter metadata in the existing issues read contract. Retain the incumbent timeline/tab composition, complete grouping, frozen historical projections, saved timezone, permissions, revision baselines and original command recovery identity.

No new dependencies, migrations, routes or lifecycle operations. Preserve all existing working-tree edits. The approved scope and evidence are recorded in research/approved-plan.md.

## Acceptance criteria

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
