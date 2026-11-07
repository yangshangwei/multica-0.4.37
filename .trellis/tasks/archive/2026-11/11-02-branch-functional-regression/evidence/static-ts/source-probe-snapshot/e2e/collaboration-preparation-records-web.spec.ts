import { test, expect, type Locator, type Page } from "@playwright/test";
import { authenticateCollaborationEditor } from "./fixtures/collaboration-editing";
import { exercisePreparationRecords, preparationRecordEnvironment, seedCollaborationPreparationRecords } from "./fixtures/collaboration-preparation-records";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

test("Web reads synthetic frozen preparation without inferring termination across refresh and viewers", async ({ page, browser }, testInfo) => {
  test.setTimeout(180_000);
  test.skip(!process.env.COLLABORATION_E2E_API_URL || !process.env.DATABASE_URL || !process.env.MULTICA_DEPLOYMENT_ID || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the isolated collaboration API/database and matching production Web build");
  preparationRecordEnvironment();
  const baseUrl = testInfo.project.use.baseURL;
  if (!baseUrl || !["localhost", "127.0.0.1"].includes(new URL(baseUrl).hostname)) throw new Error("Preparation Web evidence requires an explicit loopback production origin");
  const fixture = await seedCollaborationPreparationRecords();
  const ownerContext = await browser.newContext();
  const owner = await ownerContext.newPage();
  const errors: string[] = [];
  for (const current of [page, owner]) current.on("pageerror", (error) => errors.push(error.message));
  const captures: Record<string, unknown>[] = [];
  const capture = async (stage: string, current: Page, focus: Locator) => {
    for (const [size, viewport] of [["wide", { width: 1280, height: 1000 }], ["narrow", { width: 390, height: 844 }]] as const) {
      await current.setViewportSize(viewport);
      await focus.evaluate((element) => {
        const root = element.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]');
        if (!root) throw new Error("Preparation screenshot requires its actual account detail scroll root");
        root.scrollTop += element.getBoundingClientRect().top - root.getBoundingClientRect().top - 12;
      });
      await current.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      await expect(focus).toBeInViewport({ ratio: 1 });
      const geometry = await focus.boundingBox();
      expect(geometry).not.toBeNull();
      expect(geometry!.x).toBeGreaterThanOrEqual(0);
      expect(geometry!.x + geometry!.width).toBeLessThanOrEqual(viewport.width);
      expect(await current.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const name = `preparation-${stage}-${size}.png`;
      const bytes = await current.screenshot({ path: testInfo.outputPath(name), animations: "disabled", fullPage: false, scale: "css" });
      expect(bytes.readUInt32BE(16)).toBe(viewport.width);
      expect(bytes.readUInt32BE(20)).toBe(viewport.height);
      expect(await current.evaluate(() => ({ width: innerWidth, height: innerHeight }))).toEqual(viewport);
      captures.push({ name, viewport, focus: geometry });
    }
    await current.setViewportSize({ width: 1280, height: 1000 });
  };
  try {
    await authenticateCollaborationEditor(page, fixture.viewer.getToken()!);
    await authenticateCollaborationEditor(owner, fixture.source.getToken()!);
    const evidence = await exercisePreparationRecords(page, owner, fixture, {
      navigate: async (current, path) => { await current.goto(new URL(path, baseUrl).href); },
      refresh: async (current) => { await current.reload(); }, capture,
    });
    expect(errors).toEqual([]);
    await testInfo.attach("preparation-records-http-evidence", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
    await testInfo.attach("preparation-records-web-geometry", { contentType: "application/json", body: JSON.stringify(captures, null, 2) });
  } finally { await ownerContext.close(); }
});
