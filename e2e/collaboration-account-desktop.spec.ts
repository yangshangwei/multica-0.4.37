import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { createServer } from "node:http";
import pg from "pg";
import { _electron as electron, test, expect, type ElectronApplication, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// This suite proves native platform wiring against real account reads. Public
// collaboration creation is gated; the test-only DB seed is not creation AC67.
const apiUrl = process.env.COLLABORATION_E2E_API_URL;
const rendererUrl = process.env.COLLABORATION_ELECTRON_RENDERER_URL;
const databaseUrl = process.env.DATABASE_URL;
const root = resolve(import.meta.dirname, "..");
type Workspace = { id: string; slug: string; name: string };
type Issue = { id: string; title: string };
type TabSnapshot = {
  activeScope: string;
  activeWorkspaceSlug: string | null;
  sessionOwner: string | null;
  accountGroup: { tabs: { id: string; url: string; history: { stack: string[]; index: number }; memento: { scroll: Record<string, unknown> } }[] } | null;
  byWorkspace: Record<string, { tabs: { id: string; pinned: boolean }[] }>;
};
interface Fixture {
  token: string;
  viewerId: string;
  viewerProfile: Record<string, unknown>;
  workspaces: Workspace[];
  emptyToken: string;
  emptyViewerId: string;
  workspaceA: Workspace;
  workspaceB: Workspace;
  issues: Issue[];
}
interface NativeHarness {
  start(): void;
  deepLink(url: string): void;
  deliverLogin(token: string): void;
  setBlocked(blocked: boolean): void;
  evidence: { daemonStarts: number; nativeIssueRequests: { scope: string; path: string }[] };
}
let fixture: Fixture;
const responseEvidence = new WeakMap<ElectronApplication, { path: string; status: number }[]>();

interface ControlledApi {
  workspaces?: Workspace[];
  workspaceError?: boolean;
  workspaceWait?: Promise<void>;
  requiredAccountFlag?: "requires_account_setup" | "requires_password_change";
}

// Electron's CDP request fulfillment does not reliably preserve successful
// response metadata. Use real HTTP for controlled platform edge cases, forwarding
// all other traffic to the isolated backend. The real WS endpoint is unchanged.
async function controlledApi(overrides: ControlledApi) {
  const server = createServer(async (request, response) => {
    const path = new URL(request.url ?? "/", apiUrl!).pathname;
    const sendJSON = (status: number, value: unknown) => {
      response.writeHead(status, { "content-type": "application/json", "access-control-allow-origin": new URL(rendererUrl!).origin });
      response.end(JSON.stringify(value));
    };
    try {
      if (request.method === "GET" && path === "/api/workspaces") {
        await overrides.workspaceWait;
        if (overrides.workspaceError) return sendJSON(500, { error: "test workspace list unavailable" });
        if (overrides.workspaces) return sendJSON(200, overrides.workspaces);
      }
      if (request.method === "GET" && path === "/api/me" && overrides.requiredAccountFlag) {
        return sendJSON(200, { ...fixture.viewerProfile, [overrides.requiredAccountFlag]: true });
      }
      const headers = new Headers();
      for (const [key, value] of Object.entries(request.headers)) {
        if (value !== undefined && !["host", "connection", "content-length", "accept-encoding"].includes(key)) {
          headers.set(key, Array.isArray(value) ? value.join(", ") : value);
        }
      }
      headers.set("accept-encoding", "identity");
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const upstream = await fetch(new URL(request.url ?? "/", apiUrl), {
        method: request.method, headers, body: chunks.length ? Buffer.concat(chunks) : undefined,
      });
      const outgoing = Object.fromEntries(upstream.headers);
      delete outgoing["content-encoding"];
      delete outgoing["content-length"];
      delete outgoing["transfer-encoding"];
      response.writeHead(upstream.status, outgoing);
      response.end(Buffer.from(await upstream.arrayBuffer()));
    } catch {
      if (!response.headersSent) sendJSON(503, { error: "test proxy unavailable" });
      else response.end();
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Test API proxy did not bind TCP");
  return { url: `http://127.0.0.1:${address.port}`, close: async () => {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));
  } };
}

test.beforeAll(async () => {
  test.skip(!apiUrl || !rendererUrl || !databaseUrl || !process.env.NEXT_PUBLIC_API_URL,
    "Requires an explicitly isolated API/database and built Desktop renderer/preload");
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);
  for (const value of [apiUrl!, rendererUrl!, databaseUrl!]) {
    expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  }
  const preflight = await fetch(`${apiUrl}/api/config`, { method: "OPTIONS", headers: {
    Origin: new URL(rendererUrl!).origin,
    "Access-Control-Request-Method": "GET",
    "Access-Control-Request-Headers": "authorization,content-type",
  } });
  expect(preflight.headers.get("access-control-allow-origin")).toBe(new URL(rendererUrl!).origin);
  fixture = await seedFixture();
});

async function seedFixture(): Promise<Fixture> {
  const key = randomUUID().slice(0, 10);
  const source = new TestApiClient();
  await source.login(`desktop-source-${key}@example.invalid`, "Desktop source owner");
  await source.ensureWorkspace("Desktop external source", `desktop-source-${key}`);
  await source.markUserOnboarded();
  const sourceUser = await source.requestJSON<{ id: string }>("/api/me");
  const viewer = new TestApiClient();
  await viewer.login(`desktop-viewer-${key}@example.invalid`, "Desktop account viewer");
  const workspaceA = await viewer.ensureWorkspace("Desktop workspace A", `desktop-a-${key}`);
  await viewer.markUserOnboarded();
  const workspaceB = await viewer.requestJSON<Workspace>("/api/workspaces", { method: "POST", body: {
    name: "Desktop workspace B", slug: `desktop-b-${key}`,
  } });
  const viewerUser = await viewer.requestJSON<{ id: string } & Record<string, unknown>>("/api/me");
  const empty = new TestApiClient();
  await empty.login(`desktop-empty-${key}@example.invalid`, "Desktop viewer without a workspace");
  await empty.markUserOnboarded();
  const emptyUser = await empty.requestJSON<{ id: string }>("/api/me");
  expect(await empty.getWorkspaces()).toEqual([]);
  const issues: Issue[] = [];
  for (const label of ["A", "B"]) {
    const issue = await source.createIssue(`Desktop shared ${label} ${key}`, {
      description: Array.from({ length: 80 }, (_, i) => `Shared ${label} context paragraph ${i + 1}. This fixture provides real scrollable content for native restoration.`).join("\n\n"),
    });
    expect(typeof issue.id).toBe("string");
    issues.push({ id: issue.id, title: issue.title });
  }
  const db = new pg.Client(databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    for (const userId of [sourceUser.id, viewerUser.id, emptyUser.id]) {
      await db.query("INSERT INTO collaboration_account_fence (user_id) VALUES ($1) ON CONFLICT DO NOTHING", [userId]);
    }
    for (const issue of issues) {
      await db.query("UPDATE issue SET assignee_type = 'member', assignee_id = $2 WHERE id = $1", [issue.id, viewerUser.id]);
      await db.query("INSERT INTO issue_collaboration (issue_id) VALUES ($1)", [issue.id]);
      await db.query("INSERT INTO collaboration_session (issue_id) VALUES ($1)", [issue.id]);
      for (const userId of [sourceUser.id, viewerUser.id, emptyUser.id]) {
        await db.query("INSERT INTO issue_participant (issue_id, user_id) VALUES ($1, $2)", [issue.id, userId]);
      }
    }
    await db.query("COMMIT");
  } catch (error) {
    await db.query("ROLLBACK");
    throw error;
  } finally { await db.end(); }
  const token = viewer.getToken();
  const emptyToken = empty.getToken();
  if (!token || !emptyToken) throw new Error("Fixture authentication did not produce tokens");
  const accountRows = await empty.requestJSON<{ items: Issue[] }>("/api/me/workbench?view=participating");
  expect(accountRows.items.map((issue) => issue.id)).toEqual(expect.arrayContaining(issues.map((issue) => issue.id)));
  expect(await empty.requestJSON(`/api/collaboration/issues/${issues[0].id}`)).toMatchObject({ id: issues[0].id });
  return { token, emptyToken, viewerId: viewerUser.id, viewerProfile: viewerUser,
    workspaces: await viewer.getWorkspaces(), emptyViewerId: emptyUser.id, workspaceA, workspaceB, issues };
}

function accountPath(issue: Issue, view = "current") {
  return `/account?view=${view}&issue=${issue.id}`;
}

async function openDesktop(profile: string, options: { token?: string; deepLink?: string; serverUrl?: string } = {}) {
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

async function snapshot(page: Page): Promise<TabSnapshot> {
  return page.evaluate(() => JSON.parse(localStorage.getItem("multica_tabs") ?? "null").state);
}

async function deepLink(app: ElectronApplication, path: string) {
  await app.evaluate((_electron, value) => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
    .desktopAccountAcceptance.deepLink(`multica://account${value.slice("/account".length)}`), path);
}

async function nativeCloseShortcut(app: ElectronApplication, page: Page) {
  const window = await app.browserWindow(page);
  // Chromium's CDP keyboard dispatch does not exercise before-input-event.
  // Send native input through the actual BrowserWindow and production handler.
  await window.evaluate((target) => {
    target.focus();
    target.webContents.sendInputEvent({
      type: "keyDown", keyCode: "W", modifiers: [process.platform === "darwin" ? "meta" : "control"],
    });
  });
  await window.dispose();
}

async function nativeScreenshot(app: ElectronApplication, page: Page, path: string) {
  const window = await app.browserWindow(page);
  const png = await window.evaluate(async (target) => (await target.webContents.capturePage()).toPNG().toString("base64"));
  await window.dispose();
  writeFileSync(path, Buffer.from(png, "base64"));
}

async function attachFailure(app: ElectronApplication | undefined, testInfo: TestInfo) {
  const page = app?.windows()[0];
  if (!page || page.isClosed()) return;
  const imagePath = testInfo.outputPath("desktop-account-failure.png");
  await page.screenshot({ path: imagePath });
  await testInfo.attach("desktop-account-failure", { path: imagePath, contentType: "image/png" });
  const responsesPath = testInfo.outputPath("desktop-account-responses.json");
  writeFileSync(responsesPath, JSON.stringify(responseEvidence.get(app!) ?? [], null, 2));
  await testInfo.attach("desktop-account-failure-text", { body: await page.locator("body").innerText(), contentType: "text/plain" });
}

test("Desktop account links retain one tab across workspaces, popouts and restart", async ({ browserName: _browserName }, testInfo) => {
  test.setTimeout(120_000);
  const profile = mkdtempSync(join(tmpdir(), "multica-collaboration-desktop-"));
  const overrides: ControlledApi = {};
  const proxy = await controlledApi(overrides);
  let app: ElectronApplication | undefined;
  try {
    const native = await openDesktop(profile, { token: fixture.token, serverUrl: proxy.url });
    app = native.app;
    const page = native.page;
    await native.start();
    await expect(page.getByRole("heading", { name: "Collaboration", exact: true })).toBeVisible();
    await page.getByRole("combobox", { name: "Workspaces", exact: true }).selectOption(fixture.workspaceA.slug);
    await expect.poll(async () => (await snapshot(page)).activeWorkspaceSlug).toBe(fixture.workspaceA.slug);
    const workspaceTabId = (await snapshot(page)).byWorkspace[fixture.workspaceA.slug].tabs[0].id;
    await page.locator(`[data-tab-id="${workspaceTabId}"]`).click({ button: "right" });
    await page.getByText("Pin tab", { exact: true }).click();
    await page.getByRole("button", { name: "New tab", exact: true }).click();
    await page.screenshot({ path: testInfo.outputPath("desktop-workspace-a.png"), animations: "disabled" });
    await page.locator('nav[aria-label="Collaboration"]').getByRole("link", { name: "Collaboration", exact: true }).click();
    await page.getByRole("region", { name: "Collaboration issues" }).getByRole("link", { name: new RegExp(fixture.issues[0].title) }).click();
    await expect(page.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    const accountTabId = (await snapshot(page)).accountGroup!.tabs[0].id;
    const detail = page.locator('[data-tab-scroll-root="account-detail"]');
    await expect.poll(() => detail.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(500);
    await detail.evaluate((element) => { element.scrollTop = 320; });
    await page.getByRole("link", { name: "Participating", exact: true }).click();
    await detail.evaluate((element) => { element.scrollTop = 560; });
    await page.getByRole("button", { name: "Go back", exact: true }).click();
    await expect.poll(() => detail.evaluate((element) => element.scrollTop)).toBe(320);
    await deepLink(app, accountPath(fixture.issues[1]));
    await expect(page.getByRole("heading", { name: fixture.issues[1].title, exact: true })).toBeVisible();
    expect((await snapshot(page)).accountGroup!.tabs).toHaveLength(1);
    expect((await snapshot(page)).accountGroup!.tabs[0].id).toBe(accountTabId);
    const tabBounds = await page.locator(`[data-tab-id="${accountTabId}"]`).boundingBox();
    const forwardBounds = await page.getByRole("button", { name: "Go forward", exact: true }).boundingBox();
    expect(tabBounds!.x).toBeGreaterThanOrEqual(forwardBounds!.x + forwardBounds!.width);
    await expect(page.locator(`[data-tab-id="${accountTabId}"]`).getByRole("button", { name: "Collaboration", exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("desktop-account.png"), animations: "disabled" });

    const mainWindow = await app.browserWindow(page);
    await mainWindow.evaluate((window) => {
      window.setContentSize(900, 1000);
      window.webContents.setZoomFactor(2);
    });
    await expect.poll(() => page.evaluate(() => window.innerWidth)).toBe(450);
    await expect(page.getByRole("link", { name: "Back to list", exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
    // Native capture respects Electron zoom/device scale. CDP screenshots can
    // crop the upper-left quadrant after a native zoom change on Retina.
    await nativeScreenshot(app, page, testInfo.outputPath("desktop-account-200-percent.png"));
    await mainWindow.evaluate((window) => {
      window.webContents.setZoomFactor(1);
      window.setSize(1380, 960);
    });
    await mainWindow.dispose();

    await page.locator(`[data-tab-id="${accountTabId}"]`).click({ button: "right" });
    const childPromise = app.waitForEvent("window");
    await page.getByText("Open as new window", { exact: true }).click();
    const child = await childPromise;
    await expect(child.getByRole("heading", { name: fixture.issues[1].title, exact: true })).toBeVisible();
    await child.screenshot({ path: testInfo.outputPath("desktop-account-popout.png"), animations: "disabled" });
    await child.getByRole("region", { name: "Collaboration issues" }).getByRole("link", { name: new RegExp(fixture.issues[0].title) }).click();
    await expect(child.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: fixture.issues[1].title, exact: true })).toBeVisible();
    await Promise.all([child.waitForEvent("close", { timeout: 10_000 }), nativeCloseShortcut(app, child)]);
    await page.getByRole("combobox", { name: "Workspaces", exact: true }).selectOption(fixture.workspaceB.slug);
    await expect.poll(async () => (await snapshot(page)).activeWorkspaceSlug).toBe(fixture.workspaceB.slug);
    await deepLink(app, accountPath(fixture.issues[0]));
    await expect(page.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    const before = await snapshot(page);
    expect(Object.keys(before.byWorkspace)).toEqual(expect.arrayContaining([fixture.workspaceA.slug, fixture.workspaceB.slug]));
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    for (const headers of native.accountHeaders) {
      expect(headers["x-workspace-id"]).toBeUndefined();
      expect(headers["x-workspace-slug"]).toBeUndefined();
    }
    expect(native.errors).toEqual([]);
    expect(await app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    await app.close();
    const restarted = await openDesktop(profile, { token: fixture.token, serverUrl: proxy.url });
    app = restarted.app;
    await restarted.start();
    await expect(restarted.page.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    const after = await snapshot(restarted.page);
    expect(after.accountGroup!.tabs[0].id).toBe(accountTabId);
    expect(after.accountGroup!.tabs[0].history).toEqual(before.accountGroup!.tabs[0].history);
    expect(Object.keys(after.byWorkspace)).toEqual(Object.keys(before.byWorkspace));
    expect(after.byWorkspace[fixture.workspaceA.slug]).toEqual(before.byWorkspace[fixture.workspaceA.slug]);
    expect(after.byWorkspace[fixture.workspaceA.slug].tabs[0].pinned).toBe(true);
    await expect.poll(() => restarted.page.locator('[data-tab-scroll-root="account-detail"]').evaluate((element) => element.scrollTop)).toBe(320);
    // A controlled authoritative membership list isolates native eviction:
    // removing workspace A must retain account history and workspace B.
    overrides.workspaces = fixture.workspaces.filter((workspace) => workspace.id !== fixture.workspaceA.id);
    await restarted.page.reload();
    await expect(restarted.page.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    await expect.poll(async () => Object.keys((await snapshot(restarted.page)).byWorkspace)).not.toContain(fixture.workspaceA.slug);
    expect((await snapshot(restarted.page)).accountGroup!.tabs[0].id).toBe(accountTabId);
    expect((await snapshot(restarted.page)).byWorkspace[fixture.workspaceB.slug]).toBeDefined();
    expect(restarted.errors).toEqual([]);
  } catch (error) {
    await attachFailure(app, testInfo);
    throw error;
  } finally {
    await app?.close().catch(() => undefined);
    await proxy.close();
    rmSync(profile, { recursive: true, force: true });
  }
});

test("Desktop cold account links work without workspaces and survive a controlled list error", async ({ browserName: _browserName }, testInfo) => {
  test.setTimeout(90_000);
  const profile = mkdtempSync(join(tmpdir(), "multica-collaboration-empty-"));
  const overrides: ControlledApi = {};
  const proxy = await controlledApi(overrides);
  let app: ElectronApplication | undefined;
  try {
    const path = accountPath(fixture.issues[0], "participating");
    const native = await openDesktop(profile, { token: fixture.emptyToken, serverUrl: proxy.url, deepLink: `multica://account${path.slice(8)}` });
    app = native.app;
    await native.start();
    await expect(native.page.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    expect((await snapshot(native.page)).activeScope).toBe("account");
    expect(Object.keys((await snapshot(native.page)).byWorkspace)).toEqual([]);
    await native.page.screenshot({ path: testInfo.outputPath("desktop-zero-workspaces.png"), animations: "disabled" });
    // Only the workspace list is controlled; identity and account content still
    // come from the actual isolated backend.
    overrides.workspaceError = true;
    await native.page.reload();
    await expect(native.page.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    await expect(native.page.getByRole("button", { name: "Refresh", exact: true }).first()).toBeVisible();
    await native.page.screenshot({ path: testInfo.outputPath("desktop-workspace-list-error.png"), animations: "disabled" });
    expect(native.errors).toEqual([]);
    await Promise.all([native.page.waitForEvent("close", { timeout: 10_000 }), nativeCloseShortcut(app, native.page)]);
  } catch (error) {
    await attachFailure(app, testInfo);
    throw error;
  } finally {
    await app?.close().catch(() => undefined);
    await proxy.close();
    rmSync(profile, { recursive: true, force: true });
  }
});

test("Desktop login opens the pending account destination while workspace loading is stalled", async ({ browserName: _browserName }, testInfo) => {
  test.setTimeout(60_000);
  const profile = mkdtempSync(join(tmpdir(), "multica-collaboration-login-"));
  let app: ElectronApplication | undefined;
  let releaseWorkspaceList = () => {};
  const workspaceList = new Promise<void>((resolve) => { releaseWorkspaceList = resolve; });
  const proxy = await controlledApi({ workspaceWait: workspaceList });
  try {
    const path = accountPath(fixture.issues[0]);
    const native = await openDesktop(profile, { serverUrl: proxy.url, deepLink: `multica://account${path.slice(8)}` });
    app = native.app;
    await native.start();
    await expect(native.page.getByRole("textbox", { name: /email/i })).toBeVisible();
    expect(native.accountHeaders).toHaveLength(0);
    await app.evaluate((_electron, token) => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
      .desktopAccountAcceptance.deliverLogin(token), fixture.token);

    await expect(native.page.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    expect((await snapshot(native.page)).activeScope).toBe("account");
    expect(native.errors).toEqual([]);
  } catch (error) {
    await attachFailure(app, testInfo);
    throw error;
  } finally {
    releaseWorkspaceList();
    await app?.close().catch(() => undefined);
    await proxy.close();
    rmSync(profile, { recursive: true, force: true });
  }
});

for (const [flag, heading] of [
  ["requires_account_setup", "Set up your account"],
  ["requires_password_change", "Set a new password"],
] as const) {
  test(`Desktop account entry obeys the ${flag} guard`, async ({ browserName: _browserName }, testInfo) => {
    test.setTimeout(60_000);
    const profile = mkdtempSync(join(tmpdir(), "multica-collaboration-guard-"));
    const proxy = await controlledApi({ requiredAccountFlag: flag });
    let app: ElectronApplication | undefined;
    try {
      const path = accountPath(fixture.issues[0]);
      const native = await openDesktop(profile, { token: fixture.token, serverUrl: proxy.url, deepLink: `multica://account${path.slice(8)}` });
      app = native.app;
      await native.start();
      await expect(native.page.getByRole("heading", { name: heading, exact: true })).toBeVisible();
      expect(native.accountHeaders).toHaveLength(0);
      await expect(native.page.getByRole("heading", { name: "Collaboration", exact: true })).toHaveCount(0);
      expect(native.errors).toEqual([]);
    } catch (error) {
      await attachFailure(app, testInfo);
      throw error;
    } finally {
      await app?.close().catch(() => undefined);
      await proxy.close();
      rmSync(profile, { recursive: true, force: true });
    }
  });
}

test("Desktop viewer and server changes discard the previous account tab and close popouts", async ({ browserName: _browserName }, testInfo) => {
  test.setTimeout(90_000);
  const profile = mkdtempSync(join(tmpdir(), "multica-collaboration-identity-"));
  let app: ElectronApplication | undefined;
  try {
    const native = await openDesktop(profile, { token: fixture.token });
    app = native.app;
    await native.start();
    await deepLink(app, accountPath(fixture.issues[0]));
    await expect(native.page.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    const previousId = (await snapshot(native.page)).accountGroup!.tabs[0].id;
    const popout = app.waitForEvent("window");
    await native.page.evaluate((path) => {
      const api = (window as unknown as { desktopAPI: { openIssueWindow(input: { path: string; title: string }): Promise<unknown> } }).desktopAPI;
      return api.openIssueWindow({ path, title: "Shared task" });
    }, accountPath(fixture.issues[0]));
    const child = await popout;
    await expect(child.getByRole("heading", { name: fixture.issues[0].title, exact: true })).toBeVisible();
    const childClosed = child.waitForEvent("close");
    await native.page.evaluate((token) => localStorage.setItem("multica_token", token), fixture.emptyToken);
    await native.page.reload();
    await childClosed;
    await expect(native.page.getByText("No issues need your handling.", { exact: true })).toBeVisible();
    const newViewer = await snapshot(native.page);
    expect(newViewer.sessionOwner).toBe(JSON.stringify([apiUrl, fixture.emptyViewerId]));
    expect(newViewer.accountGroup!.tabs[0].id).not.toBe(previousId);
    expect(newViewer.accountGroup!.tabs[0].history.stack).toEqual(["/account?view=current"]);
    expect(newViewer.byWorkspace).toEqual({});
    await deepLink(app, accountPath(fixture.issues[1], "participating"));
    await expect(native.page.getByRole("heading", { name: fixture.issues[1].title, exact: true })).toBeVisible();
    await app.close();
    const otherEndpoint = apiUrl!.replace("127.0.0.1", "localhost");
    const changedServer = await openDesktop(profile, { token: fixture.emptyToken, serverUrl: otherEndpoint });
    app = changedServer.app;
    await changedServer.start();
    await expect(changedServer.page.getByText("No issues need your handling.", { exact: true })).toBeVisible();
    const newServer = await snapshot(changedServer.page);
    expect(newServer.sessionOwner).toBe(JSON.stringify([otherEndpoint, fixture.emptyViewerId]));
    expect(newServer.accountGroup!.tabs[0].history.stack).toEqual(["/account?view=current"]);
    expect(changedServer.errors).toEqual([]);
  } catch (error) {
    await attachFailure(app, testInfo);
    throw error;
  } finally {
    await app?.close().catch(() => undefined);
    rmSync(profile, { recursive: true, force: true });
  }
});
