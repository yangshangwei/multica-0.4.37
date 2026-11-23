# Research: Managed-submission M1 preview regression

- Query: Smallest real regression for the managed submission preview, followed by its pure-read/R4 coexistence boundary.
- Scope: internal; source/spec inspection only, with ownership limited to this Markdown/JSON pair.
- Date: 2026-11-17 (Asia/Shanghai).
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

### Initial admitted slice: one new test file, one top-level test, five leaves

Initial RED ownership: **only** new `server/internal/service/collaboration_completion_preview_managed_submission_test.go` in main's `pay241-r27-preview-20261117-x12xjirm/source-red-r01`. Keep accepted R26 product/test bytes and all existing fixtures unchanged.

| Exact top-level identity | Exact leaf | Expected contract |
| --- | --- | --- |
| `TestCollaborationCompletionPreviewManagedSubmission` | `enforce_authorization_available` | Genuine verified submission gives `authorization_available`, exact binding and zero preview writes |
| same | `policy_off_non_authorizing` | Same genuine stage/submission under fixture M1 off gives `policy_off`, no observation/binding/expiry, all permission flagsfalse |
| same | `stale_target` | Change only the request's `ExpectedIssueRevision` to a different valid revision; actual preview refuses409 `delivery_completion_conflict`, no durable effects |
| same | `wrong_actor` | Actual outside-source handler still has root participation but refuses403 `delivery_completion_denied`; reviewer is an authorized manager and is unsuitable for this negative |
| same | `expired_credential` | Actual creator JWT with past expiry and retained matching password-session refuses403 `delivery_completion_denied` |

These names match main's frozen actual AST: **1 top-level /5 leaf /6 named identities**. Do not count wrapper events as additional business cases. Negative leaves do not need a passing-preview prerequisite; preserve all five terminals even when enforce/off previews are RED. Main reports original RED running with compile exit0 (permit prefix `9e5c`, source prefix `f1aa77`); this researcher has not executed or accepted that run.

### Exact existing fixture and request construction

1. `f := newStageCompletionFixture(t)` (`collaboration_stage_completion_test.go:48`). It calls `newStageExecutionFixture` (`collaboration_stage_execution_fixture_test.go:180`), which uses actual managed registration/consent, code approval, runtime claim/test report, settlement, QA/release decisions, authenticated controlled execution and `AdvanceStage(...verification...)`. No direct ready-stage row is inserted by the proposed test.
2. `stageCompletionM1Policy(t,f,mode)` (`collaboration_stage_completion_tx_test.go:419`) is the existing fixture-only policy setup, with one fresh fixture per leaf and enforce except the explicit off leaf. Do not call `stageCompletionM1Grant`: the preview baseline needs **zero delivery authorizations/terminals**. Default artifact/product roles already suffice; independent-role fixtures are for separate outside-consumer/R38 controls.
3. `submission := stageCompletionSubmit(t,f)` (`collaboration_stage_completion_tx_test.go:74`) calls real `StatusOptions` and **real `SubmitAcceptance(f.stage.handlerCtx,...)`**, requiring its actual `DeliveryConfirmation.SubmissionID`.
4. Read the resulting current root/access and exact `GetCollaborationStageSubmission`/original operation. Require root `in_review`, current submission equals the returned ID, matching root/workspace/candidate/lifecycle/policy/role/stage/scope/release/fact bindings, and the genuine original `submit_acceptance/acceptance_submitted` receipt. Record setup failure separately; it is not preview RED.
5. Reuse `stageCompletionAcceptParams(t,f,submission)` (`:89`), fresh `ResolveIssueAccess(f.ctx,nil,root)`, and `completionCollaborationTargetFromWorkflow` (`delivery_completion_collaboration.go:493`) as `stageCompletionM1ForLifecycle` (`completion_tx_test.go:475`) already does. That helper constructs a value; it does not execute a replacement workflow. Never use cached `f.issue.Issue` revisions after submission.
6. Request is `CollaborationCompletionPreviewRequest{SourceIssueID:f.stage.child, ApprovalID:util.UUIDToString(f.stage.artifact.ApprovalID), Target:target}`. Target kind must be `collaboration_submission`, with exact submission ID, root issue revision/status, decimal collaboration/lifecycle/participant/account epochs, current `StatusOptionsVersion` and `AcceptanceNote`.
7. Reader is **`f.ctx`**, the current creator/source owner/artifact initiator, using **`completionRequestCredential(ctx)`**. `org := pgtype.UUID{}`. The managed fixture creates actual password credential rows and JWT/password-session contexts: `newStageRuntimeFixture:30–51`, `newManagedCollaborationFixture:57–80`; finite JWT expiry comes from `stageCredential`. Do not replace this with a standalone legacy JWT, administrator, daemon context or a freshly invented issuer snapshot.

