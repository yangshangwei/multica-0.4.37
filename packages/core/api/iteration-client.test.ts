// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
const ws = "10000000-0000-4000-8000-000000000001";
const response = (data: unknown, status = 200) =>
  new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
afterEach(() => vi.unstubAllGlobals());
describe("iteration capability compatibility", () => {
  it("confirms workspace access before classifying old-server 404", async () => {
    const fetch = vi
      .fn()
      .mockResolvedValueOnce(response({}, 404))
      .mockResolvedValueOnce(response({ id: ws }));
    vi.stubGlobal("fetch", fetch);
    expect(
      await new ApiClient("https://api.test").getIterationCapabilities(ws),
    ).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(2);
  });
  it("does not probe or downgrade the workspace-access-denied 404", async () => {
    const fetch = vi
      .fn()
      .mockImplementation(async () =>
        response({ error: "Denied", code: "workspace_access_denied" }, 404),
      );
    vi.stubGlobal("fetch", fetch);
    await expect(
      new ApiClient("https://api.test").getIterationCapabilities(ws),
    ).rejects.toMatchObject({
      status: 404,
      body: { code: "workspace_access_denied" },
    });
    expect(fetch).toHaveBeenCalledTimes(1);
  });
  it.each([400, 401, 403, 503])(
    "does not treat HTTP %s as unsupported",
    async (status) => {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(response({ error: "denied" }, status)),
      );
      await expect(
        new ApiClient("https://api.test").getIterationCapabilities(ws),
      ).rejects.toThrow();
    },
  );
  it("does not hide a missing workspace", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({}, 404)));
    await expect(
      new ApiClient("https://api.test").getIterationCapabilities(ws),
    ).rejects.toThrow();
  });
  it("rejects malformed capabilities", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(response({ workspace_id: ws, supported: "true" })),
    );
    await expect(
      new ApiClient("https://api.test").getIterationCapabilities(ws),
    ).rejects.toThrow();
  });
});
describe("iteration endpoint malformed responses", () => {
  const client = new ApiClient("https://api.test");
  const draft = {
    operation: "disable" as const,
    iteration_id: null,
    expected_iteration_revision: null,
    expected_scope_revision: null,
    expected_settings_revision: 1,
    reason: "disable",
    moves: [],
    start: null,
  };
  it.each([
    ["settings", () => client.getIterationSettings(ws)],
    ["list", () => client.listIterations(ws)],
    ["detail", () => client.getIteration(ws, ws)],
    ["issues", () => client.getIterationIssues(ws, ws)],
    ["events", () => client.getIterationEvents(ws, ws)],
    ["preview", () => client.previewIteration(ws, draft)],
    [
      "apply",
      () =>
        client.applyIterationOperation(ws, {
          request_id: ws,
          preview_hash: "hash",
          draft,
        }),
    ],
    ["lookup", () => client.getIterationOperation(ws, ws)],
    [
      "enable",
      () =>
        client.enableIterations(ws, {
          request_id: ws,
          expected_revision: 1,
          confirmed_timezone: "UTC",
        }),
    ],
    [
      "create",
      () =>
        client.createIteration(ws, {
          request_id: ws,
          name: "I1",
          description: null,
          coordinator_user_id: null,
          start_date: "2026-10-01",
          end_date: "2026-10-14",
          confirmed_timezone: "UTC",
        }),
    ],
    [
      "edit",
      () =>
        client.updateIteration(ws, ws, {
          request_id: ws,
          expected_revision: 1,
          fields: { name: "I2" },
          reason: "correct",
        }),
    ],
  ] as const)(
    "rejects malformed %s without an invented receipt",
    async (_name, call) => {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(response({ workspace_id: ws })),
      );
      await expect(call()).rejects.toThrow("Invalid iteration response");
    },
  );
});
