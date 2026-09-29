import { randomUUID } from "node:crypto";
import { test, expect, type Page, type Locator, type Response } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

// Run against the real API configured with the isolated deterministic AI provider.
// No browser routes replace optimization, creation, or persistence endpoints.
const refined = "Build a team weekly report feature.";
const questions = [
  "Who writes and reads the weekly report?",
  "Should reports be written manually or generated from task progress?",
];
const optimizePath = "/api/issues/optimize-description";

function responseFor(response: Response, path: string) {
  return new URL(response.url()).pathname.replace(/\/$/, "") === path
    && response.request().method() === "POST";
}

async function setup(page: Page, mode: "manual" | "agent" = "manual") {
  const suffix = randomUUID().slice(0, 12);
  const api = new TestApiClient();
  await api.login(`e2e-assist-${suffix}@multica.ai`, "AI flow owner");
  const workspace = await api.ensureWorkspace("AI flow " + suffix, "assist-" + suffix);
  await api.markUserOnboarded();
  const token = api.getToken();
  if (!token) throw new Error("Missing fixture token");
  await page.addInitScript(({ token, mode }) => {
    localStorage.setItem("multica_token", token);
    localStorage.setItem("multica:chat:isOpen", "false");
    localStorage.setItem("multica_create_mode", JSON.stringify({ state: { lastMode: mode }, version: 0 }));
  }, { token, mode });
  return { api, workspace, suffix };
}

async function open(page: Page, slug: string, mode: "manual" | "agent" = "manual") {
  await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "New Issue", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: mode === "manual" ? "New Issue" : "Quick create issue", exact: true });
  await expect(dialog).toBeVisible({ timeout: 30_000 });
  return { dialog, editor: dialog.locator('.ProseMirror.rich-text-editor[contenteditable="true"]') };
}

