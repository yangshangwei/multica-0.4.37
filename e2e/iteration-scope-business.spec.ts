import { test } from "@playwright/test";
import { iterationEmptyPlanFlow, iterationScopeBusinessFlow } from "./fixtures/iteration-scope-business";
import { p1Failure, p1Session } from "./fixtures/project-p1";

test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires an isolated I1 API and a production Web build");

for (const locale of ["en", "zh-Hans"] as const) {
  test(`ISB Web ${locale}: planned scope, exact change details, retained filters and frozen history`, async ({ page }, info) => {
    test.setTimeout(180_000);
    const session = await p1Session(page, locale);
    try { await iterationScopeBusinessFlow(page, session, info, { locale }); }
    catch (error) { await p1Failure(page, info).catch(() => {}); throw error; }
    finally { await session.api.deleteFeatureWorkspace(session.workspace.id); }
  });
}

test("ISB Web: empty planning history and cancellation before versus after a real empty start", async ({ page }, info) => {
  test.setTimeout(90_000);
  const session = await p1Session(page);
  try { await iterationEmptyPlanFlow(page, session, info); }
  catch (error) { await p1Failure(page, info).catch(() => {}); throw error; }
  finally { await session.api.deleteFeatureWorkspace(session.workspace.id); }
});
