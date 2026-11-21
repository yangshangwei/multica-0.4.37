import { expect, type Page } from "@playwright/test";

interface SidebarOptions {
  timeout?: number;
}

const navigationName = /^(Application navigation|应用导航)$/;

export function sidebarNewButton(page: Page) {
  return page.getByRole("navigation", { name: navigationName }).getByRole("button", { name: /^(New|新建)$/ });
}

export async function openSidebarNavigation(page: Page, { timeout = 15_000 }: SidebarOptions = {}) {
  const navigation = page.getByRole("navigation", { name: navigationName });
  const trigger = page.locator('[data-slot="sidebar-trigger"]:visible').first();
  const compact = await page.evaluate(() => matchMedia("(max-width: 1023px)").matches);
  if (compact) {
    // A breakpoint change first retires the desktop branch. A closed sheet's
    // portal need not contain a navigation element until it is opened.
    await expect(page.locator('[data-slot="sidebar-container"]')).toHaveCount(0, { timeout });
  }
  await expect.poll(async () => await navigation.isVisible() || await trigger.isVisible(), { timeout }).toBe(true);
  if (!await navigation.isVisible() || await page.locator('[data-slot="sidebar"][data-state="collapsed"]').count() > 0) {
    await trigger.click({ timeout });
  }
  await expect(navigation).toBeVisible({ timeout });
  if (compact) {
    const sheet = page.locator('[data-slot="sidebar"][data-mobile="true"]');
    await expect.poll(() => sheet.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      const overlay = document.querySelector<HTMLElement>('[data-slot="sheet-overlay"]');
      const surfaces = overlay ? [element, overlay] : [element];
      return bounds.width > 0 && bounds.left >= -0.5 && bounds.right <= innerWidth + 0.5
        && bounds.top >= -0.5 && bounds.bottom <= innerHeight + 0.5
        && surfaces.every((surface) => getComputedStyle(surface).opacity === "1"
          && !surface.hasAttribute("data-starting-style") && !surface.hasAttribute("data-ending-style")
          && surface.getAnimations().every((animation) => animation.playState !== "running" && !animation.pending))
        && element.contains(document.elementFromPoint(bounds.left + bounds.width / 2, bounds.top + bounds.height / 2));
    }), { timeout, message: "Compact navigation must be painted inside the viewport after its sheet transition" }).toBe(true);
  }
  return navigation;
}

export async function openNewIssue(page: Page, options: SidebarOptions = {}) {
  await openSidebarNavigation(page, options);
  await sidebarNewButton(page).click(options);
  await page.getByRole("menuitem", { name: /^(New Issue|新建任务)/ }).click(options);
}

export async function openNewCollaborationIssue(page: Page, options: SidebarOptions = {}) {
  await openSidebarNavigation(page, options);
  await sidebarNewButton(page).click(options);
  await page.getByRole("menuitem", { name: /^(New collaboration issue|新建协作任务)$/ }).click(options);
}

export async function openAccountMenu(page: Page) {
  const navigation = await openSidebarNavigation(page);
  await navigation.getByRole("button", { name: "Multica", exact: true }).click();
  await expect(page.getByRole("menuitem", { name: /^(Log out|退出登录)$/ })).toBeVisible();
}
