# Design: Desktop Close Behavior

## Architecture

```
main process (Electron)
├─ close-behavior.ts        NEW — prefs + close interception glue
├─ tray.ts                  NEW — Tray lifecycle + menu
├─ index.ts                 MOD — wire close interception, second-instance, isQuitting flag
└─ updater-preferences.ts  (pattern source; untouched)

preload
└─ index.ts                 MOD — expose closeBehaviorAPI

renderer (apps/desktop only — NOT packages/views)
└─ src/components/
    ├─ close-behavior-prompt.tsx     NEW — AlertDialog modal listening for prompt IPC
    └─ desktop-behavior-settings-tab.tsx  NEW — radiogroup settings tab

build
├─ resources/tray-icon.ico           NEW (Windows)
├─ resources/tray-iconTemplate.png   NEW (Linux, template-style for menubar)
└─ electron-builder.yml              MOD — extraResources additions
```

No changes to `packages/core`, `packages/ui`, `packages/views`, or the Go
server. This is a desktop-platform feature.

## Data Flow

### Prefs: load → decide → act

```
app.whenReady()
   └─* createWindow()
   │    └─* new BrowserWindow(...)
   │    └─* close-behavior.applyCloseBehavior(mainWindow, prefsOrNull)
   │           mainWindow.on("close", async (event) => {
   │             if (darwin) return;
   │             if (isQuitting) return;                          // explicit quit
   │             const pref = await loadClosePreferences(userDataPath);
   │
   │             if (pref === "minimize" && tray.isSupported()) {
   │               event.preventDefault();
   │               mainWindow.hide(); tray.show();
   │               return;
   │             }
   │             if (pref === "minimize" && !tray.isSupported()) {
   │               console.warn("tray unsupported; falling back to quit");
   │               return; // let close proceed
   │             }
   │             if (pref === "ask") {
   │               event.preventDefault();
   │               const choice = await promptRendererChoice(mainWindow);  // IPC, 5s timeout
   │               if (choice.remember) saveClosePreferences(...);
   │               applyChoice(choice.action); // may set isQuitting before quit
   │             }
   │             // "quit" falls through; default close path runs
   │           });
```

### Prompt IPC round-trip (requestId correlation)

```
main                                         renderer
 │                                             │
 ├── "close-behavior:prompt" ─ requestId ──►  onPrompt handler
 │                                             │  mount AlertDialog
 │  ◄──── "close-behavior:respond" ──────────  user picks { action, remember }
 │
 ▼ applyChoice(...)
```

Timeout: if no `respond` within 5000ms, treat as `quit` and log. This is the
only path that calls `app.exit()` directly (skipping renderer gracefully
shutdown) if `mainWindow.isDestroyed()` mid-flight; everything else goes
through `app.quit()`.

### Tray lifecycle

```
createTray(mainWindow)           called once after window construction
  ├─ if linux gnome-no-appindicator → return null
  ├─ tray = new Tray(iconPath)
  ├─ tray.setContextMenu(Menu.buildFromTemplate([Show, Quit]))
  ├─ tray.on("click", show)
  └─ return tray

window.on("close") with pref=minimize → tray.setVisible(true); window.hide()
window.on("restore")                → tray.setVisible(false)
app.on("before-quit")               → tray.destroy()
```

Tray icon is **only shown when the window is hidden**; otherwise the dock /
taskbar icon is sufficient. (This matches Discord / Slack and avoids the
"two icons for one app" smell.)

## File-by-file changes

### 1. `apps/desktop/src/main/close-behavior.ts` (new)

```ts
export type CloseBehavior = "quit" | "minimize" | "ask";
export const DEFAULT_CLOSE_BEHAVIOR: CloseBehavior = "ask";

export function closePreferencesPath(userDataPath: string): string;
export async function loadClosePreferences(filePath: string): Promise<CloseBehavior>;
export async function saveClosePreferences(filePath: string, value: CloseBehavior): Promise<void>;

export interface ApplyOptions {
  mainWindow: BrowserWindow;
  getTray: () => Tray | null;
  setIsQuitting: () => void;
  promptChoice: (w: BrowserWindow) => Promise<{ action: CloseBehavior; remember: boolean }>;
}
export function applyCloseBehavior(opts: ApplyOptions): void;
```

`loadClosePreferences` mirrors `loadUpdaterPreferences` (apps/desktop/src/main/updater-preferences.ts):
- `readFile` try/catch → default
- `JSON.parse` try/catch → default
- Shape guard returns default for unknown strings

### 2. `apps/desktop/src/main/tray.ts` (new)

```ts
export interface TrayHandle {
  show(): void;
  hide(): void;
  destroy(): void;
  isSupported(): boolean;    // false if constructor returned null
}

export function buildTrayIconPath(resourcesPath: string): string;
export function isTraySupported(): boolean;     // linux gnome+no-appindicator → false
export function createTray(mainWindow: BrowserWindow, opts: {
  resourcesPath: string;
  onShow: () => void;
  onQuit: () => void;
}): TrayHandle | null;
```

