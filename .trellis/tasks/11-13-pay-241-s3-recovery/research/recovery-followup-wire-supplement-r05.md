# S3 block recovery: creation and human-history wire supplement

Date: 2026-11-15. Coordinator-selected technical supplement for independent planning review. The r05 check records the coordinator's later selection of actual-handoff keysets, explicit legacy provenance, 5-second/64-KiB read bounds and a history-only signed cursor. S3 remains `planning`; no product implementation, endpoint execution or acceptance is claimed.

This resolves the two public transport choices left in `recovery-followup-block-contract-v2-20261114.md` and identified by S2 `research/s3-continuation-prerequisites-20261115.md`. The v2 repair authority, one-transaction effects, pending-submission priority, original-key recovery, metadata validation and strict runtime-v1 boundary remain its source contracts. The v1 findings and all original evidence stay available.

## 1. Selected-project creation options

Reuse `GET /api/collaboration/creation-options`. Its current registration is in `server/cmd/server/router.go`; the current Core method is `getCollaborationCreationOptions` in `packages/core/api/client.ts`. Add only the optional query parameter `project_id` to this read. It requires a nonempty `source_workspace_id` and a valid UUID; resolve the project through the current actor's existing source-project authorization and verify its source workspace before projecting it.

New successful responses include one closed `stage_policy_observation` union:

```json
{
  "protocol_version": 1,
  "state": "not_selected",
  "source_workspace_id": null,
  "project_id": null,
  "policy_revision": null,
  "managed": null
}
```

```json
{
  "protocol_version": 1,
  "state": "known",
  "source_workspace_id": "<authorized source workspace UUID>",
  "project_id": "<selected project UUID>",
  "policy_revision": "<nonnegative decimal string>",
  "managed": true
}
```

- With a source workspace but no selected project, `state` remains `not_selected`, `source_workspace_id` names that authorized source, and the other three fields remain explicit null. Keep the original status choices. Null management is unknown applicability, not `false`.
- With a selected project, `state` is `known`, all identities and the decimal policy revision are present, and `managed` is the actual enabled policy value. A supported project with no policy row uses revision `"0"` and `managed:false`.
- Only `known` plus `managed:true` filters statuses whose resolved category is `blocked`. Custom blocked keys follow the same category rule. Default status selection must remain a returned selectable status.
- An unreadable project is the existing content-free 404. Failure to establish the required current policy/schema is 503; it never produces a successful `known:false` or empty-policy response. Unknown/duplicate query keys and invalid parameter combinations are 400.
- An older server omitting the whole group remains unknown in Core. A partial, contradictory or malformed new group cannot be used to infer an unmanaged project.

All six union keys are required when the group is present. IDs are canonical nonzero UUIDs; `policy_revision` is a canonical nonnegative signed-64-bit decimal string, including exactly `"0"`; `managed` is an actual boolean in `known` and explicit null in `not_selected`. Core correlates the group with the requested source/project and includes selected project in the existing creation-options Query key. A previous project's placeholder result cannot filter the new project's choices. The existing `parent_search` contract remains unchanged.

**The observation is a read-only UI fact. It is not added to the creation POST body or its digest.** New and old creation requests retain their existing canonical shape; an unexpected `stage_policy_observation` POST field is still rejected. The owning creation transaction independently resolves the actual effective project, locks its current policy using the accepted S2 order, and rejects a new managed blocked root before any creation effects. A stale options response therefore grants no exemption. Already committed original creation keys recover before new-admission checks and keep their original bytes.

Use the existing 409 `status_unavailable` for that managed blocked selection. It is already recognized by the creation savepoint/refusal owner and Core creation-result enum; preserve the exact five-field own result with `outcome:"rejected",code:"status_unavailable"`. Roll back every attempted creation effect before the owner commits this refusal. Replaying that key after policy/status recovery returns its stored result, not a delayed creation. Schema/dependency failure remains 503 unavailable and is never reclassified as this known business refusal.

This choice avoids a second creation authorization token and preserves old request identities while giving the shared form an explicit project-scoped filtering signal.

