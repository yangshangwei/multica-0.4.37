// @vitest-environment node
import { expect, it } from "vitest";
import { parseAdminInstallationList, parseAdminInstallationDetail, parseAdminUnassociatedRuntimes } from "./installation-schemas";
const id = "11111111-1111-4111-8111-111111111111", date = "2026-10-01T00:00:00Z";
const axis = {
  state: "unknown", observed_at: null, source: "none", freshness: "unknown", reason_code: "missing"
};
const installation = {
  id, deployment_id: id, organization_id: id, lifecycle: "active", display_name: "Laptop", groups: [], desktop_version: null, os: null, admission: "accepting", admission_version: 1, created_at: date, updated_at: date, runtime_count: 2, binding_count: 1, client_activity: { ...axis, state: "inactive" }, daemon_reachability: { ...axis, state: "reachable" }, execution_readiness: axis
};
it("normalizes admission CAS versions and retains only explicit action metadata", () => {
  const base = { items: [installation], as_of: date, scope: id, next_cursor: null, data_quality: "complete" };
  expect(parseAdminInstallationList(base)?.items[0]).toMatchObject({ admissionVersion: "1", allowedActions: [] });
  expect(parseAdminInstallationList({ ...base, items: [{ ...installation, admission_version: "9223372036854775807", allowed_actions: ["stop_admission"] }] })?.items[0]).toMatchObject({ admissionVersion: "9223372036854775807", allowedActions: ["stop_admission"] });
  expect(parseAdminInstallationList({ ...base, items: [{ ...installation, admission_version: Number.MAX_SAFE_INTEGER + 1 }] })).toBeNull();
});
it("retains independent axes and strips secret/proof fields", () => {
  const parsed = parseAdminInstallationList({
    items: [{
      ...installation, public_key: "PRIVATE", metadata: { path: "/PRIVATE" }
    }], as_of: date, scope: id, next_cursor: null, data_quality: "complete"
  });
  expect(parsed?.items[0]).toMatchObject({
    runtimeCount: 2, clientActivity: { state: "inactive" }, daemonReachability: { state: "reachable" }, executionReadiness: { state: "unknown" }
  });
  expect(JSON.stringify(parsed)).not.toContain("PRIVATE");
});
it("keeps dependency failure distinct from offline and fails closed on malformed metadata", () => {
  const parsed = parseAdminInstallationDetail({
    installation: {
      ...installation, daemon_reachability: {
        ...axis, state: "unavailable", freshness: "unavailable"
      }
    }, runtimes: [], bindings: [], users: [], scope: id, as_of: date
  });
  expect(parsed?.installation.daemonReachability.state).toBe("unavailable");
  expect(parseAdminInstallationList({
    items: [{ id }], as_of: date, scope: id
  })).toBeNull();
});
it("shows legacy runtimes only as unassociated metadata", () => {
  const parsed = parseAdminUnassociatedRuntimes({
    items: [{
      id, workspace_id: id, owner_id: null, provider: "codex", status: "offline", last_seen_at: null, created_at: date, association: "unassociated", device_info: "PRIVATE"
    }], as_of: date, scope: id, next_cursor: null, data_quality: "complete"
  });
  expect(parsed?.items[0]).toMatchObject({ association: "unassociated", lastSeenAt: null });
  expect(JSON.stringify(parsed)).not.toContain("PRIVATE");
});
it("does not turn new lifecycle/admission enums into invented retirement or stop policy", () => {
  const parsed = parseAdminInstallationList({
    items: [{
      ...installation, lifecycle: "new_value", admission: "future_value"
    }], as_of: date, scope: id, next_cursor: null, data_quality: "complete"
  });
  expect(parsed?.items[0]).toMatchObject({ lifecycle: "unknown", admission: "unknown" });
});
it("does not mix one status axis into another", () => {
  const parsed = parseAdminInstallationList({
    items: [{ ...installation, client_activity: { ...axis, state: "ready" } }], as_of: date, scope: id, next_cursor: null, data_quality: "complete"
  });
  expect(parsed?.items[0]?.clientActivity.state).toBe("unknown");
});
