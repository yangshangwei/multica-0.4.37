import { test } from "@playwright/test";
import { RecoveryEvidence } from "./fixtures/collaboration-recovery";
import { exercisePayCreation, payCreationEnvironmentReady, payCreationJourneys } from "./fixtures/collaboration-pay-creation";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

for (const [index, platform] of (["Web", "Desktop"] as const).entries()) {
  test(payCreationJourneys[index]!, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!payCreationEnvironmentReady(),
      "Requires PAY241_TC01_CREATION_ACCEPTANCE=1, owned password API/database, production Web and matching Desktop renderer/preload");
    const evidence = new RecoveryEvidence(info, `${platform.toLowerCase()}-pay-creation`);
    evidence.set({ case_id: "TC-01", platform, planned_platforms: 2, planned_sessions: 3, planned_UI_creations: 1,
      planned_empty_title_preventions: 1, planned_original_GETs: 1, planned_result_GETs: 1, planned_same_key_API_replays: 1,
      retries: 0, whole_TC01_acceptance: false, P1_mapping_confirmed: false, global_business_fields_required: false });
    try {
      await exercisePayCreation(browser, platform, info, evidence);
      evidence.set({ status: "PASSED" });
    } catch (error) {
      evidence.set({ status: "FAILED_OR_INCOMPLETE", error_type: error instanceof Error ? error.name : "unknown_error" });
      throw error;
    } finally {
      await info.attach(evidence.name, { path: info.outputPath(`${evidence.name}.json`), contentType: "application/json" });
    }
  });
}