Detection heuristic for `isTraySupported()` on Linux:
- `process.platform !== "linux"` → true
- `process.env.XDG_CURRENT_DESKTOP` lowercase contains "gnome" → check `process.env.XDG_SESSION_TYPE === "wayland"` → false (Wayland GNOME without extensions)
- Otherwise true (KDE, XFCE, Cinnamon, MATE, legacy X11 GNOME all work)

This is heuristic, not authoritative; AC4 covers the runtime-failure fallback.

### 3. `apps/desktop/src/main/index.ts` (modified)

- Import `close-behavior` and `tray`.
- Add module-level `let isQuitting = false`.
- `app.on("before-quit")` sets `isQuitting = true` (existing logic, just augmented).
- In `createWindow()`, after existing listeners, call `applyCloseBehavior(...)`.
- Update `window-all-closed` so it still calls `app.quit()` only when NOT minimized-hidden. Concretely:
  ```ts
  app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
      const pref = /* current pref, sync via cache */ ;
      if (pref === "minimize" && trayHandle) return; // app intentionally still running
      app.quit();
    }
  });
  ```
- Update `second-instance` handler: in addition to `focus()`, call
  `mainWindow.show()` and `mainWindow.restore()` if `!mainWindow.isVisible()`.

The pref read at `window-all-closed` time should reuse the cached value
from `applyCloseBehavior`'s immediate-load, not hit the disk again.

### 4. `apps/desktop/src/preload/index.ts` (modified)

```ts
const closeBehaviorAPI = {
  get: (): Promise<CloseBehavior> =>
    ipcRenderer.invoke("close-behavior:get"),
  set: (value: CloseBehavior): Promise<void> =>
    ipcRenderer.invoke("close-behavior:set", value),
  isTraySupported: (): Promise<boolean> =>
    ipcRenderer.invoke("close-behavior:is-tray-supported"),
  onPrompt: (handler: (req: PromptRequest) => void): (() => void) => {
    const listener = (_e, req) => handler(req);
    ipcRenderer.on("close-behavior:prompt", listener);
    return () => ipcRenderer.removeListener(...);
  },
  respond: (requestId: string, result: { action: CloseBehavior; remember: boolean }): void =>
    ipcRenderer.send("close-behavior:respond", { requestId, ...result }),
};
contextBridge.exposeInMainWorld("closeBehaviorAPI", closeBehaviorAPI);
```

### 5. `apps/desktop/src/renderer/src/components/close-behavior-prompt.tsx` (new)

- Registered once in `App.tsx`.
- Calls `window.closeBehaviorAPI.onPrompt(({ requestId }) => setOpen(requestId))`.
- Renders `AlertDialog` from `@multica/ui` with title+description from i18n (`desktop.close_behavior.prompt.*`).
- Three buttons (Quit / Minimize / Cancel) + "Remember my choice" checkbox.
- On Quit/Minimize click: `respond(requestId, { action, remember })` and clear open state.
- On Cancel click: `respond(requestId, { action: "ask", remember: false })` (re-ask next time) and clear open state.
- If tray unsupported and the user picks Minimize anyway (shouldn't happen —
  main guards first — but defensive), show a brief toast notification
  explaining fallback.

The component only renders on desktop; it imports nothing from
`packages/views` beyond i18n helpers, satisfying the package-boundary rule
(shared views have no `electron` access).

### 6. `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.tsx` (new)

Follows the pattern from `runtime-config-settings-tab.tsx`:

```tsx
export function DesktopBehaviorSettingsTab() {
  const { t } = useT("desktop"); // or "settings" — see i18n decision below
  const [behavior, setBehavior] = useState<CloseBehavior>("ask");
  const [traySupported, setTraySupported] = useState(true);

  useEffect(() => {
    window.closeBehaviorAPI.get().then(setBehavior);
    window.closeBehaviorAPI.isTraySupported().then(setTraySupported);
  }, []);

  return (
    <SettingsTab title=... description=...>
      <RadioGroup value={behavior} onValueChange={v => window.closeBehaviorAPI.set(v)}>
        <RadioGroup.Item value="quit" ... />
        <RadioGroup.Item value="minimize" disabled={!traySupported} ... />
        <RadioGroup.Item value="ask" ... />
      </RadioGroup>
      {!traySupported && <p className="text-caption text-muted-foreground">{t(...unsupported)}</p>}
    </SettingsTab>
  );
}
```

Wired into `apps/desktop/src/renderer/src/routes.tsx` (or wherever the
desktop Settings tree is composed) alongside the existing
`runtime-config-settings-tab.tsx`.

### 7. i18n strings

Where do desktop-only strings live?

**Decision**: keep them in `packages/views/locales/{en,zh-Hans}/desktop.json`
as a new namespace.

