import { randomUUID } from "node:crypto";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("capability market restores type and draft through history while resource management stays separate", async ({ page }, info) => {
  test.setTimeout(120_000);
  const api = new TestApiClient();
  const slug = `capability-market-${randomUUID().slice(0, 12)}`;
  await api.login(`${slug}@multica.ai`, "Capability market tester");
  const workspace = await api.ensureWorkspace("Capability market acceptance", slug);
  expect(workspace.slug).toBe(slug);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    await page.addInitScript((token) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica:chat:isOpen", "false");
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
      document.cookie = "multica-locale=en; path=/; SameSite=Lax";
    }, api.getToken());
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${slug}/skills`);
    await expect(page.getByText("No skills yet", { exact: true })).toBeVisible();
    for (const [label, path] of [["Skills", "skills"], ["MCP", "mcp"], ["Runtimes", "runtimes"], ["Capability market", "marketplace"]]) {
      await expect(page.getByRole("link", { name: label, exact: true })).toHaveAttribute("href", `/${slug}/${path}`);
    }
    await page.getByRole("link", { name: "Capability market", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Capability market", exact: true })).toBeVisible();
    const types = page.getByRole("tablist", { name: "Capability type", exact: true });
    const mcp = types.getByRole("tab", { name: "MCP", exact: true });
    const skills = types.getByRole("tab", { name: "Skills", exact: true });
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    await expect(page.getByTestId("mcp-market")).toBeVisible();
    const navigationScreenshot = info.outputPath("capability-market-navigation.png");
    await page.screenshot({ path: navigationScreenshot, animations: "disabled" });
    await info.attach("capability-market-navigation", { path: navigationScreenshot, contentType: "image/png" });
    await mcp.focus();
    await page.keyboard.press("ArrowRight");
    await expect(skills).toBeFocused();
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(`/${slug}/marketplace?type=skills`);
    await expect(skills).toHaveAttribute("aria-selected", "true");
    await page.goBack();
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    await page.goForward();
    await expect(skills).toHaveAttribute("aria-selected", "true");
    await page.reload();
    await expect(skills).toHaveAttribute("aria-selected", "true");

    await page.goto(`/${slug}/marketplace?type=invalid&context=keep#catalog`);
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    await skills.click();
    await expect(page).toHaveURL(`/${slug}/marketplace?type=skills&context=keep#catalog`);
    await mcp.click();
    await skills.click();
    const preview = page.getByRole("button", { name: "Preview multica-code-review", exact: true });
    await preview.click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "Use this template", exact: true }).click();
    const name = dialog.getByRole("textbox", { name: "Name", exact: true });
    const draftName = `unsaved-${slug}`;
    await name.fill(draftName);
    await page.goBack();
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    await expect(dialog).toHaveCount(0);
    await expect(page.getByTestId("mcp-market")).toBeVisible();
    await page.goForward();
    await expect(name).toHaveValue(draftName);
    const screenshot = info.outputPath("market-restored-skill-draft.png");
    await page.screenshot({ path: screenshot, animations: "disabled" });
    await info.attach("market-restored-skill-draft", { path: screenshot, contentType: "image/png" });
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    await expect(page.getByRole("alertdialog")).toContainText("Your changes to this copy have not been saved.");
    await page.getByRole("button", { name: "Discard changes", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    expect(await api.requestJSON("/api/skills")).toEqual([]);

    await page.getByRole("link", { name: "Skills", exact: true }).click();
    await expect(page).toHaveURL(`/${slug}/skills`);
    await expect(page.getByRole("link", { name: "From template", exact: true })).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Search templates", exact: true })).toHaveCount(0);
    await page.getByRole("link", { name: "MCP", exact: true }).click();
    await expect(page).toHaveURL(`/${slug}/mcp`);
    await expect(page.getByRole("button", { name: "Add custom server", exact: true })).toBeVisible();
    await expect(page.getByTestId("mcp-market")).toHaveCount(0);
    await page.keyboard.press("ControlOrMeta+k");
    const commands = page.getByRole("dialog", { name: "Search", exact: true });
    await commands.getByRole("combobox").fill("capabilities");
    await commands.getByRole("option", { name: "Capability market", exact: true }).click();
    await expect(commands).toHaveCount(0);
    await expect(page).toHaveURL(`/${slug}/marketplace`);
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    await page.goBack();
    await expect(page).toHaveURL(`/${slug}/mcp`);
    await page.getByRole("button", { name: "Browse capability market", exact: true }).click();
    await expect(page).toHaveURL(`/${slug}/marketplace?type=mcp`);
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    expect(errors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("capability market isolates workspace resources, preferences and transient drafts", async ({ page }, info) => {
  test.setTimeout(120_000);
  page.setDefaultTimeout(15_000);
  const api = new TestApiClient();
  const run = randomUUID().slice(0, 12);
  await api.login(`market-isolation-${run}@multica.ai`, "Market isolation tester");
  const first = await api.ensureWorkspace(`Market first ${run}`, `market-first-${run}`);
  const second = await api.requestJSON<{ id: string; name: string; slug: string }>("/api/workspaces", {
    method: "POST", body: { name: `Market second ${run}`, slug: `market-second-${run}` },
  });
  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const firstName = `first-only-${run}`;
    const secondName = `second-only-${run}`;
    await api.requestJSON("/api/skills", {
      method: "POST", body: { name: firstName, description: "First workspace only", content: "# First workspace" },
    });
    api.setWorkspaceId(second.id);
    api.setWorkspaceSlug(second.slug);
    await api.requestJSON("/api/skills", {
      method: "POST", body: { name: secondName, description: "Second workspace only", content: "# Second workspace" },
    });
    await page.addInitScript(({ token, slug }) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica:chat:isOpen", "false");
      const key = `multica_skills_view:${slug}`;
      if (!localStorage.getItem(key)) {
        localStorage.setItem(key, JSON.stringify({ version: 1, state: { libraryView: "market", templatesCollapsed: false } }));
      }
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
      document.cookie = "multica-locale=en; path=/; SameSite=Lax";
    }, { token: api.getToken(), slug: first.slug });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${first.slug}/skills`);
    await expect(page.getByText(firstName, { exact: true })).toBeVisible();
    await expect(page.getByText(secondName, { exact: true })).toHaveCount(0);
    await expect(page.getByRole("textbox", { name: "Search templates", exact: true })).toHaveCount(0);
    await page.getByRole("link", { name: "From template", exact: true }).click();
    const builtin = page.getByRole("tab", { name: "Platform built-ins", exact: true });
    const all = page.getByRole("tab", { name: "All", exact: true });
    const deployment = page.getByRole("tab", { name: "Deployment-provided", exact: true });
    await builtin.click();
    const quality = page.getByRole("group", { name: "Categories", exact: true })
      .getByRole("button", { name: "Testing & quality", exact: true });
    await quality.click();
    const search = page.getByRole("textbox", { name: "Search templates", exact: true });
    await search.fill("multica-code-review");
    const preview = page.getByRole("button", { name: "Preview multica-code-review", exact: true });
    await preview.click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "Use this template", exact: true }).click();
    const draftName = `discard-on-workspace-change-${run}`;
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill(draftName);

    await page.goto(`/${second.slug}/marketplace?type=skills`);
    await expect(dialog).toHaveCount(0);
    await expect(all).toHaveAttribute("aria-selected", "true");
    await expect(search).toHaveValue("");
    await expect(quality).toHaveAttribute("aria-pressed", "false");
    await deployment.click();
    await expect(deployment).toHaveAttribute("aria-selected", "true");

    // Use the real workspace switcher so the same application instance must
    // rehydrate the prior workspace's preferences without retaining a draft.
    await page.getByRole("button", { name: new RegExp(`${second.name}$`) }).click();
    await page.getByRole("menuitem", { name: new RegExp(`${first.name}$`) }).click();
    await expect(page).toHaveURL(`/${first.slug}/issues`);
    await page.getByRole("link", { name: "Skills", exact: true }).click();
    await expect(page.getByText(firstName, { exact: true })).toBeVisible();
    await expect(page.getByText(secondName, { exact: true })).toHaveCount(0);
    await page.getByRole("link", { name: "From template", exact: true }).click();
    await expect(builtin).toHaveAttribute("aria-selected", "true");
    await expect(quality).toHaveAttribute("aria-pressed", "true");
    await expect(search).toHaveValue("");
    await expect(dialog).toHaveCount(0);
    await preview.click();
    await dialog.getByRole("button", { name: "Use this template", exact: true }).click();
    await expect(dialog.getByRole("textbox", { name: "Name", exact: true })).not.toHaveValue(draftName);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await page.getByRole("button", { name: new RegExp(`${first.name}$`) }).click();
    await page.getByRole("menuitem", { name: new RegExp(`${second.name}$`) }).click();
    await page.getByRole("link", { name: "Skills", exact: true }).click();
    await expect(page.getByText(secondName, { exact: true })).toBeVisible();
    await expect(page.getByText(firstName, { exact: true })).toHaveCount(0);
    await page.getByRole("link", { name: "From template", exact: true }).click();
    await expect(deployment).toHaveAttribute("aria-selected", "true");
    await expect(search).toHaveValue("");
    const screenshot = info.outputPath("market-second-workspace-isolated.png");
    await page.screenshot({ path: screenshot, animations: "disabled" });
    await info.attach("market-second-workspace-isolated", { path: screenshot, contentType: "image/png" });
    for (const workspace of [first, second]) {
      api.setWorkspaceId(workspace.id);
      api.setWorkspaceSlug(workspace.slug);
      expect(await api.requestJSON("/api/skills")).toHaveLength(1);
    }
    const persisted = await page.evaluate(() => Object.entries(localStorage)
      .filter(([key]) => key.startsWith("multica_skills_view:")));
    expect(JSON.stringify(persisted)).not.toContain(draftName);
  } finally {
    for (const workspace of [second, first]) {
      api.setWorkspaceId(workspace.id);
      api.setWorkspaceSlug(workspace.slug);
      await api.deleteFeatureWorkspace(workspace.id);
    }
  }
});

test("capability market separates catalog failures and polls only the active type", async ({ page }, info) => {
  test.setTimeout(180_000);
  page.setDefaultTimeout(15_000);
  const api = new TestApiClient();
  const slug = `market-recovery-${randomUUID().slice(0, 12)}`;
  await api.login(`${slug}@multica.ai`, "Market recovery tester");
  const workspace = await api.ensureWorkspace("Market recovery acceptance", slug);
  const counts = { mcp: 0, skills: 0 };
  let failMcp = true;
  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    await page.addInitScript((token) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica:chat:isOpen", "false");
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
      document.cookie = "multica-locale=en; path=/; SameSite=Lax";
    }, api.getToken());
    await page.route(new RegExp(`/api/workspaces/${workspace.id}/mcp-servers/templates(?:\\?.*)?$`), async (route) => {
      counts.mcp++;
      if (failMcp) {
        await route.fulfill({ status: 503, json: { error: "mcp_catalog_unavailable" } });
      } else {
        await route.continue();
      }
    });
    await page.route(/\/api\/skills\/templates(?:\?.*)?$/, async (route) => {
      counts.skills++;
      await route.continue();
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${slug}/marketplace?type=mcp`);
    const market = page.getByTestId("mcp-market");
    await expect(market.getByRole("alert")).toContainText("Could not load the MCP catalog. Try again.");
    await expect(market.getByText("No MCP templates available.", { exact: true })).toHaveCount(0);
    const types = page.getByRole("tablist", { name: "Capability type", exact: true });
    await types.getByRole("tab", { name: "Skills", exact: true }).click();
    await expect(page.getByRole("button", { name: "Preview multica-code-review", exact: true })).toBeVisible();
    const inactiveMcp = counts.mcp;
    const activeSkills = counts.skills;
    await expect.poll(() => counts.skills, { timeout: 35_000, message: "The active Skill catalog polls the real API" }).toBeGreaterThan(activeSkills);
    expect(counts.mcp, "An inactive failed MCP catalog must stop requesting").toBe(inactiveMcp);
    failMcp = false;
    await types.getByRole("tab", { name: "MCP", exact: true }).click();
    const preview = market.getByRole("button", { name: "View configuration: Chrome DevTools", exact: true });
    await expect(preview).toBeVisible();
    await expect(market.getByRole("alert")).toHaveCount(0);
    const inactiveSkills = counts.skills;
    const activeMcp = counts.mcp;
    await expect.poll(() => counts.mcp, { timeout: 35_000, message: "The active MCP catalog polls the real API" }).toBeGreaterThan(activeMcp);
    expect(counts.skills, "A retained inactive Skill catalog must stop requesting").toBe(inactiveSkills);

    failMcp = true;
    await market.getByRole("button", { name: "Deployment provided", exact: true }).click();
    await market.getByRole("button", { name: "All sources", exact: true }).click();
    await expect(market.getByRole("alert")).toContainText("Previously loaded templates are still shown.");
    await expect(preview).toBeVisible();
    await preview.click();
    const dialog = page.getByRole("dialog");
    const draft = dialog.getByRole("textbox", { name: "Configuration name", exact: true });
    await draft.fill("retained-after-catalog-failure");
    await page.goBack();
    await expect(dialog).toHaveCount(0);
    await page.goForward();
    await expect(draft).toHaveValue("retained-after-catalog-failure");
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    failMcp = false;
    await market.getByRole("alert").getByRole("button", { name: "Retry", exact: true }).click();
    await expect(market.getByRole("alert")).toHaveCount(0);
    await expect(preview).toBeVisible();
    const screenshot = info.outputPath("market-recovered-cached-catalog.png");
    await page.screenshot({ path: screenshot, animations: "disabled" });
    await info.attach("market-recovered-cached-catalog", { path: screenshot, contentType: "image/png" });
    await info.attach("catalog-request-counts", { body: JSON.stringify({ counts, inactiveMcp, inactiveSkills }), contentType: "application/json" });
    expect(await api.requestJSON(`/api/workspaces/${workspace.id}/mcp-servers`)).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
