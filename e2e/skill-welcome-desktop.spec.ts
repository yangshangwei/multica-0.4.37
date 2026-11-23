import { expect } from "@playwright/test";
import { test } from "./fixtures/project-p1-desktop";
import { nativeScreenshot } from "./fixtures/collaboration-native";

test("Electron skills welcome keeps creation and the carousel reachable in a narrow pane", async ({ native }, info) => {
  const { page, desktop, workspace } = native;
  const resources = page.getByRole("navigation", { name: "Application navigation", exact: true })
    .getByRole("button", { name: "Resources & connections", exact: true });
  if (await resources.getAttribute("aria-expanded") !== "true") await resources.click();
  const skills = page.getByRole("link", { name: "Skills", exact: true }).and(page.locator('[data-sidebar="menu-button"]'));
  await skills.click();
  const welcome = page.getByRole("region", { name: "Turn your team’s know-how into a skill", exact: true });
  await expect(welcome).toBeVisible();
  const create = welcome.getByRole("button", { name: "New skill", exact: true });
  await create.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Modify from template", { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "Close", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(create).toBeFocused();
  await expect(welcome.getByRole("link", { name: "Read the docs", exact: true })).toHaveAttribute("href", new URL(`/${workspace.slug}/docs/skills`, page.url()).href);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.evaluate(() => document.fonts.ready);
  await nativeScreenshot(desktop, page, info.outputPath("desktop-welcome-wide.png"));

  const window = await desktop.browserWindow(page);
  try {
    await window.evaluate((target) => target.setContentSize(720, 700));
    await welcome.evaluate((element) => { element.scrollTop = 0; });
    await expect(create).toBeInViewport();
    expect(await welcome.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    const next = welcome.getByRole("button", { name: "Next example", exact: true });
    await next.scrollIntoViewIfNeeded();
    await expect(next).toBeInViewport();
    await next.click();
    await expect(welcome.getByRole("heading", { name: "Assign it to your agents", exact: true })).toBeVisible();
    await page.evaluate(() => new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
    await nativeScreenshot(desktop, page, info.outputPath("desktop-welcome-narrow-scrolled.png"));
  } finally {
    await window.dispose();
  }
});
