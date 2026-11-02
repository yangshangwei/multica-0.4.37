# Completion evidence

The optional source parent now uses one searchable picker. Clicking opens recent tasks without entering a title/number; title, bare number and complete displayed identifier searches work. Selection retains the UUID, transient errors retain the form and offer retry, and authoritative denial clears protected choices.

## Verification

- UI implementer: 95 passing focused tests in five files, affected views TypeScript/lint and a final 7-test selector replay.
- Final checker: no confirmed defects, affected views TypeScript/lint, 68 passing Core boundary tests in three files, Go formatting and task whitespace checks.
- Backend: original identifier regressions failed, repaired SQL/service passed 13 guarded service tests on a fresh task-owned database; sqlc generation, go vet and formatting passed.
- Production builds: isolated Next Web and Go API compiled successfully. All 13 task-owned source fingerprints matched the candidate source snapshot before startup; browser API origin, health PID/commit, API binary SHA-256 and Web build ID were verified.
- Production Chromium: final corrected candidate replay passed both English/Chinese cases in 10.2 seconds, with no skipped/flaky/unexpected tests. It verified default browsing, title/bare/full-identifier queries, no matches, reopen/clear, keyboard/Escape, transient error/retry, one explicit creation POST and the persisted parent UUID.
- Four candidate screenshots cover 1280×950 and 390×844 in both locales, without horizontal overflow or page errors. Visual verdict: 96/pass. The mechanical UI detector returned no findings.

The original functional candidate run used the baseline binary's path in one evidence hash field. Its results/captures were retained separately, then the final replay bound the candidate binary correctly. No product code change was needed for that correction.

## Ownership and preserved parallel work

Another session is concurrently auditing creation-source locking and editing the same creation source/test files. This task's UI author confirmed those source-lock imports, mutation observer, mount helper and three tests are not part of this repair. The coordinator pinned this task's 13 exact, already verified source files in a private payload that matches the production browser snapshot, and will stage those versions without changing the live working files. Unrelated task/spec/journal/desktop/E2E edits remain with their owners.

Specs now record the shared picker/recovery behavior and the exact source-prefix search contract. No confirmed defect or product acceptance item remains open. Native installed Electron, physical touch, and the entire monorepo/backend suite were not executed for this scoped repair.
