import { test, expect, type Locator, type Page, type TestInfo } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { TestApiClient } from "./fixtures";
import enSkills from "../packages/views/locales/en/skills.json" with { type: "json" };

// This is the real catalog → draft → create path. The source catalog and the
// create request are never intercepted. Parsing edge cases belong to the core
// template-draft suite; this spec checks the saved result across UI/API layers.

const WORKER =
  process.env.TEST_PARALLEL_INDEX ?? process.env.TEST_WORKER_INDEX ?? "0";
const RUN_ID =
  process.env.E2E_RUN_ID ??
  `${Date.now().toString(36)}-${process.pid.toString(36)}`;
// This embedded template includes a supporting file, so preservation is tested
// with real content rather than an empty files array.
const TEMPLATE_NAME = "multica-agent-evaluation";
const TEMPLATE_HEADING = "智能体评测";

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
  const textStyles = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = 1;
    const context = canvas.getContext("2d", { willReadFrequently: true });
    const rgba = (color: string) => {
      if (!context) return null;
      context.clearRect(0, 0, 1, 1);
      context.fillStyle = color;
      context.fillRect(0, 0, 1, 1);
      return Array.from(context.getImageData(0, 0, 1, 1).data);
    };
    const entry = Array.from(document.querySelectorAll('[role="tabpanel"]'))
      .find((element) => element.getClientRects().length > 0);
    const popup = document.querySelector('[role="dialog"]');
    const related = Array.from(popup?.querySelectorAll('ul > li > a[href*="/skills/"]') ?? []);
    const samples = new Set([
      ...Array.from(entry?.querySelectorAll("h2, p, button") ?? []),
      ...Array.from(popup?.querySelectorAll('[data-slot="dialog-title"], button[aria-pressed="true"] > span > span') ?? []),
      ...related,
      ...related.map((link) => link.closest("ul")?.previousElementSibling),
    ]);
    return {
      theme: document.documentElement.className,
      viewport: { width: innerWidth, height: innerHeight },
      samples: [...samples].filter((element): element is Element => !!element && element.getClientRects().length > 0)
        .map((element) => {
          const style = getComputedStyle(element);
          const backgroundLayers = [];
          for (let ancestor: Element | null = element; ancestor; ancestor = ancestor.parentElement) {
            const ancestorStyle = getComputedStyle(ancestor);
            backgroundLayers.push({ tag: ancestor.tagName, color: ancestorStyle.backgroundColor,
              rgba: rgba(ancestorStyle.backgroundColor), opacity: ancestorStyle.opacity });
          }
          return { text: element.textContent?.trim(), foreground: style.color, foregroundRgba: rgba(style.color), background: style.backgroundColor,
            fontSize: style.fontSize, fontWeight: style.fontWeight, fontFamily: style.fontFamily, backgroundLayers };
        }),
    };
  });
  await testInfo.attach(`${name}-text-styles`, {
    body: Buffer.from(JSON.stringify(textStyles, null, 2)), contentType: "application/json",
  });
}

async function expectInViewport(page: Page, control: Locator) {
  await expect(control).toBeVisible();
  const viewport = page.viewportSize();
  expect(viewport).not.toBeNull();
  await expect.poll(async () => {
    const bounds = await control.boundingBox();
    return bounds !== null && bounds.x >= 0 && bounds.y >= 0
      && bounds.x + bounds.width <= viewport!.width
      && bounds.y + bounds.height <= viewport!.height;
  }).toBe(true);
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() =>
    Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) <= window.innerWidth,
  )).toBe(true);
  for (const dialog of await page.getByRole("dialog").or(page.getByRole("alertdialog")).all()) {
    await expectInViewport(page, dialog);
    await expect.poll(() => dialog.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
  }
}

