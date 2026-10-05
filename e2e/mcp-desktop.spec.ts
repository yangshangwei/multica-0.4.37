import { createServer } from "node:http";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import { _electron as electron, expect, test, type ElectronApplication, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { verifyDeploymentMcpCatalog } from "./fixtures/mcp-deployment";

const recipes = [
  ["chrome-devtools", "Chrome DevTools"],
  ["playwright", "Playwright"],
  ["sequential-thinking", "Sequential Thinking"],
  ["serena", "Serena"],
  ["codebase-memory", "Codebase Memory MCP"],
  ["repomix", "Repomix"],
  ["markitdown", "MarkItDown MCP"],
  ["dbhub", "DBHub"],
  ["postgres-mcp", "Postgres MCP Pro"],
] as const;

test("native desktop MCP preserves collection, settings and agent discovery flows", async ({}, info) => {
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires a local password-mode API and freshly built desktop renderer/preload");
  test.setTimeout(240_000);
  const apiBase = process.env.NEXT_PUBLIC_API_URL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
  const root = resolve(import.meta.dirname, "..");
  const renderer = join(root, "apps/desktop/out/renderer");
  const profile = await mkdtemp(join(tmpdir(), "multica-mcp-desktop-"));
  const username = `mcp_desktop_${Date.now()}`;
  const password = "desktop-mcp-test";
  const api = new TestApiClient();
  let registered = false;
  let workspaceId: string | undefined;
  let desktop: ElectronApplication | undefined;
  let page: Page | undefined;
  const errors: string[] = [];

  // Serve the real desktop bundle and relay requests unchanged to the local
  // Go API. The existing native fixture isolates daemon/updater side effects.
  const server = createServer(async (req, res) => {
    try {
      const pathname = new URL(req.url ?? "/", "http://localhost").pathname;
      if (pathname.startsWith("/api/") || pathname.startsWith("/auth/") || pathname === "/health") {
        const chunks: Buffer[] = [];
        for await (const chunk of req) chunks.push(chunk);
        const headers = new Headers();
        for (const key of ["content-type", "authorization", "x-workspace-id", "x-workspace-slug", "accept-language"]) {
          const value = req.headers[key];
          if (typeof value === "string") headers.set(key, value);
        }
        const response = await fetch(`${apiBase}${req.url}`, {
          method: req.method, headers, body: chunks.length ? Buffer.concat(chunks) : undefined,
        });
        res.writeHead(response.status, { "Content-Type": response.headers.get("content-type") ?? "application/json" });
        res.end(Buffer.from(await response.arrayBuffer()));
        return;
      }
      const file = resolve(renderer, `.${pathname === "/" ? "/index.html" : pathname}`);
      if (!file.startsWith(`${renderer}/`)) { res.writeHead(404); res.end(); return; }
      const data = await readFile(file);
      const mime: Record<string, string> = {
        ".html": "text/html", ".js": "text/javascript", ".css": "text/css",
        ".woff2": "font/woff2", ".svg": "image/svg+xml",
      };
      res.writeHead(200, { "Content-Type": mime[extname(file)] ?? "application/octet-stream" });
      res.end(data);
    } catch { res.writeHead(502); res.end(); }
  });

  try {
    const registration = await fetch(`${apiBase}/auth/register`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password, name: "Desktop MCP verification" }),
    });
    expect(registration.status).toBe(201);
    registered = true;
    await api.loginPassword(username, password);
    const workspace = await api.ensureWorkspace("Desktop MCP verification", `mcp-desktop-${Date.now()}`);
    workspaceId = workspace.id;
    await api.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
    await api.requestJSON("/api/me/onboarding/complete", { method: "POST" });
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const runtime = await api.seedProjectRuntime();
    const agent = await api.requestJSON<{ id: string; name: string }>("/api/agents", {
      method: "POST", body: { name: "Native MCP reviewer", runtime_id: runtime.id, permission_mode: "private" },
    });

    await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
    const address = server.address();
    if (!address || typeof address === "string") throw new Error("Missing local renderer address");
    const origin = `http://127.0.0.1:${address.port}`;
    desktop = await electron.launch({
      executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin"
        ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"),
      args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
      env: {
        ...process.env, CHANGELOG_ELECTRON_PROFILE: profile,
        CHANGELOG_E2E_API_URL: origin, CHANGELOG_ELECTRON_RENDERER_URL: origin,
        CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en-US",
      },
    });
    page = await desktop.firstWindow();
    page.on("pageerror", (error) => errors.push(error.message));
    await page.locator("#root > *").first().waitFor({ state: "attached" });
    await page.context().addInitScript((token) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "dark");
    }, api.getToken());
    await page.reload();
    await page.getByRole("link", { name: "MCP", exact: true }).click();
    await expect(page.getByRole("heading", { name: "MCP", exact: true })).toHaveCount(1);
    const market = page.getByTestId("mcp-market");
    await expect(market.locator(".grid > div")).toHaveCount(recipes.length);
    const capture = async (name: string) => {
      await page!.evaluate(() => document.fonts.ready);
      expect(await page!.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const path = info.outputPath(`${name}.png`);
      await page!.screenshot({ path, animations: "disabled" });
      await info.attach(name, { path, contentType: "image/png" });
    };
    const resize = async (width: number) => {
      await desktop!.evaluate(({ BrowserWindow }, value) => {
        BrowserWindow.getAllWindows()[0]!.setContentSize(value, 900);
      }, width);
    };
    for (const [width, columns] of [[1380, 3], [800, 2], [420, 1]]) {
      await resize(width!);
      await expect.poll(() => market.locator(".grid > div").evaluateAll((cards) => {
        const top = cards[0]!.getBoundingClientRect().top;
        return cards.filter((card) => Math.abs(card.getBoundingClientRect().top - top) < 2).length;
      })).toBe(columns);
      await capture(`native-market-${width}`);
    }
    await page.getByRole("button", { name: "Add custom server", exact: true }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await capture("native-custom-narrow");
    await page.keyboard.press("Escape");
    await resize(1380);
    await market.getByRole("button", { name: "Browser", exact: true }).click();
    await expect(market.getByRole("status")).toHaveText("Templates found: 2");
    await expect(market.getByRole("button", { name: "Documentation & knowledge", exact: true })).toBeVisible();
    await market.getByRole("searchbox").fill("playwright");
    await expect(market.getByRole("status")).toHaveText("Templates found: 1");
    await expect(market.getByRole("button", { name: "View configuration: Playwright", exact: true })).toBeVisible();
    await capture("native-browser-search");
    await market.getByRole("searchbox").clear();
    await market.getByRole("button", { name: "All templates", exact: true }).click();

    for (const [key, title] of recipes) {
      const preview = market.getByRole("button", { name: `View configuration: ${title}`, exact: true });
      await preview.focus();
      await page.keyboard.press("Enter");
      const dialog = page.getByRole("dialog").last();
      const name = key === "playwright" ? "native-browser" : `native-${key}`;
      await dialog.getByRole("textbox", { name: "Configuration name", exact: true }).fill(name);
      if (key === "serena") {
        await dialog.getByLabel("Project directory", { exact: false }).fill("/tmp/mcp-desktop-e2e-project");
      } else if (key === "dbhub" || key === "postgres-mcp") {
        const input = dialog.getByLabel("Database connection URL", { exact: false });
        await expect(input).toHaveAttribute("type", "password");
        await input.fill("postgresql://mcp_test:desktop-test-secret@127.0.0.1:65432/example");
        if (key === "dbhub") await capture("native-database-input-masked");
      }
      await dialog.getByRole("button", { name: "Save and continue", exact: true }).click();
      const agentCheckbox = dialog.getByRole("checkbox", { name: agent.name, exact: true });
      await expect(agentCheckbox).toBeVisible();
      await expect(agentCheckbox).not.toBeChecked();
      const saved = (await api.requestJSON<{ id: string; name: string }[]>(`/api/workspaces/${workspace.id}/mcp-servers`)).find((server) => server.name === name);
      expect(saved).toMatchObject({ template_key: key, template_version: key === "postgres-mcp" ? "2" : "1", transport: "stdio" });
      expect(saved).not.toHaveProperty("config");
      expect(await api.requestJSON(`/api/agents/${agent.id}/mcp-servers`)).not.toEqual(
        expect.arrayContaining([expect.objectContaining({ id: saved!.id })]),
      );
      await agentCheckbox.check();
      await dialog.getByRole("button", { name: "Assign selected", exact: true }).click();
      await dialog.getByRole("button", { name: "Done", exact: true }).click();
      await expect(preview).toBeFocused();
      expect(await api.requestJSON(`/api/agents/${agent.id}/mcp-servers`)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: saved!.id, name, transport: "stdio", enabled: true })]),
      );
    }
    expect(await api.requestJSON(`/api/workspaces/${workspace.id}/mcp-servers`)).toHaveLength(recipes.length);
    await page.getByRole("tab", { name: "Shared configurations", exact: true }).click();
    await expect(page.getByText("native-browser", { exact: true })).toBeVisible();
    await capture("native-shared-configurations");

    await page.getByRole("link", { name: "Settings", exact: true }).click();
    await page.getByRole("tab", { name: "MCP", exact: true }).click();
    await expect(page.getByRole("heading", { name: "MCP", exact: true })).toHaveCount(1);
    await page.getByRole("tab", { name: "MCP market", exact: true }).click();
    await expect(market.locator(".grid > div")).toHaveCount(recipes.length);
    const workspaceTab = page.getByRole("tab", { name: "Shared configurations", exact: true });
    const marketTab = page.getByRole("tab", { name: "MCP market", exact: true });
    await expect.poll(async () => {
      const left = await workspaceTab.boundingBox();
      const right = await marketTab.boundingBox();
      return Math.abs(left!.y - right!.y);
    }).toBeLessThan(2);
    await capture("native-settings-market");

    // Restore the agent destination through the desktop's persisted session;
    // native window.location stays on the renderer URL, not the memory route.
    await page.evaluate((url) => {
      const saved = JSON.parse(localStorage.getItem("multica_tabs")!);
      const group = saved.state.byWorkspace[saved.state.activeWorkspaceSlug];
      const active = group.tabs.find((tab: { id: string }) => tab.id === group.activeTabId);
      active.url = url;
      active.resourceKey = url;
      active.history = { stack: [url], index: 0 };
      active.memento = { scroll: {}, view: {} };
      localStorage.setItem("multica_tabs", JSON.stringify(saved));
    }, `/${workspace.slug}/agents/${agent.id}`);
    await page.reload();
    await page.getByRole("tab", { name: "Capabilities", exact: true }).click();
    await page.getByRole("tab", { name: "MCP", exact: true }).click();
    await page.getByRole("button", { name: "Add tools", exact: true }).click();
    await page.getByRole("dialog").getByRole("tab", { name: "MCP market", exact: true }).click();
    await expect(market.locator(".grid > div")).toHaveCount(recipes.length);
    await resize(600);
    await capture("native-agent-market-narrow");
    await page.keyboard.press("Escape");
    await resize(1380);
    if (process.env.E2E_MCP_TEMPLATE_DIR) {
      await page.getByRole("link", { name: "MCP", exact: true }).click();
      await verifyDeploymentMcpCatalog({ page, api, workspaceId: workspace.id, agent, capture });
    }
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.reload();
    await page.getByRole("link", { name: "MCP", exact: true }).click();
    await page.getByRole("tab", { name: "MCP 市场", exact: true }).click();
    await page.evaluate(() => { document.documentElement.classList.remove("dark"); document.documentElement.classList.add("light"); });
    await capture("native-chinese-light");
    await market.getByRole("button", { name: "浏览器", exact: true }).click();
    await expect(market.getByRole("status")).toHaveText("找到 2 个模板");
    await capture("native-browser-chinese-light");
    expect(errors).toEqual([]);
    expect(await desktop.evaluate(() => (globalThis as unknown as {
      changelogAcceptance: { daemonStarts: number };
    }).changelogAcceptance.daemonStarts)).toBe(0);
  } catch (error) {
    if (page && !page.isClosed()) {
      await info.attach("native-mcp-failure", { body: await page.screenshot(), contentType: "image/png" });
      await info.attach("native-mcp-failure-text", { body: await page.locator("body").innerText(), contentType: "text/plain" });
    }
    throw error;
  } finally {
    await desktop?.close();
    server.closeAllConnections();
    await new Promise<void>((done) => server.close(() => done()));
    try { if (workspaceId) await api.deleteFeatureWorkspace(workspaceId); }
    finally {
      if (registered) await api.deletePasswordAccount(username);
      await rm(profile, { recursive: true, force: true });
    }
  }
});
