import { randomUUID } from "node:crypto";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("device login authenticates a distinct user without granting another user's workspace", async ({ page, request, baseURL }) => {
  test.skip(process.env.E2E_DEVICE_AUTH !== "1", "Requires a dedicated API with MULTICA_DEVICE_AUTH_ENABLED=true");

  const config = await request.get("/api/config");
  expect(config.ok()).toBe(true);
  expect(await config.json()).toMatchObject({ device_auth_available: true });

  const api = new TestApiClient();
  const slug = `e2e-device-${randomUUID().slice(0, 12)}`;
  const owner = await api.login(`${slug}@example.invalid`, "Device auth workspace owner");
  const workspace = await api.ensureWorkspace("Private device auth regression", slug);
  try {
    if (!baseURL) throw new Error("Device auth fixture requires a Web base URL");
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: baseURL }]);
    const deviceResponse = page.waitForResponse((response) =>
      new URL(response.url()).pathname === "/auth/device" && response.request().method() === "POST",
    );
    await page.goto(`/${slug}/issues`);
    const response = await deviceResponse;
    expect(response.status()).toBe(200);
    const login = await response.json();
    expect(login.user.id).toEqual(expect.any(String));
    expect(login.user.id).not.toBe(owner.user.id);
    expect(login.token).toEqual(expect.any(String));
    await expect(page).toHaveURL("/onboarding");

    // First-use onboarding precedes workspace access. Complete that separate
    // gate through the real API, then revisit the other user's workspace.
    const completed = await request.post("/api/me/onboarding/complete", {
      headers: { Authorization: `Bearer ${login.token}` },
      data: { completion_path: "runtime_skipped" },
    });
    expect(completed.ok()).toBe(true);
    await page.goto(`/${slug}/issues`);
    await expect(page.getByRole("heading", { name: "Workspace not available", exact: true })).toBeVisible();
    await expect(page).toHaveURL(`/${slug}/issues`);

    const denied = await request.get(`/api/workspaces/${workspace.id}`, {
      headers: { Authorization: `Bearer ${login.token}` },
    });
    // Non-members receive 404 so workspace identities cannot be enumerated.
    expect(denied.status()).toBe(404);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
