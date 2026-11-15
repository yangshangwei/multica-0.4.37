// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  CollaborationKnowledgeProposalSchema,
  CollaborationKnowledgeProtocolError,
  parseCollaborationKnowledgeCandidates,
  parseCollaborationKnowledgeDetail,
  parseCollaborationKnowledgeReceipt,
  parseCollaborationKnowledgeTargetList,
} from "./knowledge-schema";
import {
  knowledgeCandidateFixture,
  knowledgeCandidatesFixture,
  knowledgeDetailFixture,
  knowledgeIds,
  knowledgeProposalFixture,
  knowledgeReceiptFixture,
  knowledgeTargetFixture,
} from "./knowledge-test-fixtures";

// v1 compatibility and exhaustive existing material budgets stay in knowledge-schema.test.ts.
const deliverySource = {
  kind: "delivery",
  issue_id: knowledgeIds.issue,
  submission_id: "00000000-0000-4000-8000-000000000007",
  delivery_event_seq: "9007199254740993",
  accepted_event_seq: "9007199254740994",
  sha256: "a".repeat(64),
};
const automationCandidate = {
  ...knowledgeCandidateFixture,
  version: "1",
  target: { kind: "project", workspace_id: knowledgeIds.workspace, project_id: knowledgeIds.project },
  state: "approved",
  state_revision: "2",
  proposal_origin: "automation",
  decision_origin: "automation",
  proposed_by_user_id: null,
  approved_by_user_id: null,
  approved_at: "2026-11-01T00:00:00Z",
  published_target_revision: "9007199254740995",
  decision: { actor_user_id: null, at: "2026-11-01T00:00:00Z", reason: null },
  automation: {
    job_id: "00000000-0000-4000-8000-000000000008",
    policy_revision: "0",
    rule_version: "project-knowledge-v1",
  },
  usage_status: "active",
  sources: [deliverySource],
  capabilities: { can_approve: false, can_reject: false, can_withdraw: true },
};
const { content: _content, ...automationSummary } = automationCandidate;
const automationDetail = { ...knowledgeDetailFixture, protocol_version: 2, candidate: automationCandidate };
const automationCandidates = { ...knowledgeCandidatesFixture, protocol_version: 2, items: [automationSummary] };
const automationTarget = {
  ...knowledgeTargetFixture,
  protocol_version: 2,
  target: automationCandidate.target,
  knowledge_revision: automationCandidate.published_target_revision,
  items: [automationSummary],
};

