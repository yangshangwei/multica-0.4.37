# Research: R20 QA form read failure and bounded holder capture

- Query: Explain the disabled QA approval in R20, identify the first and final real driver failures, and define the smallest diagnostic capable of identifying an exclusive holder.
- Scope: Internal; retained files and static source only. No DB, API, browser, tests, builds, process signals, or Git commands.
- Date: 2026-11-16
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Findings

### Verified result and exact identity

R20's original four cases are **1 PASS / 1 FAIL / 2 NOT_RUN**, retry zero. The complete Web/Desktop role/review/QA first case passed in 58.741 seconds. The second case failed after 19.920 seconds at the unchanged five-second `Record approval` enabled assertion in `e2e/fixtures/collaboration-stage-execution.ts:338`, called by `collaboration-stage-execution.spec.ts:218` while preparing QA approval.

- Actual issue: `01a1e4a1-949e-76c6-8746-1836809835f8`.
- Actual candidate: `01a1e4a1-9738-77c0-ad33-59ae56f424c5`. The earlier screenshot transcription's `e4f1` prefix was incorrect; the raw request identity is `e4a1`.
- Actual QA actor: `2d689ba2-82a4-4e25-b627-c8201e8a97dc`, independently present as the R1 `qa_approver`.
- Report: `2bcdfe28-c9cc-4991-bcf9-0d2292e450ad`, run `536868d8-2f90-42ff-8d65-4e05d0b49fc3`.

The private raw input is `r20-source-denial-panel-r01/evidence/read-diagnostics-r01.jsonl`, 7,320,933 bytes / 3,408 records, SHA256 `0cee235a6e912e47f07876e10751d2c3e042bd8a289a3b41c432558249273144`. It was read at 14:00:32.447354Z, then again after the coordinator reported owned API shutdown at 14:03:14.542891Z; its bytes/hash were unchanged. Last record: 13:52:51.106739Z. All source/error histories remain immutable.

### The form correctly locks on a newer failed R4

| Raw line / trace | UTC time | Real QA observation |
| --- | --- | --- |
| 2858–2859 / 962 | 13:52:39.131295–39.131366 | First recorded QA query failure in the second case: `GetMemberByUserAndWorkspace`, `*errors.errorString`, no captured SQLSTATE; issue GET returns 503. This must not be relabelled as a row-lock error. |
| 2900 / 978 | 13:52:39.872208 | First second-case PgError, for another actor: `LockDeliveryEvidenceIssueRead`, SQLSTATE 55P03. Its current SQL already uses SHARE. |
| 3049–3051 / 1026 | 13:52:42.386502–42.387143 | First recorded QA PgError: `LockArtifactUsers`, 55P03; current candidate GET returns 503. |
| 3387 / 1130 | 13:52:50.396949 | QA R4 returns 200, candidate current, report passed/usable, QA signable with no blockers. |
| 3392–3395 / 1136–1137 | 13:52:50.604851–50.652231 | Issue GET fails generically, then a new issue GET returns 200. |
| 3398 / 1138 | 13:52:50.735582 | R1 returns 200, stage QA / revision 3. |
| 3401 / 1139 | 13:52:50.875091 | QA R4 again returns 200 and is signable. Original review expiry/max QA expiry is **14:07:37Z**, still about fifteen minutes away. |
| 3404–3406 / 1140 | 13:52:51.033983–51.035123 | The new QA R4 fails at `LockArtifactUsers`, backend PID 28428, SQLSTATE 55P03; exact same candidate GET returns `503 {code:"collaboration_unavailable", error:"Collaboration data is unavailable."}`. |
| 3408 / 1141 | 13:52:51.106739 | QA T4 detail returns 200, passed/usable, exact candidate/version/content/artifact/report identity and retained logs. |

The final failing R4 source frames are `generated.(*Queries).LockArtifactUsers` → `service.LockArtifactUsers` (`artifact_authority.go:212`) → `evaluateDeliveryEvidenceInTx` (`delivery_evidence.go:104`) → `evaluateDeliveryEvidenceReadInTx` (`:84`) → `stageArtifactObservationAs` (`collaboration_stage_evidence.go:249`) → `checkStageArtifactCurrent` → `StageCodeEvidence`.

