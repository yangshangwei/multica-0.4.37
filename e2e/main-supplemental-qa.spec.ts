import { randomUUID } from "node:crypto";
import { test, expect, type Page, type Response } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

// Main 1904f9aa6 previews suggestions and requires explicit adoption. These
// tests intentionally do not depend on uncommitted automatic-refinement UI.
const optimizePath = "/api/issues/optimize-description";
const refined = "Build a team weekly report feature.";
const question = "Who writes and reads the weekly report?";
const provider = process.env.E2E_ASSIST_PROVIDER_URL;
type Issue = { id: string; identifier: string; title: string; description: string; status: string; priority: string };
type TestWorkspace = Awaited<ReturnType<TestApiClient["ensureWorkspace"]>>;

test.use({ locale: "en-US", viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure", screenshot: "only-on-failure", actionTimeout: 15_000 });
test.setTimeout(150_000);

function localURL(name: string): URL {
  const value = process.env[name];
  if (!value) throw new Error(`Supplemental QA requires explicit ${name}`);
  const url = new URL(value);
  if (!["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)) throw new Error(`${name} must use loopback`);
  return url;
}

test.beforeEach(({ baseURL }) => {
  if (!baseURL) throw new Error("Supplemental QA requires an explicit browser baseURL");
  const browserURL = new URL(baseURL);
  if (!["http:", "https:"].includes(browserURL.protocol) || !["localhost", "127.0.0.1", "[::1]"].includes(browserURL.hostname)) {
    throw new Error("Supplemental QA browser baseURL must use loopback HTTP(S)");
  }
  localURL("NEXT_PUBLIC_API_URL");
  const database = localURL("DATABASE_URL");
  if (!["postgres:", "postgresql:"].includes(database.protocol) || !database.pathname.slice(1) || database.pathname === "/multica") {
    throw new Error("Supplemental QA requires an isolated test database");
  }
});

function matches(response: Response, path: string, method = "POST") {
  return new URL(response.url()).pathname.replace(/\/$/, "") === path && response.request().method() === method;
}

async function account(label: string) {
  const suffix = randomUUID().slice(0, 12);
  const api = new TestApiClient();
  await api.login(`e2e-main-${label}-${suffix}@example.invalid`, `Main ${label} QA`);
  const workspace = await api.ensureWorkspace(`Main ${label} ${suffix}`, `main-${label}-${suffix}`);
  expect(workspace.slug).toBe(`main-${label}-${suffix}`);
  await api.markUserOnboarded();
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
  return { api, workspace, suffix };
}

async function authenticate(page: Page, api: TestApiClient, mode: "manual" | "agent" = "manual") {
  const token = api.getToken();
  if (!token) throw new Error("Missing fixture authentication");
  await page.addInitScript(({ token, mode }) => {
    localStorage.setItem("multica_token", token);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
    localStorage.setItem("multica_create_mode", JSON.stringify({ state: { lastMode: mode }, version: 0 }));
  }, { token, mode });
}

async function openCreate(page: Page, slug: string, mode: "manual" | "agent" = "manual") {
  await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "New Issue", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: mode === "manual" ? "New Issue" : "Quick create issue", exact: true });
  await expect(dialog).toBeVisible();
  return { dialog, editor: dialog.locator('.ProseMirror.rich-text-editor[contenteditable="true"]') };
}

async function requireProvider(page: Page) {
  const url = localURL("E2E_ASSIST_PROVIDER_URL");
  expect(localURL("MULTICA_LLM_BASE_URL").origin).toBe(url.origin);
  const health = await page.request.get(`${url.origin}/health`);
  expect(health.ok()).toBe(true);
  expect(await health.json()).toMatchObject({ fixture: "main-supplemental-qa" });
}

test("real email code login survives refresh and logout protects the workspace", async ({ page }, info) => {
  const { api, workspace } = await account("auth");
  try {
    // The fixture token never enters this browser; login must obtain its own.
    await page.goto(`/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/login(?:\?|$)/);
    await expect(page.getByText("Sign in to Multica", { exact: true })).toBeVisible();
    const submit = page.getByRole("button", { name: "Continue", exact: true });
    await expect(submit).toBeDisabled();
    await page.getByRole("textbox", { name: "Email", exact: true }).fill(api.getEmail());
    const sent = page.waitForResponse((response) => matches(response, "/auth/send-code"));
    await submit.click();
    expect((await sent).ok()).toBe(true);
    await expect(page.getByText("Check your email", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: /Resend in/ })).toBeDisabled();
    const database = new pg.Client(process.env.DATABASE_URL);
    await database.connect();
    let code: string;
    try {
      const result = await database.query<{ code: string }>("SELECT code FROM verification_code WHERE email = $1 AND used = FALSE AND expires_at > now() ORDER BY created_at DESC LIMIT 1", [api.getEmail()]);
      expect(result.rows).toHaveLength(1);
      code = process.env.MULTICA_DEV_VERIFICATION_CODE?.trim() || result.rows[0]!.code;
    } finally { await database.end(); }
    const verified = page.waitForResponse((response) => matches(response, "/auth/verify-code"));
    await page.locator('input[data-slot="input-otp"]').fill(code);
    expect((await verified).ok()).toBe(true);
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/issues$`), { timeout: 30_000 });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "New Issue", exact: true })).toBeVisible();
    await page.screenshot({ path: info.outputPath("authenticated-after-refresh.png") });
    // The avatar initial participates in this button's accessible name.
    await page.getByRole("button", { name: workspace.name }).click();
    await page.getByRole("menuitem", { name: "Log out", exact: true }).click();
    await expect(page).toHaveURL(/\/login(?:\?|$)/);
    await page.goto(`/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/login(?:\?|$)/);
    await expect(page.getByRole("textbox", { name: "Email", exact: true })).toBeVisible();
  } finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("manual issue status, priority and comment persist through refresh and confirmed deletion", async ({ page }, info) => {
  const { api, workspace, suffix } = await account("lifecycle");
  try {
    await authenticate(page, api);
    const { dialog, editor } = await openCreate(page, workspace.slug);
    const title = `Weekly report ${suffix}`;
    const description = "Team members submit progress and blockers for their manager.";
    await dialog.getByRole("textbox", { name: "Issue title", exact: true }).fill(title);
    await editor.fill(description);
    const created = page.waitForResponse((response) => matches(response, "/api/issues"));
    await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
    const response = await created;
    expect(response.status()).toBe(201);
    const issue: Issue = await response.json();
    expect(issue).toMatchObject({ title, description });
    await expect(dialog).toBeHidden();
    await page.getByRole("button", { name: "View issue", exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/issues/${issue.identifier}$`));
    await page.getByRole("button", { name: "Todo", exact: true }).click();
    await page.locator("button[data-picker-item]").filter({ hasText: /^In Progress$/ }).click();
    await page.getByRole("button", { name: "Add property", exact: true }).click();
    await page.getByRole("button", { name: "Priority", exact: true }).click();
    await page.locator("button[data-picker-item]").filter({ hasText: /^High$/ }).click();
    await expect.poll(() => api.requestJSON<Issue>(`/api/issues/${issue.id}`)).toMatchObject({ status: "in_progress", priority: "high" });
    await page.getByTestId("comment-composer-shell").click();
    const comment = `Start with manual reports and verify adoption. ${suffix}`;
    const composer = page.locator("div.rounded-lg.bg-card").filter({ has: page.getByRole("button", { name: "Send", exact: true }) }).locator('.ProseMirror[contenteditable="true"]');
    await composer.fill(comment);
    const commented = page.waitForResponse((result) => matches(result, `/api/issues/${issue.id}/comments`));
    await composer.press("ControlOrMeta+Enter");
    const commentResponse = await commented;
    expect(commentResponse.ok()).toBe(true);
    const saved: { id: string } = await commentResponse.json();
    await expect(page.locator(`#comment-body-${saved.id}`)).toHaveText(comment);
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: title, exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "High", exact: true })).toBeVisible();
    await expect(page.locator(`#comment-body-${saved.id}`)).toHaveText(comment);
    await page.screenshot({ path: info.outputPath("persisted-issue.png") });
    const actions = page.locator('main button[data-slot="dropdown-menu-trigger"]:not([aria-label]):has(svg.lucide-ellipsis)');
    await actions.click();
    await page.getByRole("menuitem", { name: "Delete issue", exact: true }).click();
    const confirmation = page.getByRole("alertdialog", { name: "Delete issue", exact: true });
    await confirmation.getByRole("button", { name: "Cancel", exact: true }).click();
    expect(await api.requestJSON<Issue>(`/api/issues/${issue.id}`)).toMatchObject({ id: issue.id });
    await actions.click();
    await page.getByRole("menuitem", { name: "Delete issue", exact: true }).click();
    const deleted = page.waitForResponse((result) => matches(result, `/api/issues/${issue.id}`, "DELETE"));
    await confirmation.getByRole("button", { name: "Delete", exact: true }).click();
    expect((await deleted).ok()).toBe(true);
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/issues$`));
    await expect(api.requestJSON(`/api/issues/${issue.id}`)).rejects.toThrow("404");
  } finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("an onboarded member creates another workspace through About you before workspace and runtime", async ({ page }, info) => {
  const { api, workspace, suffix } = await account("workspace");
  let additional: TestWorkspace | undefined;
  try {
    await authenticate(page, api);
    await page.goto("/workspaces/new", { waitUntil: "domcontentloaded" });
    await expect(page.locator('[aria-current="step"]').filter({ hasText: "About you" })).toBeVisible();
    await expect(page.getByRole("radio", { name: "Engineer", exact: true })).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Workspace name", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Continue on web", exact: true })).toHaveCount(0);
    await page.getByRole("radio", { name: "Engineer", exact: true }).click();
    await page.getByRole("checkbox", { name: /Code & test with agents/i }).click();
    await page.getByRole("button", { name: "Continue", exact: true }).click();
    await expect(page.locator('[aria-current="step"]').filter({ hasText: "Workspace" })).toBeVisible();
    await page.getByRole("button", { name: "Back", exact: true }).click();
    await expect(page.getByRole("radio", { name: "Engineer", exact: true })).toBeChecked();
    await expect(page.getByRole("checkbox", { name: /Code & test with agents/i })).toBeChecked();
    await page.getByRole("button", { name: "Continue", exact: true }).click();
    await page.getByRole("textbox", { name: "Workspace name", exact: true }).fill(`Additional ${suffix}`);
    await page.getByRole("textbox", { name: "URL", exact: true }).fill(`additional-${suffix}`);
    const created = page.waitForResponse((response) => matches(response, "/api/workspaces"));
    await page.getByRole("button", { name: `Create Additional ${suffix}`, exact: true }).click();
    const response = await created;
    expect(response.status()).toBe(201);
    additional = await response.json() as TestWorkspace;
    expect(additional.id).not.toBe(workspace.id);
    await expect(page.locator('[aria-current="step"]').filter({ hasText: "Connect a runtime" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Back", exact: true })).toHaveCount(0);
    const completed = page.waitForResponse((result) => matches(result, "/api/me/onboarding/complete"));
    await page.getByRole("button", { name: "Skip for now", exact: true }).click();
    expect((await completed).ok()).toBe(true);
    await expect(page).toHaveURL(new RegExp(`/${additional.slug}/projects$`), { timeout: 30_000 });
    expect((await api.getWorkspaces()).map((item) => item.id)).toEqual(expect.arrayContaining([workspace.id, additional.id]));
    await expect.poll(() => api.requestJSON("/api/me")).toMatchObject({ onboarding_questionnaire: { role: "engineer", use_case: ["ship_code"] } });
    await page.screenshot({ path: info.outputPath("additional-workspace-projects.png") });
  } finally {
    if (additional) {
      api.setWorkspaceId(additional.id);
      api.setWorkspaceSlug(additional.slug);
      await api.deleteFeatureWorkspace(additional.id);
    }
    api.setWorkspaceId(workspace.id);
    api.setWorkspaceSlug(workspace.slug);
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test.describe("AI suggestions through the real local API", () => {
  test.skip(!provider, "Requires the task-owned supplemental provider and API LLM endpoint");
  test.beforeEach(async ({ page }) => { await requireProvider(page); });

  test("previews, explicitly applies, undoes and persists only the adopted description", async ({ page }, info) => {
    const { api, workspace, suffix } = await account("assist");
    try {
      await authenticate(page, api);
      const { dialog, editor } = await openCreate(page, workspace.slug);
      const source = "I want a team weekly report feature. E2EASSISTONE";
      await dialog.getByRole("textbox", { name: "Issue title", exact: true }).fill(`Assisted ${suffix}`);
      await editor.fill(source);
      const optimized = page.waitForResponse((response) => matches(response, optimizePath));
      await dialog.getByRole("button", { name: "AI optimize description", exact: true }).click();
      expect((await optimized).status()).toBe(200);
      const suggestion = dialog.getByRole("region", { name: "AI suggestion", exact: true });
      await expect(suggestion.getByText(refined, { exact: true })).toBeVisible();
      await expect(suggestion.getByText(question, { exact: true })).toBeVisible();
      await expect(editor).toHaveText(source);
      await suggestion.getByRole("button", { name: "Apply and replace", exact: true }).click();
      await expect(editor).toHaveText(refined);
      await expect(dialog.getByRole("status")).toHaveText("Suggestion applied.");
      await dialog.getByRole("button", { name: "Undo", exact: true }).click();
      await expect(editor).toHaveText(source);
      await dialog.getByRole("button", { name: "AI optimize description", exact: true }).click();
      await suggestion.getByRole("button", { name: "Apply and replace", exact: true }).click();
      await expect(editor).toHaveText(refined);
      await page.screenshot({ path: info.outputPath("adopted-description.png") });
      const created = page.waitForResponse((response) => matches(response, "/api/issues"));
      await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
      const response = await created;
      expect(response.status()).toBe(201);
      const issue: Issue = await response.json();
      expect(await api.requestJSON<Issue>(`/api/issues/${issue.id}`)).toMatchObject({ title: `Assisted ${suffix}`, description: refined });
      await page.goto(`/${workspace.slug}/issues/${issue.identifier}`, { waitUntil: "domcontentloaded" });
      await expect(page.locator('.ProseMirror.rich-text-editor[contenteditable="true"]').filter({ hasText: refined })).toBeVisible();
    } finally { await api.deleteFeatureWorkspace(workspace.id); }
  });

  test("rejects stale adoption, cancels generation and still creates the unchanged draft after provider failure", async ({ page }) => {
    const { api, workspace, suffix } = await account("assistfailure");
    try {
      await authenticate(page, api);
      const { dialog, editor } = await openCreate(page, workspace.slug);
      await dialog.getByRole("textbox", { name: "Issue title", exact: true }).fill(`Preserved ${suffix}`);
      await editor.fill("Weekly reporting E2EASSISTDELAY");
      await dialog.getByRole("button", { name: "AI optimize description", exact: true }).click();
      await editor.fill("Keep my newer monthly reporting requirements.");
      await expect(dialog.getByRole("status")).toHaveText("Content changed. Optimize again to use your latest edits.", { timeout: 15_000 });
      await expect(dialog.getByRole("button", { name: "Apply and replace", exact: true })).toBeDisabled();
      await expect(editor).toHaveText("Keep my newer monthly reporting requirements.");
      await dialog.getByRole("button", { name: "Discard", exact: true }).click();
      await editor.fill("Cancel this E2EASSISTDELAY");
      await dialog.getByRole("button", { name: "AI optimize description", exact: true }).click();
      await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
      await expect(dialog.getByRole("button", { name: "AI optimize description", exact: true })).toBeEnabled();
      await expect(editor).toHaveText("Cancel this E2EASSISTDELAY");
      const failedDraft = "Preserve my original description E2EASSISTFAIL";
      await editor.fill(failedDraft);
      await dialog.getByRole("button", { name: "AI optimize description", exact: true }).click();
      await expect(dialog.getByRole("alert")).toContainText("Optimization failed.");
      await expect(editor).toHaveText(failedDraft);
      await expect(dialog.getByRole("button", { name: "Create Issue", exact: true })).toBeEnabled();
      const created = page.waitForResponse((response) => matches(response, "/api/issues"));
      await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
      const response = await created;
      expect(response.status()).toBe(201);
      const issue: Issue = await response.json();
      expect(await api.requestJSON<Issue>(`/api/issues/${issue.id}`)).toMatchObject({ description: failedDraft });
    } finally { await api.deleteFeatureWorkspace(workspace.id); }
  });

  test("adopted agent instructions reach the real task queue without a daemon", async ({ page }) => {
    const { api, workspace, suffix } = await account("assistagent");
    try {
      await authenticate(page, api, "agent");
      const runtime = await api.seedProjectRuntime();
      const agent = await api.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: { name: `Reporter ${suffix}`, runtime_id: runtime.id, visibility: "workspace" } });
      const { dialog, editor } = await openCreate(page, workspace.slug, "agent");
      await dialog.getByRole("button", { name: /^Creation assistant/ }).click();
      await page.locator(`[data-actor-key="agent:${agent.id}"] button[data-picker-item]`).click();
      await editor.fill("Prepare a team weekly report feature. E2EASSISTONE");
      await dialog.getByRole("button", { name: "AI optimize instructions", exact: true }).click();
      await dialog.getByRole("button", { name: "Apply and replace", exact: true }).click();
      await expect(editor).toHaveText(refined);
      const accepted = page.waitForResponse((response) => matches(response, "/api/issues/quick-create"));
      await dialog.getByRole("button", { name: "Create", exact: true }).click();
      const response = await accepted;
      expect(response.status()).toBe(202);
      const task: { task_id: string } = await response.json();
      await expect(dialog).toBeHidden();
      const database = new pg.Client(process.env.DATABASE_URL);
      await database.connect();
      try {
        const result = await database.query<{ agent_id: string; context: { prompt: string } }>("SELECT agent_id, context FROM agent_task_queue WHERE id = $1", [task.task_id]);
        expect(result.rows).toHaveLength(1);
        expect(result.rows[0]).toMatchObject({ agent_id: agent.id, context: { prompt: refined } });
      } finally { await database.end(); }
    } finally { await api.deleteFeatureWorkspace(workspace.id); }
  });
});
