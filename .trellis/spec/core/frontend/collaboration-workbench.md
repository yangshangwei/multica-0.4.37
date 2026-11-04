# Account collaboration reads, controls and recovery

## 1. Scope / Trigger

Apply to `packages/core/collaboration`, shared collaboration views and the
`/account` route. The surface includes account records, resource/participant
forms, gated creation and controlled attachment delivery. Source-page routing,
fields, iterations and discussion editing follow the shared
[source/editing contract](../../server/collaboration-source-editing.md).
General AI execution and full product acceptance retain their separate gates.
Formal results, exact initiator decisions and manual policy follow
[the delivery contract](../../server/collaboration-delivery.md); shared layout and
disclosure behavior follow [the detail contract](../../views/frontend/collaboration-detail.md).

## 2. Signatures

`CollaborationScope`, `collaborationKeys`, `protectCollaborationRead` and the
draft/operation helpers bind reads and writes to Server, viewer and access epoch.
`AccountWorkbenchPage` is shared by Web and Desktop; platform adapters own routes.
`GET /api/collaboration/issues/{id}/activity` supplies cursor-based activity.
Cancellation and Switch recover by GET with the original request key.
Creation and upload also have original-key GETs before an issue UUID is known.
The source picker, private draft/byte API and native capability signatures are
defined in [collaboration content](../../server/collaboration-content.md).
Account/issue subscriptions follow [collaboration realtime](../../server/collaboration-realtime.md).

`delivery-schema.ts` is a pure schema leaf for submission/decision/policy and
content-free `delivery_confirmation`. `collaborationDeliveryHistoryOptions` uses
a protected infinite Query; `deriveCollaborationDetailState` owns stage and
primary/secondary actions, and `collaborationHasAIActivity` owns facts-based
console presence. `useCollaborationDeliveryForm(scope, "result")` is separate
from `"comment"`; result intents carry the exact `deliveryDraftId`.

## 3. Contracts

React Query owns issue/workbench/comment/activity/notification data. Persisted
drafts contain client intent and original observations, never a second server
state store. Current and participating lists remain separate; transferring work
does not remove the participant's detail or notification-independent entry.
The synchronous StorageAdapter currently persists form/operation JSON as
plaintext localStorage. Preserve Server/viewer/source/issue/purpose scoping and
access-loss erasure; do not describe these records as client-encrypted or add a
new key lifecycle. Server uploaded bytes retain their separate encryption.

Issue detail reads use the shared recovery request deadline (10 seconds) while
retaining the caller's cancellation signal. This bounds polling, focus and
reconnect reads that have no realtime event to restart them; a timeout remains
the normal query error so the existing retry affordance can recover the panel.
After a successful human action, invalidate the account scope once and await
only the issue query's current (or realtime-restarted) read before resolving the
action. Participant removal follows this same operation path. Do not issue a
second issue invalidation for that wait, and fence the result against the
current collaboration scope and access epoch before returning it.

Pre-issue creation drafts bind Server, viewer, source workspace, draft UUID and
the exact original request/payload. A source workspace UUID is never an issue
UUID. Save intent before transport; preserve source-specific input and uploaded
file identities on ambiguous results. Explicit source or account changes fence
late results. Cancelled upload transport does not prove server rollback: retain
the original key and file identity/fingerprint until recovery can settle it.

Private file metadata must match source, uploader, draft and explicit
`intended_issue_id` (null for creation; exact issue for issue-intended uploads).
Each successful creation/comment/delivery retires its consumed upload bundle
and starts a new draft UUID. Retirement belongs in the confirmed core result
path so an unmounted composer cannot revive it. A newly mounted form observes
that state change. Reusing delivered files sends their IDs without the old draft.
Attachment-bearing comment success/recovery must confirm the same file IDs;
an older/malformed success without that correlation remains uncertain.

Resource or operation failure is not automatically parent-access loss. A missing
file/operation or unsupported file route is unavailable. Creation's
`account_unavailable` may identify its recipient rather than the caller; recheck
explicit source authority before deleting its draft. Only a confirmed authority
loss triggers protected cleanup. A failed authority refresh does not establish
that loss.

