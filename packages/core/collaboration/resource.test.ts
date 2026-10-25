// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationAccessEpoch, collaborationIssueAccessDenied, collaborationKeys, collaborationScope, clearCollaborationAccess, protectCollaborationRead } from "./access";
import { parseCollaborationIssue } from "./schema";
import { readCollaborationDrafts, saveCollaborationDraft } from "./draft";
import { canOfferCollaborationResources, canOrganizeCollaborationResources, prepareCollaborationResource, executeCollaborationResource, parseCollaborationResourceDraft,
  collaborationResourceOptionsOptions, collaborationResourcesOptions, collaborationResourceOffersOptions, collaborationResourceOfferOptions, collaborationResourceOfferCandidatesOptions } from "./resource";
import { parseCollaborationResourceIdentity, parseCollaborationResources, parseCollaborationResourceOptions } from "./resource-schema";
import { parseCollaborationResourceOffer, parseCollaborationResourceOffersPage } from "./resource-offer-schema";
import { resourceOfferFixture, resourceOffersPageFixture } from "./resource-offer-test-fixtures";
import { resourceIssue, resourcesFixture, resourceReceiptFixture, resourceIdentityFixture, resourceOptionsFixture } from "./resource-test-fixtures";
import { collaborationHostExecutionFixture } from "./test-fixtures";

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

