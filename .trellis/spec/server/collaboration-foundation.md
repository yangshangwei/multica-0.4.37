# Cross-workspace collaboration foundation

## 1. Scope / Trigger

Applies to the in-progress collaboration implementation beginning at migration
571. This is a development foundation, not a released feature. Public creation
must remain unavailable until the task's P1–P8 boundaries and acceptance pass.
The runtime supervisor, full FIFO, private-content paths, and Web/Desktop UI
are not proven by the foundation tests.

## 2. Signatures

- `auth.WithAuthenticatedPrincipal(ctx, AuthenticatedPrincipal)` is called by
  successful authentication branches. A private context key records the verified
  JWT/PAT/task/cloud credential; request actor headers cannot supply it.
- `collaboration.CapabilitiesFor(Principal, AccessState)` separates current
  source membership, participation, creator, and handler. It is not execution
  admission or proof that a run can start.
- `CollaborationService.ResolveIssueAccess(ctx, qtx, issueID)` explicitly checks
  current object read authority. Its internal result is not an HTTP DTO.
- `Create(ctx, CollaborationCreateParams)` owns a transaction; `CreateInTx(ctx,
  tx, params, IssueCountPolicy)` borrows it and a policy resolved before locks.
- `GetCreation(ctx, sourceWorkspaceID, requestID)` recovers a lost response by
  its original actor-scoped key and current object access.
- `Reorder(ctx, issueID, position, expectedRevision)` is the source-only R58
  layout operation. It does not lock a session or advance collaboration revision.
- `ListWorkbench(ctx, CollaborationWorkbenchQuery)` keeps current responsibility
  and active participation separate. Its page and count use the same authorized
  database snapshot, including when the viewer has no workspace membership.
- `EditIssue(ctx, CollaborationEditParams)` edits title/body/calendar dates using
  the shared workflow owner; `PATCH /api/collaboration/issues/{id}` accepts either
  metadata or the existing status operation, never both in one request.
- `StatusOptions(ctx, issueID, action)` projects lifecycle/manual status choices
  and the prescribed target for `request_information`, `return_information`, or
  `submit_acceptance`. Directed writes carry `expected_status` and
  `status_options_version`; returns also identify the saved information request.
- `NewCollaborationAccountReconciler(q, tx)` supplies `Reconcile(ctx, limit)` and
  `Run(ctx)`. Handler construction owns its dependencies; main starts it with the
  server sweep context independently of creation, execution and admin switches.
- `SubmitCommand`, `CancelWaitingCommand`, `StopQueue`, and `ResumeQueue` in
  `collaboration_queue.go` are internal transaction owners. Resume currently
  persists refusals only; there is no shared launch, promotion or positive Resume
  entry point. Do not expose these as complete execution APIs.
- `GET /api/collaboration/issues/{id}` and `GET /api/collaboration/creations/{requestId}?source_workspace_id=...`
  live in the Auth-only account group. They discard ambient workspace headers,
  return `Cache-Control: no-store`, and authorize the object/current actor.

The read DTO is separate from `IssueResponse`: `collaboration_revision`,
`lifecycle_epoch`, and `session.control_version` are positive decimal strings.
`source_workspace` contains only id/name/identifier; creator/handler summaries
contain user_id/name/avatar_url/external, never email or username. The shared
core parser in `packages/core/collaboration/schema.ts` rejects malformed critical
identity, authority, queue states and signed-64-bit versions as unavailable. It
always uses `parseWithFallback(..., {redact: true})`. Old Issue revision remains
a number, including in ordinary/collaboration mixed projections.

The core API transport also redacts collaboration HTTP error values and query
strings. Schema redaction alone does not cover `Response.json()` syntax errors
or transport logging. Collaboration reads discard results from a superseded
account/server and check returned issue/source identity against the request.
An authoritative current 401 retains normal authentication-expiry handling.

## 3. Contracts

Migrations 571–612 add collaboration storage and forty separate concurrent
indexes. There are no new foreign keys or inline PK/UNIQUE index builds. The
permanent session is unique per issue, command sequence per session is unique,
one run belongs to each command, and at most one unsettled run occupies a session.
`settled_at` cannot be set while `proof_state='unknown'`; a non-unknown enum alone
is not evidence of real process termination, which P4/P5 must validate.

