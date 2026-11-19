# R21 immutable receipt / R4 regression boundary check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Verdict

**PASS for the frozen RED test boundary, source delta and isolated profiles.** No blocking issue was found in the new214-line test. The leader may bind the final reviewed inputs and execute one original RED command. No Go test, build or database request was executed by this checker, and no product repair has been accepted by this pre-execution review.

The independent pure-GET compatibility gap is a valid behavior to test. Its result must not be described as proof of the actual R20 lock holder, which remains unproven. Any `GetWorkflowOperation` lock repair still requires the original RED observation and a separately bounded implementation/review.

## Meaningful regression and controls

The two coexistence tests use `newStageReviewFixture`, the real `stageArtifactReadFixture` packager/canonical spec and `f.approved(t)` before starting concurrency. They positively verify that the candidate has a currently readable artifact baseline. They use the same real reviewer, actual `GetWorkflowOperation` and actual R4 `StageCodeEvidence` on distinct PostgreSQL PIDs.

- Receipt-first holds `GetCollaborationOperation` after the real user lock and `authorizeWorkflowAccount`; the second R4 must complete while that immutable receipt read is still held. The returned artifact candidate and the released receipt must retain their exact identities.
- R4-first reuses `holdStageArtifactR4`, holding its acquired shared source/artifact fences while the second immutable receipt read completes. It verifies the original status/code/revision/confirmation and the released R4 baseline.
- Both second-request contexts are created before the held-reader context. Each keeps the original five-second budget, preventing a blocked operation from appearing successful merely because the first holder expired earlier.

The original writer control holds a real `DecideStage(return)` transaction at `GetCollaborationStageCodeCandidate`, after its original workflow user locks. The independent R4 must return PostgreSQL55P03 with a zero projection; after release the writer must commit its exact `returned` confirmation. This control does not replace the writer with a read or remove its existing fences.

The current-authority controls require a wrong actor to receive404/not_found and a real disabled-account row to receive403/forbidden, both with zero receipt. Wrong-actor refusal is followed by successful original-actor recovery. Existing GET scope/account/decode/source-independence behavior and every product/writer file remain unchanged in the RED source.

The barrier forwards real SQL and arguments. It only changes scheduling before the selected real query; no Rows result, account status, permissions or SQL outcome is mocked. Buffered result channels, deferred channel close, `sync.Once` release and registered release/cancel/drain cleanup cover early failures and explicit result consumption. No unbounded wait or test-goroutine assertion was introduced.

## Exact expected inventory and source

The static inventory is4 top-level /6 named /5 leaf tests:

1. `TestCollaborationWorkflowReceiptReadThenStageEvidence`
2. `TestCollaborationWorkflowStageEvidenceThenReceiptRead`
3. `TestCollaborationWorkflowReceiptReadOriginalWriterFence`
4. `TestCollaborationWorkflowReceiptReadCurrentAuthority`, with `wrong_actor` and `disabled_account` children

The expected original RED is two coexistence leaf failures plus three passing controls, with no skip. This is an expectation, not an executed result. Fixture/setup failure, missing names, a different failure or incomplete cleanup cannot be accepted as that RED witness.

The frozen manifest adds only `server/internal/service/collaboration_workflow_read_concurrency_test.go`:214 lines, SHA256 `9048a661…`. All5,044 original manifest byte/mode identities are unchanged; the new manifest has5,045 files and no missing path. The reviewed draft binds that manifest, the exact anchored selector, six expected names, the fresh red profile and one command with `-count=1 -timeout=5m`.

## Profile and recorder admission

All three red/green/race profiles are distinct and record zero other database connections. Their853-entry ledger (through865),663 index definitions/validity fields and211 table counts match the donor before and after cloning. Declared environment and clone-audit hashes match. All private environment files are0600; HOME and GOTMPDIR use distinct trusted Data directories, and TMPDIR/TMP/TEMP match GOTMPDIR. Recorded ancestor owner/mode/inode chains match the filesystem.

The final reviewed runner is `dd112881…`. It guards the selected task, one-command budget, owned binding/profile/output paths, exact service package, argument count, explicit selector and five-minute budget; it checks frozen source paths/hashes/modes before and after and takes the serial PAY flock. The actual Go child receives only the validated private environment.

The initial recorder gap was reported to the leader and mechanically corrected before RED: current bytes validate the actual service `TestEvent.Package`, closed action set, named identity type, duplicate run/terminal and one package terminal. Build output/failure is classified separately, including a valid empty string `ImportPath`; malformed/unknown events and build failures remain incomplete evidence. This checker did not modify or execute the runner.

The initial six-reference draft omitted the runner's imported `database/db_audit_r01.py` (`8b7f53d0…`). The leader added its exact reference before RED; the final reviewed seven-reference draft matches every current input hash. The helper's current bytes and recorded audit contracts were independently reviewed. No test/source/profile change accompanied this mechanical admission completion. The leader should include this accepted review when freezing the final one-command RED binding.

## Handoff and limits

Execute one RED observation on the frozen unchanged product with the exact4/6/5 inventory. Preserve its raw stream, source/database before and after, and every failure. Only a complete original RED witness justifies a new implementation scope; current planning permits consideration of the single immutable-receipt GET user fence while keeping account authorization, scope/decode and all writers unchanged.

No R20 runtime cause, four-case acceptance increment, S2 acceptance or parent status is claimed. Only this report and its JSON companion were written. No lint, type-check, test, API/DB/browser, build, signal, install or Git operation was run by the checker.
