import { app, Menu, Tray, nativeImage, type BrowserWindow } from "electron";
import { join } from "node:path";

/**
 * tray.ts
 *
 * System tray lifecycle for Windows / Linux close-to-tray behavior.
 *
 * Electron's Tray API has no `setVisible`; an icon appears as soon as the
 * Tray is constructed and goes away only when `tray.destroy()` runs. So
 * "show" / "hide" here mean "construct and cache a Tray" / "destroy and
 * release it" — not a visibility toggle. We deliberately attach the tray
 * only when the user has hidden the main window (per the close-behavior
 * module) and destroy it as soon as the window reappears, which matches
 * Discord / Slack conventions and avoids the "two icons for one app" smell.
 *
 * macOS is out of scope: applyCloseBehavior returns immediately on darwin,
 * so this module is never consulted there.
 */

export interface TrayHandle {
  /** True if the tray environment is supported (constructor + heuristic). */
  isSupported(): boolean;
  /** Attach a tray icon (idempotent — re-showing is a no-op). */
  show(): void;
  /** Remove the tray icon (idempotent — destroying twice is a no-op). */
  hide(): void;
  /** Tear down and never reuse. Called from before-quit. */
  destroy(): void;
}

export interface TrayI18nLabels {
  show: string;
  quit: string;
  tooltip: string;
}

export interface CreateTrayOptions {
  /** Window to toggle. The tray "Show" menu will .show()+.focus() it. */
  getMainWindow: () => BrowserWindow | null;
  /** Called when the user picks Quit from the tray menu. Caller marks isQuitting + app.quit(). */
  onQuit: () => void;
  /** Labels for the menu items (resolved by caller via app locale). */
  labels: TrayI18nLabels;
  /** Resolved filesystem path of the tray icon (PNG on Linux, ICO on Windows). */
  iconPath: string;
}

/**
 * Heuristic for "can we actually display a tray icon here". Returns false on
 * Linux GNOME 40+ under Wayland without the (commonly absent) AppIndicator
 * extension. Reading environment variables is the only portable way; the
 * Tray constructor may still succeed at runtime even when the user can't
 * see the icon, so we use this heuristic to disable the minimize-to-tray
 * affordance in Settings rather than letting the user pick a dead option.
 *
 * KDE / XFCE / Cinnamon / MATE / older X11 GNOME still report true here.
 */
export function isTrayEnvironmentSupported(
  env: NodeJS.ProcessEnv = process.env,
  platform: NodeJS.Platform = process.platform,
): boolean {
  if (platform === "darwin") return false; // macOS uses dock, not tray.
  if (platform === "win32") return true;
  // Linux
  const desktop = (env.XDG_CURRENT_DESKTOP ?? "").toLowerCase();
  const session = (env.XDG_SESSION_TYPE ?? "").toLowerCase();
  if (desktop.includes("gnome") && session === "wayland") {
    return false;
  }
  return true;
}

/**
 * Resolves the tray icon path.
 *
 * We use the existing 1024x1024 resources/icon.png on every platform —
 * Electron's nativeImage scales it down to whatever the OS expects (16px
 * on Windows, 22px on Linux), avoiding the need to ship an .ico in
 * extraResources (the existing build/icon.ico is consumed by the windows
 * installer pipeline only and never lands inside the packaged app).
 *
 * resources/** is already asarUnpacked in electron-builder.yml, so the
 * same path works in dev (under apps/desktop/) and prod (under
 * <install>/resources/). The .replace call mirrors BUNDLED_ICON_PATH in
 * index.ts: in prod `__dirname` resolves into app.asar/, but resources/
 * lives flattened under process.resourcesPath/../app.asar.unpacked/.
 *
 * The caller passes `app.getAppPath()` in dev (≈ apps/desktop/) and
 * `process.resourcesPath` in prod.
 */
export function buildTrayIconPath(opts: {
  isDev: boolean;
  resourcesPath: string;
  platform?: NodeJS.Platform;
}): string {
  const { isDev, resourcesPath } = opts;
  return isDev
    ? join(resourcesPath, "resources", "icon.png")
    : join(resourcesPath, "app.asar.unpacked", "resources", "icon.png");
}

/**
 * Construct a TrayHandle bound to a specific window. Returns null when the
 * environment cannot support a tray; callers should branch on null rather
 * than re-check the heuristic.
 *
 * The tray icon only appears when `show()` is called; constructing the
 * handle does NOT immediately attach a Tray. This lets the close-behavior
 * module wire the handle up at whenReady time and decide at close time
 * whether to actually surface it.
 */
export function createTray(opts: CreateTrayOptions): TrayHandle | null {
  if (!isTrayEnvironmentSupported()) return null;

  let tray: Tray | null = null;
  let destroyed = false;

  function ensureTray(): Tray | null {
    if (destroyed) return null;
    if (tray) return tray;
    try {
      const image = nativeImage.createFromPath(opts.iconPath);
      if (image.isEmpty()) {
        console.warn("[tray] icon image is empty at", opts.iconPath);
        return null;
      }
      const t = new Tray(image);
      t.setToolTip(opts.labels.tooltip);
      const menu = Menu.buildFromTemplate([
        {
          label: opts.labels.show,
          click: () => {
            const win = opts.getMainWindow();
            if (win && !win.isDestroyed()) {
              if (win.isMinimized()) win.restore();
              win.show();
              win.focus();
            }
          },
        },
        { type: "separator" },
        { label: opts.labels.quit, click: () => opts.onQuit() },
      ]);
      t.setContextMenu(menu);
      // Left-click (Windows / KDE) mirrors "Show". Double-click is the
      // convention on XFCE; Linux single-click may also trigger activation.
      t.on("click", () => {
        const win = opts.getMainWindow();
        if (win && !win.isDestroyed()) {
          if (win.isMinimized()) win.restore();
          win.show();
          win.focus();
        }
      });
      tray = t;
      return t;
    } catch (err) {
      console.warn("[tray] failed to construct tray icon", err);
      return null;
    }
  }

  return {
    isSupported: () => !destroyed && ensureTray() !== null,
    show: () => {
      ensureTray();
    },
    hide: () => {
      if (tray) {
        tray.destroy();
        tray = null;
      }
    },
    destroy: () => {
      destroyed = true;
      if (tray) {
        tray.destroy();
        tray = null;
      }
    },
  };
}

/**
 * Resolves tray menu labels for the current app locale. Keep this minimal —
 * only the strings the tray menu actually shows. Long-form UI strings (the
 * close prompt, settings tab, etc.) live in the renderer i18n tree where
 * they can leverage useT and parity checks.
 */
export function resolveTrayLabels(): TrayI18nLabels {
  const locale = app.getLocale().toLowerCase();
  const product = "Multica";
  if (locale.startsWith("zh")) {
    return {
      show: `显示 ${product}`,
      quit: "退出",
      tooltip: product,
    };
  }
  return {
    show: `Show ${product}`,
    quit: "Quit",
    tooltip: product,
  };
}
