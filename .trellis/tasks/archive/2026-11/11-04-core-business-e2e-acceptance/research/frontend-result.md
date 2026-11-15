# Frontend and Mobile verification

Candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`. All checks used isolated checkout `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104`. No product/test source or dependency changes were made by this agent.

## Outcome

- Non-Mobile lint/typecheck: 15/15 tasks passed, no cache hits; UI exports passed. Existing lint warnings: UI 3, views 27, desktop 1.
- Original non-Mobile Turbo run: exit 1, core passed and views had one 5000ms timeout. Docs/Web/Desktop did not execute before Turbo stopped; the next run executed those three packages and all passed.
- Unique non-Mobile inventory executed: **13244 tests in 1069 files**. First package results: **13243 PASS / 1 FAIL**.
- One later unchanged full-file replay: **11 PASS / 0 FAIL**, including the formerly failed identity at 420ms. All 13244 unique identities now have passing evidence, but the original complete run remains FAIL and the stability concern remains open.
- Mobile: typecheck/lint pass (7 existing warnings), **196 PASS / 28 files**, plus three iOS wrapper assertion groups pass. No native UI claim.

| Package | Files | Initial PASS | Initial FAIL |
| --- | ---: | ---: | ---: |
| @multica/core | 331 | 4455 | 0 |
| @multica/views | 593 | 7164 | 1 |
| @multica/docs | 9 | 62 | 0 |
| @multica/web | 34 | 290 | 0 |
| @multica/desktop | 102 | 1272 | 0 |

## Unfixed finding: initial timeout / stability risk

`packages/views/modals/create-project-local-mode.test.tsx:307` — `blocks parallel mode for a folder that is not a git repository` exceeded Vitest's unchanged 5000ms limit; reported duration 5304ms. The stack points to test declaration; no failed business expectation was reported. The test mocks desktop directory selection/validation, performs two user clicks and waits for the directory path, then checks disabled parallel mode and the non-Git hint. It does not invoke Git or a daemon.

Adjacent tests in the first run took 3685ms and 4036ms. CPU/build/test concurrency is a plausible contributor but is **not a proven root cause**. The one fresh full-file run with one worker passed all 11 cases with original assertions, original default timeout and no automatic retries. No product fix has been claimed.

Evidence: `evidence/frontend/03-nonmobile-tests.stdout.log` retains complete initial failure; `09-local-mode-replay.stdout.log` and `.json` retain the sole replay. Do not rewrite the original suite as green or count repeated cases as new coverage.

## Source and execution identity

Exact argv, workdir, start/end, exit code and raw streams are recorded per command under `evidence/frontend/`. `source-identity.json` confirms the failing test, implementation, Vitest config, package manifest, lockfile and Turbo configuration still match HEAD. `final-diff.txt` records only the parent-coordinated production-build rewrite of `apps/web/next-env.d.ts` from `.next/dev/types/routes.d.ts` to `.next/types/routes.d.ts`.

Mobile test command was split into equivalent `vitest run --maxWorkers=2` and `bash scripts/ios-run.test.sh` to preserve the package script's two stages while bounding worker count. Runtime warning logs were retained without treating mocked failure scenarios as product defects.

## Remaining limitations

The timeout remains a stability risk rather than a proven product defect; no new complete non-Mobile suite was rerun. Existing lint warnings were outside the assigned no-source-edit verification scope. Browser, real Provider, native application, backend and database coverage belong to other execution lanes.
