import { createServer } from "node:http";
import { randomUUID } from "node:crypto";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import { _electron as electron, expect, test, type ElectronApplication } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("native desktop triage navigation, review and compact history use the shared real backend", async ({}, info) => {
  test.setTimeout(180_000);
  const root = resolve(import.meta.dirname, "..");
  const renderer = join(root, "apps/desktop/out/renderer");
  const apiBase = process.env.NEXT_PUBLIC_API_URL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
  const profile = await mkdtemp(join(tmpdir(), "multica-triage-desktop-"));
  const api = new TestApiClient();
  const slug = `native-triage-${randomUUID().slice(0, 10)}`;
  let workspaceId: string | undefined;
  let desktop: ElectronApplication | undefined;
  const errors: string[] = [];
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
        const response = await fetch(`${apiBase}${req.url}`, { method: req.method, headers, body: chunks.length ? Buffer.concat(chunks) : undefined });
        res.writeHead(response.status, { "Content-Type": response.headers.get("content-type") ?? "application/json" });
        res.end(Buffer.from(await response.arrayBuffer()));
        return;
      }
      const file = resolve(renderer, `.${pathname === "/" ? "/index.html" : pathname}`);
      if (!file.startsWith(`${renderer}/`)) { res.writeHead(404); res.end(); return; }
      const data = await readFile(file);
      const mime: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".woff2": "font/woff2", ".svg": "image/svg+xml" };
      res.writeHead(200, { "Content-Type": mime[extname(file)] ?? "application/octet-stream" });
      res.end(data);
    } catch { res.writeHead(502); res.end(); }
  });
  try {
    await api.login(`${slug}@example.invalid`, "Desktop triage acceptance");
    const workspace = await api.ensureWorkspace("Desktop triage acceptance", slug);
    workspaceId = workspace.id;
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const settings = await api.requestJSON<{ revision: number }>("/api/triage/settings");
    await api.requestJSON("/api/triage/settings", { method: "PUT", body: { enabled: true, acceptance_status: "todo", require_priority: false, responsibility_mode: "none", responsibility_member_id: null, expected_revision: settings.revision } });
    const title = "Native desktop customer feedback";
    const item = await api.requestJSON<{ issue: { id: string } }>("/api/triage/items", { method: "POST", body: { request_id: randomUUID(), title } });
    await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
    const address = server.address();
    if (!address || typeof address === "string") throw new Error("Missing native fixture address");
    const origin = `http://127.0.0.1:${address.port}`;
    desktop = await electron.launch({
      executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin" ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"),
      args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
      env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_E2E_API_URL: origin, CHANGELOG_ELECTRON_RENDERER_URL: origin, CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en-US" },
    });
    const page = await desktop.firstWindow();
    page.on("pageerror", (error) => errors.push(error.message));
    await page.locator("#root > *").first().waitFor({ state: "attached" });
    await page.context().addInitScript((token) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica-locale", "en");
      localStorage.setItem("multica:chat:isOpen", "false");
    }, api.getToken());
    await page.reload();
    await page.getByRole("link", { name: /Triage/ }).click();
    const queue = page.getByRole("region", { name: "Triage queue", exact: true });
    await queue.getByRole("button", { name: new RegExp(title) }).click();
    await expect(page.getByRole("button", { name: "Accept", exact: true })).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    await page.screenshot({ path: info.outputPath("native-triage-review.png"), fullPage: true });
    await page.getByRole("button", { name: "Reject", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("Keep this request for a future milestone");
    await dialog.getByRole("button", { name: "Reject", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const result = await api.requestJSON<{ issue: { admission_status: string } }>(`/api/triage/items/${item.issue.id}`);
    expect(result.issue.admission_status).toBe("rejected");
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
    await page.getByRole("tab", { name: "History", exact: true }).click();
    await expect(page.getByRole("button", { name: new RegExp(title) }).first()).toBeVisible();
    await desktop.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0]!.setContentSize(680, 900));
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.screenshot({ path: info.outputPath("native-triage-history-compact.png"), fullPage: true });
    expect(errors).toEqual([]);
  } finally {
    await desktop?.close();
    await new Promise<void>((done) => server.close(() => done()));
    if (workspaceId) await api.deleteFeatureWorkspace(workspaceId);
    await rm(profile, { recursive: true, force: true });
  }
});
