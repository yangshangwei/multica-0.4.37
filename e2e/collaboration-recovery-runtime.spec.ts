import { test } from "@playwright/test";
import { RecoveryEvidence } from "./fixtures/collaboration-recovery";
import {
  exerciseRecoveryRuntimeNotifications, exerciseRecoveryRuntimeWithdrawal, recoveryRuntimeEnvironmentReady,
} from "./fixtures/collaboration-recovery-runtime";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

for (const platform of ["Web", "Desktop"] as const) {
  for (const [name, description, exercise] of [
    ["notifications", "proven success, retained failure and Stop notifications through original receipt recovery", exerciseRecoveryRuntimeNotifications],
    ["withdrawal", "original business reads and queued inputs across author and live grant withdrawal", exerciseRecoveryRuntimeWithdrawal],
  ] as const) {
    test(`${platform} controlled runtime recovery preserves ${description}`, async ({ browser }, info) => {
      test.setTimeout(480_000);
      test.skip(!recoveryRuntimeEnvironmentReady(), "Requires the separately authorized PAY-241 S3 password host-runtime allocation and explicit test-built executables");
      const evidence = new RecoveryEvidence(info, `${platform.toLowerCase()}-runtime-${name}`);
      try { const result = await exercise(browser, platform, info, evidence); evidence.set({ status: "PASSED", result }); }
      catch (error) { evidence.set({ status: "FAILED_OR_INCOMPLETE", error_type: error instanceof Error ? error.name : "unknown_error" }); throw error; }
      finally { await info.attach(`${platform.toLowerCase()}-runtime-${name}`, { contentType: "application/json", path: info.outputPath(`${platform.toLowerCase()}-runtime-${name}.json`) }); }
    });
  }
}
