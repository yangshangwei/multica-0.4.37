// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope, clearCollaborationAccess } from "./access";
import { parseCollaborationIssue } from "./schema";
import { readCollaborationDrafts } from "./draft";
import { prepareCollaborationResource, executeCollaborationResource, collaborationResourceOptionsOptions, collaborationResourcesOptions } from "./resource";
import { parseCollaborationResourceIdentity, parseCollaborationResources } from "./resource-schema";
import { resourceIssue, resourcesFixture, resourceReceiptFixture, resourceIdentityFixture } from "./resource-test-fixtures";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const stored = new Map<string, string>();
const storage = { keys: () => [...stored.keys()], getItem: (key: string) => stored.get(key) ?? null, setItem: (key: string, value: string) => { stored.set(key, value); }, removeItem: (key: string) => { stored.delete(key); } };
const client = new QueryClient();
afterEach(() => { client.clear(); stored.clear(); vi.unstubAllGlobals(); });
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(auth.user.id);
  const issue = parseCollaborationIssue(resourceIssue)!;
  const resources = parseCollaborationResources(resourcesFixture)!;
  const selection = { action: "consent_resource" as const, targetId: resources.targets[0]!.target_id, agentId: resourceIdentityFixture.agent_id,
    identity: parseCollaborationResourceIdentity(resourceIdentityFixture)! };
  const draft = prepareCollaborationResource(scope, issue, resources, resourceReceiptFixture.request_id, selection);
  return { scope, issue, resources, selection, draft };
}

it("checks current ownership and records the observed target and binding versions", () => {
  const { scope, issue, resources, selection, draft } = setup();
  expect(draft.payload).toMatchObject({ domain: "target_resource", input: { expected_config_version: "9007199254740993", identity: { binding_epoch: "9007199254740993" } } });
  expect(() => prepareCollaborationResource(scope, { ...issue, capabilities: { ...issue.capabilities, control_ai: false } }, resources, draft.requestId, selection)).toThrow();
  const otherOwner = structuredClone(resources);
  otherOwner.targets[0]!.resources[0]!.owner_user_id = "00000000-0000-4000-8000-000000000002";
  expect(() => prepareCollaborationResource(scope, issue, otherOwner, draft.requestId, selection)).toThrow();
  expect(() => prepareCollaborationResource(scope, issue, { ...resources, target_set_version: "1" }, draft.requestId, selection)).toThrow();
});

it("preserves an unknown resource write and recovers by GET without replacement versions", async () => {
  const { scope, draft } = setup();
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("lost response"))
    .mockResolvedValueOnce(json({ code: "operation_not_found" }, 404))
    .mockResolvedValueOnce(json(resourceReceiptFixture));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationResource(client, scope, draft, false, { storage })).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationResource(client, scope, draft, true, { storage })).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationResource(client, scope, draft, true, { storage })).resolves.toMatchObject({ code: "resource_consented" });
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["POST", "GET", "GET"]);
  expect(fetch.mock.calls[2]?.[0]).toContain(`/resources/operations/${draft.requestId}`);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("keeps optional workspace candidate denial local while an issue denial clears protected data", async () => {
  const { scope, issue } = setup();
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, issue);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ code: "resource_workspace_unavailable" }, 404)).mockResolvedValueOnce(json({ code: "not_found" }, 404)));
  await expect(client.fetchQuery(collaborationResourceOptionsOptions(client, scope, issue.id, issue.source_workspace.id))).rejects.toThrow();
  expect(client.getQueryData(queryKey)).toEqual(issue);
  // Access loss cancels in-flight Query promises before clearing cached content.
  await expect(client.fetchQuery(collaborationResourcesOptions(client, scope, issue.id))).rejects.toThrow();
  expect(client.getQueryData(queryKey)).toBeUndefined();
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(true);
});

it("accepts only a content-free own receipt once access is lost", async () => {
  const { scope, issue, draft } = setup();
  clearCollaborationAccess(client, scope, issue.id);
  const { protocol_version, issue_id, request_id, action, status, code } = resourceReceiptFixture;
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ redacted: true, protocol_version, issue_id, request_id, action, status, code })));
  await expect(executeCollaborationResource(client, scope, draft, true, { storage })).resolves.toMatchObject({ redacted: true });
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toBeUndefined();
});
