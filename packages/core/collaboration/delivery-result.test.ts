// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseCollaborationIssue, parseCollaborationOperation, parseCollaborationStatusOptions } from "./schema";
import { CollaborationActionSchema } from "./action";
import { newCollaborationDeliveryForm } from "./delivery-draft";
import { prepareCollaborationCreation } from "./creation";
import { collaborationIssueFixture } from "./test-fixtures";
import { CollaborationSubmissionSchema, parseCollaborationDeliveryHistory } from "./delivery-schema";

const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const submission = {
  id: id(2), issue_id: collaborationIssueFixture.id, event_seq: "9007199254740993", lifecycle_epoch: "1",
  author_user_id: id(3), author: null, created_at: "2026-11-03T00:00:00Z",
  result: "Original result", basis: "Checked against the request", attachment_ids: [],
  source_comment_id: null, evidence_state: "available", decision: null,
};
const issue = { ...collaborationIssueFixture, ai_policy: "manual", delivery: { current: submission, latest: submission, has_history: true } };
const base = { requestId: id(4), expectedCollaborationRevision: "1", expectedLifecycleEpoch: "1", expectedParticipantEpoch: "1", expectedAccountEpoch: "1" };
const directed = { expectedStatus: "review", statusOptionsVersion: "a".repeat(64) };

describe("formal delivery contract", () => {
  it("preserves manual policy and exact immutable text-only result facts", () => {
    expect(parseCollaborationIssue(issue)).toMatchObject({ ai_policy: "manual", delivery: issue.delivery });
  });

  it.each([
    { ai_policy: "made_up" },
    { delivery: { ...issue.delivery, current: { ...submission, issue_id: id(5) } } },
    { delivery: { ...issue.delivery, current: { ...submission, basis: null } } },
    { delivery: { ...issue.delivery, current: { ...submission, attachment_ids: [id(5), id(5)] } } },
    { delivery: { ...issue.delivery, current: { ...submission, lifecycle_epoch: "2" } } },
  ])("rejects malformed present delivery/policy instead of silently dropping it", (patch) => {
    const delivery = "delivery" in patch && patch.delivery !== undefined ? { ...patch.delivery, latest: patch.delivery.current } : issue.delivery;
    expect(parseCollaborationIssue({ ...issue, ...patch, delivery })).toBeNull();
  });

  it("keeps absent old-server evidence unknown", () => {
    const old = parseCollaborationIssue(collaborationIssueFixture);
    expect(old).not.toBeNull();
    expect(old).not.toHaveProperty("ai_policy");
    expect(old).not.toHaveProperty("delivery");
  });

  it("requires result and basis, preserves the prose and original result draft identity", () => {
    const action = { ...base, ...directed, action: "submit_acceptance", note: " Result ", basis: " Basis ", deliveryDraftId: id(6), sourceCommentId: id(7) };
    expect(CollaborationActionSchema.parse(action)).toEqual(action);
    expect(CollaborationActionSchema.safeParse({ ...action, basis: " " }).success).toBe(false);
    expect(CollaborationActionSchema.safeParse({ ...action, note: " " }).success).toBe(false);
    expect(CollaborationActionSchema.safeParse({ ...action, basis: "界".repeat(21846) }).success).toBe(false);
  });

  it("accepts exact revision and policy intents", () => {
    expect(CollaborationActionSchema.safeParse({ ...base, ...directed, action: "return_acceptance", submissionId: id(2), targetUserId: id(3), note: "Please amend" }).success).toBe(true);
    expect(CollaborationActionSchema.safeParse({ ...base, action: "change_ai_policy", aiPolicy: "manual" }).success).toBe(true);
  });

  it("retains exact content-free delivery confirmation and rejects duplicate files", () => {
    const receipt = { code: "acceptance_submitted", collaboration_revision: "2", delivery_confirmation: { submission_id: id(2), attachment_ids: [id(8)] } };
    expect(parseCollaborationOperation(receipt)).toEqual(receipt);
    expect(parseCollaborationOperation({ ...receipt, delivery_confirmation: { ...receipt.delivery_confirmation, attachment_ids: [id(8), id(8)] } })).toBeNull();
  });

  it("validates immutable author and exact decision linkage while retaining explicit legacy evidence", () => {
    const accepted = { kind: "accepted", submission_id: submission.id, event_seq: "9007199254740994", actor_user_id: id(1), actor: null, created_at: submission.created_at, reason: null, target_user_id: null };
    expect(CollaborationSubmissionSchema.safeParse({ ...submission, decision: accepted }).success).toBe(true);
    expect(CollaborationSubmissionSchema.safeParse({ ...submission, decision: { ...accepted, submission_id: id(9) } }).success).toBe(false);
    expect(CollaborationSubmissionSchema.safeParse({ ...submission, decision: { ...accepted, event_seq: submission.event_seq } }).success).toBe(false);
    expect(CollaborationSubmissionSchema.safeParse({ ...submission, author: { ...collaborationIssueFixture.creator } }).success).toBe(false);
    expect(CollaborationSubmissionSchema.safeParse({ ...submission, decision: { ...accepted, kind: "returned", target_user_id: id(3), reason: " " } }).success).toBe(false);
    expect(CollaborationSubmissionSchema.safeParse({ ...submission, evidence_state: "legacy", lifecycle_epoch: null, basis: null, decision: null }).success).toBe(true);
  });

  it("keeps bounded descending history within one exact issue", () => {
    const older = { ...submission, id: id(9), event_seq: "9007199254740992" };
    const page = { issue_id: submission.issue_id, items: [submission, older], next_cursor: "next" };
    expect(parseCollaborationDeliveryHistory(page)).toEqual(page);
    expect(parseCollaborationDeliveryHistory({ ...page, items: [older, submission] })).toBeNull();
    expect(parseCollaborationDeliveryHistory({ ...page, items: [submission, submission] })).toBeNull();
    expect(parseCollaborationDeliveryHistory({ ...page, items: [{ ...submission, issue_id: id(8) }] })).toBeNull();
    expect(parseCollaborationDeliveryHistory({ ...page, items: [] })).toBeNull();
    expect(parseCollaborationDeliveryHistory({ ...page, next_cursor: "" })).toBeNull();
  });

  it("excludes generic review and adds one directed revision status", () => {
    const catalog = { status_options_version: directed.statusOptionsVersion, default_status: "x", options: [{ key: "x", name: "X", category: "in_review" }] };
    expect(parseCollaborationStatusOptions({ ...catalog, action: "change_status" })).toBeNull();
    expect(parseCollaborationStatusOptions({ ...catalog, action: "reopen" })).toBeNull();
    expect(parseCollaborationStatusOptions({ ...catalog, action: "return_acceptance", options: [{ ...catalog.options[0], category: "in_progress" }] })).not.toBeNull();
  });

  it("stores a distinct result form and persists creation policy", () => {
    const context = { kind: "issue" as const, issueId: id(1), sourceWorkspaceId: id(9) };
    const scope = { server: "https://example.test", userId: id(3), context };
    expect(() => newCollaborationDeliveryForm(scope, "result")).not.toThrow();
    const creation = newCollaborationDeliveryForm({ ...scope, context: { kind: "creation", sourceWorkspaceId: id(9) } }, "creation");
    expect(prepareCollaborationCreation({ ...creation, title: "Work", aiPolicy: "manual" }, id(4)).input).toMatchObject({ ai_policy: "manual" });
  });
});
