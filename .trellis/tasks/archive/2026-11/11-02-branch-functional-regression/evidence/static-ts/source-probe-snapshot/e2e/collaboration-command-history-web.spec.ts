import { test, expect, type Locator, type Page } from "@playwright/test";
import { authenticateCollaborationResourcesPage } from "./fixtures/collaboration-resources";
import { exerciseCollaborationResourceOffers, seedCollaborationResourceOfferJourney } from "./fixtures/collaboration-resource-offers";
import { exerciseCollaborationCommandHistory } from "./fixtures/collaboration-command-history";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

test("cancelled waiting commands retain original content after comment changes, refresh and another account reads", async ({ page, browser }, testInfo) => {
  test.setTimeout(240_000);
  test.skip(!process.env.COLLABORATION_E2E_API_URL || !process.env.DATABASE_URL || !process.env.MULTICA_DEPLOYMENT_ID || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the task-owned API/DB and matching production Web");
  const fixture = await seedCollaborationResourceOfferJourney();
  const handlerPage = await browser.newPage();
  const navigate = async (current: Page, path: string) => { await current.goto(new URL(path, testInfo.project.use.baseURL).href); };
  try {
    await authenticateCollaborationResourcesPage(page, fixture.owner);
    await authenticateCollaborationResourcesPage(handlerPage, fixture.handler);
    const resources = await exerciseCollaborationResourceOffers(page, handlerPage, fixture, { navigate });
    const captures: Record<string, unknown>[] = [];
    const capture = async (stage: string, current: Page, focus: Locator) => {
      for (const [size, viewport] of [["wide", { width: 1280, height: 1000 }], ["narrow", { width: 390, height: 844 }]] as const) {
        await current.setViewportSize(viewport);
        await focus.scrollIntoViewIfNeeded();
        await current.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
        await expect(focus).toBeInViewport({ ratio: 1 });
        expect(await current.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        const bounds = await focus.boundingBox();
        expect(bounds!.x).toBeGreaterThanOrEqual(0);
        expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(viewport.width);
        const name = `command-history-${stage}-${size}.png`;
        const png = await current.screenshot({ path: testInfo.outputPath(name), animations: "disabled", scale: "css" });
        expect(png.readUInt32BE(16)).toBe(viewport.width);
        expect(png.readUInt32BE(20)).toBe(viewport.height);
        captures.push({ name, viewport, focus: bounds });
      }
      await current.setViewportSize({ width: 1280, height: 1000 });
    };
    const result = await exerciseCollaborationCommandHistory(handlerPage, page, fixture, resources, { navigate, capture });
    await testInfo.attach("command-history-identities", { contentType: "application/json", body: JSON.stringify(result, null, 2) });
    await testInfo.attach("command-history-capture-geometry", { contentType: "application/json", body: JSON.stringify(captures, null, 2) });
  } finally { await handlerPage.close(); }
});
