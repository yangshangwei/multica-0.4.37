# R33 policy-wait test coordination independent check

**ACCEPTED_R33_TEST_COORDINATION_FIX_BOUNDARY_ONLY_INITIAL_RED_NOT_ACCEPTED.** The frozen one-file coordination correction, new static checks and fresh RED profile are accepted with no remaining boundary blocker. Initial actual **1PASS/5FAIL/0SKIP** remains a coordination failure, not accepted business RED. A genuine new six-name run and independent RED acceptance must precede product changes.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; reviewer `/root/r32_after_check`. Only this new report pair was written/sealed0444. Review used saved files/source hashes/metadata; no Go/build/test/vet/lint/type-check, DB/SQL/Redis/API/browser/process/Git execution, source/config/old-report edit or agent spawn. Peers' files and original completed keys remain preserved.

## Findings (fixed)

- File: `server/internal/handler/delivery_policy_project_wait_test.go` in new `source-red-r02`.
- Issue: Initial concurrent leaves exited in the50ms ancillary witness helper before intended response/effect assertions. The helper checked finished once, then failed on witness-query error/context completion without rechecking a genuine ready Scan result. Publication followed frame/log work; the ancillary query acquired a pool connection inside the short interval.
- Fix: The author made exactly the permitted coordination changes: pre-acquire/warm one dedicated existing-pool read-only witness, publish the unchanged actual Scan result before frame/log work, and recheck the strict native55P03 result before fatal ancillary-error/context-done branches. Original source/run remain untouched. No reviewer source edit occurred.

## Findings (not fixed)

No unresolved defect was found in this correction boundary. Actual corrected-source RED is pending; ancillary timeout without a genuine native failure or actual first-holder blocker still fails. The initial query-error witnesses do not establish the intended complete regression outcomes. R32's current policy-lock refusal remains proved, its physical holder and R31 attribution unknown. Product/full-S2 acceptance remains separate.

## Verification

### Initial executed coordination failure

Initial result `ba917c29dbf4f835b310e94586605b0e573b446783e3fd954aa531095661e2da` and stdout `26bd70e7e3c8b9f39606a224775e7786b6bf8f1e5fefaaf500dc238bc77b296a` match their retained receipts. Independent parser reads62valid Go JSON events,6unique starts/terminals,1PASS/5FAIL/0SKIP, no parse/missing/extra/pending/drift or suite replay. Leaf outcomes are member_deniedPASS and three concurrentFAIL; both tops propagate failure.

Short/long reader fail at ancillary pg-stat query context errors; competing writer fails at observation-context completion. None reaches its intended response/effect assertions. Actual second project Scan events nevertheless retain native55P03, canonical query hash `f56557e1843a2ebb4a8d99175df7e5edd369d42445f06127536eab0cd93eab7e`, real PID/current actor/project/workspace and16.2795/2.110375/7.689167ms respectively. Those are genuine query failures, not accepted business RED. Lazy acquisition and frame/log work are identifiable coordination costs; their individual timing contribution was not measured. Cleanup cancellation errors remain secondary.

Member-denial snapshot equality passed. Original before/after database identity, migration ledger and indexes match with0otherconnections. Retained counts change collaboration_config_invalidation21→32 and agent_invocation_target0→1. Full refused-call effects were not asserted by the failed concurrent leaves; no whole-database-zero-change or per-call zero-effect claim is made.

### Frozen correction and preserved semantics

Owner `0b303509cee0f274cd10cf113afc184559f919ce36b55d25c25ab9d33b81b436`, permit `12989d75e79ed3f01eea410fae88f518796942b6ad05733c3b3da8957a2f5f0f` and copy receipt `be92a6fe…` bind only the new-copy test:

| New-copy path | SHA256 | Bytes/lines/mode |
| --- | --- | --- |
| `server/internal/handler/delivery_policy_project_wait_test.go` |`443fc8a99a88130df7005854d9759360341c982190c75b0e84100de7547f22b3`|20049/491/0644 |

Independent current hashes/modes matched5056regular files. Exactly one test changes;5055 protected originals/path set/modes unchanged, product0/added0/removed0/nonregular0. Final manifest `735b08eb19afc0a278b78b08ed99811582d6f81ae3a4d44d653f0d0ee82c9e89`, protected5055 `26a4ad38ecf668f5fd665c05aa860dee948cfbfdd0af8b97035afadfc1c0f0e3`; complete manifest/diff stays in owner JSON. Original test remains `aed914a8…` in source-red-r01; original delivery-policy handler/SQL/generated/stage caller/250ms preparation are byte-identical.

The actual QueryRow/Scan arguments, destinations/errors and native identity remain forwarded. Finished publication now precedes actual_scan event/frame work. Nonblocking finished checks still require errors.As/native55P03; missing/other results remain failures. The dedicated actual testPool connection warms the exact unchanged pg_blocking_pids SQL before the first handler transaction and later uses real second/first PIDs. Warm PID0 cannot satisfy that proof. The connection is released only after channels/cancellations and all request goroutines are joined.

Both original top-level bodies, six names, exact response/effect assertions and real GET SHARE/virtual0Scan/PUT paths remain unchanged. A genuine failure or actual first-holder blocker is mandatory before release. Short50ms, product250ms per-lock setting, long350ms hold, one-second result guard and five-second orchestration remain. No timeout/query/PID/result is fabricated. The review script's initial predicate lookup selected an earlier argument-filter expression; correcting that lookup verified the intended publication-before-logging order. It was not a source or test failure.

### Static and fresh RED profile

ROOT new static `f24101138123f0261e3e8d7aed7368865a0cb6bb91d4fe02c9d888fb634fdb56` records gofmt/vet/handler compile-only exits0, six empty hash-verified logs, unchanged5056 source, product0 and no TestMain/DB use. No reviewer execution occurred.

Fresh profile `c76e1f9cd301f02ef3c2c77795fba7a739fb7bb2ab10beb7c2aa7d72093ee651` binds `multica_pay241_20261117_s2_policywait_red_r33b` for internal/handler, distinct from the used initial RED database. New explicit environment/clone refs match and the saved clone has0connections; allocation reports no test started. Original failed DB/history is retained. Actual corrected-source six-name RED and independent acceptance remain the next gates, followed only then by the M1 product slice.

- **Lint:** PASS, retained new scoped gofmt/vet and empty logs.
- **TypeCheck:** PASS, retained new handler compile-only.
- **Tests:** initial1PASS/5FAIL coordination failure retained; corrected-source actualRED not accepted here.
- **Boundary:** PASS, one changed test/5055protected/product0, unchanged strict evidence/timing contract.
- **Product/runtime/fullS2:** NOT_ACCEPTED_HERE.

R32 after-check `ecdd27b0…`, original failed runs and successful completed keys remain preserved/unreplayed. Full A01–A12/S2/parent24/S3–S5 remain open; parent0PASS/9PARTIAL/15NOT_RUN. Captured UTC `2026-11-18T00:36:07.243598Z`. No pending work remains within this coordination review.
