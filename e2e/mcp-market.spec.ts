import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

interface ServerSummary {
  id: string;
  name: string;
  transport: string;
  template_key?: string | null;
  template_version?: string | null;
  enabled?: boolean;
}

const passwordAccounts: { api: TestApiClient; username: string }[] = [];
test.afterEach(async () => {
  for (const { api, username } of passwordAccounts.splice(0)) await api.deletePasswordAccount(username);
});

async function setup(page: Page) {
  const api = new TestApiClient();
  const slug = `mcp-market-${Date.now().toString(36)}-${process.pid}`;
  if (process.env.E2E_PASSWORD_AUTH === "1") {
    const apiBase = process.env.NEXT_PUBLIC_API_URL!;
    expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
    const username = `mcp_${Date.now()}_${process.pid}`;
    const password = "mcp-test-password";
    const registration = await fetch(`${apiBase}/auth/register`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password, name: "MCP market tester" }),
    });
    expect(registration.status).toBe(201);
    passwordAccounts.push({ api, username });
    await api.loginPassword(username, password);
    await api.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
    await api.requestJSON("/api/me/onboarding/complete", { method: "POST" });
  } else {
    await api.login(`${slug}@multica.ai`, "MCP market tester");
    await api.markUserOnboarded();
  }
  const workspace = await api.ensureWorkspace("MCP market workspace", slug);
  expect(workspace.slug).toBe(slug);
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
  const runtime = await api.seedProjectRuntime();
  const agents = [];
  for (const name of ["Browser reviewer", "Release reviewer"]) {
    agents.push(await api.requestJSON<{ id: string; name: string }>("/api/agents", {
      method: "POST", body: { name, runtime_id: runtime.id, permission_mode: "private" },
    }));
  }
  const token = api.getToken();
  if (!token) throw new Error("Fixture login did not return a token");
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
    localStorage.setItem("theme", "dark");
    document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
  }, token);
  return { api, workspace, agents, slug };
}

async function capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = info.outputPath(`${name}.png`);
  await page.screenshot({ path, animations: "disabled" });
  await info.attach(name, { path, contentType: "image/png" });
  expect(await page.evaluate(() => Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) <= innerWidth)).toBe(true);
}

async function expectDialogButtonFits(page: Page, name: string) {
  const button = page.getByRole("dialog").last().getByRole("button", { name, exact: true });
  await expect(button).toBeVisible();
  expect(await button.evaluate((element) => {
    const parent = element.closest('[role="dialog"]')!;
    const bounds = element.getBoundingClientRect();
    const container = parent.getBoundingClientRect();
    return bounds.left >= container.left && bounds.right <= container.right && element.scrollWidth <= element.clientWidth + 1;
  })).toBe(true);
}

