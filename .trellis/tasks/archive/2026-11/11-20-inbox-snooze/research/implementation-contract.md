# Frozen implementation interfaces

The user approved the final planning summary on 2026-11-21. The existing task is now `in_progress`. These names make the independent backend/Core/Views implementation lanes compatible; they refine the reviewed design without changing product scope.

## HTTP and projections

- `GET /api/inbox/snoozed` returns the same raw item array shape as active/archived lists.
- `POST /api/inbox/{id}/snooze`: `{ "snoozed_until": "RFC3339 instant", "expected_revision": 0 }` (revision is the currently observed nonnegative integer).
- `POST /api/inbox/{id}/unsnooze`: `{ "expected_revision": 1 }`.
- `POST /api/inbox/{id}/snooze/undo`: `{ "undo_token": "server-issued opaque token" }`.
- Existing ownership/workspace gates apply to the real item anchor. New operations await a server success; HTTP 409 refreshes current data while retaining an actionable error.

Additive optional `InboxItem` fields: `snoozed_until: string | null`, `snooze_revision: number`, `reminded_at: string | null`, `reminder_reason: string | null`. Missing legacy fields mean no snooze and revision 0. Unknown reminder reasons must render defensively.

Mutation receipt (schema-validated; essential identity cannot use an empty success fallback):

```typescript
interface InboxSnoozeReceipt {
  workspace_id: string;
  group_kind: "issue" | "item";
  group_id: string;
  revision: number;
  snoozed_until: string | null;
  reminded_at: string | null;
  reminder_reason?: string | null;
  undo_token?: string | null;
}
```

`undo_token` is returned for the last reversible user operation, not an expired/background action. Server-side snapshot and single-consumption/version checks own undo. A failed receipt parse cannot enter the success/undo branch.

## Core public names

- `inboxKeys.snoozed(wsId)` and `snoozedInboxListOptions(wsId)`.
- `deduplicateSnoozedInboxItems(items)`; active dedup excludes future snoozes, archived dedup retains its current group contract. Reminder order uses `reminded_at` while a group's representative remains its newest original notification.
- `useSnoozeInboxItem(wsId)`: `mutateAsync({ id, snoozedUntil, expectedRevision })` → receipt.
- `useUnsnoozeInboxItem(wsId)`: `mutateAsync({ id, expectedRevision })` → receipt.
- `useUndoInboxSnooze(wsId)`: `mutateAsync({ id, undoToken })` → receipt.
- Client API names: `listSnoozedInbox`, `snoozeInboxItem`, `unsnoozeInboxItem`, `undoInboxSnooze`.
- Pure time helper exports through `@multica/core/inbox`: `inboxSnoozePresets(now)` → `{ key, date }[]` with keys `one_hour`, `three_hours`, `tomorrow`, `three_days`, `one_week`; `toInboxLocalDateTime(date)` → datetime-local string; `parseInboxSnoozeDateTime(value, now)` → future `Date | null`, rejecting nonexistent normalized local times. Use local-calendar 09:00 for day presets and absolute durations for hours. Views owns localized labels and actual IANA timezone display.

Views uses only these public hooks/types. Core owns its own tests, schema/client, realtime and tab subject files; Views does not edit them.

## Realtime

Use `protocol.EventInboxUpdated = "inbox:updated"` as the content-minimal personal invalidation for snooze, edits, undo, suppression and reminder state transitions. Payload: `workspace_id`, `recipient_type`, `recipient_id`, `group_kind`, `group_id`, `revision` (optional `reason`). Route through the same authenticated personal delivery as existing inbox events; also fix the missing `inbox:unread` registration.

All affected source-workspace list keys and the account unread summary are invalidated, independent of the active workspace. Ordinary suppressed notifications produce no banner-triggering `inbox:new`. A genuine important delivery can publish its normal `inbox:new` only after the shared transaction commits; due/manual transitions must retain original notification identity and never create a new Inbox row. Poll/focus/reconnect recovery continues to work if a state event is missed.

## Agent ownership and verification

- Backend owner: all task-specific `server/` changes, migrations and Go tests. Do not execute handler/server packages until the coordinator provides a recorded owned database and explicit test permit. Do not create/migrate/drop/switch it yourself.
- Core owner: task-specific `packages/core/` changes and tests.
- Native identity follow-up: actual Desktop `apps/desktop/src/renderer/src/stores/tab-store.ts` preserves canonical Inbox `view=snoozed/archived` in resource identity, ignoring notification-selection query noise. Core tab presentation uses the existing localized Snoozed title through a minimal Inbox title descriptor; active/Archived display remains as before.
- Views owner: `packages/views/inbox/`, Inbox en/zh-Hans resources and component tests; the bounded Snoozed title query branch in `packages/views/layout/tab-presentation.tsx` and its test; optional sizing props in common/row-actions-menu.tsx and its test, used only for Inbox coarse-pointer hit targets. Reuse existing UI primitives; request scope expansion before editing `packages/ui/`.
- Coordinator: task artifacts, environment allocation, integration/E2E evidence, final independent review, specs and scoped commit/finish.

All agents are already dispatched implementers and must implement directly, not recursively dispatch. Preserve other concurrent edits. New behavior tests should fail for the missing feature before implementation. No new dependencies, unrelated cleanup, destructive git changes or agent-side commits.
