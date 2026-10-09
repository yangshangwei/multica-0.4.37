# Iteration progress and scope activity design

## Approved direction and spatial thesis

Operate mode, preserving the existing Multica visual identity. The reading path is result → explanation → evidence. The shared header carries identity and lifecycle context once; Progress carries delivery; Scope Changes carries why effective scope changed. Use proximity and consistent spacing instead of new nested cards. Keep three primary tabs.

Progress leads with effective completion, original commitment attainment, and remaining work. Show denominator text next to each rate. The unfinished closeout outcome follows; complete closeout metadata and daily rows are disclosures. A one-day dataset has a compact dated summary, not a 240px chart of overlapping dots. Multi-day charts retain a textual data alternative and use solid semantic colors with a dashed original-commitment line.

Scope Changes leads with initial/current-or-frozen effective scope and net change. A small filter switches relevant scope activity and all activity. The chronology is newest-first and grouped by day; an operation's sub-events are expandable without losing raw identities. Different information types have different summaries: membership moves, status/category changes, metadata edits, and lifecycle anchors.

## Existing work to retain

- Current baseline includes IterationChartTable and iteration-chart-closeout.test.tsx; use the extracted table and preserve frozen-chart assertions.
- Current parent reads already retain cached data for transient failures and distinguish read messages, access denial and definitive deletion; do not regress them while moving the header.
- Task filters and visited panels remain mounted; operation/editor/recovery owners stay outside the tab panels and keyed by stable entity identity.
- The prior audit's forced duplicate unknown-reference test is intentionally changed when non-move arrows are removed; no other historical facts are removed.

## Boundaries and file ownership

| Lane | Owner scope | Purpose |
|---|---|---|
| Progress implementer | views/iterations/iteration-page.tsx, iteration-history.tsx, iteration-progress.tsx, their progress/page/closeout tests; progressPanel/page presentation locale keys | Header and delivery hierarchy, graph/table, closeout outcome; wire the activity panel contract below |
| Activity implementer | new core/iterations/activity.ts plus test and exports; views/iterations/iteration-events.tsx, iteration-events-view.tsx and event tests; iteration-details.test.tsx event assertions; activityPanel locale keys | Complete reads, typed event projections, grouping/filtering, semantic timeline and recoverable browsing |
| Integration/check | Named task files only; e2e/fixtures/iterations-i1.ts and existing iteration Electron assertions when needed | Integrate, adapt table-disclosure interactions, run checks and repair mechanical issues |
| Main session | Task artifacts, spec update, independent visual evidence and final ownership review | Coordinate and verify |

Both implementers touch projects.json only in separate owned key subtrees, using localized apply_patch hunks. Do not parse-and-rewrite the whole JSON file. Progress may add iterations.progressPanel and necessary state-specific labels; activity may add iterations.activityPanel. All additions must exist in en and zh-Hans.

## Stable component contract

The activity lane exports `IterationEventsPanel` from `iteration-events.tsx`:

```ts
type Detail = Awaited<ReturnType<typeof api.getIteration>>;
type Props = {
  wsId: string;
  id: string;
  timezone: string;
  statistics: Detail["statistics"];
  snapshot: Detail["snapshot"];
};
```

The detail page mounts it only after the events tab is visited, retaining its mounted state. It replaces the old inline IterationEvents pager and both snapshot/live event branches. The progress lane owns that integration edit. Activity must not edit iteration-page.tsx.

IterationEventsView remains usable for existing embedded history consumers; the full Scope Changes panel owns summary/filter controls. Event rendering changes must retain the ability to inspect every record in all-activity mode.

## Data flow and historical semantics

```text
Existing validated iteration API
    → workspace-scoped React Query + protectIterationRead
    → frozen snapshot when closed / current data when active
    → typed projections (event kind, stored facts, operation groups)
    → localized view formatting and native disclosures
```

Do not recalculate authoritative counters from the filtered event list. Statistics and chronology can refresh separately; temporary read failure must be visible and retain authorized cached data.

