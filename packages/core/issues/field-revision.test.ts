// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { Issue, UpdateIssueRequest } from "../types";
import { withObservedIssueFieldRevision } from "./mutations";

type Observation = Pick<Issue, "revision" | "collaboration">;

const fields: Array<{ name: string; updates: Partial<UpdateIssueRequest> }> = [
  { name: "priority", updates: { priority: "high" } },
  { name: "member", updates: { assignee_type: "member", assignee_id: "member-1" } },
  { name: "agent", updates: { assignee_type: "agent", assignee_id: "agent-1" } },
  { name: "squad", updates: { assignee_type: "squad", assignee_id: "squad-1" } },
  { name: "unassigned", updates: { assignee_type: null, assignee_id: null } },
  { name: "assignee type alone", updates: { assignee_type: null } },
  { name: "assignee identity alone", updates: { assignee_id: null } },
];

describe("withObservedIssueFieldRevision", () => {
  it.each(fields)("pins the supplied observation for $name", ({ updates }) => {
    const original = { ...updates };
    const issue: Observation = { revision: 41 };
    expect(withObservedIssueFieldRevision(issue, updates)).toEqual({
      ...original,
      expected_revision: 41,
    });
    expect(updates).toEqual(original);
    expect(issue).toEqual({ revision: 41 });
  });

  it.each([1, Number.MAX_SAFE_INTEGER])("uses a known safe positive revision %s", (revision) => {
    expect(withObservedIssueFieldRevision({ revision }, { priority: "low" })).toEqual({
      priority: "low",
      expected_revision: revision,
    });
  });

  it.each([undefined, 0, -1, 1.5, Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY, Number.MAX_SAFE_INTEGER + 1])(
    "preserves versionless compatibility for an unavailable revision %s",
    (revision) => {
      const updates: Partial<UpdateIssueRequest> = { assignee_type: null, assignee_id: null };
      expect(withObservedIssueFieldRevision({ revision }, updates)).toBe(updates);
      expect(updates).not.toHaveProperty("expected_revision");
    },
  );

  it("keeps an explicit token instead of replacing it with the supplied issue revision", () => {
    const updates: Partial<UpdateIssueRequest> = { priority: "high", expected_revision: 7 };
    expect(withObservedIssueFieldRevision({ revision: 8 }, updates)).toBe(updates);
    expect(updates.expected_revision).toBe(7);
  });

  it("does not fabricate an observation when no issue was supplied", () => {
    const updates: Partial<UpdateIssueRequest> = { priority: "high" };
    expect(withObservedIssueFieldRevision(undefined, updates)).toBe(updates);
  });

  it("does not manufacture an ordinary token for a collaboration projection", () => {
    const account = { user_id: "user-1", name: "Handler", avatar_url: "", external: true };
    const issue: Observation = {
      revision: 41,
      collaboration: { creator: account, handler: account, has_external_collaborators: true },
    };
    const updates: Partial<UpdateIssueRequest> = { priority: "high" };
    expect(withObservedIssueFieldRevision(issue, updates)).toBe(updates);
  });

  it.each<Partial<UpdateIssueRequest>>([
    { status: "done" },
    { title: "Renamed" },
    { position: 2 },
    { due_date: null },
    { parent_issue_id: null },
    {},
  ])("leaves other mutation observations with their current owner (%j)", (updates) => {
    expect(withObservedIssueFieldRevision({ revision: 41 }, updates)).toBe(updates);
    expect(updates).not.toHaveProperty("expected_revision");
  });
});
