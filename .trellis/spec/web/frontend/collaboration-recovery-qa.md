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

## 7. Wrong vs correct

Wrong: use fetch/fulfill for a nonlost native response and count the upstream
HTTP status as client success, then remove a pending key to unlock the form.

Correct: continue the actual browser request, retain its real response, and
settle any unknown original intent through the application's same-key GET
recovery owner before a fresh explicit action.