Rationale: the renderer already imports `useT` from `@multica/views/i18n`; adding a
new namespace avoids touching unrelated files (`settings.json`, `modals.json`)
that have unrelated dirty diffs in the current git status. It's clearly
desktop-scoped, the URL routing decision (`/settings/desktop-behavior`) lives
in the desktop app only, and the desktop packaging ships these strings
already — there's no real decoupling gain from inventing a new
`apps/desktop/locales/` tree for a handful of strings.

Strings needed (English source; zh-Hans translated):

```
desktop.close_behavior.title              "When closing Multica"
desktop.close_behavior.description        "Choose what happens when you close the main window."
desktop.close_behavior.quit               "Quit Multica"
desktop.close_behavior.minimize           "Minimize to system tray"
desktop.close_behavior.ask                "Ask every time"
desktop.close_behavior.unsupported        "Your system tray is unavailable, so minimize is disabled."
desktop.close_behavior.prompt.title       "Close Multica?"
desktop.close_behavior.prompt.description "Do you want to quit Multica or keep it running in the system tray?"
desktop.close_behavior.prompt.remember    "Remember my choice"
desktop.close_behavior.prompt.quit        "Quit"
desktop.close_behavior.prompt.minimize    "Minimize to tray"
desktop.close_behavior.prompt.cancel      "Cancel"
```

**Per project conventions (CLAUDE.md UI Rules + memory note
`i18n-cjk-locales-reject-one-plural-key`), these are not plural keys**, so
no `_one` / `_other` complexity. Just plain string keys.

### 8. Resources + electron-builder.yml

Add under `apps/desktop/resources/`:
- `tray-icon.ico` — multi-size (16/32) `.ico`. Reuse `icon.png`'s artwork,
  downscaled. (Check whether there's a `build/icon.ico` already; reuse if present.)
- `tray-iconTemplate.png` — 22x22 (Linux), with `@2x` variant.

Update `electron-builder.yml`:

```yaml
extraResources:
  - from: resources/tray-icon.ico
    to: tray-icon.ico
  - from: resources/tray-iconTemplate.png
    to: tray-iconTemplate.png
```

(Existing `extraResources:` block already includes daemon binaries; extend it.)

Main process resolves icons at runtime:

```ts
const iconPath = is.dev
  ? join(__dirname, "../../resources/tray-icon.ico")          // or .png
  : join(process.resourcesPath, "tray-icon.ico");
```

## Concurrency & state

**Prefs cache**: `applyCloseBehavior` loads prefs once at window creation.
Settings updates write to disk **and** notify the in-memory cache via the
`close-behavior:set` IPC handler so a subsequent close sees the fresh value
without re-reading disk. This is a single-writer, single-reader model — safe.

**isQuitting flag**: set in:
1. `app.on("before-quit")` — covers Cmd+Q, menu Quit, OS-level shutdown
   signals, and tray menu Quit.
2. The prompt handler when the user picks Quit (need to mark before calling
   `app.quit()` so the close interception doesn't double-prompt).

**Multiple windows**: Multica Desktop currently has a single main window
plus issue-window children. The close interception is applied **only to the
main window**; child windows keep their existing behavior (close = destroy).
Decision recorded here: closing an issue window must never minimize anything;
only the main window owns the tray.

## Failure modes

| Failure | Behavior |
|---|---|
| Prefs file corrupt / missing | `loadClosePreferences` returns `"ask"` (default). Settings still works (set overwrites). |
| Renderer hangs during prompt | 5s timeout → force `quit`, log a single warn. App exits. |
| `Tray` constructor throws on Linux | `createTray` returns null. Settings UI disabled via `is-tray-supported`. Pref `minimize` falls back to `quit` at close time. |
| Prefs write fails | `saveClosePreferences` throws → caught in IPC handler → settings UI shows an error toast (or silently retries; surface detail TBD in implement) |
| `before-quit` fires before prompt responds (rare race) | `isQuitting = true` short-circuits close interception. |

## Compatibility

- `~/.multica/desktop_prefs.json` untouched.
- No server-side changes.
- No DB migrations.
- Backwards-compatible with existing installs: the new `close-preferences.json`
  simply doesn't exist → default `ask` → first close shows the prompt.

## Testing strategy

### Unit (Vitest, in main/)

- `close-behavior.test.ts`: load/save/parse/fallback paths.
- `tray.test.ts`: `buildTrayIconPath` platform branches; `isTraySupported`
  heuristic for XDG_CURRENT_DESKTOP variants (mock process.env).
- Integration of `applyCloseBehavior` with a stubbed `BrowserWindow` /
  `Tray` is kept light — Electron's own types make this fiddly; an
  integration test using a fake-EventEmitter pattern is acceptable.

### Renderer (Vitest + jsdom, in renderer/)

- `close-behavior-prompt.test.tsx`: prompt shows on event; respond called
  with correct values; remember-toggle behavior.

### E2E

Skip. Tray behaviors don't reproduce reliably under Playwright against a
headless Electron; verified manually per AC.

### Real-machine verification

See `09-30-real-machine-verification` prd.
