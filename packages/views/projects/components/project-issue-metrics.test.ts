// @vitest-environment node
import { describe, expect, it } from "vitest";
import { getProjectIssueMetrics } from "./project-issue-metrics";

describe("getProjectIssueMetrics", () => {
  it("surfaces project-level totals from the project record", () => {
    const metrics = getProjectIssueMetrics({ issue_count: 9, done_count: 5 });

    expect(metrics).toEqual({
      totalCount: 9,
      completedCount: null,
      cancelledCount: null,
      openCount: null,
      closedCount: 5,
      closureRatio: 5 / 9,
      complete: false,
    });
  });
});

describe("P1 completion metrics", () => {
  it("keeps actual completion separate from closed scope", () => {
    const project = {
      issue_count: 10, done_count: 7,
      completed_issue_count: 5, cancelled_issue_count: 2, open_issue_count: 3,
      statistics_complete: true,
    };
    expect(getProjectIssueMetrics(project)).toMatchObject({
      totalCount: 10, completedCount: 5, cancelledCount: 2, openCount: 3,
      closedCount: 7, closureRatio: 0.7, complete: true,
    });
  });

  it("does not mislabel legacy closed scope as actual completion", () => {
    expect(getProjectIssueMetrics({ issue_count: 10, done_count: 7 })).toMatchObject({
      totalCount: 10, closedCount: 7, completedCount: null, cancelledCount: null,
      openCount: null, closureRatio: 0.7, complete: false,
    });
  });

  it("shows no percentage for an empty formal task set", () => {
    const project = {
      issue_count: 0, done_count: 0, completed_issue_count: 0, cancelled_issue_count: 0,
      open_issue_count: 0, statistics_complete: true,
    };
    expect(getProjectIssueMetrics(project)).toMatchObject({
      totalCount: 0, completedCount: 0, closedCount: 0, closureRatio: null, complete: true,
    });
  });
});
