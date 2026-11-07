import { expect, type Page } from "@playwright/test";

type CollaborationDisclosure = "Attachments" | "Sub-issues" | "Knowledge candidates" | "AI execution" | "Activity" | "More actions";

// Use only for ordinary discovery. Anchors and pending requests must reveal
// their own sections without this helper so those recovery assertions stay real.
export async function revealCollaborationDisclosure(page: Page, name: CollaborationDisclosure) {
  if (name === "More actions") {
    const trigger = page.getByRole("button", { name, exact: true });
    if (await trigger.getAttribute("aria-expanded") !== "true") await trigger.click();
    const menu = page.getByRole("menu");
    await expect(menu).toBeVisible();
    return menu;
  }
  const exactName = new RegExp(`^${name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`);
  const summary = page.locator("details > summary:visible").filter({ hasText: exactName });
  await expect(summary).toBeVisible();
  const disclosure = summary.locator("..");
  if (await disclosure.getAttribute("open") === null) await summary.click();
  await expect(disclosure).toHaveAttribute("open", "");
  return disclosure;
}
