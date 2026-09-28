import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

type Squad = { id: string; name: string; leader_id: string };
type Choice = { state: string; squad_id?: string; template_key?: string; runtime_id?: string };
type Project = { id: string; execution_squads: Choice[] };
type Issue = { id: string; title: string; status: string; assignee_type: string | null; assignee_id: string | null; project_id: string | null };

async function capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = info.outputPath(`${name}.png`);
  await page.screenshot({ path, animations: "disabled" });
  await info.attach(name, { path, contentType: "image/png" });
}

async function expectNoOverflow(page: Page) {
  expect(await page.evaluate(() => Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) <= innerWidth)).toBe(true);
}

test("keeps project work visible with 1, 8 and 20 squads and persists explicit defaults and issue links", async ({ page }, info) => {
  test.setTimeout(180_000);
  const api = new TestApiClient();
  const slug = `project-squads-${Date.now().toString(36)}-${process.pid}`;
  await api.login(`${slug}@multica.ai`, "Project squad tester");
  const workspace = await api.ensureWorkspace("Project workspace", slug);
  expect(workspace.slug).toBe(slug);
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));

  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, token);

    const runtime = await api.seedProjectRuntime();
    const { squad: first } = await api.requestJSON<{ squad: Squad }>("/api/squads/from-template", {
      method: "POST", body: { template_key: "feature-delivery", runtime_id: runtime.id, name: "Product delivery" },
    });
    const squads = [first];
    for (let index = 2; index <= 20; index += 1) {
      squads.push(await api.requestJSON<Squad>("/api/squads", {
        method: "POST", body: {
          name: index === 20 ? "Customer operations and international release readiness coordination" : `Delivery squad ${index}`,
          leader_id: first.leader_id,
          description: `Owns delivery stream ${index}, including validation and handover.`,
        },
      }));
    }
    const project = await api.requestJSON<Project>("/api/projects", {
      method: "POST", body: { title: "Customer portal launch", execution_squads: squads.slice(0, 8).map((squad) => ({ squad_id: squad.id })) },
    });
    const candidate = await api.createIssue("Review customer rollout checklist", { status: "backlog" }) as Issue;
    const otherProject = await api.requestJSON<Project>("/api/projects", { method: "POST", body: { title: "Other launch" } });
    const otherIssue = await api.createIssue("Review customer rollout elsewhere", { status: "backlog", project_id: otherProject.id }) as Issue;
    const path = `/${slug}/projects/${project.id}`;
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(path);
    const summary = page.getByRole("region", { name: "Execution squads", exact: true });
    const manage = summary.getByRole("button", { name: "Manage squads", exact: true });
    await expect(summary).toContainText("8 ready");
    await expect(summary.getByRole("link")).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Create your first issue", exact: true })).toBeVisible();
    await expect(page.locator("main").getByRole("button", { name: "New Issue", exact: true })).toHaveCount(1);
    expect((await summary.boundingBox())!.height).toBeLessThan(120);
    await capture(page, info, "eight-squads-empty-wide");

    await manage.focus();
    await page.keyboard.press("Enter");
    const manager = page.getByRole("dialog", { name: "Manage squads", exact: true });
    await expect(manager.getByRole("group", { name: /Delivery squad/ })).toHaveCount(7);
    await expect(manager.getByText("Owns delivery stream 2, including validation and handover.", { exact: true })).toBeVisible();
    await manager.getByRole("button", { name: "Actions for Delivery squad 2", exact: true }).click();
    const reordered = page.waitForResponse((response) => response.request().method() === "PUT" && response.url().endsWith(`/projects/${project.id}/execution-squads`));
    await page.getByRole("menuitem", { name: "Set as new issue default", exact: true }).click();
    expect((await reordered).ok()).toBe(true);
    await expect(manager.getByRole("group", { name: "Delivery squad 2", exact: true })).toContainText("New issue default");
    const saved = await api.requestJSON<Project>(`/api/projects/${project.id}`);
    expect(saved.execution_squads.map((choice) => choice.squad_id)).toEqual([squads[1]!.id, first.id, ...squads.slice(2, 8).map((squad) => squad.id)]);
    expect(await api.countIssueDispatches(candidate.id)).toBe(0);
    await capture(page, info, "eight-squads-manager-wide");
    await page.keyboard.press("Escape");
    await expect(manager).not.toBeVisible();
    await expect(manage).toBeFocused();

    await page.locator("main").getByRole("button", { name: "New Issue", exact: true }).click();
    let issueDialog = page.getByRole("dialog", { name: "New Issue", exact: true });
    await expect(issueDialog).toBeVisible();
    await issueDialog.getByRole("textbox", { name: "Issue title", exact: true }).fill("Validate the customer release");
    const created = page.waitForResponse((response) => response.request().method() === "POST" && /\/api\/issues$/.test(response.url()));
    await issueDialog.getByRole("button", { name: "Create Issue", exact: true }).click();
    const createdResponse = await created;
    expect(createdResponse.status()).toBe(201);
    expect(createdResponse.request().postDataJSON()).toMatchObject({ project_id: project.id, assignee_type: "squad", assignee_id: squads[1]!.id, status: "todo" });
    await page.goto(path);
    await expect(page.locator("main").getByRole("button", { name: "New Issue", exact: true })).toHaveCount(1);
    await expect(page.getByRole("heading", { name: "Create your first issue", exact: true })).not.toBeVisible();

    await page.getByRole("button", { name: "Link an existing issue", exact: true }).click();
    const picker = page.getByRole("dialog", { name: "Link an issue to this project", exact: true });
    await picker.getByRole("combobox").fill("Review customer rollout");
    await expect(picker.getByRole("option")).toHaveCount(1);
    await page.route(`**/api/issues/${candidate.id}`, async (route) => {
      if (route.request().method() !== "PUT") return route.continue();
      await route.fulfill({ status: 500, json: { error: "Temporary fixture failure" } });
    });
    await picker.getByRole("option").click();
    await expect(page.getByText("Could not link the issue. Try again.", { exact: true })).toBeVisible();
    await expect(picker).toBeVisible();
    await expect(picker.getByRole("combobox")).toBeEnabled();
    await page.unroute(`**/api/issues/${candidate.id}`);
    await picker.getByRole("option").click();
    await expect(picker).not.toBeVisible();
    const linked = await api.requestJSON<Issue>(`/api/issues/${candidate.id}`);
    expect(linked).toMatchObject({ project_id: project.id, status: candidate.status, assignee_type: candidate.assignee_type, assignee_id: candidate.assignee_id });
    await expect(page.getByText(candidate.title, { exact: true })).toBeVisible();
    await capture(page, info, "eight-squads-populated-wide");

    await manage.click();
    await manager.getByRole("button", { name: "Actions for Product delivery", exact: true }).click();
    await page.getByRole("menuitem", { name: "New issue with this squad", exact: true }).click();
    issueDialog = page.getByRole("dialog", { name: "New Issue", exact: true });
    await expect(issueDialog).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(1);
    await page.keyboard.press("Escape");

    await api.requestJSON(`/api/projects/${project.id}/execution-squads`, { method: "PUT", body: { squads: squads.map((squad) => ({ squad_id: squad.id })) } });
    await page.reload();
    await expect(summary).toContainText("20 ready");
    expect((await summary.boundingBox())!.height).toBeLessThan(120);
    await page.setViewportSize({ width: 390, height: 844 });
    await expectNoOverflow(page);
    await capture(page, info, "twenty-squads-populated-narrow");
    await manage.click();
    await expect(manager.getByRole("link", { name: squads[19]!.name, exact: true })).toBeAttached();
    await manager.getByRole("link", { name: squads[19]!.name, exact: true }).scrollIntoViewIfNeeded();
    await expectNoOverflow(page);
    await capture(page, info, "twenty-squads-manager-narrow");
    await page.keyboard.press("Escape");

    await api.requestJSON(`/api/projects/${project.id}/execution-squads`, { method: "PUT", body: { squads: [{ squad_id: first.id }] } });
    await page.route(`**/api/projects/${project.id}/resources`, (route) => route.fulfill({ status: 503, json: { error: "Availability check failed" } }));
    await page.reload();
    await expect(summary).toContainText("Could not check availability");
    await expect(summary).not.toContainText("1 ready");
    await capture(page, info, "one-squad-check-failed-narrow");
    await page.unroute(`**/api/projects/${project.id}/resources`);
    await manage.click();
    await manager.getByRole("button", { name: "Retry availability", exact: true }).click();
    await expect(manager.getByText("Ready for an issue", { exact: true })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(summary).toContainText("1 ready");

    await api.requestJSON(`/api/issues/${otherIssue.id}`, { method: "PUT", body: { project_id: null } });
    for (const mode of ["table", "gantt"]) {
      await page.evaluate(({ workspaceSlug, projectId, viewMode }) => {
        localStorage.setItem(`multica_issue_surface_views:${workspaceSlug}`, JSON.stringify({
          state: { surfaces: { [`project:${projectId}`]: { state: { viewMode }, updatedAt: new Date().toISOString() } } },
          version: 0,
        }));
      }, { workspaceSlug: slug, projectId: otherProject.id, viewMode: mode });
      await page.goto(`/${slug}/projects/${otherProject.id}`);
      await expect(page.getByRole("heading", { name: "Create your first issue", exact: true })).toBeVisible();
      await expect(page.locator("main").getByRole("button", { name: "New Issue", exact: true })).toHaveCount(1);
      await expectNoOverflow(page);
      await capture(page, info, `empty-${mode}-narrow`);
    }

    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: new URL(page.url()).origin }]);
    await page.goto(path);
    const localizedSummary = page.getByRole("region", { name: "执行AI小队", exact: true });
    await expect(localizedSummary).toBeVisible();
    await expectNoOverflow(page);
    await capture(page, info, "one-squad-populated-zh-narrow");
    expect(pageErrors).toEqual([]);
  } finally {
    // The runtime is a DB fixture only; no installed agent claims queued work.
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
