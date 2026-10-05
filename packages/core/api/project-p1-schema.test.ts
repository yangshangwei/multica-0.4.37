// @vitest-environment node
import { describe, expect, it } from "vitest";
import { ProjectSchema } from "./schemas";
import { p1Project } from "../projects/test-fixtures/p1";

describe("P1 additive project fields", () => {
  it("preserves the separate completion and cancellation counts and description version", () => {
    expect(ProjectSchema.parse(p1Project)).toMatchObject({
      done_count: 7, completed_issue_count: 5, cancelled_issue_count: 2, open_issue_count: 3,
      revision: 4, description_revision: 2, statistics_complete: true,
    });
  });

  it("does not invent completion or a CAS token for a legacy response", () => {
    const { revision: _revision, description_revision: _descriptionRevision,
      completed_issue_count: _completed, cancelled_issue_count: _cancelled,
      open_issue_count: _open, statistics_complete: _complete, ...legacy } = p1Project;
    const project = ProjectSchema.parse(legacy);
    expect(project.completed_issue_count).toBeUndefined();
    expect(project.description_revision).toBeUndefined();
    expect(project.done_count).toBe(7);
  });

  it.each([
    { revision: 0 }, { issue_count: -1 }, { done_count: 1.5 }, { resource_count: -1 },
    { issue_count: 12 }, { done_count: 6 }, { description_revision: -1 },
    { revision: Number.MAX_SAFE_INTEGER + 1 }, { description_revision: 1.5 },
    { completed_issue_count: -1 }, { cancelled_issue_count: "2" },
    { open_issue_count: Number.MAX_SAFE_INTEGER + 1 }, { statistics_complete: "true" },
  ])("rejects malformed present P1 fields: %j", (fields) => {
    expect(ProjectSchema.safeParse({ ...p1Project, ...fields }).success).toBe(false);
  });
});
