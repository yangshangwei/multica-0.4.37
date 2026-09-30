# Desktop Close Behavior: Minimize to Tray / Quit / Ask

## Background

Today on Windows and Linux, closing the Multica Desktop main window terminates the
entire application immediately. There is no system tray integration, no
"minimize to background" affordance, and no way for the user to express a
preference between "exit the app" and "hide but keep running."

On macOS, the app follows the OS convention (window closes, app stays in dock,
reopens via `activate`). This is correct and stays untouched.

Users running on Windows/Linux with active daemon sessions or long-running
desktop features want the choice between true exit and minimize-to-tray.

## Goal

Give Windows / Linux users a one-time choice (with a "remember my decision"
option) between three close behaviors:

| Behavior | Effect when the user closes the main window |
|---|---|
| `quit`    | Destroy window, run `before-quit` (daemon cleanup), `app.quit()`. Current behavior. |
| `minimize`| `event.preventDefault()` → `window.hide()` → show system tray icon. App continues to run; daemon stays running. |
| `ask`     | Show a modal asking "Exit Multica?" with three options (Quit / Minimize to tray / Cancel) plus a "Remember my choice" checkbox. Once remembered, the chosen behavior applies silently. |

macOS keeps its existing platform-native behavior unchanged.

## Non-Goals

- No change to macOS window/dock semantics.
- No quit-from-tray on macOS.
- No new global hotkeys, shortcuts, or IPC beyond what the close prompt and settings surface need.
- No attempt to ship a working system tray in GNOME 40+ environments that lack AppIndicator / StatusNotifierItem support — those degrade gracefully (see "Linux GNOME fallback").
- No backfill of legacy prefs: users on existing builds keep the implicit `quit` behavior; nothing is migrated.

## Functional Requirements

### F1. Persistence of the user's choice

- Stored in a new main-process prefs file: `app.getPath("userData")/close-preferences.json`.
- Schema: `{ closeBehavior: "quit" | "minimize" | "ask" }`. Default: `ask`.
- Survives app restarts, reinstall, and OS reboot. Stored under `userData` so
  it's per-install and isolated from other Multica workspaces.
- Written atomically (tmp + rename) and parsed defensively with a defaults
  fallback on any read error / shape mismatch — same pattern as
  `updater-preferences.ts`.

### F2. Close interception (Windows / Linux only)

- `mainWindow.on("close", ...)` consults the stored preference.
- `quit` → no interception (existing path).
- `minimize` → `event.preventDefault()`, `window.hide()`, tray icon visible (or fallback if tray unsupported).
- `ask` → `event.preventDefault()`, send an IPC `close-behavior:prompt` to the renderer, wait for the user's choice (with a 5s timeout fallback to `quit` to avoid wedging the app if the renderer is hung), then apply the chosen behavior and optionally persist it.

### F3. Tray icon (Windows / Linux, when supported)

- Single tray icon with menu items: **Show Multica** / **Quit**.
- Clicking the tray icon (left click on Windows, double-click on Linux) shows the main window.
- **Quit** from the tray menu calls the existing `app.quit()` path so daemon cleanup runs.
- The tray shows the app icon; on Windows this is a `.ico` resource, on Linux a `.png`.
- Resources added under `apps/desktop/resources/` and surfaced through `electron-builder.yml` `extraResources`.

### F4. Linux GNOME fallback

If Tray is not functional (detected via `Tray` constructor failure, or via
`process.env.XDG_CURRENT_DESKTOP` indicating GNOME 40+ without AppIndicator
support), the `minimize` option must **degrade gracefully**:

- Settings UI disables the "minimize to tray" choice with an explanatory
  tooltip.
- If a previously stored preference is `minimize` but tray is unsupported on
  the current boot, the close behavior falls back to `quit` and emits a
  single `console.warn` line; no modal, no silent wobble.

### F5. Settings entry

New settings tab (or extension of an existing desktop-settings tab) in
`apps/desktop/src/renderer/src/components/`, exposing a radiogroup for the
three behaviors. Writes go through a new preload-exposed API surface.

### F6. First-run prompt

- On a fresh profile (`closeBehavior === "ask"`), the first close shows a
  modal with: Quit / Minimize to tray / Cancel buttons plus a
  "Remember my choice" checkbox.
- If the user picks Cancel, no behavior change happens; the next close will
  prompt again.
- If the user picks Quit or Minimize **with** remember, the preference is
  persisted and subsequent closes apply it silently.
- If the user picks Quit or Minimize **without** remember, apply for this
  close only; do not persist.

### F7. Single-instance focus

When the user re-launches the app while it's minimized to tray, the existing
`second-instance` handler must call `mainWindow.show()` (in addition to
`focus()`) so the window actually reappears.

### F8. Daemon lifecycle

- `minimize` does **not** trigger daemon shutdown.
- `quit` (from any path: window close, tray menu, OS-level quit) goes through
  the existing `before-quit` flow and respects the existing `autoStop` daemon
  pref.
- These paths are orthogonal. The new prefs file does not interact with
  `daemon_prefs.json`.

## Acceptance Criteria

- AC1: On Windows 10/11 and Ubuntu 22.04 KDE / XFCE, with default pref
  (`ask`), closing the window shows the prompt modal; choosing each option
  behaves as documented, and "remember choice" persists across app restart.
- AC2: With pref `minimize`, closing the window hides it; daemon continues;
  clicking the tray icon shows the window; Quit from the tray menu respects
  `autoStop`.
- AC3: With pref `quit`, closing the window exits the app exactly as before
  this change.
- AC4: On Ubuntu 22.04 GNOME (no AppIndicator), `minimize` is disabled in
  settings, closing still works (falls back to `quit` if that was the stored
  pref), and a single warning is logged.
- AC5: macOS behavior is byte-identical to today.
- AC6: `~/.multica/desktop_prefs.json` is **not** touched by this feature.
- AC7: New file `apps/desktop/src/main/close-behavior.ts` mirrors the safety
  patterns of `updater-preferences.ts` (atomic write, defaults on read
  error, no throw on corrupt JSON).

## Cross-cutting Decisions (locked at planning)

- **Q1: Does macOS get a "minimize on close" option?** No. macOS keeps
  platform-native behavior (close hides, activate reopens). Adding Windows
  semantics there would invert user expectations.
- **Q2: Linux GNOME without tray?** Accept the degradation; do not attempt
  AppIndicator polyfill or extension hints beyond a settings tooltip.
- **Q3: Where do prompt strings live?** Continue using
  `@multica/views/i18n`. Desktop-only strings go into a new `desktop.json`
  namespace (or extend `settings.json` if that's lighter); they do not go
  into `~/.multica/desktop_prefs.json`'s scope.
- **Q4: Does `minimize` stop the daemon?** No. The two lifecycles are
  independent.

## Child Tasks

- `09-30-close-prefs-and-intercept` — F1 + F2 + F7 + F8 bone structure. No UI.
- `09-30-tray-and-linux-fallback` — F3 + F4 + build resource wiring.
- `09-30-prompt-and-settings` — F5 + F6 + i18n + preload API.
- `09-30-real-machine-verification` — Verification + intranet packaging.

Each child's `prd.md` carries only what it needs; this file is the source of
truth for cross-cutting requirements.
