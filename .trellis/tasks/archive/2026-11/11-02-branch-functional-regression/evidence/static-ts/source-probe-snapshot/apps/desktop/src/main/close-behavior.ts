import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import type { BrowserWindow } from "electron";
import {
  DEFAULT_CLOSE_BEHAVIOR,
  isCloseBehavior,
  type CloseBehavior,
} from "../shared/close-behavior";

/**
 * close-behavior.ts (main process implementation)
 *
 * Owns the user's preference for what should happen when the main window's
 * close button is pressed on Windows / Linux (macOS keeps the OS-native
 * "close hides, activate reopens" behavior and doesn't consult this module).
 *
 * Persistence lives alongside the other Electron-managed prefs under
 * `app.getPath("userData")/close-preferences.json`, deliberately separate
 * from `~/.multica/desktop_prefs.json` (daemon prefs).
 *
 * Types and channel constants are re-exported from `shared/close-behavior.ts`
 * so preload can import the same contract without crossing into main/.
 */

export type { CloseBehavior };
export { DEFAULT_CLOSE_BEHAVIOR };

export function closePreferencesPath(userDataPath: string): string {
  return join(userDataPath, "close-preferences.json");
}

function parseCloseBehavior(value: unknown): CloseBehavior {
  if (
    typeof value === "object" &&
    value !== null &&
    isCloseBehavior((value as { closeBehavior?: unknown }).closeBehavior)
  ) {
    return (value as { closeBehavior: CloseBehavior }).closeBehavior;
  }
  return DEFAULT_CLOSE_BEHAVIOR;
}

export async function loadClosePreferences(
  filePath: string,
): Promise<CloseBehavior> {
  try {
    const raw = await readFile(filePath, "utf-8");
    return parseCloseBehavior(JSON.parse(raw));
  } catch {
    return DEFAULT_CLOSE_BEHAVIOR;
  }
}

export async function saveClosePreferences(
  filePath: string,
  value: CloseBehavior,
): Promise<void> {
  await mkdir(dirname(filePath), { recursive: true });
  const temporaryPath = `${filePath}.tmp`;
  await writeFile(
    temporaryPath,
    JSON.stringify({ closeBehavior: value }, null, 2),
    "utf-8",
  );
  await rename(temporaryPath, filePath);
}

export interface ClosePreferenceStore {
  get(): CloseBehavior;
  set(value: CloseBehavior): Promise<void>;
}

/** Serialize writers and publish only preferences that reached disk. */
export async function loadClosePreferenceStore(
  filePath: string,
): Promise<ClosePreferenceStore> {
  let committed = await loadClosePreferences(filePath);
  let pending = Promise.resolve();
  return {
    get: () => committed,
    set(value) {
      const save = pending.then(async () => {
        await saveClosePreferences(filePath, value);
        committed = value;
      });
      pending = save.catch(() => {});
      return save;
    },
  };
}

/**
 * Result returned by the renderer-driven prompt modal. `action === "ask"`
 * means the user chose Cancel and no behavior should change.
 *
 * Re-exported from shared so the preload surface uses the same shape.
 */
export type { CloseBehaviorPromptResult as PromptResult } from "../shared/close-behavior";
import type { CloseBehaviorPromptResult as PromptResult } from "../shared/close-behavior";

export interface ApplyCloseBehaviorOptions {
  mainWindow: BrowserWindow;
  getIsQuitting: () => boolean;
  /** Marks quitting and exits the whole app, including auxiliary windows. */
  quitApp: () => void;
  /** Resolves true only when an icon can provide a way back to the window. */
  showTray: () => Promise<boolean>;
  promptChoice: (w: BrowserWindow) => Promise<PromptResult>;
  getCachedBehavior: () => CloseBehavior;
  setCachedBehavior: (value: CloseBehavior) => Promise<void>;
}

/** Prevent close synchronously; finish any asynchronous decision before hiding. */
export function applyCloseBehavior(opts: ApplyCloseBehaviorOptions): void {
  const {
    mainWindow,
    getIsQuitting,
    quitApp,
    showTray,
    promptChoice,
    getCachedBehavior,
    setCachedBehavior,
  } = opts;

  if (process.platform === "darwin") return;

  let handlingClose = false;
  mainWindow.on("close", (event) => {
    if (getIsQuitting()) return;
    event.preventDefault();
    if (handlingClose) return;
    handlingClose = true;
    void (async () => {
      try {
        const behavior = getCachedBehavior();
        const result = behavior === "ask"
          ? await promptChoice(mainWindow)
          : { action: behavior, remember: false };
        if (getIsQuitting() || mainWindow.isDestroyed() || result.action === "ask") return;
        if (result.remember) {
          try {
            await setCachedBehavior(result.action);
          } catch (err) {
            console.warn("[close-behavior] failed to persist choice", err);
          }
        }
        if (getIsQuitting() || mainWindow.isDestroyed()) return;
        if (result.action === "minimize") {
          const shown = await showTray();
          if (getIsQuitting() || mainWindow.isDestroyed()) return;
          if (shown) {
            mainWindow.hide();
            return;
          }
          console.warn(
            "[close-behavior] tray unavailable; quitting instead of hiding",
          );
        }
        quitApp();
      } catch (err) {
        console.warn("[close-behavior] prompt failed; quitting", err);
        if (!getIsQuitting() && !mainWindow.isDestroyed()) quitApp();
      } finally {
        handlingClose = false;
      }
    })();
  });
}
