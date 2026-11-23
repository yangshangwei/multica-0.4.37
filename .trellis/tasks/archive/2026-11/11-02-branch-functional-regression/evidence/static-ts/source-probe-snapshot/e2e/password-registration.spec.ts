import { randomUUID } from "node:crypto";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test.describe("intranet password registration", () => {
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires isolated password-mode deployment");

  test("confirmed password registers without email and a second device keeps the same identity", async ({ page, browser }, info) => {
    test.setTimeout(120_000);
    const apiBase = process.env.NEXT_PUBLIC_API_URL!;
    expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
    const config = await (await page.request.get(`${apiBase}/api/config`)).json();
    expect(config.auth_mode).toBe("password");
    const username = `pw${randomUUID().replaceAll("-", "").slice(0, 20)}`;
    const password = "test horse battery staple 2026";
    const api = new TestApiClient();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const second = await browser.newContext();
    let workspace: {id: string; slug: string} | undefined;
    try {
      await page.goto("/login");
      await expect(page.locator("#password-username")).toBeVisible();
      await expect(page.locator('input[type="email"]')).toHaveCount(0);
      await page.getByRole("button", { name: /Create account|创建账号/ }).click();
      await expect(page.locator("form input")).toHaveCount(4);
      await page.locator("#password-username").fill(username);
      await page.locator("#password-name").fill("Password E2E");
      await page.locator("#password-value").fill(password);
      await page.locator("#password-confirm").fill(password);
      const registered = page.waitForResponse((r) => r.url().endsWith("/auth/register") && r.request().method() === "POST");
      await page.locator('button[type="submit"]').click();
      const response = await registered;
      expect(response.status()).toBe(201);
      const data = await response.json();
      expect(data.user.email).toBe("");
      expect(data.user.onboarded_at).toBeNull();
      await expect(page).toHaveURL(/\/onboarding/);
      await expect(page.locator("#password-username")).toHaveCount(0);

      const user = await api.loginPassword(username, password);
      expect(user.id).toBe(data.user.id);
      expect(await api.requestJSON("/api/workspaces")).toEqual([]);
      await page.getByRole("button", { name: "Continue on web" }).click();
      await page.getByRole("radio", { name: "Engineer", exact: true }).click();
      await page.getByRole("checkbox", { name: /Code & test with agents/i }).click();
      await page.getByRole("button", { name: "Continue", exact: true }).click();
      await expect(page.getByRole("heading", { name: /Name your workspace/i })).toBeVisible();
      await page.getByRole("textbox").first().fill(`Password ${username}`);
      const creating = page.waitForResponse((r) => r.url().endsWith("/api/workspaces") && r.request().method() === "POST");
      await page.getByRole("button", { name: /^Create / }).click();
      const created = await creating;
      expect(created.status()).toBe(201);
      workspace = await created.json();
      api.setWorkspaceId(workspace!.id);
      api.setWorkspaceSlug(workspace!.slug);
      await page.getByRole("button", { name: "Skip for now", exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`/${workspace!.slug}/projects`), { timeout: 30_000 });
      const completedUser = await api.requestJSON<{onboarded_at: string | null}>("/api/me");
      expect(completedUser.onboarded_at).not.toBeNull();
      const other = await second.newPage();
      await other.goto(`${process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN}/login`);
      await other.locator("#password-username").fill(username);
      await other.locator("#password-value").fill(password);
      const loggedIn = other.waitForResponse((r) => r.url().endsWith("/auth/login") && r.request().method() === "POST");
      await other.locator('button[type="submit"]').click();
      expect((await (await loggedIn).json()).user.id).toBe(user.id);
      await expect(other).toHaveURL(new RegExp(`/${workspace!.slug}/`), { timeout: 30_000 });
      await other.reload();
      await expect(other).toHaveURL(new RegExp(`/${workspace!.slug}/`));
      await expect(other.locator("#password-username")).toHaveCount(0);

      const device = await page.request.post(`${apiBase}/auth/device`, { data: { device_id: "11223344556677889900aabbccddeeff" } });
      expect(device.status()).toBe(403);
      expect(errors).toEqual([]);
      await info.attach("password-welcome", { body: await page.screenshot(), contentType: "image/png" });
    } finally {
      await second.close();
      if (workspace) await api.deleteFeatureWorkspace(workspace.id);
      await api.deletePasswordAccount(username);
    }
  });

  test("confirmation requires an exact match and password visibility stays independent", async ({ page }, info) => {
    const username = `pw${randomUUID().replaceAll("-", "").slice(0, 20)}`;
    const password = " exact password with spaces 2026 ";
    const api = new TestApiClient();
    const registrations: unknown[] = [];
    page.on("request", (request) => {
      if (request.url().endsWith("/auth/register") && request.method() === "POST") {
        registrations.push(request.postDataJSON());
      }
    });
    try {
      await page.goto("/login");
      await page.getByRole("button", { name: /Create account|创建账号/ }).click();
      await page.locator("#password-username").fill(username);
      await page.locator("#password-name").fill("Confirmation E2E");
      await page.locator("#password-value").fill(password);
      await page.locator("#password-confirm").fill(password.trim());
      await page.locator('button[type="submit"]').click();
      await expect(page.locator("#password-confirm")).toBeFocused();
      await expect(page.locator("#password-confirm")).toHaveAttribute("aria-invalid", "true");
      expect(registrations).toEqual([]);
      await page.locator('button[aria-controls="password-value"]').click();
      await expect(page.locator("#password-value")).toHaveAttribute("type", "text");
      await expect(page.locator("#password-confirm")).toHaveAttribute("type", "password");
      await page.locator('button[aria-controls="password-value"]').click();
      await page.locator('button[aria-controls="password-confirm"]').click();
      await expect(page.locator("#password-value")).toHaveAttribute("type", "password");
      await expect(page.locator("#password-confirm")).toHaveAttribute("type", "text");
      await page.locator('button[aria-controls="password-confirm"]').click();
      await page.screenshot({ path: info.outputPath("confirmation-mismatch.png") });
      await page.locator("#password-confirm").fill(password);
      const registered = page.waitForResponse((response) => response.url().endsWith("/auth/register") && response.request().method() === "POST");
      await page.locator('button[type="submit"]').click();
      const response = await registered;
      expect(response.status()).toBe(201);
      const data = await response.json();
      expect(registrations).toEqual([{ username, name: "Confirmation E2E", password }]);
      expect((await api.loginPassword(username, password)).id).toBe(data.user.id);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await api.deletePasswordAccount(username);
    }
  });
});
