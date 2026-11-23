# Independent review: remaining lifecycle Refresh locators

Reviewed at: 2026-11-08T15:27:38.165545+08:00

**Verdict: APPROVE for the frozen single-file diff against d4af1d6d3.** File `e2e/projects-p1-lifecycle.spec.ts` SHA256 `b154bf9d4550b7876f6a8c8b205536f50b020fc3266daa76f5d86ebc84c550c1`. No concrete remaining finding in this repair. Main owns the currently running lifecycle-repair-01 repeated browser evidence and subsequent complete gate.

Only L09 and the shared L16 Refresh call are changed. Both now use the same section-with-exact-Overview-heading locator as the already-fixed L14. This maps to the production Overview section and selects its actual Refresh button; it neither uses ordinal selection nor forces a click through an unrelated overlay. There are no remaining page-global Refresh calls in this spec.

L16 pairs its original exact-project404 waiter with the click using Promise.all; the response listener is installed before activation and both promises have rejection handlers. The exact workspace_access_denied JSON assertion is retained. Both generated revocation cases still make their real leave/remove operation and require204.

All original behavior gates remain: L09's synthetic503, stale statistics warning, hidden healthy status, disabled risk navigation, explicit Retry and restored navigation; L16's protected goal removal,20-second observation window measured from revocation, per-endpoint<=5, total<=40, later-buckets<=5, and renderer responsiveness<1000ms. No timing threshold, retry, skip, fixture setup or product source changed. L14 and other lifecycle cases remain untouched.

Reviewer independently ran scoped git diff --check: pass. No browser/DB/build/tests were run and no candidate file was edited by the reviewer. Original full04 remains failed with its exact strict-mode collision evidence preserved; approval of this minimal repair does not retroactively pass that run.
