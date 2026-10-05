import { createServer } from "node:http";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import { _electron as electron, expect, test, type ElectronApplication } from "@playwright/test";
import { p1Session, p1Project, p1Authenticate, p1Capture, p1NoOverflow, p1Overview } from "./fixtures/project-p1";

test("P1 real Electron preload and shared router publish, drill down, and retain Chinese compact history without a daemon", async ({}, info) => {
  test.setTimeout(180_000);
  const root = resolve(import.meta.dirname, ".."); const renderer = join(root, "apps/desktop/out/renderer");
  const apiBase = process.env.NEXT_PUBLIC_API_URL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
  const profile = await mkdtemp(join(tmpdir(), "multica-project-p1-desktop-"));
  const { api, workspace } = await p1Session();
  let desktop: ElectronApplication | undefined;
  const errors: string[] = [];
  const server = createServer(async (req, res) => {
    try {
      const pathname = new URL(req.url ?? "/", "http://localhost").pathname;
      if (pathname.startsWith("/api/") || pathname.startsWith("/auth/") || pathname === "/health") {
        const chunks: Buffer[] = []; for await (const chunk of req) chunks.push(chunk);
        const headers = new Headers();
        for (const key of ["content-type", "authorization", "x-workspace-id", "x-workspace-slug", "accept-language"]) { const value = req.headers[key]; if (typeof value === "string") headers.set(key, value); }
        const response = await fetch(`${apiBase}${req.url}`, { method: req.method, headers, body: chunks.length ? Buffer.concat(chunks) : undefined });
        res.writeHead(response.status, { "Content-Type": response.headers.get("content-type") ?? "application/json" }); res.end(Buffer.from(await response.arrayBuffer())); return;
      }
      const file = resolve(renderer, `.${pathname === "/" ? "/index.html" : pathname}`);
      if (!file.startsWith(`${renderer}/`)) { res.writeHead(404); res.end(); return; }
      const data = await readFile(file);
      const mime: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".woff2": "font/woff2", ".svg": "image/svg+xml" };
      res.writeHead(200, { "Content-Type": mime[extname(file)] ?? "application/octet-stream" }); res.end(data);
    } catch { res.writeHead(502); res.end(); }
  });
  try {
    const project = await p1Project(api, { title: "Native P1 customer delivery" });
    const issue = await api.createIssue("Native overdue delivery task", { project_id: project.id, status: "todo", due_date: "2020-01-01" });
    await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
    const address = server.address(); if (!address || typeof address === "string") throw new Error("Missing native fixture address");
    const origin = `http://127.0.0.1:${address.port}`;
    desktop = await electron.launch({ executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin" ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"), args: [join(root, "e2e/fixtures/changelog-electron.cjs")], env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_E2E_API_URL: origin, CHANGELOG_ELECTRON_RENDERER_URL: origin, CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en-US" } });
    const page = await desktop.firstWindow(); page.on("pageerror", (error) => errors.push(error.message));
    await page.locator("#root > *").first().waitFor({ state: "attached" });
    await p1Authenticate(page, api); await page.reload();
    await page.getByRole("link", { name: "Projects", exact: true }).click();
    await page.getByText("Native P1 customer delivery", { exact: true }).click();
    await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Overdue 1", exact: true }).focus(); await page.keyboard.press("Enter");
    await expect(page.getByRole("link", { name: /Native overdue delivery task/ })).toBeVisible();
    await page.getByRole("button", { name: "Overview", exact: true }).last().click();
    await page.getByRole("button", { name: "Write progress", exact: true }).click();
    const composer = page.locator('[aria-label="Write progress"]');
    await composer.locator('[contenteditable="true"]').fill("Native desktop verified this customer milestone");
    const typedAt = Date.now();
    await composer.getByRole("button", { name: "Preview publication", exact: true }).click();
    expect(Date.now() - typedAt, "Exercise preview before the 300ms editor flush").toBeLessThan(300);
    await expect(composer.getByText("No members will be notified", { exact: true })).toBeVisible();
    // This named timing regression deliberately observes beyond the editor
    // flush deadline: a late flush must not replace an already reviewed preview.
    await page.waitForTimeout(350);
    await expect(composer.getByRole("button", { name: "Publish", exact: true })).toBeVisible();
    await expect(composer.getByRole("button", { name: "Preview publication", exact: true })).toHaveCount(0);
    await composer.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(composer).toHaveCount(0);
    await expect(page.getByText("Native desktop verified this customer milestone", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Revision history", exact: true }).click();
    await p1Capture(page, info, "electron-p1-progress-history");
    expect((await p1Overview(api, project.id)).statistics.counts.total).toBe(1);
    expect(await api.countIssueDispatches(issue.id)).toBe(0);
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.context().addInitScript(() => localStorage.setItem("multica-locale", "zh-Hans"));
    await page.reload();
    await desktop.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0]!.setContentSize(680, 900));
    await expect(page.getByRole("button", { name: "记录进展", exact: true })).toBeVisible();
    await p1NoOverflow(page); await p1Capture(page, info, "electron-p1-chinese-compact");
    expect(await desktop.evaluate(() => (globalThis as unknown as { changelogAcceptance: { daemonStarts: number } }).changelogAcceptance.daemonStarts)).toBe(0);
    expect(errors).toEqual([]);
  } finally {
    await desktop?.close(); await new Promise<void>((done) => server.close(() => done()));
    await api.deleteFeatureWorkspace(workspace.id); await rm(profile, { recursive: true, force: true });
  }
});
