# 协作通知独立页

## Goal

Replace the sidebar notification popover with a dedicated, account-owned collaboration notifications page. Users can review and process cross-workspace notifications in a stable list/detail layout on Web and Electron.

## Background and accepted direction

- The current sidebar renders a roughly 384px notification popover instead of navigating to a page (`packages/views/layout/app-sidebar.tsx:650`, `packages/views/collaboration/notifications.tsx:61`).
- The preceding proposal's Option 1 specified an independent page, Unread / All / Archived views, a wide list/detail preview and retained return position.
- The user approved that option and explicitly requested task creation and execution: “请按照方案1 独立通知页（推荐），新建trellis任务，开始推进”. This approval is the product direction and execution authorization; there are no blocking product questions.

## Requirements

- **R1 — Page navigation:** Clicking Collaboration notifications navigates to the dedicated account page and highlights the sidebar item. Keep the protected unread-count observer available when the collaboration directory is collapsed. Preserve existing Beta/disclosure behavior and account access without a workspace. The default entry opens Unread.
- **R2 — Complete filtered reads:** Offer Unread, All and Archived. Unread means unarchived and unread; All means unarchived read plus unread; Archived means archived rows. Apply filtering before server cursor pagination. The unread badge always counts unarchived unread notifications, independent of the selected view. Existing legacy `include_archived` requests remain supported (`research/api.md`, notification SQL:11–28).
- **R3 — Readable master/detail:** Wide layouts keep a scrollable notification list beside collaboration detail. Compact layouts show the list or detail with an explicit back-to-list action. Rows show the existing title, event type, timestamp and identifiable unread state. Selection must not remount/reset the notification list.
- **R4 — Exact targets and unavailable content:** Select by notification UUID separately from issue UUID. Preserve exact run, comment and description targets. Removed/inaccessible records remain content-free and cannot trigger private issue reads; unknown event types retain a generic label. The existing `account-detail` container and anchor-recovery behavior remain authoritative (`research/routes.md`, `collaboration-detail.tsx:42–76`).
- **R5 — Truthful processing:** Keep explicit, per-notification Mark read and Archive actions. Await confirmed writes, expose pending/failure feedback, and invalidate all notification views so badges and list membership converge. Archive does not implicitly mark read. A selected collaboration preview may remain open when its notification leaves Unread/All after a confirmed action; recover keyboard focus when a control disappears.
- **R6 — Return restoration:** Browser/native Back and Forward and the existing Collaboration scope resume restore route-encoded view/selection and platform scroll mementos. Explicit Notifications sidebar navigation opens the default unread list; no additional section-resume store is required.
- **R7 — Shared delivery and compatibility:** Web and Electron use the same page and validated account URL. Titles/icons identify notifications. New filtered reads reject a missing/mismatched response view marker rather than silently presenting legacy-server default rows. Default/All reads continue accepting legacy envelopes.

## Acceptance Criteria

- [x] **AC1 / R1,R7:** Sidebar activation opens `/account?view=notifications&filter=unread`, shows the notifications title and active item, and creates no notification popover on Web/Electron. Unknown counts do not render as zero.
- [x] **AC2 / R2,R7:** Each filter returns only its defined state across multiple cursor pages, including tied timestamps and another recipient's sentinel rows. All filters report the same global unread count. Old include-archived requests retain mixed history; invalid filters and ambiguous explicit/legacy filter combinations fail safely.
- [x] **AC3 / R3,R4:** Wide list/detail and compact list/back/detail flows are usable with long titles and keyboard/coarse-pointer controls. Opening two notifications for one issue changes the selected notification and exact anchor without resetting list scroll.
- [x] **AC4 / R4:** Comment, run and body notifications open the intended target; removed/unavailable notifications display no private title, source or run and initiate no private detail fetch.
- [x] **AC5 / R5:** Confirmed read/archive operations update list membership and sidebar count. Failed writes remain visible and can be explicitly retried; a refresh retry never repeats a failed write. Focus has a reachable recovery target when a selected row/control disappears.
- [x] **AC6 / R6:** Back/Forward and workspace↔Collaboration scope resume restore filter, notification selection and painted list/detail scrolling on both platforms.
- [x] **AC7 / R3,R7:** Loading, zero-result, pagination-error and unavailable-server states retain visible recovery/navigation; locale parity, affected route/schema/page/platform tests, lint, typecheck and Go static checks pass.

## Out of scope

- Merging with the workspace inbox; mobile UI changes; notification generation/aggregation; bulk-read or unarchive actions; new actor/workspace display fields; new dependencies; schema migrations; publishing or production deployment.
- Existing unrelated working-copy changes remain outside this task (recorded in `research/baseline.json`).

## Evidence

- `research/routes.md`: account destination union, native path constraints, exact content targets and existing restoration seams.
- `research/api.md`: pre-pagination filtering, privacy fences, global unread semantics and legacy-server response identity.
