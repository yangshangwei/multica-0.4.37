# Native Inbox tab identity follow-up

Date: 2026-11-21. Scope: the real Desktop tab-store resource key and the shared pure tab-title descriptor, with canonical tests. No server, API, E2E, locale or Views files were edited by this owner.

## Reproduction and cause

The coordinator's real native journey reported that Cmd-opening the main Inbox while Snoozed was open reused the Snoozed tab. `resourceKeyForUrl()` returned only the pathname, so the three Inbox views shared `/workspace/inbox` despite the earlier Core `tabSubjectKey()` improvement. The Desktop store uses its own resource key, not that Core render key.

Regression-first runs proved the gap:

- Desktop canonical tab-store suite: 83 assertions, 2 new failures and 81 existing passes. Snoozed's resource key collapsed to `/acme/inbox`, and the three `openTab()` calls returned one distinct ID instead of three.
- Core canonical presentation suite: 19 assertions, 1 new failure and 18 existing passes. Unselected Snoozed returned the active Inbox navigation label instead of an Inbox-localized Snoozed label.

## Change

- `apps/desktop/src/renderer/src/stores/tab-store.ts`: exact workspace Inbox collection routes retain only canonical `view=snoozed` or `view=archived` in resource identity. Active Inbox keeps its pathname key. Selection, filters and hash remain view state. Every other resource keeps its existing pathname identity, including human/UUID issue paths.
- `packages/core/paths/tab-presentation.ts`: `TabTitleSpec` adds `{ kind: "inbox"; inboxKey: "snoozed_title" }`, localized by the Views owner through the existing `inbox.list.snoozed_title` key. Unselected/unresolved Snoozed uses this label; active/Archived navigation labels and resolved selected-resource titles remain unchanged.
- Canonical tests cover distinct native IDs for the three views, same-view selection deduplication with retained original URL, non-Inbox resource identities and the Snoozed-specific title.

## Verification

Executed after the source changes:

```bash
pnpm --filter @multica/desktop exec vitest run src/renderer/src/stores/tab-store.test.ts
pnpm --filter @multica/core exec vitest run paths/tab-presentation.test.ts paths/tab-subject.test.ts
pnpm --filter @multica/core typecheck
pnpm --filter @multica/desktop exec eslint src/renderer/src/stores/tab-store.ts src/renderer/src/stores/tab-store.test.ts
pnpm --filter @multica/core exec eslint paths/tab-presentation.ts paths/tab-presentation.test.ts
```

Results: Desktop 83/83; Core 64/64; Core typecheck and both scoped lint runs passed. Scoped `git diff --check` passed. Existing pnpm configuration warnings remain unrelated to this increment.

The separate Views owner must wire the new title descriptor before renderer admission. The coordinator owns the Desktop rebuild, its process restart and the real native E2E replay; these unit/static checks do not claim that GUI acceptance has run.
