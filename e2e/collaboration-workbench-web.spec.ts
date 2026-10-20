import { randomUUID } from "node:crypto";
import pg from "pg";
import { test, expect, type Page, type Request } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Public creation remains gated. These disposable database records prove the
// account journey and real API writes, not the new-collaboration creation AC.
const apiUrl = process.env.COLLABORATION_E2E_API_URL;
const databaseUrl = process.env.DATABASE_URL;
test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });
type Snapshot = { id: string; collaboration_revision: string; lifecycle_epoch: string; write_context: { participant_epoch: string; account_epoch: string } };
const writeContext = (issue: Snapshot) => ({ expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
  expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch });

async function authenticatePage(page: Page, token: string) {
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
}

test("Web workbench keeps account scope through comments, uncertain cancellation, handoff and revocation", async ({ page, browser }, testInfo) => {
  test.setTimeout(120_000);
  test.skip(!apiUrl || !databaseUrl || !process.env.NEXT_PUBLIC_API_URL,
    "Requires an explicitly isolated collaboration API/database and production Web build");
  for (const value of [apiUrl!, databaseUrl!, process.env.NEXT_PUBLIC_API_URL!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);

  const key = randomUUID().slice(0, 10);
  const source = new TestApiClient();
  await source.login(`web-source-${key}@example.invalid`, "Browser source owner");
  const workspace = await source.ensureWorkspace("Browser external source", `web-source-${key}`);
  await source.markUserOnboarded();
  const owner = await source.requestJSON<{ id: string }>("/api/me");
  const viewer = new TestApiClient();
  await viewer.login(`web-viewer-${key}@example.invalid`, "Browser account viewer");
  await viewer.markUserOnboarded();
  const actor = await viewer.requestJSON<{ id: string }>("/api/me");
  expect(await viewer.getWorkspaces()).toEqual([]);
  const recipient = new TestApiClient();
  const recipientEmail = `web-recipient-${key}@example.invalid`;
  await recipient.login(recipientEmail, "Browser recipient");
  await recipient.markUserOnboarded();
  expect(await recipient.getWorkspaces()).toEqual([]);
  const created: { id: string; title: string } = await source.createIssue(`Browser shared issue ${key}`, { description: "Shared description for the account-only workbench." });
  const commandId = randomUUID(), targetId = randomUUID(), agentId = randomUUID(), channelId = randomUUID(), batchId = randomUUID(), eventId = randomUUID();
  const db = new pg.Client(databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    await db.query("INSERT INTO collaboration_account_fence (user_id) SELECT unnest($1::uuid[]) ON CONFLICT DO NOTHING", [[owner.id, actor.id]]);
    await db.query("UPDATE issue SET assignee_type='member', assignee_id=$2 WHERE id=$1", [created.id, actor.id]);
    await db.query("INSERT INTO issue_collaboration (issue_id) VALUES ($1)", [created.id]);
    const session = await db.query<{ id: string }>("INSERT INTO collaboration_session (issue_id,event_seq) VALUES ($1,1) RETURNING id", [created.id]);
    const sessionId = session.rows[0]!.id;
    await db.query("INSERT INTO issue_participant (issue_id,user_id) SELECT $1,unnest($2::uuid[])", [created.id, [owner.id, actor.id]]);
    await db.query("INSERT INTO collaboration_event (id,issue_id,event_seq,actor_user_id,kind,payload) VALUES ($1,$2,1,$3,'created','{}')", [eventId, created.id, owner.id]);
    await db.query("INSERT INTO collaboration_channel (id,session_id,root_agent_id,next_batch_seq,next_command_seq,pause_reason) VALUES ($1,$2,$3,2,2,'fixture_paused')", [channelId, sessionId, agentId]);
    // No agent, runtime, grant or executable exists. R45 cancellation must work
    // even for an unavailable target. Preserve a valid historical roster so
    // missing resources are unavailable, rather than corrupting the catalogue.
    const roster = [{ role: "agent", agent_id: agentId, runtime_id: randomUUID(), execution_workspace_id: workspace.id, owner_user_id: owner.id }];
    await db.query("INSERT INTO collaboration_target (id,session_id,kind,resource_id,roster,channel_id,pause_reason) VALUES ($1,$2,'agent',$3,$5::jsonb,$4,'fixture_paused')", [targetId, sessionId, agentId, channelId, JSON.stringify(roster)]);
    await db.query("INSERT INTO collaboration_batch (id,session_id,channel_id,effective_target_id,seq) VALUES ($1,$2,$3,$4,1)", [batchId, sessionId, channelId, targetId]);
    await db.query("INSERT INTO collaboration_command (id,session_id,target_id,input_event_id,channel_id,batch_id,seq,author_user_id,participant_epoch,author_auth_epoch,lifecycle_epoch,content) VALUES ($1,$2,$3,$4,$5,$6,1,$7,1,1,1,'Test-owned waiting input')", [commandId, sessionId, targetId, eventId, channelId, batchId, actor.id]);
    await db.query("COMMIT");
  } catch (error) { await db.query("ROLLBACK"); throw error; }
  finally { await db.end(); }

  const initial = await source.requestJSON<Snapshot>(`/api/collaboration/issues/${created.id}`);
  const old = await source.requestJSON<{ comment_id: string }>(`/api/collaboration/issues/${created.id}/comments`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    content: "Older linked comment", parent_id: null, suppressed_target_ids: [], target_versions: [], expected_lifecycle_epoch: initial.lifecycle_epoch,
    expected_participant_epoch: initial.write_context.participant_epoch, expected_account_epoch: initial.write_context.account_epoch,
  } });
  const seed = new pg.Client(databaseUrl);
  await seed.connect();
  try {
    await seed.query("UPDATE comment SET created_at='2000-01-01T00:00:00Z' WHERE id=$1 AND issue_id=$2", [old.comment_id, created.id]);
    await seed.query("INSERT INTO comment (issue_id,workspace_id,author_type,author_id,content,type,dispatch_eligible) SELECT $1,$2,'member',$3,'History fixture ' || value,'comment',false FROM generate_series(1,101) value", [created.id, workspace.id, owner.id]);
  } finally { await seed.end(); }
  expect(await viewer.requestJSON(`/api/collaboration/issues/${created.id}/activity`)).toMatchObject({ issue_id: created.id });
  expect(await viewer.requestJSON(`/api/collaboration/issues/${created.id}`)).toMatchObject({ capabilities: { control_ai: true, submit_command: false } });
  expect(await viewer.requestJSON(`/api/collaboration/issues/${created.id}/mention-targets`)).toMatchObject({
    participants: expect.arrayContaining([expect.objectContaining({ user_id: owner.id, name: "Browser source owner" })]), targets: [],
  });

  const errors: string[] = [], accountHeaders: Record<string, string>[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => { if (/\/api\/(collaboration\/|me\/workbench)/.test(request.url())) accountHeaders.push(request.headers()); });
  await authenticatePage(page, viewer.getToken()!);
  const account = `/account?view=current&issue=${created.id}`;
  await page.goto(`${account}#comment-${old.comment_id}`);
  await expect(page.locator(`[id="comment-${old.comment_id}"]`)).toContainText("Older linked comment");
  await expect(page.locator(`[id="comment-${old.comment_id}"]`)).toBeFocused();
  await page.goto(account);
  await expect(page.getByRole("heading", { name: created.title, exact: true })).toBeVisible();
  await expect(page).toHaveTitle("Collaboration | Multica");

  let cancelKey = "", cancelPosts = 0;
  await page.route(`**/api/collaboration/issues/${created.id}/commands/${commandId}/cancel`, async (route) => {
    cancelPosts += 1;
    cancelKey = route.request().headers()["idempotency-key"] ?? "";
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    await route.abort("failed");
  });
  await page.getByRole("button", { name: "Cancel waiting input 1" }).click();
  await expect(page.getByText(/The result is not confirmed/)).toBeVisible();
  expect(cancelKey).not.toBe("");
  await page.reload();
  const recovered = page.waitForResponse((response) => response.url().includes(`/commands/operations/${cancelKey}`) && response.request().method() === "GET");
  await page.getByRole("button", { name: "Check original result" }).click();
  expect((await recovered).status()).toBe(200);
  await expect(page.getByText("Waiting input cancelled.")).toBeVisible();
  expect(cancelPosts).toBe(1);

  await page.getByLabel("Add comment", { exact: true }).fill("Browser comment ");
  await page.getByRole("button", { name: "Browser source owner", exact: true }).click();
  await page.getByRole("button", { name: "Preview delivery" }).click();
  await expect(page.getByText("1 notification recipients")).toBeVisible();
  await page.getByRole("button", { name: "Send comment" }).click();
  await expect(page.getByText("Comment confirmed.")).toBeVisible();

  await page.getByRole("button", { name: "Transfer", exact: true }).click();
  await page.getByLabel("Exact recipient").fill(recipientEmail);
  await page.getByRole("button", { name: "Find account" }).click();
  await expect(page.getByText("Selected account: Browser recipient")).toBeVisible();
  await page.getByLabel("Note", { exact: true }).fill("Browser handoff note");
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(page.getByText("No issues need your handling.")).toBeVisible();
  await expect(page.getByRole("heading", { name: created.title, exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "Activity", exact: true }).getByText("Browser handoff note")).toBeVisible();
  await page.getByRole("link", { name: "Participating", exact: true }).click();
  await expect(page.getByRole("region", { name: "Collaboration issues" }).getByRole("link", { name: new RegExp(created.title) })).toBeVisible();
  await expect(page.getByRole("button", { name: "Transfer", exact: true })).toHaveCount(0);
  const incoming = await recipient.requestJSON<{ items: { id: string }[] }>("/api/me/workbench?view=current");
  expect(incoming.items.map((item) => item.id)).toContain(created.id);
  const recipientPage = await browser.newPage();
  try {
    await authenticatePage(recipientPage, recipient.getToken()!);
    await recipientPage.goto(new URL(account, page.url()).href);
    await expect(recipientPage.getByRole("heading", { name: created.title, exact: true })).toBeVisible();
    await expect(recipientPage.getByRole("button", { name: "Transfer", exact: true })).toBeVisible();
  } finally { await recipientPage.close(); }

  await page.locator('[data-tab-scroll-root="account-detail"]').evaluate((element) => { element.scrollTop = 0; });
  await page.screenshot({ path: testInfo.outputPath("workbench-web-wide.png"), animations: "disabled" });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("link", { name: "Back to list" })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("workbench-web-narrow.png"), animations: "disabled" });

  const beforeRemove = await source.requestJSON<Snapshot>(`/api/collaboration/issues/${created.id}`);
  await source.requestJSON(`/api/collaboration/issues/${created.id}/participants/${actor.id}/remove`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: writeContext(beforeRemove) });
  await page.reload();
  await expect(page.getByRole("heading", { name: created.title, exact: true })).toHaveCount(0);
  await expect(page.getByText("You no longer have access to this issue.").first()).toBeVisible();
  expect(accountHeaders.length).toBeGreaterThan(5);
  expect(accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
  expect(errors).toEqual([]);
});

