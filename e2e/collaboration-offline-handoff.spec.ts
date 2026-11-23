import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect, type BrowserContext } from "@playwright/test";
import { deepLink, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import { exerciseOfflineHandoff, offlineHandoffEnvironment, seedOfflineHandoff, type HandoffClients } from "./fixtures/collaboration-offline-handoff";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

for (const platform of ["Web", "Electron"] as const) {
  test(`${platform} offline invite and assignment need no acceptance; transferred-away work survives archive and fresh login until removal`, async ({ browser }, info) => {
    test.setTimeout(240_000);
    test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires task-owned password production services");
    offlineHandoffEnvironment();
    const fixture = await seedOfflineHandoff();
    const origin = info.project.use.baseURL!;
    expect(["localhost", "127.0.0.1"]).toContain(new URL(origin).hostname);
    const contexts = new Set<BrowserContext>();
    const natives = new Set<Awaited<ReturnType<typeof openDesktop>>>();
    const profiles: string[] = [];
    const errors: string[] = [];
    const sources = ["e2e/collaboration-offline-handoff.spec.ts", "e2e/fixtures/collaboration-offline-handoff.ts"]
      .map((path) => ({ path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") }));
    const clients: HandoffClients = { async open(actor, path) {
      if (platform === "Web") {
        const context = await browser.newContext();
        contexts.add(context);
        const page = await context.newPage();
        page.on("pageerror", (error) => errors.push(error.message));
        await actor.authenticateWeb(page);
        await page.goto(new URL(path, origin).href);
        return { page, navigate: async (next) => { await page.goto(new URL(next, origin).href); }, close: async () => { await context.close(); contexts.delete(context); } };
      }
      const renderer = process.env.COLLABORATION_ELECTRON_RENDERER_URL!;
      expect(["localhost", "127.0.0.1"]).toContain(new URL(renderer).hostname);
      expect((await fetch(renderer)).status).toBe(200);
      const token = await actor.login();
      const profile = mkdtempSync(join(tmpdir(), "multica-offline-handoff-"));
      profiles.push(profile);
      const native = await openDesktop(profile, { token, deepLink: `multica://account${path.slice("/account".length)}` });
      natives.add(native);
      native.page.on("pageerror", (error) => errors.push(error.message));
      await native.start();
      return { page: native.page, navigate: async (next) => { await deepLink(native.app, next); }, close: async () => {
        expect(native.errors).toEqual([]);
        expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
        expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
        await native.app.close(); natives.delete(native);
      } };
    } };
    try {
      const evidence = await exerciseOfflineHandoff(fixture, clients);
      expect(errors).toEqual([]);
      await info.attach("offline-handoff-identities", { contentType: "application/json", body: JSON.stringify({ ...evidence, platform,
        source: sources, environment: { api: fixture.apiUrl, production_web: origin,
          electron_renderer: platform === "Electron" ? process.env.COLLABORATION_ELECTRON_RENDERER_URL : null,
          electron_os: platform === "Electron" ? process.platform : null, provider_account_access: false }, errors }, null, 2) });
    } finally {
      for (const context of contexts) await context.close().catch(() => undefined);
      for (const native of natives) await native.app.close().catch(() => undefined);
      for (const profile of profiles) rmSync(profile, { recursive: true, force: true });
    }
  });
}
