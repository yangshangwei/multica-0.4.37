# Research: TC-06 live human edit and runtime reply

- Query: Find the next precise internal stimulus after accepted TC-12, without repeating accepted S2/S3 or taking ownership of the ordinary-comment administrator fix.
- Scope: internal, service-only test supplement; external repository/CI remains separate.
- Date: 2026-11-20
- Main's committed source baseline: `d1b25077b89af8ef72e58c0b040b9ea495261b1b`. Research reads the shared tree; implementation must use Main's frozen candidate, excluding peer WIP.

## Findings

**A bounded test-only increment is supported:** a human edits the already-dispatched typed input comment from V1 to V2 while its live admitted runtime posts a distinct Agent reply to that comment. Exercise both actual commit orders; then bind a report to the original frozen candidate/source and recover original requests. This closes a missing composed internal stimulus, not the whole TC-06 case. Parent remains 9 PASS /11 PARTIAL /4 NOT_RUN and TC-06 remains PARTIAL.

The parent CSV retains concurrent human/machine content writing as an unverified clause. Archived S2 R33 explicitly accepted same-root prepared Git/report/runtime linkage while retaining concurrent business writes and real remote PR/CI (`research/r33-full-s2-a01-a12-independent-check-20261118.md:126`). Archived S3 accepts R01–R09, including output identity and field CAS, without a specific live human-comment-edit/runtime-reply/source-binding race. Accepted TC-12 changes local synthetic Git A→B and retains the original comment; it does not edit that input concurrently. None of those results is withdrawn or rerun.

### Product owners and existing coverage

| File | Contract / existing proof |
| --- | --- |
| `server/internal/service/collaboration_comments_create.go:17` | `CreateComment` atomically creates a human comment, immutable command and typed subject; lines 123–142 bind the actual command and recheck source. |
| `server/internal/service/collaboration_comments_edit.go:19` | `EditComment` owns author-only human editing, explicit comment revision and history; it does not rewrite the dispatched command or subject. |
| `server/internal/service/collaboration_comments.go:174` | `collaborationContentInTx` locks current workflow users, workspace/source, issue/session/channels/targets; lines 263–281 return the exact stored content-free receipt for an original matching request. |
| `server/internal/service/collaboration_squad.go:97` | `lockRuntimeBusiness` verifies the actual run-bound capability, current admission and reservation. |
| `server/internal/service/collaboration_squad.go:232` | `CreateRuntimeComment` and `writeRuntimeComment` insert a separate Agent comment and original runtime event under the admitted run. `ParentID` preserves the human comment link. |
| `server/internal/service/collaboration_runtime.go:119` | `lockCollaborationRuntimeSource` locks the same workflow users and source aggregate used by the human writer, giving a real transaction contention seam. |
| `server/internal/service/collaboration_squad.go:162` | `RuntimeContext.Inputs` decodes the run's frozen `PromptSnapshot`; `Comments` independently reads current discussion. V1 frozen input and V2 current comment are expected to coexist. |
| `server/internal/service/collaboration_test_subject.go:41` | A typed comment binds approved candidate/source facts to its immutable command; the frozen subject is independent of later display text edits. |
| `server/internal/service/collaboration_test_report.go:165` | `RecordRuntimeTestReport` enforces exact frozen subject/run/attempt; original report recovery precedes current admission and survives settlement. |
| `server/internal/service/collaboration_comments_test.go:192` | Existing `TestCollaborationCommentInputsShareOneWaitingBatchAndSurviveEditing` edits/deletes a waiting input and preserves commands. It does not run a live runtime reply or source-bound report. |
| `server/internal/service/collaboration_comments_test.go:300` | Existing `TestCollaborationCommentEditsSerializeAndUnknownCommitDoesNotRepeat` covers two human edits and unknown commit. |
| `server/internal/service/collaboration_comment_edit_delete_test.go:19` | Existing edit/delete both-order barrier keeps history, replies and frozen AI input; both competing actions are human content mutations. |
| `server/internal/service/collaboration_runtime_output_concurrency_test.go:57` | Existing O04 races two Agent output fingerprints, not human edit against runtime reply. |
| `server/internal/service/collaboration_test_report_boundaries_test.go:79` | Existing report convergence/supersession/recovery preserves original candidate, but does not exercise a concurrently edited source comment. |

