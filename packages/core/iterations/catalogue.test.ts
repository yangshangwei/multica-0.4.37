// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { beforeEach, expect, it, vi } from "vitest";
import { api, ApiError } from "../api";
import type { Iteration } from "../api/iteration-schemas";
import { iterationCatalogueOptions } from "./catalogue";

vi.mock("../api", async original => ({ ...await original<typeof import("../api")>(), api: { listIterations: vi.fn(), getSessionScope: vi.fn(() => "session") } }));
const period = (id: string): Iteration => ({ id, workspace_id: "w", name: id, description: null, coordinator_user_id: null, status: "planned", mode: "manual", start_date: "2026-10-01", end_date: "2026-10-14", timezone: "UTC", revision: 1, scope_revision: 1, started_at: null, logical_ended_at: null, processed_at: null });
const page = (ids: string[], next_cursor: string | null = null) => ({ workspace_id: "w", items: ids.map(period), next_cursor });
const stale = () => new ApiError("Changed", 409, "Conflict", { code: "cursor_stale" });
beforeEach(() => {
  vi.mocked(api.listIterations).mockReset();
  vi.mocked(api.getSessionScope).mockReset().mockReturnValue("session");
});

it("returns only a complete metadata catalogue and passes the query signal to every page", async () => {
  let finish!: (value: ReturnType<typeof page>) => void;
  vi.mocked(api.listIterations).mockResolvedValueOnce(page(["a"], "next"))
    .mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const options = iterationCatalogueOptions("w");
  expect(options.queryKey).toEqual(["iterations", "w", "catalogue"]);
  const client = new QueryClient();
  const request = client.fetchQuery(options);
  await vi.waitFor(() => expect(api.listIterations).toHaveBeenCalledTimes(2));
  expect(client.getQueryData(options.queryKey)).toBeUndefined();
  finish(page(["b"]));
  const items = await request;
  expect(items.map(item => item.id)).toEqual(["a", "b"]);
  expect(api.listIterations).toHaveBeenNthCalledWith(1, "w", { limit: "100" }, { signal: expect.any(AbortSignal) });
  expect(api.listIterations).toHaveBeenNthCalledWith(2, "w", { limit: "100", cursor: "next" }, { signal: expect.any(AbortSignal) });
  expect(vi.mocked(api.listIterations).mock.calls[1]?.[2]?.signal).toBe(vi.mocked(api.listIterations).mock.calls[0]?.[2]?.signal);
});

it("discards partial pages and restarts once after a stale cursor", async () => {
  vi.mocked(api.listIterations).mockResolvedValueOnce(page(["old"], "stale"))
    .mockRejectedValueOnce(stale()).mockResolvedValueOnce(page(["new"], "fresh"))
    .mockResolvedValueOnce(page(["last"]));
  expect((await new QueryClient().fetchQuery(iterationCatalogueOptions("w"))).map(item => item.id)).toEqual(["new", "last"]);
  expect(vi.mocked(api.listIterations).mock.calls.map(([, params]) => params?.cursor)).toEqual([undefined, "stale", undefined, "fresh"]);
});

it("bounds stale retries", async () => {
  vi.mocked(api.listIterations).mockRejectedValue(stale());
  await expect(new QueryClient().fetchQuery(iterationCatalogueOptions("w"))).rejects.toMatchObject({ status: 409 });
  expect(api.listIterations).toHaveBeenCalledTimes(2);
});

it("never retries unrelated failures", async () => {
  vi.mocked(api.listIterations).mockRejectedValue(new Error("offline"));
  await expect(new QueryClient().fetchQuery(iterationCatalogueOptions("w"))).rejects.toThrow("offline");
  expect(api.listIterations).toHaveBeenCalledTimes(1);
});

it("rejects repeated identities or cursors instead of publishing partial or duplicated rows", async () => {
  vi.mocked(api.listIterations).mockResolvedValueOnce(page(["a"], "next")).mockResolvedValueOnce(page(["b"], "next"));
  await expect(new QueryClient().fetchQuery(iterationCatalogueOptions("w"))).rejects.toThrow(/cursor/i);
  vi.mocked(api.listIterations).mockResolvedValueOnce(page(["a"], "next")).mockResolvedValueOnce(page(["a"]));
  await expect(new QueryClient().fetchQuery(iterationCatalogueOptions("w"))).rejects.toThrow(/identity/i);
});

it("checks both page and row workspace identities", async () => {
  vi.mocked(api.listIterations).mockResolvedValueOnce({ ...page(["a"]), workspace_id: "other" });
  await expect(new QueryClient().fetchQuery(iterationCatalogueOptions("w"))).rejects.toThrow(/workspace/i);
  vi.mocked(api.listIterations).mockResolvedValueOnce({ ...page([]), items: [{ ...period("a"), workspace_id: "other" }] });
  await expect(new QueryClient().fetchQuery(iterationCatalogueOptions("w"))).rejects.toThrow(/workspace/i);
});

it("does not publish a result returned from a previous authenticated session", async () => {
  vi.mocked(api.listIterations).mockImplementationOnce(async () => {
    vi.mocked(api.getSessionScope).mockReturnValueOnce("new-session");
    return page(["a"]);
  });
  await expect(new QueryClient().fetchQuery(iterationCatalogueOptions("w"))).rejects.toThrow("Iteration session changed");
});
