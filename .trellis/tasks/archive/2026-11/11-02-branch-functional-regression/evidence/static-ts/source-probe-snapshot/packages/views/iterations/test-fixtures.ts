import type { api } from "@multica/core/api";
import type { Issue } from "@multica/core/types";
import type {
  Iteration,
  IterationDraft,
  IterationPreview,
} from "@multica/core/iterations";
export const ws = "10000000-0000-4000-8000-000000000001";
export const source: Iteration = {
  id: "20000000-0000-4000-8000-000000000001",
  workspace_id: ws,
  name: "Source iteration",
  description: null,
  coordinator_user_id: null,
  status: "active",
  mode: "manual",
  start_date: "2026-10-01",
  end_date: "2026-10-14",
  timezone: "UTC",
  revision: 2,
  scope_revision: 3,
  started_at: "2026-10-01T00:00:00Z",
  logical_ended_at: null,
  processed_at: null,
};
export const targetA: Iteration = {
  ...source,
  id: "20000000-0000-4000-8000-000000000002",
  name: "Next A",
  status: "planned",
  started_at: null,
};
export const targetB: Iteration = {
  ...targetA,
  id: "20000000-0000-4000-8000-000000000003",
  name: "Next B",
};
export const statistics = {
  original: 2,
  current: 2,
  cancelled: 0,
  effective: 2,
  completed: 0,
  original_completed: 0,
  remaining: 2,
  added_unique: 0,
  removed_events: 0,
  reentry_events: 0,
  cancel_events: 0,
  reopen_events: 0,
  started: 2,
  initial_effective: 2,
  net_effective_change: 0,
  net_effective_change_ratio: 0,
  effective_ratio: 0,
  original_ratio: 0,
  chart: [],
  calculated_at: "2026-10-06T00:00:00Z",
};
export const settings = {
  workspace_id: ws,
  enabled: true,
  revision: 1,
  planning_timezone: "UTC",
  effective_timezone: "UTC",
  timezone_configured: true,
};
export const alpha: Issue = {
  id: "30000000-0000-4000-8000-000000000001",
  workspace_id: ws,
  number: 1,
  identifier: "ITR-1",
  title: "Alpha task",
  description: null,
  status: "todo",
  priority: "none",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: ws,
  parent_issue_id: null,
  project_id: null,
  position: 0,
  stage: null,
  start_date: null,
  due_date: null,
  metadata: {},
  properties: {},
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
  revision: 4,
  current_iteration_id: source.id,
};
export const beta: Issue = {
  ...alpha,
  id: "30000000-0000-4000-8000-000000000002",
  number: 2,
  identifier: "ITR-2",
  title: "Beta task",
};
export function issuePage(
  issues: Issue[],
  next: string | null = null,
): Awaited<ReturnType<typeof api.getIterationIssues>> {
  return {
    workspace_id: ws,
    iteration_id: source.id,
    scope_revision: source.scope_revision,
    next_cursor: next,
    total: issues.length,
    items: issues.map((issue) => ({
      issue_id: issue.id,
      identifier: issue.identifier,
      title: issue.title,
      project_id: null,
      project_name: null,
      assignee_type: null,
      assignee_id: null,
      assignee_name: null,
      status_key: issue.status,
      status_category: issue.status,
      was_completed_at_start: false,
      rollover_count: 0,
    })),
  };
}
export function previewFor(
  draft: IterationDraft,
  issues = [alpha, beta],
): IterationPreview {
  return {
    workspace_id: ws,
    actor_user_id: ws,
    draft,
    preview_hash: "hash",
    previewed_at: "2026-10-06T00:00:00Z",
    start_preview: draft.start
      ? {
          reference_date: "2026-10-06",
          effective_start_date: "2026-10-01",
          effective_end_date: "2026-10-14",
          timezone: "UTC",
        }
      : null,
    iterations: [source, targetA, targetB],
    statistics: { [source.id]: statistics },
    recipients: [],
    invalid_items: [],
    total_affected: issues.length,
    complete: true,
    issues: issues.map((issue) => ({
      issue_id: issue.id,
      identifier: issue.identifier,
      revision: issue.revision!,
      source_id: source.id,
      status_category: issue.status,
      title: issue.title,
      running_execution_count: 0,
      rollover_count: 0,
      project: { id: null, name: null, type: null, available: false },
      assignee: { id: null, name: null, type: null, available: false },
    })),
  };
}
export const receipt = {
  workspace_id: ws,
  request_id: ws,
  operation_id: ws,
  operation: "end",
  replayed: false,
  iteration_ids: [source.id],
  result: {
    snapshot_id: ws,
    deleted: false,
    settings_revision: 1,
    issue_count: 2,
  },
  committed_at: "2026-10-06T00:00:00Z",
};
