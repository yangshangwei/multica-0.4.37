// @vitest-environment node
import { describe, expect, it } from "vitest";
import { deriveCollaborationDetailState, collaborationHasAIActivity } from "./presentation";
import { parseCollaborationIssue, type CollaborationIssue } from "./schema";
import { collaborationExecutionFixture, collaborationIssueFixture } from "./test-fixtures";

const creator = collaborationIssueFixture.creator.user_id;
const other = "00000000-0000-4000-8000-000000000002";
const submission = {
  id: "00000000-0000-4000-8000-000000000003", issue_id: collaborationIssueFixture.id,
  event_seq: "1", lifecycle_epoch: "1", author_user_id: other, author: null, created_at: "2026-11-03T00:00:00Z",
  result: "Delivered", basis: "Validated", attachment_ids: [], source_comment_id: null, evidence_state: "available", decision: null,
};
const blockedCapabilities = { ...collaborationIssueFixture.capabilities, transfer: false, change_status: false,
  request_information: false, return_information: false };
const repairWorkflow = {
  protocol_version: 1, managed: true, context_version: "a".repeat(64), active_request_id: null, record_state: "none",
  actions: { request: { available: false, code: "information_repair_required" },
    return: { available: false, code: "information_request_not_active" }, repair: { available: true, code: null } },
};
function issue(patch: Record<string, unknown> = {}): CollaborationIssue {
  return parseCollaborationIssue({ ...collaborationIssueFixture, status: "in_progress", status_category: "in_progress",
    write_context: { participant_epoch: "1", account_epoch: "1" },
    capabilities: { ...collaborationIssueFixture.capabilities, request_information: true, return_information: true, submit_acceptance: true, return_acceptance: true, change_ai_policy: true }, ...patch })!;
}