Historical event titles/statuses come from stored facts. Existing status-category labels can format historical categories. Current actor/iteration directory names are references only. A missing historical title/identifier can be represented honestly with a concise localized fallback, with full ID available in details. Do not substitute today's issue state for missing before/after facts.

A migration row is shown only for a semantic move with applicable source/target facts. Explicit null is an unassigned destination; absent data is unknown. Non-move events do not produce missing-to-missing arrows. Unknown event kinds remain accessible through all activity and retain their raw kind in details.

Group by operation identity without merging unrelated operations with similar timestamps. Include the start event and its baseline records in one start group when they share the same operation. Preserve individual events, reasons, original sequence and ID in disclosures. Default scope filtering includes joins/leaves/reentries, cancellations/reopens and deleted in-scope work, transitions across cancelled membership, and relevant baseline/closeout anchors; ordinary title/assignee edits belong to all activity.

Reopen classification uses stored category facts: cancelled → non-cancelled changes effective scope; done → todo changes completion only and stays in all activity. Do not classify every event named reopen as scope growth. Order groups/events by authoritative sequence descending, using the saved iteration timezone only for day grouping and display.

For closeout summaries, classify only tasks with frozen nonterminal status as unfinished. Completed/cancelled tasks with null destinations are not new removals. Keep full destination records in a disclosure for audit and preserve unknown legacy values.

## Complete chronology and browsing

The existing API is ascending and cursor-bound (`after_sequence`, `limit`, `cursor` only). Sorting only the first returned page would falsely claim to show the latest changes and can hide all real changes behind baseline events. Introduce a complete activity query in core, following the existing complete catalogue pattern: fetch limit 100 pages, pass AbortSignal, protect the whole read with the existing authorization/session fence, check workspace/iteration identity, detect repeated cursor/event identity, restart once on cursor_stale, and reject incomplete/repeated traversal.

Use a distinct key under the existing iterations workspace prefix; retain the existing single-page API/query export for other consumers. For a closed period use snapshot.events directly and do not fetch live history. Sort/group only the complete successful result. Render a bounded number of groups initially and use local “show more” to append groups, so the DOM is not forced to show every event at once. No server sorting/migration/capability change is required.

The complete query key explicitly includes wsId and iterationId, and sets retry: false so Query retries cannot exceed the single internal cursor-stale restart. Never return a partial traversal as a successful complete history.

Tradeoff: very large active histories need multiple reads. Limit each request to the existing supported maximum, query only after visiting, cache through Query and honor cancellation. This is preferable here to a new versioned backend sorting contract or silently incomplete client filtering. Network failure retains the prior complete result; a fresh failed traversal is not labelled as an empty history.

## State and errors

- View-only filter, visible group count and disclosures are local state; no new global stores.
- First loading, truly empty, no matching scope changes, transient refresh failure and access loss have distinct outcomes.
- Retry repeats the appropriate read; it must not discard filter/visible history for transient errors. Permission/deletion errors remove protected results.
- Entity keys reset state on workspace/iteration changes. Changes in revision do not reset user inputs.

## Verification and rollback

Canonical pure tests cover event grouping/classification/fact projections and complete traversal. DOM tests cover actual user interactions, denominators, frozen closeout, disclosures and recovery wiring; do not replay the same pure matrix through DOM.

One combined closeout regression contains C/D plus completed/cancelled tasks: conflicting current data must still show frozen 2/4 and 1/4; only the two nonterminal tasks enter the unfinished destination summary; during-period removals remain 0 events while unfinished closeout removals are 1 task; terminal null targets do not increase that count; snapshot history does not call the complete activity API.

Baseline captured in `/tmp/multica-iteration-ui-baseline.rF9dTQ` with file SHA-256 and HEAD `be9acfa749824cb1511ca62a2ee3dcf5980f4862`. Other sessions are changing the checkout; review this task against that baseline as well as git. Preserve intervening edits rather than resetting files. No runtime/dependency/backend change is needed for rollback; revert only this task's hunks.
