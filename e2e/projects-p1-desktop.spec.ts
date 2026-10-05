import { expect } from "@playwright/test";
import { test, openNativeProject } from "./fixtures/project-p1-desktop";
import { progressDraft, seedProgress } from "./fixtures/project-p1-progress";
import { p1Project, p1Capture, p1NoOverflow, p1Overview } from "./fixtures/project-p1";

test("P1-D01 native navigation opens the exact project risk list from the keyboard", async ({ native }) => {
  const { page, api } = native;
  const project = await p1Project(api, { title: "Native risk navigation" });
  const issue = await api.createIssue("Native overdue delivery task", { project_id: project.id, status: "todo", due_date: "2020-01-01" });
  await openNativeProject(page, project.title);
  await page.getByRole("button", { name: "Overdue 1", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("link", { name: /Native overdue delivery task/ })).toBeVisible();
  await page.getByRole("button", { name: "Overview", exact: true }).last().click();
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
  expect((await p1Overview(api, project.id)).statistics.counts.total).toBe(1);
  expect(await api.countIssueDispatches(issue.id)).toBe(0);
});

test("P1-D02 a native fast preview survives the editor flush and publishes once", async ({ native }, info) => {
  const { page, api } = native;
  const project = await p1Project(api, { title: "Native fast preview" });
  await openNativeProject(page, project.title);
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  const composer = page.locator('[aria-label="Write progress"]');
  await composer.locator('[contenteditable="true"]').fill("Native customer milestone with trailing space ");
  const typedAt = Date.now();
  await composer.getByRole("button", { name: "Preview publication", exact: true }).click();
  expect(Date.now() - typedAt, "Exercise preview before the 300ms editor flush").toBeLessThan(300);
  await expect(composer.getByText("No members will be notified", { exact: true })).toBeVisible();
  // This timing regression must remain publishable beyond the known flush deadline.
  await page.waitForTimeout(350);
  await expect(composer.getByRole("button", { name: "Publish", exact: true })).toBeVisible();
  await expect(composer.getByRole("button", { name: "Preview publication", exact: true })).toHaveCount(0);
  await composer.getByRole("button", { name: "Publish", exact: true }).click();
  await expect(composer).toHaveCount(0);
  await expect(page.getByText("Native customer milestone with trailing space", { exact: true })).toBeVisible();
  expect((await api.requestJSON<{ items: unknown[] }>(`/api/projects/${project.id}/updates`)).items).toHaveLength(1);
  await p1Capture(page, info, "native-fast-preview-published");
});

test("P1-D03 cancelling and reloading restores the last native draft without publishing", async ({ native }) => {
  const { page, api } = native;
  const project = await p1Project(api, { title: "Native durable draft" });
  await openNativeProject(page, project.title);
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  const composer = page.locator('[aria-label="Write progress"]');
  await composer.locator('[contenteditable="true"]').fill("Last native keystrokes are still here");
  await composer.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(composer).toHaveCount(0);
  await page.reload();
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  await expect(composer.locator('[contenteditable="true"]')).toContainText("Last native keystrokes are still here");
  expect((await api.requestJSON<{ items: unknown[] }>(`/api/projects/${project.id}/updates`)).items).toHaveLength(0);
});

test("P1-D04 native revision history renders the original and corrected progress", async ({ native }, info) => {
  const { page, api } = native;
  const project = await p1Project(api, { title: "Native immutable history" });
  const created = await seedProgress(api, project, progressDraft("Original native milestone"));
  await seedProgress(api, project, progressDraft("Corrected native milestone", {
    operation: "correct", update_id: created.update_id, expected_revision: 1,
    correction_reason: "Correct the native delivery date",
  }));
  await openNativeProject(page, project.title);
  await expect(page.getByText("Corrected native milestone", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Revision history", exact: true }).click();
  const history = page.getByRole("heading", { name: "Revision history", exact: true }).locator("..");
  await expect(history.getByText("Original native milestone", { exact: true })).toBeVisible();
  await expect(history.getByText("Correct the native delivery date", { exact: false })).toBeVisible();
  await p1Capture(page, info, "native-revision-history");
});

test("P1-D05 a Chinese native window keeps progress actions readable at 680px", async ({ native }, info) => {
  const { page, api, desktop } = native;
  const project = await p1Project(api, { title: "Native Chinese compact project" });
  await seedProgress(api, project, progressDraft("桌面端保留的客户交付进展"));
  await openNativeProject(page, project.title);
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
  await page.context().addInitScript(() => localStorage.setItem("multica-locale", "zh-Hans"));
  await page.reload();
  await desktop.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0]!.setContentSize(680, 900));
  await expect(page.getByRole("button", { name: "记录进展", exact: true })).toBeVisible();
  await p1NoOverflow(page);
  await expect(page.getByText("桌面端保留的客户交付进展", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "修订历史", exact: true }).click();
  await expect(page.getByRole("heading", { name: "修订历史", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "记录进展", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("button", { name: "预览发布", exact: true })).toBeVisible();
  await p1Capture(page, info, "native-chinese-680");
});

test("P1-D06 native project tabs do not share progress drafts", async ({ native }) => {
  const { page, api } = native;
  const first = await p1Project(api, { title: "Native first draft scope" });
  const second = await p1Project(api, { title: "Native second draft scope" });
  await openNativeProject(page, first.title);
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  const composer = page.locator('[aria-label="Write progress"]');
  await composer.locator('[contenteditable="true"]').fill("Text belongs only to the first project");
  await composer.getByRole("button", { name: "Cancel", exact: true }).click();
  await openNativeProject(page, second.title);
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  await expect(composer.locator('[contenteditable="true"]')).not.toContainText("Text belongs only to the first project");
  await composer.getByRole("button", { name: "Cancel", exact: true }).click();
  await openNativeProject(page, first.title);
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  await expect(composer.locator('[contenteditable="true"]')).toContainText("Text belongs only to the first project");
  expect((await api.requestJSON<{ items: unknown[] }>(`/api/projects/${first.id}/updates`)).items).toHaveLength(0);
  expect((await api.requestJSON<{ items: unknown[] }>(`/api/projects/${second.id}/updates`)).items).toHaveLength(0);
});
