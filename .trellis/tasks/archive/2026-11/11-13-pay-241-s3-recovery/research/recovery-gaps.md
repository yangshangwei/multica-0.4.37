# Research: S3 recovery gaps and independent regression slices

- Query: what S3 can verify or improve independently of undecided S2 business roles, including real controlled timeout, immutable retries, conflicting writes, batched reconnect, notification deduplication, and blocking metadata.
- Scope: internal source/spec/test research; only this document was written.
- Date: 2026-11-13
- Requirements: parent R07/R08/R09/R10; TC14/15/16/20/21/22/23.

## Findings

### 1. Recommended independent implementation boundary

Existing code already has original request identities, immutable execution history, bounded recovery, transactional CAS, durable outbox and notification deduplication. The first independent S3 increment should add missing **composed verification** and a small **test-only timeout input**, then fix only failures actually reproduced. Do not add a second retry/outbox/notification subsystem or choose S2's business-role/phase policies.

Suggested disjoint ownership:

| Slice | Suggested files | Why |
| --- | --- | --- |
| Real process timeout and receipt recovery | new `server/internal/daemon/collaborationrun/host_timeout_recovery_test.go`; narrow fixture changes in `host_runner_test.go` only if required | Existing fixture starts a real test-owned executable and has original proof/outcome capture. No S2 decision needed. |
| Actual API host timeout acceptance | `server/internal/daemon/collaboration_host_router_test.go`, `e2e/fixtures/collaboration-host-runtime.ts`; new fixed-name S3 E2E/fixture | Test poller currently hardcodes a timeout longer than the Provider's own callback deadline, so simply withholding release proves the wrong failure. |
| Runtime/business replay and field conflict | new S3-specific tests under `server/internal/service/` and `server/internal/handler/`, reusing existing fixture/barrier helpers | Avoid editing central production writers unless a new regression demonstrates a defect. |
| Batched reconnect and notification matrix | new fixed-name `e2e/collaboration-recovery.spec.ts` and a focused shared fixture; existing Core realtime tests if a lower-layer defect appears | Current browser evidence is smaller than TC15/22's required batch/mixed-recipient assertions. |
| Blocking contract | planning only until its intersection with S2 policy/owner semantics is settled | Four required fields are not currently a persisted contract. |

Names above are proposed new files, not existing entry points. Give all new DB tests `testutil` fixtures and exact package permits; do not copy legacy open-coded INSERT/recorder patterns from old tests.

### 2. True controlled process timeout: precise gap and test seam

`server/internal/daemon/collaborationrun/host_runner_test.go:132` constructs `newHostRunFixture` using `os.Executable()` as the explicit Provider. It sets no real account or user-installed CLI. The fixture's `wait` mode (`:82`) writes its launch capture, then waits up to eight seconds for a private release file. `HostExecutor.Options.Timeout` is ten seconds (`:154`). Existing tests prove original permit/proof replay (`:410`), actual wait after stdio EOF (`:438`), transient offline completion (`:460`), and one fresh-session fallback versus Stop (`:500`). None of those explicitly drives the configured execution timeout.

A narrow regression can use the existing `wait` fixture, set `f.runner.Host.Options.Timeout` below the fixture wait, wait for the actual process-start marker, and withhold release. Observe:

1. One actual Provider start with the original run/input identity; issued permit and launch lifecycle observations are present.
2. A bounded real timeout and actual termination/wait proof; do not manufacture a `timeout` result struct.
3. The resulting failure is preserved. `host_recording.go:214` maps timeout/failed/cancelled to business `failure`; when output is absent it retains `result.Error` (`:229`). Do not invent a new run-state enum merely to make the UI say timeout.
4. Lose the first outcome/proof ACK using the existing fixture knobs, persist the ledger, then construct a **new** Runner using the same on-disk ledger/client.
5. Recovery confirms identical request IDs, attempt sequence/nonce, outcome payload, proof receipt, and zero new Claim/Permit/Provider starts. Check a second recovery is also inert.
6. A separately authorized fresh retry creates a new run/attempt identity and retains the failed original; never change the old failure to success.

`host_runner.go:225–248` only performs automatic fresh-session retry for explicit resume rejection, not arbitrary failure/timeout. Keep that policy. `host_recording_recovery_test.go:49/121/217/233/332` already proves backlog deadlines, old-seal-before-advance, cancelled backlog preservation, Stop/proof before a blocked upload is released, and malformed ACK retention. Reuse those cases as complementary evidence, not a substitute for the new real-timeout case.