`executeCollaborationAction` settles only schema-valid final refusals recognized
by `collaborationDeliveryRefusalStatus(code)`. The operation must have a positive
decimal `collaboration_revision`, no result confirmation, and an absent or
matching body `status`; HTTP errors must match that code's status. This includes
the existing statusless `stale_revision`/409 and declared committed role/file
403/404 receipts. Only workflow POST transport preserves those declared receipt
envelopes. Plain read/access denials retain normal cleanup; malformed or
contradictory declared envelopes remain unavailable, not confirmed access loss.
Interpret refusals inside the awaited `protectCollaborationRead` callback so
scope/access-epoch checks precede operation retirement and authorized invalidation.
Keep mutable result text/basis/files, and require an explicit new submit/key with
current observations. Unrecognized codes, mismatched statuses and unknown commits
keep the original operation unresolved; do not normalize every HTTP error or retry.

Result submission and completion success/recovery require a validated exact
`delivery_confirmation`, even for text-only work. Pending acceptance captures
the current submission's attachment IDs internally as `submissionAttachmentIds`
and sends its ID without rebinding result fields. Compare that original ID/file
set before retiring a bundle. Normalize valid uppercase UUID observations; retire
only the matching result form/draft, including after unmount, without clearing a
newer bundle. A refused operation retires while its unsent form remains editable.

Creation retirement requires the own-result GET's HTTP 200 and strict
actor/source/request correlation with `outcome: rejected` and a recognized code.
A 202 response, mismatched tuple or missing/malformed result remains unknown.
Confirmed refusal removes pending state but preserves the form and unconsumed
files. A later explicit submit gets a new key, even for identical input; do not
require meaningless text changes. GET, reload and receipt arrival never submit.
The old key remains rejected after the gate or recipient becomes available.

Native file capabilities are short-lived read credentials. Validate the current
API origin and exact issue/attachment path before opening them; never persist
the URL in a draft or comment. Keep previews sandboxed, revoke transient blob
URLs and abort scoped requests on close/access loss. Ordinary upload/download
helpers are not a compatibility fallback for protected files.

API clients import pure schemas, not operation modules that call the API. For
example, `client.ts` imports `command-schema.ts`; `command.ts` may call `getApi`.
Importing the latter from the client creates an initialization cycle and can
leave `ApiError` undefined under legitimate partial mocks. Shared schema leaves
may use `api/schema`, but must not import the API barrel/client or request helpers.

Activity has stable IDs and exact string sequence numbers, minimal account
summaries and explicitly projected details. New handoff events retain their
actual `handoff_id` in the same transaction. Older unlinked events stay factual
generic entries; never infer notes or identity from timestamp proximity. Unknown
payload fields and resource/credential data do not leave the projection.

Missing optional history routes on older servers are unavailable, not proof of
lost issue access. A structured object denial does clear protected data. Both
success and error paths reject late responses from an obsolete account/access
epoch. Recoveries started with current access still use the issue guard so a
fresh denial clears its cache. Only already-known access loss uses the
account-only guard and accepts content-free recovery.

Delivery history uses the optional-history transport: a missing route triggers
parent revalidation and preserves authorized detail, while a structured object
denial erases protected state. Pages enforce matching issue/author/decision IDs,
at most 100 rows, descending exact sequences, immutable overlap consistency and
cursor progress. Missing history and legacy/malformed evidence remain unavailable;
do not infer a result or decision from status or nearby comments.

Each issue denial retains the exact `ApiError` instance propagated into its
protected Query entries. A successful, current-epoch authoritative issue read
clears only that instance from the current Server/viewer cache. Cleared reads
remain data-free and pending; disabled comment/parent reads remain idle. Do not
use Query `reset()` for this recovery: it can restore revoked `initialData`.
Keep genuine child errors, other issue denials and obsolete-response fences.
An unavailable or failed authority read does not clear the denial.

Assignment notifications use `notifications.assigned`, with recipient-facing
text in both languages. `creation.assignedSelf` belongs to the self-selection
state of the creation form and must not label another person's assignment.

Command-history `reason` is a required nullable nonempty string. Successful
commands with absent/empty stored reasons must be projected as `null` by the
service; keep strict Core parsing and every nonempty cancellation/failure reason.
The full cross-client batch test must reopen actual Web and Electron history
through that parser after natural outcome and proof. See the
[command-history wire contract](../../server/collaboration-command-history.md).

Cancelled/Switch redacted receipts contain exactly `redacted`,
`protocol_version`, `issue_id`, `request_id`, `action`, `status`, `code`.
Zero-valued extra execution fields are not the same wire contract. Missing or
malformed receipts keep the original request pending; GET recovery never creates
a replacement write. Refetch current authorized state after a real receipt.

