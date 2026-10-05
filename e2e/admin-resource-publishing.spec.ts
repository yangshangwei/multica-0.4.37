import { randomUUID } from "node:crypto";
import { execFileSync } from "node:child_process";
import { createServer } from "node:http";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import { _electron as electron, request as playwrightRequest, test, expect, type Page, type ElectronApplication, type Cookie } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const report = resolve(".omx/reports/admin-resource-publishing/qa");
const apiURL = process.env.NEXT_PUBLIC_API_URL!;
const webURL = process.env.PLAYWRIGHT_BASE_URL!;
type Resource = { key: string; version: string; content_digest: string; state: string; source: string };
type Receipt = { resource: Resource; operation_id: string; replayed: boolean };
type Preview = { preview_digest: string; expected_version: string | null; files: { path: string }[] };
type AuthClient = Pick<TestApiClient, "getToken" | "requestJSON">;
type AdminSession = { apiURL: string; username: string; token: string; cookies: Cookie[] };

async function adminSession(): Promise<AdminSession> {
  const path = join(report, "admin-session.private.json");
  try {
    const saved = JSON.parse(await readFile(path, "utf8")) as AdminSession;
    if (saved.apiURL === apiURL && saved.username === process.env.E2E_PLATFORM_ADMIN_USERNAME && (await fetch(`${apiURL}/api/admin/me`, { headers: { Authorization: `Bearer ${saved.token}` } })).ok) return saved;
  } catch { /* A fresh task starts without a saved browser session. */ }
  const context = await playwrightRequest.newContext();
  try {
    const response = await context.post(`${apiURL}/auth/login`, { data: { username: process.env.E2E_PLATFORM_ADMIN_USERNAME, password: process.env.E2E_PLATFORM_ADMIN_PASSWORD } });
    expect(response.status()).toBe(200);
    const data = await response.json();
    const saved = { apiURL, username: process.env.E2E_PLATFORM_ADMIN_USERNAME!, token: data.token as string, cookies: (await context.storageState()).cookies };
    await mkdir(report, { recursive: true }); await writeFile(path, JSON.stringify(saved), { mode: 0o600 });
    return saved;
  } finally { await context.dispose(); }
}

async function capture(page: Page, name: string) {
  await mkdir(report, { recursive: true });
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: join(report, `${name}.png`), fullPage: true, animations: "disabled" });
  await writeFile(join(report, `${name}.aria.yml`), await page.locator("body").ariaSnapshot());
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

