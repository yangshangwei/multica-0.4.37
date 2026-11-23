// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationRecipient } from "./schema";

const issueId = "00000000-0000-4000-8000-000000000001";
const sourceWorkspaceId = "00000000-0000-4000-8000-000000000002";
const recipient = { user_id: "00000000-0000-4000-8000-000000000003", name: "Recipient", avatar_url: "", external: true };
const response = { issue_id: issueId, source_workspace_id: sourceWorkspaceId, recipient };

afterEach(() => setSchemaLogger(noopLogger));

it("retains the explicit lookup scope and only the minimal confirmation summary", () => {
  expect(parseCollaborationRecipient({
    ...response,
    value: "PRIVATE_QUERY",
    recipient: { ...recipient, email: "PRIVATE_EMAIL", username: "PRIVATE_USERNAME", token: "PRIVATE_TOKEN" },
  })).toEqual(response);
  expect(parseCollaborationRecipient({ ...response, issue_id: null })).toEqual({ ...response, issue_id: null });
});

it.each([
  { issue_id: undefined },
  { issue_id: "PRIVATE_ISSUE" },
  { source_workspace_id: undefined },
  { source_workspace_id: "PRIVATE_SOURCE" },
  { recipient: null },
  { recipient: { ...recipient, user_id: "PRIVATE_USER" } },
  { recipient: { ...recipient, external: "true" } },
  { recipient: { ...recipient, external: undefined } },
])("rejects incomplete confirmation authority without logging private data", (patch) => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  expect(parseCollaborationRecipient({ ...response, ...patch })).toBeNull();
  expect(warn).toHaveBeenCalledOnce();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
});