Server `control_ai=true` survives only with current write context and supported
execution data. Stop and waiting cancellation do not require a live grant,
online runtime or open execution gate. Resume submits its observed gate, target,
hold and control versions for server validation. Submission/creation rollout
remains independent. Exact-recipient lookups and edit forms reject stale responses
or baselines without silently replacing user intent.

Manual policy blocks new command/Resume/resource/Switch preparation, retaining
Stop, withdrawal, waiting cancellation and original-request recovery. Supported
current capabilities still control every action. Absent old-server policy is
unknown, not enforced manual. Assisted restoration never creates Resume intent.
AI console omission requires supported empty execution facts and no historical
input, selected run, pending request or error. Discover retained command history
only when the current execution snapshot does not already establish activity;
history bodies remain lazy and missing/error history keeps a factual entry visible.

Recipient search uses `collaborationRecipientSearchOptions` below the current
Server/viewer issue key or creation-delivery key, then `recipient-search`, the
trimmed query and `membersOnly`. Infinite pages forward `AbortSignal`, use the
matching issue/delivery access guard, remain fresh for 30 seconds and disable
automatic retries. Cancelled, obsolete-session or lost-authority reads cannot
publish candidates into the current scope.

`searchCollaborationRecipients` validates the response context, page size and
cursor progress. A malformed page is unavailable; malformed individual accounts
are dropped. Unknown scopes use the narrower related-only explanation. A
non-source-member response always masks `external` as null, and members-only
results retain only `external === false`. An older server's missing search
route is unavailable discovery, not proof of issue access loss; exact lookup
remains available. `collaborationRecipientSections` owns case-insensitive-ID
deduplication in self, selected, recent, all order. Keep the unused
`recipientQuery` field in strict delivery-draft schemas so saved drafts still
parse; new forms write an empty string.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown execution or absent write context | No inferred control authority |
| Supported participant with closed rollout gate | Stop/R45 remain available; Resume still validates |
| Unknown write response or malformed receipt | Retain original key and pending intent |
| Recognized final refusal with matching HTTP/body status, or allowed absent body status | Retire exact operation; retain form input and require explicit current-version submit |
| Unrecognized code, contradictory status or malformed declared POST receipt | Preserve uncertain intent and original-key recovery |
| Fresh recovery permission denial | Clear protected issue cache and obsolete drafts |
| Current-epoch authority read succeeds after denial | Clear that denial's propagated Query errors without restoring old data or fetching inactive reads |
| Authority recovery is unavailable or fails | Retain denial and data-free protected state |
| Access already denied before recovery | Accept only content-free own receipt |
| Optional history route absent without object denial | Show history unavailable; retain authorized detail |
| Late lookup/read after scope change | Abort/ignore; preserve the current scope |
| Missing attachment/operation or unsupported optional route | File unavailable; revalidate parent authority before cleanup |
| Recipient unavailable during creation | Preserve request/files while source authority remains valid |
| Upload cancelled after transport began | Abort transport; retain original recovery identity |
| Attachment-bearing comment omits/mismatches file confirmation | Keep original request uncertain |
| Comment settles after its component unmounts | Retire that exact bundle in core; do not clear a newer draft |
| Result success lacks/mismatches submission/file confirmation | Keep original operation and bundle unresolved |
| Missing delivery summary or legacy/unavailable result | No inferred acceptance; explicit authorized repair only |
| Manual task with retained history/pending intent/error | Preserve console discovery, Stop and original recovery; no new execution |

## 5. Good / Base / Bad Cases

- Good: a cancellation POST commits but its response is lost; reload performs
  the original GET and resolves one operation without a second POST.
- Base: ordinary MCP/autopilot flows keep a usable shared `ApiError` constructor.
- Bad: deleting an unresolved request key, restoring a stale private receipt,
  or exposing an arbitrary activity payload as UI data.

## 6. Tests Required

Core command/Switch/action/comment/control suites own recovery and epoch matrices.
Schema tests own malformed wire shapes and source-only denial. Existing MCP and
autopilot partial-import tests guard API initialization. Shared views verify
wiring, original intent, per-target outcomes and old comment anchors. Web E2E
uses production Next plus actual API for cancellation recovery, mention/comment,
handoff/activity, participating retention, another browser and revocation.
Do not count test-seeded existing records as public creation acceptance.
Creation/file suites own malformed metadata, exact native URL validation,
cancel/reload/permission fences and immutable upload keys. Web delivery acceptance
uses public creation with actual uploads and observes the account WebSocket
before checking immediate preview/detail cleanup. Native Electron exercises the
production download IPC and restart with an isolated profile. Resource form E2E
uses owned runtime/enrollment fixtures without launching an AI process.