Creation stores the source issue, participants, paused session, event/notification
intent, and completed operation in one transaction using `IssueService.CreateInTx`.
It never calls the ordinary create wrapper, publishes ordinary creation events,
or invents a source-space membership for an external handler. The gate row starts
with both `enabled=false` and `creation_enabled=false`.

A completed creation key does not depend on its old handler staying enabled or
the source membership/creation switch remaining open. Rebuild its response from
current authorized data; never replay an old private response or change keys after
an uncertain commit. The actor and payload digest remain part of the identity.

A valid creator who leaves the source workspace retains complete/cancel/reopen
authority through participation. Source structure/reorder authority is lost.
An external handler cannot turn an ended issue back into an open one through
ordinary status editing. Unknown/machine credentials never inherit human rights.
Verified JWT/PAT requests with either nonempty `X-Agent-ID` or `X-Task-ID` are
also excluded from human collaboration authority. Authentication records this
as `AgentActor`, a deny-only hint; it cannot grant authority or prove an agent's
identity. Both password and legacy authentication preserve this distinction for
reads, workflow writes, recovery and source-side search projections.

`loadIssueForUser` refuses legacy collaboration mutations. `UpdateIssue` alone
uses the explicitly checked loader variant and permits only position plus optional
expected_revision. Batch writers reject the whole selected batch before side
effects. Legacy comment editing/deletion/resolution and rerun paths stay inert.
`DeleteWorkspace` checks retained collaboration records under its existing delete
lock before cleanup, including records whose issue is already done/cancelled.

This is not the complete old-writer inventory: system writers, execution purpose
routing and controlled-content APIs still need individual verification. Do not
open creation based solely on these guards.

Machine/unknown-principal search and list projections must run before private
matching, ordering, snippets and counts. Search only matches collaboration title
or number. List reference rows omit private fields; explicit private filters
exclude them, implicit open-only filtering must not reveal their private status,
and private ordering dimensions use constants. Grouped lists cannot group by
private collaboration dimensions and omit these rows for machine principals.
The sole reference shape is id/workspace_id/number/identifier/title plus
`collaboration_reference=true, can_open=false`. Ordinary rows retain their
existing projection and filter semantics.

Queue input stores immutable content/author and account/participant/lifecycle
epochs, allocating sequence under the source issue/session locks. SQL arithmetic
must explicitly return `bigint`: sqlc can otherwise infer an integer expression
as `int32`, silently narrowing the Go contract. Cancellation only changes a
waiting command and leaves its sequence, original content and session control
unchanged. Current actor authorization is checked before reading operation
receipts; same-key different payloads are rejected.

Stop first pauses the session, invalidates claim generation and records durable
stop intent. It preserves run fence and launch nonce for historical termination
identity. The only settlement implemented here is the server-proven absence of
any launch permit (`authorized_attempt_seq=0`, no started report, unknown proof,
known preparation progress and no success outcome before writing `ending`).
An inconsistent running/success report retains the barrier. The SQL settlement
helper repeats the immutable guards but does not replace this original-evidence
check under the run lock.
Once any permit exists, business success, timeout, task state or missing process
information cannot settle the run. Durable Resume rejection cannot become a
deferred action after state changes. Either an active-run pointer or an unsettled
run causes a durable `stop_unconfirmed`, even when they disagree. Receipts with
no command omit command_sequence; real command/control versions serialize as
exact decimal strings. An uncertain commit is recovered using the original
request key, never an automatic retry of the mutation.

`auth.RevokePasswordCredentials(ctx, tx, userID, revocationKey)` now advances
existing collaboration account fences and writes durable invalidation intent in
the same security transaction. Stable keys are `password:<session version>` or
`admin:<operation UUID>`; a legacy account may have no password row, so nullable
credential_version is provenance, not the deduplication key. Migrations 613–616
retain these intents, including reconciled ones, behind guarded rollback. Normal
login/logout, individual token operations and workspace departure do not invoke
this global revocation path. Ordinary accounts with no fence acquire no rows.

