# Close Behavior (Windows / Linux)

> What happens when the user clicks the close button of the main window on
> Windows and Linux, plus the related settings UI and IPC plumbing.
>
> Introduced by task `09-30-desktop-close-behavior` (parent) and its
> children `09-30-close-prefs-and-intercept`, `09-30-tray-and-linux-fallback`,
> `09-30-prompt-and-settings` (all archived in `.trellis/tasks/archive/2026-09/`).

---

## Contract

The Multica Desktop app offers three close behaviors on Windows and Linux:

| Behavior  | Effect when the main window's close button is pressed |
|-----------|--------------------------------------------------------|
| `quit`    | Destroy the window; run `app.on("before-quit")` (daemon cleanup happens); `app.quit()`. Historical default. |
| `minimize`| `event.preventDefault()`, `window.hide()`, tray icon shows. App continues running. Daemon stays running. |
| `ask`     | Default for new profiles. Render-side AlertDialog asks the user; the response is applied and optionally persisted. |

macOS is **not** affected. `close-behavior.applyCloseBehavior` early-returns on
darwin so the OS-native close-hides / activate-reopens convention continues.

---

## Persistent State

The preference is stored at:

```
app.getPath("userData")/close-preferences.json
```

Shape: `{ "closeBehavior": "quit" | "minimize" | "ask" }`. Default: `"ask"`.

This is **separate** from `~/.multica/desktop_prefs.json` (the daemon prefs).
The daemon-manager must never touch `close-preferences.json`, and the close-
behavior module must never touch daemon prefs. The two lifecycles are
independent: `minimize` does not stop the daemon, and `quit` is what triggers
any `before-quit` daemon cleanup.

The file is read once at `whenReady` and cached in a module-level variable
(`cachedCloseBehavior`). Reads after that consult the cache; writes flow
through the `close-behavior:set` IPC handler, which updates BOTH the cache
and the disk via `saveClosePreferences` (atomic tmp+rename, same pattern as
`updater-preferences.ts`).

`applyCloseBehavior` MUST be sync. It cannot re-read the disk on every close
— `event.preventDefault()` inside the close handler is sync-only.

---

## Process Roles

### `apps/desktop/src/shared/close-behavior.ts`

The cross-process contract. Exposes the `CloseBehavior` type, the
`CLOSE_BEHAVIOR_CHANNELS` constants, the `CloseBehaviorPromptRequest` /
`CloseBehaviorPromptResult` / `CloseBehaviorSetResult` shapes, and the
`isCloseBehavior` runtime guard.

**Rule**: `main/` and `preload/` must import `CloseBehavior` and channel
constants from here, never redeclare them. Drift between these two processes
is invisible until runtime.

### `apps/desktop/src/main/close-behavior.ts`

Main-process implementation: the on-disk prefs reader/writer
(`loadClosePreferences`, `saveClosePreferences`) and the close interception
(`applyCloseBehavior`). The module owns `isQuitting` semantics — every quit
path (Cmd+Q, menu Quit, tray Quit, renderer prompt choosing Quit) flips it
via `setIsQuitting`, and the close listener short-circuits when it's true.

### `apps/desktop/src/main/tray.ts`

The TrayHandle. Electron's `Tray` has no `setVisible`; "show" means
"construct the Tray", "hide" means "destroy the Tray". The handle is
constructed in `whenReady` AFTER `createWindow()` and stays inert until
`show()` runs.

`isTrayEnvironmentSupported()` reads `XDG_CURRENT_DESKTOP` +
`XDG_SESSION_TYPE` to disable the minimize affordance on GNOME 40+ Wayland
without AppIndicator. This heuristic is NOT authoritative — the Tray
constructor may still succeed at runtime even when the user can't see the
icon — so settings also hides the option when `trayHandle.isSupported()`
returns false (the constructor fails or was never attempted).

### `apps/desktop/src/preload/index.ts`

Exposes `window.closeBehaviorAPI` (get / set / isTraySupported / onPrompt /
respond) on the renderer. Channel names come from the shared contract.

### `apps/desktop/src/renderer/src/components/close-behavior-prompt.tsx`

The AlertDialog rendered on first close under `ask`. Mounted once inside
`CoreProvider` in `App.tsx`. Dedupes double-respond via `respondedRef`
because clicking Cancel collapses the dialog → triggers
`onOpenChange(false)` → would double-fire `respond(...)`.

### `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.tsx`

The Settings tab hosting the behavior selector. Uses `<Select>` (the Base
UI primitive) with an `items` array; when tray is unsupported the minimize
option is omitted (not just disabled) so it can't be selected.

Registered in `apps/desktop/src/renderer/src/routes.tsx` via
`DesktopSettingsRoute.extraAccountTabs`.

---

## i18n

The renderer-side strings live in a dedicated `desktop.json` namespace:

```
packages/views/locales/en/desktop.json
packages/views/locales/zh-Hans/desktop.json
```

The namespace is registered in:

- `packages/views/locales/index.ts` — both locales' `RESOURCES.desktop`.
- `packages/views/i18n/resources-types.ts` — module augmentation on
  `I18nResources.desktop`.

These strings are NOT plural keys (no `_one` / `_other`) so they comply
with the CJK locales rejection rule documented in auto-memory.

Additional tab labels under `settings.desktop.tabs.behavior` extend the
existing settings namespace; do not move them into `desktop.json`.

---

## Failure Modes

| Failure                                  | Behavior |
|------------------------------------------|----------|
| `close-preferences.json` missing / corrupt / wrong shape | `loadClosePreferences` returns `"ask"`. The next set overwrites the file. |
| Renderer hangs during the prompt         | 5s timeout in `requestCloseBehaviorPrompt` → forced quit, single warn. |
| Tray constructor fails (rare)            | `createTray` returns null; `trayHandle.isSupported()` is `false`; the minimize path falls back to quit with a single warn. |
| `XDG_CURRENT_DESKTOP=GNOME` + `XDG_SESSION_TYPE=wayland` | `isTrayEnvironmentSupported()` returns false; `createTray` returns null; settings hides minimize; closing with a stale `minimize` pref falls back to real close with a single warn. |
| Prefs write fails                        | `close-behavior:set` returns `{ ok: false, reason: "persist_failed" }`; settings re-fetches the on-disk value. |
| `before-quit` fires before the prompt responds | `isQuitting = true` short-circuits the close listener, no double-prompt. |

---

## Compatibility

- macOS behavior is unchanged byte-for-byte.
- The daemon prefs (`~/.multica/desktop_prefs.json`) are not touched.
- No DB migrations, no server changes.
- Installed clients without this feature still work — the new IPC channels
  are delivered but unused, and persisted prefs irrelevant.

---

## Testing

- Main-process unit tests live beside the implementation:
  - `apps/desktop/src/main/close-behavior.test.ts` (persistence round-trip,
    corrupt-file fallback, all close-interception branches).
  - `apps/desktop/src/main/tray.test.ts` (environment heuristic matrix,
    path resolution on dev/prod, windows/linux).
- Renderer tests:
  - `apps/desktop/src/renderer/src/components/close-behavior-prompt.test.tsx`
    (four respond paths, tray-unsupported hiding the Minimize button,
    listener unsubscription).
- i18n parity is enforced by `packages/views/locales/parity.test.ts` — the
  new namespace is subject to it.
- Real-machine verification (Windows 10/11, Ubuntu 22.04 GNOME + KDE/XFCE)
  is owned by `.trellis/tasks/09-30-real-machine-verification/` and not
  covered by Vitest.