`StageCodeForms` mounts another observer of the same candidate query (`stage-code-forms.tsx:36`); its freshness contract includes refetch-on-mount. It preserves the existing record for draft identity but sets `currentProps.locked = props.locked || query.isError` at lines 45–46. The QA form combines that lock with current signability, exact evidence/report/dependency and expiry checks at lines 128–146. A successful T4 cannot clear the different R4 query's error. The screenshot showing the correct report plus a generic read error therefore matches the retained response order.

No expiry mismatch or malformed report is shown by these records. Removing the R4 error lock, using an old readable record as current authority, extending the five-second assertion, or adding default retries would conceal the actual failure.

### Read-scope audit and the unproved holder

The failing `LockArtifactUsers` SQL is already `SELECT ... ORDER BY id FOR SHARE NOWAIT` (`agent_artifact_execution.sql:63–64`). R4's initial issue, artifact-principal join, approval/execution, signer/member/password/PAT and retained QA issuer/run fences use the previously accepted shared read variants. Current ROOT matches the actual R18 API source for the inspected evidence-read, review-view, signer, QA, execution-approval, delivery-evidence, report-view and issue-handler files; the R20 forms and query source match ROOT too. This is scoped identity evidence, not complete source acceptance.

The R1 stage and collaboration-detail GETs run RR READ ONLY snapshots and use plain getters; no exclusive user lock was found in their inspected chains. T4 uses the same private evidence read transaction. The inspected human-JWT R4/T4 paths contain no newly identified exclusive user fence. Existing code intentionally retains exclusive `LockCollaborationUsers` (`FOR UPDATE`, `collaboration.sql:22–23`) in actual workflow, command, queue and current runtime writers.

Four **separate immutable recovery GETs** also still take that exclusive user query:

- `GetWorkflowOperation`, `collaboration_workflow.go:708`.
- `GetCreationResult`, `collaboration_creation_result.go:150`.
- `GetCommandOperation`, `collaboration_command_operation.go:34`.
- `getCollaborationResourceOperation`, `collaboration_switch_view.go:234`.

They are genuine static read/read contention candidates, but the R20 tracer does not capture their routes or successful locks. They are not inside the inspected R1/R4/T4 chain and are **not proved to be the R20 holder**. Actual runtime progress/message writers use `lockCollaborationRuntimeSource(... current=true)` → `LockCollaborationUsers` (`collaboration_runtime.go:119–134`) for the issue's participants. Their presence does not, by itself, assign this failure to a writer either.

Thus the immediate cause is established: **the latest QA R4 was refused by its shared current-user lock, and the form retained the corresponding read error**. The conflicting holder and its purpose are still unproved. No product or SQL repair should be selected until that distinction is resolved.

## Exact bounded diagnostic recommendation

### Files and existing startup boundary

Use a **new private copy** of the currently bound R18 API source. Keep product sources and ROOT unchanged. Limit instrumentation to:

1. Existing private `server/cmd/server/pay241_read_diagnostics_test.go` (231 lines).
2. Optional new private `server/cmd/server/pay241_read_lock_observer_test.go` for the independent observer and capture-state tests.
3. Only if needed for shutdown, one narrow cleanup call in the existing private harness stop path.

Existing startup integration already exists: `integration_test.go:51` calls `pay241DiagnosticPool`; `collaboration_stage_execution_control_test.go:340` installs `pay241DiagnosticHTTP`; `TestCollaborationStageExecutionHarness:373–403` owns the controller loop. The harness stop closure at line 369 closes the HTTP server, cancels background workers and waits for them. A new observer must close/drain there, before the pool/process exits. Build only the changed router test binary with a compile-only command from the registered frozen source; other binaries and frontend products are reusable only after their exact bindings are checked.

### Tracer: capture the actual successful holder, not just failed readers

