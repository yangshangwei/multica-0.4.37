import { randomUUID } from "node:crypto";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { authenticateCollaborationEditor, createCollaborationWithInitialFields, exerciseCollaborationDiscussion,
  exerciseCollaborationFields, seedCollaborationEditing } from "./fixtures/collaboration-editing";

const apiUrl = process.env.COLLABORATION_E2E_API_URL;

test("Web source creation, field edits and discussion share one authorized issue", async ({ page, browser }, testInfo) => {
  test.setTimeout(240_000);
  test.skip(!apiUrl || !process.env.DATABASE_URL || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the isolated collaboration API/database and production Web with creation enabled");
  for (const value of [apiUrl!, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) {
    expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  }
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);
  const fixture = await seedCollaborationEditing();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await authenticateCollaborationEditor(page, fixture.source.getToken()!);
  await page.goto(`/${fixture.workspace.slug}/issues`);
  const issue = await createCollaborationWithInitialFields(page, fixture);
  await exerciseCollaborationFields(page, fixture, issue.id);
  const discussion = await exerciseCollaborationDiscussion(page, fixture.source, issue.id, true);

  // A normal source detail URL must resolve the same object using collaboration
  // authority and controls, including a linked child in the source workspace.
  await page.goto(`/${fixture.workspace.slug}/issues/${issue.id}`);
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Edit source fields", exact: true })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("web-source-collaboration.png"), animations: "disabled" });
  await page.goto(`/${fixture.workspace.slug}/issues`);
  await expect(page.getByText(fixture.ordinary.title, { exact: true }).first()).toBeVisible();
  const externalFilter = page.getByRole("button", { name: "External collaborators", exact: true });
  await externalFilter.click();
  await expect(externalFilter).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText(fixture.title, { exact: true }).first()).toBeVisible();
  await expect(page.getByText(fixture.ordinary.title, { exact: true })).toHaveCount(0);
  await expect(page.getByText(fixture.recipientEmail, { exact: true })).toHaveCount(0);
  await page.screenshot({ path: testInfo.outputPath("web-source-external-filter.png"), animations: "disabled" });

  const recipientContext = await browser.newContext();
  try {
    const recipientPage = await recipientContext.newPage();
    const catalogRequests: string[] = [];
    recipientPage.on("pageerror", (error) => errors.push(error.message));
    recipientPage.on("request", (request) => {
      const pathname = new URL(request.url()).pathname;
      if (/^\/api\/(projects|members|properties|labels|issues)(\/|$)/.test(pathname)
        || pathname.includes(`/workspaces/${fixture.workspace.id}/iterations`)) catalogRequests.push(pathname);
    });
    await authenticateCollaborationEditor(recipientPage, fixture.recipient.getToken()!);
    await recipientPage.goto(`/account?view=current&issue=${issue.id}`);
    await expect(recipientPage.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    await expect(recipientPage.getByRole("button", { name: "Edit source fields", exact: true })).toHaveCount(0);
    await expect(recipientPage.getByRole("button", { name: "Change iteration", exact: true })).toHaveCount(0);
    await expect(recipientPage.locator(`[id="comment-${discussion.replyId}"]`).getByRole("button", { name: "Edit comment", exact: true })).toHaveCount(0);
    const options = await fixture.recipient.requestJSON<{ source_options?: unknown }>(`/api/collaboration/issues/${issue.id}/field-options`);
    expect(options.source_options).toBeUndefined();

    await recipientPage.getByRole("button", { name: "Edit issue", exact: true }).click();
    const form = recipientPage.locator("form").filter({ has: recipientPage.getByRole("heading", { name: "Edit issue", exact: true }) });
    await form.getByLabel("Priority", { exact: true }).selectOption("medium");
    await form.getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(form).toHaveCount(0);
    expect(await fixture.source.requestJSON(`/api/collaboration/issues/${issue.id}`)).toMatchObject({ priority: "medium" });
    expect(catalogRequests).toEqual([]);

    await recipientPage.setViewportSize({ width: 640, height: 900 });
    await recipientPage.getByRole("button", { name: "Edit issue", exact: true }).click();
    await expect(recipientPage.getByLabel("Description", { exact: true })).toBeVisible();
    expect(await recipientPage.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await recipientPage.screenshot({ path: testInfo.outputPath("web-external-editing-narrow.png"), animations: "disabled", fullPage: false });
    await recipientPage.getByLabel("Description", { exact: true }).fill("Unsent external field draft removed on access loss.");

    const current = await fixture.source.requestJSON<{ collaboration_revision: string; lifecycle_epoch: string;
      write_context: { participant_epoch: string; account_epoch: string } }>(`/api/collaboration/issues/${issue.id}`);
    await fixture.source.requestJSON(`/api/collaboration/issues/${issue.id}/participants/${fixture.recipientId}/remove`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
        expected_collaboration_revision: current.collaboration_revision,
        expected_lifecycle_epoch: current.lifecycle_epoch,
        expected_participant_epoch: current.write_context.participant_epoch,
        expected_account_epoch: current.write_context.account_epoch,
      },
    });
    await expect(recipientPage.getByRole("heading", { name: fixture.title, exact: true })).toHaveCount(0);
    await expect(recipientPage.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
    await expect(recipientPage.getByLabel("Description", { exact: true })).toHaveCount(0);
  } finally {
    await recipientContext.close();
  }
  expect(errors).toEqual([]);
  await testInfo.attach("source-editing-evidence", { body: JSON.stringify({ issueId: issue.id, sourceWorkspaceId: fixture.workspace.id,
    externalHandlerId: fixture.recipientId, discussion, privateCatalogRequests: 0 }, null, 2), contentType: "application/json" });
});

test("A source member outside the collaboration receives only an empty workspace refresh", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  test.skip(!apiUrl || !process.env.DATABASE_URL || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the isolated collaboration API/database and production Web");
  for (const value of [apiUrl!, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) {
    expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  }
  const fixture = await seedCollaborationEditing();
  const viewer = new TestApiClient();
  const email = `source-observer-${randomUUID()}@example.invalid`;
  await viewer.login(email, "Source member outside the collaboration");
  await viewer.markUserOnboarded();
  const invitation = await fixture.source.requestJSON<{ id: string }>(`/api/workspaces/${fixture.workspace.id}/members`, {
    method: "POST", body: { email, role: "member" },
  });
  await viewer.requestJSON(`/api/invitations/${invitation.id}/accept`, { method: "POST" });
  const issue = await fixture.source.requestJSON<{ id: string }>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: fixture.workspace.id, handler_user_id: fixture.recipientId, title: fixture.title,
    },
  });
  expect(await viewer.requestJSON("/api/me/workbench?view=participating")).toMatchObject({ items: [] });
  const payloads: unknown[] = [];
  const subscriptions: unknown[] = [];
  page.on("websocket", (socket) => {
    socket.on("framereceived", ({ payload }) => {
      try {
        const frame: { type?: string; payload?: unknown } = JSON.parse(payload.toString());
        if (frame.type === "collaboration:workbench_changed") payloads.push(frame.payload);
        if (frame.type === "subscribe_ack" || frame.type === "auth_ack") subscriptions.push({ type: frame.type, payload: frame.payload });
      } catch { /* Heartbeats are not collaboration evidence. */ }
    });
  });
  await authenticateCollaborationEditor(page, viewer.getToken()!);
  await page.goto(`/${fixture.workspace.slug}/issues`);
  await expect(page.getByText(fixture.title, { exact: true }).first()).toBeVisible();
  await expect.poll(() => subscriptions.length).toBeGreaterThan(0);
  const baseline = payloads.length;
  const current = await fixture.source.requestJSON<{ collaboration_revision: string; lifecycle_epoch: string;
    write_context: { participant_epoch: string; account_epoch: string } }>(`/api/collaboration/issues/${issue.id}`);
  const title = `Updated ${fixture.title}`;
  await fixture.source.requestJSON(`/api/collaboration/issues/${issue.id}`, {
    method: "PATCH", headers: { "Idempotency-Key": randomUUID() }, body: {
      title, expected_collaboration_revision: current.collaboration_revision,
      expected_lifecycle_epoch: current.lifecycle_epoch,
      expected_participant_epoch: current.write_context.participant_epoch,
      expected_account_epoch: current.write_context.account_epoch,
    },
  });
  await expect.poll(() => payloads.length, { timeout: 5000 }).toBeGreaterThan(baseline);
  expect(payloads.slice(baseline).every((payload) => JSON.stringify(payload) === "{}")).toBe(true);
  await expect(page.getByText(title, { exact: true }).first()).toBeVisible();
  await expect(page.getByText(fixture.title, { exact: true })).toHaveCount(0);
  expect(await viewer.requestJSON("/api/me/workbench?view=participating")).toMatchObject({ items: [] });
  await testInfo.attach("source-workspace-refresh", { body: JSON.stringify({ issueId: issue.id, payloads: payloads.slice(baseline),
    subscriptions, participating: false }, null, 2), contentType: "application/json" });
});
