import assert from "node:assert/strict";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { mkdtemp, mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, extname, join, resolve } from "node:path";
import { _electron as electron, expect } from "@playwright/test";

const root = process.cwd();
const artifactDir = resolve(root, ".omx/state/desktop-connection-recovery");
const rendererDir = resolve(root, "apps/desktop/out/renderer");
const require = createRequire(import.meta.url);
const electronLoader = join(dirname(require.resolve("playwright-core")), "lib/server/electron/loader.js");
const esbuild = require(require.resolve("esbuild", { paths: [join(root, "apps/desktop/node_modules/vite")] }));
const helperPath = join(artifactDir, "native-helpers.cjs");
await mkdir(artifactDir, { recursive: true });
await esbuild.build({
  stdin: { contents: [
    'export * from "./apps/desktop/src/main/server-switch";',
    'export * from "./apps/desktop/src/main/runtime-config-loader";',
    'export * from "./apps/desktop/src/main/runtime-config-editor";',
    'export * from "./apps/desktop/src/shared/runtime-config";',
    'export * from "./apps/desktop/src/shared/main-renderer-messages";',
    'export * from "./apps/desktop/src/shared/issue-window";',
  ].join("\n"), resolveDir: root, loader: "ts" },
  outfile: helperPath, bundle: true, platform: "node", format: "cjs", external: ["electron"],
});
await readFile(join(rendererDir, "index.html"));
const listen = async (server) => {
  await new Promise((done) => server.listen(0, "127.0.0.1", done));
  return `http://127.0.0.1:${server.address().port}`;
};
let rendererUrl;
const renderer = createServer(async (request, response) => {
  try {
    const pathname = decodeURIComponent(new URL(request.url, "http://127.0.0.1").pathname);
    let target = resolve(rendererDir, `.${pathname}`);
    if (!target.startsWith(`${rendererDir}/`)) target = join(rendererDir, "index.html");
    const types = { ".html": "text/html", ".js": "application/javascript", ".css": "text/css", ".woff2": "font/woff2", ".svg": "image/svg+xml", ".png": "image/png" };
    let body;
    try { body = await readFile(target); }
    catch { target = join(rendererDir, "index.html"); body = await readFile(target); }
    response.setHeader("Content-Type", types[extname(target)] || "application/octet-stream");
    response.end(body);
  } catch { response.writeHead(500).end(); }
});
rendererUrl = await listen(renderer);
const originalToken = "recovery-original-fixture-credential";
const userId = "00000000-0000-4000-8000-000000000001";
const user = { id: userId, name: "Recovery fixture", email: "fixture@example.invalid", onboarded_at: "2026-10-12T00:00:00Z" };
function fixture(overrides = {}) {
  const state = { me: "ok", config: "ok", workspaces: "ok", device: "ok", deviceAuth: false, ...overrides };
  const requests = [];
  const pending = [];
  const reply = (response, pathname) => {
    if (response.destroyed) return;
    const field = pathname === "/api/config" ? "config" : pathname === "/api/me" ? "me" : pathname === "/api/workspaces" ? "workspaces" : pathname === "/auth/device" ? "device" : null;
    const mode = field && state[field];
    if (mode === "hang") { pending.push([response, pathname]); return; }
    if (mode === "network") { response.destroy(); return; }
    response.setHeader("Content-Type", "application/json");
    if (mode === "error") { response.writeHead(503).end(JSON.stringify({ error: "fixture-private-response-secret", token: "fixture-private-body-token" })); return; }
    const body = pathname === "/health" ? { ok: true }
      : pathname === "/api/config" ? { auth_mode: "legacy", device_auth_available: state.deviceAuth, allow_signup: true, feature_flags: {}, server_version: "fixture" }
      : pathname === "/api/me" ? user
      : pathname === "/auth/device" ? { token: "recovery-device-fixture-credential", user }
      : pathname === "/api/inbox/unread-summary" ? { total_unread_count: 0, workspaces: [] }
      : [];
    response.end(JSON.stringify(body));
  };
  const server = createServer((request, response) => {
    response.setHeader("Access-Control-Allow-Origin", rendererUrl);
    response.setHeader("Access-Control-Allow-Headers", request.headers["access-control-request-headers"] || "*");
    response.setHeader("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS");
    if (request.method === "OPTIONS") { response.writeHead(204).end(); return; }
    const requestedPath = new URL(request.url, "http://127.0.0.1").pathname;
    const pathname = requestedPath.replace(/^\/long-endpoint-[^/]+(?=\/(?:api|auth|health))/, "");
    const log = { pathname, startedAt: Date.now(), oldCredential: request.headers.authorization === `Bearer ${originalToken}` };
    requests.push(log);
    response.on("close", () => { log.closedAt = Date.now(); });
    reply(response, pathname);
  });
  return { server, state, requests, release() { for (const [response, pathname] of pending.splice(0)) reply(response, pathname); } };
}
const apiA = fixture();
const apiB = fixture();
const originA = await listen(apiA.server);
const originB = await listen(apiB.server);
const results = [];
let app;
let profile;
let page;
let pageErrors = [];
const native = () => app.evaluate(() => globalThis.recoveryAcceptance.snapshot());
const count = (pathname) => apiA.requests.filter((request) => request.pathname === pathname).length;
async function boot({ locale = "en", token = originalToken, theme = "light", apiUrl = originA } = {}) {
  profile = await mkdtemp(join(tmpdir(), "multica-connection-recovery-"));
  const configPath = join(profile, "desktop.json");
  await writeFile(configPath, JSON.stringify({ schemaVersion: 1, apiUrl }));
  app = await electron.launch({
    executablePath: require(require.resolve("electron", { paths: [join(root, "apps/desktop")] })),
    // A custom executable path skips Playwright's automatic loader injection.
    args: ["-r", electronLoader, join(dirname(import.meta.filename), "electron.cjs")],
    timeout: 30_000,
    env: { ...process.env, RECOVERY_ACCEPTANCE_PROFILE: profile,
      RECOVERY_ACCEPTANCE_CONFIG: configPath, RECOVERY_ACCEPTANCE_RENDERER: rendererUrl,
      RECOVERY_ACCEPTANCE_PRELOAD: join(root, "apps/desktop/out/preload/index.js"),
      RECOVERY_ACCEPTANCE_HELPERS: helperPath, RECOVERY_ACCEPTANCE_LOCALE: locale },
  });
  pageErrors = [];
  app.context().on("page", (createdPage) => {
    createdPage.on("pageerror", (error) => pageErrors.push(error.message));
  });
  await app.context().addInitScript(({ token, locale, theme, rendererOrigin }) => {
    if (location.origin !== rendererOrigin) return;
    if (!localStorage.getItem("recovery-fixture-seeded")) {
      if (token) localStorage.setItem("multica_token", token);
      localStorage.setItem("multica-locale", locale === "en" ? "en" : "zh-Hans");
      localStorage.setItem("theme", theme);
      localStorage.setItem("recovery-fixture-seeded", "true");
    }
  }, { token, locale, theme, rendererOrigin: rendererUrl });
  await app.evaluate(async () => {
    await globalThis.recoveryAcceptanceReady;
    await globalThis.recoveryAcceptance.load();
  });
  page = await app.firstWindow();
  console.log(JSON.stringify({ booted: true, locale, hasCredential: Boolean(token) }));
}
async function finish() {
  assert.equal((await native()).daemonStarts, 0);
  assert.deepEqual(pageErrors, []);
  await app.close();
  app = null;
  await rm(profile, { recursive: true, force: true });
  profile = null;
}
async function capture(name) {
  await page.screenshot({ path: join(artifactDir, `${name}.png`), animations: "disabled" });
  const layout = await page.evaluate(() => ({ width: innerWidth, height: innerHeight,
    horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
    mainOverflow: document.querySelector("main")?.scrollWidth > innerWidth,
    controls: [...document.querySelectorAll("button,input,summary")].map((element) => ({
      name: element.getAttribute("aria-label") || element.textContent || "input",
      rect: { width: element.getBoundingClientRect().width, height: element.getBoundingClientRect().height },
    })),
  }));
  assert.equal(layout.horizontalOverflow, false, `${name}: horizontal overflow`);
  await writeFile(join(artifactDir, `${name}-layout.json`), JSON.stringify(layout, null, 2));
}
const retry = () => page.getByRole("button", { name: "Retry now", exact: true });
const change = () => page.getByRole("button", { name: "Change server", exact: true });
const editor = () => page.getByRole("textbox");
const retainedToken = () => page.evaluate(() => localStorage.getItem("multica_token"));