test("uses the collection canvas and adapts template columns to the available width", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace, slug } = await setup(page);
  try {
    await page.setViewportSize({ width: 1440, height: 1000 });
    for (const route of ["agents", "skills"]) {
      await page.goto(`/${slug}/${route}`);
      await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
      await capture(page, info, `reference-${route}`);
    }
    await page.goto(`/${slug}/mcp`);
    await expect(page.getByRole("heading", { name: "MCP", exact: true })).toHaveCount(1);
    const header = page.locator("header").filter({ has: page.getByRole("heading", { level: 1, name: "MCP", exact: true }) });
    await expect(header.getByRole("button", { name: "Add custom server", exact: true })).toBeVisible();
    const market = page.getByTestId("mcp-market");
    const cards = market.locator(".grid > div");
    await expect(cards).toHaveCount(5);
    for (const [width, columns] of [[1440, 3], [900, 2], [390, 1]]) {
      await page.setViewportSize({ width: width!, height: 1000 });
      await expect.poll(async () => cards.evaluateAll((elements) => {
        const first = elements[0]!.getBoundingClientRect();
        return elements.filter((element) => Math.abs(element.getBoundingClientRect().top - first.top) < 2).length;
      })).toBe(columns);
      await capture(page, info, `collection-dark-${width}`);
    }
    await page.setViewportSize({ width: 1440, height: 1000 });
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.reload();
    await expect(market.getByRole("button", { name: /Playwright/ })).toBeVisible();
    await page.evaluate(() => {
      document.documentElement.classList.remove("dark");
      document.documentElement.classList.add("light");
      document.documentElement.style.colorScheme = "light";
    });
    await capture(page, info, "collection-chinese-light-wide");
    await page.setViewportSize({ width: 390, height: 844 });
    await capture(page, info, "collection-chinese-light-narrow");
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("creates and explicitly assigns all five recipes, preserves source after rename, and reuses an instance from an agent", async ({ page }, info) => {
  test.setTimeout(180_000);
  const { api, workspace, agents, slug } = await setup(page);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const base = `/api/workspaces/${workspace.id}/mcp-servers`;
  try {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${slug}/mcp`);
    const marketTab = page.getByRole("tab", { name: "MCP market", exact: true });
    await expect(marketTab).toHaveAttribute("aria-selected", "true");
    await capture(page, info, "market-wide");
    const market = page.getByTestId("mcp-market");
    await market.getByRole("button", { name: "Documentation & knowledge", exact: true }).click();
    await expect(market.getByRole("status")).toHaveText("Templates found: 2");
    await capture(page, info, "documentation-filter-wide");
    await market.getByRole("searchbox").fill("deepwiki");
    await expect(market.getByRole("status")).toHaveText("Templates found: 1");
    await expect(market.getByRole("button", { name: "View configuration: DeepWiki", exact: true })).toBeVisible();
    await market.getByRole("searchbox").clear();
    await market.getByRole("button", { name: "All templates", exact: true }).click();

    const templateIcons = new Map<string, { drawing: string; color: string; background: string }>();
    for (const [key, title, transport] of [
      ["chrome-devtools", "Chrome DevTools", "stdio"],
      ["playwright", "Playwright", "stdio"],
      ["sequential-thinking", "Sequential Thinking", "stdio"],
      ["microsoft-learn", "Microsoft Learn", "http"],
      ["deepwiki", "DeepWiki", "http"],
    ] as const) {
      await marketTab.click();
      const preview = page.getByRole("button", { name: `View configuration: ${title}`, exact: true });
      templateIcons.set(key!, await preview.locator("svg").first().evaluate((icon) => ({
        drawing: icon.innerHTML,
        color: getComputedStyle(icon).color,
        background: getComputedStyle(icon.parentElement!).backgroundColor,
      })));
      await preview.click();
      const dialog = page.getByRole("dialog");
      await dialog.getByRole("textbox", { name: "Configuration name", exact: true }).fill(key!);
      if (key === "chrome-devtools") await capture(page, info, "setup-wide");
      if (transport === "http") {
        await expect(dialog.getByRole("listitem").filter({ hasText: "Streamable HTTP" })).toBeVisible();
        await expect(dialog.getByRole("listitem").filter({ hasText: key === "microsoft-learn" ? /training|profile/i : /indexed.*public|public.*indexed/i })).toBeVisible();
        await capture(page, info, `${key}-setup-wide`);
      }
      await dialog.getByRole("button", { name: "Save and continue", exact: true }).click();
      await expect(dialog.getByRole("button", { name: "Skip for now", exact: true })).toBeVisible();
      if (key === "chrome-devtools") {
        await expect(dialog.getByRole("checkbox", { name: agents[0]!.name, exact: true })).toBeVisible();
        await capture(page, info, "assignment-wide");
      }
      const saved = (await api.requestJSON<ServerSummary[]>(base)).find((server) => server.name === key);
      expect(saved).toMatchObject({ template_key: key, template_version: "1", transport });
      expect(saved).not.toHaveProperty("config");
      expect(await api.requestJSON<ServerSummary[]>(`/api/agents/${agents[0]!.id}/mcp-servers`)).not.toEqual(
        expect.arrayContaining([expect.objectContaining({ id: saved!.id })]),
      );
      if (key === "sequential-thinking") {
        await dialog.getByRole("button", { name: "Skip for now", exact: true }).click();
      } else {
        await dialog.getByRole("checkbox", { name: agents[0]!.name, exact: true }).check();
        await dialog.getByRole("button", { name: "Assign selected", exact: true }).click();
        await expect.poll(async () => (await api.requestJSON<ServerSummary[]>(`/api/agents/${agents[0]!.id}/mcp-servers`)).some((server) => server.id === saved!.id)).toBe(true);
        await dialog.getByRole("button", { name: "Done", exact: true }).click();
      }
      await expect(dialog).not.toBeVisible();
    }

    const servers = await api.requestJSON<ServerSummary[]>(base);
    expect(servers).toHaveLength(5);
    const playwright = servers.find((server) => server.template_key === "playwright")!;
    const deepwiki = servers.find((server) => server.template_key === "deepwiki")!;
    await api.requestJSON(`${base}/${playwright.id}`, { method: "PUT", body: { name: "browser-production" } });
    await api.requestJSON(`${base}/${deepwiki.id}`, { method: "PUT", body: { name: "public-repository-docs" } });
    await page.reload();
    const renamed = (await api.requestJSON<ServerSummary[]>(base)).find((server) => server.id === playwright.id);
    expect(renamed).toMatchObject({ name: "browser-production", template_key: "playwright" });
    await page.getByRole("tab", { name: "Shared configurations", exact: true }).click();
    for (const saved of servers) {
      const name = saved.id === playwright.id ? "browser-production" : saved.id === deepwiki.id ? "public-repository-docs" : saved.name;
      const row = page.getByRole("listitem").filter({ has: page.getByText(name, { exact: true }) });
      await expect(row.getByText(saved.transport === "http" ? "Streamable HTTP" : "STDIO", { exact: true })).toBeVisible();
      expect(await row.locator("svg").first().evaluate((icon) => ({
        drawing: icon.innerHTML,
        color: getComputedStyle(icon).color,
        background: getComputedStyle(icon.parentElement!).backgroundColor,
      }))).toEqual(templateIcons.get(saved.template_key!));
    }
    await capture(page, info, "workspace-wide");
    await api.requestJSON(`/api/agents/${agents[0]!.id}/mcp-servers/${playwright.id}/enabled`, { method: "PUT", body: { enabled: false } });
    await page.getByRole("button", { name: "Assign browser-production", exact: true }).click();
    const resume = page.getByRole("dialog");
    await resume.getByRole("checkbox", { name: agents[0]!.name, exact: true }).check();
    await resume.getByRole("button", { name: "Assign selected", exact: true }).click();
    await resume.getByRole("button", { name: "Done", exact: true }).click();
    expect((await api.requestJSON<ServerSummary[]>(`/api/agents/${agents[0]!.id}/mcp-servers`)).find((server) => server.id === playwright.id)?.enabled).toBe(false);

    await page.goto(`/${slug}/agents/${agents[1]!.id}`);
    await page.getByRole("tab", { name: "Capabilities", exact: true }).click();
    await page.getByRole("tab", { name: "MCP", exact: true }).click();
    await page.getByRole("button", { name: "Add tools", exact: true }).click();
    await capture(page, info, "agent-context-wide");
    // Reuse an existing configuration; no second instance may be created.
    await page.getByRole("button", { name: `Use sequential-thinking: Assign to ${agents[1]!.name}`, exact: true }).click();
    const dialog = page.getByRole("dialog").last();
    await dialog.getByRole("button", { name: "Done", exact: true }).click();
    expect(await api.requestJSON<ServerSummary[]>(base)).toHaveLength(5);
    expect(await api.requestJSON<ServerSummary[]>(`/api/agents/${agents[1]!.id}/mcp-servers`)).toEqual(expect.arrayContaining([expect.objectContaining({ name: "sequential-thinking", enabled: true })]));

    const longAgentName = "Release reviewer for international customer onboarding and browser validation";
    await api.requestJSON(`/api/agents/${agents[1]!.id}`, { method: "PUT", body: { name: longAgentName } });
    await page.reload();
    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByRole("button", { name: "Add tools", exact: true }).click();
    await expectDialogButtonFits(page, `Use browser-production: Assign to ${longAgentName}`);
    await capture(page, info, "agent-context-long-narrow");
    await page.getByRole("dialog").getByRole("button", { name: "Done", exact: true }).click();

    await page.goto(`/${slug}/mcp`);
    await marketTab.click();
    await page.setViewportSize({ width: 390, height: 844 });
    await capture(page, info, "market-narrow");
    await page.getByRole("button", { name: "View configuration: Playwright", exact: true }).click();
    await capture(page, info, "setup-narrow");
    await page.keyboard.press("Escape");
    const longServerName = `browser-${"production-".repeat(5)}validation`;
    await api.requestJSON(`${base}/${playwright.id}`, { method: "PUT", body: { name: longServerName } });
    await page.reload();
    await marketTab.click();
    await page.getByRole("button", { name: "View configuration: Playwright", exact: true }).click();
    await expectDialogButtonFits(page, `Use ${longServerName}`);
    await capture(page, info, "setup-long-narrow");
    await page.keyboard.press("Escape");
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.reload();
    await page.getByRole("tab", { name: "MCP 市场", exact: true }).click();
    await capture(page, info, "market-chinese-wide");
    await market.getByRole("button", { name: "文档与知识", exact: true }).click();
    await expect(market.getByRole("status")).toHaveText("找到 2 个模板");
    await capture(page, info, "documentation-filter-chinese-wide");
    await page.getByRole("tab", { name: "共享配置", exact: true }).click();
    await capture(page, info, "workspace-chinese-wide");
    expect(errors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("retries a failed assignment without recreating the saved MCP or repeating successful grants", async ({ page }) => {
  test.setTimeout(120_000);
  const { api, workspace, agents, slug } = await setup(page);
  const requests: string[] = [];
  page.on("request", (request) => {
    if (request.method() === "POST" && /\/mcp-servers$/.test(new URL(request.url()).pathname)) requests.push(new URL(request.url()).pathname);
  });
  const failedPath = `/api/agents/${agents[1]!.id}/mcp-servers`;
  const failure = `**${failedPath}`;
  try {
    await page.route(failure, (route) => route.request().method() === "POST"
      ? route.fulfill({ status: 503, json: { error: "Temporary assignment failure" } })
      : route.continue());
    await page.goto(`/${slug}/mcp`);
    await page.getByRole("button", { name: "View configuration: Playwright", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "Save and continue", exact: true }).click();
    for (const agent of agents) await dialog.getByRole("checkbox", { name: agent.name, exact: true }).check();
    await dialog.getByRole("button", { name: "Assign selected", exact: true }).click();
    await expect(dialog.getByText("Temporary assignment failure", { exact: false })).toBeVisible();
    await page.unroute(failure);
    await dialog.getByRole("button", { name: /Retry/ }).click();
    await dialog.getByRole("button", { name: "Done", exact: true }).click();
    expect(await api.requestJSON<ServerSummary[]>(`/api/workspaces/${workspace.id}/mcp-servers`)).toHaveLength(1);
    expect(requests.filter((path) => path === `/api/workspaces/${workspace.id}/mcp-servers`)).toHaveLength(1);
    expect(requests.filter((path) => path === `/api/agents/${agents[0]!.id}/mcp-servers`)).toHaveLength(1);
    expect(requests.filter((path) => path === failedPath)).toHaveLength(2);
  } finally {
    await page.unroute(failure);
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("a member can discover templates and reuse a workspace instance for their agent without creating one", async ({ page }) => {
  test.setTimeout(120_000);
  const { api: owner, workspace, slug } = await setup(page);
  const member = new TestApiClient();
  const memberEmail = `member-${slug}@multica.ai`;
  try {
    await member.login(memberEmail, "MCP member tester");
    const invitation = await owner.requestJSON<{ id: string }>(`/api/workspaces/${workspace.id}/members`, {
      method: "POST", body: { email: memberEmail, role: "member" },
    });
    await member.requestJSON(`/api/invitations/${invitation.id}/accept`, { method: "POST" });
    member.setWorkspaceId(workspace.id);
    member.setWorkspaceSlug(slug);
    await member.markUserOnboarded();
    await member.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const runtime = await member.seedProjectRuntime();
    const agent = await member.requestJSON<{ id: string; name: string }>("/api/agents", {
      method: "POST", body: { name: "Member browser agent", runtime_id: runtime.id },
    });
    const server = await owner.requestJSON<ServerSummary>(`/api/workspaces/${workspace.id}/mcp-servers`, {
      method: "POST", body: { name: "shared-browser", template_key: "playwright", template_version: "1" },
    });
    // A new context prevents the owner fixture's init script from restoring its token.
    const context = await page.context().browser()!.newContext({ baseURL: process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN });
    try {
      await context.addInitScript((token) => {
        localStorage.setItem("multica_token", token!);
        localStorage.setItem("multica:chat:isOpen", "false");
        document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
      }, member.getToken());
      const memberPage = await context.newPage();
      await memberPage.goto(`/${slug}/mcp`);
      await memberPage.getByRole("tab", { name: "MCP market", exact: true }).click();
      await memberPage.getByRole("button", { name: "View configuration: Playwright", exact: true }).click();
      await expect(memberPage.getByRole("button", { name: "Save and continue", exact: true })).toHaveCount(0);
      await memberPage.keyboard.press("Escape");
      await memberPage.goto(`/${slug}/agents/${agent.id}`);
      await memberPage.getByRole("tab", { name: "Capabilities", exact: true }).click();
      await memberPage.getByRole("tab", { name: "MCP", exact: true }).click();
      await memberPage.getByRole("button", { name: "Add tools", exact: true }).click();
      await memberPage.getByRole("button", { name: `Use shared-browser: Assign to ${agent.name}`, exact: true }).click();
      const assignment = memberPage.getByRole("dialog").last();
      await assignment.getByRole("button", { name: "Done", exact: true }).click();
      expect(await member.requestJSON<ServerSummary[]>(`/api/agents/${agent.id}/mcp-servers`)).toEqual(expect.arrayContaining([expect.objectContaining({ id: server.id })]));
      expect(await owner.requestJSON<ServerSummary[]>(`/api/workspaces/${workspace.id}/mcp-servers`)).toHaveLength(1);
    } finally {
      await context.close();
    }
  } finally {
    await owner.deleteFeatureWorkspace(workspace.id);
  }
});
