import { randomUUID } from "node:crypto";
import { test, expect, type Page, type Response } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

// AI refinement, undo, failure recovery and dispatch coverage live in
// issue-assist-flow.spec.ts, using the current editable-draft contract.
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

test("an onboarded member creates another workspace through Welcome, About you and runtime", async ({ page }, info) => {
  const { api, workspace, suffix } = await account("workspace");
  let additional: TestWorkspace | undefined;
  try {
    await authenticate(page, api);
    await page.goto("/workspaces/new", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "Continue on web", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Continue on web", exact: true }).click();
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