### Actual service seam and positive assertions

Use `beginCompletionCandidatesRead(t,f.stage,f.ctx)` (`collaboration_completion_candidates_read_concurrency_test.go:158`) unchanged. Its real `pool.Begin` has default isolation, a five-second actor context, SQL-forwarding `completionCandidatesTraceTx`, and bounded rollback cleanup. Invoke:

```go
f.svc.GetCollaborationCompletionPreviewInTx(read.ctx, read.tx,
    f.issue.Issue.ID, pgtype.UUID{}, completionRequestCredential(read.ctx), request)
```

Then call `read.finalCredential()` and real `read.tx.Commit(read.ctx)`; the service owns neither Begin nor Commit. This matches `handler/collaboration_completion_preview.go:87–109`. `PrepareArtifactTransaction` keeps250ms lock timeout/readiness (`artifact_authority.go:72`). Do not pass an R4 marker from test code to make the original preview pass.

Require protocol1; exact root/source issue IDs and numeric revisions; current `in_review` category and selected active done target; actual project/revision and enforced-policy revision; nonzero database `CheckedAt`; `Reason == "authorization_available"`; nonnil pass observation with exact approval/execution/source/spec/artifact/receipt/generation identity; nonnil canonical binding with **the original target**, child evidence and human creator consumer; empty nonnil blockers and `StageBlockersTruncated == false`. `Enforced`, `UsableForCompletion`, `SkipSupported` remain false.

Require finite `MaxExpiresAt > CheckedAt`, bounded by current artifact expiry, caller credential expiry, every applicable standing decision/issuer/evidence horizon and24-hour cap. Later reads may have later clocks/observer timestamps/derived24-hour horizon; compare stable identities plus actual horizon constraints, not fabricated identical times.

Closed request/response shapes are `delivery_completion_types.go:61–195`, `delivery_completion_preview.go:49–78`, and `collaboration_completion_preview.go:18–50`; submission targets have no ordinary `target_position`/self/admin fields. Service refusals may return an internal partially filled DTO; the real handler ignores that DTO and serializes only its defined error. Do not invent a universal all-zero internal-response contract for late refusals.

Reuse `completionCandidatesSnapshot`, `completionCandidatesCompareState` and `completionCandidatesNoWrites` (`candidates_read_concurrency_test.go:247–308`) with `f.stage`: they cover complete rows/counts for72 relations plus the existing stage-exit snapshot. Register cleanup after intentional negative setup, before transaction/barrier handles. Before/after preview equality covers grants/terminals/evaluations, operations/events/notifications, root/child statuses and epochs, queues/runs/reservations, stage submission/decisions/facts, attachment references, canonical archive metadata and gate state. Also require original stage-execution source `calls` and target unchanged.

### Why the positive should be RED on original source

`collaboration_completion_preview.go:240–247` constructs `stageMutationGuard{tx,before,access,evidence}` without `boundSubmission`, then calls `checkSubmission(nonzeroID)`. `collaboration_stage_completion.go:401–404` immediately compares the requested ID with that unset field. The response becomes200-style preview `Reason:"stage_incomplete"`, blocker `verification/stage_submission_unbound`, nil binding/expiry; this is a **static defect prediction**, not an executed result.

The real writer initializes `boundSubmission` before the same check (`collaboration_stage_completion.go:386–391`). The preview must check the existing immutable row/original operation; it must never call `bindSubmission`, `prepareBinding`, create a grant, or create a pending completion operation.

