import { randomUUID } from "node:crypto";
import { writeFile } from "node:fs/promises";
import { test, expect, type Locator, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// The existing template spec owns the request-field boundary. This flow uses
// the real catalog, creation, editing and dispatch APIs with an isolated runtime
// that has no daemon process, model account or credentials attached.

interface Template {
  key: string;
  version: number;
  title: string;
  description: string;
  prompt: string;
  cron_expression: string;
  execution_mode: "run_only" | "create_issue";
}

interface AutopilotDetail {
  autopilot: {
    id: string;
    title: string;
    description: string;
    assignee_id: string;
    template_key: string;
    template_version: number;
    execution_mode: Template["execution_mode"];
    issue_title_template: string | null;
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

interface AutopilotRun {
  id: string;
  source: string;
  status: string;
  issue_id: string | null;
  task_id: string | null;
  trigger_id: string | null;
  triggered_at: string;
}

const TEMPLATE_KEYS = [
  "workday-repo-audit",
  "release-readiness",
  "daily-change-review",
  "hourly-queue-check",
  "stale-pr-reminder",
  "bug-triage",
  "weekly-progress-report",
  "dependency-audit",
  "documentation-check",
];
const AGENT_NAME = "中文自动化验收智能体";
const CUSTOM_PROMPT =
  "每个工作日检查仓库依赖和快速测试，只记录有复现证据的问题。先查找已有任务，避免重复；没有异常时保持静默。";

test.use({
  locale: "zh-CN",
  timezoneId: "Asia/Shanghai",
  viewport: { width: 1440, height: 960 },
  trace: "retain-on-failure",
});

async function capture(page: Page, testInfo: TestInfo, name: string) {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: true, animations: "disabled" });
  await testInfo.attach(name, { path, contentType: "image/png" });
}

async function attachJSON(testInfo: TestInfo, name: string, value: unknown) {
  const path = testInfo.outputPath(`${name}.json`);
  await writeFile(path, JSON.stringify(value, null, 2));
  await testInfo.attach(name, { path, contentType: "application/json" });
}

async function expectReadablePreview(page: Page, preview: Locator) {
  await expect(preview).toBeVisible();
  const size = await preview.evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    return {
      left: bounds.left,
      right: bounds.right,
      width: element.clientWidth,
      contentWidth: element.scrollWidth,
      height: element.clientHeight,
      contentHeight: element.scrollHeight,
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

  // Long briefs remain available through the preview's own vertical scroller.
  expect(size.contentHeight).toBeGreaterThan(size.height);
  await preview.evaluate((element) => { element.scrollTop = element.scrollHeight; });
  expect(await preview.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  await preview.evaluate((element) => { element.scrollTop = 0; });
}

async function createFromPreview(
  page: Page,
  api: TestApiClient,
  template: Template,
  agentId: string,
): Promise<AutopilotDetail> {
  const create = page.getByRole("button", { name: "启用自动化", exact: true });
  await expect(create).toBeDisabled();
  await page.getByRole("button", { name: "选择智能体或小队", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: new RegExp(AGENT_NAME) }).click();
  await expect(create).toBeEnabled();
  const responsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname === "/api/autopilots/from-template" &&
    response.request().method() === "POST",
  );
  await create.click();
  expect((await responsePromise).status()).toBe(201);
  await page.waitForURL(/\/autopilots\/[0-9a-f-]+$/);
  const id = new URL(page.url()).pathname.split("/").at(-1);
  expect(id).toMatch(/^[0-9a-f-]{36}$/);
  const detail = await api.requestJSON<AutopilotDetail>(`/api/autopilots/${id}`);
  expect(detail.autopilot).toMatchObject({
    title: template.title,
    description: template.prompt,
    assignee_id: agentId,
    template_key: template.key,
    template_version: template.version,
    execution_mode: template.execution_mode,
  });
  expect(detail.triggers).toHaveLength(1);
  expect(detail.triggers[0]).toMatchObject({
    kind: "schedule",
    enabled: true,
    cron_expression: template.cron_expression,
    timezone: "Asia/Shanghai",
  });
  expect(Date.parse(detail.triggers[0].next_run_at)).not.toBeNaN();
  await expect(page.getByRole("button", { name: "编辑", exact: true })).toBeVisible();

  // Keep the fixture deterministic if a wall-clock schedule boundary happens
  // during the test. Manual runs below still exercise the real admission path.
  await api.requestJSON(`/api/autopilots/${id}/triggers/${detail.triggers[0].id}`, {
    method: "PATCH",
    body: { enabled: false },
  });
  return detail;
}

async function runNow(page: Page, api: TestApiClient, id: string) {
  const responsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname === `/api/autopilots/${id}/trigger` &&
    response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "立即运行", exact: true }).click();
  expect((await responsePromise).ok()).toBe(true);
  const { runs } = await api.requestJSON<{ runs: AutopilotRun[] }>(
    `/api/autopilots/${id}/runs`,
  );
  const run = runs.find((item) => item.source === "manual");
  expect(run).toBeDefined();
  if (!run) throw new Error("Manual run was not persisted");
  return run;
}

