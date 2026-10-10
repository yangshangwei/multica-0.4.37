import { test } from "@playwright/test";
import { p1Failure, p1Session } from "./fixtures/project-p1";
import { duplicatePickerFlow } from "./fixtures/triage-duplicate-picker";

for (const locale of ["en", "zh-Hans"] as const) {
  test(`Web duplicate picker browses all formal tasks and confirms the chosen target (${locale})`, async ({ page }, info) => {
    test.setTimeout(120_000);
    const session = await p1Session(page, locale);
    try {
      await duplicatePickerFlow(page, session, info, locale);
    } catch (error) {
      await p1Failure(page, info);
      throw error;
    } finally {
      await session.api.deleteFeatureWorkspace(session.workspace.id);
    }
  });
}
