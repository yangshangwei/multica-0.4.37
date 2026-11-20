import { test, type TestInfo } from "@playwright/test";
import { RecoveryEvidence } from "./fixtures/collaboration-recovery";
import { exerciseInformationRecovery, informationRecoveryCases, informationRecoveryEnvironmentReady,
  type InformationRecoveryCase } from "./fixtures/collaboration-information-recovery";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

async function retainedCases(info: TestInfo, name: string,
  exercise: (caseName: InformationRecoveryCase, evidence: RecoveryEvidence) => Promise<unknown>) {
  const inventory = informationRecoveryCases.map((caseName) => ({ case: caseName, status: "NOT_RUN", result: null as unknown }));
  const ledger = new RecoveryEvidence(info, name);
  ledger.set({ planned_subcases: inventory.length, inventory, retries: 0, parent_business_case_count: null });
  try {
    for (const entry of inventory) {
      const evidence = new RecoveryEvidence(info, `${name}-${entry.case}`);
      entry.status = "RUNNING"; ledger.set({ inventory });
      try {
        entry.result = await exercise(entry.case, evidence);
        entry.status = "PASSED"; evidence.set({ status: "PASSED" });
      } catch (error) {
        entry.status = "FAILED";
        evidence.set({ status: "FAILED", error_type: error instanceof Error ? error.name : "unknown_error" });
        throw error;
      } finally {
        ledger.set({ inventory });
        await info.attach(`${name}-${entry.case}`, { path: info.outputPath(`${name}-${entry.case}.json`), contentType: "application/json" });
      }
    }
    ledger.set({ status: "PASSED" });
  } catch (error) { ledger.set({ status: "FAILED_OR_INCOMPLETE" }); throw error; }
  finally {
    ledger.set({ inventory, actual_started_subcases: inventory.filter((entry) => entry.status !== "NOT_RUN").length });
    await info.attach(name, { path: info.outputPath(`${name}.json`), contentType: "application/json" });
  }
}

for (const platform of ["Web", "Desktop"] as const) {
  test(`${platform} information recovery preserves structured blocks, exact repairs, original receipts and protected history across A B C E F G`, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!informationRecoveryEnvironmentReady(), "Requires the fresh PAY-241 S3 password API, production Web and matching real Desktop information candidate");
    await retainedCases(info, `${platform.toLowerCase()}-information-recovery`,
      (caseName, evidence) => exerciseInformationRecovery(browser, platform, info, evidence, caseName));
  });
}
