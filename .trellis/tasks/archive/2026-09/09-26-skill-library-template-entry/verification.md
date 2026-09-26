# Skill template discovery verification

Status: completed and locally integrated. Implementation branch: `work/skill-library-template-entry`; integration target: `plan/skill-library-template-entry`.

## Implementation and corrections

- Replaced the duplicated inline catalog with one compact template entry and the existing template picker.
- Kept source counts, related workspace skills and localized summaries as derived presentation data; full descriptions, instructions and supporting files remain the inputs to independent copies.
- Stabilized the creation dialog across cached list failures and retries, and guarded every Desktop tab intent with an immutable workspace/path/title/intent snapshot.
- Restored focus to the live entry button when the original page body was replaced by a query transition.
- Fixed wrapped Japanese/Korean source labels by overriding the shared orientation-specific height locally; narrow footer help now uses its own row.
- The final browser run verified the explicit focus handoff before removing the editor's focused control. Base UI otherwise schedules a later popup focus that can steal focus from a related link before Enter.

## Acceptance evidence map

| Criteria | Canonical evidence |
| --- | --- |
| AC1, compact management-first entry | `skills-page.test.tsx`; production entry height and page-overflow assertions; wide and narrow screenshots |
| AC2, truthful collection/source counts | `skill-template-discovery.test.ts`, page/picker query tests; real embedded catalog count and unchanged workspace count assertions |
| AC3, direct browse and retained creation chooser | page/dialog tests; real direct and chooser entry paths with whole-session write observation |
| AC4, explicit adoption and independent creation | real `multica-agent-evaluation` template with a nonempty `references/evaluation-source-map.md`; saved body/frontmatter/files and exactly-one-create assertions |
| AC5, every related workspace instance | pure identity/source matrix, picker tests, real two-copy named links and excluded manual same-name skill |
| AC6, display-only summaries | presentation/source-sync and locale-parity suites; original catalog and existing workspace record equality after creation |
| AC7, guarded navigation | root flow tests for push/tab intent, busy/checking, stale context, unconfirmed submissions and actual existing-tab unmount; final browser dirty-link cancel/confirm sequence |
| AC8, query lifetime and workspace isolation | real QueryClient/page/dialog suite for edited draft, unknown result and pending discard through cached failure/retry; retained workspace/late-response tests |
| AC9, keyboard, focus and narrow geometry | real source/row/back/adopt/close/discard keyboard steps; 1280×720, 375×667 and 360×800 PNGs; visible text bounds and contrast samples |
| AC10, shared boundaries and four locales | locale parity/source-sync, isolated locale rendering, Web/Desktop consumer typechecks, EN/JA light and ZH/KO dark browser captures |

## Checks already collected

- Final views integration: 451 tests across 9 files passed (`views-final-03.log`).
- After the final focus-race repair: 86 affected tests across 4 files passed; views typecheck and changed-file lint passed.
- Views lint: exit 0, with 26 warnings in unchanged files outside this task.
- Web/Desktop consumer typechecks: 7 tasks successful on the final source (3 dependency tasks cached; consumer-typecheck-final-03.log).
- Strict E2E TypeScript, ESLint and test discovery passed.
- Production Web build and task-owned API startup passed; command, process, commit, source fingerprint and Web build ID are saved in the evidence directory.
- Visual review after layout repair: 95/100. The 123 measured light/dark text samples had a minimum contrast ratio of 5.38:1, with zero failures.
- Final complete production E2E: 7 passed, 0 failed, 0 skipped, 0 flaky in 37.1 seconds; 57 screenshots captured. The original focus/Enter sequence passed without added waits (`playwright-03.json`).

## Test environment and reproducibility

Evidence directory: `/Volumes/artisan/code/2026/multica-skill-library-template-entry/.omx/reports/skill-library-template-entry/`.

The `skills-entry-qa` environment owns API port 18220, Web port 13140 and database `multica_multica_skill_library_template_entry_140`. Tests use classic development-code authentication and disposable per-test workspaces; catalog/create calls are real. The Web server uses `next build --webpack` followed by `next start`, not development compilation. Fixture workspaces are deleted in cleanup; the environment database is retained.

Commands:

```bash
pnpm --filter @multica/views exec vitest run skills/lib/skill-template-discovery.test.ts skills/lib/skill-presentation.test.ts locales/parity.test.ts skills/components/skills-page.test.tsx skills/components/skills-page-template-session.test.tsx skills/components/create-skill-dialog.test.tsx skills/components/create-skill-template-flow.test.tsx skills/components/template-skill-create-panel.test.tsx navigation/app-link.test.tsx --maxWorkers=2
pnpm --filter @multica/views lint
pnpm --filter @multica/views typecheck
pnpm exec turbo run typecheck --filter=@multica/web --filter=@multica/desktop --concurrency=1
PATH="/opt/homebrew/opt/libpq/bin:$PATH" make up C=api,web ARGS="--web-mode production --ttl 12"
PATH="/opt/homebrew/opt/libpq/bin:$PATH" make status ARGS="--json" MULTICA_ARGS="--web-mode production --ttl 12"
make env-exec ARGS="-- pnpm exec playwright test e2e/skill-template-creation.spec.ts e2e/localized-template-defaults.spec.ts e2e/skill-category-taxonomy.spec.ts --project=chromium --workers=1 --retries=0"
```

The Makefile exports `MULTICA_ARGS` from `ARGS`, and the API configuration fingerprint includes it. A status call with a different `ARGS` value can therefore report a configuration mismatch despite an unchanged, correctly owned listener. Preserving the startup `MULTICA_ARGS` value makes the health, process ancestry, configuration and source checks agree; the launcher was not modified or bypassed.

Tab-fit assertions measure visible text rectangles. The shared, invisible `::after` indicator extends 5 px below a tab and contributes to `scrollHeight`; counting that as clipped text is incorrect. A controlled browser diagnostic confirmed that the new measurement accepts the invisible decoration and still rejects deliberately clipped labels.

## Remaining verification limits

- No packaged Electron/native-window smoke test has been performed. Desktop coverage uses the real shared navigation boundary, a fake adapter that activates an existing tab and unmounts the source, and both Desktop TypeScript configurations.
- Backend code, API contracts and dependencies were unchanged; Go suites and release packaging are outside this frontend verification.
- Final application-source hashes are recorded in `verified-source-files.json` so local commits and integration can be checked against the exact files exercised by the passing run.

## Final production identity

- Build: `39o5vy-prJGxRUEuG1-Ps`.
- API/Web source fingerprint: `0d7207a9a1180a39c668d338865c58698b441496be3cc5207d548f03f849220f`.
- Build base commit: `c53831eda`, with the task diff and new source files included in the fingerprint.
- Checked at: 2026-09-27T00:15:28+08:00.
- Test services stopped successfully; database and evidence retained.

## Local integration

Feature commit: `4217c383b7861deac809434e7b6d62d54c7d6124`. It was fast-forwarded into `plan/skill-library-template-entry`. All 20 verified feature source paths match the tested files, and hashes of the 10 preexisting unrelated working-tree files are unchanged.

Documentation checks: 22 local Markdown links and all 12 context-manifest references resolve. The archival commit trailer accidentally reported 24 links; the measured result is 22.