The current creation-options transaction is repeatable-read/read-only (`collaboration_creation_options.go:27`); do not call a policy helper that takes write-permitted row locks from it. The new options read validates the selected project in its already authorized source snapshot and reads supported policy state there. The actual effective project is resolved later in `IssueService.CreateInTx`, including parent inheritance when no project was pinned (`issue.go:442` onward). The root-only blocked guard must consume that resolved identity before counter/issue effects; checking only raw creation `project_id` would miss inheritance. Use a narrow internal collaboration-creation distinction, not a caller-supplied marker or a restriction on all ordinary children. Recheck the exact final S2 helper/lock contract before implementation.

## 2. Human information history

Add reads to the existing human handoff collection; keep the existing POST writer and runtime handoff endpoints separate:

| Method and path | Projection |
| --- | --- |
| `GET /api/collaboration/issues/{id}/handoffs` | Information-request, information-return and information-repair history only, newest-first by actual handoff `(created_at,id)` |
| `GET /api/collaboration/issues/{id}/handoffs/{handoff_id}` | One exact information handoff and its original event; no search by latest timestamp |
| `GET /api/collaboration/issues/{id}/handoffs/{handoff_id}/superseded-requests` | The exact immutable request relation superseded by that repair; the ID must identify an information repair |

Collection and superseded reads accept only `limit` and `cursor`: limit defaults to 50 and is a canonical decimal integer from 1 to 50; the emitted history-only cursor is at most 2048 encoded bytes. Detail accepts no query. Duplicate, unknown or malformed query input is 400 after object authority has been established. IDs use the existing authorized root loader and strict handoff UUID validation. No `max_bytes`, offset, arbitrary ordering or runtime context parameter is added.

Every read uses current collaboration issue authority, `Cache-Control: no-store` and the current-scope fence. The new adapter explicitly supplies one 5-second context for the entire read; `readCollaborationContent` supplies the existing authorized RR snapshot, not an implicit timeout. A lost read basis denies the complete response, including old IDs, empty pages and cached cursors. No source artifact lookup, M1 authorization, status mutation, event, notification or queue write occurs in these readers.

The collection envelope is closed:

```text
protocol_version: 1
issue_id: UUID
collaboration_revision: positive decimal string
lifecycle_epoch: positive decimal string
upper_handoff: {created_at: UTC microsecond string, handoff_id: UUID} | null
items: (InformationRequest | InformationReturn | InformationRepair)[]
next_cursor: bounded string | null
```

On the first page capture the greatest actual handoff `(created_at,id)` tuple in the selected projection; subsequent pages use `tuple <= upper` and `tuple < last` in the same newest-first ordering. Empty history has `upper_handoff:null`, `items:[]` and `next_cursor:null`. Do not order legacy rows by a fabricated event sequence. A later workflow revision/lifecycle or authority change invalidates the page chain before mutable resolution fields can be mixed. Fresh authorized reads may restart a chain; a cursor never restores an old authority generation.

The affected-list envelope has the same version/root/revision/lifecycle/upper/next fields, plus required `repair_handoff_id:UUID` and `superseded_request_count:nonnegative signed-64-bit decimal string`. Its `items` are the affected-request shape below. Capture its upper tuple from the exact affected relation, not all issue handoffs. The denominator is the repair's immutable full count, including legacy/unavailable items, never a page length. Zero has count `"0"`, null upper/next and an empty array. Detail is `{protocol_version:1, issue_id:UUID, item}`.

All three responses, including the complete envelope, JSON escaping and encoder newline, are at most **65,536 encoded bytes**. This is a fixed endpoint cap; no size query is added. Size the actual DTO with the existing `collaborationRecallJSONBytes` pattern. Fill a returned prefix only after reserving the real signed next cursor; the next page must still include the first row that did not fit. If even one full item and its minimal complete envelope exceed the cap, return 503 `information_context_unavailable`, not an omitted-note placeholder, silent row drop, empty success or false `next_cursor:null`. The generic comment-page budgeter reverses rows, has its own envelope and can omit content; it cannot be reused unchanged for these closed history shapes.

### History-only cursor integrity

There is no reusable general handoff sealer: current `handoffCursor` is run-specific base64 JSON, while T4's MAC key comes from a frozen run subject. Introduce only a narrow human-history v1 HMAC-SHA256/base64url cursor using the existing server signing key. Obtain the actual cached `auth.JWTSecret()` bytes and require `auth.ValidateJWTSecret(string(key))` to succeed; never validate a fresh environment value while signing with a different cached key. The known public non-production fallback is not usable here: unavailable/invalid key returns 503 `information_context_unavailable`. Reuse existing `JWT_SECRET` configuration; no new secret setting, persistent cursor token, capability or dependency is introduced.

