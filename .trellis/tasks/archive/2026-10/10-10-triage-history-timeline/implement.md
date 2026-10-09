# Implementation plan

## Approved scope and ownership

The user approved the timeline + filter-feedback increment after the review. The leader owns planning, final integration/visual verification, spec capture, local commit and task closeout. A Trellis implementer owns the triage views, matching locale messages and focused tests. A subsequent Trellis checker independently verifies those changes and fixes only local mechanical issues. Do not edit any iteration files or the existing changed `apps/web/next-env.d.ts`.

## Sequence

- [x] Read the manifests, requirements and relevant source/tests; record a narrow behavior-preserving extraction plan if needed.
- [x] Add meaningful regression coverage for any shared event-content extraction; add date/filter behavioral cases before implementation.
- [x] Implement the grouped compact global timeline with preserved navigation, snapshot disclosure, timestamps and fallback semantics.
- [x] Add the applied-history-filter summary, clear action and distinct no-match message through the existing intended-query route hook.
- [x] Update both locale files, keeping existing product terminology.
- [x] Run the focused triage suite and locale parity; run views typecheck and scoped lint. Fix task-caused failures.
- [x] Dispatch independent Trellis check on the finished diff; integrate safe findings.
- [x] Render actual components with real styles, inspect wide/narrow + locales in one batch, run the Impeccable detector, record visual verdict; if necessary make one correction batch and confirmation.
- [x] Record evidence in `verification.md`; update a small task-relevant spec; review/stage only owned files and commit using Conventional + Lore intent/trailers; archive and journal the completed task.

## Validation commands

Use scripts from `package.json`; no installation or new dependency is required.

```sh
pnpm --filter @multica/views exec vitest run triage locales/parity.test.ts
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views exec eslint triage
git diff --check -- packages/views/triage packages/views/locales/en/triage.json packages/views/locales/zh-Hans/triage.json
```

The final detector runs against changed TSX files once the UI is complete. Broaden to Web/Desktop typechecks as needed to verify shared integration. Existing unrelated working-tree changes are observed, never repaired as part of this task.

## Review-sensitive points

- The global list and single-item history share event formatting.
- Delayed URL acknowledgement must not resurrect cleared filters.
- Time grouping and labels must use the same clock; date input summary must not shift its date.
- Paging must not merge events or claim incomplete all-day counts.
- No new product-owned decisions are pending; material scope expansion returns to the user.
