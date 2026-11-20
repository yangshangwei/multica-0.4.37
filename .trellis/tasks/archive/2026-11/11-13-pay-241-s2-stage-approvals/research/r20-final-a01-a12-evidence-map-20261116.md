# PAY-241 R20 final A01–A12 evidence map

**Full S2 remains open.** This is a read-only map of retained actual Go observations, current source/reader controls and the latest original four-case result. It is not final acceptance, a new test execution or a parent24-case PASS promotion.

## Current result and relevant source

- Latest original R20 four: **1 PASS / 1 FAIL / 2 NOT_RUN**, workers1/retries0/maxFailures1. Original result SHA256 `f424ea63461dec5b13a50e932b072a8fd679786c79386835d26ec7913386f388`; stdout/stderr/reporter hashes independently match.
- E01 passed in **58.741 s**, including source-denial Web/Desktop behavior, human decisions, actual prepared CLI T1/T2/T3 and retained QA failure/retest history. This supports its composed subset, not every A01–A12 requirement.
- E02 failed in QA at the original five-second `Record approval` enabled assertion, `e2e/fixtures/collaboration-stage-execution.ts:338`; E03/E04 were not run. Main owns the new failure diagnosis; this map does not assign a speculative SQL/cache cause.
- R09 joint service normal/race each retain **116 tops / 321 named / 263 leaves**. Their four raw Go streams were rehashed and terminal sets reparsed: all expected names PASS once, zero FAIL/SKIP/missing/extra/duplicates.
- Handler normal/race each retain **99/326/275** and router each **54/208/174**. The four result/raw hashes and actual terminal name sets were rechecked, all PASS. These are retained observations; the reviewer executed zero Go commands.
- Nine selected current R18 test/handler/router sources exactly match the accepted R09 bytes, including all canonical completion positive fixtures. R17's changed R4/T4 artifact branch has accepted GREEN and scoped race12/22/18. R18's changed standing-QA/run reader branch has accepted GREEN/race5/15/11, preserved public/plain/marked writer exclusion and current ROOT production1,458 dependency binding. These supplements are scoped changes, not a freshly rerun full116/99/54 or whole ROOT equivalence.

## A01–A12 map

Every listed retained test name is present in the actual reparsed PASS terminal inventory. Product results below are the current R20 result, not a union of earlier failed rounds.

