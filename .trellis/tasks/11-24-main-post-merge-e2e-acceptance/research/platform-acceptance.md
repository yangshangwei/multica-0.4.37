# Research: Desktop, mobile, daemon and deployment acceptance

- Query: Which platform journeys can be verified on the current machine, which existing suites exercise real product execution, and what must a broad post-merge plan retain as an explicit gap?
- Scope: Internal repository inspection and read-only local prerequisite checks. No suite, app, service, provider or deployment was started.
- Date: 2026-11-24
- Requested source baseline: main `103e2bc32` (supplied by the coordinator; this researcher performed no Git operation).

## Findings

### 1. Local platform prerequisites

Read-only checks on this machine produced:

| Prerequisite | Observed state | Implication |
| --- | --- | --- |
| Host | macOS 26.2, arm64 | Native Electron and macOS Host process behavior can be tested here. |
| Node | v22.23.2 | Meets repository Node 22 and native TypeScript-strip requirements. |
| pnpm | 10.28.2 | Available; existing warning says package.json `pnpm` settings have moved. |
| Go | go1.27.0 darwin/arm64 | Local native tests are not evidence for CI's Go 1.26/Linux runtime. |
| Docker | Client/server 29.2.1 | Available for isolated PostgreSQL, Linux package and contained fixture lanes. |
| Playwright | `node_modules/.bin/playwright`, executable | Existing dependency is installed. No browser was launched by this research. |
| Electron | `apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron`, executable | Real native renderer/preload acceptance is feasible after rebuilding current source. |
| Desktop output | `apps/desktop/out/main/index.js` exists | Existence does not establish current-source build provenance. |
| PostgreSQL client | Not on PATH; `/opt/homebrew/opt/libpq/bin/psql` exists and is executable | Prepend this existing directory to a task subprocess PATH. No installation is needed. |
| Xcode | `xcodebuild -version` reports active CommandLineTools; `/Applications` contains no `Xcode*.app` | No complete Xcode installation was found. |
| iOS simulator | `xcrun simctl` unavailable | Native iOS UI acceptance is blocked on this host. |
| Maestro / adb / ios-deploy | Not on PATH | No existing native mobile automation driver was found. |
| Mobile generated projects | `apps/mobile/ios` and `apps/mobile/android` absent | Mobile source/prebuild is available, but no existing native build can be reused. |
| Cached installed legacy CLI | Pinned image `sha256:8d5ede4df2fa4b66d499803b95428b729fcb6ba753b3800e9b931b968ee16104` exists locally, Linux amd64 | The narrow old-CLI compatibility fixture can run without pulling images. |

Existing `apps/desktop/dist/mac-arm64/Multica.app` and
`multica-desktop-0.6.5-185-g61ff581d2-dirty-mac-arm64.dmg` were found. They must
**not** be admitted as current-main package evidence. There was no `.deb` or
Windows `.exe` under the same inspected dist patterns. Rebuild/package candidate
bytes from the frozen current source and retain source/artifact/bundled-CLI hashes
before admitting a packaged journey.

### 2. Existing suite shapes and their actual proof

The current `e2e/` inventory contains **22 files with `desktop` in their name**.
That number is a static filename inventory, not a case count or pass claim.
Other files such as `password-server-switch.spec.ts`, `collaboration-host-runtime.spec.ts`
and installed-client suites also use native Electron despite lacking `desktop` in
their filename.

