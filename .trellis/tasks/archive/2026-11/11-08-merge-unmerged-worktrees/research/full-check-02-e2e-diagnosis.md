# full-check-02: native E2E fixture prerequisite diagnosis

Observed at 2026-11-08T12:09:00.818578+08:00. Read-only investigation; candidate c52eeb53f637f1f5f3dc19ba5d1ac51b974fb56c and build outputs were not changed. The full gate was still running; no second test/build was launched.

## Confirmed cause and evidence

The native fixture cannot serve its renderer because this fresh integration checkout has **no `apps/desktop/out` directory**. Both required `out/renderer/index.html` and `out/preload/index.js` are absent. The run's private `check.env` has no `MULTICA_E2E_DESKTOP_OUTPUT_DIR` override; the wrapper also strips inherited `MULTICA_*` variables. `project-p1-desktop.ts:13` therefore selects this checkout's absent default output.

`project-p1-desktop.ts` serves static files using `readFile` under that renderer directory, catches read failures and returns empty502 responses. A read-only HTTP probe of the *already-running owned fixture* confirmed precisely that behavior: Playwright worker PID92091 owned loopback port60035; GET `/` returned HTTP502 with0 response bytes. The earlier port59960 exited before its probe and is not counted as supporting evidence. No fixture/API writes or test processes were started by this diagnosis.

`scripts/check.sh` runs lint/typecheck, TS tests, script/Go race/vet, API startup, production Web build/start, then Playwright. It has **no Desktop build**. Desktop's typecheck and unit-test tasks do not create `out`. The canonical desktop build is `pnpm --filter @multica/desktop build`, whose package script bundles the local CLI and runs `electron-vite build`. This compiles binaries; it does not execute an agent. The fixture launches `changelog-electron.cjs`, not production Main, and its daemon-start IPC handlers explicitly refuse execution.

The log records the first failures:

- line8606: iteration-scope-business-desktop en,33.0s;
- line8611: iteration-scope-business-desktop zh-Hans,30.7s;
- line8623: iterations-audit-desktop toolbar en,31.0s.

Corresponding Web iteration-scope-business scenarios passed. The absence of the required files and the live502 establish a native environment preparation defect independently of assertions. At this observation, the Playwright line reporter had **not yet emitted final failure stacks**, and no native failure screenshot/trace/error-context file was present in test-results. Do not claim the precise timeout stack location or that all eventual failures have this single cause until the final reporter is preserved. Default Playwright config does not enable trace/screenshot collection; the custom p1Failure helper can only capture once its Page has been acquired.

## Direct consumers of project-p1-desktop

| Spec | Cases | Required output |
| --- | ---: | --- |
| iteration-scope-business-desktop.spec.ts |2 (en/zh-Hans)|renderer + preload|
| iterations-audit-desktop.spec.ts |3 (toolbar en/zh-Hans + audit)|renderer + preload|
| iterations-i1-desktop.spec.ts |3|renderer + preload|
| projects-p1-desktop.spec.ts |6|renderer + preload|
| triage-duplicate-picker-desktop.spec.ts |2 (en/zh-Hans)|renderer + preload|

All16 cases use the fixture unconditionally. Separate native specs also consume the same local output: `triage-desktop.spec.ts` (unconditional), `password-session-persistence.spec.ts` (unconditional; supplies its own isolated password API mock), `mcp-desktop.spec.ts` and `password-binding-desktop.spec.ts` (password-mode gated). Collaboration native specs have their own gates and output identity assertions; no opt-in should be broadened as part of this correction.

## Minimal correct preparation for the next run

1. Let full-check-02 finish and preserve state, complete failure list and test-results before any rebuild or rerun. Keep it failed.
2. Build the current clean integration SHA in that checkout with `pnpm --filter @multica/desktop build`, using task-only environment settings. This is the repository's supported build; do not point tests at unrelated or stale worktree artifacts. No installed app, auto-start daemon or real agent execution is needed.
3. Require nonempty `apps/desktop/out/renderer/index.html`, referenced renderer assets and `apps/desktop/out/preload/index.js`; record exact build command, SHA, exit code and artifact SHA256s. Recheck git clean/HEAD. If only renderer/preload compilation is desired, the package's direct `electron-vite build` produces them, but the standard package script also verifies the bundled CLI.
4. Run a newly numbered **complete** gate after preparation (and after fixing any independent failures from the complete run02 report). Do not skip desktop specs, change retries/timeouts to mask missing assets, or splice run02 stage passes into success.

For durable repository prevention, make check should explicitly build the Desktop artifacts it later consumes and cover the ordering/build-failure exit in `scripts/check.test.sh`. That is a separate small orchestration change requiring a new fixed commit. A fixture preflight could provide an immediate missing-artifact error, but it does not replace preparation and is not necessary to prove the present cause.

The existing `.trellis/spec/web/frontend/e2e-run-environment.md` near line378 already documents this missing-output→502 failure mode for other native specs and requires a same-checkout build or hash-verified same-source copy. The current fixture shares that requirement.


## Continuation: standalone Desktop cases and exact build setup

