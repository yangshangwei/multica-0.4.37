// @vitest-environment node
import { describe, expect, it } from "vitest";
import { observationParams, observationDrilldown, parseAdminOverview, parseAdminHealth, parseAdminSettings, parseAdminAudit, parseAdminWorkspaces } from "./observability-schema";

const id = "11111111-1111-4111-8111-111111111111";
const window = { time_from: "2026-10-01T00:00:00Z", time_to: "2026-10-02T00:00:00Z", timezone: "Asia/Shanghai" };
const latency = { p50: null, p95: null, samples: 0, lower_bound_samples: 0, unknown_samples: 2 };
const raw = { scope: id, as_of: window.time_to, data_quality: "complete", window, rule_version: "v1",
  installations: { total: 0, retired: 0, client_active: null, daemon_reachable: null, ready: null, unassociated: 0 },
  executions: { completed: 0, failed: 0, cancelled: 2, unfinished: 0, queued: 0, running: 0, success_rate: null, queue_seconds: latency, run_seconds: latency },
  usage: { input_tokens: null, output_tokens: null, total_tokens: null, missing_tasks: 2, unpriced_tasks: 2, quality: "unknown", billing: "tokens_only" },
  alerts: { open: 0, acknowledged: 0, resolved: 0 },
};
describe("observation windows and metric boundaries", () => {
  it("pins one explicit default window and preserves deliberate filters", () => {
    const params = observationParams(new URLSearchParams("timezone=Asia%2FShanghai&status=failed"), new Date("2026-10-02T00:00:00Z"));
    expect(params.get("time_from")).toBe("2026-10-01T00:00:00.000Z");
    expect(params.get("time_to")).toBe("2026-10-02T00:00:00.000Z");
    expect(params.get("timezone")).toBe("Asia/Shanghai");
    expect(params.get("status")).toBe("failed");
  });
  it("carries the authoritative window and finished basis into execution drilldown", () => {
    const href = observationDrilldown("/admin/tasks", { timeFrom: window.time_from, timeTo: window.time_to, timezone: window.timezone }, { status: "failed", time_basis: "finished" });
    const url = new URL(href, "https://test.invalid");
    for (const [key, value] of Object.entries(window)) expect(url.searchParams.get(key)).toBe(value);
    expect(url.searchParams.get("time_basis")).toBe("finished");
    expect(url.searchParams.has("cursor")).toBe(false);
  });
  it("keeps absent usage and empty success denominator unknown instead of zero", () => {
    expect(parseAdminOverview(raw)).toMatchObject({ executions: { successRate: null }, usage: { totalTokens: null, billing: "tokens_only" } });
    expect(parseAdminOverview({ ...raw, secret: "PRIVATE PROMPT" })).not.toHaveProperty("secret");
  });
  it("rejects malformed scope and contradictory empty-denominator percentages", () => {
    expect(parseAdminOverview({ ...raw, scope: "bad" })).toBeNull();
    expect(parseAdminOverview({ ...raw, executions: { ...raw.executions, success_rate: 1 } })).toBeNull();
  });
  it("degrades future quality without claiming healthy sources", () => {
    expect(parseAdminOverview({ ...raw, data_quality: "future" })).toMatchObject({ dataQuality: "unknown" });
  });
  it("keeps closed alert history distinct from automatic recovery counts", () => {
    expect(parseAdminOverview({ ...raw, alerts: { ...raw.alerts, resolved: 2, closed: 3 } })).toMatchObject({ alerts: { resolved: 2, closed: 3 } });
  });
});
it("retains unavailable health and strips raw connection strings", () => {
  const health = parseAdminHealth({ scope: id, as_of: window.time_to, data_quality: "partial", detector_state: "unavailable", sources: [{ name: "database", state: "unavailable", checked_at: null, code: "probe_failed", connection: "SECRET" }] });
  expect(health).toMatchObject({ detectorState: "unavailable", sources: [{ state: "unavailable", checkedAt: null }] });
  expect(JSON.stringify(health)).not.toContain("SECRET");
});
it("does not turn unconfigured retention into an enforced policy", () => {
  const settings = parseAdminSettings({ scope: id, as_of: window.time_to, data_quality: "complete", configuration: { auth_mode: "password", registration_enabled: false, registration_policy: null, managed_installations_enabled: true, workspace_creation_enabled: true, read_only: true, source: "deployment" }, retention: { confirmed_operations_days: null, alerts_days: null, audit_days: null, automatic_deletion_enabled: false, policy_source: "unknown" }, refresh_intervals_seconds: { list: 15, detail: 5 }, secret: "SECRET" });
  expect(settings).toMatchObject({ retention: { auditDays: null, automaticDeletionEnabled: false } });
  expect(JSON.stringify(settings)).not.toContain("SECRET");
});
it("keeps historical actor names unknown and drops private audit snapshot fields", () => {
  const audit = parseAdminAudit({ scope: id, as_of: window.time_to, data_quality: "complete", items: [{ id, operation_id: id, actor_kind: "user", actor_user_id: id, actor_display_name: null, actor_snapshot_quality: "unknown", target_kind: "task", target_id: id, action: "task.cancel", phase: "applied", result_code: "cancelled_before_dispatch", request_id: "req", reason: "Operator request", before_state: { status: "queued", disabled: false, access_revoked: true, prompt: "SECRET" }, after_state: { status: "cancelled" }, created_at: window.time_to }] });
  expect(audit?.items[0]).toMatchObject({ actorDisplayName: null, actorSnapshotQuality: "unknown", beforeState: { status: "queued", disabled: false, access_revoked: true } });
  expect(JSON.stringify(audit)).not.toContain("SECRET");
});

it("accepts the unset resolution in a real alert audit snapshot", () => {
  const audit = parseAdminAudit({ scope: id, as_of: window.time_to, data_quality: "complete", items: [{ id, operation_id: id, actor_kind: "user", actor_user_id: id, actor_display_name: "Operator", actor_snapshot_quality: "captured", target_kind: "alert", target_id: id, action: "alert.acknowledge", phase: "applied", result_code: "alert_acknowledged", request_id: "req", reason: "Investigating", before_state: { status: "open", resolution_code: null, assignee_id: null, related_task_id: null }, after_state: { status: "acknowledged", resolution_code: null }, created_at: window.time_to }] });
  expect(audit?.items[0]?.beforeState).toMatchObject({ status: "open", resolution_code: null });
});
it("requires workspace identities and preserves the counting window", () => {
  const list = { scope: id, as_of: window.time_to, data_quality: "complete", window, items: [{ id, name: "Workspace", slug: "workspace", member_count: 2, execution_count: 3, created_at: window.time_from }] };
  expect(parseAdminWorkspaces(list)).toMatchObject({ items: [{ memberCount: 2, executionCount: 3 }], window: { timezone: window.timezone } });
  expect(parseAdminWorkspaces({ ...list, items: [{ ...list.items[0], id: "bad" }] })).toBeNull();
});