| Lane | Product code actually exercised | Important proof boundary |
| --- | --- | --- |
| Shared desktop business E2E | Real Electron renderer, built preload, production routing/IPC helpers and live isolated API | The common harness extracts Main functions and avoids the full `index.ts` daemon/install lifecycle. |
| Password server switch | Real Electron session cookies/windows and production `server-switch.ts` against two loopback HTTP fixtures | Proves same-host cross-port credential teardown; does not prove real remote-server authentication or installed daemon recovery. |
| Host Web/Electron execution | Production collaboration poller/HostRunner in a task test binary, real native processes, actual current Multica CLI, live API and Web/Electron clients | Provider is a test-owned Claude-protocol executable; account access/quota is not established. Full packaged Main is explicitly excluded. |
| Packaged full Main host | Full production packaged Main/preload/renderer, ordinary login, true native IPC, separately running native Host fixture | Explicit package/image + acceptance home are mandatory. Containerized Linux client control is not Linux native daemon evidence when the Host runs on macOS. |
| Legacy installed CLI | Actual v0.6.7 Linux amd64 offline OCI CLI bytes, exact expected issue command forwarded by a test-only stdio relay | Narrow compatibility check, no provider/daemon startup, no publisher-signature or installed-Desktop proof. |
| Native Go Host tests | Real OS process supervisor, ledger, launch/proof/cleanup and selected CLI behavior | Results only prove the OS actually running them. Windows ACL/Job object behavior needs Windows. |
| Mobile Vitest | Pure functions, serialization, query keys and headless cache/realtime updates | No RN component renderer, Hermes/device UI, gestures, keyboard or AppState suspension proof. |
| Mobile static checks | TypeScript, Expo ESLint and iOS launcher shell contract | A green check is not a native application build or mobile E2E result. |
| Updates verifier | Real HTTP full-file SHA-512, HEAD/Range and artifact/configuration contracts | Does not establish signed installation, elevation, OS trust, native updater or state retention during upgrade. |

Evidence anchors:

- `e2e/fixtures/collaboration-account-electron.mjs:14-16` deliberately avoids
  Main's daemon/install lifecycle; production functions are selected at `:31-56`.
  Daemon and updater fixtures are isolated at `:141-152`, and external network is
  refused at `:155-158`.
- `e2e/fixtures/collaboration-native.ts:18-28` requires an explicit executable,
  isolated profile and API/WS configuration; `:55-63` waits for the renderer document.
- `e2e/password-server-switch.spec.ts:16-31` defines the two loopback fixtures;
  `:64-71` verifies original identity/workspace headers cannot reach server B.
- `e2e/collaboration-host-runtime.spec.ts:15-45` names actual execution and explicit
  exclusions; `:85-144` covers Stop, retained pause, independent issue and explicit Resume.
- `e2e/collaboration-host-installed.spec.ts:36-70` requires selected package bytes
  and reports native/full-Main proof boundaries; `:74-99` verifies packaged runtime,
  renderer file URL and bundled CLI/install metadata.
- `e2e/fixtures/collaboration-installed-clients.ts:9-32` pins old CLI bytes/version;
  `:35-46` only permits an exact selected issue command; `:124-138` records scope and
  cleans only the randomly owned container/volume.

### 3. Broad platform test matrix

