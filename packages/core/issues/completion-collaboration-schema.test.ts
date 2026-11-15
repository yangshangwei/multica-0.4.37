// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { CompletionAuthorizationRequestSchema, CompletionReceiptSchema, matchesCompletionAuthorization } from "./completion-schema";
import { CompletionDraftSchema } from "./completion-draft";
import { submitIssueCompletion } from "./completion-queries";
import { completionAuthorizationFixture, completionCollaborationRequestFixture, completionIds, completionPreviewFixture, completionReceiptFixture } from "./completion-test-fixtures";

afterEach(() => vi.unstubAllGlobals());
describe("M1 collaboration target contract", () => {
  it.each(["collaboration_submission", "collaboration_self", "collaboration_admin"] as const)("parses %s without treating its ordinary child artifact as the target", (kind) => {
    const request = completionCollaborationRequestFixture(kind);
    expect(CompletionAuthorizationRequestSchema.parse(request)).toEqual(request);
    expect(CompletionAuthorizationRequestSchema.safeParse({ ...request, evidence: { ...request.evidence, issue_id: request.target.issue_id } }).success).toBe(false);
  });
  it("preserves ordinary target/evidence equality and retains collaboration authorization in the common M1 draft", () => {
    const binding = completionPreviewFixture().authorization_binding;
    const ordinary = { ...binding, operation_id: completionIds.operation, reason: "Exact review", expires_at: "2099-01-01T00:15:00Z" };
    expect(CompletionAuthorizationRequestSchema.parse(ordinary)).toEqual(ordinary);
    expect(CompletionAuthorizationRequestSchema.safeParse({ ...ordinary, evidence: { ...ordinary.evidence, issue_id: completionIds.approval } }).success).toBe(false);
    expect(CompletionDraftSchema.safeParse({ version: 1, scope: { server: "https://api.example", userId: completionIds.user, workspaceId: completionIds.workspace, issueId: completionIds.issue },
      requestKey: completionIds.key, request: completionCollaborationRequestFixture("collaboration_submission"), createdAt: 1, creationDispatched: true,
      authorizationId: null, submissionDispatched: false, revocation: null }).success).toBe(true);
  });
  it("refuses to send a retained collaboration grant through the ordinary issue writer", async () => {
    const scope = { server: "https://api.example", apiScope: "test", userId: completionIds.user, workspaceId: completionIds.workspace, issueId: completionIds.issue };
    const draft = CompletionDraftSchema.parse({ version: 1, scope: { server: scope.server, userId: scope.userId, workspaceId: scope.workspaceId, issueId: scope.issueId },
      requestKey: completionIds.key, request: completionCollaborationRequestFixture("collaboration_submission"), createdAt: 1,
      creationDispatched: true, authorizationId: completionIds.authorization, submissionDispatched: false, revocation: null });
    const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
    await expect(submitIssueCompletion(scope, draft)).rejects.toThrow("original workflow writer");
    expect(fetch).not.toHaveBeenCalled();
  });
  it.each([
    ["numeric collaboration version", { expected_collaboration_revision: 9007199254740992 }],
    ["zero lifecycle", { expected_lifecycle_epoch: "0" }],
    ["missing participant", { expected_participant_epoch: undefined }],
    ["unknown target field", { target_position: null }],
    ["missing exact submission", { submission_id: null }],
  ])("rejects %s", (_label, change) => {
    const request = completionCollaborationRequestFixture("collaboration_submission");
    expect(CompletionAuthorizationRequestSchema.safeParse({ ...request, target: { ...request.target, ...change } }).success).toBe(false);
  });
  it("retains explicit nullable self fields and rejects mixed submission/self/admin variants", () => {
    const self = completionCollaborationRequestFixture("collaboration_self");
    for (const target of [{ ...self.target, draft_id: undefined }, { ...self.target, source_comment_id: undefined }, { ...self.target, result: " " },
      { ...self.target, submission_id: completionIds.authorization }, { ...self.target, attachment_ids: [completionIds.approval, completionIds.approval] }]) {
      expect(CompletionAuthorizationRequestSchema.safeParse({ ...self, target }).success).toBe(false);
    }
    const admin = completionCollaborationRequestFixture("collaboration_admin");
    expect(CompletionAuthorizationRequestSchema.safeParse({ ...admin, consumer: { kind: "human", user_id: completionIds.user } }).success).toBe(false);
    expect(CompletionAuthorizationRequestSchema.safeParse({ ...self, consumer: admin.consumer }).success).toBe(false);
    expect(CompletionAuthorizationRequestSchema.safeParse({ ...admin, target: { ...admin.target, expected_account_epoch: "1" } }).success).toBe(false);
  });
  it.each(["collaboration_submission", "collaboration_self", "collaboration_admin"] as const)("uses the existing authorization endpoint for %s and preserves all bindings", async (kind) => {
    const request = completionCollaborationRequestFixture(kind);
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(completionAuthorizationFixture()))); vi.stubGlobal("fetch", fetch);
    await new ApiClient("https://api.example.test").createDeliveryCompletionAuthorization(completionIds.key, CompletionAuthorizationRequestSchema.parse(request), { workspaceId: completionIds.workspace });
    const [url, init] = fetch.mock.calls[0]!;
    expect(url).toBe("https://api.example.test/api/delivery-completion-authorizations");
    expect(JSON.parse(init.body)).toEqual(request);
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe(completionIds.key);
  });
});

describe("M1 exact collaboration terminal receipts", () => {
  it.each(["collaboration_submission", "collaboration_self", "collaboration_admin"] as const)("correlates %s through its original request", (kind) => {
    const request = CompletionAuthorizationRequestSchema.parse(completionCollaborationRequestFixture(kind));
    const receipt = { ...completionReceiptFixture(), collaboration_revision: "9007199254740994", ...(kind !== "collaboration_admin" ? { submission_id: completionIds.authorization } : {}) };
    const grant = { ...completionAuthorizationFixture(), state: "consumed" as const, applicability: "consumed" as const, completion_receipt: CompletionReceiptSchema.parse(receipt) };
    expect(grant.completion_receipt).toEqual(receipt);
    expect(matchesCompletionAuthorization(grant, request)).toBe(true);
    expect(matchesCompletionAuthorization({ ...grant, completion_receipt: { ...grant.completion_receipt, collaboration_revision: "9007199254740995" } }, request)).toBe(false);
    if (kind === "collaboration_submission") expect(matchesCompletionAuthorization({ ...grant, completion_receipt: { ...grant.completion_receipt, submission_id: completionIds.approval } }, request)).toBe(false);
    if (kind === "collaboration_admin") expect(matchesCompletionAuthorization({ ...grant, completion_receipt: { ...grant.completion_receipt, submission_id: completionIds.approval } }, request)).toBe(false);
  });
});
