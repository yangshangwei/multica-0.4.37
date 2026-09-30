# Child 2: tray-and-linux-fallback

## Scope

System tray lifecycle + Linux GNOME detection + packaging resources. Depends
on child 1 (close-prefs-and-intercept) — its `applyCloseBehavior` consumes
the tray handle this child wires up. Implements F3, F4 from the parent PRD.

## Deliverables

1. `apps/desktop/src/main/tray.ts` (new)
   - `TrayHandle` interface: `{ show(), hide(), destroy(), isSupported() }`.
     Electron's `Tray` has no `setVisible`, so:
     - `show()` → construct `new Tray(...)` if not alive, store reference.
     - `hide()` → `tray.destroy()` and clear reference.
     - `isSupported()` → `true` if Tray constructor succeeded previously or
       hasn't been tried; `false` if it threw / heuristic detects unsupported
       environment.
   - `buildTrayIconPath({ resourcesPath, isDev, platform })` — picks `.ico`
     on Windows, `.png` on Linux.
   - `isTrayEnvironmentSupported()` — heuristic for GNOME 40+ Wayland:
     - `process.platform !== "linux"` → true.
     - `XDG_CURRENT_DESKTOP` (lowercase) contains "gnome"
       AND `XDG_SESSION_TYPE === "wayland"` → false.
     - Otherwise true. Documented heuristic; runtime failure remains the
       authoritative fallback.
   - `createTray({ window, resourcesPath, onShow, onQuit })` — wires menu
     with labels from a lookup table (i18n handled by injecting labels from
     renderer at construction time via IPC, OR by shipping en/zh-Hans strings
     directly in the main process keyed by `app.getLocale()`; the latter is
     the simpler choice and consistent with how `electron-builder.yml` builds
     `desktop.dock.*` menu strings).
   - Tray menu: `Show <productName>` / separator / `Quit`.

2. `apps/desktop/src/main/tray.test.ts` (new)
   - Heuristic table for `isTrayEnvironmentSupported` over
     `XDG_CURRENT_DESKTOP` values (`GNOME`, `KDE`, `XFCE`, `ubuntu:GNOME`,
     `gnome-wayland`, etc) × `XDG_SESSION_TYPE`.
   - `buildTrayIconPath` on win/linux + dev/prod.

3. `apps/desktop/src/main/index.ts` modifications
   - At `whenReady`, after close-behavior prefs load, call
     `trayHandle = createTray({ window: <created in createWindow>, ... })`
     — but stash in the module-level slot introduced by child 1.
   - `applyCloseBehavior.showTray` now invokes `tray.show()`; `hideTray`
     added similarly if needed.
   - `app.on("before-quit")` -> `trayHandle?.destroy()`.
   - `window.on("show")` -> `trayHandle?.hide()` so the tray icon only
     appears when the window is hidden (the converse is handled by the
     close interception).
   - Tray menu "Quit" handler calls `isQuitting = true; app.quit()`.
   - Tray menu "Show" handler calls `mainWindow.show(); mainWindow.focus();
     trayHandle.hide();` (the hide is redone by window.on("show") anyway).

4. Resources: `apps/desktop/resources/tray-icon.ico` and
   `apps/desktop/resources/tray-iconTemplate.png` (32x32 + @2x).
   - If `resources/icon.png` is high enough resolution, derive tray icons
     with a one-shot image resize. Otherwise reuse `build/icon.png`
     directly on Linux, and let Electron's `nativeImage.createFromPath` pick
     the closest size from a multi-size `.ico` on Windows.
   - **Decision rule**: ship the smallest thing that won't visibly blur; do
     NOT add a custom icon pipeline. A single 32x32 PNG works on both.

5. `electron-builder.yml` — append to `extraResources`:
   ```yaml
   - from: resources/tray-icon.ico
     to: tray-icon.ico
   - from: resources/tray-iconTemplate.png
     to: tray-iconTemplate.png
   ```

6. `close-behavior.ts` small augmentation
   - `applyCloseBehavior` and the close-behavior `shouldOfferMinimize`
     decision now consult both the heuristic and the constructor-failure
     record. This child owns merging those two sources of truth:
     `showTray` is a no-op (with a single console.warn) when
     `isTrayEnvironmentSupported()` returns false even if the constructor
     previously succeeded.

## Out of scope

- Settings UI (child 3). IPC to ask the renderer about tray support is
  added in child 3, NOT here.
- macOS tray (excluded by parent PRD).
- Balloon notifications / system tray context-menu localization polish
  beyond the bare en/zh-Hans pair.

## Acceptance Criteria

- A/C 2.1: `pnpm vitest run apps/desktop/src/main/tray.test.ts` passes.
- A/C 2.2: `pnpm -F desktop typecheck` passes.
- A/C 2.3: On Windows, packaged build runs, pref `minimize` → close → tray
  icon appears in system notification area; left-click / "Show Multica" menu
  item → window reappears; tray "Quit" → app fully exits.
- A/C 2.4: Same steps on Ubuntu 22.04 KDE (or any X11 session). On
  GNOME+Wayland without the AppIndicator extension, the tray menu never
  appears and settings will mark minimize as unavailable (verified in
  child 3; here we just confirm `isTraySupported()` returns false).
- A/C 2.5: On GNOME+Wayland, with a stale `minimize` pref: closing the window
  performs a real close (no silent wedging), and a single warn is logged.
- A/C 2.6: electron-builder package includes the new resources. Linux
  AppImage direct-launch still uses `BUNDLED_ICON_PATH` for the window icon;
  the tray icon is separate and resolved via `process.resourcesPath` in prod.

## Notes

- `Tray` constructor must not be called when
  `isTrayEnvironmentSupported()` returns false; even constructing one on
  headless GNOME+Wayland can produce GTK warnings in stderr.
- On macOS this entire module is a no-op (`isTrayEnvironmentSupported`
  returns false on darwin so `applyCloseBehavior` falls through to its
  builtin `process.platform === "darwin"` early return).
