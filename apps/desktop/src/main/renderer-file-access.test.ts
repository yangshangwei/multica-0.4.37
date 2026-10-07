// @vitest-environment node
import type { BrowserWindow, OnBeforeRequestListenerDetails, Session, WebContents, WebFrameMain } from "electron";
import { describe, expect, it, vi } from "vitest";
import { installRendererFileAccess } from "./renderer-file-access";

type Listener = (details: OnBeforeRequestListenerDetails, callback: (response: { cancel?: boolean }) => void) => void;

function sessionFixture() {
  let listener: Listener;
  const onBeforeRequest = vi.fn((_filter, callback: Listener) => { listener = callback; });
  const session = { webRequest: { onBeforeRequest } } as unknown as Session;
  return {
    session,
    onBeforeRequest,
    cancelled(details: Partial<OnBeforeRequestListenerDetails>) {
      const callback = vi.fn();
      listener(details as OnBeforeRequestListenerDetails, callback);
      expect(callback).toHaveBeenCalledOnce();
      return callback.mock.calls[0]![0].cancel;
    },
  };
}

function windowFixture(session: Session, id = 1) {
  let destroy: () => void = () => {};
  const frame = { detached: false } as WebFrameMain;
  const contents = {
    id, session, mainFrame: frame,
    isDestroyed: vi.fn(() => false),
    once: vi.fn((_event: string, callback: () => void) => { destroy = callback; }),
  };
  const window = { webContents: contents } as unknown as BrowserWindow;
  return { window, contents, frame, destroy: () => destroy() };
}

describe("renderer file requests", () => {
  it("denies a subframe even when its URL and resource type look trusted", () => {
    const fixture = sessionFixture();
    const { window, frame } = windowFixture(fixture.session);
    installRendererFileAccess(window);
    expect(fixture.cancelled({ webContentsId: 1, frame })).toBe(false);
    expect(fixture.cancelled({ webContentsId: 1, resourceType: "mainFrame", frame: { ...frame } as WebFrameMain })).toBe(true);
    expect(fixture.onBeforeRequest).toHaveBeenCalledWith({ urls: ["file://*/*"] }, expect.any(Function));
  });

  it("uses a single session listener for shared and recreated windows", () => {
    const fixture = sessionFixture();
    const main = windowFixture(fixture.session, 1);
    const issue = windowFixture(fixture.session, 2);
    installRendererFileAccess(main.window);
    installRendererFileAccess(main.window);
    installRendererFileAccess(issue.window);
    main.destroy();
    const recreated = windowFixture(fixture.session, 3);
    installRendererFileAccess(recreated.window);
    expect(fixture.onBeforeRequest).toHaveBeenCalledOnce();
    expect(main.contents.once).toHaveBeenCalledOnce();
    expect(fixture.cancelled({ webContentsId: 1, frame: main.frame })).toBe(true);
    expect(fixture.cancelled({ webContentsId: 2, frame: issue.frame })).toBe(false);
    expect(fixture.cancelled({ webContentsId: 3, frame: recreated.frame })).toBe(false);
    expect(fixture.cancelled({ webContentsId: 2, frame: recreated.frame })).toBe(true);
  });

  it("looks up the live main frame after navigation instead of retaining the first frame", () => {
    const fixture = sessionFixture();
    const { window, contents, frame } = windowFixture(fixture.session);
    installRendererFileAccess(window);
    contents.mainFrame = { detached: false } as WebFrameMain;
    expect(fixture.cancelled({ webContentsId: 1, frame })).toBe(true);
    expect(fixture.cancelled({ webContentsId: 1, frame: contents.mainFrame })).toBe(false);
  });

  it("denies unknown, missing, detached, destroyed and inaccessible contexts", () => {
    const fixture = sessionFixture();
    const { window, contents, frame } = windowFixture(fixture.session);
    installRendererFileAccess(window);
    expect(fixture.cancelled({ frame })).toBe(true);
    expect(fixture.cancelled({ webContentsId: 99, frame })).toBe(true);
    expect(fixture.cancelled({ webContentsId: 1 })).toBe(true);
    expect(fixture.cancelled({ webContentsId: 1, frame: null })).toBe(true);
    Object.defineProperty(frame, "detached", { value: true, configurable: true });
    expect(fixture.cancelled({ webContentsId: 1, frame })).toBe(true);
    Object.defineProperty(frame, "detached", { value: false, configurable: true });
    contents.isDestroyed.mockReturnValue(true);
    expect(fixture.cancelled({ webContentsId: 1, frame })).toBe(true);
    contents.isDestroyed.mockReturnValue(false);
    expect(fixture.cancelled({ webContentsId: 1, get frame(): WebFrameMain { throw new Error("disposed"); } })).toBe(true);
    Object.defineProperty(contents, "mainFrame", { get() { throw new Error("disposed"); } });
    expect(fixture.cancelled({ webContentsId: 1, frame })).toBe(true);
  });

  it("keeps ownership separate across sessions", () => {
    const first = sessionFixture();
    const second = sessionFixture();
    const a = windowFixture(first.session, 1);
    const b = windowFixture(second.session, 2);
    installRendererFileAccess(a.window);
    installRendererFileAccess(b.window);
    expect(first.onBeforeRequest).toHaveBeenCalledOnce();
    expect(second.onBeforeRequest).toHaveBeenCalledOnce();
    expect(first.cancelled({ webContentsId: 2, frame: b.frame, webContents: b.contents as unknown as WebContents })).toBe(true);
    expect(second.cancelled({ webContentsId: 1, frame: a.frame })).toBe(true);
  });
});