#### Browser/API timeout trap

`server/internal/daemon/collaboration_host_router_test.go:136` fixes `AgentTimeout` to three minutes. `e2e/fixtures/collaboration-multi-account-provider.go:241` fixes its TCP callback read deadline to150 seconds. Withholding `daemons.release()` therefore produces callback failure **before** Agent timeout. Such a run must not be labelled execution-timeout proof.

Minimal test-only change: add an explicit optional bounded timeout field to `collaborationHostRouterInput`, retain the current three-minute default when absent, reject invalid/negative/excessive values, and pass it only from the S3 fixture. Bind through the existing bounded stdin config, not a new production setting or ambient account option. A timeout of a few seconds gives enough time to observe actual startup and keeps it below the callback deadline. Validate the helper input at the canonical Node fixture boundary and the daemon harness boundary without changing existing defaults.

The closure Provider already performs real run-local CLI writes **before** it announces the callback (`multi-account-provider.go:188`, `:201`) and then waits for release (`:261`). A S3 fixture can therefore demonstrate a persisted comment/context update followed by a real timeout, retained failure and no repeated mutation during recovery. Avoid changing the S1 five-run happy path or its callback protocol globally. For explicit process-failure cases, use a test-owned controlled exit mode; an unconfigured real model is not needed.

### 3. Proving zero repeated business effects, not merely one response

`server/internal/service/collaboration_runtime_capabilities.go:425` owns `RecordRuntimeOutcome`. It writes a run outcome event/business result and realtime invalidation; it does **not** create a comment or attachment. The Agent comment is a separate run-local CLI activity. Measure all relevant sources separately.

The existing outcome replay contract is unusually important:

- An originally issued business capability may ACK its already recorded exact event after revocation or settlement (`:443–447`); this returns no stored content and performs no new business write.
- New events still check current capability, admission, reservation, exact frozen identity and attempt (`:449–473`).
- Same event with changed payload is `outcome_conflict`; a new event cannot replace an existing outcome (`:475–500`).
- `acknowledgeRecordedRuntimeOutcome` (`:531`) binds original attempt, containment and termination identity before comparing the entire saved payload.

Recommended DB/API assertion addition: record one failed outcome and a run-local comment, then snapshot immutable IDs/payload hashes and counts for `collaboration_run_event`, `comment`, relevant command/group/run records and notifications. Replay the exact outcome/comment requests concurrently and after settlement/revocation. Expect the original ACK only where the existing content-free recovery contract permits it; expect changed payload/new request attempts to be denied. Assert exact stable IDs and unchanged business counts/hashes. Exclude asynchronous outbox-delivery markers from equality, but retain event identities. After current read authority is revoked, comment/record/attachment reads and broad machine reads remain denied; an ACK must not hydrate content.

Attachment retry already has real storage coverage:

- `collaboration_attachments_test.go:141`: lost commit retains the original encrypted object; `GetAttachmentUpload` and exact replay return the original ID with no second transaction; changed bytes conflict.
- `:187`: two concurrent same-key uploads produce one retained ciphertext object/ID and matching bytes, with loser cleanup.
- `:220`: membership lost after storage upload prevents DB commit and cleans only the rejected attempt's object.
- `:238`: delivered attachment references survive comment deletion/frozen command input.
- `server/internal/handler/collaboration_attachments_test.go:229`: slow byte reads recheck revocation.

Do not repeat this whole matrix in DOM tests. Add a browser lost-upload-response plus original-key recovery assertion only if S3 needs actual client wiring evidence; count exact storage/DB IDs, not only visible chips. Agent outcome retry and human attachment-draft retry are different scopes and must be reported as such. Never claim runtime attachment upload that the tested Provider did not perform.

### 4. Concurrent assignee and priority: current protection and missing specificity

`server/internal/service/collaboration_edit_test.go:266` tests same-key and different-key concurrent **title** edits. `:304` exercises real PostgreSQL commit barriers for **title edit versus transfer** in both orders, including late-role refusal. `collaboration_workflow_test.go:406` tests workflow key/revision competition. These establish reusable mechanics, but do not themselves identify priority/assignee-specific outcomes required by TC16.

Recommended additions, reusing `lifecycleCommitBarrierStarter` and `waitWorkflowBlocked`:

