# Research: r16 artifact-bound candidate read failure

- Query: Trace the exact reviewer R4 candidate read that preceded the r15 disabled code-review Reason field, and identify the smallest evidence collection needed before a fix.
- Scope: Internal source and retained evidence only. No API, database, browser, model, product, test or configuration mutation.
- Date: 2026-11-16
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

### Retained facts and source identity

The original r15 report has four cases: one expected, one unexpected and two skipped. Its failing result is `locator.fill` timing out after the original 20,000 ms because the code-review Reason textarea is not enabled. The selector resolved the textarea; this is not a missing selector or target ambiguity. The failed fill did not send the code-review decision POST.

The retained independent review verifies two artifact executions with creation HTTP201, human approval HTTP200, execution HTTP200 and actual 2,048-byte tar outputs. It also records reviewer source-admin promotion, scope confirmation, candidate creation and advancement to code review as HTTP200. The asserted reviewer GET200 followed by GET503 comes from the prior ROOT's captured request facts; that review explicitly says the original evidence does **not** contain the reviewer GET response body. There is no collected original SQLSTATE to attribute yet.

The runtime server source is:

`/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/server-current-r11-c54-r01/source`

The runtime frontend source is:

`/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/frontend-contract-r10-r01/current-overlay-r11-r02/source`

Twelve implicated server/SQL files and the two implicated frontend files were compared directly with current ROOT. All 14 have equal bytes. Server modes also match; the two private frontend files have read-only snapshot modes rather than current ROOT's writable modes. This is a scoped comparison, not a claim that the complete current ROOT equals either frozen graph. Exact hashes, modes and reference checks are in the companion JSON.

### Exact HTTP and transaction path

| Layer | Current ROOT location | Behavior |
| --- | --- | --- |
| GET adapter | `server/internal/handler/collaboration_stage_review.go:282` | `GetCollaborationStageCodeEvidence` resolves issue and candidate UUIDs, calls the service and writes its response. |
| R4 service | `server/internal/service/collaboration_stage_review_view.go:132` | `StageCodeEvidence` reads readiness, stage state, exact candidate/head/content, artifact authority, code/CI, retained reports/decisions, execution facts and signable steps. |
| Read transaction | `server/internal/service/collaboration_stage_evidence_read.go:15` | Starts one `REPEATABLE READ` transaction, resolves current issue access, executes the complete projection and commits. It deliberately permits permission row locks and does not declare `READ ONLY`. |
| Error envelope | `server/internal/handler/collaboration.go:91` | `CollaborationError` preserves its domain status/code; every other error becomes HTTP503 `collaboration_unavailable`. This function does not log the underlying error. |

The callback fails immediately on any error before returning a successful projection. A failed observer does not produce a valid partial R4. The transaction is rolled back on failure.

### Artifact-bearing versus successful no-artifact path

At `collaboration_stage_review_view.go:159`, a source-readable candidate with a non-null baseline artifact calls `checkStageArtifactCurrent` before normal candidate/CI assembly. A no-artifact candidate bypasses this branch.

The artifact branch is:

1. `checkStageArtifactCurrent`, `collaboration_stage_evidence.go:250`.
2. `stageArtifactObservation`, `collaboration_stage_evidence.go:228`; its `As` helper constructs the exact delivery observation request at line232.
3. `EvaluateDeliveryEvidenceInTx`, `delivery_evidence.go:79`.
4. `lockDeliveryArtifactInTx`, `delivery_evidence.go:220`, after the observer loads the execution.

This observes the selected child's current revision/project/PR and verifies the original execution, canonical spec, producer task generation, runtime, initiator, claimant and approver authority. Artifact errors and missing rows are converted to an unavailable-evidence result by `stageArtifactUnavailable`, `collaboration_stage_review.go:54`; arbitrary database errors are not.

Later branches can acquire further locks for artifact-bound reports and original approval issuers:

