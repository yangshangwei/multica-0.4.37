// @vitest-environment node
import { QueryClient, type FetchQueryOptions } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../api/client";
import type { Squad, SquadMember } from "../types";
import { squadDetailOptions, squadListOptions, squadMembersOptions, workspaceKeys } from "./queries";

const mocks = vi.hoisted(() => ({ get: vi.fn(), members: vi.fn(), list: vi.fn() }));
vi.mock("../api", () => ({ api: { getSquad: mocks.get, listSquadMembers: mocks.members, listSquads: mocks.list } }));

let client: QueryClient;
beforeEach(() => {
  vi.resetAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
});
afterEach(() => { client.clear(); });

function verifySquadBoundary<T>(name: string, options: () => FetchQueryOptions<T>, read: () => typeof mocks.get, first: T, second: T) {
 describe(`squad ${name} access boundary`, () => {
  it.each([401, 403, 404])("clears the exact cached snapshot after %i and cannot revive it after a network error", async (status) => {
    const query = options();
    client.setQueryData(query.queryKey, first);
    client.setQueryData(workspaceKeys.squads("ws-2"), [{ id: "other-squad" }]);
    const denied = new ApiError("unavailable", status, "Unavailable");
    read().mockRejectedValueOnce(denied).mockRejectedValueOnce(new Error("offline"));
    await expect(client.fetchQuery(query)).rejects.toBe(denied);
    expect(client.getQueryData(query.queryKey)).toBeUndefined();
    await expect(client.fetchQuery(query)).rejects.toThrow("offline");
    expect(client.getQueryData(query.queryKey)).toBeUndefined();
    expect(client.getQueryData(workspaceKeys.squads("ws-2"))).toEqual([{ id: "other-squad" }]);
  });

  it("keeps the newer snapshot when an already cancelled read later returns 404", async () => {
    const query = options();
    let reject!: (error: Error) => void;
    read().mockReturnValueOnce(new Promise((_resolve, fail) => { reject = fail; }));
    client.setQueryData(query.queryKey, first);
    const pending = client.fetchQuery(query).catch(() => undefined);
    await client.cancelQueries({ queryKey: query.queryKey, exact: true });
    client.setQueryData(query.queryKey, second);
    reject(new ApiError("old not found", 404, "Not Found"));
    await pending;
    await Promise.resolve();
    expect(client.getQueryData(query.queryKey)).toEqual(second);
  });

  it("preserves cached data on transient server failure", async () => {
    const query = options();
    client.setQueryData(query.queryKey, first);
    read().mockRejectedValueOnce(new ApiError("unavailable", 500, "Server Error"));
    await expect(client.fetchQuery(query)).rejects.toThrow("unavailable");
    expect(client.getQueryData(query.queryKey)).toEqual(first);
  });
});
}

const squad: Squad = {
  id: "sq-1", workspace_id: "ws-1", name: "Original squad", description: "", instructions: "",
  avatar_url: null, leader_id: "lead", creator_id: "viewer", created_at: "", updated_at: "",
  archived_at: null, archived_by: null,
};
const members: SquadMember[] = [{ id: "m-1", squad_id: "sq-1", member_type: "agent", member_id: "lead", role: "leader", created_at: "" }];
verifySquadBoundary("detail", () => squadDetailOptions("ws-1", "sq-1"), () => mocks.get, squad, { ...squad, name: "Newer squad" });
verifySquadBoundary("members", () => squadMembersOptions("ws-1", "sq-1"), () => mocks.members, members, []);

describe("squad directory access boundary", () => {
  it.each([401, 403])("does not restore denied candidates after %i followed by a transient retry failure", async (status) => {
    const query = squadListOptions("ws-1");
    client.setQueryData(query.queryKey, [squad]);
    mocks.list.mockRejectedValueOnce(new ApiError("denied", status, "Forbidden"))
      .mockRejectedValueOnce(new Error("offline"));

    await expect(client.fetchQuery(query)).rejects.toThrow("denied");
    expect(client.getQueryData(query.queryKey)).toBeUndefined();
    await expect(client.fetchQuery(query)).rejects.toThrow("offline");
    expect(client.getQueryData(query.queryKey)).toBeUndefined();
  });

  it("retains the catalog snapshot for an older server's unsupported endpoint", async () => {
    const query = squadListOptions("ws-1");
    client.setQueryData(query.queryKey, [squad]);
    mocks.list.mockRejectedValueOnce(new ApiError("unsupported", 404, "Not Found"));
    await expect(client.fetchQuery(query)).rejects.toThrow("unsupported");
    expect(client.getQueryData(query.queryKey)).toEqual([squad]);
  });
});
