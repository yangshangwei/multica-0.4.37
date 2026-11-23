import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect } from "@playwright/test";
import { captureNativeSection, deepLink, nativeScreenshot, openDesktop, responseEvidence, type NativeHarness } from "./fixtures/collaboration-native";
import { controlAccountPath, controlEnvironmentReady, exerciseCollaborationControlJourney, exerciseCollaborationPendingProofJourney, seedCollaborationControlJourney } from "./fixtures/collaboration-control-journey";

test.use({ actionTimeout: 15_000, trace: "retain-on-failure" });

for (const [name, exercise] of [
  ["Desktop controls recover partial Resume, isolate agent/squad lanes and stop the whole issue", exerciseCollaborationControlJourney],
  ["Desktop pending termination proof keeps A paused while whole Resume releases B and squad S", exerciseCollaborationPendingProofJourney],
] as const) {
test(name, async ({ browserName: _browserName }, testInfo) => {
  test.setTimeout(360_000);
  test.skip(!controlEnvironmentReady() || !process.env.COLLABORATION_ELECTRON_RENDERER_URL,
    "Requires the task-owned password/managed deployment, execution gate and matching production native renderer");
  expect(["localhost", "127.0.0.1"]).toContain(new URL(process.env.COLLABORATION_ELECTRON_RENDERER_URL!).hostname);
  const fixture = await seedCollaborationControlJourney();
  const profile = mkdtempSync(join(tmpdir(), "multica-control-journey-"));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  const captures: Record<string, unknown>[] = [];
  try {
    native = await openDesktop(profile, { token: fixture.creator.getToken()!,
      deepLink: `multica://account${controlAccountPath(fixture).slice("/account".length)}` });
    const { app, page } = native;
    await native.start();
    const evidence = await exercise(page, fixture, {
      navigate: (_page, path) => deepLink(app, path),
      capture: async (stage, currentPage, focus) => {
        captures.push(...await captureNativeSection(app, currentPage, focus, stage, (name) => testInfo.outputPath(name)));
      },
    });
    expect(native.errors).toEqual([]);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    expect(await app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
      .desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    await testInfo.attach("desktop-control-journey-identities", { contentType: "application/json", body: JSON.stringify({ ...evidence,
      client_os: process.platform, resource_os: "synthetic remote linux", native_containment_claim: false }, null, 2) });
  } catch (error) {
    if (native && !native.page.isClosed()) {
      try {
        const path = testInfo.outputPath("desktop-control-failure.png");
        await nativeScreenshot(native.app, native.page, path);
        await testInfo.attach("desktop-control-failure", { path, contentType: "image/png" });
        await testInfo.attach("desktop-control-responses", { contentType: "application/json", body: JSON.stringify(responseEvidence.get(native.app) ?? [], null, 2) });
        await testInfo.attach("desktop-control-page", { contentType: "text/plain", body: await native.page.locator("body").innerText() });
      } catch { /* Preserve the first failure if the window is no longer capturable. */ }
    }
    throw error;
  } finally {
    await testInfo.attach("desktop-control-capture-geometry", { contentType: "application/json", body: JSON.stringify(captures, null, 2) });
    await native?.app.close().catch(() => undefined);
    rmSync(profile, { recursive: true, force: true });
  }
});
}
