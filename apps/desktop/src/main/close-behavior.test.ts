// @vitest-environment node
import { EventEmitter } from "node:events";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyCloseBehavior,
  closePreferencesPath,
  DEFAULT_CLOSE_BEHAVIOR,
  loadClosePreferences,
  loadClosePreferenceStore,
  saveClosePreferences,
  type ApplyCloseBehaviorOptions,
  type CloseBehavior,
  type PromptResult,
} from "./close-behavior";
import type { BrowserWindow } from "electron";

describe("closePreferencesPath", () => {
  it("joins well-known filename under userData", () => {
    expect(closePreferencesPath("/tmp/xyz")).toBe(
      join("/tmp/xyz", "close-preferences.json"),
    );
  });
});

describe("loadClosePreferences", () => {
  let dir = "";

  beforeEach(async () => {
    dir = await mkdtemp(join(tmpdir(), "close-behavior-test-"));
  });

  afterEach(async () => {
    await rm(dir, { recursive: true, force: true });
  });

  it("returns default when file missing", async () => {
    const file = closePreferencesPath(dir);
    expect(await loadClosePreferences(file)).toBe(DEFAULT_CLOSE_BEHAVIOR);
  });

  it("returns default on invalid JSON", async () => {
    const file = closePreferencesPath(dir);
    await writeFile(file, "{not json", "utf-8");
    expect(await loadClosePreferences(file)).toBe(DEFAULT_CLOSE_BEHAVIOR);
  });

  it("returns default on unexpected shape", async () => {
    const file = closePreferencesPath(dir);
    await writeFile(file, JSON.stringify({ closeBehavior: "explode" }), "utf-8");
    expect(await loadClosePreferences(file)).toBe(DEFAULT_CLOSE_BEHAVIOR);
  });

  it("returns default on non-object JSON", async () => {
    const file = closePreferencesPath(dir);
    await writeFile(file, JSON.stringify(42), "utf-8");
    expect(await loadClosePreferences(file)).toBe(DEFAULT_CLOSE_BEHAVIOR);
  });

  it.each<CloseBehavior>(["quit", "minimize", "ask"])(
    "round-trips %s",
    async (value) => {
      const file = closePreferencesPath(dir);
      await saveClosePreferences(file, value);
      expect(await loadClosePreferences(file)).toBe(value);
      // Atomic write: file should contain just one valid JSON object, no tmp leftovers.
      const raw = await readFile(file, "utf-8");
      expect(JSON.parse(raw)).toEqual({ closeBehavior: value });
    },
  );

  it("overwrites existing value", async () => {
    const file = closePreferencesPath(dir);
    await saveClosePreferences(file, "quit");
    await saveClosePreferences(file, "minimize");
    expect(await loadClosePreferences(file)).toBe("minimize");
  });

  it("retains the last committed preference on write failure and recovers", async () => {
    const file = closePreferencesPath(dir);
    const store = await loadClosePreferenceStore(file);
    await mkdir(`${file}.tmp`);
    await expect(store.set("minimize")).rejects.toThrow();
    expect(store.get()).toBe("ask");
    await rm(`${file}.tmp`, { recursive: true });
    await store.set("quit");
    expect(store.get()).toBe("quit");
    expect(await loadClosePreferences(file)).toBe("quit");
  });

  it("serializes overlapping saves and keeps cache and disk consistent", async () => {
    const file = closePreferencesPath(dir);
    const store = await loadClosePreferenceStore(file);
    const saves = [store.set("minimize"), store.set("quit")];
    expect(store.get()).toBe("ask");
    await Promise.all(saves);
    expect(store.get()).toBe("quit");
    expect(await loadClosePreferences(file)).toBe("quit");
  });
});

// =============================================================================
// applyCloseBehavior
// =============================================================================

type FakeCloseEvent = { preventDefault: () => void };

class FakeBrowserWindow extends EventEmitter {
  public hid = false;
  public closed = false;
  isDestroyed() {
    return this.closed;
  }
  hide() {
    this.hid = true;
  }
  close() {
    this.closed = true;
  }
}

function makeHarness(overrides?: Partial<ApplyCloseBehaviorOptions>) {
  const win = new FakeBrowserWindow();
  const state = {
    behavior: "quit" as CloseBehavior,
    isQuitting: false,
    traySupported: true,
    trayShown: 0,
    quitCalls: 0,
  };
  const promptChoice = async (_w: BrowserWindow): Promise<PromptResult> => ({
    action: "ask",
    remember: false,
  });

  const options: ApplyCloseBehaviorOptions = {
    mainWindow: win as unknown as BrowserWindow,
    getIsQuitting: () => state.isQuitting,
    quitApp: () => {
      state.quitCalls += 1;
      state.isQuitting = true;
    },
    showTray: async () => {
      if (!state.traySupported) return false;
      state.trayShown += 1;
      return true;
    },
    promptChoice,
    getCachedBehavior: () => state.behavior,
    setCachedBehavior: async (v) => {
      state.behavior = v;
    },
    ...overrides,
  };

  return { win, state, options };
}

