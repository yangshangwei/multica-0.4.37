import { test } from "@playwright/test";
import { RecoveryEvidence } from "./fixtures/collaboration-recovery";
import {
  exerciseRecoveryNotifications, exerciseRecoveryRevocations, exerciseRecoveryUploads, recoveryExtrasEnvironmentReady,
} from "./fixtures/collaboration-recovery-extras";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

for (const platform of ["Web", "Desktop"] as const) {
  for (const [name, description, exercise] of [
    ["notifications", "typed mentions, original notification events and owner withdrawal", exerciseRecoveryNotifications],
    ["uploads", "three original multipart intents through GET, matching retry and changed reselection", exerciseRecoveryUploads],
    ["revocations", "offline and delayed protected reads across participant and source membership loss", exerciseRecoveryRevocations],
  ] as const) {
    test(`${platform} recovery extras preserve ${description}`, async ({ browser }, info) => {
      test.setTimeout(360_000);
      test.skip(!recoveryExtrasEnvironmentReady(), "Requires the separately authorized PAY-241 S3 managed password Extras-A allocation");
      const evidence = new RecoveryEvidence(info, `${platform.toLowerCase()}-extras-${name}`);
      try { const result = await exercise(browser, platform, info, evidence); evidence.set({ status: "PASSED", result }); }
      catch (error) { evidence.set({ status: "FAILED_OR_INCOMPLETE", error_type: error instanceof Error ? error.name : "unknown_error" }); throw error; }
      finally { await info.attach(`${platform.toLowerCase()}-extras-${name}`, { path: info.outputPath(`${platform.toLowerCase()}-extras-${name}.json`), contentType: "application/json" }); }
    });
  }
}