The common helper must invalidate before its no-runtime early return and must
not lock source issues while holding the user lock. Private-schema admin tests
clone both fence and invalidation tables; a public search_path fallback must not
redirect fixture security writes into public tables. Processing the persisted
invalidation fanout is separate from the security transaction. Native containment,
sealed-attempt proofs, runtime enrollment and daemon purpose routing remain
release prerequisites.

Migration 617 adds `last_issue_id` to each retained invalidation. A reconciliation
step locks one pending intent, then workspace/issue/collaboration/session in the
source order; source intersections use NOWAIT. Cancellation/revocation/stop/event
and the UUID cursor commit together. The next account epoch cannot overtake an
earlier pending one. A failed account is excluded only for the current bounded
pass so another account can progress. Migrations 618–619 persist
`last_attempted_at` and its concurrent pending-scan index; polls order by
`COALESCE(last_attempted_at, created_at)`. Successful progress stamps the attempt
with its effects. After a failed/unknown source transaction has rolled back,
one independent, intent-only `SKIP LOCKED` update records the attempt without
moving its cursor. This prevents a full batch of failed accounts from starving
later accounts across batches or restarts. A concurrent owner records its own
attempt; scheduling never waits on that owner or replays source effects. A failed
scheduling update is reported alongside the original error. New scans recover
the stored cursor. Migration rollback refuses retained intents, including
completed ones.

Fanout compares command/grant authority with the immutable intent epoch, not the
account's latest enabled state. It preserves fresh commands/grants, roles and
membership after rapid disable/restore, and uses `stopRun` for obsolete possibly
live work. Existing forced roots and unknown barriers survive. Already-ending
runs do not repeatedly advance claim generations. Source writers must validate
current account epochs under their shared user locks so obsolete work cannot be
inserted behind a committed cursor; future grant/promotion writers must preserve
this condition. Logs contain bounded outcome/SQLSTATE codes, not raw DB errors.

Invitation, transfer, removal and leave use `collaboration_workflow.go` and
actor-scoped durable operations. They lock and recheck current users/roles/epochs,
preserve source membership independence, and commit participation, handoff facts,
notifications, waiting cancellation and resource withdrawal atomically. Transfer
never changes session execution. Removal returns a removed handler to creator;
creator cannot leave, and a handler must transfer before leaving. Re-enrollment
advances the revoked participant epoch and never revives old commands/grants.

Workflow HTTP input carries Idempotency-Key and four positive int64 strings:
expected_collaboration_revision/lifecycle_epoch/participant_epoch/account_epoch
(each epoch key has the expected_ prefix). Detail write_context contains the
current participant/account epochs only when both exist; absent context has no
invented defaults. Replies contain code and collaboration_revision. A departed
actor's operation recovery exposes only that minimal historical receipt, never a
fresh issue projection. Never convert a refused operation into a new request.

Directed handoffs live in `collaboration_handoff.go` and reuse the same owner.
`POST .../handoffs` requires `purpose` (`transfer` or `information`); a missing
purpose is a 400, never an implied transfer. An information request moves
responsibility to the target, keeps a blocked-category key or selects canonical
`blocked`, and stores requester, previous status and parent. Only the current
recipient-handler may push a nested request; any other request or ordinary
handler change supersedes the whole open chain. `information-returns` takes the
open top request ID plus `expected_status` and `status_options_version`; the
recipient comes from the stored requester, a removed/disabled requester is
`return_target_unavailable` with no re-invite. All directed actions compare the
exact options shown for the original intent. The digest binds the source,
action, relevant catalog facts and saved return-request identity; a same-key
rename, category/archival change or different restore target returns
`status_options_changed` with current authorized choices. Replaying that key
retains the refusal even if the catalog is later restored. An ordinary transfer
rejects status confirmation fields. A return restores the parent request when it
was nested. `acceptance-submissions` hands a noncreator handler's result to the
creator with an in_review-category key and supersedes the chain; it never
completes. System status keys cannot be archived, so restore fallback to
canonical `in_progress` applies only to archived custom keys. None of these
actions touches queue, run, command or grant state. Detail exposes
`current_information_request` and `request_information`/`return_information`/
`submit_acceptance` capabilities; clients treat missing values as unavailable.

