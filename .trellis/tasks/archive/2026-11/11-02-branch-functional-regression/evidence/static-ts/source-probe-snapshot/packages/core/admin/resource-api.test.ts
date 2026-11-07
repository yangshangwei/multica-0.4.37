// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
import { setCurrentWorkspace } from "../platform/workspace-storage";

afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); });
it("sends multipart with session safeguards and no workspace or JSON content-type headers", async () => {
  setCurrentWorkspace("old-workspace", "old-id");
  const fetch = vi.fn().mockResolvedValue(new Response("null"));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://admin.example");
  api.setToken("human-session");
  await api.previewAdminResource("skill", { key: "sample", file: new Blob(["test"]), filename: "SKILL.md" });
  const init = fetch.mock.calls[0]?.[1];
  const headers = new Headers(init.headers);
  expect(headers.get("Authorization")).toBe("Bearer human-session");
  expect(headers.has("Content-Type")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
  expect(init.body.get("key")).toBe("sample");
  expect(init.redirect).toBe("error");
});
it("rejects a multipart response whose credentials changed while reading the body", async () => {
  let release!: (value: unknown) => void;
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, status: 200, json: () => new Promise(resolve => { release = resolve; }) }));
  const api = new ApiClient("https://admin.example");
  const request = api.previewAdminResource("skill", { key: "sample", file: new Blob(["test"]), filename: "SKILL.md" });
  await vi.waitFor(() => expect(release).toBeDefined());
  api.setToken("replacement");
  release(null);
  await expect(request).rejects.toThrow("Admin session changed");
});

it("never logs a resource error body that could repeat uploaded secrets", async () => {
  const logger = { info: vi.fn(), error: vi.fn(), warn: vi.fn(), debug: vi.fn() };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "uploaded-secret-value", code: "resource_invalid" }), { status: 400 })));
  const api = new ApiClient("https://admin.example", { logger });
  await expect(api.previewAdminResource("mcp", { key: "sample", file: new Blob(["uploaded-secret-value"]), filename: "mcp.json" })).rejects.toMatchObject({ status: 400 });
  expect(JSON.stringify(logger.error.mock.calls)).not.toContain("uploaded-secret-value");
});

it("sends publish revision and digest independently with the supplied operation key", async () => {
  const fetch = vi.fn().mockResolvedValue(new Response("null")); vi.stubGlobal("fetch", fetch);
  await new ApiClient("").publishAdminResource("skill", { key: "sample", file: new Blob(["data"]), filename: "SKILL.md", previewDigest: "content-digest", expectedVersion: "mutation-revision", reason: "update" }, "original-operation");
  const init = fetch.mock.calls[0]?.[1];
  expect(new Headers(init.headers).get("Idempotency-Key")).toBe("original-operation");
  expect(init.body.get("expected_version")).toBe("mutation-revision");
  expect(init.body.get("preview_digest")).toBe("content-digest");
});
