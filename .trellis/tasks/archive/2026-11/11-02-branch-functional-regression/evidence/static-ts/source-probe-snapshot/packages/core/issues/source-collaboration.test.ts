// @vitest-environment node
import { describe, expect, it } from "vitest";
import { IssueSchema } from "../api/schemas";

const account = { user_id: "11111111-1111-4111-8111-111111111111", name: "External colleague", avatar_url: "", external: true };
const issue = { id: "22222222-2222-4222-8222-222222222222", workspace_id: "33333333-3333-4333-8333-333333333333", number: 1,
  identifier: "SRC-1", title: "Source task", description: null, status: "todo", priority: "none", assignee_type: "member", assignee_id: account.user_id,
  creator_type: "member", creator_id: account.user_id, parent_issue_id: null, project_id: null, position: 0, start_date: null, due_date: null,
  metadata: {}, properties: {}, created_at: "2026-10-19T00:00:00Z", updated_at: "2026-10-19T00:00:00Z" };

describe("source collaboration projection", () => {
  it("strips private account fields while retaining source identity", () => {
    const parsed = IssueSchema.parse({ ...issue, collaboration: { creator: account, handler: { ...account, email: "private@example.test", username: "private" }, has_external_collaborators: true } });
    expect(parsed.collaboration).toEqual({ creator: account, handler: account, has_external_collaborators: true });
    expect(parsed.id).toBe(issue.id);
  });
  it.each([true, "collaboration", {}, { creator: account, handler: { ...account, user_id: "bad" }, has_external_collaborators: true }])("never treats a malformed marker as ordinary", (collaboration) => {
    expect(IssueSchema.safeParse({ ...issue, collaboration }).success).toBe(false);
  });
  it("keeps ordinary responses compatible", () => {
    expect(IssueSchema.parse(issue).collaboration).toBeUndefined();
  });
});

import type { Issue } from "../types";
import { sourceCollaborationAction } from "./source-collaboration";

const shared = IssueSchema.parse({ ...issue, collaboration: { creator: account, handler: account, has_external_collaborators: true } }) as Issue;
describe("source collaboration update boundary", () => {
  it("keeps column-local position writes out of workflow revisions", () => {
    expect(sourceCollaborationAction(shared, { position: 4, status: shared.status, assignee_type: shared.assignee_type, assignee_id: shared.assignee_id })).toBeNull();
  });
  it("opens transfer for an assignee change instead of a generic assignment", () => {
    expect(sourceCollaborationAction(shared, { assignee_id: "other", position: 4 })?.action).toBe("transfer");
  });
  it("separates terminal category drags from ordinary collaboration status edits", () => {
    expect(sourceCollaborationAction(shared, { status: "custom_review" }, "in_review")?.action).toBe("change_status");
    expect(sourceCollaborationAction(shared, { status: "custom_done" }, "done")?.action).toBe("lifecycle");
    expect(sourceCollaborationAction(shared, { status: "cancelled" })?.action).toBe("lifecycle");
  });
  it("uses the field workflow and leaves ordinary task behavior to the existing action", () => {
    expect(sourceCollaborationAction(shared, { title: "New title" })?.action).toBe("edit");
    expect(sourceCollaborationAction({ ...shared, collaboration: undefined }, { status: "done" })).toBeNull();
  });
});

import { issueMatchesListFilter } from "./surface/membership";
it("does not insert an ordinary update into an external collaboration list", () => {
  expect(issueMatchesListFilter(shared, undefined, { external_collaborators: true })).toBe(true);
  expect(issueMatchesListFilter({ ...shared, collaboration: undefined }, undefined, { external_collaborators: true })).toBe(false);
  expect(issueMatchesListFilter({ status: "todo" }, undefined, { external_collaborators: true })).toBe("unknown");
});
