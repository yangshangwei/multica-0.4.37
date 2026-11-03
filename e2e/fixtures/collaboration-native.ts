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
  const measure = () => focus.evaluate((element, selector) => {
    const scrollRoot = element.closest<HTMLElement>(selector);
    if (!scrollRoot) throw new Error("Native capture requires the actual account detail scroll root");
    const box = element.getBoundingClientRect();
    const rootBox = scrollRoot.getBoundingClientRect();
    const rect = (value: DOMRect) => ({ x: value.x, y: value.y, width: value.width, height: value.height,
      top: value.top, right: value.right, bottom: value.bottom, left: value.left });
    return { focusHeight: box.height, availableHeight: scrollRoot.clientHeight - 24,
      focus: rect(box), root: rect(rootBox),
      viewport: { width: innerWidth, height: innerHeight, devicePixelRatio },
      scroll: { top: scrollRoot.scrollTop, clientHeight: scrollRoot.clientHeight, scrollHeight: scrollRoot.scrollHeight,
        maxScroll: Math.max(0, scrollRoot.scrollHeight - scrollRoot.clientHeight) },
      visibleStart: Math.max(0, rootBox.top - box.top), visibleEnd: Math.min(box.height, rootBox.bottom - box.top),
      currentFocusHeight: box.height, achievedOffset: rootBox.top + 12 - box.top };
  }, rootSelector);
  type Frame = Awaited<ReturnType<typeof measure>>;
  const sameLayout = (left: Frame, right: Frame) =>
    (["x", "width", "height"] as const).every((key) => Math.abs(left.focus[key] - right.focus[key]) <= 1) &&
    (["x", "y", "width", "height"] as const).every((key) => Math.abs(left.root[key] - right.root[key]) <= 1) &&
    left.scroll.clientHeight === right.scroll.clientHeight &&
    left.viewport.width === right.viewport.width && left.viewport.height === right.viewport.height;
  const samePart = (left: Frame, right: Frame) =>
    (["x", "y", "width", "height", "top", "right", "bottom", "left"] as const)
      .every((key) => Math.abs(left.focus[key] - right.focus[key]) <= 1 && Math.abs(left.root[key] - right.root[key]) <= 1) &&
    Math.abs(left.scroll.top - right.scroll.top) <= 1 && sameLayout(left, right);
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
          const stabilitySamples: Frame[] = [];
          diagnostic.stabilitySamples = stabilitySamples;
          await expect.poll(async () => {
            await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
            const current = await measure();
            const previous = stabilitySamples.at(-1);
            const stable = previous !== undefined && sameLayout(previous, current);
            stabilitySamples.push(current);
            diagnostic.layoutStable = stable;
            persistDiagnostic();
            return stable;
          }, { message: "Native capture requires two stable frame-spanning layout samples" }).toBe(true);
          const dimensions = stabilitySamples.at(-1);
          if (!dimensions) throw new Error("Native capture did not obtain a stable layout snapshot");
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
            const geometry = await measure();
            const observation: Record<string, unknown> = { part, offset, recordedAt: new Date().toISOString(), scrollRequest,
              coveredThroughBefore: coveredThrough, matchesFrozenLayout: sameLayout(dimensions, geometry), ...geometry };
            observations.push(observation);
            persistDiagnostic();
            expect(sameLayout(dimensions, geometry), "Native capture layout changed after freezing offsets").toBe(true);
            expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
            expect(geometry.focus.x).toBeGreaterThanOrEqual(geometry.root.x);
            expect(geometry.focus.right).toBeLessThanOrEqual(geometry.root.right);
            expect(geometry.visibleStart).toBeLessThanOrEqual(coveredThrough + 1);
            expect(geometry.visibleEnd).toBeGreaterThan(geometry.visibleStart);
            coveredThrough = Math.max(coveredThrough, geometry.visibleEnd);
            const name = `desktop-${stage}-${size}${offsets.length > 1 ? `-part-${part + 1}` : ""}.png`;
            await nativeScreenshot(app, page, outputPath(name));
            const afterCapture = await measure();
            observation.afterCapture = afterCapture;
            observation.samePartAfterCapture = samePart(geometry, afterCapture);
            observation.matchesFrozenAfterCapture = sameLayout(dimensions, afterCapture);
            persistDiagnostic();
            expect(samePart(geometry, afterCapture), "Native capture moved or resized the measured part").toBe(true);
            expect(sameLayout(dimensions, afterCapture), "Native capture layout changed during compositor capture").toBe(true);
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