describe("knowledge automation v2 protocol", () => {
  it("reads a system proposal and decision without inventing a human actor", () => {
    expect(parseCollaborationKnowledgeDetail(automationDetail)?.candidate).toEqual(automationCandidate);
  });

  it("preserves exact accepted-delivery identity in source and target summaries", () => {
    expect(parseCollaborationKnowledgeCandidates(automationCandidates)?.items).toEqual([automationSummary]);
    expect(parseCollaborationKnowledgeTargetList(automationTarget)?.items).toEqual([automationSummary]);
  });

  it("keeps approved history separate from a reopen pause", () => {
    const candidate = { ...automationCandidate, usage_status: "paused_review" };
    expect(parseCollaborationKnowledgeDetail({ ...automationDetail, candidate })?.candidate).toMatchObject({
      state: "approved",
      state_revision: "2",
      usage_status: "paused_review",
      approved_at: automationCandidate.approved_at,
      decision_origin: "automation",
    });
  });

  it("preserves the original automatic approval after a later human withdrawal", () => {
    const candidate = { ...automationCandidate, state: "withdrawn", state_revision: "3", usage_status: "inactive",
      decision_origin: "human", decision: { actor_user_id: knowledgeIds.actor, at: "2026-11-01T00:00:01Z", reason: "No longer applicable" } };
    expect(parseCollaborationKnowledgeDetail({ ...automationDetail, candidate })?.candidate).toEqual(candidate);
  });

  it("reads human v2 without treating a null pending decision as an automatic decision", () => {
    const candidate = { ...knowledgeCandidateFixture, proposal_origin: "human", decision_origin: "human", automation: null,
      usage_status: "inactive" };
    expect(parseCollaborationKnowledgeDetail({ ...automationDetail, candidate })?.candidate).toEqual(candidate);
  });

  it("keeps unavailable v2 audit metadata while rejecting source material and private reasons", () => {
    const { content: _body, summary: _summary, content_sha256: _hash, sources: _sources, evidence: _evidence,
      applicability: _applicability, ...audit } = automationCandidate;
    const candidate = { ...audit, availability: "source_unavailable",
      decision: { actor_user_id: null, at: automationCandidate.approved_at } };
    expect(parseCollaborationKnowledgeDetail({ ...automationDetail, candidate })?.candidate).toEqual(candidate);
    for (const change of [{ content: "PRIVATE" }, { sources: [deliverySource] },
      { decision: { ...candidate.decision, reason: "PRIVATE COMPARISON TEXT" } }]) {
      expect(() => parseCollaborationKnowledgeDetail({ ...automationDetail, candidate: { ...candidate, ...change } }))
        .toThrow(CollaborationKnowledgeProtocolError);
    }
  });

  it("keeps v2 restricted rows content-free without inventing a visible source or actor", () => {
    const candidate = { candidate_id: knowledgeIds.candidate, state: "approved", state_revision: "2",
      availability: "restricted", capabilities: { can_withdraw: true } };
    expect(parseCollaborationKnowledgeDetail({ ...automationDetail, candidate })?.candidate).toEqual(candidate);
    expect(() => parseCollaborationKnowledgeDetail({ ...automationDetail,
      candidate: { ...candidate, automation: automationCandidate.automation } })).toThrow(CollaborationKnowledgeProtocolError);
  });

  it("retains each operation receipt's declared version without upgrading v1", () => {
    expect(parseCollaborationKnowledgeReceipt(knowledgeReceiptFixture)).toEqual(knowledgeReceiptFixture);
    const receipt = { ...knowledgeReceiptFixture, protocol_version: 2 };
    expect(parseCollaborationKnowledgeReceipt(receipt)).toEqual(receipt);
    expect(() => parseCollaborationKnowledgeReceipt({ ...receipt, protocol_version: 3 })).toThrow(CollaborationKnowledgeProtocolError);
  });

  it("allows a human v2 proposal to cite an exact delivery without granting system identity", () => {
    const proposal = { ...knowledgeProposalFixture, protocol_version: 2, sources: [deliverySource] };
    expect(CollaborationKnowledgeProposalSchema.parse(proposal)).toEqual(proposal);
    expect(CollaborationKnowledgeProposalSchema.safeParse({ ...proposal,
      proposal_origin: "automation", automation: automationCandidate.automation }).success).toBe(false);
  });

  it("rejects new identities or delivery sources falsely labelled as v1", () => {
    expect(() => parseCollaborationKnowledgeDetail({ ...automationDetail, protocol_version: 1 }))
      .toThrow(CollaborationKnowledgeProtocolError);
    expect(() => parseCollaborationKnowledgeCandidates({ ...automationCandidates, protocol_version: 1 }))
      .toThrow(CollaborationKnowledgeProtocolError);
    expect(() => parseCollaborationKnowledgeTargetList({ ...automationTarget, protocol_version: 1 }))
      .toThrow(CollaborationKnowledgeProtocolError);
    expect(CollaborationKnowledgeProposalSchema.safeParse({ ...knowledgeProposalFixture, sources: [deliverySource] }).success).toBe(false);
  });

  it.each([
    ["a fabricated proposal actor", { proposed_by_user_id: knowledgeIds.actor }],
    ["a fabricated approval actor", { approved_by_user_id: knowledgeIds.actor }],
    ["a missing rule identity", { automation: { job_id: automationCandidate.automation.job_id, policy_revision: "0" } }],
    ["a human decision without its actor", { decision_origin: "human" }],
    ["a pending candidate marked active", { state: "pending" }],
    ["a numeric accepted sequence", { sources: [{ ...deliverySource, accepted_event_seq: 9007199254740994 }] }],
    ["an absent accepted event", { sources: [{ ...deliverySource, accepted_event_seq: undefined }] }],
    ["a source from another issue", { sources: [{ ...deliverySource, issue_id: knowledgeIds.actor }] }],
    ["an unknown usage state", { usage_status: "probably_active" }],
    ["an undeclared material field", { unchecked_model_reason: "PRIVATE COMPARISON TEXT" }],
  ])("rejects malformed v2 with %s", (_label, change) => {
    expect(() => parseCollaborationKnowledgeDetail({ ...automationDetail,
      candidate: { ...automationCandidate, ...change } })).toThrow(CollaborationKnowledgeProtocolError);
  });
});