### Peer task boundary

`.trellis/tasks/11-20-comment-admin-human-authority/prd.md` owns the known ordinary `handler/comment.go` machine-owner/admin exception, real credential matrix, edit/delete/attachment branches and possible product fix. It remains a separate planning task. Do not change or accept that behavior here. The proposed slice uses dedicated collaboration human `EditComment` and runtime `CreateRuntimeComment`; it is not a claim that all ordinary comments reject machine edits or that all Agent state writes are prohibited.

### Smallest implementation and finite denominator

Proposed new file only: `server/internal/service/collaboration_concurrent_content_test.go`.

Proposed name: `TestCollaborationHumanEditRuntimeReplyBothCommitOrders`, one top with `human_edit_first` and `runtime_reply_first` leaves. Per mode: one top, two leaves, three named results; normal and race use separate new service databases; Main preregisters a 10-minute suite bound in `docs/plans/2026-11-20-pay-241-concurrent-content-proof.md`. This is one bounded TC-06 supplement, not six business cases. No attachments, local Git execution, browser, router, real Provider or broad denial matrix is needed.

1. Reuse `newStageRuntimeFixture(t, true)` (`collaboration_test_report_test.go:27`). It provides an approved synthetic candidate in QA, typed comment input and managed preparation. Call `f.input(t)` and retain its exact create request/body and returned comment/command. Use actual `CreateComment`, issue `resumeManaged`, `ListRuntimeRuns`, `claimPreparedCandidate`, `permitPreparedClaim`, and `RuntimeTestSubject`. Do not seed a run or fabricate a runtime identity. The Git/CI/preparation/consent/settlement remain clearly labelled fixtures; `refreshPreparationConsent` uses the retired test-only reconfiguration helper.
2. Capture the live run's full original command, group input, subject, candidate, prompt snapshot, attempt and initial run events, original create operation and original human comment event. The human input content is unique V1; candidate ref/source revision must match the selected candidate and claim. Build an author `EditComment` at revision1 to unique V2. Build a runtime reply with a fresh UUID, `ParentID` equal to the human comment and no machine-target mentions, using the actual issued `permit.BusinessToken`.
3. Use `runtimeSourceCommitStarter` wrapping `recipientCommitStarter` / `recipientCommitTx` (`collaboration_runtime_scan_test.go:25`, `collaboration_recipient_eligibility_test.go:307–337`) on a copied service for the leading writer; the wrapper selects the actual source-lock transaction rather than a preliminary transaction. It signals actual backend PID before commit while all production locks remain held. Start the follower on the original service and call `waitFIFOBlocked(t, f.collaborationQueueFixture, blocker, done)` (`collaboration_fifo_test.go:164`); this queries `pg_blocking_pids`, so the test must observe actual blocking before releasing. Use the existing 15-second writer context and 5-second blocked-writer helper; no sleep, enlarged timeout or fake response. Run each order in an independent fixture and join both goroutines before any failure/exit path.
4. Both writers should succeed exactly once. The human comment is V2/revision2 with the same human author; Agent reply is separate revision1 with admitted Agent identity and original parent. Verify actual two event IDs/sequence order follows the held commit order. Do not require a CAS conflict: these are intentionally distinct comment writes. No new command/run/delegation is expected from an unmentioned runtime reply.
5. Compare complete unchanged pre-existing command/group-input/test-subject/candidate/run/attempt and original operation/event rows after the race. Compare timeline/runtime event *sets* as immutable prior rows plus the explicitly expected additions, not equality of the entire set across writes. Session `event_seq` advances; do not normalize it away in a purported whole-row invariant. Explicitly verify expected sequence delta/order and preserve all other session fields. Human V1→V2 history keeps exact actor/time/request; current `RuntimeContext.Comments` sees V2 and reply while frozen `Inputs` still contains V1. `RuntimeTestSubject` is byte-for-byte identical to the original subject.
6. While the run is still live, exact-replay original human create request, original edit body/key and runtime reply UUID/body; require their original receipts and zero mutations to the complete post-race business collections. The human create receipt retains revision1 even though a separate current comment read is revision2. Check `GetCommentOperation` for both human originals. A changed original human payload must get exact `idempotency_conflict` with no new operation/effect; a changed runtime reply body at the same UUID must get `comment_conflict` with no effect. These are narrow original-key controls, not a replay of the accepted authorization matrix.
7. Submit one synthetic report through `RecordRuntimeTestReport`, using the original typed subject and actual claim identity. Settle through the existing `finishSquadRun` fixture if the final scope needs a retained completed report. Verify public report/run/command/source join and retained human/Agent comments and their original events. Exact report POST and receipt GET recover the original report ACK after settlement without duplication; original human operation recovery remains exact. Compare the complete post-settlement business snapshot before/after recovery. Do not demand a runtime comment POST succeed after settlement: its current-authority check precedes original comment recovery.

