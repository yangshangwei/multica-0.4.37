# Research: shared sidebar visual verification environment

- Query: Find the fastest safe baseline/candidate visual verification path for the approved default-collapse + quiet Beta sidebar change.
- Scope: internal repository/runtime, read-only except this research file
- Date: 2026-11-22

## Findings

### Fastest current frontend preview

The human-owned Electron source renderer is running at `http://localhost:5666/`:

- `lsof -nP -iTCP:5666 -sTCP:LISTEN` found Node PID **18420**.
- `lsof -a -p 18420 -d cwd -Fn` binds that renderer to this checkout's `apps/desktop` directory.
- `curl --noproxy '*' http://localhost:5666/` returns the Multica HTML with `/@vite/client` and `/src/main.tsx`; this is a live Vite source preview, not a production build.
- `make status ARGS="work-management-local --json"` reports the human-owned Desktop running, launcher **18206**, renderer **18420**, API **18572**, Web **13492** stopped. The live native app can be inspected through root's CUA surface; it should not be restarted for this task.
- `apps/desktop/.env.development.local` selects API `http://localhost:18572` and WS `ws://localhost:18572/ws`.

The shared frontend change is visible through this preview as source hot reload. Root can capture a fresh native baseline/candidate, test disclosure/title/notification controls, switch EN/ZH and inspect compact/zoom behavior using the existing native app. The research role did not interact with CUA or mutate app settings.

### API and registry caveat

The API on 18572 answers `/health` with PID **18138**, commit **70f8528db**, started **2026-11-21T07:31:07Z**; its cwd is this checkout's `server`. Nevertheless `make status` marks it **mismatch**, so the registry does not prove the expected process/build. Its source commit is older than the root's current branch. A live native screenshot establishes frontend presentation, not whole-current-source backend acceptance.

Plain `make status ARGS="--json"` picks a different first record, `check-20261119151016-76544`, owned by an agent with API18573/Web13493/renderer5667. All those services are stopped. Many historical check manifests point at the same checkout. Always use an explicit environment name when inspecting/running a known record.

`scripts/dev-env.sh:1301` implements `cmd_up`; its existing-directory branch at approximately line1379 reuses the first registered environment even when a different `--name` was supplied. Therefore `make up --name <new>` alone is **not** reliable isolation for this directory. Do not use it to restart/replace the human-owned app.

### Baseline evidence and freshness

Existing baseline layout captures are under:

`artifacts/qa/unified-sidebar-navigation-20261121/journey-final-results/`

Useful exact references:

- `global-scope-navigation-We-2f8bc-s-and-both-scroll-positions-chromium/web-scope-workspace.png`: expanded collaboration above Workspace, wide English.
- `global-scope-navigation-We-2f8bc-s-and-both-scroll-positions-chromium/web-scope-wide-zh.png`: wide Chinese collaboration/account directory.
- `global-scope-navigation-De-bddde-s-and-both-scroll-positions-chromium/desktop-scope-workspace.png`: actual native wide English.
- `global-scope-navigation-De-bddde-s-and-both-scroll-positions-chromium/desktop-scope-compact-zh.png`: actual native compact Chinese.

`final-runtime-manifest.json` binds those captures to this checkout, production Web13493/API18573/renderer5667, and the prior final source hashes. `research/final-acceptance.json` in the archived unified-sidebar task dates acceptance 2026-11-22. At research time, the current `app-sidebar.tsx` and `sidebar-navigation-group.tsx` hashes differ from that retained manifest; those captures predate the latest footer/title work and cannot prove a pixel-exact current baseline.

The freshest footer and alignment evidence is:

- `.omx/state/sidebar-account-footer/ralph-progress.json`: actual CUA native source screenshot/interaction evidence on 2026-11-22; static Multica header, Help lower-left and named account control lower-right. The candidate screenshot was returned in the earlier conversation, not saved to a named local file.
- `.omx/state/sidebar-title-alignment/ralph-progress.json`: actual native CUA evidence of aligned heading text/right disclosure arrows. Again the final candidate is conversation-only.
- `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/codex-clipboard-tLQGaG.png`: existing user reference file cited by the footer verdict; it exists, but is a reference rather than proof of the latest changed footer.

Root has now captured fresh current native baselines in this task:

- `research/visual/native-baseline-workspace-zh.png`
- `research/visual/native-baseline-zh.png`

Use those current source/native captures as the comparison references. Do not describe old unified captures as the exact current UI.

### Narrow production Web/native journey