| A | Requirement | Representative actual retained PASS evidence | Current product proof / final gate |
| --- | --- | --- | --- |
| A01 | Roles, stable business owner/current handler and authority | `TestStageRolesRequireSourceAdminParticipantsAndSeparation`<br>`TestStagePolicyRevisionCASAuthorityAndOriginalKey`<br>`TestStageHTTPActionsConfirmationsRefusalsAndRecovery` | E01 PASS; full A remains open |
| A02 | Immutable proposals, decisions, single materialization/no dispatch | `TestStageProposalVersionsDecisionsAndSingleMaterialization`<br>`TestStageProposalConcurrentAcceptCreatesOneChildSet`<br>`TestStageProposalAcceptRollsBackEveryChildOnRefusal`<br>`TestStageChildCreationRequiresApprovedProposalForMachines` | E01 PASS; full A remains open |
| A03 | Exact PR/head/trust/strict CI refusal | `TestStageCodeReviewStrictCIRefusals`<br>`TestStageCodeReviewDigestAndSourceDrift`<br>`TestStageCodeVCSWithoutRepositoryCheckIdentityIsUnsupported` | E01 PASS; full A remains open |
| A04 | Human reviewer, self/machine/revocation exclusion | `TestStageCodeCandidateVersionsAndReviewerIsolation`<br>`TestStageCodeReviewSourceMembershipRejoinDoesNotReviveApproval`<br>`TestStageCodeReviewSignerRevocationAndExpiryBounds`<br>`TestStageAdvanceRechecksOriginalApprovalAtFinalClock` | E01 PASS; full A remains open |
| A05 | Immutable actual run/case/log, QA return/retest history | `TestStageTestReportActualInputClaimAndReceipt`<br>`TestStageQAPassedReportNeedsSuccessfulSettledExecution`<br>`TestStageTestReportHostCheckpointExactLogAndRedaction`<br>`TestStageTestReportLostCommitRecoversOneOriginalEvent` | E01 PASS, E02 FAIL; full A remains open |
| A06 | QA/build/release authority and transitive issuer validity | `TestStageExecutionReleaseRollbackAndOriginalReceipt`<br>`TestStageExecutionSigningStateDriftPreventsDispatch`<br>`TestStageExecutionR4ReflectsTransitiveArtifactAuthority` | E02 FAIL; full A remains open |
| A07 | Target/physical conflict, rollback observation and single execution | `TestStageExecutionTargetAndPhysicalIdentity`<br>`TestStageExecutionUnknownBlocksRebindingAndCrossWorkspace`<br>`TestStageExecutionMetricAmbiguityInvalidatesExistingRollback`<br>`TestStageExecutionMetricReverseOrderAndOlderArrivalCannotResolveAmbiguity` | E02 FAIL; full A remains open |
| A08 | Versions, authority/expiry, original keys and late history | `TestStageCompletionFinalCurrentCredentialHorizons`<br>`TestStageCompletionFinalClockRunsAfterWriterReceipts`<br>`TestStageScopeRoleRestorationDoesNotReviveApproval`<br>`TestStageExecutionLateAuthenticatedFactDoesNotRenewApproval` | E01 PASS, E02 FAIL, E03 NOT_RUN, E04 NOT_RUN; full A remains open |
| A09 | Stage plus M1 all writers/three modes/R38 and outside consumer | `TestStageCompletionAllM1ModesAndActualWriters`<br>`TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt`<br>`TestStageCompletionPrivateGuardTransactionAndSingleUse`<br>`TestStageCompletionGuardRequiresM1Finalization` | E04 NOT_RUN; full A remains open |
| A10 | Current source privacy and mid-revocation reads/cache | `TestStageArtifactReportReadAuthorityMatchesR4`<br>`TestStageArtifactReportAuthorityLocksAcrossReadAndQA`<br>`TestCollaborationStageCompletionPreviewRouterReadRevocationOrders`<br>`TestStageSourceDeniedDigestAndCachedScopeSelection` | E01 PASS, E02 FAIL, E04 NOT_RUN; full A remains open |
| A11 | Matching production Web/Desktop real four-case verification | Accepted source/build/runtime inputs; actual four and after still required. | E01 PASS, E02 FAIL, E03 NOT_RUN, E04 NOT_RUN; full A remains open |
| A12 | Independent complete evidence, scoped diff/after and parent ledger | Accepted source/build/runtime inputs; actual four and after still required. | E01 PASS, E02 FAIL, E03 NOT_RUN, E04 NOT_RUN; full A remains open |

## A09 / Q01: the positive composition is already executed

`TestStageCompletionAllM1ModesAndActualWriters` has **off/observe/enforce × submit/submission/self/admin**, with all **12 verified leaves** actually PASS in normal and race. In the current, byte-identical source (`collaboration_stage_completion_tx_test.go:744`), each positive fixture reaches controlled verified deployment. Non-submit consumers lose their source membership; the independent artifact issuer remains P while consumer E is distinct. Enforce still refuses a missing grant, then obtains the real source-issued grant and consumes exactly one terminal. R38 uses real creator disable/current administrator and its own grant, does not invent initiator acceptance, and reopens/replays the original completed key without a new completion.

The separate `TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt:1250` covers wrong consumer, Agent actor, revoked issuer/consumer and private-field-free original receipts. `TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery` remains useful real-router M1/self/submission/admin evidence, but its older stage-off fixture is not relabelled as the stage-enabled positive. E04 alone supplies only the noted self/R38 negatives. **No new full Go run is needed solely to fill those already executed positives.**

## W10: minimal matching CLI negative supplement

This is an evidence item, not a discovered vulnerability. Inspected E01/E02 source executes matching CLI report positives; the retained W10 code gate separately says ordinary matching completion runtime was not run. Current CLI source still requires `Target.Kind == "ordinary"` (`cmd_issue_completion.go:880`) and uses the existing ordinary PUT (`:479`); mcb/local-human identity fences remain separate.