it("refuses v6 consent when all execution metadata is absent without breaking v5 consent", () => {
  const { scope, issue, resources, selection, draft } = setup();
  const hostIssue = parseCollaborationIssue({ ...issue, execution: collaborationHostExecutionFixture })!;
  expect(() => prepareCollaborationResource(scope, hostIssue, resources, draft.requestId, selection)).toThrow();
  expect(parseCollaborationResourceDraft(prepareCollaborationResource(scope, issue, resources, draft.requestId, selection)).execution_view_version).toBeUndefined();
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

function offerSetup() {
  const base = setup();
  const offers = parseCollaborationResourceOffersPage(resourceOffersPageFixture)!;
  const offer = parseCollaborationResourceOffer(resourceOfferFixture)!;
  const candidate = parseCollaborationResourceOptions(resourceOptionsFixture)!.items[0]!;
  const selection = { action: "offer_resource" as const, candidate, executionWorkspaceId: resourceOptionsFixture.execution_workspace_id, offers };
  return { ...base, offers, offer, candidate, selection };
}

it("allows a source-hidden participating owner to offer without organization or submit-command capability", () => {
  const { scope, issue, resources, selection, draft } = offerSetup();
  const participantIssue = { ...issue, creator: { ...issue.creator, user_id: resourceIdentityFixture.agent_id }, handler: { ...issue.handler, user_id: resourceIdentityFixture.agent_id },
    capabilities: { ...issue.capabilities, invite: false } };
  expect(participantIssue.capabilities.submit_command).toBe(false);
  expect(canOfferCollaborationResources(scope, participantIssue)).toBe(true);
  expect(canOrganizeCollaborationResources(scope, participantIssue)).toBe(false);
  const offered = prepareCollaborationResource(scope, participantIssue, { ...resources, can_organize: false, target_set_version: "1" }, draft.requestId, selection);
  expect(parseCollaborationResourceDraft(offered)).toMatchObject({ action: "offer_resource", expected_participant_epoch: "1", execution_workspace_id: selection.executionWorkspaceId });
  expect(JSON.stringify(offered.payload)).not.toMatch(/target_set_version|grant_id|binding_id|agent_name/);
  for (const invalid of [
    { ...selection, executionWorkspaceId: resourceIdentityFixture.agent_id },
    { ...selection, offers: { ...selection.offers, issue_id: resourceIdentityFixture.agent_id } },
    { ...selection, offers: { ...selection.offers, lifecycle_epoch: "2" } },
    { ...selection, offers: { ...selection.offers, can_offer: false } },
    { ...selection, candidate: { ...selection.candidate, available: false } },
    { ...selection, candidate: { ...selection.candidate, resources: selection.candidate.resources.map((resource) => ({ ...resource, owner_user_id: resourceIdentityFixture.agent_id })) } },
  ]) expect(() => prepareCollaborationResource(scope, participantIssue, resources, draft.requestId, invalid)).toThrow();
});

it("freezes an organizer's adoption and lets the supplying owner withdraw a stale offer on a closed issue", () => {
  const { scope, issue, resources, offer, draft, selection } = offerSetup();
  const adopted = prepareCollaborationResource(scope, issue, resources, draft.requestId, { action: "adopt_resource_offer", offer });
  expect(parseCollaborationResourceDraft(adopted)).toEqual({ action: "adopt_resource_offer", expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: "1", expected_account_epoch: "1", offer_id: offer.offer_id, expected_offer_version: "1", expected_target_set_version: resources.target_set_version });
  expect(() => prepareCollaborationResource(scope, { ...issue, capabilities: { ...issue.capabilities, invite: false } }, resources, draft.requestId, { action: "adopt_resource_offer", offer })).toThrow();
  expect(() => prepareCollaborationResource(scope, issue, resources, draft.requestId, { action: "adopt_resource_offer", offer: { ...offer, can_adopt: false } })).toThrow();
  const closed = { ...issue, status_category: "done" as const, session: { ...issue.session, queue_state: "closed" as const } };
  const withdrawn = prepareCollaborationResource(scope, closed, { ...resources, target_set_version: "1" }, draft.requestId,
    { action: "withdraw_resource_offer", offer: { ...offer, available: false, reason: "stale_offer", can_adopt: false } });
  expect(parseCollaborationResourceDraft(withdrawn)).toEqual({ action: "withdraw_resource_offer", expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: "1", expected_account_epoch: "1", offer_id: offer.offer_id, expected_offer_version: "1" });
  expect(() => prepareCollaborationResource(scope, issue, resources, draft.requestId, { action: "withdraw_resource_offer", offer: { ...offer, offered_by: issue.handler, can_withdraw: false } })).toThrow();
  expect(() => prepareCollaborationResource(scope, closed, resources, draft.requestId, selection)).toThrow();
});

it("keeps an adoption's original key across a lost response and a crossed-offer recovery", async () => {
  const { scope, issue, resources, offer } = offerSetup();
  const draft = prepareCollaborationResource(scope, issue, resources, resourceReceiptFixture.request_id, { action: "adopt_resource_offer", offer });
  const receipt = { ...resourceReceiptFixture, action: "adopt_resource_offer", code: "resource_offer_adopted", offer_id: offer.offer_id, offer_version: "2", grant_id: null };
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("lost response"))
    .mockResolvedValueOnce(json({ ...receipt, offer_id: issue.id })).mockResolvedValueOnce(json(receipt));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationResource(client, scope, draft, false, { storage })).rejects.toThrow();
  await expect(executeCollaborationResource(client, scope, draft, true, { storage })).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationResource(client, scope, draft, true, { storage })).resolves.toMatchObject({ offer_id: offer.offer_id, offer_version: "2" });
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["POST", "GET", "GET"]);
  expect(JSON.parse(fetch.mock.calls[0]![1].body)).toMatchObject({ expected_offer_version: "1", expected_target_set_version: resources.target_set_version });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("settles a correlated stale-offer refusal without revising the intent or hiding the issue", async () => {
  const { scope, issue, resources, offer } = offerSetup();
  const draft = prepareCollaborationResource(scope, issue, resources, resourceReceiptFixture.request_id, { action: "withdraw_resource_offer", offer });
  client.setQueryData(collaborationKeys.issue(scope, issue.id), issue);
  const fetch = vi.fn().mockResolvedValue(json({ ...resourceReceiptFixture, action: "withdraw_resource_offer", status: 409, code: "stale_offer", scope: "issue", target_id: null,
    grant_id: null, offer_id: offer.offer_id, offer_version: "2" }, 409));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationResource(client, scope, draft, false, { storage })).resolves.toMatchObject({ code: "stale_offer" });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(parseCollaborationResourceDraft(draft)).toMatchObject({ expected_offer_version: "1" });
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toEqual(issue);
  expect(fetch).toHaveBeenCalledOnce();
});

