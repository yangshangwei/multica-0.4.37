import { randomUUID } from "node:crypto";
import { expect, test as base, type Page } from "@playwright/test";
import { p1Authenticate, p1Capture, p1DB, p1Failure, p1Member, p1NoOverflow, p1Overview, p1Project, p1Raw, p1Session, type P1Issue, type P1Project } from "./fixtures/project-p1";

type Session = Awaited<ReturnType<typeof p1Session>>;
const test = base.extend<{ p1: Session }>({
  p1: async ({ page }, use, info) => {
    const session = await p1Session(page);
    try { await use(session); }
    finally {
      try { if (info.status !== info.expectedStatus) await p1Failure(page, info); }
      finally { await session.api.deleteFeatureWorkspace(session.workspace.id); }
    }
  },
});

const timezoneLabel = "IANA timezone, for example Asia/Shanghai. Leave empty to use UTC.";
const composer = (page: Page) => page.locator('[aria-label="Write progress"]');
async function openProject(page: Page, session: Session, project: P1Project) {
  await page.goto(`/${session.workspace.slug}/projects/${project.id}?section=overview`);
  await page.getByRole("heading", { name: "Overview", exact: true }).waitFor();
}
const planningTimezone = (page: Page) => page.getByRole("combobox", { name: "Planning timezone", exact: true });
const planningTimezoneValue = (page: Page) => planningTimezone(page).locator('[data-slot="select-value"]');
async function openPlanningSettings(page: Page, session: Session) {
  await page.goto(`/${session.workspace.slug}/settings?tab=workspace`);
  await expect(planningTimezone(page)).toBeEnabled();
}
async function choosePlanningTimezone(page: Page, timezone: string) {
  await planningTimezone(page).click();
  await page.getByRole("option", { name: timezone, exact: true }).click();
  await expect(planningTimezoneValue(page)).toHaveText(timezone);
}
async function savePlanningTimezone(page: Page, session: Session, timezone: string | null) {
  const saved = page.waitForResponse((response) => response.request().method() === "PUT"
    && new URL(response.url()).pathname === `/api/workspaces/${session.workspace.id}/planning-timezone`);
  await page.getByRole("button", { name: timezone === null ? "Restore default" : "Save timezone", exact: true }).click();
  const response = await saved;
  expect(response.status()).toBe(200);
  expect(response.request().postDataJSON()).toEqual({ planning_timezone: timezone });
  expect(await response.json()).toMatchObject({ planning_timezone: timezone, effective_timezone: timezone ?? "Asia/Shanghai", configured: timezone !== null });
  await expect(planningTimezoneValue(page)).toHaveText(timezone ?? "Asia/Shanghai");
  await expect(page.getByText("Planning timezone saved", { exact: true })).toBeVisible();
}
async function openDelete(page: Page) {
  await page.locator("button").filter({ has: page.locator("svg.lucide-ellipsis") }).click();
  await page.getByRole("menuitem", { name: "Delete project", exact: true }).click();
}
async function runningWork(session: Session) {
  const project = await p1Project(session.api, { status: "in_progress" });
  const issue: P1Issue = await session.api.createIssue("Existing running work", { project_id: project.id, status: "todo" });
  const runtime = await session.api.seedProjectRuntime();
  const agent = await session.api.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: "Synthetic lifecycle fixture", runtime_id: runtime.id, permission_mode: "private" } });
  const execution = randomUUID();
  await p1DB("INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,dispatched_at,started_at) VALUES($1,$2,$3,$4,'running',now(),now())", [execution, agent.id, runtime.id, issue.id]);
  return { project, issue, execution };
}

