# Knowledge schemas, leases and original-intent recovery

## 1. Scope / trigger

Apply to `packages/core/collaboration/knowledge-*`, API knowledge methods and
realtime bindings. The [server contract](../../server/collaboration-knowledge.md)
owns authority; [shared UI](../../views/frontend/collaboration-knowledge.md) owns
review interaction. Private server data belongs only in TanStack Query.
This includes automated candidate provenance, project policies, accepted-event
jobs and their content-free original-request recovery.

An explicit `CollaborationUnavailableError` stops interval polling for that
projection. Manual refresh, focus and reconnect may check availability again;
transport errors keep the normal bounded refresh policy. Never clear an
unavailable result merely to schedule another polling request.

## 2. Signatures

- API methods: `getCollaborationKnowledgeCandidates`,
  `getCollaborationKnowledgeCandidate`, `getCollaborationKnowledgeTarget`,
  `proposeCollaborationKnowledge`, `approveCollaborationKnowledge`,
  `rejectCollaborationKnowledge`, `withdrawCollaborationKnowledge`, and
  `getCollaborationKnowledgeOperation` in
  [`api/client.ts`](../../../../packages/core/api/client.ts).
- `useCollaborationKnowledgeCandidates(scope, issueId, page?)`,
  `useCollaborationKnowledgeCandidate(scope, candidateId)` and
  `useCollaborationKnowledgeTarget(scope, target, page?)` return Query data
  `{value, boundary}`. Capture `scope` with `useCollaborationScope()`.
- `prepareKnowledgeProposal` and `prepareKnowledgeDecision` return frozen
  `KnowledgeIntent {scope,candidateId,requestId,action,requestJSON,issueId?}`.
  `executeKnowledgeIntent` awaits its exact receipt; `recoverKnowledgeOperation`
  is GET-only. `useCollaborationKnowledgeAction` has `retry:false` and `gcTime:0`.
- `buildKnowledgeSources(client, scope, issueId, selected)` returns exact typed
  refs with SHA-256, not source bodies. There are no environment keys in Core.
- `KNOWLEDGE_SUMMARY_MAX_BYTES` (2048), `KNOWLEDGE_CONTENT_MAX_BYTES` (65536),
  `KNOWLEDGE_REFERENCE_MAX_BYTES` (2048) and `KNOWLEDGE_REASON_MAX_BYTES` (1000)
  are the UTF-8 limits the schemas enforce, matching the server's `octet_length`
  checks. Views compare them with `collaborationTextByteLength`.
- `dismissKnowledgeOperation(scope, identity, storage?)` removes one stored
  recovery identity locally; it throws `CollaborationUnavailableError` for another
  or obsolete scope and `KnowledgeRecoveryStorageError` when removal fails.
- V2 API methods: `getProjectKnowledgePolicy`, `updateProjectKnowledgePolicy`,
  `getProjectKnowledgePolicyOperation`, `getCollaborationKnowledgeJobs`,
  `retryCollaborationKnowledgeJob` and `getCollaborationKnowledgeJobOperation`.
  Project reads/writes capture explicit workspace identity; operation recovery
  is account-scoped and does not depend on current workspace membership.
  Policy/jobs negotiate header version 2; policy/retry request schemas have no
  body `protocol_version`. Unknown body fields are not a version-upgrade path.
- `useProjectKnowledgePolicy(scope, workspaceId, projectId)` and
  `useKnowledgeAutomationJobs(scope, issueId, enabled?)` use the same leased
  `{value,boundary}` Query contract. Current queued/generating/checking/settling
  jobs refresh every three seconds while other visible projections use the
  normal interval. `selectKnowledgeAutomationJob` matches exact issue, project,
  accepted submission and accepted event, and excludes manual tasks.
- `prepareProjectKnowledgePolicy` and `prepareKnowledgeAutomationRetry` freeze
  `KnowledgeAutomationIntent` with v2 identity and serialized bytes.
  `executeKnowledgeAutomationIntent` accepts only matching sealed receipts;
  `recoverKnowledgeAutomationOperation` is GET-only after reload.

## 3. Contracts

### Strict private API boundary

[`knowledge-schema.ts`](../../../../packages/core/collaboration/knowledge-schema.ts)
uses strict discriminated schemas. Valid older JSON with absent protocol support
or a bodyless absent route returns `null`; malformed claimed-v1/v2 support, invalid
JSON, mismatched identity, unexpected success status or broken pages throws a
content-free `CollaborationKnowledgeProtocolError`. Structured denials retain
`ApiError`; they are not an empty/unsupported response. Versions/revisions stay
exact signed-int64 decimal strings; missing capabilities default false and
truthy non-booleans fail. Reads use the existing ten-second deadline and current
authentication/cancellation fences.