| Priority | Domain | Normal journey | Negative / recovery / boundary journey | Existing canonical entry |
| --- | --- | --- | --- | --- |
| P0 | Desktop identity/session | Onboarding → workspace → persisted login; password binding/change | Expired session; same-host server switch clears cookies/PAT/workspace/cache/child windows; failed candidate probe retains original session | `password-binding-desktop.spec.ts`, `password-server-switch.spec.ts`, `password-session-persistence.spec.ts`; desktop server-switch/auth tests |
| P0 | Account versus workspace navigation | Account tab with zero workspaces; deep-link and detail popout; workspace switching | Stale workspace group heals; delayed/outgoing scope cannot restore identity; child windows cannot exceed sender authority | `collaboration-account-desktop.spec.ts`, `global-scope-navigation.spec.ts`; desktop account-navigation tests |
| P0 | Cross-client collaboration | Web/Electron user inputs preserve each author and one shared Agent identity; live controls update both clients | Scoped Stop ends actual child process; later inputs remain paused across reload/restart; explicit Resume; separate issue continues | `collaboration-host-runtime.spec.ts`, `collaboration-cross-client-execution.spec.ts`, `collaboration-host-installed.spec.ts` |
| P0 | Daemon capability and identity | Selected server/profile; managed enrollment; current CLI/provider explicitly chosen; reconnect ready state | Stale PAT after password migration; 401-only refresh; failed mint preserves old credential; logout/server change cancels queued sync; independent CLI/network failure states | Main daemon-manager/auth/managed-recovery tests; `collaboration-host-switch.spec.ts`; daemon diagnostics spec |
| P0 | Native untrusted content | Trusted renderer assets, image/PDF and interactive HTTP previews remain usable | Sandboxed attachment/full-page/inline HTML cannot fetch `file://`; detached/unregistered frames fail closed | `node apps/desktop/scripts/verify-renderer-file-access.mjs` and renderer-file-access unit tests |
| P1 | Desktop business parity | Task/project/iteration creation, assignment, properties, status, history; MCP/skill templates | Disabled capability preserves historical read; shortcut focus; keyboard reorder; long text, narrow/zoomed windows; no native page error | `projects-p1-desktop.spec.ts`, `iterations-i1-desktop.spec.ts`, `iteration-scope-business-desktop.spec.ts`, `triage-duplicate-picker-desktop.spec.ts` |
| P1 | Notification/inbox parity | Bell → account/workspace detail; read/unread/snooze; mention navigation | Membership/revocation; stale pointer recovery; reload/undo/time precision; notification remains main-tab owned | `collaboration-mention-journeys-desktop.spec.ts`, `collaboration-offline-result-notifications.spec.ts`, `inbox-snooze.spec.ts` |
| P1 | Locale/theme/native layout | Chinese/English, saved language across restart; settings Help/Updates; dark/narrow view | Retired language fallback; long endpoints/errors; no horizontal overflow; reduced-motion/200% zoom and full scroll coverage | `retained-languages-desktop.spec.ts`, `desktop-settings.spec.ts`, native section compositor helper |
| P1 | Mobile semantic parity | Status/assignee/priority, project progress and inbox rows match web semantics in pure data tests | Response drift; missing fields; permission/count/key parity; stale realtime revision; cancellation/cache updates | 28 current Node `.test.ts` files under `apps/mobile/lib` and `apps/mobile/data`; mobile test script |
| P1 | Mobile native session/recovery | Login → task/project/inbox → cross-client update | Background >30 s then foreground; dead fetch cancellation; offline/reconnect; 401 clears identity/cache; keyboard-safe sheets and gesture/copy actions | No installed native test driver or native suite found; explicit blocked/manual lane on this host |
| P1 | Installed backward compatibility | Existing legacy CLI issue reads/writes against new API; full Main login and retained state | Malformed/drifted newer responses; old-client version constraints; no workspace/identity leak; upgrade/reinstall retains intended data | `collaboration-installed-clients.spec.ts`, `collaboration-installed-main.spec.ts`, host-installed spec |
| P1 | Deployment/upgrade/docs | Fresh isolated selfhost stack, auth configuration, migration + health + real login; docs build/link parity | Auth mode/key/deployment ID retention; interrupted private admin credential publication retry; knowledge/collaboration off/on gates; metadata/path/hash/Range failures | selfhost/offline/admin-initialization script tests, deployment docs, isolated optional rehearsal |
| P2 | OS-specific release acceptance | Windows candidate install/upgrade/reinstall with managed runtime; macOS native candidate | Windows Jobs/ACL/long paths, protocol uninstall ownership, elevation/signature/offline trust; updater restart/state retention | Existing CI native matrix and opt-in Desktop Smoke workflow; not established locally |

### 4. Accurate commands and environment contracts for the coordinator's plan

The commands below are recommended future validation entries, **not executed**
by this research. The coordinator must provide this task's fresh output directory,
isolated database and frozen candidate services; do not reuse prior `.env` secrets.

#### Static/headless platform gates

```bash
pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/desktop lint
pnpm --filter @multica/desktop test
pnpm --filter @multica/mobile typecheck
pnpm --filter @multica/mobile lint
pnpm --filter @multica/mobile test
pnpm --filter @multica/docs typecheck
pnpm --filter @multica/docs test
pnpm --filter @multica/docs build
node --test scripts/desktop-updates.test.mjs
node --test apps/desktop/scripts/verify-updates.test.mjs
bash scripts/selfhost-config.test.sh
node --test scripts/admin-init-compose.test.mjs
node --test scripts/offline-build-changelog.test.mjs
```

