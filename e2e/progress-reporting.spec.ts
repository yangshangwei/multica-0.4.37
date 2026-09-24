import { randomUUID } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import { test as base, expect, type Locator, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

interface RoleTemplate {
  key: string;
  version: number;
  name: string;
  title: string;
  instructions: string;
  autonomy_level: string;
  max_concurrent_tasks: number;
  skill_names: string[];
}

interface ReportTemplate {
  key: string;
  version: number;
  title: string;
  prompt: string;
  cron_expression: string;
  execution_mode: string;
}

interface Reporter {
  id: string;
  name: string;
  runtime_id: string;
  template_key: string;
  template_version: number;
  instructions: string;
  autonomy_level: string;
  max_concurrent_tasks: number;
  skills: { id: string; name: string; enabled: boolean }[];
}

interface AutopilotDetail {
  autopilot: {
    id: string;
    title: string;
    description: string;
    assignee_id: string;
    assignee_type: string;
    template_key: string;
    template_version: number;
    execution_mode: string;
    issue_title_template: string;
  };
  triggers: {
    id: string;
    kind: string;
    enabled: boolean;
    cron_expression: string;
    timezone: string;
    next_run_at: string;
  }[];
}

interface ReportRun {
  id: string;
  autopilot_id: string;
  source: string;
  status: string;
  issue_id: string | null;
  trigger_id: string | null;
  triggered_at: string;
  completed_at: string | null;
}

interface ReportIssue {
  id: string;
  title: string;
  description: string;
  status: string;
  assignee_id: string;
  assignee_type: string;
}

interface ReportingFixture {
  api: TestApiClient;
  slug: string;
  runtime: { id: string; daemon_id: string };
}

const ROLE_KEY = "progress-reporter";
const ROLE_NAME = "进展报告员";
const DAILY_KEY = "daily-progress-report";
const WEEKLY_KEY = "weekly-progress-report";

function requiredTemplate<T extends { key: string }>(templates: T[], key: string): T {
  const template = templates.find((item) => item.key === key);
  if (!template) throw new Error(`The real API is missing built-in template ${key}`);
  return template;
}

async function attachJSON(testInfo: TestInfo, name: string, value: unknown) {
  const path = testInfo.outputPath(`${name}.json`);
  await writeFile(path, JSON.stringify(value, null, 2));
  await testInfo.attach(name, { path, contentType: "application/json" });
}

async function capture(page: Page, testInfo: TestInfo, name: string) {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: true, animations: "disabled" });
  await testInfo.attach(name, { path, contentType: "image/png" });
}

const test = base.extend<{ reporting: ReportingFixture }>({
  reporting: async ({ page, baseURL }, use, testInfo) => {
    const slug = `e2e-reports-${randomUUID().slice(0, 12)}`;
    const api = new TestApiClient();
    await api.login(`${slug}@multica.ai`, "进展报告验收用户");
    const workspace = await api.ensureWorkspace("进展报告验收工作区", slug);
    // ensureWorkspace may fall back to an existing workspace; require this
    // test's unique slug before writing or deleting any fixture data.
    expect(workspace.slug).toBe(slug);
    try {
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
      const token = api.getToken();
      if (!token || !baseURL) throw new Error("Authenticated reporting fixture is incomplete");
      await page.context().addCookies([
        { name: "multica-locale", value: "zh-Hans", url: baseURL },
        { name: "multica_logged_in", value: "1", url: baseURL },
      ]);
      await page.addInitScript((value) => {
        localStorage.setItem("multica_token", value);
        localStorage.setItem("multica:chat:isOpen", "false");
        localStorage.setItem("theme", "dark");
      }, token);
      const runtime = await api.seedProjectRuntime();
      await use({ api, slug, runtime });
      if (testInfo.status !== testInfo.expectedStatus) {
        await capture(page, testInfo, "failure-before-workspace-cleanup");
        await attachJSON(testInfo, "failure-page-before-cleanup", {
          url: page.url(), text: await page.locator("body").innerText(),
        });
      }
    } finally {
      await api.deleteFeatureWorkspace(workspace.id);
      const removed = !(await api.getWorkspaces()).some((item) => item.id === workspace.id);
      await attachJSON(testInfo, "isolated-workspace-cleanup", { workspace_id: workspace.id, slug, removed });
      expect(removed).toBe(true);
    }
  },
});

