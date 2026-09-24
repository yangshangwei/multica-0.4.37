import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { TestApiClient } from "./fixtures";

// This is the real catalog → draft → create path. The source catalog and the
// create request are never intercepted. Parsing edge cases belong to the core
// template-draft suite; this spec checks the saved result across UI/API layers.

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ||
  `http://localhost:${process.env.PORT || "8080"}`;
const WORKER =
  process.env.TEST_PARALLEL_INDEX ?? process.env.TEST_WORKER_INDEX ?? "0";
const RUN_ID =
  process.env.E2E_RUN_ID ??
  `${Date.now().toString(36)}-${process.pid.toString(36)}`;
const TEMPLATE_NAME = "multica-code-review";

interface SkillSnapshot {
  id: string;
  workspace_id: string;
  name: string;
  description: string;
  content: string;
  config: Record<string, unknown>;
  created_by: string;
  files: { path: string; content: string }[];
}

interface TemplateSnapshot {
  name: string;
  version: number;
  description: string;
  content: string;
  files: { path: string; content: string }[];
}

async function capture(page: Page, testInfo: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const screenshotPath = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path: screenshotPath, animations: "disabled" });
  await testInfo.attach(name, {
    path: screenshotPath,
    contentType: "image/png",
  });
}

async function expectControlInViewport(page: Page, name: string) {
  const control = page.getByRole("dialog").getByRole("button", {
    name,
    exact: true,
  });
  await expect(control).toBeVisible();
  const bounds = await control.boundingBox();
  expect(bounds).not.toBeNull();
  const viewport = page.viewportSize();
  expect(viewport).not.toBeNull();
  expect(bounds!.y).toBeGreaterThanOrEqual(0);
  expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(viewport!.height);
}