- Preserve the native `pgx.QueryTracer` contract; do not wrap Rows/Scan/DBTX, alter SQL results, add SQL reentry on the API connection, or inject credentials.
- Current `TraceQueryStart` ignores contexts without one of its selected GET scopes; current `TraceQueryEnd` retains successful locks only for `LockLifecycleIssue`. That is exactly why the real user holder is missing.
- Propagate **metadata-only** HTTP scope to affected issue operation/current-runtime routes, including writes and recovery GETs. Keep response-body capture restricted to the existing safe R1/R4/T4 GET list; never tee POST/write bodies, credentials, query text, or argument values.
- Independently capture known lock-query start/end even when there is **no HTTP scope**, so current runtime/reconciler/background contexts are visible. Record only canonical query name, SQL hash, actual PID, monotonic sequence/time, per-PID transaction generation, safe verified principal/route metadata where available, native TxStatus at the callback, result/error type/SQLSTATE, and source frames.
- Minimum holder watchlist: `LockCollaborationUsers`, `LockPasswordUser`, named user-profile/admin updates, `LockCollaborationIssue`, `LockLifecycleIssue`, `LockTaskCapabilityPrincipals`, and corresponding shared read locks. Log begin/commit/rollback boundaries for watched transactions, including failed/uncertain terminals and observed SQLSTATE. Do not log arguments or raw SQL.
- A successful user-lock QueryEnd establishes acquisition completed; it does not establish release. Track through that same PID/transaction's commit or rollback, recording both start/end and whether a terminal is actually observed. Transaction generations prevent a recycled pool PID from being confused with its next request.

### Separate observer connection

- Allocate a dedicated task-owned pool from the same private database identity: max 1/min 0, **tracer nil**, dedicated `application_name`, `default_transaction_read_only=on`, short statement timeout (for example 750 ms), and explicit close/drain. Keep API pool sizing/configuration unchanged. No new extension or dependency.
- On an actual 55P03, enqueue a bounded capture request to one observer worker rather than querying recursively from `TraceQueryEnd`. Record event time, enqueue time, sample time and delay, queue drops, capture limits and errors; never claim zero instrumentation overhead.
- Query only metadata from `pg_stat_activity`, `pg_locks` and `pg_blocking_pids`, filtered to this database / relevant PIDs and relation IDs. Safe fields include PID, backend/xact/query timestamps, backend_xid, state, wait-event type/name, lock type/mode/granted/virtualtransaction/transactionid/relation/page/tuple and blocking PID edges. Obtain query name/hash/frames from the in-process tracer registry; **do not select raw `pg_stat_activity.query`**, query arguments, connection strings or secrets.
- If a separate connection is queried synchronously to improve capture latency, classify the execution as diagnostic only and record that added latency. Prefer one buffered worker and a short bounded pre/post-error metadata window while a watched lock-owning transaction is active.

### Proof limits and acceptance gate

NOWAIT failures do not leave the failed reader waiting in `pg_blocking_pids`. An empty blocker list is not proof of no blocker. Row locks often reside in tuple headers rather than one visible granted tuple lock per holder; a relation-level `RowShareLock` does not prove the row lock was exclusive. A post-error sample may arrive after the holder commits. Query/HTTP timestamp overlap alone does not prove the target row was held.

Require a successful **actual exclusive** user-lock acquisition with bound source frames, the same still-active transaction/XID in the observer, and an actual lock/blocking edge or equivalent same-target proof before assigning the R4 failure to that holder. A metadata sample may prove a genuine waiting writer's blocking edge even though the NOWAIT reader has already returned; retain the exact scope/target limitation. If no same-target edge survives capture, report “holder unproved” instead of inferring it from the SQL name or nearby timestamps.

If the bounded capture identifies only a real writer, preserve writer/reader contention and exercise the existing explicit user recovery in a separately reviewed fixture change. If it identifies an immutable GET acquiring the exclusive user lock, first write a real two-connection regression: hold that actual GET after user acquisition, finish an authorized R4 for the same user before releasing it, and separately retain a genuine writer-first 55P03/no-private-projection control. Do not turn that regression into a mock of SQL return values.

Run **only the existing second case once** in a new diagnostic suffix, with the original helper/body, five-second assertion, twenty-second operation budget and 480-second scenario budget. Label it diagnostic, not a new four-case acceptance run. Preserve R20's original `1/1/2` result and every prior failure. After diagnosis, any repair needs RED/GREEN, independent checks, new source/product bindings and a fresh original four-case acceptance.

## Separately selected R21 receipt-read regression

The coordinator selected the independently observable `GetWorkflowOperation` read/read gap as the next bounded regression. This does not assign the R20 holder. Its new private source is `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r21-receipt-read-concurrency-r01/implement/source-r01`, derived from the immutable R18 canonical source. The coordinator owns database admission and all actual RED/GREEN/race execution.

**Scope:** new `server/internal/service/collaboration_workflow_read_concurrency_test.go`; after a recorded RED and coordinator permission, only `GetWorkflowOperation` in existing `collaboration_workflow.go`. The other three exclusive recovery GETs remain outside this change. No product code or test was edited or run by this researcher.

