# Research: R29 consumed authorization receipt GET diagnosis

- Query: Exact E04 receipt409 branch/evidence limits and minimal genuine GetByID read regression/repair boundary.
- Scope: internal; retained R28 runtime and frozen source read-only. Ownership only this new Markdown/JSON pair.
- Date: 2026-11-18 (Asia/Shanghai).
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

**The completed write and Desktop consumption succeeded; the later Web receipt read failed.** Actual R28 original four is3PASS/1FAIL/0NOT_RUN,retry0. Do not replay its completion, authorization or original submitted result. Main stopped owned API/bridge72766/72762 normally; recorded after-stop connection0, restored gate epoch56,49 settled runs/35 complete artifacts and old unknown preserved. This research executes no live probe.

Runtime root is `/Volumes/artisan/.multica-acceptance/pay241-r28-preview-read-20261117-sz4vj59y`; Go source is its `source-green-r01`. Consumed frontend is `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/frontend-base-r01`, per actual command/report. E04 is `runtime/four-scenarios-r01/test-results/collaboration-stage-execut-365ef-nistrative-or-project-exits-chromium/stage-execution-evidence.json`.

### Exact recorded sequence and409 envelope

Root=`01a1ea65-ec93-7b42-920e-3166cecbf7b0`, grant=`01a1ea66-873a-7510-b2f3-d33f046435f8`, operation=`09faa47a-c020-4cef-970c-da529fa29df5`, actual P issuer/consumer=`249c1022-4cb2-45e9-ac5b-2145918e40b4`, submission=`05d3f8f9-6e00-4015-a21c-6f37d72d3061`.

| UTC recorded time | Actual event | Result / original request log |
| --- | --- | --- |
|16:45:48.810 | Web formal submission |200, immutable submission ID |
|16:45:50.290 | Web original authorization |201, original grant/key; expires17:00:11Z |
|16:45:53.006 | Desktop reference GET |200, issued original grant |
|16:45:53.452 | Desktop original lifecycle POST |200 `completed`, collaboration revision18; same interrupted Web key/body/hash |
|16:45:53.482 | Desktop consumed authorization GET |200 `state:consumed`, completed revision5, same submission/revision18; log`:16632`, request`d50bcc74` |
|16:45:53.664 | Web original operation GET |200, same completed response; log`:16648`, request`f7b89d46` |
|16:45:53.690 | Web consumed authorization GET |409; log`:16650`, request`a84d0a67`,21.226ms |

Exact failed body is **`{"code":"delivery_completion_conflict","error":"delivery_completion_conflict","message":"delivery_completion_conflict"}`**, response SHA256 `782119d92e4d3174648ea871956ee71020047c532497324f312b12bbc8df40a1`. The earlier Desktop consumed response SHA is `d27b96a939235836eaaa7c5dda96b8b78e7de44705d544152027d97a1984a1a7`; Web operation response matches completed POST SHA `2a55c7436032cea5efab74578065ff593a6c5ea0cbf9a9a074406de74ab6672e`.

Frozen E04`:564–570` installs two response waiters in Promise.all. **Actual HTTP is sequential**: Core `collaboration/operation.ts:45–46` awaits original operation recovery, then`:62–68` awaits authorization GET. View `workflow.tsx:55–65` calls that real action. Two simultaneous waiters are not proof of simultaneous operation/grant requests.

Assertion`:571` passed;`:572` requires Web consumed GET200 and failed. Later same-receipt/final-journey assertion, screenshot and admin disable/unfinished-root denial/restore block were not reached. Early project-exit probes **were reached**: E04 HTTP indices41–45 at16:45:29–30 record clear/move409, delete-impact200, managed delete409 and unrelated empty delete204. Their complete W11/W12 acceptance still awaits final audit; do not label those earlier probes NOT_RUN.

### Exact service/handler path and what409 proves

`handler/delivery_completion_authorization.go:145,154–235` validates actual principal/UUID, begins the real transaction, calls **`GetDeliveryCompletionAuthorizationInTx`**, ignores DTO on error, then selector/final-credential checks and commit. It sets no-store. A failed commit maps503 outcome_unknown, so the recorded409 is a **pre-commit conflict classification**, not an unknown completion commit.

`service/delivery_completion_authorization.go:210–222` calls `completionReadSetup` (250ms/schema/current credential), **`LockDeliveryCompletionAuthorization`**, `completionReadAuthority`, then `completionView`. Same-P issuer branch at`:346–347` calls `completionIssuerRead:354–388`, which validates current access/exact original artifact execution. It does **not** call `completionRecheckCollaborationIntent`, compare current target issue revision/status to the old target, or require old lifecycle epochs to be current. `completionStoredRequest:478` checks immutable grant/request consistency; `completionReceipt:417` checks immutable grant/terminal identity and before/after; `completionView:392` prefers terminal state over expiry.