async function login(page: Page) {
  await page.context().addCookies((await adminSession()).cookies);
  await page.goto(`${webURL}/admin/resources`);
  await expect(page.getByRole("heading", { name: "Resource publishing", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Publish a resource", exact: true })).toBeVisible();
}

async function adminClient() {
  for (const value of [apiURL, webURL]) expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
  const session = await adminSession();
  const api: AuthClient = {
    getToken: () => session.token,
    async requestJSON<T>(path: string, options: { method?: string; body?: unknown; headers?: Record<string, string> } = {}): Promise<T> {
      const response = await request(api, path, { method: options.method, headers: { "Content-Type": "application/json", ...options.headers }, body: options.body === undefined ? undefined : JSON.stringify(options.body) });
      if (!response.ok) throw new Error(`${options.method ?? "GET"} ${path} failed: ${response.status}`);
      return response.json();
    },
  };
  return api;
}

async function request(api: AuthClient, path: string, init: RequestInit = {}) {
  return fetch(`${apiURL}${path}`, { ...init, headers: { Authorization: `Bearer ${api.getToken()}`, ...init.headers } });
}

async function preview(api: AuthClient, kind: string, key: string, file: string): Promise<Preview> {
  const body = new FormData(); body.set("key", key); body.set("file", new Blob([await readFile(file)]), file.split("/").at(-1)!);
  const response = await request(api, `/api/admin/resources/${kind}/preview`, { method: "POST", body });
  expect(response.status).toBe(200);
  return response.json();
}

async function publish(api: AuthClient, kind: string, key: string, file: string, draft: Preview, operation = randomUUID()) {
  const body = new FormData(); body.set("file", new Blob([await readFile(file)]), file.split("/").at(-1)!);
  body.set("preview_digest", draft.preview_digest); body.set("expected_version", draft.expected_version ?? ""); body.set("reason", "Synthetic publishing acceptance");
  return request(api, `/api/admin/resources/${kind}/${key}/publish`, { method: "POST", body, headers: { "Idempotency-Key": operation } });
}

async function upload(page: Page, key: string, file: string) {
  await page.getByRole("button", { name: "Publish a resource", exact: true }).click();
  await page.getByLabel("Resource key", { exact: true }).fill(key);
  await page.getByLabel("Resource file", { exact: true }).setInputFiles(file);
  await page.getByRole("button", { name: "Validate and preview", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Validated preview", exact: true })).toBeVisible();
}

async function addSkill(page: Page, key: string, copyName: string) {
  await page.getByRole("tab", { name: "Skill templates", exact: true }).click();
  await page.getByRole("tab", { name: "Deployment-provided", exact: true }).click();
  await page.getByRole("button", { name: `Preview ${key}`, exact: true }).click();
  const dialog = page.getByRole("dialog").last();
  await dialog.getByRole("button", { name: "Use this template", exact: true }).click();
  await dialog.locator("#template-skill-name").fill(copyName);
  await dialog.getByRole("button", { name: "Create skill", exact: true }).click();
  await expect(dialog).toHaveCount(0);
}

async function addMcp(page: Page, title: string, copyName: string, agentName: string) {
  await page.getByRole("tab", { name: "MCP market", exact: true }).click();
  const market = page.getByTestId("mcp-market");
  await market.getByRole("button", { name: "Deployment provided", exact: true }).click();
  await market.getByRole("button", { name: `View configuration: ${title}`, exact: true }).click();
  const dialog = page.getByRole("dialog").last();
  await dialog.getByRole("textbox", { name: "Configuration name", exact: true }).fill(copyName);
  await dialog.getByRole("button", { name: "Save and continue", exact: true }).click();
  await expect(dialog.getByRole("checkbox", { name: agentName, exact: true })).not.toBeChecked();
  await dialog.getByRole("button", { name: "Skip for now", exact: true }).click();
}

async function nativeConsumer(api: TestApiClient, workspace: { id: string; slug: string }, key: string, title: string, agentName: string) {
  const root = resolve(".");
  const build = process.env.E2E_RESOURCE_DESKTOP_BUILD ?? join(report, "../desktop-build");
  const renderer = resolve(build, "renderer");
  const profile = await mkdtemp(join(tmpdir(), "multica-resource-qa-"));
  let desktop: ElectronApplication | undefined;
  const server = createServer(async (req, res) => {
    try {
      const pathname = new URL(req.url ?? "/", "http://localhost").pathname;
      if (pathname.startsWith("/api/") || pathname.startsWith("/auth/") || pathname === "/health") {
        const chunks: Buffer[] = []; for await (const chunk of req) chunks.push(chunk);
        const headers = new Headers();
        for (const name of ["content-type", "authorization", "x-workspace-id", "x-workspace-slug", "accept-language"]) {
          const value = req.headers[name]; if (typeof value === "string") headers.set(name, value);
        }
        const response = await fetch(`${apiURL}${req.url}`, { method: req.method, headers, body: chunks.length ? Buffer.concat(chunks) : undefined });
        res.writeHead(response.status, { "Content-Type": response.headers.get("content-type") ?? "application/json" });
        res.end(Buffer.from(await response.arrayBuffer())); return;
      }
      const file = resolve(renderer, `.${pathname === "/" ? "/index.html" : pathname}`);
      if (!file.startsWith(`${renderer}/`)) { res.writeHead(404); res.end(); return; }
      const data = await readFile(file);
      const mime: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".woff2": "font/woff2", ".svg": "image/svg+xml" };
      res.writeHead(200, { "Content-Type": mime[extname(file)] ?? "application/octet-stream" }); res.end(data);
    } catch { res.writeHead(502); res.end(); }
  });
  try {
    await new Promise<void>(done => server.listen(0, "127.0.0.1", done));
    const address = server.address(); if (!address || typeof address === "string") throw new Error("No renderer port");
    const origin = `http://127.0.0.1:${address.port}`;
    desktop = await electron.launch({ executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin" ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"), args: [join(root, "e2e/fixtures/changelog-electron.cjs")], env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_E2E_API_URL: origin, CHANGELOG_ELECTRON_RENDERER_URL: origin, CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en-US", CHANGELOG_ELECTRON_PRELOAD: resolve(build, "preload/index.js") } });
    const page = await desktop.firstWindow();
    const errors: string[] = []; page.on("pageerror", error => errors.push(error.message));
    await page.context().addInitScript(token => { localStorage.setItem("multica_token", token!); localStorage.setItem("multica:chat:isOpen", "false"); localStorage.setItem("theme", "dark"); }, api.getToken());
    await page.reload();
    await page.getByRole("link", { name: "Skills", exact: true }).click();
    await addSkill(page, key, `native-${key}`);
    await capture(page, "native-skill-created");
    await page.getByRole("link", { name: "MCP", exact: true }).click();
    await addMcp(page, title, `native-${key}`, agentName);
    await capture(page, "native-mcp-created-unassigned");
    expect(errors).toEqual([]);
    expect(await desktop.evaluate(() => (globalThis as unknown as { changelogAcceptance: { daemonStarts: number } }).changelogAcceptance.daemonStarts)).toBe(0);
    expect(await api.requestJSON(`/api/workspaces/${workspace.id}/mcp-servers?mcp_source_version=1`)).toEqual(expect.arrayContaining([expect.objectContaining({ name: `native-${key}`, template_source: "deployment" })]));
  } finally {
    await desktop?.close(); server.closeAllConnections(); await new Promise<void>(done => server.close(() => done())); await rm(profile, { recursive: true, force: true });
  }
}

test.describe("administrator resource publishing", () => {
  test.use({ actionTimeout: 15000 });
  test.skip(process.env.E2E_RESOURCE_PUBLISHING !== "1", "Requires task-owned API, Web, database and managed storage");

  test("first resource page screenshot", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 }); await login(page); await capture(page, "resources-first-desktop");
  });

  test("localized responsive states and deployment-disabled feedback", async ({ page, context }) => {
    test.setTimeout(180_000);
    await login(page);
    // Keep the administrator's saved locale unchanged; vary only this browser's
    // locale cookie and the read response used by UserLocaleSync.
    await page.route("**/api/me", async route => {
      const response = await route.fetch(); const data = await response.json();
      await route.fulfill({ response, json: { ...data, language: null } });
    });
    const audits = [];
    for (const locale of ["en", "zh-Hans"]) {
      await context.addCookies([{ name: "multica-locale", value: locale, url: webURL }]);
      for (const width of [1440, 390]) {
        await page.setViewportSize({ width, height: 1000 });
        await page.reload();
        await expect(page.getByRole("button", { name: locale === "en" ? "Publish a resource" : "发布资源", exact: true })).toBeVisible();
        for (const theme of ["light", "dark"]) {
          await page.evaluate(value => { document.documentElement.classList.remove("light", "dark"); document.documentElement.classList.add(value); }, theme);
          await capture(page, `resources-${locale}-${width}-${theme}`);
          const audit = await page.locator("main").evaluateAll(roots => {
            const controls = roots.flatMap(root => [...root.querySelectorAll('button, input, select, textarea, summary, a')]);
            return controls.filter(el => el.getClientRects().length && getComputedStyle(el).visibility !== "hidden").map(el => ({ name: el.textContent?.trim() || el.getAttribute("aria-label"), height: el.getBoundingClientRect().height, width: el.getBoundingClientRect().width })).filter(el => el.height < 43.5 || el.width < 43.5);
          });
          const contrast = await page.locator("main").evaluateAll(roots => {
            const canvas = document.createElement("canvas"); canvas.width = canvas.height = 1;
            const ctx = canvas.getContext("2d", { willReadFrequently: true })!;
            const rgba = (value: string) => { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data]; };
            const blend = (top: number[], bottom: number[]) => top.slice(0, 3).map((value, index) => value * (top[3]! / 255) + bottom[index]! * (1 - top[3]! / 255));
            const background = (el: Element | null): number[] => el ? blend(rgba(getComputedStyle(el).backgroundColor), background(el.parentElement)) : [255, 255, 255];
            const luminance = (rgb: number[]) => rgb.reduce((sum, value, index) => { const c = value / 255; return sum + (c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4) * [0.2126, 0.7152, 0.0722][index]!; }, 0);
            const samples = roots.flatMap(root => [...root.querySelectorAll("p, h1, h2, h3, dt, dd, span, button, summary")]).filter(el => el.getClientRects().length && [...el.childNodes].some(node => node.nodeType === Node.TEXT_NODE && node.textContent?.trim()) && !el.closest(":disabled")).map(el => {
              const style = getComputedStyle(el); const bg = background(el); const fg = blend(rgba(style.color), bg); const l1 = luminance(bg); const l2 = luminance(fg);
              const ratio = (Math.max(l1, l2) + .05) / (Math.min(l1, l2) + .05);
              const required = parseFloat(style.fontSize) >= 24 || parseFloat(style.fontSize) >= 18.66 && parseInt(style.fontWeight) >= 700 ? 3 : 4.5;
              return { text: el.textContent?.trim().slice(0, 100), ratio: Math.round(ratio * 100) / 100, required };
            });
            return { sampled: samples.length, minimum: Math.min(...samples.map(sample => sample.ratio)), failures: samples.filter(sample => sample.ratio < sample.required) };
          });
          audits.push({ locale, width, theme, undersizedControls: audit, contrast });
        }
      }
    }
    await writeFile(join(report, "touch-target-audit.json"), JSON.stringify(audits, null, 2));
    await context.addCookies([{ name: "multica-locale", value: "en", url: webURL }]);
    await page.route("**/api/admin/resources?kind=skill", async route => {
      const response = await route.fetch(); const data = await response.json();
      await route.fulfill({ response, json: { ...data, enabled: false, can_publish: false } });
    });
    await page.goto(`${webURL}/admin/resources?kind=skill`);
    await expect(page.getByText(/Publishing is not enabled for this deployment/)).toBeVisible();
    await expect(page.getByRole("button", { name: "Publish a resource", exact: true })).toHaveCount(0);
    await capture(page, "publishing-disabled-capability-fixture");
    await page.unroute("**/api/admin/resources?kind=skill");
    await page.route("**/api/admin/resources?kind=skill", route => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "resource_store_unavailable" }) }));
    await page.reload();
    await expect(page.getByRole("alert").filter({ hasText: "Resource publishing is temporarily unavailable" })).toBeVisible();
    await capture(page, "publishing-unavailable-response-fixture");
  });

  test("real uploads reach Web and native markets; revisions, recovery and withdrawal preserve copies", async ({ page, browser }) => {
    test.setTimeout(300_000);
    const admin = await adminClient();
    const id = randomUUID().slice(0, 8); const skillKey = `qa-skill-${id}`; const mcpKey = `qa-mcp-${id}`; const title = `QA MCP ${id}`;
    const username = `pubqa${id}`; const consumer = new TestApiClient();
    const password = `fixture-${randomUUID()}`;
    await consumer.registerPassword(username, password, `Publishing QA ${id}`);
    const workspace = await consumer.ensureWorkspace(`Publishing QA ${id}`, `publishing-qa-${id}`);
    await consumer.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
    await consumer.requestJSON("/api/me/onboarding/complete", { method: "POST" });
    await consumer.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const runtime = await consumer.seedProjectRuntime();
    const agent = await consumer.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: { name: `QA unassigned ${id}`, runtime_id: runtime.id, permission_mode: "private" } });
    const initialAgentSkills = await consumer.requestJSON(`/api/agents/${agent.id}/skills`);
    const directory = await mkdtemp(join(tmpdir(), "multica-resource-files-"));
    const skillFile = join(directory, "skill.zip"); const replacement = join(directory, "SKILL.md"); const mcpFile = join(directory, "mcp.json");
    const content = `---\nname: ${skillKey}\ndescription: Synthetic publishing acceptance\n---\n# Publishing QA\nKeep this copied revision unchanged.\n`;
    execFileSync("python3", ["-c", "import sys,zipfile; z=zipfile.ZipFile(sys.argv[1],'w'); z.writestr('bundle/SKILL.md',sys.argv[2]); z.writestr('bundle/references/guide.md','Synthetic attachment preserved.'); z.close()", skillFile, content]);
    await writeFile(replacement, content.replace("unchanged", "updated"));
    await writeFile(mcpFile, JSON.stringify({ schema_version: 1, titles: { en: title, zh: `发布测试 ${id}` }, descriptions: { en: "Synthetic resource publishing acceptance" }, category: "documentation", config: { type: "http", url: "https://mcp.example.test/never-executed" } }));
    const context = await browser.newContext({ locale: "en-US" });
    const errors: string[] = []; page.on("pageerror", error => errors.push(error.message));
    let skillRevision = ""; let mcpRevision = "";
    try {
      await page.setViewportSize({ width: 1440, height: 1000 }); await login(page);
      await upload(page, skillKey, skillFile);
      await expect(page.getByText("references/guide.md", { exact: true })).toBeVisible();
      await capture(page, "skill-preview-desktop");
      await page.getByRole("button", { name: "Confirm publication", exact: true }).click();
      await expect(page.getByText("Enter a reason to continue.", { exact: true })).toBeVisible();
      await page.getByLabel("Reason", { exact: true }).fill("Synthetic skill package acceptance");
      let writes = 0; let operation = "";
      const responseFault = await page.context().newCDPSession(page);
      await responseFault.send("Fetch.enable", { patterns: [{ urlPattern: `*/api/admin/resources/skill/${skillKey}/publish`, requestStage: "Response" }] });
      responseFault.on("Fetch.requestPaused", async event => {
        if (event.request.method !== "POST") { await responseFault.send("Fetch.continueRequest", { requestId: event.requestId }); return; }
        writes++; operation = Object.entries(event.request.headers).find(([name]) => name.toLowerCase() === "idempotency-key")?.[1] ?? "";
        await writeFile(join(report, "publish-response-metadata.json"), JSON.stringify({ status: event.responseStatusCode, operation, writes }));
        await responseFault.send("Fetch.failRequest", { requestId: event.requestId, errorReason: "Failed" });
      });
      await page.getByRole("button", { name: "Confirm publication", exact: true }).click();
      await expect(page.getByText(/Resource published\. Supported clients/)).toBeVisible({ timeout: 15000 });
      expect(writes).toBe(1);
      const receipt = await admin.requestJSON<Receipt>(`/api/admin/resources/operations/${operation}`); skillRevision = receipt.resource.version;
      expect(receipt.resource).toMatchObject({ key: skillKey, source: "managed", state: "published" });
      await responseFault.send("Fetch.disable"); await responseFault.detach();
      await capture(page, "skill-published-recovered");
      await page.getByRole("tab", { name: "MCP", exact: true }).click();
      await expect(page.getByRole("tab", { name: "MCP", exact: true })).toHaveAttribute("aria-selected", "true");
      await upload(page, mcpKey, mcpFile);
      await page.getByLabel("Reason", { exact: true }).fill("Synthetic MCP publishing acceptance");
      const mcpPublished = page.waitForResponse(response => response.url().endsWith(`/mcp/${mcpKey}/publish`) && response.request().method() === "POST");
      await page.getByRole("button", { name: "Confirm publication", exact: true }).click();
      mcpRevision = ((await (await mcpPublished).json()) as Receipt).resource.version;
      await expect(page.getByText(/Resource published\. Supported clients/)).toBeVisible({ timeout: 15000 });
      const skillCatalog = await consumer.requestJSON<{ templates: Array<{ name: string; version: number; content: string; files: unknown[] }> }>("/api/skills/templates");
      expect(skillCatalog.templates).toEqual(expect.arrayContaining([expect.objectContaining({ name: skillKey, version: 0, content })]));
      const mcpCatalog = await consumer.requestJSON<{ templates: Array<{ key: string; source: string; version: string }> }>(`/api/workspaces/${workspace.id}/mcp-servers/templates?language=en`);
      expect(mcpCatalog.templates).toEqual(expect.arrayContaining([expect.objectContaining({ key: mcpKey, source: "deployment" })]));
      expect((await context.request.post(`${webURL}/auth/login`, { data: { username, password } })).status()).toBe(200);
      const marketPage = await context.newPage(); await marketPage.goto(`${webURL}/${workspace.slug}/skills`);
      await addSkill(marketPage, skillKey, `web-${skillKey}`); await capture(marketPage, "web-skill-created");
      await marketPage.goto(`${webURL}/${workspace.slug}/mcp`); await addMcp(marketPage, title, `web-${mcpKey}`, agent.name); await capture(marketPage, "web-mcp-created-unassigned");
      const skills = await consumer.requestJSON<Array<{ id: string; name: string }>>("/api/skills"); const skillCopy = skills.find(item => item.name === `web-${skillKey}`)!;
      expect(skillCopy).toBeTruthy();
      const savedSkill = await consumer.requestJSON(`/api/skills/${skillCopy.id}`);
      const savedMcp = await consumer.requestJSON(`/api/workspaces/${workspace.id}/mcp-servers?mcp_source_version=1`);
      expect(await consumer.requestJSON(`/api/agents/${agent.id}/mcp-servers`)).toEqual([]);
      if (process.env.E2E_RESOURCE_NATIVE === "1") await nativeConsumer(consumer, workspace, skillKey, title, agent.name);
      expect(await consumer.requestJSON(`/api/agents/${agent.id}/skills`)).toEqual(initialAgentSkills);
      expect(await consumer.requestJSON(`/api/agents/${agent.id}/mcp-servers`)).toEqual([]);
      await writeFile(mcpFile, JSON.stringify({ schema_version: 1, titles: { en: title, zh: `发布测试 ${id}` }, descriptions: { en: "Updated synthetic template" }, category: "documentation", config: { type: "http", url: "https://mcp.example.test/revised-never-executed" } }));
      for (const [kind, key, file, before] of [["skill", skillKey, replacement, skillRevision], ["mcp", mcpKey, mcpFile, mcpRevision]]) {
        const stalePreview = await preview(admin, kind!, key!, file!);
        await page.goto(`${webURL}/admin/resources?kind=${kind}`);
        await page.getByRole("group", { name: `Actions: ${key}`, exact: true }).getByRole("button", { name: "Update", exact: true }).click();
        await expect(page.getByLabel("Resource key", { exact: true })).toHaveAttribute("readonly", "");
        await page.getByLabel("Resource file", { exact: true }).setInputFiles(file!);
        await page.getByRole("button", { name: "Validate and preview", exact: true }).click();
        await expect(page.getByRole("heading", { name: "Validated preview", exact: true })).toBeVisible();
        await page.getByLabel("Reason", { exact: true }).fill("Synthetic explicit replacement acceptance");
        const response = page.waitForResponse(item => item.url().endsWith(`/${kind}/${key}/publish`) && item.request().method() === "POST");
        await page.getByRole("button", { name: "Confirm update", exact: true }).click();
        const updated = await (await response).json() as Receipt;
        expect(updated.resource.version).not.toBe(before);
        if (kind === "skill") skillRevision = updated.resource.version; else mcpRevision = updated.resource.version;
        await expect(page.getByText(/Resource published\. Supported clients/)).toBeVisible();
        const conflict = await publish(admin, kind!, key!, file!, stalePreview); expect(conflict.status).toBe(409);
      }
      expect(await consumer.requestJSON(`/api/skills/${skillCopy.id}`)).toEqual(savedSkill);
      for (const [kind, key, version] of [["skill", skillKey, skillRevision], ["mcp", mcpKey, mcpRevision]]) {
        await page.goto(`${webURL}/admin/resources?kind=${kind}`);
        await page.getByRole("group", { name: `Actions: ${key}`, exact: true }).getByRole("button", { name: "Withdraw", exact: true }).click();
        await expect(page.getByText(`Target: ${key} · current revision: ${version}`, { exact: true })).toBeVisible();
        await page.getByLabel("Reason", { exact: true }).fill("End synthetic publishing acceptance");
        await page.getByRole("button", { name: "Confirm withdrawal", exact: true }).click();
        await expect(page.getByText("Resource withdrawn. Existing workspace copies and bindings are unchanged.", { exact: true })).toBeVisible();
      }
      expect(await consumer.requestJSON(`/api/skills/${skillCopy.id}`)).toEqual(savedSkill);
      const afterMcp = await consumer.requestJSON<unknown[]>(`/api/workspaces/${workspace.id}/mcp-servers?mcp_source_version=1`);
      expect(afterMcp).toEqual(expect.arrayContaining(savedMcp as unknown[]));
      expect((await consumer.requestJSON<{ templates: { name: string }[] }>("/api/skills/templates")).templates.some(item => item.name === skillKey)).toBe(false);
      expect((await consumer.requestJSON<{ templates: { key: string }[] }>(`/api/workspaces/${workspace.id}/mcp-servers/templates`)).templates.some(item => item.key === mcpKey)).toBe(false);
      expect((await request(consumer, "/api/admin/resources?kind=skill")).status).toBe(403);
      const forbiddenBody = new FormData(); forbiddenBody.set("key", `forbidden-${id}`); forbiddenBody.set("file", new Blob([content]), "SKILL.md");
      expect((await request(consumer, "/api/admin/resources/skill/preview", { method: "POST", body: forbiddenBody })).status).toBe(403);
      expect(errors).toEqual([]);
      await writeFile(join(report, "functional-result.json"), JSON.stringify({ status: "passed", skillKey, mcpKey, native: process.env.E2E_RESOURCE_NATIVE === "1", lostResponseWrites: writes, receiptOperation: operation, revisionConflict: 409, copiesPreserved: true, forbidden: 403, daemonExecution: false }, null, 2));
    } catch (error) {
      await capture(page, "functional-failure"); throw error;
    } finally {
      await context.close().catch(() => {});
      // Withdraw only this run's synthetic keys, even after a mid-flow failure.
      for (const [kind, key] of [["skill", skillKey], ["mcp", mcpKey]]) {
        const list = await admin.requestJSON<{ items: Resource[] }>(`/api/admin/resources?kind=${kind}`);
        const active = list.items.find(item => item.key === key && item.source === "managed" && item.state === "published");
        if (active) await admin.requestJSON(`/api/admin/resources/${kind}/${key}/withdraw`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { expected_version: active.version, reason: "Synthetic publishing acceptance cleanup" } });
      }
      await consumer.deleteFeatureWorkspace(workspace.id); await consumer.deletePasswordAccount(username); await rm(directory, { recursive: true, force: true });
    }
  });
});