**After genuine RED**, initial product repair proposal is **only `collaboration_completion_preview.go`**, initializing the temporary read guard's bound ID from the already validated exact target before the unchanged row/operation/handoff checker (`collaboration_stage_completion_binding.go:100–130`). This generic initializer covers both submission and admin-current-submission without changing any access, nomination or lock helper. Ordinary and self-without-submission miss this predicate. Add the single genuine admin binding supplement below; a full R38 lock/authority matrix is unnecessary unless its helpers change.

Retain unchanged control sources `TestStageCompletionSubmissionRequiresItsOriginalOperation` (`completion_tx_test.go:152`) and `TestStageCompletionImmutableSubmissionLocksInOwningTransaction` (`:232`); they cover wrong original-operation refusal and immutable row fences. Add the subsequent preview-specific `missing_stage_binding` negative before claiming that removal/bypass of the preview checker would be caught. Current-authority candidate/R4 and router controls remain accepted only under exact source/evidence bindings. Initializer GREEN does not prove a full preview lock repair.

### Minimal admin-current-submission binding supplement

Propose **`TestCollaborationCompletionPreviewAdminCurrentSubmission/admin_available`**, separately frozen/admitted:1 top-level/1 leaf/2 named, without altering the running initial five-leaf source. If later combined in one frozen source, the two tests are2 top-level/6 leaves/8 named; this is a prospective inventory, not observed execution.

1. `f := newStageCompletionFixture(t, stageCompletionIndependentArtifact, stageCompletionIndependentProductRole)` (`completion_tx_test.go:658,672`, same order). These existing setup hooks make artifact owner/original product scope signer the current reviewer before genuine code/QA/release/verification, so disabling the creator cannot invalidate those retained issuers. Keep all real role/scope service calls; no direct ready facts.
2. `stageCompletionM1Policy(t,f,"enforce")`; `submission := stageCompletionSubmit(t,f)`; `admin := stageCompletionAdmin(t,f)` (`:689`). Add actual source membership with `f.fx.Member(t,f.workspace,util.UUIDToString(admin.admin),"admin")`, matching the existing router admin-read fixture (`completion_preview_integration_test.go:42,559`). Super-admin status alone supplies no artifact visibility; do not insert an admin issue participant.
3. `stageCompletionDisableCreator(t,f,admin)` (`:721`) invokes real `PlatformAdminService.ChangeAccount` with the original password/version. Then read fresh `p := admin.params(t,"complete")` (`admin_collaboration_test.go:97`); it gets current issue/status/catalog/collaboration revision. Never reuse pre-disable creator access or cached root revision.
4. Build the exact value in a preparation-only borrowed transaction: real `PrepareArtifactTransaction`, current `GetIssue`, **`completionAdminNomination(ctx,tx,issue,admin.admin,admin.org,true)`**, then **`collaborationDeliverySummary(ctx,db.New(tx),access)`**. Require nonnil actual `Current`, its ID equals the genuine submitted ID, and evidence available; the nomination holds actual collaboration/session/event-sequence authority. Pass that real `summary.Current` to `completionAdminTarget(&collaborationWorkflowTx{collaborationQueueTx:&collaborationQueueTx{access:access}},p,summary.Current)` (`delivery_completion_collaboration.go:543`). This is value construction, not a replacement writer. Final-check the actual admin credential and commit preparation.
5. On a new default-isolation borrowed transaction, call the actual preview with `root=f.issue.Issue.ID`, `org=admin.org`, credential **`completionRequestCredential(admin.adminCtx)`**, source `f.stage.child`, selected `f.stage.artifact.ApprovalID`, and that target. Target kind is `collaboration_admin`; it includes the genuine nonnil submission, current numeric issue revision/status, decimal collaboration/lifecycle revisions, status-options version and administration reason, with no normal participant/account epochs.
6. Require `authorization_available`, pass observation, empty blockers, exact admin target/submission/source evidence, bounded expiry and `Consumer{Kind:"admin",UserID:admin.admin,OrganizationID:admin.org}` with all permission flagsfalse. Compare72-relation snapshot/source counters plus **`adminCollaborationStateSnapshot(t,admin)`** (`admin_collaboration_authority_test.go:134`, including root admin operation/audit rows). Take baseline after actual disable/source-member setup. No M1 grant, new binding, admin completion operation, accepted decision, attachment change or lifecycle write; never call `stageCompletionM1Grant` or `admin.actions.ChangeLifecycle` in this regression.

