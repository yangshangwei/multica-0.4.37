# R33 policy-wait test coordination follow-up

**R33_COORDINATION_FOLLOWUP_SOURCE_FROZEN_STATIC_HANDOFF.** The approved three coordination changes are frozen in the new source-red-r02 test. ROOT's new actual RED remains required. The original1PASS/5FAIL run is retained as a test-coordination failure and is not accepted business RED.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; owner `/root/r32_policy_implement`; captured UTC `2026-11-18T00:30:13.146759+00:00`. Only the permitted test in the new copy and this new report pair were written; reports are sealed0444.

## Retained failure and diagnosis

Original result SHA256 `ba917c29dbf4f835b310e94586605b0e573b446783e3fd954aa531095661e2da` and raw stdout SHA256 `26bd70e7e3c8b9f39606a224775e7786b6bf8f1e5fefaaf500dc238bc77b296a` match. Its6 named terminals are1PASS/5FAIL: the three concurrent leaves stop in the ancillary50ms witness coordinator, propagating to both top-level tests; member_denied passes. The short/long/competing real project Scan records retain native55P03 and measured16.2795/2.110375/7.689167ms. These are genuine query-error witnesses, but the intended policy response, long-budget and stale-CAS assertions were not reached.

The initial helper checked the finished channel once, then treated an ancillary pg-stat query error or observation-context completion as fatal without checking a now-ready query result. Finished publication also followed frame/log construction, while a lazily acquired witness connection could consume the50ms observation interval. That coordination path explains the premature exits; secondary cancellation/cleanup errors are not promoted to the original cause. A controlled product RED was not established by that run.

## Exactly three approved changes

ROOT permit SHA256 `12989d75e79ed3f01eea410fae88f518796942b6ad05733c3b3da8957a2f5f0f` and copy receipt SHA256 `be92a6fe1cd7752277ad36e0e336bbaa34a415aacfc2629656ba877566a6c2ef` select only `server/internal/handler/delivery_policy_project_wait_test.go` beneath `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-red-r02`.

1. Publish the actual project Scan result immediately after the existing state update, before collecting source frames/logs. Original SQL, arguments, destinations, error and frame capture remain.
2. Reuse the original strict errors.As/55P03 check in a nonblocking helper, at loop entry and again before fatal witness-query-error/context-done branches. A consumed genuine result returnstrue immediately; missing or other results remain non-authorizing.
3. Acquire one actual testPool witness connection and warm the exact existing read-only pg_blocking_pids statement before starting the first holder. The same connection supplies subsequent real-PID observations and is released after all request goroutines are joined. The warm PID0 comparison cannot satisfy the later proof.

The50ms observation/release schedule,250ms original per-lock setting and release guard,350ms long hold, one-second response guard, five-second orchestration, original request/authority, all snapshot checks and all6 test assertions are unchanged. A real native55P03 or actual first-holder blocker is still mandatory. No PostgreSQL error, row, PID or passing result is fabricated; no product query/default/CAS/permission is changed.

## Frozen source and static proof

- New test: `server/internal/handler/delivery_policy_project_wait_test.go`, 491 lines/20049 bytes, mode0644, SHA256 `443fc8a99a88130df7005854d9759360341c982190c75b0e84100de7547f22b3`.
- `gofmt -w` on that one new-copy test: exit0, empty output.
- New-copy manifest:5056 regular files, unchanged paths/modes, exactly one changed test,5055 protected original files byte/mode-identical, product delta0.
- Final path-sorted compact sorted-key manifest SHA256 `735b08eb19afc0a278b78b08ed99811582d6f81ae3a4d44d653f0d0ee82c9e89`; protected5055 manifest SHA256 `26a4ad38ecf668f5fd665c05aa860dee948cfbfdd0af8b97035afadfc1c0f0e3`. Full manifest and selected diff are embedded in JSON.
- Static text reconstruction proves that reverting only the allowed QueryRow publication move, ProveAttempt/helper change, WaitCase witness acquisition/lifetime and necessary import/constant reconstructs the original entire test byte-for-byte. Both top-level test bodies and all six named identities remain identical.
- Original source-red-r01 test SHA256 remains `aed914a8acd64465219a8fcb07541af3caed74e899c54796b873a6deaa8340a0`; original result/stdout and earlier implementation JSON still match their retained hashes. The original full-manifest receipt remains unchanged; this turn made no original-source write.

Go AST/compile/vet/test/list/TestMain, SQL generation, database/API/Redis/browser/process-control/Git were **NOT_RUN by this implementer**. All verification here is filesystem/static plus retained raw parsing. No current database state or connection count is claimed from execution.

## ROOT handoff

Fresh RED-only handler profile allocation SHA256 `c76e1f9cd301f02ef3c2c77795fba7a739fb7bb2ab10beb7c2aa7d72093ee651` is ROOT-owned. ROOT can extract `source_after_manifest`, perform its compilation/static admission and execute the unchanged selector `^(TestDeliveryPolicyProjectWaitShortReader|TestDeliveryPolicyProjectWaitBoundaries)$`. Independent acceptance of genuine new RED must precede the M1-only product slice. Preserve initial TestMain/fixture invalidation history and distinguish it from per-operation effects. No further author edit is pending; fullS2, original successful completions and historical holder attribution remain separate gates.
