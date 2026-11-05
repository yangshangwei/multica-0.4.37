# Independent plan review: M1 critical-path PR smoke

Reviewer: `trellis-check` / `ci_plan_review`
Reviewed baseline: `5aef9c19ad3e764d09cff939edfded53f8c513bd` plus current task artifacts.
Scope: design and existing source only; no application, database, browser or model started.

## Verdict

**Clear to implement the bounded four-path smoke.** The coordinator incorporated the two concrete launcher/environment findings below in design.md under “独立审查修订”. No further product decision is required. The implementation contract below is the minimum needed to make the existing isolation functions usable without introducing another deployment framework.

## Findings (resolved in the design)

1. **Do not call the broad environment entry point for an already allocated database.** `scripts/dev-env.sh:1311` (`cmd_up`) can adopt another environment, rewrite the database/ports at `:1412`, and calls `ensure_database` (`:412`), whose failure branch starts the shared container. This conflicts with exact task/runner resource ownership. The accepted revision sources the script to reuse allocation, manifest, health and component functions, and directly verifies the preallocated database before `migrate_database`, `start_api` and `start_web`. It excludes `cmd_up`, `ensure_database`, automatic GC and destroy. The launcher retains the database; the allocating owner decides its later cleanup.

2. **A new Playwright config does not isolate fixture environment loading.** `e2e/fixtures.ts:7` independently imports `e2e/env.ts`, which probes `.env.worktree` and `.env`. The API client also has a legacy database fallback when the variable is absent. The accepted revision rebuilds a narrow environment and rejects ambient root dotenv files and non-example Web dotenv files before any migration/build/fixture. It supplies the complete explicit database and loopback URL set to every child. No fixture code changes are necessary for this slice.

No product files were changed by this review.

## Concrete implementation handoff

### Environment and lifecycle

- Accept an explicit private configuration/allocation input with expected database name and exact URL; validate URL scheme, loopback host, nonempty database, declared ownership and `SELECT current_database()` equality before any write. An inherited URL is not the expected identity. Refuse missing `psql`, database failure, port conflict or ownership mismatch; never fall back to defaults or shared services.
- Start with a new private run directory, private `MULTICA_DEV_HOME`, private home/state/upload/tmp paths, and an explicit browser installation/cache path. Reuse the existing registry lock and `allocate_offset`; set `NAME`, `DIR`, `OWNER`, lifecycle metadata, `OFFSET`, `BACKEND_PORT`, `FRONTEND_PORT`, `DB_NAME`, `PROFILE` and `WEB_MODE=production` before `bind_paths`/`save_manifest`. Preserve the allocated DB when selecting ports. Refuse reusing a prior run directory or report file.
- Do not source caller-provided dotenv as shell code. Parse the small supported input as data, validate it, then generate a private canonical file using the repository's quoting rules. `load_env_file` (`dev-env.sh:327`) sources its input; it is appropriate only for the launcher's own generated file. Avoid passing it through Make's different include grammar.
- Build the child environment from known inputs. Supply synthetic `MULTICA_AUTH_MODE=legacy`, development verification code, `APP_ENV=development`, explicit local origins/HTTP/WebSocket URLs, local uploads, bounded API pool, and disabled cloud/device/managed/platform-admin/external integrations. Exclude ambient credentials, publishers and account configuration. The Web build remains production despite the synthetic API auth mode.
- Register cleanup **before** either start call; set the owned-start flag before invoking a start function so a partially failed startup is still cleaned up. Call only `stop_component web` and `stop_component api` for this run. Cleanup failure makes the final result fail. Do not use the broad `down`/`destroy` component loop, which includes daemon/desktop handling.
- `start_api` verifies PID/process group/commit/config identity (`:579`); `start_web` builds with `pnpm --filter @multica/web build`, checks source/config stability, and runs `next start` (`:772`). Reuse those functions. Verify the rendered Web API/WS endpoints as well as health before fixture writes. Only API and Web start; the selected fixtures themselves use `SimulatedDaemon`, never a provider CLI.
- Retain `api.running.json`, `web.running.json`, production BUILD_ID, actual source/config identities, migration/API/Web logs and a sanitized allocation summary. Registry manifests and env files contain DATABASE_URL and **must not** be included in CI artifacts. Upload an explicit allowlist of evidence files, not the whole private state directory.

