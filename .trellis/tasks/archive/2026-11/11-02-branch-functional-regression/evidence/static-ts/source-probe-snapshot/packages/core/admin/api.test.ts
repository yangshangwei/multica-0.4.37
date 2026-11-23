// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
import { setCurrentWorkspace } from "../platform/workspace-storage";

afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); });

it("parses the admin endpoint before returning permission data", async () => {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ role: "super_admin" }), { status: 200 }));
  vi.stubGlobal("fetch", fetch);
  const controller = new AbortController();
  expect(await new ApiClient("https://admin.example").getAdminMe({ signal: controller.signal })).toBeNull();
  expect(fetch).toHaveBeenCalledWith("https://admin.example/api/admin/me", expect.objectContaining({ credentials: "include", signal: expect.any(AbortSignal) }));
});

it("retains the forbidden status for the independent admin guard", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "Forbidden", code: "admin_forbidden" }), { status: 403 })));
  await expect(new ApiClient("").getAdminMe()).rejects.toMatchObject({ status: 403 });
});

it("never inherits workspace routing headers even when a stale route mirror exists", async () => {
  setCurrentWorkspace("previous-workspace", "previous-id");
  const fetch = vi.fn().mockResolvedValue(new Response("null", { status: 200 }));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://admin.example");
  api.setToken("human-session");
  await api.getAdminMe();
  const headers = new Headers(fetch.mock.calls[0]?.[1]?.headers);
  expect(headers.get("Authorization")).toBe("Bearer human-session");
  expect(headers.has("X-Workspace-Slug")).toBe(false);
  expect(headers.has("X-Workspace-ID")).toBe(false);
});
