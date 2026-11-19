# Research: R28 submission-preview read repair design

- Query: Smallest three-file, explicit pure-submission-preview repair after genuine R28 read/read RED.
- Scope: internal; source/spec inspection only. Write ownership is this new Markdown/JSON pair.
- Date: 2026-11-17 (Asia/Shanghai).
- Source: `/Volumes/artisan/.multica-acceptance/pay241-r28-preview-read-20261117-sz4vj59y/source-red-r01`.
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

### Gate and exact scope

Main reports accepted R27R02 binding/UTC baseline, normal/race each8 named PASS, ROOT integration/format/vet/compile PASS. R28 adds the separately owned five-top/13-leaf/18-named test. **No product edit until main retains genuine R28 RED and binds the actual failed query/transaction identities.** This is a design proposal, not executed RED, a patch or permission to repair an unobserved historical holder.

Proposed product ownership remains exactly `server/internal/service/{collaboration_completion_preview.go,delivery_completion_collaboration.go,collaboration_stage_completion.go}`. No new type, public boolean, context flag, read token, HTTP field, transaction owner, SQL/generated file, migration or dependency. Ordinary/self/admin paths, attachment locks, writer transactions and accepted binding/UTC behavior remain on their existing dispatches.

### Minimal structural changes in the three files

**`collaboration_completion_preview.go`, `GetCollaborationCompletionPreviewInTx:178`.** After the existing request-kind/UUID checks, current root/source/credential checks and issue/status-version comparison, select read dispatches only for `request.Target.Kind == "collaboration_submission"`. Construct the existing **value** `collaborationStageEvidenceReadTx{Tx: actualBorrowedTx}` once in this branch. Use that same value for the selected calls and their `db.New` queries. Other kinds keep the original actual transaction and original helpers. Four call sites change dispatch, without duplicating the preview body:

| Existing call | Submission-only dispatch proposal |
| --- | --- |
| `completionCollaborationIntent` at`:208` | Proposed private `readCompletionCollaborationIntent` |
| `lockDeliveryArtifactInTx` at`:228` | Existing `readDeliveryArtifactInTx` |
| `stageCompletionFacts` at`:231` | Proposed private `readStageCompletionFacts` |
| `EvaluateDeliveryEvidenceInTx` at`:265` | Existing `evaluateDeliveryEvidenceReadInTx` |

Keep `lockCompletionPreviewRoot:63` unchanged: its only callers are candidate GET and preview; R26 already retains shared root/source issue and root-collaboration fences. Keep chosen source vs canonical stage child distinct: final `stageCompletionMatchesM1` still checks their exact artifact/approval/head binding. Temporary guard `boundSubmission`, original submission/operation/handoff checks, final credentials, UTC/microsecond expiry and response/error semantics are unchanged.

**`delivery_completion_collaboration.go`, `completionCollaborationIntent:198`.** Add the private read entry with the same existing arguments/return shape; constrain it to submission. Factor the existing intent body into one small private mode helper, proposed `completionCollaborationIntentInTx(...,readProjection bool)`. Original `completionCollaborationIntent` passes **constant false**; new read entry passes true. The mode affects only the normal consumer access dispatch at`:205`: existing `readCompletionCollaborationAccess` replaces existing `completionCollaborationAccess`. Preserve every current creator/handler/capability/epoch/status/catalog/current-submission predicate. `completionIntentDigest:284` and its nonempty attachment `LockCollaborationAttachments:318` stay unchanged. No implicit marker inference in the original writer entry.

**`collaboration_stage_completion.go`.** Add a private `readStageCompletionFacts` entry, selecting a private common facts-body mode with nil final-check and read=true. Original `stageCompletionFacts` and `stageCompletionFactsWithFinalCheck:123–127` retain their signatures and select false. Add a private state-body mode, proposed `stageCompletionStateInTx(...,readProjection bool)`; original `stageCompletionState:83` passes **constant false**, even when explicitly given a marked transaction. Only the read facts branch selects true. This is factoring of the existing two bodies and two lock dispatches, not a second validator:

| Lock point | Read=true | Original/default=false |
| --- | --- | --- |
| Stage task via `loadStageDetails:115` | Actual SHARE fence below, then existing getter/details | Existing `loadStageDetails(...,true)` → UPDATE |
| Canonical code child at`:160` | Existing `LockDeliveryEvidenceIssueRead` | Existing `LockLifecycleIssue` |

