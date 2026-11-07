import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect } from "@playwright/test";
import { nativeScreenshot, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import { crossClientEnvironmentReady, observeCrossClientWrites } from "./fixtures/collaboration-cross-client-execution";
import { createLifecycleRuntimeTransport, exerciseLifecycleRemoval, lifecycleControl, lifecyclePath,
  refreshLifecyclePage, seedLifecycleControls, startLifecycleDaemons } from "./fixtures/collaboration-lifecycle-controls";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "only-on-failure" });

for (const platform of ["Web", "Electron"] as const) {
  test(`${platform} creator removal keeps paused inputs and original natural proof through client and daemon reconnect`, async ({ browser }, info) => {
    test.setTimeout(360_000);
    test.skip(!crossClientEnvironmentReady(), "Requires root-allocated task-owned production Web/password API/renderer and explicit Linux fixtures");
    const origin = info.project.use.baseURL!;
    expect(["localhost", "127.0.0.1"]).toContain(new URL(origin).hostname);
    const { fixture, independent, peerId, peerBefore } = await seedLifecycleControls();
    const context = await browser.newContext();
    const page = await context.newPage();
    const nativeActor = platform === "Electron" ? fixture.webActor : fixture.desktopActor;
    const webActor = platform === "Web" ? fixture.webActor : fixture.desktopActor;
    const profile = mkdtempSync(join(tmpdir(), "multica-lifecycle-controls-"));
    const sources = ["e2e/collaboration-lifecycle-controls.spec.ts", "e2e/fixtures/collaboration-lifecycle-controls.ts",
      "e2e/fixtures/collaboration-cross-client-execution.ts", "e2e/fixtures/collaboration-native.ts", "e2e/fixtures/collaboration-cross-client-provider.go"]
      .map((path) => ({ path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") }));
    let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
    let proxy: Awaited<ReturnType<typeof createLifecycleRuntimeTransport>> | undefined;
    let daemons: Awaited<ReturnType<typeof startLifecycleDaemons>> | undefined;
    const errors: string[] = [];
    const requestObservers: ReturnType<typeof observeCrossClientWrites>[] = [];
    const captures: { stage: string; client: string; path: string }[] = [];
    const reconnects: { web_fetch_failed: boolean; electron_renderer_fetch_failed: boolean;
      native_account_ipc_blocked: boolean; restored_fetch_status: number[]; native_account_ipc_restored: boolean }[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
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
      requestObservers.push(observeCrossClientWrites(page, fixture.issueId, "web", webActor.id),
        observeCrossClientWrites(native.page, fixture.issueId, "electron", nativeActor.id));
      const initialResume = await lifecycleControl(creatorPage, fixture, "resume");
      expect(initialResume.receipt).toMatchObject({ scope: "issue", code: "resumed", queue_state: "active" });
      await refreshLifecyclePage(handlerPage, fixture);
      proxy = await createLifecycleRuntimeTransport(fixture.apiUrl);
      daemons = await startLifecycleDaemons(fixture, proxy);
      const evidence = await exerciseLifecycleRemoval(fixture, creatorPage, handlerPage, platform === "Web" ? "web" : "electron", {
        proxy, daemons, independent, peerId, peerBefore,
        async reconnectClients() {
          const desktop = native!;
          const contexts = [context, desktop.app.context()];
          // Renderer fetch is Chromium networking. The production native
          // account IPC has its own server-switch fence; assert both layers.
          await desktop.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
            .desktopAccountAcceptance.setBlocked(true));
          let failures: boolean[] = [];
          let blocked: { ok: boolean };
          try {
            for (const current of contexts) await current.setOffline(true);
            failures = await Promise.all([page, desktop.page].map((current) => current.evaluate(async (url) => {
              try { await fetch(url, { cache: "no-store", credentials: "include" }); return false; }
              catch { return true; }
            }, `${fixture.apiUrl}/api/config`)));
            expect(failures).toEqual([true, true]);
            blocked = await desktop.page.evaluate((path) => (window as unknown as {
              desktopAPI: { openAccount(value: string): Promise<{ ok: boolean }> };
            }).desktopAPI.openAccount(path), lifecyclePath(fixture));
            expect(blocked).toEqual({ ok: false });
          } finally {
            for (const current of contexts) await current.setOffline(false);
            await desktop.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
              .desktopAccountAcceptance.setBlocked(false));
          }
          const status = await Promise.all([page, native!.page].map((current) => current.evaluate(async (url) =>
            (await fetch(url, { cache: "no-store", credentials: "include" })).status, `${fixture.apiUrl}/api/config`)));
          expect(status).toEqual([200, 200]);
          const restored = await desktop.page.evaluate((path) => (window as unknown as {
            desktopAPI: { openAccount(value: string): Promise<{ ok: boolean }> };
          }).desktopAPI.openAccount(path), lifecyclePath(fixture));
          expect(restored).toEqual({ ok: true });
          reconnects.push({ web_fetch_failed: failures[0]!, electron_renderer_fetch_failed: failures[1]!,
            native_account_ipc_blocked: !blocked!.ok, restored_fetch_status: status, native_account_ipc_restored: restored.ok });
        },
        async capture(stage, currentPage, focus) {
          await focus.scrollIntoViewIfNeeded();
          await expect(focus).toBeVisible();
          const path = info.outputPath(`${platform.toLowerCase()}-${stage}.png`);
          if (currentPage === native!.page) await nativeScreenshot(native!.app, currentPage, path);
          else await currentPage.screenshot({ path, animations: "disabled" });
          captures.push({ stage, client: currentPage === native!.page ? "electron" : "web", path });
        },
      });
      expect(errors).toEqual([]);
      expect(native.errors).toEqual([]);
      expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
      expect(reconnects).toHaveLength(2);
      await info.attach("lifecycle-control-identities", { contentType: "application/json", body: JSON.stringify({ ...evidence,
        creator_platform: platform, sources, initial_resume: initialResume, captures, reconnects,
        writes: requestObservers.flatMap((observer) => observer.writes), environment: { api: fixture.apiUrl, production_web: origin,
          native_renderer: process.env.COLLABORATION_ELECTRON_RENDERER_URL, native_os: process.platform,
          execution_os: "actual Linux namespaces with explicit test-owned stdlib provider", provider_account_access: false,
          renderer_network: "Chromium fetch interrupted on both clients",
          native_ipc_boundary: "Production window:open-account server-switch rejection and explicit restore; independent of renderer fetch" },
        remaining_AC34: "No same-lane squad alias or whole-pause/new-target journey in this bounded removal slice",
        remaining_AC57: "No complete/cancel live-run UI in this bounded removal slice" }, null, 2) });
    } catch (error) {
      await info.attach("lifecycle-control-attempt", { contentType: "application/json", body: JSON.stringify({ creator_platform: platform, sources,
        issue_id: fixture.issueId, writes: requestObservers.flatMap((observer) => observer.writes),
        runtime_transport: proxy?.evidence(), provider_observations: daemons?.evidence(), captures, reconnects, errors }, null, 2) });
      throw error;
    } finally {
      // Process/socket cleanup only owns this fixture and is never counted as
      // product Stop or natural proof acceptance in the completed evidence.
      await daemons?.close();
      await proxy?.close();
      for (const observer of requestObservers) observer.dispose();
      await native?.app.close().catch(() => undefined);
      await context.close();
      rmSync(profile, { recursive: true, force: true });
    }
  });
}
