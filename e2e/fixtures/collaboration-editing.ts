import { randomUUID } from "node:crypto";
import { expect, type Locator, type Page, type Route } from "@playwright/test";
import { TestApiClient } from "../fixtures";

type Reference = { id: string; name: string };
type IterationReference = Reference & { revision: number };

export interface CollaborationEditingFixture {
  source: TestApiClient;
  recipient: TestApiClient;
  workspace: { id: string; slug: string; name: string };
  recipientId: string;
  recipientEmail: string;
  recipientName: string;
  title: string;
  labels: [Reference, Reference];
  property: Reference;
  project: { id: string; title: string };
  parent: { id: string; title: string };
  ordinary: { id: string; title: string };
  iterations: [IterationReference, IterationReference];
}

// This fixture creates definitions through their ordinary authorized APIs. The
// shared issue itself is created by each real client under test.
export async function seedCollaborationEditing(): Promise<CollaborationEditingFixture> {
  const key = randomUUID().slice(0, 10);
  const source = new TestApiClient();
  await source.login(`editing-source-${key}@example.invalid`, "Editing source owner");
  const workspace = await source.ensureWorkspace("Editing source workspace", `editing-source-${key}`);
  await source.markUserOnboarded();

  const recipient = new TestApiClient();
  const recipientEmail = `editing-recipient-${key}@example.invalid`;
  const recipientName = `External editor ${key}`;
  await recipient.login(recipientEmail, recipientName);
  await recipient.markUserOnboarded();
  const recipientId = (await recipient.requestJSON<{ id: string }>("/api/me")).id;

  const labels: [Reference, Reference] = [
    await source.requestJSON<Reference>("/api/labels", { method: "POST", body: { name: "Customer request", color: "#2563eb" } }),
    await source.requestJSON<Reference>("/api/labels", { method: "POST", body: { name: "Validated", color: "#16a34a" } }),
  ];
  const property = await source.requestJSON<Reference>("/api/properties", {
    method: "POST", body: { name: "Delivery notes", type: "text" },
  });
  const project = await source.requestJSON<{ id: string; title: string }>("/api/projects", {
    method: "POST", body: { title: "Source delivery project" },
  });
  const parent = await source.createIssue("Source parent reference", {
    description: "Private parent content must not enter the external collaboration projection.",
  });
  const ordinary = await source.createIssue("Ordinary issue outside collaboration");
  const base = `/api/workspaces/${workspace.id}`;
  const settings = await source.requestJSON<{ effective_timezone: string }>(`${base}/iteration-settings`);
  const startDate = new Date().toISOString().slice(0, 10);
  const endDate = new Date(Date.now() + 14 * 86400_000).toISOString().slice(0, 10);
  for (const name of ["Initial delivery", "Next delivery"]) {
    await source.requestJSON(`${base}/iterations`, { method: "POST", body: {
      request_id: randomUUID(), name, start_date: startDate, end_date: endDate,
      confirmed_timezone: settings.effective_timezone,
    } });
  }
  const periods = await source.requestJSON<{ items: IterationReference[] }>(`${base}/iterations`);
  const first = periods.items.find((item) => item.name === "Initial delivery");
  const next = periods.items.find((item) => item.name === "Next delivery");
  if (!first || !next) throw new Error("The isolated editing fixture did not create both iterations");
  return { source, recipient, workspace, recipientId, recipientEmail, recipientName,
    title: `Source collaboration ${key}`, labels, property, project, parent, ordinary, iterations: [first, next] };
}

export async function authenticateCollaborationEditor(page: Page, token: string) {
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
}

