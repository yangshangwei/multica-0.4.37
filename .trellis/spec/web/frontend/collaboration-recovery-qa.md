# Collaboration recovery QA

## 1. Scope / trigger

Use this contract for current password API, production Web and real Electron
recovery evidence. `e2e/collaboration-recovery.spec.ts` owns field contention and
batch authoring; each independently accepted run keeps its exact candidate,
database, configuration and complete denominator. Field acceptance does not
accept the authored batch, extra recovery paths or full PAY-241 S3.

## 2. Signatures

- Explicit environment: `PAY241_S3_RECOVERY_ACCEPTANCE=1`,
  `E2E_PASSWORD_AUTH=1`, recorded `DATABASE_URL`/`TASK_DATABASE_NAME`,
  `COLLABORATION_E2E_API_URL`, `NEXT_PUBLIC_API_URL`, production Web origin,
  `COLLABORATION_ELECTRON_SOURCE_ROOT` and matching renderer URL.
- Collaboration priority uses the actual UI `PATCH .../issues/{id}`;
  transfer uses `POST .../issues/{id}/handoffs`. Both retain the original
  idempotency key and four workflow observations.
- Original results use `GET .../issues/{id}/operations/{requestId}`.
  Ordinary child CAS uses numeric `expected_revision` on its canonical PUT.
- Web authenticates through actual password cookies and CSRF. Electron uses a
  password-issued bearer and the existing native renderer/preload fixture.

## 3. Contracts

Capture both genuine UI requests from the same collaboration observation
before forwarding either. Hold an actual request only to establish order; a
normal response reaches the browser through unchanged `route.continue()` and
is collected by exact Request identity. Deliberate committed ACK loss alone
uses real upstream `route.fetch()` followed by abort. Never fabricate business
JSON, response headers, permissions or revision conflicts.

In the retained Electron39.8.7/Chromium142/Playwright1.58.2 environment with the
production `webSecurity:false` preferences, fetch/fulfill produced browser
status0 and empty headers for GET and PATCH despite equal upstream body hashes.
A separate unauthenticated `/health` probe compared natural, fetch/fulfill and
continue paths without business writes. This is a measured harness transport
boundary; it does not establish a product CORS or account defect.

A direct original-operation GET proves the remote result. If the application
still owns a pending intent, retire it through its actual Check original result
control, correlated with the saved key and observations, and require zero POST.
The control can be in a portaled form or the issue action section. A recovered
refusal restores the original input/observation; close the real form before a
fresh intent. Do not clear storage, force controls or recycle the old key.

Ordinary field pickers currently omit optional `expected_revision`. Test those
unmodified UI writes separately from explicit numeric API CAS and the resulting
UI projection. Do not inject a version into an intercepted UI request or claim
mandatory UI CAS. Both historical success/refusal receipts survive later fresh
authorized changes.

Cookie Web authenticates before its successful WS101 upgrade and has no
auth_ack frame. Token Electron requires auth_ack before issue subscribe/ack.
Batch acceptance additionally needs the old physical socket closed, a distinct
replacement, complete authoritative page IDs/versions and actual Load more.
Record frame types/scopes/IDs and header presence; never auth frames or tokens.

## 4. Validation and error matrix

| Observation | Required disposition |
| --- | --- |
| Priority first, old transfer observation | Actual409 stale_revision; winning priority only |
| Transfer first, former noncreator handler edit | Actual403 forbidden; priority unchanged |
| Explicit ordinary child version1 competition | One200/one409 revision_conflict, numeric expected1/actual2 |
| Ordinary UI omitted version | Compatibility evidence, separate from mandatory-CAS claims |
| Upstream200 but native browser0 | Preserve failed run and diagnose transport; not a business success |
| Pending intent plus disabled actions | Exact original UI recovery; no storage reset or forced new request |
| Later successful intent | New key/current observation; prior success/refusal unchanged |
| One failed/unstarted case in a selected round | Whole round not accepted; preserve all denominator identities |

## 5. Good / base / bad cases

- Good: a fresh four-case run retains eight orderings, exact original/fresh
  requests, actual DB relationships, no execution effects and independent check.
- Base: a service/HTTP or authoring check retains its own smaller scope.
- Bad: combine Web passes from failed rounds, interpret a hidden/disabled
  control as permission proof, or accept an upstream result the browser did not
  actually receive.

## 6. Required verification

Field runs preserve exactly4 tops/8 orderings, retry0, complete raw results and
per-order checkpoints. Compare final fields/revisions, actual original keys,
operation IDs/hashes/results, events/handoffs, child activity and unchanged
parent/execution state. Audit runs through their real session→issue relation.
Release/join all held requests and waiters, clean every partially acquired
native/context/profile, and save the primary error before cleanup.

Main records actual database/migrations/indexes/gate and process identity before
entry and after owned shutdown. Restore the allocation's actual booleans and a
monotonic epoch; a template's creation flag can already be true. Protected
history stays retained. Independently run strict targeted TypeScript/ESLint on
the frozen harness; an ignored root E2E file is not lint evidence.

The accepted R06 field run has4 tops/8 orderings, actual8 roots/4 children,
36 operations/40 events/12 handoffs/20 ordinary activity rows, and zero new
run/command/dispatch. Its ordinary omitted-version and empty-execution limits
keep parent TC-16 PARTIAL. The first five failed rounds remain not accepted.
55-comment/WS, notification, upload, revocation and blocking need separate runs.

