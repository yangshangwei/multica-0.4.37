import type { Issue } from "../types";
import type { CompletionPreview } from "./completion-schema";

export const completionIds = {
  issue: "11111111-1111-4111-8111-111111111111",
  workspace: "22222222-2222-4222-8222-222222222222",
  user: "33333333-3333-4333-8333-333333333333",
  project: "44444444-4444-4444-8444-444444444444",
  approval: "55555555-5555-4555-8555-555555555555",
  authorization: "66666666-6666-4666-8666-666666666666",
  operation: "77777777-7777-4777-8777-777777777777",
  key: "88888888-8888-4888-8888-888888888888",
};

export function completionPreviewFixture() {
  const target = { kind: "ordinary" as const, issue_id: completionIds.issue, expected_issue_revision: 7, expected_status: "in_review", target_status: "done", target_position: null };
  return {
    protocol_version: 1 as const, issue_id: completionIds.issue, issue_revision: 7,
    current_status: "in_review", current_category: "in_review", target_status: "done", target_category: "done",
    project_id: completionIds.project, project_revision: 3, policy_mode: "enforce" as const, policy_revision: 2,
    checked_at: "2099-01-01T00:00:00Z", reason: "authorization_available",
    observation: {
      mode: "observe", policy_version: "delivery-artifact-observe-v1", verdict: "pass" as const, reason: "artifact_matches",
      issue_id: completionIds.issue, issue_revision: 7, project_id: completionIds.project, project_revision: 3,
      approval_id: completionIds.approval, execution_id: completionIds.operation, source_revision: "a".repeat(40), observed_head_sha: "a".repeat(40),
      revision_provenance: "database_observed_pr_head_not_remote_verified", spec_digest: "b".repeat(64), artifact_digest: "c".repeat(64), receipt_digest: "d".repeat(64),
      producer_task_generation: 0, observed_at: "2099-01-01T00:00:00Z", valid_until: "2099-01-01T01:00:00Z",
    },
    authorization_binding: {
      contract_version: "delivery-completion-enforcement-v1" as const, source_workspace_id: completionIds.workspace,
      target, project: { id: completionIds.project, expected_revision: 3, expected_policy_revision: 2 },
      evidence: { issue_id: completionIds.issue, expected_issue_revision: 7, approval_id: completionIds.approval, expected_source_revision: "a".repeat(40), expected_spec_digest: "b".repeat(64), expected_artifact_digest: "c".repeat(64) },
      consumer: { kind: "human" as const, user_id: completionIds.user },
    },
    max_expires_at: "2099-01-01T01:00:00Z", enforced: false as const, usable_for_completion: false as const, skip_supported: false as const,
  } satisfies CompletionPreview;
}

export function completionAuthorizationFixture(operationId = completionIds.operation) {
  return {
    authorization_id: completionIds.authorization, operation_id: operationId, issue_id: completionIds.issue,
    issue_revision: 7, target_status: "done", contract_version: "delivery-completion-enforcement-v1" as const,
    created_at: "2099-01-01T00:00:00Z", expires_at: "2099-01-01T00:15:00Z", state: "issued" as const, applicability: "not_checked" as const, replayed: false,
  };
}

export function completionReceiptFixture(operationId = completionIds.operation) {
  return { authorization_id: completionIds.authorization, operation_id: operationId, issue_id: completionIds.issue, completed_revision: 8, target_status: "done", completed_at: "2099-01-01T00:01:00Z", replayed: false };
}

export function completionIssueFixture(overrides: Partial<Issue> = {}): Issue {
  return {
    id: completionIds.issue, workspace_id: completionIds.workspace, number: 1, identifier: "M1-1", title: "Delivery", description: null,
    status: "in_review", status_category: "in_review", priority: "none", assignee_type: null, assignee_id: null, creator_type: "member", creator_id: completionIds.user,
    parent_issue_id: null, project_id: completionIds.project, position: 1, stage: null, start_date: null, due_date: null, metadata: {}, properties: {},
    created_at: "2099-01-01T00:00:00Z", updated_at: "2099-01-01T00:00:00Z", revision: 7, ...overrides,
  };
}
