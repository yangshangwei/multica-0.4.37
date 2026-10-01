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
| `quit`    | Prevent the window-only close, then request `app.quit()` so every window exits through daemon cleanup. |
| `minimize`| Prevent close synchronously, await successful tray creation, then hide the window. App and daemon continue running. |
| `ask`     | Default for new profiles. Render-side AlertDialog asks the user; the response is applied and optionally persisted. |

`applyCloseBehavior` early-returns on darwin, preserving native window close /
activate behavior. The behavior settings tab is registered only for Windows
and Linux; macOS must not offer preferences that have no effect.

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

`loadClosePreferenceStore` reads the file once before window creation. Both
settings and remembered prompt choices use its serialized `set` operation.
The cache changes only after atomic tmp+rename succeeds; a failed save leaves
the last committed value readable and does not block subsequent saves.

`event.preventDefault()` MUST run synchronously inside the close listener.
Prompt, save, and tray checks may complete asynchronously after interception.
Recheck quitting/window liveness after awaits and deduplicate repeated closes.

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
(`applyCloseBehavior`). Main owns `isQuitting`; every explicit quit and
`before-quit` sets it. Interception invokes `quitApp`, never just
`mainWindow.close()`, because independent issue windows may remain alive.

### `apps/desktop/src/main/close-behavior-prompt.ts`

The prompt coordinator assigns unique IDs and validates both the originating
WebContents and request ID on acknowledgements and responses. Only delivery
has a five-second timeout. After the renderer acknowledges mounting the
dialog, the user may take unlimited time to choose.

Cancel and release listeners on app exit, renderer destruction/crash, or
main-frame non-same-document navigation. Same-document and subframe navigation
must not cancel a visible prompt. Actual window unresponsiveness fails the
request; elapsed user decision time does not imply a hung renderer.

### `apps/desktop/src/main/daemon-quit.ts`

Prevent `before-quit` synchronously before reading preferences. Repeated quit
requests also wait while cleanup is pending. Drain existing daemon lifecycle
operations before the final optional stop, prevent new daemon launches after
shutdown starts, then mark cleanup ready before calling `app.quit()` again.
Cleanup failures are logged and still allow application exit.

### `apps/desktop/src/main/tray.ts`

The TrayHandle. Electron's `Tray` has no `setVisible`; "show" means
"construct the Tray", "hide" means "destroy the Tray". The handle is
constructed in `whenReady` AFTER `createWindow()` and stays inert until
`show()` runs.

`isSupported(): Promise<boolean>` never constructs a Tray. Linux support is
queried in `tray-support.ts` through the session D-Bus property
`org.kde.StatusNotifierWatcher.IsStatusNotifierHostRegistered`. Use bounded
`gdbus`, or `busctl` when gdbus is absent; missing tools, missing host, malformed
output and query failures report unsupported. Desktop name and X11/Wayland
alone do not establish support. Legacy-only trays without a StatusNotifier
host are conservatively unsupported.

`show(): Promise<boolean>` rechecks support and returns true only after icon
and menu creation succeed. Hide the window only after true. `hide`/`destroy`
invalidate pending creations; partial setup failures destroy the candidate.
Attach the show-event teardown inside `createWindow` so recreated windows
also remove the tray when restored.

### `apps/desktop/src/preload/index.ts`

Exposes `window.closeBehaviorAPI` (get / set / isTraySupported / onPrompt /
acknowledge / respond). Channel names come from the shared contract.

### `apps/desktop/src/renderer/src/components/close-behavior-prompt.tsx`

The AlertDialog rendered on first close under `ask`. Mounted once inside
`CoreProvider` in `App.tsx`. A pending-request ref deduplicates button,
dialog-close and component-unmount responses. Unmount cancels any unanswered
request, including error-boundary recovery. Acknowledge after rendering and
recheck tray support on every opening; hide Minimize until support is confirmed.

### `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.tsx`

The Settings tab hosting the behavior selector. Uses `<Select>` (the Base
UI primitive) with a complete translated `items` label map. When tray is
unsupported, omit only the rendered minimize option so it cannot be selected.
Keep its label in `items`: a preference saved in a previous supported session
still needs to display its translated name without rewriting the preference.

Registered by `components/desktop-settings-route.tsx`, imported by `routes.tsx`,
only when `desktopAPI.appInfo.os` is `windows` or `linux`.

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
| Prompt delivery not acknowledged within 5s / window becomes unresponsive | Request rejects; close interception requests application exit. |
| User reads an acknowledged prompt for more than 5s | Dialog remains open; no automatic exit. |
| Renderer reload/crash or prompt component unmount | Cancel pending choice; next close can request a fresh prompt. |
| Tray host missing or construction fails | `show()` returns false; never hide an inaccessible window; fall back to application exit with a warning. |
| Prefs write fails | Return `{ ok: false, reason: "persist_failed" }`; settings re-fetches the unchanged committed value. |
| `before-quit` fires before the prompt responds | Mark quitting, cancel pending prompt, and wait for daemon cleanup; no double-prompt. |

---

## Compatibility

- macOS retains native close/activate behavior; its ineffective settings tab is removed.
- The daemon prefs (`~/.multica/desktop_prefs.json`) are not touched.
- No DB migrations, no server changes.
- Installed clients without this feature still work — the new IPC channels
  are delivered but unused, and persisted prefs irrelevant.

---

## Testing

- Main-process unit tests live beside the implementation:
  - `apps/desktop/src/main/close-behavior.test.ts` (persistence round-trip,
    corrupt-file fallback, all close-interception branches).
  - `apps/desktop/src/main/close-behavior-prompt.test.ts` (delivery deadline,
    unlimited user response time, correlation, crash/reload cleanup).
  - `apps/desktop/src/main/tray.test.ts` (capability query side effects,
    construction failure, cancellation, restore/quit menu actions).
  - `apps/desktop/src/main/tray-support.test.ts` (session D-Bus result matrix,
    missing tools, bounded failures, desktop/session independence).
  - `apps/desktop/src/main/daemon-quit.test.ts` (synchronous interception,
    repeated quit, lifecycle ordering, autoStop and failure handling).
- Renderer tests:
  - `apps/desktop/src/renderer/src/components/close-behavior-prompt.test.tsx`
    (four respond paths, tray-unsupported hiding the Minimize button,
    acknowledgement, unmount cancellation and listener unsubscription).
  - `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.test.tsx`
    (translated saved minimize label with no tray, filtered choices and no
    automatic preference rewrite).
  - `apps/desktop/src/renderer/src/components/desktop-settings-route.test.tsx`
    (Windows/Linux presence and macOS/unknown absence).
- i18n parity is enforced by `packages/views/locales/parity.test.ts` — the
  new namespace is subject to it.
- Real-machine verification (Windows 10/11, Ubuntu 22.04 GNOME + KDE/XFCE)
  is owned by `.trellis/tasks/09-30-real-machine-verification/` and not
  covered by Vitest.
