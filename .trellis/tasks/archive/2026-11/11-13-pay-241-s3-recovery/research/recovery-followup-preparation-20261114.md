# Research: PAY-241 S3 recovery follow-up preparation

- Query: make the remaining S3 attachment/replay, reconnect, conflict, notification, revocation and four-field blocking increment ready for a bounded implementation review, reusing archived timeout-proof and the current S2 contracts.
- Scope: internal source, specification and retained-evidence research. This is preparation, not an implementation or acceptance candidate.
- Date: 2026-11-14
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`
- Research owner: `/root/s3_recovery_prepare`; coordinator: `/root`. No recursive delegation.
- Machine-readable inventory, source SHA-256 values, recommendations, remaining tests and capture drift: `recovery-followup-preparation-20261114.json` beside this file.

## Findings

### 1. Current dependency and evidence boundary

S3 remains `planning`. Its PRD, design and implementation plan already exist. Do not create another business task or repeat timeout-proof. Product implementation waits for the accepted, stable S2 contract and coordinator handoff of the shared server/Core/Views files.

The latest retained S2 reports supersede the initial 09:48Z handoff and the older coordinator summary:

- `server-increment-2b-acl-fix-check.{md,json}` closes S2B-C01 within its frozen increment.
- `server-increment-2c-followup-check.md` closes S2C-C01/C02, permits the same server owner to continue 2d, and expressly does not accept all S2. It binds a 4,939-file reconstruction and independent focused normal15/race15. The earlier 55P03 race failure remains unattributed; there is no evidence establishing its blocker.
- `client-increment-3d-check.md` approves the bounded client increment after C3D-C01/C02; final Core550/Views112. Old route code-free404/405 is unavailable, whereas explicit source denial fences private evidence. These counts are overlapping test evidence, not business cases.
- `server-increment-2d-client-contract.md` freezes normal/R38 source candidates, root completion previews and safe activity. Current numeric issue revision and collaboration decimal revisions are different values. Keep source issuer, actual completion consumer and stable business owner separate.
- The live stage/workflow/API/locale graph also contains another task's knowledge automation changes. `.trellis/tasks/11-14-project-knowledge-automation/task.json` says `in_progress`; its plan explicitly owns lifecycle, delivery, router, knowledge sources, Core and shared UI. Migration files currently reach `860_knowledge_automation_execution_policy.up.sql`. Recheck numbering later; do not allocate S3 migrations now.

The parent CSV still has **4 PARTIAL / 20 NOT_RUN**. TC14/20/21 have partial evidence; TC15/16/22/23 remain open. This research does not change that CSV, task status, implementation checklist or any accepted report.

During this capture, S2 `implement.md` received its14:28Z update: the sole server owner has begun2d; E2E4 has independent13-fixture/static evidence with actual scenario execution still0; the coordinator's isolated production Web build is in progress without a claimed result. This task-document change is recorded in the JSON drift inventory. The research does not infer later build or2d acceptance.

The archived timeout subtask is:

`.trellis/tasks/archive/2026-11/11-13-pay-241-s3-timeout-proof/`

Its `report.md`, `research/check-review.md` and original implementation/runtime JSON establish T01–T06 for the bounded controlled fixture: actual 15s execution timeout, two Stop orderings, lost committed outcome/proof ACK, a different poller PID recovering the original ledger, explicit new successful run and retained old failure. The runtime source was `feb87aee7` plus four runtime/E2E files; two pure unit files ran separately in the root checkout. It proves input-file retention, **not Agent output attachment upload**, real-model quality, current final S2 source or installed Desktop Main. Reuse these facts with their original source identities; do not rebadge them as a new S3 run.

### 2. Files found and ownership map

All paths below are repository-relative. The JSON manifest records exact observed bytes; line references are research-time locations and must be re-resolved after S2/peer edits.

| Files | Existing responsibility / proposed S3 ownership |
| --- | --- |
| `server/internal/service/collaboration_attachments.go`; `collaboration_attachment_access.go` | Encrypted upload identity, current authority, retained binding, native-capability read basis. Reuse; change only after a reproduced recovery failure. |
| `server/internal/handler/collaboration_attachments.go`; same-name tests | Real multipart/byte adapters and post-storage read authorization. Canonical HTTP layer. |
| `packages/core/collaboration/attachment.ts`, `delivery-access.ts`, `delivery-draft.ts`, `delivery.test.ts` | Original upload fingerprints/keys, persisted selection, source/account fences. Headless client owner. |
| `packages/views/collaboration/attachment-picker.tsx`; picker/workflow upload tests | Explicit Recover/Retry/reselect and cancel interaction. Small wiring regressions only. |
| `server/internal/service/collaboration_runtime_capabilities.go`, `collaboration_outcome_receipt_test.go` | Exact recorded outcome ACK versus a new/changed late outcome. Runtime ledger owner remains existing service. |
| `server/internal/service/collaboration_comments_create.go`, `collaboration_comments_edit.go`, `collaboration_comments.go` | Atomic human comment/input, original receipt and current write authority. |
| `server/internal/service/collaboration_comments_recall.go`; handler same-name file; `server/pkg/db/queries/collaboration_comments.sql` | Actual v2 byte-budgeted public/runtime paging; legacy list remains compatible. |
| `packages/core/realtime/collaboration.ts`, `packages/core/api/ws-client.ts`, `packages/core/collaboration/content-queries.ts` | Authenticated reconnect, GET-only invalidation and authorized page requests. |
| `packages/views/collaboration/collaboration-detail.tsx`, `comment-list.tsx` | Page merging, actual Load more, stable comment/editor identity. |
| `server/internal/service/collaboration_edit_test.go`, `collaboration_source_authority_race_test.go`; `server/internal/handler/issue_revision_test.go` | Existing real commit/PID barriers and ordinary issue CAS. Extend with priority/handler-specific assertions. |
| `server/internal/service/collaboration_notifications.go`, `collaboration_mentions.go`, `collaboration_result_notifications_test.go`; `server/pkg/db/queries/collaboration_notifications.sql` | Frozen original recipients, typed target deduplication, proven terminal notifications. |
| `server/internal/service/collaboration_handoff.go`, `collaboration_workflow.go`, `collaboration_lifecycle.go`, `collaboration_creation_fields.go`, `admin_collaboration.go` | **One server owner after S2**: information chain, all root status/handler/creation writers and R38. |
| `server/pkg/db/queries/collaboration_workflow.sql`, `collaboration_handoff.sql`; exact generated counterparts; a coordinator-numbered migration | Persist new information metadata beside existing handoff identity. Shared generated files need hunk-aware integration. |
| `server/internal/handler/collaboration_workflow.go`, `collaboration.go`; `packages/core/collaboration/action.ts`, `schema.ts`; `packages/core/api/client.ts` | Additive request/detail contract and old original-key serialization. Do not replace these mixed files with frozen copies. |
| `server/internal/service/collaboration_activity.go`, `collaboration_runtime_handoff.go`, `collaboration_handoff_snapshot.go`; `server/pkg/protocol/collaboration_handoff.go` | Exact handoff/return actor-time provenance and protected indirect readers. Any runtime metadata extension needs compatibility review. |
| `packages/views/collaboration/workflow.tsx`, `workflow.test.tsx`, activity/run-handoff consumers, existing bilingual collaboration locales | Shared Web/Desktop four-field form and historical display; no platform fork or new store. |
| New suggested `e2e/collaboration-recovery.spec.ts` + `e2e/fixtures/collaboration-recovery.ts` | One E2E owner for the final composed S3 journey; reuse existing auth/native/host helpers. Names are proposals, not claims of existing files. |

A bounded first test increment can use new `collaboration_recovery_*_test.go` files in service/handler instead of rewriting mature suites. Server implementation, SQL, migration/schema readiness, protocol and CLI changes stay with one owner. Client contract work may follow a frozen transport in parallel; E2E authoring uses that contract but executes only against the integrated candidate. Coordinator retains DB, build, real local execution, specification, commits and archive ownership.

### 3. Attachments, uncertain responses and late outcomes

#### Current contract

`UploadAttachmentDraft` uses `(uploader, draft_id, upload_request_id)` and hashes actual bytes (`collaboration_attachments.go:81`). Exact replay checks current attachment authority before returning metadata (`:85`); changed bytes/name/type/size/intended issue conflict (`:163`). Each storage attempt has a private random object and encryption key. The transaction rechecks the original lookup and current context after upload (`:130`). An uncertain DB commit preserves a possibly winning encrypted object (`:153`); a definite loser cleans only its own object.

`GetAttachmentUpload` (`:324`) is a **content-bearing metadata lookup under current attachment authority**, not an unrestricted content-free recovery receipt. It must deny after the uploader loses every read basis. Delivery/comment own-operation recovery is different: `collaboration_comments_edit.go:146` permits an authenticated original actor's minimal receipt after departure, but strips target and attachment IDs when issue access is gone. Do not require every old operation to return the same private JSON after revocation.

Core fingerprints the original file before persistence (`attachment.ts:41`), saves upload intent before network mutation (`:53`), and chooses POST only when given matching file bytes; no file means original-key GET (`:63`). A changed reselected file refuses locally. Scope/access checks surround hashing, the response, and draft mutation. The picker has `retry:false` and separate Recover/Retry/reselect controls (`attachment-picker.tsx:37`, `:100`). Cancellation preserves uncertain original intent; cancellation is not proof that the server did not commit.

For runtime outcomes, `RecordRuntimeOutcome` (`collaboration_runtime_capabilities.go:428`) first checks the issued business-token identity. It may ACK its already recorded exact event after settlement/revocation without returning content (`:446`, `acknowledgeRecordedRuntimeOutcome:534`). A new event must pass live admission/reservation/attempt/identity and cannot replace an old failure. A replayed outcome is not a repeated comment or attachment; those are independent transactions and need separate IDs/counts.

#### Recommended next tests

| ID | Layer and stimulus | Required evidence |
| --- | --- | --- |
| U1 | Existing service unknown-commit + concurrent same upload tests, rerun on final allocated source | One winning attachment ID/digest/ciphertext object, unchanged bytes, exact old-key result; losers delete only their own object. Preserve original failures. |
| U2 | Real Web upload; forward the multipart request to the actual API, lose only its successful response | Persist original request/draft/fingerprint before POST. Reload or restart client, use Recover GET, obtain the same ID, retain one selected file. Independently audit storage and DB; visible chips alone are insufficient. |
| U3 | Explicit Retry while the outcome is unknown, using the same original key and exact file; changed-file reselect negative | Same-key server replay converges. Changed bytes never become a new upload under the old key. GET404/503/null/malformed response does not authorize a new key or delete original intent. |
| U4 | Upload/storage pause followed by source/participant loss, plus a delayed client response released after the fence | Recheck current authority before commit and before client state mutation; no new private selection or bytes. Distinguish source loss with retained participation from loss of all read bases. |
| U5 | Separate explicit delivery/comment with selected upload, ACK loss, exact replay, later display removal/deletion | One attachment binding/reference event per original operation; delivered bytes and frozen command inputs remain; no duplicate comment or notifications. |
| L1 | Already recorded outcome replay versus new/changed late outcomes after timeout/Stop/settlement/revocation | Exact original ACK may remain; changed body/event/attempt/nonce/proof identity refuses. Old failure and termination facts unchanged; zero extra dispatch/claim/permit/launch. |
| L2 | Explicit human new command after authorized recovery | New request/input/run IDs; old failed result/history still present. A late old response cannot overwrite the new UI or revive old execution. |

Reuse `TestCollaborationAttachmentUnknownCommitPreservesOriginalBytes` (`collaboration_attachments_test.go:141`), `...ConcurrentUploadKeepsOnlyWinner:187`, `...UploadRechecksAuthorityAfterStorage:220`, `...CommandSnapshotOutlivesCommentDeletion:238`, and `TestCollaborationAttachmentSlowReadRechecksRevocationBeforeBytes` (handler `:229`). Existing Core `delivery.test.ts:125/166/189/203/215` covers original-key recovery, cancellation, hash fencing, old-server absence and ambiguous denial. Add composed evidence without repeating their complete matrices through DOM tests.

**Current product scope:** router runtime attachment endpoints are read-only (`server/cmd/server/router.go:1428`). Human draft uploads are separately authenticated. Do not claim an Agent output-upload retry was exercised by a Provider that only reads an input file. If TC14 requires a new Agent output-upload product, that remains an explicit missing capability; this research does not introduce it.

### 4. Fifty-five comments, real WS reauthentication and complete paging

The current shared client sends `limit=50,max_bytes=65536` (`content-queries.ts:16`). Because `max_bytes` is present, the real handler selects **`RecallComments`**, not the legacy `ListComments` (`handler/collaboration_comments.go:76`). `RecallComments` counts authorized rows including tombstones in one read snapshot (`service/collaboration_comments_recall.go:201`), reads newest-first with `(created_at,id)` cursor (`collaboration_comments.sql:21`), and returns each page chronologically. Its budgeter (`collaboration_comments_recall.go:123`) may return fewer than50 rows and explicit omitted content; the next cursor must keep the first row that did not fit available. Thus55 records guarantee more than one default page, but do not guarantee exactly50+5 or exactly two pages.

`collaboration-detail.tsx:79` merges page IDs and sorts them; `:133` owns the real Load more button. `comment-list.tsx:27` keeps one component identity when a separately recovered comment joins a page. DOM deduplication can hide a duplicated API page; inspect the returned page chain as well as rendered IDs.

`bindCollaborationRealtime` ignores event sequences at/below the authoritative snapshot or seen bigint cursor (`realtime/collaboration.ts:34`), invalidates on changes, and on authenticated reconnect cancels stale GETs then invalidates (`:125`). It does not Resume or replay a pending POST. `ws-client.ts` authenticates before resubscription; existing `ws-client-account.test.ts:48/75` is mocked transport evidence, not an actual socket journey.

Proposed P1 scenario, separately for production Web and matching Electron renderer:

1. Use two real synthetic accounts and one shared issue. Observe an actual account socket, `auth_ack` and issue `subscribe_ack`, with a connection identity and timestamps. Store only frame type/allowed keys; never tokens or raw auth frames.
2. Disconnect the observer's real transport, not just replace a callback with a fake. The writer commits **55 uniquely marked comments** through the public API, plus specified edits/deletes/replies and one final task-version change. Retain every original request key, comment ID, revision, event ID/sequence and receipt. Pause new writes before the final completeness comparison.
3. Restore connectivity. Require a replacement socket that authenticates before subscribing and then actual authorized GETs. A polling-only or fresh-page-load success cannot by itself satisfy the reauthentication assertion.
4. Use the actual Load more control until `next_cursor` is null. Retain every request limit/max_bytes/cursor and response IDs/returned/total/truncation metadata. For this small-text scenario require complete bodies; if a body is omitted, follow its real content reference or report that content coverage as incomplete.
5. Compare the exact persistent ID set, chronological `(created_at,id)` ordering, revisions, reply parents, tombstones and final task state against the writer ledger. There must be55 unique committed comment IDs, including deleted IDs represented by tombstones; do not count only55 visible nondeleted bodies. Additional pre-existing/control comments use a separately listed denominator.
6. Reconnect/reload again without new intent. Compare counts/hashes and ensure no comment/command/upload/delivery/Resume POST came from recovery. A user clicking a separate explicit Recover/Retry control is another test window.
7. Repeat with the observer removed while offline and with a delayed first/page GET released after known revocation. Private content must not rehydrate. Rejoin allows fresh authorized GETs but does not revive an old access epoch or native capability.

Do not claim a frozen cross-request snapshot that the cursor does not provide. The final equality check uses a quiescent writer and one completed read pass. A separate concurrent-write case can demonstrate eventual recovery to a second known watermark after invalidation, preserving the first pass rather than merging two inconsistent captures into one supposed snapshot.

### 5. Priority versus handler, and ordinary child CAS

The existing service test is stronger than a title-only test but still incomplete for TC16: `editParams` (`collaboration_edit_test.go:25`) writes **both title and urgent priority**, while `TestCollaborationEditAndTransferSerializeAtCommit:304` asserts the final title/handler/history, not the complete priority pair. Reuse its real first-commit barrier and `waitWorkflowBlocked`, and make the new priority-only intent and expected final fields explicit.

`workflowInTx` obtains current authority before applying the requested revision (`collaboration_workflow.go:557`, `:591`). Therefore handler-transfer-first can correctly make an old handler's priority edit403, whereas priority-first gives the competing same-observation transfer409 `stale_revision`. Do not weaken current authority to force both losers to be409.

Recommended service tests:

- Priority A versus priority B from the same collaboration revision, both controlled commit orders: one success, one durable stale refusal; exact winning priority and one edit event.
- Priority-only edit versus handler transfer from the same revision, both orders. Inspect winning handler/priority, ordinary and collaboration revisions, original operation codes/IDs, handoff/event identity, and unchanged execution ledger. Transfer-first former-handler write must remain forbidden even with a fresh key; a still-authorized actor may perform an explicit fresh-intent follow-up after reading current state.
- Same key+same payload converges; same key+different priority or target conflicts. An old refused key remains refused after conditions improve.

For an **ordinary child** of the collaboration root, use actual `PUT /api/issues/{id}` requests with the same explicit numeric `expected_revision`. Change priority and member assignee in both orders, observe a real lock waiter, and require one200/one409 with the winning row and one semantic history effect. The original `TestConcurrentRevisionWritesHaveExactlyOneWinner` (`issue_revision_test.go:308`) uses title/content base comparisons, not this priority/assignee CAS.

`UpdateIssueRequest.ExpectedRevision` remains optional (`handler/issue.go:3426`); guards currently appear at `:4229` and the batch path `:5608`. Keep a missing-token legacy control and report its compatibility semantics separately. Do not silently make CAS globally mandatory or equate source issue numeric revision to collaboration revision. New DB tests use `testutil` rather than copying old open-coded SQL/recorder fixtures.

### 6. Repeated and mixed human/Agent mentions

`parseCollaborationMentions` (`collaboration_mentions.go:27`) deduplicates people by UUID and Agent/squad targets by `(kind,resource_id)` before admission. A plain display string such as `@AI` is not a typed target. `CreateComment` (`collaboration_comments_create.go:37`) stores one comment and event, then appends at most one input per eligible target with that original event identity (`:106`). Content edits do not rewrite frozen inputs.

Human notifications come from the **active collaboration participant set**, not ordinary `issue_subscriber`. The actor is excluded (`collaboration_notifications.go:45`); mentions change `is_mention`. The DB unique identity is `(recipient_user_id, issue_id, event_seq)` (`collaboration_notifications.sql:1`). Both old and new mention sets are expanded against the same locked current participant set for edit deltas (`collaboration_notifications.go:66`). Replaying the original operation must not expand a later recipient set.

Recommended N1 matrix:

| Stimulus | Assert |
| --- | --- |
| Same eligible human twice, also an ordinary participant | Exactly one original-event notification to them, `is_mention=true`; one nonmention notice to each other eligible nonactor participant. |
| Same authorized typed Agent twice | One original target/input/command; later actual claim/launch counted separately. |
| Mixed repeated person + Agent in one comment, exact original HTTP replay before and after settlement/reconnect | Same comment/event/request/target/input IDs, original recipients, one command and no new notification or launch. |
| Same mention, surrounding prose edit, or repeated `@all` after a new participant joins | No new mention-only notification for unchanged mentions, no AI reexecution. |
| Add one new eligible person on edit | One `mention_added` event/notice for that delta; original comment-create recipients remain immutable. |
| Self, outsider, removed/disabled person, invalid UUID, display-only Agent text, suppressed/unavailable Agent | No implicit enrollment, private directory disclosure or duplicate input; target-local refusal cannot erase an otherwise valid human comment. |
| Original recipient removed, rejoined, or new participant joins before delivery replay | Historical recipient set does not expand. Current readers get only currently authorized links/content; old snapshots/credentials do not revive. |

Count `collaboration_notification`, ordinary inbox/subscriber rows, comment events, commands/groups/runs and actual launches separately. Proven result notifications follow terminal group/proof, not merely an outcome or a rendered result card. Reuse `collaboration_result_notifications_test.go:105` and the existing offline-results E2E fixture as complementary evidence. A configurable subscribe/unsubscribe product beyond active participation is not established by the current implementation and must not be faked by inserting `issue_subscriber` rows.

### 7. Mid-recovery revocation and indirect reads

Test at least these identities independently: outside participant; source-only member; source-member participant; original artifact issuer; source manager; ordinary member with no artifact permission; original request actor after departure; business capability for an old run. Removing one read basis must preserve an independent surviving basis, while removing the last basis must fence private content.

| Surface to probe after the same real revoke | Contract / source anchor |
| --- | --- |
| Root detail/workbench, comments legacy+v2/page/detail/history/reactions, activity, memory/versions, delivery history, command/input/run records and human run-handoff | Account routes at `router.go:1670` onward; current `ResolveIssueAccess`/protected readers. Include cached cursors and selected old IDs, not only a fresh first page. |
| Draft original upload metadata, draft/issue list and metadata, authenticated bytes/download, prior signed content/download capability | `collaboration_attachments.go:277/301/324`; handler `:295` reauthorizes after slow storage and before bytes. Source/participant identity is part of native capability, so remove+rejoin does not revive it. |
| R1, private R4/T4, QA report reference, normal/R38 completion candidate/preview | Keep the accepted S2 original-artifact ACL and source issuer/consumer checks. A known R1 denial fences late R4/T4; plain unsupported route404/405 is not evidence of revocation. Do not repeat the closed S2 defect as an unresolved finding. |
| Runtime context, preparation/file read, comment page/full content, memory, live/frozen handoff and handoff source chunks | Runtime routes `router.go:1419`; `lockRuntimeBusiness` in `collaboration_squad.go:85`; `RuntimeAttachment:517`, `RuntimeHandoff:140`, `RuntimeHandoffSource` in `collaboration_handoff_snapshot.go:217`. Use the original run token after revocation; ordinary user/daemon headers cannot replace it. |
| Knowledge candidate/source/approved material derived from a comment, record, handoff or delivery | Trace actual selected source closure in current knowledge readers. This is a concurrently changing peer area; capture final combined source and current API contract before defining assertions. No real model call is required to test read denial. |
| Own workflow/comment/discussion/delivery receipt and already-recorded runtime outcome ACK | May retain only the defined original actor/event/result identity. `collaboration_comments_edit.go:146` strips private targets/files on loss; `collaboration_workflow.go:657` and `collaboration_discussion.go:203` own minimal recovery. Such an ACK does not grant any of the above reads. |
| Queued WS payload and client in-flight GET | Server final-send authorization and client access epochs must prevent stale private hydration. Observe account/issue scope behavior, and verify fresh reauth/GET cannot restore an obsolete generation. |

Use a real API membership/participant/token/grant removal, and deterministic backend barriers where proving transaction order. A fabricated404 response or a fake WS event covers a client parser/fence only. No raw capability, private byte content, auth frame or credential is copied into the public report; use IDs, hashes and sanitized status/code. Already downloaded bytes cannot be recalled; acceptance concerns later reads and live UI/cache behavior.

### 8. Four block fields through the existing information handoff

#### Existing facts and the actual gap

The HTTP handoff carries `target_user_id`, `purpose`, `note`, four workflow observations and directed status observations (`handler/collaboration_workflow.go:30`). `RequestInformation` (`service/collaboration_handoff.go:152`) validates target/current transfer authority, creates or replaces a nested information chain, transfers the handler, writes the blocked source status and records one handoff/event. It has no persisted unblock condition or estimated resolution time; even `note` is not currently required by its dedicated branch.

`ReturnInformation:205` consumes only the exact current top addressed to the actual handler. It does not reinvite a departed/disabled requester. It marks the original request returned, creates a separate `information_return` handoff with `parent_request_id` equal to the consumed request, and restores the saved status/parent (`:245`). The mutable original request row has a resolution string; **the actual return actor/time belongs to the exact return handoff/event**, not an existing `resolved_at` field on that request. Preserve this provenance rather than add fabricated dates.

`CreateCollaborationHandoff` (`collaboration_workflow.sql:36`) already owns issue/actor/from/to/purpose/note/requester/parent/previous_status. Root detail currently exposes the open request's note, requester/recipient, restore status and creation time (`collaboration_handoff.go:409`, Core `schema.ts:205`). Activity joins its exact `handoff_id` and verifies actor/purpose (`collaboration_activity.go:166`). The runtime handoff exposes exact original request/event/source ID (`collaboration_runtime_handoff.go:196`) while source text is currently the note. None of these proves the missing two fields.

#### Recommended frozen request/read contract for the next review

Use the existing request-information operation and original ID; do not create a generic block engine.

- **Reason:** existing `note`, nonblank on a new stage-managed information request. Preserve original note bytes in the request hash; use trimming to validate, not to silently rewrite a saved key.
- **Responsible person:** existing explicit `target_user_id`/recipient, validated under the normal sorted user/participant locks. Do not add or infer another owner field.
- **Unblock condition:** suggested optional wire `unblock_condition`, nonblank valid UTF-8, bounded to4096 bytes. Required on a new stage-managed information request.
- **Estimated resolution:** suggested optional `estimated_resolution_at`, an explicit timezone-bearing RFC3339 instant normalized for storage as `timestamptz`. Required on new managed requests; validate that a newly created estimate is later than the final transaction clock. Never reject or rewrite a previously committed exact original request because its estimate later passes. The estimate is informative; expiry must not schedule, auto-return, auto-Resume or reopen anything.

4096 bytes and the field names are concrete **research recommendations for the bounded review**, not an existing frozen S3 wire contract. Apply a compatible finite bound to the managed reason too, and freeze how the existing64KiB JSON envelope handles escaped Unicode before implementation. The field matrix must test missing/null/empty/whitespace/invalid UTF-8 or timestamp/offset/oversize/unknown/duplicate keys. Avoid a new arbitrary maximum number of future days without a product reason.

Persist the two nullable fields on the original `issue_handoff` information row. No FK or new secondary ledger is necessary. Do not backfill legacy values, derive ETA from due date, or infer condition from prose. Both missing means legacy/unmanaged; partial or malformed present new metadata is unavailable/invalid, not a default complete record. Additive current/detail/history projection should expose the two together under an explicit complete-versus-legacy contract; absent old-server metadata must not make an installed client believe the new managed requirement is satisfied.

Use the existing `loadStageRoot` semantics (`collaboration_stage.go:203`): a collaboration root with an enabled matching current project policy is managed, even without a materialized stage-task row. Obtain/recheck this applicability using S2's accepted source/project/policy lock order; an unlocked policy read in the action is insufficient. Stage policy and M1 off/observe/enforce are different rules. New-block completeness must not disappear because M1 is off or observe.

Old/unmanaged requests retain their existing behavior. Add optional Go pointer fields with explicit `omitempty` semantics that preserve old `json.Marshal(p)` payload hashes (`collaboration_workflow.go:415`); an all-zero value struct is not automatically omitted. Verify explicit old-key request bodies and stored receipts before changing the DTO. Parse/normalize timestamps for storage without mutating the already identified request. Core `action.ts:21` and `api/client.ts:1695` must preserve absence on old draft replay, rather than silently insert empty/null fields. Do not rename `note` or mutate an old request with new fields. After a confirmed refusal, a user may submit a fresh complete request explicitly; automatic reconnection does not do so.

Keep a completed new block's four facts and original request identity available through exact current/history reads. Return should project original request ID, actual return handoff/event ID, returned-by and returned-at from the authoritative rows, plus the actual restored status and existing fallback flag. It must not report the current handler, stable business owner or issue `updated_at` as the original resolver.

Runtime snapshot/source compatibility needs a separate explicit decision in this same review. Existing v1 handoff sources hash only `note`; silently changing that source text to concatenated new fields would break retained snapshot hashes. Prefer additive versioned structured block metadata bound to the immutable request and included in newly formed snapshot bytes, while preserving old snapshot/source reconstruction exactly. If it cannot fit the existing strict reader/budget contract, retain the old protocol and mark the new metadata unavailable there until a negotiated version exists. Update human run-handoff, protocol/CLI/builtin source maps only if that transport actually changes. Do not duplicate private metadata into unrestricted activity or WS hints.

#### Complete blocked-entry and chain-integrity inventory

| Actual path | Current behavior | Required S3 handling / verification |
| --- | --- | --- |
| `RequestInformation` → `handoff` | Prescribed blocked category; may create a nested/replacement request (`handoff.go:170/191`; `workflow.go:227`) | Validate all four facts before enrollment/status/handler/pointer/event writes. One original operation owns every effect; required-write failure rolls back. |
| `PATCH /collaboration/issues/{id}` status → `ChangeStatus` | `collaborationStatusCategoryAllowed` accepts blocked for `change_status` (`lifecycle.go:68/422`) | Managed direct entry to blocked must refuse with an actionable existing-handoff requirement; hide/route that option in shared UI. Do not accept an unbound four-field payload on a separate generic status writer. |
| Human lifecycle `reopen` | Any permitted nonterminal except in_review includes blocked; `changeLifecycle` then clears old chain (`lifecycle.go:348/523/570`) | Managed reopen-to-blocked must refuse. Explicitly reopen to a supported nonblocked status, then issue a complete new information request. Keep queue paused and original lifecycle/receipt semantics. |
| R38 `AdminCollaborationService.ChangeLifecycle` | Uses the same status validator and `changeLifecycle`, with optional successor (`admin_collaboration.go:200/227/260`) | Cover actual admin reopen-to-blocked as well; do not assume the human route guard covers this caller. Cancel/complete and original admin recovery retain S2 behavior. |
| `CreateInTx` → `prepareCollaborationCreationFields` | Initial status accepts every nonterminal category, including blocked (`creation_fields.go:85`) | A new managed root must not start blocked without a complete bound handoff. Minimal recommendation: refuse that initial status and require create nonblocked→information request. Creation options must match, including custom blocked keys. Do not fabricate a request in the creation projection. |
| `ReturnInformation` restores saved blocked status | A nested return can legitimately reactivate its open parent; an old standalone saved blocked status also remains selectable (`handoff.go:95/253`) | Require the exact complete open parent assigned to the restored recipient when a **new managed return** lands blocked. Never invent a parent. Legacy/unmanaged returns retain compatibility; an incomplete managed legacy context is explicitly unavailable and requires deliberate repair, without rewriting its original receipt. |
| Direct nonblocked status edit while an information request is active | Current `ChangeStatus` can change status without clearing/returning the active information chain | Managed unresolved block must use the exact return/cancel path; a generic status edit cannot falsely count as resolving the request. Include custom nonblocked targets and same-status no-op controls. |
| Ordinary `Transfer`, current-handler removal | `changeHandler` supersedes the chain but preserves current status (`workflow.go:207/283`) | Do not leave an apparently complete current block with an invented recipient. Suggested ordinary transfer while managed+blocked: require a new complete information handoff instead. **Security removal must still succeed**; retain the old record as superseded/unavailable and show recovery-required until an authorized explicit complete request is made. |
| Account disable/revocation | Current authority becomes unavailable; source search found no direct account-disable writer that supersedes the information pointer | Test the retained request becoming unavailable; do not label it returned or claim the pointer was automatically cleared. Security revocation must not depend on block metadata validity. |
| Submit/return acceptance, terminal cancel/complete | Acceptance uses in_review/in_progress and can supersede information; terminal lifecycle supersedes all (`handoff.go:274/351`, `lifecycle.go:573`) | They are not extra blocked-entry writers. Test that they cannot fabricate a successful unblock, lose the historical four facts or bypass S2 final completion gates. |
| Source project reassociation / policy enable with a preexisting blocked root | Changes whether `loadStageRoot` is managed, although not a status write | Keep history/legacy classification explicit. Never backfill a fictitious condition/ETA or reinterpret a committed old key. Recheck policy/project at every new transition; cover this boundary in review and retain S2 project-exit guards. |
| Ordinary issue PUT/batch/move, PR merge/failed-task reset | Ordinary handlers route/reject collaboration writes; `WriteIssueStatus` checks `IsCollaborationIssue` and returns unchanged (`issue_status_write.go:63`); move delegates to UpdateIssue | Retain actual negative controls for a collaboration root. Ordinary children keep ordinary semantics; four-field policy in this increment is for managed roots. |
| Workspace status catalog | Key/category are immutable after create (`handler/issue_status.go:80`); labels/order/archive may change | Compare resolved **category**, never only literal `status == "blocked"`. Test custom blocked key, catalog change between read/write, and saved status archived/fallback. Renaming does not itself change semantics. |

All three source status write primitives observed are `handoff`'s direct `UpdateIssueStatus`, `recordStatusChange`, and creation's ordinary issue insert; `recordStatusChange` alone cannot protect the first or third. The independent check must repeat the writer search against final source, especially after S2 2d and knowledge changes. No database trigger or second workflow is proposed.

Recommended block tests B1–B8: complete managed request + unchanged business owner; each missing/invalid field; exact old-key replay/hash compatibility and unknown response recovery; nested returns and exact actor/time/restored status; normal/admin/creation/custom-status bypass refusals; both block-write versus policy/source/participant change commit orders; removed/disabled recipient and security-removal integrity; malformed/legacy read and cross-client form recovery. A return/time estimate must never dispatch or Resume an Agent, advance the business stage, or consume an M1 approval.

### 9. Sequence, test ownership and acceptance handoff

1. **Before product edits:** coordinator accepts S2 and freezes its combined source plus peer boundaries. Re-resolve all references, final writer inventory and current schema readiness. Hash the selected candidate. Keep original failing reports and old source frozen.
2. **Bounded composed-regression increment:** service/handler U1/U4/U5/L1/CAS/N1, canonical Core late-response/original-key gaps, and E2E U2/U3/P1/N1/revocation authoring. Prefer existing helpers; no production changes unless a failing regression identifies one. Archived timeout-proof remains reused evidence with a separate final-source smoke if justified by later shared changes.
3. **Freeze S3 information contract:** one server owner reviews request/read fields, migration and original-key serialization; inventory all status/handler paths above. Client owner consumes the frozen additive contract. Preserve normal/R38 M1 completion semantics and safe S2 activity. No general workflow/scheduler.
4. **Implement blocking with focused red→green:** migrate only coordinator-selected package DBs after DDL/sqlc/compile freeze; test actual SQL readiness, old rows and retained-history down refusal. Nullable columns likely require no new index. If an index is needed, separate CONCURRENTLY file and register retry cleanup.
5. **Independent normal/race and static review:** exact service, handler and actual-router packages each use their own assigned DB, with normal/race separated as assigned. Use genuine transaction/PID barriers for both orderings, no sleeps. Protocol/CLI use the allowed private no-account HOME/TMP launcher. Core and Views own parsing versus visible wiring once, plus scoped lint/types. Counts include their exact named inventory and skips; do not merge rerun denominators.
6. **Actual final S3 journey:** new source-matched API/CLI/base Host/Git harness where required, production Web and matching Desktop. Use synthetic accounts and real membership APIs, explicit test-only paths and private controls. No model account, payment system, production deploy or public fixture endpoint. Keep real socket auth, exact original keys, complete comment page chains and independent comment/file/result/notification/launch counts.
7. **Full independent S3 check:** match all R01–R09/TC14–16/20–23 subassertions to a source-bound artifact. Missing real model semantics, Agent output upload if still unsupported, installed Main or external systems remain specifically NOT_RUN, not implied passes. Only the coordinator updates the parent matrix/remaining gaps, applies Lore commits and archives a task that meets its own acceptance.

The researcher ran no tests, builds, DB queries/migrations, APIs, browser/Provider/model calls, Git operations or task-state commands. No implementation/check JSONL was loaded. Existing test names establish code locations, not current execution results. The above commands/check groups are a proposed permit request for the coordinator, not authorization to borrow an old package/API database. No environment configuration or secret was read.

## Related specs

- `CLAUDE.md`: package boundaries, API fallback/parsing, default no-real-agent tests, DB fixtures and migrations.
- `.trellis/workflow.md`: research is durable Phase1.2 preparation; native implementation and independent checks belong to their assigned owners.
- `.trellis/spec/server/collaboration-content.md`: upload/draft identity, encrypted objects, current native byte authority and immutable references.
- `.trellis/spec/server/collaboration-foundation.md`: current object/account authority, original workflow keys and directed handoff contract; historical runtime sections are not the current host execution contract.
- `.trellis/spec/server/collaboration-source-editing.md`: CAS, source membership fences, current editor authority and content-free invalidation.
- `.trellis/spec/server/collaboration-realtime.md`: account scopes, final-send authorization, GET-only reconnect and actual socket evidence.
- `.trellis/spec/server/collaboration-runtime-recovery.md`: original proof/outcome/recording identity, bounded recovery and explicit Resume.
- `.trellis/spec/server/collaboration-stage-approvals.md`: original issuer/artifact ACL, historical stale report reads, R1/private-cache fences, schema and retained evidence.
- `.trellis/spec/server/database-test-isolation.md`: per-package allocation, API isolation, source-bound diagnostics and no ambient account HOME.
- Core/Views spec indexes and `.trellis/spec/guides/index.md`: shared headless/UI separation and one canonical test layer per behavior.

## External references and versions

No external network source was needed or contacted. Primary sources are the repository code, checked reports and specs listed above. Version references are declared dependencies, not a claim that they were installed/run here: Go1.26.6, pgx/v5 v5.9.2, gorilla/websocket v1.5.3 (`server/go.mod`); TanStack Query ^5.96.2, Zod ^4.1.5, Zustand ^5.0.0, TypeScript ^5.9.3, Vitest ^4.1.0 (`pnpm-workspace.yaml`); Playwright ^1.58.2 (`package.json`). There is no new dependency recommendation.

## Caveats / Not Found

- No current complete S3 follow-up, actual55-comment reconnect, field-specific two-order ordinary-child race, composed mixed notification journey or complete four-field block contract has been accepted. This document is a reviewable next increment, not those results.
- Exact source hashes are a capture of a changing shared workspace; S2 and peer knowledge writers were active. The JSON preserves initial/final captures and any differences. An unchanged sampled file is not a frozen or compiled whole graph. Never copy a frozen whole shared file over live peer changes.
- Existing automatic reconnect is GET-only. An explicit `recoverCollaborationAction` (`action.ts:89`) can fall back to the **same** original write on generic GET404; bound M1 completion explicitly refuses that fallback. Do not broaden the E2E no-write window to a user-requested action or change this compatibility behavior inadvertently.
- The four-field proposal's validation bounds, additive history projection and runtime metadata version still need the scoped independent contract review before implementation. The product decision to enrich the existing information handoff and keep stable S2 owner distinct is already settled; no user permission round is needed for routine implementation choices.
- Read authority loss is not always loss of both source and participation. Old stale reports remain readable by a still-authorized artifact reader; invalid current applicability is a different property.
- S4 genuine external systems and S5 final-candidate performance remain separate, and parent acceptance stays open. Controlled provider labels/test-account signatures must remain explicit.
