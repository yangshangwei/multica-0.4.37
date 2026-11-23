import { test, expect, type Locator, type Page, type TestInfo } from "@playwright/test";
import { authenticateCollaborationResourcesPage } from "./fixtures/collaboration-resources";
import {
  exerciseDurableMentionNotifications, exerciseMixedMentionJourney, exercisePartialMentionRefusal,
  mentionAccountPath, mentionEnvironmentReady, seedCollaborationMentionJourney, sendOfflineHumanMention,
} from "./fixtures/collaboration-mention-journeys";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });
test.beforeEach(({ browserName: _browserName }, testInfo) => {
  test.setTimeout(240_000);
  test.skip(!mentionEnvironmentReady(), "Requires task-owned loopback API/DB and a matching production Web build");
  expect(["localhost", "127.0.0.1"]).toContain(new URL(testInfo.project.use.baseURL!).hostname);
});

async function captureSection(page: Page, focus: Locator, stage: string, testInfo: TestInfo) {
  const geometry = [];
  for (const [size, viewport] of [["wide", { width: 1280, height: 1000 }], ["narrow", { width: 390, height: 844 }]] as const) {
    await page.setViewportSize(viewport);
    await focus.scrollIntoViewIfNeeded();
    await expect(focus).toBeVisible();
    await page.evaluate(async () => {
      await document.fonts.ready;
      await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
    });
    const bounds = await focus.boundingBox();
    if (!bounds) throw new Error("The mention capture section has no visible bounds");
    expect(bounds.x).toBeGreaterThanOrEqual(0);
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(viewport.width);
    expect(bounds.y).toBeGreaterThanOrEqual(0);
    expect(bounds.y + bounds.height).toBeLessThanOrEqual(viewport.height);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const name = `${stage}-${size}.png`;
    const png = await page.screenshot({ path: testInfo.outputPath(name), animations: "disabled", fullPage: false, scale: "css" });
    expect(png.readUInt32BE(16)).toBe(viewport.width);
    expect(png.readUInt32BE(20)).toBe(viewport.height);
    geometry.push({ name, viewport, focus: bounds });
  }
  await page.setViewportSize({ width: 1280, height: 1000 });
  await testInfo.attach(`${stage}-geometry`, { contentType: "application/json", body: JSON.stringify(geometry, null, 2) });
}

for (const [name, exercise] of [
  ["Web mixed H+A+B+S mentions retain one comment and exact queued target associations", exerciseMixedMentionJourney],
  ["Web owner revocation after preview refuses only A and original-key replay stays deduplicated", exercisePartialMentionRefusal],
] as const) {
  test(name, async ({ page, browser }, testInfo) => {
    const fixture = await seedCollaborationMentionJourney();
    const humanContext = await browser.newContext();
    try {
      const humanPage = await humanContext.newPage();
      const errors: string[] = [];
      for (const currentPage of [page, humanPage]) currentPage.on("pageerror", (error) => errors.push(error.message));
      await authenticateCollaborationResourcesPage(page, fixture.creator);
      await authenticateCollaborationResourcesPage(humanPage, fixture.handler);
      const evidence = await exercise(page, humanPage, fixture, {
        navigate: async (currentPage, path) => { await currentPage.goto(new URL(path, testInfo.project.use.baseURL).href); },
        capture: (stage, currentPage, focus) => captureSection(currentPage, focus, stage, testInfo),
      });
      expect(errors).toEqual([]);
      await testInfo.attach("mention-journey-identities", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
    } finally { await humanContext.close(); }
  });
}

test("Web offline mentions survive a fresh context and edits notify only newly added people", async ({ page, browser }, testInfo) => {
  const fixture = await seedCollaborationMentionJourney();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await authenticateCollaborationResourcesPage(page, fixture.creator);
  const adapter = {
    navigate: async (currentPage: Page, path: string) => { await currentPage.goto(new URL(path, testInfo.project.use.baseURL).href); },
    capture: (stage: string, currentPage: Page, focus: Locator) => captureSection(currentPage, focus, stage, testInfo),
  };
  const previousContext = await browser.newContext();
  const previousPage = await previousContext.newPage();
  try {
    previousPage.on("pageerror", (error) => errors.push(error.message));
    await authenticateCollaborationResourcesPage(previousPage, fixture.handler);
    await adapter.navigate(previousPage, mentionAccountPath(fixture, "current"));
    await expect(previousPage.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  } finally { await previousContext.close(); }
  expect(previousPage.isClosed()).toBe(true);
  const offline = await sendOfflineHumanMention(page, fixture, adapter);
  await fixture.handler.login(fixture.handler.getEmail(), fixture.handlerName);
  expect((await fixture.handler.requestJSON<{ id: string }>("/api/me")).id).toBe(fixture.handlerId);

  const freshContext = await browser.newContext();
  try {
    expect(freshContext).not.toBe(previousContext);
    const humanPage = await freshContext.newPage();
    humanPage.on("pageerror", (error) => errors.push(error.message));
    await authenticateCollaborationResourcesPage(humanPage, fixture.handler);
    const evidence = await exerciseDurableMentionNotifications(page, humanPage, fixture, offline, adapter);
    expect(errors).toEqual([]);
    await testInfo.attach("durable-mention-identities", { contentType: "application/json",
      body: JSON.stringify({ ...evidence, previous_context_closed_before_send: true, login_after_send: "legacy API", fresh_context: true }, null, 2) });
  } finally { await freshContext.close(); }
});