For this initializer-only delta, retain exact existing **`TestCollaborationStageCompletionPreviewRouterR38Scope/workspace_organization_link_mismatch`** (`:569`,404 no private body) and **`TestCollaborationStageCompletionPreviewRouterCurrentPasswordAndOrganization/password_session_changed/completion-preview`** (`:771`,403 denial). They are not rerun here. Only if organization/password/access/nomination helpers actually change, separately admit new `wrong_organization` and `stale_password_session` preview leaves using the same genuine setup: keep the real admin identity while changing the exact organization argument/association, or increment the real password version through canonical CAS while retaining the original credential. Do not fabricate a second internal tenant or renew the context to bypass revocation. Preserve original admin advisory and row locks; no full R38 lock matrix is needed for the generic initializer.

### Subsequent pure-read slice: do not bundle into the initial RED

The binding-only positive can expose all additional locks in one unheld transaction, but successful unheld preview cannot prove coexistence. After independently accepted binding GREEN, freeze a new read/read RED baseline and separate inventory:

| Proposed top-level identity | Leaves |
| --- | --- |
| `TestCollaborationCompletionPreviewManagedSubmissionR4Coexistence` | `r4_first`, `preview_first` |
| `TestCollaborationCompletionPreviewManagedSubmissionSiblingReads` | `same_creator` |
| `TestCollaborationCompletionPreviewManagedSubmissionWriterFences` | `writer_first`, `reader_first`, `stage_state_plain`, `stage_state_read_marker` |
| `TestCollaborationCompletionPreviewManagedSubmissionCurrentAuthority` | `disabled_account`, `changed_password`, `revoked_artifact_manager`, `revoked_qa_issuer` |
| `TestCollaborationCompletionPreviewManagedSubmissionBindingRefusals` | `missing_stage_binding`, `stale_stage` |

Subsequent proposal is5 top-level/13 leaves/18 named, separately admitted; initial inventory stays unchanged and supplies wrong-source/expiry/request-staleness controls. A sibling is necessary because R4 reads the stage task without its exclusive preview task fence; R4/R4 proof alone misses preview/preview task contention.

For `r4_first`, reuse **`holdStandingExecutionR4(t,f)`** (`collaboration_stage_execution_read_concurrency_test.go:60`) on this already verified fixture. It does not create an unknown intent; it calls actual R4 as the real release approver and holds at `ListStageExecutionIntents` after QA issuer collaboration/session and report/artifact locks. Its access probe asserts those real branches ran. Avoid the earlier `holdStageArtifactR4` barrier before reports/standing approvals when claiming complete nested-lock coverage. Require second preview/final credential/Commit terminal on a distinct nonzero PID **before release**; then release/drain held R4 and compare its exact baseline.

For `preview_first`, complete actual preview and final credential check but hold its real transaction uncommitted. Call actual `StageCodeEvidence` on a second connection with `stageProjectionBarrierStarter{began:...}`; require success/actual terminal before first commit. Sibling similarly holds a completed first preview while the second actually returns/commits. Keep R4 repeatable-read isolation and preview default isolation unchanged, all five-second contexts and250ms artifact lock budget unchanged, and clean up release/cancel/drain/rollback before final snapshots.

`writer_first` uses unchanged `stageProjectionWriterRequest` and actual `EvaluateDeliveryEvidenceInTx` until a real pass, then holds that transaction; preview must return409 with captured55P03 and no serialized observation/binding, recovering only after release. `reader_first` holds full successful preview and invokes that actual observer on another backend; original `LockLifecycleIssue` must refuse before `LockArtifactUsers`. For `stage_state_plain/read_marker`, hold only the real stage-task SHARE fence on one connection, then invoke the original stage facts/state writer on another, both plain and explicitly marked: its UPDATE task fence must still refuse. Call real `PrepareArtifactTransaction` to retain its250ms budget; never inject a query result. Reuse existing plain/marked completion-access and QA-run writer controls instead of duplicating their matrices.

