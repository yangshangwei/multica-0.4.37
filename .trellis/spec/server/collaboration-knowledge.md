# Collaboration knowledge candidates and invalidation

## 1. Scope / trigger

Apply to human and automated knowledge proposals, exact-version decisions,
project policies, accepted-event jobs, private projections, source availability
and target invalidation. Execution has a separate
[preparation and launch contract](./collaboration-knowledge-execution.md).
These are implemented contracts, not a release or Provider acceptance claim.

## 2. Signatures

Human candidate endpoints negotiate `X-Multica-Knowledge-Version: 2`; an absent
header means v1. Candidate write bodies declare the same `protocol_version`.
Both readers are strict, with canonical UUIDs and exact signed-int64 decimal
strings for versions/revisions. Routes are in
[`router.go`](../../../server/cmd/server/router.go); strict DTOs and budgets are in
[`collaboration_knowledge.go`](../../../server/pkg/protocol/collaboration_knowledge.go).
Policy/jobs endpoints require header version 2. Their policy/retry bodies contain
only the fields below, without `protocol_version`; they use the strict workflow
body decoder rather than the candidate DTO decoder.

| Endpoint | Contract |
| --- | --- |
| `GET/POST /api/collaboration/issues/{id}/knowledge-candidates` | All-version source pages / proposal with client-selected `candidate_id` and `request_id` |
| `GET /api/collaboration/knowledge-candidates/{candidateId}` | Current candidate detail, same-lineage history and required `history_truncated` |
| `POST .../knowledge-candidates/{candidateId}/approve` | `request_id`, `expected_state_revision`, `expected_version`, `expected_content_sha256` |
| `POST .../knowledge-candidates/{candidateId}/reject` or `/withdraw` | `request_id`, `expected_state_revision`, nonempty `reason` |
| `GET .../knowledge-candidates/{candidateId}/operations/{requestId}` | Original actor's content-free receipt; no source ID required |
| `GET /api/projects/{id}/knowledge` or `/api/workspaces/{id}/knowledge` | Authorized target projection with explicit workspace context |
| `GET/PUT /api/projects/{id}/knowledge-policy` | V2 leased policy; PUT `{request_id,expected_revision,is_enabled,max_candidates}` requires an explicit boolean and max 1–3 |
| `GET /api/projects/{id}/knowledge-policy/operations/{requestId}` | Original human actor's content-free policy receipt; remains routable after workspace departure |
| `GET /api/collaboration/issues/{id}/knowledge-jobs` | V2 leased status, exact accepted submission/event, counts and model/usage metadata; latest 20 jobs plus `has_more` |
| `POST .../knowledge-jobs/{jobId}/retry` | `{request_id,expected_revision}`; current source reader and target confirmer, same accepted job, new execution revision |
| `GET .../knowledge-jobs/{jobId}/operations/{requestId}` | Original actor's content-free retry receipt, without source material |

Migrations 747–763 add retained `collaboration_knowledge_candidate`, independent
`collaboration_knowledge_target`, durable `collaboration_knowledge_invalidation`
and internal ordinary-task delivery metadata. Each of the 16 indexes has its own
concurrent migration; relationships have no database foreign keys. Transaction
primitives live in [`collaboration_knowledge.sql`](../../../server/pkg/db/queries/collaboration_knowledge.sql).

Migrations 761–763 index only negotiated task history (`knowledge_context_version
IS NOT NULL`): agent/runtime keyset btrees and a `jsonb_path_ops` GIN over
retained runtime, daemon and owner claim snapshots. They add no exposure marker,
backfill or history row cap.

Migrations 844–860 and 865 add `project_knowledge_policy`,
`collaboration_knowledge_{job,batch,attempt,evaluation,validity}`, execution-policy
bindings, candidate origin and strict v1/v2 retained storage. Relationships remain
application-owned; every new index is a separate concurrent migration. Keep sqlc
and the public-schema workspace-deletion manifest synchronized. Policy and target
rows are explicitly removed with their owning workspace/project; candidate,
job, batch, attempt, evaluation, validity and invalidation history are retained.