test.use({
  locale: "zh-CN",
  timezoneId: "Asia/Shanghai",
  viewport: { width: 1440, height: 1000 },
  trace: "retain-on-failure",
  screenshot: "only-on-failure",
});

async function expectReadablePreview(preview: Locator) {
  await expect(preview).toBeVisible();
  const size = await preview.evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    return {
      left: bounds.left,
      right: bounds.right,
      width: element.clientWidth,
      contentWidth: element.scrollWidth,
      editable: element instanceof HTMLElement && element.isContentEditable,
      whiteSpace: getComputedStyle(element).whiteSpace,
      viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
    };
  });
  expect(size.editable).toBe(false);
  expect(size.whiteSpace).toBe("pre-wrap");
  expect(size.width).toBeGreaterThan(200);
  expect(size.left).toBeGreaterThanOrEqual(0);
  expect(size.right).toBeLessThanOrEqual(size.viewportWidth);
  expect(size.contentWidth).toBeLessThanOrEqual(size.width + 1);
  expect(size.documentWidth).toBeLessThanOrEqual(size.viewportWidth + 1);
  // Both reporting briefs are longer than the fixed preview. Their last
  // instructions must remain reachable through the preview's own scroller.
  await preview.evaluate((element) => { element.scrollTop = element.scrollHeight; });
  expect(await preview.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  await preview.evaluate((element) => { element.scrollTop = 0; });
}

async function createReportAutomation(page: Page, api: TestApiClient, template: ReportTemplate, reporter: Reporter) {
  await expect(page.locator("pre")).toHaveText(template.prompt);
  await expect(page.getByRole("button", { name: /^时区: Shanghai GMT\+8$/ })).toBeVisible();
  const create = page.getByRole("button", { name: "启用自动化", exact: true });
  await expect(create).toBeDisabled();
  await page.getByRole("button", { name: "选择智能体或AI小队", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: new RegExp(reporter.name) }).click();
  await expect(create).toBeEnabled();
  const pending = page.waitForResponse((response) =>
    new URL(response.url()).pathname === "/api/autopilots/from-template" && response.request().method() === "POST",
  );
  await create.click();
  const response = await pending;
  expect(response.status()).toBe(201);
  // Existing autopilot-template.spec.ts owns the complete request-field matrix.
  expect(response.request().postDataJSON()).toMatchObject({
    template_key: template.key, assignee_id: reporter.id, timezone: "Asia/Shanghai", language: "zh",
  });
  await page.waitForURL(/\/autopilots\/[0-9a-f-]+$/);
  const id = new URL(page.url()).pathname.split("/").at(-1);
  const detail = await api.requestJSON<AutopilotDetail>(`/api/autopilots/${id}`);
  expect(detail.autopilot).toMatchObject({
    title: template.title,
    description: template.prompt,
    assignee_id: reporter.id,
    assignee_type: "agent",
    template_key: template.key,
    template_version: template.version,
    execution_mode: "create_issue",
    issue_title_template: `${template.title} — {{date}}`,
  });
  expect(detail.triggers).toHaveLength(1);
  expect(detail.triggers[0]).toMatchObject({
    kind: "schedule", enabled: true, cron_expression: template.cron_expression, timezone: "Asia/Shanghai",
  });
  expect(Date.parse(detail.triggers[0].next_run_at)).not.toBeNaN();
  // Prevent a clock-boundary schedule from adding a second report while this
  // deterministic fixture exercises the real manual admission path.
  await api.requestJSON(`/api/autopilots/${id}/triggers/${detail.triggers[0].id}`, {
    method: "PATCH", body: { enabled: false },
  });
  return detail;
}

