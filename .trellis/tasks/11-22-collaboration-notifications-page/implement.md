# Collaboration notifications page implementation plan

**Goal:** Deliver the user-approved independent collaboration notifications page on Web and Electron.

**Architecture:** Reuse the strict `/account` route and existing collaboration detail. Extend the existing notification read before cursor pagination and validate response view identity. React Query owns every notification projection; existing route history and platform mementos own selection/restoration.

**Tech stack:** Go/Chi/sqlc/PostgreSQL; TypeScript/React/TanStack Query; shared Base UI components; Next.js and Electron navigation adapters; Vitest and Playwright.

## Phase gate and ownership

- [x] User chose Option 1 and explicitly authorized a new Trellis task plus execution.
- [x] PRD convergence pass: Goal, accepted facts, R1–R7, AC1–AC7, exclusions and evidence are final; no blocking product question remains.
- [x] Route/API evidence recorded; design and implementation contracts written.
- [x] Curate and validate both context manifests; start the task.
- [x] Execute three bounded native Trellis implementation lanes below. Agents must read task artifacts/specs, respect current worktree changes, avoid recursive dispatch, and leave commits/spec edits to the main coordinator.

## Lane A — API and query semantics

**Ownership:**

- `server/internal/handler/collaboration_notifications.go` and its test.
- `server/internal/service/collaboration_notifications.go` and its test only when needed for service-owned validation.
- `server/pkg/db/queries/collaboration_notifications.sql`, regenerated `server/pkg/db/generated/collaboration_notifications.sql.go`.
- `packages/core/api/client.ts`, a focused notification API test.
- `packages/core/collaboration/notification-view.ts`, `content-schema.ts`, `content-schema.test.ts`, `content-queries.ts`, relevant query tests, `collaboration/index.ts`.

**Steps:**

1. Write/observe failing focused tests for explicit view identity/filter requests and true server-filtered cursor results. The coordinator allocates an isolated migrated test database; do not run DB suites on an ambient deployment database.
2. Implement the pure view leaf (`COLLABORATION_NOTIFICATION_VIEWS`, `CollaborationNotificationView`, `isCollaborationNotificationView`), schema marker and typed `getCollaborationNotifications({cursor?, view?, includeArchived?})` request/response checks.
3. Change `collaborationNotificationsOptions(client, scope, view = "all")` to key/request each view under the existing notification prefix. Update old internal callers rather than adding a boolean adapter.
4. Add view parse/service/SQL predicates, reject simultaneous explicit/legacy selectors, preserve legacy behavior/count/privacy and regenerate via `make sqlc`.
5. Run focused Core tests and lint/typecheck for owned files; run allowed handler/service notification tests sequentially on the coordinator-provided isolated test database. Report exact commands and inventories.

## Lane B — Route/platform/sidebar integration

**Ownership:**

- `packages/core/paths/account.ts`, `account.test.ts`, `paths/index.ts`, `navigation/scope-history.test.ts`.
- `packages/views/layout/app-sidebar.tsx`, `app-sidebar.test.tsx`.
- Both Web and Desktop `platform/account-workbench-route.tsx` and their tests.
- Desktop `components/tab-bar.tsx` and focused tab tests; `stores/account-tab.test.ts`.
- Native issue-window parser/test only as required to keep notification selections in the main account tab; route icon descriptor/map/tests only if needed for Bell.
- `e2e/global-scope-navigation.spec.ts`: update the old popover block to page navigation/collapse behavior and keep the broader scope-resume assertions.

**Steps:**

1. Write/observe failing route roundtrip/rejection/restoration and active sidebar-link tests.
2. Implement the target/destination union using the view type leaf from Lane A; preserve legacy URL/output shapes.
3. Make the sidebar notification entry an active AppLink to default Unread while retaining its protected count observer and collapsed directory semantics. Remove the popover import; coordinate new labels with Lane C.
4. Update document/tab titles and icon presentation. Keep route trees and platform restoration implementation unchanged unless tests establish a required local fix. Exclude notification destinations from implicit issue-window popout.
5. Extend platform/history/window tests. Run scoped routes/navigation/sidebar/platform tests and owned-file lint.

## Lane C — Shared page, links and visible behavior

**Ownership:**

- New `packages/views/collaboration/notifications-page.tsx` and canonical shared page test.
- `packages/views/collaboration/notifications.tsx` and existing notification tests; `collaboration/index.ts`.
- `packages/views/collaboration/account-workbench-page.tsx`, its shared test for dispatch/narrowing.
- Both `packages/views/locales/{en,zh-Hans}/collaboration.json` locale files.
- `collaboration/run-navigation.test.tsx`: adapt the removed popover setup while retaining actual run/anchor assertions.
- `collaboration/activity.tsx` and its test: retain notification filter/selection when an in-preview activity link targets a comment; clear conflicting old fragments.