- Same observed collaboration revision: priority A versus priority B yields one commit and one `stale_revision`; no mixed state or duplicate activity.
- Same revision: handler transfer versus priority edit, both commit orders. When transfer wins, the former handler's later edit may correctly be403 rather than409 because current authority is checked. Assert unchanged losing field, exact winning handler/priority and event/receipt IDs.
- Same request key + same payload converges to the original result; same key + different priority/target is an idempotency conflict.
- After a deliberate fresh read and explicit new request, a still-authorized actor can make the intended subsequent change; preserve the stale request's original refusal.
- Ordinary linked child: exercise two actual handler requests changing priority/assignee with the same explicit `expected_revision`, assert exactly one success and one409 plus canonical row values/history. Existing `issue_revision_test.go:308` only races title/content baselines, so it does not cover this field pair.

Ordinary issue `ExpectedRevision` remains optional (`server/internal/handler/issue.go:3426`, checks at `:4218`, batch path `:5595`). Do not silently make it mandatory for all old clients during S3. Report the tested explicit-version contract separately from missing-token compatibility. A policy requiring rejection of all blind legacy writes belongs in a reviewed compatibility/enablement decision, aligned with S2.

### 5. Batched disconnect/reconnect: add a composed browser assertion

Core already implements read-only recovery:

- `packages/core/realtime/collaboration.ts:34–42`: exact bigint event-sequence dedup against snapshot and seen cursor.
- `:58–67`: access loss advances the fence/clears protected state, then performs authorized reads.
- `:125–132`: reconnect cancels stale reads and invalidates queries, without cancelling/replaying a pending POST.
- `packages/core/collaboration/content-queries.ts:7/16`: comments/activity/notifications are paginated Query state, with mount/focus/reconnect recovery and15-second polling fallback.
- `packages/core/realtime/collaboration.test.ts:171`: GET-only reconnect and missed revocation; `:184` stale/malformed frames; `ws-client-account.test.ts:48/75` auth-before-resubscribe and old-socket callbacks.

The existing offline-result scenario contains six retained workflow/run notification kinds and current Web/Electron recovery; it is valuable but not a many-page discussion batch. S1's single-comment reappearance cannot certify TC15's whole batch.

Suggested independent browser fixture:

1. Two independent accounts, one shared task, actual account WS authentication/issue subscription observed before disconnect.
2. Close/disconnect one observer while the other commits at least55 uniquely identified comments (crossing default page size50), selected edits/deletes, and known task version changes. Record every request/receipt/comment/event identity. No Agent needed for this batch subcase.
3. Reconnect without resubmission. Observe a new authenticated socket and the required fresh authoritative reads; fetch/render all pages using the UI's real load-more controls.
4. Compare exact persistent ID sets and current revision/content/tombstones with the writer's committed ledger; assert no repeated IDs, no missing expected objects, and no new POST/dispatch caused by reconnect or reload.
5. Repeat reconnect with duplicate/stale hints only at a lower-layer test seam; distinguish real WS evidence from fake-frame unit tests.
6. Separate removal-while-offline subcase: on reconnect content/attachments/run details disappear and all direct/indirect reads deny; retained own receipts remain content-free only as allowed. A polling-only success is not proof of actual WS integration.

Do not assert all55 comments appear without paging if the product intentionally exposes the first50 with a load-more affordance. "No loss" means the authorized persistent/page chain is complete; WS is invalidation, not a durable frame transcript.

### 6. Mixed people/Agent mention and notification deduplication

Current collaboration subscription semantics are **active participants**, not ordinary `issue_subscriber` rows. `collaboration_notifications.go:45` sends one comment notification per participant except the actor and marks `is_mention`; `:68` sends edit notifications only for newly added people/`@all` membership under one locked participant set. `collaboration_workflow.sql:88` excludes revoked/disabled participants. Plain text, invalid user IDs or Agent display names do not create human subscriptions or authority.

Existing authoritative tests:

- `collaboration_notifications_test.go:14`: repeated exact body edit and unchanged mentions do not duplicate notifications; outsider is not enrolled, no Agent dispatch from `@AI` text.
- `collaboration_result_notifications_test.go:105`: required notification failure rolls back terminal proof; concurrent exact proof produces one per original recipient; a later joined participant does not receive a replayed historical result.
- `collaboration_comments_test.go:399`: comment-edit mention delta uses current participants.
- `e2e/fixtures/collaboration-offline-result-notifications.ts:194`: three real namespace processes (completed, missing outcome, stopped), proof withheld/released/replayed, one result notification per recipient and no duplicate source inbox/subscriber rows.

Recommended additional service + UI wiring matrix:

