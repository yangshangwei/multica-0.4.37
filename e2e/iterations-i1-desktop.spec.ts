import { test } from "./fixtures/project-p1-desktop";
import { expect } from "@playwright/test";
import { iterationClosureFlow } from "./fixtures/iterations-i1";
import { p1Capture, p1NoOverflow } from "./fixtures/project-p1";

test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires isolated I1 test API fixture");

test("I1 Electron manual planning, atomic carryover and immutable history", async ({ native }, info) => {
  test.setTimeout(180_000);
  await iterationClosureFlow(native.page, native.api, native.workspace, info, { recover: true });
  await expect(native.page.getByText("Unknown page", { exact: true })).toHaveCount(0);
  await native.desktop.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0]!.setContentSize(680, 900));
  await p1NoOverflow(native.page);
  await native.page.getByRole("table", { name: "Chart data", exact: true }).scrollIntoViewIfNeeded();
  await p1Capture(native.page, info, "i1-native-680-table");
});
