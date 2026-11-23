# Batched closure participation preparation review

Independent source review of the second performance repair; runtime regression and original performance gates are pending. No API/database/browser/test execution by this reviewer.

The new PrepareMembershipChanges helper requires the same caller-owned complete iteration/issue/reference/status-catalog locks. Service prepareClosure gathers the same membership inputs in existing p.issues lock order and keeps Reason/Rollover aligned. It performs preparation before any event/pointer/snapshot write and does not change transaction isolation, retries, clock sampling or authority.

The SQL batch consists of the original individual SELECT ... WHERE workspace_id/iteration_id/issue_id FOR UPDATE statements, queued in issue order with source before target. It does not merge them into a pre-wait snapshot. All results are consumed and the batch is closed before later fact/reentry queries. The result lookup key includes workspace, iteration and issue.

Empty input returns without a batch. One input delegates to the original public helper, whose query/validation path is retained by a private lock-lookup parameter. Multi-input duplicates are rejected rather than deduplicating/changing alignment. No-op membership inputs do not request participation locks, matching the original early return. Multi-input validation may acquire later locks before discovering an invalid item, but cannot write, and caller authorization is already held; this behavior is explicitly documented.

Missing source participation remains an error. Missing target participation remains allowed; historical target rows remain available for reentry rules. CurrentJoinedAt checks still reject a source already left or a target already current. HasStartedCurrentParticipation, InOriginal and HasIterationActiveJoin semantics are unchanged. Custom status reads happen after the batch only under the already-held catalog fence, so status authority cannot drift during preparation.

Initial source review found no blocker. Required evidence before final acceptance: single/batch SQL equivalence, empty/single/duplicate/missing-source/target/soft-left/current-target cases with aligned output and rollback/no-write assertions; actual all-iteration race matrix; original three performance tests with unchanged thresholds. Author is adding those regressions.


Final edge-test source review: canonical SQL normalization compares single and batch FOR UPDATE statements; actual DB cases cover absent target/previously-left target, already-current target, absent source and inactive source. Every case reads issue pointers in the same transaction to assert preparation performed no writes, and successful results retain input count/alignment. Empty and duplicate input cases use nil transaction to prove no SQL path. The real service disable test asserts0sequential participation reads/1batch and all membership pointers cleared after committed apply. These focused fixtures own isolated rows; they do not replace existing service lock/concurrency tests. No additional blocking source findings.


## Final frozen evidence

Verified participation-final/result.json:68top-level/100named racePASS,0FAIL/SKIP,exit0, logSHA98eaf444b8b525de39bdec3a1fac14401144d144b75a2a2049bc2e6c2891bdc8. Verified all8runtime files and the new regression match product-final-hashes.json. Verified performance3 log hash588bddc3cf113cb9270cc1450a578b6ad21ee4da3552239be0e231aa9d4c6807 and terminal3PASS. ClosureP95=1662.866ms<2000; disableApplyP95=1937.854ms<3000; triage list/filter/action P95=133.460/64.665/13.987ms, each<2000. Original thresholds and existing5warmup/30measured I1 sample policy were unchanged.

Independent source, semantic and targeted-performance review is approved for this exact freeze. Backend author owns final complete service/handler race; root owns rebuilt runtime/package acceptance. Performance pass is sample/environment-specific and does not turn earlier failed runs into passes. Author is documenting the batch contract in server/iterations.md; this reviewer did not duplicate that edit.
