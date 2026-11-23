// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseKnowledgeAutomationJobs, parseProjectKnowledgePolicy, parseProjectKnowledgePolicyReceipt,
  parseKnowledgeAutomationRetryReceipt, selectKnowledgeAutomationJob } from "./knowledge-automation-schema";
import { knowledgeIds, knowledgeLeaseFixture } from "./knowledge-test-fixtures";

const policy = { ...knowledgeLeaseFixture, protocol_version: 2, project_id: knowledgeIds.project, workspace_id: knowledgeIds.workspace,
  revision: "0", is_enabled: true, max_candidates: 3, rule_version: "project-knowledge-v1", origin: "system",
  changed_by_user_id: null, updated_at: null, can_manage: true };
const job = { id: knowledgeIds.request, issue_id: knowledgeIds.issue, project_id: knowledgeIds.project,
  submission_id: knowledgeIds.candidate, accepted_event_seq: "9007199254740993", revision: "1", phase: "completed", reason_code: null,
  source_current: true, can_retry: false, counts: { saved: 1, restored: 0, revised: 0, pending: 0, skipped: 0 },
  attempts: [{ stage: "generating", state: "succeeded", model: "served-model", usage: { status: "known", input_tokens: 0,
    output_tokens: 0, total_tokens: 0, cached_input_tokens: null, reasoning_tokens: null } }] };
const jobs = { ...knowledgeLeaseFixture, protocol_version: 2, issue_id: knowledgeIds.issue, items: [job], has_more: false };

describe("knowledge policy and job protocol", () => {
  it("preserves system defaults and explicit human disablement without fabricated actors", () => {
    expect(parseProjectKnowledgePolicy(policy)).toEqual(policy);
    const override = { ...policy, revision: "2", is_enabled: false, max_candidates: 1, origin: "human",
      changed_by_user_id: knowledgeIds.actor, updated_at: policy.projection_checked_at };
    expect(parseProjectKnowledgePolicy(override)).toEqual(override);
    for (const change of [{ is_enabled: "true" }, { max_candidates: 0 }, { max_candidates: 4 }, { revision: 1 },
      { changed_by_user_id: knowledgeIds.actor }, { origin: "human" }, { can_manage: 1 }]) {
      expect(() => parseProjectKnowledgePolicy({ ...policy, ...change })).toThrow();
    }
  });

  it("keeps actual zero usage distinct from unknown and partial counts", () => {
    expect(parseKnowledgeAutomationJobs(jobs)).toEqual(jobs);
    for (const usage of [{ status: "unknown" }, { status: "partial", input_tokens: 0 },
      { status: "unknown", input_tokens: null, output_tokens: null, total_tokens: null, cached_input_tokens: null, reasoning_tokens: null }]) {
      const value = { ...jobs, items: [{ ...job, attempts: [{ ...job.attempts[0], usage }] }] };
      expect(parseKnowledgeAutomationJobs(value)).toEqual(value);
    }
    for (const usage of [{ status: "known", input_tokens: 0 }, { status: "unknown", output_tokens: 0 },
      { status: "partial" }, { status: "known", input_tokens: -1, output_tokens: 0, total_tokens: 0 }]) {
      expect(() => parseKnowledgeAutomationJobs({ ...jobs, items: [{ ...job, attempts: [{ ...job.attempts[0], usage }] }] })).toThrow();
    }
  });

  it("rejects private details, foreign jobs, duplicate identities and malformed paging", () => {
    for (const value of [{ ...jobs, items: [{ ...job, issue_id: knowledgeIds.actor }] }, { ...jobs, items: [job, job] },
      { ...jobs, has_more: "true" }, { ...jobs, has_more: true, items: [] },
      { ...jobs, items: [{ ...job, source_current: false, can_retry: true }] },
      { ...jobs, items: [{ ...job, reason_code: "PRIVATE FREE TEXT" }] },
      { ...jobs, items: [{ ...job, attempts: [{ ...job.attempts[0], reason: "PRIVATE MODEL REASON" }] }] },
      { ...jobs, items: [{ ...job, raw_response: "PRIVATE BODY" }] }]) expect(() => parseKnowledgeAutomationJobs(value)).toThrow();
  });

  it("retains every server-admitted stage attempt and rejects excess history", () => {
    const attempts = Array.from({ length: 7 }, (_, batch) => ["generating", "checking"].flatMap((stage) =>
      Array.from({ length: 3 }, (_, attempt) => ({ ...job.attempts[0], stage,
        state: attempt === 2 ? "succeeded" : "failed", model: `served-model-batch-${batch + 1}` })))).flat();
    const complete = { ...jobs, items: [{ ...job, attempts }] };
    expect(parseKnowledgeAutomationJobs(complete)).toEqual(complete);
    expect(() => parseKnowledgeAutomationJobs({ ...jobs, items: [{ ...job, attempts: [...attempts, job.attempts[0]] }] })).toThrow();
  });

  it("requires v2 for new surfaces and never treats malformed claimed support as absent", () => {
    expect(parseProjectKnowledgePolicy({})).toBeNull();
    expect(parseKnowledgeAutomationJobs(undefined)).toBeNull();
    expect(() => parseProjectKnowledgePolicy({ ...policy, protocol_version: 1 })).toThrow();
    expect(() => parseKnowledgeAutomationJobs({ ...jobs, protocol_version: 3 })).toThrow();
  });

  it("accepts only content-free known original-operation outcomes", () => {
    const policyReceipt = { protocol_version: 2, request_id: knowledgeIds.request, project_id: knowledgeIds.project,
      revision: "1", code: "knowledge_policy_updated" };
    const retryReceipt = { protocol_version: 2, request_id: knowledgeIds.request, issue_id: knowledgeIds.issue, job_id: job.id,
      revision: "2", code: "knowledge_job_retried" };
    expect(parseProjectKnowledgePolicyReceipt(policyReceipt)).toEqual(policyReceipt);
    expect(parseKnowledgeAutomationRetryReceipt(retryReceipt)).toEqual(retryReceipt);
    expect(() => parseProjectKnowledgePolicyReceipt({ ...policyReceipt, code: "probably_done" })).toThrow();
    expect(() => parseKnowledgeAutomationRetryReceipt({ ...retryReceipt, reason: "PRIVATE" })).toThrow();
  });

  it("selects only the exact acceptance and current project, retaining reopen status", () => {
    const parsed = parseKnowledgeAutomationJobs(jobs)!;
    const selection = { issueId: knowledgeIds.issue, projectId: knowledgeIds.project, submissionId: knowledgeIds.candidate,
      acceptedEventSeq: job.accepted_event_seq, aiPolicy: "assisted" };
    expect(selectKnowledgeAutomationJob(parsed, selection)?.id).toBe(job.id);
    for (const change of [{ projectId: null }, { aiPolicy: "manual" }, { submissionId: knowledgeIds.actor },
      { acceptedEventSeq: "9007199254740994" }]) expect(selectKnowledgeAutomationJob(parsed, { ...selection, ...change })).toBeNull();
    const reopened = parseKnowledgeAutomationJobs({ ...jobs, items: [{ ...job, source_current: false }] })!;
    expect(selectKnowledgeAutomationJob(reopened, selection)?.source_current).toBe(false);
  });
});
