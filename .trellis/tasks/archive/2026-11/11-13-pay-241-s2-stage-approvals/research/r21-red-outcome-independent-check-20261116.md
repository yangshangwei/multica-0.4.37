# R21 original RED outcome independent check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Verdict

**The original RED observation is complete, but the required RED gate is NOT ACCEPTED.** Actual inventory is4 top-level /6 named /5 leaf tests, zero skip, with three failed leaves and two passing authority controls. The expected RED required only the two coexistence failures and a passing original-writer control. No product lock repair or complete regression result is accepted by this review.

The original result, raw stream, source before/after and database records are retained. The two intended pure-GET failures are real observations and remain useful evidence. They do not establish the actual R20 lock holder.

## Observed outcomes

| Original test | Retained result |
| --- | --- |
| Receipt GET held, then same-actor R4 | FAIL at line81: PostgreSQL55P03, could not obtain lock on row in relation `user` |
| R4 held, then same-actor receipt GET | FAIL at line120: context deadline exceeded during the original five-second read budget |
| Original `DecideStage(return)` writer control | Unexpected FAIL at line160: writer completed before the selected barrier, with logged error `<nil>` |
| Wrong actor receipt read | PASS |
| Disabled account receipt read | PASS |

The two intended tests reached their real transaction/barrier and distinct-PID checks before their failure assertions. The first reader's final receipt and the reverse held R4's final projection are consumed before failure, but their later success assertions are not reached; this review does not retroactively count those unexecuted assertions.

The actual service package terminal is FAIL and command exit is1. `R21_TARGETED_OBSERVATION_COMPLETE` describes complete raw/inventory and cleanup recording, rather than a successful expected RED gate.

## Original writer-control gap

The original test selects `GetCollaborationStageCodeCandidate` as its writer barrier at line147. Actual `DecideStage` calls `t.codeCandidate` at `collaboration_stage_review.go:243`; that helper calls **`GetCollaborationStageCodeHead`** at line31. The selected query therefore never fires on this path. The control exits at its unexpected-result branch before starting R4 or asserting55P03/zero projection and the final returned confirmation.

The raw only logs `<nil>` for that branch; it does not print the returned receipt. The source mismatch explains the missing barrier, but no exact original receipt is reconstructed from the subsequently cleaned database. A nil error cannot be used to claim that the writer control passed.

The earlier static boundary review missed this concrete query-name mismatch. Its original PASS report remains preserved; this actual RED report records the newly exposed limitation. Future barrier reviews should follow the precise production helper to its actual query name and require the writer control to execute its full refusal/commit assertions.

The proposed test-only correction is to select `GetCollaborationStageCodeHead` and include the unexpected receipt in the failure message. Keep both coexistence assertions, original five-second budgets, writer55P03/zero-projection/returned assertions and authority controls. Freeze a new copy and use a fresh red profile for the full4/6/5 original-product RED observation. The corrected copy is a separate pending witness, not a replacement for this failed control or a GREEN product repair.

## Source, raw and cleanup reconciliation

The raw run/terminal sets exactly match the six expected names and the recorded terminal values. The unexpected failed-name set contains only `TestCollaborationWorkflowReceiptReadOriginalWriterFence`. Original argv/cwd matches the consumed binding, and source-before/source-after both record5,045 files with zero drift; the original214-line test still has SHA256 `9048a661…`.

The recorded database ledger and index definitions remain unchanged and after other connections are zero. Required fixture tables are empty. The whole database is not empty: retained configuration invalidations increase21→27 and knowledge invalidations18→33, while schema/singleton rows remain. Those retained counts are not erased or relabelled as a cleanup failure.

## Recorder provenance

The original consumed binding remains unchanged at `f8f83cb2…`. Its recorder reference points to `run-service-check-r01.py` with hash `dd112881…`. That path now contains a separate r02-profile adaptation and therefore does not match the old reference when rehashed in place.

The exact original consumed bytes were preserved before adaptation in `evidence/recorder-before-r02-profile-allowlist.py`; its hash independently matches `dd112881…`. The explicit recorder-provenance receipt binds the original consumed binding, original path/hash, retained copy and updated recorder separately. This review accepts that retained-byte provenance and preserves the original unresolved current-path reference; it does not rewrite the old binding or treat the new recorder as the old input.

## Frozen r02 test-only correction boundary — PASS

The new immutable `implement/source-control-r02` copy is independently checked. Its test SHA256 is `0dfec7c3…`, source manifest `c65bff32…`, inventory `48c96427…` and exact two-line patch `dcbd60a6…`.

The only changes are the writer needle to the actual `GetCollaborationStageCodeHead` and printing the unexpected receipt/error. Both pure-GET failure assertions, writer55P03/zero projection/final `returned` assertions, authority controls, five-second contexts and cleanup remain byte-for-byte unchanged. The exact4/6/5 inventory is unchanged. All other5,044 source manifest byte/mode identities remain equal to the retained original source; no product file changes.

The three fresh `profiles-r02` allocations (`ae7aa457…`) record zero other connections and no started Go commands. Their853-entry ledger,663 indexes and211 table counts match the retained donor contract. Declared environment and clone-audit hashes match. This review does not reuse the earlier bad-control database as the new RED witness.

The reviewed r02 draft (`27e027ff…`) has eight matching input references, the corrected source/server cwd, unchanged anchored selector/six names/five-minute command budget, and the explicit fresh red_r21r02 database. The new immutable `run-service-check-r02.py` (`ba39d217…`) keeps the source/profile/serial-run/event-classifier guards and separately binds the imported database auditor. The original runner/binding/result remain retained separately.

**The frozen test-only r02 boundary and its single fresh original-product RED input are accepted.** The leader may add this review to the final r02 command binding and execute that original RED once. Its actual outcome remains unobserved here, and the original r01 RED gate remains unaccepted. No product repair is accepted or authorized by this boundary supplement.

## Handoff and limits

No product change is authorized by this outcome check. Retain both observed pure-read failures and the invalid original writer control. Independently freeze/check the minimal test-only supplement, execute a fresh complete original-product RED with the expected controls, and only then consider the bounded immutable-receipt user-fence repair.

The original R20 runtime cause, four-case acceptance, S2 and parent status remain open. This checker wrote only this report and its JSON companion, and ran no Go, test, API/DB/browser, build, signal, install or Git operation.
