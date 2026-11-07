// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
import { parseCollaborationCommentHistory } from "./content-schema";

const issue = "00000000-0000-4000-8000-000000000001";
const comment = "00000000-0000-4000-8000-000000000002";
const entry = { event_id: issue, event_seq: "9007199254740993", kind: "comment_edited", actor: null, created_at: "2026-10-18T00:00:00Z", comment_revision: "2", before: "Original", after: "Edited" };
const history = { issue_id: issue, comment_id: comment, items: [entry] };
afterEach(() => vi.unstubAllGlobals());

it("retains comment history and refuses missing versions and repeated events", () => {
  expect(parseCollaborationCommentHistory(history)?.items[0]?.before).toBe("Original");
  expect(parseCollaborationCommentHistory({ ...history, items: [entry, entry] })).toBeNull();
  expect(parseCollaborationCommentHistory({ ...history, items: [{ ...entry, comment_revision: undefined }] })).toBeNull();
});

it("binds the history response to both the issue and requested comment", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(history)))
    .mockResolvedValueOnce(new Response(JSON.stringify({ ...history, comment_id: issue }))));
  const api = new ApiClient("https://server.example");
  await expect(api.getCollaborationCommentHistory(issue, comment)).resolves.toEqual(history);
  await expect(api.getCollaborationCommentHistory(issue, comment)).resolves.toBeNull();
});
