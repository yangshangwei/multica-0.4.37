import { test } from "./fixtures/project-p1-desktop";
import { duplicatePickerFlow } from "./fixtures/triage-duplicate-picker";

for (const locale of ["en", "zh-Hans"] as const) {
  test(`Native duplicate picker browses all formal tasks and confirms the chosen target (${locale})`, async ({ native }, info) => {
    test.setTimeout(120_000);
    await duplicatePickerFlow(native.page, native, info, locale, true);
  });
}
