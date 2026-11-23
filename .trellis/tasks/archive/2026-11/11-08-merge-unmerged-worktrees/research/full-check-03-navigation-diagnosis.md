# Full-check-03 global navigation diagnosis

Status: preliminary read-only investigation, awaiting original final timeout stack. Updated 2026-11-08T13:38:30.252552+08:00. Candidate00e99892fdf2ad65b1bdd04669c800bf97d4649a remains frozen while full03 runs.

## Original evidence

- Full log item112/line9164: Web global-scope-navigation failed at180 seconds. The same spec passed3.7 seconds in focused e2e-activated-01; adjacent Desktop variant passed6.2 seconds in full03. These observations do not classify the failure as harmless/flaky.
- Original test-results contains web-scope-workspace.png and web-scope-resumed.png, but no compact screenshot. Thus the test completed the first workspace/account/filter/list+detail-scroll resume assertions and entered or approached the later injected read-recovery segment.
- error-context.md retains24 collaboration items, Search="Resume collaboration", Sort="Title", and selected issue `01a1b9a5-07ab-7b4a-9960-691fefceeae2`. No detail error/Refresh banner is present in its final snapshot. There is no retained trace in this case's output directory.
- Main observed no waiting locks/active PostgreSQL query while the test was waiting. Independently read API log: this exact selected issue returned initial short503s then200, including a200 at13:32:45.354 and successful15-second poll reads through13:35:30.679 (last12–33ms). A stalled database or sustained selected-issue endpoint failure does not explain the3-minute wait.

## Current hypothesis and uncertainty

The narrow candidate is test-side synchronization around its injected recovery request, response completion, double requestAnimationFrame or active route teardown. The test has unbounded plain awaits for recoveryReady and Promise.allSettled(activeRecoveries), plus browser promises for response.finished() and double rAF. Earlier query aborts during global scope changes can leave a different interception lifecycle from the intended current refresh. This is not yet a confirmed root cause: final stack must identify the blocked await; a future bounded/trace probe may be needed after the full run ends.

Do not remove the retained error/recovery/scroll assertions, expand timing budgets or introduce retries based on this hypothesis. Do not change a product query policy to accommodate an unproven test issue.

## Exact selected-issue API observations

Source: /Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification/dev-registry/envs/check-20261108044032-9601/logs/api.log

```json
[
  {
    "line": 7589,
    "text": "13:32:44.009 INF http request method=OPTIONS path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=15\u00b5s request_id=yangshangweideMac-mini.local/elhNHPSr4o-002183"
  },
  {
    "line": 7591,
    "text": "13:32:44.013 ERR http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=503 duration=2.781ms request_id=faaadbea user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7594,
    "text": "13:32:44.050 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=37.432ms request_id=3f4198d1 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7671,
    "text": "13:32:44.382 ERR http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=503 duration=15.818ms request_id=f07e7eb3 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7680,
    "text": "13:32:44.443 ERR http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=503 duration=58.813ms request_id=b3757bba user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7683,
    "text": "13:32:44.494 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=50.055ms request_id=90808664 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7845,
    "text": "13:32:45.354 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=152.474ms request_id=cbfd1934 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7857,
    "text": "13:33:00.372 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=14.779ms request_id=f0858e8f user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7870,
    "text": "13:33:15.420 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=27.826ms request_id=1cd09199 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7882,
    "text": "13:33:30.466 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=33.33ms request_id=976fd8ac user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7895,
    "text": "13:33:45.494 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=21.773ms request_id=a3994015 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7907,
    "text": "13:34:00.521 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=21.369ms request_id=e8cbd3a2 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7919,
    "text": "13:34:15.553 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=24.31ms request_id=b0d608f6 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7931,
    "text": "13:34:30.577 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=14.018ms request_id=d700c957 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7944,
    "text": "13:34:45.610 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=23.409ms request_id=bef76ec0 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7956,
    "text": "13:35:00.635 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=20.776ms request_id=c54a90b2 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7970,
    "text": "13:35:15.653 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=12.338ms request_id=1f667850 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  },
  {
    "line": 7981,
    "text": "13:35:30.679 INF http request method=GET path=/api/collaboration/issues/01a1b9a5-07ab-7b4a-9960-691fefceeae2 status=200 duration=13.277ms request_id=931b6bd3 user_id=ac3f3626-e901-4181-aef0-870984390631 client_platform=web client_version=0.4.40 client_os=macos"
  }
]
```

Snapshot SHA256: `a1283e0b59cc67572becbbbdb4a4b241d9c4edd2e5646868c8c06c81aefb9e00`.

## Next step

Await original final Playwright stack; preserve error-context/screenshots and log before another runner touches test-results. Then correlate the failed source line with the successful API read timeline. No candidate edits, tests, browser launches, builds or database writes were performed by this reviewer.

## Final run result and diagnostic limit

Full03 completed with exit2:328 E2E passed,112 skipped,1 failed. All three owned renderer/Web/API services stopped. The final reporter gives only `Test timeout of180000ms exceeded` and the existing error-context path, without a source-line stack. No trace exists for this original test. Therefore the exact blocked promise remains unproved; no product/test fix is justified solely from this run.

Recommended next action after copying original evidence: rerun the original Web case against a fresh owned production environment with trace enabled, preserving assertions/timeouts/retries. If it passes without revealing the stall, use a separately retained diagnostic copy with named steps/events around the existing recovery awaits, route lifecycle and rAF; capture document visibility and requestfailed/response identity. A passing diagnostic/replay is not a passing original complete gate. Any repair must retain the held real renderer response, exact200/response completion and unchanged scroll620/800 assertions. No retries/timeout increase or assertion removal recommended.

## Confirmed follow-up

Archived unchanged-source navigation-diagnostic-03 reproduced3 failures among20 repeats. Repeat5 trace identifies the blocked operation as page.waitForResponse at old line166, waiting on the first intercepted request, which ended net::ERR_ABORTED; a later request to the same selected issue returned200. Initial cleanup/rAF hypotheses are superseded by this direct trace evidence. See navigation-recovery-independent-review.md for the exact sanitized timings and review of the finite pre-release request-set repair. Original full03 and all diagnostic failures remain failed records.
