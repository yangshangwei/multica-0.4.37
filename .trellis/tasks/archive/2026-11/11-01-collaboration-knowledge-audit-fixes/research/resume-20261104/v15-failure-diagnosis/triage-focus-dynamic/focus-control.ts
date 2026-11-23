import { writeFile } from "node:fs/promises";
import type { Page, TestInfo } from "@playwright/test";

const checkpoints = new WeakMap<Page, unknown>();

export async function installInitialFocusControl(page: Page, mode: string) {
  await page.evaluate((controlMode) => {
    const state = {
      mode: controlMode,
      captured: 0,
      pending: 0,
      released: 0,
      callbacks: [] as { source: string; stack: string; queued: number; nativeFrame?: number; released?: number; releasePhase?: string }[],
      snapshots: [] as unknown[],
    };
    (window as any).__triageControl = state;
    const originalRAF = window.requestAnimationFrame.bind(window);
    let pending: FrameRequestCallback | null = null;
    const snapshot = (phase: string) => ({
      phase,
      time: performance.now(),
      active: document.activeElement instanceof HTMLElement ? { tag: document.activeElement.tagName, role: document.activeElement.getAttribute("role"), label: document.activeElement.getAttribute("aria-label") } : null,
      target: document.querySelector<HTMLInputElement>('input[aria-label="Duplicate target task"]')?.value ?? null,
      reason: document.querySelector<HTMLTextAreaElement>('textarea[aria-label="Reason"]')?.value ?? null,
    });
    window.requestAnimationFrame = (callback: FrameRequestCallback) => {
      const source = Function.prototype.toString.call(callback);
      const target = document.querySelector<HTMLInputElement>('input[aria-label="Duplicate target task"]');
      const reason = document.querySelector<HTMLTextAreaElement>('textarea[aria-label="Reason"]');
      if (!state.captured && target && reason && source.includes(".focus(")) {
        state.captured++;
        state.pending++;
        pending = callback;
        const entry = { source, stack: new Error().stack ?? "", queued: performance.now() } as (typeof state.callbacks)[number];
        state.callbacks.push(entry);
        state.snapshots.push(snapshot("actual-focus-callback-queued"));
        return originalRAF((time) => {
          entry.nativeFrame = time;
          state.snapshots.push(snapshot("actual-focus-native-frame"));
          if (controlMode === "await-initial") {
            state.pending--;
            state.released++;
            pending = null;
            entry.released = performance.now();
            entry.releasePhase = "native-frame-before-initial-focus-await";
            callback(time);
            state.snapshots.push(snapshot("actual-focus-native-frame-after"));
          }
          // Redirect control retains this exact real queued callback until the
          // server-side textarea focus/select -> keyboard.insertText boundary.
        });
      }
      return originalRAF(callback);
    };
    (window as any).__triageReleaseInitialFocus = () => {
      state.snapshots.push(snapshot("server-before-keyboard-insert-before-release"));
      if (!pending || state.pending !== 1) throw new Error("No single pending real focus callback at control boundary");
      const callback = pending;
      pending = null;
      state.pending--;
      state.released++;
      const entry = state.callbacks[0];
      entry.released = performance.now();
      entry.releasePhase = "server-after-textarea-focus-select-before-keyboard-insertText";
      callback(performance.now());
      state.snapshots.push(snapshot("server-before-keyboard-insert-after-release"));
      return state;
    };
    (window as any).__triageInspectBeforeInsert = () => {
      state.snapshots.push(snapshot("server-after-focus-await-before-keyboard-insertText"));
      if (state.pending !== 0) throw new Error("Focus await did not drain the real queued callback");
      return state;
    };
  }, mode);
}

export async function retainControlCheckpoint(page: Page) {
  checkpoints.set(page, await page.evaluate(() => ({ focus: (window as any).__triageFocus, control: (window as any).__triageControl })));
}

export async function saveControlCheckpoint(page: Page, info: TestInfo) {
  const current = await page.evaluate(() => ({ focus: (window as any).__triageFocus, control: (window as any).__triageControl }));
  const path = info.outputPath("initial-focus-control.json");
  await writeFile(path, JSON.stringify({ early: checkpoints.get(page), current }, null, 2) + "\n");
  await info.attach("initial-focus-control", { path, contentType: "application/json" });
}