test("publishes one built-in reporter and adjacent daily and weekly templates", async ({ reporting }, testInfo) => {
  const { api } = reporting;
  const { templates: roles } = await api.requestJSON<{ templates: RoleTemplate[] }>("/api/agents/templates?language=zh");
  const { templates } = await api.requestJSON<{ templates: ReportTemplate[] }>("/api/autopilots/templates?language=zh");
  await attachJSON(testInfo, "real-reporting-catalogs", { roles, templates });

  expect.soft(roles).toHaveLength(14);
  expect.soft(roles.filter((role) => role.key === ROLE_KEY)).toHaveLength(1);
  expect.soft(templates).toHaveLength(10);
  const dailyIndex = templates.findIndex((template) => template.key === DAILY_KEY);
  const weeklyIndex = templates.findIndex((template) => template.key === WEEKLY_KEY);
  expect.soft(dailyIndex).toBeGreaterThanOrEqual(0);
  expect.soft(weeklyIndex).toBe(dailyIndex + 1);

  const role = requiredTemplate(roles, ROLE_KEY);
  expect(role).toMatchObject({ title: ROLE_NAME, autonomy_level: "contributor", max_concurrent_tasks: 1, skill_names: ["multica-progress-report"] });
  expect(role.instructions).toBe((await readFile("server/internal/service/builtin_agent_templates/progress-reporter/INSTRUCTIONS.md", "utf8")).replace(/\n+$/, ""));
  const daily = requiredTemplate(templates, DAILY_KEY);
  const weekly = requiredTemplate(templates, WEEKLY_KEY);
  expect(daily).toMatchObject({ title: "每日进展报告", version: 1, cron_expression: "0 18 * * *", execution_mode: "create_issue" });
  expect(weekly).toMatchObject({ title: "每周进展报告", version: 2, cron_expression: "0 17 * * 1", execution_mode: "create_issue" });
  expect(daily.prompt).toMatch(/24\s*小时/);
  expect(weekly.prompt).toMatch(/7\s*天/);
  for (const template of [daily, weekly]) {
    const source = await readFile(`server/internal/service/builtin_autopilot_templates/${template.key}/PROMPT.md`, "utf8");
    expect(template.prompt).toBe(source.replace(/\n+$/, ""));
  }
  const english = await api.requestJSON<{ templates: RoleTemplate[] }>("/api/agents/templates?language=en");
  expect(requiredTemplate(english.templates, ROLE_KEY)).toMatchObject({ title: "Progress Reporter", instructions: role.instructions });
});

