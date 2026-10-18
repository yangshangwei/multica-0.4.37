// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import {
  MainRendererMessageQueue,
  parseMainRendererChannelState,
} from "./main-renderer-messages";

describe("MainRendererMessageQueue", () => {
  it("keeps a server-editor handoff until main subscribes, including window recreation", () => {
    const queue = new MainRendererMessageQueue();
    const send = vi.fn();
    queue.enqueue("runtime-config:edit", null, send);
    queue.resetReady();
    expect(send).not.toHaveBeenCalled();
    expect(parseMainRendererChannelState({ channel: "runtime-config:edit", ready: true }))
      .toEqual({ channel: "runtime-config:edit", ready: true });
    queue.setReady("runtime-config:edit", true, send);
    expect(send).toHaveBeenCalledExactlyOnceWith("runtime-config:edit", null);
  });

  it("holds messages until their matching listener is ready", () => {
    const queue = new MainRendererMessageQueue();
    const send = vi.fn();

    queue.enqueue("auth:token", "token-a", send);
    queue.setReady("invite:open", true, send);
    expect(send).not.toHaveBeenCalled();

    queue.setReady("auth:token", true, send);
    expect(send).toHaveBeenCalledWith("auth:token", "token-a");
  });

  it("delivers immediately while a channel is ready", () => {
    const queue = new MainRendererMessageQueue();
    const send = vi.fn();

    queue.setReady("inbox:open", true, send);
    queue.enqueue("inbox:open", { itemId: "item-1" }, send);

    expect(send).toHaveBeenCalledOnce();
  });

  it("keeps queued work across a renderer readiness reset", () => {
    const queue = new MainRendererMessageQueue();
    const send = vi.fn();

    queue.setReady("invite:open", true, send);
    queue.resetReady();
    queue.enqueue("invite:open", "invite-1", send);
    expect(send).not.toHaveBeenCalled();

    queue.setReady("invite:open", true, send);
    expect(send).toHaveBeenCalledWith("invite:open", "invite-1");
  });

  it("can discard account-scoped pending messages", () => {
    const queue = new MainRendererMessageQueue();
    const send = vi.fn();

    queue.enqueue("inbox:open", { itemId: "old-account-item" }, send);
    queue.clear("inbox:open");
    queue.setReady("inbox:open", true, send);

    expect(send).not.toHaveBeenCalled();
  });
});

describe("parseMainRendererChannelState", () => {
  it("accepts only allowlisted channels with an explicit boolean", () => {
    expect(
      parseMainRendererChannelState({ channel: "auth:token", ready: true }),
    ).toEqual({ channel: "auth:token", ready: true });
    expect(
      parseMainRendererChannelState({ channel: "shell:openExternal", ready: true }),
    ).toBeNull();
    expect(
      parseMainRendererChannelState({ channel: "auth:token", ready: "yes" }),
    ).toBeNull();
  });
});

it("queues account deep links until their own listener is ready", () => {
  const queue = new MainRendererMessageQueue();
  const send = vi.fn();
  queue.enqueue("account:open", "/account?view=current", send);
  queue.setReady("invite:open", true, send);
  expect(send).not.toHaveBeenCalled();
  queue.setReady("account:open", true, send);
  expect(send).toHaveBeenCalledWith("account:open", "/account?view=current");
});