Current controls first establish a genuine available preview as the respective creator or reviewer. Change the fresh fixture's real disabled/password/member row through existing canonical writes, then take a new no-write baseline and read with the original credential/request. Viewer demotion must deny the selected artifact; retained QA manager demotion must produce the defined no-binding stage blocker even while the creator remains authorized. Use `stageDemoteArtifactManager(t,f.stageRuntimeFixture,user)` unchanged (its existing50ms belongs to that revoker, not the preview); changed-password setup can use `ChangePasswordCredential` from `queries/password.sql:11`, retaining its version CAS and real session-version increment.

Exact sequential authority expectations: creator disabled/password changed can fail initial `ResolveIssueAccess` with404 `not_found` (`collaboration.go:103–124`); selected-artifact reviewer demotion is403 `delivery_completion_denied`; retained QA demotion is a no-binding stage blocker from the original chain (`stage_evidence_unavailable` for loss of artifact read). Keep these distinctions and verify the reached source; do not force every current-authority refusal to403/409.

`missing_stage_binding` deletes only the exact freshly created stage-submission row after genuine SubmitAcceptance; require `stage_incomplete/stage_submission_unbound` without a binding/expiry. `stale_stage` uses actual `stageCompletionRework` (`completion_tx_test.go:731`, real DecideStage return + AdvanceStage development), rebuilds fresh target observations for the same retained submission, and requires `stage_incomplete/stage_not_reached`. No direct stage/epoch rewrite or synthetic passing facts. Deliberate negative setup is outside per-read equality; original sealed histories are untouched.

### Source lock inventory remaining after R26

| Source/call | Lock / required disposition for later read-only repair |
| --- | --- |
| `collaboration_completion_preview.go:97–112` root issue/root collaboration/source issue | R26 accepted SHARE NOWAIT; retain exact root/child/project/current access checks |
| `delivery_completion_collaboration.go:198–206` intent → `completionCollaborationAccess:57–69` | Root collaboration/session UPDATE NOWAIT; pure preview needs explicit existing read-access dispatch; original writer wrapper stays false |
| same `completionIntentDigest:316–319` | Selected attachments UPDATE, only when nonempty; no attachment in initial fixture, so attachment claim needs a separate real upload/submission slice |
| `collaboration_completion_preview.go:228`; `delivery_evidence.go:234–327` | Selected artifact's task-principal child NO KEY UPDATE, approval/execution UPDATE NOWAIT; use existing read artifact helper only for preview |
| `collaboration_stage_completion.go:115`, `collaboration_stage.go:238–245` | Stage task UPDATE; preview needs an actual shared task fence with same missing-head semantics, not bare `lock=false` |
| `collaboration_stage_completion.go:160` | Candidate child NO KEY UPDATE NOWAIT; explicit pure read variant needed |
| `stageExecutionApprovalChain:93–108` / `stageQAReportAs` | Retained QA issuer collaboration/session UPDATE and exact report run UPDATE; existing private R4 marker/read dispatches already cover this chain if preserved through same physical transaction |
| `stageArtifactObservationAs:248–251`; preview `:265`; `lockStageFinalSigners:267` → original-source read | Default observer/source checks repeat exclusive child/principal/approval/execution fences for retained issuers; existing read dispatch must follow the exact same transaction through every branch |
| Stage submission/code head/operation/handoff; project/policy/schema ledger; members/users/credentials; PR/link/check/integration tables | Already SHARE/KEY SHARE or scoped immutable reads; keep original predicates/current-signers/final clock/readiness and no-write behavior |
| R38 `completionAdminNomination:115–171` | Exclusive advisory key70856394211471801 and root/session UPDATE plus real admin/password/org checks; separate admin slice, keep original serialization until that slice has its own RED |

