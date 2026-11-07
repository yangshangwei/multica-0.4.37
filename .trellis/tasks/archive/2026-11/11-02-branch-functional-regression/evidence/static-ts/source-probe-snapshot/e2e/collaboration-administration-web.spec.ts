import { test, expect, type Locator, type Page, type TestInfo } from "@playwright/test";
import { administrationEnvironment, administrationWebPath, exerciseCollaborationAdministration,
  loginAdministrationWeb, seedCollaborationAdministration } from "./fixtures/collaboration-administration";

// Authentication request bodies must never enter browser trace artifacts.
test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

async function captureWebSection(page: Page, focus: Locator, stage: string, info: TestInfo) {
  const captures: Record<string, unknown>[] = [];
  for (const [size, viewport] of [["wide", { width: 1380, height: 1120 }], ["narrow", { width: 390, height: 844 }]] as const) {
    await page.setViewportSize(viewport);
    await page.evaluate(async () => {
      await document.fonts.ready;
      await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
    });
    const dimensions = await focus.evaluate(element => {
      const root = element.closest("main");
      if (!root) throw new Error("Administration capture requires its real scroll container");
      return { height: element.getBoundingClientRect().height, available: root.clientHeight - 24 };
    });
    expect(dimensions.available).toBeGreaterThan(100);
    const last = Math.max(0, dimensions.height - dimensions.available);
    const offsets = [0];
    for (let offset = dimensions.available; offset < last; offset += dimensions.available) offsets.push(offset);
    if (last > 0) offsets.push(last);
    let coveredThrough = 0;
    for (const [part, offset] of offsets.entries()) {
      await focus.evaluate((element, position) => {
        const root = element.closest("main")!;
        root.scrollTop += element.getBoundingClientRect().top - root.getBoundingClientRect().top - 12 + position;
      }, offset);
      await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      const geometry = await focus.evaluate(element => {
        const scroll = element.closest("main")!;
        const root = scroll.getBoundingClientRect();
        const box = element.getBoundingClientRect();
        return { viewport: { width: innerWidth, height: innerHeight }, root: root.toJSON(), focus: box.toJSON(),
          overflow: scroll.scrollWidth > scroll.clientWidth || document.documentElement.scrollWidth > innerWidth,
          visibleStart: Math.max(0, root.top - box.top), visibleEnd: Math.min(box.height, root.bottom - box.top) };
      });
      expect(geometry.viewport).toEqual(viewport);
      expect(geometry.overflow).toBe(false);
      expect(geometry.focus.x).toBeGreaterThanOrEqual(geometry.root.x);
      expect(geometry.focus.right).toBeLessThanOrEqual(geometry.root.right);
      expect(geometry.visibleStart).toBeLessThanOrEqual(coveredThrough + 1);
      expect(geometry.visibleEnd).toBeGreaterThan(geometry.visibleStart);
      coveredThrough = Math.max(coveredThrough, geometry.visibleEnd);
      const name = `administration-${stage}-${size}${offsets.length > 1 ? `-part-${part + 1}` : ""}.png`;
      const png = await page.screenshot({ path: info.outputPath(name), fullPage: false, scale: "css", animations: "disabled" });
      expect(png.readUInt32BE(16)).toBe(viewport.width);
      expect(png.readUInt32BE(20)).toBe(viewport.height);
      expect(await page.evaluate(() => ({ width: innerWidth, height: innerHeight }))).toEqual(viewport);
      captures.push({ name, ...geometry });
    }
    expect(coveredThrough).toBeGreaterThanOrEqual(dimensions.height - 1);
  }
  await page.setViewportSize({ width: 1380, height: 1120 });
  return captures;
}

test("Web administers disabled-creator collaborations with explicit succession and original receipt recovery", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  test.skip(process.env.E2E_PLATFORM_ADMIN !== "1" || process.env.E2E_PASSWORD_AUTH !== "1",
    "Requires the independent password/admin-enabled production Web environment");
  administrationEnvironment();
  const baseUrl = testInfo.project.use.baseURL;
  if (!baseUrl || !["localhost", "127.0.0.1"].includes(new URL(baseUrl).hostname)) throw new Error("Administration Web acceptance requires an explicit loopback production origin");
  const fixture = await seedCollaborationAdministration();
  await page.addInitScript(() => {
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  });
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  const captures: Record<string, unknown>[] = [];
  try {
    await loginAdministrationWeb(page, new URL(administrationWebPath(fixture), baseUrl).href);
    const cookieIdentity = await page.request.get(new URL("/api/admin/me", baseUrl).href);
    expect(cookieIdentity.status()).toBe(200);
    expect(await cookieIdentity.json()).toMatchObject({ user_id: fixture.administratorId, organization_id: fixture.organizationId, role: "super_admin" });
    const evidence = await exerciseCollaborationAdministration(page, fixture, {
      remount: async () => { await page.reload(); },
      dropFinalResponse: true,
      capture: async (stage, current, focus) => { captures.push(...await captureWebSection(current, focus, stage, testInfo)); },
    });
    expect(errors).toEqual([]);
    await testInfo.attach("administration-web-evidence", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
    await testInfo.attach("administration-web-geometry", { contentType: "application/json", body: JSON.stringify(captures, null, 2) });
  } catch (error) {
    if (new URL(page.url()).pathname === "/admin/collaboration") {
      await page.screenshot({ path: testInfo.outputPath("administration-web-failure.png"), fullPage: false, scale: "css" }).catch(() => undefined);
    }
    throw error;
  }
});
