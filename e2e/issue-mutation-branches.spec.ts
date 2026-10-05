import { randomUUID } from "node:crypto";
import {
  expect,
  test as base,
  type Page,
  type Request,
  type Response,
  type Route,
} from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Cache/enum matrices remain canonical in core/issues/mutations.test.tsx.
// These cases exercise browser rollback, draft retention and real-server retry.
type Issue = {
  id: string;
  identifier: string;
  title: string;
  description: string;
  status: string;
  priority: string;
};
type Comment = { id: string; content: string };
type Fixture = {
  api: TestApiClient;
  workspace: Awaited<ReturnType<TestApiClient["ensureWorkspace"]>>;
};

const test = base.extend<{ fixture: Fixture }>({
  fixture: async ({ page }, use) => {
    const suffix = randomUUID().slice(0, 12);
    const api = new TestApiClient();
    await api.login(`e2e-mutations-${suffix}@example.invalid`, "Mutation QA");
    const workspace = await api.ensureWorkspace(
      `Mutation QA ${suffix}`,
      `mutation-qa-${suffix}`,
    );
    try {
      expect(workspace.slug).toBe(`mutation-qa-${suffix}`);
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", {
        method: "PATCH",
        body: { language: "en" },
      });
      const token = api.getToken();
      if (!token) throw new Error("Mutation fixture authentication is missing");
      await page.addInitScript((token) => {
        localStorage.setItem("multica_token", token);
        localStorage.setItem("multica-locale", "en");
        localStorage.setItem("multica:chat:isOpen", "false");
        localStorage.setItem(
          "multica_create_mode",
          JSON.stringify({ state: { lastMode: "manual" }, version: 0 }),
        );
      }, token);
      await use({ api, workspace });
    } finally {
      await api.deleteFeatureWorkspace(workspace.id);
    }
  },
});

test.use({
  locale: "en-US",
  viewport: { width: 1440, height: 1000 },
  trace: "retain-on-failure",
  screenshot: "only-on-failure",
  actionTimeout: 15_000,
});

function matches(response: Response, path: string, method: string) {
  return new URL(response.url()).pathname === path &&
    response.request().method() === method;
}

/** Hold one browser write; remove the fault before any retry reaches the API. */
async function holdFirstMutation(
  page: Page,
  path: string,
  method: string,
  error?: string,
) {
  const requests: Request[] = [];
  const isTarget = (request: Request) =>
    new URL(request.url()).pathname === path && request.method() === method;
  page.on("request", (request) => {
    if (isTarget(request)) requests.push(request);
  });
  let release!: () => void;
  const released = new Promise<void>((resolve) => { release = resolve; });
  const pattern = (url: URL) => url.pathname === path;
  let held = false;
  const handler = async (route: Route) => {
    if (!isTarget(route.request()) || held) {
      await route.continue();
      return;
    }
    held = true;
    await released;
    if (error) {
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error }),
      });
    } else {
      await route.continue();
    }
    // Removing an active Playwright handler resolves its route automatically.
    // Finish this response first, then remove the fault before the next write.
    await page.unroute(pattern, handler);
  };
  await page.route(pattern, handler);
  return { requests, release };
}

