import { test } from "@playwright/test";
import { RecoveryEvidence } from "./fixtures/collaboration-recovery";
import { exerciseRequirementsHistory, requirementsHistoryEnvironmentReady, requirementsHistoryJourneys } from "./fixtures/collaboration-requirements-history";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

for (const [index, platform] of (["Web", "Desktop"] as const).entries()) {
  test(requirementsHistoryJourneys[index]!, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!requirementsHistoryEnvironmentReady(),
      "Requires PAY241_TC02_REQUIREMENTS_ACCEPTANCE=1, owned password API/database, production Web and matching Desktop renderer/preload");
    const evidence = new RecoveryEvidence(info, `${platform.toLowerCase()}-requirements-history`);
    evidence.set({ case_id: "TC-02", platform, planned_platforms: 2, planned_sessions: 3,
      planned_versions: 3, planned_observations: 9, planned_explicit_edits: 2, retries: 0 });
    try {
      await exerciseRequirementsHistory(browser, platform, info, evidence);
      evidence.set({ status: "PASSED" });
    } catch (error) {
      evidence.set({ status: "FAILED_OR_INCOMPLETE", error_type: error instanceof Error ? error.name : "unknown_error" });
      throw error;
    } finally {
      await info.attach(evidence.name, { path: info.outputPath(`${evidence.name}.json`), contentType: "application/json" });
    }
  });
}