1. Same person appears repeatedly and is also an active participant; a single comment event creates one notification for that person, flagged mention, and one ordinary notification for each other eligible participant.
2. Repeated mention of the same authorized Agent creates the existing single target/group input, not one dispatch per textual occurrence; compare original command/input/group IDs.
3. One comment mentions both person and Agent; replay its exact HTTP key/body before/after result settlement and after reconnect. Assert immutable comment ID, original recipients, one Agent command, no extra launch, and no new notice rows.
4. Edit surrounding prose or keep the same mention: no new mention notice/Agent execution. Add a new human mention: exactly one delta notice under current authority. A revoked/disabled/outsider mention does not enroll or reveal that person's private source.
5. Actor self-mention is excluded. Ordinary workspace-only subscriber without collaboration access must not receive private collaboration body/anchors through a second inbox.

If the parent requires a separately configurable subscribe/unsubscribe feature beyond active participation, current code does not implement that product concept. Preserve that gap rather than populating `issue_subscriber` to make a test pass.

### 7. Blocking metadata: real implementation gap requiring S2 alignment

Current blocking uses information handoff:

- HTTP `collaborationHandoffBody` (`server/internal/handler/collaboration_workflow.go:30`) has workflow observations, target user, purpose, note and status option observations. There is no unblock-condition or expected-resolution timestamp.
- Service `CollaborationInformationRequestParams` (`collaboration_handoff.go:20`) embeds transfer plus status observations. The view (`:59`) exposes request ID, requester/recipient, note, previous/return status and created time.
- `RequestInformation` (`:152`) requires current transfer authority, validates recipient, records exact nested request and sets the active pointer. It transfers current handling to the information recipient.
- `ReturnInformation` (`:205`) only consumes the current request addressed to the current handler; it refuses a departed/disabled requester. It writes a separate `information_return` handoff linked through `parent_request_id` and restores saved status (fallback only if the saved source status is unavailable).
- `server/pkg/db/queries/collaboration_workflow.sql:36` stores `actor_user_id/from_user_id/to_user_id/purpose/note/requester_user_id/parent_request_id/previous_status`; `:61` marks resolution returned. Return actor/time already belongs to the separate exact handoff/event; do not fabricate it from current handler/updated_at.
- Public projection: `server/internal/handler/collaboration.go:72/86`; Core `packages/core/collaboration/schema.ts:209/265`; shared `packages/views/collaboration/workflow.tsx:236/288`.

R10's four mandatory fields (reason, accountable owner, release condition, estimated release time) are **not** all represented or enforced. The information recipient/current handler is not automatically the stable business owner intended by S2. Request-information is also not a generic arbitrary block/unblock operation.

Recommended planning contract: decide whether the new S2-enabled task's blocking record enriches the existing handoff or is a distinct policy-owned record; reuse current request identity/restore-status/events. New metadata must be validated atomically before a block transition, version-bound, and checked at every applicable status writer. For a proper unblock, retain exact actor/time, consumed block identity and restored business phase/status. Do not globally require new fields on legacy information requests or change `in_review` semantics without the S2 enablement/compatibility decision.

Keep TC23 open while only the existing request/return loop is verified. Do not count that old loop as proof of absent condition/time fields.

## Related specs

- `.trellis/spec/server/collaboration-runtime-recovery.md`
- `.trellis/spec/server/collaboration-command-history.md`
- `.trellis/spec/server/collaboration-realtime.md`
- `.trellis/spec/server/collaboration-source-editing.md`
- `.trellis/spec/server/collaboration-delivery.md`
- `.trellis/spec/server/database-test-isolation.md`
- `.trellis/spec/web/frontend/collaboration-closure-qa.md`
- S2 environment handoff: `../11-13-pay-241-s2-stage-approvals/research/verification-plan.md` (relative to this task directory).

## External references

None: all findings use repository source, tests and preserved contracts. No external model, payment, repository account or deployment was contacted.

## Caveats / Not Found

- No tests were run and no code/environment/database/Git state was changed. Test presence is not a current pass.
- The suggested composed browser/timeout cases require fresh task-owned API/database/production Web/Desktop builds; each result must keep its exact source/configuration identity and failure history.
- Controlled executable timeout establishes platform process supervision and recovery, not real commercial model reliability/quality.
- The timeout **test-only seam** and field-specific/batch assertions can proceed independently of S2. Mandatory generic blocker fields, blind legacy write policy and any new configurable subscription product remain S2/parent decisions.