- Retained reports: `collaboration_stage_review_view.go:190` through `testReportSourceRead`, then `stageArtifactSourceReadAs`, `collaboration_stage_evidence_read.go:64`.
- Retained approved decisions: `stageReviewState`, `collaboration_stage_review_view.go:17`, and `lockStageFinalSigners`, `collaboration_stage_signer.go:212`.
- Execution-source readiness/targets: `stageExecutionEvidence`, `collaboration_stage_execution_view.go:39`.

### Lock seams relevant to a concurrent-read hypothesis

| Observer seam | SQL location | Lock |
| --- | --- | --- |
| Selected child | `server/pkg/db/queries/lifecycle.sql:5` | `LockLifecycleIssue`: `FOR NO KEY UPDATE NOWAIT`. |
| Child project | `server/pkg/db/queries/project.sql:92` | `LockProjectForAssociationNowait`: `FOR SHARE NOWAIT`. |
| Current PR/link set | `server/internal/service/delivery_evidence.go:118` | Four PR/link tables in `SHARE MODE NOWAIT`. |
| Task, agent, runtime and producer principals | `server/pkg/db/queries/task_token.sql:16` | `LockTaskCapabilityPrincipals`: `FOR SHARE ... NOWAIT`. |
| User/member authority | `server/pkg/db/queries/agent_artifact_execution.sql:63` | `LockArtifactUsers` / `LockArtifactMembers`: `FOR SHARE NOWAIT`. |
| Original artifact approval | `server/pkg/db/queries/agent_approval.sql:64` | `LockAgentApprovalRequest`: `FOR UPDATE NOWAIT`. |
| Artifact execution | `server/pkg/db/queries/agent_artifact_execution.sql:36` | `LockAgentArtifactExecutionByApproval`: `FOR UPDATE NOWAIT`. |

Two otherwise legitimate artifact-bound R4 reads for the same child/approval/execution can therefore contend on exclusive `NOWAIT` locks. The current handler would hide a raw `pgconn.PgError` such as SQLSTATE55P03 behind `collaboration_unavailable`. This is a source-supported **hypothesis**, not attribution of the original r15 failure.

Read-only evidence must continue to fence member role/account changes. Changing the generic read transaction to `READ ONLY`, dropping all permission locks, substituting an administrator principal or using cached positive evidence would violate the existing contract.

### Why the Reason field stays locked

`packages/views/collaboration/stage-code-forms.tsx:45` combines parent `locked` with candidate-query `isError`. The fieldset passes that lock to the Reason textarea at lines153/184. Candidate/evidence selection and signability separately control submission readiness at lines141–146/188.

`packages/views/collaboration/stage-panel.tsx:94` locks the parent for pending operation, unconfirmed intent, storage error, stage-query error or unavailable stage read. Therefore a fresh read error correctly disables editing. Missing candidate selection does not explain the disabled Reason field. Keep this fail-closed rule while diagnosing the service.

### Evidence-supported hypotheses, in order

1. **Current artifact-reader contention.** An exclusive child/approval/execution observer lock is held by another legitimate candidate read or task lifecycle transaction. Confirm using the exact underlying error/query; a 55P03 at these seams supports this hypothesis.
2. **Repeatable-read invalidation after a current-authority mutation.** If the actual error is40001, identify the changed row and its snapshot/lock order. The documented revocation-before-authority-lock case requires discarding the projection; do not relabel that expected authority failure as an arbitrary product defect.
3. **Another non-domain observer/retained-data failure.** Decode errors, runtime lookup errors, adapter errors or invalid retained data also reach the generic503 envelope. Only the first actual error identifies the correct branch.
4. **Named readiness failure.** Actual response codes such as `stage_schema_unavailable`, `test_report_unavailable` or `stage_execution_unavailable` narrow the branch immediately. Do not conflate these with `collaboration_unavailable` or infer that the already-successful packager is disabled.

### Smallest useful diagnosis and regression shape

The final handoff is now in `research/new-session-handoff-20261116-r15.md`. The coordinator reports that the original random reviewer password and session state were not retained, so authenticating that old reviewer is not currently possible within the original credential contract. Do not reset that password, fabricate a principal or substitute an administrator.

