import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect, type Locator, type Page, type TestInfo, type Browser } from "@playwright/test";
import { nativeScreenshot, openDesktop } from "./fixtures/collaboration-native";
import { crossClientEnvironmentReady, observeCrossClientWrites } from "./fixtures/collaboration-cross-client-execution";
import { lifecycleControl, lifecyclePath, refreshLifecyclePage } from "./fixtures/collaboration-lifecycle-controls";
import {
  createLifecycleRuntimeTransport, exerciseLiveClose, exerciseSameLaneControls, seedLifecycleControls, startLifecycleDaemons,
} from "./fixtures/collaboration-lane-close-controls";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "only-on-failure" });

async function laneCloseClients(browser: Browser, info: TestInfo, platform: "Web" | "Electron", liveAction?: "complete" | "cancel") {
  const seed = await seedLifecycleControls();
  const { fixture } = seed;
  const origin = info.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(origin).hostname);
  const context = await browser.newContext();
  const page = await context.newPage();
  const profile = mkdtempSync(join(tmpdir(), "multica-lane-close-controls-"));
  const nativeActor = platform === "Electron" ? fixture.webActor : fixture.desktopActor;
  const webActor = platform === "Web" ? fixture.webActor : fixture.desktopActor;
  const sources = ["e2e/collaboration-lane-close-controls.spec.ts", "e2e/fixtures/collaboration-lane-close-controls.ts",
    "e2e/fixtures/collaboration-lifecycle-controls.ts", "e2e/fixtures/collaboration-cross-client-execution.ts",
    "e2e/fixtures/collaboration-native.ts", "e2e/fixtures/collaboration-cross-client-provider.go"]
    .map((path) => ({ path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") }));
  const captures: { stage: string; client: string; path: string }[] = [];
  const observers: ReturnType<typeof observeCrossClientWrites>[] = [];
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  let proxy: Awaited<ReturnType<typeof createLifecycleRuntimeTransport>> | undefined;
  let daemons: Awaited<ReturnType<typeof startLifecycleDaemons>> | undefined;
  const capture = async (stage: string, currentPage: Page, focus: Locator) => {
    await focus.scrollIntoViewIfNeeded();
    await expect(focus).toBeVisible();
    const path = info.outputPath(`${platform.toLowerCase()}-${stage}-${currentPage === native!.page ? "electron" : "web"}.png`);
    if (currentPage === native!.page) await nativeScreenshot(native!.app, currentPage, path);
    else await currentPage.screenshot({ path, animations: "disabled" });
    captures.push({ stage, client: currentPage === native!.page ? "electron" : "web", path });
  };
  try {
    await webActor.authenticateWeb(page);
    await context.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await page.goto(new URL(lifecyclePath(fixture), origin).href);
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    native = await openDesktop(profile, { token: nativeActor.client.getToken()!, deepLink: `multica://account${lifecyclePath(fixture).slice("/account".length)}` });
    await native.start();
    await expect(native.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const creatorPage = platform === "Web" ? page : native.page;
    const handlerPage = platform === "Web" ? native.page : page;
    observers.push(observeCrossClientWrites(page, fixture.issueId, "web", webActor.id),
      observeCrossClientWrites(native.page, fixture.issueId, "electron", nativeActor.id));
    let initialResume: Awaited<ReturnType<typeof lifecycleControl>> | undefined;
    let evidence: Record<string, unknown>;
    if (liveAction) {
      initialResume = await lifecycleControl(creatorPage, fixture, "resume");
      expect(initialResume.receipt).toMatchObject({ code: "resumed", queue_state: "active" });
      await refreshLifecyclePage(handlerPage, fixture);
      proxy = await createLifecycleRuntimeTransport(fixture.apiUrl);
      daemons = await startLifecycleDaemons(fixture, proxy);
      evidence = await exerciseLiveClose(seed, creatorPage, handlerPage, platform === "Web" ? "web" : "electron", liveAction, { proxy, daemons, capture });
    } else {
      evidence = await exerciseSameLaneControls(seed, creatorPage, handlerPage, platform === "Web" ? "web" : "electron", capture);
    }
    expect(errors).toEqual([]);
    expect(native.errors).toEqual([]);
    expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    await info.attach("lane-close-control-identities", { contentType: "application/json", body: JSON.stringify({ ...evidence,
      creator_platform: platform, sources, initial_resume: initialResume, captures, writes: observers.flatMap((observer) => observer.writes),
      environment: { api: fixture.apiUrl, production_web: origin, native_renderer: process.env.COLLABORATION_ELECTRON_RENDERER_URL,
        native_os: process.platform, execution_os: liveAction ? "actual Linux PID namespaces; test-owned synthetic stdlib executable" : "offline managed resources; no execution",
        provider_account_access: false, real_provider_smoke: false, production_gate_change: false, packaged_main_startup: false } }, null, 2) });
  } catch (error) {
    await info.attach("lane-close-control-attempt", { contentType: "application/json", body: JSON.stringify({ creator_platform: platform, action: liveAction,
      sources, issue_id: fixture.issueId, writes: observers.flatMap((observer) => observer.writes), captures, errors,
      native_errors: native?.errors, runtime_transport: proxy?.evidence(), provider_observations: daemons?.evidence() }, null, 2) });
    throw error;
  } finally {
    // Fixture-owned process cleanup is distinct from the actual UI Stop/close
    // and natural termination evidence asserted before this block.
    await daemons?.close();
    await proxy?.close();
    for (const observer of observers) observer.dispose();
    await native?.app.close().catch(() => undefined);
    await context.close();
    rmSync(profile, { recursive: true, force: true });
  }
}

for (const platform of ["Web", "Electron"] as const) {
  test(`${platform} same-lane agent and squad controls retain independent holds and whole-issue pause for new targets`, async ({ browser }, info) => {
    test.setTimeout(180_000);
    test.skip(!crossClientEnvironmentReady(), "Requires the root-owned production/password API, renderer and explicit fixture identities");
    await laneCloseClients(browser, info, platform);
  });
  for (const action of ["complete", "cancel"] as const) {
    test(`${platform} creator ${action} cancels every author's waiting inputs while live natural proofs remain unconfirmed`, async ({ browser }, info) => {
      test.setTimeout(240_000);
      test.skip(!crossClientEnvironmentReady(), "Requires root-owned production/password API, Electron and explicit Linux fixtures");
      await laneCloseClients(browser, info, platform, action);
    });
  }
}
