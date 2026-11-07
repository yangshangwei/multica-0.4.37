import { test, expect } from "@playwright/test";
import { authenticateControlWeb, controlEnvironmentReady, exerciseCollaborationControlJourney, exerciseCollaborationPendingProofJourney, seedCollaborationControlJourney } from "./fixtures/collaboration-control-journey";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

for (const [name, exercise] of [
  ["Web controls recover partial Resume, isolate agent/squad lanes and stop the whole issue", exerciseCollaborationControlJourney],
  ["Web pending termination proof keeps A paused while whole Resume releases B and squad S", exerciseCollaborationPendingProofJourney],
] as const) {
test(name, async ({ page }, testInfo) => {
  test.setTimeout(300_000);
  test.skip(!controlEnvironmentReady(), "Requires the task-owned password/managed deployment with execution enabled and matching production Web");
  const webUrl = testInfo.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(webUrl).hostname);
  const fixture = await seedCollaborationControlJourney();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await authenticateControlWeb(page, fixture, webUrl);
  const geometry: Record<string, unknown>[] = [];
  const evidence = await exercise(page, fixture, {
    navigate: async (currentPage, path) => { await currentPage.goto(new URL(path, webUrl).href); },
    capture: async (stage, currentPage, focus) => {
      for (const [size, viewport] of [["wide", { width: 1280, height: 1000 }], ["narrow", { width: 390, height: 844 }]] as const) {
        await currentPage.setViewportSize(viewport);
        await focus.scrollIntoViewIfNeeded();
        await focus.evaluate((element) => element.scrollIntoView({ block: "center", inline: "nearest", behavior: "instant" }));
        await expect(focus).toBeVisible();
        await currentPage.evaluate(async () => {
          await document.fonts.ready;
          await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
        });
        const bounds = await focus.boundingBox();
        if (!bounds) throw new Error("The control result has no visible capture bounds");
        expect(bounds.x).toBeGreaterThanOrEqual(0);
        expect(bounds.x + bounds.width).toBeLessThanOrEqual(viewport.width);
        expect(bounds.y).toBeGreaterThanOrEqual(0);
        expect(bounds.y + bounds.height).toBeLessThanOrEqual(viewport.height);
        expect(await currentPage.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        const name = `${stage}-${size}.png`;
        const png = await currentPage.screenshot({ path: testInfo.outputPath(name), animations: "disabled", fullPage: false, scale: "css" });
        expect(png.readUInt32BE(16)).toBe(viewport.width);
        expect(png.readUInt32BE(20)).toBe(viewport.height);
        geometry.push({ name, viewport, focus: bounds });
      }
      await currentPage.setViewportSize({ width: 1280, height: 1000 });
    },
  });
  expect(errors).toEqual([]);
  await testInfo.attach("control-journey-identities", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
  await testInfo.attach("control-capture-geometry", { contentType: "application/json", body: JSON.stringify(geometry, null, 2) });
});
}
