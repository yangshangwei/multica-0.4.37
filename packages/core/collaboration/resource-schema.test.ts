// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { CollaborationResourceInputSchema, collaborationResourceRequest, matchesCollaborationResourceReceipt, parseCollaborationResources, parseCollaborationResourceReceipt, parseCollaborationResourceIdentity, parseCollaborationResourceOptions } from "./resource-schema";
import { resourcesFixture, resourceIdentityFixture, resourceReceiptFixture, resourceOptionsFixture } from "./resource-test-fixtures";
import { parseCollaborationResourceOffer } from "./resource-offer-schema";
import { resourceOfferFixture } from "./resource-offer-test-fixtures";
import { parseCollaborationSwitchProposal } from "./switch-schema";
import { switchProposal } from "./switch-test-fixtures";
afterEach(() => setSchemaLogger(noopLogger));

it("requires complete unique resource rosters and paired owner grant facts", () => {
  expect(parseCollaborationResources(resourcesFixture)?.targets[0]?.config_version).toBe("9007199254740993");
  const invalid = structuredClone(resourcesFixture);
  invalid.targets[0]!.resources.push(invalid.targets[0]!.resources[0]!);
  expect(parseCollaborationResources(invalid)).toBeNull();
  expect(parseCollaborationResources({ ...resourcesFixture, can_organize: undefined })).toBeNull();
  expect(parseCollaborationResourceOptions(resourceOptionsFixture)?.items).toHaveLength(1);
});

it("rejects partial host metadata across resource, offer and Switch reads", () => {
  const metadata = { execution_mode: "host_process_supervision_v1", agent_owner_user_id: resourceIdentityFixture.issue_id,
    provider: "codex", provider_version: "0.118.0", account_source: "runtime_configuration" };
  const parse = (fields: object) => [
    parseCollaborationResources({ ...resourcesFixture, targets: [{ ...resourcesFixture.targets[0], resources: [{ ...resourcesFixture.targets[0]!.resources[0], ...fields }] }] }),
    parseCollaborationResourceOffer({ ...resourceOfferFixture, resources: [{ ...resourceOfferFixture.resources[0], ...fields }] }),
    parseCollaborationSwitchProposal({ ...switchProposal, resources: switchProposal.resources.map((resource) => ({ ...resource, ...fields })) }),
  ];
  expect(parse(metadata).every((value) => value !== null)).toBe(true);
  for (const patch of [
    { execution_mode: undefined }, { execution_mode: "linux_pid_namespace_v1" }, { provider: undefined },
    { provider_version: undefined }, { provider_version: null }, { account_source: undefined },
  ]) expect(parse({ ...metadata, ...patch })).toEqual([null, null, null]);
  expect(parse({}).every((value) => value !== null)).toBe(true);
});

it("keeps binding epochs exact and rejects an unregistered identity claiming readiness", () => {
  expect(parseCollaborationResourceIdentity(resourceIdentityFixture)?.identity?.binding_epoch).toBe("9007199254740993");
  expect(parseCollaborationResourceIdentity({ ...resourceIdentityFixture, identity: { ...resourceIdentityFixture.identity, binding_epoch: 1 } })).toBeNull();
  expect(parseCollaborationResourceIdentity({ ...resourceIdentityFixture, identity: null, reason: "" })).toBeNull();
  expect(parseCollaborationResourceIdentity({ ...resourceIdentityFixture, identity: null, reason: "runtime_registration_required" })?.identity).toBeNull();
});

it("accepts only action-consistent receipts and exact redacted outcomes", () => {
  expect(parseCollaborationResourceReceipt(resourceReceiptFixture)?.code).toBe("resource_consented");
  expect(parseCollaborationResourceReceipt({ ...resourceReceiptFixture, grant_id: null })).toBeNull();
  expect(parseCollaborationResourceReceipt({ ...resourceReceiptFixture, action: "register_target" })).toBeNull();
  const { protocol_version, issue_id, request_id, action, status, code } = resourceReceiptFixture;
  const redacted = { redacted: true, protocol_version, issue_id, request_id, action, status, code };
  expect(parseCollaborationResourceReceipt(redacted)?.redacted).toBe(true);
  expect(parseCollaborationResourceReceipt({ ...redacted, grant_id: null })).toBeNull();
});

it("redacts malformed owner binding data from schema diagnostics", () => {
  const spy = vi.fn();
  setSchemaLogger({ ...noopLogger, warn: spy });
  parseCollaborationResourceIdentity({ ...resourceIdentityFixture, identity: { ...resourceIdentityFixture.identity, binding_epoch: "private-binding-sentinel" } });
  expect(spy).toHaveBeenCalledOnce();
  expect(JSON.stringify(spy.mock.calls)).not.toContain("private-binding-sentinel");
});

it.each(["resource_owner_not_participant", "resource_owner_unavailable"])("recognizes a durable %s refusal", (code) => {
  expect(parseCollaborationResourceReceipt({ ...resourceReceiptFixture, action: "register_target", target_id: null, grant_id: null, scope: "issue", status: 409, code })?.code).toBe(code);
});

