// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationResourceOffer, parseCollaborationResourceOffersPage } from "./resource-offer-schema";
import { resourceOfferFixture as offer, resourceOffersPageFixture as page } from "./resource-offer-test-fixtures";

afterEach(() => setSchemaLogger(noopLogger));
const otherId = "00000000-0000-4000-8000-000000000092";

it("accepts an explicitly offered resource and strips undisclosed authority from every summary", () => {
  expect(parseCollaborationResourceOffersPage(page)).toEqual(page);
  const withPrivateFields = { ...offer, configuration_fingerprint: "secret", resource_authority: [{ installation_id: otherId }],
    resources: offer.resources.map((resource) => ({ ...resource, identity: { binding_id: otherId }, grant_id: otherId, owner: { ...resource.owner, email: "secret" } })) };
  expect(parseCollaborationResourceOffer(withPrivateFields)).toEqual(offer);
  expect(parseCollaborationResourceOffer({ ...offer, available: false, reason: "stale_offer", can_adopt: false })?.can_withdraw).toBe(true);
});

it("requires complete state facts and refuses terminal offers that claim new actions", () => {
  const withdrawn = { ...offer, version: "2", state: "withdrawn", withdrawn_at: "2026-10-21T02:00:00Z", available: false, reason: "offer_withdrawn", can_adopt: false, can_withdraw: false };
  const adopted = { ...offer, version: "2", state: "adopted", adopted_at: "2026-10-21T02:00:00Z", adopted_by: offer.offered_by,
    target_id: otherId, available: false, reason: "offer_adopted", can_adopt: false, can_withdraw: false };
  expect(parseCollaborationResourceOffer(withdrawn)).toEqual(withdrawn);
  expect(parseCollaborationResourceOffer(adopted)).toEqual(adopted);
  for (const invalid of [
    { ...offer, state: "unknown" }, { ...offer, available: false }, { ...offer, reason: "stale_offer" },
    { ...offer, adopted_at: adopted.adopted_at }, { ...offer, target_id: otherId }, { ...offer, withdrawn_at: withdrawn.withdrawn_at },
    { ...withdrawn, withdrawn_at: null }, { ...withdrawn, can_withdraw: true }, { ...withdrawn, can_adopt: true },
    { ...withdrawn, available: true, reason: "" }, { ...adopted, adopted_by: null }, { ...adopted, adopted_at: null },
    { ...adopted, target_id: null }, { ...adopted, withdrawn_at: withdrawn.withdrawn_at }, { ...adopted, can_withdraw: true },
    { ...offer, created_at: "tomorrow" }, { ...offer, version: 1 }, { ...offer, version: "0" }, { ...offer, protocol_version: 2 },
  ]) expect(parseCollaborationResourceOffer(invalid)).toBeNull();
});

it("validates frozen roster, owner and runtime identities without exposing a partial squad", () => {
  const member = { ...offer.resources[0]!, role: "member", agent_id: otherId };
  const squad = { ...offer, kind: "squad", resource_id: otherId, resources: [{ ...offer.resources[0]!, role: "leader" }, member] };
  expect(parseCollaborationResourceOffer(squad)?.resources).toHaveLength(2);
  for (const invalid of [
    { ...offer, resource_id: otherId }, { ...offer, execution_workspace_id: otherId }, { ...offer, resources: [] },
    { ...offer, resources: [...offer.resources, ...offer.resources] }, { ...squad, resources: [member] },
    { ...squad, resources: [offer.resources[0]!, member] }, { ...offer, offered_by: { ...offer.offered_by, user_id: otherId } },
    { ...offer, resources: [{ ...offer.resources[0], runtime: { ...offer.resources[0]!.runtime, id: otherId } }] },
    { ...offer, resources: [{ ...offer.resources[0], owner: { ...offer.offered_by, user_id: otherId } }] },
  ]) expect(parseCollaborationResourceOffer(invalid)).toBeNull();
});

it("rejects mismatched and duplicate page identities, malformed cursors and absent fields", () => {
  for (const invalid of [
    { ...page, items: [{ ...offer, issue_id: otherId }] }, { ...page, items: [offer, offer] },
    { ...page, lifecycle_epoch: 1 }, { ...page, target_set_version: "9223372036854775808" },
    { ...page, can_offer: undefined }, { ...page, next_cursor: undefined }, { ...page, next_cursor: "" },
    { ...page, next_cursor: "invalid cursor" }, { ...page, next_cursor: "a".repeat(513) },
    { ...page, items: [], next_cursor: "aabb" },
  ]) expect(parseCollaborationResourceOffersPage(invalid)).toBeNull();
  expect(parseCollaborationResourceOffersPage({ ...page, items: [offer, { ...offer, offer_id: otherId }] })?.items).toHaveLength(2);
});

it("redacts malformed resource values from diagnostics", () => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  parseCollaborationResourceOffer({ ...offer, offer_id: "private-offer-sentinel" });
  expect(warn).toHaveBeenCalledOnce();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("private-offer-sentinel");
});
