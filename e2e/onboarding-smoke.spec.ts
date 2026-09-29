import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { waitForPageText } from "./helpers";

// Smoke test for the onboarding flow: welcome → About you (role +
// use case on ONE screen) → workspace → runtime → Projects. The source question
// is intentionally absent — it moved to the workspace source-backfill
// prompt (MUL-5159). Captures screenshots for review. Uses a unique
// email per run so the user is always a fresh, un-onboarded user
// landing on /onboarding.

const EMAIL = `onboarding-v3-${Date.now()}@localhost`;
const SHOTS_DIR = "../shots-rail";

test.use({ viewport: { width: 1440, height: 900 } });

test("onboarding — answer path completes on Projects after skipping runtime", async ({ page }) => {
  const api = new TestApiClient();
  await api.login(EMAIL, "OBv3 Tester");
  const token = api.getToken();
  if (!token) throw new Error("E2E login did not return an auth token");

  await page.addInitScript((t) => {
    localStorage.setItem("multica_token", t);
  }, token);
  await page.goto("/onboarding", { waitUntil: "domcontentloaded" });
  await waitForPageText(page, "Continue on web");

  // 1. Welcome screen
  await expect(page.getByRole("button", { name: "Continue on web" })).toBeVisible({ timeout: 15000 });
  await page.screenshot({ path: `${SHOTS_DIR}/01-welcome.png`, fullPage: false });

  // Click Continue on web to advance to About you
  await page.getByRole("button", { name: "Continue on web" }).click();

  // 2. About you step — both questions live on this one screen and the
  //    source question must NOT exist anywhere in the flow.
  await expect(page.getByText("From SDLC to ADLC. Build with agents.")).toBeVisible({ timeout: 10000 });
  await expect(page.getByText("What's your role in the development process?")).toBeVisible();
  await expect(page.getByText("What would you like to do with agents?")).toBeVisible();
  // The rail names every step and marks the current one; the ordinal
  // counter it replaced is gone.
  await expect(page.locator('[data-slot="stepper-title"]')).toHaveText([
    "About you",
    "Workspace",
    "Connect a runtime",
    "First project",
  ]);
  const projectExit = page.locator('[data-slot="stepper-item"]').filter({
    has: page.getByText("First project", { exact: true }),
  });
  await expect(projectExit).toHaveCount(1);
  await expect(projectExit).not.toHaveAttribute("aria-current", "step");
  await expect(projectExit.locator('button, a, [role="button"]')).toHaveCount(0);
  await projectExit.getByText("First project", { exact: true }).click();
  await expect(page.getByText("From SDLC to ADLC. Build with agents.")).toBeVisible();
  await expect(
    page.locator('[aria-current="step"]').filter({ hasText: "About you" }),
  ).toBeVisible();
  await expect(page.getByText("How did you hear about Multica?")).toHaveCount(0);
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${SHOTS_DIR}/02-about-you.png` });

  // Answer both groups, then Continue → workspace step.
  await page.getByRole("radio", { name: /Engineer/i }).click();
  await page.getByRole("checkbox", { name: /Code & test with agents/i }).click();
  await page.getByRole("button", { name: "Continue" }).click();

  // 3. Workspace step
  await expect(page.getByRole("heading", { name: /Name your workspace/i })).toBeVisible({ timeout: 10000 });
  await expect(
    page.locator('[aria-current="step"]').filter({ hasText: "Workspace" }),
  ).toBeVisible();
  await page.waitForTimeout(600);
  await page.screenshot({ path: `${SHOTS_DIR}/03-workspace.png` });

  // 4. Runtime step — the rail should now show two completed steps and mark
  //    "Connect a runtime" current.
  await page.getByRole("textbox").first().fill(`Rail QA ${Date.now()}`);
  const created = page.waitForResponse((response) =>
    new URL(response.url()).pathname === "/api/workspaces" && response.request().method() === "POST",
  );
  await page.getByRole("button", { name: /^Create /i }).click();
  const response = await created;
  expect(response.status()).toBe(201);
  const workspace: { id: string; slug: string } = await response.json();
  api.setWorkspaceId(workspace.id);
  api.setWorkspaceSlug(workspace.slug);
  try {
    await expect(
      page.locator('[aria-current="step"]').filter({ hasText: "Connect a runtime" }),
    ).toBeVisible({ timeout: 20000 });
    await expect(projectExit).not.toHaveAttribute("aria-current", "step");
    await expect(projectExit.locator('button, a, [role="button"]')).toHaveCount(0);
    await expect(page.getByText("How did you hear about Multica?")).toHaveCount(0);
    await page.waitForTimeout(800);
    await page.screenshot({ path: `${SHOTS_DIR}/06-runtime.png` });

    // The rail previews the exit; only the runtime CTA finalizes onboarding.
    // The welcome dialog's Got it action subsequently opens the setup guide,
    // so verify the Projects landing before dismissing that dialog.
    const completed = page.waitForResponse((result) =>
      new URL(result.url()).pathname === "/api/me/onboarding/complete" && result.request().method() === "POST",
    );
    await page.getByRole("button", { name: /Skip for now/ }).click();
    expect((await completed).ok()).toBe(true);
    // The destination route can compile on its first visit in a dev server.
    await expect(page).toHaveURL(`/${workspace.slug}/projects`, { timeout: 20000 });
    await expect(page.getByRole("group", { name: "Onboarding steps" })).toHaveCount(0);
    await expect(page.getByRole("dialog", { name: "Welcome to Multica" })).toBeVisible();
    await page.screenshot({ path: `${SHOTS_DIR}/07-projects.png` });
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("onboarding — one skip clears the whole questionnaire step", async ({ page }) => {
  const api = new TestApiClient();
  await api.login(`skip-${Date.now()}@localhost`, "Skipper");
  const token = api.getToken();
  if (!token) throw new Error("E2E login did not return an auth token");

  await page.addInitScript((t) => localStorage.setItem("multica_token", t), token);
  await page.goto("/onboarding", { waitUntil: "domcontentloaded" });
  await waitForPageText(page, "Continue on web");

  await page.getByRole("button", { name: "Continue on web" }).click();
  await expect(page.getByText("From SDLC to ADLC. Build with agents.")).toBeVisible({ timeout: 10000 });

  // A single Skip covers role + use case — next stop is workspace.
  await page.getByRole("button", { name: "Skip" }).click();
  await expect(page.getByRole("heading", { name: /Name your workspace/i })).toBeVisible({ timeout: 10000 });
  await page.waitForTimeout(600);
  await page.screenshot({ path: `${SHOTS_DIR}/04-after-skip.png` });
});

test("onboarding — zh-Hans renders Chinese labels", async ({ page, context, baseURL }) => {
  await context.addCookies([
    {
      name: "multica-locale",
      value: "zh-Hans",
      url: baseURL ?? "http://localhost:3000",
    },
  ]);
  const api = new TestApiClient();
  await api.login(`zh-${Date.now()}@localhost`, "中文用户");
  const token = api.getToken();
  if (!token) throw new Error("E2E login did not return an auth token");

  await page.addInitScript((t) => localStorage.setItem("multica_token", t), token);
  await page.goto("/onboarding", { waitUntil: "domcontentloaded" });
  await waitForPageText(page, "在 web 端继续");

  // Click the CTA by name. `getByRole("button").first()` used to stand in for
  // it, but the welcome screen renders the pinned Log out button first in DOM
  // order — so this step was signing the user out and the assertions below
  // were waiting on a page that had already redirected to login.
  await page.getByRole("button", { name: "在 web 端继续" }).click();

  // About-you screen — Chinese headline + both sub-questions.
  await expect(page.getByText("从 SDLC 到 ADLC，开启智能体协作研发")).toBeVisible({ timeout: 10000 });
  await expect(page.getByText("你在研发协作中的角色是？")).toBeVisible();
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${SHOTS_DIR}/05-about-you-zh.png` });
});
