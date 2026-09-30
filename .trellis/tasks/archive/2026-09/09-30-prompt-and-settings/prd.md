# Child 3: prompt-and-settings

## Scope

Renderer-side UX: the AlertDialog modal on first close, the Settings tab to
change the behavior later, plus the preload bridge between main and
renderer. Depends on child 1 (close-prefs-and-intercept) — its IPC handlers
and module state are consumed here. Also expects child 2 to land first if
you want to test the minimize button; otherwise the minimize button renders
disabled because `isTraySupported()` returns false. Implements F5, F6 from
the parent PRD.

## Deliverables

1. `apps/desktop/src/preload/index.ts` — add `closeBehaviorAPI`:

   ```ts
   import type { CloseBehavior, PromptResult } from "../main/close-behavior";

   const closeBehaviorAPI = {
     get: (): Promise<CloseBehavior> =>
       ipcRenderer.invoke("close-behavior:get"),
     set: (value: CloseBehavior): Promise<{ ok: boolean; reason?: string }> =>
       ipcRenderer.invoke("close-behavior:set", value),
     isTraySupported: (): Promise<boolean> =>
       ipcRenderer.invoke("close-behavior:is-tray-supported"),
     onPrompt: (handler: (req: { requestId: string }) => void): (() => void) => {
       const listener = (_e: unknown, req: { requestId: string }) => handler(req);
       ipcRenderer.on("close-behavior:prompt", listener);
       return () => { ipcRenderer.removeListener("close-behavior:prompt", listener); };
     },
     respond: (requestId: string, result: PromptResult): void =>
       ipcRenderer.send("close-behavior:respond", { requestId, ...result }),
   };
   contextBridge.exposeInMainWorld("closeBehaviorAPI", closeBehaviorAPI);
   ```

   Also update `apps/desktop/src/preload/index.d.ts` with the matching
   `Window.closeBehaviorAPI` typing.

2. `apps/desktop/src/renderer/src/components/close-behavior-prompt.tsx`
   (new, desktop-only):
   - Mounted once at the root of `App.tsx` (next to existing providers).
   - Calls `window.closeBehaviorAPI.onPrompt(({ requestId }) => setOpen(requestId))`
     once on mount, cleanup on unmount.
   - Renders `AlertDialog` from `@multica/ui` (matches existing modal
     patterns; check `packages/views/modals/quick-create-actor-picker.tsx`
     for canonical structure but DO NOT import from views — this component
     stays in the desktop renderer to keep platform plumbing separate).
   - Title + description via `useT("desktop")` (namespace `desktop`, see
     i18n section).
   - Three actions: Quit (destructive) / Minimize to tray (default) /
     Cancel (ghost). A "Remember my choice" checkbox above the action row.
   - Minimize button is `disabled` when `isTraySupported()` returns false
     (fetched once on mount via `window.closeBehaviorAPI.isTraySupported()`).
     If disabled, also show a tooltip / helper text explaining why.
   - On click: `respond(requestId, { action, remember })` and `setOpen(null)`.
   - A11y: ESC key = Cancel; focus traps inside dialog (handled by
     `AlertDialog` Base UI primitive).

3. `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.tsx`
   (new): Settings tab hosting a radio group of the three behaviors.
   - Modelled on `runtime-config-settings-tab.tsx` — same `<SettingsTab>`
     wrapper, similar form spacing.
   - Uses `RadioGroup` from `@multica/ui` if available, else three
     `<label><input type="radio">` rows matching the existing settings form
     patterns; do not introduce a new component library.
   - On mount: load current value via `window.closeBehaviorAPI.get()`;
     load tray support flag via `window.closeBehaviorAPI.isTraySupported()`.
   - On change: `window.closeBehaviorAPI.set(next)`; show a brief inline
     success indicator or toast — match existing settings conventions
     (`runtime-config-settings-tab.tsx` shows recent save state).
   - Minimize option is `disabled`+tooltip when tray unsupported.

4. `apps/desktop/src/renderer/src/routes.tsx` — register the new settings
   component in whatever settings tree the desktop currently uses (see how
   `runtime-config-settings-tab` is wired).

5. `apps/desktop/src/renderer/src/App.tsx` — mount
   `<CloseBehaviorPrompt />` next to the other global handlers.

6. i18n strings — new namespace `desktop.json` in
   `packages/views/locales/en/desktop.json` and
   `packages/views/locales/zh-Hans/desktop.json`:

   ```
   desktop.close_behavior.title              "When closing Multica"
   desktop.close_behavior.description        "Choose what happens when you close the main window."
   desktop.close_behavior.quit               "Quit Multica"
   desktop.close_behavior.minimize           "Minimize to system tray"
   desktop.close_behavior.ask                "Ask every time"
   desktop.close_behavior.unsupported        "Your system does not support a system tray, so this option is disabled."
   desktop.close_behavior.prompt.title       "Close Multica?"
   desktop.close_behavior.prompt.description "Quit entirely, or keep Multica running in the system tray?"
   desktop.close_behavior.prompt.remember    "Remember my choice"
   desktop.close_behavior.prompt.quit        "Quit"
   desktop.close_behavior.prompt.minimize    "Minimize to tray"
   desktop.close_behavior.prompt.cancel      "Cancel"
   ```

   zh-Hans translations follow the glossary in
   `apps/docs/content/docs/developers/conventions.zh.mdx` (verify
   "system tray" 译法 against existing strings — likely "系统托盘").
   No `_one` / `_other` plural keys here; this avoids the CJK plural-key
   rejection issue documented in auto-memory.

7. `apps/desktop/src/renderer/src/components/close-behavior-prompt.test.tsx`
   (new, vitest + jsdom):
   - Shows the prompt on `close-behavior:prompt` event.
   - Respond called with `{ action: "quit", remember: false }` when Quit is
     clicked without remember.
   - Respond called with `{ action: "minimize", remember: true }` when
     Minimize is clicked with the checkbox checked.
   - Respond called with `{ action: "ask", remember: false }` on Cancel.
   - Minimize button disabled when `closeBehaviorAPI.isTraySupported()`
     resolves false.

## Out of scope

- Tray icon implementation (child 2).
- Real-machine packaging + verification (child 4).
- macOS behavior changes (none).

## Acceptance Criteria

- A/C 3.1: All listed unit tests pass: `pnpm -F desktop test` (or
  `vitest run src/renderer`).
- A/C 3.2: Desktop typecheck passes (web + node configs).
- A/C 3.3: Manual dev mode walkthrough: launch `pnpm dev:desktop`, kill
  the prefs file, close the window → prompt appears; check Remember,
  choose Quit → next close skips the prompt; select Minimize + Remember,
  then the next close hides the window (and on unsupported tray, falls
  back to a real close with a console warn).
- A/C 3.4: Settings tab loads current value; changing it persists across
  restart.
- A/C 3.5: zh-Hans translation reviewed against the glossary; no plural
  keys introduced. The `i18n parity dead-plural guard` continues to pass
  (`pnpm -F @multica/views test` if such a guard exists).

## Notes

- This component never runs on web (desktop-only by file location + import
  source: it lives under `apps/desktop/`, not under `packages/views/`).
- IPC `close-behavior:prompt` is single-flight for the main window. The
  renderer-side handler should likewise treat multiple prompts as
  no-ops once one is open (singleton).