The same completed grant already succeeded after root became done/lifecycle advanced,208ms before the failed Web read. This and the source rule out an **unconditional persistent old-target/lifecycle revalidation defect**. They do not prove the later refusal's exact low-level cause or that no current source/principal changed transiently.

| Read-chain fence / check | Frozen behavior |
| --- | --- |
| GetByID / by-request row getter | `queries/delivery_completion.sql:27,30`: both **UPDATE NOWAIT**, despite GET |
| `completionReadAuthority:338` root | `LockLifecycleIssue` NO KEY UPDATE NOWAIT |
| `completionIssuerRead:362` collaboration/session | Existing `completionCollaborationAccess` selects UPDATE NOWAIT |
| `completionSourceIssue(...false):369` | Child NO KEY UPDATE; false means structural exactness, **not shared/unlocked mode** |
| `EvaluateDeliveryEvidenceInTx:381` | Default exclusive child/principal-issue/approval/execution fences; project/PR/member/current credentials retain existing shared fences |
| `completionView` / receipt | Immutable terminal getter and local consistency; no durable write or current-target admission |

`GetWorkflowOperation` (`collaboration_workflow.go:693–741`) holds only actual user SHARE plus own-operation SELECT/current password check; it does not lock this grant/root/session/child. The exact operation/grant pair has no independently identified conflicting row. Same-grant GET siblings and artifact-bound R4 are **statically incompatible**, at grant UPDATE or nested artifact fences respectively; real regression still required.

### Historical holder remains unproved

API log`:16651` records same-root Desktop R4 candidate `01a1ea65-ef37-7e5b-89c8-5621f536f865`, request`e7497078`,200/178.341ms, beside Web grant409. This is an observed contender, **not a physical holder witness**: timestamps are second-granularity and no lock-release/query/PID record is attached. No other same-grant GET is proven to hold its lock at the failure.

Frozen `cmd/server/pay241_read_diagnostics_test.go:103–111` admits only collaboration **candidate GET**. Grant/operation GET are excluded; its query tracer requires that admitted scope. The retained diagnostic stream has no matching grant/operation ID; runner/API logs retain generic HTTP status but no correlated first PgError/query/SQLSTATE/backend ID. `deliveryCompletionError`/handler`:19–42` map55P03/40P01/40001/23505 and semantic conflicts to this same409. **Do not label original409 as55P03 or assign R4/another GET as holder.** Raising caps alone would not cover the omitted route; diagnosis should use deterministic new-fixture reads.

### Dispatched minimal owning-layer RED:3 top /10 leaf /13 named

Initial write scope is only NEW `server/internal/service/delivery_completion_consumed_read_concurrency_test.go` under `/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/source-red-r01` (accepted R28 baseline). Main owns fresh RED/GREEN/race service DBs; allocation SHA `0805d51e38e0a81a6966c5e9f84ab2a4e949d05dab60d1979581f90e8c100274`. Research neither executes them nor changes source.

| Exact top | Exact leaves |
| --- | --- |
| `TestDeliveryCompletionConsumedGetReadCoexistence` | `r4_first`, `authorization_first`, `same_grant_siblings`, `operation_first`, `operation_after_authorization` |
| `TestDeliveryCompletionConsumedGetWriterFences` | `writer_first`, `reader_first` |
| `TestDeliveryCompletionConsumedGetCurrentAuthority` | `wrong_actor`, `changed_password`, `source_revoked` |

Use bare **`newStageCompletionFixture` → `stageCompletionM1Policy(enforce)` → `stageCompletionSubmit` → `stageCompletionAcceptParams` → `stageCompletionM1ForLifecycle(f.ctx,params)` → actual `ChangeLifecycle(f.ctx,params with binding)` once**. These are new isolated fixture writes, not original replay. Same current P is issuer/consumer as runtime; real grant/consumed terminal must exist. Do not reuse a preview fixture's zero-grant assertion/pre-registered equality cleanup. Establish actual consumed GetByID200 and own operation completed baseline before concurrency; compare immutable receipt and original IDs/revisions plus separate current issue state.

Every GetByID receives actual borrowed `pgx.Tx` through existing `completionCandidatesTraceTx` and actor credential; final credential/real commit matches handler. Hold a completed first real read uncommitted, then require distinct nonzero PID/actual second terminal before release. Use existing SQL-forwarding barriers for R4/operation methods; keep five-second test readers/default isolation/250ms prep. No stub rows, forced ready state, copied cached200 or altered epochs/permissions/deadlines.

