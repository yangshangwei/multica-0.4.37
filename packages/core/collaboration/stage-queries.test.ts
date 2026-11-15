// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient, type InfiniteData } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope, CollaborationUnavailableError } from "./access";
import {
  collaborationStageCodeCandidateOptions, collaborationStageKeys, collaborationStagePolicyOptions, collaborationStageProposalOptions, collaborationStageProposalsOptions, collaborationStageViewOptions,
} from "./stage-queries";
import { parseCollaborationStagePolicy, parseCollaborationStageProposalDetail, parseCollaborationStageProposalPage, parseCollaborationStageView, parseCollaborationStageCodeCandidate,
  type CollaborationStageProposalPage, type CollaborationStageRead } from "./stage-schema";
import { stageIds, stagePolicyFixture, stageProposalDetailFixture, stageProposalListItemFixture, stageProposalPageFixture, stageViewManagedFixture, stageCodeViewFixture } from "./stage-test-fixtures";
import { collaborationTestReportKeys } from "./stage-test-report-queries";
import { stageReportDetailFixture } from "./stage-test-report-fixtures";
import { saveCollaborationDraft, readCollaborationDrafts } from "./draft";
import { parseCollaborationCommentDraft } from "./comment";

const actor = stageIds.product;
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const saved = vi.hoisted(() => new Map<string, string>());
vi.mock("../platform/storage", () => ({ defaultStorage: {
  keys: () => [...saved.keys()], getItem: (key: string) => saved.get(key) ?? null,
  setItem: (key: string, value: string) => saved.set(key, value), removeItem: (key: string) => saved.delete(key),
} }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); saved.clear(); vi.restoreAllMocks(); });
function setup() {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  clients.push(client);
  return { api, client, scope: collaborationScope(actor) };
}
const view = parseCollaborationStageView(stageViewManagedFixture)!;
const page = (items: unknown[], next_cursor: string | null): CollaborationStageRead<CollaborationStageProposalPage> =>
  ({ state: "available", value: parseCollaborationStageProposalPage({ ...stageProposalPageFixture, items, next_cursor })! });
const item = (proposalId: string) => ({ ...stageProposalListItemFixture, proposal_id: proposalId });

describe("query keys", () => {
  it("live under the collaboration issue key so issue invalidation and access loss cover them", () => {
    const { scope } = setup();
    const issueKey = collaborationKeys.issue(scope, stageIds.issue);
    const keys = [collaborationStageKeys.view(scope, stageIds.issue), collaborationStageKeys.proposals(scope, stageIds.issue),
      collaborationStageKeys.proposal(scope, stageIds.issue, stageIds.proposal.toUpperCase()), collaborationStageKeys.policy(scope, stageIds.issue, stageIds.workspace, stageIds.project)];
    for (const key of keys) expect(key.slice(0, issueKey.length)).toEqual([...issueKey]);
    expect(new Set(keys.map((key) => JSON.stringify(key))).size).toBe(4);
    expect(collaborationStageKeys.proposal(scope, stageIds.issue, stageIds.proposal.toUpperCase())).toContain(stageIds.proposal);
    // Workspace-scoped policy data carries its workspace and project.
    expect(collaborationStageKeys.policy(scope, stageIds.issue, stageIds.workspace, stageIds.project).slice(-2)).toEqual([stageIds.workspace, stageIds.project]);
  });
});