async function refine(page: Page, dialog: Locator, editor: Locator, sentinel: string) {
  await editor.fill(`I want a team weekly report feature. ${sentinel}`);
  const response = page.waitForResponse((res) => responseFor(res, optimizePath));
  await dialog.getByRole("button", { name: "Help me refine", exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(editor).toHaveText(refined);
}

test.use({ locale: "en-US", screenshot: "only-on-failure", trace: "retain-on-failure" });
test.describe("AI refinement through real server and task creation", () => {
  test.setTimeout(120_000);
  test.skip(!process.env.E2E_ASSIST_PROVIDER_URL, "Requires the deterministic AI provider and isolated API configuration");

  test("refines a vague feature, preserves folded answers, merges and undoes, then persists exactly once", async ({ page }, info) => {
    const { api, workspace, suffix } = await setup(page);
    try {
      await page.setViewportSize({ width: 1280, height: 900 });
      const { dialog, editor } = await open(page, workspace.slug);
      const title = "Weekly report " + suffix;
      await dialog.getByRole("textbox", { name: "Issue title", exact: true }).fill(title);
      await refine(page, dialog, editor, "E2E_ASSIST_TWO");
      await expect(dialog.getByRole("button", { name: "Key details · 2", exact: true })).toHaveAttribute("aria-expanded", "true");
      await dialog.getByLabel(questions[0], { exact: true }).fill("Team members write it; their team lead reads it.");
      await dialog.getByRole("button", { name: "Key details · 2", exact: true }).click();
      await expect(dialog.getByLabel(questions[0], { exact: true })).toBeHidden();
      await dialog.getByRole("button", { name: /Key details · 2/ }).click();
      await expect(dialog.getByLabel(questions[0], { exact: true })).toHaveValue("Team members write it; their team lead reads it.");
      await dialog.getByRole("button", { name: "Let the assignee decide", exact: true }).nth(1).click();
      await expect(dialog.getByLabel(questions[1], { exact: true })).toHaveValue("Let the assignee decide");
      await dialog.getByRole("button", { name: "Add answers to draft", exact: true }).click();
      await expect(dialog.getByRole("status")).toHaveText("Added 2 details");
      await expect(editor).toContainText("Team members write it; their team lead reads it.");
      await dialog.getByRole("button", { name: "Undo", exact: true }).click();
      await expect(editor).toHaveText(refined);
      await expect(dialog.getByLabel(questions[0], { exact: true })).toHaveValue("Team members write it; their team lead reads it.");
      await expect(dialog.getByLabel(questions[1], { exact: true })).toHaveValue("Let the assignee decide");
      await dialog.getByRole("button", { name: "Add answers to draft", exact: true }).click();
      await page.screenshot({ path: info.outputPath("manual-refined.png") });
      const createdResponse = page.waitForResponse((res) => responseFor(res, "/api/issues"));
      await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
      const response = await createdResponse;
      expect(response.status()).toBe(201);
      const issue: { id: string } = await response.json();
      const saved = await api.requestJSON<{ title: string; description: string }>(`/api/issues/${issue.id}`);
      expect(saved.title).toBe(title);
      expect(saved.description).toContain(refined);
      expect(saved.description.match(/Additional details/g)).toHaveLength(1);
      for (const question of questions) expect(saved.description).toContain(question);
      expect(saved.description).toContain("Let the assignee decide");
      await expect(dialog).toBeHidden();
      await page.goto(`/${workspace.slug}/issues/${issue.id}`);
      await expect(page.locator('.ProseMirror.rich-text-editor[contenteditable="true"]').filter({ hasText: refined })).toBeVisible();
    } finally { await api.deleteFeatureWorkspace(workspace.id); }
  });

  for (const count of [0, 1]) {
    test(`handles ${count} dynamic clarification questions and undo`, async ({ page }) => {
      const { api, workspace } = await setup(page);
      try {
        const { dialog, editor } = await open(page, workspace.slug);
        const sentinel = count === 0 ? "E2E_ASSIST_ZERO" : "E2E_ASSIST_ONE";
        await refine(page, dialog, editor, sentinel);
        await expect(dialog.getByPlaceholder("Add a short answer...", { exact: true })).toHaveCount(count);
        if (!count) await expect(dialog.getByRole("status")).toHaveText("Refined, ready to create");
        await dialog.getByRole("button", { name: "Undo", exact: true }).click();
        await expect(editor).toContainText(sentinel);
        await expect(dialog.getByPlaceholder("Add a short answer...", { exact: true })).toHaveCount(0);
      } finally { await api.deleteFeatureWorkspace(workspace.id); }
    });
  }

  test("preserves edits during generation, supports cancellation and real provider failure", async ({ page }) => {
    const { api, workspace } = await setup(page);
    try {
      const { dialog, editor } = await open(page, workspace.slug);
      await editor.fill("Weekly reports E2E_ASSIST_DELAY");
      const done = page.waitForResponse((res) => responseFor(res, optimizePath));
      await dialog.getByRole("button", { name: "Help me refine", exact: true }).click();
      await editor.fill("Keep my later edit about monthly reports.");
      await done;
      await expect(dialog.getByRole("status")).toHaveText("Content changed. Optimize again to use your latest edits.");
      await expect(editor).toHaveText("Keep my later edit about monthly reports.");
      await dialog.getByRole("button", { name: "Discard", exact: true }).click();
      await editor.fill("Cancel this E2E_ASSIST_DELAY");
      await dialog.getByRole("button", { name: "Help me refine", exact: true }).click();
      await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
      await expect(dialog.getByRole("button", { name: "Help me refine", exact: true })).toBeEnabled();
      await expect(editor).toHaveText("Cancel this E2E_ASSIST_DELAY");
      await editor.fill("Preserve this draft E2E_ASSIST_FAIL");
      await dialog.getByRole("button", { name: "Help me refine", exact: true }).click();
      await expect(dialog.getByRole("alert")).toContainText("Optimization failed.");
      await expect(editor).toHaveText("Preserve this draft E2E_ASSIST_FAIL");
      await editor.fill("Retry with complete requirements E2E_ASSIST_ZERO");
      await dialog.getByRole("button", { name: "Try again", exact: true }).click();
      await expect(editor).toHaveText(refined);
    } finally { await api.deleteFeatureWorkspace(workspace.id); }
  });

  for (const mode of ["manual", "agent"] as const) {
    test(`${mode} keeps creation reachable at desktop and phone widths with long content`, async ({ page }, info) => {
      const { api, workspace } = await setup(page, mode);
      try {
        const { dialog, editor } = await open(page, workspace.slug, mode);
        await refine(page, dialog, editor, "E2E_ASSIST_TWO");
        await editor.fill("Weekly reporting requirements.\n".repeat(80));
        for (const viewport of [{ width: 1280, height: 900 }, { width: 390, height: 844 }, { width: 667, height: 375 }]) {
          await page.setViewportSize(viewport);
          const submit = dialog.getByRole("button", { name: mode === "manual" ? "Create Issue" : "Create", exact: true });
          await expect(submit).toBeInViewport();
          await dialog.getByLabel(questions[0], { exact: true }).fill("Team leads");
          await expect(dialog.getByRole("button", { name: "Add answers to draft", exact: true })).toBeEnabled();
          await expect(submit).toBeInViewport();
          const box = await dialog.boundingBox();
          expect(box).not.toBeNull();
          expect(box!.width).toBeLessThanOrEqual(viewport.width);
          await expect.poll(async () => (await dialog.boundingBox())?.height ?? Infinity).toBeLessThanOrEqual(viewport.height * 0.85 + 1);
          await page.screenshot({ path: info.outputPath(`${mode}-${viewport.width}.png`) });
        }
      } finally { await api.deleteFeatureWorkspace(workspace.id); }
    });
  }

  test("agent creation queues the refined prompt through the real API without executing a daemon", async ({ page }) => {
    const { api, workspace, suffix } = await setup(page, "agent");
    try {
      const runtime = await api.seedProjectRuntime();
      const agent = await api.requestJSON<{ id: string; name: string }>("/api/agents", {
        method: "POST", body: { name: "Weekly reporter " + suffix, runtime_id: runtime.id, visibility: "workspace" },
      });
      const { dialog, editor } = await open(page, workspace.slug, "agent");
      await dialog.getByRole("button", { name: /Creation assistant/ }).click();
      await page.locator("button[data-picker-item]").filter({ hasText: agent.name }).click();
      await refine(page, dialog, editor, "E2E_ASSIST_ONE");
      // Unanswered clarifications must never block task creation.
      const accepted = page.waitForResponse((res) => responseFor(res, "/api/issues/quick-create"));
      await dialog.getByRole("button", { name: "Create", exact: true }).click();
      const response = await accepted;
      expect(response.status()).toBe(202);
      const task: { task_id: string } = await response.json();
      await expect(dialog).toBeHidden();
      if (!process.env.DATABASE_URL) throw new Error("Explicit isolated DATABASE_URL required");
      const db = new pg.Client(process.env.DATABASE_URL);
      await db.connect();
      try {
        const saved = await db.query<{ context: { prompt: string }; agent_id: string }>(
          "SELECT context, agent_id FROM agent_task_queue WHERE id = $1", [task.task_id],
        );
        expect(saved.rows).toHaveLength(1);
        expect(saved.rows[0]).toMatchObject({ agent_id: agent.id, context: { prompt: refined } });
      } finally { await db.end(); }
    } finally { await api.deleteFeatureWorkspace(workspace.id); }
  });
});