Policy/retry DTOs and transactions live in
`collaboration_knowledge_policy.go` and `collaboration_knowledge_automation_jobs.go`;
there is no public command that can act as the system approver.

## 3. Contracts

### Formal acceptance, rules and automatic validity

- An absent project override is `origin:system`, `revision:"0"`, enabled, max 3,
  rule `project-knowledge-v1`, with null human actor/time. A current human project
  lead or workspace owner/admin may write a CAS override. Built-in rule upgrades
  retain explicit settings. Knowledge use, extraction model configuration and
  project adoption policy are separate controls.
- Only exact formal project collaboration acceptance, including self-acceptance,
  writes a unique `(issue,accepted_event_seq)` job in the business transaction.
  No-project, manual-policy and disabled-policy cases write no job. Done, run
  success, returned/cancelled delivery and historical backfill are not triggers.
  Extraction runs after commit through the router-owned server LLM client; it
  never enqueues or replays the closed business input or uses a personal daemon.
- Delivery refs bind `issue_id`, `submission_id`, `delivery_event_seq`,
  `accepted_event_seq` and SHA-256 of the canonical complete result/basis.
  Quotes additionally name the exact result/basis field. Sources are inert:
  do not fetch attachments/URLs or execute commands from evidence.
- Each generation batch proposes 0–3 entries, each with body ≤2 KiB, summary
  ≤256 bytes and ≤4 exact quotes. Complete source input is ≤8 items/128 KiB;
  complete target comparison is ≤16 items/128 KiB. Generation sees permitted
  source material and same-source predecessors; checking also sees the full
  readable comparison. Authorize and budget-check the complete comparison before
  either model call. An unreadable/incomplete comparison cannot prove novelty.
  Each stage has ≤288 KiB decoded input and ≤2 MiB complete wire JSON; generation
  output is ≤6000 completion tokens/24 KiB, checking ≤3000 tokens/16 KiB, with
  private check reasons ≤1000 UTF-8 bytes. A valid empty generation completes
  without a checking call.
  Model reasons require the complete actual input-source closure, regardless of
  the model's declared related refs; public status contains fixed reason codes.
- A short settlement transaction rechecks accepted event/close epoch, source
  identity/hash/access, project, manual policy, effective policy/revision, target
  revision, exact quotes, expiry, human decisions and the complete target budget.
  Exact/semantic duplicates skip; conflicts, unsupported evidence, uncertainty,
  changed bindings and capacity stay pending. Save the fitting set and all
  outcomes atomically; never truncate or evict old contributions.
- Successful stage history and published outcomes persist. Expired leases and uncertain calls are
  retained as uncertainty; an authorized explicit retry advances execution,
  retaining saved create outcomes, settled predecessor slots and the acceptance-wide quota
  of newly published lineages. Count each automation-root lineage for that job
  once any version has a `published_target_revision`, including later human
  approval and withdrawn/superseded history. Unpublished pending proposals do not
  spend this publication quota; restoring/revising an existing lineage does not
  spend new-lineage quota. The current execution's policy limit bounds remaining
  automatic publication. Later human decisions cannot reset the count or permit
  a withdrawn automatic lineage to be resurrected.
- Job status preserves all permitted latest-execution stage-attempt records per
  job, up to 42: seven batches × two stages × three allowed attempt numbers.
  A 43-record projection fails closed. The current worker writes attempt 1 only,
  normally producing at most 14 records; this does not reduce the projection
  limit or permit dropping valid retained attempts.
- Automatic proofs apply to immutable `approval_origin:automation`, not merely
  an automated proposal or the latest decision origin. A human-approved automated
  proposal retains human approval's consumption semantics.
