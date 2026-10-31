# Core B1 API and consumer seam

This is a client implementation contract, not evidence that HTTP routes, managed injection, or product views exist. The authoritative approved write contract is design §4; full read DTO fields below make its human projection requirements concrete for the later HTTP owner.

## Wire contract

All version/revision values are exact signed-int64 decimal strings. Every supported response has `protocol_version: 1`. A valid older JSON response without this field, or a bodyless absent route, returns `null` (unsupported). Malformed claimed support, wrong response identity, cyclic/oversized pages, unexpected successful HTTP status, and invalid JSON throw a content-free `CollaborationKnowledgeProtocolError`. Structured 4xx denials/conflicts retain `ApiError`; they are not classified as an unsupported route. Reads use the existing 10-second request deadline and cancellation/authentication fences.

Every private GET includes `projection_checked_at`, `valid_until`, and required nullable `next_expiry_at`. The API methods are:

| Method | Route and response |
| --- | --- |
| `getCollaborationKnowledgeCandidates(issueId, page, options)` | GET source `/api/collaboration/issues/{id}/knowledge-candidates`; `{...lease, issue_id, items, next_cursor, capabilities:{can_propose}}` |
| `getCollaborationKnowledgeCandidate(candidateId, options)` | GET `/api/collaboration/knowledge-candidates/{id}`; `{...lease, candidate, history, history_truncated}` |
| `getCollaborationKnowledgeTarget(target, page, options)` | GET `/api/projects/{id}/knowledge` or `/api/workspaces/{id}/knowledge`; `{...lease, target, knowledge_revision, items, next_cursor}` with explicit captured workspace header |
| `proposeCollaborationKnowledge(issueId, request, options)` | POST source collection; exact design proposal, preselected `candidate_id` and `request_id` |
| `approveCollaborationKnowledge(candidateId, request, options)` | POST candidate `/approve`; exact state revision, content version and SHA-256 |
| `rejectCollaborationKnowledge` / `withdrawCollaborationKnowledge` | POST corresponding candidate action; exact state revision and reason |
| `getCollaborationKnowledgeOperation(candidateId, requestId, options)` | GET candidate `/operations/{requestId}` with no source ID or workspace header |

`page` is `{cursor?,limit?}`, default 20 and maximum 50. A nonempty next cursor requires a nonempty page, must differ from the requested cursor, and all IDs within a page are unique. Candidate detail `history` contains at most 50 summary rows for the same lineage/source/target. Every detail requires the explicit boolean `history_truncated`; missing, null or non-boolean values in claimed-v1 responses are malformed, never defaulted to false. The source collection pages **all candidate versions**, not lineage heads, so older versions remain reachable without another endpoint.

Candidate rows discriminate on `availability`:

- `available`: `candidate_id,lineage_id,version,state,state_revision,target,source_workspace_id,source_issue_id,proposed_by_user_id,proposed_at,expires_at,supersedes_candidate_id,derived_from_candidate_id,approved_by_user_id,approved_at,published_target_revision,decision,capabilities,summary,content_sha256,sources,evidence,applicability`. `decision` is nullable `{actor_user_id,at,reason}`. Only the main detail candidate has `content`; lists/history reject it.
- `source_unavailable`: audit metadata above, plus `capabilities`; no summary/body/hash/sources/evidence/applicability. A decision may expose only actor/time, never its potentially quoted reason. Approval cannot be granted. A pending candidate may carry `can_reject:true` for a human with current source Read and target confirmer authority, including when its material is unavailable or it has expired; this grants only content-free state reduction.
- `restricted`: exactly `candidate_id,state,state_revision,availability,capabilities:{can_withdraw}`. No source/target identity, version, hash or prose. A restricted detail has empty history. The target envelope already identifies the authorized target.

Missing capability fields become `false`; truthy non-booleans fail validation. Approval requires an available pending candidate and the server's current `can_approve:true` (which excludes expiry). Rejection requires a non-restricted pending candidate and `can_reject:true`; material availability and expiry do not prevent this narrowing action. Restricted target-only rows cannot gain rejection or body access. Withdrawal still requires pending/approved state and its server grant. The server remains the authorization authority.

Sources/evidence/targets match `server/pkg/protocol/collaboration_knowledge.go`. Every evidence anchor must occur in required sources. UTF-8/NUL, decoded material, source scope, union fields, correction/derivation IDs and UTC timestamp shape are validated. Future expiry is intentionally rechecked by a new server write after original-request lookup; an elapsed timestamp never prevents retry of the original immutable payload.