Derive a separate key with HMAC-SHA256 over the literal domain `collaboration-human-information-history-cursor-v1`. The base64url object has one strict version-1 payload and MAC. Its canonical MAC input binds viewer/root, projection (`information_history` or `information_repair_affected`), current collaboration revision/lifecycle, current authority fingerprint, upper and last handoff tuples, and exact repair ID or explicit null. Reject unknown/duplicate/case-alias/trailing fields, noncanonical UUID/decimal/time values, oversized encoding and invalid MAC using constant-time comparison. Altered/invalid cursors are 400; valid cursors with changed current binding are 409. Check live issue authority before decoding or looking up cursor-selected rows.

The authority fingerprint comes from the same authorized snapshot: current issue authority epoch, nullable participant/account epochs and the real source-member row identity/creation time when that basis exists, together with verified credential generation. Source-only readers legitimately have null participant/account epochs; do not invent a positive default. Do not serialize credentials, token hashes, password material or signing-key bytes. Removal/rejoin of either read basis cannot revive the old fingerprint. `auth.JWTSecret` is cached with `sync.Once`; key rotation is an actual process replacement/restart under the new configuration, not environment hot reload. A rotated key cannot make an old cursor usable; a fresh authorized first page is required.

## 3. Exact item shapes and provenance

Each item has `kind`, exact `handoff_id`, actual UTC microsecond handoff `created_at`, and one closed `provenance` object:

```text
{state:"linked", event_id:UUID, event_sequence:positive decimal string}
{state:"legacy_unlinked", event_id:null, event_sequence:null}
{state:"unavailable", event_id:null, event_sequence:null,
 reason_code:"information_context_unavailable"}
```

`linked` requires one exact original event in the same root, with matching handoff ID, human actor and purpose/kind. Missing old links are `legacy_unlinked`, not guessed latest events, zero UUIDs/sequences or a reason to omit the row. Ambiguous/mismatched/partial provenance is `unavailable`. These states grant no historical source-basis authority. Common full-row fields for linked or legacy-unlinked items are `actor_user_id:UUID`, original `note:string`, `from_user_id:UUID|null`, `to_user_id:UUID|null`, actual `previous_status:string`, and stored `resolution:"open"|"returned"|"superseded"`. Nullable legacy identities remain explicit null. Current account display summaries do not substitute for original actor IDs. An unavailable item is a closed minimal stub containing only kind/handoff/created-at/provenance; do not invent missing actor, status or prose fields.

- **InformationRequest:** kind `information_request`; `requester_user_id:UUID|null`, `parent_request_id:UUID|null`, required strict v2 `block` complete/legacy/unavailable union, and `superseded_by_repair_handoff_id:UUID|null`. Here the information request identity is `handoff_id`; it is not the workflow operation's original Idempotency-Key. Additive old root-detail responses may omit block for unknown support, but this new history endpoint must classify it explicitly. The original reason/recipient fields remain in their containing handoff.
- **InformationReturn:** kind `information_return`; `consumed_request_id:UUID|null` is the exact information handoff consumed by the return; linked new returns require it non-null. `restored_status:string|null` and `return_status_fallback:boolean|null` come from the original return transaction/event, never a later catalog query or current handler. New returns record these facts with their exact original request/event and actual return handoff/event. An old linked return may derive the fallback only when both original saved key and exact committed target are verifiable; otherwise the value is null/unavailable, not false. Unlinked legacy returns retain the actual row/parent identity and explicit null restoration facts.
- **InformationRepair:** kind `information_repair`; new repair rows are linked to their actual `information_repaired` event and have `prior_active_request_id:UUID|null`, `restored_status:string`, `target_user_id:UUID` and the closed v2 `basis` reference. Target must equal the actual non-null `to_user_id`; prior status is the actual blocked key. `superseded_request_count` is a canonical nonnegative signed-64-bit decimal string. Repairs never use legacy provenance to invent a basis; corrupt partial new facts use the unavailable stub. Ordinary `purpose='transfer'` is not enough to classify a row as repair: require the exact repair event. The affected list uses the separate bounded read above.