- Reopening advances the source epoch under existing source locks. New resolve,
  replay and launch authorization immediately reject stale automatic proofs,
  before asynchronous target hints. Approved history remains `paused_review`;
  unexpired paused approvals still reserve capacity. Reacceptance fixes all
  affected predecessors in up to six review batches plus one new-entry batch.
  Unchanged normalized body and summary receive a new validity revision when
  supported; changed material may replace its exact lineage only after passing
  the same gates. Human withdrawal/revision wins over late automatic results. Untouched automatic
  pending successors alone are not a human veto.
- V2 represents `proposal_origin`, `decision_origin`, nullable system actor IDs,
  automation provenance and `usage_status`. V1 serves only complete representable
  legacy scopes; new identity/delivery/validity shapes produce
  `knowledge_protocol_unsupported`, never a filtered fake-empty scope or a
  fabricated human approver. Original v1 receipts retain their exact version.

### Retained versions and current human authority

- A candidate's source, target, body, summary, hash, references, applicability,
  expiry and original proposal facts are immutable. `lineage_id` is the first
  candidate ID; versions increase within that lineage. State changes use
  `state_revision` CAS. Deleting a candidate is refused.
- Correction creates a new candidate with `supersedes_candidate_id`, the same
  source/target/lineage, and the exact predecessor. The old approved version
  stays active while correction is pending. Approval atomically supersedes the
  previous approved version, approves the reviewed version, advances the target
  revision, records the source event, original operation and target hint. At most
  one version per lineage is approved. Project-to-workspace derivation creates a
  distinct proposal via `derived_from_candidate_id`; it leaves the project
  contribution intact.
- `KnowledgeCapabilitiesFor` requires a current human principal. Proposal needs
  source Read plus creator or current-handler identity. An external current
  handler needs no source membership: the source workspace and already-visible
  current source project are lawful destinations. Arbitrary destinations still
  require normal visibility. This does not grant directory access.
- Approval needs current source Read, available unexpired material, target
  membership, and a human project lead or workspace owner/admin. Expiry is
  decided on the database clock (`clock_timestamp()`), the clock the approval
  UPDATE uses, before any target-revision or correction write. The same DB
  timestamp selects active contributions for the complete target budget,
  including approval of a candidate without its own expiry; application clock
  skew must not omit still-active rows or count already-expired ones. An expired
  candidate seals `409 knowledge_expired`. If expiry passes after that check,
  the attempt rolls back as a retryable conflict only after the database clock
  confirms expiry. A subsequent retry with remaining budget seals the same409;
  exhaustion remains503 knowledge_busy, with no partial write. A non-expiry
  missing UPDATE row retains404 rather than becoming an expiry retry. Rejection needs
  source Read and that target role, but may reduce an unavailable/expired pending
  candidate. Source withdrawal needs source Read plus creator/current handler;
  target confirmers can instead withdraw through a restricted control row.
  Closed issue state and an extra participant row are not additional requirements.
- Operations reuse the existing `(scope, actor, request)` namespace across all
  collaboration kinds. Changed kind/payload conflicts; retries recover the
  original outcome before checking new-write expiry. Candidate-only recovery
  verifies current human/credential and original actor/action/candidate, even
  after source/target-role loss. It grants no source read authority.

### Required source material and projections

`KnowledgeSourceAvailability` borrows an authorized transaction and grants no
authority itself. Each required source is `memory` (version), `comment`
(comment ID/revision), `run_record` (run/record ID), or v2 `delivery` (exact
submission and delivery/acceptance events), tied to one source issue. Delivery
hashes use the server's canonical complete result/basis JSON; the other kinds
hash exact decoded text. Old memory/comment versions remain valid if
exactly restorable; current comment tombstones and source purge/delete suppress
their historical copies. Missing/conflicting material is unavailable; database
failure is an error. Retained accepted delivery text can remain available after
reopen while its automatic consumption proof is paused. Test/commit evidence is
inert text anchored to a required source; `verification` is the proposer's
attributed claim.

