# Knowledge schemas, leases and original-intent recovery

## 1. Scope / trigger

Apply to `packages/core/collaboration/knowledge-*`, API knowledge methods and
realtime bindings. The [server contract](../../server/collaboration-knowledge.md)
owns authority; [shared UI](../../views/frontend/collaboration-knowledge.md) owns
review interaction. Private server data belongs only in TanStack Query.

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

## 3. Contracts

### Strict private API boundary

[`knowledge-schema.ts`](../../../../packages/core/collaboration/knowledge-schema.ts)
uses strict discriminated schemas. Valid older JSON with absent protocol support
or a bodyless absent route returns `null`; malformed claimed-v1 support, invalid
JSON, mismatched identity, unexpected success status or broken pages throws a
content-free `CollaborationKnowledgeProtocolError`. Structured denials retain
`ApiError`; they are not an empty/unsupported response. Versions/revisions stay
exact signed-int64 decimal strings; missing capabilities default false and
truthy non-booleans fail. Reads use the existing ten-second deadline and current
authentication/cancellation fences.

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

### Exact sources and write recovery

The source builder hashes exact decoded UTF-8 text without trimming. Memory uses
the chosen historical version; comments first check the current tombstone then
resolve exact current/history `after` or next-version `before`; run records follow
authorized pages to the exact ID. Missing/conflicting history, cursor cycles,
purge, timeout or access loss refuse. The ten-second deadline includes reading,
parsing and hash completion. Never replace the selected version with latest.

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
responses do not prove non-application.

`KnowledgeReceiptCleanupError` carries an already validated content-free receipt.
Apply its known success/refusal, release the completed intent, and report local
cleanup separately; never offer another POST for that known result. Session-local
settled IDs suppress false unresolved notices; reload can GET-recover remaining
storage. Elapsed proposal expiry never rewrites a preserved request.

## 4. Validation & error matrix

| Condition | Client result |
| --- | --- |
| Valid missing protocol/route | Scoped unsupported state (`null`) |
| Claimed support malformed, identity/page mismatch or HTTP 202 | Protocol/uncertain failure; no fabricated empty list or receipt |
| Source-unavailable pending with `can_reject:true` | Allow content-free rejection; unavailable approval remains blocked |
| Restricted row | Only current server-granted withdrawal; no rejection/body/source access |
| Lease expiry, refresh failure or authority transition | Erase body first; late responses cannot restore it |
| No prior attempt + proven unsealed refusal | `KnowledgeUnusedIntentError`; permit explicit editing/new intent |
| Prior uncertain attempt + later refusal/not-found | Preserve original intent and recovery identity |
| Known receipt + local removal failure | `KnowledgeReceiptCleanupError`; retain known result, GET-only recovery |

## 5. Good / base / bad cases

- Good: source purge erases mounted and inactive detail/pages before current GET;
  a failed refresh leaves loading/error UI without the old body.
- Base: old backend support is unavailable while the surrounding page still works.
- Bad: cancel with rollback restoring private data, persist a draft copied from a
  server body, or turn any 404 into a replacement request UUID.

## 6. Tests required

Canonical pure matrices live beside `knowledge-schema`, `knowledge-lease`,
`knowledge-cache`, `knowledge-actions`, `knowledge-recovery`, and
`knowledge-sources`. `api/collaboration-knowledge.test.ts` verifies wire identity,
private logs and explicit target headers. Realtime knowledge/provider tests own
socket binding and teardown. Assert monotonic delay/skew, identical boundary
pagination, late-body denial, exact UTF-8 source hashes, overlapping/previous
dispatch, no-write versus known-receipt cleanup, and absence of private fields in
storage. Shared UI tests consume those contracts rather than repeat the matrices.

## 7. Wrong vs correct

Wrong: `invalidateQueries()` alone after a hint and keep stale text until GET
returns; catch a cleanup error and retry a successful POST.

Correct: use `expireKnowledgeProjections()` to erase rollback snapshots first,
and interpret `KnowledgeUnusedIntentError` and `KnowledgeReceiptCleanupError`
according to whether the server outcome is unknown, unused or already confirmed.
