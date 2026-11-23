import { spawnSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import pg from "pg";
import { test, expect } from "@playwright/test";

// CLI/service tests own the concurrency, history and failure matrices. This
// opt-in test owns the real first-login browser journey on a fresh task database.
test("initialized administrator changes the temporary password before entering administration", async ({ page, playwright }) => {
  test.skip(process.env.E2E_ADMIN_AUTO_INIT !== "1", "Requires a fresh isolated administrator-initialization deployment");
  test.setTimeout(120_000);
  const databaseURL = process.env.DATABASE_URL;
  const binary = process.env.E2E_PASSWORD_SERVER_BINARY;
  const apiURL = process.env.NEXT_PUBLIC_API_URL;
  expect(databaseURL && binary && apiURL, "Explicit task database, server binary and API are required").toBeTruthy();
  for (const value of [databaseURL!, apiURL!]) {
    expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
  }
  expect(new URL(databaseURL!).pathname).toMatch(/admin[_-](auto[_-])?init/);
  const database = new pg.Client(databaseURL);
  await database.connect();
  const directory = await mkdtemp(join(tmpdir(), "multica-admin-init-e2e-"));
  const credentialsFile = join(directory, "admin-initial-password.txt");
  const initialize = () => {
    const result = spawnSync(binary!, ["platform-admin", "initialize", "--credentials-file", credentialsFile, "--reason", "Isolated browser acceptance"], {
      env: process.env, encoding: "utf8", timeout: 60_000,
    });
    expect(result.status, result.stderr).toBe(0);
    return JSON.parse(result.stdout) as { status: string; user_id: string; credentials_file?: string };
  };
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  const anonymous = await playwright.request.newContext();
  try {
    const config = await (await anonymous.get(`${apiURL}/api/config`)).json();
    expect(config.auth_mode).toBe("password");
    const existing = await database.query("SELECT count(*) FROM platform_admin_initialization");
    expect(existing.rows[0].count, "Use a fresh task database; never reset an existing administrator").toBe("0");

    const first = initialize();
    expect(first.status).toBe("initialized");
    expect((await stat(credentialsFile)).mode & 0o777).toBe(0o600);
    const credentials = JSON.parse(await readFile(credentialsFile, "utf8")) as { username: string; initial_password: string };
    expect(credentials.username).toBe("platform_admin");
    expect(typeof credentials.initial_password).toBe("string");
    const before = await database.query("SELECT password_hash, session_version FROM user_password_credential WHERE user_id=$1", [first.user_id]);

    const destination = "/admin?source=initialization";
    await page.goto(destination);
    await expect(page.locator("#password-username")).toBeVisible();
    await page.locator("#password-username").fill(credentials.username);
    await page.locator("#password-value").fill(credentials.initial_password);
    const login = page.waitForResponse(response => response.url().endsWith("/auth/login") && response.request().method() === "POST");
    await page.locator('button[type="submit"]').click();
    expect((await login).status()).toBe(200);
    await expect(page.locator("#password-current")).toBeVisible();
    const restrictedState = await page.context().storageState();
    for (const path of ["/api/admin/me", "/api/workspaces"]) {
      expect((await page.request.get(path)).status()).toBe(403);
    }
    expect((await page.request.post("/api/cli-token")).status()).toBe(403);

    await page.locator("#password-current").fill(credentials.initial_password);
    await page.locator("#password-value").fill(credentials.initial_password);
    const rejected = page.waitForResponse(response => response.url().endsWith("/api/me/password/change") && response.request().method() === "POST");
    await page.locator('button[type="submit"]').click();
    const rejection = await rejected;
    expect(rejection.status()).toBe(400);
    expect((await rejection.json()).code).toBe("password_unchanged");
    await expect(page.locator("#password-error")).toHaveAttribute("role", "alert");
    await expect(page.locator("#password-error")).toContainText("Choose a password different from your temporary password.");

    const finalPassword = `Acceptance-${randomUUID()}`;
    await page.locator("#password-value").fill(finalPassword);
    const changed = page.waitForResponse(response => response.url().endsWith("/api/me/password/change") && response.request().method() === "POST");
    await page.locator('button[type="submit"]').click();
    expect((await changed).status()).toBe(200);
    await expect.poll(() => new URL(page.url()).pathname + new URL(page.url()).search).toBe(destination);
    await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
    const identity = await page.request.get("/api/admin/me");
    expect(identity.status()).toBe(200);
    expect(await identity.json()).toMatchObject({ user_id: first.user_id, role: "super_admin" });
    await page.goto("/admin/users");
    await expect(page.getByRole("heading", { name: "Accounts", exact: true })).toBeVisible();

    const oldSession = await playwright.request.newContext({ storageState: restrictedState });
    try {
      expect((await oldSession.get(`${apiURL}/api/me`)).status()).toBe(401);
    } finally {
      await oldSession.dispose();
    }
    expect((await anonymous.post(`${apiURL}/auth/login`, { data: { username: credentials.username, password: credentials.initial_password } })).status()).toBe(401);
    const after = await database.query("SELECT password_hash, session_version, must_change_password FROM user_password_credential WHERE user_id=$1", [first.user_id]);
    expect(after.rows[0].password_hash !== before.rows[0].password_hash).toBe(true);
    expect(Number(after.rows[0].session_version)).toBeGreaterThan(Number(before.rows[0].session_version));
    expect(after.rows[0].must_change_password).toBe(false);

    await rm(credentialsFile);
    const repeated = initialize();
    expect(repeated).toMatchObject({ status: "already_initialized", user_id: first.user_id });
    expect(repeated.credentials_file).toBeUndefined();
    const retained = await database.query("SELECT password_hash, session_version FROM user_password_credential WHERE user_id=$1", [first.user_id]);
    expect(retained.rows[0].password_hash === after.rows[0].password_hash).toBe(true);
    expect(retained.rows[0].session_version).toBe(after.rows[0].session_version);
    expect((await anonymous.post(`${apiURL}/auth/login`, { data: { username: credentials.username, password: finalPassword } })).status()).toBe(200);
    expect((await database.query("SELECT count(*) FROM platform_admin_initialization")).rows[0].count).toBe("1");
    expect(errors).toEqual([]);
  } finally {
    await anonymous.dispose();
    await database.end();
    await rm(directory, { recursive: true, force: true });
  }
});