Every body-bearing GET is `no-store` and carries `projection_checked_at`,
`valid_until` (at most 30 seconds later), and required nullable `next_expiry_at`.
Credential expiry can shorten the lease. Pages (default 20, maximum 50) retain
the first authorization interval and recheck current authority; detail history
is bounded to 50 with an explicit truncation boolean. Source pages retain every
candidate version.

| Availability | Allowed projection |
| --- | --- |
| `available` | Authorized audit metadata, summary/hash/refs/evidence/applicability; body only in the main detail candidate |
| `source_unavailable` | Lawful source/audit metadata and allowed controls; no body, summary, hash, refs, evidence, applicability or quoted decision reason |
| `restricted` | Exactly candidate ID, state, state revision, availability and `capabilities.can_withdraw`; no source identity, version, prose or history |

Withdrawal/expiry do not purge lawful historical text; source unavailability does
suppress it in **every** list, detail, history and target projection. Approval
never expands source ACLs. Contributions are never appended to `project.description`,
`workspace.context`, stable files or skills.

### Locks, durable hints and credential freshness

Human/preparation locking preserves user/credential → ordered parent
workspace/member → target project/revision → source → candidate order. Parent
existence uses `KEY SHARE NOWAIT`; project-lead authorization alone requires the
project `SHARE NOWAIT` lock. Owner/admin approval and source-authorized withdrawal
do not acquire that lead hotspot. Target initialization saves/restores
`lock_timeout`, bounds the first unique insert to 25ms, and rolls back the whole
transaction on contention. Counters are independent of parent edits; missing
counters with published history are corruption, never empty knowledge.

The PAT writer lock (`LockKnowledgePAT`) is `FOR SHARE` without NOWAIT.
Revocation is a plain `UPDATE ... SET revoked` on a non-key column and takes
`FOR NO KEY UPDATE`. `FOR SHARE` conflicts with that lock; `FOR KEY SHARE` would
not, so it cannot be used. The asynchronous `last_used_at` refresh takes the
same lock as revocation, so no lock mode can wait out one and still block the
other. The writer instead waits under a saved and restored 250ms
`lock_timeout`, which is below the default 1s `deadlock_timeout`. If a
revocation commits during the wait, the recheck fails with `403
knowledge_permission_denied`. A wait past the bound raises a retryable 55P03 and
ends in `503 knowledge_busy`.

Ordinary `UpdateProject` metadata edits use `LockProjectForUpdate` (`FOR NO KEY
UPDATE`), not the execution-squad `FOR UPDATE` lock. This preserves revision
serialization and conflicts with lead/association `SHARE` readers while allowing
knowledge's parent-existence `KEY SHARE`. Test the actual handler held before
commit against actual preparation; testing only generated parent-lock queries
would miss a stronger lock acquired by the writer. Squad/resource/delete paths
keep their existing exclusive locks.

Direct collection changes commit revision plus hint together. Source purge,
delete, ACL/participant changes append only a scan in the authoritative writer's
transaction; member changes scan the source workspace and account/credential
changes use conservative `all_sources` scans. No target lookup is added to the
source writer. The worker freezes a candidate-ID upper bound, scans ever-published
history, and commits at most one routed candidate/target per short transaction.
Pending-only targets do not emit hints. Cause/target deduplication, revision, hint
and cursor advance atomically; a busy target yields without losing the cursor.
Deleted targets are not recreated; retained routing/outbox survives source deletion.

The workspace deletion manifest classifies `collaboration_knowledge_candidate`
and `collaboration_knowledge_invalidation` as **keep**, including completed hints
and pending scans. Source-workspace deletion still refuses retained collaboration
records. `collaboration_knowledge_target` is **delete**: the existing parent-delete
transaction calls `DeleteKnowledgeTargetsInTx` to retain final hints before
removing its counters. Deleting a target never cascades into candidate versions
or invalidation history. `TestWorkspaceDeletionManifestPreservesKnowledgeHistory`
checks the actual handler against immutable candidate, pending scan, delivered
hint and final-hint retention; schema coverage must classify all three tables.

