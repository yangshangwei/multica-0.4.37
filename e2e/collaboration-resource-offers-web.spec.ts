import { test, expect, type Locator, type Page } from "@playwright/test";
import { authenticateCollaborationResourcesPage } from "./fixtures/collaboration-resources";
import { exerciseCollaborationResourceOffers, resourceOfferAccountPath, seedCollaborationResourceOfferJourney } from "./fixtures/collaboration-resource-offers";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

test("a private owner offers a resource to an external organizer and independently authorizes it", async ({ page, browser }, testInfo) => {
  test.setTimeout(240_000);
  test.skip(!process.env.COLLABORATION_E2E_API_URL || !process.env.DATABASE_URL || !process.env.MULTICA_DEPLOYMENT_ID || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the task-owned API/DB with creation enabled and matching production Web");
  const fixture = await seedCollaborationResourceOfferJourney();
  const organizerContext = await browser.newContext();
  const organizerPage = await organizerContext.newPage();
  const captures: { name: string; viewport: { width: number; height: number }; focus: { x: number; y: number; width: number; height: number } }[] = [];
  const capture = async (stage: string, currentPage: Page, focus: Locator) => {
    for (const [size, viewport] of [["wide", { width: 1280, height: 1000 }], ["narrow", { width: 390, height: 844 }]] as const) {
      await currentPage.setViewportSize(viewport);
      await focus.scrollIntoViewIfNeeded();
      await expect(focus).toBeVisible();
      await currentPage.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      const bounds = await focus.boundingBox();
      expect(bounds).not.toBeNull();
      expect(bounds!.x).toBeGreaterThanOrEqual(0);
      expect(bounds!.y).toBeGreaterThanOrEqual(0);
      expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(viewport.width);
      expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(viewport.height);
      expect(await currentPage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      const name = `resource-offers-${stage}-${size}.png`;
      const bytes = await currentPage.screenshot({ path: testInfo.outputPath(name), animations: "disabled", fullPage: false, scale: "css" });
      expect(bytes.readUInt32BE(16)).toBe(viewport.width);
      expect(bytes.readUInt32BE(20)).toBe(viewport.height);
      captures.push({ name, viewport, focus: bounds! });
    }
    await currentPage.setViewportSize({ width: 1280, height: 1000 });
  };
  try {
    await authenticateCollaborationResourcesPage(page, fixture.owner);
    await authenticateCollaborationResourcesPage(organizerPage, fixture.handler);
    const identities = await exerciseCollaborationResourceOffers(page, organizerPage, fixture, {
      navigate: async (currentPage, path) => { await currentPage.goto(new URL(path, testInfo.project.use.baseURL).href); }, capture,
    });
    await testInfo.attach("resource-offer-journey-identities", { contentType: "application/json", body: JSON.stringify(identities, null, 2) });

    // Web's initial locale comes from its cookie and then the account profile.
    await fixture.handler.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await organizerPage.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: new URL(organizerPage.url()).origin }]);
    const chinesePage = await organizerPage.context().newPage();
    try {
      await authenticateCollaborationResourcesPage(chinesePage, fixture.handler, "zh-Hans");
      await chinesePage.goto(new URL(resourceOfferAccountPath(fixture), organizerPage.url()).href);
      await expect(chinesePage.locator("html")).toHaveAttribute("lang", "zh-CN");
      await chinesePage.getByRole("button", { name: "管理 AI 资源", exact: true }).click();
      const offers = chinesePage.getByRole("region", { name: "已提供的资源", exact: true });
      await expect(offers.getByText("已采纳", { exact: true })).toBeVisible();
      await expect(offers.getByText("已撤回", { exact: true })).toBeVisible();
      await expect(chinesePage.getByText(fixture.undisclosedAgentName, { exact: true })).toHaveCount(0);
      await capture("zh-history", chinesePage, offers);
    } finally { await chinesePage.close(); }
    await testInfo.attach("resource-offer-capture-geometry", { contentType: "application/json", body: JSON.stringify(captures, null, 2) });
  } finally { await organizerContext.close(); }
});
