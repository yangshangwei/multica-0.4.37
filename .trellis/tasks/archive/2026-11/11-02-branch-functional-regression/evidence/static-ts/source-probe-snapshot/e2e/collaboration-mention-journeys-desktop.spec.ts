import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect, type Page, type TestInfo } from "@playwright/test";
import type { TestApiClient } from "./fixtures";
import { captureNativeSection, deepLink, nativeScreenshot, openDesktop, responseEvidence, type NativeHarness } from "./fixtures/collaboration-native";
import {
  exerciseDurableMentionNotifications, exerciseMixedMentionJourney, exercisePartialMentionRefusal,
  mentionAccountPath, mentionEnvironmentReady, seedCollaborationMentionJourney, sendOfflineHumanMention,
  type CollaborationMentionJourney, type MentionJourneyAdapter,
} from "./fixtures/collaboration-mention-journeys";

test.use({ actionTimeout: 15_000, trace: "retain-on-failure" });
test.beforeEach(() => {
  test.setTimeout(300_000);
  test.skip(!mentionEnvironmentReady() || !process.env.COLLABORATION_ELECTRON_RENDERER_URL,
    "Requires the task-owned API/DB, production native renderer and explicit enrollment deployment");
  expect(["localhost", "127.0.0.1"]).toContain(new URL(process.env.COLLABORATION_ELECTRON_RENDERER_URL!).hostname);
});

type NativeSession = Awaited<ReturnType<typeof openDesktop>>;
interface NativeJourney {
  fixture: CollaborationMentionJourney;
  creator: NativeSession;
  human: NativeSession;
  adapter: MentionJourneyAdapter;
  close(session: NativeSession): Promise<void>;
  start(actor: TestApiClient): Promise<NativeSession>;
}

async function withNativeJourney(testInfo: TestInfo, exercise: (journey: NativeJourney) => Promise<unknown>) {
  const fixture = await seedCollaborationMentionJourney();
  const profiles: string[] = [];
  const active = new Set<NativeSession>();
  const captures: Record<string, unknown>[] = [];
  const start = async (actor: TestApiClient) => {
    const token = actor.getToken();
    if (!token) throw new Error("The synthetic native actor has no session");
    const profile = mkdtempSync(join(tmpdir(), "multica-mention-journey-"));
    profiles.push(profile);
    const native = await openDesktop(profile, { token,
      deepLink: `multica://account${mentionAccountPath(fixture, "current").slice("/account".length)}` });
    active.add(native);
    await native.start();
    await expect(native.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    return native;
  };
  const verifyNative = async (native: NativeSession) => {
    expect(native.errors).toEqual([]);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
      .desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
  };
  const close = async (native: NativeSession) => {
    await verifyNative(native);
    await native.app.close();
    active.delete(native);
  };
  const sessionFor = (page: Page) => {
    const native = [...active].find((item) => item.page === page);
    if (!native) throw new Error("This page does not belong to an active isolated native session");
    return native;
  };
  const adapter: MentionJourneyAdapter = {
    navigate: (page, path) => deepLink(sessionFor(page).app, path),
    capture: async (stage, page, focus) => {
      captures.push(...await captureNativeSection(sessionFor(page).app, page, focus, stage, (name) => testInfo.outputPath(name)));
    },
  };
  try {
    const creator = await start(fixture.creator);
    const human = await start(fixture.handler);
    const evidence = await exercise({ fixture, creator, human, adapter, close, start });
    for (const native of active) await verifyNative(native);
    await testInfo.attach("desktop-mention-journey-identities", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
  } catch (error) {
    for (const [index, native] of [...active].entries()) {
      if (native.page.isClosed()) continue;
      try {
        const path = testInfo.outputPath(`desktop-mention-failure-${index}.png`);
        await nativeScreenshot(native.app, native.page, path);
        await testInfo.attach(`desktop-mention-failure-${index}`, { path, contentType: "image/png" });
        await testInfo.attach(`desktop-mention-responses-${index}`, { contentType: "application/json",
          body: JSON.stringify(responseEvidence.get(native.app) ?? [], null, 2) });
        await testInfo.attach(`desktop-mention-page-${index}`, { contentType: "text/plain", body: await native.page.locator("body").innerText() });
      } catch { /* Keep the original failure when the failed window cannot be captured. */ }
    }
    throw error;
  } finally {
    await testInfo.attach("desktop-mention-capture-geometry", { contentType: "application/json", body: JSON.stringify(captures, null, 2) });
    for (const native of active) await native.app.close().catch(() => undefined);
    for (const profile of profiles) rmSync(profile, { recursive: true, force: true });
  }
}

test("Desktop mixed H+A+B+S mentions retain one comment and exact queued target associations", async ({ browserName: _browserName }, testInfo) => {
  await withNativeJourney(testInfo, ({ fixture, creator, human, adapter }) =>
    exerciseMixedMentionJourney(creator.page, human.page, fixture, adapter));
});

test("Desktop owner revocation after preview refuses only A and original-key replay stays deduplicated", async ({ browserName: _browserName }, testInfo) => {
  await withNativeJourney(testInfo, ({ fixture, creator, human, adapter }) =>
    exercisePartialMentionRefusal(creator.page, human.page, fixture, adapter));
});

test("Desktop offline mentions survive a fresh profile and edits notify only newly added people", async ({ browserName: _browserName }, testInfo) => {
  await withNativeJourney(testInfo, async ({ fixture, creator, human, adapter, close, start }) => {
    await close(human);
    expect(human.page.isClosed()).toBe(true);
    const offline = await sendOfflineHumanMention(creator.page, fixture, adapter);
    await fixture.handler.login(fixture.handler.getEmail(), fixture.handlerName);
    expect((await fixture.handler.requestJSON<{ id: string }>("/api/me")).id).toBe(fixture.handlerId);
    const freshHuman = await start(fixture.handler);
    expect(freshHuman.app).not.toBe(human.app);
    const evidence = await exerciseDurableMentionNotifications(creator.page, freshHuman.page, fixture, offline, adapter);
    return { ...evidence, previous_application_closed_before_send: true, login_after_send: "legacy API", fresh_profile: true };
  });
});
