import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { waitForPageText } from "./helpers";

// The role-template creation flow, end to end through the real app.
//
// Auth and the workspace go through the real backend, as every spec does. The
// template list, the runtime list and the create call are intercepted at the
// network boundary: the flow needs an ONLINE runtime to be submittable, and a
// real one would mean a live daemon. Intercepting the create is also how the test
// asserts the thing that matters most — the exact body the flow sends, which is
// what makes the created agent's provenance trustworthy.

const E2E_WORKER =
  process.env.TEST_PARALLEL_INDEX ?? process.env.TEST_WORKER_INDEX ?? "0";
const E2E_RUN_ID =
  process.env.E2E_RUN_ID ?? `${Date.now().toString(36)}-${process.pid.toString(36)}`;
const EMAIL = `e2e-role-template-${E2E_WORKER}-${E2E_RUN_ID}@multica.ai`;
const NAME = "E2E Role Template User";
const workspacesToDelete: Array<{ api: TestApiClient; id: string }> = [];

const RUNTIME_ID = "33333333-3333-4333-8333-333333333333";
const RUNTIME_OWNER_ID = "55555555-5555-4555-8555-555555555555";
const CREATED_AGENT_ID = "44444444-4444-4444-8444-444444444444";

const TEMPLATES = [
  {
    key: "product-analyst",
    version: 1,
    name: "Product Analyst",
    title: "Product Analyst",
    description: "Turns a vague request into a decidable one.",
    autonomy_level: "observer",
    avatar_emoji: "🔍",
    max_concurrent_tasks: 3,
    skill_names: ["multica-requirement-clarification"],
    instructions:
      "# Product Analyst\n\n## Responsibilities\n\nRestate the request as an outcome.",
  },
  {
    key: "code-reviewer",
    version: 1,
    name: "Code Reviewer",
    title: "Code Reviewer",
    description: "Reads the diff and reports located findings.",
    autonomy_level: "observer",
    avatar_emoji: "🔬",
    max_concurrent_tasks: 3,
    skill_names: ["multica-code-review"],
    instructions:
      "# 代码审查员\n\n## 职责\n\n阅读 diff，给出包含文件位置和行号的发现。",
  },
  {
    key: "diagnostician",
    version: 1,
    name: "Diagnostician",
    title: "Diagnostician",
    description: "Turns an observed failure into an evidence-backed root cause.",
    autonomy_level: "contributor",
    avatar_emoji: "🩺",
    max_concurrent_tasks: 1,
    skill_names: ["multica-debugging"],
    instructions:
      "# 诊断工程师\n\n## 职责\n\n复现并最小化失败，验证假设，不提交正式修复。",
  },
];

async function login(page: Page): Promise<string> {
  const api = new TestApiClient();
  await api.login(EMAIL, NAME);
  const workspace = await api.ensureWorkspace(
    `E2E Role Template WS ${E2E_WORKER}`,
    `e2e-role-tpl-${E2E_WORKER}-${E2E_RUN_ID}`,
  );
  workspacesToDelete.push({ api, id: workspace.id });
  await api.markUserOnboarded();
  const token = api.getToken();
  if (!token) throw new Error("login did not return a token");
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
  return workspace.slug;
}

/** Mocks the template catalog, one online runtime, and the create call. Returns a
 *  getter for the create body the flow actually sent. */
async function mockTemplateApis(page: Page) {
  const captured: { body?: Record<string, unknown> } = {};

  await page.route("**/api/agents/templates**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ templates: TEMPLATES }),
    }),
  );

  await page.route("**/api/runtimes?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        {
          id: RUNTIME_ID,
          workspace_id: "ws",
          name: "E2E Runtime",
          hostname: "e2e-host",
          provider: "claude",
          runtime_mode: "local",
          status: "online",
          visibility: "public",
          // Must have an owner: isRuntimeUsableForUser rejects an ownerless
          // runtime for everyone, public included, because the server needs an
          // owner to mint the agent's task token (MUL-3292). With null here the
          // configure step never becomes submittable and the footer stays
          // disabled — which is what this fixture did until the spec was first
          // actually executed.
          owner_id: RUNTIME_OWNER_ID,
          created_at: "2026-09-01T00:00:00Z",
          updated_at: "2026-09-01T00:00:00Z",
        },
      ]),
    }),
  );

  await page.route("**/api/agents/from-template", async (route) => {
    captured.body = route.request().postDataJSON();
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({
        id: CREATED_AGENT_ID,
        workspace_id: "ws",
        runtime_id: RUNTIME_ID,
        name: "Code Reviewer",
        description: TEMPLATES[1].description,
        instructions: TEMPLATES[1].instructions,
        avatar_url: null,
        runtime_mode: "local",
        runtime_config: {},
        custom_args: [],
        visibility: "private",
        permission_mode: "private",
        invocation_targets: [],
        status: "idle",
        max_concurrent_tasks: 3,
        model: "",
        owner_id: "user",
        skills: [],
        template_key: "code-reviewer",
        template_version: 1,
        autonomy_level: "observer",
        created_at: "2026-09-01T00:00:00Z",
        updated_at: "2026-09-01T00:00:00Z",
        archived_at: null,
        archived_by: null,
      }),
    });
  });

  return () => captured.body;
}

