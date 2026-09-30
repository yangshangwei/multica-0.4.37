# Child 1: close-prefs-and-intercept

## Scope

Foundation only — no UI, no tray. Wire the close interception and prefs
persistence so that all subsequent children build on a stable in-memory +
on-disk contract.

Implements F1, F2, F7, F8 from the parent PRD.

## Deliverables

1. `apps/desktop/src/main/close-behavior.ts`
   - `CloseBehavior` type + `DEFAULT_CLOSE_BEHAVIOR`
   - `closePreferencesPath(userDataPath)`
   - `loadClosePreferences(filePath)` / `saveClosePreferences(filePath, value)`
   - `applyCloseBehavior(opts)` — sync `close` listener with all branches

2. `apps/desktop/src/main/close-behavior.test.ts`
   - Persistence round-trip, corrupt-file fallback, round-trip for each
     behavior.
   - `applyCloseBehavior` coverage: macOS no-op, isQuitting short-circuit,
     quit passthrough, minimize hides+shows-tray, ask → each prompt outcome,
     prompt-throw → forced quit.

3. `apps/desktop/src/main/index.ts` modifications
   - Module-level: `isQuitting`, `cachedCloseBehavior`, `trayHandle`
     (placeholder), `pendingClosePromptResolve`.
   - Load prefs in `whenReady` before `createWindow()`.
   - IPC handlers: `close-behavior:get`, `close-behavior:set`,
     `close-behavior:is-tray-supported`, `close-behavior:respond`.
   - `requestCloseBehaviorPrompt(window)` — single-flight, 5s timeout.
   - `app.on("before-quit", ...)` flips `isQuitting = true`.

## Out of scope (pushed to later children)

- Tray creation / destruction / menu (child 2).
- Settings UI (child 3).
- Preload API surface (child 3 — but stubs may land here to make IPC testable).
- Real-machine Linux GNOME verification (child 4).

## Acceptance Criteria

- A/C 1.1: `pnpm vitest run apps/desktop/src/main/close-behavior.test.ts`
  passes (≥ 18 tests).
- A/C 1.2: `pnpm -F desktop typecheck` passes for both `tsconfig.node.json`
  and `tsconfig.web.json`.
- A/C 1.3: Closing the window with `cachedCloseBehavior === "quit"` (the
  default if no prefs file exists **and** the default weren't "ask") runs the
  pre-existing close path with no behavior change.
- A/C 1.4: Deleting `close-preferences.json` and restarting the app does not
  throw; defaults apply.
- A/C 1.5: Existing daemon prefs (`~/.multica/desktop_prefs.json`) untouched;
  `daemon-manager.ts` not modified.
- A/C 1.6: `applyCloseBehavior` does NOT register a listener on macOS
  (asserted via unit test inspecting `listenerCount`).

## Notes for implementer

- Don't touch `daemon-manager.ts`. The before-quit hook is added **in
  index.ts**, in the same `else` branch that already holds the single-instance
  lock — no functional conflict with the daemon-manager's own before-quit
  cleanup.
- Reuses the `updater-preferences.ts` writer pattern (tmp + rename). Do not
  reach for `electron-store` or new deps.
