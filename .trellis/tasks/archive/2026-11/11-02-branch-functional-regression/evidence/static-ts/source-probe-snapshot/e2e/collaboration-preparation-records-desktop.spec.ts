import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect } from "@playwright/test";
import { captureNativeSection, deepLink, nativeScreenshot, openDesktop, responseEvidence, type NativeHarness } from "./fixtures/collaboration-native";
import { exercisePreparationRecords, preparationAccountPath, preparationRecordEnvironment, seedCollaborationPreparationRecords } from "./fixtures/collaboration-preparation-records";

test("Desktop reads synthetic preparation with native 200 percent captures and unchanged proof", async ({ browserName: _browserName }, testInfo) => {
  test.setTimeout(240_000);
  test.skip(!process.env.COLLABORATION_E2E_API_URL || !process.env.DATABASE_URL || !process.env.MULTICA_DEPLOYMENT_ID || !process.env.NEXT_PUBLIC_API_URL || !process.env.COLLABORATION_ELECTRON_RENDERER_URL,
    "Requires the isolated API/database and rebuilt production Electron renderer/preload");
  preparationRecordEnvironment();
  expect(["localhost", "127.0.0.1"]).toContain(new URL(process.env.COLLABORATION_ELECTRON_RENDERER_URL!).hostname);
  const fixture = await seedCollaborationPreparationRecords();
  const viewerProfile = mkdtempSync(join(tmpdir(), "multica-preparation-viewer-"));
  const ownerProfile = mkdtempSync(join(tmpdir(), "multica-preparation-owner-"));
  const applications: Awaited<ReturnType<typeof openDesktop>>[] = [];
  const captures: Record<string, unknown>[] = [];
  try {
    const viewer = await openDesktop(viewerProfile, { token: fixture.viewer.getToken()!,
      deepLink: `multica://account${preparationAccountPath(fixture).slice("/account".length)}` });
    applications.push(viewer);
    const owner = await openDesktop(ownerProfile, { token: fixture.source.getToken()!,
      deepLink: `multica://account${preparationAccountPath(fixture).slice("/account".length)}` });
    applications.push(owner);
    await viewer.start();
    await owner.start();
    const evidence = await exercisePreparationRecords(viewer.page, owner.page, fixture, {
      navigate: (page, path) => deepLink(page === viewer.page ? viewer.app : owner.app, path),
      refresh: async (page, path) => { await page.reload(); await deepLink(page === viewer.page ? viewer.app : owner.app, path); },
      capture: async (stage, page, focus) => {
        captures.push(...await captureNativeSection(page === viewer.page ? viewer.app : owner.app, page, focus,
          `preparation-${stage}`, (name) => testInfo.outputPath(name)));
      },
    });
    for (const native of applications) {
      expect(native.errors).toEqual([]);
      expect(native.accountHeaders.length).toBeGreaterThan(0);
      expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
      expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    }
    await testInfo.attach("preparation-records-native-http-evidence", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
    await testInfo.attach("preparation-records-native-geometry", { contentType: "application/json", body: JSON.stringify(captures, null, 2) });
  } catch (error) {
    for (const [index, native] of applications.entries()) {
      await nativeScreenshot(native.app, native.page, testInfo.outputPath(`preparation-native-failure-${index + 1}.png`)).catch(() => undefined);
      await testInfo.attach(`preparation-native-responses-${index + 1}`, { contentType: "application/json", body: JSON.stringify(responseEvidence.get(native.app) ?? [], null, 2) });
    }
    throw error;
  } finally {
    for (const native of applications.reverse()) await native.app.close().catch(() => undefined);
    rmSync(viewerProfile, { recursive: true, force: true });
    rmSync(ownerProfile, { recursive: true, force: true });
  }
});
