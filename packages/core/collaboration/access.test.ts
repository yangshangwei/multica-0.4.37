// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { CollaborationUnavailableError, collaborationIssueAccessDenied, collaborationKeys, collaborationScope, clearCollaborationAccess, protectCollaborationRead } from "./access";

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

  it("clears propagated denial from inactive comment queries after authoritative access recovery without fetching them", async () => {
    const { client, scope } = setup();
    const issueKey = collaborationKeys.issue(scope, issueId);
    const commentKey = [...issueKey, "comment", ""];
    const workbenchKey = collaborationKeys.workbench(scope, "current");
    const request = vi.fn(async () => ({ content: "No selected comment" }));
    const observer = new QueryObserver(client, { queryKey: commentKey, queryFn: request, enabled: false, initialData: { content: "Private comment" } });
    const unsubscribe = observer.subscribe(() => {});
    try {
      client.setQueryData(workbenchKey, { title: "Private issue" });
      clearCollaborationAccess(client, scope, issueId);
      expect(observer.getCurrentResult().error).toMatchObject({ status: 403 });

      const recovered = { title: "Now authorized" };
      await expect(client.fetchQuery({ queryKey: issueKey, queryFn: () => protectCollaborationRead(client, scope, issueId, async () => recovered, true) })).resolves.toEqual(recovered);

      expect(collaborationIssueAccessDenied(client, scope, issueId)).toBe(false);
      expect(observer.getCurrentResult()).toMatchObject({ error: null, data: undefined, status: "pending", fetchStatus: "idle" });
      expect(client.getQueryState(workbenchKey)).toMatchObject({ error: null, data: undefined, status: "pending" });
      expect(request).not.toHaveBeenCalled();
    } finally {
      unsubscribe();
    }
  });

  it("keeps real child errors and another issue's denial when one issue regains access", async () => {
    const { client, scope } = setup();
    const commentKey = [...collaborationKeys.issue(scope, issueId), "comment", ""];
    const otherId = "00000000-0000-4000-8000-0000000000ac";
    const otherKey = collaborationKeys.issue(scope, otherId);
    client.setQueryData(commentKey, { content: "Private comment" });
    client.setQueryData(otherKey, { title: "Other private issue" });
    clearCollaborationAccess(client, scope, otherId);
    clearCollaborationAccess(client, scope, issueId);
    const resourceKey = [...collaborationKeys.issue(scope, issueId), "activity"];
    const unavailable = new CollaborationUnavailableError();
    await expect(client.fetchQuery({ queryKey: resourceKey, retry: false, queryFn: async () => { throw unavailable; } })).rejects.toBe(unavailable);

    await protectCollaborationRead(client, scope, issueId, async () => ({ title: "Authorized again" }), true);

    expect(client.getQueryState(commentKey)?.error).toBeNull();
    expect(client.getQueryState(resourceKey)?.error).toBe(unavailable);
    expect(client.getQueryState(otherKey)?.error).toMatchObject({ status: 403 });
    expect(collaborationIssueAccessDenied(client, scope, otherId)).toBe(true);
  });

  it("retains denial while an authoritative recovery read is unavailable", async () => {
    const { client, scope } = setup();
    const commentKey = [...collaborationKeys.issue(scope, issueId), "comment", ""];
    client.setQueryData(commentKey, { content: "Private comment" });
    clearCollaborationAccess(client, scope, issueId);
    const error = client.getQueryState(commentKey)?.error;

    await expect(protectCollaborationRead(client, scope, issueId, async () => null, true)).rejects.toBeInstanceOf(CollaborationUnavailableError);

    expect(client.getQueryState(commentKey)).toMatchObject({ error, data: undefined, status: "error" });
    expect(collaborationIssueAccessDenied(client, scope, issueId)).toBe(true);
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
