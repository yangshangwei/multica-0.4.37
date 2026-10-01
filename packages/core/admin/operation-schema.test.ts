// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseAdminOperation, parseAdminCancellationResult, operationNeedsPolling } from "./operation-schema";
const id = "11111111-1111-4111-8111-111111111111";
const operation = { id, organization_id: id, actor_id: id, target_id: id, target_kind: "task", kind: "task.cancel", state: "applied", result_code: "awaiting_daemon_confirmation", confirmation: "pending", reconciliation_state: "pending", version: 1, accepted_at: "2026-10-01T00:00:00Z", updated_at: "2026-10-01T00:00:00Z" };
describe("administrative operation receipts", () => {
  it("uses effective root results without displaying a contradictory follower state", () => {
    const parsed = parseAdminOperation({ ...operation, root_operation_id: id, state: "succeeded", confirmation: "confirmed", result_code: "daemon_stopped", own_state: "applied", private_payload: "SECRET" });
    expect(parsed).toMatchObject({ state: "succeeded", confirmation: "confirmed", rootOperationId: id });
    expect(JSON.stringify(parsed)).not.toContain("SECRET");
    expect(JSON.stringify(parsed)).not.toContain("own_state");
  });
  it("degrades future states and rejects malformed identities", () => {
    expect(parseAdminOperation({ ...operation, state: "future", confirmation: "future", reconciliation_state: "future" })).toMatchObject({ state: "unknown", confirmation: "unknown", reconciliationState: "unknown" });
    expect(parseAdminOperation({ ...operation, id: "bad" })).toBeNull();
  });
  it("keeps cancellation acceptance separate from process confirmation and bounds polling", () => {
    const pending = parseAdminOperation(operation)!;
    expect(operationNeedsPolling(pending, 0)).toBe(true);
    expect(operationNeedsPolling(pending, 30_000)).toBe(false);
    expect(operationNeedsPolling({ ...pending, confirmation: "unconfirmed", reconciliationState: "unconfirmed" }, 0)).toBe(false);
    expect(operationNeedsPolling({ ...pending, state: "succeeded", confirmation: "confirmed" }, 0)).toBe(false);
    expect(operationNeedsPolling({ ...pending, state: "unknown" }, 0)).toBe(false);
  });
  it("requires current target identity and a real fence in cancellation responses", () => {
    const target = { id, status: "cancelled", state_version: "2", execution_fence: { runtime_id: null, dispatched_at: null, target_version: "2" } };
    expect(parseAdminCancellationResult({ operation, target })).toMatchObject({ operation: { state: "applied" }, target: { id, stateVersion: "2" } });
    expect(parseAdminCancellationResult({ operation, target: { id } })).toBeNull();
  });
});