`CollaborationKnowledgeInvalidation.Run` drains scans/hints every 500ms, at most
100 each per pass. Relay failure retains the original hint ID; successful sends
use `<hint-uuid>:target`. Each hint has a 5s delivery budget. A failed hint
records `last_attempted_at`, which moves it behind the other pending hints. When
only the hint's own budget expired, the record uses a fresh bounded context.
The failed hint is skipped for the rest of the pass, so it cannot block later
hints. Three consecutive failures end the pass, and the errors are joined. A
retry republishes the same event ID, which clients dedupe. The entire public
event is:

```json
{"type":"collaboration:knowledge_changed","payload":{"target_kind":"project","target_id":"11111111-1111-4111-8111-111111111111","knowledge_revision":"2"},"event_id":"22222222-2222-4222-8222-222222222222:target"}
```

Final send rebuilds canonical bytes after rejecting unknown/duplicate/aliased
outer or payload fields. It rechecks current target visibility and the socket's
**original credential**, including a direct current PAT lookup independent of
PATCache, or current password-token version. Missing freshness capability drops
knowledge. Reconnect creates a new credential binding. The existing
`collaboration:workbench_changed` payload remains empty-only. These checks finish
before the write pump sets the write deadline, so their own 5s bounds never
spend the 10s write budget. Every physical write sets its deadline immediately
before `WriteMessage`. One hint triggers these uncached checks on every
connection in the workspace room, so the database load grows with room size.
This is accepted as is.

Reserved sqlc overrides are `*.knowledge_delivery` and
`*.knowledge_context_version` with `json:"-"`, including CTE-derived task rows.
Table-only overrides miss generated projections. Internal metadata, raw
credentials, body/summary/refs and cursor/error text stay out of generic logs.

## 4. Validation & error matrix

| Condition | Outcome |
| --- | --- |
| Candidate DTO unknown/duplicate/cross-kind fields, malformed UUID/UTF-8/text | `400 invalid_knowledge_request` |
| Multiple knowledge-version header values | `400 invalid_knowledge_request` |
| One unsupported knowledge-version header value, including an explicit empty value; or v1/absent header on policy/jobs endpoints | `409 knowledge_protocol_unsupported` |
| Candidate write body version differs from its supported header version | `400 invalid_knowledge_request` |
| Policy/retry body is malformed, has duplicate/unknown fields (including `protocol_version`), or has trailing JSON | `400 invalid_request`, unsealed |
| Policy/retry workflow body exceeds 64 KiB / media type is not JSON | `413 request_too_large` / `415 json_required`, unsealed |
| Policy body omits `is_enabled` or `max_candidates`, or decoded `request_id`/`expected_revision`/`max_candidates` fails canonical/range validation | `400 invalid_knowledge_request`, unsealed |
| New proposal `expires_at` not after `proposed_at`, by the application clock or by the database clock at INSERT (CHECK `collaboration_knowledge_candidate_check5`, matched by table and constraint name) | `400 invalid_knowledge_request`, unsealed; any other `23514` stays `503 knowledge_unavailable` |
| Body >64KiB, summary >2KiB, metadata >16KiB or request wire limit | `413 knowledge_candidate_too_large` |
| Missing reviewed revision/version/hash | `428 knowledge_revision_required` |
| Current role/read/credential denied | `403 knowledge_permission_denied`; no hidden-source projection |
| Stale state revision, reviewed body, lifecycle, expiry or source | `409 knowledge_revision_conflict`, `knowledge_content_changed`, `knowledge_state_conflict`, `knowledge_expired` or `knowledge_source_unavailable` |
| Target >16 active items or >128KiB full material | `409 knowledge_target_budget_exceeded`; no truncation/publication |
| Reused request with different kind/payload | `409 idempotency_conflict` |
| V1 candidate scope requires automation, delivery or validity semantics | `409 knowledge_protocol_unsupported`; no lossy projection |
| Policy CAS mismatch | Sealed `409 knowledge_policy_revision_conflict` receipt with v2, original request/project IDs and current revision |
| Retry CAS mismatch / changed source / current policy disabled | Sealed `409 knowledge_job_revision_conflict`, `knowledge_source_changed` or `knowledge_automation_disabled` receipt with original request/issue/job IDs and revision |
| Contention after bounded transaction retry / dependency failure | `503 knowledge_busy` / `knowledge_unavailable` |

