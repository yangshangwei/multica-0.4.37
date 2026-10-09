# Triage history presentation

## Scope and owners

Use this contract for the shared Web/Desktop triage history, its filters, and
event snapshot disclosures. The backend audit contract remains in
[`server/triage.md`](../../server/triage.md).

- `TriageHistoryTimeline({ wsId, entries, onOpenIssue, onOpenImport })` owns the
  global list presentation; `TriagePage` keeps queries, routes and pagination.
- `groupTriageHistoryEntries(entries, locale, timeZone?)` in
  `triage-history-dates.ts` owns display-calendar grouping.
- `TriageHistoryContent({ wsId, entry, compact? })` shares actor, reason and
  snapshot formatting with the existing single-item `TriageHistoryRecord`.
- `triageHistoryFilters(params)` and `TRIAGE_HISTORY_FILTER_KEYS` in
  `triage-ui.ts` define the effective global-history query fields.

## Event identity and time

Group adjacent events without sorting, merging equal timestamps, or changing
the server's page order. A group key is the display day plus its occurrence,
not its first event ID: prepending a new event must not remount already-open
disclosures. Keep each row keyed by event ID. Repeated noncontiguous days stay
separate because the API owns ordering.

Group keys use the Gregorian/Latin-numeral calendar on the same clock used by
the labels. UI dates use the current locale; the default clock is browser
local time, matching the existing history. Reuse `formatInTimeZone`; invalid
timestamps form an intelligible unavailable-date group instead of throwing.
Absolute day headings include the year. Compact times retain full timestamps
with seconds and timezone in their accessible text.

An action describes the historical decision, not the current task status.
Imports display the retained filename and cumulative import results. Do not
infer review-batch identity from equal timestamps or import provenance, and do
not call a loaded-page count an all-day total. Keep the existing snapshot
formatter and single-item execution/retry behavior when changing presentation.

## Filter and empty-page behavior

The effective fields are `q`, `source`, `result`, `processed_by`,
`processed_after`, and `processed_before`. Derive feedback from the intended
parameters returned by `useTriageRoute`, not a separate filter store or a stale
platform URL. Known values use localized labels/member names; date-only values
retain their entered date without timezone conversion.

Clear filters through one `changeParams` call deleting those fields and
`offset`. Preserve `view`, task/import selection and unrelated parameters.
First-page recovery deletes only `offset`, preserving all filtering intent.

| Successful response | Required feedback |
|---|---|
| Entries present | Date-grouped timeline |
| Entries empty, positive total | Empty-current-page copy and return-to-first-page action |
| Entries empty, total zero, active conditions | No matching history and clear-filters action |
| Entries empty, total zero, no active conditions | Genuine empty-history copy |

```tsx
// Wrong: a page can be empty while matching records exist elsewhere.
const noMatches = entries.length === 0 && filters.length > 0;

// Correct: distinguish the page from the complete result set first.
const emptyPage = entries.length === 0 && total > 0;
const noMatches = entries.length === 0 && total === 0 && filters.length > 0;
```

## Verification

- `triage-history-dates.test.ts`: calendar boundaries, DST, invalid values,
  locale-independent identity, and preservation of incoming order/equal times.
- `triage-history.test.tsx`: semantic dates, full accessible time, task/import
  navigation, shared snapshot behavior, and open disclosure identity on prepend.
- `triage-history-filter-summary.test.tsx` and `triage-ui.test.ts`: canonical
  field allowlist, readable values and safe unknown-value fallback.
- `triage-page.test.tsx`: clear and first-page actions, all three empty-result
  states, and delayed route acknowledgement preserving the latest intention.
- Browser verification: native summary Tab/Enter, long text in both locales,
  narrow/wide wrapping, real bounded scrolling and caption contrast. A standalone
  fixture must reproduce the shell's flex-column parent; a fixed-height block
  alone does not prove scrolling or bottom pagination placement.
