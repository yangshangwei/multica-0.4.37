// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { parseCollaborationCommentReceipt } from "./content-schema";
import { collaborationExecutionIssueFixture as issue } from "./test-fixtures";
import { collaborationRunRecordsFixture as records } from "./run-records-test-fixtures";
import { resourceIdentityFixture, resourceOptionsFixture, resourcesFixture } from "./resource-test-fixtures";
import { resourceOfferFixture, resourceOffersPageFixture } from "./resource-offer-test-fixtures";
import { switchConsentInput, switchIssueId, switchRequestId } from "./switch-test-fixtures";

afterEach(() => vi.unstubAllGlobals());

it("negotiates execution v6 on every execution read while accepting old-server v5", async () => {
  const mentions = { protocol_version: 2, issue_id: issue.id, participants: [], can_submit_commands: false, targets: [] };
  const fetch = vi.fn();
  for (const body of [issue, records, resourcesFixture, mentions]) fetch.mockResolvedValueOnce(new Response(JSON.stringify(body)));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  expect((await api.getCollaborationIssue(issue.id))?.execution?.protocol_version).toBe(5);
  expect(await api.getCollaborationRunRecords(records.issue_id, records.run.id)).not.toBeNull();
  expect(await api.getCollaborationResources(resourcesFixture.issue_id)).not.toBeNull();
  expect(await api.getCollaborationMentionTargets(issue.id)).not.toBeNull();
  expect(fetch.mock.calls.map(([url]) => new URL(url).searchParams.get("execution_view_version"))).toEqual(["6", "6", "6", "6"]);
});

it.each([
  { name: "records", code: "invalid_records_query", body: records, read: (api: ApiClient) => api.getCollaborationRunRecords(records.issue_id, records.run.id) },
  { name: "resource options", code: "invalid_resource_scope", body: resourceOptionsFixture, read: (api: ApiClient) => api.getCollaborationResourceOptions(resourceOptionsFixture.issue_id, resourceOptionsFixture.execution_workspace_id) },
  { name: "offer options", code: "invalid_resource_scope", body: resourceOptionsFixture, read: (api: ApiClient) => api.getCollaborationResourceOfferOptions(resourceOptionsFixture.issue_id, resourceOptionsFixture.execution_workspace_id) },
  { name: "resource identity", code: "invalid_resource_scope", body: resourceIdentityFixture, read: (api: ApiClient) => api.getCollaborationResourceIdentity(resourceIdentityFixture.issue_id, resourceIdentityFixture.target_id, resourceIdentityFixture.agent_id, null) },
  { name: "offer list", code: "invalid_resource_offer_query", body: resourceOffersPageFixture, read: (api: ApiClient) => api.listCollaborationResourceOffers(resourceOffersPageFixture.issue_id) },
  { name: "offer detail", code: "invalid_resource_scope", body: resourceOfferFixture, read: (api: ApiClient) => api.getCollaborationResourceOffer(resourceOfferFixture.issue_id, resourceOfferFixture.offer_id) },
])("reads $name from an actual legacy query contract without changing its scope", async ({ code, body, read }) => {
  // These are the exact refusal codes used by the pre-v6 handlers for an
  // additional query parameter; they do not ignore unknown query metadata.
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ code }), { status: 400 }))
    .mockResolvedValueOnce(new Response(JSON.stringify(body)));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  api.setToken("original-account");
  expect(await read(api)).not.toBeNull();
  expect(fetch).toHaveBeenCalledTimes(2);
  const first = new URL(fetch.mock.calls[0]![0]);
  const second = new URL(fetch.mock.calls[1]![0]);
  expect(first.searchParams.get("execution_view_version")).toBe("6");
  first.searchParams.delete("execution_view_version");
  expect(second.toString()).toBe(first.toString());
  for (const [, init] of fetch.mock.calls) {
    expect(init.method ?? "GET").toBe("GET");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer original-account");
  }
});

it.each([401, 403, 404, 500])("does not downgrade an execution read after HTTP %s", async (status) => {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: "invalid_records_query" }), { status }));
  vi.stubGlobal("fetch", fetch);
  await new ApiClient("https://server.example").getCollaborationRunRecords(records.issue_id, records.run.id).catch(() => null);
  expect(fetch).toHaveBeenCalledOnce();
});

it("retries a legacy query refusal once and never retries malformed success or a write", async () => {
  const api = new ApiClient("https://server.example");
  const fetch = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ code: "invalid_records_query" }), { status: 400 })));
  vi.stubGlobal("fetch", fetch);
  await expect(api.getCollaborationRunRecords(records.issue_id, records.run.id)).rejects.toThrow();
  expect(fetch).toHaveBeenCalledTimes(2);
  fetch.mockClear().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ ...records, run: { ...records.run, execution_mode: "host_process_supervision_v1" } }))));
  expect(await api.getCollaborationRunRecords(records.issue_id, records.run.id)).toBeNull();
  expect(fetch).toHaveBeenCalledOnce();
  fetch.mockClear().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ code: "invalid_resource_scope" }), { status: 400 })));
  await api.switchCollaborationTarget(switchIssueId, switchRequestId, { ...switchConsentInput, execution_view_version: 6 });
  expect(fetch).toHaveBeenCalledOnce();
  expect(JSON.parse(fetch.mock.calls[0]![1].body).execution_view_version).toBe(6);
});

it.each(["cancelled", "superseded"])("does not retry a legacy refusal after its read was %s", async (reason) => {
  const api = new ApiClient("https://server.example");
  const controller = new AbortController();
  const fetch = vi.fn().mockImplementation(async () => {
    if (reason === "cancelled") controller.abort();
    else api.invalidateSession();
    return new Response(JSON.stringify({ code: "invalid_resource_scope" }), { status: 400 });
  });
  vi.stubGlobal("fetch", fetch);
  await expect(api.getCollaborationResourceOptions(resourceOptionsFixture.issue_id, resourceOptionsFixture.execution_workspace_id, { signal: controller.signal })).rejects.toMatchObject({ name: "AbortError" });
  expect(fetch).toHaveBeenCalledOnce();
});

it("retains a mixed comment while only its incompatible host target is blocked", () => {
  const target = issue.execution.targets[0]!;
  const refused = { target_id: target.id, kind: target.kind, resource_id: target.resource_id, target_epoch: target.target_epoch,
    config_version: target.config_version, hold_reason: target.hold_reason, hold_version: target.hold_version,
    state: "blocked", code: "collaboration_client_upgrade_required", command_id: null, channel_id: null, batch_id: null };
  const raw = { status: 201, code: "comment_created", protocol_version: 2, issue_id: issue.id, comment_id: issue.id,
    comment_revision: "1", event_id: issue.id, event_seq: "1", targets: [refused] };
  expect(parseCollaborationCommentReceipt(raw)).toMatchObject({ status: 201, targets: [{ state: "blocked", code: "collaboration_client_upgrade_required", command_id: null }] });
  expect(parseCollaborationCommentReceipt({ ...raw, targets: [{ ...refused, state: "waiting", command_id: issue.id, command_sequence: "1" }] })).toBeNull();
});