async function openIssue(page: Page, fixture: Fixture, title: string, priority = "none") {
  const issue = await fixture.api.createIssue(title, {
    status: "todo",
    priority,
  }) as Issue;
  await page.goto(`/${fixture.workspace.slug}/issues/${issue.id}`, {
    waitUntil: "domcontentloaded",
  });
  await expect(page.getByRole("button", { name: title, exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();
  return issue;
}

async function chooseProperty(page: Page, current: string, next: string) {
  await page.getByRole("button", { name: current, exact: true }).click();
  const option = page.locator("button[data-picker-item]").filter({
    hasText: new RegExp(`^${next}$`),
  });
  await option.click();
  await expect(option).toBeHidden();
}

async function openDelete(page: Page) {
  await page.locator(
    'main button[data-slot="dropdown-menu-trigger"]:not([aria-label]):has(svg.lucide-ellipsis)',
  ).click();
  await page.getByRole("menuitem", { name: "Delete issue", exact: true }).click();
  const dialog = page.getByRole("alertdialog", { name: "Delete issue", exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

function commentEditor(page: Page) {
  return page.locator("div.rounded-lg.bg-card").filter({
    has: page.getByRole("button", { name: "Send", exact: true }),
  }).locator('.ProseMirror[contenteditable="true"]');
}

test("failed status update rolls back the optimistic value before a successful retry", async ({ page, fixture }) => {
  const issue = await openIssue(page, fixture, "Status rollback");
  const path = `/api/issues/${issue.id}`;
  const fault = await holdFirstMutation(page, path, "PUT", "Status update unavailable");
  try {
    await chooseProperty(page, "Todo", "In Progress");
    await expect.poll(() => fault.requests.length).toBe(1);
    expect(fault.requests[0]!.postDataJSON()).toMatchObject({ status: "in_progress" });
    await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
    expect(await fixture.api.requestJSON<Issue>(path)).toMatchObject({ status: "todo" });
    fault.release();
    await expect(page.getByText("Status update unavailable", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();
    expect(await fixture.api.requestJSON<Issue>(path)).toMatchObject({ status: "todo" });

    const retried = page.waitForResponse((response) => matches(response, path, "PUT"));
    await chooseProperty(page, "Todo", "In Progress");
    expect((await retried).ok()).toBe(true);
    expect(await fixture.api.requestJSON<Issue>(path)).toMatchObject({ status: "in_progress" });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
    expect(fault.requests).toHaveLength(2);
  } finally { fault.release(); }
});

test("failed priority update restores the existing priority and retry persists", async ({ page, fixture }) => {
  const issue = await openIssue(page, fixture, "Priority rollback", "low");
  const path = `/api/issues/${issue.id}`;
  const fault = await holdFirstMutation(page, path, "PUT", "Priority update unavailable");
  try {
    await chooseProperty(page, "Low", "High");
    await expect.poll(() => fault.requests.length).toBe(1);
    expect(fault.requests[0]!.postDataJSON()).toMatchObject({ priority: "high" });
    await expect(page.getByRole("button", { name: "High", exact: true })).toBeVisible();
    expect(await fixture.api.requestJSON<Issue>(path)).toMatchObject({ priority: "low" });
    fault.release();
    await expect(page.getByText("Priority update unavailable", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Low", exact: true })).toBeVisible();
    expect(await fixture.api.requestJSON<Issue>(path)).toMatchObject({ priority: "low" });

    const retried = page.waitForResponse((response) => matches(response, path, "PUT"));
    await chooseProperty(page, "Low", "High");
    expect((await retried).ok()).toBe(true);
    expect(await fixture.api.requestJSON<Issue>(path)).toMatchObject({ priority: "high" });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "High", exact: true })).toBeVisible();
    expect(fault.requests).toHaveLength(2);
  } finally { fault.release(); }
});

test("failed deletion keeps the confirmation and issue until a successful retry", async ({ page, fixture }) => {
  const issue = await openIssue(page, fixture, "Delete retry");
  const path = `/api/issues/${issue.id}`;
  const fault = await holdFirstMutation(page, path, "DELETE", "Delete temporarily unavailable");
  fault.release();
  const dialog = await openDelete(page);
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page.getByText("Delete temporarily unavailable", { exact: true })).toBeVisible();
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Delete", exact: true })).toBeEnabled();
  await expect(page).toHaveURL(new RegExp(`/issues/${issue.identifier}$`));
  expect(await fixture.api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id });

  const deleted = page.waitForResponse((response) => matches(response, path, "DELETE"));
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  expect((await deleted).ok()).toBe(true);
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(new RegExp(`/${fixture.workspace.slug}/issues$`));
  await expect(fixture.api.requestJSON(path)).rejects.toThrow("404");
  expect(fault.requests).toHaveLength(2);
});

test("cancelling after a failed deletion leaves the issue readable after reload", async ({ page, fixture }) => {
  const issue = await openIssue(page, fixture, "Cancel failed deletion");
  const path = `/api/issues/${issue.id}`;
  const fault = await holdFirstMutation(page, path, "DELETE", "Delete cancellation checkpoint");
  fault.release();
  const dialog = await openDelete(page);
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page.getByText("Delete cancellation checkpoint", { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(dialog).toBeHidden();
  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
  expect(await fixture.api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id });
  expect(fault.requests).toHaveLength(1);
});

test("failed manual creation retains title and description and retry creates exactly one issue", async ({ page, fixture }) => {
  await page.goto(`/${fixture.workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "New Issue", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New Issue", exact: true });
  const title = "Retain manual issue draft";
  const description = "The complete description must survive the rejected request.";
  const titleInput = dialog.getByRole("textbox", { name: "Issue title", exact: true });
  const editor = dialog.locator('.ProseMirror.rich-text-editor[contenteditable="true"]');
  await titleInput.fill(title);
  await editor.fill(description);
  const fault = await holdFirstMutation(page, "/api/issues", "POST", "Create temporarily unavailable");
  fault.release();
  await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
  await expect(page.getByText("Create temporarily unavailable", { exact: true })).toBeVisible();
  await expect(dialog).toBeVisible();
  await expect(titleInput).toHaveText(title);
  await expect(editor).toHaveText(description);
  expect(await fixture.api.requestJSON<{ issues: Issue[]; total: number }>("/api/issues"))
    .toMatchObject({ issues: [], total: 0 });

  const created = page.waitForResponse((response) => matches(response, "/api/issues", "POST"));
  await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
  const response = await created;
  expect(response.status()).toBe(201);
  const issue = await response.json() as Issue;
  expect(issue).toMatchObject({ title, description });
  await expect(dialog).toBeHidden();
  const persisted = await fixture.api.requestJSON<{ issues: Issue[]; total: number }>("/api/issues");
  expect(persisted.total).toBe(1);
  expect(persisted.issues).toEqual([expect.objectContaining({ id: issue.id, title, description })]);
  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(page.getByText(title, { exact: true })).toBeVisible();
  expect(fault.requests).toHaveLength(2);
});

test("failed comment send preserves the draft across reload and retry posts exactly once", async ({ page, fixture }) => {
  const issue = await openIssue(page, fixture, "Comment draft recovery");
  const path = `/api/issues/${issue.id}/comments`;
  const text = "Keep my draft until the server accepts it.";
  await page.getByTestId("comment-composer-shell").click();
  const editor = commentEditor(page);
  await editor.fill(text);
  const fault = await holdFirstMutation(page, path, "POST", "Comment temporarily unavailable");
  fault.release();
  await editor.press("ControlOrMeta+Enter");
  await expect(page.getByText("Comment temporarily unavailable", { exact: true })).toBeVisible();
  await expect(editor).toHaveText(text);
  expect(await fixture.api.requestJSON<Comment[]>(path)).toEqual([]);
  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(editor).toHaveText(text);

  const commented = page.waitForResponse((response) => matches(response, path, "POST"));
  await editor.press("ControlOrMeta+Enter");
  const response = await commented;
  expect(response.ok()).toBe(true);
  const comment = await response.json() as Comment;
  await expect(page.locator(`#comment-body-${comment.id}`)).toHaveText(text);
  await expect(editor).toHaveText("");
  expect(await fixture.api.requestJSON<Comment[]>(path)).toEqual([
    expect.objectContaining({ id: comment.id, content: text }),
  ]);
  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(page.locator(`#comment-body-${comment.id}`)).toHaveText(text);
  await expect(page.getByTestId("comment-composer-shell")).toBeVisible();
  expect(fault.requests).toHaveLength(2);
});

test("repeated send shortcuts while a comment is pending create only one comment", async ({ page, fixture }) => {
  const issue = await openIssue(page, fixture, "Comment single flight");
  const path = `/api/issues/${issue.id}/comments`;
  const text = "Only one comment should reach the server.";
  await page.getByTestId("comment-composer-shell").click();
  const editor = commentEditor(page);
  await editor.fill(text);
  const held = await holdFirstMutation(page, path, "POST");
  try {
    await editor.press("ControlOrMeta+Enter");
    await expect.poll(() => held.requests.length).toBe(1);
    await expect(page.getByTestId("content").getByRole("button", { name: "Send", exact: true })).toBeDisabled();
    await expect(editor).toHaveText(text);
    await editor.press("ControlOrMeta+Enter");
    await editor.press("ControlOrMeta+Enter");
    expect(await fixture.api.requestJSON<Comment[]>(path)).toEqual([]);
    const accepted = page.waitForResponse((response) => matches(response, path, "POST"));
    held.release();
    expect((await accepted).ok()).toBe(true);
    await expect(editor).toHaveText("");
    const comments = await fixture.api.requestJSON<Comment[]>(path);
    expect(comments).toEqual([expect.objectContaining({ content: text })]);
    await expect(page.locator(`#comment-body-${comments[0]!.id}`)).toHaveText(text);
    expect(held.requests).toHaveLength(1);
  } finally { held.release(); }
});