**Steps:**

1. Write/observe failing page tests for filter navigation, list/detail dispatch, exact links, stable list DOM/scroll, read/archive feedback and unavailable content.
2. Immediately before UI edits read the Impeccable craft floor; preserve existing app tokens/chrome/typography and follow conventions for Chinese copy.
3. Build the dedicated page with typed notification view/query interfaces from Lane A and destination interfaces from Lane B. Keep list subtree keyed by scope/filter only; keep `account-detail` for the existing detail component. Use existing restoration/realtime seams and avoid new stores.
4. Replace the obsolete notification popover with a simple navigation link for the non-sidebar workbench case; reuse the factual event label helper. Remove dead popup exports/imports/tests.
5. Add localized filter/empty/list/back/action labels and run locale parity plus canonical page/workbench/notification tests and owned-file lint.

## Coordinator integration and proof

- [x] Resolve review R6/AC6 follow-up: optional committed history-return signal in shared navigation/Web/native existing tab state, bounded notification list/detail memento consumption, and distinct same-issue A/B Back/Forward offsets. UI follow-up owns navigation/page/detail production and canonical tests; E2E follow-up owns only the new acceptance test's same-issue case. No new store/persisted metadata or broad workbench behavior change.
- [x] Confirm all lanes match the shared signatures and no unowned baseline file changed.
- [x] Run monorepo `pnpm typecheck` and `pnpm lint`; focused Core/view/Web/Desktop test suites; Go notification handler/service tests and `go -C server vet ./internal/handler ./internal/service ./pkg/db/generated`.
- [x] Run mechanical UI detector once on changed notification/sidebar targets. Review its findings in context.
- [x] Add/run focused browser/native acceptance using the existing account notification fixture patterns: Unread/All/Archived, known badge, read/archive, exact comment/run/body target, long titles, compact back flow, Back/scope resume and list/detail scroll retention.
- [x] Adapt existing recorded-results, mention, recovery and offline notification fixture helpers that still reference the removed popover. Their legacy retained-read flow should explicitly use All and assert real page navigation/anchors rather than popup visibility.
- [x] Use a task-owned production Web environment (`make up C=api,web ARGS="--web-mode production"` with isolated configuration) and actual Electron harness. Do not use Next dev for E2E. Record ports/database identity, commands and stop method; clean up owned processes after proof while retaining evidence/database.
- [x] Record visual verdict before any post-capture UI fix; allow one batched defect fix plus one confirmation capture.
- [x] Dispatch native Trellis check for independent spec/quality review; resolve substantive findings, rerun only checks affected by new changes.
- [ ] Update navigation/workbench/server notification contracts, task verification evidence and journal; commit only this task's files using conventional intent + Lore trailers; archive/finish after the quality gate passes.

## Focused commands

```sh
pnpm --filter @multica/core exec vitest run paths/account.test.ts navigation/scope-history.test.ts api/collaboration-notifications.test.ts collaboration/content-schema.test.ts collaboration/result-notification.test.ts
pnpm --filter @multica/views exec vitest run collaboration/notifications-page.test.tsx collaboration/notifications.test.tsx collaboration/account-workbench-page.test.tsx layout/app-sidebar.test.tsx collaboration/run-navigation.test.tsx platform/scroll-restoration.test.tsx locales/parity.test.ts
pnpm --filter @multica/web exec vitest run platform/account-workbench-route.test.tsx platform/scroll-restoration.test.tsx
pnpm --filter @multica/desktop exec vitest run src/renderer/src/platform/account-workbench-route.test.tsx src/renderer/src/stores/account-tab.test.ts src/renderer/src/platform/navigation.test.tsx src/shared/issue-window.test.ts
pnpm typecheck
pnpm lint
go -C server vet ./internal/handler ./internal/service ./pkg/db/generated
```

DB commands require the coordinator's recorded private test environment and exact package permit. Final E2E paths/fixtures will be selected from current harness source before running; do not add real-agent subprocess activity or quota use.

## Continuation completion evidence

The continuation finished the approved R6 follow-up and live acceptance. Production Web exposed complete-location/intent ordering, UA history restoration and retained-offset departure capture gaps; all fixes stay in the existing navigation/memento owners. Receipt-aware focus recovery stays within the shared notification row. Final production Web and actual Electron acceptance passed all six selected journeys without retries (`research/e2e-final-r19.json`). Canonical TS inventory contains 565 passing cases; focused Go notification tests, vet, locale parity and UI detector also pass. See `research/verification.md` for exact commands, limits and cleanup.
