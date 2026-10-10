# Native Electron verification runtime

Prepared on 2026-10-10 for iteration Tasks density verification. The task-owned API was used for verification and has now been stopped and destroyed with its disposable database/profile/registry. No Web server, daemon, user Desktop instance, or agent CLI was launched.

## Runtime identity and verification

- Runtime root: `/tmp/multica-iteration-density-20261010T115735Z-82e616`
- Environment: `multica-iteration-density-20261010t115735z-82e616`
- API: `http://localhost:18573`
- API listener PID: `98640`; launcher PID: `98612`
- API commit: `cfa0254eb`; start time: `2026-10-10T11:58:01Z`
- Database: `multica_iteration_density_20261010t115735z_82e616_api`, local PostgreSQL, 590 migrations applied
- Task env: `/tmp/multica-iteration-density-20261010T115735Z-82e616/api.env` (mode 0600)
- Registry: `/tmp/multica-iteration-density-20261010T115735Z-82e616/dev`
- API provenance: `/tmp/multica-iteration-density-20261010T115735Z-82e616/dev/envs/multica-iteration-density-20261010t115735z-82e616/api.running.json`
- Effective config evidence: `/tmp/multica-iteration-density-20261010T115735Z-82e616/config-evidence.json`
- Synthetic fixture readiness: `/tmp/multica-iteration-density-20261010T115735Z-82e616/fixture-readiness.json`

`/health` and the registry ownership checks passed. `/api/config` returns legacy auth, device auth unavailable, managed installation unsupported, platform admin unavailable, signup enabled, workspace creation enabled, messaging/VCS integrations disabled, and analytics disabled. The process env also has iterations enabled, development verification code configured, rates set to 1000, and shared Redis disabled. A synthetic email-code login, workspace creation, and workspace deletion all succeeded without printing tokens.

The original root `.env` SHA-256 is unchanged from the pre-copy hash. `.env.worktree` did not exist and was not created. All external credentials/integration endpoints in the task copy were disabled or replaced with local task values. The launcher starts with a small clean environment, stripping inherited Multica production/task identity, and uses dedicated profiles, workspaces, desktop app data, uploads and temporary files.

## Commands

Run these from the repository root. `run.py` sanitizes inherited environment variables and calls the existing `dev-env.sh exec` for the explicit task environment; it injects the task `DATABASE_URL`, `NEXT_PUBLIC_API_URL`, auth settings, and `MULTICA_E2E_DESKTOP_OUTPUT_DIR`.

Build the native renderer/preload **after implementation edits are stable**:

```bash
python3 /tmp/multica-iteration-density-20261010T115735Z-82e616/run.py exec pnpm --filter @multica/desktop exec electron-vite build --outDir /tmp/multica-iteration-density-20261010T115735Z-82e616/desktop-out
```

This uses the installed electron-vite 5.0.0 CLI `--outDir` option, which separates `main`, `preload` and `renderer` under that absolute root. It avoids `apps/desktop/out` and skips the package `build` script's unnecessary CLI bundling. The native fixture launches its test harness (`e2e/fixtures/changelog-electron.cjs`), compiled preload, and renderer; it does not run the compiled application main. The harness installs daemon/CLI stubs, and `project-p1-desktop.ts` asserts zero daemon starts. I have **not built** the Desktop output.

Native Playwright command (replace the native spec and artifact directory with the parent's chosen task test):

```bash
python3 /tmp/multica-iteration-density-20261010T115735Z-82e616/run.py exec pnpm exec playwright test e2e/iteration-scope-business-desktop.spec.ts --workers=1 --retries=0 --output .trellis/tasks/archive/2026-10/10-10-iteration-task-tab-density/artifacts/native
```

The `project-p1-desktop.ts` fixture serves that isolated renderer itself, proxies API/WebSocket requests to port 18573, launches Electron with a temporary profile, uses TestApiClient synthetic users/workspaces, and cleans up its fixture workspace/profile. A Web listener is not required. The API uses development auth while the native renderer build is a production build.

Inspect:

```bash
python3 /tmp/multica-iteration-density-20261010T115735Z-82e616/run.py status --json
```

Stop only this task's API, keeping DB and evidence:

```bash
python3 /tmp/multica-iteration-density-20261010T115735Z-82e616/run.py down --components api
```

After the task is finished and its evidence is retained, destroy only its disposable runtime database/profile/registry:

```bash
python3 /tmp/multica-iteration-density-20261010T115735Z-82e616/run.py destroy --yes
```

The API was left alive for the parent's native checks, which are now complete. The manifest carries a 24-hour TTL, but its custom registry is not scanned by unrelated default-registry launches: use the explicit cleanup command above.

## Build provenance strategy

Before and after the Desktop build, capture the repository commit and a source fingerprint covering `apps/desktop/src`, `apps/desktop/electron.vite.config.ts`, the shared `packages/` sources, and workspace/lock configuration; include untracked source. A changed fingerprint means rebuild before interpreting native results. Save the exact build command, log, output root and SHA-256 of compiled preload, renderer HTML and renderer assets with the Playwright output. Copy `api.running.json` and the config evidence beside the native result.

The API launcher's `source_id` fingerprints the checkout at API startup, including the then-current uncommitted changes. It is not proof of the later renderer build: the parent must record the Desktop build separately after its concurrent UI edits settle.

## Isolation decisions and limitations

`dev-env.sh up` reuses a same-checkout registry entry even when another `--name` is requested. The default registry already contains an ownership mismatch, so this setup never called `make up` there and never stopped/adopted its processes. The task-local startup script reuses the existing launcher's allocator, env loader, migration runner, manifest writer, detached process launcher and ownership validation, with an explicit unique database name (rather than a potentially retained path/slot-derived DB). Every filesystem mutation outside this report is under the external runtime root or the unique local database.

`psql` is installed at `/opt/homebrew/opt/libpq/bin/psql` but was absent from the incoming PATH. The launcher adds that installed directory. Native CUA remains unavailable; Playwright's Electron fixture is the available real native verification surface.
