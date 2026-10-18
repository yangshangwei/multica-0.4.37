// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationResources, parseCollaborationResourceReceipt, parseCollaborationResourceIdentity, parseCollaborationResourceOptions } from "./resource-schema";
import { resourcesFixture, resourceIdentityFixture, resourceReceiptFixture, resourceOptionsFixture } from "./resource-test-fixtures";
afterEach(() => setSchemaLogger(noopLogger));

it("requires complete unique resource rosters and paired owner grant facts", () => {
  expect(parseCollaborationResources(resourcesFixture)?.targets[0]?.config_version).toBe("9007199254740993");
  const invalid = structuredClone(resourcesFixture);
  invalid.targets[0]!.resources.push(invalid.targets[0]!.resources[0]!);
  expect(parseCollaborationResources(invalid)).toBeNull();
  expect(parseCollaborationResources({ ...resourcesFixture, can_organize: undefined })).toBeNull();
  expect(parseCollaborationResourceOptions(resourceOptionsFixture)?.items).toHaveLength(1);
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
