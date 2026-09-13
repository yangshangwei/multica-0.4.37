import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

type Workspace = { id: string; slug: string };
type Project = {
  id: string; title: string;
  execution_squad: { state: string; template_key?: string; squad_id?: string; runtime_id?: string };
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
  await expect(dialog.getByRole("combobox", { name: "Execution squad" })).toContainText(/feature delivery/i);
  await dialog.getByRole("textbox", { name: "Project title", exact: true }).fill(title);
  await expect(dialog.getByRole("combobox", { name: "Execution squad" })).toBeInViewport();
  await expect(dialog.getByRole("button", { name: "Create Project", exact: true })).toBeEnabled();
  if (screenshotPath) await page.screenshot({ path: screenshotPath, animations: "disabled" });
  const saved = page.waitForResponse((response) => response.request().method() === "POST" && /\/api\/projects$/.test(response.url()));
  await dialog.getByRole("button", { name: "Create Project", exact: true }).click();
  const response = await saved;
  expect(response.status()).toBe(201);
  const project = await response.json() as Project;
  await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/projects/${project.id}$`), { timeout: 30_000 });
  await expect(page.getByRole("region", { name: "Execution squad", exact: true })).toBeVisible({ timeout: 30_000 });
  return project;
}

test.describe("workspace built-in defaults", () => {
  test.setTimeout(120_000);

  test("keeps built-in resources available on request and retains a project without a runtime", async ({ page }, info) => {
    const { api, workspace } = await signIn(page, "no-runtime");
    try {
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`/${workspace.slug}/squads`);
      await expect(page.getByRole("heading", { name: /built-in/i })).toBeVisible();
      const catalogToggle = page.getByRole("region", { name: "Built-in squads" }).getByRole("button", { name: /^Built-in squads/ });
      await expect(catalogToggle).toHaveAttribute("aria-expanded", "false");
      await expect(page.getByText("Feature Delivery Squad", { exact: true })).toHaveCount(0);
      await catalogToggle.click();
      await expect(catalogToggle).toHaveAttribute("aria-expanded", "true");
      await expect(page.getByText("Feature Delivery Squad", { exact: true }).first()).toBeVisible();
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