test("creates the reporter from the Chinese catalog and reopens its ordinary agent", async ({ page, reporting }, testInfo) => {
  test.setTimeout(120_000);
  const { api, slug, runtime } = reporting;
  const { templates } = await api.requestJSON<{ templates: RoleTemplate[] }>("/api/agents/templates?language=zh");
  const role = requiredTemplate(templates, ROLE_KEY);

  await page.goto(`/${slug}/agents`);
  await page.getByRole("button", { name: /^内置智能体\s+\d+$/ }).click();
  const row = page.getByRole("listitem", { name: ROLE_NAME, exact: true });
  await expect(row).toBeVisible();
  await expect(row.getByRole("button", { name: "打开智能体", exact: true })).toHaveCount(0);
  await row.scrollIntoViewIfNeeded();
  await capture(page, testInfo, "builtin-reporter-catalog-1440");
  await row.getByRole("button", { name: "查看模板", exact: true }).click();
  await page.waitForURL((url) => url.searchParams.get("template") === ROLE_KEY);
  await expect(page.getByText("角色指令", { exact: true })).toBeVisible();
  await expect(page.locator("pre")).toHaveText(role.instructions);

  // The other entry point is the same role picker pictured in the request.
  await page.goto(`/${slug}/agents/new/template`);
  await expect(page.getByRole("heading", { name: "从角色模板开始", exact: true })).toBeVisible();
  const card = page.getByRole("button", { name: new RegExp(ROLE_NAME) });
  await expect(card).toHaveCount(1);
  await expect(card).toContainText("参与实现");
  await card.scrollIntoViewIfNeeded();
  await capture(page, testInfo, "reporter-role-picker-1440");
  await page.setViewportSize({ width: 390, height: 844 });
  await card.scrollIntoViewIfNeeded();
  const bounds = await card.boundingBox();
  expect(bounds).not.toBeNull();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);
  await capture(page, testInfo, "reporter-role-picker-390");
  await card.click();
  const preview = page.locator("pre");
  await expect(preview).toHaveText(role.instructions);
  await expectReadablePreview(preview);
  await expect(page.locator("#agent-create-instructions")).toHaveCount(0);
  await expect(page.getByText("该角色附带的 Skills", { exact: true })).toBeVisible();
  await expect(page.locator("span.rounded-full.font-mono")).toHaveText("进展报告");
  await preview.scrollIntoViewIfNeeded();
  await capture(page, testInfo, "reporter-instructions-390");
  await page.setViewportSize({ width: 1440, height: 1000 });
  await preview.scrollIntoViewIfNeeded();
  await capture(page, testInfo, "reporter-instructions-1440");
  await page.locator("#agent-create-name").fill(ROLE_NAME);
  const create = page.getByRole("button", { name: "创建并打开", exact: true });
  await expect(create).toBeEnabled();
  const pending = page.waitForResponse((response) =>
    new URL(response.url()).pathname === "/api/agents/from-template" && response.request().method() === "POST",
  );
  await create.click();
  const response = await pending;
  expect(response.status()).toBe(201);
  expect(response.request().postDataJSON()).toMatchObject({ template_key: ROLE_KEY, runtime_id: runtime.id, name: ROLE_NAME });
  const created = await response.json() as Reporter;
  await page.waitForURL((url) => url.pathname === `/${slug}/agents/${created.id}`);
  const reporter = await api.requestJSON<Reporter>(`/api/agents/${created.id}`);
  expect(reporter).toMatchObject({
    name: ROLE_NAME, runtime_id: runtime.id, template_key: ROLE_KEY, template_version: role.version,
    instructions: role.instructions, autonomy_level: "contributor", max_concurrent_tasks: 1,
  });
  expect(reporter.skills).toHaveLength(1);
  expect(reporter.skills[0]).toMatchObject({ name: "multica-progress-report", enabled: true });
  const skill = await api.requestJSON<{ content: string; name: string }>(`/api/skills/${reporter.skills[0].id}`);
  expect(skill.content).toContain("in_review");
  expect(skill.content).toMatch(/日报|周报/);
  await attachJSON(testInfo, "real-created-reporter", { reporter, skill });

  await page.goto(`/${slug}/agents`);
  await page.getByRole("button", { name: /^内置智能体\s+\d+$/ }).click();
  await expect(row.getByRole("button", { name: "查看模板", exact: true })).toBeVisible();
  await row.scrollIntoViewIfNeeded();
  await capture(page, testInfo, "builtin-reporter-catalog-created-1440");
  await row.getByRole("button", { name: "查看模板", exact: true }).click();
  await expect(page.locator("pre")).toHaveText(role.instructions);
  await page.goto(`/${slug}/agents`);
  await page.getByRole("button", { name: /^内置智能体\s+\d+$/ }).click();
  await row.getByRole("button", { name: "打开智能体", exact: true }).click();
  await page.waitForURL((url) => url.pathname === `/${slug}/agents/${reporter.id}`);
  expect((await api.requestJSON<Reporter[]>("/api/agents")).filter((agent) => agent.template_key === ROLE_KEY)).toHaveLength(1);
});

