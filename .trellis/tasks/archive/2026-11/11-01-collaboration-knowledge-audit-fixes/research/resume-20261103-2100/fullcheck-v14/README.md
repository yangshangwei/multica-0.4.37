# V14 complete check — failed

The single frozen run on `d0b132fff0c477eb2d744933e13db730240231aa` ran from **2026-11-03 14:14:24 to 15:22:18 UTC**. Make and launcher exited **2**. The registered final identity auditor exited **0 / pass**; that confirms frozen identities, not acceptance. No automatic rerun, source/config edit, CCG execution or paid Provider call occurred.

Actual results:

- Static checks: 15 passed. TypeScript: 13,244 passed across 1,069 named files.
- Go: 82 packages on 82 distinct recorded databases, all race; 20,963 named passes, 0 failures, 22 explicit skips. Nine packages compile without tests.
- Playwright: 441 cases; 334 passed, 16 failed, 91 explicitly skipped, 0 unrun, 0 retries. Seven Web and one native Desktop knowledge case passed.

Failure grouping: 13 cases directly reject the non-UUID deployment ID `collaboration-closure-v14-01a1a186` in PostgreSQL. Two preparation-record cases reject the same field through production protocol validation before sealing. The sixteenth case separately times out waiting for “Remove parent issue”; its root cause still needs retained-trace diagnosis.

The prelaunch review validated frozen identities and parser/Make safety but missed the deployment-ID UUID contract. Keep that original review and this failure; correct a newly numbered environment, add the missing semantic preflight control, and review the changed bytes. Do not modify v14 or weaken production/test assertions.

Complete inventories and actual hashes: `result-summary.json`, `go-inventory.json`, `typescript-inventory.json`, `playwright-inventory.json`, `browser-failures.json`, `skip-review.json`, and `verification.json`. All 22 Go and 91 browser skips match the prior explicit identities; none is credited as a pass. The deployed knowledge skip still requires separate OFF/ON execution.

All 803 current browser artifacts were copied into `.omx/private/final-check-v14/retained-test-results` and verified before any future default output reuse; `artifact-retention.json` records them. The earlier v13 retained 878 files also verified unchanged. Original directed failures remain in their own private result paths.

Task #7 remains open. After configuration repair and independent menu diagnosis, a new complete gate and current CCG observations are required. Root owns live API/Web/database identity evidence and final task disposition. Knowledge budget/calls remain 0/0 and production knowledge remains off.
