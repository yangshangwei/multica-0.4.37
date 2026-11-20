import { randomUUID } from "node:crypto";
import { createServer, request as httpRequest } from "node:http";
import type { Duplex } from "node:stream";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import { _electron as electron, expect, test, type Page, type TestInfo } from "@playwright/test";

interface FixtureIssue {
  surface: "web" | "desktop";
  issueID: string;
  issueIdentifier: string;
  issueRevision: number;
  approvalID: string;
  executionID: string;
  sourceRevision: string;
  specDigest: string;
  artifactDigest: string;
  receiptDigest: string;
}

interface AcceptanceFixture {
  apiBase: string;
  email: string;
  token: string;
  workspaceID: string;
  workspaceSlug: string;
  projectID: string;
  policyRevision: number;
  fixtures: FixtureIssue[];
  artifactPackaging: "local_synthetic_success";
  providerCalls: 0;
}

interface Authorization {
  authorization_id: string;
  operation_id: string;
  state: string;
  completion_receipt?: {
    authorization_id: string;
    operation_id: string;
    issue_id: string;
    completed_revision: number;
    target_status: string;
  } | null;
}

const fixtureFile = process.env.M1_FIXTURE_FILE;
const desktopRendererOrigin = process.env.M1_DESKTOP_RENDERER_ORIGIN ?? "http://127.0.0.1:5668";
const root = resolve(import.meta.dirname, "..");
const desktopOutput = process.env.MULTICA_E2E_DESKTOP_OUTPUT_DIR ?? join(root, "apps/desktop/out");

async function loadFixture(): Promise<AcceptanceFixture> {
  if (!fixtureFile) throw new Error("M1_FIXTURE_FILE is required for this task-scoped acceptance run");
  const fixture = JSON.parse(await readFile(fixtureFile, "utf8")) as AcceptanceFixture;
  expect(fixture.artifactPackaging).toBe("local_synthetic_success");
  expect(fixture.providerCalls).toBe(0);
  expect(fixture.fixtures.map((item) => item.surface)).toEqual(["web", "desktop"]);
  return fixture;
}

async function apiJSON<T>(fixture: AcceptanceFixture, path: string): Promise<T> {
  const response = await fetch(`${fixture.apiBase}${path}`, {
    headers: { Authorization: `Bearer ${fixture.token}`, "X-Workspace-ID": fixture.workspaceID },
  });
  const text = await response.text();
  if (!response.ok) throw new Error(`GET ${path} -> ${response.status}: ${text}`);
  return JSON.parse(text) as T;
}

