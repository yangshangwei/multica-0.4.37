import { randomUUID } from "node:crypto";
import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

async function expectNoOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const panel = page.getByRole("tabpanel");
  expect(await panel.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
}

for (const locale of ["en", "zh-Hans"] as const) {
  test(`integration catalog navigation, preferences and responsive layout (${locale})`, async ({ page }, testInfo) => {
    test.setTimeout(120_000);
    page.setDefaultTimeout(15_000);
    const api = new TestApiClient();
    const slug = `catalog-${randomUUID().slice(0, 12)}`;
    let workspaceId: string | undefined;
    const errors: string[] = [];
    const copy = locale === "en" ? {
      catalog: "Integrations", code: "Code hosting", tasks: "Task management", chat: "Communication & collaboration",
      back: "All integrations", connected: "Connected", disconnected: "Not connected", off: "Features off",
      master: "Enable GitHub features", disconnect: "Disconnect", cancel: "Cancel",
      planned: "Planned", preferences: "Preferences", sidebar: "Pull Request sidebar", coauthor: "Co-authored-by trailer", autoLink: "Auto-link issues and PRs",
    } : {
      catalog: "集成", code: "代码托管", tasks: "任务管理", chat: "沟通与协作",
      back: "全部集成", connected: "已连接", disconnected: "未连接", off: "功能已关闭",
      master: "启用 GitHub 功能", disconnect: "断开", cancel: "取消",
      planned: "待规划", preferences: "偏好设置", sidebar: "Pull Request 侧栏", coauthor: "Co-authored-by trailer", autoLink: "任务 ↔ PR 自动关联",
    };
    try {
      await api.login(`${slug}@example.invalid`, "Integration catalog test");
      const workspace = await api.ensureWorkspace("Integration catalog test", slug);
      workspaceId = workspace.id;
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await api.requestJSON(`/api/workspaces/${workspace.id}`, {
        method: "PATCH", body: { settings: { github_enabled: true, github_pr_sidebar_enabled: true, co_authored_by_enabled: false, github_auto_link_prs_enabled: true } },
      });
      const token = api.getToken();
      if (!token) throw new Error("Missing isolated test token");
      await page.addInitScript(({ token, locale }) => {
        localStorage.setItem("multica_token", token);
        localStorage.setItem("multica-locale", locale);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, { token, locale });
      page.on("pageerror", (error) => errors.push(error.message));
      await page.route("**/api/config", async (route) => {
        const response = await route.fetch();
        const config = await response.json();
        await route.fulfill({ response, json: { ...config, messaging_integrations_enabled: false, vcs_integration_available: true, feature_flags: { ...config.feature_flags, composio_mcp_apps: false } } });
      });
      let connected = true;
      await page.route(`**/api/workspaces/${workspace.id}/github/installations**`, (route) => {
        if (route.request().method() === "DELETE") {
          connected = false;
          return route.fulfill({ status: 204 });
        }
        return route.fulfill({ json: {
          configured: true, can_manage: true,
          installations: connected ? [{ id: "test-installation", workspace_id: workspace.id, account_login: "a-very-long-organization-name-for-responsive-layout-checks", account_type: "Organization", account_avatar_url: null, created_at: new Date().toISOString(), connected_by: "Test owner" }] : [],
        } });
      });
      await page.route(`**/api/workspaces/${workspace.id}/vcs/connections`, (route) => route.fulfill({ json: { configured: true, can_manage: true, connections: [] } }));
      const settings = `/${workspace.slug}/settings`;
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`${settings}?tab=integrations`, { waitUntil: "domcontentloaded" });
      const panel = page.getByRole("tabpanel");
      const github = panel.getByRole("link", { name: "GitHub", exact: true });
      await expect(github).toBeVisible({ timeout: 30_000 });
      await expect(github).toContainText(copy.connected);
      await expect(panel.getByRole("heading", { level: 3 })).toHaveText([copy.code, copy.tasks, copy.chat]);
      await expect(panel.getByText(copy.planned, { exact: true })).toHaveCount(4);
      await expect(panel.getByRole("switch")).toHaveCount(0);
      for (const name of ["ONES", "Plane", "Kaneo", locale === "en" ? "Fuxin" : "孚信"]) {
        await expect(panel.getByRole("link", { name, exact: true })).toHaveCount(0);
      }
      for (const width of [320, 390, 768, 900, 1024, 1440]) {
        await page.setViewportSize({ width, height: 1000 });
        await expectNoOverflow(page);
        const columns = await panel.locator("section").first().locator(".grid").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length);
        if (width <= 390) expect(columns).toBe(1);
        if (width === 1440) expect(columns).toBe(2);
        if (width === 390 || width === 1440) await page.screenshot({ path: testInfo.outputPath(`catalog-${locale}-${width}.png`), fullPage: true });
      }
      await github.focus();
      await page.keyboard.press("Enter");
      await expect(page).toHaveURL(/tab=integrations&integration=github/);
      await expect(panel.getByRole("link", { name: copy.back })).toBeFocused();
      await expect(panel.getByRole("switch", { name: copy.master, exact: true })).toBeChecked();
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(panel.getByRole("switch", { name: copy.master, exact: true })).toBeChecked();
      await page.goBack();
      await expect(github).toBeVisible();
      await expect(github).toBeFocused();
      await page.goForward();
      await expect(panel.getByRole("switch", { name: copy.master, exact: true })).toBeChecked();
      for (const width of [320, 390, 768, 900, 1024, 1440]) {
        await page.setViewportSize({ width, height: 1000 });
        await expectNoOverflow(page);
        if (width === 390 || width === 1440) await page.screenshot({ path: testInfo.outputPath(`github-${locale}-${width}.png`), fullPage: true });
      }
      await panel.getByRole("switch", { name: copy.master, exact: true }).click();
      await expect(panel.getByRole("switch", { name: copy.master, exact: true })).not.toBeChecked();
      for (const name of [copy.sidebar, copy.coauthor, copy.autoLink]) {
        await expect(panel.getByRole("switch", { name, exact: true })).not.toBeChecked();
        await expect(panel.getByRole("switch", { name, exact: true })).toBeDisabled();
      }
      await expect(panel.getByRole("button", { name: copy.disconnect, exact: true })).toBeVisible();
      await panel.getByRole("link", { name: copy.back }).click();
      await expect(github).toContainText(copy.connected);
      await expect(github).toContainText(copy.off);
      await github.click();
      await panel.getByRole("switch", { name: copy.master, exact: true }).click();
      await expect(panel.getByRole("switch", { name: copy.sidebar, exact: true })).toBeChecked();
      await expect(panel.getByRole("switch", { name: copy.coauthor, exact: true })).not.toBeChecked();
      await expect(panel.getByRole("switch", { name: copy.autoLink, exact: true })).toBeChecked();
      await panel.getByRole("button", { name: copy.disconnect, exact: true }).click();
      await page.getByRole("alertdialog").getByRole("button", { name: copy.cancel, exact: true }).click();
      expect(connected).toBe(true);
      await panel.getByRole("button", { name: copy.disconnect, exact: true }).click();
      await page.getByRole("alertdialog").getByRole("button", { name: copy.disconnect, exact: true }).click();
      await expect(page.getByRole("alertdialog")).toHaveCount(0);
      expect(connected).toBe(false);
      await panel.getByRole("link", { name: copy.back }).click();
      await expect(github).toContainText(copy.disconnected);
      await page.goto(`${settings}?tab=github&keep=yes#context`, { waitUntil: "domcontentloaded" });
      await expect(panel.getByRole("switch", { name: copy.master, exact: true })).toBeVisible();
      await page.getByRole("tab", { name: copy.preferences, exact: true }).click();
      await expect(page).toHaveURL(`${new URL(settings, testInfo.project.use.baseURL).href}?tab=preferences&keep=yes#context`);
      await page.goto(`${settings}?tab=integrations&integration=ones`, { waitUntil: "domcontentloaded" });
      await expect(github).toBeVisible();
      await expect(panel.getByRole("status")).toBeVisible();
      await expect(panel.getByRole("switch")).toHaveCount(0);
      expect(errors).toEqual([]);
    } finally {
      if (workspaceId) {
        const response = await fetch(new URL(`/api/workspaces/${workspaceId}`, testInfo.project.use.baseURL), {
          method: "DELETE", signal: AbortSignal.timeout(15_000),
          headers: { Authorization: `Bearer ${api.getToken()}`, "X-Workspace-ID": workspaceId },
        });
        expect(response.status).toBe(204);
      }
    }
  });
}
