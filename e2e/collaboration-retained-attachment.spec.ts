import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect, type Browser, type BrowserContext, type Page, type TestInfo } from "@playwright/test";
import type { TestApiClient } from "./fixtures";
import { deepLink, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import { exerciseRetainedAttachments, exerciseTypedCreation, retainedAttachmentEnvironment, seedRetainedAttachments,
  type RetainedSession } from "./fixtures/collaboration-retained-attachments";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

const sourcePaths = ["e2e/collaboration-retained-attachment.spec.ts", "e2e/fixtures/collaboration-retained-attachments.ts",
  "e2e/fixtures/collaboration-control-journey.ts", "e2e/fixtures/collaboration-native.ts", "e2e/fixtures/collaboration-account-electron.mjs"];
const digest = (bytes: Buffer) => createHash("sha256").update(bytes).digest("hex");

function retainedClients(browser: Browser, info: TestInfo) {
  const { apiUrl, rendererUrl } = retainedAttachmentEnvironment();
  const webUrl = info.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(webUrl).hostname);
  expect(new URL(webUrl).hostname).toBe(new URL(apiUrl).hostname);
  const contexts = new Set<BrowserContext>();
  const natives = new Set<Awaited<ReturnType<typeof openDesktop>>>();
  const profiles: string[] = [];
  const errors: string[] = [];
  const requests: { platform: string; actor_id: string; path: string; method: string; request_id: string | null; ambient_workspace: boolean }[] = [];
  const closedNatives: { errors: string[]; no_ambient_workspace: boolean; daemon_starts: number }[] = [];
  const sources = sourcePaths.map((path) => ({ path, sha256: digest(readFileSync(path)) }));

  function observe(page: Page, platform: string, actorId: string) {
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (!/^\/api\/(collaboration\/|me\/workbench)/.test(path)) return;
      const headers = request.headers();
      requests.push({ platform, actor_id: actorId, path, method: request.method(), request_id: headers["idempotency-key"] ?? null,
        ambient_workspace: "x-workspace-id" in headers || "x-workspace-slug" in headers });
    });
  }

  async function web(actor: TestApiClient, actorId: string, path: string, authenticate: (page: Page) => Promise<void>): Promise<RetainedSession> {
    const context = await browser.newContext();
    contexts.add(context);
    const page = await context.newPage();
    observe(page, "Web", actorId);
    await authenticate(page);
    await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await page.goto(new URL(path, webUrl).href);
    return {
      page, platform: "Web", navigate: async (next) => { await page.goto(new URL(next, webUrl).href); },
      restart: async () => {
        const current = new URL(page.url());
        await context.close(); contexts.delete(context);
        return web(actor, actorId, `${current.pathname}${current.search}${current.hash}`, authenticate);
      },
      download: async (button, name, bytes) => {
        const pending = page.waitForEvent("download");
        await button.click();
        const download = await pending;
        expect(await download.failure()).toBeNull();
        expect(download.suggestedFilename()).toBe(name);
        const path = await download.path();
        if (!path) throw new Error("The browser did not retain its downloaded file");
        const actual = readFileSync(path);
        expect(actual).toEqual(bytes);
        return { sha256: digest(actual), filename: name, bytes: actual.length };
      },
    };
  }

  async function nativeFacts(native: Awaited<ReturnType<typeof openDesktop>>) {
    const daemonStarts = await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts);
    expect(native.errors).toEqual([]);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    const noAmbient = native.accountHeaders.every((headers) => !("x-workspace-id" in headers) && !("x-workspace-slug" in headers));
    expect(noAmbient).toBe(true);
    expect(daemonStarts).toBe(0);
    return { errors: [...native.errors], no_ambient_workspace: noAmbient, daemon_starts: daemonStarts };
  }

  async function electron(actor: TestApiClient, actorId: string, path?: string, existingProfile?: string): Promise<RetainedSession> {
    const profile = existingProfile ?? mkdtempSync(join(tmpdir(), "multica-retained-attachment-"));
    if (!existingProfile) profiles.push(profile);
    const native = await openDesktop(profile, { token: actor.getToken()!,
      ...(path ? { deepLink: `multica://account${path.slice("/account".length)}` } : {}) });
    natives.add(native);
    observe(native.page, "Electron", actorId);
    await native.start();
    return {
      page: native.page, platform: "Electron", navigate: async (next) => { await deepLink(native.app, next); },
      restart: async () => {
        closedNatives.push(await nativeFacts(native));
        await native.app.close(); natives.delete(native);
        return electron(actor, actorId, undefined, profile);
      },
      download: async (button, name, bytes, stage) => {
        const path = info.outputPath(`${stage}-${name}`);
        await native.app.evaluate(({ session }, output) => {
          const pending = new Promise<{ state: string; filename: string }>((resolve) => {
            session.defaultSession.once("will-download", (_event, item) => {
              item.setSavePath(output);
              item.once("done", (_event, state) => resolve({ state, filename: item.getFilename() }));
            });
          });
          (globalThis as unknown as { collaborationRetainedDownload: typeof pending }).collaborationRetainedDownload = pending;
        }, path);
        await button.click();
        const result = await native.app.evaluate(() => (globalThis as unknown as {
          collaborationRetainedDownload: Promise<{ state: string; filename: string }>;
        }).collaborationRetainedDownload);
        expect(result).toEqual({ state: "completed", filename: name });
        const actual = readFileSync(path);
        expect(actual).toEqual(bytes);
        return { sha256: digest(actual), filename: result.filename, bytes: actual.length };
      },
    };
  }

  return {
    web, electron,
    async evidence() {
      expect(errors).toEqual([]);
      expect(requests.length).toBeGreaterThan(0);
      expect(requests.every((request) => !request.ambient_workspace)).toBe(true);
      const currentNatives = [];
      for (const native of natives) currentNatives.push(await nativeFacts(native));
      return { sources, requests, errors, native_sessions: [...closedNatives, ...currentNatives],
        environment: { api: apiUrl, production_web: webUrl, renderer: natives.size > 0 ? rendererUrl : null,
          native_os: natives.size > 0 ? process.platform : null, real_provider_account_access: false, installed_full_main: false,
          source_preflight: ".trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-22/content-admin-preflight.json" } };
    },
    async close() {
      for (const context of contexts) await context.close().catch(() => undefined);
      for (const native of natives) await native.app.close().catch(() => undefined);
      for (const profile of profiles) rmSync(profile, { recursive: true, force: true });
    },
  };
}

