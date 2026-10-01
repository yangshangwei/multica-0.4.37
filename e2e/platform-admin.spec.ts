import { randomUUID } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const reportDirectory = resolve(".omx/reports/platform-admin");

function authVersion(api: TestApiClient): number {
  const payload = api.getToken()?.split(".")[1];
  if (!payload) throw new Error("Password fixture did not receive a JWT");
  const claims = JSON.parse(Buffer.from(payload, "base64url").toString("utf8"));
  if (!Number.isSafeInteger(claims.auth_version) || claims.auth_version < 1) {
    throw new Error("Password fixture JWT has no valid auth_version");
  }
  return claims.auth_version;
}

async function passwordLogin(page: Page, username: string, password: string) {
  await expect(page.locator("#password-username")).toBeVisible();
  await page.locator("#password-username").fill(username);
  await page.locator("#password-value").fill(password);
  const response = page.waitForResponse((request) => request.url().endsWith("/auth/login") && request.request().method() === "POST");
  await page.locator('button[type="submit"]').click();
  expect((await response).status()).toBe(200);
}

async function screenshot(page: Page, name: string) {
  await page.screenshot({ path: resolve(reportDirectory, `${name}.png`), fullPage: true });
  await writeFile(resolve(reportDirectory, `${name}.aria.yml`), await page.locator("body").ariaSnapshot());
}

