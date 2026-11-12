// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import type { StorageAdapter } from "../types/storage";
import { completionIds as ids, completionAuthorizationFixture, completionIssueFixture, completionPreviewFixture, completionReceiptFixture } from "./completion-test-fixtures";
import { deliveryCompletionScope } from "./completion";
import { readCompletionDrafts } from "./completion-draft";
import { authorizeIssueCompletion, recoverIssueCompletion, submitIssueCompletion, retireCompletionDraft, revokeIssueCompletion } from "./completion-queries";

const current = vi.hoisted(() => ({ userId: "33333333-3333-4333-8333-333333333333" }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: current.userId } }) } }));
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { "Content-Type": "application/json" } });
function storage(): StorageAdapter {
  const data = new Map<string, string>();
  return { getItem: key => data.get(key) ?? null, setItem: (key, value) => { data.set(key, value); }, removeItem: key => { data.delete(key); }, keys: () => [...data.keys()] };
}
beforeEach(() => { current.userId = ids.user; setApiInstance(new ApiClient("https://api.test")); });
afterEach(() => vi.unstubAllGlobals());
const scope = () => deliveryCompletionScope(ids.user, ids.workspace, ids.issue);
const input = { reason: "Human accepts this artifact", expiresAt: "2099-01-01T00:15:00Z" };

