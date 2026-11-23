import type { CollaborationKnowledgeCandidate, CollaborationKnowledgeCandidates, CollaborationKnowledgeDetail,
  CollaborationKnowledgeProposal, CollaborationKnowledgeReceipt, CollaborationKnowledgeTargetList } from "./knowledge-schema";

export const knowledgeIds = {
  actor: "00000000-0000-4000-8000-000000000001", issue: "00000000-0000-4000-8000-000000000002",
  candidate: "00000000-0000-4000-8000-000000000003", request: "00000000-0000-4000-8000-000000000004",
  workspace: "00000000-0000-4000-8000-000000000005", project: "00000000-0000-4000-8000-000000000006",
};
export const knowledgeLeaseFixture = { protocol_version: 1 as const, projection_checked_at: "2026-11-01T00:00:00Z",
  valid_until: "2026-11-01T00:00:30Z", next_expiry_at: null };
export const knowledgeProposalFixture: CollaborationKnowledgeProposal = {
  protocol_version: 1, request_id: knowledgeIds.request, candidate_id: knowledgeIds.candidate,
  target: { kind: "workspace", workspace_id: knowledgeIds.workspace }, summary: "PRIVATE SUMMARY", content: "PRIVATE BODY",
  sources: [{ kind: "memory", issue_id: knowledgeIds.issue, version: "7", sha256: "a".repeat(64) }], evidence: [],
  applicability: { surfaces: ["issue", "chat"] }, expires_at: null, supersedes_candidate_id: null, derived_from_candidate_id: null,
};
export const knowledgeCandidateFixture: Extract<CollaborationKnowledgeCandidate, { availability: "available" }> = {
  candidate_id: knowledgeIds.candidate, lineage_id: knowledgeIds.candidate, version: "9007199254740993", state: "pending", state_revision: "1",
  availability: "available", source_workspace_id: knowledgeIds.workspace, source_issue_id: knowledgeIds.issue,
  target: knowledgeProposalFixture.target, proposed_by_user_id: knowledgeIds.actor, proposed_at: "2026-10-31T23:00:00Z",
  expires_at: null, supersedes_candidate_id: null, derived_from_candidate_id: null, approved_at: null, approved_by_user_id: null,
  published_target_revision: null, decision: null, capabilities: { can_approve: true, can_reject: true, can_withdraw: true },
  summary: knowledgeProposalFixture.summary, content: knowledgeProposalFixture.content, content_sha256: "b".repeat(64),
  sources: knowledgeProposalFixture.sources, evidence: [], applicability: knowledgeProposalFixture.applicability,
};
const { content: _content, ...summary } = knowledgeCandidateFixture;
const { content: _body, summary: _summary, content_sha256: _hash, sources: _sources, evidence: _evidence, applicability: _applicability, ...audit } = knowledgeCandidateFixture;
export const knowledgeUnavailableCandidateFixture: Extract<CollaborationKnowledgeCandidate, { availability: "source_unavailable" }> = {
  ...audit, availability: "source_unavailable", decision: null, capabilities: { can_approve: false, can_reject: true, can_withdraw: true },
};
export const knowledgeCandidatesFixture: CollaborationKnowledgeCandidates = { ...knowledgeLeaseFixture, issue_id: knowledgeIds.issue,
  items: [summary], next_cursor: null, capabilities: { can_propose: true } };
export const knowledgeDetailFixture: CollaborationKnowledgeDetail = { ...knowledgeLeaseFixture, candidate: knowledgeCandidateFixture, history: [], history_truncated: false };
export const knowledgeTargetFixture: CollaborationKnowledgeTargetList = { ...knowledgeLeaseFixture, target: knowledgeProposalFixture.target,
  knowledge_revision: "9007199254740993", items: [summary], next_cursor: null };
export const knowledgeReceiptFixture: CollaborationKnowledgeReceipt = { protocol_version: 1, request_id: knowledgeIds.request, action: "approve",
  code: "knowledge_approved", candidate_id: knowledgeIds.candidate, version: "9007199254740993", state_revision: "2", state: "approved", target_knowledge_revision: "9007199254740993" };