test.describe("platform administration access", () => {
  test.skip(process.env.E2E_PLATFORM_ADMIN !== "1", "Requires a task-owned password deployment and bootstrapped administrator");

  test("zero-workspace cookie access, ordinary-user denial, and live role revocation", async ({ page, browser }) => {
    test.setTimeout(120_000);
    const apiBase = process.env.NEXT_PUBLIC_API_URL;
    const webBase = process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN;
    const administratorUsername = process.env.E2E_PLATFORM_ADMIN_USERNAME;
    const administratorPassword = process.env.E2E_PLATFORM_ADMIN_PASSWORD;
    expect(apiBase && webBase && administratorUsername && administratorPassword, "Explicit task fixture environment is required").toBeTruthy();
    for (const url of [apiBase!, webBase!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(url).hostname);
    expect((await (await page.request.get(`${apiBase}/api/config`)).json()).auth_mode).toBe("password");

    const administrator = new TestApiClient();
    const adminUser = await administrator.loginPassword(administratorUsername!, administratorPassword!);
    expect(await administrator.getWorkspaces()).toEqual([]);
    expect(await administrator.requestJSON("/api/admin/me")).toMatchObject({ user_id: adminUser.id, role: "super_admin", supported: true });

    const candidate = new TestApiClient();
    const username = `pa${randomUUID().replaceAll("-", "").slice(0, 20)}`;
    const password = `test-admin-${randomUUID()}`;
    let candidateUser: { id: string } | undefined;
    let observerAssigned = false;
    let candidateVersion = 0;
    const ordinaryContext = await browser.newContext({ locale: "en-US", viewport: { width: 1440, height: 900 } });
    const ordinaryPage = await ordinaryContext.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    ordinaryPage.on("pageerror", (error) => errors.push(error.message));
    const changeRole = (role: "platform_observer" | null, expectedRole: "platform_observer" | null) =>
      administrator.requestJSON(`/api/admin/users/${candidateUser!.id}/role`, {
        method: "POST",
        headers: { "Idempotency-Key": randomUUID() },
        body: {
          role, expected_role: expectedRole, expected_auth_version: candidateVersion,
          password: administratorPassword, reason: "Isolated platform administration browser acceptance",
        },
      });

    try {
      await mkdir(reportDirectory, { recursive: true });
      candidateUser = await candidate.registerPassword(username, password, "Platform access acceptance");
      candidateVersion = authVersion(candidate);
      expect(await candidate.getWorkspaces()).toEqual([]);

      await ordinaryPage.goto(`${webBase}/admin`);
      await expect.poll(() => new URL(ordinaryPage.url()).pathname).toBe("/login");
      expect(new URL(ordinaryPage.url()).searchParams.get("next")).toBe("/admin");
      await passwordLogin(ordinaryPage, username, password);
      await expect(ordinaryPage.getByRole("heading", { name: "Administration access denied" })).toBeVisible();
      expect((await ordinaryPage.request.get(`${webBase}/api/admin/me`)).status()).toBe(403);
      expect((await ordinaryPage.request.get(`${webBase}/api/me`)).status()).toBe(200);
      await screenshot(ordinaryPage, "ordinary-user-denied");

      const destination = "/admin?status=queued&time_from=2026-10-01#scope";
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`${webBase}${destination}`);
      await expect.poll(() => new URL(page.url()).pathname).toBe("/login");
      expect(new URL(page.url()).searchParams.get("next")).toBe(destination);
      await expect(page.locator("#password-username")).toBeVisible();
      await screenshot(page, "login-reference");
      await passwordLogin(page, administratorUsername!, administratorPassword!);
      await expect(page).toHaveURL(`${webBase}${destination}`);
      await expect(page.getByRole("heading", { name: "Administration access", exact: true })).toBeVisible();
      await expect(page.getByText("super_admin", { exact: true })).toBeVisible();
      const cookieProbe = await page.request.get(`${webBase}/api/admin/me`);
      expect(cookieProbe.status()).toBe(200);
      const currentIdentity = await cookieProbe.json();
      expect(currentIdentity).toMatchObject({ user_id: adminUser.id, role: "super_admin", supported: true });
      expect((await page.context().cookies()).some((cookie) => cookie.httpOnly && cookie.name === "multica_auth")).toBe(true);
      expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();
      await expect(page.getByText(currentIdentity.organization_id, { exact: true })).toBeVisible();
      expect(await administrator.getWorkspaces()).toEqual([]);
      await screenshot(page, "admin-desktop");
      await page.setViewportSize({ width: 390, height: 844 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await screenshot(page, "admin-mobile");

      await changeRole("platform_observer", null);
      observerAssigned = true;
      candidateVersion += 1;
      // Role grants revoke the pre-promotion session. The observer must log in
      // again using the real password flow before administration becomes visible.
      expect((await ordinaryPage.request.get(`${webBase}/api/me`)).status()).toBe(401);
      await candidate.loginPassword(username, password);
      candidateVersion = authVersion(candidate);
      await ordinaryPage.goto(`${webBase}/admin`);
      await expect.poll(() => new URL(ordinaryPage.url()).pathname).toBe("/login");
      await passwordLogin(ordinaryPage, username, password);
      await expect(ordinaryPage.getByText("platform_observer", { exact: true })).toBeVisible();
      await expect(ordinaryPage.getByText(currentIdentity.organization_id, { exact: true })).toBeVisible();
      await screenshot(ordinaryPage, "observer-authorized");

      await ordinaryPage.bringToFront();
      await changeRole(null, "platform_observer");
      observerAssigned = false;
      // Let the actual foreground polling interval observe revocation; do not
      // reload or fake an API response to hide a retained authorization cache.
      await expect(ordinaryPage.getByRole("heading", { name: "Administration access denied" })).toBeVisible({ timeout: 25_000 });
      await expect(ordinaryPage.getByText("platform_observer", { exact: true })).toHaveCount(0);
      await expect(ordinaryPage.getByText(currentIdentity.organization_id, { exact: true })).toHaveCount(0);
      expect((await ordinaryPage.request.get(`${webBase}/api/me`)).status()).toBe(200);
      expect((await ordinaryPage.request.get(`${webBase}/api/admin/me`)).status()).toBe(403);
      await expect(ordinaryPage).toHaveURL(`${webBase}/admin`);
      await screenshot(ordinaryPage, "observer-revoked");
      expect(errors).toEqual([]);
    } finally {
      await ordinaryContext.close();
      if (observerAssigned) await changeRole(null, "platform_observer");
      if (candidateUser) await candidate.deletePasswordAccount(username);
    }
  });
});
