import { test, expect, type Locator, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// This production-browser flow owns catalog discovery, keyboard access and
// real copy creation. Filter edge cases and query failures live in the views
// suites. Only the template GET is intercepted to supply deployment examples;
// authentication, workspace data and the final write use the task-owned API.

const RUN_ID = process.env.E2E_RUN_ID ?? `${Date.now().toString(36)}-${process.pid.toString(36)}`;
const WORKER = process.env.TEST_PARALLEL_INDEX ?? process.env.TEST_WORKER_INDEX ?? "0";
const WIDE = { width: 1440, height: 900 };
const NARROW = { width: 390, height: 844 };
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

interface TemplateSnapshot {
  name: string;
  version: number;
  description: string;
  category: string;
  icon: string;
  content: string;
  files: { path: string; content: string }[];
}

interface SkillSnapshot {
  id: string;
  workspace_id: string;
  name: string;
  content: string;
  config: Record<string, unknown>;
  files: { path: string; content: string }[];
}

const DEPLOYMENT_TEMPLATES: TemplateSnapshot[] = [
  {
    name: "e2e-release-acceptance",
    description: "Review release evidence and verify the critical user journey before shipping.",
    category: "quality",
    icon: "flask-conical",
  },
  {
    name: "e2e-release-operations",
    description: "Prepare a release plan with deployment checks, monitoring and recovery steps.",
    category: "operations",
    icon: "rocket",
  },
  {
    name: "e2e-architecture-notes",
    description: "Document a design decision, its constraints and the alternatives considered.",
    category: "design",
    icon: "landmark",
  },
  {
    name: "e2e-discovery-brief",
    description: "Turn a user problem into a scoped brief with testable acceptance criteria.",
    category: "research",
    icon: "compass",
  },
].map((template) => ({
  ...template,
  version: 1,
  content: `---\nname: ${template.name}\ndescription: ${template.description}\nmetadata:\n  category: ${template.category}\n  icon: ${template.icon}\n---\n# Workflow\n\n${template.description}\n\nRead references/checklist.md and record the evidence.\n`,
  files: [{ path: "references/checklist.md", content: "# Checklist\n\n- Verify the expected result.\n- Record the evidence.\n" }],
}));

async function capture(page: Page, testInfo: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, animations: "disabled" });
  await testInfo.attach(name, { path, contentType: "image/png" });
}

async function expectWithinWidth(page: Page, control: Locator) {
  await expect(control).toBeVisible();
  const width = page.viewportSize()!.width;
  await expect.poll(async () => {
    const bounds = await control.boundingBox();
    return bounds !== null && bounds.x >= 0 && bounds.x + bounds.width <= width;
  }).toBe(true);
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() =>
    Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) <= innerWidth,
  )).toBe(true);
}