V2 schemas strictly distinguish human/automation proposal and decision origins,
nullable system actor IDs, automation rule/job/policy provenance, delivery refs
and active/paused/inactive usage. They do not invent human approvers or downgrade
unrepresentable v2 material. Policy defaults are system origin, revision `"0"`,
enabled/max 3, null actor/time; human overrides require a positive revision and
real actor/time. Core job-page schemas accept at most 20 unique exact-issue jobs,
and up to 42 complete latest-execution stage-attempt records per job; 43 records
are rejected. Preserve all valid retained attempts within this projection limit.
The current worker normally produces at most 14 records (seven batches × two
stages × attempt 1); this is not the schema limit. Counts and model/usage metadata
remain exact. Usage `known`, `partial` and `unknown` preserve actual zero and
missing/null values; unknown is not zero cost. Raw model responses, private
reasons and source bodies are rejected.

Available list/history rows reject `content`; only an available detail candidate
has a body. Unavailable and restricted variants reject prohibited material,
including quoted decision reasons. Detail requires `history_truncated:boolean`
and at most 50 same-lineage/source/target rows. Source pages preserve all versions;
IDs are unique per page and nonempty next cursors require progress and a nonempty
page. Target methods capture the explicit workspace header and clear the ambient
slug; their private log classification independently removes query/cursor/error
text. Recovery uses candidate/request only, with no source/workspace header.

### Display lease and erasure

`KnowledgeProjectionPolicy` starts from monotonic request time, so network delay
consumes visibility. Deadline is the minimum of 30 seconds, server check interval
and nearest expiry. Same-check replay, pagination, late/out-of-order response and
wall-clock drift cannot extend it. Visible queries refresh every 15 seconds.
Pass `{cursor:value.next_cursor,boundary,limit}` from the exact first page; never
clone, persist or reconstruct `boundary`. A new first-page read invalidates its
old page chain.

`expireKnowledgeProjections` increments the generation **before** cancellation,
erases data and TanStack's cancellation rollback snapshot, removes inactive
entries, then refetches mounted observers. Current-read failure, refresh, deadline,
source/access/target hint, disconnect and visibility/online transitions clear
private data. Standalone QueryClient reads have expiry timers too. Erase the
entire current server/viewer knowledge scope because hidden source/target
relationships cannot be inferred safely. Metadata-only recovery queries remain
separate. Never put projections or copied source bodies in StorageAdapter,
Zustand, optimistic state or durable drafts.

`bindKnowledgeRealtime` is reference-counted per QueryClient/viewer/connection.
Only strict three-field `collaboration:knowledge_changed`, established source/
access hints and empty-only workbench hints trigger its current reads. Revisions
may arrive out of order and never synthesize state. Target-only hooks require no
source subscription. A stale socket close cannot clear a replacement connection's
data. A lease is a bounded display rule, not runtime authorization.

When transport bindings overlap for the same QueryClient/viewer, only the newest
binding owns disconnect and disposal erasure. An older binding must unsubscribe
without clearing the replacement's projection. The current binding's disconnect
and final disposal still erase private data. Realtime tests must load a mounted
query through its normal read, overlap transports, and check both stale cleanup
and current-connection erasure.

### Exact sources and write recovery

For memory/comment/run_record, the source builder hashes exact decoded UTF-8
text without trimming. Memory uses
the chosen historical version; comments first check the current tombstone then
resolve exact current/history `after` or next-version `before`; run records follow
authorized pages to the exact ID. Missing/conflicting history, cursor cycles,
purge, timeout or access loss refuse. The ten-second deadline includes reading,
parsing and hash completion. Never replace the selected version with latest.
For a v2 delivery selection used by correction/derivation, re-read the available
candidate detail and retain only its matching issue/submission/delivery-event/
accepted-event source reference. The server owns the complete canonical
result/basis hash; Core does not reconstruct it with JavaScript JSON serialization
or copy delivery bodies into component state.

Before POST, preserve only client-generated
`{server,userId,candidateId,requestId,action}` through StorageAdapter with read-back
confirmation. No payload, source ID/ref, hash, server state or receipt is stored.
Each new human intent gets fresh UUIDs. Uncertain retries keep the same frozen
serialized bytes and IDs; after reload only the stored identity is available for
GET recovery. Current source/target-role loss does not erase original-actor
receipt recovery. Other actors and obsolete live authentication scopes cannot
settle it.

`KnowledgeUnusedIntentError` proves the current intent unused: storage/local
validation before any dispatch, or an exact recognized unsealed rejection with
no prior stored identity, prior dispatch or overlap. Human text may then be edited
and a new intent explicitly created; `cleanupFailed` separately reports local
removal failure. Prior uncertainty survives later 4xx, storage errors and missing
GET results. `idempotency_conflict`, malformed/unknown errors, 5xx and stale-auth
responses do not prove non-application. Only an explicit human
`dismissKnowledgeOperation` removes a stored identity without a receipt: it checks
the current scope, sends nothing, keeps in-memory attempt counts and projections,
and is not evidence of non-application. A late original write still lands
idempotently on the server and appears in the normal lists.

