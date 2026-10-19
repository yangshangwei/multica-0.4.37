import { test, expect } from "@playwright/test";
import { authenticateCollaborationEditor } from "./fixtures/collaboration-editing";
import { attachRecordedChild, detachRecordedChild, exerciseRecordedPages, recordedAccountPath,
  removeRecordViewer, seedCollaborationRecordJourney, showRecordedUsage } from "./fixtures/collaboration-records";

test("Web shares ordinary child references and retained run records through account notification links", async ({ page, browser }, testInfo) => {
  test.setTimeout(180_000);
  const apiUrl = process.env.COLLABORATION_E2E_API_URL;
  test.skip(!apiUrl || !process.env.DATABASE_URL || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the task-owned collaboration API/database and production Web");
  for (const value of [apiUrl!, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) {
    expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  }
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);
  const fixture = await seedCollaborationRecordJourney();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await authenticateCollaborationEditor(page, fixture.member.getToken()!);
  await page.goto(`/${fixture.workspace.slug}/issues/${fixture.issue.id}`);
  await expect(page.getByRole("heading", { name: fixture.issue.title, exact: true })).toBeVisible();
  const children = page.getByRole("region", { name: "Sub-issues", exact: true });
  const childLink = children.getByRole("link", { name: `${fixture.child.identifier} ${fixture.child.title}`, exact: true });
  await expect(childLink).toHaveAttribute("href", `/${fixture.workspace.slug}/issues/${fixture.child.id}`);
  await expect(page.getByRole("button", { name: "Edit source fields", exact: true })).toHaveCount(0);
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.bringToFront();
  await page.getByRole("button", { name: "Copy link", exact: true }).click();
  const sharedLink = await page.evaluate(() => navigator.clipboard.readText());
  expect(new URL(sharedLink).pathname + new URL(sharedLink).search).toBe(recordedAccountPath(fixture));
  await page.screenshot({ path: testInfo.outputPath("web-record-source-children.png"), animations: "disabled" });
  await page.goto(`/${fixture.workspace.slug}/issues/${fixture.issue.id}#run-${fixture.runId}`);
  await expect(page).toHaveURL(new RegExp(`/${fixture.workspace.slug}/issues/${fixture.issue.identifier}#run-${fixture.runId}$`));
  const sourceRun = page.locator(`[id="run-${fixture.runId}"]`);
  await expect(sourceRun).toBeFocused();
  await expect(sourceRun.getByText(fixture.input, { exact: true })).toBeVisible();
  await showRecordedUsage(page, fixture.runId);
  await page.screenshot({ path: testInfo.outputPath("web-source-run-anchor.png"), animations: "disabled" });
  await page.getByRole("button", { name: "Copy link", exact: true }).click();
  const runLink = await page.evaluate(() => navigator.clipboard.readText());
  expect(new URL(runLink).pathname + new URL(runLink).search + new URL(runLink).hash).toBe(recordedAccountPath(fixture, true));
  await expect(page.getByRole("link", { name: "Open in collaboration", exact: true })).toHaveAttribute("href", recordedAccountPath(fixture, true));
  await page.getByRole("link", { name: "Open in collaboration", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`#run-${fixture.runId}$`));
  await expect(sourceRun.getByText(fixture.input, { exact: true })).toBeVisible();
  await page.goto(`/${fixture.workspace.slug}/issues/${fixture.issue.id}`);
  await expect(childLink).toBeVisible();

  const outsideContext = await browser.newContext();
  try {
    const outside = await outsideContext.newPage();
    const catalogRequests: string[] = [];
    const frames: { type: string; payload?: Record<string, unknown> }[] = [];
    outside.on("pageerror", (error) => errors.push(error.message));
    outside.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (/^\/api\/(issues|members|projects|properties|labels)(\/|$)/.test(path)) catalogRequests.push(path);
    });
    outside.on("websocket", (socket) => socket.on("framereceived", ({ payload }) => {
      try { frames.push(JSON.parse(payload.toString())); } catch { /* Ignore non-JSON heartbeats. */ }
    }));
    await authenticateCollaborationEditor(outside, fixture.viewer.getToken()!);
    await outside.goto(sharedLink);
    const outsideChildren = outside.getByRole("region", { name: "Sub-issues", exact: true });
    await expect(outsideChildren.getByText(fixture.child.title, { exact: true })).toBeVisible();
    await expect(outsideChildren.getByRole("link")).toHaveCount(0);
    await expect(outside.getByText("PRIVATE_ORDINARY_CHILD_BODY_DO_NOT_PROJECT", { exact: true })).toHaveCount(0);
    await expect.poll(() => frames.some((frame) => frame.type === "subscribe_ack" && frame.payload?.scope === "issue" && frame.payload.id === fixture.issue.id)).toBe(true);
    const before = await fixture.source.requestJSON<{ collaboration_revision: string }>(`/api/collaboration/issues/${fixture.issue.id}`);

    await childLink.click();
    await expect(page).toHaveURL(new RegExp(`/${fixture.workspace.slug}/issues/${fixture.child.identifier}$`));
    await detachRecordedChild(page);
    await expect(outsideChildren.getByText(fixture.child.title, { exact: true })).toHaveCount(0, { timeout: 5000 });
    await attachRecordedChild(page, fixture);
    await expect(outsideChildren.getByText(fixture.child.title, { exact: true })).toBeVisible({ timeout: 5000 });
    const hints = frames.filter((frame) => frame.type === "collaboration:children_changed");
    expect(hints.length).toBeGreaterThanOrEqual(2);
    for (const hint of hints) expect(hint.payload).toEqual({ issue_id: fixture.issue.id });
    expect(await fixture.source.requestJSON(`/api/collaboration/issues/${fixture.issue.id}`)).toMatchObject({ collaboration_revision: before.collaboration_revision });

    await outside.reload();
    await expect(outsideChildren.getByText(fixture.child.title, { exact: true })).toBeVisible();
    await outside.getByRole("button", { name: /Notifications/ }).click();
    const notification = outside.getByRole("listitem").filter({ has: outside.getByText("AI execution failed", { exact: true }) });
    await notification.getByRole("link", { name: fixture.issue.title, exact: true }).click();
    await expect(outside).toHaveURL(new RegExp(`#run-${fixture.runId}$`));
    const run = outside.locator(`[id="run-${fixture.runId}"]`);
    await expect(run).toBeFocused();
    await exerciseRecordedPages(outside, fixture);
    await run.getByRole("button", { name: `Hide records for ${fixture.runId}`, exact: true }).click();
    await expect(outside).toHaveURL(new URL(recordedAccountPath(fixture), outside.url()).href);
    await outside.getByRole("button", { name: /Notifications/ }).click();
    await outside.getByRole("listitem").filter({ has: outside.getByText("AI execution failed", { exact: true }) }).getByRole("link", { name: fixture.issue.title, exact: true }).click();
    await expect(run.getByText(fixture.input, { exact: true })).toBeVisible();
    await expect(run).toBeFocused();
    await run.getByRole("button", { name: `Hide records for ${fixture.runId}`, exact: true }).evaluate((element) => element.scrollIntoView({ block: "start" }));
    await outside.screenshot({ path: testInfo.outputPath("web-records-wide.png"), animations: "disabled" });
    await outside.setViewportSize({ width: 390, height: 844 });
    await showRecordedUsage(outside, fixture.runId);
    expect(await outside.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await outside.screenshot({ path: testInfo.outputPath("web-records-narrow.png"), animations: "disabled", fullPage: false });
    await run.getByText(/^Recorded step 2\./).evaluate((element) => element.scrollIntoView({ block: "start" }));
    expect(await outside.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await outside.screenshot({ path: testInfo.outputPath("web-records-body-narrow.png"), animations: "disabled", fullPage: false });
    // Selecting shared body/process text must not create an ordinary source-copy action.
    await run.getByText(fixture.input, { exact: true }).evaluate((element) => {
      const range = document.createRange(); range.selectNodeContents(element);
      const selection = window.getSelection(); selection?.removeAllRanges(); selection?.addRange(range);
      document.dispatchEvent(new Event("selectionchange"));
    });
    await expect(outside.getByText("Create sub-issue from here", { exact: true })).toHaveCount(0);
    expect(catalogRequests).toEqual([]);
    expect(await outside.evaluate((input) => Object.values(localStorage).some((value) => value.includes(input)), fixture.input)).toBe(false);

    await removeRecordViewer(fixture);
    await expect(outside.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
    await expect(outside.getByText(fixture.input, { exact: true })).toHaveCount(0);
    await expect(outside.getByRole("region", { name: "Run records", exact: true })).toHaveCount(0);
    await outside.reload();
    await expect(outside.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
    const denied = await fetch(`${apiUrl}/api/collaboration/issues/${fixture.issue.id}/runs/${fixture.runId}/records`, { headers: { Authorization: `Bearer ${fixture.viewer.getToken()!}` } });
    expect(denied.status).toBe(404);
    const notifications = await fixture.viewer.requestJSON<{ items: { kind: string; can_open: boolean; run_id?: string }[] }>("/api/me/collaboration-notifications");
    const oldResult = notifications.items.find((item) => item.kind === "ai_failed");
    expect(oldResult).toMatchObject({ can_open: false });
    expect(oldResult?.run_id).toBeUndefined();
    await testInfo.attach("record-journey-evidence", { contentType: "application/json", body: JSON.stringify({ issueId: fixture.issue.id,
      childId: fixture.child.id, groupId: fixture.groupId, representativeRunId: fixture.runId, sharedLink, childrenHints: hints,
      catalogRequests, recordSource: "seeded retained source history; runtime ingestion verified separately", pageErrors: errors }, null, 2) });
  } finally { await outsideContext.close(); }
  expect(errors).toEqual([]);
});
