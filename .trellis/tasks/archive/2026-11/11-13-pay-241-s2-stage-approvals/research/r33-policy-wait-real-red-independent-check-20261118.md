# R33 genuine policy-wait RED independent check

**ACCEPTED_R33_GENUINE_POLICY_PROJECT_WAIT_RED_CONTROLS_PASS.** The corrected-source run establishes both intended old-source failures and passes the long-reader/member controls. Exact named result is **2PASS/4FAIL/0SKIP** across2top/4leaf/6named; leaf result is2PASS/2FAIL. The two failed tops propagate their failed leaves. This accepts genuine RED evidence only, not product/GREEN/runtime/full-S2 completion.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; reviewer `/root/r32_after_check`; source `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-red-r02`. Only this new report pair was written/sealed0444. Saved raw output/source hashes/static/profile evidence were reviewed; no source/config change, Go/build/test/vet/lint/type-check, DB/SQL/Redis/API/browser/process/Git execution or agent spawn. ROOT owns subsequent implementation/execution.

## Findings (fixed)

None. No reviewer source change was needed or authorized.

## Findings (not fixed)

- The requested short-reader coexistence is absent on unchanged product source: actual GET200 remains held, actual PUT reaches the canonical exclusive project query and returns native55P03/HTTP409busy before the50.968708ms release, rather than required200/one append.
- The requested waiting-writer serialization/CAS path is absent: first actual PUT holds one uncommitted append; second same-revision PUT returns immediate55P03/409busy before52.02325ms release, rather than reaching the required stale-CAS refusal after the winner. Winner commits exactly one policy row; no losing append.
- Recommendation: ROOT may proceed with the separately planned M1-only explicit `LockDeliveryPolicyProjectWait` SQL/generated method and sole PUT call-site switch. Preserve the original query and stage-policy NOWAIT caller, existing250ms per-lock budget, predicates, authority/credential checks and CAS. Product verification and spec synchronization remain separate gates.

These are the two intended regression failures, not new setup/coordination failures. No controlled fixture PID identifies the old R32 physical holder. Full S2 and original successful completion histories remain open/preserved.

## Verification

### Actual exact run and denominator

Result `3b94b4a452213059922556d102f58735f06a7ba8c1be6434a1f16b5819bfc92f`, raw stdout `1a8b5e80e14b6ac89311c034b1530929561e652181854f28bb0050c537161176` (32978bytes), empty stderr and command/profile/permit hashes agree. Independent parser reads75valid Go JSON events,6unique starts and6unique terminal names, no parse/missing/extra/duplicate/pending/drift/skips/pause/parallel events. PackageFAIL/exit1 is expected RED; suite_reranfalse.

Actual command is the admitted normal `go test -json -count=1 -timeout=120s ./internal/handler -run '^(TestDeliveryPolicyProjectWaitShortReader|TestDeliveryPolicyProjectWaitBoundaries)$'` from the frozen source/server. Permit `5e59a4bf…` binds5062references, one command and the accepted coordination review. Ambient environment inheritance isfalse; fresh RED database is `multica_pay241_20261117_s2_policywait_red_r33b`. This is the new numbered r02 run, not a replay/reclassification of initial r01.

| Exact leaf | Terminal | Actual evidence |
| --- | --- | --- |
| `TestDeliveryPolicyProjectWaitShortReader/short_reader_release` |FAIL|GET200; PUT409busy vs required200; native failure before50.968708ms release; policy rows0 |
| `TestDeliveryPolicyProjectWaitBoundaries/long_reader` |PASS|GET200; PUT409busy/native55P03; reader held369.668583ms; no append |
| `TestDeliveryPolicyProjectWaitBoundaries/competing_writer` |FAIL|firstPUT200/one append; second409busy vs required stale-CAS; release52.02325ms |
| `TestDeliveryPolicyProjectWaitBoundaries/member_denied` |PASS|current member GET200/PUT403management-denied before writer query; no append |

Both tops fail only by propagation. Four leaf outcomes are not added to the original runtime four-case denominator. The original R32 runtime remains3PASS/1FAIL/0NOT_RUN,retry0.

### Genuine query/barrier/authority sequence

All three concurrent leaves retain a warmed actual witness PID29991, first real holder PID29994 and second real PUT PID29995. These are distinct connections within each leaf; their reuse across sequential leaves is normal pool reuse. Original250ms SET LOCAL preparation succeeds in each actual handler transaction.