`e2e/global-scope-navigation.spec.ts:72` contains just **two** complete tests (Web and actual Electron). It already clicks the accessibly named Collaboration resume heading before selecting Participating (`:123`), so default collapse is broadly compatible. If Beta is appended to a New menu item's DOM text, retain its existing explicit accessible name or root must narrowly update the exact menu-name selectors; current tests expect `New collaboration issue` exactly (`:119`, `:127`).

The journey verifies exact workspace/account restoration, search/sort and both scroll positions, current-scope no-op, read-error recovery, compact EN/ZH sheet dismissal, native deep links and 200% zoom. It creates synthetic actors/workspaces, registers collaboration fixture rows in the isolated database, and cleans up those exact actors/rows (`:19`, `:40`, `:49`, `:313`). It uses public password registration/onboarding when `E2E_PASSWORD_AUTH=1`; the isolated legacy check environment uses synthetic invalid-example email actors and the existing dev verification-code fixture. No human credentials are required.

`e2e/fixtures/sidebar-navigation.ts:12` handles actual compact responsive retirement, sheet geometry, animations, and hit-testing; use it before clicking navigation on narrow windows. `e2e/fixtures/collaboration-native.ts:18` launches this checkout's Electron executable with a temporary userData profile; `:56` requires an explicit production renderer URL. `nativeScreenshot` (`:79`) uses `webContents.capturePage`, suitable for actual native screenshots. This is renderer/preload/native-window acceptance with fixture Main, not a packaged installer or full production Main/daemon claim.

Prior exact build commands and measured times are retained in `artifacts/qa/unified-sidebar-navigation-20261121/final-build-results.json`:

```sh
pnpm --filter @multica/web build
pnpm -C apps/desktop exec electron-vite build
```

Prior final journey command (2 tests, exit0, 21.8s in its recorder):

```sh
pnpm exec playwright test e2e/global-scope-navigation.spec.ts --workers=1 --retries=0 --reporter=line --output=artifacts/qa/unified-sidebar-navigation-20261121/journey-final-results
```

Those are historical results, not results for this task's changed code.

### Safe isolated subset runner recipe

The fastest **production** route is to reuse the source-safe allocation/start/cleanup helpers from `scripts/check.sh`, without running the whole repository gate. Its final guard (`:223`) calls `check_main` only when executed directly; sourcing loads helpers without starting services/tests. `prepare_check_environment` (`:61`) creates a new task database, a private copied env/signing key, allocated ports and exact Web/native origins while preserving `.env`. `start_check_desktop_renderer` (`:140`) serves the built renderer with registry PID/hash checks. `check_cleanup` (`:38`) stops only processes this run marked as started.

