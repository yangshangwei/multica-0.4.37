import { createServer, request as httpRequest } from "node:http";
import type { Duplex } from "node:stream";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import { _electron as electron, expect, test as base, type ElectronApplication, type Page } from "@playwright/test";
import { p1Authenticate, p1Failure, p1Session } from "./project-p1";

type NativeSession = Awaited<ReturnType<typeof p1Session>> & { page: Page; desktop: ElectronApplication };
export const test = base.extend<{ native: NativeSession }>({
  native: async ({}, use, info) => {
    const root = resolve(import.meta.dirname, "../..");
    const desktopOutput = process.env.MULTICA_E2E_DESKTOP_OUTPUT_DIR ?? join(root, "apps/desktop/out");
    const renderer = join(desktopOutput, "renderer");
    const apiBase = process.env.NEXT_PUBLIC_API_URL!;
    expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
    const profile = await mkdtemp(join(tmpdir(), "multica-project-p1-desktop-"));
    const session = await p1Session();
    let desktop: ElectronApplication | undefined;
    let page: Page | undefined;
    const errors: string[] = [];
    const upgradedSockets = new Set<Duplex>();
    const server = createServer(async (req, res) => {
      try {
        const pathname = new URL(req.url ?? "/", "http://localhost").pathname;
        if (pathname.startsWith("/api/") || pathname.startsWith("/auth/") || pathname === "/health") {
          const chunks: Buffer[] = [];
          for await (const chunk of req) chunks.push(chunk);
          const headers = new Headers();
          for (const key of ["content-type", "authorization", "x-workspace-id", "x-workspace-slug", "accept-language"]) {
            const value = req.headers[key]; if (typeof value === "string") headers.set(key, value);
          }
          const response = await fetch(`${apiBase}${req.url}`, { method: req.method, headers, body: chunks.length ? Buffer.concat(chunks) : undefined });
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
    // Preserve the browser-facing Host/Origin as a real reverse proxy does.
    // Otherwise native flows silently lose all realtime coverage at this fixture.
    server.on("upgrade", (req, socket, head) => {
      const incoming = new URL(req.url ?? "/", "http://localhost");
      if (incoming.pathname !== "/ws") { socket.destroy(); return; }
      const target = new URL(apiBase);
      target.pathname = "/ws";
      target.search = incoming.search;
      upgradedSockets.add(socket);
      const upstream = httpRequest(target, { headers: req.headers });
      socket.on("error", () => upstream.destroy());
      socket.on("close", () => { upgradedSockets.delete(socket); upstream.destroy(); });
      upstream.on("error", () => socket.destroy());
      upstream.on("response", response => { response.resume(); socket.destroy(); });
      upstream.on("upgrade", (response, peer, peerHead) => {
        upgradedSockets.add(peer);
        peer.on("error", () => socket.destroy());
        peer.on("close", () => { upgradedSockets.delete(peer); socket.destroy(); });
        socket.on("close", () => peer.destroy());
        const headers = [];
        for (let i = 0; i < response.rawHeaders.length; i += 2) headers.push(`${response.rawHeaders[i]}: ${response.rawHeaders[i + 1]}`);
        socket.write(`HTTP/1.1 ${response.statusCode} ${response.statusMessage}\r\n${headers.join("\r\n")}\r\n\r\n`);
        if (peerHead.length) socket.write(peerHead);
        if (head.length) peer.write(head);
        socket.pipe(peer); peer.pipe(socket);
      });
      upstream.end();
    });
    try {
      await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
      const address = server.address();
      if (!address || typeof address === "string") throw new Error("Missing native fixture address");
      const origin = `http://127.0.0.1:${address.port}`;
      desktop = await electron.launch({
        executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin" ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"),
        args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
        env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_E2E_API_URL: origin, CHANGELOG_ELECTRON_RENDERER_URL: origin, CHANGELOG_ELECTRON_PRELOAD: join(desktopOutput, "preload/index.js"), CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en-US" },
      });
      page = await desktop.firstWindow();
      page.on("pageerror", (error) => errors.push(error.message));
      await page.locator("#root > *").first().waitFor({ state: "attached" });
      await p1Authenticate(page, session.api); await page.reload();
      await use({ ...session, page, desktop });
      expect(errors).toEqual([]);
      expect(await desktop.evaluate(() => (globalThis as unknown as { changelogAcceptance: { daemonStarts: number } }).changelogAcceptance.daemonStarts)).toBe(0);
    } finally {
      try { if (page && info.status !== info.expectedStatus) await p1Failure(page, info); }
      finally {
        await desktop?.close();
        for (const socket of upgradedSockets) socket.destroy();
        await new Promise<void>((done) => server.close(() => done()));
        try { await session.api.deleteFeatureWorkspace(session.workspace.id); }
        finally { await rm(profile, { recursive: true, force: true }); }
      }
    }
  },
});

export async function openNativeProject(page: Page, title: string) {
  await page.getByRole("link", { name: "Projects", exact: true }).and(page.locator('[data-sidebar="menu-button"]')).click();
  await page.getByText(title, { exact: true }).click();
  await page.getByRole("heading", { name: "Overview", exact: true }).waitFor();
}