test("creates an edited independent skill after previewing and cancelling without writes", async ({
  page,
}, testInfo) => {
  // This production-server flow verifies creation plus wide/narrow visual states.
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
  try {
    const token = api.getToken();
    if (!token) throw new Error("E2E login did not return a token");
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", {
      method: "PATCH",
      body: { language: "en" },
    });
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      if (!localStorage.getItem("theme")) localStorage.setItem("theme", "light");
      document.cookie = "multica-locale=en; path=/; SameSite=Lax";
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, token);

    const initialSkills = await api.requestJSON<SkillSnapshot[]>("/api/skills");
    expect(initialSkills).toEqual([]);
    const catalog = await api.requestJSON<{ templates: TemplateSnapshot[] }>("/api/skills/templates");
    expect(catalog.templates).toHaveLength(15);
    const source = catalog.templates.find((item) => item.name === TEMPLATE_NAME);
    expect(source).toBeDefined();
    expect(source!.files.map((file) => file.path)).toContain("references/evaluation-source-map.md");
    const catalogBefore = JSON.stringify(catalog);
    const membersBefore = await api.requestJSON(`/api/workspaces/${workspace.id}/members`);
    const agentsBefore = await api.requestJSON("/api/agents");
    const browserSkillWrites: { method: string; path: string }[] = [];
    page.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (["POST", "PUT", "PATCH", "DELETE"].includes(request.method()) && path.startsWith("/api/skills")) {
        browserSkillWrites.push({ method: request.method(), path });
      }
    });

    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto(`/${slug}/skills`);
    await expect(page.locator("html")).toHaveClass(/\blight\b/);
    const marketTab = page.getByRole("tab", { name: enSkills.market.title, exact: true });
    const workspaceTab = page.getByRole("tab", { name: enSkills.market.workspace, exact: true });
    const marketSearch = page.getByRole("textbox", { name: enSkills.market.search, exact: true });
    const previewTemplate = page.getByRole("button", { name: `Preview ${TEMPLATE_NAME}`, exact: true });
    await expect(marketTab).toHaveAttribute("aria-selected", "true");
    await expect(marketTab).toContainText("15");
    await expect(page.getByRole("tab", { name: enSkills.market.source_builtin, exact: true })).toContainText("15");
    await expect(page.getByRole("tab", { name: enSkills.market.source_deployment, exact: true })).toContainText("0");
    await expect(page.getByText(enSkills.market.deployment_empty, { exact: true })).toBeVisible();
    await expect(previewTemplate).toBeVisible();
    await expect(page.locator("header").getByText("15", { exact: true })).toHaveCount(0);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "desktop-skill-market");
    const newSkill = page.getByRole("button", { name: "New skill", exact: true }).first();
    await newSkill.click();
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
    await expectInViewport(page, dialog.getByRole("button", { name: "Close", exact: true }));
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "small-chooser");
    await dialog.getByRole("button", { name: /^Modify from template/ }).click();
    await expect(dialog.getByRole("heading", { name: enSkills.create.template.preview_label, exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Use this template", exact: true })).toBeVisible();
    // This fixture runs against the embed-only catalog. Both specialists must
    // participate in the same official grouping and purpose search as older skills.
    const builtinTab = dialog.getByRole("tab", { name: /Platform built-in/ });
    const deploymentTab = dialog.getByRole("tab", { name: /Provided by this deployment/ });
    await expect(builtinTab).toHaveAccessibleName(/Platform built-in\s*15/);
    await expect(deploymentTab).toHaveAccessibleName(/Provided by this deployment\s*0/);
    const templateSearch = dialog.getByRole("textbox", { name: enSkills.create.template.search_placeholder });
    await expect(templateSearch).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(builtinTab).toBeFocused();
    await page.keyboard.press("ArrowRight");
    await expect(deploymentTab).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(deploymentTab).toHaveAttribute("aria-selected", "true");
    await expect(dialog.getByText(enSkills.create.template.deployment_empty_hint, { exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: enSkills.create.template.use_template, exact: true })).toBeDisabled();
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "small-template-deployment-empty");
    await page.keyboard.press("ArrowLeft");
    await expect(builtinTab).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(builtinTab).toHaveAttribute("aria-selected", "true");
    await capture(page, testInfo, "small-template-list");
    for (const name of ["multica-experience-validation", "multica-migration-review"] as const) {
      await templateSearch.fill(enSkills.builtin_role_skills[name].description);
      await expect(dialog.getByRole("tab", { name: /Platform built-in\s*1/ })).toBeVisible();
      await expect(dialog.getByRole("button", { name: new RegExp(`^${name}`) }))
        .toContainText(enSkills.builtin_role_skills[name].summary);
    }
    await templateSearch.fill("no-template-matches-this-query");
    await expect(dialog.getByRole("tabpanel", { name: /^Platform built-in\s*0$/ })
      .getByText(enSkills.create.template.no_matches, { exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: enSkills.create.template.use_template, exact: true })).toBeDisabled();
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "small-template-no-matches");
    await page.setViewportSize({ width: 1280, height: 720 });
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "desktop-template-no-matches");
    await page.setViewportSize({ width: 375, height: 667 });
    await dialog.getByRole("button", { name: enSkills.create.template.clear_search, exact: true }).click();
    await expect(templateSearch).toHaveValue("");
    await expect(templateSearch).toBeFocused();
    const templateRow = dialog.getByRole("button", { name: new RegExp(`^${TEMPLATE_NAME}\\b`) });
    const useTemplate = dialog.getByRole("button", { name: "Use this template", exact: true });
    await templateRow.focus();
    await page.keyboard.press("Enter");
    await expect(useTemplate).toBeFocused();
    await dialog.getByRole("button", { name: "Back to templates", exact: true }).click();
    await expect(templateRow).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(useTemplate).toBeFocused();
    await expect(dialog).toContainText(TEMPLATE_HEADING);
    await expect(dialog.getByText(enSkills.builtin_role_skills[TEMPLATE_NAME].description, { exact: true })).toBeVisible();
    await expectInViewport(page, useTemplate);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "small-template-preview");
    await page.setViewportSize({ width: 360, height: 800 });
    await expectInViewport(page, useTemplate);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "narrow-template-preview");
    await page.setViewportSize({ width: 375, height: 667 });
    expect(await api.requestJSON("/api/skills")).toEqual([]);
    expect(browserSkillWrites).toEqual([]);

    // Browsing has no dirty draft and closes directly, without materializing
    // any built-in catalog entries in this empty workspace.
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(newSkill).toBeFocused();
    expect(await api.requestJSON("/api/skills")).toEqual([]);
    expect(browserSkillWrites).toEqual([]);
    await expect(page.getByPlaceholder(enSkills.page.search_placeholder)).toBeHidden();
    await expect(marketTab).toHaveAttribute("aria-selected", "true");
    await expectInViewport(page, marketSearch);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "small-skill-market");
    await page.setViewportSize({ width: 360, height: 800 });
    await expectInViewport(page, marketSearch);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "narrow-skill-market");

    // A workspace-owned skill with the canonical template name must never be
    // used as, or overwritten by, the built-in source. This row is disposable.
    const existing = await api.requestJSON<SkillSnapshot>("/api/skills", {
      method: "POST",
      body: {
        name: TEMPLATE_NAME,
        description: "Workspace custom source to preserve",
        content: "# Workspace custom source\n\nKeep this existing skill unchanged.\n",
        files: [{ path: "references/owned.txt", content: "Workspace-owned file\n" }],
      },
    });
    const existingBefore = await api.requestJSON(`/api/skills/${existing.id}`);

    await page.setViewportSize({ width: 1280, height: 720 });
    await page.reload();
    await expect(page.locator("header").getByText("1", { exact: true })).toHaveCount(0);
    await expect(workspaceTab).toContainText("1");
    await expect(marketTab).toHaveAttribute("aria-selected", "true");
    await previewTemplate.focus();
    await page.keyboard.press("Enter");
    await expect(templateRow).toHaveAttribute("aria-pressed", "true");
    await expect(dialog.getByText(enSkills.builtin_role_skills[TEMPLATE_NAME].description, { exact: true })).toBeVisible();
    await expect(dialog.getByRole("heading", { name: enSkills.create.template.preview_label, exact: true })).toBeVisible();
    await capture(page, testInfo, "desktop-direct-template-preview");
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(previewTemplate).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(dialog).toContainText(TEMPLATE_HEADING);
    await expect(dialog.getByText(enSkills.create.template.related_none, { exact: true })).toBeVisible();
    const finalInstruction = dialog.getByRole("heading", { name: "交付与边界", exact: true });
    await finalInstruction.scrollIntoViewIfNeeded();
    await expectInViewport(page, finalInstruction);
    await dialog.getByText(enSkills.builtin_role_skills[TEMPLATE_NAME].description, { exact: true }).scrollIntoViewIfNeeded();
    await templateRow.hover();
    await expect(templateRow).toHaveAttribute("aria-pressed", "true");
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "desktop-template-preview");
    await dialog.getByRole("button", { name: "Use this template", exact: true }).click();
    const name = dialog.getByLabel("Name", { exact: true });
    const description = dialog.getByLabel("Description", { exact: true });
    const instructions = dialog.getByLabel("Instructions", { exact: true });
    await expect(dialog.getByRole("heading", { name: enSkills.create.template.draft_title, exact: true })).toBeVisible();
    await expect(name).toHaveValue(`${TEMPLATE_NAME}-copy`);
    await expect(name).toBeFocused();
    await expect(description).toHaveValue(enSkills.builtin_role_skills[TEMPLATE_NAME].description);
    const originalBody = await instructions.inputValue();
    expect(originalBody).toBe(source!.content.slice(source!.content.indexOf("\n---", 4) + 5));
    expect(originalBody).toContain(TEMPLATE_HEADING);
    expect(originalBody).not.toContain("Workspace custom source");
    expect(originalBody).not.toMatch(/^---/);
    await expect(dialog.getByRole("group", { name: enSkills.create.template.included_files })
      .getByRole("button", { name: "references/evaluation-source-map.md", exact: true })).toBeVisible();

    const copyName = `e2e-evaluation-copy-${RUN_ID}`;
    const copyDescription = "Evaluate changes while preserving the original template.";
    const editedBody = `${originalBody}\n\n## QA 自定义检查\n\n保留原模板，检查自己的副本。\n\n---\n\nKeep this body separator.\n`;
    await name.fill(copyName);
    await description.fill(copyDescription);
    await instructions.fill(editedBody);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "desktop-template-edit");
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    const discard = page.getByRole("alertdialog", { name: enSkills.create.template.discard_title });
    await expect(discard).toBeVisible();
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "desktop-template-discard");
    await page.setViewportSize({ width: 375, height: 667 });
    await expectInViewport(page, discard.getByRole("button", { name: enSkills.create.template.keep_editing, exact: true }));
    await expectInViewport(page, discard.getByRole("button", { name: enSkills.create.template.discard, exact: true }));
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "small-template-discard");
    await page.setViewportSize({ width: 360, height: 800 });
    await expectInViewport(page, discard.getByRole("button", { name: enSkills.create.template.keep_editing, exact: true }));
    await expectInViewport(page, discard.getByRole("button", { name: enSkills.create.template.discard, exact: true }));
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "narrow-template-discard");
    await page.setViewportSize({ width: 375, height: 667 });
    await discard.getByRole("button", { name: enSkills.create.template.keep_editing, exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect(discard).toHaveCount(0);
    await expect(dialog.getByRole("button", { name: "Close", exact: true })).toBeFocused();
    await expect(name).toHaveValue(copyName);
    await expect(description).toHaveValue(copyDescription);
    await expect(instructions).toHaveValue(editedBody);
    await expectInViewport(page, dialog.getByRole("button", { name: "Create skill", exact: true }));
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "small-template-edit");
    expect(await api.requestJSON("/api/skills")).toHaveLength(1);
    expect(browserSkillWrites).toEqual([]);

    const createResponsePromise = page.waitForResponse((response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/skills",
    );
    await dialog.getByRole("button", { name: "Create skill", exact: true }).click();
    const createResponse = await createResponsePromise;
    expect(createResponse.status()).toBe(201);
    const created: SkillSnapshot = await createResponse.json();
    await expect(dialog).toHaveCount(0);
    await expect(page).toHaveURL(new RegExp(`/${slug}/skills$`));
    await expect(marketTab).toHaveAttribute("aria-selected", "true");
    const createdLink = page.getByRole("article").filter({ has: previewTemplate })
      .getByRole("link", { name: enSkills.market.open_created, exact: true });
    await expect(createdLink).toHaveAttribute("href", `/${slug}/skills/${created.id}`);
    expect(browserSkillWrites).toEqual([{ method: "POST", path: "/api/skills" }]);
    const saved = await api.requestJSON<SkillSnapshot>(`/api/skills/${created.id}`);
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
    expect(saved.content).toContain("\nmetadata:\n  category: quality\n  icon: bot\n");
    expect(saved.content.slice(saved.content.indexOf("\n---", 4) + 5)).toBe(editedBody);
    expect(saved.config).toMatchObject({
      template_source: { name: TEMPLATE_NAME, version: source!.version },
    });
    expect(saved.config).not.toHaveProperty("origin");
    expect(saved.files.map(({ path, content }) => ({ path, content }))).toEqual(source!.files);
    expect(await api.requestJSON("/api/skills")).toHaveLength(2);
    expect(await api.requestJSON(`/api/skills/${existing.id}`)).toEqual(existingBefore);
    expect(JSON.stringify(await api.requestJSON("/api/skills/templates"))).toBe(catalogBefore);
    expect(await api.requestJSON(`/api/workspaces/${workspace.id}/members`)).toEqual(membersBefore);
    expect(await api.requestJSON("/api/agents")).toEqual(agentsBefore);
    await createdLink.click();
    await expect(page).toHaveURL(new RegExp(`/${slug}/skills/${created.id}$`), { timeout: 30_000 });
    await capture(page, testInfo, "small-created-detail");
    await page.setViewportSize({ width: 1280, height: 720 });
    await capture(page, testInfo, "desktop-created-detail");

    // A second independent copy must appear by name, while the manual canonical
    // name above is still excluded. Opening either copy must never create a skill.
    const related = await api.requestJSON<SkillSnapshot>("/api/skills", {
      method: "POST",
      body: {
        name: `e2e-team-evaluation-${RUN_ID}`,
        description: "Team-specific evaluation guidance",
        content: "# Team evaluation\n\nKeep this independent copy.\n",
        config: { template_source: { name: TEMPLATE_NAME, version: source!.version } },
      },
    });
    await page.evaluate(() => localStorage.setItem("theme", "dark"));
    await page.goto(`/${slug}/skills`);
    await expect(page.locator("html")).toHaveClass(/\bdark\b/);
    await expect(page.locator("header").getByText("3", { exact: true })).toHaveCount(0);
    await expect(workspaceTab).toContainText("3");
    await expect(marketTab).toContainText("15");
    await workspaceTab.click();
    await expect(page.getByRole("region", { name: enSkills.market.source_deployment, exact: true })).toHaveCount(0);
    await expect(page.getByText(enSkills.market.deployment_empty, { exact: true })).toHaveCount(0);
    await expect(previewTemplate).toHaveCount(0);
    const fromTemplate = page.getByRole("button", { name: enSkills.market.from_template, exact: true });
    await expectInViewport(page, fromTemplate);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "dark-workspace-skills");
    await fromTemplate.click();
    await expect(marketTab).toBeFocused();
    await previewTemplate.click();
    await expect(dialog.getByRole("link", { name: copyName, exact: true })).toBeVisible();
    await expect(dialog.getByRole("link", { name: related.name, exact: true })).toBeVisible();
    await expect(dialog.getByText(enSkills.create.template.related_count_other.replace("{{count}}", "2"), { exact: true })).toBeVisible();
    await expect(dialog.locator(`a[href^="/${slug}/skills/"]`)).toHaveCount(2);
    await expect(dialog.locator(`a[href$="/skills/${existing.id}"]`)).toHaveCount(0);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "dark-related-skills-desktop");
    await page.setViewportSize({ width: 375, height: 667 });
    await expectInViewport(page, useTemplate);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "dark-related-skills-small");
    await useTemplate.click();
    const unsavedName = `unsaved-evaluation-${RUN_ID}`;
    await name.fill(unsavedName);
    await dialog.getByRole("button", { name: enSkills.create.template.choose_another, exact: true }).click();
    const relatedLink = dialog.getByRole("link", { name: related.name, exact: true });
    await relatedLink.focus();
    await page.keyboard.press("Enter");
    await expect(discard).toBeVisible();
    await expect(page).toHaveURL(`/${slug}/skills`);
    await expectNoHorizontalOverflow(page);
    await capture(page, testInfo, "dark-related-navigation-discard");
    await discard.getByRole("button", { name: enSkills.create.template.keep_editing, exact: true }).click();
    await expect(discard).toHaveCount(0);
    await expect(relatedLink).toBeFocused();
    await useTemplate.click();
    await expect(name).toHaveValue(unsavedName);
    expect(browserSkillWrites).toEqual([{ method: "POST", path: "/api/skills" }]);
    await dialog.getByRole("button", { name: enSkills.create.template.choose_another, exact: true }).click();
    await relatedLink.focus();
    await page.keyboard.press("Enter");
    await discard.getByRole("button", { name: enSkills.create.template.discard, exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/${slug}/skills/${related.id}$`));
    await expect(dialog).toHaveCount(0);
    expect(browserSkillWrites).toEqual([{ method: "POST", path: "/api/skills" }]);
    await expect(page.getByText(enSkills.create.template.created, { exact: true })).toHaveCount(0);
    expect(await api.requestJSON<SkillSnapshot[]>("/api/skills")).toHaveLength(3);
    expect(await api.requestJSON(`/api/skills/${saved.id}`)).toEqual(saved);
    expect(await api.requestJSON(`/api/skills/${existing.id}`)).toEqual(existingBefore);
    expect(JSON.stringify(await api.requestJSON("/api/skills/templates"))).toBe(catalogBefore);
    expect(await api.requestJSON(`/api/workspaces/${workspace.id}/members`)).toEqual(membersBefore);
    expect(await api.requestJSON("/api/agents")).toEqual(agentsBefore);
    await writeFile(testInfo.outputPath("verification.json"), JSON.stringify({
      status: "passed",
      workspaceSlug: slug,
      createdSkillId: saved.id,
      createdSkillName: saved.name,
      sourceTemplate: source!.name,
      sourceVersion: source!.version,
      preservedWorkspaceSkillId: existing.id,
      templateCounts: { total: 15, builtin: 15, deployment: 0 },
      marketCreationRetainedBrowsingContext: true,
      createdSkillOpenedThroughCardLink: true,
      previewSkillCount: 0,
      cancelledSkillCount: 0,
      createdSkillCount: 1,
      finalWorkspaceSkillCount: 3,
      relatedSkillIds: [saved.id, related.id],
      browserCreateRequests: browserSkillWrites.length,
      browserSkillWrites,
      supportingFilePaths: source!.files.map((file) => file.path),
      discardedNavigationPreservedDraftAndFocus: true,
      metadataAndFrontmatterMatch: true,
      bodyAndSupportingFilesMatch: true,
      sourceCatalogUnchanged: true,
      existingWorkspaceSkillUnchanged: true,
      membersAndAgentsUnchanged: true,
    }, null, 2));
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
    expect((await api.getWorkspaces()).some((item) => item.id === workspace.id)).toBe(false);
  }
});