test("discovers deployment templates and creates a workspace copy while keeping market context", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  page.setDefaultTimeout(15_000);
  page.on("pageerror", (error) => console.error(`[browser pageerror] ${error.stack ?? error.message}`));
  const api = new TestApiClient();
  const slug = `e2e-skill-market-${WORKER}-${RUN_ID}`;
  await api.login(`${slug}@multica.ai`, "E2E Skill Market User");
  const workspace = await api.ensureWorkspace("E2E Skill Market", slug);
  // ensureWorkspace may reuse the first workspace; never mutate or delete it.
  expect(workspace.slug).toBe(slug);
  expect(workspace.id).toMatch(UUID);

  try {
    const token = api.getToken();
    if (!token) throw new Error("E2E login did not return a token");
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
      document.cookie = "multica-locale=en; path=/; SameSite=Lax";
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, token);

    expect(await api.requestJSON("/api/skills")).toEqual([]);
    const realCatalog = await api.requestJSON<{ templates: TemplateSnapshot[] }>("/api/skills/templates");
    expect(realCatalog.templates.length).toBeGreaterThan(0);
    await page.route("**/api/skills/templates", async (route) => {
      if (route.request().method() !== "GET") return route.continue();
      await route.fulfill({ json: { templates: [...DEPLOYMENT_TEMPLATES, ...realCatalog.templates] } });
    });

    await page.setViewportSize(WIDE);
    await page.goto(`/${slug}/skills`);
    const market = page.getByRole("tab", { name: "Skill templates", exact: true });
    const workspaceTab = page.getByRole("tab", { name: "Workspace skills", exact: true });
    const search = page.getByRole("textbox", { name: "Search templates", exact: true });
    const deployment = page.getByRole("tab", { name: "Deployment-provided", exact: true });
    const template = DEPLOYMENT_TEMPLATES[0]!;
    const preview = page.getByRole("button", { name: `Preview ${template.name}`, exact: true });
    await expect(market).toHaveAttribute("aria-selected", "true");
    await expect(search).toBeVisible();
    await expect(preview).toBeVisible();
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "wide-empty-market");

    // An explicit choice overrides the empty-workspace default after reload.
    await market.focus();
    await page.keyboard.press("ArrowLeft");
    await page.keyboard.press("Enter");
    await expect(workspaceTab).toHaveAttribute("aria-selected", "true");
    await page.reload();
    await expect(workspaceTab).toHaveAttribute("aria-selected", "true");
    await workspaceTab.focus();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("Enter");
    await expect(market).toHaveAttribute("aria-selected", "true");

    await search.focus();
    await page.keyboard.press("Tab");
    await expect(page.getByRole("tab", { name: "All", exact: true })).toBeFocused();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("Enter");
    await expect(deployment).toHaveAttribute("aria-selected", "true");
    await search.fill("release");
    const categories = page.getByRole("group", { name: "Categories", exact: true });
    const quality = categories.getByRole("button", { name: "Testing & quality", exact: true });
    await quality.click();
    await expect(preview).toBeVisible();
    await expect(page.getByRole("button", { name: "Preview e2e-release-operations", exact: true })).toBeHidden();
    await page.getByRole("tab", { name: "Platform built-ins", exact: true }).click();
    await expect(preview).toBeHidden();
    await deployment.click();
    await expect(preview).toBeVisible();

    // Opening by keyboard selects this exact source and restores the opener.
    await preview.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("heading", { name: template.name, exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Use this template", exact: true })).toBeEnabled();
    await capture(page, testInfo, "wide-named-preview");
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(preview).toBeFocused();
    await page.keyboard.press("Enter");
    await dialog.getByRole("button", { name: "Use this template", exact: true }).click();
    const name = dialog.getByLabel("Name", { exact: true });
    await expect(name).toBeFocused();
    await expect(name).toHaveValue(`${template.name}-copy`);
    const copyName = `release-check-${RUN_ID}`;
    await name.fill(copyName);
    await dialog.getByRole("button", { name: "Create skill", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(preview).toBeFocused();
    await expect(page).toHaveURL(new RegExp(`/${slug}/skills$`));
    await expect(market).toHaveAttribute("aria-selected", "true");
    await expect(deployment).toHaveAttribute("aria-selected", "true");
    await expect(quality).toHaveAttribute("aria-pressed", "true");
    await expect(search).toHaveValue("release");

    const saved = await api.requestJSON<SkillSnapshot[]>("/api/skills");
    expect(saved).toHaveLength(1);
    const copy = await api.requestJSON<SkillSnapshot>(`/api/skills/${saved[0]!.id}`);
    expect(copy.id).toMatch(UUID);
    expect(copy.workspace_id).toBe(workspace.id);
    expect(copy.name).toBe(copyName);
    expect(copy.config).toMatchObject({
      template_source: { name: template.name, version: template.version },
      presentation: { category: template.category, icon: template.icon },
    });
    expect(copy.files.map(({ path, content }) => ({ path, content }))).toEqual(template.files);
    const card = page.getByRole("article").filter({ has: preview });
    const copyLink = card.getByRole("link", { name: "View skill", exact: true });
    await expect(copyLink).toHaveAttribute("href", `/${slug}/skills/${copy.id}`);
    await expect(card).toContainText("1 related skill in this workspace");
    await capture(page, testInfo, "wide-market-created-copy");

    await workspaceTab.click();
    const fromTemplate = page.getByRole("button", { name: "From template", exact: true });
    await expect(fromTemplate).toBeVisible();
    await expect(preview).toHaveCount(0);
    await expect(page.getByRole("region", { name: "Deployment-provided", exact: true })).toHaveCount(0);
    await expect(workspaceTab).toContainText("1");
    await expect(page.locator("header").getByText("1", { exact: true })).toHaveCount(0);
    await expect(page.getByText(copyName, { exact: true })).toBeVisible();
    await capture(page, testInfo, "wide-populated-workspace");
    await page.evaluate(() => document.documentElement.classList.add("dark"));
    await capture(page, testInfo, "wide-workspace-dark");
    await page.evaluate(() => document.documentElement.classList.remove("dark"));
    await page.setViewportSize(NARROW);
    const workspaceSearch = page.getByRole("textbox", { name: "Search workspace skills...", exact: true });
    await expectWithinWidth(page, workspaceSearch);
    await expectWithinWidth(page, fromTemplate);
    await workspaceSearch.fill(copyName);
    await expect(page.getByText(copyName, { exact: true })).toBeVisible();
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "narrow-workspace-search");
    await workspaceSearch.clear();
    await fromTemplate.focus();
    await page.keyboard.press("Enter");
    await expect(market).toBeFocused();
    await expect(deployment).toHaveAttribute("aria-selected", "true");
    await expect(search).toHaveValue("release");
    await workspaceTab.click();
    await page.reload();
    await expect(workspaceTab).toHaveAttribute("aria-selected", "true");
    await expect(preview).toHaveCount(0);
    await fromTemplate.click();
    await expect(deployment).toHaveAttribute("aria-selected", "true");
    await categories.getByRole("button", { name: "All", exact: true }).click();

    await page.setViewportSize(NARROW);
    await expectWithinWidth(page, search);
    await expectWithinWidth(page, market);
    await expectWithinWidth(page, preview);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "narrow-deployment-market");
    await preview.click();
    await expect(dialog.getByRole("heading", { name: template.name, exact: true })).toBeVisible();
    await expectWithinWidth(page, dialog.getByRole("button", { name: "Use this template", exact: true }));
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "narrow-named-preview");
    await page.keyboard.press("Escape");
    await expect(preview).toBeFocused();
    await copyLink.click();
    await expect(page).toHaveURL(new RegExp(`/${slug}/skills/${copy.id}$`));
    await expect(page.getByText(copyName, { exact: true }).first()).toBeVisible();

    await testInfo.attach("verification", {
      body: Buffer.from(JSON.stringify({
        workspaceId: workspace.id,
        workspaceSlug: slug,
        copiedSkillId: copy.id,
        templateName: template.name,
        intercepted: ["GET /api/skills/templates"],
        realApi: ["authentication", "workspace setup", "skill creation", "copy metadata", "workspace cleanup"],
        presentations: [WIDE, NARROW],
      }, null, 2)),
      contentType: "application/json",
    });
  } finally {
    await api.cleanup();
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
