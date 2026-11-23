# Full-check-04 P1 lifecycle diagnosis

Status: read-only, awaiting final stack; 2026-11-08T15:18:53.067129+08:00. Candidate d4af1d6d396176e086c0c8dfdfac66663003b756 remains frozen during full04.

Item234, the owner-removes-member P1-L16 case, failed in1.7 seconds. L14 and the preceding member-leaves L16 case passed. This timing suggests an immediate exception rather than the20-second traffic-bound observation. The final failure stack has not yet printed; no root cause is claimed solely from duration.

The same spec still contains the exact known ambiguous locator from failed full02 L14:

- L16 line344: page-global exact Refresh. Both leave/removal variants execute this same source line.
- L09 line187: another page-global exact Refresh for the same Overview control.
- L14 line302: already scopes Refresh to the section containing exact Overview heading.

Thus the narrow likely cause is the conditional sidebar Refresh colliding with the Overview Refresh again. The1.7-second failure is consistent with that earlier confirmed strict-mode error. Available failure.png is subject to the previously recorded shared p1Failure filename overwrite; it is not sufficient to prove current page state. No error-context or trace exists for this failure at this reading.

After final stack confirmation and full-run completion, use one small overviewRefresh(page) locator helper for all three Overview Refresh sites in this spec, including the already-fixed L14 site. Bind wait/click with Promise.all in L16 as in L14 to prevent a later context close from producing a second orphaned waiter error. Keep204 revocation, exact overview404/body, protected-content removal, original20-second request counts (<=40 total, <=5 after first bucket, per-endpoint limits), and renderer responsiveness assertions unchanged. Do not increase timeouts/retries or change product behavior.

L08/L09 also use page-global Retry at178/192. These belong to their specific existing load/refresh error alerts; they are not the exact Refresh collision and have no current failure evidence. If the mechanical cleanup includes them, scope to the already-asserted corresponding alert text rather than a broad page or ordinal. No source edits/tests/builds have been performed in this diagnosis.

## Final stack confirms root cause

Full04 ended exit2,328 passed/112 skipped/1 failed. Original final stack explicitly reports L16 line344 strict-mode violation: exact Refresh matched both content and sidebar buttons. Its response waiter at343 then rejected when the finally block closed the context. The intended overview refresh never executed, so the original failure did not reach the20-second traffic or responsiveness assertions. This is the same locator defect as full02 L14, left at a second call site; no new product failure is established.

All owned renderer/Web/API services stopped, and the isolated Go database was dropped by the normal cleanup. Preserve this failed run; main owns the narrow same-spec repair and repeated original lifecycle verification. No patch or test was executed by this reviewer.