The later failure at log line8690 / item183 is `password-session-persistence.spec.ts:10` (30.9s). This test **does not require the real API to switch to password mode**. It starts its own isolated loopback mock implementing `/api/config` with password auth, `/auth/login`, `/api/me`, empty workspaces and invitations. Its profile is retained across three Electron processes within the test to prove persisted login and revoked-session cleanup, then removed. Its renderer is hardcoded to this checkout's `apps/desktop/out/renderer`; the shared `changelog-electron.cjs` defaults preload to this checkout's `apps/desktop/out/preload/index.js`. Missing static files here return404 JSON, rather than the project fixture's502. Both missing paths remain a confirmed setup blocker; no captured final stack yet proves whether the observed30.9s stopped at launch or a later readiness operation.

The other unconditional standalone native case is `triage-desktop.spec.ts:9`. It uses the task-owned real legacy API selected by `NEXT_PUBLIC_API_URL`, authenticates a synthetic account, creates its own workspace/triage data, and serves the same local renderer/preload through a loopback proxy. Missing files produce502. Together with the16 project-p1 cases, these make18 default native cases using the local changelog Main fixture and requiring its output. This count does not cover the separate Knowledge native fixture described below. No additional build location or auth-mode change is required for these18.

Other direct Electron entry points have explicit configuration gates and must retain them:

- `changelog-desktop`, `desktop-settings`, `retained-languages-desktop`, `quick-create-actor-picker`: explicit local `CHANGELOG_ELECTRON_RENDERER_URL` + `CHANGELOG_E2E_API_URL`; retained languages additionally requires the matching DB/API and CORS preflight.
- `quick-create-actor-picker-phase2`: those native origins plus an explicit deterministic recommendation provider.
- `mcp-desktop` / `password-binding-desktop`: `E2E_PASSWORD_AUTH=1` plus their real password-mode fixtures; the binding case also uses task-only DB and migration-window configuration.
- `password-server-switch`: password opt-in, but transpiles its small native helper into the temporary profile and serves synthetic HTML, so it does not require built renderer/preload.
- `admin-resource-publishing`: `E2E_RESOURCE_PUBLISHING=1`, and native consumer only with `E2E_RESOURCE_NATIVE=1`; separate output `E2E_RESOURCE_DESKTOP_BUILD` (default task report desktop-build). This is not a default native dependency.
- Most Collaboration native/installed/provider suites require their own explicit environment/provenance/feature gates; do not activate them indiscriminately to repair artifact preparation. Knowledge has an unconditional exception below.

### Exact preparation command (after run02 ends)

Run from `/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108`:

```sh
pnpm --filter @multica/desktop build
```

`apps/desktop/package.json` defines this as `pnpm run bundle-cli && electron-vite build`. `electron.vite.config.ts` does not override `build.outDir`; installed electron-vite5 defaults under the package root to `out/main`, `out/preload`, and `out/renderer`. Thus it satisfies every default native fixture path directly. Do not use a global `--outDir` or merely set `MULTICA_E2E_DESKTOP_OUTPUT_DIR`: only project-p1 honors that variable; the two standalone default tests do not.

The renderer API/WS origin is supplied at runtime by the isolated fixture's `runtime-config:get` IPC through the real preload, so no special production URL, VITE environment override or real password API is needed to compile these artifacts. Use the clean candidate and standard production build, not the staging mode or a borrowed checkout. The build's bundled CLI is compiled and ad-hoc signed on macOS; these fixture launches still use the isolated Main stub and do not start the bundled daemon or paid agents.

The environment must already have this checkout's installed Electron executable and dependencies (the run visibly starts that executable). macOS uses `apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron`. Linux would additionally need an X display / Xvfb for the headed Electron process; this run is macOS, so adding Linux display flags is not applicable.


### Additional unconditional Knowledge native prerequisite

`e2e/collaboration-knowledge.spec.ts:293` declares `Desktop native source review and independent project/workspace contributions` without `test.skip`. Its beforeAll calls `legacyLaneMissing()` and fails for non-isolated/non-legacy configuration; the test itself throws unless both `COLLABORATION_ELECTRON_RENDERER_URL` and `COLLABORATION_ELECTRON_SOURCE_ROOT` are supplied. This spec had not appeared in the full02 progress log as of item202, so its collection/execution outcome is not yet claimed here. The original18 count is only the local changelog fixture group.

`fixtures/collaboration-native.ts` additionally consumes `COLLABORATION_E2E_API_URL` and derives `COLLABORATION_ELECTRON_WS_URL`; it launches the separate collaboration-account Electron harness, which uses same-source preload and compiles production native routing helpers into a task profile. It starts about:blank, then the test explicitly starts the renderer; all daemon-start methods reject execution. The renderer must be served over loopback HTTP, and actual API/WS endpoints must match the newly allocated full-check environment. CORS/WS allowed origins must include that renderer origin.

Do not set stale run02 API ports in a source env for full03: check.sh allocates new ports and currently only overwrites NEXT_PUBLIC_API_URL / NEXT_PUBLIC_WS_URL, not the separate Collaboration API variable. Nor does `MULTICA_E2E_DESKTOP_OUTPUT_DIR` configure this harness. If the complete report/collection confirms this native case is part of the full gate, build first and provide the matching source-root + task-owned static renderer + allocated API/WS configuration as one owned environment. The current `check_allowed_origins` supports an explicit loopback Collaboration renderer origin, but renderer server lifecycle and dynamic API mapping still need orchestration. Preserve other provider/installed gates. Do not hide an unconditional test with a new skip.
