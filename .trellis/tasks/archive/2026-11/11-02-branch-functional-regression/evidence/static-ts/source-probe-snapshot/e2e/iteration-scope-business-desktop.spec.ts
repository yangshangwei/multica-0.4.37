import { expect } from "@playwright/test";
import { test } from "./fixtures/project-p1-desktop";
import { iterationScopeBusinessFlow } from "./fixtures/iteration-scope-business";

for (const locale of ["en", "zh-Hans"] as const) {
  test(`ISB Electron ${locale}: planned scope, exact change details, retained filters and frozen history`, async ({ native }, info) => {
    test.setTimeout(180_000);
    await iterationScopeBusinessFlow(native.page, native, info, { locale, native: true });
    await expect(native.page.getByText("Unknown page", { exact: true })).toHaveCount(0);
  });
}
