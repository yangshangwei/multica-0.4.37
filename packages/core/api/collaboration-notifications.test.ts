// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";
import { setCurrentWorkspace } from "../platform/workspace-storage";

const id = (tail: number) => `00000000-0000-4000-8000-${String(tail).padStart(12, "0")}`;
const timestamp = "2026-11-22T00:00:00Z";
const notification = {
  id: id(1), issue_id: id(2), event_seq: "9007199254740993", kind: "comment_created", is_mention: true,
  source_kind: "comment", source_id: id(3), issue_title: "Shared issue", can_open: true,
  created_at: timestamp, read_at: null, archived_at: null,
};
const page = { items: [notification], unread_count: 7, next_cursor: null };
const response = (value: unknown) => new Response(JSON.stringify(value), { headers: { "Content-Type": "application/json" } });
afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); setSchemaLogger(noopLogger); });

describe("collaboration notification transport", () => {
  it.each(["unread", "all", "archived"] as const)("requests %s with the original cursor and no workspace headers", async (view) => {
    setCurrentWorkspace("ambient", id(9));
    const wire = { ...page, view, items: [{ ...notification, archived_at: view === "archived" ? timestamp : null }] };
    const fetch = vi.fn().mockResolvedValue(response(wire));
    vi.stubGlobal("fetch", fetch);
    const cancellation = new AbortController();
    const signal = cancellation.signal;
    expect(await new ApiClient("https://server.example").getCollaborationNotifications({ view, cursor: "next+page/=" }, { signal })).toEqual(wire);
    const [url, init] = fetch.mock.calls[0]!;
    expect(new URL(url).searchParams).toEqual(new URLSearchParams({ limit: "50", cursor: "next+page/=", view }));
    expect(new URL(url).pathname).toBe("/api/me/collaboration-notifications");
    const headers = new Headers(init.headers);
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
    expect(init.signal.aborted).toBe(false);
    cancellation.abort();
    expect(init.signal.aborted).toBe(true);
    expect(init.method ?? "GET").toBe("GET");
  });

  it.each([undefined, "all"] as const)("accepts an unarchived legacy envelope for the %s view", async (view) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(page)));
    expect(await new ApiClient("https://server.example").getCollaborationNotifications({ view })).toEqual(page);
  });

  it.each(["unread", "archived"] as const)("requires the %s response identity even on an empty page", async (view) => {
    const api = new ApiClient("https://server.example");
    const empty = { items: [], unread_count: 7, next_cursor: null };
    for (const marker of [undefined, "all", view === "unread" ? "archived" : "unread"]) {
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ ...empty, view: marker })));
      expect(await api.getCollaborationNotifications({ view })).toBeNull();
    }
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ ...empty, view })));
    expect(await api.getCollaborationNotifications({ view })).toEqual({ ...empty, view });
  });

  it("refuses a mismatching explicit All marker", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ items: [], unread_count: 7, next_cursor: null, view: "archived" })));
    expect(await new ApiClient("https://server.example").getCollaborationNotifications({ view: "all" })).toBeNull();
  });

  it("preserves the legacy mixed include-archived request", async () => {
    const mixed = { ...page, items: [notification, { ...notification, id: id(4), archived_at: timestamp }] };
    const fetch = vi.fn().mockResolvedValue(response(mixed));
    vi.stubGlobal("fetch", fetch);
    expect(await new ApiClient("https://server.example").getCollaborationNotifications({ includeArchived: true })).toEqual(mixed);
    const query = new URL(fetch.mock.calls[0]?.[0]).searchParams;
    expect(query.get("include_archived")).toBe("true");
    expect(query.has("view")).toBe(false);
  });

  it.each([true, false])("refuses an explicit view combined with includeArchived=%s before transport", async (includeArchived) => {
    const fetch = vi.fn().mockResolvedValue(response(page));
    vi.stubGlobal("fetch", fetch);
    await expect(new ApiClient("https://server.example").getCollaborationNotifications({ view: "unread", includeArchived })).rejects.toBeInstanceOf(TypeError);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("does not let the default legacy view return archived membership", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ ...page, items: [{ ...notification, archived_at: timestamp }] })));
    expect(await new ApiClient("https://server.example").getCollaborationNotifications()).toBeNull();
  });

  it("validates and redacts malformed notification payloads", async () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ ...page, view: "unread", items: [{ ...notification, event_seq: "PRIVATE_SENTINEL" }] })));
    expect(await new ApiClient("https://server.example").getCollaborationNotifications({ view: "unread" })).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_SENTINEL");
  });

  it.each(["read", "archive"] as const)("retains the confirmed %s receipt without private issue data", async (action) => {
    const receipt = { id: notification.id, read_at: action === "read" ? timestamp : null, archived_at: action === "archive" ? timestamp : null };
    const fetch = vi.fn().mockResolvedValue(response(receipt));
    vi.stubGlobal("fetch", fetch);
    expect(await new ApiClient("https://server.example").updateCollaborationNotification(notification.id, action)).toEqual(receipt);
    expect(fetch.mock.calls[0]?.[0]).toBe(`https://server.example/api/me/collaboration-notifications/${notification.id}/${action}`);
    expect(fetch.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});
