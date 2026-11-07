# Shared v9 result and v10 preparation reconciliation

Verdict: pass-with-notes, zero blocking evidence discrepancies. This verdict covers read-only evidence reconciliation; it does not clear the v10 launcher or complete either task.

The retained v9 whole check failed with make exit 2 on candidate 64a9994ec549e8dff73ab14456dc9a9f4b63f7c7. The actual raw log and structured Playwright report match their retained hashes. Static 15 tasks and TypeScript 13,082 tests passed. All 82 Go package records match their actual JSONL hashes and counts: 20,852 named pass events, zero failures and 22 explicit skips. Nine packages contain no tests; subtests are counted as named events.

The browser scheduled 439 identities with one worker, zero retries and one attempt each: 334 passed, 16 failed and 89 skipped. All 16 original failed identities appear exactly once in the retained directed green results, comprising catalog13, memory1, invitation1 and preferences1. Directed green does not replace the failed whole check.

## Failure attribution

- Thirteen catalog cases are supported as task preparation failures by the retained absent-directory snapshot and real catalog GET 503 observations. The catalog tests themselves were unchanged in their 13/13 directed replay. Missing-session coverage failed during the subsequent market visit; the earlier session semantics are not inferred to be broken.
- Memory failed at the original Purge v2 selector while history was collapsed. The reviewed two-line test addition opens Version history and checks Restore remains absent before preserving every original purge/API assertion. The cause of that earlier collapsed state remains separately unknown.
- Preferences failed after its original priority item became unstable and detached. Three test readiness statements wait for the natural dialog/title focus and visible menu; no forced focus/click, sleep, retry or product change was introduced. The precise earlier focus race is not proven solely by the directed pass.
- Invitation failed because its expected issues redirect had not occurred within 5 seconds. Its unmodified directed scenario passed. The original cause remains unknown and must stay unknown until evidence supports a cause.

## Knowledge and skips

All three canonical L7 termination regressions passed in the retained actual Go logs. This preserves L7 as not reproduced and already test-covered, rather than an invented product repair. The knowledge browser file passed seven Web scenarios and one native Desktop scenario. The deployed startup OFF/ON probe was explicitly skipped for missing COLLABORATION_KNOWLEDGE_EXPECTED; this run cannot satisfy CCG2. The result JSON records all 22 Go skip names/reasons and all 89 browser skips by file.

## V10 observation

The clean repair candidate is 118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9. Its committed server/apps/packages trees equal v9 exactly; only the two owned E2E test files change in that source scope. The actual v10 launcher, data-only environment and unchanged Go/pnpm adapters match preparation hashes. Syntax parsing passed without execution.

The preparation reported 32/32 fake preflight passes; those were not rerun by this reviewer. At observation, independent clearance and make-check-v10 JSON/log did not exist. Owner work remains: independent executable review/clearance, staged renderer/preload installation, live catalog/API/database/renderer identity and exclusive window, followed by one new complete check. The inherited Playwright -v9 basenames are under the distinct final-check-v10 namespace and do not reuse the old files.

## Verification and scope

Fresh verification was file-based JSON parsing, 82 actual Go record/log hash and count comparisons, raw browser inventory and directed identity matching, exact original-source preservation for both test additions, selected v10 hash/product-tree checks and nonexecuting Python syntax parsing. Repository lint/typecheck/tests were not rerun because no source was changed and this reviewer had read-only resource authority. No database, service, Provider, account, commit or staging action ran.

[Machine-readable v9 result](shared-check-v9-result.json) · [Review JSON](shared-verification-review.json)