describe("stage view", () => {
  it("B01 fences source reports and late R4 responses before publishing an unreadable R1, then accepts fresh restoration", async () => {
    const f = setup();
    const denied = parseCollaborationStageView({ ...view, children: { ...view.children,
      items: view.children.items.map((child) => ({ ...child, can_open: false, content_digest: null })) } })!;
    const options = collaborationStageViewOptions(f.client, f.scope, stageIds.issue, stageIds.workspace);
    const code = collaborationStageCodeCandidateOptions(f.client, f.scope, stageIds.issue, stageIds.candidate1, stageIds.workspace);
    const sibling = collaborationStageKeys.codeCandidate(f.scope, stageIds.child1, stageIds.candidate2, stageIds.workspace);
    const report = collaborationTestReportKeys.detail(f.scope, stageIds.child1, stageIds.workspace, stageIds.run, stageReportDetailFixture.report_id);
    const other = collaborationTestReportKeys.detail(f.scope, stageIds.child2, stageIds.project, stageIds.run, stageReportDetailFixture.report_id);
    const readableCode = parseCollaborationStageCodeCandidate(stageCodeViewFixture)!;
    for (const key of [code.queryKey, sibling]) f.client.setQueryData(key, { state: "available", value: readableCode });
    for (const key of [report, other]) f.client.setQueryData(key, { state: "available", value: stageReportDetailFixture });
    const issueKey = collaborationKeys.issue(f.scope, stageIds.issue);
    f.client.setQueryData(issueKey, { id: stageIds.issue, title: "Public issue" });
    const original = { scope: { server: f.scope.server, userId: f.scope.userId, issueId: stageIds.issue }, requestId: stageIds.request,
      kind: "comment" as const, payload: { domain: "create_comment", input: {
        content: `[@Agent](mention://agent/${stageIds.agent}) test this`, parent_id: null, suppressed_target_ids: [], execution_view_version: 6,
        expected_lifecycle_epoch: "1", expected_participant_epoch: "1", expected_account_epoch: "1",
        target_versions: [{ target_id: stageIds.agent, expected_target_epoch: "1", expected_config_version: "1" }],
        test_subject: { candidate_id: readableCode.candidate.candidate_id, candidate_version: readableCode.candidate.version, candidate_digest: readableCode.candidate.content_digest },
      } }, createdAt: 1 };
    expect(parseCollaborationCommentDraft(original).test_subject?.candidate_id).toBe(stageIds.candidate1);
    saveCollaborationDraft(original);
    let resolve!: (value: { state: "available"; value: typeof readableCode }) => void;
    const readCode = vi.spyOn(f.api, "getCollaborationStageCodeCandidate").mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
    const pending = f.client.fetchQuery(code).catch(() => undefined);
    vi.spyOn(f.api, "getCollaborationStage").mockResolvedValueOnce({ state: "available", value: denied }).mockResolvedValueOnce({ state: "available", value: view });
    await expect(f.client.fetchQuery(options)).resolves.toMatchObject({ state: "available", value: denied });
    for (const key of [code.queryKey, sibling, report]) expect(f.client.getQueryData(key)).toBeUndefined();
    expect(f.client.getQueryData(other)).toBeDefined();
    expect(f.client.getQueryData(issueKey)).toEqual({ id: stageIds.issue, title: "Public issue" });
    expect(readCollaborationDrafts(original.scope)).toEqual([original]);
    expect(collaborationIssueAccessDenied(f.client, f.scope, stageIds.issue)).toBe(false);
    resolve({ state: "available", value: readableCode });
    await pending;
    expect(f.client.getQueryData(code.queryKey)).toBeUndefined();
    await expect(f.client.fetchQuery(options)).resolves.toMatchObject({ state: "available", value: view });
    readCode.mockResolvedValueOnce({ state: "available", value: readableCode });
    await expect(f.client.fetchQuery(code)).resolves.toMatchObject({ state: "available", value: readableCode });
  });
  it("B01 requires an actual source workspace and does not interpret an empty child list as denial", async () => {
    const f = setup();
    const read = vi.spyOn(f.api, "getCollaborationStage").mockResolvedValue({ state: "available", value: { ...view, children: { items: [], truncated: false } } });
    await expect(f.client.fetchQuery(collaborationStageViewOptions(f.client, f.scope, stageIds.issue, ""))).rejects.toThrow();
    expect(read).not.toHaveBeenCalled();
    const code = collaborationStageKeys.codeCandidate(f.scope, stageIds.issue, stageIds.candidate1, stageIds.workspace);
    f.client.setQueryData(code, { state: "available", value: parseCollaborationStageCodeCandidate(stageCodeViewFixture)! });
    await f.client.fetchQuery(collaborationStageViewOptions(f.client, f.scope, stageIds.issue, stageIds.workspace));
    expect(f.client.getQueryData(code)).toBeDefined();
  });
  it("caches the read state, including an older server, as data rather than an error to retry", async () => {
    const f = setup();
    const read = vi.spyOn(f.api, "getCollaborationStage").mockResolvedValueOnce({ state: "available", value: view }).mockResolvedValueOnce({ state: "unsupported" });
    const options = collaborationStageViewOptions(f.client, f.scope, stageIds.issue, stageIds.workspace);
    await expect(f.client.fetchQuery(options)).resolves.toEqual({ state: "available", value: view });
    await expect(f.client.fetchQuery(options)).resolves.toEqual({ state: "unsupported" });
    expect(read).toHaveBeenCalledWith(stageIds.issue, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(collaborationIssueAccessDenied(f.client, f.scope, stageIds.issue)).toBe(false);
    expect(options.retry).toBe(false);
  });
  it("stops polling a server without the route and keeps polling an available view", () => {
    const f = setup();
    const interval = collaborationStageViewOptions(f.client, f.scope, stageIds.issue, stageIds.workspace).refetchInterval as (query: { state: { data?: unknown } }) => number | false;
    expect(interval({ state: { data: { state: "unsupported" } } })).toBe(false);
    expect(interval({ state: { data: { state: "available", value: view } } })).toBeGreaterThan(0);
    expect(interval({ state: {} })).toBeGreaterThan(0);
  });
  it("clears protected issue state only on a structured object denial", async () => {
    const f = setup();
    vi.spyOn(f.api, "getCollaborationStage").mockRejectedValueOnce(new ApiError("Collaboration stage request failed", 503, "Unavailable"))
      .mockRejectedValueOnce(new ApiError("Collaboration stage request failed", 404, "Not Found"));
    const options = collaborationStageViewOptions(f.client, f.scope, stageIds.issue, stageIds.workspace);
    await expect(f.client.fetchQuery(options)).rejects.toMatchObject({ status: 503 });
    expect(collaborationIssueAccessDenied(f.client, f.scope, stageIds.issue)).toBe(false);
    // The access fence cancels the read it denies, so the caller sees the cancellation.
    await expect(f.client.fetchQuery(options)).rejects.toBeDefined();
    expect(collaborationIssueAccessDenied(f.client, f.scope, stageIds.issue)).toBe(true);
    expect(f.client.getQueryData(options.queryKey)).toBeUndefined();
  });
});

describe("proposal list pages", () => {
  it("follows the server cursor and stops at the end", async () => {
    const f = setup();
    const read = vi.spyOn(f.api, "getCollaborationStageProposals").mockResolvedValueOnce(page([item(stageIds.proposal)], "older")).mockResolvedValueOnce(page([item(stageIds.child1)], null));
    const options = collaborationStageProposalsOptions(f.client, f.scope, stageIds.issue);
    const data = await f.client.fetchInfiniteQuery({ ...options, pages: 2 });
    expect(data.pages).toHaveLength(2);
    expect(read.mock.calls.map(([, params]) => params)).toEqual([{ limit: 20, cursor: undefined }, { limit: 20, cursor: "older" }]);
    expect(options.getNextPageParam!(data.pages[1]!, data.pages, "older", data.pageParams)).toBeUndefined();
  });
  it("refuses a proposal repeated across pages and a cursor that loops", () => {
    const f = setup();
    const share = collaborationStageProposalsOptions(f.client, f.scope, stageIds.issue).structuralSharing as (previous: unknown, next: unknown) => unknown;
    const chain = (pages: CollaborationStageRead<CollaborationStageProposalPage>[], pageParams: (string | undefined)[]): InfiniteData<CollaborationStageRead<CollaborationStageProposalPage>, string | undefined> => ({ pages, pageParams });
    expect(() => share(undefined, chain([page([item(stageIds.proposal)], "a"), page([item(stageIds.child1)], null)], [undefined, "a"]))).not.toThrow();
    expect(() => share(undefined, chain([page([item(stageIds.proposal)], "a"), page([item(stageIds.proposal)], null)], [undefined, "a"]))).toThrow(CollaborationUnavailableError);
    expect(() => share(undefined, chain([page([item(stageIds.proposal)], "a"), page([item(stageIds.child1)], "a")], [undefined, "a"]))).toThrow(CollaborationUnavailableError);
    // An unavailable first page is a state to render, not a chain to validate.
    expect(() => share(undefined, chain([{ state: "unsupported" }], [undefined]))).not.toThrow();
  });
  it("does not continue past a page that is not available", () => {
    const f = setup();
    const options = collaborationStageProposalsOptions(f.client, f.scope, stageIds.issue);
    expect(options.getNextPageParam!({ state: "unavailable", reason: "malformed" }, [], undefined, [undefined])).toBeUndefined();
    const more = page([item(stageIds.proposal)], "older");
    expect(options.getNextPageParam!(more, [more], undefined, [undefined])).toBe("older");
    expect(options.getNextPageParam!(more, [more], "older", [undefined, "older"])).toBeUndefined();
  });
});

describe("proposal detail and project policy", () => {
  it("reads a proposal under the issue guard and keeps a missing proposal as data", async () => {
    const f = setup();
    const detail = parseCollaborationStageProposalDetail(stageProposalDetailFixture)!;
    vi.spyOn(f.api, "getCollaborationStageProposal").mockResolvedValueOnce({ state: "available", value: detail }).mockResolvedValueOnce({ state: "not_found" });
    const options = collaborationStageProposalOptions(f.client, f.scope, stageIds.issue, stageIds.proposal);
    await expect(f.client.fetchQuery(options)).resolves.toMatchObject({ state: "available" });
    await expect(f.client.fetchQuery(options)).resolves.toEqual({ state: "not_found" });
    expect(collaborationIssueAccessDenied(f.client, f.scope, stageIds.issue)).toBe(false);
  });
  it("reads the project policy for the captured workspace without touching issue access on a member denial", async () => {
    const f = setup();
    const policy = parseCollaborationStagePolicy(stagePolicyFixture)!;
    const read = vi.spyOn(f.api, "getProjectStagePolicy").mockResolvedValueOnce({ state: "available", value: policy }).mockResolvedValueOnce({ state: "denied" });
    const options = collaborationStagePolicyOptions(f.client, f.scope, stageIds.issue, stageIds.workspace, stageIds.project);
    await expect(f.client.fetchQuery(options)).resolves.toEqual({ state: "available", value: policy });
    await expect(f.client.fetchQuery(options)).resolves.toEqual({ state: "denied" });
    expect(read).toHaveBeenCalledWith(stageIds.workspace, stageIds.project, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(collaborationIssueAccessDenied(f.client, f.scope, stageIds.issue)).toBe(false);
  });
});

describe("R4 protected candidate read", () => {
  it("uses the issue invalidation prefix, forwards cancellation and erases evidence on current object denial", async () => {
    const f = setup();
    const value = parseCollaborationStageCodeCandidate(stageCodeViewFixture)!;
    const read = vi.spyOn(f.api, "getCollaborationStageCodeCandidate").mockResolvedValueOnce({ state: "available", value })
      .mockRejectedValueOnce(new ApiError("Unavailable", 404, "Not Found"));
    const options = collaborationStageCodeCandidateOptions(f.client, f.scope, stageIds.issue, stageIds.candidate1, stageIds.workspace);
    const prefix = collaborationKeys.issue(f.scope, stageIds.issue);
    expect(options.queryKey.slice(0, prefix.length)).toEqual([...prefix]);
    await expect(f.client.fetchQuery(options)).resolves.toMatchObject({ state: "available" });
    expect(read).toHaveBeenCalledWith(stageIds.issue, stageIds.candidate1, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    await expect(f.client.fetchQuery(options)).rejects.toBeDefined();
    expect(f.client.getQueryData(options.queryKey)).toBeUndefined();
  });
});
