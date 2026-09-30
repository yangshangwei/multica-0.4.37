import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

interface ServerSummary {
  id: string;
  name: string;
  template_key?: string | null;
  template_version?: string | null;
  enabled?: boolean;
}

async function setup(page: Page) {
  const api = new TestApiClient();
  const slug = `mcp-market-${Date.now().toString(36)}-${process.pid}`;
  await api.login(`${slug}@multica.ai`, "MCP market tester");
  const workspace = await api.ensureWorkspace("MCP market workspace", slug);
  expect(workspace.slug).toBe(slug);
  await api.markUserOnboarded();
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

test("creates all three recipes, preserves source after rename, and reuses an instance from an agent", async ({ page }, info) => {
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

    for (const [key, title] of [
      ["chrome-devtools", "Chrome DevTools"],
      ["playwright", "Playwright"],
      ["sequential-thinking", "Sequential Thinking"],
    ]) {
      await marketTab.click();
      await page.getByRole("button", { name: `View configuration: ${title}`, exact: true }).click();
      const dialog = page.getByRole("dialog");
      await dialog.getByRole("textbox", { name: "Configuration name", exact: true }).fill(key!);
      if (key === "chrome-devtools") await capture(page, info, "setup-wide");
      await dialog.getByRole("button", { name: "Save and continue", exact: true }).click();
      await expect(dialog.getByRole("button", { name: "Skip for now", exact: true })).toBeVisible();
      if (key === "chrome-devtools") {
        await expect(dialog.getByRole("checkbox", { name: agents[0]!.name, exact: true })).toBeVisible();
        await capture(page, info, "assignment-wide");
      }
      const saved = (await api.requestJSON<ServerSummary[]>(base)).find((server) => server.name === key);
      expect(saved).toMatchObject({ template_key: key, template_version: "1" });
      expect(saved).not.toHaveProperty("config");
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
    expect(servers).toHaveLength(3);
    const playwright = servers.find((server) => server.template_key === "playwright")!;
    await api.requestJSON(`${base}/${playwright.id}`, { method: "PUT", body: { name: "browser-production" } });
    await page.reload();
    const renamed = (await api.requestJSON<ServerSummary[]>(base)).find((server) => server.id === playwright.id);
    expect(renamed).toMatchObject({ name: "browser-production", template_key: "playwright" });
    await page.getByRole("tab", { name: "Shared configurations", exact: true }).click();
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
    expect(await api.requestJSON<ServerSummary[]>(base)).toHaveLength(3);
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