Root may write and execute a bounded Bash runner using this sequence after implementation and review are complete:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd /Volumes/artisan/code/2026/multica-0.4.37
source scripts/check.sh
sidebar_source_env="$(env_file_path "${ENV_FILE:-$(detect_env_file)}")"
load_env_file "$sidebar_source_env"
prepare_check_environment "$sidebar_source_env"
trap check_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"
export NO_PROXY="localhost,127.0.0.1,::1"
export VITE_API_URL="http://localhost:$BACKEND_PORT"
export VITE_WS_URL="ws://localhost:$BACKEND_PORT/ws"
export VITE_APP_URL="http://localhost:$FRONTEND_PORT"
ensure_database
migrate_database
CHECK_STARTED_API=true
start_api
pnpm -C apps/desktop exec electron-vite build
CHECK_STARTED_RENDERER=true
start_check_desktop_renderer
CHECK_STARTED_WEB=true
start_web
print_status_json > .trellis/tasks/11-22-collaboration-sidebar-beta/research/production-preview-status.json
pnpm exec playwright test e2e/global-scope-navigation.spec.ts --workers=1 --retries=0 --reporter=line --output=artifacts/qa/collaboration-sidebar-beta-20261122
CHECK_COMPLETE=true
```

`start_web` in production mode owns the production build/start lock and performs the Web build. Do not separately run a competing Next build. This recipe has **not** been executed by research; root must read actual output, confirm listener/build identities and inspect the generated captures. It omits the unrelated full TS and Go phases, which root handles through appropriate focused checks.

### File preservation, cleanup and bounds

- The subset uses **no `.env.check`** and does not read/write/replace it. Existing `.env.check` needs no restoration for this recipe. `prepare_check_environment` copies its selected source env into a newly allocated `~/.multica/dev/envs/check-<timestamp>-<pid>/check.env`, mode0600. It reads `.env` (or the explicitly chosen source) and does not rewrite it. Avoid `ensure_dev_code`, `rewrite_env_ports`, `cmd_up` and `start_desktop`, which are outside this recipe and may update source or native dev env files.
- The live user's `apps/desktop/.env.development.local`, native userData/profile and existing context stay untouched: this recipe builds/serves a separate production renderer and launches the E2E Electron fixture with temporary userData. Explicit VITE URLs above configure only the runner's build environment; they do not write native dev config.
- The Web build updates generated `apps/web/.next`/MDX output and may rewrite `apps/web/next-env.d.ts`. Root already has a pre-existing dirty change there. Preserve its exact bytes/mode before the build and restore that precise snapshot afterward if the generated route-types line changes; never restore it from git. Source identity normalizes only that exact dev/build import line (`scripts/dev-env.sh:630`), so restoring the snapshot after the run does not weaken the source guard. Backing up `.env`, `.env.check` and native dev env hashes before the run can substantiate preservation, but no secret values need to enter logs.
- Production `start_web` refuses concurrent/reused Web output and owns `.multica/web-build.lock.d`; it releases that lock on exit. Freeze implementation/check edits and commits before startup: the source guard covers HEAD, all tracked diff plus untracked source under apps/packages/server/scripts, and rejects source changes during the build. New untracked task research/screenshots do not enter that source set.
- `check_cleanup` stops only flagged renderer/Web/API processes whose pid/ownership files belong to this new allocation; it does not stop the human-owned native app or call registry GC/destroy. The isolated API database/env/manifest are **retained** for inspection with a24h TTL; the test itself removes its synthetic actors/rows. If a later explicit cleanup is desired, name this exact new record and never force-close unrelated database connections. The recipe never creates the isolated Go DB, so `CHECK_GO_DB_CREATED` staysfalse.
- Allocation-lock waiting is about10s. Renderer startup loops50×0.2s with curl probes max2s. API startup uses a300-second *counter* and max3s health probes; Web startup uses the same counter and max15s probes. These counters are **not hard wall-clock limits** (probes add time), and the build subprocess itself has no built-in hard deadline.
- Each selected browser/native test declares180s; Electron launch timeout20s, first window15s, renderer document20s, navigation/paint readiness15s. One worker and zero retries bound repetition. The prior normal complete two-test run took21.8s, production Web build62.6s and native build3.3s.
- Neither `gtimeout` nor `timeout` is installed on PATH in this session; Python3 is available. Run the whole subset through an external Python subprocess watchdog using `Popen(..., start_new_session=True)` and a **600s total deadline**. On expiry sendSIGTERM to that runner's process group, allow30s for Bash EXIT/TERM cleanup, then fail/kill the runner group if it still remains. Launch with the exec tool's short initial yield and poll normally so root can keep reporting progress. Record a deadline/cleanup failure as a failed verification, not a skipped pass. No additional dependency is needed.

## Files and related specs

- `Makefile:185`: `up`; `:191`: status; `:203`: env-exec, all routed to the registry script.
- `scripts/dev-env.sh:1201`: safe status JSON fields; `:1301`: existing-directory up reuse; `:1673`: explicit-record env-exec.
- `scripts/check.sh:61`: isolated production allocation; `:140`: built native renderer; `:164`: full default gate, avoid invoking for this small increment unless desired.
- `playwright.config.ts:10`: selects PLAYWRIGHT_BASE_URL then FRONTEND_ORIGIN, starts no servers.
- `e2e/global-scope-navigation.spec.ts`: canonical two-client restoration and screenshots.
- `e2e/fixtures/sidebar-navigation.ts`: responsive/painted navigation readiness.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production Web and explicit isolated/native prerequisites.
- `.trellis/spec/core/frontend/global-scope-navigation.md`: account-safe global ownership, disclosure/resume/compact contracts.
- `.trellis/tasks/archive/2026-11/11-21-unified-sidebar-navigation/verification.md`: source-bound earlier verification limits.

## Caveats / Not Found

- Old footer/title sessions had conversation-only final captures. Root has now supplied fresh baseline native files in this task; candidate captures still need the changed-source inspection.
- Default Python urllib localhost request followed this session's proxy and returned502. `curl --noproxy '*'` reached the loopback renderer/API immediately; configure loopback proxy bypass in any runner.
- An unfiltered `make list --json` pipeline failed during this research; explicit named status and bounded manifest inspection were used instead. No registry repair or cleanup was performed.
- Research started no service, created no actor/data, ran no browser/native journey, modified no product/spec/config and performed no direct git operation. Only this research document was written.
