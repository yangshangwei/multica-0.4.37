import { writeFile } from "node:fs/promises";
import type { Page, TestInfo } from "@playwright/test";

export async function installFocusObserver(page: Page, locale: string) {
  await page.evaluate((language) => {
    const targetLabel = language === "zh-Hans" ? "重复目标任务" : "Duplicate target task";
    const reasonLabel = language === "zh-Hans" ? "原因" : "Reason";
    const state = {
      locale: language,
      started: performance.now(),
      events: [] as unknown[],
      stages: [] as unknown[],
      frame: 0,
      dropped: 0,
      opened: false,
      reasonInputObserved: false,
    };
    (window as any).__triageFocus = state;
    const target = () => document.querySelector<HTMLInputElement>(`input[aria-label="${targetLabel}"]`);
    const reason = () => document.querySelector<HTMLTextAreaElement>(`textarea[aria-label="${reasonLabel}"]`);
    const describe = (element: EventTarget | null) => {
      const el = element instanceof HTMLElement ? element : null;
      return el ? { tag: el.tagName, role: el.getAttribute("role"), label: el.getAttribute("aria-label"), id: el.id } : null;
    };
    const snapshot = (stage: string) => {
      const input = target();
      const textarea = reason();
      return {
        stage,
        time: performance.now(),
        frame: state.frame,
        active: describe(document.activeElement),
        target: input?.value ?? null,
        reason: textarea?.value ?? null,
      };
    };
    const push = (destination: unknown[], value: unknown) => {
      if (destination.length < 2000) destination.push(value);
      else state.dropped++;
    };
    const listener = (event: Event) => {
      const input = target();
      const textarea = reason();
      if (!input || !textarea || (event.target !== input && event.target !== textarea)) return;
      const value = snapshot(event.type);
      push(state.events, {
        ...value,
        type: event.type,
        eventTime: event.timeStamp,
        targetElement: describe(event.target),
        inputType: event instanceof InputEvent ? event.inputType : null,
        data: event instanceof InputEvent ? event.data : null,
        isTrusted: event.isTrusted,
        stack: (new Error().stack ?? "").split("\n").slice(0, 8),
      });
      if (event.target === textarea && event.type === "focusin") push(state.stages, snapshot("before-reason-fill-focus"));
      if (event.target === textarea && event.type === "input") {
        state.reasonInputObserved = true;
        push(state.stages, snapshot("after-reason-fill-input"));
      }
      if (event.target === input && event.type === "input") push(state.stages, snapshot("target-input"));
      if (event.target === input && event.type === "pointerdown") push(state.stages, snapshot(state.reasonInputObserved ? "return-target-click-before" : "first-target-click-before"));
      if (event.target === input && event.type === "click") push(state.stages, snapshot("target-click-after"));
    };
    for (const type of ["focusin", "focusout", "beforeinput", "input", "change", "pointerdown", "click"]) {
      document.addEventListener(type, listener, true);
    }
    const mutations = new MutationObserver(() => {
      if (!state.opened && target() && reason()) {
        state.opened = true;
        push(state.stages, snapshot("dialog-open-mutation"));
      }
    });
    mutations.observe(document.documentElement, { childList: true, subtree: true });
    let previous = "";
    let stopped = false;
    const tick = () => {
      state.frame++;
      if (state.opened && target() && reason()) {
        const observed = snapshot("render-frame-change");
        const signature = JSON.stringify([observed.active, observed.target, observed.reason]);
        if (signature !== previous) {
          push(state.stages, observed);
          previous = signature;
        }
      }
      if (!stopped) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
    (window as any).__triageFocusStop = () => {
      stopped = true;
      mutations.disconnect();
      for (const type of ["focusin", "focusout", "beforeinput", "input", "change", "pointerdown", "click"]) {
        document.removeEventListener(type, listener, true);
      }
      push(state.stages, snapshot("diagnostic-final"));
      return state;
    };
  }, locale);
}

export async function saveFocusObserver(page: Page, info: TestInfo, locale: string) {
  const observation = await page.evaluate(() => (window as any).__triageFocusStop?.() ?? { unavailable: true });
  const path = info.outputPath(`focus-observation-${locale}.json`);
  await writeFile(path, JSON.stringify(observation, null, 2) + "\n");
  await info.attach(`focus-observation-${locale}`, { path, contentType: "application/json" });
}
