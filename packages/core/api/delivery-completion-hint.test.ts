// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
  status,
  headers: { "Content-Type": "application/json" },
});

const matching = {
  issue_id: "issue-1",
  issue_revision: 4,
  target_status: "done",
  target_category: "done",
  project_id: "project-1",
  policy_revision: 3,
  policy_mode: "observe",
  hint: "matching",
  reason: "observation_still_matches",
  checked_at: "2026-11-12T00:00:00Z",
  enforced: false,
  usable_for_completion: false,
};

afterEach(() => vi.unstubAllGlobals());

describe("ApiClient.getDeliveryCompletionHint", () => {
  it("sends exactly the three-field null-key request and parses public facts", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response(matching));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await expect(client.getDeliveryCompletionHint("issue-1", {
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    })).resolves.toEqual(matching);

    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    });
    expect(String(fetchMock.mock.calls[0]?.[0])).toContain("/api/issues/issue-1/delivery-evidence/completion-hint");
  });

  it("pins an explicitly scoped hint read to its workspace", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response(matching));
    vi.stubGlobal("fetch", fetchMock);
    await new ApiClient("https://api.example.test").getDeliveryCompletionHint("issue-1", {
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    }, { workspaceId: "workspace-1" });

    expect(fetchMock.mock.calls[0]?.[1]?.headers).toMatchObject({
      "X-Workspace-ID": "workspace-1",
      "X-Workspace-Slug": "",
    });
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    });
  });

  it("does not forward runtime-only extra fields or a smuggled key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response(matching));
    vi.stubGlobal("fetch", fetchMock);
    const input = {
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: "private-key",
      operation_id: "must-not-cross",
    } as unknown as Parameters<ApiClient["getDeliveryCompletionHint"]>[1];

    await new ApiClient("https://api.example.test").getDeliveryCompletionHint("issue-1", input);

    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    });
  });

  it("keeps attention responses non-authorizing", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ ...matching, hint: "attention", reason: "evaluation_required" }));
    vi.stubGlobal("fetch", fetchMock);
    const result = await new ApiClient("https://api.example.test").getDeliveryCompletionHint("issue-1", {
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    });
    expect(result).toMatchObject({ hint: "attention", enforced: false, usable_for_completion: false });
  });

  it("strips optional private evaluation data from the public DTO", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ ...matching, evaluation: { evaluation_key: "secret", observation: "private" } }));
    vi.stubGlobal("fetch", fetchMock);
    const result = await new ApiClient("https://api.example.test").getDeliveryCompletionHint("issue-1", {
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    });
    expect(result).toEqual(matching);
    expect(result).not.toHaveProperty("evaluation");
    expect(JSON.stringify(result)).not.toContain("secret");
  });

  it("rejects malformed 2xx instead of returning a fallback success", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ ...matching, issue_revision: "4" }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(new ApiClient("https://api.example.test").getDeliveryCompletionHint("issue-1", {
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    })).rejects.toMatchObject({ status: 502, body: { code: "invalid_delivery_completion_hint_response" } });
  });
});
