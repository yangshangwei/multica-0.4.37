import { expect, test } from "@playwright/test";
import { p1Session, p1Failure, p1Capture, p1NoOverflow } from "./fixtures/project-p1";
import { iterationClosureFlow } from "./fixtures/iterations-i1";

test("I1 Web manual planning, atomic carryover and immutable history", async ({ page }, info) => {
  test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires isolated I1 test API fixture");
  test.setTimeout(180_000);
  const { api, workspace } = await p1Session(page);
  try {
    await page.goto(`/${workspace.slug}/issues`);
    await iterationClosureFlow(page, api, workspace, info);
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.addInitScript(() => localStorage.setItem("multica-locale", "zh-Hans"));
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    await expect(page.getByRole("heading", { name: "关闭时快照", exact: true })).toBeVisible();
    await page.getByRole("table", { name: "图表数据", exact: true }).scrollIntoViewIfNeeded();
    await p1NoOverflow(page);
    await p1Capture(page, info, "i1-web-chinese-390");
  } catch (error) {
    await info.attach("original-error", { body: String(error), contentType: "text/plain" });
    await p1Failure(page, info).catch(() => {});
    throw error;
  }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("I1 Web starts the chosen next period atomically and disables the complete workspace", async ({ page }, info) => {
  test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires isolated I1 test API fixture");
  test.setTimeout(180_000);
  const { api, workspace } = await p1Session(page);
  try {
    await page.goto(`/${workspace.slug}/issues`);
    await iterationClosureFlow(page, api, workspace, info, { handoff: true });
  } catch (error) {
    await info.attach("original-error", { body: String(error), contentType: "text/plain" });
    await p1Failure(page, info).catch(() => {});
    throw error;
  } finally { await api.deleteFeatureWorkspace(workspace.id); }
});