An affected-list item contains `request_id:UUID` (the original information handoff ID, not the operation key), `request_event_id:UUID|null`, the same closed provenance union, `resolution:"superseded"` and `superseded_by_repair_handoff_id:UUID`. A linked event UUID equals provenance.event_id; legacy/unavailable uses explicit null. All items are checked against the immutable relation and the same root. A missing/unreadable repair or wrong-purpose ID does not become an authorized empty list. Retain legacy rows in every denominator/page reconciliation. A `historical_information` repair basis still requires a real exact origin event and verifiable active-chain relation; missing links cannot become that permission. With no pending submission, `human_selected` may supersede old unlinked rows and truthfully retain their null provenance.

Detail uses the same item union and fixed bounds. Record completeness/provenance is separate from current actionability; human history may show a complete old record that cannot currently be returned. Root current projection uses the actual pointer: null pointer means none, while a non-null pointer with a missing/invalid row is unavailable. Do not collapse that observation through the old helper's open=false result.

### Original-key and pending-return integration

`repair_information` joins the existing directed-workflow replay class: current active human participation gates receipt lookup; only a **new** repair checks both Transfer and ChangeStatus inside its action, along with managed/blocked/open context. A successfully committed original key is not recomputed after handler/policy/context changes. Syntactic absent/null validation happens before pointer conversion, but managed new-field requirements and ETA final-clock checks must not run ahead of original-operation recovery. Own content-free GET follows its existing account rules; this adds no new read or write privilege.

The existing status-action allowlist and Core status-options schema must recognize this new action explicitly. The chosen `default_status:""` means deliberate selection only for `repair_information` with its valid `information_repair_context`; current schemas require a nonempty included default for all old actions. Use the new closed variant, not a global relaxation of their defaults or categories.

Pending-submission priority also applies to a new managed normal return. The current `collaborationReturnStatus` fallback can select in_progress when a saved in_review key is archived; it cannot be copied as an exit from a still-pending result. A legitimate blocked-to-blocked nested return remains allowed when it reactivates its exact complete parent with a verifiable pending-safe exit. Eventual nonblocked restoration preserves the exact submission and uses a verifiable in_review return to its current eligible creator, or the explicit pending-submission repair/current in_review selection. Never restore ordinary work or a different acceptance handler through a fallback. A creator's source-workspace departure with retained current participation is not an unavailable creator; actual account/participation/recipient eligibility decides. Security removal remains independently legal, does not invent repair links, and never revives old stage approvals, queues or runs.

## 4. Activity and runtime boundary

Keep the stage activity allowlist and existing opaque WS/inbox hints. An `information_repaired` activity may identify its exact handoff/event, actor/time, previous/restored status, source-basis kind and safe basis IDs. It carries no note, condition, ETA, source prose, artifact digest or affected-request bodies in generic metadata. Human readers obtain structured details from the protected routes above.

The human runtime handoff read currently shares the strict `CollaborationHandoffTransfer` v1 decoder. These new human-history endpoints therefore own the block/repair DTOs; none of their fields is inserted into the runtime v1 envelope, old persisted snapshots or rehydrated original source bytes. The v2 supplement's explicit missing-marker rule for newly created runtime snapshots remains unchanged.

## 5. Required review and execution evidence

Independent planning review must check these choices against the current creation/source readers, cursor helpers and final accepted S2 lock order, including the v1 C02 finding. It must resolve any concrete route or field collision before S3 activation. Then copy the accepted contracts into S3 design/implementation manifests; research alone does not activate S3.

Required owning-layer regressions cover no selected project, known off/on, custom blocked category, missing policy dependencies, cross-source project denial, older/malformed group, policy changes between options and the real creation transaction, and unchanged old creation-key recovery. History regressions cover exact request/return/repair provenance, legacy/partial metadata, zero/51+ affected requests, watermark/byte-budget continuity, foreign/changed cursors, current-authority loss and zero writes. Shared Core parsing and Web/Desktop current/history rendering consume these same envelopes.

Planning evidence: current router exposes POST handoffs and creation/status-options reads; current Core creation-options method has only source workspace and parent search; v2 fixes repair/pending delivery semantics but leaves these transport choices open. This document selects the read contracts. Product tests, database/API/browser execution and S3 acceptance remain NOT_RUN.
