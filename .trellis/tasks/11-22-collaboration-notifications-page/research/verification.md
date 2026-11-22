# Collaboration notification page verification

The independent account notification page satisfies AC1–AC7 on shared Web and
Desktop delivery. The final production Web/actual Electron run passed all six
selected journeys without retries. Earlier failed runs and their diagnostics
remain separate evidence; they are not included in the passing inventory.

## Result and scope

- Sidebar entry opens `/account?view=notifications&filter=unread` with an active
  item, notification title/Bell and protected authoritative count even when the
  Collaboration directory is collapsed.
- Unread, All and Archived filter before server cursor pagination, preserve the
  global unarchived unread count and reject missing/mismatched filtered response
  markers. Legacy Default/All and mixed include-archived requests retain their
  supported semantics.
- Rows retain notification UUID separately from issue UUID and exact run,
  comment and body anchors. Removed/inaccessible records stay content-free and
  do not initiate private detail reads.
- Read/archive await validated receipts, refresh every notification projection,
  preserve archive/read independence and recover focus to surviving targets.
  Query Refresh never replays a failed write.
- Wide/compact layouts retain list geometry and same-issue draft nodes. Back,
  Forward and workspace↔Collaboration resume restore filters, notification
  selection and independent list/detail offsets. Explicit repeated source links
  still focus their anchors.
- Web navigation publishes one complete committed destination, owns transient
  history outside Suspense, temporarily uses manual browser restoration for
  physical notification entries, and snapshots the departing account's actual
  offsets into the existing memento map. It introduces no second scroll/state
  store. Other routes retain their prior browser scroll policy.

Source changes are scoped to 64 task files plus 11 coding-spec/index files.
`research/commit-source-files.json` is the exact inventory. The three baseline
tracked files and unrelated untracked artifacts remain outside the task.

## Static and owning-layer evidence

| Inventory | Files | Passing cases |
| --- | --- | --- |
| Core route/API/schema/query/access/mutation | 8 | 147 |
| Shared Views page/detail/links/sidebar/locale/restoration | 12 | 194 |
| Desktop navigation/tab/store/title/window admission | 7 | 180 |
| Web navigation/scroll/account route | 3 | 44 |
| **Distinct TypeScript cases** | **30** | **565** |

Commands were run from the original checkout. The final Views log and retained
Core/native logs bind their exact inventories. Native Trellis implementers
ran the final Web owner suites and scoped lint/typecheck; the coordinator ran
the repository gates and owning Go/live acceptance.

```sh
pnpm --filter @multica/core exec vitest run paths/account.test.ts navigation/scope-history.test.ts api/collaboration-notifications.test.ts collaboration/content-schema.test.ts collaboration/result-notification.test.ts collaboration/notification-queries.test.ts collaboration/access.test.ts
pnpm --filter @multica/core exec vitest run collaboration/notification-mutation.test.tsx

pnpm --filter @multica/views exec vitest run collaboration/notifications-page.test.tsx collaboration/notifications.test.tsx collaboration/account-workbench-page.test.tsx collaboration/activity.test.tsx collaboration/run-navigation.test.tsx collaboration/collaboration-detail.test.tsx collaboration/detail-auxiliary.test.tsx collaboration/delivery-panel.test.tsx layout/app-sidebar.test.tsx layout/route-icon-components.test.tsx platform/scroll-restoration.test.tsx locales/parity.test.ts

pnpm --filter @multica/desktop exec vitest run src/renderer/src/platform/navigation.test.tsx src/renderer/src/stores/account-tab.test.ts src/renderer/src/stores/tab-store.test.ts
pnpm --filter @multica/desktop exec vitest run src/renderer/src/platform/account-workbench-route.test.tsx src/renderer/src/components/tab-bar.test.tsx src/shared/issue-window.test.ts src/renderer/src/platform/issue-window-navigation.test.tsx

pnpm --filter @multica/web exec vitest run platform/navigation.test.tsx platform/scroll-restoration.test.tsx platform/account-workbench-route.test.tsx
pnpm lint
pnpm typecheck
go -C server vet ./internal/handler ./internal/service ./pkg/db/generated
```

Repository lint passes with 31 pre-existing unrelated warnings; no lint errors.
Typecheck passes for all nine scheduled tasks. Focused Web/Views lint has zero
file warnings. The detector returned `[]`. Task whitespace checks and Go
formatting checks pass.

Go DB suites ran sequentially on separate newly migrated task-owned databases:

