import { expect } from "@playwright/test";
import { test } from "./fixtures/project-p1-desktop";
import { nativeScreenshot } from "./fixtures/collaboration-native";

test("Electron MCP welcome retains real creation and reachable manual examples in a narrow pane", async ({ native }, info) => {
  const { page, desktop } = native;
  const resources = page.getByRole("navigation", { name: "Application navigation", exact: true })
    .getByRole("button", { name: "Resources & connections", exact: true });
  if (await resources.getAttribute("aria-expanded") !== "true") await resources.click();
  const mcp = page.getByRole("link", { name: "MCP", exact: true }).and(page.locator('[data-sidebar="menu-button"]'));
  await mcp.click();
  const welcome = page.getByRole("region", { name: "Connect your agents to your tools", exact: true });
  await expect(welcome).toBeVisible();
  await expect(page.getByRole("heading", { level: 1, name: "MCP", exact: true })).toHaveCount(1);
  const create = welcome.getByRole("button", { name: "Add custom server", exact: true });
  await expect(page.getByRole("button", { name: "Add custom server", exact: true })).toHaveCount(1);
  await create.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("textbox", { name: "Server name", exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(create).toBeFocused();
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.evaluate(() => document.fonts.ready);
  await nativeScreenshot(desktop, page, info.outputPath("desktop-mcp-welcome-wide.png"));

  const window = await desktop.browserWindow(page);
  try {
    await window.evaluate((target) => target.setContentSize(720, 700));
    await welcome.evaluate((element) => { element.scrollTop = 0; });
    await expect(welcome.getByRole("button", { name: "Browse capability market", exact: true })).toBeInViewport();
    expect(await welcome.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    const next = welcome.getByRole("button", { name: "Next example", exact: true });
    await next.scrollIntoViewIfNeeded();
    await expect(next).toBeInViewport();
    await next.click();
    await expect(welcome.getByRole("heading", { name: "Configure the connection", exact: true })).toBeVisible();
    await next.click();
    await expect(welcome.getByRole("heading", { name: "Assign it to your agents", exact: true })).toBeVisible();
    await expect(next).toBeDisabled();
    await expect(welcome.locator('[role="group"][inert][aria-hidden="true"]')).toHaveCount(2);
    await page.evaluate(() => new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
    await nativeScreenshot(desktop, page, info.outputPath("desktop-mcp-welcome-narrow-scrolled.png"));
  } finally {
    await window.dispose();
  }
});