`apps/mobile/package.json:15-35` supplies mobile scripts; `apps/mobile/vitest.config.ts:4-12,22-26`
defines the Node-only proof boundary. `apps/mobile/scripts/ios-run.sh:22-23` runs Expo
prebuild then native iOS run; this requires full Xcode and is not runnable on the
observed host. The CI mobile gate is the same headless/static set
(`.github/workflows/mobile-verify.yml:16-18,65-66`), not native UI acceptance.

#### Default full gate and native business lane

```bash
PATH="/opt/homebrew/opt/libpq/bin:$PATH" make check
pnpm --filter @multica/desktop build
pnpm exec playwright test --workers=1 --retries=0 e2e/desktop-settings.spec.ts e2e/collaboration-account-desktop.spec.ts e2e/password-server-switch.spec.ts
node apps/desktop/scripts/verify-renderer-file-access.mjs
```

`make check` is helpful but **cannot alone satisfy the broad conditional matrix**:
`scripts/check.sh:118-123` forces legacy auth and turns managed installations,
knowledge, platform admin and device auth off. It sets a fresh signing key and
task origins (`:90-117`), runs guarded race/vet (`:193-202`), then builds Desktop,
serves renderer and production Web before one-worker/no-retry Playwright (`:205-219`).

Prepare a separate explicit password/managed/admin/knowledge/host lane. Common
native prereqs include `NEXT_PUBLIC_API_URL`, `DATABASE_URL`, matching
`COLLABORATION_E2E_API_URL`, `COLLABORATION_ELECTRON_RENDERER_URL` and source root,
`CHANGELOG_E2E_API_URL` / `CHANGELOG_ELECTRON_RENDERER_URL` for the older native
harness, plus appropriate opt-ins such as `E2E_PASSWORD_AUTH=1`. All origins must
be task loopback endpoints and renderer origin must be in the same API's CORS
allowlist. A successful OPTIONS status without allow-origin is not success
(`.trellis/spec/desktop/frontend/quality-guidelines.md`, Served renderer configuration).

#### Native Host process lane

```bash
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -p 1 -parallel 2 -count=1 -timeout=5m ./pkg/agent ./internal/daemon/collaborationrun ./internal/daemon ./internal/cli -run '^(TestHost|TestCollaborationHost)'
node scripts/check-collaboration-native-tests.mjs TASK_NATIVE_TESTS_JSONL macOS
pnpm exec playwright test --workers=1 --retries=0 e2e/collaboration-host-runtime.spec.ts e2e/collaboration-host-switch.spec.ts e2e/collaboration-host-squad.spec.ts
```

The Go JSON stream must be retained as `TASK_NATIVE_TESTS_JSONL`, not merely shown
on screen. The native checker requires 11 top-level cases and rejects failed/skipped
descendants (`scripts/check-collaboration-native-tests.mjs:9-57`). Its Windows
extensions are only real Windows evidence.

The Host browser lane requires explicit **built fixture executables** in
`COLLABORATION_HOST_DAEMON`, `COLLABORATION_HOST_PROVIDER`, `COLLABORATION_HOST_CLI`,
plus `E2E_PASSWORD_AUTH=1`, a task-only deployment UUID, API/DB/Web/native-renderer
configuration. `hostRuntimeConfigured()` uses *some* executable configuration to
enter, then `hostEnvironment()` rejects every missing/invalid field
(`e2e/fixtures/collaboration-host-runtime.ts:23-24,104-118`). Its managed execution
gates are opened transactionally only on the isolated database and restored after
the actual daemon closes (`:121-150`).

Shortest build entry for those three binaries, once `TASK_BIN` is an absolute,
canonical task-owned directory and the current source is frozen:

```bash
go -C server test -c -p 2 -o "$TASK_BIN/daemon-test" ./internal/daemon
go -C server build -o "$TASK_BIN/multica" ./cmd/multica
go build -o "$TASK_BIN/host-provider" ./e2e/fixtures/collaboration-host-provider.go
export COLLABORATION_HOST_DAEMON="$TASK_BIN/daemon-test"
export COLLABORATION_HOST_CLI="$TASK_BIN/multica"
export COLLABORATION_HOST_PROVIDER="$TASK_BIN/host-provider"
```