`delivery-result.test.ts`, `delivery-history.test.ts`,
`result-operation.test.ts` and `presentation.test.ts` own exact delivery/policy
parsing, cross-issue/author/decision correlation, descending/cursor/history bounds,
optional route versus object denial, committed and malformed POST refusals,
text-only/file confirmation, uppercase draft retirement, old/new bundle separation
and unknown/manual action derivation. Views keeps happy-path wiring and named
accessibility/recovery regressions; it does not repeat the pure helper matrices.

Mixed-mention journeys keep one H+A+B+S comment and correlate each queued AI
command with its target, channel, batch and input event. Revoke A only after the
real preview, then verify B's queue entry, H's single located notification and
the identical original-key receipt. A subsequent unavailable-target preview is
read-only. Toggling an AI checkbox invalidates and unmounts the preview: browser
tests click the observed control, then assert the new preview rather than wait
for the retired checkbox's checked state.

Offline notification acceptance closes the recipient client before delivery
and performs a real login before opening its fresh context/profile. Reusing an
old token alone does not establish login-after-offline. Read/archive preserve
current handling; comment/body edits notify only newly effective human
recipients and retain the appropriate comment/body anchor.

Control journeys distinguish an actual server-issued Claim/Permit with withheld
proof from kernel containment. The former verifies the UI's unconfirmed Stop,
partial Resume and immutable original receipt; actual process/proof behavior
comes from the Linux supervisor tests. Exercise agent, squad and whole-issue
controls, compare every unselected lane/hold, and require a fresh observed
explicit Resume after renewed consent. Public runtime credentials remain in
fixture closures and never enter screenshots, traces or attached evidence.

`collaboration/operation.test.ts` owns the statusless stale-revision refusal and
unknown/denial/late-epoch matrix. Shared workflow tests must use the actual
statusless 409 shape; remount recovery instead needs a genuinely lost response.
`e2e/collaboration-workbench-web.spec.ts` verifies a real concurrent handoff,
immediate refusal, no recovery GET or automatic POST, and an explicit same-page
new-key completion. A browser reload must not hide this regression.

`collaboration/access.test.ts` owns denial recovery for disabled Query observers,
revoked initial data, unrelated errors and another issue's denial. Browser
acceptance keeps the denied recipient's page open through a transfer, then
verifies the old prompt disappears and a comment saves without an entire-page
reload. `views/collaboration/notifications.test.tsx` verifies the Chinese
assignment text independently from the creation form's self-selection text.

## 7. Wrong vs Correct

Wrong: import request-bearing cancellation helpers from `ApiClient`, or always
drop the issue scope for GET recovery.

Correct: import the pure schema leaf; retain the issue guard until access loss
is already known, then allow only the exact content-free receipt contract.

Wrong: settle every HTTP error, erase form input on a committed file refusal, or
normalize a receipt outside the access guard.
Correct: validate the recognized code/status/envelope within the protected request;
retire the exact operation, preserve unsent form input and require explicit retry.

Wrong: treat completed status as initiator acceptance or retire a result form from
a generic successful code without its exact submission/file confirmation.
Correct: retain unavailable decision facts and require correlated confirmation;
the R38 administrative exception never invents an initiator decision.

## Recorded-run and ordinary-child reads

See [the recorded-run contract](../../server/collaboration-records.md). Keep
record pages inside the issue access/epoch guard, distinguish `run_not_found`
from parent denial, and validate assembled page chains before cache writes.
Cursor loops or conflicting immutable records refuse the page; identical
overlap and a consistent full refresh remain valid. Missing telemetry is null,
not zero. Result links preserve the old notification source shape and add an
optional `run_id`; current denial removes that anchor.

`views/collaboration/execution-panel.test.tsx` routes mock responses by endpoint
so handoff requests cannot consume the ordered records-pagination queue. Keep
the actual handoff child/query boundary exercised; its dedicated suite owns the
full matrix. Adding an independent query must not turn a later-page denial into
an earlier, unrelated response.
