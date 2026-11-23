// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { CollaborationKnowledgeApproveRequestSchema, CollaborationKnowledgeDecisionRequestSchema, CollaborationKnowledgeHintSchema,
  CollaborationKnowledgeProposalSchema, CollaborationKnowledgeProtocolError, KNOWLEDGE_CONTENT_MAX_BYTES, KNOWLEDGE_REASON_MAX_BYTES,
  KNOWLEDGE_REFERENCE_MAX_BYTES, KNOWLEDGE_SUMMARY_MAX_BYTES, parseCollaborationKnowledgeCandidates,
  parseCollaborationKnowledgeDetail, parseCollaborationKnowledgeReceipt, parseCollaborationKnowledgeTargetList } from "./knowledge-schema";
import { knowledgeCandidateFixture, knowledgeCandidatesFixture, knowledgeDetailFixture, knowledgeIds, knowledgeUnavailableCandidateFixture,
  knowledgeProposalFixture, knowledgeReceiptFixture, knowledgeTargetFixture } from "./knowledge-test-fixtures";

afterEach(() => setSchemaLogger(noopLogger));
describe("knowledge protocol boundaries", () => {
  it("accepts a content-free rejection grant for unavailable pending material without granting approval or text", () => {
    const detail = { ...knowledgeDetailFixture, candidate: knowledgeUnavailableCandidateFixture };
    expect(parseCollaborationKnowledgeDetail(detail)?.candidate).toEqual(knowledgeUnavailableCandidateFixture);
    expect(parseCollaborationKnowledgeCandidates({ ...knowledgeCandidatesFixture, items: [knowledgeUnavailableCandidateFixture] })?.items).toEqual([knowledgeUnavailableCandidateFixture]);
    expect(parseCollaborationKnowledgeTargetList({ ...knowledgeTargetFixture, items: [knowledgeUnavailableCandidateFixture] })?.items).toEqual([knowledgeUnavailableCandidateFixture]);
    expect(() => parseCollaborationKnowledgeDetail({ ...detail, candidate: { ...knowledgeUnavailableCandidateFixture,
      capabilities: { ...knowledgeUnavailableCandidateFixture.capabilities, can_approve: true } } })).toThrow();
    for (const field of ["summary", "content", "content_sha256", "sources", "evidence"]) {
      expect(() => parseCollaborationKnowledgeDetail({ ...detail, candidate: { ...knowledgeUnavailableCandidateFixture, [field]: "PRIVATE" } })).toThrow();
    }
    expect(() => parseCollaborationKnowledgeDetail({ ...detail, candidate: { candidate_id: knowledgeIds.candidate, availability: "restricted", state: "pending", state_revision: "1",
      capabilities: { can_withdraw: true, can_reject: true } } })).toThrow();
  });
  it("requires explicit detail history completeness and preserves true/false without inferred defaults", () => {
    for (const history_truncated of [true, false]) {
      expect(parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, history_truncated })).toHaveProperty("history_truncated", history_truncated);
    }
    for (const history_truncated of [undefined, null, "false", 0]) {
      expect(() => parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, history_truncated })).toThrow(CollaborationKnowledgeProtocolError);
    }
  });
  it("bounds detail history at 50 while source pages retain multiple versions of the same lineage", () => {
    const history = Array.from({ length: 51 }, (_, index) => ({ ...knowledgeCandidatesFixture.items[0],
      candidate_id: `00000000-0000-4000-8000-${String(index + 100).padStart(12, "0")}`, version: String(index + 1) }));
    expect(parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, history: history.slice(0, 50), history_truncated: true })?.history).toHaveLength(50);
    expect(() => parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, history, history_truncated: true })).toThrow(CollaborationKnowledgeProtocolError);
    const versions = history.slice(0, 2);
    expect(parseCollaborationKnowledgeCandidates({ ...knowledgeCandidatesFixture, items: versions })?.items.map((item) => item.candidate_id))
      .toEqual(versions.map((item) => item.candidate_id));
  });
  it("distinguishes an older unsupported response from malformed claimed support without logging private material", () => {
    const warn = vi.fn(); setSchemaLogger({ ...noopLogger, warn });
    expect(parseCollaborationKnowledgeCandidates({ items: [] })).toBeNull();
    expect(parseCollaborationKnowledgeDetail(undefined)).toBeNull();
    expect(() => parseCollaborationKnowledgeDetail({ protocol_version: 1, content: "PRIVATE SENTINEL" })).toThrow(CollaborationKnowledgeProtocolError);
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE SENTINEL");
    expect(() => parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, protocol_version: "1" })).toThrow();
  });
  it("retains exact bigint revisions and human projections for all endpoints", () => {
    expect(parseCollaborationKnowledgeCandidates(knowledgeCandidatesFixture)).toEqual(knowledgeCandidatesFixture);
    expect(parseCollaborationKnowledgeDetail(knowledgeDetailFixture)).toEqual(knowledgeDetailFixture);
    expect(parseCollaborationKnowledgeTargetList(knowledgeTargetFixture)).toEqual(knowledgeTargetFixture);
    expect(parseCollaborationKnowledgeReceipt(knowledgeReceiptFixture)?.version).toBe("9007199254740993");
    expect(() => parseCollaborationKnowledgeReceipt({ ...knowledgeReceiptFixture, version: Number("9007199254740993") })).toThrow();
  });
  it("keeps missing capabilities unknown and rejects truthy non-booleans", () => {
    const candidate = { ...knowledgeCandidateFixture, capabilities: undefined };
    const parsed = parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, candidate });
    expect(parsed?.candidate.capabilities).toEqual({ can_approve: false, can_reject: false, can_withdraw: false });
    for (const value of [1, "true", {}, null]) expect(() => parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture,
      candidate: { ...candidate, capabilities: { can_approve: value } } })).toThrow();
  });
  it("allows only content-free restricted control rows and source-unavailable audit rows", () => {
    const restricted = { candidate_id: knowledgeIds.candidate, availability: "restricted", state: "approved", state_revision: "2", capabilities: { can_withdraw: true } };
    expect(parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, candidate: restricted })?.candidate).toEqual(restricted);
    for (const field of ["source_issue_id", "source_workspace_id", "summary", "content", "content_sha256", "sources", "reason", "version"]) {
      expect(() => parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, candidate: { ...restricted, [field]: "PRIVATE" } })).toThrow();
    }
    const { content: _body, summary: _summary, content_sha256: _hash, sources: _sources, evidence: _evidence, applicability: _applicability, ...audit } = knowledgeCandidateFixture;
    const unavailable = { ...audit, availability: "source_unavailable", capabilities: { can_approve: false, can_reject: false, can_withdraw: true } };
    expect(parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, candidate: unavailable })?.candidate).toEqual(unavailable);
    expect(() => parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, candidate: { ...unavailable, content: "OLD PRIVATE BODY" } })).toThrow();
    expect(() => parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, candidate: { ...unavailable,
      decision: { actor_user_id: knowledgeIds.actor, at: knowledgeDetailFixture.projection_checked_at, reason: "OLD PRIVATE QUOTE" } } })).toThrow();
    expect(() => parseCollaborationKnowledgeDetail({ ...knowledgeDetailFixture, candidate: restricted, history: knowledgeCandidatesFixture.items })).toThrow();
  });
  it("rejects cross-source, cross-target, duplicate or body-bearing list projections", () => {
    expect(() => parseCollaborationKnowledgeCandidates({ ...knowledgeCandidatesFixture, issue_id: knowledgeIds.actor })).toThrow();
    expect(() => parseCollaborationKnowledgeCandidates({ ...knowledgeCandidatesFixture, items: [knowledgeCandidateFixture] })).toThrow();
    expect(() => parseCollaborationKnowledgeCandidates({ ...knowledgeCandidatesFixture, items: [...knowledgeCandidatesFixture.items, ...knowledgeCandidatesFixture.items] })).toThrow();
    expect(() => parseCollaborationKnowledgeTargetList({ ...knowledgeTargetFixture, target: { kind: "workspace", workspace_id: knowledgeIds.actor } })).toThrow();
  });
  it("requires exact reviewed fields for approval and state revision plus explanation for narrowing decisions", () => {
    const input = { protocol_version: 1, request_id: knowledgeIds.request, expected_state_revision: "1", expected_version: "9007199254740993", expected_content_sha256: "a".repeat(64) };
    expect(CollaborationKnowledgeApproveRequestSchema.parse(input)).toEqual(input);
    for (const field of ["expected_state_revision", "expected_version", "expected_content_sha256"]) expect(CollaborationKnowledgeApproveRequestSchema.safeParse({ ...input, [field]: undefined }).success).toBe(false);
    expect(CollaborationKnowledgeDecisionRequestSchema.safeParse({ protocol_version: 1, request_id: knowledgeIds.request, expected_state_revision: "1", reason: "" }).success).toBe(false);
  });
  it("enforces UTF-8 byte budgets, required evidence anchors and strict proposal fields", () => {
    expect(CollaborationKnowledgeProposalSchema.parse(knowledgeProposalFixture)).toEqual(knowledgeProposalFixture);
    for (const change of [{ content: "😀".repeat(16_385) }, { summary: "é".repeat(1025) }, { content: "bad\0text" }, { content: "bad\ud800text" },
      { sources: [] }, { unexpected: true }, { target: { ...knowledgeProposalFixture.target, project_id: knowledgeIds.project } },
      { evidence: [{ kind: "run_record", run_id: knowledgeIds.actor, record_id: "log", verification: "verified" }] }]) {
      expect(CollaborationKnowledgeProposalSchema.safeParse({ ...knowledgeProposalFixture, ...change }).success).toBe(false);
    }
    const evidence = { kind: "test", source: knowledgeProposalFixture.sources[0], reference: "Recorded test result", verification: "observed" };
    expect(CollaborationKnowledgeProposalSchema.safeParse({ ...knowledgeProposalFixture, evidence: [evidence] }).success).toBe(true);
    expect(CollaborationKnowledgeProposalSchema.safeParse({ ...knowledgeProposalFixture,
      evidence: Array.from({ length: 6 }, () => ({ ...evidence, reference: "\n".repeat(2000) + "x" })) }).success).toBe(true);
    expect(CollaborationKnowledgeProposalSchema.safeParse({ ...knowledgeProposalFixture,
      evidence: Array.from({ length: 8 }, () => ({ ...evidence, reference: "x".repeat(2048) })) }).success).toBe(false);
    // Expiry is rechecked by a new server write after original-key lookup, not by a client retry.
    expect(CollaborationKnowledgeProposalSchema.safeParse({ ...knowledgeProposalFixture, expires_at: "2020-01-01T00:00:00Z" }).success).toBe(true);
    expect(CollaborationKnowledgeProposalSchema.safeParse({ ...knowledgeProposalFixture, expires_at: "2020-01-01T00:00:00+08:00" }).success).toBe(false);
  });
  // L9: views compare these exact constants with collaborationTextByteLength before submit.
  it("exports the UTF-8 byte limits its schemas enforce", () => {
    // "知" is 3 UTF-8 bytes; the ASCII tail lands exactly on the limit, one more byte exceeds it.
    const bytes = (count: number) => "知".repeat(Math.floor(count / 3)) + "a".repeat(count % 3);
    const decision = (reason: string) => CollaborationKnowledgeDecisionRequestSchema.safeParse({ protocol_version: 1, request_id: knowledgeIds.request, expected_state_revision: "1", reason }).success;
    const proposal = (change: Partial<typeof knowledgeProposalFixture>) => CollaborationKnowledgeProposalSchema.safeParse({ ...knowledgeProposalFixture, ...change }).success;
    const evidence = (reference: string) => proposal({ evidence: [{ kind: "test", source: knowledgeProposalFixture.sources[0]!, reference, verification: "observed" }] });
    expect([KNOWLEDGE_SUMMARY_MAX_BYTES, KNOWLEDGE_CONTENT_MAX_BYTES, KNOWLEDGE_REFERENCE_MAX_BYTES, KNOWLEDGE_REASON_MAX_BYTES]).toEqual([2048, 65_536, 2048, 1000]);
    expect([decision(bytes(KNOWLEDGE_REASON_MAX_BYTES)), decision(bytes(KNOWLEDGE_REASON_MAX_BYTES + 1))]).toEqual([true, false]);
    expect([proposal({ summary: bytes(KNOWLEDGE_SUMMARY_MAX_BYTES) }), proposal({ summary: bytes(KNOWLEDGE_SUMMARY_MAX_BYTES + 1) })]).toEqual([true, false]);
    expect([proposal({ content: bytes(KNOWLEDGE_CONTENT_MAX_BYTES) }), proposal({ content: bytes(KNOWLEDGE_CONTENT_MAX_BYTES + 1) })]).toEqual([true, false]);
    expect([evidence(bytes(KNOWLEDGE_REFERENCE_MAX_BYTES)), evidence(bytes(KNOWLEDGE_REFERENCE_MAX_BYTES + 1))]).toEqual([true, false]);
  });
  it("permits only three content-free target hint fields and content-free operation receipts", () => {
    const hint = { target_kind: "project", target_id: knowledgeIds.project, knowledge_revision: "9223372036854775807" };
    expect(CollaborationKnowledgeHintSchema.parse(hint)).toEqual(hint);
    for (const field of ["source_id", "candidate_id", "actor_id", "cause_id", "summary", "body", "hash", "count", "reason"]) {
      expect(CollaborationKnowledgeHintSchema.safeParse({ ...hint, [field]: "private" }).success).toBe(false);
      expect(() => parseCollaborationKnowledgeReceipt({ ...knowledgeReceiptFixture, [field]: "private" })).toThrow();
    }
    expect(CollaborationKnowledgeHintSchema.safeParse({ ...hint, knowledge_revision: "9223372036854775808" }).success).toBe(false);
  });
});
