import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { test, type TestInfo } from "@playwright/test";
import { z } from "zod";
import { RecoveryEvidence } from "./fixtures/collaboration-recovery";
import {
  exerciseRecoveryOrdinaryUI, ordinaryUIJourneyNames, ordinaryUISubcaseNames,
  recoveryOrdinaryUIEnvironmentReady, seedRecoveryOrdinaryUI,
} from "./fixtures/collaboration-recovery-ordinary-ui";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });
test.describe.configure({ retries: 0 });

const status = z.enum(["NOT_RUN", "RUNNING", "PASSED", "FAILED_OR_INCOMPLETE"]);
const inventorySchema = z.object({
  planned_tops: z.literal(2), planned_subcases: z.literal(6), workers: z.literal(1), retries: z.literal(0),
  tests: z.array(z.object({ name: z.string(), status, error_type: z.string().nullable(),
    subcases: z.array(z.object({ name: z.enum(ordinaryUISubcaseNames), status, result: z.unknown(),
      evidence_path: z.string().nullable(), error_type: z.string().nullable() })).length(3),
  })).length(2),
});

// Persist the entire round before setup; a replacement worker retains the
// failed top and every unstarted subcase instead of reconstructing a pass set.
function roundInventory(info: TestInfo) {
  if (info.config.workers !== 1 || info.project.retries !== 0 || info.retry !== 0) {
    throw new Error("Ordinary UI CAS requires workers=1 and retries=0");
  }
  mkdirSync(info.project.outputDir, { recursive: true, mode: 0o700 });
  const path = join(info.project.outputDir, "ordinary-ui-cas-round-inventory.json");
  const value = inventorySchema.parse(existsSync(path) ? JSON.parse(readFileSync(path, "utf8")) : {
    planned_tops: 2, planned_subcases: 6, workers: 1, retries: 0,
    tests: ordinaryUIJourneyNames.map((name) => ({ name, status: "NOT_RUN", error_type: null,
      subcases: ordinaryUISubcaseNames.map((name) => ({ name, status: "NOT_RUN", result: null,
        evidence_path: null, error_type: null })),
    })),
  });
  if (JSON.stringify(value.tests.map((entry) => entry.name)) !== JSON.stringify(ordinaryUIJourneyNames) ||
    value.tests.some((entry) => JSON.stringify(entry.subcases.map((item) => item.name)) !== JSON.stringify(ordinaryUISubcaseNames))) {
    throw new Error("The ordinary UI round differs from its exact two-top/six-subcase inventory");
  }
  const save = () => writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 });
  save();
  return { path, value, save };
}

for (const [index, platform] of (["Web", "Desktop"] as const).entries()) {
  test(ordinaryUIJourneyNames[index]!, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!recoveryOrdinaryUIEnvironmentReady(),
      "Requires PAY241_S3_ORDINARY_UI_CAS_ACCEPTANCE=1 and the explicit S3 password/API/production Web/matching native allocation");
    const inventory = roundInventory(info), top = inventory.value.tests[index]!;
    if (top.status !== "NOT_RUN") throw new Error("Each original ordinary UI top must start exactly once");
    const name = `${platform.toLowerCase()}-ordinary-ui-cas`, evidence = new RecoveryEvidence(info, name);
    top.status = "RUNNING"; inventory.save();
    evidence.set({ platform, planned_tops: 2, planned_subcases: 6, selected_top: top.name, subcases: top.subcases,
      workers: 1, retries: 0, UI_revision_injection: false, business_case_count: null });
    let primary: unknown, attachmentFailed = false;
    try {
      const fixture = await seedRecoveryOrdinaryUI(evidence);
      for (const entry of top.subcases) {
        const caseName = `${name}-${entry.name}`, caseEvidence = new RecoveryEvidence(info, caseName);
        entry.status = "RUNNING"; entry.evidence_path = info.outputPath(`${caseName}.json`); inventory.save();
        caseEvidence.set({ platform, subcase: entry.name, parent_issue_id: fixture.recovery.issueId,
          child_issue_id: fixture.children[entry.name].id, source_workspace_id: fixture.recovery.workspace.id });
        evidence.set({ subcases: top.subcases });
        try {
          entry.result = await exerciseRecoveryOrdinaryUI(browser, platform, info, caseEvidence, fixture, entry.name);
          entry.status = "PASSED"; caseEvidence.set({ status: "PASSED", result: entry.result });
        } catch (error) {
          entry.status = "FAILED_OR_INCOMPLETE"; entry.error_type = error instanceof Error ? error.name : "unknown_error";
          caseEvidence.set({ status: entry.status, original_error_type: entry.error_type });
          throw error;
        } finally {
          inventory.save(); evidence.set({ subcases: top.subcases });
        }
      }
      top.status = "PASSED"; evidence.set({ status: "PASSED" });
    } catch (error) {
      primary = error; top.status = "FAILED_OR_INCOMPLETE"; top.error_type = error instanceof Error ? error.name : "unknown_error";
      evidence.set({ status: top.status, original_error_type: top.error_type,
        original_error_sha256: createHash("sha256").update(error instanceof Error ? error.message : String(error)).digest("hex") });
    } finally {
      inventory.save();
      const attachments = [info.attach(name, { path: info.outputPath(`${name}.json`), contentType: "application/json" }),
        info.attach("ordinary-ui-cas-round-inventory", { path: inventory.path, contentType: "application/json" }),
        ...top.subcases.filter((entry) => entry.evidence_path !== null).map((entry) =>
          info.attach(`${name}-${entry.name}`, { path: entry.evidence_path!, contentType: "application/json" }))];
      const outcomes = await Promise.allSettled(attachments);
      attachmentFailed = outcomes.some((result) => result.status === "rejected");
      if (attachmentFailed) {
        top.status = "FAILED_OR_INCOMPLETE"; top.error_type ??= "evidence_attachment_failed"; inventory.save();
      }
      evidence.set({ final_round_inventory: inventory.value,
        actual_started_tops: inventory.value.tests.filter((entry) => entry.status !== "NOT_RUN").length,
        actual_started_subcases: inventory.value.tests.flatMap((entry) => entry.subcases).filter((entry) => entry.status !== "NOT_RUN").length,
        evidence_attachment_outcomes: outcomes.map((result) => ({ status: result.status,
          ...(result.status === "rejected" ? { error_type: result.reason instanceof Error ? result.reason.name : "unknown_error" } : {}) })),
        primary_failure_preserved: primary !== undefined, ...(attachmentFailed ? { status: top.status } : {}) });
    }
    if (primary !== undefined) throw primary;
    if (attachmentFailed) throw new Error("Ordinary UI evidence attachment failed; original and cleanup facts are retained");
  });
}