Use existing fixture methods exactly:

```go
f := newStageReviewFixture(t)
stageArtifactReadFixture(t, f)
candidate, approvalRequest, originalReceipt := f.approved(t)
```

The artifact setup is necessary to reach R4's actual current-user SHARE fence. A plain PR-only candidate may bypass that lock and would be an ineffective counterexample. `f.approved` returns the actual review request, including its original `RequestID`, so no receipt row needs to be fabricated.

- **Receipt first:** copy the service and install `stageProjectionBarrierStarter` with needle `GetCollaborationOperation`. Its real `QueryRow` barrier stops after `LockCollaborationUsers` and `authorizeWorkflowAccount` have finished, before receipt lookup (`workflow.go:708–718`). Hold that GET on a five-second context. A second real `StageCodeEvidence` for the **same reviewer**, on a different PostgreSQL PID, must finish while the receipt GET is still held. Compare exact candidate/artifact identity and then the recovered confirmation/code/status against `originalReceipt`.
- **R4 first:** reuse `holdStageArtifactR4(t, f, candidate)` (`evidence_read_concurrency_test.go:78`). While its actual artifact/user authority fences remain held, the same reviewer's actual `GetWorkflowOperation` must finish within five seconds. Release the first reader only after the second has actually finished; releasing it early would allow the old exclusive lock to pass accidentally.
- **Writer control:** hold the original `LockCollaborationUsers` SQL on a real separate transaction and require R4 to refuse with 55P03 and a zero projection. Label this an original writer-fence control. If full writer behavior is needed, hold an actual `DecideStage` return before `GetCollaborationStageCodeCandidate`, after its workflow user fences, then require R4 to refuse and release the writer to obtain its real receipt. Do not mock SQL results.
- **Authority controls:** the wrong actor reading the reviewer's key retains `404 not_found`; disabled or temporarily disabled current account retains `403 forbidden` (`authorizeWorkflowAccount:647–663`). Password mode still requires the matching current password session/version. The original receipt may remain readable after source-member/participant departure; do not introduce a source-membership prerequisite.

The minimum prospective implementation is to use the existing `q.LockArtifactUsers` shared query **only at the immutable GET boundary**, leaving its user-presence check, `authorizeWorkflowAccount`, actor/scope/request lookup, receipt decoder and original result shape intact. Keep all real workflow writer `LockCollaborationUsers` calls unchanged. The new same-layer counterexample, current authority controls, original stage/recovery suites and scoped normal/race/vet checks are the proof gate; no pass is claimed here.

## Files found

- `packages/views/collaboration/stage-code-forms.tsx` — retained R4 identity, independent R4 error lock, exact QA readiness.
- `packages/core/collaboration/stage-queries.ts` — candidate refetch-on-mount and explicit retry-false freshness contract.
- `server/internal/service/artifact_authority.go` and `delivery_evidence.go` — current-user shared fence and authoritative artifact read chain.
- `server/internal/service/collaboration_test_report_view.go` — T4 current source, exact subject and settled-run applicability.
- `server/internal/service/collaboration_workflow.go`, creation-result, command-operation and switch-view — separate recovery GETs using exclusive user fences.
- Private R18 `server/cmd/server/pay241_read_diagnostics_test.go`, integration test and execution-control harness — existing tracer/startup/stop integration.
- Private R20 original four-case result, frozen read JSONL and failure screenshots — exact failure and response chronology.

## External references

No network reference was required. Repository SQL and installed pgx callbacks are the primary contracts. PostgreSQL lock metadata limitations are explicitly retained; no extension was installed or assumed.

## Related specs

- `.trellis/spec/server/collaboration-stage-approvals.md:58–106` — RR projection, read/read compatibility, writer fences and exact error attribution.
- `.trellis/spec/server/database-test-isolation.md` — private, explicitly owned database and exact package admission.
- `.trellis/spec/server/verification-environment.md` — frozen input/build/configuration and final evidence identity.

## Caveats / Not Found

- The current trace proves the final QA 55P03 and HTTP response, but not the exclusive holder or precise browser-consumption state. No default retry or production lock change is justified yet.
- Generic `errors.errorString` and `connLockError` captures omit error strings/status. Their exact causes remain separate and unproved.
- This investigation writes only these research reports; it executed no diagnostic, regression, DB query or build and changed no acceptance ledger.