for (const platform of ["Web", "Electron"] as const) {
  test(`${platform} typed human and owned AI creation stays atomic and idle from its first private upload through restart`, async ({ browser }, info) => {
    test.setTimeout(180_000);
    test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires task-owned password production services");
    const clients = retainedClients(browser, info);
    try {
      const fixture = await seedRetainedAttachments();
      const session = platform === "Web"
        ? await clients.web(fixture.owner, fixture.ownerId, "/account", fixture.authenticateOwnerWeb)
        : await clients.electron(fixture.owner, fixture.ownerId, "/account");
      const creation = await exerciseTypedCreation(session, fixture);
      await info.attach("typed-creation-identities", { contentType: "application/json", body: JSON.stringify({ ...creation, ...await clients.evidence() }, null, 2) });
    } finally { await clients.close(); }
  });
}

test("Web and Electron retain exact comment file bytes and immutable AI input after display removal, deletion, refresh and cross-client restart", async ({ browser }, info) => {
  test.setTimeout(240_000);
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires task-owned password production services");
  const clients = retainedClients(browser, info);
  try {
    const fixture = await seedRetainedAttachments();
    expect(await fixture.handler.getWorkspaces()).toEqual([]);
    const path = `/account?view=participating&issue=${fixture.issueId}`;
    const author = await clients.web(fixture.creator, fixture.creatorId, path, fixture.authenticateWeb);
    const reader = await clients.electron(fixture.handler, fixture.handlerId, path);
    const retained = await exerciseRetainedAttachments(author, reader, fixture);
    await info.attach("retained-attachment-identities", { contentType: "application/json", body: JSON.stringify({ ...retained, ...await clients.evidence() }, null, 2) });
  } finally { await clients.close(); }
});
