import { createHmac, randomUUID } from "node:crypto";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("an obsolete session returns to sign-in and can open the MCP market after a fresh login", async ({ page }, info) => {
  test.setTimeout(120_000);
  const apiBase = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
  const api = new TestApiClient();
  const slug = `session-recovery-${Date.now().toString(36)}-${process.pid}`;
  const email = `${slug}@multica.ai`;
  await api.login(email, "Session recovery tester");
  const workspace = await api.ensureWorkspace("Session recovery", slug);
  await api.markUserOnboarded();
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });

  // A valid local signature for an identity that has never existed in this DB
  // reproduces a cookie carried over from another local development database.
  const encode = (value: unknown) => Buffer.from(JSON.stringify(value)).toString("base64url");
  const now = Math.floor(Date.now() / 1000);
  const payload = `${encode({ alg: "HS256", typ: "JWT" })}.${encode({ sub: randomUUID(), email: "missing-session@example.test", iat: now, exp: now + 300 })}`;
  const secret = process.env.JWT_SECRET || "multica-dev-secret-change-in-production";
  const obsoleteToken = `${payload}.${createHmac("sha256", secret).update(payload).digest("base64url")}`;
  const origin = process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN!;
  await page.context().addCookies([
    { name: "multica_auth", value: obsoleteToken, url: origin, httpOnly: true, sameSite: "Lax" },
    { name: "multica_logged_in", value: "1", url: origin, sameSite: "Lax" },
  ]);
  const authErrors: string[] = [];
  const pageErrors: string[] = [];
  const identityStatuses: number[] = [];
  page.on("console", (message) => {
    if (message.type() === "error" && message.text().includes("auth init")) authErrors.push(message.text());
  });
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("response", (response) => {
    if (new URL(response.url()).pathname === "/api/me" && response.request().method() === "GET") identityStatuses.push(response.status());
  });
  try {
    await page.goto(`/${slug}/mcp`);
    await expect(page).toHaveURL(/\/login(?:\?|$)/);
    await expect(page.getByPlaceholder("you@example.com")).toBeVisible();
    expect(identityStatuses.some((status) => status === 401 || status === 404)).toBe(true);
    await page.getByPlaceholder("you@example.com").fill(email);
    await page.getByRole("button", { name: "Continue", exact: true }).click();
    await page.locator('input[autocomplete="one-time-code"]').fill(process.env.MULTICA_DEV_VERIFICATION_CODE || "888888");
    await expect(page).toHaveURL(new RegExp(`/${slug}/`), { timeout: 30_000 });
    await page.goto(`/${slug}/mcp`);
    await expect(page.getByRole("tab", { name: "MCP market", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "View Playwright", exact: true })).toBeVisible();
    expect(authErrors).toEqual([]);
    expect(pageErrors).toEqual([]);
    expect(identityStatuses.filter((status) => status === 401 || status === 404).length).toBeLessThanOrEqual(2);
    const screenshot = info.outputPath("recovered-mcp-market.png");
    await page.screenshot({ path: screenshot });
    await info.attach("recovered-mcp-market", { path: screenshot, contentType: "image/png" });
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