New-proposal failures before persistence may be unsealed; they do not fabricate
a receipt. Existing-candidate decisions may seal a content-free refusal. A
missing operation is not evidence that an uncertain write never committed.
Policy/retry PUT/POST handlers serialize a sealed refusal receipt with HTTP 409,
including same-key write replay. Their operation GET returns HTTP 200 with the
same sealed code, version, request/resource IDs and revision; these receipts have
no body `status` field. A generic error writer would drop identity/version/revision
and turn a known refusal into client uncertainty. Unsealed errors keep generic handling
and must not fabricate an empty receipt. GET recovery reauthorizes the original
actor independently of their current source/target role.

## 5. Good / base / bad cases

- Good: an external handler proposes to the visible source project; its human
  lead reviews the exact tuple; a later purge clears all material projections.
- Good: acceptance commits despite model failure; reacceptance restores only
  currently supported automatic proofs and preserves human withdrawal.
- Base: a pending proposal has no runtime effect and sends no target hint.
- Bad: target admin role exposes source text, an old approved row is edited in
  place, a purge waits on target fanout, or missing history becomes an empty set.

## 6. Tests required

Automation worker/rule/policy suites assert formal-event-only jobs, zero model
calls without configuration, all-source reason privacy, complete comparison
before any model call, lease uncertainty, acceptance-wide lineage quota across
retries/human decisions, full multi-batch revalidation, and both commit orders
for reopen and human overrides. Handler `collaboration_knowledge_version_test.go`
asserts sealed non-2xx policy/retry receipts and original-key recovery. Workspace
deletion must pass `TestWorkspaceDeletionManifestCoversPublicSchema`, not only a
selected retention subset. Current integration evidence belongs to
`11-14-project-knowledge-automation/research`; older passes are bounded snapshots.

Keep policy/protocol, migrations/generated serialization, transactional service,
HTTP and socket tests at their owning layers. Canonical suites are
`internal/collaboration/knowledge_test.go`, protocol/migration knowledge suites,
service `collaboration_knowledge_{authority,concurrency,view,credentials,resolve,invalidation}_test.go`,
`pkg/db/generated/knowledge_serialization_test.go`, and
`cmd/server/collaboration_knowledge_realtime_test.go`.

Assert both lock commit orders, exact correction CAS, original-key recovery after
role loss, base strings unchanged, all projection sentinels removed after purge,
pending-only silence, crash/replay cursor stability, deleted routing, wildcard
tags, a timed-out hint that does not block later hints
(`TestKnowledgeTimedOutHintDoesNotBlockLaterHints`), and final socket denial even
when Redis contains stale PAT authority.
Execution/static evidence is recorded separately in the
[task research](../../tasks/archive/2026-11/10-30-collaboration-knowledge-promotion/research/).

## 7. Wrong vs correct

Wrong: replace a sealed policy refusal with `{error:code}`, or turn off automation
and deploy a reader that cannot recognize its retained history.

Correct: return the original status and content-free receipt, retain strict v1/v2
readers, and recheck the current automatic proof at every consumption boundary.

Wrong: broadcast a candidate/source event and let target clients remove its body;
authorize a cached PAT because the account is still active.

Correct: publish only the three target fields, authorize the original credential
at final send, and make each current GET independently suppress unavailable
material. Use the [Core lease](../core/frontend/collaboration-knowledge.md) as a
bounded display fallback, never runtime authorization.
