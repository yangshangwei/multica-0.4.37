# Archived notification denial loop: repair boundary

2026-11-23. The source is the task's private `src` at `73a28ed90` plus approved readiness fixes. Core has stopped all acceptance app processes and released the source freeze; the owned database remains for later replay. No assertion wording change can resolve the reproduced request storm.

## Evidence and cause

`core/offline-target-diagnostic/playwright.log` records the correct old destination: notification view `all`, the archived transfer notification ID, and the removed issue ID. The detail area is visible, has no private title, but contains 24 characters and neither expected denial message. `request-counts.json` records, over the 14.55-second case, 314 notification 200s, 352 notification 503s, 84 issue 404s and 524 issue 503s. This is not ordinary polling or a missing target selector.

The archived notification is intentionally absent from the All view. `notifications-page.tsx` permits its exact issue to use the canonical access boundary, and binds issue realtime while parent list data is present. A `subscribe_error: forbidden` enters `recoverAccess` in `core/realtime/collaboration.ts`, unconditionally clears private issue and non-issue account projections, including notification list data, then refreshes account reads. Clearing the parent list makes the page's `readableIssueId` null and unbinds issue realtime. The refreshed list makes it eligible again; a new binder loses its local `subscriptionDenied` flag and subscribes again. Another forbidden event repeats the purge/refetch cycle. The HTTP protection already avoids repeating `clearCollaborationAccess` when its retained issue denial is true, but the realtime subscription-error path does not.

The first post-removal comment notification is still present and redacted, so `Issue unavailable` is correct there. The archived missing-row target must instead reach the stable canonical denial. Both original and attempted alternate wording failed at that final target because of the loop; the previous mistaken inference that the original failed at the first assertion is withdrawn.

## Smallest proposed repair

Keep the fix at the realtime denial boundary: record `subscriptionDenied=true` for every forbidden response, but perform `recoverAccess` only when that issue is not already denied in the retained `QueryClient` access state. First-denial privacy clearing, epoch fencing and HTTP recovery remain intact. A remounted already-denied subscription cannot purge the newly recovered parent list again. Genuine account membership invalidation still revalidates the denied subscription; an acknowledged subscription still refreshes the authoritative issue read.

Expected write scope: `packages/core/realtime/collaboration.ts`, its canonical `.test.ts`, and a real-binder integration regression in the notification page test if needed. No global retry suppression, private-data retention, new access store or altered server authorization. The page and core HTTP recovery paths should remain unchanged unless the regression demonstrates a second required owning fix. The parent has authorized this scoped product correction.

## Required regression and replay

Use a real QueryClient and actual realtime binder with a deterministic forbidden transport. Exercise the parent notification-list data dependency and binder recreation, prove bounded subscriptions/GETs and retained denied state with no private fields, then simulate genuine renewed permission and verify explicit recovery/ack can restore authorized reads. Also cover repeated forbidden without binder recreation and ensure new denial after a successful recovery clears data again.

Run the canonical core suite red before the fix, then green; run the affected notification/access/realtime suites and scoped lint/typecheck. Preserve the diagnostic and earlier false wording attempts. After file-ready notification, core replays both unchanged Web/Electron offline-handoff originals with their actual archived-link denial and all API404/redaction/nonmembership checks.

## Implemented fix and local checks

The parent approved the canonical subscription-error guard. Only two private source files changed:

- `packages/core/realtime/collaboration.ts`: import the existing retained denial predicate, set `subscriptionDenied=true` as before, and call `recoverAccess` only if the issue is not already denied. All `access_changed`, source-membership, account invalidation, reconnect, acknowledgement, HTTP recovery, and first-denial privacy paths remain unchanged.
- `packages/core/realtime/collaboration.test.ts`: reuse its real QueryClient and actual binder, expose a real dispose/rebind step in the existing setup, and add one regression. It verifies the first denial clears private issue/list/draft data; repeated and remounted forbidden events preserve freshly redacted parent data and do not issue more reads; genuine membership revalidation and successful authoritative HTTP recovery can clear the denial; a subsequent fresh refusal clears private state again.

No notification UI conditional, new store, artificial polling deadline, or global retry setting changed. The existing browser failure provides integration red evidence; the new canonical binder test owns the repeated-denial/recovery matrix, avoiding a second synthetic DOM implementation of the same loop.

Logs under `.omx/private/qa-work/main-readiness-20261123/notification-denial/`:

| Evidence | Result |
| --- | --- |
| `core-red.log` | Original 11 realtime tests passed; new regression failed because repeated/refreshed binding advanced epoch to **3 instead of 1**. |
| `core-green.log` | **4 files / 27 tests passed**, including complete realtime, collaboration queries, notification queries and query-access suites. One worker, no file parallelism. |
| `lint.log`, `lint.exit` | Scoped ESLint max-warnings=0 passed, exit 0. Existing pnpm configuration notice only. |
| `typecheck.log`, `typecheck.exit` | Complete Core typecheck passed, exit 0. |
| Scoped `git diff --check` | Passed. |

The parent independently read the product guard and approved its exact boundary. The two files were frozen and announced ready to core before any rebuilt browser replay. Exactly **one new unit case** was added; the four-suite replay's 27 successes must not be added wholesale to prior package counts.

Final hashes:

```text
9b6d86ee975a345d7bd1023b854def7dc6748341e76ace4551518a73bcf2c6dc  packages/core/realtime/collaboration.ts
11757ec64499a74dc257061ef5cb467b3d746cbd8e628a49680e8726392b62d7  packages/core/realtime/collaboration.test.ts
```

Production Web/Electron original replays and bounded actual request counts remain pending rebuilt acceptance; this local result does not itself claim the browser storm is closed. The final fixture must retain its original generic canonical denial for the archived missing row and `Issue unavailable` for the known redacted comment row, plus private-content absence, API404, redacted notification and zero-membership assertions.