test("P1-L01 cancelling the completion warning leaves project and work unchanged", async ({ page, p1 }) => {
  const project = await p1Project(p1.api, { status: "in_progress" });
  await p1.api.createIssue("Unfinished commitment", { project_id: project.id, status: "todo" });
  await openProject(page, p1, project);
  await page.getByRole("button", { name: "In Progress", exact: true }).click();
  await page.getByRole("menuitem", { name: "Completed", exact: true }).click();
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toContainText("1 open issues");
  await expect(dialog).toContainText("No acceptance recorded");
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  expect((await p1.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe("in_progress");
  expect(await p1DB("SELECT id FROM project_state_change WHERE project_id=$1", [project.id])).toHaveLength(0);
});

test("P1-L02 completion records the user's reason without completing issues or execution", async ({ page, p1 }, info) => {
  const { project, issue, execution } = await runningWork(p1);
  await openProject(page, p1, project);
  await page.getByRole("button", { name: "In Progress", exact: true }).click();
  await page.getByRole("menuitem", { name: "Completed", exact: true }).click();
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toContainText("1 open issues");
  await dialog.getByRole("textbox", { name: "Completion reason (optional)" }).fill("Customer accepted the project scope; follow-up work stays open.");
  await p1Capture(page, info, "completion-warning-with-reason");
  await dialog.getByRole("button", { name: "Complete project", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  expect((await p1.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe("completed");
  expect((await p1.api.requestJSON<P1Issue>(`/api/issues/${issue.id}`)).status).toBe("todo");
  expect(await p1DB("SELECT status,issue_id FROM agent_task_queue WHERE id=$1", [execution])).toEqual([{ status: "running", issue_id: issue.id }]);
  expect(await p1DB("SELECT reason FROM project_state_change WHERE project_id=$1 AND to_status='completed'", [project.id])).toEqual([{ reason: "Customer accepted the project scope; follow-up work stays open." }]);
});

test("P1-L03 status controls pause cancel and reopen only the project", async ({ page, p1 }) => {
  const { project, issue, execution } = await runningWork(p1);
  await openProject(page, p1, project);
  let current = "In Progress";
  for (const [status, label] of [["paused", "Paused"], ["cancelled", "Cancelled"], ["in_progress", "In Progress"], ["completed", "Completed"]]) {
    await page.getByRole("button", { name: current, exact: true }).click();
    await page.getByRole("menuitem", { name: label, exact: true }).click();
    if (status === "completed") await page.getByRole("alertdialog").getByRole("button", { name: "Complete project", exact: true }).click();
    await expect(page.getByRole("button", { name: label, exact: true })).toBeVisible();
    await expect.poll(async () => (await p1.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe(status);
    expect(await p1DB("SELECT status,issue_id FROM agent_task_queue WHERE id=$1", [execution])).toEqual([{ status: "running", issue_id: issue.id }]);
    expect((await p1.api.requestJSON<P1Issue>(`/api/issues/${issue.id}`)).status).toBe("todo");
    current = label!;
  }
  expect(await p1.api.countIssueDispatches(issue.id)).toBe(1);
});

test("P1-L04 a member cannot delete a project but retains ordinary read access", async ({ browser, p1 }, info) => {
  const project = await p1Project(p1.api);
  const member = await p1Member(p1.workspace);
  const context = await browser.newContext();
  const page = await context.newPage();
  try {
  await p1Authenticate(page, member.api);
  await openProject(page, p1, project);
  await page.locator("button").filter({ has: page.locator("svg.lucide-ellipsis") }).click();
  await expect(page.getByRole("menuitem", { name: "Delete project", exact: true })).toHaveCount(0);
  await page.keyboard.press("Escape");
  const response = await p1Raw(member.api, p1.workspace.id, `/api/projects/${project.id}`, "DELETE");
  expect(response.status).toBe(403);
  expect(await response.json()).toMatchObject({ code: "project_permission_denied" });
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
  await expect(page.getByLabel(timezoneLabel)).toHaveCount(0);
  expect((await member.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).id).toBe(project.id);
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await context.close(); }
});

test("P1-L05 cancelling project deletion sends no delete request", async ({ page, p1 }) => {
  const project = await p1Project(p1.api);
  await p1.api.createIssue("Preserved on cancellation", { project_id: project.id });
  await openProject(page, p1, project);
  let deletes = 0;
  page.on("request", (request) => { if (request.method() === "DELETE" && request.url().endsWith(`/api/projects/${project.id}`)) deletes++; });
  await openDelete(page);
  await expect(page.getByRole("alertdialog")).toContainText("1 linked issues are preserved");
  await page.getByRole("alertdialog").getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByRole("alertdialog")).toHaveCount(0);
  expect(deletes).toBe(0);
  expect((await p1.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).id).toBe(project.id);
});

test("P1-L06 owner deletion through the dialog preserves issues and running execution", async ({ page, p1 }) => {
  const { project, issue, execution } = await runningWork(p1);
  await openProject(page, p1, project);
  const impact = await p1.api.requestJSON(`/api/projects/${project.id}/delete-impact`);
  expect(impact).toMatchObject({ preserves_issues: true, preserves_executions: true, formal_issue_count: 1 });
  await openDelete(page);
  await expect(page.getByRole("alertdialog")).toContainText("1 linked issues are preserved");
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/${p1.workspace.slug}/projects$`));
  expect(await p1.api.requestJSON<P1Issue>(`/api/issues/${issue.id}`)).toMatchObject({ project_id: null, status: "todo" });
  expect(await p1DB("SELECT status,issue_id FROM agent_task_queue WHERE id=$1", [execution])).toEqual([{ status: "running", issue_id: issue.id }]);
  expect(await p1.api.countIssueDispatches(issue.id)).toBe(1);
});

test("P1-L07 a failed deletion keeps the project readable and can be retried", async ({ page, p1 }) => {
  const project = await p1Project(p1.api);
  await openProject(page, p1, project);
  await page.route(`**/api/projects/${project.id}`, (route) => route.request().method() === "DELETE" ? route.fulfill({ status: 503, json: { error: "Fixture deletion unavailable" } }) : route.continue());
  await openDelete(page);
  const rejected = page.waitForResponse((r) => r.request().method() === "DELETE" && r.url().endsWith(`/api/projects/${project.id}`));
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete", exact: true }).click();
  expect((await rejected).status()).toBe(503);
  await expect(page.getByRole("alertdialog")).toBeVisible();
  expect((await p1.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).id).toBe(project.id);
  await page.unroute(`**/api/projects/${project.id}`);
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/${p1.workspace.slug}/projects$`));
});

test("P1-L08 an initially unavailable overview recovers through Retry", async ({ page, p1 }) => {
  const project = await p1Project(p1.api);
  await page.route(`**/api/projects/${project.id}/overview`, (route) => route.fulfill({ status: 503, json: { error: "Fixture overview unavailable", code: "project_health_unavailable" } }));
  await page.goto(`/${p1.workspace.slug}/projects/${project.id}?section=overview`);
  await expect(page.getByRole("alert").filter({ hasText: "Could not load project management" })).toBeVisible();
  await page.unroute(`**/api/projects/${project.id}/overview`);
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByText("No formal issues yet", { exact: true })).toBeVisible();
});

test("P1-L09 refresh failure marks old statistics stale and disables risk navigation", async ({ page, p1 }) => {
  const project = await p1Project(p1.api, { lead_type: "member", lead_id: p1.owner.id });
  await p1.api.createIssue("Assigned healthy task", { project_id: project.id, assignee_type: "member", assignee_id: p1.owner.id });
  await openProject(page, p1, project);
  await page.route(`**/api/projects/${project.id}/overview`, (route) => route.fulfill({ status: 503, json: { error: "Fixture refresh unavailable" } }));
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(page.getByText("Refresh failed. These are the last available statistics.", { exact: true })).toBeVisible();
  await expect(page.getByText("No current risk signals", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /^Overdue / })).toBeDisabled();
  await page.unroute(`**/api/projects/${project.id}/overview`);
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByRole("button", { name: /^Overdue / })).toBeEnabled();
});

test("P1-L10 unsupported capabilities retain the description and hide unsafe editing", async ({ page, p1 }) => {
  const project = await p1Project(p1.api);
  await page.route(`**/api/workspaces/${p1.workspace.id}/project-capabilities`, async (route) => {
    const response = await route.fetch();
    await route.fulfill({ response, json: { ...await response.json(), overview: false, updates: false, description_cas: false, planning_timezone: false } });
  });
  await page.goto(`/${p1.workspace.slug}/projects/${project.id}?section=overview`);
  await expect(page.getByText("This server does not support safe description editing. Update the server to edit; your text is preserved.", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Goal template", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Write progress", exact: true })).toHaveCount(0);
  await expect(page.getByText(project.description!, { exact: true })).toBeVisible();
  expect((await p1.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toBe(project.description);
});

test("P1-L11 an owner selects and changes the shared planning timezone and restores the Shanghai default", async ({ page, p1 }) => {
  const project = await p1Project(p1.api);
  await openProject(page, p1, project);
  await openPlanningSettings(page, p1);
  await expect(planningTimezoneValue(page)).toHaveText("Asia/Shanghai");
  expect(await p1.api.requestJSON(`/api/workspaces/${p1.workspace.id}/planning-timezone`)).toMatchObject({ planning_timezone: null, effective_timezone: "Asia/Shanghai", configured: false });
  for (const timezone of ["UTC", "Europe/London"]) {
    await choosePlanningTimezone(page, timezone);
    await savePlanningTimezone(page, p1, timezone);
    await expect.poll(async () => (await p1Overview(p1.api, project.id)).statistics.timezone).toBe(timezone);
  }
  await page.reload();
  await expect(planningTimezoneValue(page)).toHaveText("Europe/London");
  await savePlanningTimezone(page, p1, null);
  await expect(page.getByRole("button", { name: "Restore default", exact: true })).toHaveCount(0);
  expect((await p1Overview(p1.api, project.id)).statistics.timezone).toBe("Asia/Shanghai");
});

test("P1-L12 the picker excludes invalid timezones and rejected writes leave planning unchanged", async ({ page, p1 }) => {
  const project = await p1Project(p1.api);
  const path = `/api/workspaces/${p1.workspace.id}/planning-timezone`;
  let writes = 0;
  page.on("request", (request) => { if (request.method() === "PUT" && new URL(request.url()).pathname === path) writes++; });
  await openPlanningSettings(page, p1);
  const original = await p1.api.requestJSON(path);
  await planningTimezone(page).click();
  await expect(page.getByRole("option", { name: "Not/A_Real_Zone", exact: true })).toHaveCount(0);
  await page.keyboard.press("Escape");
  await expect(planningTimezoneValue(page)).toHaveText("Asia/Shanghai");
  await expect(page.getByRole("button", { name: "Save timezone", exact: true })).toHaveCount(0);
  expect(writes).toBe(0);
  // The selector cannot create arbitrary values; the API still rejects an invalid caller value.
  const rejected = await p1Raw(p1.api, p1.workspace.id, path, "PUT", { planning_timezone: "Not/A_Real_Zone" });
  expect(rejected.status).toBe(422);
  expect(await rejected.json()).toMatchObject({ code: "validation_failed", field_errors: [{ field: "planning_timezone" }] });
  expect(await p1.api.requestJSON(path)).toEqual(original);
  expect((await p1Overview(p1.api, project.id)).statistics.timezone).toBe("Asia/Shanghai");
  await expect(planningTimezone(page)).toBeEnabled();
  await choosePlanningTimezone(page, "Europe/London");
  await savePlanningTimezone(page, p1, "Europe/London");
  expect(writes).toBe(1);
  await expect.poll(async () => (await p1Overview(p1.api, project.id)).statistics.timezone).toBe("Europe/London");
});

test("P1-L13 loss of timezone write permission preserves readable project and local progress", async ({ page, p1 }) => {
  const project = await p1Project(p1.api);
  await openProject(page, p1, project);
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  await composer(page).locator('[contenteditable="true"]').fill("My draft must survive an operation denial");
  const settingsPage = await page.context().newPage();
  let demoted = false;
  try {
    await p1Authenticate(settingsPage, p1.api);
    await openPlanningSettings(settingsPage, p1);
    await choosePlanningTimezone(settingsPage, "UTC");
    await p1DB("UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2", [p1.workspace.id, p1.owner.id]);
    demoted = true;
    const failure = settingsPage.waitForResponse((r) => r.request().method() === "PUT" && r.url().endsWith("/planning-timezone"));
    await settingsPage.getByRole("button", { name: "Save timezone", exact: true }).click();
    const response = await failure;
    expect(response.status()).toBe(403);
    expect(await response.json()).toMatchObject({ code: "project_permission_denied" });
    await expect(planningTimezoneValue(settingsPage)).toHaveText("UTC");
    expect(await p1.api.requestJSON(`/api/workspaces/${p1.workspace.id}/planning-timezone`)).toMatchObject({ planning_timezone: null, effective_timezone: "Asia/Shanghai" });
    await expect(composer(page).locator('[contenteditable="true"]')).toContainText("My draft must survive an operation denial");
    await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
    expect((await p1.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).id).toBe(project.id);
  } finally {
    if (demoted) await p1DB("UPDATE member SET role='owner' WHERE workspace_id=$1 AND user_id=$2", [p1.workspace.id, p1.owner.id]);
    await settingsPage.close();
  }
});

test("P1-L14 real workspace revocation clears visible drafts even without WebSocket delivery", async ({ browser, p1 }, info) => {
  const member = await p1Member(p1.workspace);
  const project = await p1Project(p1.api, { description: "Protected project goal" });
  const context = await browser.newContext();
  const page = await context.newPage();
  try {
  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => socket.close());
  await p1Authenticate(page, member.api);
  await openProject(page, p1, project);
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  await composer(page).locator('[contenteditable="true"]').fill("Protected unsent progress");
  await expect.poll(() => page.evaluate(() => Object.values(localStorage).some((value) => value.includes("Protected unsent progress")))).toBe(true);
  expect((await p1Raw(member.api, p1.workspace.id, `/api/workspaces/${p1.workspace.id}/leave`, "POST")).status).toBe(204);
  const denial = page.waitForResponse((r) => r.url().endsWith(`/api/projects/${project.id}/overview`) && r.status() === 404);
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  expect(await (await denial).json()).toMatchObject({ code: "workspace_access_denied" });
  await expect(composer(page)).toHaveCount(0);
  await expect(page.getByText("Protected project goal", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Protected unsent progress", { exact: true })).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => Object.values(localStorage).some((value) => value.includes("Protected unsent progress")))).toBe(false);
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await context.close(); }
});

// P1-L14 owns what the denial clears; the cleanup's idempotence is unit-tested in
// packages/core/projects/access-lifecycle.test.ts and use-project-access-guard.test.tsx.
// This test owns the browser traffic that follows: a revocation the page learns
// about only from HTTP once refetched members, agents, squads and pins at ~1,250
// requests/s until the tab stalled, while P1-L14 could still pass with tracing off.
const revocations = {
  "the member leaves": async (session: Session, member: Awaited<ReturnType<typeof p1Member>>) =>
    p1Raw(member.api, session.workspace.id, `/api/workspaces/${session.workspace.id}/leave`, "POST"),
  "an owner removes the member": async (session: Session, member: Awaited<ReturnType<typeof p1Member>>) => {
    const [row] = await p1DB<{ id: string }>("SELECT id FROM member WHERE workspace_id=$1 AND user_id=$2", [session.workspace.id, member.user.id]);
    return p1Raw(session.api, session.workspace.id, `/api/workspaces/${session.workspace.id}/members/${row!.id}`, "DELETE");
  },
};
for (const [revocation, revoke] of Object.entries(revocations)) {
  test(`P1-L16 when ${revocation} without WebSocket delivery, requests stay bounded for 20 seconds`, async ({ browser, p1 }, info) => {
    const member = await p1Member(p1.workspace);
    const project = await p1Project(p1.api, { description: "Protected project goal" });
    const context = await browser.newContext();
    const page = await context.newPage();
    const requests: { at: number; path: string }[] = [];
    try {
      await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => socket.close());
      await p1Authenticate(page, member.api);
      await openProject(page, p1, project);
      context.on("request", (request) => {
        const { pathname } = new URL(request.url());
        if (pathname.startsWith("/api/")) requests.push({ at: Date.now(), path: pathname.replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, ":id") });
      });
      expect((await revoke(p1, member)).status).toBe(204);
      const revokedAt = Date.now();
      const denial = page.waitForResponse((r) => r.url().endsWith(`/api/projects/${project.id}/overview`) && r.status() === 404);
      await page.getByRole("button", { name: "Refresh", exact: true }).click();
      expect(await (await denial).json()).toMatchObject({ code: "workspace_access_denied" });
      await expect(page.getByText("Protected project goal", { exact: true })).toHaveCount(0);
      await page.waitForTimeout(20_000 - (Date.now() - revokedAt));

      // Assert the counts before touching the page again: a looping renderer
      // answers evaluate only after ~20s, which would eat the test budget.
      const observed = requests.filter((entry) => entry.at >= revokedAt && entry.at < revokedAt + 20_000);
      const perFiveSeconds = [0, 1, 2, 3].map((slot) => observed.filter((entry) => Math.floor((entry.at - revokedAt) / 5_000) === slot).length);
      const byPath: Record<string, number> = {};
      for (const { path } of observed) byPath[path] = (byPath[path] ?? 0) + 1;
      await info.attach("revocation-requests", { body: JSON.stringify({ revocation, total: observed.length, perFiveSeconds, byPath }, null, 2), contentType: "application/json" });
      expect(Object.entries(byPath).filter(([, count]) => count > 5), "endpoints refetched in a loop").toEqual([]);
      expect(observed.length, `requests in the 20s after revocation: ${JSON.stringify(perFiveSeconds)}`).toBeLessThanOrEqual(40);
      expect(perFiveSeconds.slice(1).reduce((sum, count) => sum + count, 0), "traffic stops once the denial is handled").toBeLessThanOrEqual(5);

      const started = Date.now();
      await page.evaluate(() => document.title);
      expect(Date.now() - started, "the renderer stays responsive").toBeLessThan(1_000);
    } catch (error) {
      // A looping page cannot be captured in time; bound the capture so the
      // count failure, not a test timeout, stays the reported cause.
      await Promise.race([p1Failure(page, info).catch(() => undefined), new Promise((resolve) => setTimeout(resolve, 5_000))]);
      throw error;
    } finally { await context.close(); }
  });
}

test("P1-L15 Chinese compact editing supports keyboard preview and an unobstructed publish button", async ({ page, p1 }, info) => {
  const project = await p1Project(p1.api);
  await p1.api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
  await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: process.env.PLAYWRIGHT_BASE_URL! }]);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/${p1.workspace.slug}/projects/${project.id}?section=overview`);
  await page.getByRole("button", { name: "记录进展", exact: true }).focus();
  await page.keyboard.press("Enter");
  const editor = page.locator('[aria-label="记录进展"]');
  await editor.locator('[contenteditable="true"]').fill("客户验收记录已复核，下一步完成交付。");
  await editor.getByRole("button", { name: "预览发布", exact: true }).focus();
  await page.keyboard.press("Enter");
  const publish = editor.getByRole("button", { name: "发布", exact: true });
  await expect(publish).toBeEnabled();
  await publish.scrollIntoViewIfNeeded();
  await p1NoOverflow(page);
  await publish.evaluate((element) => element.scrollIntoView({ block: "center" }));
  expect(await publish.evaluate((element) => {
    const box = element.getBoundingClientRect();
    return [0.25, 0.5, 0.75].every((x) => [0.25, 0.5, 0.75].every((y) => {
      const target = document.elementFromPoint(box.x + box.width * x, box.y + box.height * y);
      return target === element || element.contains(target);
    }));
  })).toBe(true);
  await p1Capture(page, info, "chinese-compact-keyboard-and-publish");
});
