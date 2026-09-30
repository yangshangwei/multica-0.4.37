import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import type { BrowserWindow } from "electron";

/**
 * close-behavior.ts
 *
 * Owns the user's preference for what should happen when the main window's
 * close button is pressed on Windows / Linux (macOS keeps the OS-native
 * "close hides, activate reopens" behavior and doesn't consult this module).
 *
 * Persistence lives alongside the other Electron-managed prefs under
 * `app.getPath("userData")/close-preferences.json`, deliberately separate
 * from `~/.multica/desktop_prefs.json` (daemon prefs).
 */

export type CloseBehavior = "quit" | "minimize" | "ask";

export const DEFAULT_CLOSE_BEHAVIOR: CloseBehavior = "ask";

const VALID_BEHAVIORS: ReadonlySet<string> = new Set([
  "quit",
  "minimize",
  "ask",
]);

export function closePreferencesPath(userDataPath: string): string {
  return join(userDataPath, "close-preferences.json");
}

function parseCloseBehavior(value: unknown): CloseBehavior {
  if (
    typeof value === "object" &&
    value !== null &&
    typeof (value as { closeBehavior?: unknown }).closeBehavior === "string" &&
    VALID_BEHAVIORS.has((value as { closeBehavior: string }).closeBehavior)
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

/**
 * Result returned by the renderer-driven prompt modal. `action === "ask"`
 * means the user chose Cancel and no behavior should change.
 */
export interface PromptResult {
  action: CloseBehavior;
  remember: boolean;
}

export interface ApplyCloseBehaviorOptions {
  mainWindow: BrowserWindow;
  /**
   * Reads the current "user really wants to exit" flag. When true the close
   * handler must not intercept — every quit path (Cmd+Q, menu Quit, tray
   * Quit, renderer prompt chosing Quit) flips this flag before the next
   * close event fires.
   */
  getIsQuitting: () => boolean;
  /** Mark that an explicit quit has been initiated; suppresses interception. */
  setIsQuitting: () => void;
  /**
   * Show the tray icon (Electron's Tray has no setVisible; the caller is
   * expected to create-and-store or destroy-and-clear the Tray instance).
   */
  showTray: () => void;
  /**
   * True if the current platform+session can actually display a tray icon.
   */
  isTraySupported: () => boolean;
  /**
   * Modal prompt: asks the renderer to ask the user what to do. May throw /
   * time out — callers must handle that as a graceful fallback.
   */
  promptChoice: (w: BrowserWindow) => Promise<PromptResult>;
  /** Reads the in-memory cached preference. */
  getCachedBehavior: () => CloseBehavior;
  /** Updates the in-memory cache and persists. */
  setCachedBehavior: (value: CloseBehavior) => Promise<void>;
}

/**
 * Attaches the close interception to the given main window. Reads the
 * preference from the in-memory cache (already hydrated by the caller).
 *
 * Behavior branch:
 *  - "quit"     → no interception, default close proceeds.
 *  - "minimize" → if tray is supported: hide window, show tray. Otherwise:
 *                 log a one-line warning and let the default close proceed.
 *  - "ask"      → forward to renderer via `promptChoice`; apply the chosen
 *                 action and optionally persist via `setCachedBehavior`.
 *
 * Returns nothing; interception is delivered by side effect on `mainWindow`.
 */
export function applyCloseBehavior(opts: ApplyCloseBehaviorOptions): void {
  const {
    mainWindow,
    getIsQuitting,
    setIsQuitting,
    showTray,
    isTraySupported,
    promptChoice,
    getCachedBehavior,
    setCachedBehavior,
  } = opts;

  if (process.platform === "darwin") return;

  mainWindow.on("close", (event) => {
    // Explicit quit paths (Cmd+Q, menu Quit, tray Quit, prompt-chose-Quit)
    // must not be re-intercepted.
    if (getIsQuitting()) return;

    const behavior = getCachedBehavior();
    if (behavior === "quit") return;

    if (behavior === "minimize") {
      if (isTraySupported()) {
        event.preventDefault();
        mainWindow.hide();
        showTray();
      } else {
        // Single-line warn only — no UI surface here. Settings UI is the
        // discoverable place that explains the fallback.
        console.warn(
          "[close-behavior] minimize requested but tray unsupported; closing instead",
        );
      }
      return;
    }

    // behavior === "ask"
    event.preventDefault();
    void (async () => {
      try {
        const result = await promptChoice(mainWindow);
        if (result.action === "ask") return; // user cancelled; no-op
        if (result.remember) {
          try {
            await setCachedBehavior(result.action);
          } catch (err) {
            console.warn("[close-behavior] failed to persist choice", err);
          }
        }
        if (result.action === "minimize") {
          if (isTraySupported()) {
            mainWindow.hide();
            showTray();
            return;
          }
          // Selected minimize but tray unavailable → fall through to quit.
          console.warn(
            "[close-behavior] prompt chose minimize but tray unsupported; quitting",
          );
        }
        // result.action === "quit"
        setIsQuitting();
        mainWindow.close();
      } catch (err) {
        // Renderer hang / IPC timeout / other prompt failure: fall back to
        // true quit so the user isn't left with an unclosable window.
        console.warn("[close-behavior] prompt failed; quitting", err);
        setIsQuitting();
        mainWindow.close();
      }
    })();
  });
}