test("creates an edited independent skill after previewing and cancelling without writes", async ({
  page,
}, testInfo) => {
  // The first navigation to skill details compiles that route in local Next
  // development servers; keep that build time outside the product wait limit.
  test.setTimeout(120_000);
  const api = new TestApiClient();
  const slug = `e2e-skill-tpl-${WORKER}-${RUN_ID}`;
  const login = await api.login(
    `e2e-skill-tpl-${WORKER}-${RUN_ID}@multica.ai`,
    "E2E Skill Template User",
  );
  const workspace = await api.ensureWorkspace("E2E Skill Template QA", slug);
  // ensureWorkspace may reuse the first workspace: fail before any fixture
  // writes or cleanup if it did not create/select this test's unique workspace.
  expect(workspace.slug).toBe(slug);
  const token = api.getToken();
  if (!token) throw new Error("E2E login did not return a token");

  async function request(endpoint: string, init?: RequestInit) {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
        "X-Workspace-Slug": workspace.slug,
        ...init?.headers,
      },
    });
    expect(response.ok, `${init?.method ?? "GET"} ${endpoint}`).toBeTruthy();
    return response;
  }

  try {
    await api.markUserOnboarded();
    await request("/api/me", {
      method: "PATCH",
      body: JSON.stringify({ language: "en" }),
    });
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
      document.cookie = "multica-locale=en; path=/; SameSite=Lax";
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, token);

    const initialSkills = await (await request("/api/skills")).json();
    expect(initialSkills).toEqual([]);
    const catalog: { templates: TemplateSnapshot[] } = await (
      await request("/api/skills/templates")
    ).json();
    expect(catalog.templates).toHaveLength(15);
    const source = catalog.templates.find((item) => item.name === TEMPLATE_NAME);
    expect(source).toBeDefined();
    const catalogBefore = JSON.stringify(catalog);
    const membersBefore = await (
      await request(`/api/workspaces/${workspace.id}/members`)
    ).json();
    const agentsBefore = await (await request("/api/agents")).json();

    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto(`/${slug}/skills`);
    await page.getByRole("button", { name: "New skill", exact: true }).first().click();
    const dialog = page.getByRole("dialog");
    const methods = [
      "Create manually",
      "Modify from template",
      "Import from local",
      "Import from URL",
      "Copy from runtime",
    ];
    for (const method of methods) {
      await expect(dialog.getByRole("button", { name: new RegExp(`^${method}`) })).toBeVisible();
    }
    await capture(page, testInfo, "desktop-chooser");

    await page.setViewportSize({ width: 375, height: 667 });
    await dialog.getByRole("button", { name: /^Copy from runtime/ }).focus();
    await expect(dialog.getByRole("button", { name: /^Copy from runtime/ })).toBeInViewport();
    await expectControlInViewport(page, "Close");
    await capture(page, testInfo, "small-chooser");
    await dialog.getByRole("button", { name: /^Modify from template/ }).click();
    await expect(dialog.getByRole("button", { name: "Use this template", exact: true })).toBeVisible();
    const templateRow = dialog.getByRole("button", { name: new RegExp(`^${TEMPLATE_NAME}\\b`) });
    const useTemplate = dialog.getByRole("button", { name: "Use this template", exact: true });
    await templateRow.focus();
    await page.keyboard.press("Enter");
    await expect(useTemplate).toBeFocused();
    await dialog.getByRole("button", { name: "Back to templates", exact: true }).click();
    await expect(templateRow).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(useTemplate).toBeFocused();
    await expect(dialog).toContainText("代码审查");
    await expectControlInViewport(page, "Use this template");
    await capture(page, testInfo, "small-template-preview");
    expect(await (await request("/api/skills")).json()).toEqual([]);

    // Browsing has no dirty draft and closes directly, without materializing
    // any built-in catalog entries in this empty workspace.
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    expect(await (await request("/api/skills")).json()).toEqual([]);

    // A workspace-owned skill with the canonical template name must never be
    // used as, or overwritten by, the built-in source. This row is disposable.
    const existing: SkillSnapshot = await (
      await request("/api/skills", {
        method: "POST",
        body: JSON.stringify({
          name: TEMPLATE_NAME,
          description: "Workspace custom source to preserve",
          content: "# Workspace custom source\n\nKeep this existing skill unchanged.\n",
          files: [{ path: "references/owned.txt", content: "Workspace-owned file\n" }],
        }),
      })
    ).json();
    const existingBefore = await (
      await request(`/api/skills/${existing.id}`)
    ).json();

    await page.setViewportSize({ width: 1280, height: 720 });
    await page.reload();
    await page.getByRole("button", { name: "New skill", exact: true }).first().click();
    await dialog.getByRole("button", { name: /^Modify from template/ }).click();
    await dialog.getByRole("button", { name: new RegExp(`^${TEMPLATE_NAME}\\b`) }).click();
    await expect(dialog).toContainText("代码审查");
    await capture(page, testInfo, "desktop-template-preview");
    await dialog.getByRole("button", { name: "Use this template", exact: true }).click();
    const name = dialog.getByLabel("Name", { exact: true });
    const description = dialog.getByLabel("Description", { exact: true });
    const instructions = dialog.getByLabel("Instructions", { exact: true });
    await expect(name).toHaveValue(`${TEMPLATE_NAME}-copy`);
    const originalBody = await instructions.inputValue();
    expect(originalBody).toContain("代码审查");
    expect(originalBody).not.toContain("Workspace custom source");
    expect(originalBody).not.toMatch(/^---/);

    const copyName = `e2e-review-copy-${RUN_ID}`;
    const copyDescription = "Review changes while preserving the original template.";
    const editedBody = `${originalBody}\n\n## QA 自定义检查\n\n保留原模板，检查自己的副本。\n\n---\n\nKeep this body separator.\n`;
    await name.fill(copyName);
    await description.fill(copyDescription);
    await instructions.fill(editedBody);
    await capture(page, testInfo, "desktop-template-edit");
    await page.setViewportSize({ width: 375, height: 667 });
    await expectControlInViewport(page, "Create skill");
    await capture(page, testInfo, "small-template-edit");
    expect(await (await request("/api/skills")).json()).toHaveLength(1);

    let createRequests = 0;
    page.on("request", (request) => {
      if (request.method() === "POST" && new URL(request.url()).pathname === "/api/skills") createRequests++;
    });
    const createResponsePromise = page.waitForResponse((response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/skills",
    );
    await dialog.getByRole("button", { name: "Create skill", exact: true }).click();
    const createResponse = await createResponsePromise;
    expect(createResponse.status()).toBe(201);
    const created: SkillSnapshot = await createResponse.json();
    await expect(page).toHaveURL(new RegExp(`/${slug}/skills/${created.id}$`), { timeout: 30_000 });
    await expect(dialog).toHaveCount(0);
    expect(createRequests).toBe(1);
    const saved: SkillSnapshot = await (
      await request(`/api/skills/${created.id}`)
    ).json();
    expect(saved.id).not.toBe(existing.id);
    expect(saved.workspace_id).toBe(workspace.id);
    expect(saved.created_by).toBe(login.user.id);
    expect(saved.name).toBe(copyName);
    expect(saved.description).toBe(copyDescription);
    expect(saved.content).toContain(`\nname: ${copyName}\n`);
    const descriptionLine = saved.content.split("\n").find((line) => line.startsWith("description:"));
    expect([
      `description: ${copyDescription}`,
      `description: "${copyDescription}"`,
      `description: '${copyDescription}'`,
    ]).toContain(descriptionLine);
    expect(saved.content).toContain("\nuser-invocable: false\n");
    expect(saved.content.slice(saved.content.indexOf("\n---", 4) + 5)).toBe(editedBody);
    expect(saved.config).toMatchObject({
      template_source: { name: TEMPLATE_NAME, version: source!.version },
    });
    expect(saved.config).not.toHaveProperty("origin");
    expect(saved.files.map(({ path, content }) => ({ path, content }))).toEqual(source!.files);
    expect(await (await request("/api/skills")).json()).toHaveLength(2);
    expect(await (await request(`/api/skills/${existing.id}`)).json()).toEqual(existingBefore);
    expect(JSON.stringify(await (await request("/api/skills/templates")).json())).toBe(catalogBefore);
    expect(await (await request(`/api/workspaces/${workspace.id}/members`)).json()).toEqual(membersBefore);
    expect(await (await request("/api/agents")).json()).toEqual(agentsBefore);
    await capture(page, testInfo, "small-created-detail");
    await page.setViewportSize({ width: 1280, height: 720 });
    await capture(page, testInfo, "desktop-created-detail");
    await writeFile(testInfo.outputPath("verification.json"), JSON.stringify({
      status: "passed",
      workspaceSlug: slug,
      createdSkillId: saved.id,
      createdSkillName: saved.name,
      sourceTemplate: source!.name,
      sourceVersion: source!.version,
      preservedWorkspaceSkillId: existing.id,
      previewSkillCount: 0,
      cancelledSkillCount: 0,
      createdSkillCount: 2,
      browserCreateRequests: createRequests,
      metadataAndFrontmatterMatch: true,
      bodyAndSupportingFilesMatch: true,
      sourceCatalogUnchanged: true,
      existingWorkspaceSkillUnchanged: true,
      membersAndAgentsUnchanged: true,
    }, null, 2));
  } finally {
    await api.cleanup();
    await request(`/api/workspaces/${workspace.id}`, { method: "DELETE" });
  }
});
