# Project lifecycle Refresh locator completion

Full-check-04 ended15:25 with exit2:328 E2E passed,112 skipped,1 failed. The owner-removes-member L16 case failed strict locator resolution because a global Refresh button locator matched both the intended Overview refresh and a sidebar refresh. This is the same locator ambiguity already corrected in L14; the remaining source sites were L09 and L16's two parameterized revocation cases.

Boundary: change only integration `e2e/projects-p1-lifecycle.spec.ts`. Restrict both remaining Refresh locators to the section containing the exact Overview heading, matching L14's established selector. Pair L16's exact overview404 response waiter and the scoped click in Promise.all, then retain its workspace_access_denied body assertion. No helper extraction, production change, timeout/retry/skip adjustment or source-root edit.

All original L09 injected503/stale-statistics/risk-disable/Retry-recovery assertions remain. L16 retains the20-second observation window, per-endpoint/per5-second/total traffic limits, content clearing and subsecond renderer responsiveness assertion. Both member-leaves and owner-removes variants use the corrected path.

Static verification: TypeScript parser reports no syntax diagnostics; `git diff --check` passes. Main owns complete lifecycle and revocation-repeat browser validation, fixed commit and full gate. Candidate frozen after this two-site edit; this implementer ran no browser/build/fullgate and made no commit.
