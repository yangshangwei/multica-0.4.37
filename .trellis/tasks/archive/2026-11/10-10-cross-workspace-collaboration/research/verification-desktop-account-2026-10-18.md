# Desktop account check — final handoff

Desktop source frozen. No commits, task metadata or Trellis specs were changed by this lane. Base HEAD: `bd168d18edb37829203cd97c980d3e36895cce64`; verification includes the existing uncommitted Desktop work and this review's fixes.

## Findings fixed

- `apps/desktop/src/renderer/src/stores/tab-store.ts`: account rehydration selected the final occurrence of a repeated URL rather than the saved history index. Preserve the original index through canonicalization/filtering. Both regressions failed before the fix.
- `apps/desktop/src/renderer/src/platform/navigation.tsx`: account entry published the tab before clearing ambient workspace headers. All push/replace/content/new-tab paths now clear the mirror before the store notification. Four regressions failed before the fix; stale outgoing layout coverage also added.
- `apps/desktop/src/renderer/src/App.tsx`: auth-token login still awaited an unrelated workspace request, blocking an authenticated pending account link. Removed the redundant list fetch; the existing shared Query owns workspace loading. The native stalled-list regression failed before the fix and now passes.
- `apps/desktop/src/shared/issue-window.ts`, renderer issue-window/navigation, Main/preload and adjacent tests: account details lacked a native popout. Extended the existing tagged issue-window context with account/workspace scope, preserving auth-session closure and workspace issue behavior. Account lists hand off to the main tab; detailed account links navigate within popouts. The new route wrapper lives in `platform/account-workbench-route.tsx`.
- `apps/desktop/electron.vite.config.ts`: native bundles externalized raw shared TypeScript, including an unsupported `require('@multica/core/paths/account')` in the sandboxed preload. Bundle core validators into both native outputs. The built outputs contain no such require; real preload works.
- Desktop shell/tab presentation: absent workspace sidebar was still treated as expanded, placing the account tab beneath native navigation controls; its title was Unknown page. Reserve toolbar clearance without a sidebar, omit the unavailable sidebar toggle, and use the localized account tab/window title. Added failing regressions and verified native geometry and screenshots.
- Diagnostic route reporter: removed its unused workspace selector/argument. Account bucketing was handed to the shared-core owner and verified by the Desktop regression.
- Shell mocks and typed window fixtures were updated to cover the new composition/context rather than failing on absent mock exports.
- `e2e/collaboration-account-desktop.spec.ts` and `e2e/fixtures/collaboration-account-electron.mjs`: added six real Electron scenarios, production routing/IPC extraction, strict test-owned profiles, genuine native keyboard input, and a loopback HTTP proxy for explicitly controlled edge cases. Proxy requests otherwise reach the real API; WS goes directly to the real API. No installed daemon or agent CLI runs.

## Verification

- Focused Desktop unit tests: **271 passed / 22 files** (`desktop-unit.log`).
- Desktop node and renderer typecheck: **pass** (`desktop-typecheck.log`).
- Changed Desktop TS/config lint: **37 files pass** (`desktop-lint.log`). New E2E spec and native fixture lint: **pass** (`desktop-e2e-lint.log`).
- Standalone strict E2E TypeScript check: **pass** (`desktop-e2e-typecheck.log`).
- Desktop main/preload/renderer build: **pass** (`desktop-build.log`), invoked with `pnpm --filter @multica/desktop exec electron-vite build`; no bundle-cli/start step.
- Native acceptance: **6 passed**, 21.8 seconds (`desktop-native-iteration7.log`). Final complete navigation case with corrected native Retina screenshot: **1 passed**, 9.2 seconds (`desktop-native-zoom.log`).
- Native file access: **pass**, Electron **39.8.7**, `failures: []` (`desktop-file-access.log`), invoked with `node apps/desktop/scripts/verify-renderer-file-access.mjs`.
- `git diff --check -- apps/desktop e2e/collaboration-account-desktop.spec.ts e2e/fixtures/collaboration-account-electron.mjs`: **pass**.
- Visual verdict **95/pass**: `desktop-visual-verdict-02.json`; persisted under `.omx/state/desktop-account-check/ralph-progress.json`.

Native cases prove A → account → B, one account tab across repeated issue/view links, warm and cold scroll restoration, preserved pins/MRU/history and restart selection, account retained during controlled workspace removal, actual zero-workspace account reads, controlled workspace HTTP500, login while workspace HTTP remains pending, controlled setup/password flags, queued cold native links, account popout navigation, native Cmd+W, viewer/server reset and popout closure. Native zoom uses a 900-point content window at factor 2, asserts 450 CSS pixels, and captures through `webContents.capturePage()`.

## Re-run and ownership

The task-owned static renderer is still listening at `http://127.0.0.1:52651`, **PID 64950**, tool session 15453. It serves `apps/desktop/out/renderer`; rebuild assets in place before the root's final shared-core check. Launch command:

```sh
python3 -m http.server 52651 --bind 127.0.0.1 --directory apps/desktop/out/renderer
```

No Electron acceptance or proxy server remains running. Each proxy is owned by its Playwright worker, binds an ephemeral loopback port, and closes in `finally`. The root owns API PID 70981 and its database lifecycle; this lane did not stop them.

Exact six-scenario command (from repository root):

```sh
COLLABORATION_E2E_API_URL=http://127.0.0.1:52649 COLLABORATION_ELECTRON_RENDERER_URL=http://127.0.0.1:52651 NEXT_PUBLIC_API_URL=http://127.0.0.1:52649 DATABASE_URL='postgres://postgres@127.0.0.1:51974/collaboration_browser_resume?sslmode=disable' MULTICA_DEV_VERIFICATION_CODE=123456 pnpm exec playwright test e2e/collaboration-account-desktop.spec.ts --workers=1 --retries=0 --output=/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-resume-20261018-23vtl_pb/desktop-native-final
```

Focused unit command:

```sh
pnpm --filter @multica/desktop test account-tab account-bootstrap account-navigation navigation tab-coordinator tab-store main-renderer-messages issue-window workspace-route-layout desktop-layout tab-bar diagnostic-route-reporter connection-recovery-boundary use-open-settings-shortcut
pnpm --filter @multica/desktop typecheck
pnpm exec tsc --noEmit --strict --target ES2022 --module ESNext --moduleResolution Bundler --esModuleInterop --allowSyntheticDefaultImports --skipLibCheck --types node e2e/collaboration-account-desktop.spec.ts
```

## Not fixed / integration limits

- Root owns CLAUDE/Trellis spec sync: document persistent account/workspace/none scope, v5 owner-bound persistence, pre-navigation mirror clearing, independent workspace-list recovery, account detail popouts and native bundling of shared validators.
- Shared-core cycle/control-mask work and full Web verification were assigned to `workbench_check` after this lane's source freeze. Root should rebuild Desktop and rerun the six scenarios once that integration settles.
- These native checks are macOS evidence and use isolated native-service stubs with production routing/IPC functions; they do not prove a packaged full Main startup on Windows/Linux.
- Collaboration rows are seeded only in the root-owned test DB after ordinary issue setup. Public collaboration creation remains intentionally gated, so this is not AC67 or a full create/attachment/AI business journey. Required account flags and workspace failure/removal are explicitly controlled HTTP cases, not backend lifecycle acceptance.