async function authenticate(page: Page, fixture: AcceptanceFixture) {
  await page.addInitScript(({ token }) => {
    localStorage.setItem("multica_token", token);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, { token: fixture.token });
  if (page.url() !== "about:blank") await page.reload();
}

async function completeFromKeyboard(
  page: Page,
  fixture: AcceptanceFixture,
  item: FixtureIssue,
  info: TestInfo,
  navigate: (() => Promise<void>) | undefined,
  resize: (size: { width: number; height: number }) => Promise<void>,
  wide: { width: number; height: number },
  narrow: { width: number; height: number },
) {
  const errors: string[] = [];
  const writes: Array<{ method: string; path: string; body: unknown }> = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (path === "/api/delivery-completion-authorizations" || path === `/api/issues/${item.issueID}`) {
      writes.push({ method: request.method(), path, body: request.postDataJSON() });
    }
  });
  await authenticate(page, fixture);
  if (navigate) await navigate();
  else {
    const issuePath = `/${fixture.workspaceSlug}/issues/${item.issueIdentifier}`;
    const base = page.url().startsWith("http") ? page.url() : (process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:3000");
    await page.goto(new URL(issuePath, base).href);
  }
  const issue = await apiJSON<{ title: string; status: string }>(fixture, `/api/issues/${item.issueID}`);
  expect(issue.status, "Fixture issues must be fresh and not previously consumed").toBe("in_review");
  const issueTitle = issue.title;
  await expect(page.getByRole("button", { name: issueTitle, exact: true })).toBeVisible();

  const statusTrigger = page.getByRole("button", { name: "In Review", exact: true }).last();
  await statusTrigger.focus();
  await page.keyboard.press("Enter");
  const done = page.locator("button[data-picker-item]").filter({ hasText: /^Done$/ });
  await expect(done).toBeVisible();
  await done.focus();
  await page.keyboard.press("Enter");

  const dialog = page.getByRole("dialog", { name: "Authorize issue completion", exact: true });
  await expect(dialog).toBeVisible();
  await expect.poll(() => writes.filter((write) => write.method === "POST" && write.path === "/api/delivery-completion-authorizations").length).toBe(0);
  const evidence = dialog.locator(`input[name="completion-evidence"][value="${item.approvalID}"]`);
  await expect(evidence).toBeVisible();
  await expect(evidence, "The task-owned approval must still be current and selectable").toBeEnabled();
  const previewResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === `/api/issues/${item.issueID}/delivery-evidence/completion-preview`,
  );
  await evidence.focus();
  await expect(evidence).toBeFocused();
  await page.keyboard.press("Space");
  const preview = await previewResponse;
  expect(preview.status()).toBe(200);
  expect(await preview.json()).toMatchObject({ enforced: false, usable_for_completion: false, skip_supported: false,
    observation: { verdict: "pass" } });
  await expect(dialog.getByText(/Current observation:\s*pass/)).toBeVisible();
  await expect(dialog).not.toContainText("evaluation_key");

  await resize(wide);
  await page.getByLabel("Acceptance reason", { exact: true }).fill("Human reviewed this exact local synthetic artifact.");
  await dialog.locator("details").first().locator("summary").click();
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: info.outputPath(`${item.surface}-review-wide.png`), animations: "disabled" });

  await resize(narrow);
  const narrowBounds = await dialog.boundingBox();
  expect(narrowBounds).not.toBeNull();
  expect(narrowBounds!.x).toBeGreaterThanOrEqual(0);
  expect(narrowBounds!.x + narrowBounds!.width).toBeLessThanOrEqual(narrow.width + 1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: info.outputPath(`${item.surface}-review-narrow.png`), animations: "disabled" });

  const authorize = dialog.getByRole("button", { name: "Authorize this version", exact: true });
  await expect(authorize).toBeEnabled();
  const authorizationResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === "/api/delivery-completion-authorizations" && response.request().method() === "POST",
  );
  await authorize.focus();
  await page.keyboard.press("Enter");
  const authorizationWrite = await authorizationResponse;
  expect(authorizationWrite.status()).toBe(201);
  const authorizationRequest = authorizationWrite.request().postDataJSON() as { operation_id: string };
  const grant = await authorizationWrite.json() as Authorization;
  expect(grant.state).toBe("issued");
  await expect(dialog.getByRole("status")).toContainText("This version is authorized.");
  await page.screenshot({ path: info.outputPath(`${item.surface}-authorized.png`), animations: "disabled" });

  const submit = dialog.getByRole("button", { name: "Submit completion", exact: true });
  await expect(submit).toBeEnabled();
  const completionResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === `/api/issues/${item.issueID}` && response.request().method() === "PUT",
  );
  await submit.focus();
  await page.keyboard.press("Enter");
  const submitted = await completionResponse;
  expect(submitted.status()).toBe(200);
  const completionRequest = submitted.request().postDataJSON() as {
    status: string;
    completion: { authorization_id: string; operation_id: string; expected_issue_revision: number };
  };
  expect(completionRequest).toEqual({
    status: "done",
    completion: {
      authorization_id: grant.authorization_id,
      operation_id: authorizationRequest.operation_id,
      expected_issue_revision: item.issueRevision,
    },
  });
  await expect(dialog.getByRole("status")).toContainText("Completion was recorded for this operation.");
  await expect(dialog.getByText(/Current issue status:\s*Done/)).toBeVisible();

  const consumed = await apiJSON<Authorization>(fixture, `/api/delivery-completion-authorizations/${grant.authorization_id}`);
  const current = await apiJSON<{ id: string; status: string; revision: number }>(fixture, `/api/issues/${item.issueID}`);
  expect(consumed).toMatchObject({ state: "consumed", completion_receipt: {
    authorization_id: grant.authorization_id,
    operation_id: authorizationRequest.operation_id,
    issue_id: item.issueID,
    completed_revision: item.issueRevision + 1,
    target_status: "done",
  } });
  expect(current).toMatchObject({ id: item.issueID, status: "done", revision: consumed.completion_receipt!.completed_revision });
  expect(writes.filter((write) => write.path === "/api/delivery-completion-authorizations" && write.method === "POST")).toHaveLength(1);
  expect(writes.filter((write) => write.path === `/api/issues/${item.issueID}` && write.method === "PUT")).toHaveLength(1);
  await page.screenshot({ path: info.outputPath(`${item.surface}-current-done.png`), animations: "disabled" });
  expect(errors).toEqual([]);
  return { authorizationID: grant.authorization_id, operationID: authorizationRequest.operation_id, receipt: consumed.completion_receipt, current };
}

