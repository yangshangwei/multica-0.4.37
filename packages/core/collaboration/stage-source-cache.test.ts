// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { collaborationScope, collaborationKeys, collaborationIssueAccessDenied } from "./access";
import { collaborationStageCodeCandidateOptions } from "./stage-queries";
import { readCollaborationStageSource, eraseCollaborationStageSource } from "./stage-source-cache";
import { parseCollaborationStageCodeCandidate } from "./stage-schema";
import { stageCodeViewFixture, stageIds } from "./stage-test-fixtures";
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((c) => c.clear()); vi.restoreAllMocks(); });
const allowed = parseCollaborationStageCodeCandidate(stageCodeViewFixture)!;
const denied = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, baseline: null }, blocked_evidence: 1,
  observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
  signable: stageCodeViewFixture.signable.map((s) => ({ ...s, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "code_review" }] })) })!;
it("C02 source denial erases sibling and cross-issue R4 caches and fences late responses without erasing issue identity", async () => {
  const api = new ApiClient("https://server.example");setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });clients.push(client);
  const scope = collaborationScope(stageIds.product);
  const a = collaborationStageCodeCandidateOptions(client, scope, stageIds.issue, stageIds.candidate1, stageIds.workspace);
  const b = collaborationStageCodeCandidateOptions(client, scope, stageIds.issue, stageIds.candidate2, stageIds.workspace);
  const c = collaborationStageCodeCandidateOptions(client, scope, stageIds.child1, stageIds.candidate1, stageIds.workspace);
  const other = collaborationStageCodeCandidateOptions(client, scope, stageIds.child2, stageIds.candidate1, stageIds.project);
  const read = vi.spyOn(api, "getCollaborationStageCodeCandidate").mockImplementation(async (issueId, candidateId) => ({ state: "available", value: { ...allowed, issue_id: issueId, candidate: { ...allowed.candidate, candidate_id: candidateId } } }));
  await Promise.all([client.fetchQuery(a), client.fetchQuery(b), client.fetchQuery(c), client.fetchQuery(other)]);
  client.setQueryData(collaborationKeys.issue(scope, stageIds.issue), { id: stageIds.issue });
  let late!: (value: { state: "available"; value: typeof allowed }) => void;
  read.mockImplementationOnce(() => new Promise((resolve) => { late = resolve; }));
  const pending = client.fetchQuery(b).catch(() => undefined);
  read.mockResolvedValueOnce({ state: "available", value: denied });
  await client.fetchQuery(a);
  expect(client.getQueryData(b.queryKey)).toBeUndefined();
  expect(client.getQueryData(c.queryKey)).toBeUndefined();
  expect(client.getQueryData(other.queryKey)).toMatchObject({ state: "available" });
  expect(client.getQueryData(collaborationKeys.issue(scope, stageIds.issue))).toEqual({ id: stageIds.issue });
  late({ state: "available", value: allowed });await pending;
  expect(client.getQueryData(b.queryKey)).toBeUndefined();
  read.mockResolvedValueOnce({ state: "available", value: allowed });
  await expect(client.fetchQuery(b)).resolves.toMatchObject({ state: "available" });
});

it("C02 fences a late error before it can revoke otherwise-readable collaboration identity", async () => {
  setApiInstance(new ApiClient("https://server.example"));const client = new QueryClient();clients.push(client);
  const scope = collaborationScope(stageIds.product);
  let reject!: (error: Error) => void;
  const read = readCollaborationStageSource(client, scope, stageIds.issue, stageIds.workspace, ["candidate"], () => new Promise((_resolve, failure) => { reject = failure; }));
  const outcome = read.catch((error: unknown) => error);
  eraseCollaborationStageSource(client, scope, stageIds.workspace);
  reject(new ApiError("Denied", 404, "Not Found"));
  await expect(outcome).resolves.toMatchObject({ name: "AbortError" });
  expect(collaborationIssueAccessDenied(client, scope, stageIds.issue)).toBe(false);
});
it("C02 repeated workspace denial preserves an already-anonymous candidate identity", () => {
  setApiInstance(new ApiClient("https://server.example"));const client = new QueryClient();clients.push(client);
  const scope = collaborationScope(stageIds.product);
  const options = collaborationStageCodeCandidateOptions(client, scope, stageIds.issue, stageIds.candidate1, stageIds.workspace);
  client.setQueryData(options.queryKey, { state: "available", value: denied });
  eraseCollaborationStageSource(client, scope, stageIds.workspace);
  expect(client.getQueryData(options.queryKey)).toEqual({ state: "available", value: denied });
});
