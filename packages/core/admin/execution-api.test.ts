// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
const id = "11111111-1111-4111-8111-111111111111";
afterEach(() => vi.unstubAllGlobals());
it("parses list endpoints and preserves signed cursors and filters", async () => {
  const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({
    items: [], next_cursor: null, as_of: "2026-10-01T00:00:00Z", scope: id
  }), { status: 200 }));
  vi.stubGlobal("fetch", fetcher);
  const client = new ApiClient("https://example.test");
  const result = await client.getAdminTasks(new URLSearchParams({ cursor: "signed.cursor", status: "failed" }));
  expect(result?.items).toEqual([]);
  expect(fetcher.mock.calls[0]?.[0]).toBe("https://example.test/api/admin/tasks?cursor=signed.cursor&status=failed");
});
it("fails closed on malformed task and issue response bodies", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({
    id, content_access: true, error: "PRIVATE"
  }), { status: 200 }))));
  const client = new ApiClient("https://example.test");
  expect(await client.getAdminTask(id)).toBeNull();
  expect(await client.getAdminIssues(new URLSearchParams())).toBeNull();
});
