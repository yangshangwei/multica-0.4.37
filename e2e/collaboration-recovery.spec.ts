import { test, type TestInfo } from "@playwright/test";
import { RecoveryEvidence, exerciseRecoveryBatch, exerciseRecoveryOrdinary, exerciseRecoveryPriority, recoveryEnvironmentReady } from "./fixtures/collaboration-recovery";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

async function retainedOrders<T extends string>(info: TestInfo, name: string, orders: readonly T[],
  exercise: (order: T, evidence: RecoveryEvidence) => Promise<unknown>) {
  const inventory = orders.map((order) => ({ order, status: "NOT_RUN", result: null as unknown }));
  const ledger = new RecoveryEvidence(info, name);
  ledger.set({ planned_subcases: orders.length, inventory, retries: 0, business_case_count: null });
  try {
    for (const entry of inventory) {
      const evidence = new RecoveryEvidence(info, `${name}-${entry.order}`);
      entry.status = "RUNNING"; ledger.set({ inventory });
      try {
        entry.result = await exercise(entry.order, evidence);
        entry.status = "PASSED"; evidence.set({ status: "PASSED" });
      } catch (error) {
        entry.status = "FAILED";
        evidence.set({ status: "FAILED", error_type: error instanceof Error ? error.name : "unknown_error" });
        throw error;
      } finally {
        ledger.set({ inventory });
        await info.attach(`${name}-${entry.order}`, { path: info.outputPath(`${name}-${entry.order}.json`), contentType: "application/json" });
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
  test(`${platform} recovery priority and handler contention preserve original receipts in both submission orders`, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!recoveryEnvironmentReady(), "Requires the newly allocated PAY-241 S3 password API, production Web and matching Desktop source");
    await retainedOrders(info, `${platform.toLowerCase()}-priority-transfer`, ["priority", "transfer"] as const,
      (order, evidence) => exerciseRecoveryPriority(browser, platform, info, evidence, order));
  });

  test(`${platform} recovery ordinary child numeric API CAS preserves optional UI compatibility in both submission orders`, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!recoveryEnvironmentReady(), "Requires the newly allocated PAY-241 S3 password API, production Web and matching Desktop source");
    await retainedOrders(info, `${platform.toLowerCase()}-ordinary-child`, ["priority", "assignee"] as const,
      (order, evidence) => exerciseRecoveryOrdinary(browser, platform, info, evidence, order));
  });

  test(`${platform} recovery reads 55 offline comments through complete pages and a reauthenticated live WebSocket`, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!recoveryEnvironmentReady(), "Requires the newly allocated PAY-241 S3 password API, production Web and matching Desktop source");
    await retainedOrders(info, `${platform.toLowerCase()}-55-comment-recovery`, ["complete_batch"] as const,
      (_order, evidence) => exerciseRecoveryBatch(browser, platform, info, evidence));
  });
}
