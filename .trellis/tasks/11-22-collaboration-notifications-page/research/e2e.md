# Notification page browser/native acceptance slice

## Changed interaction and canonical selectors

The new page is a route destination, so tests must not retain a redundant button role or old popup behavior. Existing fixture flows that read and then archive the same row should explicitly open All, because Unread correctly removes the row after read.

- Heading: `Collaboration notifications` / `协作通知`.
- Filter navigation: `Notification views` / `通知视图`; native links Unread / All / Archived.
- Row: `[data-notification-id]`, containing a native issue-title selection link plus native `Mark read` / `Archive` buttons.
- List scroll root: `[data-tab-scroll-root="account-notifications-list"]`.
- Detail root remains `[data-tab-scroll-root="account-detail"]`.
- Compact back link: `Back to notifications` / `返回通知列表`.
- Header query-only retry: `Refresh notifications`; it must not replay a failed write.

## Existing obsolete fixture call sites

- `e2e/collaboration-records-web.spec.ts:92,101`: page entry and exact run links; closing run records must retain notification filter/selection while clearing only run anchor.
- `e2e/collaboration-account-desktop.spec.ts:501,510,515`: same behavior in actual account tab, including history/window/restart assertions.
- `e2e/fixtures/collaboration-mention-journeys.ts:327,576`: comment/body notification entry and explicit read retaining the row in All.
- `e2e/fixtures/collaboration-recovery-runtime.ts:688`: proven result links and retained records.
- `e2e/fixtures/collaboration-offline-handoff.ts:172,196,300`: read/archive and later exact old links; return through normal route navigation rather than popup toggle.
- `e2e/fixtures/collaboration-offline-result-notifications.ts:255,317`: same result/read/archive semantics across synthetic offline outcomes.
- `e2e/fixtures/collaboration-recovery-extras.ts:270`: explicit comment-notification read and source anchor. Keep its replay/ownership evidence; no automatic-read claim.
- `e2e/global-scope-navigation.spec.ts:139–150` is already assigned to the route lane.

Search before editing to find additional affected call sites; keep all existing data, original receipt, anchor, no-workspace and ownership assertions.

## Focused new proof

Add a canonical shared Web/Desktop acceptance test using `TestApiClient` and recorded/synthetic notification fixture data; no real agent subprocess or provider account. Use a task-owned API database distinct from DB-backed Go packages.

1. Enter via sidebar link, assert main-page heading, active nav and no notification popover.
2. Traverse Unread / All / Archived; load a second notification page and prove correct membership/count.
3. Select exact issue, comment/run/body source, including multiple notification IDs for the same issue.
4. Mark read, archive, and verify the authoritative badge/list transition and retained authorized preview.
5. Exercise long title, wide master/detail and compact list/detail/back without horizontal overflow.
6. Set actual list/detail scroll positions, navigate away and Back or scope-resume; prove selected notification and painted offsets return.
7. Capture settled screenshots on wide and compact Web plus native renderer; compare with existing app/detail token authority before any UI fix.

Execute against production Web and built Electron renderer/preload using `research/start-qa.sh` and `research/run-qa-command.sh`; keep test credentials in the private registry configuration and record only sanitized evidence.
