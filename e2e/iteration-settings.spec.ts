import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { iterationSettingsFeedbackFlow } from "./fixtures/iterations-i1";
import { p1Authenticate, p1Capture, p1Failure, p1NoOverflow, p1Session } from "./fixtures/project-p1";

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

test("workspace iteration settings survive lost closure responses and update another client", async ({ page, context }, info) => {
  test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires an isolated iteration API");
  test.setTimeout(180_000);
  const { api, workspace } = await p1Session(page);
  const base = `/api/workspaces/${workspace.id}`;
  const peer = await context.newPage();
  try {
    const capability = await api.requestJSON<{ supported: boolean; enabled: boolean }>(`${base}/iteration-capabilities`);
    expect(capability).toMatchObject({ supported: true, enabled: false });
    await p1Authenticate(peer, api);
    await peer.goto(`/${workspace.slug}/issues`);
    const peerEntry = peer.getByRole("link", { name: "Iterations", exact: true }).and(peer.locator('[data-sidebar="menu-button"]'));
    await expect(peerEntry).toHaveCount(0);

    await page.goto(`/${workspace.slug}/settings?tab=iterations`);
    await expect(page.getByRole("heading", { name: "Iteration settings", exact: true })).toBeVisible();
    const toggle = page.getByRole("switch", { name: "Enable iterations", exact: true });
    await expect(toggle).not.toBeChecked();
    await expect(toggle).toBeEnabled();
    await toggle.click();
    await expect(toggle).toBeChecked();
    await expect(peerEntry).toBeVisible();
    expect(await api.requestJSON(`${base}/iteration-settings`)).toMatchObject({ enabled: true });

    const settings = await api.requestJSON<{ effective_timezone: string }>(`${base}/iteration-settings`);
    const created = await api.requestJSON<{ iteration_ids: string[] }>(`${base}/iterations`, {
      method: "POST",
      body: {
        request_id: randomUUID(), name: "Settings closure preserves planned history",
        start_date: "2027-01-04", end_date: "2027-01-17",
        confirmed_timezone: settings.effective_timezone,
      },
    });
    await expect(page.getByText("0 active · 1 planned", { exact: true })).toBeVisible();
    await p1Capture(page, info, "iteration-settings-enabled");

    await toggle.click();
    const dialog = page.getByRole("dialog");
    // The modal hides the background from role queries; the saved switch state must remain on.
    await expect(page.getByRole("switch", { name: "Enable iterations", exact: true, includeHidden: true })).toBeChecked();
    await dialog.getByLabel("Reason", { exact: true }).fill("Suspend planning after reviewing all periods");
    await dialog.getByRole("button", { name: "Preview changes", exact: true }).click();
    await expect(dialog.getByText("Settings closure preserves planned history", { exact: false }).first()).toBeVisible();
    let originalRequestId: string | undefined;
    await page.route("**/iteration-operations", async (route) => {
      if (route.request().method() !== "POST" || originalRequestId) { await route.continue(); return; }
      const body = route.request().postDataJSON();
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      originalRequestId = body.request_id;
      await route.abort("connectionreset");
    });
    await dialog.getByRole("button", { name: "Disable all iterations", exact: true }).click();
    await expect.poll(() => originalRequestId).toBeTruthy();
    await expect.poll(async () => (await api.requestJSON<{ enabled: boolean }>(`${base}/iteration-settings`)).enabled).toBe(false);
    await page.reload();
    await page.unroute("**/iteration-operations");
    await expect(page.getByRole("region", { name: "Unconfirmed iteration requests", exact: true })).toBeVisible();
    await page.getByRole("button", { name: /^Check request: Disable all iterations/ }).click();
    await expect(page.getByRole("button", { name: /^Check request: Disable all iterations/ })).toHaveCount(0);
    await expect(toggle).not.toBeChecked();
    await expect(peerEntry).toHaveCount(0);
    await expect(page.getByRole("link", { name: "View history", exact: true })).toBeVisible();
    expect(await api.requestJSON(`${base}/iteration-operations/${originalRequestId}`)).toMatchObject({ request_id: originalRequestId, operation: "disable" });
    expect(await api.requestJSON(`${base}/iterations/${created.iteration_ids[0]}`)).toMatchObject({ iteration: { status: "cancelled" } });

    await toggle.click();
    await expect(toggle).toBeChecked();
    expect(await api.requestJSON(`${base}/iterations/${created.iteration_ids[0]}`)).toMatchObject({ iteration: { status: "cancelled" } });
    await page.setViewportSize({ width: 390, height: 900 });
    await p1NoOverflow(page);
    await p1Capture(page, info, "iteration-settings-after-recovery");
  } catch (error) {
    await info.attach("original-error", { body: String(error), contentType: "text/plain" });
    await p1Failure(page, info).catch(() => {});
    throw error;
  } finally {
    await peer.close();
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("workspace iteration settings report normal closure without remounting", async ({ page }, info) => {
  test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires an isolated iteration API");
  const { api, workspace } = await p1Session(page);
  try {
    await page.goto(`/${workspace.slug}/issues`);
    await iterationSettingsFeedbackFlow(page, api, workspace, info);
  } catch (error) {
    await info.attach("original-error", { body: String(error), contentType: "text/plain" });
    await p1Failure(page, info).catch(() => {});
    throw error;
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
