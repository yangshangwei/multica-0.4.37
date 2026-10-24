import { parseAdminCollaborationCandidates, parseAdminCollaborationDetail, parseAdminCollaborationResult } from "./collaboration-schema";
import type { AdminCollaborationInput } from "./operation-draft";

export const adminCollaborationTestId = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const id = adminCollaborationTestId;
export const adminCollaborationDetailFixture = parseAdminCollaborationDetail({
  scope: id(2), issue_id: id(3), identifier: "SOURCE-42", source_workspace_id: id(9), source_name: "Source workspace",
  status: "done", status_category: "done", collaboration_revision: "7", queue_state: "closed", unsettled_runs: 2,
  creator: { user_id: id(5), name: "Original creator", avatar_url: "", disabled: true },
  handler: { user_id: id(6), name: "Previous handler", avatar_url: "", disabled: true }, successor_required: true,
  actions: [{ action: "reopen", status_options_version: "a".repeat(64), default_status: "todo",
    options: [{ key: "todo", name: "Todo", category: "todo" }, { key: "in_progress", name: "In progress", category: "in_progress" }] }],
})!;
export const adminCollaborationCandidatesFixture = parseAdminCollaborationCandidates({
  scope: id(2), issue_id: id(3), collaboration_revision: "7",
  items: [{ user_id: id(7), name: "First participant", avatar_url: "", external: true }], next_cursor: null,
})!;
export const adminCollaborationInputFixture: AdminCollaborationInput = {
  id: id(3), key: id(4), action: "collaboration", creatorUserId: id(5), handlerUserId: id(6),
  body: { action: "reopen", expectedCollaborationRevision: "7", expectedStatus: "done", targetStatus: "todo",
    statusOptionsVersion: "a".repeat(64), reason: "Restore responsibility", successorUserId: id(7) },
};
export const adminCollaborationResultFixture = parseAdminCollaborationResult({
  operation: { id: id(8), organization_id: id(2), target_id: id(3), actor_id: id(1), target_kind: "collaboration_issue",
    kind: "collaboration.reopen", state: "applied", result_code: "reopened", confirmation: "not_required", reconciliation_state: "complete", version: 1,
    applied_at: "2026-10-21T08:00:00Z" },
  result: { issue_id: id(3), request_id: id(4), action: "reopen", code: "reopened", collaboration_revision: "9", creator_user_id: id(5),
    previous_handler_user_id: id(6), handler_user_id: id(7), previous_status: "done", status: "todo", queue_state: "paused", control_version: "4", unsettled_runs: 2 },
})!;