Short/long readers acquired real `LockProjectForAssociationNowait` SHARE for the same project/workspace/current JWT actor, then held actual `GetDeliveryPolicy` Scan ErrNoRows with transaction statusT; after release original virtual0/off GET responses remain exact. Second PUT's canonical `LockDeliveryPolicyProject` Scan has the matching actor/JWT/UUID arguments, true PostgreSQL55P03 and SQL SHA256 `f56557e1843a2ebb4a8d99175df7e5edd369d42445f06127536eab0cd93eab7e`. Its real source frames identify generated scanner209 and deliveryPolicyInTx135. Recorded failure-before-holder-release precedes the actual release in each leaf.

Short-reader native Scan fails in3.11025ms, with failure witness00:37:34.109349Z before release00:37:34.156932Z. The test reaches original GET200/PUT409 response logging, protected snapshot and required200 assertion; no ancillary-witness setup error remains.

Long-reader native Scan fails immediately in0.517542ms on the original NOWAIT path, with the GET held until369.668583ms after project entry. PASS proves the refusal/no-append boundary under a long-held real reader; it does not prove this unchanged NOWAIT query waited250ms. The timeout-bound waiting behavior belongs to the future GREEN source. Reader completion/release and both protected snapshots pass.

Competing winner reaches real CreateDeliveryPolicy Scan with one uncommitted append and transactionT. Second real PUT's55P03 Scan occurs0.538958ms after entry; native failure witness00:37:34.715033Z precedes release00:37:34.765664Z. FirstPUT200 and exact revision1/requested mode/current-human/timestamp binding checks pass. The losing request creates0rows; its intended `delivery_policy_revision_conflict` assertion fails on busy. This is a distinct desired-contract RED, not acceptance of generic409 as stale CAS.

Member denial's PASS proves the actual current member GET/PUT permissions, exact403error, no project-writer entry/append and protected fixture equality. It is fixture JWT/current DB authority, not router/login evidence. The unchanged observer forwards actual SQL/Scan/errors; no PostgreSQL error/actor/PID or product response is fabricated.

### Effects, source and retained failure

Five raw effect records cover four leaf cases because long-reader logs the same protected snapshot twice. All protected_equaltrue. Per-case policy rows are0(short),0(long),1(winner),0(member). Short/long/member full selected before/after hashes are equal; winner's full hash changes for the allowed policy append while all protected non-policy facts remain equal. These scope project/issue/authority/stage/artifact/task/evaluation/authorization/terminal history; exact append assertions remain separate.

Before/after database audits identify the same fresh owning database with0otherconnections, unchanged migration ledger `d648c6e258f50d6abbeb3f025475ea6b3bb6ff02501c57713897fc1be606ec3e` and indexes. Retained count changes are collaboration_config_invalidation21→32, agent_invocation_target0→1 and project_delivery_policy0→1. The last is the allowed committed winner; fixture/TestMain history remains. No whole-database-zero-change claim is made.

Frozen test stays `443fc8a99a88130df7005854d9759360341c982190c75b0e84100de7547f22b3`,20049bytes/491lines/0644. Accepted coordination/source boundary `ee1f1bbe…` retains5056files/5055protected/product0, final manifest `735b08eb19afc0a278b78b08ed99811582d6f81ae3a4d44d653f0d0ee82c9e89`; actual result input_drift[] confirms the bound inputs. No full source/old control graph was re-reviewed here.

Retained static `f2410113…` gofmt/vet/compile-only exits0 and six empty logs match; this reviewer ran no checks. Initial r01 `ba917c29…` remains1PASS/5FAIL coordination failure, not retroactively accepted RED. The present r02 completed intended assertions and is separately accepted.

- **Lint:** PASS, retained scoped gofmt/vet evidence.
- **TypeCheck:** PASS, retained exact-source handler compile-only evidence.
- **Tests:** FAIL as genuine RED,2PASS/4FAIL/0SKIP named;2PASS/2FAIL leaf; boundary controlsPASS.
- **Source/evidence scope:** PASS, unchanged frozen test/product0 and complete exact actual run.
- **Product/GREEN/runtime/fullS2:** NOT_ACCEPTED_HERE.

ROOT owns the next three-file M1-only product slice, canonical generation and same-source GREEN/race/focused original controls. Original query/stage NOWAIT must remain unchanged. Prior completed keys are unreplayed, original runtime failures retained, and full A01–A12/parent24/S3–S5 remain open; parent0PASS/9PARTIAL/15NOT_RUN. Captured UTC `2026-11-18T00:42:43.010346Z`. No pending work remains within this actual RED review.