test.describe("agent role templates", () => {
  test.afterEach(async () => {
    for (const { api, id } of workspacesToDelete.splice(0)) {
      await api.deleteFeatureWorkspace(id);
    }
  });

  test("opens existing members without mutation and restores template browsing", async ({ page }, info) => {
    const slug = await login(page);
    const createdBody = await mockTemplateApis(page);
    const saved = (id: string, name: string, templateKey: string | null, archivedAt: string | null = null) => ({
      id, name, workspace_id: "ws", runtime_id: RUNTIME_ID,
      description: "Saved description remains unchanged.", instructions: "Saved instructions remain unchanged.",
      avatar_url: null, runtime_mode: "local", runtime_config: {}, custom_args: [],
      visibility: "private", permission_mode: "private", invocation_targets: [],
      status: "idle", max_concurrent_tasks: 1, model: "", owner_id: RUNTIME_OWNER_ID,
      skills: [], template_key: templateKey, template_version: 1, autonomy_level: "observer",
      created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
      archived_at: archivedAt, archived_by: null,
    });
    const agents = [
      saved("66666666-6666-4666-8666-666666666661", "Payments analyst", "product-analyst"),
      saved("66666666-6666-4666-8666-666666666662", "Checkout analyst", "product-analyst"),
      saved("66666666-6666-4666-8666-666666666663", "Release reviewer", "code-reviewer"),
      saved("66666666-6666-4666-8666-666666666664", "Retired analyst", "product-analyst", "2026-09-24T00:00:00Z"),
      saved("66666666-6666-4666-8666-666666666665", "Diagnostician", null),
    ];
    await page.route("**/api/agents?**", (route) => route.fulfill({ json: agents }));
    for (const agent of agents) {
      await page.route(`**/api/agents/${agent.id}`, (route) => route.fulfill({ json: agent }));
    }
    const mutationRequests: string[] = [];
    const pageErrors: string[] = [];
    page.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (["POST", "PUT", "PATCH", "DELETE"].includes(request.method()) && /^\/api\/(agents(?:\/|$)|squads\/[^/]+\/members$)/.test(path)) {
        mutationRequests.push(`${request.method()} ${path}`);
      }
    });
    page.on("pageerror", (error) => pageErrors.push(error.message));

    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${slug}/agents/new/template`);
    const analyst = page.getByRole("listitem", { name: "Product Analyst", exact: true });
    await expect(analyst.getByText("2 existing agents", { exact: true })).toBeVisible();
    await expect(analyst.getByRole("link", { name: "Open Checkout analyst", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Open Retired analyst", exact: true })).toHaveCount(0);
    await expect(page.getByRole("listitem", { name: "Code Reviewer", exact: true }).getByText("1 existing agent", { exact: true })).toBeVisible();
    const diagnostician = page.getByRole("listitem", { name: "Diagnostician", exact: true });
    await expect(diagnostician.getByText("No agents created from this template.", { exact: true })).toBeVisible();
    await expect(analyst.getByRole("link", { name: "Create another", exact: true })).toHaveAttribute("href", `/${slug}/agents/new/template?template=product-analyst`);

    for (const width of [1440, 768, 390]) {
      await page.setViewportSize({ width, height: 1000 });
      await page.screenshot({ path: info.outputPath(`role-templates-${width}.png`), animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
    await analyst.getByRole("link", { name: "Open Payments analyst", exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/agents/${agents[0].id}$`));
    await expect(page.getByRole("heading", { name: "Payments analyst", exact: true })).toBeVisible();
    await page.goBack();
    await expect(analyst.getByText("2 existing agents", { exact: true })).toBeVisible();

    await page.setViewportSize({ width: 390, height: 460 });
    const gallery = page.locator('[data-tab-scroll-root="agent-role-templates"]');
    const createLink = diagnostician.getByRole("link", { name: "Create agent", exact: true });
    await createLink.scrollIntoViewIfNeeded();
    const scrollTop = await gallery.evaluate((element) => element.scrollTop);
    expect(scrollTop).toBeGreaterThan(0);
    for (const nativeBack of [true, false]) {
      await createLink.click();
      await waitForPageText(page, "Role instructions");
      if (nativeBack) await page.goBack();
      else await page.getByRole("button", { name: "Go back", exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`/${slug}/agents/new/template$`));
      await expect.poll(() => gallery.evaluate((element) => element.scrollTop)).toBe(scrollTop);
    }
    expect(createdBody()).toBeUndefined();
    expect(mutationRequests).toEqual([]);
    expect(pageErrors).toEqual([]);
  });

  test("offers a template starting point and creates from a role", async ({
    page,
  }) => {
    const slug = await login(page);
    const createdBody = await mockTemplateApis(page);

    await page.goto(`/${slug}/agents/new`);
    // Three starting points now: blank, template, AI.
    await waitForPageText(page, "Use a template");

    await page.getByText("Use a template").first().click();

    // Step one is the role grid, with the localized autonomy label rather than
    // the raw wire value.
    await waitForPageText(page, "Start from a role template");
    await expect(page.getByText("Product Analyst").first()).toBeVisible();
    await expect(page.getByText("Observer").first()).toBeVisible();

    await page.getByRole("listitem", { name: "Code Reviewer", exact: true }).getByRole("link", { name: "Create agent", exact: true }).click();

    // Step two shows the instructions that will be copied, read-only, plus the
    // skill the role brings.
    await waitForPageText(page, "Role instructions");
    await expect(
      page.getByText("阅读 diff，给出包含文件位置和行号的发现。"),
    ).toBeVisible();
    await expect(page.getByText("multica-code-review")).toBeVisible();
    // The role's prompt is not an editable field on this step.
    await expect(page.locator("#agent-create-instructions")).toHaveCount(0);

    // The footer's commit label ("Create & open agent" for a non-squad flow).
    await page.getByRole("button", { name: /Create & open agent/ }).click();

    await expect
      .poll(() => createdBody(), { timeout: 10_000 })
      .toMatchObject({
        template_key: "code-reviewer",
        runtime_id: RUNTIME_ID,
        name: "Code Reviewer",
        permission_mode: "private",
      });

    // Instructions, skills and autonomy are the backend's to apply — sending them
    // from the client would let a caller claim a template it did not use.
    const body = createdBody() ?? {};
    expect(body).not.toHaveProperty("instructions");
    expect(body).not.toHaveProperty("skill_ids");
    expect(body).not.toHaveProperty("autonomy_level");

    await expect(page).toHaveURL(new RegExp(`/${slug}/agents/${CREATED_AGENT_ID}`));
  });

  test("shows the diagnostician evidence contract and server-owned defaults", async ({
    page,
  }) => {
    const slug = await login(page);
    const createdBody = await mockTemplateApis(page);

    await page.goto(`/${slug}/agents/new`);
    await waitForPageText(page, "Use a template");
    await page.getByText("Use a template").first().click();
    await waitForPageText(page, "Start from a role template");

    await expect(page.getByRole("listitem", { name: "Diagnostician", exact: true })).toBeVisible();
    await page.getByRole("listitem", { name: "Diagnostician", exact: true }).getByRole("link", { name: "Create agent", exact: true }).click();
    await waitForPageText(page, "Role instructions");
    await expect(page.getByText("复现并最小化失败，验证假设，不提交正式修复。")).toBeVisible();
    await expect(page.getByText("multica-debugging")).toBeVisible();
    await expect(page.getByText("Contributor")).toBeVisible();
    await page.getByRole("button", { name: /Create & open agent/ }).click();

    await expect
      .poll(() => createdBody(), { timeout: 10_000 })
      .toMatchObject({
        template_key: "diagnostician",
        runtime_id: RUNTIME_ID,
        name: "Diagnostician",
        permission_mode: "private",
      });

    const body = createdBody() ?? {};
    expect(body).not.toHaveProperty("instructions");
    expect(body).not.toHaveProperty("skill_ids");
    expect(body).not.toHaveProperty("autonomy_level");
    await expect(page).toHaveURL(new RegExp(`/${slug}/agents/${CREATED_AGENT_ID}`));
  });
});