Metadata editing rechecks active creator/handler authority under the same
workflow locks, without requiring source membership. It retains the command
input, responsibility, information chain, grant, lifecycle/authority epochs and
queue state. Issue values, collaboration revision, required I1 facts, history and
receipt are atomic. A no-op stores only its receipt. History has before/after,
the real actor and a database post-lock `edited_at`; display must not substitute
the caller's time. Full history reads and newly added mention notifications are
still P3 work, not supplied by the metadata endpoint.

Text omission/null preserves the current value; an empty string sets empty text.
Date omission preserves, while null/empty clears. Reuse `ParseCalendarDate`,
including the ordinary UTC-midnight RFC3339 input compatibility. Details return
date-only strings or explicit null. Core `editCollaborationIssue` preserves
omission/null and original versions/key; missing dates from an older server are
unknown display values, never automatic clear instructions. Both metadata and
status PATCH use one strict JSON decoder; explicit null fields count when
rejecting a mixed operation.

The shared stopRun helper distinguishes a cancellation intent from its final
disposition: only proven never-launched work can be cancelled; any possibly live
run is stopped. Existing forced roots remain unchanged. This matters for
participant removal racing a launch permit, even without a started report.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/machine/restricted principal creates | 403 forbidden before dependencies |
| Source membership absent for a new creation | 403 forbidden, no partial rows |
| Creation gate closed | 503 collaboration_unavailable |
| Same completed key and same payload | Same issue, fresh authorized projection |
| Same key with changed payload | 409 idempotency_conflict |
| Unrelated/disabled/machine reads private issue | 404 not_found |
| Legacy mutation targets collaboration | 409 unsupported_collaboration_action |
| Mixed legacy batch contains collaboration | Whole batch refused before mutation |
| Source workspace contains collaboration history | 409 workspace_has_collaboration_records |
| Retained data during down migration | Refuse rollback |
| Uncommitted writer during down migration | NOWAIT lock refusal, before emptiness check |
| Same queue key and same payload | Original content-free receipt, no new command |
| Same queue key with different payload | 409 idempotency_conflict |
| Cancellation after promotion | 409 command_not_waiting, no implicit Stop |
| Stop after any issued permit | Paused queue, unsettled run, no fabricated proof |
| Resume with an unsettled execution | 409 stop_unconfirmed, no deferred intent |
| Resume without completed runtime admission implementation | Refusal; never writes active |
| Handoff without a known purpose | 400 invalid_handoff_purpose |
| Return of a consumed, superseded or nonrecipient request | 409 information_request_not_active |
| Return when requester left or is disabled | 409 return_target_unavailable, no re-invite |
| Directed handoff omits a valid status/digest | 400 invalid_status_request |
| Directed target, catalog facts or saved request differ | 409 status_options_changed; original key stays refused |
| Ordinary transfer carries directed status fields | 400 invalid_request |
| Acceptance submitted by the creator-handler | 409 already_creator |
| PATCH mixes metadata and status, including explicit null | 400 invalid_request; no writes |
| Active creator/handler edits content/dates without source membership | Allowed after current role/version checks |
| Older invalidation encounters freshly authorized work | Preserve work at or above the intent epoch |
| One account's source cannot be reconciled | Retain cursor, report failure, process other accounts within the bound |

## 5. Good/Base/Bad Cases

- Good: two concurrent identical creation requests return one source issue and
  one paused session; external handler has no source-space member row.
- Base: ordinary issue batches and empty ordinary workspace deletion retain
  existing behavior. Old numeric Issue/comment revision wire types stay intact.
- Good: after disable/restore, the consumer cancels old waiting inputs and requests
  stop for the old run while retaining a fresh command and its unknown barrier.
- Good: a status name changes without changing its key; the directed handoff
  refuses the displayed old digest and requires a new explicit request.
- Bad: treating a creator's source departure as participant revocation, calling
  ordinary rerun to resume a shared queue, or equating PGID disappearance with
  proof that detached descendants ended.

## 6. Tests Required

- `internal/collaboration/access_test.go`: independent membership/participation,
  departed creator, ended-handler status bypass, unknown/machine/restricted users.
- `internal/middleware/auth_principal_test.go`: verified credential context wins
  over client actor headers and stale upstream principal context.
