import { randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { _electron as electron, expect, type ElectronApplication, type Page, type Locator } from "@playwright/test";

const apiUrl = process.env.COLLABORATION_E2E_API_URL;
const root = resolve(process.env.COLLABORATION_ELECTRON_SOURCE_ROOT ?? resolve(import.meta.dirname, "../.."));

export interface NativeHarness {
  start(): void;
  deepLink(url: string): void;
  deliverLogin(token: string): void;
  setBlocked(blocked: boolean): void;
  evidence: { daemonStarts: number; nativeIssueRequests: { scope: string; path: string }[] };
}
export const responseEvidence = new WeakMap<ElectronApplication, { path: string; status: number }[]>();

export async function openDesktop(profile: string, options: { token?: string; deepLink?: string; serverUrl?: string } = {}) {
  const executablePath = join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin"
    ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron");
  const env: Record<string, string> = {
    ...Object.fromEntries(Object.entries(process.env).filter((entry): entry is [string, string] => entry[1] !== undefined)),
    COLLABORATION_ELECTRON_PROFILE: profile,
    COLLABORATION_E2E_API_URL: options.serverUrl ?? apiUrl!,
    COLLABORATION_ELECTRON_WS_URL: `${apiUrl!.replace("http", "ws")}/ws`,
    COLLABORATION_ELECTRON_DEEP_LINK: options.deepLink ?? "" };
  delete env.ELECTRON_RUN_AS_NODE;
  const app = await electron.launch({ executablePath, args: [join(root, "e2e/fixtures/collaboration-account-electron.mjs")], env, timeout: 20_000 });
  let page: Page;
  try {
    page = await app.firstWindow({ timeout: 15_000 });
  } catch (error) {
    await app.close().catch(() => undefined);
    throw error;
  }
  const responses: { path: string; status: number }[] = [];
  responseEvidence.set(app, responses);
  page.on("response", (response) => {
    const path = new URL(response.url()).pathname;
    if (path.startsWith("/api/")) responses.push({ path, status: response.status() });
  });
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const accountHeaders: Record<string, string>[] = [];
  page.on("request", (request) => {
    if (/\/api\/(collaboration\/|me\/workbench)/.test(request.url())) accountHeaders.push(request.headers());
  });
  await app.context().addInitScript(({ token, seed }) => {
    if (localStorage.getItem("desktop-account-test-seed") === seed) return;
    localStorage.setItem("desktop-account-test-seed", seed);
    if (token) localStorage.setItem("multica_token", token);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, { token: options.token, seed: randomUUID() });
  const start = () => app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.start());
  return { app, page, errors, accountHeaders, start };
}

export async function deepLink(app: ElectronApplication, path: string) {
  await app.evaluate((_electron, value) => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
    .desktopAccountAcceptance.deepLink(`multica://account${value.slice("/account".length)}`), path);
}

/** Workspace selection is contextual; the fixed shell switch first resumes it. */
export async function selectDesktopWorkspace(page: Page, workspace: { name: string }) {
  await page.getByRole("navigation", { name: "Application navigation", exact: true }).getByRole("button", { name: "Workspaces", exact: true }).click();
  const trigger = page.locator('[data-sidebar="header"] [data-sidebar="menu-button"]').first();
  await expect(trigger).toBeVisible();
  await trigger.click();
  await page.getByRole("menuitem").filter({ hasText: workspace.name }).click();
}

export async function nativeScreenshot(app: ElectronApplication, page: Page, path: string) {
  const window = await app.browserWindow(page);
  const png = await window.evaluate(async (target) => (await target.webContents.capturePage()).toPNG().toString("base64"));
  await window.dispose();
  writeFileSync(path, Buffer.from(png, "base64"));
}


/** Native zoom uses the physical compositor and the actual scroll container. */
export async function captureNativeSection(app: ElectronApplication, page: Page, focus: Locator, stage: string,
  outputPath: (name: string) => string, rootSelector = '[data-tab-scroll-root="account-detail"]') {
  const captures: Record<string, unknown>[] = [];
      const window = await app.browserWindow(page);
      try {
        for (const [size, width, height, zoom] of [["wide", 1380, 1120, 1], ["200-percent", 900, 1200, 2]] as const) {
          const native = await window.evaluate((target, value) => {
            target.webContents.setZoomFactor(value.zoom); target.setContentSize(value.width, value.height);
            return { zoomFactor: target.webContents.getZoomFactor(), bounds: target.getBounds(),
              contentBounds: target.getContentBounds(), contentSize: target.getContentSize() };
          }, { width, height, zoom });
          const observations: Record<string, unknown>[] = [];
          const diagnostic: Record<string, unknown> = { stage, size, width, height, zoom, native, observations };
          const diagnosticPath = outputPath(`desktop-${stage}-${size}-geometry.json`);
          const persistDiagnostic = () => writeFileSync(diagnosticPath,
            `${JSON.stringify(diagnostic, null, 2)}\n`, { mode: 0o600 });
          persistDiagnostic();
          await expect.poll(() => page.evaluate(() => innerWidth)).toBe(width / zoom);
          await page.evaluate(async () => {
            await document.fonts.ready;
            await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
          });
          const dimensions = await focus.evaluate((element, selector) => {
            const root = element.closest<HTMLElement>(selector);
            if (!root) throw new Error("Native capture requires the actual account detail scroll root");
            const box = element.getBoundingClientRect();
            return { focusHeight: box.height, availableHeight: root.clientHeight - 24,
              focus: box.toJSON(), root: root.getBoundingClientRect().toJSON(),
              viewport: { width: innerWidth, height: innerHeight, devicePixelRatio },
              scroll: { top: root.scrollTop, clientHeight: root.clientHeight, scrollHeight: root.scrollHeight,
                maxScroll: Math.max(0, root.scrollHeight - root.clientHeight) } };
          }, rootSelector);
          diagnostic.initial = dimensions;
          persistDiagnostic();
          // macOS bounds native windows to the display. At 200% long sections
          // need overlapping captures; prove complete scroll coverage instead
          // of pretending the whole section fits in one compositor frame.
          expect(dimensions.availableHeight).toBeGreaterThan(100);
          const last = Math.max(0, dimensions.focusHeight - dimensions.availableHeight);
          const offsets = [0];
          for (let offset = dimensions.availableHeight; offset < last; offset += dimensions.availableHeight) offsets.push(offset);
          if (last > 0) offsets.push(last);
          diagnostic.offsets = offsets;
          let coveredThrough = 0;
          for (const [part, offset] of offsets.entries()) {
            const scrollRequest = await focus.evaluate((element, { position, selector }) => {
              const root = element.closest<HTMLElement>(selector)!;
              const before = root.scrollTop;
              const delta = element.getBoundingClientRect().top - root.getBoundingClientRect().top - 12 + position;
              root.scrollTop += element.getBoundingClientRect().top - root.getBoundingClientRect().top - 12 + position;
              return { before, requestedTop: before + delta, immediateTop: root.scrollTop, requestedOffset: position };
            }, { position: offset, selector: rootSelector });
            await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
            const geometry = await focus.evaluate((element, selector) => {
              const box = element.getBoundingClientRect();
              const scrollRoot = element.closest<HTMLElement>(selector)!;
              const root = scrollRoot.getBoundingClientRect();
              return { viewport: { width: innerWidth, height: innerHeight }, focus: box.toJSON(), root: root.toJSON(),
                visibleStart: Math.max(0, root.top - box.top), visibleEnd: Math.min(box.height, root.bottom - box.top),
                scroll: { top: scrollRoot.scrollTop, clientHeight: scrollRoot.clientHeight, scrollHeight: scrollRoot.scrollHeight,
                  maxScroll: Math.max(0, scrollRoot.scrollHeight - scrollRoot.clientHeight) },
                currentFocusHeight: box.height, achievedOffset: root.top + 12 - box.top };
            }, rootSelector);
            observations.push({ part, offset, recordedAt: new Date().toISOString(), scrollRequest,
              coveredThroughBefore: coveredThrough, ...geometry });
            persistDiagnostic();
            expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
            expect(geometry.focus.x).toBeGreaterThanOrEqual(geometry.root.x);
            expect(geometry.focus.right).toBeLessThanOrEqual(geometry.root.right);
            expect(geometry.visibleStart).toBeLessThanOrEqual(coveredThrough + 1);
            expect(geometry.visibleEnd).toBeGreaterThan(geometry.visibleStart);
            coveredThrough = Math.max(coveredThrough, geometry.visibleEnd);
            const name = `desktop-${stage}-${size}${offsets.length > 1 ? `-part-${part + 1}` : ""}.png`;
            await nativeScreenshot(app, page, outputPath(name));
            captures.push({ name, width, height, zoom, ...geometry });
            diagnostic.coveredThrough = coveredThrough;
            diagnostic.lastCapture = name;
            persistDiagnostic();
          }
          expect(coveredThrough).toBeGreaterThanOrEqual(dimensions.focusHeight - 1);
        }
        await window.evaluate((target) => { target.webContents.setZoomFactor(1); target.setContentSize(1380, 1120); });
      } finally { await window.dispose(); }

  return captures;
}