test("adopts, edits and dispatches Chinese briefs through the real template API", async ({
  page,
  baseURL,
}, testInfo) => {
  // Local Next.js servers compile the catalog, detail and editor on first use.
  test.setTimeout(120_000);
  const runId = randomUUID().slice(0, 12);
  const slug = `e2e-ap-tpl-zh-${runId}`;
  const api = new TestApiClient();
  await api.login(`${slug}@multica.ai`, "中文自动化验收用户");
  const workspace = await api.ensureWorkspace("中文自动化验收工作区", slug);
  // ensureWorkspace can reuse a user's first workspace; never write to or
  // delete anything unless it selected this test's unique workspace.
  expect(workspace.slug).toBe(slug);

  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    const token = api.getToken();
    if (!token || !baseURL) throw new Error("Authenticated E2E setup is incomplete");
    await page.context().addCookies([
      { name: "multica-locale", value: "zh-Hans", url: baseURL },
      { name: "multica_logged_in", value: "1", url: baseURL },
    ]);
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
    }, token);

    const runtime = await api.seedProjectRuntime();
    const agent = await api.requestJSON<{ id: string }>("/api/agents", {
      method: "POST",
      body: {
        name: AGENT_NAME,
        runtime_id: runtime.id,
        permission_mode: "private",
        max_concurrent_tasks: 3,
      },
    });
    const { templates } = await api.requestJSON<{ templates: Template[] }>(
      "/api/autopilots/templates?language=zh",
    );
    expect(templates.map((template) => template.key)).toEqual(TEMPLATE_KEYS);
    for (const template of templates) {
      expect.soft(/\p{Script=Han}/u.test(template.title), `${template.key}: Chinese catalog title`).toBe(true);
      expect.soft(/\p{Script=Han}/u.test(template.prompt), `${template.key}: Chinese execution brief`).toBe(true);
    }
    const audit = templates.find((template) => template.key === "workday-repo-audit");
    const review = templates.find((template) => template.key === "daily-change-review");
    if (!audit || !review) throw new Error("Required built-in templates are missing");
    expect.soft(audit.title).toBe("工作日仓库巡检");

    const [catalogResponse] = await Promise.all([
      page.waitForResponse((response) => {
        const url = new URL(response.url());
        return url.pathname === "/api/autopilots/templates" && url.searchParams.get("language") === "zh";
      }),
      page.goto(`/${slug}/autopilots/new/template`),
    ]);
    expect(catalogResponse.status()).toBe(200);
    await expect(page.getByRole("heading", { name: "从自动化模板开始" })).toBeVisible();
    for (const template of templates) {
      await expect(page.getByRole("button", { name: new RegExp(template.title) }).first()).toBeVisible();
    }
    await page.getByRole("button", { name: new RegExp(audit.title) }).first().click();
    await page.waitForURL((url) => url.searchParams.get("template") === audit.key);
    // A local Next.js locale rebuild can replace this route with its loading
    // shell after the URL has changed. Wait for the rendered preview itself.
    await expect(page.getByText("交给智能体做的事", { exact: true })).toBeVisible({ timeout: 45_000 });
    const preview = page.locator("pre");
    await expect(preview).toHaveCount(1);
    expect(await preview.textContent()).toBe(audit.prompt);
    await expectReadablePreview(page, preview);
    await capture(page, testInfo, "workday-preview-1440");
    await page.setViewportSize({ width: 390, height: 844 });
    await preview.scrollIntoViewIfNeeded();
    await expectReadablePreview(page, preview);
    await capture(page, testInfo, "workday-preview-390");
    await page.setViewportSize({ width: 1440, height: 960 });

    const createdAudit = await createFromPreview(page, api, audit, agent.id);
    expect(createdAudit.autopilot.issue_title_template).toBeNull();
    const renderedBrief = page.locator('[data-rich-content][data-density="document"]').first();
    const heading = audit.prompt.match(/^#{1,6}\s+(.+)$/m)?.[1];
    if (!heading) throw new Error("The audit brief has no readable section heading");
    await expect(renderedBrief).toContainText(heading);
    await capture(page, testInfo, "workday-detail-1440");

    const issuesBefore = await api.requestJSON<{ issues: { id: string }[] }>("/api/issues");
    expect(issuesBefore.issues).toEqual([]);
    const auditRun = await runNow(page, api, createdAudit.autopilot.id);
    expect(auditRun).toMatchObject({ status: "running", issue_id: null });
    expect(auditRun.task_id).toMatch(/^[0-9a-f-]{36}$/);
    const issuesAfter = await api.requestJSON<{ issues: { id: string }[] }>("/api/issues");
    expect(issuesAfter.issues).toEqual([]);

    await page.getByRole("button", { name: "编辑", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "编辑自动化", exact: true });
    const editor = dialog.locator('.rich-text-editor[contenteditable="true"]');
    await editor.fill(CUSTOM_PROMPT);
    // Replacing the document's text preserves its opening H1 block. Use the
    // editor's paragraph shortcut so the saved brief is intentionally plain.
    await editor.press("ControlOrMeta+Alt+0");
    await expect(editor.locator("p").first()).toHaveText(CUSTOM_PROMPT);
    await expect(editor.locator("h1")).toHaveCount(0);
    // ContentEditor publishes Markdown to its host after a 300 ms debounce.
    await page.waitForTimeout(350);
    const editResponse = page.waitForResponse((response) =>
      new URL(response.url()).pathname === `/api/autopilots/${createdAudit.autopilot.id}` &&
      response.request().method() === "PATCH",
    );
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    expect((await editResponse).status()).toBe(200);
    await expect(dialog).toBeHidden();
    await page.reload();
    await expect(page.locator('[data-rich-content][data-density="document"]').first()).toHaveText(CUSTOM_PROMPT);
    const edited = await api.requestJSON<AutopilotDetail>(`/api/autopilots/${createdAudit.autopilot.id}`);
    expect(edited.autopilot.description).toBe(CUSTOM_PROMPT);

    // Reading the catalog in another UI language must not rewrite an adopted
    // brief or create a second source for the canonical Chinese template body.
    const englishCatalog = await api.requestJSON<{ templates: Template[] }>(
      "/api/autopilots/templates?language=en",
    );
    expect(englishCatalog.templates.find((template) => template.key === audit.key)?.prompt).toBe(audit.prompt);
    expect((await api.requestJSON<AutopilotDetail>(`/api/autopilots/${createdAudit.autopilot.id}`)).autopilot.description).toBe(CUSTOM_PROMPT);
    await capture(page, testInfo, "workday-edited-detail-1440");

    await page.goto(`/${slug}/autopilots/new/template?template=${review.key}`);
    await expect(page.locator("pre")).toHaveText(review.prompt);
    const createdReview = await createFromPreview(page, api, review, agent.id);
    expect.soft(createdReview.autopilot.issue_title_template).toBe("每日变更回顾 — {{date}}");
    const reviewRun = await runNow(page, api, createdReview.autopilot.id);
    expect(reviewRun.status).toBe("issue_created");
    expect(reviewRun.issue_id).toMatch(/^[0-9a-f-]{36}$/);
    const issue = await api.requestJSON<{ title: string; description: string }>(
      `/api/issues/${reviewRun.issue_id}`,
    );
    // Manual runs have no schedule trigger and deliberately format dates in
    // UTC. Scheduled runs' timezone matrix belongs to the backend suite.
    expect(reviewRun.trigger_id).toBeNull();
    const date = new Date(reviewRun.triggered_at).toISOString().slice(0, 10);
    expect.soft(issue.title).toBe(`每日变更回顾 — ${date}`);
    expect(issue.description).toContain(review.prompt);
    await attachJSON(testInfo, "real-api-verification", {
      templates, createdAudit, edited, auditRun, createdReview, reviewRun, issue,
    });
    const issueLink = page.getByRole("link", { name: /已关联任务/ });
    await expect(issueLink).toHaveAttribute("href", `/${slug}/issues/${reviewRun.issue_id}`);
    await capture(page, testInfo, "daily-review-dispatched-1440");
    await issueLink.click();
    await page.waitForURL((url) => url.pathname === `/${slug}/issues/${reviewRun.issue_id}`, { timeout: 45_000 });
    await expect(page.getByText(issue.title, { exact: true }).first()).toBeVisible({ timeout: 15_000 });
    await capture(page, testInfo, "daily-review-issue-1440");
  } catch (error) {
    await capture(page, testInfo, "failure");
    await attachJSON(testInfo, "failure-page", {
      url: page.url(), text: await page.locator("body").innerText(),
    });
    throw error;
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
    const remaining = await api.getWorkspaces();
    const removed = !remaining.some((item) => item.id === workspace.id);
    await attachJSON(testInfo, "workspace-cleanup", { workspace_id: workspace.id, slug, removed });
    expect(removed).toBe(true);
  }
});
