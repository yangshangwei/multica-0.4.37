import { randomUUID } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

async function login(page: Page, username: string, password: string) {
  await expect(page.locator("#password-username")).toBeVisible();
  await page.locator("#password-username").fill(username);
  await page.locator("#password-value").fill(password);
  await page.locator('button[type="submit"]').click();
}

async function capture(page: Page, name: string) {
  const directory = resolve(".omx/reports/platform-admin-accounts");
  await mkdir(directory, { recursive: true });
  await page.screenshot({ path: resolve(directory, `${name}.png`), fullPage: true });
  await writeFile(resolve(directory, `${name}.aria.yml`), await page.locator("body").ariaSnapshot());
}

test.describe("platform account administration", () => {
  test.skip(process.env.E2E_PLATFORM_ADMIN !== "1", "Requires a task-owned password-mode production build");

  test("account controls, lost response recovery, forced password change and observer UI", async ({ page, browser }) => {
    test.setTimeout(150_000);
    const web = process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN;
    const api = process.env.NEXT_PUBLIC_API_URL;
    const adminUsername = process.env.E2E_PLATFORM_ADMIN_USERNAME;
    const adminPassword = process.env.E2E_PLATFORM_ADMIN_PASSWORD;
    expect(web && api && adminUsername && adminPassword).toBeTruthy();
    for (const url of [web!, api!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(url).hostname);
    const administrator = new TestApiClient();
    await administrator.loginPassword(adminUsername!, adminPassword!);
    const candidate = new TestApiClient();
    const username = `s05${randomUUID().replaceAll("-", "").slice(0, 20)}`;
    const name = `Account acceptance ${randomUUID().slice(0, 8)}`;
    const originalPassword = `fixture-original-${randomUUID()}`;
    const temporaryPassword = `fixture-temporary-${randomUUID()}`;
    const personalPassword = `fixture-personal-${randomUUID()}`;
    const user = await candidate.registerPassword(username, originalPassword, name);
    let roleAssigned = false;
    const observerContext = await browser.newContext({ locale: "en-US" });
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`${web}/admin/users?q=${username}`);
      await login(page, adminUsername!, adminPassword!);
      await expect(page.getByRole("link", { name, exact: true })).toBeVisible();
      await capture(page, "accounts-desktop");
      await page.setViewportSize({ width: 390, height: 844 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await capture(page, "accounts-mobile");
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.getByRole("link", { name, exact: true }).click();
      await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
      await capture(page, "account-detail");

      await page.getByRole("button", { name: "Disable account", exact: true }).click();
      await page.getByLabel("Reason", { exact: true }).fill("Synthetic account access review");
      await page.getByLabel("Your current password", { exact: true }).fill("incorrect-password");
      await page.getByRole("button", { name: "Confirm change", exact: true }).click();
      await expect(page.getByRole("alert").filter({ hasText: "Your password could not be verified" })).toBeVisible();
      await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
      await expect(page.getByLabel("Your current password", { exact: true })).toHaveValue("");

      let committedKey = "";
      let disableWrites = 0;
      await page.route(`**/api/admin/users/${user.id}/disable`, async (route) => {
        disableWrites += 1;
        committedKey = route.request().headers()["idempotency-key"] ?? "";
        const response = await route.fetch();
        expect(response.status()).toBe(200);
        // Drop only the already-committed HTTP response. The app must look up
        // the same key instead of silently issuing another mutation.
        await route.abort("failed");
      });
      await page.getByLabel("Your current password", { exact: true }).fill(adminPassword!);
      await page.getByRole("button", { name: "Confirm change", exact: true }).click();
      await expect(page.getByText("The change was applied.", { exact: true })).toBeVisible();
      expect(disableWrites).toBe(1);
      expect(committedKey).not.toBe("");
      const recovered = await administrator.requestJSON<{ items: Array<{ target_id: string; kind: string }> }>(`/api/admin/operations?idempotency_key=${committedKey}`);
      expect(recovered.items).toHaveLength(1);
      expect(recovered.items[0]).toMatchObject({ target_id: user.id, kind: "user.disable" });
      await page.unroute(`**/api/admin/users/${user.id}/disable`);
      expect((await page.request.get(`${api}/api/me`, { headers: { Authorization: `Bearer ${candidate.getToken()}` } })).status()).toBe(401);
      await capture(page, "account-disabled-reconciled");
      await page.getByRole("button", { name: "Close confirmation", exact: true }).click();

      await page.getByRole("button", { name: "Restore account", exact: true }).click();
      await page.getByLabel("Reason", { exact: true }).fill("Restore synthetic account after access review");
      await page.getByLabel("Your current password", { exact: true }).fill(adminPassword!);
      await page.getByRole("button", { name: "Confirm change", exact: true }).click();
      await expect(page.getByText("The change was applied.", { exact: true })).toBeVisible();
      expect((await page.request.get(`${api}/api/me`, { headers: { Authorization: `Bearer ${candidate.getToken()}` } })).status()).toBe(401);
      await page.getByRole("button", { name: "Close confirmation", exact: true }).click();

      await page.getByRole("button", { name: "Reset password", exact: true }).click();
      await page.getByLabel("Reason", { exact: true }).fill("Synthetic password reset acceptance");
      await page.getByLabel("Target account's temporary password", { exact: true }).fill(temporaryPassword);
      await page.getByLabel("Confirm temporary password", { exact: true }).fill(temporaryPassword);
      await page.getByLabel("Your current administrator password", { exact: true }).fill(adminPassword!);
      await page.locator('button[type="submit"]').filter({ hasText: "Reset password" }).click();
      await expect(page.getByText("The password was reset.", { exact: true })).toBeVisible();
      await candidate.loginPassword(username, temporaryPassword);
      expect((await page.request.get(`${api}/api/workspaces`, { headers: { Authorization: `Bearer ${candidate.getToken()}` } })).status()).toBe(403);
      await candidate.requestJSON("/api/me/password/change", { method: "POST", body: { current_password: temporaryPassword, new_password: personalPassword } });
      await candidate.loginPassword(username, personalPassword);
      await page.getByRole("button", { name: "Close confirmation", exact: true }).click();
      await page.reload();

      await page.getByRole("button", { name: "Change platform role", exact: true }).click();
      await page.getByRole("combobox", { name: "Platform role", exact: true }).selectOption("platform_observer");
      await page.getByLabel("Reason", { exact: true }).fill("Read-only administration acceptance");
      await page.getByLabel("Your current password", { exact: true }).fill(adminPassword!);
      await page.getByRole("button", { name: "Confirm change", exact: true }).click();
      await expect(page.getByText("The change was applied.", { exact: true })).toBeVisible();
      roleAssigned = true;
      await candidate.loginPassword(username, personalPassword);
      await page.goto(`${web}/admin/administrators?q=${username}`);
      await expect(page.getByRole("link", { name, exact: true })).toBeVisible();
      await capture(page, "administrators");

      const observer = await observerContext.newPage();
      await observer.goto(`${web}/admin/users/${user.id}`);
      await login(observer, username, personalPassword);
      await expect(observer.getByText("Only a super administrator can reset another account's password. Your access is read-only.")).toBeVisible();
      await expect(observer.getByRole("button", { name: "Reset password", exact: true })).toBeDisabled();
      await expect(observer.getByRole("button", { name: "Disable account", exact: true })).toHaveCount(0);
      await capture(observer, "observer-account-readonly");
      expect(errors).toEqual([]);
    } finally {
      await observerContext.close();
      if (roleAssigned) {
        const detail = await administrator.requestJSON<{ user: { auth_version: number } }>(`/api/admin/users/${user.id}`);
        await administrator.requestJSON(`/api/admin/users/${user.id}/role`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { role: null, expected_role: "platform_observer", expected_auth_version: detail.user.auth_version, password: adminPassword, reason: "Remove synthetic browser-test access" } });
      }
      await candidate.deletePasswordAccount(username);
    }
  });
});