Receipt fields are exactly design §4, with actions `propose|approve|reject|withdraw`. Confirmed success codes are `knowledge_proposed|knowledge_approved|knowledge_rejected|knowledge_withdrawn`, correlated to action/state; approval must name the reviewed version, next state revision, and nonzero target revision. Unknown outcomes remain uncertain. Create accepts HTTP 200/201; other successful responses require HTTP 200. HTTP 202 never confirms recovery.

Malformed, permission or validation failures before a new candidate is persisted remain ordinary unsealed `ApiError`s. The client does not fabricate candidate state/version or a receipt from those errors; their original-operation lookup can be not-found. Successful creation atomically records candidate plus operation, and authorized refusals for existing candidates may be sealed. Network/commit ambiguity and 5xx responses retain the identical candidate/request/payload and recovery identity; they are not treated as definitive outcomes.

Target knowledge routes have a separate private logging classification: request/response log paths omit cursor/query data, and error logs use a content-free message. This classification does not remove the explicit target workspace headers or change ordinary route logging.

## Query and action use

The collaboration barrel exports the new schemas/types, API-independent intents and hooks. `useCollaborationScope()` supplies the live server/viewer identity. `useCollaborationKnowledgeCandidates`, `useCollaborationKnowledgeCandidate`, and `useCollaborationKnowledgeTarget` return Query data as `{value,boundary}`. `value` is the typed wire projection; `boundary` is metadata only. For a subsequent page pass `{cursor:value.next_cursor, boundary, limit}` from that exact first page. Do not clone, persist or reconstruct the boundary. A new first-page read invalidates old pages; consumers render only pages belonging to the current first boundary.

`prepareKnowledgeProposal` preselects both UUIDs; `prepareKnowledgeDecision` captures the exact observed state/content tuple. Both return a frozen `KnowledgeIntent` containing immutable serialized request bytes. `useCollaborationKnowledgeAction` awaits `executeKnowledgeIntent`; it performs no optimistic publication and never generates a replacement request during retry. Preserve this same intent for an uncertain in-memory retry.

Before transport, the core writes only the client-generated recovery identity `{server,userId,candidateId,requestId,action}` through `StorageAdapter`, one key per request with read-back confirmation. No body, summary, source ID/ref, expected hash, server state or receipt is persisted. `readPendingKnowledgeOperations` reads only the current actor/server's records. After reload use `recoverKnowledgeOperation` for GET-only settlement, without reconstructing or resending private proposal bytes. A missing/malformed result leaves the record pending. A confirmed original outcome retires it. A fresh login by the same actor can recover it; an obsolete live request scope or another actor cannot. Storage without key enumeration or confirmed writes fails before POST.

## Cache and realtime contract

`KnowledgeProjectionPolicy` measures time from monotonic request start, clamps to 30 seconds/server interval/nearest expiry, charges network delay, and refuses expired/obsolete/out-of-order results. The same first-page authorization and replayed server check cannot extend an earlier deadline. Visible hooks refresh at most every 15 seconds. No server projection enters storage or Zustand.

Before refresh, failed current reads, expiry, authority hints, disconnect or visibility/online transitions, clear private Query data and its cancellation rollback snapshot. Inactive Query entries are removed; mounted observers remain connected with data absent. Timers also enforce the deadline for standalone QueryClient reads. Generation increments precede cancellation and refetch, so late responses cannot restore previous content. Source/target relations may be hidden or not fetched yet, so the conservative invalidation unit is the entire current viewer/server knowledge scope. Content-free recovery queries are separate.

`bindKnowledgeRealtime` is reference-counted per QueryClient/viewer/connection. It consumes the exact strict three-field `collaboration:knowledge_changed` payload, shared source/access schemas, and the unchanged empty-only workbench payload. Revisions may arrive out of order and always cause a current GET; they are never used to synthesize candidate state. Target-only hooks do not subscribe to source rooms. `WSClient.onDisconnected` and the existing React WSProvider now expose immediate close/teardown to this binding; stale socket closes cannot affect the replacement socket. Focus/online responders are also reference-counted. No new provider, router or visible UI was added.

## Remaining integration work

The later HTTP owner must implement these projections and maintain the first-page authorization boundary across cursor pages; expose honest bounded history semantics; agree the success codes; and run cross-language HTTP fixtures. Views must render unavailable/restricted states, preserve the frozen in-memory intent for uncertain retries, recover saved identities on remount, and consume current page boundaries. Backend authority, target outbox, G2 runtime injection/session gates, Web/Desktop product flows and real-provider behavior remain outside B1.
