# Block R02 fixture/source independent diagnosis

**NOT ACCEPTED.** Service normal/race each has16 tops/43 functional named PASS, but five handoff rows remain in the actual scoped audit. Handler normal has2PASS/1FAIL across3 tops; its failing history call returns400 invalid issue_id. Full profile and server acceptance must wait for a newly numbered source/fresh database check. This review executed no DB/API/TestMain and changed no product/test or historical evidence.

## Findings (not fixed)

- **BLOCK-QA-01:** the new pending-information tests use `newStageCompletionFixture`, whose chain selects `newStageRuntimeFixture`/managed root and does not register issue_handoff cleanup. The simpler stage/handoff fixture does register it, which cannot be assumed for this different owning chain. The actual five retained rows line up with the two pending tops and their exact timestamps/notes/purposes: acceptance+information+repair transfer for the preserved result; acceptance+information for the unavailable creator. Parent issues/users/workspaces being absent is not proof of dependent cleanup. The single implementation owner should register parameterized checked cleanup for the exact new-test issue before any pending submission effects, keeping prior failed DB/history as retained evidence. No shared fixture deletion, FK/cascade or production retention change is justified.
- **BLOCK-QA-02:** the new handler read fixture calls `withURLParam` twice. That helper builds a new Chi context on every call; adding handoffID replaces and loses id. Router and `informationReadRequest` already agree on id. Construct the test request with existing `testutil.WithURLParams(req,"id",issue,"handoffID",handoff)` once, retaining all200/400/404 body/privacy assertions. Do not rename product route parameters to fit the bad test request.

## Findings (fixed)

None by this reviewer. Main owns dispatch of fresh R03 source and exact package/mode databases. All R01/R02 raw/source/count evidence remains immutable.

## Verification

- Actual selected source/cleanup/request constructors and retained raw failures inspected; hashes in sibling JSON.
- Lint/typecheck/tests: no reviewer execution; the supplied R02 compile/pure/vet pass is separate from failed full DB acceptance.
- No product bug is established by these two fixture construction/cleanup failures. Full S3 remains in progress.
