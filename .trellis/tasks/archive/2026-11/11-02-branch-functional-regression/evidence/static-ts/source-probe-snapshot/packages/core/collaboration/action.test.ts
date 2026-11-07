// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope } from "./access";
import { parseCollaborationActionDraft, recoverCollaborationAction, sendCollaborationAction, type CollaborationAction } from "./action";
import type { CollaborationDraft } from "./draft";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const issueId = "00000000-0000-4000-8000-000000000002";
const key = "00000000-0000-4000-8000-000000000003";
const action: CollaborationAction = {
  action: "transfer", requestId: key, expectedCollaborationRevision: "9007199254740993", expectedLifecycleEpoch: "1",
  expectedParticipantEpoch: "1", expectedAccountEpoch: "1", targetUserId: auth.user.id, note: "Original handoff",
};
const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
afterEach(() => vi.unstubAllGlobals());

describe("collaboration human action recovery", () => {
  it("recovers the original receipt without performing another handoff", async () => {
    setApiInstance(new ApiClient("https://server.example"));
    const scope = collaborationScope(auth.user.id);
    const receipt = { code: "transferred", collaboration_revision: "9007199254740994" };
    const fetchMock = vi.fn().mockResolvedValue(response(receipt));
    vi.stubGlobal("fetch", fetchMock);
    const draft: CollaborationDraft = { scope: { ...scope, issueId }, requestId: key, kind: "operation", payload: action, createdAt: 1 };
    expect(await recoverCollaborationAction(scope, draft)).toEqual(receipt);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0]?.[0]).toContain(`/operations/${key}`);
  });

  it("retries the unchanged key, versions and payload when the first outcome is unknown", async () => {
    setApiInstance(new ApiClient("https://server.example"));
    const scope = collaborationScope(auth.user.id);
    const receipt = { code: "transferred", collaboration_revision: "9007199254740994" };
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new TypeError("Connection lost"))
      .mockResolvedValueOnce(response({ code: "operation_not_found", error: "Not found" }, 404))
      .mockResolvedValueOnce(response(receipt));
    vi.stubGlobal("fetch", fetchMock);
    await expect(sendCollaborationAction(scope, issueId, action)).rejects.toThrow();
    const draft: CollaborationDraft = { scope: { ...scope, issueId }, requestId: key, kind: "operation", payload: action, createdAt: 1 };
    expect(await recoverCollaborationAction(scope, draft)).toEqual(receipt);
    const first = fetchMock.mock.calls[0]?.[1];
    const retry = fetchMock.mock.calls[2]?.[1];
    expect(retry?.body).toBe(first?.body);
    expect(new Headers(retry?.headers).get("Idempotency-Key")).toBe(key);
    expect(JSON.parse(retry?.body)).toMatchObject({ expected_collaboration_revision: "9007199254740993", note: "Original handoff", purpose: "transfer" });
  });

  it("rejects a corrupt kind-specific payload and a key mismatch before replay", () => {
    const draft: CollaborationDraft = { scope: { server: "https://server.example", userId: auth.user.id, issueId }, requestId: key, kind: "operation", payload: { ...action, requestId: issueId }, createdAt: 1 };
    expect(() => parseCollaborationActionDraft(draft)).toThrow();
    expect(() => parseCollaborationActionDraft({ ...draft, payload: { ...action, expectedCollaborationRevision: 9 } })).toThrow();
  });
});
