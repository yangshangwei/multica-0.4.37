# Confirmed iteration operations

The I1 APIs use complete server previews and durable request identities. Shared
implementation lives in `packages/core/iterations` and `packages/views/iterations`;
Web and Desktop only wire routes/platform adapters. Rollout defaults off.

## Request and permission boundaries

- `prepareIterationDraft` derives remaining work and starting terminal choices
  from one complete preliminary preview, then requests the final confirmation.
  Do not replace it with one HTTP request per issue or client-visible pages.
- `useIterationCommand` persists the complete original payload and request ID
  under server/actor/workspace/action identity. Unknown results recover through
  GET or the same exact POST, never a newly generated ID.
- Distinguish a lookup `operation_not_found` from a write `iteration_not_found`.
  The former retains the original identity; the latter is definitively rejected
  and permits editing a new intent.
- `isIterationAccessDenied` includes authenticated 401/403 and the middleware's
  explicit `404 workspace_access_denied`. That 404 must not become an old-server
  capability fallback or trigger a recovery POST. Other resource404s do not
  revoke the whole workspace.
- `protectIterationRead` fences late responses with an authorization epoch and
  captured API session scope. Check the session before returning data or handling
  an access error: a delayed old-account 403 must not clear the new account
  query cache or persisted commands. Pass Query AbortSignal to underlying reads.
  On revocation, cancel queries with `revert:false` before erasing protected
  data; normal cancellation can otherwise restore the stale successful result.
  Clear mutation payloads, persisted pending commands and active observer data.

## UI ownership and recovery

- Key period detail/workspace and issue assignment boundaries by stable entity
  identity, not revision. A cached A→B navigation must not reuse A's hidden
  issue ID, editor fields, cursor or preview.
- Lock inputs and confirmation while a preview is in flight (or explicitly
  discard obsolete generations). Clearing an old preview on input change is
  insufficient if its unresolved request can reinstall it later.
- Keep pending recovery independent of current period status, workspace enabled
  state and rollout availability. A WebSocket can announce completed before a
  successful HTTP response is lost. `IterationRecovery` still reads the exact
  original operation on the closed/reloaded page; status alone proves nothing
  about which request committed.
- `pending-signal.ts` only signals durable client command changes; server state
  remains in Query. Its external-store snapshot is a stable revision number.
- Ordinary issue/task/project/member/status events also affect iteration
  projections. The realtime classifier invalidates them before the generic
  event handler's specific-event early return, with workspace-scoped debounce.

## Route and display integration

Adding a route also requires desktop tab subject/presentation/icon/name lookup,
diagnostic path bucketing, editor internal-link registration and their parity
tests. Router wiring alone can render a working page under an "Unknown page" tab.
Use saved period timezone for historical timestamps, localized frozen categories,
and actionable validation text. The name limit counts Unicode code points.

Canonical regressions: core `command.test.tsx`, `access.test.ts`, `prepare.test.ts`,
`realtime.test.ts`, the mounted realtime WS-instance suite, views iteration
navigation/operation/assignment suites, and `e2e/iterations-i1*.spec.ts`.
Native E2E must proxy WebSocket upgrades as well as HTTP; otherwise assertions
on server state can pass while the actual rendered statistics stay stale.

Assignable iteration choices require both a supported manual mode and a planned
or active status. Unknown modes remain visible through read-only catalogue/detail
surfaces but must not become editable through issue create or batch selectors.
Optional legacy rollover counts and historical metadata remain unknown; never
render missing values as known zero or an empty label collection.
