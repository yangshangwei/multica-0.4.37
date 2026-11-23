// @vitest-environment node
import { EventEmitter } from "node:events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CloseBehaviorPromptCoordinator } from "./close-behavior-prompt";
import { CLOSE_BEHAVIOR_CHANNELS } from "../shared/close-behavior";

function makeWindow() {
  return Object.assign(new EventEmitter(), {
    isDestroyed: () => false,
    webContents: Object.assign(new EventEmitter(), {
      isDestroyed: () => false,
      send: vi.fn(),
    }),
  });
}

describe("CloseBehaviorPromptCoordinator", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("lets the user consider an acknowledged dialog indefinitely", async () => {
    const coordinator = new CloseBehaviorPromptCoordinator();
    const window = makeWindow();
    const settled = vi.fn();
    const choice = coordinator.request(window);
    void choice.then(settled);
    const [, { requestId }] = window.webContents.send.mock.calls[0];
    expect(window.webContents.send).toHaveBeenCalledWith(CLOSE_BEHAVIOR_CHANNELS.prompt, { requestId });
    coordinator.acknowledge(window.webContents, requestId);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(settled).not.toHaveBeenCalled();
    coordinator.respond(window.webContents, { requestId, action: "minimize", remember: true });
    await expect(choice).resolves.toEqual({ action: "minimize", remember: true });
    expect(window.listenerCount("unresponsive")).toBe(0);
    expect(window.webContents.listenerCount("destroyed")).toBe(0);
  });

  it("times out delivery only when no matching acknowledgement arrives", async () => {
    const coordinator = new CloseBehaviorPromptCoordinator();
    const window = makeWindow();
    const choice = coordinator.request(window);
    const rejected = expect(choice).rejects.toThrow("delivery timed out");
    const [, { requestId }] = window.webContents.send.mock.calls[0];
    coordinator.acknowledge(makeWindow().webContents, requestId);
    coordinator.acknowledge(window.webContents, "stale");
    await vi.advanceTimersByTimeAsync(5000);
    await rejected;
    expect(window.listenerCount("unresponsive")).toBe(0);
  });

  it("ignores invalid, stale, and other-window responses", async () => {
    const coordinator = new CloseBehaviorPromptCoordinator();
    const window = makeWindow();
    const settled = vi.fn();
    const choice = coordinator.request(window);
    void choice.then(settled);
    const [, { requestId }] = window.webContents.send.mock.calls[0];
    coordinator.respond(window.webContents, null);
    coordinator.respond(window.webContents, { requestId, action: "invalid" });
    coordinator.respond(makeWindow().webContents, { requestId, action: "quit" });
    coordinator.respond(window.webContents, { requestId: "stale", action: "quit" });
    await Promise.resolve();
    expect(settled).not.toHaveBeenCalled();
    coordinator.respond(window.webContents, { requestId, action: "ask", remember: "true" });
    await expect(choice).resolves.toEqual({ action: "ask", remember: false });
    expect(vi.getTimerCount()).toBe(0);
  });

  it("deduplicates repeated close requests and gives new prompts unique IDs", async () => {
    const coordinator = new CloseBehaviorPromptCoordinator();
    const window = makeWindow();
    const choice = coordinator.request(window);
    await expect(coordinator.request(window)).resolves.toEqual({ action: "ask", remember: false });
    expect(window.webContents.send).toHaveBeenCalledOnce();
    const oldRequest = window.webContents.send.mock.calls[0][1];
    coordinator.cancel();
    await expect(choice).resolves.toEqual({ action: "ask", remember: false });
    const next = coordinator.request(window);
    expect(window.webContents.send.mock.calls[1][1].requestId).not.toBe(oldRequest.requestId);
    coordinator.cancel();
    await next;
    expect(vi.getTimerCount()).toBe(0);
  });

  it("cancels when the originating renderer is destroyed", async () => {
    const coordinator = new CloseBehaviorPromptCoordinator();
    const window = makeWindow();
    const choice = coordinator.request(window);
    window.webContents.emit("destroyed");
    await expect(choice).resolves.toEqual({ action: "ask", remember: false });
    expect(vi.getTimerCount()).toBe(0);
  });

  it("fails when an acknowledged renderer becomes unresponsive", async () => {
    const coordinator = new CloseBehaviorPromptCoordinator();
    const window = makeWindow();
    const choice = coordinator.request(window);
    const [, { requestId }] = window.webContents.send.mock.calls[0];
    coordinator.acknowledge(window.webContents, requestId);
    window.emit("unresponsive");
    await expect(choice).rejects.toThrow("unresponsive");
    expect(vi.getTimerCount()).toBe(0);
  });

  it.each(["crash", "reload"])("releases an acknowledged prompt after renderer %s", async (reason) => {
    const coordinator = new CloseBehaviorPromptCoordinator();
    const window = makeWindow();
    const settled = vi.fn();
    const choice = coordinator.request(window);
    void choice.then(settled);
    coordinator.acknowledge(window.webContents, window.webContents.send.mock.calls[0][1].requestId);
    if (reason === "crash") window.webContents.emit("render-process-gone");
    else window.webContents.emit("did-start-navigation", {}, "file:///index.html", false, true);
    await Promise.resolve();
    expect(settled).toHaveBeenCalledWith({ action: "ask", remember: false });
    expect(window.webContents.listenerCount("render-process-gone")).toBe(0);
    expect(window.webContents.listenerCount("did-start-navigation")).toBe(0);
    const next = coordinator.request(window);
    expect(window.webContents.send).toHaveBeenCalledTimes(2);
    coordinator.cancel();
    await next;
  });

  it("keeps a visible prompt during same-document and subframe navigation", async () => {
    const coordinator = new CloseBehaviorPromptCoordinator();
    const window = makeWindow();
    const settled = vi.fn();
    const choice = coordinator.request(window);
    void choice.then(settled);
    window.webContents.emit("did-start-navigation", {}, "file:///index.html#/inbox", true, true);
    window.webContents.emit("did-start-navigation", {}, "https://example.test", false, false);
    await Promise.resolve();
    expect(settled).not.toHaveBeenCalled();
    coordinator.cancel();
    await choice;
  });
});
