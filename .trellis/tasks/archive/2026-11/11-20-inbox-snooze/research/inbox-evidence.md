# Inbox snooze evidence and approved intent

Research date: 2026-11-20. Read-only native Codex researchers inspected the backend and shared frontend. No task activation, product edits, migrations or product tests were performed during research.

## Approved source

Session `01a1f964-33c8-7df1-8ee1-71463e19018a` contains the user's reference screenshots, the design summary and the response authorizing this task and implementation planning. `trellis mem extract 01a1f964-33c --phase all` recovered the full 12-turn dialogue. The approved brief is `artifacts/design/inbox-snooze-20261120/design-brief.md`.

Approved behavior: group-wide personal snooze; 1h/3h and tomorrow/3days/1week at local 09:00; future custom time; bottom Snoozed entry before Archived; undo; change time; remind now; server expiry returning unread; ordinary updates remain suppressed; direct mention, assignment and execution failure wake early; cross-device state/count recovery. Batch snooze and native mobile UI are excluded. The prototype is design evidence, not persistence or delivery proof.

## Shared page and group identity

- `apps/web/app/[workspaceSlug]/(dashboard)/inbox/page.tsx:1` exports the shared page; `apps/desktop/src/renderer/src/routes.tsx:265` mounts the same `InboxPage`.
- `packages/views/inbox/components/inbox-view.ts:13` currently accepts only `inbox` and `archived`.
- `packages/core/inbox/queries.ts:91` and `:105` deduplicate task-associated notifications by `issue_id`, falling back to individual item identity for issue-less rows. The newest row supplies the displayed read state; older comment anchors can be retained. There is no `all_read`/`allRead` group contract.
- `packages/core/inbox/queries.ts:75` counts unread displayed groups. `packages/core/inbox/filter-store.ts:180` filters the displayed newest fields.
- Backend `server/pkg/db/queries/inbox.sql:169` counts raw unread rows, whereas `:173` already counts the newest group row per workspace. Plan aligns these queries to the displayed newest-group rule instead of introducing any-unread-sibling semantics.

## UI, selection and time

- `packages/views/inbox/components/inbox-item-actions.tsx:13` and `:33` own the shared row action model. Both `inbox-context-menu.tsx:130` and `inbox-row-menu.tsx:18` consume it.
- `packages/views/common/row-actions-menu.tsx:13` currently supports callback items, not arbitrary nested pickers; a shared picker opened by an action avoids coupling this primitive to Inbox.
- `packages/views/inbox/components/inbox-list-item.tsx:165` owns the hover actions. Preserve click propagation guards, focus and hover-capability behavior for a clock affordance.
- `packages/views/inbox/components/inbox-page.tsx:412` advances selected-row removal to the next older item, then the previous one. Existing archive handlers at `:424`/`:438` advance before sending; new picker operations should instead wait for success under the root state rules.
- `inbox-page.tsx:109`, `:206`, `:226`, `:248`, `:276` and `:603` own view/selection URL recovery and fallback. `:329` auto-marks selected rows read; `:322` has a manual-unread guard. New reminder flows must not be immediately consumed by stale detail selection.
- `inbox-list.tsx:165` memoizes the footer for virtualized focus stability. `inbox-page.tsx:825` owns detail actions. Main bulk actions at `:475` and `:548` have no multiselect model.
- `packages/views/triage/triage-ui.ts:102` has related local-calendar presets; `triage-action-dialog.tsx:41` has a localized time input with actual IANA timezone. It is bound to triage copy and cannot be imported as an Inbox business component. Its 90-day limit at `:174` is not an Inbox requirement.
- Locale owners are `packages/views/locales/en/inbox.json` and `zh-Hans/inbox.json`; use the terminology and Chinese voice in `apps/docs/content/docs/developers/conventions.zh.mdx`.

## API, Query and transport

