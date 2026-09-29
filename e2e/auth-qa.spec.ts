import { randomUUID } from "node:crypto";
import { test, expect } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

test.use({ locale: "en-US", screenshot: "only-on-failure", trace: "retain-on-failure" });

test("email login verifies a real code, restores access after reload, and logout protects the workspace", async ({ page }, info) => {
  test.setTimeout(120_000);
  const suffix = randomUUID().slice(0, 12);
  const email = `e2e-auth-qa-${suffix}@multica.ai`;
  const api = new TestApiClient();
  // Provision an onboarded account. Its API credential is never copied into
  // the browser: the browser must establish its own session through the UI.
  await api.login(email, "Email login QA");
  const workspace = await api.ensureWorkspace("Email QA " + suffix, "email-qa-" + suffix);
  await api.markUserOnboarded();
  try {
    await page.goto(`/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/login(?:\?|$)/, { timeout: 30_000 });
    await expect(page.getByText("Sign in to Multica", { exact: true })).toBeVisible();
    const submit = page.getByRole("button", { name: "Continue", exact: true });
    await expect(submit).toBeDisabled();
    await page.getByRole("textbox", { name: "Email", exact: true }).fill(email);
    const sent = page.waitForResponse((response) => new URL(response.url()).pathname === "/auth/send-code"
      && response.request().method() === "POST");
    await submit.click();
    expect((await sent).ok()).toBe(true);
    await expect(page.getByText("Check your email", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: /Resend in/ })).toBeDisabled();

    if (!process.env.DATABASE_URL) throw new Error("Explicit isolated DATABASE_URL required");
    const db = new pg.Client(process.env.DATABASE_URL);
    await db.connect();
    let code: string;
    try {
      const result = await db.query<{ code: string }>(
        "SELECT code FROM verification_code WHERE email = $1 AND used = FALSE AND expires_at > now() ORDER BY created_at DESC LIMIT 1",
        [email],
      );
      expect(result.rows).toHaveLength(1);
      code = process.env.MULTICA_DEV_VERIFICATION_CODE?.trim() || result.rows[0].code;
    } finally { await db.end(); }
    const verified = page.waitForResponse((response) => new URL(response.url()).pathname === "/auth/verify-code"
      && response.request().method() === "POST");
    await page.locator('input[data-slot="input-otp"]').fill(code);
    expect((await verified).ok()).toBe(true);
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/issues$`), { timeout: 30_000 });
    await expect(page.getByRole("button", { name: "New Issue", exact: true })).toBeVisible();
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "New Issue", exact: true })).toBeVisible();
    await page.screenshot({ path: info.outputPath("email-authenticated.png") });
    await page.getByRole("button", { name: "Email QA " + suffix }).click();
    await page.getByRole("menuitem", { name: "Log out", exact: true }).click();
    await expect(page).toHaveURL(/\/login(?:\?|$)/, { timeout: 30_000 });
    await page.goto(`/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/login(?:\?|$)/, { timeout: 30_000 });
    await expect(page.getByRole("textbox", { name: "Email", exact: true })).toBeVisible();
  } finally { await api.deleteFeatureWorkspace(workspace.id); }
});
