// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { IssueCreateDefaults } from "./types";
import { mergeIssueCreateDefaults } from "./create-defaults";

const fallback: IssueCreateDefaults = { assignee_type: "squad", assignee_id: "project-squad", status: "todo" };
const cases: Array<{ name: string; layers: Array<IssueCreateDefaults | undefined>; expected: IssueCreateDefaults }> = [
  { name: "project fallback", layers: [fallback, { project_id: "project-1" }], expected: { ...fallback, project_id: "project-1" } },
  { name: "scope assignee", layers: [fallback, { assignee_type: "member", assignee_id: "member-1" }], expected: { status: "todo", assignee_type: "member", assignee_id: "member-1" } },
  { name: "caller and group precedence", layers: [fallback, { status: "backlog", assignee_type: "member", assignee_id: "member-1" }, { status: "in_progress", assignee_type: "agent", assignee_id: "agent-1" }], expected: { status: "in_progress", assignee_type: "agent", assignee_id: "agent-1" } },
  { name: "explicit unassigned group", layers: [fallback, { assignee_type: null, assignee_id: null }], expected: { status: "todo", assignee_type: null, assignee_id: null } },
  { name: "clear by id", layers: [fallback, { assignee_id: null }], expected: { status: "todo", assignee_type: null, assignee_id: null } },
  { name: "clear by type", layers: [fallback, { assignee_type: null }], expected: { status: "todo", assignee_type: null, assignee_id: null } },
  { name: "incomplete changed type", layers: [fallback, { assignee_type: "member" }], expected: { status: "todo", assignee_type: null, assignee_id: null } },
  { name: "incomplete changed id", layers: [fallback, { assignee_id: "another-target" }], expected: { status: "todo", assignee_type: null, assignee_id: null } },
];

describe("issue creation defaults", () => {
  it.each(cases)("merges $name without mixing assignee identities", ({ layers, expected }) => {
    expect(mergeIssueCreateDefaults(...layers)).toEqual(expected);
  });
});