- `packages/core/types/inbox.ts:43` and `packages/core/api/schemas.ts:2880` own Inbox response fields. Legacy omitted-field preservation has coverage in `packages/core/api/client.test.ts:868`.
- `packages/core/api/client.ts:5041` and `:5063` schema-parse Inbox lists. Some old mutations return typed raw fetch results; new success/undo receipts must use strict schema parsing rather than copying that shortcut.
- `packages/core/inbox/queries.ts:5`, `:14` and `:28` define workspace-scoped list keys/options. Add a distinct snoozed key and quantity projection.
- `packages/core/inbox/mutations.ts:87` and `:127` patch archive/unarchive siblings, establishing the existing group boundary.
- `packages/core/inbox/ws-updaters.ts:5` invalidates Inbox under the source workspace on new items, while projection/delete logic at `:20`/`:74` currently covers only active and archived queries.
- `packages/core/realtime/use-realtime-sync.ts:594` refreshes source workspace plus account summary before notification preferences and banners. Generic Inbox events at `:787` only invalidate the active workspace; extend this with actual event workspace identity.
- Backend `EventInboxUnread` is defined/published but is absent from the personal event registration and recipient routing in `server/cmd/server/listeners.go:123`–`:179`. New state events require both registrations; do not treat a constant alone as delivery. Existing event names are in `server/pkg/protocol/events.go:47`–`:53`.
- `packages/core/triage/queries.ts:59`/`:66` provides a 30s query/focus recovery pattern. Inbox currently has no interval recovery. Server expiry still owns state transitions.
- `packages/core/paths/tab-subject.ts:40`, `:105`, `:166` records only an archived boolean today. Snoozed needs a separate persisted tab identity while existing active/archive keys remain stable.

## Backend due processing and producers

- No Inbox snooze persistence, query or endpoint exists today. Existing archive behavior uses workspace + recipient + `COALESCE(issue_id,id)` group identity.
- `server/internal/handler/triage_notifications.go:271` offers a 15s restart-safe due sweep, started at `server/cmd/server/main.go:684`; its issue-global snooze cannot own personal Inbox state. `triage_notifications.go:126` demonstrates bounded due scanning, `FOR UPDATE SKIP LOCKED`, current recipient/member rechecks and postcommit publication. Triage visibility also uses the timestamp predicate at `triage_queries.go:68`–`:86`.
- `server/cmd/server/notification_listeners.go:563`–`:674` gives all direct `@member`, `@all` and `@squad` recipients the same `mentioned` type/source details. Persist an actual source marker before applying direct-only wake policy.
- Ordinary comments must preserve `CreateCommentInboxItem` UUIDv5 source/recipient deduplication and `pgx.ErrNoRows` handling described in `.trellis/spec/server/ordinary-notification-replay.md`.
- Other Inbox producers include `server/internal/service/task.go:6298`, `:7690`, `:7791`; `server/internal/service/autopilot.go:957`; `server/cmd/server/autopilot_failure_monitor.go:248`; `server/internal/handler/triage_notifications.go:238`; `server/pkg/db/queries/project_update.sql:84`; and `iteration_notifications.sql:25`. Coverage limited to the ordinary comment listener would leave these paths unsuppressed.
- `notification_listeners.go:763` uses `issue_assigned` for actual assignment to the recipient; `:1061` uses `task_failed`. `agent_blocked` occurs in preference/tier maps but is not a separately verified Inbox producer, so do not invent it as a wake-up source.
- Collaboration notifications use a separate table/API and are not folded into ordinary `inbox_item` by this task.
- Existing effective in_review/done/cancelled handling auto-archives old `task_failed` rows after fresh status notifications. Expiry must operate only on nonarchived rows and cancel a plan when no active rows remain.
- `server/internal/handler/handler.go:1323` resolves an Inbox anchor through `loadInboxItemForUser`, validating UUID, workspace and member recipient ownership. Membership-gated routes live at `server/cmd/server/router.go:2235`; existing Inbox endpoints are registered at `:2752`–`:2769`, including separate active and archived GET routes. Add a corresponding `/snoozed` GET route.
- Workspace cleanup currently deletes Inbox rows in `server/pkg/db/queries/workspace_delete.sql:356`; extend its explicit manifest checks. Member revocation uses fences and a transaction in `server/internal/handler/workspace_revoke.go:61`/`:193`, so snooze writes and due scans must follow compatible current-authority checks and lock order.