test("Web enforces the explicit evidence authorization and shows a narrow keyboard flow", async ({ page }, info) => {
  test.setTimeout(120_000);
  test.skip(!fixtureFile, "Requires an explicit task-owned M1 fixture from m1-renderer-fixture-20261113.mjs");
  const fixture = await loadFixture();
  const item = fixture.fixtures.find((candidate) => candidate.surface === "web");
  if (!item) throw new Error("Web fixture issue is missing");
  const result = await completeFromKeyboard(page, fixture, item, info, undefined,
    (size) => page.setViewportSize(size), { width: 1440, height: 1000 }, { width: 390, height: 844 });
  await info.attach("web-completion-result", { body: JSON.stringify(result, null, 2), contentType: "application/json" });
});

test("Electron renderer enforces the explicit evidence authorization in a narrow window", async ({ request }, info) => {
  test.setTimeout(120_000);
  test.skip(!fixtureFile, "Requires an explicit task-owned M1 fixture from m1-renderer-fixture-20261113.mjs");
  const fixture = await loadFixture();
  const item = fixture.fixtures.find((candidate) => candidate.surface === "desktop");
  if (!item) throw new Error("Desktop fixture issue is missing");
  const renderer = join(desktopOutput, "renderer");
  const profile = await mkdtemp(join(tmpdir(), "multica-m1-s1-renderer-"));
  const sockets = new Set<Duplex>();
  const server = createServer(async (req, res) => {
    try {
      const pathname = new URL(req.url ?? "/", "http://localhost").pathname;
      if (pathname.startsWith("/api/") || pathname.startsWith("/auth/") || pathname === "/health") {
        const chunks: Buffer[] = [];
        for await (const chunk of req) chunks.push(Buffer.from(chunk));
        const headers = new Headers();
        for (const key of ["content-type", "authorization", "x-workspace-id", "x-workspace-slug", "accept-language", "idempotency-key"]) {
          const value = req.headers[key];
          if (typeof value === "string") headers.set(key, value);
        }
        const response = await fetch(`${fixture.apiBase}${req.url}`, { method: req.method, headers, body: chunks.length ? Buffer.concat(chunks) : undefined });
        res.writeHead(response.status, { "Content-Type": response.headers.get("content-type") ?? "application/json", "Cache-Control": response.headers.get("cache-control") ?? "no-store" });
        res.end(Buffer.from(await response.arrayBuffer()));
        return;
      }
      const file = resolve(renderer, `.${pathname === "/" ? "/index.html" : pathname}`);
      if (!file.startsWith(`${renderer}/`)) { res.writeHead(404); res.end(); return; }
      const data = await readFile(file);
      const types: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".woff2": "font/woff2", ".svg": "image/svg+xml", ".png": "image/png", ".webp": "image/webp" };
      res.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream" });
      res.end(data);
    } catch {
      res.writeHead(502); res.end();
    }
  });
  server.on("upgrade", (req, socket, head) => {
    const incoming = new URL(req.url ?? "/", desktopRendererOrigin);
    if (incoming.pathname !== "/ws") { socket.destroy(); return; }
    const target = new URL(fixture.apiBase);
    target.pathname = "/ws";
    target.search = incoming.search;
    sockets.add(socket);
    const upstream = httpRequest(target, { headers: req.headers });
    socket.on("error", () => upstream.destroy());
    socket.on("close", () => { sockets.delete(socket); upstream.destroy(); });
    upstream.on("error", () => socket.destroy());
    upstream.on("response", (response) => { response.resume(); socket.destroy(); });
    upstream.on("upgrade", (response, peer, peerHead) => {
      sockets.add(peer);
      peer.on("error", () => socket.destroy());
      peer.on("close", () => { sockets.delete(peer); socket.destroy(); });
      socket.on("close", () => peer.destroy());
      const headers: string[] = [];
      for (let index = 0; index < response.rawHeaders.length; index += 2) headers.push(`${response.rawHeaders[index]}: ${response.rawHeaders[index + 1]}`);
      socket.write(`HTTP/1.1 ${response.statusCode} ${response.statusMessage}\r\n${headers.join("\r\n")}\r\n\r\n`);
      if (peerHead.length) socket.write(peerHead);
      if (head.length) peer.write(head);
      socket.pipe(peer); peer.pipe(socket);
    });
    upstream.end();
  });
  let desktop: Awaited<ReturnType<typeof electron.launch>> | undefined;
  try {
    await new Promise<void>((done) => server.listen(Number(new URL(desktopRendererOrigin).port), "127.0.0.1", done));
    const origin = desktopRendererOrigin;
    const preflight = await request.fetch(`${fixture.apiBase}/api/config`, { method: "OPTIONS", headers: {
      Origin: origin, "Access-Control-Request-Method": "GET", "Access-Control-Request-Headers": "authorization,content-type",
    } });
    expect(preflight.headers()["access-control-allow-origin"]).toBe(origin);
    desktop = await electron.launch({
      executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin"
        ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"),
      args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
      env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_E2E_API_URL: origin,
        CHANGELOG_ELECTRON_RENDERER_URL: origin, CHANGELOG_ELECTRON_PRELOAD: join(desktopOutput, "preload/index.js"),
        CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en-US" },
    });
    const page = await desktop.firstWindow();
    await page.locator("#root > *").first().waitFor({ state: "attached" });
    const result = await completeFromKeyboard(page, fixture, item, info, async () => {
      await page.getByRole("link", { name: "Workspace issues", exact: true }).click();
      await page.getByRole("link", { name: new RegExp(item.issueIdentifier) }).click();
    },
      ({ width, height }) => desktop!.evaluate(({ BrowserWindow }, size) => BrowserWindow.getAllWindows()[0]!.setContentSize(size.width, size.height), { width, height }),
      { width: 1380, height: 1000 }, { width: 680, height: 900 });
    expect(await desktop.evaluate(() => (globalThis as unknown as { changelogAcceptance: { daemonStarts: number } }).changelogAcceptance.daemonStarts)).toBe(0);
    await info.attach("desktop-completion-result", { body: JSON.stringify(result, null, 2), contentType: "application/json" });
  } finally {
    await desktop?.close();
    for (const socket of sockets) socket.destroy();
    await new Promise<void>((done) => server.close(() => done()));
    await rm(profile, { recursive: true, force: true });
  }
});
