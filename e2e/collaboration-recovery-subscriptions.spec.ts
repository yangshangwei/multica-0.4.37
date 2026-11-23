import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { test, type TestInfo } from "@playwright/test";
import { z } from "zod";
import { RecoveryEvidence } from "./fixtures/collaboration-recovery";
import { exerciseRecoverySubscriptions, recoverySubscriptionsEnvironmentReady, subscriptionJourneyNames } from "./fixtures/collaboration-recovery-subscriptions";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });
test.describe.configure({ retries: 0 });

const inventorySchema = z.object({
  planned_tops: z.literal(2), workers: z.literal(1), retries: z.literal(0),
  tests: z.array(z.object({ name: z.string(), status: z.enum(["NOT_RUN", "RUNNING", "PASSED", "FAILED_OR_INCOMPLETE"]),
    result: z.unknown(), error_type: z.string().nullable() })).length(2),
});

// This round ledger survives a worker replacement after a failed first top.
// Its complete denominator is saved before any registration or fixture body.
function roundInventory(info: TestInfo) {
  if (info.config.workers !== 1 || info.project.retries !== 0 || info.retry !== 0) throw new Error("Ordinary subscriber acceptance requires workers=1 and retries=0");
  mkdirSync(info.project.outputDir, { recursive: true, mode: 0o700 });
  const path = join(info.project.outputDir, "ordinary-subscription-round-inventory.json");
  const value = existsSync(path) ? inventorySchema.parse(JSON.parse(readFileSync(path, "utf8"))) : inventorySchema.parse({
    planned_tops: 2, workers: 1, retries: 0,
    tests: subscriptionJourneyNames.map((name) => ({ name, status: "NOT_RUN", result: null, error_type: null })),
  });
  if (JSON.stringify(value.tests.map((entry) => entry.name)) !== JSON.stringify(subscriptionJourneyNames)) throw new Error("The subscription round inventory differs from its exact two selected tops");
  const save = () => writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 });
  save();
  return { path, value, save };
}

for (const [index, platform] of (["Web", "Desktop"] as const).entries()) {
  test(subscriptionJourneyNames[index]!, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!recoverySubscriptionsEnvironmentReady(), "Requires PAY241_S3_SUBSCRIPTION_ACCEPTANCE=1 and the existing explicit S3 password/source/database allocation");
    const inventory = roundInventory(info), entry = inventory.value.tests[index]!;
    if (entry.status !== "NOT_RUN") throw new Error("Each original ordinary subscriber top must start exactly once");
    const name = `${platform.toLowerCase()}-ordinary-subscriptions`, evidence = new RecoveryEvidence(info, name);
    entry.status = "RUNNING"; inventory.save();
    evidence.set({ planned_top_tests: inventory.value, selected_top: entry.name, active_phase: "ordinary_subscription_journey",
      business_case_count: null, literal_event_replay_layer: "Go notification listener", current_client_scope: "ordinary manual subscriber and inbox" });
    let primary: unknown, attachmentFailed = false;
    try {
      entry.result = await exerciseRecoverySubscriptions(browser, platform, info, evidence);
      entry.status = "PASSED"; evidence.set({ status: "PASSED", result: entry.result });
    } catch (error) {
      primary = error; entry.status = "FAILED_OR_INCOMPLETE"; entry.error_type = error instanceof Error ? error.name : "unknown_error";
      evidence.set({ status: "FAILED_OR_INCOMPLETE", original_error_type: entry.error_type,
        original_error_sha256: createHash("sha256").update(error instanceof Error ? error.message : String(error)).digest("hex") });
    } finally {
      inventory.save();
      evidence.set({ final_top_inventory: inventory.value, actual_started_tops: inventory.value.tests.filter((item) => item.status !== "NOT_RUN").length });
      const attached = await Promise.allSettled([
        info.attach(name, { path: info.outputPath(`${name}.json`), contentType: "application/json" }),
        info.attach("ordinary-subscription-round-inventory", { path: inventory.path, contentType: "application/json" }),
      ]);
      attachmentFailed = attached.some((result) => result.status === "rejected");
      if (attachmentFailed) {
        entry.status = "FAILED_OR_INCOMPLETE";
        entry.error_type ??= "evidence_attachment_failed";
        inventory.save();
      }
      evidence.set({ evidence_attachment_outcomes: attached.map((result) => result.status === "fulfilled" ? { status: result.status }
        : { status: result.status, error_type: result.reason instanceof Error ? result.reason.name : "unknown_error" }),
        primary_failure_preserved: primary !== undefined, final_top_inventory: inventory.value,
        ...(attachmentFailed ? { status: "FAILED_OR_INCOMPLETE" } : {}) });
    }
    if (primary !== undefined) throw primary;
    if (attachmentFailed) throw new Error("Ordinary subscriber evidence attachment failed; cleanup outcomes are retained");
  });
}
