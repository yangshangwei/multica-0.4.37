import { test } from "./fixtures/project-p1-desktop";
import { expect } from "@playwright/test";
import { iterationClosureFlow, iterationSettingsFeedbackFlow } from "./fixtures/iterations-i1";
import { p1Capture, p1NoOverflow } from "./fixtures/project-p1";

test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires isolated I1 test API fixture");

test("I1 Electron manual planning, atomic carryover and immutable history", async ({ native }, info) => {
  test.setTimeout(180_000);
  await iterationClosureFlow(native.page, native.api, native.workspace, info, { recover: true });
  await expect(native.page.getByText("Unknown page", { exact: true })).toHaveCount(0);
  await native.desktop.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0]!.setContentSize(680, 900));
  await p1NoOverflow(native.page);
  await native.page.getByRole("tab", { name: "Progress", exact: true }).click();
  await native.page.getByRole("tabpanel", { name: "Progress", exact: true }).locator("summary").filter({ hasText: "View chart data" }).focus();
  await native.page.keyboard.press("Enter");
  await native.page.getByRole("table", { name: "Chart data", exact: true }).scrollIntoViewIfNeeded();
  await p1Capture(native.page, info, "i1-native-680-table");
});

test("I1 Electron settings report normal closure without remounting", async ({ native }, info) => {
  await iterationSettingsFeedbackFlow(native.page, native.api, native.workspace, info);
  await expect(native.page.getByText("Unknown page", { exact: true })).toHaveCount(0);
});

test("I1 Electron atomic handoff preserves existing next-period commitments", async ({ native }, info) => {
  test.setTimeout(180_000);
  await iterationClosureFlow(native.page, native.api, native.workspace, info, { handoff: true });
});