it.each([false, true])("settles a durable offer visibility refusal through its original request (recovery=%s)", async (recovery) => {
  const { scope, issue, resources, selection } = offerSetup();
  const draft = prepareCollaborationResource(scope, issue, resources, resourceReceiptFixture.request_id, selection);
  const issueKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(issueKey, issue);
  const receipt = { ...resourceReceiptFixture, action: "offer_resource", status: 404, code: "resource_options_unavailable", scope: "issue", target_id: null, grant_id: null };
  const fetch = vi.fn();
  if (recovery) fetch.mockRejectedValueOnce(new TypeError("lost response"));
  fetch.mockResolvedValueOnce(json(receipt, recovery ? 200 : 404));
  vi.stubGlobal("fetch", fetch);
  if (recovery) {
    await expect(executeCollaborationResource(client, scope, draft, false, { storage })).rejects.toThrow();
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  }
  await expect(executeCollaborationResource(client, scope, draft, recovery, { storage })).resolves.toMatchObject({ status: 404, code: "resource_options_unavailable" });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(client.getQueryData(issueKey)).toEqual(issue);
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(false);
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(recovery ? ["POST", "GET"] : ["POST"]);
  expect(parseCollaborationResourceDraft(draft)).toMatchObject({ action: "offer_resource", expected_participant_epoch: "1" });
});

it("scopes offer queries and erases only the denied issue's offer data", async () => {
  const { scope, issue, offer, offers } = offerSetup();
  const pageKey = collaborationResourceOffersOptions(client, scope, issue.id).queryKey;
  const detailKey = collaborationResourceOfferOptions(client, scope, issue.id, offer.offer_id).queryKey;
  const candidateKey = collaborationResourceOfferCandidatesOptions(client, scope, issue.id, offer.execution_workspace_id).queryKey;
  const unrelated = collaborationResourceOffersOptions(client, scope, offer.offer_id).queryKey;
  const otherPage = { ...offers, issue_id: offer.offer_id, items: [] };
  client.setQueryData(pageKey, offers);
  client.setQueryData(detailKey, offer);
  client.setQueryData(candidateKey, parseCollaborationResourceOptions(resourceOptionsFixture)!);
  client.setQueryData(unrelated, otherPage);
  client.setQueryData(collaborationKeys.issue(scope, issue.id), issue);
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "resource_offer_not_found" }, 404)).mockResolvedValueOnce(json({ code: "not_found" }, 404));
  vi.stubGlobal("fetch", fetch);
  await expect(client.fetchQuery(collaborationResourceOfferOptions(client, scope, issue.id, offer.offer_id))).rejects.toThrow();
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(false);
  await expect(client.fetchQuery(collaborationResourceOffersOptions(client, scope, issue.id))).rejects.toThrow();
  for (const key of [pageKey, detailKey, candidateKey]) expect(client.getQueryData(key)).toBeUndefined();
  expect(client.getQueryData(unrelated)).toEqual(otherPage);
});

it.each(["success", "denial"] as const)("fences a late offer %s without retiring a newer draft after authority recovers", async (outcome) => {
  const { scope, issue, resources, selection, offer } = offerSetup();
  const draft = prepareCollaborationResource(scope, issue, resources, resourceReceiptFixture.request_id, selection);
  let resolve!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((done) => { resolve = done; })));
  const pending = executeCollaborationResource(client, scope, draft, false, { storage });
  clearCollaborationAccess(client, scope, issue.id);
  await protectCollaborationRead(client, scope, issue.id, async () => issue, true);
  const epoch = collaborationAccessEpoch(client, scope, issue.id);
  const fresh = prepareCollaborationResource(scope, issue, resources, offer.offer_id, selection);
  saveCollaborationDraft(fresh, storage);
  client.setQueryData(collaborationKeys.issue(scope, issue.id), issue);
  resolve(outcome === "denial" ? json({ code: "not_found" }, 404) : json({ ...resourceReceiptFixture, action: "offer_resource", code: "resource_offered", scope: "issue", target_id: null,
    grant_id: null, offer_id: offer.offer_id, offer_version: "1" }, 201));
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  expect(collaborationAccessEpoch(client, scope, issue.id)).toBe(epoch);
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toEqual(issue);
  expect(readCollaborationDrafts(fresh.scope, storage)).toContainEqual(fresh);
});
