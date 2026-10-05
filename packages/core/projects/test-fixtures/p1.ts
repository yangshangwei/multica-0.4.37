// Shared wire fixtures for the P1 API contract. No client-generated statistics.
export const P1_WORKSPACE_ID = "11111111-1111-4111-8111-111111111111";
export const P1_PROJECT_ID = "22222222-2222-4222-8222-222222222222";
export const P1_MEMBER_ID = "33333333-3333-4333-8333-333333333333";
export const P1_UPDATE_ID = "44444444-4444-4444-8444-444444444444";
export const P1_REQUEST_ID = "55555555-5555-4555-8555-555555555555";
export const P1_TIMESTAMP = "2026-10-05T12:00:00Z";

export const p1Project = {
  id: P1_PROJECT_ID, workspace_id: P1_WORKSPACE_ID, title: "Launch",
  description: "## Goal\nShip the release", icon: null, status: "in_progress", priority: "medium",
  lead_type: "member", lead_id: P1_MEMBER_ID, start_date: "2026-10-01", due_date: "2026-10-10",
  created_at: P1_TIMESTAMP, updated_at: P1_TIMESTAMP,
  revision: 4, description_revision: 2, issue_count: 10, done_count: 7,
  completed_issue_count: 5, cancelled_issue_count: 2, open_issue_count: 3,
  statistics_complete: true, resource_count: 0,
};
export const p1Member = {
  id: P1_MEMBER_ID, name: "Ada", avatar_url: null, availability: "active",
};
export const p1Statistics = {
  workspace_id: P1_WORKSPACE_ID, project_id: P1_PROJECT_ID, project_revision: 4,
  snapshot_version: "snapshot-1", calculated_at: P1_TIMESTAMP,
  reference_date: "2026-10-05", timezone: "UTC", timezone_configured: false,
  complete: true, incomplete_reasons: [],
  counts: { total: 10, completed: 5, cancelled: 2, open: 3, blocked: 1,
    overdue: 1, unassigned: 1, in_review: 1, risk_union: 2,
    unknown_status: 0, execution_environment_unavailable: 0 },
  closure_ratio: 0.7, project_overdue: false, lead_valid: true,
  latest_update_at: null, progress_age_days: 4, health: "risk", reasons: ["blocked"],
};
export const p1Overview = {
  workspace_id: P1_WORKSPACE_ID, project_id: P1_PROJECT_ID, description_revision: 2,
  statistics: p1Statistics, latest_acceptance: null, current_description_acceptance: null,
};
export const p1Draft = {
  operation: "create", update_id: null, expected_revision: null, kind: "progress",
  body: "Completed the login flow", health_judgment: null, evidence: [], acceptance: null,
  expected_description_revision: null, include_statistics: false, correction_reason: null,
};
export const p1Preview = {
  workspace_id: P1_WORKSPACE_ID, project_id: P1_PROJECT_ID, draft: p1Draft,
  evidence_versions: [], description_revision: null, recipients: [], statistics_snapshot: null,
  preview_hash: "preview-1", previewed_at: P1_TIMESTAMP,
};
export const p1Revision = {
  workspace_id: P1_WORKSPACE_ID, project_id: P1_PROJECT_ID, update_id: P1_UPDATE_ID,
  revision: 1, editor: p1Member, created_at: P1_TIMESTAMP, kind: "progress", body: p1Draft.body,
  health_judgment: null, correction_reason: null, evidence: [], statistics_snapshot: null, acceptance: null,
};
export const p1WriteResult = {
  workspace_id: P1_WORKSPACE_ID, project_id: P1_PROJECT_ID, request_id: P1_REQUEST_ID,
  update_id: P1_UPDATE_ID, result_revision: 1, replayed: false, result: p1Revision,
  author: p1Member, published_at: P1_TIMESTAMP,
};
