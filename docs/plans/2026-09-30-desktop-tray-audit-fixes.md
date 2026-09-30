# Desktop Tray Audit Fixes Implementation Plan

**Goal:** Fix all seven reviewed tray and close-behavior defects without adding dependencies.

**Architecture:** Keep the existing Electron main/preload/renderer boundaries. Separate prompt delivery acknowledgement from human response time; route exit decisions through app.quit; commit preference cache updates only after successful writes. Query the actual Linux StatusNotifier host before hiding a window, and keep capability reads free of visible side effects. Intercept daemon shutdown synchronously and resume application exit only after cleanup.

**Tech Stack:** Electron, TypeScript, React, Vitest, existing OS D-Bus utilities.

## Scope and ownership

- Parent: close-behavior interception, prompt coordinator and IPC, preference persistence/cache, macOS settings gate, integration and documentation.
- Tray lane: `src/main/tray.ts`, its tests and a Linux host probe helper/tests if needed.
- Daemon lane: `src/main/daemon-manager.ts` shutdown wiring, a shutdown helper and its tests.
- Preserve all unrelated existing working-tree changes. No dependency additions, release, or deployment.

## Execution and regression coverage

1. Write failing regressions for the native tray handle: capability reads do not create icons; failed construction never hides a window; Linux support follows host registration rather than desktop/session names; destroy cancels pending creation.
2. Implement a fresh asynchronous Linux host probe using existing D-Bus command tools with bounded execution. `TrayHandle.isSupported(): Promise<boolean>` is side-effect free; `show(): Promise<boolean>` reports whether an icon was created. Unknown capability fails closed.
3. Write failing shutdown regressions: before-quit is prevented synchronously; repeated exit requests cannot bypass cleanup; autoStop false and cleanup failures still resume exit.
4. Implement the daemon shutdown state transition and wire it into the existing manager.
5. Write close/prompt regressions: acknowledged dialog stays open beyond five seconds; unacknowledged delivery times out; stale/wrong-window responses are ignored; prompt cancellation cleans listeners; quit closes the application with auxiliary windows alive; tray creation succeeds before hiding.
6. Implement the prompt coordinator and shared/preload acknowledgement channel. Bind each main window's show handler during window creation. Cancel outstanding prompts when application exit begins.
7. Write preference regressions for failed writes and overlapping writes; centralize serialized save-before-cache behavior for both settings and prompt paths.
8. Hide the behavior settings tab on macOS and cover the platform gate. Preserve existing supported-platform UI.
9. Run focused tests while iterating, then all desktop tests, desktop node/web type checks, relevant ESLint and diff checks. Update the desktop close-behavior spec and report native verification limits honestly.

## Simplifications

- Remove desktop-name heuristics, the unconditional human-response timer, duplicate preference mutation paths, and window-only exit fallbacks.
- Keep new helpers limited to independently testable lifecycle responsibilities; no generic framework or parallel state system.

## Validation commands

```sh
pnpm -C apps/desktop exec vitest run
pnpm -C apps/desktop run typecheck
pnpm -C apps/desktop exec eslint src/main src/preload src/shared src/renderer/src/components/close-behavior-prompt.tsx src/renderer/src/components/desktop-behavior-settings-tab.tsx src/renderer/src/routes.tsx
git diff --check
```

Native verification uses an isolated Electron process and temporary profile without starting real agents or changing the user's installed app. Linux host behavior is covered through command-boundary tests; Windows/Linux desktop-shell acceptance requires those platforms.

## Completion evidence

- All seven audit findings are implemented, including follow-up regressions for renderer reload/crash/unmount and daemon startup racing shutdown.
- Desktop Vitest: 72 files, 824 tests passed.
- Desktop main/preload and renderer TypeScript checks passed.
- ESLint passed for main, preload, shared contracts, and the affected renderer components/routes; `git diff --check` passed.
- `electron-vite build` passed for main, preload, and renderer. Existing shared CSS `::highlight` optimizer and mixed static/dynamic import warnings remain outside this change.
- Isolated native Electron smoke passed with actual IPC and two hidden BrowserWindows: an acknowledged prompt remained pending for 5,200 ms; both windows closed on application exit; three quit requests performed one daemon cleanup; pending startup completed before the final stop. No real agent or daemon was invoked.
- Final independent integration review found no remaining issue in the corrected prompt and shutdown paths.
- Windows/Linux shell behavior has not been verified on real machines. Linux conservatively reports unsupported if neither D-Bus tool or no StatusNotifier host is available.
