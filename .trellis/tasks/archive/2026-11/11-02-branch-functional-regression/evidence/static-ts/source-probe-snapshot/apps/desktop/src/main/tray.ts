import { app, Menu, Tray, nativeImage, type BrowserWindow } from "electron";
import { join } from "node:path";
import { isTrayEnvironmentSupported } from "./tray-support";

export { isTrayEnvironmentSupported } from "./tray-support";

export interface TrayHandle {
  /** Query platform capability without attaching an icon. */
  isSupported(): Promise<boolean>;
  /** Check current support and attach an icon; true means the tray is ready. */
  show(): Promise<boolean>;
  /** Remove the icon and cancel any pending show. */
  hide(): void;
  /** Remove the icon and permanently disable this handle. */
  destroy(): void;
}

export interface TrayI18nLabels {
  show: string;
  quit: string;
  tooltip: string;
}

export interface CreateTrayOptions {
  getMainWindow: () => BrowserWindow | null;
  onQuit: () => void;
  labels: TrayI18nLabels;
  iconPath: string;
}

/** Resources are unpacked beside app.asar in production. */
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

/** Electron attaches icons at construction, so only show() creates a Tray. */
export function createTray(opts: CreateTrayOptions): TrayHandle | null {
  if (process.platform === "darwin") return null;

  let tray: Tray | null = null;
  let destroyed = false;
  let generation = 0;

  function showWindow(): void {
    const win = opts.getMainWindow();
    if (!win || win.isDestroyed()) return;
    if (win.isMinimized()) win.restore();
    win.show();
    win.focus();
  }

  function hide(): void {
    generation += 1;
    const previous = tray;
    tray = null;
    previous?.destroy();
  }

  return {
    isSupported: async () => {
      if (destroyed) return false;
      const supported = await isTrayEnvironmentSupported();
      return !destroyed && supported;
    },
    show: async () => {
      if (destroyed) return false;
      const requestedGeneration = generation;
      const supported = await isTrayEnvironmentSupported();
      if (!supported || destroyed || requestedGeneration !== generation) return false;
      if (tray) return true;

      let candidate: Tray | null = null;
      try {
        const image = nativeImage.createFromPath(opts.iconPath);
        if (image.isEmpty()) {
          console.warn("[tray] icon image is empty at", opts.iconPath);
          return false;
        }
        candidate = new Tray(image);
        candidate.setToolTip(opts.labels.tooltip);
        candidate.setContextMenu(Menu.buildFromTemplate([
          { label: opts.labels.show, click: showWindow },
          { type: "separator" },
          { label: opts.labels.quit, click: opts.onQuit },
        ]));
        candidate.on("click", showWindow);
        tray = candidate;
        return true;
      } catch (error) {
        candidate?.destroy();
        console.warn("[tray] failed to construct tray icon", error);
        return false;
      }
    },
    hide,
    destroy: () => {
      destroyed = true;
      hide();
    },
  };
}

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