### Fixed test and evidence contract

The single Chromium project must execute exactly these source/title pairs:

| File | Leaf test title |
| --- | --- |
| `e2e/core-business-journeys.spec.ts` | `G1-01 assigning an agent queues one task that the daemon runs to a live completed reply` |
| `e2e/core-business-journeys.spec.ts` | `G1-02 a failed run notifies the creator, retries exactly once and can be stopped before it runs` |
| `e2e/core-business-agent-loops.spec.ts` | `G6 a plain comment re-engages the assignee, an @mention engages another agent and /note engages none` |
| `e2e/core-business-agent-loops.spec.ts` | `G8 a running task streams its transcript, leaves the issue status alone and stays stopped after a late completion` |

- G1 leaf tests live under `G1 agent execution loop`; selection must account for Playwright's suite-qualified grep text, while the inventory validates actual file/project/title identity. Do not use a loose `G1|G6|G8` grep that can silently add cases.
- Parse the installed Playwright JSON format and reject malformed structure, global errors, unexpected/duplicate/missing test identity, non-passed expected status, non-expected aggregate status, multiple results, retry greater than zero, and any non-passed result. Four summary successes or process exit zero alone are insufficient. If a chosen test becomes skipped or expected-to-fail, the gate must fail.
- Preserve both Playwright exit and inventory result. A nonzero test process cannot be rescued by a partial four-pass report, and a zero exit cannot rescue missing tests. Always retain the first report/log; do not retry to green inside this launcher.
- Capture source/config identity again after the run and require it to match startup/build. The existing source fingerprint includes tracked task/docs diffs, so coordinate a brief write freeze for all covered files; evidence written outside Git does not perturb it. No changes to fingerprint exclusions belong in this child.
- Launcher tests should prove wrong/missing DB stops before migration/build/fixture, busy unowned listeners are untouched, partial startup cleans owned processes, failure evidence survives, and teardown failure stays nonzero. Inventory tests should independently cover wrong file/project/title, extra/duplicate/missing cases, skip, expected failure, failed/timeout/interrupted result, retry and malformed/empty report. Fake processes prove the launcher contract, not browser behavior.

### Workflow

Use `pull_request` for `main` and `codex/cross-workspace-collaboration`, with read-only contents permissions and no repository secrets or `pull_request_target`. Include both business paths and smoke/config/fixture/lockfile/development-launcher paths in the trigger policy; changes to the smoke itself must run it. Install locked repository dependencies and the corresponding Chromium system dependencies, provision the runner-owned PG17/pgvector service, and pass only synthetic values to the launcher. Preserve artifacts with an `always()` step and fail on missing required report/evidence. A new named status is not proof that remote branch protection requires it.

## Verification and remaining boundaries

- Lint: not run; plan-only review, no source edits.
- TypeCheck/build: not run; no implementation exists for this child yet.
- Tests: not run; no database/browser/service commands executed.
- Read-only evidence: task PRD/design/implement/manifests, repository conventions, DB/E2E isolation specs, existing `dev-env.sh` allocation/start/stop/provenance functions, `check.sh`, Playwright config/environment/fixtures, all four selected browser paths, and the existing named Go inventory validator.
- The final local four-path production-Web run must wait until the two parallel source lanes are frozen and migrations match. The parent owns allocation and grants the implementer the exact run permit.
- Hosted GitHub execution, branch-protection changes, real daemon/provider execution, paid-model behavior, broad NFR/capacity and M1 overall acceptance remain outside this child. Do not infer them from these four passes.
