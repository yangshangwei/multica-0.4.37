import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect } from "@playwright/test";
import { administrationAccountPath, administrationEnvironment, exerciseCollaborationAdministration,
  seedCollaborationAdministration } from "./fixtures/collaboration-administration";
import { captureNativeSection, deepLink, nativeScreenshot, openDesktop, responseEvidence, type NativeHarness } from "./fixtures/collaboration-native";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

test("Desktop administers disabled-creator collaborations in the account shell and recovers the retained receipt", async ({ browserName: _browserName }, testInfo) => {
  test.setTimeout(240_000);
  test.skip(process.env.E2E_PLATFORM_ADMIN !== "1" || process.env.E2E_PASSWORD_AUTH !== "1" || !process.env.COLLABORATION_ELECTRON_RENDERER_URL,
    "Requires the independent password/admin API and rebuilt production Electron renderer/preload");
  administrationEnvironment();
  expect(["localhost", "127.0.0.1"]).toContain(new URL(process.env.COLLABORATION_ELECTRON_RENDERER_URL!).hostname);
  const fixture = await seedCollaborationAdministration();
  const profile = mkdtempSync(join(tmpdir(), "multica-s5-administration-"));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  const captures: Record<string, unknown>[] = [];
  try {
    const path = administrationAccountPath(fixture);
    native = await openDesktop(profile, { token: fixture.administrator.getToken()!, serverUrl: fixture.apiUrl,
      deepLink: `multica://account${path.slice("/account".length)}` });
    const current = native;
    await current.start();
    const evidence = await exerciseCollaborationAdministration(current.page, fixture, {
      remount: async () => { await current.page.reload(); await deepLink(current.app, path); },
      capture: async (stage, page, focus) => {
        captures.push(...await captureNativeSection(current.app, page, focus, `administration-${stage}`,
          name => testInfo.outputPath(name), '[data-tab-scroll-root="account-administration"]'));
      },
    });
    expect(current.errors).toEqual([]);
    expect(current.accountHeaders.every(headers => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    expect((responseEvidence.get(current.app) ?? []).some(response => response.path.startsWith(`/api/collaboration/issues/${fixture.issueId}`))).toBe(false);
    expect(await current.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    await testInfo.attach("administration-native-evidence", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
    await testInfo.attach("administration-native-geometry", { contentType: "application/json", body: JSON.stringify(captures, null, 2) });
  } catch (error) {
    if (native) {
      await nativeScreenshot(native.app, native.page, testInfo.outputPath("administration-native-failure.png")).catch(() => undefined);
      await testInfo.attach("administration-native-responses", { contentType: "application/json", body: JSON.stringify(responseEvidence.get(native.app) ?? [], null, 2) });
    }
    throw error;
  } finally {
    await native?.app.close().catch(() => undefined);
    rmSync(profile, { recursive: true, force: true });
  }
});