The separate fresh batch R01 is now independently accepted for both platforms:
each has55 original offline comments plus2 explicitly enumerated controls,
57 unique persistent IDs including one tombstone, one reply and the edited
revision. Both actual passes have50+7 rows with complete body/tombstone and
cursor continuity, not just DOM deduplication. Compare exact persisted
microsecond tuples separately from the UI's existing millisecond/thread order.
After another real disconnect, a second complete quiescent pass preserves the
same ID/revision set and every original receipt, with zero observer business
mutation and zero new run/command/dispatch.

Each observer created four sockets: three authenticated identities (initial and
two replacements), plus one failed attempt while intentionally offline. Keep
that full transport denominator. Web's replacements use actual cookie101 and
issue subscribe acknowledgements; Electron's replacements record auth,
auth_ack and issue subscription in order. These assertions establish persistent
read convergence and real reauthentication; they do not assert delivery of
every invalidation frame. Parent TC-15 can pass this exact batch outcome while
other S3 recovery and structured-block requirements remain open.

## 7. Wrong vs correct

Wrong: use fetch/fulfill for a nonlost native response and count the upstream
HTTP status as client success, then remove a pending key to unlock the form.

Correct: continue the actual browser request, retain its real response, and
settle any unknown original intent through the application's same-key GET
recovery owner before a fresh explicit action.

## Human recovery: evidence boundaries

- An issue WebSocket `subscribe_ack` proves transport subscription, not ordinary
  inbox delivery. Compare notification IDs **and recipients** for the original
  event before and after replay/reconnect. A waiting Agent command is not a run
  or a result notification. Test ordinary subscribers and actual runs separately.
- For a lost upload acknowledgement, match `(uploader, draft, request)` to the
  winning attachment, stored object, original comment/issue references and
  authorized download digest. A same-name, same-size file with different bytes
  must not send another POST. If an upload directory spans rounds, reconcile
  the retained objects from each database rather than treating them as orphans.
  Keep the read-only audit SQL with its results; never record keys or file bytes.
- The file preview and Attachments drawer are separate dialogs. Close each via
  its own control and assert it is hidden before acting on the comment composer.
  A request cancelled after access loss does not prove that a stale response
  reached the renderer or that a slow server-side byte read was interrupted.
- Compare execution state before and after a handoff using the same negotiated
  issue-detail projection as the renderer. Do not drop the protocol version
  from the equality check. An unkeyed recipient search or resolution POST is a
  lookup, not a handoff; retain the requests and reject any unexpected write.
- Password registration for large isolated suites may opt in with
  `E2E_WAIT_FOR_REGISTRATION_WINDOW=1`: only an explicit `429 rate_limited`
  with a valid `Retry-After` of 1–300 seconds permits one identical registration
  after the wait. Never retry business writes or relax the product limiter.

## Ordinary subscription recovery

`e2e/collaboration-recovery-subscriptions.spec.ts` has one complete journey per
platform. Use real source members and an ordinary child, click Subscribe and
Unsubscribe, and retain the original subscriber row, comment UUID and inbox ID.
The author, an existing optout and a nonwatcher are separate delivery controls.
Both comments use the ordinary unkeyed UI submission path. Verify the first
notice's read state and exact comment anchor through physical workspace socket
replacement and reload, then prove no delivery for the distinct second comment.

Tiptap's empty-editor placeholder decoration disappears after typing. Retain
the original editor and its actual submit owner before filling; assert node
identity, connected state, containment and a unique enabled Send within that
owner before and after fill. A page-wide Send locator is ambiguous when a reply
composer is also present. Keep structural diagnostics bounded and content-free.
The inbox displays the comment preview; do not invent a literal notification
label that the product does not render.

Keep every transport request in the evidence. Only POST plus the exact pathname
`/api/issues/table/rows` or `/api/issues/table/facets` is classified as a query
read for strict business-operation equality: both handlers use
`beginIssueTableSnapshot` with `REPEATABLE READ READ ONLY`. Other POST routes
retain their existing classification. Ordinary original-event listener replay
has a separate server regression; a browser reconnect alone does not prove it.

## Controlled runtime recovery

`e2e/collaboration-recovery-runtime.spec.ts` has four tops and ten subcases:
success/failure/Stop notification recovery and author/resource-grant withdrawal
on each platform. The full allocation expects ten controlled Provider turns and
twelve original human inputs. Record actual denominators and retain incomplete
rounds; readiness probes and queued commands do not count as Provider turns.

Activate a fresh collaboration session with the existing issue-scope Resume
before sending initial inputs. A target Resume cannot clear an issue's initial
paused state and returns the original `409 issue_paused` receipt. Subsequent
target controls remain target-scoped. Preserve original run, command, attempt,
grant, capability and reservation identities through recovery and withdrawal.
Use the original issued business token for before/after requests, retain its
hash and refusal codes, and never substitute a newly issued credential.

Run notification navigation selects and automatically opens the exact run's
records. The same toggle has accessible name `View records for <full run UUID>`
when closed and `Hide records for <full run UUID>` when open. Match both anchored
names inside the exact run disclosure, inspect `aria-expanded`, and retain all
panel, launch, host-mode, result and proof assertions.

Bind explicit test-built daemon, CLI and Provider executables. Observe actual
process descendants with full command width and kernel executable paths; retain
earlier path/role observations when an exited process is no longer inspectable.
Classify probes using their original observation, since final process arguments
may lose their flags. Disclose genuinely unobserved short processes and
reconcile Provider turns with original run/attempt ledgers and provider records.
Reservation row counts alone do not prove release: retain the exact run-linked
release timestamp, revoked capability and terminal observation for every run.