`server/internal/daemon/collaboration_host_router_test.go:23-25` defines the
Playwright-owned poller entry `__multica_collaboration_host_router_poller`;
`e2e/fixtures/collaboration-host-runtime.ts:349` launches that entry. The CLI
module path is confirmed by `Makefile:376`. This executes production poller/
HostRunner with the explicit fake protocol provider, not an ambient account CLI.
After supplying the password/managed/deployment/renderer environment above, the
shortest shared Web + actual Electron + daemon journey is
`pnpm exec playwright test --workers=1 --retries=0 e2e/collaboration-host-runtime.spec.ts`.
Add `e2e/collaboration-host-installed.spec.ts` only after packaging the current
full-Main candidate and satisfying its explicit package/profile/hash inputs.

#### Packaged candidate and legacy CLI lanes

```bash
pnpm exec playwright test --workers=1 --retries=0 e2e/collaboration-installed-clients.spec.ts
pnpm exec playwright test --workers=1 --retries=0 e2e/collaboration-installed-main.spec.ts e2e/collaboration-host-installed.spec.ts
```

First command additionally needs the explicitly built Linux stdio relay
`COLLABORATION_INSTALLED_CLI_RELAY` and pinned cached old package. It cannot start
an arbitrary installed CLI command.

For native full Main, select exactly one native candidate
`COLLABORATION_INSTALLED_MAIN_EXECUTABLE` and a fresh
`MULTICA_PACKAGED_ACCEPTANCE_HOME`, or a task-owned Linux image/package directory/.deb.
Pin ASAR/CLI/executable/package/source SHA as demanded by the fixture. No missing
image is pulled implicitly. Build/package validation must use `--publish never`;
do not create a release tag to fabricate upgrade ordering
(`.trellis/spec/desktop/frontend/intranet-updates.md:101-129`).

### 5. Model account access is a separate scope

Default Go tests must use explicit fake or missing agent executables, as required
by root CLAUDE.md. `scripts/go-test-with-agent-cli-guard.sh:19-57` installs task
sentinels and fails if an ambient provider is invoked.

Commercial provider smoke tests are behind `agentintegration` and the **additional**
`MULTICA_RUN_REAL_AGENT_SMOKE=1` gate
(`server/pkg/agent/real_agent_smoke_integration_test.go:1,10-15`). They can access
authenticated accounts and consume quota. A general request for broad E2E is not
specific authorization to use those accounts. Keep those cases explicit
`not-run / requires provider-account authorization`; do not set this gate merely
to make inventory appear complete. Synthetic tagged fixtures and fake Host
providers can test actual runtime execution without commercial-model access.

### 6. Deployment and documentation proof boundaries

The existing updates verifier supports:

```bash
node apps/desktop/scripts/verify-updates.mjs --url TASK_HTTP_UPDATE_DIRECTORY --metadata latest.yml --expected-version TASK_CANDIDATE_VERSION
```

This checks full-file SHA-512 plus HEAD/Range/reference contracts; it explicitly
does not install updates (`apps/desktop/scripts/verify-updates.mjs:101-108`). Use a
task-only loopback service/storage; no production publish is required. Real native
install/update proof is separate (`.trellis/spec/desktop/frontend/intranet-updates.md:81-99`).

For offline upgrade rehearsal, use independent compose project/ports/storage and
synthetic accounts. Preserve original authentication mode, JWT/encryption keys,
deployment UUID and existing data. Rehearse migrations, actual config and original
account login; `/healthz` alone is insufficient. If an upgrade bundle is built,
both packaged `README.md` and `操作文档.md` must byte-match
`docs/offline-upgrade.zh-CN.md` and retain the authentication migration/admin
initialization sections (`docs/offline-upgrade.zh-CN.md:458-469`). No real deployment,
release tag or publication follows automatically from validation.