test("adopts daily and weekly reports and completes fixture delivery on only their report issues", async ({ page, reporting }, testInfo) => {
  test.setTimeout(150_000);
  const { api, slug, runtime } = reporting;
  const reporter = await api.requestJSON<Reporter>("/api/agents/from-template", {
    method: "POST", body: { template_key: ROLE_KEY, name: ROLE_NAME, runtime_id: runtime.id, permission_mode: "private", language: "zh" },
  });
  const { templates } = await api.requestJSON<{ templates: ReportTemplate[] }>("/api/autopilots/templates?language=zh");
  const daily = requiredTemplate(templates, DAILY_KEY);
  const weekly = requiredTemplate(templates, WEEKLY_KEY);

  await page.goto(`/${slug}/autopilots`);
  await page.getByRole("button", { name: /^内置模板\s+10$/ }).click();
  const catalog = page.getByRole("region", { name: "内置模板", exact: true });
  const catalogTitles = await catalog.getByRole("listitem").allTextContents();
  const dailyIndex = catalogTitles.findIndex((title) => title.includes(daily.title));
  expect(dailyIndex).toBeGreaterThanOrEqual(0);
  expect(catalogTitles[dailyIndex + 1]).toContain(weekly.title);
  await catalog.getByRole("button", { name: new RegExp(daily.title) }).scrollIntoViewIfNeeded();
  await capture(page, testInfo, "daily-weekly-catalog-1440");
  await catalog.getByRole("button", { name: new RegExp(daily.title) }).click();
  await page.waitForURL((url) => url.searchParams.get("template") === DAILY_KEY);
  await expect(page.locator("pre")).toHaveText(daily.prompt);
  await expectReadablePreview(page.locator("pre"));
  await capture(page, testInfo, "daily-report-preview-1440");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator("pre").scrollIntoViewIfNeeded();
  await expectReadablePreview(page.locator("pre"));
  await capture(page, testInfo, "daily-report-preview-390");
  await page.setViewportSize({ width: 1440, height: 1000 });
  const dailyAutomation = await createReportAutomation(page, api, daily, reporter);

  await page.goto(`/${slug}/autopilots/new/template`);
  const pickerCards = page.locator("main button.group");
  await expect(pickerCards).toHaveCount(10);
  const pickerTitles = await pickerCards.allTextContents();
  expect(pickerTitles[templates.findIndex((template) => template.key === DAILY_KEY)]).toContain(daily.title);
  expect(pickerTitles[templates.findIndex((template) => template.key === WEEKLY_KEY)]).toContain(weekly.title);
  await page.getByRole("button", { name: new RegExp(weekly.title) }).click();
  await expect(page.locator("pre")).toHaveText(weekly.prompt);
  await capture(page, testInfo, "weekly-report-preview-1440");
  const weeklyAutomation = await createReportAutomation(page, api, weekly, reporter);
  expect(weeklyAutomation.autopilot.id).not.toBe(dailyAutomation.autopilot.id);
  expect(weeklyAutomation.autopilot.description).not.toBe(dailyAutomation.autopilot.description);

  // Seed source tasks and an actual completion transition through public APIs.
  // The runtime fixture has no daemon or model account. The report text below
  // is explicitly test-authored delivery, not an LLM generation-quality test.
  const done = await api.createIssue("报告来源：本期完成的业务任务", { status: "todo" });
  await api.updateIssue(done.id, { status: "done" });
  const ongoing = await api.createIssue("报告来源：进行中的业务任务", { status: "in_progress" });
  const blocked = await api.createIssue("报告来源：阻塞的业务任务", { status: "blocked" });
  const sourceIds = [done.id, ongoing.id, blocked.id];
  const sourceBefore = await Promise.all(sourceIds.map((id) => api.requestJSON<ReportIssue>(`/api/issues/${id}`)));
  const reports: unknown[] = [];

  for (const [template, detail] of [[daily, dailyAutomation], [weekly, weeklyAutomation]] as const) {
    await page.goto(`/${slug}/autopilots/${detail.autopilot.id}`);
    const before = await api.requestJSON<{ issues: ReportIssue[] }>("/api/issues");
    const pending = page.waitForResponse((response) =>
      new URL(response.url()).pathname === `/api/autopilots/${detail.autopilot.id}/trigger` && response.request().method() === "POST",
    );
    await page.getByRole("button", { name: "立即运行", exact: true }).click();
    expect((await pending).ok()).toBe(true);
    const { runs } = await api.requestJSON<{ runs: ReportRun[] }>(`/api/autopilots/${detail.autopilot.id}/runs`);
    expect(runs).toHaveLength(1);
    const run = runs[0];
    expect(run).toMatchObject({ autopilot_id: detail.autopilot.id, source: "manual", status: "issue_created", trigger_id: null });
    expect(run.issue_id).toMatch(/^[0-9a-f-]{36}$/);
    const issue = await api.requestJSON<ReportIssue>(`/api/issues/${run.issue_id}`);
    // Manual runs intentionally use UTC; scheduled timezone interpolation is
    // covered by the real-DB autopilot template backend suite.
    expect(issue).toMatchObject({
      title: `${template.title} — ${new Date(run.triggered_at).toISOString().slice(0, 10)}`,
      assignee_id: reporter.id, assignee_type: "agent",
    });
    expect(issue.description).toContain(template.prompt);
    expect(await api.countIssueDispatches(issue.id)).toBe(1);
    const afterDispatch = await api.requestJSON<{ issues: ReportIssue[] }>("/api/issues");
    expect(afterDispatch.issues.map((item) => item.id).sort()).toEqual([...before.issues.map((item) => item.id), issue.id].sort());
    const issueLink = page.getByRole("link", { name: /已关联任务/ });
    await expect(issueLink).toHaveAttribute("href", `/${slug}/issues/${issue.id}`);
    await issueLink.click();
    await page.waitForURL((url) => url.pathname === `/${slug}/issues/${issue.id}`);
    await expect(page.getByText(issue.title, { exact: true }).first()).toBeVisible();

    const marker = `${template.title}：端到端测试夹具投递（非模型生成）`;
    const content = [
      `## ${marker}`, "", "统计范围：当前隔离验收工作区；时区：Asia/Shanghai。",
      `统计窗口：过去 ${template.key === DAILY_KEY ? "24 小时" : "7 天"}；本期新建 3，关闭 1，净变化 +2。`,
      "关闭口径：本期转为 done 的业务任务；排除日报、周报自身生成的任务。", "",
      `- 已完成：[${done.title}](/${slug}/issues/${done.id})`,
      `- 进行中：[${ongoing.title}](/${slug}/issues/${ongoing.id})`,
      `- 阻塞：[${blocked.title}](/${slug}/issues/${blocked.id})；下一步：由负责人确认解除条件。`,
    ].join("\n");
    const comment = await api.requestJSON<{ id: string; content: string }>(`/api/issues/${issue.id}/comments`, {
      method: "POST", body: { content, suppress_agent_ids: [reporter.id] },
    });
    expect(comment.content).toBe(content);
    // Persisted comment alone does not complete a create_issue run.
    expect((await api.requestJSON<{ runs: ReportRun[] }>(`/api/autopilots/${detail.autopilot.id}/runs`)).runs[0].status).toBe("issue_created");
    await page.reload();
    await expect(page.getByText(marker, { exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: done.title, exact: true })).toHaveAttribute("href", `/${slug}/issues/${done.id}`);

    // The authenticated fixture performs the permitted report-only transition;
    // no SQL shortcut marks a run complete and no business task is written.
    await api.updateIssue(issue.id, { status: "in_review" });
    await expect.poll(async () => (await api.requestJSON<{ runs: ReportRun[] }>(`/api/autopilots/${detail.autopilot.id}/runs`)).runs[0].status).toBe("completed");
    const completed = (await api.requestJSON<{ runs: ReportRun[] }>(`/api/autopilots/${detail.autopilot.id}/runs`)).runs[0];
    expect(completed.completed_at).not.toBeNull();
    expect((await api.requestJSON<ReportIssue>(`/api/issues/${issue.id}`)).status).toBe("in_review");
    const afterDelivery = await api.requestJSON<{ issues: ReportIssue[] }>("/api/issues");
    expect(afterDelivery.issues.map((item) => item.id).sort()).toEqual(afterDispatch.issues.map((item) => item.id).sort());
    const sourceAfter = await Promise.all(sourceIds.map((id) => api.requestJSON<ReportIssue>(`/api/issues/${id}`)));
    expect(sourceAfter.map((source) => ({ id: source.id, status: source.status }))).toEqual(sourceBefore.map((source) => ({ id: source.id, status: source.status })));
    await page.reload();
    const reportHeading = page.getByText(marker, { exact: true });
    await expect(reportHeading).toBeVisible();
    // Scroll as a user would: the virtualized issue timeline can overwrite
    // programmatic scroll offsets while its long description is measured.
    await page.mouse.move(700, 500);
    await expect(async () => {
      await page.mouse.wheel(0, 800);
      await expect(reportHeading).toBeInViewport({ timeout: 1000 });
    }).toPass({ timeout: 15_000 });
    await attachJSON(testInfo, `${template.key}-comment-viewport`, await reportHeading.boundingBox());
    await capture(page, testInfo, `${template.key}-delivered-1440`);
    await page.goto(`/${slug}/autopilots/${detail.autopilot.id}`);
    await expect(page.getByText("已完成", { exact: true })).toBeVisible();
    await page.getByText("已完成", { exact: true }).scrollIntoViewIfNeeded();
    await expect(page.getByText("已完成", { exact: true })).toBeInViewport();
    await capture(page, testInfo, `${template.key}-completed-1440`);
    reports.push({ template_key: template.key, autopilot: detail, run: completed, issue, comment, sourceBefore, sourceAfter });
  }
  await attachJSON(testInfo, "real-api-report-delivery-fixture", { execution: "Controlled fixture delivery; no model process or account", reports });
});
