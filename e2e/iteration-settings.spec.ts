import { expect, test } from "@playwright/test";
import { p1Capture, p1Failure, p1NoOverflow, p1Session } from "./fixtures/project-p1";

for (const copy of [
  {
    locale: "en", iterations: "Iterations", changeTimezone: "Change in workspace settings",
    timezone: "Planning timezone", save: "Save timezone", saved: "Planning timezone saved",
    summary: "New iterations use the workspace timezone: UTC.", enable: "Enable iterations",
  },
  {
    locale: "zh-Hans", iterations: "迭代", changeTimezone: "前往修改",
    timezone: "规划时区", save: "保存时区", saved: "规划时区已保存",
    summary: "新建迭代使用工作空间时区：UTC", enable: "启用迭代",
  },
]) {
  test(`workspace planning timezone has one editor (${copy.locale})`, async ({ page }, info) => {
    test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires an isolated iteration API");
    const { api, workspace } = await p1Session(page, copy.locale);
    const base = `/api/workspaces/${workspace.id}`;
    try {
      await page.goto(`/${workspace.slug}/settings?tab=iterations`);
      const changeTimezone = page.getByRole("link", { name: copy.changeTimezone, exact: true });
      await expect(changeTimezone).toHaveAttribute("href", `/${workspace.slug}/settings?tab=workspace`);
      await expect(page.getByRole("combobox", { name: copy.timezone, exact: true })).toHaveCount(0);
      await expect(page.getByRole("button", { name: copy.save, exact: true })).toHaveCount(0);
      await changeTimezone.click();
      await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/settings\\?tab=workspace$`));
      const editor = page.getByRole("combobox", { name: copy.timezone, exact: true });
      await expect(editor).toHaveCount(1);
      await expect(editor).toContainText("Asia/Shanghai");
      await editor.click();
      await page.getByRole("option", { name: "UTC", exact: true }).click();
      await page.getByRole("button", { name: copy.save, exact: true }).click();
      await expect(page.getByText(copy.saved, { exact: true })).toBeVisible();
      expect(await api.requestJSON(`${base}/planning-timezone`)).toMatchObject({ planning_timezone: "UTC", effective_timezone: "UTC" });

      await page.getByRole("tab", { name: copy.iterations, exact: true }).click();
      await expect(page.getByText(copy.summary, { exact: true })).toBeVisible();
      await expect(page.getByRole("combobox", { name: copy.timezone, exact: true })).toHaveCount(0);
      const toggle = page.getByRole("switch", { name: copy.enable, exact: true });
      await expect(toggle).toBeEnabled();
      await toggle.click();
      await expect(toggle).toBeChecked();
      expect(await api.requestJSON(`${base}/iteration-settings`)).toMatchObject({ enabled: true, effective_timezone: "UTC" });
      await p1NoOverflow(page);
      await p1Capture(page, info, `planning-timezone-${copy.locale}-desktop`);
      await page.setViewportSize({ width: 390, height: 900 });
      await expect(changeTimezone).toBeVisible();
      await p1NoOverflow(page);
      await p1Capture(page, info, `planning-timezone-${copy.locale}-narrow`);
    } catch (error) {
      await info.attach("original-error", { body: String(error), contentType: "text/plain" });
      await p1Failure(page, info).catch(() => {});
      throw error;
    } finally {
      await api.deleteFeatureWorkspace(workspace.id);
    }
  });
}
