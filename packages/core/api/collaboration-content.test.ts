// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import type { CollaborationCommentInput } from "../collaboration/content-schema";

const issueId = "00000000-0000-4000-8000-000000000001";
const otherId = "00000000-0000-4000-8000-000000000002";
const requestId = "00000000-0000-4000-8000-000000000003";
const response = (value: unknown) => new Response(JSON.stringify(value), { headers: { "Content-Type": "application/json" } });
afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); });

describe("collaboration content transport", () => {
  it("omits ambient workspace headers and rejects a valid page for another issue", async () => {
    setCurrentWorkspace("ambient", otherId);
    const fetchMock = vi.fn().mockResolvedValue(response({ issue_id: otherId, items: [], next_cursor: null }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://server.example");
    expect(await client.getCollaborationComments(issueId)).toBeNull();
    const headers = new Headers(fetchMock.mock.calls[0]?.[1]?.headers);
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
  });

  it("serializes the original comment versions and key unchanged on replay", async () => {
    const receipt = { status: 201, code: "comment_created", protocol_version: 2, issue_id: issueId, comment_id: otherId, comment_revision: "1", event_id: requestId, event_seq: "1", targets: [] };
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(response(receipt)));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://server.example");
    const input: CollaborationCommentInput = { content: "Original", parent_id: null, suppressed_target_ids: [], expected_lifecycle_epoch: "9007199254740993", expected_participant_epoch: null, expected_account_epoch: null, target_versions: [] };
    expect(await client.createCollaborationComment(issueId, requestId, input)).toEqual(receipt);
    await client.createCollaborationComment(issueId, requestId, input);
    for (const call of fetchMock.mock.calls) {
      expect(new Headers(call[1].headers).get("Idempotency-Key")).toBe(requestId);
      expect(JSON.parse(call[1].body)).toEqual(input);
    }
  });

  it("does not accept a read/archive receipt for a different notification", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ id: otherId, read_at: null, archived_at: null })));
    expect(await new ApiClient("https://server.example").updateCollaborationNotification(issueId, "archive")).toBeNull();
  });
});
