import { createHmac, randomUUID } from "node:crypto";
import { createServer } from "node:http";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import pg from "pg";
import { _electron as electron, expect, test, type ElectronApplication } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("an existing Desktop account binds password login without losing its history", async ({}, info) => {
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires a local password-mode server and migration window");
  test.setTimeout(120_000);
  const apiBase = process.env.NEXT_PUBLIC_API_URL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
  expect(["localhost", "127.0.0.1"]).toContain(new URL(process.env.DATABASE_URL!).hostname);
  expect(process.env.APP_ENV).not.toBe("production");
  const jwtSecret = process.env.JWT_SECRET || "multica-dev-secret-change-in-production";
  const cutoff = Date.parse(process.env.MULTICA_PASSWORD_MIGRATION_CUTOFF!);
  expect(Number.isFinite(cutoff)).toBe(true);
  expect(Date.parse(process.env.MULTICA_PASSWORD_MIGRATION_DEADLINE!)).toBeGreaterThan(Date.now());
  const root = resolve(import.meta.dirname, "..");
  const renderer = join(root, "apps/desktop/out/renderer");
  const profile = await mkdtemp(join(tmpdir(), "multica-legacy-binding-"));
  const api = new TestApiClient();
  const database = new pg.Client(process.env.DATABASE_URL);
  await database.connect();
  const username = `00${Date.now()}`;
  const initialPassword = "oldpass1";
  const finalPassword = "abc123";
  let userID: string | undefined;
  let workspace: { id: string; slug: string } | undefined;
  let savedHash: string | undefined;
  let desktop: ElectronApplication | undefined;
  const proxy = createServer(async (req, res) => {
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
      const types: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".woff2": "font/woff2", ".svg": "image/svg+xml" };
      const data = await readFile(file);
      res.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream" });
      res.end(data);
    } catch { res.writeHead(502); res.end(); }
  });
  await new Promise<void>((done) => proxy.listen(0, "127.0.0.1", done));
  const address = proxy.address();
  if (!address || typeof address === "string") throw new Error("Missing fixture address");
  const origin = `http://127.0.0.1:${address.port}`;
  const options = {
    executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin" ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"),
    args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
    env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_E2E_API_URL: origin, CHANGELOG_ELECTRON_RENDERER_URL: origin, CHANGELOG_ELECTRON_SYSTEM_LOCALE: "zh-CN" },
  };
  try {
    const registration = await fetch(`${apiBase}/auth/register`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, password: initialPassword, name: "老账号演示" }) });
    expect(registration.status).toBe(201);
    const original = await api.loginPassword(username, initialPassword);
    userID = original.id;
    workspace = await api.ensureWorkspace("绑定演示工作空间", `binding-${randomUUID().slice(0, 8)}`);
    const issue = await api.createIssue("绑定前已存在的历史任务");
    expect(issue.id).toBeTruthy();
    const comment = await api.requestJSON<{ id: string; content: string }>(`/api/issues/${issue.id}/comments`, { method: "POST", body: { content: "绑定前的历史评论" } });
    await database.query('UPDATE "user" SET email=$1, onboarded_at=now() WHERE id=$2', [`${username}@example.invalid`, userID]);
    const removed = await database.query<{ password_hash: string }>("DELETE FROM user_password_credential WHERE user_id=$1 RETURNING password_hash", [userID]);
    expect(removed.rowCount).toBe(1);
    savedHash = removed.rows[0].password_hash;

    // Reproduce a pre-migration session only for this newly created fixture user.
    const header = Buffer.from(JSON.stringify({ alg: "HS256", typ: "JWT" })).toString("base64url");
    const claims = Buffer.from(JSON.stringify({ sub: userID, iat: Math.floor(cutoff / 1000) - 60, exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64url");
    const payload = `${header}.${claims}`;
    const legacyToken = `${payload}.${createHmac("sha256", jwtSecret).update(payload).digest("base64url")}`;
    const oldHeaders = { Authorization: `Bearer ${legacyToken}` };
    expect((await (await fetch(`${apiBase}/api/me`, { headers: oldHeaders })).json()).requires_account_setup).toBe(true);
    expect((await fetch(`${apiBase}/api/workspaces`, { headers: oldHeaders })).status).toBe(403);

    desktop = await electron.launch(options);
    let page = await desktop.firstWindow();
    await page.locator("#password-username").waitFor();
    await page.evaluate((token) => localStorage.setItem("multica_token", token), legacyToken);
    await page.reload();
    await expect(page.getByText("设置登录账号", { exact: true }).first()).toBeVisible();
    await expect(page.locator("#password-name")).toHaveValue("老账号演示");
    await page.screenshot({ path: info.outputPath("before-binding.png") });
    await page.locator("#password-username").fill(username);
    await page.locator("#password-value").fill(finalPassword);
    const binding = page.waitForResponse((response) => response.url().endsWith("/api/me/password/setup") && response.request().method() === "POST");
    await page.locator('button[type="submit"]').click();
    const response = await binding;
    expect(response.status()).toBe(200);
    expect((await response.json()).user.id).toBe(userID);
    await expect(page.locator("#password-username")).toHaveCount(0);
    await page.screenshot({ path: info.outputPath("after-binding.png") });
    await expect(page.getByText("绑定演示工作空间", { exact: true }).first()).toBeVisible();
    expect((await fetch(`${apiBase}/api/me`, { headers: oldHeaders })).status).toBe(401);
    const bound = await api.loginPassword(username, finalPassword);
    expect(bound.id).toBe(userID);
    expect((await api.getWorkspaces()).map(item => item.id)).toContain(workspace.id);
    const preserved = await database.query("SELECT issue.id, issue.creator_id, comment.id AS comment_id, comment.content FROM issue JOIN comment ON comment.issue_id=issue.id WHERE issue.id=$1 AND comment.id=$2", [issue.id, comment.id]);
    expect(preserved.rows[0]).toMatchObject({ id: issue.id, creator_id: userID, comment_id: comment.id, content: comment.content });
    await desktop.close();
    desktop = await electron.launch(options);
    page = await desktop.firstWindow();
    await expect(page.getByText("绑定演示工作空间", { exact: true }).first()).toBeVisible();
    await expect(page.locator("#password-username")).toHaveCount(0);
    await page.screenshot({ path: info.outputPath("after-restart.png") });
    await writeFile(info.outputPath("binding-result.json"), JSON.stringify({ userIdBefore: userID, userIdAfter: bound.id, workspaceId: workspace.id, issueId: issue.id, commentId: comment.id, username, oldTokenRevoked: true, passwordLoginPassed: true, restartRestored: true }, null, 2));
  } finally {
    await desktop?.close();
    try {
      if (userID) {
        if (savedHash) await database.query("INSERT INTO user_password_credential (user_id,username,password_hash) VALUES ($1,$2,$3) ON CONFLICT (user_id) DO NOTHING", [userID, username, savedHash]);
        try { await api.loginPassword(username, finalPassword); } catch { await api.loginPassword(username, initialPassword); }
        if (workspace) await api.deleteFeatureWorkspace(workspace.id);
        await api.deletePasswordAccount(username);
      }
    } finally {
      await database.end();
      proxy.closeAllConnections();
      await new Promise<void>((done) => proxy.close(() => done()));
      await rm(profile, { recursive: true, force: true });
    }
  }
});
