# Independent review: navigation recovery request identity

Reviewed at: 2026-11-08T14:11:23.455925+08:00

**Verdict: APPROVE for the frozen single-file test repair** (`e2e/global-scope-navigation.spec.ts`, SHA256 `050d1e0a02cfe59d543b7e2e70b09d5e0f64cf50ddc17009fb81d934622dab29`). No concrete remaining finding in this patch. The main-owned both-platform repeated browser run and a fresh full gate remain required; this is not a claim that those running checks already passed.

## Root cause supported by original trace

The unchanged20-repeat diagnostic produced17 passes and3 failures (repeat5/10/14), retained separately from failed full03. Reviewer independently parsed archived repeat5 trace: `pw:api@244` is `page.waitForResponse` at old line166, started30260.648ms and ended207673.957ms with the180-second test timeout. `route.continue` operations247 and248 both completed. Thus the stalled operation is the response identity waiter, not the earlier hypothesized route cleanup or rAF wait.

The exact selected-issue network evidence, omitting auth/session headers:

```json
[
  {
    "monotonic_time_ms": 30258.751,
    "method": "GET",
    "status": -1,
    "failure": "net::ERR_ABORTED",
    "continued": true
  },
  {
    "monotonic_time_ms": 30275.689,
    "method": "GET",
    "status": 200,
    "failure": null,
    "continued": true
  }
]
```

The original code permanently bound recovery to the first intercepted Request. That request was aborted during query replacement, while a subsequent request to the same selected issue returned200. Because the predicate required object identity with the canceled request, successful replacement and later polling could never resolve it. The full03 final snapshot showing recovered content and fast ongoing API200s is consistent with this trace, but the repeated trace provides the direct proof.

## Repair review

- Scope remains the exact selected collaboration issue path. The route handler enrolls actual Request objects only after recovery mode begins and before the explicit release; a local boolean closes enrollment synchronously before releasing held requests.
- The response listener is registered before release and requires status200 plus membership in this finite held set. Later automatic polling is continued normally but cannot be enrolled and therefore cannot satisfy the waiter. This avoids replacing the original predicate with an arbitrary next successful endpoint response.
- A second real keyboard Refresh is performed while the first request is held, and the test requires an increase in held request count. This drives the superseding-request condition through the existing UI/query behavior instead of depending on incidental earlier refetch timing. Both requests stay held until the original pre-release scroll assertion.
- No synthetic success response is introduced: `route.continue()` still delivers real renderer transport. Existing response status200 and `response.finished() === null`, banner disappearance and post-response scroll620 assertions remain unchanged.
- Initial saved list/detail positions, workspace/account resume, filters/sort, global navigation bounds, explicit-anchor focus, later scroll800 resume, compact/Chinese layout and native page-error assertions are all retained. No test timeout increase, retry, skip, product-source or dependency change.
- The original route cleanup still releases pending handlers, settles active operations and unregisters the interceptor. The deterministic replacement assertion adds coverage for the observed canceled-read case without changing product behavior.

## Verification

Reviewer performed static diff/source and archived trace inspection only. No browser/DB/build/tests were launched and no candidate source was edited while navigation-repair-01 was running. Main owns its final pass/fail inventory and preservation of all original failure traces. Final approval for landing still requires complete fixed-SHA make check exit0 and the previously reviewed root preservation gate.

## Follow-up: finite enrollment repair did not pass native repetition

navigation-repair-01 finished19 passes and1 Desktop failure (repeat4). The above static approval is superseded for completion purposes: the finite pre-release-set design is insufficient, and no complete-gate approval follows.

Reviewer read the failed native repeat4 trace. `pw:api@261` records page.waitForResponse at line176 from54052.930 to84056.927ms, ending in its30-second timeout. The two held `route.continue` calls ran at54065.305/54065.996ms; additional calls through the same exact-issue interceptor ran at54129.150/54166.270ms, after enrollment closed. A later call ran at69203.663ms. This supports a post-release replacement outside the set. The native trace has no network records (`0-trace.network` empty) because tracing was attached to the unused Web fixture context; request cancellation statuses cannot be independently recovered from this native zip. Do not label them proven solely from the action trace.

Main selected a bounded logical-refresh test contract: preserve actual Refresh controls, a held renderer-transport barrier and scroll620, then actual selected-issue200/finished, banner disappearance and scroll620/800. Permit immediate post-release replacement requests in the observed recovery set. No timer monkeypatch, additional skip, synthetic response or timeout increase. Strict exclusion of every possible poll was an extra condition introduced during repair, not the original user behavior requirement; remove that unproven claim rather than imply10 seconds alone proves causal origin. A fresh repeated browser run and final review are still required.

## Revised logical-refresh review — 2026-11-08T14:18:18.479515+08:00

**Static verdict: APPROVE** for `e2e/global-scope-navigation.spec.ts` SHA256 `30275c5ae3cd1c923aee6aa4c56b62f886316a116cdb55dda03e0182c6fae6b8`, pending the main-owned repeated browser run and full gate.

The response waiter now begins before the first explicit Refresh and has a10-second budget for the entire recovery window. The route handler only tracks the selected issue after recovery mode begins, and continues enrolling immediate replacement requests after the hold is released. Successful recovery still requires a real200 response belonging to that intercepted request set; no route.fetch/fulfill or synthetic success is introduced.

Two real keyboard Refresh actions and an increased held request count exercise superseding transport. Before release every such request remains held and scroll620 must match. The same completed-response, banner disappearance, restored620, later800, list/filter/workspace/native/compact/Chinese assertions remain. This addresses the failed finite-cutoff assumption while retaining the original explicit-refresh user outcome. It does not claim to distinguish the causal origin of every background invalidation/poll; no timer override or product-source change is used.

Reviewer independently ran scoped git diff --check: pass. No browser/test/build was launched or candidate source edited by reviewer. The19/20 first repair remains a failed run; this revised patch needs its own complete evidence.
