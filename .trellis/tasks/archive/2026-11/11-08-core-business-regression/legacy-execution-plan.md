# Remaining legacy execution

Prepared scripts close **14 previously skipped identities**: detail Web/Electron 2, Device 1, Assist 8, Recommendation 3. The complete recommendation spec also reruns one already-covered default-precedence case, so collection contains **15 tests across 4 specs**. Collection exited 0; no test body was executed.

Private execution files:

- `/Users/artisan/.multica/qa/core-20261108/legacy/run.py` — Device/Assist/Recommendation variants.
- `/Users/artisan/.multica/qa/core-20261108/legacy/detail.py` — read-only provenance capture, with optional explicit execution.
- `manifest.json`, `collection-inventory.json`, `prepared.json` in that directory.

The password and controlled-runtime controllers remain untouched by this preparation and are under separate review.

## Detail journey: current read-only evidence

`detail.py --snapshot-id detail-preflight-1` completed read-only against the actual allocated API, production Web, built renderer and database. Evidence is in `/Users/artisan/.multica/qa/core-20261108/legacy/detail-preflight-1/`:

- Task/source-root/allocation binding and allocation hash.
- Actual listener PIDs, launch commands, API health/configuration, production Build ID.
- Renderer index/preload hashes, served index comparison, five required product source hashes plus the two test files.
- Explicit read-only PostgreSQL query proving the allocated database and manual-only collaboration gates.

The current API and Web `source_id` values differ because two test files changed after the original build: `e2e/collaboration-workbench-web.spec.ts` and `packages/core/platform/core-provider.test.tsx`. Comparison with the preserved pre-build `source-sha256.json` proves all production source bytes remain unchanged. The native renderer launcher did not record a compiler source ID; its proof therefore explicitly labels `source_id` as the **current source snapshot**, paired with the unchanged-production-source audit and exact built/served artifact hashes. It does not claim a compiler-stamped renderer identity.

Root should create a fresh snapshot immediately before the detail run because listener PIDs may change after other variants restore the API. The helper refuses any new product drift rather than expanding its two-file test-only allowance.

```sh
# Read-only refresh, safe while no source/build changes are happening.
python3 /Users/artisan/.multica/qa/core-20261108/legacy/detail.py \
  --snapshot-id detail-preflight-2

# Only after root releases the baseline API; this executes two real journeys.
python3 /Users/artisan/.multica/qa/core-20261108/legacy/detail.py \
  --snapshot-id detail-run-1 --execute
```

The helper supplies supported `COLLABORATION_DETAIL_TASK`, `_ALLOCATION_FILE` and `_PROVENANCE_FILE` values. Existing fixture validation remains unchanged. It uses the current legacy database and does not restart or reconfigure services.

## Device, Assist and Recommendation

Run these only when root has finished baseline/detail browser work using the shared API. Each variant gets a fresh local database, private HOME/TMP/upload/catalog directories and exact fixed Web/API/renderer origins. API mode is legacy; platform-admin, managed installation and Cloud are disabled. Device mode is enabled only in its own variant, with no inherited auto-join workspace/role. API pools use max 6/min 1.

```sh
python3 /Users/artisan/.multica/qa/core-20261108/legacy/run.py \
  --execute --run-id legacy-variants-1

# A separately numbered focused run, if a prior failure needs diagnosis.
python3 /Users/artisan/.multica/qa/core-20261108/legacy/run.py \
  --execute --run-id assist-2 --ids lg-assist
```

| ID | Full spec | Actual provider configuration |
| --- | --- | --- |
| lg-device | auth-device.spec.ts | No provider; `MULTICA_DEVICE_AUTH_ENABLED=true` and `E2E_DEVICE_AUTH=1` |
| lg-assist | issue-assist-flow.spec.ts | Exact current `e2e/support/assist-provider.mjs`; task-owned loopback listener; `/v1` as server LLM base URL and synthetic API key |
| lg-recommend | quick-create-actor-picker-phase2.spec.ts | Exact current `e2e/fixtures/creator-recommendation-provider.mjs`; separate task-owned loopback listener/protocol |

Helpers request port zero and the controller obtains the actual loopback listener from the owned child PID, avoiding guessed or shared provider ports. It verifies `/health`, records helper source hashes and call records, and terminates only the owned helper process. No commercial LLM key/endpoint, SMTP or external-channel configuration is inherited. These tests prove the real UI → API → controlled HTTP model fixture → persisted product flow; they do not prove commercial-model reasoning.

Go binaries build only at execution with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`. Every Playwright invocation uses one worker and zero retries, preserves named results and rejects skips. First failure stops the lane. Cleanup retains the original failure and writes `final.json` even if API restoration fails. Interrupted subprocess groups are terminated before restoration; helper stop and restore errors are reported separately. The current original API is restored, not replaced with another environment.

Preparation validation: both scripts pass `py_compile`, preparation CLI passes, 15-case collection passes, and detail read-only preflight passes. Provider startup, database provisioning, service switches and test bodies remain NOT_RUN until root invocation. Runtime cleanup protections have been inspected, not exercised during this preparation.

## Packaged Main prerequisites and cached build inputs

No current candidate app package exists under the isolated checkout's `apps/desktop/dist`. The ordinary `~/Library/Caches/electron` ZIP directory is currently absent. Existing usable distribution inputs are separately recorded in `distribution-prerequisites.json`:

- Native macOS Electron distribution under the current checkout's `apps/desktop/node_modules/electron/dist`.
- Extracted Linux arm64 Electron distribution at `/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/core-e2e-20261104/linux-package/electron-linux-arm64`.

These are base Electron inputs, not current-source Multica packages. Reusing an old app.asar or bundled CLI would not prove this candidate. Copy the selected distribution into an owned packaging staging directory, record its hashes, then pass `-c.electronDist=<owned-distribution>` to prevent a missing ZIP cache from initiating a download.

Packaging must happen in a separate source-matched checkout or after the shared renderer is no longer served, because the package wrapper rebuilds `out` and stages its target CLI. Candidate commands after that preparation are:

```sh
CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @multica/desktop package \
  --mac dir --arm64 --publish never \
  -c.electronDist=<owned-mac-electron-dist> -c.directories.output=<owned-mac-output>

CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @multica/desktop package \
  --linux dir --arm64 --publish never \
  -c.electronDist=<owned-linux-electron-dist> -c.directories.output=<owned-linux-output>
```

Use offline Go/npm inputs and remove signing/publishing credentials from packaging processes. Confirm electron-builder's remaining tools are cached before running; no package command has been executed here.

For native macOS full Main, bind `COLLABORATION_INSTALLED_MAIN_EXECUTABLE`, task-owned process HOME and `MULTICA_PACKAGED_ACCEPTANCE_HOME`, current source/overlay, executable/app.asar/bundled-CLI hashes, and a missing task-owned DSH path. For Linux full Main, bind the cached harness image plus current app directory/volume and matching arm64 payload; Host-installed additionally takes its ASAR hash, or DEB plus executable/CLI pins. Cached harness image identities are listed in `controlled-runtime-plan.md`. Actual local package launch, restart, daemon FD behavior and shared-issue evidence remain separate from directory packaging. Signing, notarization, package-manager lifecycle and automatic updater acceptance are not implied.