Use the already bound CLI `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r18-stage-execution-read-lock-fix-r01/runtime/build-r01/multica` (SHA256 `c089ace5853d7614c0db8fd01b88d937f0927d24a653a30ad456238b4202fae2`), a newly logged-in **synthetic creator P** (`1a4a48b5-659d-4e08-ab6f-3084905c9dfb`), its source workspace `e37632b9-1082-4664-9b83-b4eb74d40898`, and the retained first-PASS root `01a1e4a0-afa2-72f9-94c2-3c3c9ba256f1`. Inject the JWT only through private `MULTICA_TOKEN`, with a fresh private CWD/HOME and no inherited task/run context. No real-account credentials or installed ambient CLI lookup.

Two bounded commands, after a new main-owned permit and a verified before snapshot:

```text
<bound-cli> --server-url http://127.0.0.1:64922 --workspace-id e37632b9-1082-4664-9b83-b4eb74d40898 issue completion prepare 01a1e4a0-afa2-72f9-94c2-3c3c9ba256f1 --status done
<bound-cli> --server-url http://127.0.0.1:64922 --workspace-id e37632b9-1082-4664-9b83-b4eb74d40898 issue update 01a1e4a0-afa2-72f9-94c2-3c3c9ba256f1 --status done --priority <actual before.issue.priority>
```

The first checks the ordinary completion read/scope boundary. The second keeps priority equal to its actual current value: that makes `issueUpdateIsStatusOnly=false` and reaches the real existing PUT, rather than merely failing at the advisory completion hint. Do not guess priority, force a completion grant or change the original four-case body/denominator. Record actual method/path/status/error and a nonzero CLI exit; a generic503/auth/configuration failure is not sufficient refusal evidence.

Before and after, run these exact parameterized reads in `BEGIN READ ONLY ISOLATION LEVEL REPEATABLE READ`, with a five-second local statement timeout and `ROLLBACK`. Parameters are root UUID and source workspace UUID. Private returned collaboration/stage JSON should be SHA256-hashed by the recorder; no database extension is needed.

```sql
SELECT id::text,workspace_id::text,status,priority,revision FROM issue WHERE id=$1::uuid AND workspace_id=$2::uuid;
SELECT to_jsonb(c) AS snapshot FROM issue_collaboration c WHERE issue_id=$1::uuid;
SELECT to_jsonb(s) AS snapshot FROM collaboration_stage_task s WHERE issue_id=$1::uuid;
SELECT (SELECT count(*) FROM collaboration_operation WHERE scope_id=$1::uuid) AS operations,(SELECT count(*) FROM collaboration_event WHERE issue_id=$1::uuid) AS events,(SELECT count(*) FROM issue_handoff WHERE issue_id=$1::uuid) AS handoffs,(SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1::uuid) AS stage_submissions,(SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1::uuid) AS authorizations,(SELECT count(*) FROM delivery_completion_terminal t JOIN delivery_completion_authorization a ON a.id=t.authorization_id WHERE a.issue_id=$1::uuid) AS terminals,(SELECT count(*) FROM delivery_evidence_evaluation WHERE issue_id=$1::uuid) AS evaluations,(SELECT count(*) FROM agent_task_queue WHERE issue_id=$1::uuid) AS queued_tasks;
```

Require stable issue status/priority/revision, stable collaboration/stage snapshots and equal scoped operation/event/handoff/submission/authorization/terminal/evaluation/queue counts. The real queue table is `agent_task_queue`; do not use an invented `task` table or an absent command/run issue_id predicate. This is a separate W10 receipt, not an extra or edited E2E business case.

## Remaining acceptance

Resolve E02 and bind a new complete original four-case run. Then collect independent source/output/database/process/gate/artifact after, locate/add W10's matched negative receipt, and let main update the parent ledger/gaps before full A12/S2 archival. Parent24 stays **0 PASS / 9 PARTIAL / 15 NOT_RUN** until the owning main session proves a promotion. Real S4 targets, final S5 performance, formal business sign-off and full installed Desktop Main remain outside these local S2 observations.

This reviewer changed only the two assigned report files; no product/spec/task/matrix edits, lint/typecheck/test/DB/API/build/signal/Git commands. Reference hashes, JSON structure, representative actual PASS names and the raw event summaries were verified locally. Complete machine evidence and query/argv shapes: `r20-final-a01-a12-evidence-map-20261116.json`.
