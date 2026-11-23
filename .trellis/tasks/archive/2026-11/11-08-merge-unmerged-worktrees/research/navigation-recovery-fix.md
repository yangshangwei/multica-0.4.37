# Navigation recovery request synchronization repair

## Confirmed evidence and boundary

Original full03 on00e99892 failed the Web global navigation case while328 passed and112 skipped. The unchanged source repeated20 times in navigation-diagnostic-03 produced17 passes and3 failures. Repeat5 trace identifies the blocked `page.waitForResponse` at original line166: first intercepted exact selected-issue request started30258.751 and has status-1, replacement30275.689 returned200. Both route.continue operations completed; the banner disappeared and later reads succeeded while the singleton Request identity waiter remained pending. This is test synchronization with a superseded request, not evidence of a product authorization/navigation failure.

Only `e2e/global-scope-navigation.spec.ts` changes. Collect the actual selected-issue Request objects intercepted while the recovery barrier is closed. Register a response waiter before releasing it; accept only a200 belonging to that finite held set. Close enrollment before release so arbitrary later polling cannot satisfy the explicit recovery assertion. Retain original HTTP status, response.finished() null, banner disappearance, real refresh activation,620 scroll before/after and subsequent800 restoration assertions. No product query settings, timeouts, retries, skips, dependencies or helper abstraction change.

## Deterministic regression inside the existing journey

The existing real Refresh control is activated a second time while the first read remains held, then the test waits for the held request count to exceed its value before the second activation. This proves a new intercepted read rather than relying on an earlier background replacement. React Query's refetch replaces the pending request; this directly exercises the cancellation/replacement that the original repeat trace exposed. The original first-request-only waiter cannot accept the replacement. Both browser variants retain the same journey and exact issue endpoint, without a duplicate suite or synthetic API response.

Main owns20-repeat Web/Desktop browser verification and final gate. This child does not launch browser tests, build, commit or merge. Candidate edits are frozen after syntax/static verification below; real browser results remain required.


Static verification: TypeScript parse diagnostics empty and `git diff --check` passed. No claim of completed browser regression is made by this child. Candidate frozen for the main session's repeats.


## Follow-up after navigation-repair-01

The first repair produced19 passes and1 Desktop failure. Native trace has no browser network, but its action timeline shows two held `route.continue` calls followed by additional exact-issue intercept continuations64/101ms after release, then15-second polling. The finite membership set therefore still excluded prompt replacements occurring after release. Its30-second native response waiter timed out despite recovered UI. Retain this failed repair as evidence; its prior “finite held set” design above describes the rejected implementation, not the final contract.

Read-only product inspection confirms `query.refetch()` uses cancellation by default, while realtime `invalidateQueries` can independently replace selected-issue reads. Poll interval is15seconds; QueryObserver resets its interval on query updates, but a previous scheduled poll can be near a boundary, so a shorter wait alone does not prove every accepted request originated from the explicit click.

Final minimal synchronization keeps tracking exact selected-issue requests throughout the recovery phase, including replacements arriving after release. The waiter is registered before the first explicit Refresh activation (while the barrier is closed), requires status200 and membership in this route-intercepted set, and has an explicit10-second deadline (stricter than existing native30-second/Web test180-second waits). The release barrier still blocks all successful selected-issue reads until the before-release620 assertion. Two real Refresh activations force pending-request replacement; response completion must be null, banner must disappear and620/800 scroll assertions remain intact. The accepted successful recovery read may be an automatic refetch; this test does not assert transport-level causality exclusive to the click. It asserts recovery after the actual control was activated and transport was deliberately released, rather than the lifetime of a superseded request object.

No clock mocking, production refetch policy, retries, skips or timeout increases. Native tracing is not added to this source change: the parent may use a separately preserved diagnostic runner if further evidence is needed. Only the request-enrollment close and explanatory comments change from repair01, plus the stricter response deadline. Main owns both-platform repeated browser validation.


The final ordering starts the10-second response deadline before the first explicit Refresh, covering both activations and the release instead of permitting an unbounded pre-release delay. The installed TanStack QueryObserver updates/reset its15-second polling interval on query updates, and this journey first drains injected failed routes before refreshing. Under that existing idle-to-fetch transition, the stricter window completes before its next scheduled poll; cancellation/realtime replacement remains permitted. This is the recorded implementation premise, not a universal guarantee that no prior scheduled polling callback or automatic invalidation could ever run. Tests continue to prove actual successful selected-issue recovery and UI/scroll behavior.