describe("completion original-operation recovery", () => {
  it("retains a timed-out create and recovers issued without another POST", async () => {
    const store = storage(), s = scope();
    const fetchMock = vi.fn().mockRejectedValueOnce(new Error("response lost"));
    vi.stubGlobal("fetch", fetchMock);
    const first = await authorizeIssueCompletion(s, completionPreviewFixture(), input, { storage: store });
    expect(first.outcome).toBe("unknown");
    const draft = readCompletionDrafts(s, store)[0]!;
    fetchMock.mockImplementation(async (url: string) => response(url.includes("delivery-completion-authorizations") ? completionAuthorizationFixture(draft.request.operation_id) : completionIssueFixture()));
    const recovered = await recoverIssueCompletion(s, draft, { storage: store });
    expect(recovered.outcome).toBe("authorized");
    expect(recovered.draft.submissionDispatched).toBe(false);
    expect(fetchMock.mock.calls.filter(([, init]) => init?.method === "POST")).toHaveLength(1);
  });

  it("never re-PUTs an unknown completion merely because the authorization is issued", async () => {
    const store = storage(), s = scope();
    let operationId = "";
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      if (init?.method === "POST") operationId = JSON.parse(String(init.body)).operation_id;
      if (init?.method === "PUT") throw new Error("commit unknown");
      return response(url.includes("delivery-completion-authorizations") ? completionAuthorizationFixture(operationId) : completionIssueFixture());
    });
    vi.stubGlobal("fetch", fetchMock);
    const authorized = await authorizeIssueCompletion(s, completionPreviewFixture(), input, { storage: store });
    const failed = await submitIssueCompletion(s, authorized.draft, { storage: store });
    expect(failed.outcome).toBe("unknown");
    const replay = await submitIssueCompletion(s, failed.draft, { storage: store });
    expect(replay.outcome).toBe("unknown");
    expect(fetchMock.mock.calls.filter(([, init]) => init?.method === "PUT")).toHaveLength(1);
    expect(() => retireCompletionDraft(s, replay, { storage: store })).toThrow();
  });

  it("treats missing recovery and lock conflicts as unknown, never a new create", async () => {
    const store = storage(), s = scope();
    const fetchMock = vi.fn().mockRejectedValueOnce(new Error("lost"));
    vi.stubGlobal("fetch", fetchMock);
    const first = await authorizeIssueCompletion(s, completionPreviewFixture(), input, { storage: store });
    for (const status of [404, 409, 503]) {
      fetchMock.mockImplementation(async () => response({ code: "delivery_completion_not_found" }, status));
      expect((await recoverIssueCompletion(s, first.draft, { storage: store })).outcome).toBe("unknown");
    }
    expect(fetchMock.mock.calls.filter(([, init]) => init?.method === "POST")).toHaveLength(1);
  });

  it("keeps a historical receipt separate from a reopened current issue", async () => {
    const store = storage(), s = scope();
    let operationId = "", consumed = false;
    vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
      if (init?.method === "POST") operationId = JSON.parse(String(init.body)).operation_id;
      const issued = completionAuthorizationFixture(operationId);
      if (init?.method === "PUT") { consumed = true; throw new Error("lost committed response"); }
      if (url.includes("delivery-completion-authorizations")) return response(consumed ? { ...issued, state: "consumed", applicability: "consumed", completion_receipt: completionReceiptFixture(operationId) } : issued);
      return response(completionIssueFixture({ status: "todo", status_category: "todo", revision: 9 }));
    }));
    const authorized = await authorizeIssueCompletion(s, completionPreviewFixture(), input, { storage: store });
    const pending = await submitIssueCompletion(s, authorized.draft, { storage: store });
    const result = await recoverIssueCompletion(s, pending.draft, { storage: store });
    expect(result.outcome).toBe("consumed");
    expect(result.issue?.status).toBe("todo");
    expect(result.submittedNow).toBe(false);
    retireCompletionDraft(s, result, { storage: store });
    expect(readCompletionDrafts(s, store)).toEqual([]);
  });

  it("persists before network dispatch and coalesces duplicate same-intent authorization", async () => {
    const store = storage(), s = scope();
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      expect(readCompletionDrafts(s, store)[0]?.creationDispatched).toBe(true);
      return response(completionAuthorizationFixture(JSON.parse(String(init?.body)).operation_id));
    });
    vi.stubGlobal("fetch", fetchMock);
    await Promise.all([authorizeIssueCompletion(s, completionPreviewFixture(), input, { storage: store }), authorizeIssueCompletion(s, completionPreviewFixture(), input, { storage: store })]);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const otherScope = deliveryCompletionScope(ids.user, ids.workspace, ids.key);
    await expect(authorizeIssueCompletion(otherScope, { ...completionPreviewFixture(), issue_id: ids.key }, input, { storage: { ...store, setItem: () => {} } })).rejects.toBeDefined();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("does not publish a late result into a replaced identity", async () => {
    const store = storage(), s = scope();
    vi.stubGlobal("fetch", vi.fn(async (_url: string, init?: RequestInit) => {
      current.userId = ids.operation;
      return response(completionAuthorizationFixture(JSON.parse(String(init?.body)).operation_id));
    }));
    await expect(authorizeIssueCompletion(s, completionPreviewFixture(), input, { storage: store })).rejects.toMatchObject({ name: "AbortError" });
    expect(readCompletionDrafts(s, store)[0]?.authorizationId).toBe(null);
  });

  it("allows the first revoke after creation but queries after a revoked-response timeout", async () => {
    const store = storage(), s = scope();
    let operationId = "";
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith("/revocations")) throw new Error("lost revoke");
      if (init?.method === "POST") operationId = JSON.parse(String(init.body)).operation_id;
      return response(url.includes("delivery-completion-authorizations") ? completionAuthorizationFixture(operationId) : completionIssueFixture());
    });
    vi.stubGlobal("fetch", fetchMock);
    const authorized = await authorizeIssueCompletion(s, completionPreviewFixture(), input, { storage: store });
    const revoked = await revokeIssueCompletion(s, authorized.draft, "Changed my mind", { storage: store });
    expect(revoked.outcome).toBe("unknown");
    expect((await revokeIssueCompletion(s, revoked.draft, "Changed my mind", { storage: store })).outcome).toBe("unknown");
    expect(fetchMock.mock.calls.filter(([url]) => String(url).endsWith("/revocations"))).toHaveLength(1);
  });
});
