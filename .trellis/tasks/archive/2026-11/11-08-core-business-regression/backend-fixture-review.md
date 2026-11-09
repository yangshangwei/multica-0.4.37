# Backend tagged fixture review

Read-only independent review of rk1_run_test.go, rk1_recording_recovery_test.go and collaboration_knowledge_host_composite_test.go against production Runner/Host recovery paths and backend-result.md. No API/database/provider test ran in this reviewer lane.

## Required correction (implemented; verification in progress)

Both initial fixture recovery helpers used errors.Is on the whole original error, then replaced it with the recovery-only Runner result. Production runHost returns errors.Join(executeErr, stopErr, finishErr); a permanent execution/session/protocol failure joined with transient recording transport/deadline can therefore become nil after metadata recovery. Confirmed with author: no other helper guard prevents this information loss. Existing delayed-outcome and lost-seal regressions only cover pure recoverable errors.

Author is correcting both tagged helpers to admit recovery normalization only when every leaf of the wrapped/joined error tree is a recoverable deadline/ErrTransport. Mixed errors must remain untouched. New pure error-tree red/green evidence and final diff review remain required.

## Other reviewed semantics

- Recovery-only Runner has Ledger and Client, no Host or Controller; it cannot physically launch another provider. It selects the exact run, CanProve=true and Unconfirmed=false, under a bounded10s context while the outer context remains live.
- Existing regressions assert one physical provider, one claim/outcome/proof, original permit replay bytes, no permit after proof, and identical seal bytes after dropped response. These meaningfully distinguish receipt recovery from execution retry.
- Expiry fixture now approves a new5s candidate immediately before the second preparation, requires that exact candidate/version in the successful preparation while still live, then waits for expiry before authorization. It does not mutate immutable candidate rows or extend product timeouts. Negative second-run authorization still calls Runner directly and asserts409, no second physical launch, settled never-launched proof, preserved earlier success and content-free recovery.
- Ordinary failed-provider evidence and recording/result/proof criteria remain separate from RunErr, but that does not justify erasing mixed errors. The correction above remains necessary.

Final classifier source reviewed: `recording_recovery_errors_test.go` recursively inspects both `Unwrap() []error` and `Unwrap() error`; every terminal leaf must be exactly `context.DeadlineExceeded` or `collaborationrun.ErrTransport`. Nil, cancelled, permanent and mixed trees are rejected. Both original recovery callers use this predicate, so mixed original errors remain untouched. No further blocking source issue found.

Author reported deterministic RED for provider+transport, protocol+deadline and nested mixed errors. The complete ten-case classifier plus two physical recovery regressions are running, followed by full tagged/host replay. Final passing evidence still needs attachment; this note approves the source correction, not an unfinished run. The fourth new/modified fixture file must be counted in final source inventory.


Verified terminal `backend/recovery-final/result.json`: race run exit0, 3 top-level /13 named PASS, zero FAIL/SKIP, comprising all10 classifier table cases and both physical receipt-recovery regressions. Log SHA256 `f539d53bbc3c831b49ebc7288d27b543bcafd2fbc4d5f49fa293397e911cc693`. Independent source/regression review is closed without remaining blocking findings. New complete tagged/host runs remain backend-author/root acceptance work.