try {
  apiA.state.me = "hang";
  await boot();
  const started = Date.now();
  await expect(change()).toBeVisible({ timeout: 15_000 });
  assert.ok(Date.now() - started < 14_000, "Hanging boot must reach recovery at its request deadline");
  assert.equal(await retainedToken(), originalToken);
  await page.getByText("Connection details", { exact: true }).click();
  const details = await page.locator("body").innerText();
  assert.match(details, /0\.4\.40-recovery-test/);
  assert.match(details, /respond in time|timed out|timeout/i);
  for (const secret of [originalToken, "fixture-private-response-secret", "fixture-private-body-token"]) assert.ok(!details.includes(secret));
  await page.getByRole("button", { name: "Copy details", exact: true }).click();
  await expect(page.getByText("Copied", { exact: true })).toBeVisible();
  const clipboardText = await app.evaluate(({ clipboard }) => clipboard.readText());
  assert.match(clipboardText, /0\.4\.40-recovery-test/);
  assert.ok(!clipboardText.includes(originalToken));
  await capture("recovery-en-normal");
  await app.evaluate(() => globalThis.recoveryAcceptance.resize(420, 640));
  await capture("recovery-en-narrow");
  await app.evaluate(() => globalThis.recoveryAcceptance.resize(1000, 780));
  await page.evaluate(() => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: {
      writeText: async () => { throw new Error("Fixture clipboard failure"); },
    } });
    document.execCommand = () => false;
  });
  await page.getByRole("button", { name: "Copy details", exact: true }).click();
  await expect(page.getByText("Could not copy. Select the details and copy them manually.", { exact: true })).toBeVisible();
  await expect(retry()).toBeEnabled({ timeout: 15_000 });
  const beforeRetry = count("/api/me");
  await retry().dblclick();
  await expect(page.getByRole("button", { name: /Retrying|Connecting/, exact: true })).toBeDisabled();
  await expect.poll(() => count("/api/me")).toBe(beforeRetry + 1);
  await expect(change()).toBeEnabled();
  await change().focus();
  await page.keyboard.press("Enter");
  await expect(editor()).toHaveValue(originA);
  await editor().fill("invalid-address");
  await page.getByRole("button", { name: "Test connection", exact: true }).click();
  await expect(editor()).toHaveAttribute("aria-invalid", "true");
  assert.equal((await native()).nativeTests, 0);
  const unavailable = "http://127.0.0.1:1";
  await editor().fill(unavailable);
  await page.getByRole("button", { name: "Test connection", exact: true }).click();
  await expect(page.getByRole("button", { name: "Test connection", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "Save and connect", exact: true }).click();
  await expect(page.getByRole("button", { name: "Save and connect", exact: true })).toBeEnabled();
  assert.equal((await native()).apiUrl, originA);
  assert.equal((await native()).resets, 0);
  assert.equal(await retainedToken(), originalToken);
  const unsaved = `${originB}/unsaved-address`;
  await editor().fill(unsaved);
  user.language = "zh-Hans";
  apiA.state.me = "ok";
  apiA.release();
  await expect.poll(async () => (await native()).authSessions.includes(userId)).toBe(true);
  await expect.poll(() => page.evaluate(() => localStorage.getItem("multica-locale"))).toBe("zh-Hans");
  await expect(editor()).toHaveValue(unsaved);
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await capture("editor-en-preserved");
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await expect(change()).not.toBeVisible();
  results.push({ scenario: "identity deadline, retained credential, details/copy, duplicate retry, invalid endpoint and editor persistence across recovered language preference", passed: true });
  await finish();
  delete user.language;

  apiA.state.me = "error";
  await boot({ locale: "zh-CN", theme: "dark" });
  await expect(page.getByRole("button", { name: "更换服务器", exact: true })).toBeVisible();
  await page.getByText("连接详情", { exact: true }).click();
  await expect(page.getByText("503", { exact: true })).toBeVisible();
  const serverDetails = await page.locator("body").innerText();
  for (const secret of [originalToken, "fixture-private-response-secret", "fixture-private-body-token"]) assert.ok(!serverDetails.includes(secret));
  await capture("recovery-zh-dark");
  const other = app.waitForEvent("window");
  await app.evaluate(() => globalThis.recoveryAcceptance.openIssue());
  const child = await other;
  await child.getByRole("button", { name: "更换服务器", exact: true }).click();
  await expect(editor()).toHaveValue(originA);
  assert.equal((await native()).handoffs, 1);
  await expect(child.getByRole("textbox")).toHaveCount(0);
  await editor().fill(originB);
  await page.getByRole("button", { name: "测试连接", exact: true }).click();
  await expect(page.getByRole("button", { name: "测试连接", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "保存并连接", exact: true }).click();
  await expect.poll(async () => (await native()).apiUrl).toBe(originB);
  await expect.poll(async () => (await native()).blocked).toBe(false);
  await expect.poll(async () => await retainedToken()).toBe(null);
  assert.equal((await native()).resets, 1);
  assert.equal((await native()).windows, 1);
  assert.equal(apiB.requests.some((request) => request.oldCredential), false);
  assert.equal(child.isClosed(), true);
  results.push({ scenario: "Chinese/dark, child-to-main editor, native validation/reset/save/reload, no credential to replacement server", passed: true });
  await finish();

  const longEndpoint = `${originA}/long-endpoint-${"department-".repeat(25)}`;
  await boot({ apiUrl: longEndpoint });
  await expect(change()).toBeVisible();
  await app.evaluate(() => globalThis.recoveryAcceptance.resize(420, 640));
  await page.getByText("Connection details", { exact: true }).click();
  await capture("recovery-en-long-endpoint");
  await change().click();
  await expect(editor()).toHaveValue(longEndpoint);
  await capture("editor-en-long-endpoint");
  results.push({ scenario: "long configured endpoint, expanded details and editor at 420×640", passed: true });
  await finish();

  apiA.state.me = "ok";
  apiA.state.config = "hang";
  await boot({ token: null });
  await expect(change()).toBeVisible({ timeout: 15_000 });
  assert.equal(await retainedToken(), null);
  await expect(page.getByText("Check your network and server address. You can retry or change the server.", { exact: true })).toBeVisible();
  await capture("recovery-en-tokenless");
  results.push({ scenario: "tokenless configuration deadline and neutral recovery", passed: true });
  await finish();

  apiA.state.config = "ok";
  apiA.state.deviceAuth = true;
  apiA.state.device = "hang";
  await boot({ token: null });
  await expect(change()).toBeVisible({ timeout: 15_000 });
  apiA.state.device = "ok";
  apiA.release();
  await expect.poll(async () => (await native()).authSessions.includes(userId), { timeout: 15_000 }).toBe(true);
  assert.equal(await retainedToken(), "recovery-device-fixture-credential");
  results.push({ scenario: "device login deadline and successful automatic recovery", passed: true });
  await finish();

  apiA.state.deviceAuth = false;
  apiA.state.workspaces = "error";
  await boot();
  await expect(change()).toBeVisible({ timeout: 15_000 });
  assert.ok((await native()).authSessions.includes(userId));
  assert.equal(await retainedToken(), originalToken);
  await capture("recovery-en-workspaces");
  apiA.state.workspaces = "ok";
  await expect(change()).not.toBeVisible({ timeout: 20_000 });
  results.push({ scenario: "workspace list failure keeps authenticated credential and recovers automatically", passed: true });
  await finish();
  await writeFile(join(artifactDir, "acceptance.json"), JSON.stringify({
    results, renderer: rendererDir, electron: "39", isolated: true,
    productionHelpers: ["probeServer", "ServerSwitchCoordinator", "parseRuntimeConfig", "loadRuntimeConfig", "saveRuntimeConfig", "clearServerCookies", "MainRendererMessageQueue", "openRuntimeConfigEditorFromWindow"],
    requests: [...apiA.requests, ...apiB.requests].map(({ pathname, startedAt, closedAt, oldCredential }) => ({ pathname, startedAt, closedAt, oldCredential })),
  }, null, 2));
  console.log(JSON.stringify({ passed: results.length, artifacts: artifactDir, isolated: true }));
} catch (error) {
  if (page && !page.isClosed()) {
    await page.screenshot({ path: join(artifactDir, "failure.png") }).catch(() => {});
    await writeFile(join(artifactDir, "failure.txt"), await page.locator("body").innerText()).catch(() => {});
  }
  throw error;
} finally {
  if (app) await app.close().catch(() => {});
  if (profile) await rm(profile, { recursive: true, force: true });
  for (const server of [renderer, apiA.server, apiB.server]) {
    server.closeAllConnections();
    await new Promise((done) => server.close(done));
  }
}
