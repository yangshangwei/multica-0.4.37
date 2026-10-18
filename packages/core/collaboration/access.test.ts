// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { collaborationKeys, collaborationScope, clearCollaborationAccess, protectCollaborationRead } from "./access";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const issueId = "00000000-0000-4000-8000-0000000000ab";
const clients: QueryClient[] = [];
afterEach(() => { for (const client of clients.splice(0)) client.clear(); });
function setup() {
  const api = new ApiClient("https://server.example");
  setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  return { api, client, scope: collaborationScope(auth.user.id) };
}

describe("collaboration read ownership", () => {
  it("separates current/participating lists, viewers and replacement API instances", () => {
    const { scope } = setup();
    expect(collaborationKeys.workbench(scope, "current")).not.toEqual(collaborationKeys.workbench(scope, "participating"));
    expect(collaborationKeys.issue(scope, issueId)).not.toEqual(collaborationKeys.issue({ ...scope, userId: "other" }, issueId));
    setApiInstance(new ApiClient(scope.server));
    expect(collaborationScope(auth.user.id).apiScope).not.toBe(scope.apiScope);
  });

  it("erases warm protected data and prevents a late read from surviving revocation", async () => {
    const { client, scope } = setup();
    const key = collaborationKeys.issue(scope, issueId);
    client.setQueryData(key, { title: "private sentinel" });
    let complete!: (value: string) => void;
    const pending = protectCollaborationRead(client, scope, issueId.toUpperCase(), () => new Promise<string>((resolve) => { complete = resolve; }));
    const cancellation = vi.spyOn(client, "cancelQueries");
    clearCollaborationAccess(client, scope, issueId);
    complete("late private sentinel");
    await expect(pending).rejects.toThrow();
    expect(client.getQueryData(key)).toBeUndefined();
    expect(cancellation).toHaveBeenCalledWith(expect.anything(), { revert: false });
  });

  it("ignores an old-session denial rather than clearing the new viewer's reads", async () => {
    const { api, client, scope } = setup();
    let fail!: (reason: unknown) => void;
    const pending = protectCollaborationRead(client, scope, issueId, () => new Promise((_resolve, reject) => { fail = reject; }));
    api.setToken("new-session");
    const currentScope = collaborationScope(auth.user.id);
    client.setQueryData(collaborationKeys.issue(currentScope, issueId), { title: "current" });
    fail(new ApiError("Denied", 403, "Forbidden"));
    await expect(pending).rejects.toThrow();
    expect(client.getQueryData(collaborationKeys.issue(currentScope, issueId))).toEqual({ title: "current" });
  });

  it("ignores a delayed denial after the same session recovers a newer access epoch", async () => {
    const { client, scope } = setup();
    const key = collaborationKeys.issue(scope, issueId);
    let fail!: (reason: unknown) => void;
    const pending = protectCollaborationRead(client, scope, issueId, () => new Promise((_resolve, reject) => { fail = reject; }));
    clearCollaborationAccess(client, scope, issueId);
    const recovered = await protectCollaborationRead(client, scope, issueId, async () => ({ title: "current permission" }), true);
    client.setQueryData(key, recovered);
    fail(new ApiError("Old denial", 403, "Forbidden"));
    await expect(pending).rejects.toMatchObject({ name: "AbortError" });
    expect(client.getQueryData(key)).toEqual(recovered);
    await expect(protectCollaborationRead(client, scope, issueId, async () => "still allowed")).resolves.toBe("still allowed");
  });

  it("keeps lawful content after a transient outage but clears it after authoritative denial", async () => {
    const { client, scope } = setup();
    const key = collaborationKeys.issue(scope, issueId);
    client.setQueryData(key, { title: "private" });
    await expect(protectCollaborationRead(client, scope, issueId, async () => { throw new ApiError("Busy", 500, "Error"); })).rejects.toThrow();
    expect(client.getQueryData(key)).toEqual({ title: "private" });
    await expect(protectCollaborationRead(client, scope, issueId, async () => { throw new ApiError("Denied", 404, "Not Found"); })).rejects.toThrow();
    expect(client.getQueryData(key)).toBeUndefined();
  });
});
