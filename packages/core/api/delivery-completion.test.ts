// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { completionIds as ids, completionPreviewFixture, completionAuthorizationFixture, completionIssueFixture, completionReceiptFixture } from "../issues/completion-test-fixtures";

const response = (value: unknown) => new Response(JSON.stringify(value), { headers: { "Content-Type": "application/json" } });
afterEach(() => vi.unstubAllGlobals());

describe("ordinary completion API", () => {
  it("uses the explicit workspace and exact preview fields, stripping no private key into the request", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response(completionPreviewFixture()));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");
    const input = { expected_issue_revision: 7, target_status: "done", approval_id: ids.approval };
    await client.previewDeliveryCompletion(ids.issue, input, { workspaceId: ids.workspace });
    expect(JSON.parse(fetchMock.mock.calls[0]![1].body)).toEqual(input);
    expect(fetchMock.mock.calls[0]![1].headers["X-Workspace-ID"]).toBe(ids.workspace);
  });

  it("rejects malformed, unsafe or contradictory preview data instead of enabling acceptance", async () => {
    const client = new ApiClient("https://api.example.test");
    for (const change of [{ issue_revision: Number.MAX_SAFE_INTEGER + 1 }, { enforced: true }, { policy_mode: "future" }, { target_status: "other" }]) {
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ ...completionPreviewFixture(), ...change })));
      await expect(client.previewDeliveryCompletion(ids.issue, { expected_issue_revision: 7, target_status: "done", approval_id: ids.approval }, { workspaceId: ids.workspace })).rejects.toMatchObject({ status: 502 });
    }
  });

  it("keeps creation key in the header and request recovery workspace in the query", async () => {
    const fetchMock = vi.fn().mockImplementation(async () => response(completionAuthorizationFixture()));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");
    const request = { ...completionPreviewFixture().authorization_binding, operation_id: ids.operation, expires_at: "2099-01-01T00:15:00Z", reason: "Accepted" };
    await client.createDeliveryCompletionAuthorization(ids.key, request, { workspaceId: ids.workspace });
    expect(fetchMock.mock.calls[0]![1].headers["Idempotency-Key"]).toBe(ids.key);
    expect(JSON.parse(fetchMock.mock.calls[0]![1].body)).toEqual(request);
    await client.getDeliveryCompletionAuthorizationByKey(ids.key, { workspaceId: ids.workspace });
    expect(String(fetchMock.mock.calls[1]![0])).toContain(`by-request-key/${ids.key}?workspace_id=${ids.workspace}`);
  });

  it("requires a matching completion receipt and keeps newer current state separate", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ ...completionIssueFixture({ status: "todo", status_category: "todo", revision: 9 }), completion_receipt: completionReceiptFixture() }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");
    const binding = { authorization_id: ids.authorization, operation_id: ids.operation, expected_issue_revision: 7 };
    const result = await client.submitDeliveryCompletion(ids.issue, { status: "done", completion: binding }, { workspaceId: ids.workspace });
    expect(result.issue.status).toBe("todo");
    expect(result.receipt.completed_revision).toBe(8);
    expect(JSON.parse(fetchMock.mock.calls[0]![1].body)).toEqual({ status: "done", completion: binding });
    fetchMock.mockResolvedValue(response({ ...completionIssueFixture({ revision: 8 }), completion_receipt: { ...completionReceiptFixture(), operation_id: ids.key } }));
    await expect(client.submitDeliveryCompletion(ids.issue, { status: "done", completion: binding }, { workspaceId: ids.workspace })).rejects.toMatchObject({ status: 502 });
  });

  it("does not expose private evaluation fields from an authorization response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ ...completionAuthorizationFixture(), evaluation_key: "private", issuer_credential: "secret" })));
    const result = await new ApiClient("https://api.example.test").getDeliveryCompletionAuthorization(ids.authorization, { workspaceId: ids.workspace });
    expect(result).not.toHaveProperty("evaluation_key");
    expect(result).not.toHaveProperty("issuer_credential");
  });
});