The implementation owner should keep per-row snapshots local to the new test, reusing existing snapshot patterns rather than changing production helpers. Suggested complete collection inventory: issue, collaboration, session, target/channel/batch/group, command, group inputs/dependencies, run, attempt, termination, business capability metadata (no raw credentials), frozen test subject, stage candidate/decisions, operations, timeline events, run events, comments, notifications, attachments/references (empty), delegation and ordinary dispatch queue. Across the race use exact original-row preservation plus explicit allowed additions; across pure replay require complete equality. Preserve snapshots privately before inherited fixture cleanup and report final SQL leftovers separately.

### Receipt and source distinctions that must remain explicit

- Human create/edit replay returns saved `CollaborationCommentReceipt`, not current content. `GetComment`/history independently proves current V2 and retained V1.
- Runtime reply recovery uses the original comment UUID/body and currently valid `mcb_`; `CreateRuntimeComment` checks live authority first. Do its success replay while live. The original runtime event remains immutable after settlement even though fresh POST is no longer admitted.
- Report ACK recovery has a different contract: originally issued capability and original report identity are recovered before current admission. It can be checked after settlement.
- Current discussion V2 does not amend frozen command V1 or select a different candidate. No code revision is introduced, and this is not another TC-12 A→B repair.
- A caller-supplied runtime UUID equal to an existing human comment would currently hit a plain INSERT uniqueness failure, not necessarily `comment_conflict`; that unrelated collision/error-mapping path is outside this minimum stimulus.

## Related specs

- `.trellis/spec/server/collaboration-source-editing.md`: human comment revisions, frozen AI inputs, original receipts, real commit/PID contention proof.
- `.trellis/spec/server/collaboration-runtime-output.md`: run-owned Agent identities and current-authority replay.
- `.trellis/spec/server/collaboration-stage-approvals.md`: typed candidate/report source binding and honest fixture boundaries.
- `.trellis/spec/server/collaboration-content.md`: content/attachment and history ownership.
- `.trellis/spec/server/database-test-isolation.md`, `.trellis/spec/server/verification-environment.md`: Main-owned source/config/normal-race databases, safe compile-only and retained raw evidence.

## External references

None needed or fetched. This uses existing Go/PostgreSQL owners and repository test helpers, with no new dependency or SDK.

## Caveats / Not Found

Research ran no tests, database/API operations, services, model accounts or Git operations and changed no product/spec/parent ledger. No pass is claimed yet. Main owns plan, execution permit, independent check, parent record and Git. The original context is sufficient; no historical conversation search was needed.

This supplement establishes service transaction/content/source correspondence with synthetic managed runtime and PR/CI observations. It does not establish HTTP/router/password login, UI/Web/Desktop, actual daemon checkout, remote branch/PR/CI, real model analysis or payment/business repair, deployment, performance or signing. TC-06 and the parent remain open even if this increment passes. Ordinary-comment administrator security behavior is explicitly owned by its separate task.
