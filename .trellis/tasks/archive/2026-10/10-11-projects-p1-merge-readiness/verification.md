# Full verification — 2026-10-12

One complete `make check` passed, exit **0**, in **2073.63 seconds** (34m34s).
Run: **2026-10-12 07:02:14–07:36:48 Asia/Shanghai**.

```sh
PATH=/opt/homebrew/opt/libpq/bin:$PATH ENV_FILE=/Users/artisan/.multica/dev/envs/check-20261011123514-33760/check.env make check MULTICA_ARGS=
```

Executed from `/Volumes/artisan/code/2026/multica-projects-p1-merge-readiness` at
`6a892c3d021282632a6efa1efed5f2943dc0b111` with the pending editor fix, its tests,
and two spec updates. Initial/final SHA-256 manifests prove all four intentional
files and HEAD stayed unchanged throughout the run. No retries or second full run.

| Stage | Actual result |
| --- | --- |
| Static | `pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force`: 15/15 tasks; `pnpm check:ui-exports` passed |
| TypeScript | Forced, package-serial tests with two workers: 5 packages, 913 files, 10,970 tests passed |
| Script / Go | `test-go.test.sh`, `dev-env.test.sh`, `check.test.sh` passed; protected `test-go.sh --race`: 70 successful packages (28 executed, 42 cached); `go vet -p 2 ./...` passed |
| API | Isolated API healthy at `http://localhost:18956`, listener PID 70349, matching commit/source |
| Production Web | Production build and `next start --port 13876` passed; build ID `cnG18aHP3_FmkVOUa59d5` |
| Playwright | `pnpm exec playwright test --workers=1 --retries=0`: **258 passed / 70 skipped / 0 failed**, 11.9m |

The unchanged P1-L14 (`projects-p1-lifecycle.spec.ts:286`, 2.0s) and P1-P05
(`projects-p1-progress.spec.ts:105`, 1.6s) both passed with the real editor.
`content-editor.test.tsx` passed all 38 tests in the full TS run.

## Environment and retained evidence

New environment: `check-20261011230215-23632`. API and Web share source fingerprint
`96e61f96aabb2482379ee5f0443361ad5d9b1697f386fb2b0fc9516308cad54d`.
`make check` stopped its API/Web and dropped the isolated Go database successfully;
afterward neither allocated port had a listener. Its API database and environment
record remain retained by the supported workflow.

Local, ignored reports are under
`.omx/reports/projects-p1-merge-readiness/2026-10-12/`:

- `full-check.log`, `full-check.metadata.json`, `full-check.summary.json`.
- `verified-snapshot.initial.json`, `verified-snapshot.final.json`, `initial-tracked.diff`.
- `full-check-skipped-tests.json`: all 70 exact skipped identities.
- `full-check-provenance/check-20261011230215-23632/`: preserved
  `api.running.json`, `web.running.json`, `verification.running.json`; originals
  also remain under `/Users/artisan/.multica/dev/envs/check-20261011230215-23632/`.

The only generated tracked change is the expected `apps/web/next-env.d.ts` import
rewrite from `.next/dev/types/routes.d.ts` to `.next/types/routes.d.ts`, left for
the parent to inspect. This verification lane changed no implementation or specs.

## Conditional coverage limits

The 70 skips are recorded as skips, not passes:

| Required opt-in fixture | Skipped |
| --- | ---: |
| Isolated I1 Web/Electron fixtures | 19 |
| Password authentication / migration / password-mode MCP fixtures | 19 |
| Platform administration, initialization and managed resource publishing | 10 |
| Deterministic AI/refinement/recommendation providers | 11 |
| Changelog Web/Desktop feed fixtures | 2 |
| Other separately configured desktop renderer fixtures | 4 |
| Explicit task-owned CLI binary | 3 |
| Device authentication | 1 |
| Task-owned MCP template directory | 1 |

Mobile lint/types/tests are explicitly excluded by this command. Default Go output
summarizes packages, not individual skips; real-agent `agentintegration` smoke
was not enabled. This run does not establish CI or unexecuted platform acceptance.
The parent owns focused committed-candidate checks and PR/main integration.


## Additional acceptance: I1 opt-in on the committed candidate

On **2026-10-12 07:55:09 Asia/Shanghai**, the seven explicitly selected I1 specs
ran once against committed candidate `0d1c9ca58f356a786346292c3c7bdbe3bce650e5`.
Result: **19 passed / 0 failed / 0 skipped / 0 flaky**, one worker, zero retries;
Playwright duration **107.7s**, command duration **110.26s**, exit **0**.

