// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { clearCollaborationAccess, collaborationScope } from "./access";
import { expireKnowledgeProjections } from "./knowledge-cache";
import { knowledgeAutomationJobsOptions, projectKnowledgePolicyOptions } from "./knowledge-automation-queries";
import { knowledgeIds, knowledgeJobsFixture, knowledgePolicyFixture } from "./knowledge-test-fixtures";

vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const cleanups: (() => void)[] = [];
afterEach(() => { cleanups.splice(0).forEach((cleanup) => cleanup()); vi.restoreAllMocks(); });
function setup() {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const client = new QueryClient(), scope = collaborationScope(knowledgeIds.actor);
  cleanups.push(() => { expireKnowledgeProjections(client, scope, false); client.clear(); });
  return { api, client, scope };
}

it("routes policy and job metadata through the same leased knowledge erasure", async () => {
  const f = setup();
  vi.spyOn(f.api, "getProjectKnowledgePolicy").mockResolvedValue(knowledgePolicyFixture);
  vi.spyOn(f.api, "getCollaborationKnowledgeJobs").mockResolvedValue(knowledgeJobsFixture);
  const policy = projectKnowledgePolicyOptions(f.client, f.scope, knowledgeIds.workspace, knowledgeIds.project);
  const jobs = knowledgeAutomationJobsOptions(f.client, f.scope, knowledgeIds.issue);
  expect((await f.client.fetchQuery(policy)).value).toEqual(knowledgePolicyFixture);
  expect((await f.client.fetchQuery(jobs)).value).toEqual(knowledgeJobsFixture);
  expect(policy.queryKey).toContain(knowledgeIds.workspace);
  expireKnowledgeProjections(f.client, f.scope, false);
  expect(f.client.getQueryData(policy.queryKey)).toBeUndefined();
  expect(f.client.getQueryData(jobs.queryKey)).toBeUndefined();
});

it("does not restore late job metadata after source access is revoked", async () => {
  const f = setup();
  let resolve!: (value: typeof knowledgeJobsFixture) => void;
  vi.spyOn(f.api, "getCollaborationKnowledgeJobs").mockImplementation(() => new Promise((done) => { resolve = done; }));
  const jobs = knowledgeAutomationJobsOptions(f.client, f.scope, knowledgeIds.issue);
  const read = f.client.fetchQuery(jobs);
  clearCollaborationAccess(f.client, f.scope, knowledgeIds.issue);
  resolve(knowledgeJobsFixture);
  await expect(read).rejects.toBeDefined();
  expect(f.client.getQueryData(jobs.queryKey)).toBeUndefined();
});
