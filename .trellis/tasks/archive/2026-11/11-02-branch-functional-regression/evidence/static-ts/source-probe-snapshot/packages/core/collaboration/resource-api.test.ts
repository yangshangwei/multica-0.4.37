// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { resourceIssue, resourcesFixture, resourceOptionsFixture, resourceIdentityFixture } from "./resource-test-fixtures";
import { resourceOfferFixture, resourceOffersPageFixture } from "./resource-offer-test-fixtures";

afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); });
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
it("reads management scopes without ambient workspace headers and rejects crossed response identities", async () => {
  const api = new ApiClient("https://server.example");
  setCurrentWorkspace("unrelated", "00000000-0000-4000-8000-000000000077");
  const participants = { protocol_version: 1, issue_id: resourceIssue.id, collaboration_revision: resourceIssue.collaboration_revision,
    participants: [{ ...resourceIssue.creator, account_active: false }] };
  const calls = [
    { run: () => api.getCollaborationResources(resourceIssue.id), response: resourcesFixture, suffix: "/resources" },
    { run: () => api.getCollaborationResourceOptions(resourceIssue.id, resourceIssue.source_workspace.id), response: resourceOptionsFixture, suffix: "/resources/options?execution_workspace_id=" },
    { run: () => api.getCollaborationResourceIdentity(resourceIssue.id, resourceIdentityFixture.target_id, resourceIdentityFixture.agent_id, null), response: resourceIdentityFixture, suffix: "/resources/identity?target_id=" },
    { run: () => api.getCollaborationParticipants(resourceIssue.id), response: participants, suffix: "/participants" },
    { run: () => api.getCollaborationResourceOfferOptions(resourceIssue.id, resourceIssue.source_workspace.id), response: resourceOptionsFixture, suffix: "/resources/offers/options?execution_workspace_id=" },
    { run: () => api.listCollaborationResourceOffers(resourceIssue.id), response: resourceOffersPageFixture, suffix: "/resources/offers?limit=50" },
    { run: () => api.getCollaborationResourceOffer(resourceIssue.id, resourceOfferFixture.offer_id), response: resourceOfferFixture, suffix: `/resources/offers/${resourceOfferFixture.offer_id}` },
  ];
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  for (const item of calls) {
    fetch.mockResolvedValueOnce(json(item.response));
    await expect(item.run()).resolves.toEqual(item.response);
    const request = fetch.mock.calls.at(-1)!;
    expect(request[0]).toContain(item.suffix);
    const headers = new Headers(request[1].headers);
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
    fetch.mockResolvedValueOnce(json({ ...item.response, issue_id: "00000000-0000-4000-8000-000000000077" }));
    await expect(item.run()).resolves.toBeNull();
  }
});

it("isolates missing workspace or runtime scope without masking a real issue denial", async () => {
  const api = new ApiClient("https://server.example");
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "resource_workspace_unavailable" }, 404))
    .mockResolvedValueOnce(json({ code: "resource_not_found" }, 404)).mockResolvedValueOnce(json({ code: "not_found" }, 404));
  vi.stubGlobal("fetch", fetch);
  await expect(api.getCollaborationResourceOptions(resourceIssue.id, resourceIssue.source_workspace.id)).resolves.toBeNull();
  await expect(api.getCollaborationResourceIdentity(resourceIssue.id, resourceIdentityFixture.target_id, resourceIdentityFixture.agent_id, null)).resolves.toBeNull();
  await expect(api.getCollaborationResources(resourceIssue.id)).rejects.toMatchObject({ status: 404 });
});

it("checks offer, workspace and cursor identities before returning optional offer reads", async () => {
  const api = new ApiClient("https://server.example");
  const fetch = vi.fn().mockResolvedValueOnce(json({ ...resourceOfferFixture, offer_id: resourceIssue.id }))
    .mockResolvedValueOnce(json({ ...resourceOptionsFixture, execution_workspace_id: resourceOfferFixture.offer_id }))
    .mockResolvedValueOnce(json({ ...resourceOffersPageFixture, can_offer: undefined }))
    .mockResolvedValueOnce(json({ ...resourceOffersPageFixture, next_cursor: "aabb" }))
    .mockResolvedValueOnce(json(resourceOffersPageFixture));
  vi.stubGlobal("fetch", fetch);
  await expect(api.getCollaborationResourceOffer(resourceIssue.id, resourceOfferFixture.offer_id)).resolves.toBeNull();
  await expect(api.getCollaborationResourceOfferOptions(resourceIssue.id, resourceOptionsFixture.execution_workspace_id)).resolves.toBeNull();
  await expect(api.listCollaborationResourceOffers(resourceIssue.id)).resolves.toBeNull();
  await expect(api.listCollaborationResourceOffers(resourceIssue.id, "aabb")).resolves.toBeNull();
  await expect(api.listCollaborationResourceOffers(resourceIssue.id, "aabb")).resolves.toEqual(resourceOffersPageFixture);
  expect(fetch.mock.calls.at(-1)?.[0]).toContain("limit=50&cursor=aabb");
  await expect(api.listCollaborationResourceOffers(resourceIssue.id, "bad cursor")).rejects.toThrow();
  expect(fetch).toHaveBeenCalledTimes(5);
});

it.each(["missing", "old-server", "object-denied"] as const)("keeps the %s offer read boundary distinct", async (kind) => {
  const api = new ApiClient("https://server.example");
  const fetch = vi.fn().mockResolvedValue(json(kind === "missing" ? { code: "resource_offer_not_found" } : kind === "object-denied" ? { code: "not_found" } : {}, 404));
  vi.stubGlobal("fetch", fetch);
  const result = api.getCollaborationResourceOffer(resourceIssue.id, resourceOfferFixture.offer_id);
  if (kind === "object-denied") await expect(result).rejects.toMatchObject({ status: 404 });
  else await expect(result).resolves.toBeNull();
});

it.each([200, 404])("rejects an offer read's late HTTP %s after a credential change", async (status) => {
  const api = new ApiClient("https://server.example");
  api.setToken("test-only-first-account");
  let resolve!: (value: Response) => void;
  const fetch = vi.fn().mockReturnValue(new Promise<Response>((done) => { resolve = done; }));
  vi.stubGlobal("fetch", fetch);
  const result = api.getCollaborationResourceOffer(resourceIssue.id, resourceOfferFixture.offer_id);
  api.setToken("test-only-second-account");
  resolve(json(status === 200 ? resourceOfferFixture : { code: "not_found" }, status));
  await expect(result).rejects.toMatchObject({ name: "AbortError" });
});
