import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { test, expect, _electron as electron, type ElectronApplication, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Desktop uses a memory router. The persisted active session is its exact
// destination; window.location remains the renderer URL throughout navigation.
async function activeDesktopRoute(page: Page) {
  return page.evaluate(() => {
    const raw = localStorage.getItem("multica_tabs");
    if (!raw) return null;
    const { state }: {
      state: {
        activeWorkspaceSlug: string | null;
        byWorkspace: Record<string, {
          activeTabId: string;
          tabs: { id: string; url: string }[];
        }>;
      };
    } = JSON.parse(raw);
    if (!state.activeWorkspaceSlug) return null;
    const group = state.byWorkspace[state.activeWorkspaceSlug];
    return group?.tabs.find((tab) => tab.id === group.activeTabId)?.url ?? null;
  });
}

test("desktop Help selects Updates by keyboard and grouped settings show the stopped daemon in Chinese", async ({}, testInfo) => {
  test.skip(
    !process.env.CHANGELOG_ELECTRON_RENDERER_URL || !process.env.CHANGELOG_E2E_API_URL,
    "Requires built desktop preload and a local renderer/API fixture",
  );
  test.setTimeout(90_000);

  const root = resolve(import.meta.dirname, "..");
  const profile = mkdtempSync(join(tmpdir(), "multica-desktop-settings-"));
  const slug = `e2e-desktop-settings-${randomUUID().slice(0, 12)}`;
  const api = new TestApiClient();
  let workspaceId: string | undefined;
  let desktop: ElectronApplication | undefined;
  let page: Page | undefined;
  const pageErrors: string[] = [];

  try {
    await api.login(`${slug}@example.invalid`, "Desktop settings regression");
    const workspace = await api.ensureWorkspace("Desktop settings regression", slug);
    workspaceId = workspace.id;
    expect(workspace.slug).toBe(slug);
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    const token = api.getToken();
    if (!token) throw new Error("Desktop settings fixture authentication is incomplete");

    const executablePath = join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin"
      ? "Electron.app/Contents/MacOS/Electron"
      : process.platform === "win32" ? "electron.exe" : "electron");
    desktop = await electron.launch({
      executablePath,
      args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
      env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile },
    });
    page = await desktop.firstWindow();
    page.on("pageerror", (error) => pageErrors.push(error.message));
    await page.waitForURL(new URL(process.env.CHANGELOG_ELECTRON_RENDERER_URL!).href);
    await page.locator("#root > *").first().waitFor({ state: "attached", timeout: 30_000 });
    await page.context().addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica-locale", "zh-Hans");
      localStorage.setItem("multica:chat:isOpen", "false");
    }, token);
    await page.reload({ waitUntil: "domcontentloaded" });

    const help = page.getByRole("button", { name: "帮助", exact: true });
    await expect(help).toBeVisible({ timeout: 45_000 });
    await help.focus();
    await page.keyboard.press("ArrowDown");
    const menu = page.getByRole("menu");
    await expect(menu).toBeVisible();
    await expect(menu.getByRole("menuitem", { name: "文档", exact: true })).toBeFocused();
    for (const name of ["变更说明", "反馈", "桌面端版本 0.4.40-test"]) {
      await page.keyboard.press("ArrowDown");
      await expect(menu.getByRole("menuitem", { name, exact: true })).toBeFocused();
    }
    const menuScreenshot = testInfo.outputPath("desktop-version-menu.png");
    await page.screenshot({ path: menuScreenshot });
    await testInfo.attach("desktop-version-menu", { path: menuScreenshot, contentType: "image/png" });
    await page.keyboard.press("Enter");

    await expect.poll(() => activeDesktopRoute(page!)).toBe(`/${slug}/settings?tab=updates`);
    await expect(page.getByRole("tab", { name: "更新", exact: true })).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("tabpanel").getByRole("heading", { name: "更新", exact: true })).toBeVisible();
    await expect(page.getByRole("tabpanel").getByText("v0.4.40-test", { exact: true })).toBeVisible();
    await expect(menu).toHaveCount(0);
    await expect(help).toHaveAttribute("aria-expanded", "false");

    const navigation = page.getByRole("tablist");
    for (const group of ["个人设置", "工作区管理", "任务配置", "连接与扩展"]) {
      await expect(navigation.getByText(group, { exact: true })).toBeVisible();
    }
    await navigation.getByRole("tab", { name: "守护进程", exact: true }).click();
    await expect.poll(() => activeDesktopRoute(page!)).toBe(`/${slug}/settings?tab=daemon`);
    await expect(navigation.getByRole("tab", { name: "守护进程", exact: true })).toHaveAttribute("aria-selected", "true");
    const daemon = page.getByRole("tabpanel");
    await expect(daemon.getByRole("heading", { name: "守护进程", exact: true })).toBeVisible();
    await expect(daemon.getByText("设置本地智能体守护进程如何随桌面应用启动和停止。", { exact: true })).toBeVisible();
    await expect(daemon.getByText("已安装 multica CLI，可通过 PATH 环境变量找到并调用。", { exact: true })).toBeVisible();
    for (const label of ["登录后自动启动", "退出应用时自动停止"]) {
      await expect(daemon.getByText(label, { exact: true })).toBeVisible();
    }
    const preferences = daemon.getByRole("switch");
    await expect(preferences).toHaveCount(2);
    for (const preference of await preferences.all()) {
      await expect(preference).toBeEnabled();
      await expect(preference).not.toBeChecked();
    }
    const diagnostics = daemon.locator("section").filter({
      has: page.getByRole("heading", { name: "诊断信息", exact: true }),
    });
    for (const label of ["运行状态", "已停止", "已运行时长", "守护进程 ID", "服务器地址", "默认（default）"]) {
      await expect(diagnostics.getByText(label, { exact: true })).toBeVisible();
    }
    const daemonScreenshot = testInfo.outputPath("desktop-daemon-settings.png");
    await page.screenshot({ path: daemonScreenshot });
    await testInfo.attach("desktop-daemon-settings", { path: daemonScreenshot, contentType: "image/png" });

    // The existing fixture isolates native IPC. This proves renderer/preload
    // wiring and stopped-state presentation, never real daemon/model execution.
    const services = await desktop.evaluate(() => (globalThis as unknown as {
      changelogAcceptance: { daemonStarts: number; externalLinks: string[]; installCalls: number };
    }).changelogAcceptance);
    expect(services).toEqual({ daemonStarts: 0, externalLinks: [], installCalls: 0 });
    expect(pageErrors).toEqual([]);
  } catch (error) {
    if (page && !page.isClosed()) {
      await testInfo.attach("desktop-settings-failure", {
        body: await page.screenshot(), contentType: "image/png",
      });
      await testInfo.attach("desktop-settings-failure-text", {
        body: `${await page.locator("body").innerText()}\n\n${pageErrors.join("\n")}`,
        contentType: "text/plain",
      });
    }
    throw error;
  } finally {
    try {
      await desktop?.close();
    } finally {
      try {
        if (workspaceId) {
          await api.deleteFeatureWorkspace(workspaceId);
          expect((await api.getWorkspaces()).some((workspace) => workspace.id === workspaceId)).toBe(false);
        }
      } finally {
        rmSync(profile, { recursive: true, force: true });
      }
    }
  }
});
