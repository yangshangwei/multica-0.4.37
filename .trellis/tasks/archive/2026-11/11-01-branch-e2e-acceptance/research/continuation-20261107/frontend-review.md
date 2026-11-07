# Independent frontend evidence review

**The reported13320 PASS /4 FAIL across1074 unique package/file pairs is correct.** The three completed Turbo runs cover all five non-Mobile workspaces that define a test script. Core331 files:4460PASS/1FAIL; docs9:62PASS; views598:7236PASS/3FAIL; desktop102:1272PASS; web34:290PASS. Their file rows sum to13324 tests and contain no repeated package/file pair.

Static validation has15 successful tasks,30 lint warnings (views27,UI3) and0 errors. UI export validation passed. Script validation initially passed96 Node tests (84 top-level plus12 children). Review found two changed script suites absent from that command; the coordinator subsequently ran both: `scripts/changelog-workflow.test.mjs` passed5 tests, and `scripts/test-go.test.sh` reported PASS/exit0. Final script evidence is101 Node tests plus the separate shell gate. The shell gate is not invented as another named Node case.

`units-first` was deliberately interrupted before a JSON report was emitted. Its process ledger says complete/exit1, while the separate interruption record explains the control action. It is excluded from completed unit totals and product-failure counts. The two independent reproductions remain14PASS/1FAIL and106PASS/3FAIL; neither is added to the main unit total. Vitest JSON suite/group counters are not treated as file counts.

Both source mappings in `defects-current.md` are supported by frozen code and raw failures:

- QA-20261107-001: `e1312009e2` adds squad chooser/manual/template builders; diagnostic patterns still recognize only the list and detail. Manual/template fall back to masking; chooser matches the generic ID pattern. The missing specificity is proved; no privacy leak or squad-submission failure is inferred.
- QA-20261107-003: `cd95c8e40f` changes the built-in progress-report description to current-project scope. Both locale catalogs retain workspace/project text. Exact-default matching therefore preserves the new Chinese source string and omits default bilingual search descriptions. This matches all three independently reproduced failures.

No additional source defect was discovered in this evidence audit. No product repair or test rerun was performed by the reviewer. The coordinator owns browser results and the final candidate verdict; the four unit failures keep the frontend test gate red. Full unit counts are supported by file/package output; complete individual-assertion JSON exists for the two reproduction files. `frontend-review.json` binds reviewed inputs by SHA256 and records all1074 file identities.
