import { test, expect, _electron as electron } from "@playwright/test";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { TestApiClient } from "./fixtures";

test("desktop Help, tab restoration and updater action use the in-app reader", async () => {
  test.skip(!process.env.CHANGELOG_ELECTRON_RENDERER_URL || !process.env.CHANGELOG_E2E_API_URL, "Requires built desktop preload and a local renderer/API fixture");
  test.setTimeout(90_000);
  const root = resolve(import.meta.dirname, "..");
  const profile = mkdtempSync(join(tmpdir(), "multica-changelog-electron-"));
  const api = new TestApiClient();
  await api.login(`changelog-desktop-${Date.now()}@example.invalid`, "Desktop changelog acceptance");
  const workspace = await api.ensureWorkspace("Desktop changelog acceptance", `changelog-desktop-${Date.now()}`);
  await api.markUserOnboarded();
  const executablePath = process.platform === "darwin"
    ? join(root, "apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron")
    : join(root, "apps/desktop/node_modules/electron/dist/electron");
  const desktop = await electron.launch({
    executablePath,
    args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
    env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile },
  });
  const errors: string[] = [];
  desktop.process().stderr?.on("data", (data) => errors.push(String(data)));
  try {
    const page = await desktop.firstWindow();
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("requestfailed", (request) => errors.push(`${request.method()} ${request.url()}: ${request.failure()?.errorText}`));
    await page.waitForURL(new URL(process.env.CHANGELOG_ELECTRON_RENDERER_URL!).href);
    await page.locator("#root > *").first().waitFor({ state: "attached", timeout: 30_000 });
    await page.evaluate((token) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, api.getToken());
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "帮助", exact: true })).toBeVisible({ timeout: 45_000 });
    await page.getByRole("button", { name: "帮助", exact: true }).click();
    await page.getByRole("menuitem", { name: "变更说明", exact: true }).click();
    await expect(page.getByRole("heading", { name: "变更说明", exact: true })).toBeVisible();
    await expect(page.locator('[data-tab-active="true"]')).toHaveAttribute("aria-label", "变更说明");
    await expect(page.getByText("当前桌面版本", { exact: true })).toBeVisible();
    await expect(page.getByText("0.4.40-test", { exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "内网协作、团队模板与中文体验" })).toBeVisible();
    await page.mouse.move(850, 60);
    await page.screenshot({ path: join(root, ".omx/state/desktop-changelog/reader-electron.png") });

    const headerTop = await page.getByRole("heading", { name: "变更说明", exact: true }).evaluate((node) => node.getBoundingClientRect().top);
    const historyLink = page.getByRole("navigation", { name: "浏览版本" }).getByRole("link", { name: /v0\.4\.37/ });
    await historyLink.click();
    const release = page.getByRole("heading", { name: "任务列表更快，长时间执行更稳定" });
    await expect(release).toBeFocused();
    expect(await page.getByRole("heading", { name: "变更说明", exact: true }).evaluate((node) => node.getBoundingClientRect().top)).toBe(headerTop);
    const body = page.locator('[data-tab-scroll-root="changelog"]');
    await body.evaluate((node) => { node.scrollTop = 0; });
    await historyLink.click();
    expect(await body.evaluate((node) => node.scrollTop)).toBeGreaterThan(0);
    await body.evaluate((node) => { node.scrollTop = 320; });
    await page.getByRole("button", { name: "New tab", exact: true }).click();
    await page.locator('[data-tab-id] [aria-label="变更说明"]').click();
    await expect.poll(() => page.locator('[data-tab-scroll-root="changelog"]').evaluate((node) => node.scrollTop)).toBe(320);

    await desktop.evaluate(({ BrowserWindow }) => {
      BrowserWindow.getAllWindows()[0]!.webContents.send("updater:update-downloaded", { version: "v999.0.0" });
    });
    await expect(page.getByRole("button", { name: "查看变更说明", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "查看变更说明", exact: true }).click();
    await expect(page.getByText("当前部署的历史中还没有此版本。", { exact: true })).toBeVisible();
    const services = await desktop.evaluate(() => (globalThis as unknown as { changelogAcceptance: { daemonStarts: number; externalLinks: string[] } }).changelogAcceptance);
    expect(services.daemonStarts).toBe(0);
    expect(services.externalLinks).toEqual([]);
  } catch (error) {
    const page = desktop.windows()[0];
    if (page) {
      await page.screenshot({ path: join(root, ".omx/state/desktop-changelog/electron-error.png") });
      writeFileSync(join(root, ".omx/state/desktop-changelog/electron-error.txt"), `${await page.locator("body").innerText()}\n\n${errors.join("\n")}`);
    }
    throw error;
  } finally {
    await desktop.close();
    await api.deleteFeatureWorkspace(workspace.id);
    rmSync(profile, { recursive: true, force: true });
  }
});