One documentation drift candidate merits explicit runtime verification:
`docs/offline-upgrade.zh-CN.md:59` describes the knowledge context environment default
as true, whereas checklist `:468` says false. Do not decide effective defaults from
that wording alone; inspect current code and assert `/api/config` plus on/off behavior.

### 7. Recent acceptance does not replace the new source-bound run

- Prior R3 final report says 514/514 admitted browser executions passed across 174
  files, plus headless/native-fixture/backend lanes. It expressly excludes native
  mobile UI, Windows/macOS installers/signing/updaters, commercial models and
  distributed capacity (`.trellis/tasks/archive/2026-11/11-20-full-e2e-regression-20261120/research/r3-final-results.md:3-7,36-49,59-61`).
- The just-merged continuation documents repairs to private cursor keys,
  PostgreSQL snooze precision, UTC deadlines, scoped retained-history fixtures and
  `/var` temp-path canonicalization. It records a matching tested candidate and
  19 successful PR checks but distinguishes the separately queued push-to-main run
  (`.trellis/tasks/archive/2026-11/11-23-cross-workspace-main-readiness/research/merge-continuation-20261124.md:22-42,47-73,78-101`).
- Plan a new current-source inventory and fresh output roots. Prior counts are
  research inputs, never new PASS results. Missing conditional prerequisites must
  show as missing/unrun with the exact reason rather than being hidden by a green
  parent suite or the default `make check` configuration.

## Related Specs and Files Found

- `.trellis/spec/desktop/frontend/quality-guidelines.md`: onboarding/status catalog,
  daemon credential refresh, selected profile and served renderer prerequisites.
- `.trellis/spec/desktop/frontend/connection-recovery.md:44-115`: bounded boot,
  truthful retry/editor persistence, sender authority and required isolated native acceptance.
- `.trellis/spec/desktop/frontend/renderer-file-access.md:18-43`: file-boundary
  negative/positive matrix and mandatory native script.
- `.trellis/spec/desktop/frontend/intranet-updates.md:74-129`: real transport versus
  installed acceptance, candidate provenance, native profile/hash/termination requirements.
- `.trellis/spec/desktop/frontend/account-navigation.md`: account/workspace tabs,
  popouts and bootstrap identity boundaries.
- `.trellis/spec/desktop/frontend/daemon-diagnostics.md`: independently visible
  runtime/CLI/network/auth recovery conditions.
- `apps/mobile/CLAUDE.md`: mobile sharing, timeout/cancellation, realtime semantic
  parity, background recovery and keyboard/formSheet/gesture rules.
- `apps/mobile/vitest.config.ts`: intentionally Node-only mobile test scope.
- `.github/workflows/ci.yml:634-682`: native Host matrix on Ubuntu/macOS/Windows;
  current OS-specific gate requires actual evidence.
- `.github/workflows/desktop-smoke.yml:5-29,120-130`: separately opt-in installed
  Windows candidate/business/collaboration/signature lanes.
- `docs/desktop-intranet-update-runbook.zh-CN.md`: first-install/download/update
  service and authentication migration handoff.

## Caveats / Not Found

1. No native mobile automation suite or usable Xcode/iOS simulator was found.
   Responsive Chromium viewports cannot replace native mobile E2E. Keep that
   platform blocked with prerequisite evidence unless a real device runner becomes available.
2. Do not infer Windows/Linux Host behavior from macOS execution or cross-compilation.
   Existing matching-source CI evidence can be cited separately, not relabeled local E2E.
3. No suite was run and no service/model account was accessed by this research.
   Counts above are filename inventory or quoted historical evidence only.
4. Several desktop/mobile frontend index/quality documents remain placeholders;
   the active topic specs, root CLAUDE.md and mobile CLAUDE.md supply real contracts.
5. Existing binary/output/package files lack fresh current-main provenance. Rebuild,
   hash and bind exact candidate bytes before claiming installed/full-Main acceptance.
6. Full commercial-model, signed installer/updater, real-device and distributed-load
   behavior remains a named boundary even after all feasible local suites pass.
