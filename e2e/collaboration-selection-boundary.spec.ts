import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect } from "@playwright/test";
import { openDesktop, nativeScreenshot, type NativeHarness } from "./fixtures/collaboration-native";
import { inspectOrdinarySelection, inspectSharedSelection, observeSelectionRequests, seedSelectionBoundary, selectionBoundaryEnvironment } from "./fixtures/collaboration-selection-boundary";

// Authentication bodies and session cookies must not enter trace artifacts.
test.use({ actionTimeout: 15_000, screenshot: "off", trace: "off" });

test("AC87 Web and actual Electron hide subtask creation for shared body/comment selections while ordinary children retain it", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires the task-owned password production Web and actual Electron environment");
  const environment = selectionBoundaryEnvironment();
  const webUrl = testInfo.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(webUrl).hostname);
  const renderer = await fetch(environment.rendererUrl);
  expect(renderer.status).toBe(200);
  const fixture = await seedSelectionBoundary();
  const initial = await fixture.client.requestJSON(`/api/collaboration/issues/${fixture.issue.id}`);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const webRequests = observeSelectionRequests(page);
  const profile = mkdtempSync(join(tmpdir(), "multica-ac87-native-"));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  let nativeRequests: ReturnType<typeof observeSelectionRequests> | undefined;
  let ordinaryRequests: ReturnType<typeof observeSelectionRequests> | undefined;
  const evidence: Record<string, unknown> = {
    criterion: "AC87", actor_id: fixture.actorId, issue_id: fixture.issue.id,
    comment_id: fixture.receipt.comment_id, ordinary_issue_id: fixture.ordinary.id, ordinary_comment_id: fixture.ordinaryCommentId,
    environment: { api: environment.apiUrl, web: webUrl, renderer: environment.rendererUrl, electron_os: process.platform, provider_started: false },
    source: ["e2e/collaboration-selection-boundary.spec.ts", "e2e/fixtures/collaboration-selection-boundary.ts"]
      .map((path) => ({ path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") })),
  };
  try {
    await fixture.authenticateWeb(page);
    await page.goto(new URL(fixture.accountPath, webUrl).href);
    evidence.web_shared = await inspectSharedSelection(page, fixture);
    expect(webRequests.requests).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath("web-shared-selection.png"), animations: "disabled" });
    native = await openDesktop(profile, { token: fixture.client.getToken()!, deepLink: `multica://account${fixture.accountPath.slice("/account".length)}` });
    nativeRequests = observeSelectionRequests(native.page);
    await native.start();
    evidence.electron_shared = await inspectSharedSelection(native.page, fixture);
    expect(nativeRequests.requests).toEqual([]);
    await nativeScreenshot(native.app, native.page, testInfo.outputPath("electron-shared-selection.png"));
    expect(await fixture.client.requestJSON(`/api/collaboration/issues/${fixture.issue.id}`)).toEqual(initial);

    await page.goto(new URL(fixture.ordinaryPath, webUrl).href);
    evidence.web_ordinary = await inspectOrdinarySelection(page, fixture);
    expect(webRequests.requests).toEqual([{ method: "GET", path: `/api/comments/${fixture.ordinaryCommentId}/sub-issue-preview` }]);
    const windowPending = native.app.waitForEvent("window");
    await native.page.evaluate((path) => {
      const api = (window as unknown as { desktopAPI: { openIssueWindow(input: { path: string; title: string }): Promise<unknown> } }).desktopAPI;
      return api.openIssueWindow({ path, title: "AC87 ordinary source selection" });
    }, fixture.ordinaryPath);
    const ordinaryWindow = await windowPending;
    ordinaryRequests = observeSelectionRequests(ordinaryWindow);
    evidence.electron_ordinary = await inspectOrdinarySelection(ordinaryWindow, fixture);
    expect(ordinaryRequests.requests).toEqual([{ method: "GET", path: `/api/comments/${fixture.ordinaryCommentId}/sub-issue-preview` }]);
    expect(nativeRequests.requests).toEqual([]);
    expect(await fixture.client.requestJSON(`/api/collaboration/issues/${fixture.issue.id}`)).toEqual(initial);
    expect(native.errors).toEqual([]);
    expect(errors).toEqual([]);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
  } finally {
    evidence.web_source_context_requests = webRequests.requests;
    evidence.electron_shared_source_context_requests = nativeRequests?.requests ?? [];
    evidence.electron_ordinary_source_context_requests = ordinaryRequests?.requests ?? [];
    evidence.web_errors = errors;
    await testInfo.attach("selection-boundary-identities", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
    webRequests.dispose();
    nativeRequests?.dispose();
    ordinaryRequests?.dispose();
    await native?.app.close().catch(() => undefined);
    rmSync(profile, { recursive: true, force: true });
  }
});
