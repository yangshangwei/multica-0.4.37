# Research: S3 block recovery contract v2

- Query: close S3P-C01, where a complete new information request created from an orphaned/legacy/replaced blocked state could still have no legal return path.
- Scope: internal contract correction; source inspection and direct discussion with `/root/s3_contract_check`. No product implementation or test execution.
- Date: 2026-11-14
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`
- Owner: `/root/s3_recovery_prepare`.
- Original research: `recovery-followup-preparation-20261114.md/json`, retained byte-for-byte. This supplement replaces its §8 recommendations where they conflict; the other six-topic research and all historical test limits remain.
- Status: one selected recommendation for coordinator review, following checker convergence. This is not a released wire contract or product acceptance.

## Findings

### 1. Why the original recommendation could deadlock

`handoff` always records `PreviousStatus: before.Status` (`server/internal/service/collaboration_workflow.go:249`). `RequestInformation` adds a parent only when the current top's recipient and current handler both equal the actor (`collaboration_handoff.go:180`). A creator replacing someone else's chain, a fresh request after removal cleared the pointer, and a legacy blocked issue can therefore create a new request with **actual `previous_status=blocked` and no usable parent**.

V1 required a complete open parent for a return to blocked while also refusing generic unblock and ordinary transfer. Its recommendation to create a new complete request did not repair the return target. This is **S3P-C01/P1, a planning defect**; no product failure was executed or claimed. Original research and the checker's original finding must remain available.

The selected correction is **repair once, then optionally request again**:

1. One explicit `repair_information` request runs **one existing workflow transaction**. It commits a real blocked→nonblocked status change, a selected eligible current handler, one handoff, exact supersession links, an audit event and its original-key receipt atomically.
2. The user may stop in that repaired nonblocked state. If more information is still required, they explicitly submit a separate new four-field information request, with a new key and fresh observations. That request truthfully saves its now-nonblocked previous status.

Repair is not a sequence of `Transfer` HTTP then `ChangeStatus` HTTP. Those are capabilities and existing internal primitives that the single transaction reuses. The optional later request is **not part of repair's atomicity**. Nothing automatically retries, creates B2, returns an information request, Resumes a queue or advances a business stage.

### 2. Normal requests and returns remain distinct from repair

For a new stage-managed request, allow:

- A currently nonblocked, nonterminal issue, following existing information-request authority and target rules. This includes a legitimate in_review interruption whose exact current pending submission is preserved.
- A currently blocked issue only for a **normal nested request**: actor is the current top recipient and handler; the active chain's records are complete; their exact parent/requester/recipient identities are consistent; the chain has a verifiable legal exit to a nonblocked state. Normal top/ancestor return prerequisites are rechecked. Any cycle, missing source/event, incomplete ancestor or unavailable return path is non-authorizing.

Do not admit a parentless replacement while already blocked. Creator replacement, orphan/legacy context, and an unavailable old requester use explicit repair first. There is no arbitrary new maximum depth. Verify ancestry with cycle detection under the existing bounded read/write transaction; inability to prove a chain before its existing budget expires is unavailable, not permission to assume a complete prefix. Repair with an explicit human-selected work status does not need to manufacture a valid ancestor chain when no pending delivery exists.

A normal nested return can still go blocked→blocked while reactivating its exact complete open parent, then the parent can return to its own saved nonblocked state. A source status archived since the original request uses only the existing explicitly displayed, version-bound fallback semantics; do not relabel a fallback as the original status. In_review restoration must also preserve the exact pending submission, as below.

An old committed operation is recovered before applying these rules to a **new** mutation. Do not retroactively reject or recompute its original receipt. Old/unmanaged requests keep their existing contract; a managed legacy read reports its limitations rather than inventing missing fields.

### 3. One explicit repair action, one transaction

#### Human authority

The actual authenticated human must be an active current participant with **both existing `Transfer` and `ChangeStatus` capabilities**, on an open current blocked issue. This means the current creator or handler under `server/internal/collaboration/access.go:97`, not an arbitrary source member, stage business owner, model, Agent, daemon or R38 administrator.

The selected target uses the existing current recipient eligibility and enrollment checks under sorted user locks. A target equal to the current handler, including the actor, is allowed for repair: this can be a real status-only recovery. Record `from_user_id == to_user_id` truthfully; do not claim a responsibility transfer occurred. Normal `RequestInformation` keeps its existing self/current-handler target refusal.

Source membership is not added as a blanket requirement to an outside authorized participant. Source/project/stage-policy identity and revision are observed and locked using the final accepted S2 order to establish whether the managed contract applies; that observation is not a grant of private source-artifact access.

No new R38 repair capability is added. Where no normal actor/required pending-delivery target is currently eligible, preserve the actual refusal. Existing independently authorized R38 cancel→reopen with successor can create a **new lifecycle**, subject to its current disabled-creator scope and S2 reset/reconfirmation gates. That is not a return of the old information chain.

#### Selected request shape

Use the existing route, with one new strict discriminator:

```json
POST /api/collaboration/issues/{id}/handoffs
Idempotency-Key: <Krepair>
{
  "purpose": "information_repair",
  "target_user_id": "<explicit eligible account UUID>",
  "note": "<explicit repair explanation>",
  "expected_collaboration_revision": "<decimal>",
  "expected_lifecycle_epoch": "<decimal>",
  "expected_participant_epoch": "<decimal>",
  "expected_account_epoch": "<decimal>",
  "expected_status": "<selected nonblocked source status key>",
  "status_options_version": "<existing catalog/action digest>",
  "information_repair": {
    "context_version": "<server-projected hex64>",
    "active_request_id": "<observed UUID or explicit null>",
    "basis": {"kind": "human_selected"}
  }
}
```

`active_request_id` must be present, even when null. It is an observation, never permission to clear whichever pointer happens to be current. `information_repair` is legal only for the repair purpose. Repair accepts neither new block condition/ETA nor a completion/M1 authorization binding. Transfer and ordinary information requests reject repair-only fields rather than ignoring them.

The service operation kind is **`repair_information`**. Do not describe it interchangeably as ordinary transfer/change_status or reuse either operation's digest kind. `note` is nonblank valid UTF-8 without NUL, at most4096 bytes. Target/status/purpose/source-basis/reference changes under the same key are conflicts, not updates to the repair.

The closed `basis` union is:

| Kind | Additional required fields | Meaning and validation |
| --- | --- | --- |
| `historical_information` | `origin_request_id`, `origin_event_id` | Server verifies the exact currently selected active chain leads to this original information handoff/event, whose actual saved status is nonblocked. Historical role validity is not revived. A missing/corrupt/detached source cannot be selected by guessing the latest timestamp. The selected status must equal that saved currently usable key. |
| `pending_submission` | `submission_id`, `submission_event_id` | Server verifies the exact currently pending submission and original acceptance-submitted event; status must be a currently selectable in_review key, and target must be the current eligible creator. This branch has priority whenever a pending submission exists. |
| `human_selected` | none | A new explicit human decision using current authorized state. Allowed only with no pending submission and a status category in backlog/todo/in_progress. It is labelled as a new choice, never as a historical saved status. |

All unexpected, missing, null or mis-typed discriminator fields are rejected. A historical origin with an archived/unavailable key cannot silently fall back inside repair. With no pending submission, the user can make a **fresh explicitly human-selected** repair. With pending submission, only the pending branch below is available. The existing normal-return fallback remains separately identified and unchanged where valid.

The repair options read uses the existing `GET .../status-options?action=repair_information`, retaining no-store and current object/actor authorization. It returns the ordinary status catalog envelope plus `information_repair_context`:

```json
{
  "protocol_version": 1,
  "context_version": "<hex64>",
  "active_request_id": null,
  "historical_origin": null,
  "pending_submission": null,
  "human_selection_allowed": true
}
```

When present, `historical_origin` names exactly `{origin_request_id, origin_event_id, status}`. `pending_submission` names exactly `{submission_id, submission_event_id, creator_user_id}`. No prose or private delivery/stage source is copied into these options. For this new action `default_status` is the empty string and selection is explicit; no old action changes its default contract. The catalog `options` list contains only admissible categories for the selected context: pending→in_review; otherwise backlog/todo/in_progress plus a genuinely usable historical origin. A conflicting/missing pending-delivery projection is unavailable, not a successful empty no-pending result.

`context_version` binds the actual issue/source/project, current stage-policy contract/revision/enabled state, current status/handler, active pointer and request facts, current lifecycle and relevant actor/target/return-account observations, and the exact pending-submission identity if any. The writer derives and rechecks these under the established locks, together with the four workflow observations and catalog digest. A mismatch is `information_context_changed`; it never selects a new basis on the caller's behalf. Target-specific eligibility is rechecked even when the options context was generated before a target was selected.

#### Transaction and audit effects

Use the existing `mutateCollaborationWorkflow` owner, current actor/target lock discipline, revision conflict/refusal recording and one commit. Within that transaction:

1. Recheck current human identity, role, all observations, source/project/policy applicability, selected source basis and target. No repair is applicable after the issue leaves blocked.
2. Resolve the exact current pending delivery before permitting any ordinary work-status selection. No unavailable read becomes "no pending delivery."
3. Create one **existing `issue_handoff` row with persisted `purpose='transfer'`**, actual actor/from/to, repair note, and `previous_status=the actual current blocked key`. The wire purpose/operation kind identifies repair; the stored handoff remains a truthful existing status/responsibility fact. Its ID is `R`.
4. Perform the real source status/handler change with the existing iteration record and required transaction effects. Record equal from/to for self-target. No separate HTTP calls or visible intermediate partial success.
5. For exactly the information rows this repair changes from `resolution='open'` to `superseded`, atomically set the new nullable **`superseded_by_repair_handoff_id=R`**. Already returned/superseded rows are untouched; an earlier removal's supersession must not be retroactively credited to repair. Clear the active pointer in this same transaction.
6. Append one `information_repaired` collaboration event with exact repair handoff ID, observed old pointer/null, source-basis references, actual prior/restored status, selected target, original request ID and exact affected-row count. Required outbox/invalidation/iteration/audit/receipt failure rolls back every effect.
7. Seal the original workflow operation with `code='information_repaired'` and the final collaboration revision. New receipt extension `information_repair_confirmation` contains `{repair_handoff_id, prior_active_request_id, status, handler_user_id, superseded_count}`. It contains no note/condition/ETA, does not claim `information_returned`, and is available through the existing actor-owned operation GET under its current account checks.

This is a bounded extension of the existing tables/transaction, **not a new repair ledger**. Add `superseded_by_repair_handoff_id` as nullable without a foreign key. The one chosen audit representation is that exact row relationship; do not replace it with an unlimited event array, truncate a list and claim it complete, or guess by timestamp. History for a repair filters by `(issue_id, superseded_by_repair_handoff_id)` and uses the existing bounded cursor/paging style. The stored affected count provides the denominator. Enumerating old rows is a read, never a second repair phase.

Any new index must be a separate CONCURRENTLY migration with cleanup registration. Do not add an index just to satisfy this research. A highest-numbered retained-history down guard must refuse if any new block metadata, repair relation, or `information_repaired` event exists; it must run before dropping supporting fields/indexes. No destructive cleanup or rewriting old requests makes rollback pass.

### 4. Pending acceptance is a protected subcase

`collaboration_delivery_test.go:79` explicitly protects request/return of information during a pending submission; normal return restores in_review with the exact old delivery. Repair must preserve that invariant.

If `currentDelivery` is present:

- Only `basis.kind='pending_submission'` is admissible, with the exact current submission and original event. `historical_information` and `human_selected` are refused for this state, even if a supplied status happens to be in_review.
- The target is the **current eligible creator**. A handler cannot choose themselves or another person to acquire acceptance authority. A disabled/departed/unavailable required target refuses.
- The chosen status must be a current selectable in_review-category source key, confirmed by the catalog digest. A renamed key is fine; an archived saved key requires an explicitly chosen currently available in_review key. This is recorded as a new repair target while retaining the original status source/event; it is not backfilled as the old key.
- Submission ID, author, result, basis, attachments, lifecycle, original digest, stage candidate and M1 authorization bindings remain exactly unchanged. No accepted/returned decision, new submission, execution callback or completion receipt is created. Later real acceptance/return still uses its existing exact ID and final S2/M1 checks.
- If current pending evidence is unavailable or no legal in_review/creator target exists, repair is unavailable. Do not use ordinary status selection as an escape. Existing authorized normal cancel or scoped R38 cancel/reopen remains a distinct disposition with its own lifecycle effects.

The same guard takes priority over archival/custom-status fallback. This prevents repair from silently changing a pending result into ordinary work or giving its acceptance to a new handler.

### 5. Complete record, current actionability and legacy transport

The two original proposed data fields remain `unblock_condition` and `estimated_resolution_at`. The managed reason (`note`) and condition are each nonblank valid UTF-8 without NUL, at most4096 bytes. A new managed information request requires all four facts, including its eligible explicit target.

Handler parsing distinguishes **absent**, explicit null, and invalid values using raw-field presence before Go pointer conversion. Only legitimate absence on an old/unmanaged request becomes nil with `omitempty`; null is not allowed to alias an old omitted field/hash. If either new field is supplied, the new pair and reason must be valid even for an unmanaged request. All old accepted request shapes retain their old hash and replay; the new fields are not inserted during recovery.

Accept RFC3339 timestamps with an explicit timezone and **zero to six fractional decimal digits**. Reject greater precision rather than silently round it into a different future instant. Parse to UTC microsecond precision for persistence, compare that exact persistable instant with the final database clock for a new write, and preserve the original input string in the request digest. A past ETA on a retained row does not make its immutable metadata incomplete and does not reschedule or refuse an exact committed replay.

#### Record projection

In the existing human `current_information_request` and human handoff/activity detail, add one optional strict `block` union; absence means **old-server support unknown**, never a complete record:

```text
{protocol_version:1, state:"complete", unblock_condition:string,
 estimated_resolution_at:UTC-RFC3339-microsecond-string}
{protocol_version:1, state:"legacy"}
{protocol_version:1, state:"unavailable", reason_code:"information_metadata_invalid"}
```

The containing exact handoff already owns reason/note, recipient/to, request ID and original creation actor/time; the block object does not duplicate them. `complete` requires both stored fields and the original reason/recipient to be valid. Both stored fields absent on an old row yields legacy. Partial/inconsistent fields yield unavailable, never legacy or defaults. A legacy row is readable as historical legacy data and remains compatible on unmanaged tasks.

#### Current workflow projection

Add an optional root `information_workflow` projection (absence on old servers is unknown). Its complete v1 shape is:

```text
protocol_version:1
managed:boolean
context_version:hex64
active_request_id:UUID|null
record_state:"none"|"complete"|"legacy"|"unavailable"
actions:{
 request:{available:boolean, code:string|null},
 return:{available:boolean, code:string|null},
 repair:{available:boolean, code:string|null}
}
```

This projection is current authority. `record_state=complete` does **not** imply that its requester, target, parent, source status or pending submission is currently usable. A complete record can have return=false and repair=true. A blocked issue with no active pointer has `record_state=none`, request=false and repair=true for a currently authorized repair actor when a valid basis/target is possible. No read access means the whole protected read denies; it does not send a private unavailable stub.

Use these closed reason codes in this new action projection and corresponding new semantic refusals:

| Code | Meaning |
| --- | --- |
| `information_repair_required` | New managed request cannot legally nest/exit from the current blocked context. |
| `information_metadata_required` | A new managed request omitted required block metadata. |
| `information_metadata_invalid` | Supplied/new stored metadata is malformed or partial. |
| `information_context_changed` | Exact pointer/context observation no longer matches. |
| `information_context_unavailable` | Context/chain provenance is missing or cannot be safely established. |
| `information_repair_not_applicable` | Issue is not currently managed+blocked/open for this new action. |
| `information_repair_basis_unavailable` | Selected historical/pending provenance is not usable. |
| `information_repaired` | Exact new repair committed. This never aliases a normal return. |

Keep existing `forbidden`, `stale_revision`, `stale_authority`, `status_options_changed`, `status_unavailable`, `return_target_unavailable`, `target_unavailable`, `acceptance_pending`, `delivery_unavailable` and `idempotency_conflict` where they already describe the condition. Metadata/bad discriminator errors are400; changed/inapplicable/context/basis/stale conflicts409; current identity/role refusal403; unreadable object404; schema/dependency unavailable503. A transient read failure is unavailable, not proof of an empty context or successful repair. GET404 never permits a fresh key automatically.

#### History and source projections

- Information-request history/detail retains original request ID/event, note/recipient, actual `previous_status`, complete/legacy/unavailable block metadata, current stored resolution and nullable `superseded_by_repair_handoff_id`.
- A normal return cites the consumed request and its actual return handoff/event; actual return actor/time and restored status remain distinct from repair.
- `information_repaired` activity cites its actual repair handoff, actor/time, previous/restored status and source-basis reference. The paged affected-request list is the exact relation above. A repair must display **superseded**, not returned/resolved-by-answer. Existing IDs, safe activity bounds and paging remain.
- Same current issue authorization protects all human detail/history/source reads, including old IDs/cursors and repair relation lookup. Do not copy these fields into source-only reference cards, ordinary inbox bodies or WS hints. The stable S2 business-owner ID remains separate and unchanged.

#### Freeze runtime at strict v1

**Choose runtime v1 unchanged for this increment.** There is no new handoff protocol version, new source kind, new source text or new field in `CollaborationHandoffTransfer`. `collaboration_handoff.go:217/256/351` strictly checks fields/version; `collaboration_handoff_snapshot.go:128` reconstructs a handoff source from its exact note. Therefore an "additive" metadata field in the old runtime envelope is invalid.

New live envelopes/new snapshots with an information request may use the already-existing `missing` array code `information_block_metadata_unavailable_v1`. It means the new condition/ETA/actionability metadata is not supplied by v1. It grants no new source access. Previously persisted snapshots and their original sources/note/hash/bytes remain exactly unchanged; do not insert the new missing marker into recovered old snapshots. A repair's persisted handoff remains a supported transfer fact with its original repair note; it does not pretend to be a normal information return. Human history provides the new structured repair details.

Full new runtime metadata is explicitly **NOT_RUN/not implemented** in this S3 slice. Do not leave a protocol-v2 alternative for an implementer to choose later or silently hydrate new fields during old source reconstruction.

#### Creation options preserve unmanaged compatibility

Current creation options are source-workspace/search based (`collaboration_creation_fields.go:26`); they cannot know the selected project's stage policy. Extend that existing read with optional explicit `project_id` for a selected project, validated using the existing source-project authority. Project-aware results include `stage_policy_observation` with the exact project/policy revision and managed boolean; only then may its selectable statuses omit blocked for managed creation. Without selected project, keep the old/unmanaged choices and mark management applicability unknown. Do not globally remove blocked from all creation options.

Create still resolves selected project/current policy inside its real transaction. A stale/absent option observation is never permission for managed blocked creation. Legacy clients sending blocked to a now-managed selected project receive a clear refusal; old completed creation keys retain their original replay. No new handoff is fabricated during creation merely to fill the fields.

### 6. Complete state sequences

Notation: C=creator, H=handler/requester, Q=information recipient, Z=another eligible recipient; S=valid ordinary nonblocked status. Every request has its own fresh observed versions and original key unless replay is expressly stated.

| Sequence | Actual transitions and final result |
| --- | --- |
| A — creator replaces another recipient | S/H → `B1(H→Q, previous=S,parent=null)` → blocked/Q. Direct creator B2 is refused `information_repair_required`. C explicitly repairs with historical B1 source to S/C (R.previous=blocked; B1 superseded_by=R; pointer=null). C may stop here. Later separate `B2(C→Z,previous=S,parent=null)` → blocked/Z; normal Return B2 → S/C. B1 is never labelled returned. |
| B — current recipient removed | S/H → B1→blocked/Q. C removes Q through the real security API; existing removal returns handler C and clears/supersedes B1, status stays blocked. R by C uses an explicit human-selected S/C with observed pointer=null. B1's earlier removal relation remains null; R does not retroactively claim it. Separate B2 then Return finishes at S/C. |
| C — legacy blocked becomes managed | Legacy blocked/H with no complete active chain → real current policy enable or project association. New information is refused. Authorized H/C explicitly chooses S/current eligible target in R, with no invented origin. R records actual previous blocked and human choice. Separate complete B2 truthfully saves S and returns to S. Legacy metadata remains absent. |
| D — incomplete legacy parent | blocked/H has legacy L as top. A proposed complete nested B2 is refused because it would have to return to incomplete/invalid L. R supersedes the actual open affected rows with exact repair links, moves to S/H; B2 and normal return now work. No fake parent or altered L.previous_status is stored. |
| E — original requester leaves/disables | B1 requester H becomes removed/disabled while Q is current recipient/handler. Normal Return still refuses `return_target_unavailable`; H is not reenrolled. Q, with existing current Transfer+ChangeStatus authority, can explicitly repair to S/Q (self-target truthful), using genuine historical source or explicit human selection if needed. Separate B2 by Q returns to Q, not to H. |
| F — normal complete nesting | S/H→B1(H→Q)→blocked/Q→B2(Q→Z,previous=blocked,parent=B1). Return B2 by Z→blocked/Q with active B1; Return B1 by Q→S/H. No repair occurs, both original return identities are retained, and ordinary nested behavior remains compatible. |
| G — pending submission | Exact D is pending in_review/C. B1(C→Q) blocks while D stays unchanged. A legal repair by C or Q selects pending basis D/event, current in_review key and eligible C: blocked/Q→in_review/C, B1 superseded, D unchanged. Only a later normal acceptance/return can decide D. Human-selected in_progress or target=Q is refused. |
| H — required pending creator unavailable | As G, but C disabled. Normal repair cannot substitute another target or discard D. It remains unavailable. A separately authorized existing R38 cancel and reopen with successor creates a new lifecycle, preserves cancelled/old history and requires S2 reconfirmation; it is not B1 return or normal repair. |

For A/B/C/D/E, **R must visibly and durably commit the nonblocked state before a new B2 exists**. The client must not enqueue B2 as deferred work, auto-submit it after reconnect or reuse Krepair. In B, detached historical B1 may be shown as context but is not an auto-selected origin without a verifiable relation to the current state; an explicit human decision is sufficient and honestly named.

### 7. Keys, partial progress and failure states

| Boundary | Required outcome |
| --- | --- |
| Krepair succeeds, response lost before client knows | All repair effects are committed once. Recover Krepair through the original operation GET or exact explicit same-key replay. No new repair key or B2 is generated by recovery. |
| Krepair not known committed; GET404/503/null/malformed | Keep original repair intent. Preserve existing explicit same-key recovery policy. Unknown is not rejection or permission to start B2. |
| Krepair same body repeated concurrently/after B2 or after target role changes | The existing original receipt remains the result where current actor replay/read policy permits; no recomputation or second supersession. Own content-free GET follows existing current account restrictions. |
| Krepair reused with different target/status/basis/source ID/note | `idempotency_conflict`; previous committed effects unchanged. Missing versus explicit-null new fields cannot collide. |
| New repair key after successful repair, issue still nonblocked | `information_repair_not_applicable`; no new handoff/revision/event. |
| R succeeds, then user cancels the form or never asks B2 | Valid nonblocked repaired state remains. This is a successful completed repair, not an incomplete atomic workflow. |
| R succeeds, then B2's fresh observations are stale | B2 returns its own stale refusal under KB2; R remains successful, issue remains nonblocked. User refreshes and explicitly creates a different fresh request if desired. |
| R succeeds, actor/target permission revoked before B2 | B2 refuses using current authority. R is not undone. No deferred request is created, no old owner revived. A still-authorized actor may act later with a fresh intent. |
| R succeeds, B2 commits but its ACK is lost | State is blocked with **complete B2 whose actual saved status was nonblocked**. Recover KB2 independently; normal Return B2 remains available under current authority. Do not recover Krepair and assume it describes B2. |
| Permission changes after committed B2 | Use the same current-authority normal Return or explicit repair rules as E, never automatic fallback. Old private reads/capabilities remain fenced. |
| Required repair SQL/iteration/event/receipt write fails | Whole repair transaction rolls back: status/handler/pointer/old resolutions/repair links/repair row/revisions/event/receipt unchanged. Preserve the real failure; no partial success claim. |
| Repair wins vs new information/return/remove/policy writer; reverse ordering | Real transaction/PID barriers prove one serial current-state interpretation. Loser rechecks versions/authority/context; no stale pointer clearing or unrecorded chain loss. Security revocation/removal is never blocked by an incomplete metadata requirement. |

The existing helper hashes the service payload before mutation (`collaboration_workflow.go:415`). Optional new fields on old information DTOs use nil pointers with `omitempty`; a zero-value struct is not a substitute. Repair payload is its own strict kind. Normal repair semantic refusals use the existing durable workflow refusal path where applicable; dependency/commit errors do not become a fake persisted refusal.

### 8. Required regression matrix for implementation and independent check

These are proposed tests, not executions. Root assigns exact service/handler/router normal/race databases later.

| ID | Canonical assertion |
| --- | --- |
| V2-01 | A creator replacement: direct B2 refused, one atomic R to nonblocked, later B2 saves true S, normal return succeeds; original B1 facts untouched except exact supersession relation. |
| V2-02 | B actual recipient removal: pointer=null/blocked, repair succeeds, old removal is not relabelled repair, later B2/Return succeeds. |
| V2-03 | C legacy blocked/no chain→managed by policy and by project association; no fabricated metadata, explicit human-selected repair and later return path. |
| V2-04 | D legacy/incomplete/cyclic/missing parent: normal nest refuses, repair does not require pretending chain completeness, later B2 is returnable. |
| V2-05 | E actual requester removal and account disable; old return refuses, self-target repair records equal from/to, absent actor never reenrolled, later B2 returns to the new real requester. |
| V2-06 | F complete normal nested two-layer return unchanged, including custom blocked keys and complete-vs-legacy distinction. |
| V2-07 | G pending priority over human selection: exact submission+event and current creator only; changed status/target/submission/basis refuse. Submission, attachments, stage/M1 bindings unchanged. Archived in_review source requires explicit current in_review selection. |
| V2-08 | H disabled pending creator: normal repair denied; no new R38 repair; existing authorized cancel/reopen/successor retains original evidence/new lifecycle semantics. |
| V2-09 | Same repair key/same body concurrent and after later actions; different note/basis/source/target/status conflicts; new key on nonblocked no-ops/refuses as specified; original refusal never recomputes. |
| V2-10 | Lost repair ACK and original-key recovery; repaired state with no B2; separate B2 stale, actor/target revoked, or lost ACK. Verify independent Krepair/KB2 and no deferred automatic B2. |
| V2-11 | Every required repair write failure rolls back all rows and links. Unknown commit keeps original intent; exact later receipt recovers one effect. |
| V2-12 | Actual source/project/policy, participant/account/target changes in both commit orders with a reached lock/PID barrier. Include missing source membership with independent participation as an allowed control. |
| V2-13 | Current complete record with return denied but repair allowed; legacy row; malformed partial metadata; unsupported old-server absence; revoked source/detail/history cursor denial; no private late hydration. |
| V2-14 | Field presence absent/null/missing pair, duplicate/unknown fields, UTF-8/NUL/4096-byte boundary and escaped envelope, RFC3339 timezone, six-versus-seven fractional digits and final persisted-instant clock. Old accepted hashes/receipt payloads unchanged. |
| V2-15 | Repair event affected count equals full paginated superseded relation ID set with no duplicate/omitted IDs; prior returned/superseded rows unchanged. Retained-history down rejects before field/index removal. |
| V2-16 | Existing runtime v1 decode and source hashes unchanged; new live/new snapshot missing marker allowed; previously saved snapshot raw bytes/identity/note hashes exactly unchanged; no new runtime metadata accepted. |
| V2-17 | Project-aware creation options hide blocked only for actual managed selected project; omitted/unmanaged options unchanged; policy changes before creation refuse at final writer; original create-key replay preserved. |
| V2-18 | Every successful/failed/replayed repair causes **zero new command/input/group/run/Resume/launch/stage-advance/approval-consumption**. Live preexisting execution ledger state remains owned by its existing runtime, not by the block ETA or repair. |

Service tests own transaction/state/key rules; handler tests own strict field presence and transport; Core tests own parsing/persistence/late-response and independent keys; shared Views tests own explicit selection, one atomic repair request, honest superseded display and no automatic B2. Actual API/Web/Desktop acceptance exercises A/B/C/E/G and partial progress with matched source. Do not reproduce the entire backend matrix in component tests.

### 9. Files and implementation boundary

| Source / required future owner | Relevant location |
| --- | --- |
| Single S3 server owner after accepted S2 | `collaboration_handoff.go:152/205`; `collaboration_workflow.go:207/249/403`; `collaboration_lifecycle.go:198/422`; current source/policy locks. |
| Repair persistence and migration | `server/pkg/db/queries/collaboration_workflow.sql`; existing `issue_handoff` schema in `571_collaboration_schema.up.sql:20`. Add only reviewed columns/queries and exact generated counterparts; latest migration number belongs to coordinator. |
| Human transport and source-aware creation | `server/internal/handler/collaboration_workflow.go:30/192`; `collaboration.go`; `collaboration_creation_fields.go:26/85`. |
| Current/detail/history | `collaboration_activity.go`; `collaboration_handoff.sql:2/53`; existing account read authority and cursor patterns. |
| Client wire/recovery | `packages/core/collaboration/action.ts:18/89`; `schema.ts:205`; `packages/core/api/client.ts:1695`; existing source/account fences and original draft persistence. |
| Shared human UI | `packages/views/collaboration/workflow.tsx`; existing current request/activity/history and creation consumers. No app-specific business implementation. |
| Runtime compatibility controls only | `server/pkg/protocol/collaboration_handoff.go:217/256/351`; `collaboration_handoff_snapshot.go:128`; no new protocol version. |

This research owner writes only this v2 Markdown and JSON. Do not edit v1, task state, shared source/tests, spec, migrations or another task. Source hashes in JSON are research captures, not a compiled coherent S2 candidate. Actual implementation waits for root review and the same shared-file ownership handoff as before.

## Related specs

- `.trellis/spec/server/collaboration-foundation.md`: current human workflow ownership, original keys and handoff identity.
- `.trellis/spec/server/collaboration-source-editing.md`: version-bound status/catalog/source authority and real writer barriers.
- `.trellis/spec/server/collaboration-delivery.md`: immutable pending submission and exact human disposition.
- `.trellis/spec/server/collaboration-stage-approvals.md`: current source/policy/final authority and retained evidence; no permission substitution.
- `.trellis/spec/server/collaboration-runtime-recovery.md`: retained runtime identities, no automatic resume/reexecution.
- `.trellis/spec/server/database-test-isolation.md`: exact-package DB allocation and source-bound evidence.
- `.trellis/workflow.md`: role-isolated persisted research and independent contract review before implementation.

## External references

None. Source behavior and the checker conversation are the evidence. Timestamp precision follows the repository's PostgreSQL `timestamptz`/Go `pgtype.Timestamptz` storage boundary; this proposal explicitly rejects sub-microsecond input instead of relying on an implicit rounding policy. No database connection, network service or model call occurred.

## Caveats / Not Found

- S3P-C01 is a research/plan defect; v2 does not claim a product fix, passing regression or current S3 acceptance. Root still reviews the resulting contract.
- The repair action and human DTO additions above do not exist in product code yet. They are the selected bounded recommendation, not multiple implementation alternatives.
- Real runtime v2 block metadata is intentionally outside this increment; human structured history plus the unchanged v1 missing-state contract must not be reported as full runtime delivery of all four fields.
- If current authority, source provenance or pending-delivery evidence makes every legal action unavailable, retain the explicit refusal and legitimate existing administrative path. Do not create an owner, reenroll an unavailable requester or reset a key to make a test pass.
- S2/peer knowledge source remains concurrent. Root must integrate exact owned hunks and rebind final source before tests. Parent matrix and S3 planning status are unchanged by this supplement.
