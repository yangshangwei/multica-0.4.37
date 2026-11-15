import { expect, type Locator } from "@playwright/test";

// Editing, deletion, history and result preparation live in each comment
// row's menu. Its content is portaled, so items are found from the page.

/** Opens one comment row's menu and returns the named item. */
export async function commentMenuItem(row: Locator, name: string, more = "Comment actions"): Promise<Locator> {
  await row.getByRole("button", { name: more, exact: true }).click();
  return row.page().getByRole("menuitem", { name, exact: true });
}

/** Proves the opened menu offers none of the named actions, then closes it. */
export async function expectCommentMenuWithout(row: Locator, names: string[]): Promise<void> {
  const page = row.page();
  // History is offered on every comment, so its presence proves the menu is open.
  await expect(await commentMenuItem(row, "Comment history")).toBeVisible();
  for (const name of names) await expect(page.getByRole("menuitem", { name, exact: true })).toHaveCount(0);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("menuitem", { name: "Comment history", exact: true })).toHaveCount(0);
}

/** Opens a comment's history if needed and returns its region inside the row. */
export async function openCommentHistory(row: Locator): Promise<Locator> {
  const region = row.getByRole("region", { name: "Comment history", exact: true });
  if (await region.count() === 0) await (await commentMenuItem(row, "Comment history")).click();
  await expect(region).toBeVisible();
  return region;
}

export async function closeCommentHistory(row: Locator): Promise<void> {
  await row.getByRole("button", { name: "Hide history", exact: true }).click();
  await expect(row.getByRole("region", { name: "Comment history", exact: true })).toHaveCount(0);
}
