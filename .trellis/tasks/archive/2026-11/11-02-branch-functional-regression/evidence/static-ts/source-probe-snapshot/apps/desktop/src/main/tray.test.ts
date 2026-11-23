// @vitest-environment node
import type { BrowserWindow, MenuItemConstructorOptions } from "electron";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { buildTrayIconPath, createTray } from "./tray";

const mocks = vi.hoisted(() => ({
  supported: vi.fn<() => Promise<boolean>>(),
  image: { isEmpty: vi.fn(() => false) },
  tray: {
    setToolTip: vi.fn(),
    setContextMenu: vi.fn(),
    on: vi.fn(),
    destroy: vi.fn(),
  },
  construct: vi.fn(),
  menu: vi.fn(),
}));

vi.mock("./tray-support", () => ({
  isTrayEnvironmentSupported: mocks.supported,
}));
vi.mock("electron", () => ({
  app: { getLocale: () => "en" },
  Tray: vi.fn(function () {
    mocks.construct();
    return mocks.tray;
  }),
  Menu: { buildFromTemplate: mocks.menu },
  nativeImage: { createFromPath: () => mocks.image },
}));

function setup() {
  const window = {
    isDestroyed: vi.fn(() => false),
    isMinimized: vi.fn(() => false),
    restore: vi.fn(),
    show: vi.fn(),
    focus: vi.fn(),
  };
  const onQuit = vi.fn();
  const getMainWindow = vi.fn(() => window as unknown as BrowserWindow);
  const handle = createTray({
    getMainWindow,
    onQuit,
    labels: { show: "Show Multica", quit: "Quit", tooltip: "Multica" },
    iconPath: "/resources/icon.png",
  });
  if (!handle) throw new Error("Expected a tray handle");
  return { handle, window, getMainWindow, onQuit };
}

beforeEach(() => {
  vi.stubGlobal("process", { ...process, platform: "linux" });
  vi.clearAllMocks();
  mocks.supported.mockReset().mockResolvedValue(true);
  mocks.image.isEmpty.mockReset().mockReturnValue(false);
  mocks.construct.mockReset();
  mocks.tray.setContextMenu.mockReset();
  vi.spyOn(console, "warn").mockImplementation(() => {});
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("createTray", () => {
  it("keeps capability queries free of visible tray icons", async () => {
    const { handle } = setup();
    expect(await handle.isSupported()).toBe(true);
    expect(await handle.isSupported()).toBe(true);
    expect(mocks.construct).not.toHaveBeenCalled();
  });

  it("does not construct an icon without a registered Linux host", async () => {
    mocks.supported.mockResolvedValue(false);
    const { handle } = setup();
    expect(await handle.isSupported()).toBe(false);
    expect(await handle.show()).toBe(false);
    expect(mocks.construct).not.toHaveBeenCalled();
  });

  it("checks the host again when showing after a successful capability query", async () => {
    mocks.supported.mockResolvedValueOnce(true).mockResolvedValueOnce(false);
    const { handle } = setup();
    expect(await handle.isSupported()).toBe(true);
    expect(await handle.show()).toBe(false);
    expect(mocks.construct).not.toHaveBeenCalled();
  });

  it("creates one icon for concurrent show calls and releases it when hidden", async () => {
    const { handle } = setup();
    expect(await Promise.all([handle.show(), handle.show()])).toEqual([true, true]);
    expect(mocks.construct).toHaveBeenCalledTimes(1);
    handle.hide();
    handle.hide();
    expect(mocks.tray.destroy).toHaveBeenCalledTimes(1);
    expect(await handle.show()).toBe(true);
    expect(mocks.construct).toHaveBeenCalledTimes(2);
  });

  it.each(["hide", "destroy"] as const)("cancels a pending show on %s", async (operation) => {
    const { handle } = setup();
    let resolveSupport!: (supported: boolean) => void;
    mocks.supported.mockReturnValueOnce(new Promise((resolve) => { resolveSupport = resolve; }));
    const pending = handle.show();
    handle[operation]();
    resolveSupport(true);
    expect(await pending).toBe(false);
    expect(mocks.construct).not.toHaveBeenCalled();
  });

  it("cannot be reused after destroy and cleans up only once", async () => {
    const { handle } = setup();
    await handle.show();
    handle.destroy();
    handle.destroy();
    expect(await handle.show()).toBe(false);
    expect(await handle.isSupported()).toBe(false);
    expect(mocks.tray.destroy).toHaveBeenCalledTimes(1);
    expect(mocks.construct).toHaveBeenCalledTimes(1);
  });

  it.each(["empty image", "constructor failure", "menu failure"])(
    "reports failure for %s and cleans up any partially initialized icon",
    async (failure) => {
      if (failure === "empty image") mocks.image.isEmpty.mockReturnValue(true);
      if (failure === "constructor failure") mocks.construct.mockImplementation(() => { throw new Error("constructor"); });
      if (failure === "menu failure") mocks.tray.setContextMenu.mockImplementation(() => { throw new Error("menu"); });
      const { handle } = setup();
      expect(await handle.show()).toBe(false);
      expect(mocks.tray.destroy).toHaveBeenCalledTimes(failure === "menu failure" ? 1 : 0);
    },
  );

  it("restores and focuses the window from both the Show menu and tray activation", async () => {
    const { handle, window, onQuit } = setup();
    await handle.show();
    const template: MenuItemConstructorOptions[] = mocks.menu.mock.calls[0][0];
    const show = template[0].click!;
    const quit = template[2].click!;
    const activate = mocks.tray.on.mock.calls.find(([event]) => event === "click")![1];
    window.isMinimized.mockReturnValue(true);
    show(undefined!, undefined!, undefined!);
    activate();
    expect(window.restore).toHaveBeenCalledTimes(2);
    expect(window.show).toHaveBeenCalledTimes(2);
    expect(window.focus).toHaveBeenCalledTimes(2);
    quit(undefined!, undefined!, undefined!);
    expect(onQuit).toHaveBeenCalledTimes(1);
  });

  it("ignores activation after the main window has been destroyed", async () => {
    const { handle, window } = setup();
    await handle.show();
    window.isDestroyed.mockReturnValue(true);
    const activate = mocks.tray.on.mock.calls.find(([event]) => event === "click")![1];
    activate();
    expect(window.show).not.toHaveBeenCalled();
  });

  it("returns no handle on macOS", () => {
    vi.stubGlobal("process", { ...process, platform: "darwin" });
    expect(createTray({
      getMainWindow: () => null,
      onQuit: vi.fn(),
      labels: { show: "Show", quit: "Quit", tooltip: "Multica" },
      iconPath: "/icon.png",
    })).toBeNull();
  });
});

describe("buildTrayIconPath", () => {
  it("resolves the development icon", () => {
    expect(buildTrayIconPath({ isDev: true, resourcesPath: "/repo/apps/desktop" }))
      .toBe("/repo/apps/desktop/resources/icon.png");
  });

  it("resolves the unpacked production icon", () => {
    expect(buildTrayIconPath({ isDev: false, resourcesPath: "/opt/Multica/resources" }))
      .toBe("/opt/Multica/resources/app.asar.unpacked/resources/icon.png");
  });
});