async function toggleLabel(page: Page, container: Locator, name: string) {
  await container.getByRole("button", { name: "Labels", exact: true }).click();
  await expect(page.getByPlaceholder("Find a label", { exact: true })).toBeVisible();
  await page.getByRole("button", { name, exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(page.getByPlaceholder("Find a label", { exact: true })).toBeHidden();
}

async function setTextProperty(page: Page, container: Locator, name: string, value: string) {
  await container.getByRole("button", { name, exact: true }).click();
  const input = page.getByPlaceholder("Enter value…", { exact: true });
  await input.fill(value);
  await input.press("Enter");
  await expect(input).toBeHidden();
}

export async function createCollaborationWithInitialFields(page: Page, fixture: CollaborationEditingFixture) {
  await page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
  await expect(dialog.getByLabel("Source workspace", { exact: true })).toHaveValue(fixture.workspace.id);
  await dialog.getByLabel("Title", { exact: true }).fill(fixture.title);
  await dialog.getByLabel("Description", { exact: true }).fill("One issue shared with an external handler.");
  await dialog.getByLabel("Full username, email, or account ID", { exact: true }).fill(fixture.recipientEmail);
  await dialog.getByRole("button", { name: "Find account", exact: true }).click();
  await dialog.getByRole("button", { name: "Use this account", exact: true }).click();
  await dialog.getByText("Initial fields", { exact: true }).click();
  await dialog.getByLabel("Status", { exact: true }).selectOption("todo");
  await dialog.getByLabel("Priority", { exact: true }).selectOption("high");
  await toggleLabel(page, dialog, fixture.labels[0].name);
  await setTextProperty(page, dialog, fixture.property.name, "Initial delivery context");
  await dialog.getByLabel("Project", { exact: true }).selectOption(fixture.project.id);
  await dialog.getByLabel("Parent issue", { exact: true }).selectOption(fixture.parent.id);
  await dialog.getByLabel("Stage under parent", { exact: true }).fill("2");
  await dialog.getByLabel("Iteration", { exact: true }).selectOption(fixture.iterations[0].id);
  const created = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/collaboration/issues" && response.request().method() === "POST");
  await dialog.getByRole("button", { name: "Create collaboration issue", exact: true }).click();
  const response = await created;
  expect(response.status()).toBe(201);
  const issue: { id: string } = await response.json();
  expect(await fixture.source.requestJSON(`/api/collaboration/issues/${issue.id}`)).toMatchObject({
    priority: "high", handler: { user_id: fixture.recipientId },
    labels: expect.arrayContaining([expect.objectContaining({ id: fixture.labels[0].id })]),
    properties: { [fixture.property.id]: "Initial delivery context" },
    source_structure: { project: { id: fixture.project.id }, parent_issue: { id: fixture.parent.id }, stage: 2,
      iteration: { id: fixture.iterations[0].id } },
  });
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  return issue;
}

export async function exerciseCollaborationFields(page: Page, fixture: CollaborationEditingFixture, issueId: string) {
  await page.getByRole("button", { name: "Edit issue", exact: true }).click();
  let form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Edit issue", exact: true }) });
  await toggleLabel(page, form, fixture.labels[1].name);
  await setTextProperty(page, form, fixture.property.name, "Reviewed delivery context");
  await form.getByLabel("Priority", { exact: true }).selectOption("urgent");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(form).toHaveCount(0);
  expect(await fixture.source.requestJSON(`/api/collaboration/issues/${issueId}`)).toMatchObject({
    priority: "urgent", properties: { [fixture.property.id]: "Reviewed delivery context" },
    labels: expect.arrayContaining([expect.objectContaining({ id: fixture.labels[1].id })]),
  });

  await page.getByRole("button", { name: "Edit source fields", exact: true }).click();
  form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Edit source fields", exact: true }) });
  await form.getByLabel("Stage under parent", { exact: true }).fill("3");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(form).toHaveCount(0);
  expect(await fixture.source.requestJSON(`/api/collaboration/issues/${issueId}`)).toMatchObject({ source_structure: { stage: 3 } });

  await page.getByRole("button", { name: "Change iteration", exact: true }).click();
  form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Change iteration", exact: true }) });
  await form.getByLabel("Iteration", { exact: true }).selectOption(fixture.iterations[1].id);
  await expect(form.getByRole("button", { name: "Confirm", exact: true })).toBeDisabled();
  await form.getByLabel("Reason for switching or leaving the iteration", { exact: true }).fill("Continue in the next delivery period.");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(form).toHaveCount(0);
  expect(await fixture.source.requestJSON(`/api/collaboration/issues/${issueId}`)).toMatchObject({ source_structure: { iteration: { id: fixture.iterations[1].id } } });
}

export async function sendCollaborationComment(page: Page, content: string) {
  const composer = page.getByRole("form", { name: "Comments", exact: true });
  await composer.getByLabel("Add comment", { exact: true }).fill(content);
  await composer.getByRole("button", { name: "Preview delivery", exact: true }).click();
  const send = composer.getByRole("button", { name: "Send comment", exact: true });
  await expect(send).toBeEnabled();
  const response = page.waitForResponse((item) => /\/api\/collaboration\/issues\/[^/]+\/comments$/.test(new URL(item.url()).pathname)
    && item.request().method() === "POST");
  await send.click();
  const posted = await response;
  expect(posted.status()).toBe(201);
  const receipt: { comment_id: string; comment_revision: string } = await posted.json();
  await expect(page.locator(`[id="comment-${receipt.comment_id}"]`)).toBeVisible();
  return receipt;
}

export async function exerciseCollaborationDiscussion(page: Page, source: TestApiClient, issueId: string, recover = false) {
  const original = "Original discussion context retained in history.";
  const edited = "Reviewed discussion context with the original reply retained.";
  const parent = await sendCollaborationComment(page, original);
  const parentRow = page.locator(`[id="comment-${parent.comment_id}"]`);
  await parentRow.getByRole("button", { name: "Reply", exact: true }).click();
  const reply = await sendCollaborationComment(page, "A reply that survives parent editing and deletion.");
  expect(await source.requestJSON(`/api/collaboration/issues/${issueId}/comments/${reply.comment_id}`)).toMatchObject({ parent_id: parent.comment_id });

  await parentRow.getByRole("button", { name: "Edit comment", exact: true }).click();
  await parentRow.getByLabel("Edit comment text", { exact: true }).fill(edited);
  const commentURL = `**/api/collaboration/issues/${issueId}/comments/${parent.comment_id}`;
  let intercepted = false;
  let patchCount = 0;
  const countPatch = (request: { method(): string; url(): string }) => {
    if (request.method() === "PATCH" && new URL(request.url()).pathname.endsWith(`/comments/${parent.comment_id}`)) patchCount += 1;
  };
  const loseResponse = async (route: Route) => {
    if (route.request().method() !== "PATCH" || intercepted) return route.continue();
    intercepted = true;
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    await route.abort("failed");
  };
  page.on("request", countPatch);
  if (recover) await page.route(commentURL, loseResponse);
  try {
    await parentRow.getByRole("button", { name: "Save comment", exact: true }).click();
    if (recover) {
      await expect(parentRow.getByRole("button", { name: "Check original result", exact: true })).toBeEnabled();
      await page.unroute(commentURL, loseResponse);
      await page.reload();
      await expect(parentRow.getByLabel("Edit comment text", { exact: true })).toHaveValue(edited);
      await parentRow.getByRole("button", { name: "Check original result", exact: true }).click();
    }
    await expect(parentRow.getByRole("form", { name: "Edit comment", exact: true })).toHaveCount(0);
    await expect(parentRow.getByText(edited, { exact: true })).toBeVisible();
    expect(patchCount).toBe(1);
  } finally {
    page.off("request", countPatch);
    if (recover) await page.unroute(commentURL, loseResponse);
  }
  await parentRow.getByRole("button", { name: "Comment history", exact: true }).click();
  // Creation records the original as "after"; the edit records it as "before".
  await expect(parentRow.getByText(original, { exact: true })).toHaveCount(2);
  for (const entry of await parentRow.getByText(original, { exact: true }).all()) await expect(entry).toBeVisible();
  await parentRow.getByRole("button", { name: "Comment history", exact: true }).click();

  const before = await source.requestJSON<{ comment_revision: string }>(`/api/collaboration/issues/${issueId}/comments/${parent.comment_id}`);
  await parentRow.getByRole("button", { name: "Choose a reaction", exact: true }).click();
  await page.getByRole("button", { name: "👍", exact: true }).click();
  await expect(parentRow.getByRole("button", { name: "Remove your 👍 reaction", exact: true })).toHaveAttribute("aria-pressed", "true");
  await parentRow.getByRole("button", { name: "Resolve thread", exact: true }).click();
  await expect(parentRow.getByRole("button", { name: "Reopen thread", exact: true })).toBeEnabled();
  await parentRow.getByRole("button", { name: "Reopen thread", exact: true }).click();
  await expect(parentRow.getByRole("button", { name: "Resolve thread", exact: true })).toBeEnabled();
  await parentRow.getByRole("button", { name: "Remove your 👍 reaction", exact: true }).click();
  await expect(parentRow.getByRole("button", { name: "Remove your 👍 reaction", exact: true })).toHaveCount(0);
  expect(await source.requestJSON(`/api/collaboration/issues/${issueId}/comments/${parent.comment_id}`)).toMatchObject({ comment_revision: before.comment_revision });

  const issueReactions = page.getByRole("region", { name: "Issue reactions", exact: true });
  await issueReactions.getByRole("button", { name: "Choose a reaction", exact: true }).click();
  await page.getByRole("button", { name: "🎉", exact: true }).click();
  await expect(issueReactions.getByRole("button", { name: "Remove your 🎉 reaction", exact: true })).toHaveAttribute("aria-pressed", "true");

  await parentRow.getByRole("button", { name: "Delete comment", exact: true }).click();
  await parentRow.getByRole("button", { name: "Confirm deletion", exact: true }).click();
  await expect(parentRow.getByText("Comment deleted", { exact: true })).toBeVisible();
  const replyRow = page.locator(`[id="comment-${reply.comment_id}"]`);
  await expect(replyRow.getByText("A reply that survives parent editing and deletion.", { exact: true })).toBeVisible();
  await replyRow.getByRole("link", { name: "View parent comment", exact: true }).click();
  await expect(parentRow).toBeVisible();
  await parentRow.getByRole("button", { name: "Comment history", exact: true }).click();
  await expect(parentRow.getByText(original, { exact: true })).toHaveCount(2);
  for (const entry of await parentRow.getByText(original, { exact: true }).all()) await expect(entry).toBeVisible();
  return { parentId: parent.comment_id, replyId: reply.comment_id, recovered: recover, patchCount };
}
