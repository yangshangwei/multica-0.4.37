import { randomUUID } from "node:crypto";
import { test, expect, type Response } from "@playwright/test";
import { TestApiClient } from "./fixtures";

type Issue = {
  id: string;
  identifier: string;
  title: string;
  description: string;
  status: string;
  priority: string;
};

function responseFor(response: Response, path: string, method: string) {
  return new URL(response.url()).pathname.replace(/\/$/, "") === path
    && response.request().method() === method;
}

test.use({ screenshot: "only-on-failure", trace: "retain-on-failure", actionTimeout: 15_000 });

test("persists a manually created feature request through edits, workflow, comments and confirmed deletion", async ({ page }, info) => {
  test.setTimeout(180_000);
  const suffix = randomUUID().slice(0, 8);
  const api = new TestApiClient();
  await api.login(`lifecycle-${suffix}@multica.ai`, "Lifecycle QA owner");
  const workspace = await api.ensureWorkspace("Lifecycle QA", `lifecycle-${suffix}`);
  try {
    await api.markUserOnboarded();
    const token = api.getToken();
    if (!token) throw new Error("Fixture authentication did not return a token");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("multica_create_mode", JSON.stringify({ state: { lastMode: "manual" }, version: 0 }));
    }, token);
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "New Issue", exact: true }).click();
    const title = `Team weekly report ${suffix}`;
    const description = "Let team members write a weekly report and share progress with their manager.";
    const titleInput = page.getByRole("textbox", { name: "Issue title", exact: true });
    await titleInput.fill(title);
    const dialog = page.getByRole("dialog").filter({ has: titleInput });
    await dialog.locator('.ProseMirror[contenteditable="true"]:not([aria-label="Issue title"])').fill(description);
    const createdResponse = page.waitForResponse((response) => responseFor(response, "/api/issues", "POST"));
    await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
    const created = await createdResponse;
    expect(created.ok()).toBe(true);
    const issue: Issue = await created.json();
    expect(issue).toMatchObject({ title, description });
    await expect(dialog).toBeHidden();
    await page.getByRole("button", { name: "View issue", exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/issues/${issue.identifier}$`), { timeout: 60_000 });
    await expect(page.getByRole("button", { name: "Properties", exact: true })).toBeVisible();

    const updatedTitle = `Weekly report for the product team ${suffix}`;
    const updatedDescription = "Each team member submits progress, next steps and blockers. Managers can read the current week's reports in one list.";
    await page.getByRole("button", { name: title, exact: true }).click();
    await page.getByRole("textbox", { name: "Issue title", exact: true }).fill(updatedTitle);
    const descriptionEditor = page.locator('.ProseMirror[contenteditable="true"]').filter({ hasText: description });
    await descriptionEditor.fill(updatedDescription);
    await page.getByRole("button", { name: "Todo", exact: true }).click();
    await page.locator("button[data-picker-item]").filter({ hasText: /^In Progress$/ }).click();
    await page.getByRole("button", { name: "Add property", exact: true }).click();
    await page.getByRole("button", { name: "Priority", exact: true }).click();
    await page.locator("button[data-picker-item]").filter({ hasText: /^High$/ }).click();
    await expect.poll(() => api.requestJSON<Issue>(`/api/issues/${issue.id}`), { timeout: 15_000 }).toMatchObject({
      title: updatedTitle, description: updatedDescription, status: "in_progress", priority: "high",
    });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: updatedTitle, exact: true })).toBeVisible({ timeout: 30_000 });
    await expect(page.locator('.ProseMirror[contenteditable="true"]').filter({ hasText: updatedDescription })).toBeVisible();
    await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "High", exact: true })).toBeVisible();

    await page.getByTestId("comment-composer-shell").click();
    const comment = `Start with manual weekly reports; automatic task summaries can follow. ${suffix}`;
    const composer = page.locator('div.rounded-lg.bg-card').filter({ has: page.getByRole("button", { name: "Send", exact: true }) }).locator('.ProseMirror[contenteditable="true"]');
    await composer.fill(comment);
    const submitted = page.waitForResponse((response) => responseFor(response, `/api/issues/${issue.id}/comments`, "POST"));
    await composer.press("ControlOrMeta+Enter");
    const commentResponse = await submitted;
    expect(commentResponse.ok()).toBe(true);
    const savedComment: { id: string } = await commentResponse.json();
    await expect(page.locator(`#comment-body-${savedComment.id}`)).toHaveText(comment);
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: updatedTitle, exact: true })).toBeVisible({ timeout: 30_000 });
    await expect(page.locator(`#comment-body-${savedComment.id}`)).toHaveText(comment);
    await page.screenshot({ path: info.outputPath("persisted-feature-task.png"), animations: "disabled" });

    const actions = page.locator('main button[data-slot="dropdown-menu-trigger"]:not([aria-label]):has(svg.lucide-ellipsis)');
    await actions.click();
    await page.getByRole("menuitem", { name: "Delete issue", exact: true }).click();
    const confirmation = page.getByRole("alertdialog", { name: "Delete issue", exact: true });
    await expect(confirmation).toBeVisible();
    await confirmation.getByRole("button", { name: "Cancel", exact: true }).click();
    expect(await api.requestJSON<Issue>(`/api/issues/${issue.id}`)).toMatchObject({ id: issue.id });
    await actions.click();
    await page.getByRole("menuitem", { name: "Delete issue", exact: true }).click();
    const deleted = page.waitForResponse((response) => responseFor(response, `/api/issues/${issue.id}`, "DELETE"));
    await confirmation.getByRole("button", { name: "Delete", exact: true }).click();
    expect((await deleted).ok()).toBe(true);
    await expect(confirmation).toBeHidden();
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/issues$`));
    await expect(page.locator(`a[href$="/issues/${issue.id}"], a[href$="/issues/${issue.identifier}"]`)).toHaveCount(0);
    await expect(api.requestJSON(`/api/issues/${issue.id}`)).rejects.toThrow("404");
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