- Handler: `multica_check_20261122072738_98971_go_handler`, four named tests
  covering filtered cursors/recipient sentinels/counts, invalid selectors,
  exact/private targets and recipient/object authority.
- Service: `multica_check_20261122072738_98971_go_service`, three named tests
  covering early query validation, idempotent archive without read and lost
  access redaction.

```sh
bash .trellis/tasks/11-22-collaboration-notifications-page/research/test-notification-package.sh handler
bash .trellis/tasks/11-22-collaboration-notifications-page/research/test-notification-package.sh service
```

`go-handler-r1.log`, `go-service-r1.log` and `go-vet-r1.log` retain the results.
Migration/source SQL and generated sqlc output were checked in the original
implementation lane; no schema migration was required by this page.

## Production Web and actual Electron

Final run: `research/e2e-final-r19.json` / `.log`, **6 passed** in approximately
90 seconds, one worker and zero retries:

1. Desktop child links, recorded pages and result notifications across popouts
   and restart.
2. Web complete filtered notification pages, exact targets, processing and
   list/detail restoration.
3. Desktop identical notification journey in actual Electron.
4. Web ordinary-child references and retained records through notification links.
5. Web global switching with restored task, filters and both scroll positions.
6. Desktop identical global-switch journey.

```sh
PLAYWRIGHT_JSON_OUTPUT_NAME=<task-research>/e2e-final-r19.json \
  bash <qa-source>/.trellis/tasks/11-22-collaboration-notifications-page/research/run-qa-command.sh \
  pnpm --dir /Volumes/artisan/code/2026/multica-0.4.37 exec playwright test \
  e2e/collaboration-notifications-page.spec.ts e2e/collaboration-records-web.spec.ts \
  e2e/collaboration-account-desktop.spec.ts e2e/global-scope-navigation.spec.ts \
  -g 'collaboration notifications: complete|Web shares ordinary child references|Desktop child links|global switching resumes' \
  --workers=1 --retries=0 --reporter=line,json --output=<task-research>/e2e-final-r19
```

`web-acceptance-final-r19.json` and `desktop-acceptance-final-r19.json` each retain
three complete server view/page traces, six restoration observations, four
settled screenshot/geometry captures, exactly two notification writes and zero
page errors. Same-issue A/B returns restore `{list: 0, detail: 0}` and
`{list: 360, detail: 620}` while retaining list/detail/composer/draft nodes.
The real keyboard Archive case ends with the surviving list focused.

The QA source is an ignored detached checkout under
`.omx/private/qa-work/notifications-20261122/src`, because another environment
owns this checkout's shared Web build. Its API database is
`multica_check_20261122074342_12482_api`; API `localhost:18417`, production Web
`localhost:13337`, built native renderer `127.0.0.1:5511`. The passing Web build
ID is `1ZcOlEkf48BDSMVGHihga`. `qa-build-final-r7.json` binds source/configuration
hashes and listener identity. The only post-build source delta is the documented
comment correction in `final-comment-only.diff`; executable behavior is unchanged.

The native harness starts actual Electron with the candidate renderer/preload
and public API/WS, isolated profiles and synthetic retained notification/run
history. It performs no daemon or provider launch. These checks prove the human
notification UI/navigation contract, not runtime ingestion, installed full Main,
other operating systems or real model execution.

## Visual and cleanup evidence

The final verdict is `visual/final-r19.json`: **95 / pass** against the existing
app/sidebar/detail visual authority. Wide Web/native and 390px compact/200%
native captures preserve page chrome, readable long titles, selected/unread
states and independent scrolling with no horizontal overflow. A transient
cached read error in one early wide capture recovered; the processed wide and
compact captures retain clean authorized detail and zero final page errors.

Owned application listeners are stopped through:

```sh
bash .trellis/tasks/11-22-collaboration-notifications-page/research/stop-qa.sh
```

The script validates recorded listener/source ownership, stops only the task
renderer/Web/API, verifies all three ports are free and retains databases/logs.
`qa-isolated-environment.json` records final cleanup. Earlier allocated task
listeners were also stopped; unrelated development environments were untouched.

## Remaining limits

Mobile UI, bulk processing/unarchive, notification generation, provider quota,
installed production Main, publishing and deployment remain outside this task.
The older managed/password/offline recovery allocations were statically
collected with their original gates; they are not relabelled as live passes.
No known blocking defect remains in the approved notification-page scope.

Source/spec commit: `00aaf3f5e5dda98ead4b5f6075aa9aa73e3d3fc5`. The final task evidence commit preserves this report, the original failed diagnostics and the passing inventories before archive/journal bookkeeping.
