import { createServer } from "node:http";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import { _electron as electron, expect, test, type ElectronApplication } from "@playwright/test";

// The real built renderer, preload and disk storage run in separate Electron
// processes. The local API and existing native-service fixture isolate this
// persistence test from user accounts, CLI profiles and running agents.
test("Desktop remembers password login across restart and rejects a revoked session", async ({}) => {
  test.setTimeout(90_000);
  const root = resolve(import.meta.dirname, "..");
  const renderer = join(root, "apps/desktop/out/renderer");
  const profile = await mkdtemp(join(tmpdir(), "multica-remember-login-"));
  const token = "isolated-remembered-session";
  const user = {
    id: "00000000-0000-4000-8000-000000000001", username: "11052", name: "Remembered user",
    email: "", avatar_url: null, onboarded_at: "2026-10-01T00:00:00Z",
    requires_account_setup: false, requires_password_change: false,
  };
  let loginRequests = 0;
  let revoked = false;
  const server = createServer(async (req, res) => {
    const pathname = new URL(req.url ?? "/", "http://localhost").pathname;
    const json = (status: number, body: unknown) => {
      res.writeHead(status, { "Content-Type": "application/json" });
      res.end(JSON.stringify(body));
    };
    if (pathname === "/api/config") return json(200, {
      auth_mode: "password", password_auth_available: true, password_signup_available: true,
      allow_signup: true, device_auth_available: false,
    });
    if (pathname === "/auth/login") {
      loginRequests += 1;
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString());
      return body.username === user.username && body.password === "abc123"
        ? json(200, { token, user }) : json(401, { code: "invalid_credentials" });
    }
    if (pathname === "/api/me") return req.headers.authorization === `Bearer ${token}` && !revoked
      ? json(200, user) : json(401, { error: "Session revoked" });
    if (pathname === "/api/workspaces" || pathname === "/api/invitations") return json(200, []);
    if (pathname.startsWith("/api/")) return json(404, { error: "Outside this session fixture" });
    const file = resolve(renderer, `.${pathname === "/" ? "/index.html" : pathname}`);
    if (!file.startsWith(`${renderer}/`)) return json(404, {});
    try {
      const data = await readFile(file);
      const types: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".woff2": "font/woff2", ".svg": "image/svg+xml" };
      res.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream" });
      res.end(data);
    } catch { json(404, {}); }
  });
  await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Missing local fixture address");
  const origin = `http://127.0.0.1:${address.port}`;
  const executablePath = join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin"
    ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron");
  const options = {
    executablePath,
    args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
    env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_E2E_API_URL: origin, CHANGELOG_ELECTRON_RENDERER_URL: origin, CHANGELOG_ELECTRON_SYSTEM_LOCALE: "zh-CN" },
  };
  let desktop: ElectronApplication | undefined;
  const sessions = () => desktop!.evaluate(() => (globalThis as unknown as {
    changelogAcceptance: { authSessions: (string | null)[] };
  }).changelogAcceptance.authSessions);
  try {
    desktop = await electron.launch(options);
    let page = await desktop.firstWindow();
    await page.locator("#password-username").fill(user.username);
    await page.locator("#password-value").fill("abc123");
    await page.locator('button[type="submit"]').click();
    await expect.poll(sessions).toContain(user.id);
    expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBe(token);
    expect(await page.evaluate(() => Object.values(localStorage))).not.toContain("abc123");
    await desktop.close();

    desktop = await electron.launch(options);
    page = await desktop.firstWindow();
    await expect.poll(async () => [...new Set(await sessions())]).toEqual([user.id]);
    await expect(page.locator("#password-username")).toHaveCount(0);
    expect(loginRequests).toBe(1);
    expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBe(token);
    await desktop.close();

    revoked = true;
    desktop = await electron.launch(options);
    page = await desktop.firstWindow();
    await expect(page.locator("#password-username")).toBeVisible();
    await expect.poll(async () => [...new Set(await sessions())]).toEqual([null]);
    expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();
    expect(loginRequests).toBe(1);
  } finally {
    await desktop?.close();
    server.closeAllConnections();
    await new Promise<void>((done) => server.close(() => done()));
    await rm(profile, { recursive: true, force: true });
  }
});