The coordinator owns restoration of the already-bound stopped runtime with its database/history retained. Its stated next slice is one diagnostic execution of the original E02 with fresh actors established through the same real account/role flow. Passively capture the diagnostic reviewer's candidate GET and adjacent issue/stage GET as complete sanitized response bodies with timestamp, status, current user/workspace/issue/candidate identities and stage/epoch. Add a test-only pgx QueryTracer that records the first failing query and structured error. This remains separate from the four canonical-case result and adds no canonical-case PASS.

If that isolated candidate read succeeds, compare overlapping ordinary P/R or same-candidate reads, without a new approval or history mutation. A private, passive error seam should capture the **first** service/transaction error with request identity, operation name and structured SQLSTATE/query—not just the final error envelope. The generic adapter's service-return point and the read callback/commit split are the narrow capture sites; the observer seams above locate deeper errors after their actual type is known.

The existing `stageArtifactReportFixture` and `stageSourceFenceStarter` provide a deterministic two-reader reproduction seam without sleeps. The existing R4 branch in `TestStageArtifactReportAuthorityLocksAcrossReadAndQA` intercepts `GetCollaborationAttempt` after the artifact authority locks. In that one-shot callback, call a second shallow-copied service whose `TxStarter` is the raw fixture pool, reading the same exact candidate as a second legitimate initiator/manager. This avoids recursive wrapper interception; the first transaction still holds its observer locks. Record callback hit count1, the second read's actual structured error and the first read's successful projection. A captured55P03 here establishes a separate deterministic read-read defect, but it does not retroactively assign that cause to r15 without its actual diagnostic response/error.

If read-read contention is confirmed, preserve the failing HTTP/driver evidence and add that same-layer regression. Keep initiator/manager positives, ordinary-member/demoted-manager denial, revocation-first snapshot rejection, original-signer expiry and the full artifact identity checks. The narrow repair candidate is a read-observer lock policy with compatible shared locks for the selected child/approval/execution while retaining permission fences. Do not change the shared `LockLifecycleIssue` SQL used by real writers, or remove all M1 locks. Which observer seams require that policy follows from the actual failing query and regression, rather than a speculative global lock rewrite. Do not change fieldset locking, action deadlines, authorization or add blanket retries to make the UI pass.

### Existing patterns and related specs

- `.trellis/spec/server/collaboration-stage-approvals.md:48`: frozen artifact read authority, RR permission locks and whole-projection rejection on invalidated authority snapshots.
- `.trellis/spec/server/controlled-artifact-execution.md`: confined packager/receipt/exact execution identity.
- `.trellis/tasks/11-13-pay-241-s2-stage-approvals/prd.md`: A08 current authority and original-key recovery; A10 privacy; A11 actual two-client journeys; A12 final independent check.
- `TestStageArtifactReportReadAuthorityMatchesR4`, `collaboration_test_report_artifact_test.go:125`: canonical positive/negative artifact authority matrix.
- `TestStageArtifactReportAuthorityLocksAcrossReadAndQA`, same file:231: current permission lock fence.
- `TestStageArtifactReportRevocationBeforeAuthorityLockDiscardsSnapshot`, same file:279: current RR snapshot rejection pattern.
- `TestStageHTTPCodeReviewContractAndProtectedEvidence`, `collaboration_stage_review_test.go:13`: actual handler envelope and protected evidence.

No external SDK/API documentation is needed for this slice; it traces the repository's existing pgx/PostgreSQL behavior. A PostgreSQL lock-mode reference would be relevant when choosing a confirmed contention repair, after the actual SQLSTATE is captured.

## Caveats / Not Found

- The original reviewer503 body and original first SQL/driver error are absent from the retained evidence reviewed here. Neither55P03 nor40001 is established as the r15 cause.
- No API, SQL, browser, test suite or diagnostic probe was executed. There is no new canonical-case result or runtime/source-integrity verdict.
- Original r15 remains1PASS/1FAIL/2NOT_RUN; A06, A07, A09, A11, A12 and full S2 remain open.
- The 14-file byte comparison is limited to the implicated source paths. Private frontend snapshot mode differences were recorded, not classified as source drift.
- No product, tests, spec, task metadata, parent matrix, configuration or retained evidence was changed by this research slice.
