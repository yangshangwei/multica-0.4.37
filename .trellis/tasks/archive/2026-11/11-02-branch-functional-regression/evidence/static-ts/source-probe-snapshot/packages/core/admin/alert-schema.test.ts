// @vitest-environment node
import { expect, it } from "vitest";
import { parseAdminAlert, parseAdminAlertResult, canControlAlert, adminAlertChangeSchema } from "./alert-schema";
const id = "11111111-1111-4111-8111-111111111111";
const raw = { id, organization_id: id, rule: "execution_failed", severity: "warning", status: "open", subject_kind: "task", subject_id: id,
  first_seen_at: "2026-10-01T00:00:00Z", last_seen_at: "2026-10-01T00:00:00Z", occurrence_count: 1, version: "9223372036854775807", allowed_actions: ["acknowledge", "assign", "close"] };
it("keeps versions lossless and strips private diagnostic content", () => {
  const alert = parseAdminAlert({ ...raw, error: "PRIVATE EXCEPTION", prompt: "PRIVATE PROMPT" });
  expect(alert).toMatchObject({ version: raw.version, assigneeId: null, resolutionCode: null });
  expect(JSON.stringify(alert)).not.toContain("PRIVATE");
});
it("unknown rules and states never enable controls, and observers never write", () => {
  const alert = parseAdminAlert(raw)!;
  expect(canControlAlert("super_admin", alert, "close")).toBe(true);
  expect(canControlAlert("platform_observer", alert, "close")).toBe(false);
  expect(canControlAlert("super_admin", parseAdminAlert({ ...raw, rule: "future", status: "future" })!, "close")).toBe(false);
  expect(canControlAlert("super_admin", { ...alert, status: "closed" }, "assign")).toBe(false);
  expect(canControlAlert("super_admin", { ...alert, rule: "queue_timeout", status: "resolved", conditionActive: null }, "close")).toBe(false);
  expect(canControlAlert("super_admin", { ...alert, rule: "queue_timeout", status: "resolved", conditionActive: false }, "close")).toBe(true);
});
it("never treats a missing operation receipt as mutation success", () => {
  expect(parseAdminAlertResult({ target: raw })).toBeNull();
  expect(parseAdminAlert({ ...raw, id: "wrong" })).toBeNull();
});
it("allows recovered non-failure alerts to close without an execution-failure disposition", () => {
  expect(adminAlertChangeSchema.safeParse({ action: "close", expectedVersion: "2", reason: "Recovery verified" }).success).toBe(true);
  expect(adminAlertChangeSchema.safeParse({ action: "close", expectedVersion: "2", reason: "Recovery verified", relatedTaskId: id }).success).toBe(false);
});
it("keeps automatic recovery evidence distinct from manually selectable dispositions", () => {
  expect(parseAdminAlert({ ...raw, rule: "queue_timeout", status: "resolved", resolution_code: "queue_left", condition_active: false })).toMatchObject({ resolutionCode: "queue_left" });
  expect(parseAdminAlert({ ...raw, rule: "queue_timeout", status: "resolved", resolution_code: "task_removed", condition_active: false })).toMatchObject({ resolutionCode: "task_removed" });
  expect(adminAlertChangeSchema.safeParse({ action: "close", expectedVersion: "2", reason: "Close recovered alert", resolutionCode: "queue_left" }).success).toBe(false);
});
