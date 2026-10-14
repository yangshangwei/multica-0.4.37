// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";

const requester = { user_id: "00000000-0000-4000-8000-000000000002", name: "Developer", avatar_url: "", external: true };
const recipient = { user_id: "00000000-0000-4000-8000-000000000003", name: "Product", avatar_url: "", external: false };
const request = {
  id: "00000000-0000-4000-8000-000000000004", requester, recipient, note: "PRIVATE_QUESTION",
  return_status: "in_progress", return_status_fallback: false, created_at: "2026-10-13T12:00:00Z",
};
const directed = { request_information: true, return_information: true, submit_acceptance: true };

afterEach(() => setSchemaLogger(noopLogger));

describe("collaboration directed handoff boundary", () => {
  it("parses the open request with its requester and restore target", () => {
    const issue = parseCollaborationIssue({ ...collaborationIssueFixture, current_information_request: request, capabilities: { ...collaborationIssueFixture.capabilities, ...directed } });
    expect(issue?.current_information_request).toEqual(request);
    expect(issue?.capabilities).toMatchObject(directed);
  });

  it("keeps no request distinct from an unavailable restore target", () => {
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, current_information_request: null })?.current_information_request).toBeNull();
    expect(parseCollaborationIssue(collaborationIssueFixture)?.current_information_request).toBeUndefined();
    const unavailable = parseCollaborationIssue({ ...collaborationIssueFixture, current_information_request: { ...request, return_status: null, return_status_fallback: true } });
    expect(unavailable?.current_information_request?.return_status).toBeNull();
  });

  // The requester decides where a return lands; a damaged value never degrades
  // to "no request" or to some other account.
  it.each([
    ["missing requester", { ...request, requester: undefined }],
    ["invalid requester id", { ...request, requester: { ...requester, user_id: "PRIVATE_ID" } }],
    ["invalid request id", { ...request, id: "not-a-uuid" }],
    ["empty restore target", { ...request, return_status: "" }],
    ["missing fallback flag", { ...request, return_status_fallback: undefined }],
    ["string payload", "PRIVATE_REQUEST"],
  ])("rejects a malformed present request: %s", (_, value) => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, current_information_request: value })).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE");
  });

  it("treats missing or malformed directed capabilities from an older server as unavailable", () => {
    expect(parseCollaborationIssue(collaborationIssueFixture)?.capabilities).toMatchObject({
      request_information: false, return_information: false, submit_acceptance: false,
    });
    const malformed = { ...collaborationIssueFixture.capabilities, request_information: "true", return_information: 1, submit_acceptance: null };
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, capabilities: malformed })?.capabilities).toMatchObject({
      request_information: false, return_information: false, submit_acceptance: false,
    });
  });
});
