import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import { test, expect, type Page } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

const apiUrl = process.env.COLLABORATION_E2E_API_URL;
type Snapshot = { id: string; collaboration_revision: string; lifecycle_epoch: string; write_context: { participant_epoch: string; account_epoch: string } };
type FileMetadata = { id: string; draft_id: string; issue_id: string | null; intended_issue_id: string | null; version: string; name: string };
type Frame = { type: string; payload?: { scope?: string; id?: string; issue_id?: string; event_seq?: string } };
const writeContext = (issue: Snapshot) => ({ expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
  expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch });
async function authenticate(page: Page, token: string) {
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
}

test("Web creates one private issue after lost upload/create responses and revokes an open preview through account WS", async ({ page, browser }, testInfo) => {
  test.setTimeout(180_000);
  test.skip(!apiUrl || !process.env.DATABASE_URL || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the isolated collaboration API/database and production Web build with creation explicitly enabled");
  for (const value of [apiUrl!, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);
  const key = randomUUID().slice(0, 10);
  const source = new TestApiClient();
  await source.login(`delivery-source-${key}@example.invalid`, "Delivery source owner");
  const workspace = await source.ensureWorkspace("Delivery source", `delivery-source-${key}`);
  await source.markUserOnboarded();
  expect(await source.requestJSON("/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const viewer = new TestApiClient();
  const viewerEmail = `delivery-viewer-${key}@example.invalid`;
  await viewer.login(viewerEmail, "Delivery recipient");
  await viewer.markUserOnboarded();
  const viewerId = (await viewer.requestJSON<{ id: string }>("/api/me")).id;
  expect(await viewer.getWorkspaces()).toEqual([]);
  const errors: string[] = [], requestScopes: boolean[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    if (!request.url().includes("/api/collaboration/")) return;
    requestScopes.push(!("x-workspace-id" in request.headers()) && !("x-workspace-slug" in request.headers()));
  });
  await authenticate(page, source.getToken()!);
  await page.goto("/account");
  await page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
  let dialog = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
  const title = `Created collaboration ${key}`;
  await dialog.getByLabel("Title", { exact: true }).fill(title);
  await dialog.getByLabel("Description", { exact: true }).fill("Creation body stays on the original issue.");
  await dialog.getByLabel("Full username, email, or account ID").fill(viewerEmail);
  await expect(dialog.getByRole("button", { name: "Create collaboration issue", exact: true })).toBeDisabled();
  await dialog.getByRole("button", { name: "Find account", exact: true }).click();
  await dialog.getByRole("button", { name: "Use this account", exact: true }).click();
  let uploaded: FileMetadata | undefined, uploadKey = "", uploadPosts = 0;
  page.on("request", (request) => { if (new URL(request.url()).pathname === "/api/collaboration/attachments/drafts" && request.method() === "POST") uploadPosts += 1; });
  await page.route("**/api/collaboration/attachments/drafts", async (route) => {
    uploadKey = route.request().headers()["idempotency-key"] ?? "";
    const response = await route.fetch();
    expect(response.status()).toBe(201);
    uploaded = await response.json();
    await route.abort("failed");
  }, { times: 1 });
  await dialog.getByLabel("Attach files", { exact: true }).setInputFiles({ name: "creation-note.txt", mimeType: "text/plain", buffer: Buffer.from("Private creation attachment") });
  await expect(dialog.getByRole("button", { name: "Check upload", exact: true })).toBeEnabled();
  expect(uploaded).toMatchObject({ issue_id: null, intended_issue_id: null });
  expect(uploadKey).not.toBe("");
  await page.reload();
  await page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
  dialog = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
  await expect(dialog.getByLabel("Title", { exact: true })).toHaveValue(title);
  await expect(dialog.getByLabel("Description", { exact: true })).toHaveValue("Creation body stays on the original issue.");
  const uploadRecovery = page.waitForResponse((response) => response.url().includes(`/drafts/${uploaded!.draft_id}/operations/${uploadKey}`));
  await dialog.getByRole("button", { name: "Check upload", exact: true }).click();
  expect((await uploadRecovery).status()).toBe(200);
  await expect(dialog.getByRole("checkbox", { name: "Include creation-note.txt", exact: true })).toBeChecked();

  let createdId = "", createKey = "", createPosts = 0;
  page.on("request", (request) => { if (new URL(request.url()).pathname === "/api/collaboration/issues" && request.method() === "POST") createPosts += 1; });
  await page.route("**/api/collaboration/issues", async (route) => {
    createKey = route.request().headers()["idempotency-key"] ?? "";
    expect(route.request().postDataJSON()).toMatchObject({ source_workspace_id: workspace.id, handler_user_id: viewerId, attachment_ids: [uploaded!.id] });
    const response = await route.fetch();
    expect(response.status()).toBe(201);
    createdId = (await response.json()).id;
    await route.abort("failed");
  }, { times: 1 });
  await dialog.getByRole("button", { name: "Create collaboration issue", exact: true }).click();
  await expect(dialog.getByRole("button", { name: "Check creation result", exact: true })).toBeEnabled();
  expect(createdId).not.toBe("");
  await page.reload();
  await page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
  dialog = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
  const creationRecovery = page.waitForResponse((response) => response.url().includes(`/creations/${createKey}?`));
  await dialog.getByRole("button", { name: "Check creation result", exact: true }).click();
  expect((await creationRecovery).status()).toBe(200);
  await expect(page).toHaveURL(new RegExp(`/account\\?view=participating&issue=${createdId}$`));
  await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
  expect(uploadPosts).toBe(1); expect(createPosts).toBe(1);
  const recovered = await source.requestJSON<{ id: string }>(`/api/collaboration/creations/${createKey}?source_workspace_id=${workspace.id}`);
  expect(recovered.id).toBe(createdId);

  const recipientContext = await browser.newContext();
  try {
    const recipientPage = await recipientContext.newPage();
    const frames: Frame[] = [], accountSockets: string[] = [];
    recipientPage.on("pageerror", (error) => errors.push(error.message));
    recipientPage.on("websocket", (socket) => {
      if (new URL(socket.url()).searchParams.get("scope") !== "account") return;
      accountSockets.push(new URL(socket.url()).pathname);
      socket.on("framereceived", ({ payload }) => {
        try {
          const value: unknown = JSON.parse(payload.toString());
          if (typeof value === "object" && value !== null && "type" in value && typeof value.type === "string" &&
            (value.type.startsWith("collaboration:") || value.type === "subscribe_ack" || value.type === "auth_ack")) frames.push(value as Frame);
        } catch { /* Non-JSON transport frames are not collaboration evidence. */ }
      });
    });
    await authenticate(recipientPage, viewer.getToken()!);
    await recipientPage.goto(`/account?view=current&issue=${createdId}`);
    await expect(recipientPage.getByRole("heading", { name: title, exact: true })).toBeVisible();
    await expect.poll(() => frames.some((frame) => frame.type === "subscribe_ack" && frame.payload?.scope === "issue" && frame.payload.id === createdId)).toBe(true);
    expect(accountSockets.length).toBeGreaterThan(0);
    const files = await viewer.requestJSON<{ attachments: FileMetadata[] }>(`/api/collaboration/issues/${createdId}/attachments`);
    expect(files.attachments).toEqual(expect.arrayContaining([expect.objectContaining({ id: uploaded!.id, issue_id: createdId })]));

    const comment = page.getByRole("form", { name: "Comments", exact: true });
    const pixel = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg==", "base64");
    await comment.getByLabel("Add comment", { exact: true }).fill("A comment with a separately delivered private file.");
    await comment.getByLabel("Attach files", { exact: true }).setInputFiles({ name: "comment-preview.png", mimeType: "image/png", buffer: pixel });
    await expect(comment.getByRole("checkbox", { name: "Include comment-preview.png", exact: true })).toBeChecked();
    await comment.getByRole("button", { name: "Preview delivery", exact: true }).click();
    await expect(comment.getByRole("button", { name: "Send comment", exact: true })).toBeEnabled();
    const commentResponse = page.waitForResponse((response) => response.url().endsWith(`/issues/${createdId}/comments`) && response.request().method() === "POST");
    await comment.getByRole("button", { name: "Send comment", exact: true }).click();
    const posted = await commentResponse;
    expect(posted.status()).toBe(201);
    expect(posted.request().postDataJSON().draft_id).not.toBe(uploaded!.draft_id);
    const commentReceipt = await posted.json();
    expect(commentReceipt.attachment_ids).toHaveLength(1);
    const imageId = commentReceipt.attachment_ids[0];
    const recipientAttachments = recipientPage.getByRole("region", { name: "Attachments", exact: true });
    await expect(recipientAttachments.getByRole("button", { name: "Preview comment-preview.png", exact: true })).toBeVisible();

    const capability = await viewer.requestJSON<{ url: string }>(`/api/collaboration/issues/${createdId}/attachments/${imageId}/capabilities`, { method: "POST", body: { intent: "download" } });
    const nativeUrl = new URL(capability.url, apiUrl!);
    expect(nativeUrl.origin).toBe(new URL(apiUrl!).origin);
    const range = await fetch(nativeUrl, { headers: { Range: "bytes=0-7" } });
    expect(range.status).toBe(206);
    expect(Buffer.from(await range.arrayBuffer())).toEqual(pixel.subarray(0, 8));
    const downloadPromise = recipientPage.waitForEvent("download");
    await recipientAttachments.getByRole("button", { name: "Download comment-preview.png", exact: true }).click();
    const download = await downloadPromise;
    expect(await download.failure()).toBeNull();
    expect(await readFile((await download.path())!)).toEqual(pixel);

    // Removing a display reference keeps permanent issue delivery readable.
    await source.requestJSON(`/api/collaboration/issues/${createdId}/comments/${commentReceipt.comment_id}`, { method: "PATCH", headers: { "Idempotency-Key": randomUUID() },
      body: { content: "The comment no longer displays its file.", expected_comment_revision: commentReceipt.comment_revision, attachment_ids: [] } });
    expect(await viewer.requestJSON(`/api/collaboration/issues/${createdId}/attachments/${imageId}`)).toMatchObject({ id: imageId, issue_id: createdId });
    await recipientPage.getByRole("textbox", { name: "Add comment", exact: true }).fill("Viewer unsent private draft");
    await recipientAttachments.getByRole("button", { name: "Preview comment-preview.png", exact: true }).click();
    await expect(recipientPage.getByRole("img", { name: "comment-preview.png", exact: true })).toBeVisible();
    await expect.poll(() => recipientPage.getByRole("img", { name: "comment-preview.png", exact: true }).evaluate((image) => (image as HTMLImageElement).naturalWidth)).toBe(1);
    await expect(recipientPage.getByRole("img", { name: "comment-preview.png", exact: true })).toHaveAttribute("src", /^blob:/);
    const beforeRemoval = frames.length;
    const latest = await source.requestJSON<Snapshot>(`/api/collaboration/issues/${createdId}`);
    await source.requestJSON(`/api/collaboration/issues/${createdId}/participants/${viewerId}/remove`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: writeContext(latest) });
    // A real account WS invalidation must arrive before the UI assertion; no reload.
    await expect.poll(() => frames.slice(beforeRemoval).some((frame) => frame.type === "collaboration:access_changed" && frame.payload?.issue_id === createdId), { timeout: 5000 }).toBe(true);
    await expect(recipientPage.getByRole("dialog")).toHaveCount(0, { timeout: 5000 });
    await expect(recipientPage.getByRole("heading", { name: title, exact: true })).toHaveCount(0);
    await expect(recipientPage.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
    expect(await recipientPage.evaluate(() => Object.values(localStorage).some((value) => value.includes("Viewer unsent private draft")))).toBe(false);
    expect((await fetch(nativeUrl)).status).toBe(404);
    await recipientPage.screenshot({ path: testInfo.outputPath("collaboration-preview-revoked.png"), animations: "disabled" });
    await testInfo.attach("creation-delivery-evidence", { body: JSON.stringify({ issueId: createdId, uploadKey, createKey, uploadPosts, createPosts,
      sourceWorkspaceId: workspace.id, accountSocketCount: accountSockets.length, frames, noAmbientWorkspace: requestScopes.every(Boolean) }, null, 2), contentType: "application/json" });
  } finally { await recipientContext.close(); }
  expect(errors).toEqual([]);
  expect(requestScopes.every(Boolean)).toBe(true);
  // Collaboration records are retained by contract in this disposable database.
  // Do not bypass that contract through ordinary issue/workspace deletion.
});

test("Web explicitly resubmits rejected creation input without losing files or reviving the rejected key", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  test.skip(!apiUrl || !process.env.DATABASE_URL || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the isolated collaboration API/database and production Web build with creation explicitly enabled");
  for (const value of [apiUrl!, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);
  const suffix = randomUUID().slice(0, 10);
  const creator = new TestApiClient();
  await creator.login(`creation-correction-${suffix}@example.invalid`, "Creation correction owner");
  const workspace = await creator.ensureWorkspace("Creation correction", `creation-correction-${suffix}`);
  await creator.markUserOnboarded();
  const actor = await creator.requestJSON<{ id: string }>("/api/me");
  const recipient = new TestApiClient();
  const recipientEmail = `creation-recipient-${suffix}@example.invalid`;
  await recipient.login(recipientEmail, "Unavailable creation recipient");
  await recipient.markUserOnboarded();
  const target = await recipient.requestJSON<{ id: string }>("/api/me");
  expect(await creator.requestJSON("/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const db = new pg.Client(process.env.DATABASE_URL!);
  const errors: string[] = [], createKeys: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/api/collaboration/issues" && request.method() === "POST") createKeys.push(request.headers()["idempotency-key"] ?? "");
  });
  await db.connect();
  try {
    await authenticate(page, creator.getToken()!);
    await page.goto("/account");
    await page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
    let dialog = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
    const title = `Corrected creation ${suffix}`;
    await dialog.getByLabel("Title", { exact: true }).fill(title);
    await dialog.getByLabel("Description", { exact: true }).fill("Keep this input and its unconsumed private file.");
    await dialog.getByLabel("Full username, email, or account ID").fill(recipientEmail);
    await dialog.getByRole("button", { name: "Find account", exact: true }).click();
    await dialog.getByRole("button", { name: "Use this account", exact: true }).click();
    const uploadedResponse = page.waitForResponse((response) => response.url().endsWith("/api/collaboration/attachments/drafts") && response.request().method() === "POST");
    await dialog.getByLabel("Attach files", { exact: true }).setInputFiles({ name: "correction-note.txt", mimeType: "text/plain", buffer: Buffer.from("Preserve this uploaded identity") });
    const uploaded = await (await uploadedResponse).json();
    expect(uploaded).toMatchObject({ source_workspace_id: workspace.id, issue_id: null, intended_issue_id: null });
    await expect(dialog.getByRole("checkbox", { name: "Include correction-note.txt", exact: true })).toBeChecked();
    await db.query('UPDATE "user" SET disabled_at=now() WHERE id=$1', [target.id]);
    const refusalResponse = page.waitForResponse((response) => response.url().endsWith("/api/collaboration/issues") && response.request().method() === "POST");
    const resultResponse = page.waitForResponse((response) => /\/creations\/[^/]+\/result\?/.test(response.url()));
    await dialog.getByRole("button", { name: "Create collaboration issue", exact: true }).click();
    const refusal = await refusalResponse;
    expect(refusal.status()).toBe(403);
    const rejectedInput: unknown = refusal.request().postDataJSON();
    expect(await (await resultResponse).json()).toEqual({ request_id: createKeys[0], source_workspace_id: workspace.id,
      actor_user_id: actor.id, outcome: "rejected", code: "account_unavailable" });
    const message = "The issue was not created. Your draft and files are preserved; you can submit again.";
    await expect(dialog.getByText(message, { exact: true })).toBeVisible();
    await expect(dialog.getByLabel("Title", { exact: true })).toBeEnabled();
    await expect(dialog.getByRole("checkbox", { name: "Include correction-note.txt", exact: true })).toBeChecked();
    await expect(dialog.getByRole("button", { name: "Create collaboration issue", exact: true })).toBeEnabled();

    await page.reload();
    await page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
    dialog = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
    await expect(dialog.getByLabel("Title", { exact: true })).toHaveValue(title);
    await expect(dialog.getByRole("checkbox", { name: "Include correction-note.txt", exact: true })).toBeChecked();
    await expect(dialog.getByRole("button", { name: "Create collaboration issue", exact: true })).toBeEnabled();
    expect(createKeys).toHaveLength(1);
    await db.query('UPDATE "user" SET disabled_at=NULL WHERE id=$1', [target.id]);
    const correctedResponse = page.waitForResponse((response) => response.url().endsWith("/api/collaboration/issues") && response.request().method() === "POST");
    await dialog.getByRole("button", { name: "Create collaboration issue", exact: true }).click();
    const corrected = await correctedResponse;
    expect(corrected.status()).toBe(201);
    expect(corrected.request().postDataJSON()).toEqual(rejectedInput);
    expect(corrected.request().postDataJSON()).toMatchObject({ handler_user_id: target.id, draft_id: uploaded.draft_id, attachment_ids: [uploaded.id] });
    const issue = await corrected.json();
    await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
    expect(createKeys).toHaveLength(2);
    expect(createKeys[0]).not.toBe(createKeys[1]);
    expect(await creator.requestJSON(`/api/collaboration/issues/${issue.id}/attachments`)).toMatchObject({ attachments: [expect.objectContaining({ id: uploaded.id })] });
    const replay = await fetch(`${apiUrl}/api/collaboration/issues`, { method: "POST",
      headers: { Authorization: `Bearer ${creator.getToken()}`, "Content-Type": "application/json", "Idempotency-Key": createKeys[0]! }, body: JSON.stringify(rejectedInput) });
    expect(replay.status).toBe(403);
    expect(await replay.json()).toMatchObject({ code: "account_unavailable" });
    const workbench = await creator.requestJSON<{ items: { id: string; title: string }[] }>("/api/me/workbench?view=participating");
    expect(workbench.items.filter((item) => item.title === title)).toEqual([expect.objectContaining({ id: issue.id })]);
    expect(errors).toEqual([]);
    await testInfo.attach("creation-rejection-correction", { contentType: "application/json", body: JSON.stringify({ issueId: issue.id, uploadId: uploaded.id,
      rejectedRequestId: createKeys[0], correctedRequestId: createKeys[1], originalRejectionRetained: true }) });
  } finally { await db.end(); }
});