## Required checks and evidence limits

Canonical tests belong next to time helpers, API schemas/client, Inbox query/mutation/ws logic, tab identity, shared components and backend handlers/listeners. Browser tests must use production Web and the real shared Desktop renderer/native harness, with isolated fixture accounts and TestApiClient.

Planning can choose a consistent API/table contract, but the subsequent implementation must demonstrate transactional version checks, actual producer coverage, exact direct-mention classification, failure retention and current authority. No actual scheduler, database, cross-device transport, runtime screenshot or product acceptance claim is made by this research.

## Critical source identity follow-up and plan review

The independent plan review identified three bounded contract gaps: generic random IDs cannot prove a new critical source; the first snooze/insert needs a shared fence even before a state row exists; and bulk operations must use the effective overdue read projection rather than raw `read` bits. The design and execution plan now explicitly address these and include named regressions.

The backend follow-up verified these exact source identities:

- Comment direct mention: original `comment_id`, extracted/forwarded at `notification_listeners.go:909`–`:957`.
- Description direct mention: original issue UUID + committed issue revision, from creation and description-diff events at `notification_listeners.go:687`–`:717` and `:876`–`:894`.
- Assignment: original issue UUID + committed revision, at `notification_listeners.go:700`–`:709`/`:738`–`:772`. `handler/issue.go:90`/`:338` exposes persisted revision; committed responses are published at `:4496`/`:5697`; `queries/issue.sql:239` increments it on real changes. Do not read the later current revision during delayed notification delivery.
- Task failure: original persisted `task_id` in `service/task.go:7064`–`:7077`. Normal/recovered failures reuse it at `:5279`/`:5996`; retry creates a fresh task identity at `queries/agent.sql:546`–`:578`. The failed transition at `queries/agent.sql:1418` stores `completed_at=now()` and only applies to active statuses. `notification_listeners.go:1044`–`:1070` currently discards the task identity and writes empty details; fix that propagation.
- Delegated failure exhaustion: the persisted explanation comment at `service/task.go:6242`–`:6269`, with original failed task identity in details at `:6289`–`:6310`. Keep this occurrence distinct from the original task failure.

Activity listeners create new activity UUIDs on each bus receipt (`activity_listeners.go:133`–`:146`/`:283`–`:290`), so activity IDs, receipt times, assignment from/to values and later current-task lookup cannot substitute for these keys. `autopilot_paused` stays outside the approved wake allowlist.

Historical comment mentions retain `details.comment_id` and can be adopted exactly. Historical assignment rows lack committed revision, and historical ordinary failures may have empty details. Arbitrary pre-rollout association cannot be reconstructed; the design therefore uses explicit snooze revision/time baseline fences and a minimal persisted critical-source receipt, rather than claiming new writer IDs adopt unknown old sources. Baseline failure time comes from the actual task's persisted `completed_at`, not from the newly inserted Inbox row.

Final independent review confirmed that all three identified gaps are addressed at the document-contract level and found no remaining concrete contradiction preventing final planning presentation. `task.py validate` passed both manifests (10 entries each), and the seven task artifacts passed JSON/reference, placeholder, ID-inventory and whitespace checks. These are planning checks only; the task remains `planning`, and product implementation, unit/integration tests and runtime/browser acceptance have not started.

Implementation follow-up (2026-11-21): the backend identified that stored `created_at`/`completed_at` use PostgreSQL transaction-start `now()`. A source transaction can begin before Snooze and commit afterward, so a timestamp cutoff would suppress a genuinely new committed source. The coordinator chose a bounded visible-source ID baseline consumed in the existing critical-source receipt table, with committed issue revision watermark for assignment/description. Design/implementation artifacts were corrected and real transaction-interleaving tests required; this supersedes the earlier timestamp-baseline recommendation without changing approved product scope.
