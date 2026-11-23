# Triage welcome and empty-queue navigation

## Scope and owners

Shared Web/Desktop `TriagePage` owns the data-derived first-use branch, routes,
dialogs and focus. `TriageWelcome({ docsHref, onCreate, onImport })` owns only the
localized introduction, real intake buttons and passive workflow example.

## First-use contract

- Check only an enabled, supported, authorized default ready-queue homepage.
  Both raw and deferred search, any filters, nonzero pagination, selected tasks
  and import-batch deep links take priority over first-use presentation.
- Require successful, non-placeholder, error-free empty queue data with zero
  workspace `counts.pending`. Pending includes snoozed tasks. A positive value
  from the separate counts query also rules out the welcome.
- Probe `triageHistoryOptions(wsId, { limit: 1 })` only for that empty homepage.
  This unfiltered key checks history existence, including committed imports,
  without changing the history tab's filtered/paginated query. Server data stays
  in React Query; do not persist a separate seen-welcome flag.
- The existence probe uses `staleTime: Infinity`, with `"always"` refetch on
  mount, window focus and reconnect. Existing `invalidateTriage` invalidates
  the whole workspace triage prefix after mutations, imports and live events.
  Require `!isStale` before treating its successful zero as proof of first use:
  an invalidated empty cache must not reintroduce the welcome after the final
  pending task is accepted. Pending, failed and placeholder probes provide no
  first-use evidence.
- Opening the empty filter panel or changing sort preserves the queue. Those
  view preferences suppress the hero but do not suppress known review-history
  navigation. Queries remain scoped by workspace ID.

## Feedback, actions and focus

The ordinary ready-empty heading remains mounted while history is checked, so
the existing post-decision focus target survives. Once positive history is
confirmed, add the history-navigation action. If pending tasks are all snoozed,
offer snoozed navigation instead. Active filters keep the clear-filters action;
history pagination retains its own empty-current-page recovery.

Use the existing `selectView` behavior to clear selection and pagination and
focus the destination tab after an empty-state navigation action. The welcome
opens the same create/import dialogs as the header, and hides duplicate header
intake actions while visible. Use `DOCS_SLUGS.triage` with `paths.docsPage` for
the already published in-app guide. Illustration rows are examples, not tasks
or buttons, and remain outside the focus order.

## Validation

`triage-page-recovery.test.tsx` owns real QueryClient first-use, invalidation,
refresh-failure, sort/filter-panel and snoozed-navigation regressions.
`triage-page.test.tsx` retains existing queue, history, tab and selection tests.
`e2e/triage.spec.ts` exercises actual docs and intake dialogs, creation/import
transitions, ordinary acceptance focus, and bilingual light/dark responsive
screenshots. Browser evidence confirms Web; native Electron execution requires
its own evidence beyond consuming the shared component and typechecking.