Later core preview product proposal after genuine read/read RED: `collaboration_completion_preview.go`, `delivery_completion_collaboration.go` for a private read intent/access dispatch, and `collaboration_stage_completion.go` for explicit read state/facts. Keep new dispatch/marker creation confined to **collaboration_submission** for this slice; dispatching all preview kinds also changes self/admin and requires those separate slices. Reuse shared artifact/observer/access/report helpers; preserve actual writer wrappers/defaults even when explicitly given a marked transaction. A task SHARE fence plus existing same-transaction getter can preserve absent-head behavior; no migration/dependency is needed. Do not change SQL/generated files or generic `loadStageDetails` defaults without separate ownership.

Ordinary `GetDeliveryCompletionPreviewInTx` still calls the default observer (`delivery_completion_preview.go:334`): if changed, it needs its own ordinary preview/observer two-order regression using the existing ordinary fixture. Self uses `prepareDeliveryMode(...false)` and attachment validation; if its helper changes, add genuine Transfer/self-result and upload/bind controls. R38 needs `stageCompletionAdmin`, `stageCompletionDisableCreator`, independent artifact/product signers and real password/org negatives; core submission concurrency does not close it.

### Commands and evidence gate

Main's admitted runner supplies the exact private source/profile/database; do not execute commands directly from ambient configuration. Initial selector is `^TestCollaborationCompletionPreviewManagedSubmission$`; argv is `go test -json -count=1 -timeout=10m ./internal/service -run '<selector>'`. Same-source fresh GREEN uses identical test bytes/selector; race adds `-race` on its separately allocated fresh clone. Preserve all named terminals and the first actual SQL/Scan/Rows.Err/Exec/Commit PgError through the existing forwarding trace; blocker RED without a PgError is valid and must not be relabeled lock RED.

Initial retained writer selector, only if exact prior bindings cannot be reused: `^(TestStageCompletionSubmissionRequiresItsOriginalOperation|TestStageCompletionImmutableSubmissionLocksInOwningTransaction)$`. Subsequent selector: `^TestCollaborationCompletionPreviewManagedSubmission(R4Coexistence|SiblingReads|WriterFences|CurrentAuthority|BindingRefusals)$`. Changed shared read dispatches require existing `TestStageExecutionReadWriterAccessFences`, `TestStageExecutionRunReadFences`, `TestStageArtifactReadProjectionWriterFences`, `TestStageExecutionReadRetainedIssuerFences`; retain `TestStageCompletionAllM1ModesAndActualWriters` for actual completion/no-binding controls by exact source/evidence. Do not add repeated totals.

## Files found / related specs / external references

Primary owning sources and reused helpers are cited above under `server/internal/service/`; actual adapter is `server/internal/handler/collaboration_completion_preview.go`. Existing router `server/cmd/server/collaboration_stage_completion_preview_integration_test.go:33–35` explicitly says its real-auth fixture does **not** prove stage-enabled completion. Its unconfigured-stage negative and stage-disabled positive miss this verified branch.

Read `CLAUDE.md`, `.trellis/workflow.md`, server specs `index.md`, `collaboration-stage-approvals.md`, `delivery-completion-enforcement.md`, `database-test-isolation.md`, `verification-environment.md`, and task PRD/design/implement, sealed R26 GREEN review and R25 preview/read plan. Role-isolated implement/check JSONL was not loaded. No external docs/APIs/accounts; `server/go.mod` pins Go1.26.6 and pgx/v5 v5.9.2, and all claims trace to owning repository source.

## Caveats / Not Found

- This report is static research, **zero researcher-executed RED/GREEN/tests/build/SQL/API/browser/process/Git operations**. R26 acceptance and main's currently running initial RED are separately retained/reported evidence, not newly accepted preview proof; main already integrated and compiled/vetted R26 ROOT.
- R24/R25 original holders remain **UNPROVED**; R25 remains1PASS/1FAIL/2NOT_RUN. Full preview, original four cases, A01–A12, S2, parent24 and S3–S5 acceptance remain open.
- The artifact fixture packages real bytes but does not expose its archive path. Complete metadata equality is not a fresh tar-byte audit; use a separately owned fixture-output exposure or existing router `archive()` for any such claim.