`KnowledgeReceiptCleanupError` carries an already validated content-free receipt.
Apply its known success/refusal, release the completed intent, and report local
cleanup separately; never offer another POST for that known result. Session-local
settled IDs suppress false unresolved notices; reload can GET-recover remaining
storage. Elapsed proposal expiry never rewrites a preserved request.

Automation recovery stores only `{scope:{server,userId},protocolVersion:2,
requestId,kind,projectId}` or the retry's `issueId,jobId`, with read-back
confirmation. No workspace header, payload, model material or receipt is stored.
An uncertain retry retains its frozen bytes and original IDs; reload cannot
reconstruct a write. A sealed HTTP 409 policy/retry refusal is a known receipt
only when its version, code and all request/resource IDs validate.
Write success requires HTTP 200 and refusal HTTP 409. Operation GET uses HTTP 200
for either sealed outcome; interpret its code rather than inventing a body
`status` field or treating HTTP 200 recovery as write success.
`KnowledgeAutomationReceiptCleanupError` carries that known outcome when local
removal fails. Missing/malformed receipts, unknown errors and prior uncertainty
remain unresolved; only explicit human local removal dismisses the identity.

## 4. Validation & error matrix

| Condition | Client result |
| --- | --- |
| Valid missing protocol/route | Scoped unsupported state (`null`) |
| Claimed support malformed, identity/page mismatch or HTTP 202 | Protocol/uncertain failure; no fabricated empty list or receipt |
| Source-unavailable pending with `can_reject:true` | Allow content-free rejection; unavailable approval remains blocked |
| Restricted row | Only current server-granted withdrawal; no rejection/body/source access |
| Lease expiry, refresh failure or authority transition | Erase body first; late responses cannot restore it |
| No prior attempt + proven unsealed refusal | `KnowledgeUnusedIntentError`; permit explicit editing/new intent |
| Prior uncertain attempt + later refusal/not-found | Preserve original intent and recovery identity until explicit human local removal |
| Human removal of a stored identity | Remove that exact key only; no request or projection expiry; other scopes refused; a later same-key retry stays uncertain |
| Known receipt + local removal failure | `KnowledgeReceiptCleanupError`; retain known result, GET-only recovery |
| Valid sealed policy/retry refusal over HTTP409 | Return the matching content-free receipt; show refusal, release completed intent and never display success |
| Retry offered for an old accepted event or paused/noncurrent source | Exact selection/capability validation blocks the new intent |
| Unknown/partial model usage | Preserve presence/null/zero; show unknown values rather than invented totals |

## 5. Good / base / bad cases

- Good: source purge erases mounted and inactive detail/pages before current GET;
  a failed refresh leaves loading/error UI without the old body.
- Base: old backend support is unavailable while the surrounding page still works.
- Bad: cancel with rollback restoring private data, persist a draft copied from a
  server body, or turn any 404 into a replacement request UUID.

## 6. Tests required

`knowledge-automation-{schema,status-schema,actions,queries}.test.ts` and API
knowledge tests assert system defaults/origins, exact accepted-event selection,
job/usage bounds including acceptance of 42 complete stage-attempt records and
rejection of 43, leased erasure, false-to-true policy recovery, sealed non-2xx
receipts, uncertainty, identity-only storage and role-loss GET recovery. A server
handler failure cannot be hidden by a mocked client-only pass.

Canonical pure matrices live beside `knowledge-schema`, `knowledge-lease`,
`knowledge-cache`, `knowledge-actions`, `knowledge-recovery`, and
`knowledge-sources`. `api/collaboration-knowledge.test.ts` verifies wire identity,
private logs and explicit target headers. Realtime knowledge/provider tests own
socket binding and teardown. Assert monotonic delay/skew, identical boundary
pagination, late-body denial, exact UTF-8 source hashes, exported byte-limit
boundaries, overlapping/previous dispatch, no-write versus known-receipt cleanup,
explicit local removal, and absence of private fields in storage. Shared UI tests
consume those contracts rather than repeat the matrices.

## 7. Wrong vs correct

Wrong: `invalidateQueries()` alone after a hint and keep stale text until GET
returns; catch a cleanup error and retry a successful POST.

Correct: use `expireKnowledgeProjections()` to erase rollback snapshots first,
and interpret `KnowledgeUnusedIntentError` and `KnowledgeReceiptCleanupError`
according to whether the server outcome is unknown, unused or already confirmed.