The child query keeps **the original** `content.Baseline.ChildIssueID` and `access.Issue.WorkspaceID`, then repeats unchanged parent/project checks. Preserve all remaining state/facts logic: policy/readiness/864 ledger, virtual head, code/scope/current CI, release/rollback dependencies, actual execution source/fact, retained issuers and final clock/callback order. Do not change generic `loadStageDetails`, actual Prepare/Finish consumers, or Finish's transaction identity/single-use checks.

### Exact task SHARE fence, with absent-head semantics

There is **no existing generated/product `LockCollaborationStageTaskShared` symbol**. The approved R28 test's `r28StageStateFence` already contains the canonical id-only literal:

```sql
SELECT issue_id FROM collaboration_stage_task WHERE issue_id=$1 FOR SHARE NOWAIT
```

Use that same fence inside the allowed Go read-state branch, with the same transaction and real root ID, before `loadStageDetails(...,false)`. The latter calls **existing `GetCollaborationStageTask`** (`collaboration_stage.go:238–249`, `queries/collaboration_stage.sql:42`) and fills the same role/scope facts. No `.sql` or generated-code change is needed. An Exec row count may be0/1, or QueryRow `pgx.ErrNoRows` may be accepted as an absent head; other errors remain fatal. Never convert missing head to404, synthesize ready facts or call `EnsureCollaborationStageTask`.

Existing `loadStageTask` maps the getter's `pgx.ErrNoRows` to `stageTaskDefaults(issue),taskExists=false`: development/stage revision1, no role/scope, followed by the original nonauthorizing stage blocker. The unchanged root collaboration SHARE fence also remains held; actual stage head creation acquires its original workflow root UPDATE before `ensureStageTask:360–372`. Thus this is not a `lock=false` gap or an unlocked absent-head race. Keep id-only predicates and existing workspace checks; do not introduce joins to imitate another subsystem.

### Concrete marker must survive the whole nested chain

`tx.(collaborationStageEvidenceReadTx)` is an **exact value-type assertion** (`collaboration_stage_evidence_read.go:92`, `stage_execution_approvals.go:94,105`, `stage_evidence.go:248`). A pointer, custom outer wrapper or passing `.Tx` misses it. Wrap the actual SQL-forwarding instrumentation **inside** the value marker. Never call `readCollaborationStageEvidence`, which Begins/sets repeatable-read; preview retains its original borrowed/default-isolation transaction,250ms preparation and five-second deadline.

| Nested path receiving the same marked transaction | Existing selected behavior, no helper change |
| --- | --- |
| `stageCompletionFacts` body → `stageExecutionApprovalChain:201` → QA issuer at`stage_execution_approvals.go:93–108` | `readCompletionCollaborationAccess` and `stageQAReportReadAs` |
| QA report → `testReportSourceReadAs` → `testReportFrozenSourceReadAs` → `stageArtifactSourceReadAs:71–111` | `readDeliveryArtifactInTx` for exact original frozen artifact |
| `stageQAReportReadAs` → `stageQAReportInTx:63–75` | Exact run SHARE, require one row, repeat original scoped recorded-run getter |
| Release issuer reobservation at`stage_completion.go:254` → `stageArtifactObservationAs` | Existing read observer with all original metadata/current authority |
| Final `checkStageFinalAuthorityWithFinalCheck` → `lockStageFinalSigners:267` → original-source artifact checks | Same marked source-read dispatch for every retained approver, not the completer's credentials |

Keep `sources.Targets`/integration checks, final password/PAT/member fences and database-clock rechecks in this same transaction. Inherited `Begin` is unmarked; none of these current read chains should Begin. Mode creation belongs only to the validated submission preview; existing R4/T4 mode creation remains unchanged. Main can record the additional private read-entry contract in specs after accepted GREEN; researcher does not edit specs or the marker declaration.

### Existing callers and writer-default proof map