const originalPlatform = process.platform;

function setPlatform(p: NodeJS.Platform) {
  Object.defineProperty(process, "platform", { value: p, configurable: true });
}

afterEach(() => {
  setPlatform(originalPlatform);
});

describe("applyCloseBehavior", () => {
  it("requests application exit when an auxiliary window is still alive", async () => {
    setPlatform("win32");
    const quitApp = vi.fn();
    const { win, state, options } = makeHarness({
      promptChoice: async () => ({ action: "quit", remember: false }),
    });
    state.behavior = "ask";
    applyCloseBehavior({ ...options, quitApp });
    win.emit("close", { preventDefault: vi.fn() });
    await new Promise((resolve) => setImmediate(resolve));
    expect(quitApp).toHaveBeenCalledOnce();
    expect(win.closed).toBe(false);
  });

  it("creates the tray successfully before hiding the window", async () => {
    setPlatform("win32");
    let finishShowing!: (shown: boolean) => void;
    const { win, state, options } = makeHarness({
      showTray: () => new Promise<boolean>((resolve) => { finishShowing = resolve; }),
    });
    state.behavior = "minimize";
    applyCloseBehavior(options);
    win.emit("close", { preventDefault: vi.fn() });
    expect(win.hid).toBe(false);
    finishShowing(true);
    await new Promise((resolve) => setImmediate(resolve));
    expect(win.hid).toBe(true);
  });

  it("does nothing on darwin", () => {
    setPlatform("darwin");
    const { win, options } = makeHarness();
    applyCloseBehavior(options);
    expect(win.listenerCount("close")).toBe(0);
  });

  it("does not intercept when isQuitting is true (regardless of behavior)", () => {
    setPlatform("win32");
    const { win, state, options } = makeHarness();
    state.behavior = "minimize"; // would normally intercept
    state.isQuitting = true;     // but user explicitly quit
    applyCloseBehavior(options);

    let prevented = false;
    const event: FakeCloseEvent = {
      preventDefault: () => {
        prevented = true;
      },
    };
    win.emit("close", event);
    expect(prevented).toBe(false);
    expect(win.hid).toBe(false);
    expect(state.trayShown).toBe(0);
  });

  it("turns the persisted quit preference into application exit", () => {
    setPlatform("linux");
    const { win, state, options } = makeHarness();
    state.behavior = "quit";
    applyCloseBehavior(options);

    let prevented = false;
    const event: FakeCloseEvent = {
      preventDefault: () => {
        prevented = true;
      },
    };
    win.emit("close", event);
    expect(prevented).toBe(true);
    expect(state.quitCalls).toBe(1);
    expect(win.hid).toBe(false);
  });

  it("minimize: prevents default and hides window when tray supported", async () => {
    setPlatform("win32");
    const { win, state, options } = makeHarness();
    state.behavior = "minimize";
    applyCloseBehavior(options);

    let prevented = false;
    const event: FakeCloseEvent = {
      preventDefault: () => {
        prevented = true;
      },
    };
    win.emit("close", event);
    expect(prevented).toBe(true);
    await new Promise((resolve) => setImmediate(resolve));
    expect(win.hid).toBe(true);
    expect(state.trayShown).toBe(1);
  });

  it("minimize: falls back to application exit when tray unsupported", async () => {
    setPlatform("linux");
    const { win, state, options } = makeHarness();
    state.behavior = "minimize";
    state.traySupported = false;
    applyCloseBehavior(options);

    let prevented = false;
    const event: FakeCloseEvent = {
      preventDefault: () => {
        prevented = true;
      },
    };
    win.emit("close", event);
    expect(prevented).toBe(true);
    await new Promise((resolve) => setImmediate(resolve));
    expect(state.quitCalls).toBe(1);
    expect(win.hid).toBe(false);
  });

  it("ask: prevents default and calls promptChoice", async () => {
    setPlatform("win32");
    let promptCalls = 0;
    const { win, state, options } = makeHarness({
      promptChoice: async () => {
        promptCalls += 1;
        return { action: "ask", remember: false }; // Cancel path
      },
    });
    state.behavior = "ask";
    applyCloseBehavior(options);

    let prevented = false;
    const event: FakeCloseEvent = {
      preventDefault: () => {
        prevented = true;
      },
    };
    win.emit("close", event);
    expect(prevented).toBe(true);

    // Wait for the promise chain to settle.
    await new Promise((r) => setImmediate(r));
    expect(promptCalls).toBe(1);
    expect(win.hid).toBe(false);
    expect(win.closed).toBe(false);
  });

  it("ask → quit: exits the app and doesn't persist when remember=false", async () => {
    setPlatform("win32");
    const { win, state, options } = makeHarness({
      promptChoice: async () => ({ action: "quit", remember: false }),
    });
    state.behavior = "ask";
    applyCloseBehavior(options);
    expect(win.listenerCount("close")).toBeGreaterThan(0);

    const event: FakeCloseEvent = { preventDefault: () => undefined };
    win.emit("close", event);
    await new Promise((r) => setImmediate(r));
    await new Promise((r) => setImmediate(r)); // let all awaits run

    expect(state.isQuitting).toBe(true);
    expect(state.quitCalls).toBe(1);
    expect(win.closed).toBe(false);
    expect(state.behavior).toBe("ask"); // unchanged
  });

  it("ask → minimize + remember: persists and hides", async () => {
    setPlatform("linux");
    const { win, state, options } = makeHarness({
      promptChoice: async () => ({ action: "minimize", remember: true }),
    });
    state.behavior = "ask";
    applyCloseBehavior(options);

    const event: FakeCloseEvent = { preventDefault: () => undefined };
    win.emit("close", event);
    await new Promise((r) => setImmediate(r));
    await new Promise((r) => setImmediate(r));

    expect(state.behavior).toBe("minimize");
    expect(win.hid).toBe(true);
    expect(state.trayShown).toBe(1);
    expect(state.isQuitting).toBe(false);
  });

  it("ask → minimize, tray unsupported: quits instead", async () => {
    setPlatform("linux");
    const { win, state, options } = makeHarness({
      promptChoice: async () => ({ action: "minimize", remember: false }),
    });
    state.behavior = "ask";
    state.traySupported = false;
    applyCloseBehavior(options);

    const event: FakeCloseEvent = { preventDefault: () => undefined };
    win.emit("close", event);
    await new Promise((r) => setImmediate(r));
    await new Promise((r) => setImmediate(r));

    expect(state.isQuitting).toBe(true);
    expect(state.quitCalls).toBe(1);
    expect(win.closed).toBe(false);
  });

  it("prompt throws → forced quit", async () => {
    setPlatform("linux");
    const { win, state, options } = makeHarness({
      promptChoice: async () => {
        throw new Error("renderer hung");
      },
    });
    state.behavior = "ask";
    applyCloseBehavior(options);

    const event: FakeCloseEvent = { preventDefault: () => undefined };
    win.emit("close", event);
    await new Promise((r) => setImmediate(r));
    await new Promise((r) => setImmediate(r));

    expect(state.isQuitting).toBe(true);
    expect(state.quitCalls).toBe(1);
    expect(win.closed).toBe(false);
  });

  it("does not hide after an explicit quit overtakes tray creation", async () => {
    setPlatform("win32");
    let finishShowing!: (shown: boolean) => void;
    const { win, state, options } = makeHarness({
      showTray: () => new Promise<boolean>((resolve) => { finishShowing = resolve; }),
    });
    state.behavior = "minimize";
    applyCloseBehavior(options);
    win.emit("close", { preventDefault: vi.fn() });
    state.isQuitting = true;
    finishShowing(true);
    await new Promise((resolve) => setImmediate(resolve));
    expect(win.hid).toBe(false);
    expect(state.quitCalls).toBe(0);
  });

  it("ignores duplicate closes while waiting for a choice", async () => {
    setPlatform("win32");
    let answer!: (result: PromptResult) => void;
    const promptChoice = vi.fn(() => new Promise<PromptResult>((resolve) => { answer = resolve; }));
    const { win, state, options } = makeHarness({ promptChoice });
    state.behavior = "ask";
    applyCloseBehavior(options);
    const preventDefault = vi.fn();
    win.emit("close", { preventDefault });
    win.emit("close", { preventDefault });
    expect(preventDefault).toHaveBeenCalledTimes(2);
    expect(promptChoice).toHaveBeenCalledOnce();
    answer({ action: "ask", remember: false });
    await new Promise((resolve) => setImmediate(resolve));
    expect(state.quitCalls).toBe(0);
    win.emit("close", { preventDefault });
    expect(promptChoice).toHaveBeenCalledTimes(2);
    answer({ action: "ask", remember: false });
  });
});
