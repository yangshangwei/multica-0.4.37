import { writeFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import { p1Session } from "../../../../../../e2e/fixtures/project-p1";

test("verify project welcome actions through the existing application flows", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  const origin = info.project.use.baseURL;
  if (typeof origin !== "string") throw new Error("Task-owned Web origin is required");
  const errors: string[] = [];
  const writes: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    if (request.method() !== "GET" && /\/api\/(projects|issues|agents|squads)(\/|\?|$)/.test(request.url())) {
      writes.push(`${request.method()} ${new URL(request.url()).pathname}`);
    }
  });
  try {
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: origin }]);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${workspace.slug}/projects`);
    const welcomeTitle = "Bring issues, agents and AI squads together around a shared goal";
    const welcome = page.getByRole("region", { name: welcomeTitle, exact: true });
    const create = welcome.getByRole("button", { name: "Create your first project", exact: true });
    await expect(create).toBeVisible();
    expect(writes).toEqual([]);
    await create.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText("New Project", { exact: true })).toBeVisible();
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).not.toBeVisible();
    const docs = welcome.getByRole("link", { name: "Learn about projects", exact: true });
    await docs.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/docs/projects$`));
    await expect(page.getByRole("heading", { name: /^(Projects|项目)$/ }).first()).toBeVisible();
    expect(writes).toEqual([]);
    const created = await api.requestJSON<{ id: string }>("/api/projects", {
      method: "POST", body: { title: "Registration activation QA", execution_squads: [] },
    });
    await page.goto(`/${workspace.slug}/projects`);
    await expect(page.getByRole("link", { name: "Registration activation QA", exact: true })).toBeVisible();
    await expect(page.getByRole("region", { name: welcomeTitle, exact: true })).toHaveCount(0);
    expect(errors).toEqual([]);
    await writeFile(info.outputPath("actions-evidence.json"), JSON.stringify({
      renderer: "production Web / Chromium", workspace: workspace.slug, createdProject: created.id,
      keyboardCreate: true, existingCreateModal: true, existingProjectGuide: true,
      passiveExample: writes.length === 0, populatedDirectoryTransition: true, pageErrors: errors,
    }, null, 2) + "\n");
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