R4 **after completion can be stale yet legitimately readable**; obtain/compare its actual unheld baseline. Use the artifact/post-authority barrier (`holdStageArtifactR4` or existing starter with real P context), not `holdStandingExecutionR4`'s valid-QA-chain probe or a fake current candidate. Operation's two orders are **passing controls**, not forced RED; its generic held helper hardcodes reviewer context, so use actual completed P/request key with the existing starter rather than query another actor's receipt.

Writer-first must call real `RevokeDeliveryCompletionAuthorizationInTx` on this fresh consumed grant: it reaches original UPDATE and `GetDeliveryCompletionTerminal`, returns semantic409 while its borrowed transaction retains locks. Hold it, verify reader refusal/first real PgError, then rollback and fresh consumed read recovery. Reader-first holds real GetByID and requires the real revoke's55P03 at original authorization UPDATE before terminal/effects; after release revoke still semantically conflicts with consumed, without rewriting its terminal. A bare409 cannot prove writer fencing. Current actor/password/source-member negatives are real mutations/context on the fresh fixture; register72-relation/adapter/admin equality **after** intentional changes and before handles.

### Repair boundary and original controls after genuine RED

**No speculative patch.** If this real RED proves GetByID read exclusion, initial product proposal is **`service/delivery_completion_authorization.go` only**, an explicit GetByID-only read authority path: compatible grant/root/child fences and existing `readCompletionCollaborationAccess`/`readDeliveryArtifactInTx`/`evaluateDeliveryEvidenceReadInTx`, preserving every original current-actor/source/producer/spec/receipt check and immutable terminal. Existing id getter plus an actual SHARE row fence can avoid generated SQL changes. Do not remove authority/row fencing, renew a grant, alter receipt consistency, or invoke old-target admission to read history.

Keep original `completionIssuerRead` for create-key replay/revoke and all actual grant locks in Create/Prepare/Finish/Revoke exclusive; by-request GET, other target/consumer kinds and R38 stay unchanged unless the actual delta explicitly includes them. Adding a general marker does not fix the first grant lock or source-child wrapper. Any additional product/helper/SQL scope needs main's bounded dispatch after actual failed-query evidence.

| Original affected control scope | Exact selectors/names; preserve evidence separately from R28 old58 |
| --- | --- |
| Currently service-admissible grant/receipt/current consumer | `^(TestDeliveryCompletionReceiptRejectsMismatchedRetainedFacts|TestDeliveryCompletionReceiptValidatesClosedVariants|TestStageCompletionAllM1ModesAndActualWriters|TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt)$` —4 top/63 named/44 leaf, source-derived; real grant create/consume/reopen-key flows, original consumer/issuer isolation, closed/mismatched receipts |
| Handler grant lifecycle/access/replay | `^(TestDeliveryCompletionAuthorizationActualArtifactAtomicConsumeAndReplay|TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant|TestDeliveryCompletionAuthorizationRollbackRevokeAndTransactionIdentity|TestDeliveryCompletionAuthorizationTaskAndConsumerIsolation|TestDeliveryCompletionAuthorizationConsumeRevokeAndEvidenceLocks)$` under`./internal/handler`; includes issued/by-key create replay, consumed-after-reopen/no rewrite, revoked terminal/original identity, current access/task consumers, real consume/revoke/evidence locks |
| If HTTP/current selector/consumer wiring changes | `^(TestDeliveryCompletionAuthorizationRouterCurrentCredentialRecheck|TestDeliveryCompletionAuthorizationRouterConflictingWorkspaceSelectors|TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery)$` under`./cmd/server`; meaningful retained actual current-credential/selector/separate-issuer/self/admin original receipt scope |

The latter packages are **not admitted by the three service allocations**: main must select their own fresh package permits if executed. Reuse exact unchanged retained controls by source/default/caller bindings; changed grant-read code cannot be covered solely by old preview58. Preserve initializer/consumption/final clock/immutable grant/terminal and grant-key replay branches by source audit; broaden only if those branches change. A separate historical-after-reopen GetByID leaf is justified only for a reached transition/consistency defect; persistent old-target failure is not established here.

## Files / specs / external references and caveats

Sources above are frozen Go/frontend copies; exact E04 result/report/HTTP/API log and diagnostic references/hashes are in JSON. Read current `CLAUDE.md`, workflow and server completion/stage/test-isolation/verification contracts; no implement/check manifests, credential store, external docs/account or unrelated history scan. This researcher wrote only these reports and executed no SQL/Go/tests/list/TestMain/API/browser/build/signal/Git or completed write.

Original3/1/0 and Desktop completed/consumed facts remain immutable; full journey/A01–A12/S2/parent24/S3–S5 are not accepted by this diagnosis. New regression may prove a reachable read defect while original physical holder remains **UNPROVED**. Body is retained; original query/SQLSTATE/PID is the explicit remaining gap.