const offerId = "00000000-0000-4000-8000-000000000091";
const authority = { expected_lifecycle_epoch: "1", expected_participant_epoch: "2", expected_account_epoch: "3" };
const offerActions = [
  { action: "offer_resource", code: "resource_offered", status: 201, scope: "issue", target_id: null, offer_version: "1" },
  { action: "withdraw_resource_offer", code: "resource_offer_withdrawn", status: 200, scope: "issue", target_id: null, offer_version: "2" },
  { action: "adopt_resource_offer", code: "resource_offer_adopted", status: 201, scope: "target", target_id: resourceIdentityFixture.target_id, offer_version: "2" },
] as const;

it.each(offerActions)("validates the exact $action outcome without granting or resuming", (outcome) => {
  const receipt = { ...resourceReceiptFixture, ...outcome, grant_id: null, offer_id: offerId };
  expect(parseCollaborationResourceReceipt(receipt)).toMatchObject(outcome);
  for (const invalid of [
    { offer_id: null }, { offer_id: undefined }, { offer_version: undefined }, { offer_version: 1 },
    { offer_version: outcome.offer_version === "1" ? "2" : "1" }, { scope: outcome.scope === "issue" ? "target" : "issue" },
    { status: outcome.status === 201 ? 200 : 201 }, { code: "resource_consented" }, { grant_id: resourceReceiptFixture.grant_id },
  ]) expect(parseCollaborationResourceReceipt({ ...receipt, ...invalid })).toBeNull();
  const { protocol_version, issue_id, request_id, action, status, code } = receipt;
  const redacted = { redacted: true, protocol_version, issue_id, request_id, action, status, code };
  expect(parseCollaborationResourceReceipt(redacted)).toEqual(redacted);
  expect(parseCollaborationResourceReceipt({ ...redacted, offer_id: null })).toBeNull();
  expect(parseCollaborationResourceReceipt({ ...redacted, offer_version: "0" })).toBeNull();
});

it("requires frozen offer identity and version on adopt/withdraw requests and responses", () => {
  for (const action of ["adopt_resource_offer", "withdraw_resource_offer"] as const) {
    const input = CollaborationResourceInputSchema.parse({ ...authority, action, offer_id: offerId, expected_offer_version: "1",
      ...(action === "adopt_resource_offer" ? { expected_target_set_version: "9007199254740993" } : {}) });
    const receipt = parseCollaborationResourceReceipt({ ...resourceReceiptFixture, ...offerActions.find((item) => item.action === action), grant_id: null, offer_id: offerId })!;
    if (receipt.redacted === true || input.action !== "adopt_resource_offer" && input.action !== "withdraw_resource_offer") throw new Error("Expected a full offer receipt and offer intent");
    expect(matchesCollaborationResourceReceipt(receipt, resourceReceiptFixture.issue_id, resourceReceiptFixture.request_id, input)).toBe(true);
    expect(matchesCollaborationResourceReceipt({ ...receipt, offer_id: resourceReceiptFixture.issue_id }, resourceReceiptFixture.issue_id, resourceReceiptFixture.request_id, input)).toBe(false);
    expect(matchesCollaborationResourceReceipt(receipt, resourceReceiptFixture.issue_id, resourceReceiptFixture.request_id, { ...input, expected_offer_version: "9007199254740993" })).toBe(false);
    const request = collaborationResourceRequest(resourceReceiptFixture.issue_id, input);
    expect(request.path).toBe(`/api/collaboration/issues/${resourceReceiptFixture.issue_id}/resources/offers/${offerId}/${action === "adopt_resource_offer" ? "adopt" : "withdraw"}`);
    expect(request.body).toEqual({ ...authority, expected_offer_version: "1", ...(action === "adopt_resource_offer" ? { expected_target_set_version: "9007199254740993" } : {}) });
  }
});

it("supplies a complete owned roster without consent identity or a target-set observation", () => {
  const candidate = resourceOptionsFixture.items[0]!;
  const roster = candidate.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }));
  const input = { ...authority, action: "offer_resource", kind: candidate.kind, resource_id: candidate.resource_id,
    execution_workspace_id: resourceOptionsFixture.execution_workspace_id, roster };
  const parsed = CollaborationResourceInputSchema.parse(input);
  expect(collaborationResourceRequest(resourceReceiptFixture.issue_id, parsed)).toEqual({
    path: `/api/collaboration/issues/${resourceReceiptFixture.issue_id}/resources/offers`,
    body: { ...authority, kind: input.kind, resource_id: input.resource_id, execution_workspace_id: input.execution_workspace_id, roster },
  });
  expect(CollaborationResourceInputSchema.safeParse({ ...input, expected_target_set_version: "1" }).success).toBe(false);
  expect(CollaborationResourceInputSchema.safeParse({ ...input, identity: resourceIdentityFixture.identity }).success).toBe(false);
  expect(CollaborationResourceInputSchema.safeParse({ ...input, roster: [...roster, ...roster] }).success).toBe(false);
  expect(CollaborationResourceInputSchema.safeParse({ ...input, execution_workspace_id: offerId }).success).toBe(false);
});