describe("human detail stage and next action", () => {
  it("offers an atomic result completion to the initiator doing their own work", () => {
    expect(deriveCollaborationDetailState(issue(), creator)).toMatchObject({ stage: "working", primary: "complete", canSelfComplete: true, canAccept: false });
  });
  it("gives the external handler start then submit without initiator decisions", () => {
    const assigned = { handler: { ...collaborationIssueFixture.handler, user_id: other } };
    expect(deriveCollaborationDetailState(issue({ ...assigned, status: "todo", status_category: "todo" }), other).primary).toBe("change_status");
    expect(deriveCollaborationDetailState(issue(assigned), other)).toMatchObject({ primary: "submit_acceptance", canAccept: false, canSelfComplete: false });
  });
  it("binds acceptance to the exact pending available result and suppresses bypasses", () => {
    const pending = issue({ status: "review", status_category: "in_review", delivery: { current: submission, latest: submission, has_history: true } });
    const state = deriveCollaborationDetailState(pending, creator);
    expect(state).toMatchObject({ stage: "pending_acceptance", primary: "complete", canAccept: true, canSelfComplete: false, currentSubmission: submission });
    expect(state.secondary).toContain("return_acceptance");
    expect(state.secondary).not.toContain("transfer");
    expect(state.secondary).not.toContain("change_status");
    expect(deriveCollaborationDetailState(pending, other)).toMatchObject({ primary: "discuss", canAccept: false });
  });
  it("prioritizes the recorded information return while preserving the pending result", () => {
    const pending = issue({ status: "blocked", status_category: "blocked", delivery: { current: submission, latest: submission, has_history: true },
      current_information_request: { id: submission.id, requester: collaborationIssueFixture.creator,
        recipient: { ...collaborationIssueFixture.handler, user_id: other }, note: "Clarify", return_status: "review", return_status_fallback: false, created_at: submission.created_at } });
    expect(deriveCollaborationDetailState(pending, other)).toMatchObject({ stage: "waiting_information", primary: "return_information", currentSubmission: submission, canAccept: false });
    expect(deriveCollaborationDetailState(pending, creator).secondary).not.toContain("complete");
  });
  it("does not infer delivery or acceptance from legacy review/status", () => {
    const review = issue({ status: "review", status_category: "in_review" });
    expect(deriveCollaborationDetailState(review, creator)).toMatchObject({ stage: "legacy_review", primary: "change_status", canAccept: false, canSelfComplete: false });
    const legacy = { ...submission, evidence_state: "legacy", basis: null, lifecycle_epoch: null };
    expect(deriveCollaborationDetailState(issue({ delivery: { current: legacy, latest: legacy, has_history: true } }), creator)).toMatchObject({ primary: "return_acceptance", canAccept: false });
  });
  it("repairs authoritative review without a submission by explicitly recording a self result", () => {
    const review = issue({ status: "review", status_category: "in_review", delivery: { current: null, latest: null, has_history: false } });
    expect(deriveCollaborationDetailState(review, creator)).toMatchObject({ stage: "legacy_review", primary: "complete", canAccept: false, canSelfComplete: true });
  });
  it("keeps retained result after reopening without making it pending", () => {
    const accepted = { ...submission, decision: { kind: "accepted", submission_id: submission.id, event_seq: "2", actor_user_id: creator, actor: null, created_at: submission.created_at, reason: null, target_user_id: null } };
    const reopened = issue({ lifecycle_epoch: "2", delivery: { current: null, latest: accepted, has_history: true } });
    expect(deriveCollaborationDetailState(reopened, creator)).toMatchObject({ stage: "working", canAccept: false, latestSubmission: accepted });
  });
  it("retains discuss and read when workflow observations are missing", () => {
    expect(deriveCollaborationDetailState(issue({ write_context: null }), creator)).toMatchObject({ primary: "discuss", secondary: [], canSelfComplete: false });
  });
  it("requires information repair for a known managed block without offering self completion or generic changes", () => {
    const blocked = issue({ status: "blocked", status_category: "blocked", capabilities: blockedCapabilities, information_workflow: repairWorkflow });
    const state = deriveCollaborationDetailState(blocked, creator);
    expect(state).toMatchObject({ stage: "waiting_information", primary: "repair_information", canSelfComplete: false });
    for (const action of ["complete", "submit_acceptance", "transfer", "change_status", "request_information"]) expect(state.secondary).not.toContain(action);
    expect(deriveCollaborationDetailState(issue({ status: "blocked", status_category: "blocked" }), creator).canSelfComplete).toBe(true);
  });
  it("offers explicit repair for an actual pending submission while generic capabilities remain closed", () => {
    const pending = issue({ status: "blocked", status_category: "blocked", capabilities: blockedCapabilities,
      information_workflow: repairWorkflow, delivery: { current: submission, latest: submission, has_history: true } });
    const state = deriveCollaborationDetailState(pending, creator);
    expect(state).toMatchObject({ primary: "repair_information", currentSubmission: submission, canAccept: false, canSelfComplete: false });
    for (const action of ["complete", "return_acceptance", "transfer", "change_status"]) expect(state.secondary).not.toContain(action);
  });
  it.each([
    { label: "source-only read without write context", viewer: creator, patch: { write_context: null } },
    { label: "missing editor authority", viewer: creator, patch: { capabilities: { ...blockedCapabilities, edit: false } } },
    { label: "a different current role", viewer: other, patch: {} },
    { label: "current repair refusal", viewer: creator, patch: { information_workflow: { ...repairWorkflow,
      actions: { ...repairWorkflow.actions, repair: { available: false, code: "forbidden" } } } } },
    { label: "unknown older support", viewer: creator, patch: { information_workflow: undefined } },
  ])("does not infer repair authority from one flag with $label", ({ viewer, patch }) => {
    const blocked = issue({ status: "blocked", status_category: "blocked", capabilities: blockedCapabilities,
      information_workflow: repairWorkflow, ...patch });
    const state = deriveCollaborationDetailState(blocked, viewer);
    expect([state.primary, ...state.secondary]).not.toContain("repair_information");
  });
  it("keeps an authorized current request and return independent of generic transfer and status capabilities", () => {
    const blocked = issue({ status: "blocked", status_category: "blocked", handler: { ...collaborationIssueFixture.handler, user_id: other },
      capabilities: { ...blockedCapabilities, request_information: true, return_information: true },
      current_information_request: { id: submission.id, requester: collaborationIssueFixture.creator,
        recipient: { ...collaborationIssueFixture.handler, user_id: other }, note: "Original complete reason", return_status: "in_progress",
        return_status_fallback: false, created_at: submission.created_at,
        block: { protocol_version: 1, state: "complete", unblock_condition: "Verify the original receipt", estimated_resolution_at: "2026-11-18T08:00:00.000001Z" } },
      information_workflow: { ...repairWorkflow, active_request_id: submission.id, record_state: "complete",
        actions: { request: { available: true, code: null }, return: { available: true, code: null }, repair: { available: true, code: null } } } });
    const state = deriveCollaborationDetailState(blocked, other);
    expect(state.primary).toBe("return_information");
    expect(state.secondary).toContain("request_information");
    expect(state.secondary).not.toContain("transfer");
    expect(state.secondary).not.toContain("change_status");
  });
  it("honors known current information denial even when historical metadata is complete", () => {
    const blocked = issue({ status: "blocked", status_category: "blocked", current_information_request: {
      id: submission.id, requester: collaborationIssueFixture.creator, recipient: { ...collaborationIssueFixture.handler, user_id: other },
      note: "Original complete reason", return_status: "in_progress", return_status_fallback: false, created_at: submission.created_at,
      block: { protocol_version: 1, state: "complete", unblock_condition: "Verify the receipt", estimated_resolution_at: "2026-11-18T08:00:00.000001Z" },
    }, information_workflow: { protocol_version: 1, managed: true, context_version: "a".repeat(64), active_request_id: submission.id, record_state: "complete",
      actions: { request: { available: false, code: "forbidden" }, return: { available: false, code: "forbidden" }, repair: { available: false, code: "forbidden" } },
    } });
    const state = deriveCollaborationDetailState(blocked, other);
    expect(state).toMatchObject({ stage: "waiting_information", primary: "discuss" });
    for (const action of ["return_information", "repair_information", "request_information", "transfer", "change_status"]) expect(state.secondary).not.toContain(action);
  });
});

describe("AI console presence", () => {
  const empty = () => issue({ ai_policy: "manual", execution: { ...collaborationExecutionFixture, channels: [], targets: [], commands: [], groups: [], runs: [] } });
  it("omits only the supported empty console while retaining unknown/history/recovery", () => {
    expect(collaborationHasAIActivity(empty())).toBe(false);
    expect(collaborationHasAIActivity(issue())).toBe(true);
    expect(collaborationHasAIActivity(empty(), { pending: [{}] })).toBe(true);
    expect(collaborationHasAIActivity(empty(), { selectedRunId: submission.id })).toBe(true);
    expect(collaborationHasAIActivity(empty(), { error: new Error("Unavailable") })).toBe(true);
    expect(collaborationHasAIActivity(empty(), { history: "unknown" })).toBe(true);
  });
  it("preserves actual AI facts even under manual policy and closed deployment", () => {
    expect(collaborationHasAIActivity(issue({ ai_policy: "manual", execution: { ...collaborationExecutionFixture, execution_gate: { enabled: false, epoch: "1" } } }))).toBe(true);
  });
});
