import { expect } from "@playwright/test";

// Packaged Main's Node controller cannot import the TypeScript fixture helper.
export async function selectCollaborationRecipient(page, form, recipient) {
  await form.getByRole("button", { name: "Recipient", exact: true }).click();
  const list = page.getByRole("listbox", { name: "Recipient", exact: true });
  const popup = page.locator('[data-slot="popover-content"]').filter({ has: list });
  await popup.getByRole("textbox", { name: "Filter options", exact: true }).fill(recipient.id);
  const lookupLabel = "Find by full username, email, or account ID";
  await popup.getByRole("button", { name: lookupLabel, exact: true }).click();
  await popup.getByLabel(lookupLabel, { exact: true }).fill(recipient.id);
  await popup.getByRole("button", { name: "Find account", exact: true }).click();
  await expect(popup.getByText(recipient.name, { exact: true })).toBeVisible();
  await popup.getByRole("button", { name: "Use this account", exact: true }).click();
  await expect(form.getByText(`Selected account: ${recipient.name}`, { exact: true })).toBeVisible();
}
