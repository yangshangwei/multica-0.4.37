import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect, type BrowserContext } from "@playwright/test";
import { openDesktop, deepLink, type NativeHarness } from "./fixtures/collaboration-native";
import type { HandoffClients } from "./fixtures/collaboration-offline-handoff";
import { crossClientEnvironmentReady } from "./fixtures/collaboration-cross-client-execution";
import { notificationRuntimeTransport, publishOfflineRunResults, publishOfflineWorkflow, readOfflineResults,
  removeOfflineResultViewer, seedOfflineResultNotifications } from "./fixtures/collaboration-offline-result-notifications";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

for (const platform of ["Web", "Electron"] as const) {
  test(`${platform} offline external participant logs in to information, acceptance and proven AI result notifications without source inbox duplication`, async ({ browser }, info) => {
    test.setTimeout(300_000);
    test.skip(!crossClientEnvironmentReady(), "Requires task-owned password production services and explicit test-built Linux executables");
    const state = await seedOfflineResultNotifications();
    const { fixture, recipient } = state;
    const origin = info.project.use.baseURL!;
    expect(new URL(origin).hostname).toBe(new URL(fixture.apiUrl).hostname);
    const contexts = new Set<BrowserContext>();
    const natives = new Set<Awaited<ReturnType<typeof openDesktop>>>();
    const profiles: string[] = [], errors: string[] = [];
    const sourcePaths = ["e2e/collaboration-offline-result-notifications.spec.ts", "e2e/fixtures/collaboration-offline-result-notifications.ts",
      "e2e/fixtures/collaboration-offline-handoff.ts", "e2e/fixtures/collaboration-lifecycle-controls.ts", "e2e/fixtures/collaboration-cross-client-execution.ts",
      "e2e/fixtures/collaboration-native.ts", "e2e/fixtures/collaboration-cross-client-provider.go"];
    const sources = sourcePaths.map((path) => ({ path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") }));
    const binarySources = [process.env.COLLABORATION_CROSS_CLIENT_DAEMON!, process.env.COLLABORATION_CROSS_CLIENT_PROVIDER!]
      .map((path) => ({ path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") }));
    const frames: { type: string; keys: string[] }[] = [];
    const clients: HandoffClients = { async open(actor, path) {
      if (platform === "Web") {
        const context = await browser.newContext(); contexts.add(context);
        const page = await context.newPage();
        page.on("pageerror", (error) => errors.push(error.message));
        await actor.authenticateWeb(page);
        page.on("websocket", (socket) => {
          if (new URL(socket.url()).searchParams.get("scope") !== "account") return;
          socket.on("framereceived", ({ payload }) => {
            try {
              const value = JSON.parse(payload.toString()) as { type: string; payload?: Record<string, unknown> };
              if (value.type === "subscribe_ack" || value.type.startsWith("collaboration:")) frames.push({ type: value.type, keys: Object.keys(value.payload ?? {}) });
            } catch { /* Non-JSON transport frames carry no acceptance evidence. */ }
          });
        });
        await page.goto(new URL(path, origin).href);
        return { page, navigate: async (next) => { await page.goto(new URL(next, origin).href); }, close: async () => { await context.close(); contexts.delete(context); } };
      }
      const profile = mkdtempSync(join(tmpdir(), "multica-offline-results-")); profiles.push(profile);
      const token = await actor.login();
      const native = await openDesktop(profile, { token, deepLink: `multica://account${path.slice("/account".length)}` });
      natives.add(native);
      native.page.on("pageerror", (error) => errors.push(error.message));
      native.page.on("websocket", (socket) => {
        if (new URL(socket.url()).searchParams.get("scope") !== "account") return;
        socket.on("framereceived", ({ payload }) => {
          try {
            const value = JSON.parse(payload.toString()) as { type: string; payload?: Record<string, unknown> };
            if (value.type === "subscribe_ack" || value.type.startsWith("collaboration:")) frames.push({ type: value.type, keys: Object.keys(value.payload ?? {}) });
          } catch { /* Keep only typed, content-free account hints. */ }
        });
      });
      await native.start();
      return { page: native.page, navigate: async (next) => { await deepLink(native.app, next); }, close: async () => {
        expect(native.errors).toEqual([]);
        expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
        expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
        await native.app.close(); natives.delete(native);
      } };
    } };
    let transport: Awaited<ReturnType<typeof notificationRuntimeTransport>> | undefined;
    const evidence: Record<string, unknown> = { sources, binary_sources: binarySources, platform, issue_id: fixture.issueId,
      recipient_id: recipient.id, environment: { api: fixture.apiUrl, production_web: origin, native_renderer: platform === "Electron" ? process.env.COLLABORATION_ELECTRON_RENDERER_URL : null,
        real_provider_account_access: false, provider: "Existing explicit Linux stdlib executable; genuine failed/incomplete record after withheld outcome", installed_full_main: false } };
    try {
      const prior = await clients.open(recipient, "/account?view=participating");
      await expect(prior.page.getByRole("region", { name: "Collaboration issues", exact: true })).toBeVisible();
      await prior.close();
      expect(prior.page.isClosed()).toBe(true);
      const before = recipient.logins;
      evidence.offline_workflow = await publishOfflineWorkflow(state);
      expect(recipient.logins).toBe(before);
      const sourceContext = await browser.newContext(); contexts.add(sourceContext);
      const sourcePage = await sourceContext.newPage();
      sourcePage.on("pageerror", (error) => errors.push(error.message));
      await fixture.webActor.authenticateWeb(sourcePage);
      await sourcePage.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
      await sourcePage.goto(new URL(state.accountPath, origin).href);
      transport = await notificationRuntimeTransport(fixture.apiUrl);
      evidence.proven_results = await publishOfflineRunResults(state, sourcePage, transport);
      expect(recipient.logins).toBe(before);
      const read = await readOfflineResults(state, clients);
      expect(recipient.logins).toBe(before + 1);
      evidence.recipient_login_after_six_events = true;
      evidence.archived = read.archived;
      evidence.source_isolation = read.source_isolation;
      const frameCount = frames.length;
      evidence.removal = await removeOfflineResultViewer(state, read.session, read.archived);
      await expect.poll(() => frames.slice(frameCount).some((frame) => frame.type === "collaboration:access_changed"), { timeout: 15_000 }).toBe(true);
      evidence.account_frames = frames;
      await read.session.close();
      expect(errors).toEqual([]);
      evidence.page_errors = errors;
    } finally {
      if (transport) evidence.transport_final = transport.evidence();
      await info.attach("offline-result-notification-identities", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
      await transport?.close();
      for (const context of contexts) await context.close().catch(() => undefined);
      for (const native of natives) await native.app.close().catch(() => undefined);
      for (const profile of profiles) rmSync(profile, { recursive: true, force: true });
    }
  });
}