The exact command and process timestamps are in
`.omx/reports/projects-p1-merge-readiness/2026-10-12/i1-opt-in-final/run.metadata.json`.
It uses the supported `make env-exec` entry point with
`MULTICA_RUN_I1_E2E=1` and `PLAYWRIGHT_JSON_OUTPUT_FILE` placed after `env-exec`
loads the environment, and `--workers=1 --retries=0 --reporter=line,json`.

| Selected spec | Passed |
| --- | ---: |
| `e2e/iteration-scope-business-desktop.spec.ts` | 2 |
| `e2e/iteration-scope-business.spec.ts` | 3 |
| `e2e/iteration-settings.spec.ts` | 4 |
| `e2e/iterations-audit-desktop.spec.ts` | 3 |
| `e2e/iterations-i1-desktop.spec.ts` | 3 |
| `e2e/iterations-i1-history.spec.ts` | 1 |
| `e2e/iterations-i1.spec.ts` | 3 |

These **19 identities match all 19 I1 skips in the original full run**. There
are **0 additional tests overlapping its 258 passed identities** and no unmatched
cases. Identity mapping uses file/full title/project because an in-test skip
reports its skip statement location rather than the passing declaration line.
The original **258 passed / 70 skipped / 0 failed** result above remains unchanged;
this is separate supplemental coverage on the later committed candidate.

The remaining **51 original conditional skips** still require separate fixtures:
password/migration/password-mode MCP (19), platform administration/initialization/
managed publishing (10), deterministic AI providers (11), changelog Web/Desktop (2),
other dedicated Desktop renderer fixtures (4), explicit CLI binary (3), device
authentication (1), and a task-owned MCP template directory (1).

Environment `check-20261011123514-33760` passed identity checks before and after:
API `localhost:18951` / PID 77399; production Web `localhost:13871` / PID 81139,
build `ryOc6YqV-OUNGZ5bo-IVU`; source fingerprint
`213bb9278cbdc9c0074e17b05c55dcfe048af9c04de503dbacb5289f0d14bfe4`.
HEAD and the tracked diff remained unchanged. The parent-owned API/Web remain
running; this lane did not restart, rebuild, stop or modify either service.

All supplemental evidence is under the same ignored `i1-opt-in-final/` directory:
`run.log`, `report.json`, `run.metadata.json`, `coverage-mapping.json`,
`api.running.json`, `web.running.json`, `verification.running.json`,
`verification.after.json`, and browser `artifacts/`. No source or spec edits.

## Committed source and integration

The four verified files were committed without content changes as `c9229c4a3`
(editor implementation, regressions and draft contract) and `8ca432f0c`
(default-enabled settings guidance). Commit `0d1c9ca58` incorporated concurrent
main corrections; its only additional file change was the Compose overlay test.
`node --test scripts/offline-changelog.test.mjs` passed 33 tests with 3 explicit
environment-dependent skips. The bitmap gate against updated main passed.

Both original draft cases then passed 10/10 on the committed production build,
with zero retries (see `research/p1-editor-draft-recovery.md`). The dedicated I1
run above covered 19 additional identities, so 277 distinct browser cases passed
and 51 original conditional cases remain outside these configured environments.

[PR #3](https://github.com/yangshangwei/multica-0.4.37/pull/3) passed all 16 checks
and merged at `2026-10-12T00:26:40Z` as
`327a539481f4f9c004388e2d230d038877f7ad02`. The merge preserves history, contains
the tested candidate, and has an identical file tree. Local main was safely
fast-forwarded to that commit. Both the main CI and Mobile Verify runs passed;
immutable run links and source checks are in `evidence/integration-result.json`.
The image gate ran on the PR; its main-push job is skipped by workflow design.

All task-owned API/Web processes have been stopped. The supported environment
workflow retains API databases and logs for TTL-based inspection. Bookkeeping
records are retained on `codex/projects-p1-merge-readiness`; product changes are
on main. No production deployment or release tag was part of this task.

SSH port 22 and HTTPS transport briefly failed during local synchronization.
A per-command SSH port 443 override, with `HostKeyAlias=github.com` and strict
verification of the existing host key, succeeded. No persistent remote or SSH
configuration was changed.
