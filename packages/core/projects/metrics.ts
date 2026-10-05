import type { Project } from "../types/project";

export function getProjectIssueMetrics(project: Pick<Project, "issue_count" | "done_count" |
  "completed_issue_count" | "cancelled_issue_count" | "open_issue_count" | "statistics_complete">) {
  const complete = project.statistics_complete === true &&
    project.completed_issue_count !== undefined && project.cancelled_issue_count !== undefined && project.open_issue_count !== undefined;
  return {
    totalCount: project.issue_count,
    closedCount: project.done_count,
    completedCount: complete ? project.completed_issue_count! : null,
    cancelledCount: complete ? project.cancelled_issue_count! : null,
    openCount: complete ? project.open_issue_count! : null,
    closureRatio: project.issue_count > 0 ? project.done_count / project.issue_count : null,
    complete,
  };
}
