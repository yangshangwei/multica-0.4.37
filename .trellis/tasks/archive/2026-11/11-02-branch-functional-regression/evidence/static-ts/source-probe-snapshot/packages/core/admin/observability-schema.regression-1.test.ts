// @vitest-environment node
import { expect, it } from "vitest";
import { parseAdminAudit } from "./observability-schema";

// Regression: QA-002 — resource publication made the entire audit page unavailable.
// Found by /qa on 2026-10-05.
// Report: .gstack/qa-reports/2026-10-05-full-e2e/report.md
const id = "11111111-1111-4111-8111-111111111111";
const version = "22222222-2222-4222-8222-222222222222";

function audit(action: string, snapshotVersion: unknown) {
  return {
    scope: id, as_of: "2026-10-05T04:00:00Z", data_quality: "complete",
    items: [{
      id, operation_id: id, actor_kind: "user", actor_user_id: id,
      actor_display_name: "Publisher", actor_snapshot_quality: "captured",
      target_kind: "resource", target_id: id, action, phase: "applied",
      result_code: "succeeded", request_id: "resource-publish", reason: "Publish test resource",
      before_state: {},
      after_state: { version: snapshotVersion, state: "published", config: { token: "PRIVATE" } },
      created_at: "2026-10-05T04:00:00Z",
    }],
  };
}

it.each(["resource.publish", "resource.withdraw"])("retains UUID revisions for %s without exposing resource content", (action) => {
  const parsed = parseAdminAudit(audit(action, version));
  expect(parsed?.items[0]).toMatchObject({ action, afterState: { version, state: "published" } });
  expect(parsed?.items[0]?.afterState).not.toHaveProperty("config");
  expect(JSON.stringify(parsed)).not.toContain("PRIVATE");
});

it.each(["12", 12])("preserves the numeric version contract for %s", (value) => {
  expect(parseAdminAudit(audit("alert.acknowledge", value))?.items[0]?.afterState.version).toBe("12");
});

it("rejects malformed versions while keeping authentication versions numeric", () => {
  expect(parseAdminAudit(audit("resource.publish", "not-a-version"))).toBeNull();
  const raw = audit("resource.publish", version);
  Object.assign(raw.items[0]!.after_state, { auth_version: version });
  expect(parseAdminAudit(raw)).toBeNull();
});
