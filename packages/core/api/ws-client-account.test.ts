// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { WSClient } from "./ws-client";

class Socket {
  static OPEN = 1;
  static sockets: Socket[] = [];
  readyState = 1;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  send = vi.fn();
  close = vi.fn();
  constructor(readonly url: string) { Socket.sockets.push(this); }
  message(type: string, payload?: unknown) { this.onmessage?.({ data: JSON.stringify({ type, payload }) }); }
}

const clients: WSClient[] = [];
afterEach(() => {
  clients.splice(0).forEach((client) => client.disconnect());
  Socket.sockets = [];
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function setup() {
  vi.stubGlobal("WebSocket", Socket);
  const client = new WSClient("wss://server.example/ws?workspace_slug=stale&workspace_id=stale");
  clients.push(client);
  client.setAuth("private-token", null);
  return client;
}

it("uses an explicit account connection without ambient workspace or URL credentials", () => {
  const client = setup();
  client.connect();
  const socket = Socket.sockets[0]!;
  const url = new URL(socket.url);
  expect(url.searchParams.get("scope")).toBe("account");
  expect(url.searchParams.has("workspace_slug")).toBe(false);
  expect(url.searchParams.has("workspace_id")).toBe(false);
  expect(socket.url).not.toContain("private-token");
  socket.onopen?.();
  expect(JSON.parse(socket.send.mock.calls[0]![0])).toEqual({ type: "auth", payload: { token: "private-token" } });
});

it("authenticates before subscribing, shares subscriptions and replays them after reconnect", () => {
  vi.useFakeTimers();
  const client = setup();
  const first = client.subscribeScope("issue", "shared-issue");
  const second = client.subscribeScope("issue", "shared-issue");
  const ready = vi.fn();
  client.onAuthenticated(ready);
  client.connect();
  const socket = Socket.sockets[0]!;
  socket.onopen?.();
  expect(socket.send.mock.calls.map(([frame]) => JSON.parse(frame).type)).toEqual(["auth"]);
  socket.message("auth_ack");
  expect(ready).toHaveBeenCalledTimes(1);
  expect(socket.send.mock.calls.map(([frame]) => JSON.parse(frame).type)).toEqual(["auth", "subscribe"]);
  first();
  expect(socket.send).toHaveBeenCalledTimes(2);
  socket.onclose?.();
  vi.advanceTimersByTime(1200);
  const replacement = Socket.sockets[1]!;
  replacement.onopen?.();
  replacement.message("auth_ack");
  expect(ready).toHaveBeenCalledTimes(2);
  expect(replacement.send.mock.calls.map(([frame]) => JSON.parse(frame).type)).toEqual(["auth", "subscribe"]);
  second();
  expect(JSON.parse(replacement.send.mock.calls[2]![0])).toEqual({ type: "unsubscribe", payload: { scope: "issue", id: "shared-issue" } });
});

it("ignores late socket callbacks after reconnect and explicit disconnect", () => {
  vi.useFakeTimers();
  const client = setup();
  const received = vi.fn();
  client.on("collaboration:access_changed", received);
  client.connect();
  const socket = Socket.sockets[0]!;
  const late = socket.onmessage!;
  socket.onclose?.();
  vi.advanceTimersByTime(1200);
  late({ data: JSON.stringify({ type: "collaboration:access_changed", payload: { issue_id: "old" } }) });
  expect(received).not.toHaveBeenCalled();
  const replacement = Socket.sockets[1]!;
  const afterDisconnect = replacement.onmessage!;
  client.disconnect();
  afterDisconnect({ data: JSON.stringify({ type: "collaboration:access_changed", payload: { issue_id: "old" } }) });
  expect(received).not.toHaveBeenCalled();
});

it("does not log malformed protected frames", () => {
  vi.stubGlobal("WebSocket", Socket);
  const logger = { info: vi.fn(), debug: vi.fn(), warn: vi.fn(), error: vi.fn() };
  const client = new WSClient("wss://server.example/ws", { logger });
  clients.push(client);
  client.setAuth("token", null);
  client.connect();
  Socket.sockets[0]!.onmessage?.({ data: "PRIVATE_COLLABORATION_BODY" });
  Socket.sockets[0]!.message("PRIVATE_COLLABORATION_BODY", {});
  expect(JSON.stringify(logger.warn.mock.calls)).not.toContain("PRIVATE_COLLABORATION_BODY");
  expect(JSON.stringify(logger.debug.mock.calls)).not.toContain("PRIVATE_COLLABORATION_BODY");
});
