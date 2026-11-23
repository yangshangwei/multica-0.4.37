import { test } from "@playwright/test";
import { RecoveryEvidence } from "./fixtures/collaboration-recovery";
import { exerciseRuntimeOutput, runtimeOutputEnvironmentReady } from "./fixtures/collaboration-runtime-output";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

for (const platform of ["Web", "Desktop"] as const) {
  test(`${platform} controlled Agent output preserves original files, receipts, results and current authority`, async ({ browser }, info) => {
    test.setTimeout(720_000);
    test.skip(!runtimeOutputEnvironmentReady(), "Requires the separately permitted fresh PAY-241 output allocation and explicit test-built executables");
    const evidence = new RecoveryEvidence(info, `${platform.toLowerCase()}-runtime-output`);
    try { const result = await exerciseRuntimeOutput(browser, platform, info, evidence); evidence.set({ status: "PASSED", result }); }
    catch (error) { evidence.set({ status: "FAILED_OR_INCOMPLETE", error_type: error instanceof Error ? error.name : "unknown_error" }); throw error; }
    finally { await info.attach(`${platform.toLowerCase()}-runtime-output`, { contentType: "application/json", path: info.outputPath(`${platform.toLowerCase()}-runtime-output.json`) }); }
  });
}
