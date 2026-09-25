import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

type Workspace = { id: string; slug: string };
type Project = {
  id: string; title: string;
  execution_squad: { state: string; template_key?: string; squad_id?: string; runtime_id?: string };
  execution_squads: { state: string; template_key?: string; squad_id?: string; runtime_id?: string }[];
};

async function signIn(page: Page, suffix: string) {
  const api = new TestApiClient();
  const run = `${Date.now().toString(36)}-${suffix}`;
  await api.login(`e2e-defaults-${run}@multica.ai`, "Workspace defaults tester");
  const workspace = await api.ensureWorkspace(`Defaults ${suffix}`, `defaults-${run}`);
  await api.markUserOnboarded();
  const token = api.getToken();
  if (!token) throw new Error("Fixture login failed");
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
  return { api, workspace };
}

async function createProject(page: Page, workspace: Workspace, title: string, screenshotPath?: string): Promise<Project> {
  await page.goto(`/${workspace.slug}/projects`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: /create.*project/i }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "Choose execution squads" })).toContainText(/feature delivery/i);
  await dialog.getByRole("textbox", { name: "Project title", exact: true }).fill(title);
  await expect(dialog.getByRole("button", { name: "Choose execution squads" })).toBeInViewport();
  await expect(dialog.getByRole("button", { name: "Create Project", exact: true })).toBeEnabled();
  if (screenshotPath) await page.screenshot({ path: screenshotPath, animations: "disabled" });
  const saved = page.waitForResponse((response) => response.request().method() === "POST" && /\/api\/projects$/.test(response.url()));
  await dialog.getByRole("button", { name: "Create Project", exact: true }).click();
  const response = await saved;
  expect(response.status()).toBe(201);
  const project = await response.json() as Project;
  await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/projects/${project.id}$`), { timeout: 30_000 });
  await expect(page.getByRole("region", { name: "Execution squads", exact: true })).toBeVisible({ timeout: 30_000 });
  return project;
}

test.describe("workspace built-in defaults", () => {
  test.setTimeout(120_000);

  test("saves multiple squads, supports select-all and clear, and keeps creation usable on narrow screens", async ({ page }, info) => {
    const { api, workspace } = await signIn(page, "multi-squad");
    try {
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`/${workspace.slug}/projects`);
      await page.getByRole("button", { name: /create.*project/i }).click();
      const dialog = page.getByRole("dialog").first();
      await dialog.getByRole("textbox", { name: "Project title", exact: true }).fill("Website launch");
      await expect(dialog.getByText(/AI agents that plan, carry out, and review/)).toBeVisible();
      await page.screenshot({ path: info.outputPath("create-project-desktop.png"), animations: "disabled" });
      await dialog.getByRole("button", { name: "Choose execution squads" }).click();
      await page.getByRole("button", { name: "Select all", exact: true }).click();
      const checked = page.getByRole("checkbox", { checked: true });
      expect(await checked.count()).toBeGreaterThan(1);
      await page.getByRole("button", { name: "Clear", exact: true }).click();
      await expect(checked).toHaveCount(0);
      await page.getByRole("checkbox", { name: /Feature Delivery/i }).check();
      await page.getByRole("checkbox", { name: /Bug Fix/i }).check();
      await page.screenshot({ path: info.outputPath("create-project-multiselect.png"), animations: "disabled" });
      await page.keyboard.press("Escape");
      await expect(page.getByRole("checkbox", { name: /Feature Delivery/i })).not.toBeVisible();
      await page.setViewportSize({ width: 390, height: 844 });
      await expect(dialog.getByRole("button", { name: "Create Project", exact: true })).toBeInViewport();
      await page.screenshot({ path: info.outputPath("create-project-narrow.png"), animations: "disabled" });
      const bounds = await dialog.boundingBox();
      expect(bounds).not.toBeNull();
      expect(bounds!.x).toBeGreaterThanOrEqual(0);
      expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);
      await info.attach("narrow-layout", { contentType: "application/json", body: JSON.stringify(await page.evaluate(() => ({
        viewport: innerWidth, documentWidth: document.documentElement.scrollWidth,
        overflowingElements: [...document.querySelectorAll("body *")].filter((element) => {
          const rect = element.getBoundingClientRect();
          return rect.width > 0 && rect.right > innerWidth + 1 && getComputedStyle(element).visibility !== "hidden";
        }).slice(0, 8).map((element) => ({ tag: element.tagName, classes: element.className })),
      }))) });
      const saved = page.waitForResponse((response) => response.request().method() === "POST" && /\/api\/projects$/.test(response.url()));
      await dialog.getByRole("button", { name: "Create Project", exact: true }).click();
      const response = await saved;
      expect(response.status()).toBe(201);
      const project = await response.json() as Project;
      expect(project.execution_squads.map((squad) => squad.template_key)).toEqual(["feature-delivery", "bug-fix"]);
      expect(project.execution_squad.template_key).toBe("feature-delivery");
      await expect(page).toHaveURL(new RegExp(`/projects/${project.id}$`));
      await page.reload();
      await expect(page.getByText("Your project and squad choice are saved. Connect a runtime, then finish setup.")).toHaveCount(2);

      await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
      await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: new URL(page.url()).origin }]);
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`/${workspace.slug}/projects`);
      await page.getByRole("button", { name: "新建项目", exact: true }).click();
      const localizedDialog = page.getByRole("dialog").first();
      await localizedDialog.getByRole("textbox", { name: "请输入项目标题（必填）", exact: true }).fill("官网改版");
      await expect(localizedDialog.getByText(/由多个智能体分工协作/)).toBeVisible();
      await page.screenshot({ path: info.outputPath("create-project-zh-desktop.png"), animations: "disabled" });
      await page.setViewportSize({ width: 390, height: 844 });
      await expect(localizedDialog.getByRole("button", { name: "创建项目", exact: true })).toBeInViewport();
      await page.screenshot({ path: info.outputPath("create-project-zh-narrow.png"), animations: "disabled" });
    } finally {
      await api.deleteFeatureWorkspace(workspace.id);
    }
  });

  test("keeps built-in resources available on request and retains a project without a runtime", async ({ page }, info) => {
    const { api, workspace } = await signIn(page, "no-runtime");
    try {
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`/${workspace.slug}/squads`);
      await expect(page.getByRole("tab", { name: "Workspace squads" })).toHaveAttribute("aria-selected", "true");
      await expect(page.getByText("Feature Delivery Squad", { exact: true })).toHaveCount(0);
      await page.getByRole("tab", { name: "Squad templates" }).click();
      await expect(page.getByRole("tab", { name: "Squad templates" })).toHaveAttribute("aria-selected", "true");
      await expect(page.getByRole("region", { name: "Squad templates" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Feature Delivery Squad", exact: true })).toBeVisible();
      await page.screenshot({ path: info.outputPath("builtin-squads-desktop.png"), animations: "disabled" });
      await page.goto(`/${workspace.slug}/skills`);
      await expect(page.getByRole("heading", { name: /built-in/i })).toBeVisible();
      await page.goto(`/${workspace.slug}/agents`);
      await expect(page.getByRole("heading", { name: /built-in/i })).toBeVisible();

      const project = await createProject(page, workspace, "Starter without a runtime", info.outputPath("project-create-desktop.png"));
      expect(project.execution_squad).toMatchObject({ state: "needs_runtime", template_key: "feature-delivery" });
      await expect(page.getByText("Your project and squad choice are saved. Connect a runtime, then finish setup.")).toBeVisible({ timeout: 30_000 });
      await page.reload();
      await expect(page.getByText("Your project and squad choice are saved. Connect a runtime, then finish setup.")).toBeVisible({ timeout: 30_000 });
      const saved = await api.requestJSON<{ projects: Project[] }>("/api/projects");
      expect(saved.projects).toHaveLength(1);
      expect((await api.requestJSON<unknown[]>("/api/squads"))).toHaveLength(0);
      await page.setViewportSize({ width: 430, height: 932 });
      await page.screenshot({ path: info.outputPath("project-needs-runtime-narrow.png"), animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    } catch (error) {
      await page.screenshot({ path: info.outputPath("failure-before-cleanup.png"), animations: "disabled" });
      throw error;
    } finally {
      await api.deleteFeatureWorkspace(workspace.id);
    }
  });

  test("prepares a squad, dispatches an issue and enables an automation only on submission", async ({ page }, info) => {
    const { api, workspace } = await signIn(page, "ready");
    try {
      await api.seedProjectRuntime();
      await page.setViewportSize({ width: 1440, height: 1000 });
      const project = await createProject(page, workspace, "Starter delivery");
      expect(project.execution_squad.state).toBe("configured");
      expect(project.execution_squad.squad_id).toBeTruthy();
      await expect(page.getByRole("button", { name: "Hand to squad" })).toBeEnabled();
      await page.screenshot({ path: info.outputPath("project-ready-desktop.png"), animations: "disabled" });
      const squads = await api.requestJSON<{ id: string }[]>("/api/squads");
      expect(squads).toHaveLength(1);
      const repeated = await api.requestJSON<Project>(`/api/projects/${project.id}/execution-squad`, {
        method: "PUT", body: { template_key: "feature-delivery", runtime_id: project.execution_squad.runtime_id },
      });
      expect(repeated.execution_squad.squad_id).toBe(project.execution_squad.squad_id);
      expect(await api.requestJSON<unknown[]>("/api/squads")).toHaveLength(1);

      // The ordinary project empty-state action inherits the same default
      // squad; users do not have to discover the dedicated shortcut first.
      await page.locator("main").getByRole("button", { name: "New Issue", exact: true }).click();
      const issueDialog = page.getByRole("dialog");
      await issueDialog.getByRole("textbox", { name: "Issue title", exact: true }).fill("Verify the starter workflow");
      await expect(issueDialog.getByRole("button", { name: "Create Issue", exact: true })).toBeEnabled();
      await page.screenshot({ path: info.outputPath("incumbent-issue-dialog-reference.png"), animations: "disabled" });
      const createdIssue = page.waitForResponse((response) => response.request().method() === "POST" && /\/api\/issues$/.test(response.url()));
      await issueDialog.getByRole("button", { name: "Create Issue", exact: true }).click();
      const issueResponse = await createdIssue;
      expect(issueResponse.status()).toBe(201);
      expect(issueResponse.request().postDataJSON()).toMatchObject({
        project_id: project.id, assignee_type: "squad", assignee_id: project.execution_squad.squad_id, status: "todo",
      });
      const issue = await issueResponse.json() as { id: string };
      await expect.poll(() => api.countIssueDispatches(issue.id)).toBeGreaterThan(0);

      await page.goto(`/${workspace.slug}/projects/${project.id}`);
      await page.getByRole("link", { name: "Add automation", exact: true }).click();
      await page.getByText("Daily Change Review", { exact: true }).first().click();
      await expect(page.getByRole("button", { name: "Enable automation", exact: true })).toBeEnabled();
      expect((await api.requestJSON<{ autopilots: unknown[] }>("/api/autopilots")).autopilots).toHaveLength(0);
      await page.screenshot({ path: info.outputPath("automation-project-prefill.png"), animations: "disabled" });
      const createdAutomation = page.waitForResponse((response) => response.request().method() === "POST" && /\/api\/autopilots\/from-template$/.test(response.url()));
      await page.getByRole("button", { name: "Enable automation", exact: true }).click();
      const automationResponse = await createdAutomation;
      expect(automationResponse.status()).toBe(201);
      expect(automationResponse.request().postDataJSON()).toMatchObject({
        project_id: project.id, assignee_type: "squad", assignee_id: project.execution_squad.squad_id,
      });
      const automations = await api.requestJSON<{ autopilots: unknown[] }>("/api/autopilots");
      expect(automations.autopilots).toHaveLength(1);
    } catch (error) {
      await page.screenshot({ path: info.outputPath("failure-before-cleanup.png"), animations: "disabled" });
      throw error;
    } finally {
      // Deletes queued work and the fake runtime; no daemon ever claims it.
      await api.deleteFeatureWorkspace(workspace.id);
    }
  });
});
