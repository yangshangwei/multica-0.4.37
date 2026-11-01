# Collaboration knowledge candidates and invalidation

## 1. Scope / trigger

Apply to human knowledge proposals, exact-version decisions, private projections,
source availability and target invalidation. Execution has a separate
[preparation and launch contract](./collaboration-knowledge-execution.md).
These are implemented contracts, not a release or Provider acceptance claim.

## 2. Signatures

All human endpoints use `protocol_version: 1`, canonical UUIDs and exact decimal
strings for versions/revisions. Routes are in
[`router.go`](../../../server/cmd/server/router.go); strict DTOs and budgets are in
[`collaboration_knowledge.go`](../../../server/pkg/protocol/collaboration_knowledge.go).

| Endpoint | Contract |
| --- | --- |
| `GET/POST /api/collaboration/issues/{id}/knowledge-candidates` | All-version source pages / proposal with client-selected `candidate_id` and `request_id` |
| `GET /api/collaboration/knowledge-candidates/{candidateId}` | Current candidate detail, same-lineage history and required `history_truncated` |
| `POST .../knowledge-candidates/{candidateId}/approve` | `request_id`, `expected_state_revision`, `expected_version`, `expected_content_sha256` |
| `POST .../knowledge-candidates/{candidateId}/reject` or `/withdraw` | `request_id`, `expected_state_revision`, nonempty `reason` |
| `GET .../knowledge-candidates/{candidateId}/operations/{requestId}` | Original actor's content-free receipt; no source ID required |
| `GET /api/projects/{id}/knowledge` or `/api/workspaces/{id}/knowledge` | Authorized target projection with explicit workspace context |

Migrations 747–760 add retained `collaboration_knowledge_candidate`, independent
`collaboration_knowledge_target`, durable `collaboration_knowledge_invalidation`
and internal ordinary-task delivery metadata. Each of the 13 indexes has its own
concurrent migration; relationships have no database foreign keys. Transaction
primitives live in [`collaboration_knowledge.sql`](../../../server/pkg/db/queries/collaboration_knowledge.sql).

## 3. Contracts

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
  membership, and a human project lead or workspace owner/admin. Rejection needs
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
(comment ID/revision), or `run_record` (run/record ID), tied to one source issue
and exact SHA-256 of decoded text. Old memory/comment versions remain valid if
exactly restorable; current comment tombstones and source purge/delete suppress
their historical copies. Missing/conflicting material is unavailable; database
failure is an error. Test/commit evidence is inert text anchored to a required
source; `verification` is the proposer's attributed claim.

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
use `<hint-uuid>:target`. The entire public event is:

```json
{"type":"collaboration:knowledge_changed","payload":{"target_kind":"project","target_id":"11111111-1111-4111-8111-111111111111","knowledge_revision":"2"},"event_id":"22222222-2222-4222-8222-222222222222:target"}
```

Final send rebuilds canonical bytes after rejecting unknown/duplicate/aliased
outer or payload fields. It rechecks current target visibility and the socket's
**original credential**, including a direct current PAT lookup independent of
PATCache, or current password-token version. Missing freshness capability drops
knowledge. Reconnect creates a new credential binding. The existing
`collaboration:workbench_changed` payload remains empty-only.

Reserved sqlc overrides are `*.knowledge_delivery` and
`*.knowledge_context_version` with `json:"-"`, including CTE-derived task rows.
Table-only overrides miss generated projections. Internal metadata, raw
credentials, body/summary/refs and cursor/error text stay out of generic logs.

## 4. Validation & error matrix

| Condition | Outcome |
| --- | --- |
| Unknown/duplicate/cross-kind fields, malformed UUID/UTF-8/text | `400 invalid_knowledge_request` |
| Body >64KiB, summary >2KiB, metadata >16KiB or request wire limit | `413 knowledge_candidate_too_large` |
| Missing reviewed revision/version/hash | `428 knowledge_revision_required` |
| Current role/read/credential denied | `403 knowledge_permission_denied`; no hidden-source projection |
| Stale state revision, reviewed body, lifecycle, expiry or source | `409 knowledge_revision_conflict`, `knowledge_content_changed`, `knowledge_state_conflict`, `knowledge_expired` or `knowledge_source_unavailable` |
| Target >16 active items or >128KiB full material | `409 knowledge_target_budget_exceeded`; no truncation/publication |
| Reused request with different kind/payload | `409 idempotency_conflict` |
| Contention after bounded transaction retry / dependency failure | `503 knowledge_busy` / `knowledge_unavailable` |

New-proposal failures before persistence may be unsealed; they do not fabricate
a receipt. Existing-candidate decisions may seal a content-free refusal. A
missing operation is not evidence that an uncertain write never committed.

## 5. Good / base / bad cases

- Good: an external handler proposes to the visible source project; its human
  lead reviews the exact tuple; a later purge clears all material projections.
- Base: a pending proposal has no runtime effect and sends no target hint.
- Bad: target admin role exposes source text, an old approved row is edited in
  place, a purge waits on target fanout, or missing history becomes an empty set.

## 6. Tests required

Keep policy/protocol, migrations/generated serialization, transactional service,
HTTP and socket tests at their owning layers. Canonical suites are
`internal/collaboration/knowledge_test.go`, protocol/migration knowledge suites,
service `collaboration_knowledge_{authority,concurrency,view,credentials,resolve,invalidation}_test.go`,
`pkg/db/generated/knowledge_serialization_test.go`, and
`cmd/server/collaboration_knowledge_realtime_test.go`.

Assert both lock commit orders, exact correction CAS, original-key recovery after
role loss, base strings unchanged, all projection sentinels removed after purge,
pending-only silence, crash/replay cursor stability, deleted routing, wildcard
tags, and final socket denial even when Redis contains stale PAT authority.
Execution/static evidence is recorded separately in the
[task research](../../tasks/10-30-collaboration-knowledge-promotion/research/).

## 7. Wrong vs correct

Wrong: broadcast a candidate/source event and let target clients remove its body;
authorize a cached PAT because the account is still active.

Correct: publish only the three target fields, authorize the original credential
at final send, and make each current GET independently suppress unavailable
material. Use the [Core lease](../core/frontend/collaboration-knowledge.md) as a
bounded display fallback, never runtime authorization.