- `internal/service/collaboration_test.go`: real DB atomic rollback, concurrent
  replay, no fabricated membership/ordinary AI, gate and authority, recovery after
  source departure/handler disable, and denial after creator disable.
- `internal/handler/collaboration_boundary_test.go`: legacy direct/batch/comment/
  rerun refusal, retained-space deletion, source reorder, machine detail denial;
  pair with ordinary batch and workspace-deletion controls.
- `internal/migrations/collaboration_schema_test.go`: actual additive migrations,
  valid/no-FK indexes, session/run uniqueness, unknown-proof barrier, populated
  rollback refusal, busy writer refusal, and empty maintenance rollback.
- `internal/handler/collaboration_test.go`: external participant reads without
  source workspace context, private account fields omitted, exact decimal
  versions and machine/restricted/unrelated denial. Core `schema.test.ts` covers
  malformed responses, private-log redaction and the legacy numeric wire.
- `internal/handler/collaboration_boundary_test.go`: private body/comment sentinel
  search cannot match or affect machine counts; title references remain visible;
  normal/open-only lists and private filters never expose collaboration fields.
- `internal/service/collaboration_queue_test.go`: real DB concurrent sequence
  allocation, same-key replay, current-role waiting cancellation, session lock
  contention, stale Stop, never-permitted settlement, and uncertain commit recovery.
  These tests seed runs; they do not prove actual promotion or native termination.
- `cmd/server/collaboration_routes_test.go`: signed JWT/PAT account reads with
  unrelated workspace headers, real task-token denial, participant revocation
  and absence of a public creation route through the production router. The
  legacy-agent matrix covers JWT/PAT in both authentication modes, safe title
  references and private-content/search/workflow denial.
- `internal/service/collaboration_handoff_test.go`: nested information chain
  unwinding layer by layer with one iteration fact per action, unavailable
  requester without re-invite, archived custom restore target, acceptance to the
  creator, roles/closed issue, and rollback of every required write. Route
  wiring is `cmd/server/collaboration_handoff_routes_test.go`; core detail
  parsing is `packages/core/collaboration/handoff-schema.test.ts`. Include exact
  directed status/digest mapping, same-key metadata drift and durable refusal
  replay after catalog restoration or responsibility changes.
- `internal/service/collaboration_edit_test.go` and handler counterpart: real
  transfer/disable versus edit ordering, current roles, null/omission/date wire,
  required-write rollback, exact versions and untouched explicit AI input.
  `cmd/server/collaboration_edit_routes_test.go` verifies the shared PATCH route;
  core edit schema/API tests cover malformed/private/stale-session responses.
- `internal/service/collaboration_account_reconciliation_test.go`: actual
  disable/restore writer, old/fresh epochs, multiple consumers, bounded cursor
  restart, held/missing-source fairness, unknown commit and required-write
  rollback, startup/cancellation, and guarded migrations 617–619 rollback. The
  separate review/fairness regressions exercise a failed prefix as large as the
  batch, process restart, and scheduling-write failure. Seeded run evidence
  verifies server barriers only, never native containment or true exit.

Use a task-owned PostgreSQL database; never run these destructive migration
rehearsals against an application schema. Provider tests must use fake executables.
The migrate CLI's down command walks the complete reverse migration chain; it is
not a one-step rollback. Rehearsals select explicit SQL files in private fixture
schemas. Export and verify DATABASE_URL before Go tests: several older TestMain
functions otherwise fall back to an application database, while service suites
may skip without failing the process.

## 7. Wrong vs Correct

Wrong: `IssueService.Create(...)` then add participants/session in another
transaction, or retry failed creation using a new request key.

Correct: resolve policy before locks, call `CollaborationService.Create` (or its
borrowed-transaction primitive), and recover uncertain outcomes with the original
actor/source/request identity.

Wrong: source membership implies shared AI control, or a blank actor header
implies a human.

Correct: verified credential context plus current participation determines human
collaboration rights; source membership separately controls source structure.

Wrong: redact the response body but leave description/comment search predicates,
private ordering or HTTP error logs untouched.

Correct: apply machine reference projection before matching/counting and protect
both schema and transport logs; test with private-only sentinel text.