test("Web stale lifecycle conflict immediately reports refusal and preserves input for explicit retry", async ({ page, browser }, testInfo) => {
  test.setTimeout(120_000);
  test.skip(!apiUrl || !databaseUrl || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the isolated collaboration API/database and production Web with creation enabled");
  for (const value of [apiUrl!, databaseUrl!, process.env.NEXT_PUBLIC_API_URL!]) {
    expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
  }
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);

  const key = randomUUID().slice(0, 10);
  const creator = new TestApiClient();
  const creatorEmail = `web-conflict-creator-${key}@example.invalid`;
  const creatorName = `Browser conflict creator ${key}`;
  await creator.login(creatorEmail, creatorName);
  await creator.markUserOnboarded();
  const source = await creator.ensureWorkspace("Browser conflict source", `web-conflict-source-${key}`);
  const creatorAccount = await creator.requestJSON<{ id: string }>("/api/me");
  const handler = new TestApiClient();
  await handler.login(`web-conflict-handler-${key}@example.invalid`, "Browser external handler");
  await handler.markUserOnboarded();
  const external = await handler.ensureWorkspace("Browser conflict external", `web-conflict-external-${key}`);
  const handlerAccount = await handler.requestJSON<{ id: string }>("/api/me");
  expect((await creator.getWorkspaces()).map((workspace) => workspace.id)).toEqual([source.id]);
  expect((await handler.getWorkspaces()).map((workspace) => workspace.id)).toEqual([external.id]);
  expect(source.id).not.toBe(external.id);
  expect(await creator.requestJSON("/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const issue = await creator.requestJSON<Snapshot & { title: string }>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: source.id, handler_user_id: handlerAccount.id,
      title: `Browser lifecycle conflict ${key}`, description: "A real concurrent handoff must refuse an outdated completion without losing its form.", attachment_ids: [],
    },
  });

  const handlerContext = await browser.newContext();
  let releaseOriginal = () => {};
  try {
    const handlerPage = await handlerContext.newPage();
    const errors: string[] = [];
    const lifecyclePosts: Request[] = [];
    const operationReads: string[] = [];
    const lifecycle = `/api/collaboration/issues/${issue.id}/lifecycle`;
    for (const view of [page, handlerPage]) view.on("pageerror", (error) => errors.push(error.message));
    page.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (path === lifecycle && request.method() === "POST") lifecyclePosts.push(request);
      if (path.startsWith(`/api/collaboration/issues/${issue.id}/operations/`) && request.method() === "GET") operationReads.push(path);
    });
    await authenticatePage(page, creator.getToken()!);
    await authenticatePage(handlerPage, handler.getToken()!);
    const account = `/account?view=participating&issue=${issue.id}`;
    await page.goto(account);
    await handlerPage.goto(new URL(account, page.url()).href);
    for (const view of [page, handlerPage]) {
      await expect(view.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
    }
    await page.getByRole("button", { name: "Complete issue", exact: true }).click();
    const form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Complete issue", exact: true }) });
    const status = form.getByLabel("Status", { exact: true });
    await status.selectOption("done");
    await expect(status).toHaveValue("done");

    // Delay only the first real POST. The other account changes the same issue
    // before the original request reaches the server; no receipt is fabricated.
    const continueOriginal = new Promise<void>((resolve) => { releaseOriginal = resolve; });
    await page.route(`**${lifecycle}`, async (route) => {
      await continueOriginal;
      await route.continue();
    }, { times: 1 });
    const originalRequest = page.waitForRequest((request) => new URL(request.url()).pathname === lifecycle && request.method() === "POST");
    const originalResponse = page.waitForResponse((response) => new URL(response.url()).pathname === lifecycle && response.request().method() === "POST");
    await form.getByRole("button", { name: "Confirm", exact: true }).click();
    const original = await originalRequest;
    const originalKey = original.headers()["idempotency-key"];
    expect(originalKey).toBeTruthy();
    expect(lifecyclePosts).toHaveLength(1);
    expect(original.postDataJSON()).toMatchObject({ expected_collaboration_revision: issue.collaboration_revision, status: "done" });

    await handlerPage.getByRole("button", { name: "Transfer", exact: true }).click();
    const transfer = handlerPage.locator("form").filter({ has: handlerPage.getByRole("heading", { name: "Transfer", exact: true }) });
    await transfer.getByLabel("Exact recipient", { exact: true }).fill(creatorEmail);
    await transfer.getByRole("button", { name: "Find account", exact: true }).click();
    await expect(transfer.getByText(`Selected account: ${creatorName}`, { exact: true })).toBeVisible();
    await transfer.getByLabel("Note", { exact: true }).fill("Concurrent handoff makes the original completion stale.");
    const transferred = handlerPage.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${issue.id}/handoffs` && response.request().method() === "POST");
    await transfer.getByRole("button", { name: "Confirm", exact: true }).click();
    expect((await transferred).status()).toBe(200);
    const latest = await creator.requestJSON<Snapshot & { handler: { user_id: string }; status: string }>(`/api/collaboration/issues/${issue.id}`);
    expect(latest.handler.user_id).toBe(creatorAccount.id);
    expect(latest.collaboration_revision).not.toBe(issue.collaboration_revision);
    expect(latest.status).toBe("todo");

    releaseOriginal();
    const refused = await originalResponse;
    expect(refused.status()).toBe(409);
    const refusedBody = await refused.json();
    expect(refusedBody).toMatchObject({ code: "stale_revision" });
    // Core owns statusless-response coverage; servers may also echo HTTP status.
    if (Object.hasOwn(refusedBody, "status")) expect(refusedBody.status).toBe(refused.status());
    await expect(page.getByText("The server did not apply this action. Refresh the issue before trying again.", { exact: true })).toBeVisible();
    await expect(page.getByText("Could not load collaboration. Try again.", { exact: true })).toHaveCount(0);
    await expect(page.getByText("The result is not confirmed. Keep this request and check its original outcome.", { exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Check original result", exact: true })).toHaveCount(0);
    await expect(form).toBeVisible();
    await expect(status).toHaveValue("done");
    const useCurrent = form.getByRole("button", { name: "Keep input and use current version", exact: true });
    await expect(useCurrent).toBeEnabled();
    expect(lifecyclePosts).toHaveLength(1);
    expect(operationReads).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath("lifecycle-conflict-refused.png"), animations: "disabled" });

    await useCurrent.click();
    await expect(useCurrent).toHaveCount(0);
    await expect(status).toHaveValue("done");
    expect(lifecyclePosts).toHaveLength(1);
    const completed = page.waitForResponse((response) => new URL(response.url()).pathname === lifecycle && response.request().method() === "POST");
    await form.getByRole("button", { name: "Confirm", exact: true }).click();
    const committed = await completed;
    expect(committed.status()).toBe(200);
    const freshKey = committed.request().headers()["idempotency-key"];
    expect(freshKey).toBeTruthy();
    expect(freshKey).not.toBe(originalKey);
    expect(committed.request().postDataJSON()).toMatchObject({ expected_collaboration_revision: latest.collaboration_revision, status: "done" });
    await expect(form).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Reopen issue", exact: true })).toBeVisible();
    const final = await creator.requestJSON<Snapshot & { status: string; handler: { user_id: string }; execution: { commands: unknown[]; groups: unknown[]; runs: unknown[] } }>(`/api/collaboration/issues/${issue.id}`);
    expect(final).toMatchObject({ id: issue.id, status: "done", handler: { user_id: creatorAccount.id }, execution: { commands: [], groups: [], runs: [] } });
    expect(await creator.countIssueDispatches(issue.id)).toBe(0);
    expect(lifecyclePosts).toHaveLength(2);
    expect(operationReads).toEqual([]);
    expect(errors).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath("lifecycle-conflict-completed.png"), animations: "disabled" });
    await testInfo.attach("lifecycle-conflict-evidence", { body: JSON.stringify({ issueId: issue.id, sourceWorkspaceId: source.id,
      externalWorkspaceId: external.id, originalKey, freshKey, originalRevision: issue.collaboration_revision, currentRevision: latest.collaboration_revision,
      refusedStatus: refused.status(), refusedBody, committedStatus: committed.status(), lifecyclePosts: lifecyclePosts.length,
      operationReads, finalStatus: final.status, unexpectedRuns: final.execution.runs.length, pageErrors: errors }, null, 2), contentType: "application/json" });
  } finally {
    releaseOriginal();
    await handlerContext.close();
  }
});