| Helper/entry | Actual writer/default callers retained | Required evidence/control |
| --- | --- | --- |
| `completionCollaborationIntent` | `completionFreezeCollaboration:355`, `completionRecheckCollaborationIntent:401`; self/admin preview | Source audit constant false; original exact M1 writer/binding tests |
| `completionCollaborationAccess` | Consumer/recovery/freezing/finished checks in `delivery_completion_collaboration.go:188,352,441`; authorization`:362,535`; stage binding`:57,150` | Existing false wrapper`:41`; `TestStageExecutionReadWriterAccessFences` tests plain/marked root and session |
| `stageCompletionState` | `prepareStageMutation:368`, Finish missing-guard check`:450`, original facts | R28 `stage_state_plain` and `stage_state_read_marker`: UPDATE refuses a held real SHARE row, then recovers after release |
| `stageCompletionFacts` / `WithFinalCheck` | `prepareStageMutation:382`, Finish final callbacks`:518`; self/admin preview | False dispatch audit; existing real `TestStageCompletionAllM1ModesAndActualWriters`, final binding/clock controls |
| `EvaluateDeliveryEvidenceInTx`, `lockDeliveryArtifactInTx` | Authorization creation/recovery/consumption, ordinary preview, unmarked stage writes | Existing constant false wrappers `delivery_evidence.go:80,235`; R28 actual observer both orders and retained artifact writer controls |
| `stageQAReportAs` | QA and unmarked standing-chain writes | Existing constant false wrapper `stage_qa.go:20`; `TestStageExecutionRunReadFences` plain/marked and actual run writers |
| Marker-sensitive artifact/source/QA-chain helpers | Existing R4/T4 reads are marked; actual writers create no new marker and retain upfront issue/task/child fences | Preserve type checks unchanged; `TestStageExecutionReadRetainedIssuerFences` and R28 revoked QA/viewer controls |

There is no existing direct plain/marked intent mode test; do not claim the access test directly tests intent. The constant-false wrapper and unchanged writer caller map are source-audit obligations. Public/direct observer/access/QA writers keep their false mode even on an explicitly marked transaction; original stage-state read-mode inference is prohibited.

### Test helper and final binding to genuine RED

The accepted224-line `completionPreviewSubmissionFixture` registers noWrites before returning (`managed_submission_test.go:136`). Keep it unchanged. The new `r28NewPreviewFixture`/`r28PreviewTarget` should build only the genuine verified/submitted fixture and exact target; intentional account/password/member/stage/submission negative changes occur **before** `r28NoWrites`. Then register transaction/barrier cleanups so LIFO release/cancel/drain/rollback precedes the final snapshots. The new adapter/admin snapshots remain projections of real fixture state, not new admin/ready facts.

Retain approved selector `^TestCollaborationCompletionPreviewManagedSubmission(R4Coexistence|SiblingReads|WriterFences|CurrentAuthority|BindingRefusals)$` and all13 leaves. Main binds actual RED firstPg query/hash/UUID/backend/phase to these proposed locks; an earlier refusal does not prove every later fence ran. After a scoped patch, same test bytes need fresh normal/race, existing affected writer/default controls and independent review. Do not change source permissions, roles, epochs, timeouts, isolation, pool settings or assertions to obtain GREEN.

Expected protected gates: real second read terminal while first remains held, distinct nonzero PIDs, exact stable preview/R4 identities, zero durable authorization/status/event/queue/attachment/source effects, current authority/stale/binding refusals and real writer contention. Preserve raw RED and all failure identities. R27's8 PASS is retained binding evidence, not R28 GREEN or full preview/S2 acceptance.

## Files found / specs / external references

Owning files and line-backed patterns are listed above. Reused sources: `delivery_evidence.go`, `collaboration_stage_evidence_read.go`, `collaboration_stage_execution_approvals.go`, `collaboration_stage_qa.go`, `collaboration_stage_signer.go`, `collaboration_stage.go`, the approved new read-concurrency test and accepted managed-submission test. Existing SQL/getter/default shapes are `queries/{collaboration_stage.sql,delivery_evidence_read.sql}`. Companion JSON binds inspected source hashes and call/control maps; the coder's evolving test snapshot is not a frozen manifest.

Read task PRD/design/implement, `CLAUDE.md`, `.trellis/workflow.md`, server stage/completion/test-isolation/verification specs. No implement/check JSONL, external API/docs/account, SQL, test, build, browser, Git or process action was used. Repository versions are Go1.26.6/pgx-v5 v5.9.2; no new dependency is proposed.

## Caveats / Not Found

- This researcher executed no R28 tests/SQL and changed no product/test source. Proposed new helper names are design names, not existing symbols or repaired code.
- Nonempty submitted attachments still reach original UPDATE in `completionIntentDigest`; only a separate real RED can widen that scope. Ordinary/self/admin/R38 remain unchanged and unproved by this13-leaf read slice.
- Full original four cases, A01–A12/S2, parent24 and S3–S5 remain open. Historical R24/R25 holders remain unproved; accepted metadata snapshots are not a new tar-byte audit.
