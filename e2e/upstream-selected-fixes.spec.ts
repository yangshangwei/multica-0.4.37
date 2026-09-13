import { randomUUID } from "node:crypto";
import { test, expect, type Locator, type Page, type Response } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

type Workspace = { id: string; slug: string };
type Issue = { id: string; title: string; description?: string };
type Comment = { id: string; content: string };
type InboxNotification = {
  id: string;
  type: string;
  issue_id: string | null;
  body: string | null;
  archived: boolean;
  details: { comment_id?: string } | null;
};
type Scenario = { api: TestApiClient; workspace: Workspace; suffix: string };

async function withDatabase<T>(run: (client: pg.Client) => Promise<T>): Promise<T> {
  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) throw new Error("E2E fixtures require an explicit DATABASE_URL");
  const client = new pg.Client(databaseUrl);
  await client.connect();
  try {
    return await run(client);
  } finally {
    await client.end();
  }
}

async function createScenario(label: string): Promise<Scenario> {
  const suffix = Date.now().toString(36) + "-" + randomUUID().slice(0, 8);
  const api = new TestApiClient();
  await api.login("e2e-upstream-" + suffix + "@multica.ai", "Upstream fixes owner");
  const workspace = await api.ensureWorkspace("Upstream " + label, "upstream-" + suffix);
  await api.markUserOnboarded();
  return { api, workspace, suffix };
}

async function addMember(scenario: Scenario): Promise<TestApiClient> {
  const member = new TestApiClient();
  await member.login(
    "e2e-upstream-member-" + scenario.suffix + "@multica.ai",
    "Upstream fixes member",
  );
  const user = await member.requestJSON<{ id: string }>("/api/me");
  // Membership and a registered runtime are setup preconditions. Comments,
  // notifications, issue edits and quick-create all use the real HTTP API.
  await withDatabase((client) => client.query(
    "INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')",
    [scenario.workspace.id, user.id],
  ));
  member.setWorkspaceId(scenario.workspace.id);
  member.setWorkspaceSlug(scenario.workspace.slug);
  await member.markUserOnboarded();
  return member;
}

async function authenticateBrowser(page: Page, api: TestApiClient, mode = "manual") {
  const token = api.getToken();
  if (!token) throw new Error("Fixture authentication did not return a token");
  await page.addInitScript(({ token: value, createMode }) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
    localStorage.setItem(
      "multica_create_mode",
      JSON.stringify({ state: { lastMode: createMode }, version: 0 }),
    );
  }, { token, createMode: mode });
}

function isApiResponse(response: Response, path: string, method = "GET"): boolean {
  const pathname = new URL(response.url()).pathname;
  const normalized = pathname.endsWith("/") ? pathname.slice(0, -1) : pathname;
  return normalized === path && response.request().method() === method;
}

function notificationRow(page: Page, title: string): Locator {
  // The editable issue title is also a div[role=button]. A notification row
  // carries its own archive/menu buttons; the title does not.
  return page.locator('div[role="button"]').filter({
    has: page.getByText(title, { exact: true }),
  }).filter({
    has: page.locator("button"),
  });
}

async function createCommentNotification(scenario: Scenario, content: string) {
  const issue: Issue = await scenario.api.createIssue("Inbox regression " + scenario.suffix);
  await scenario.api.requestJSON("/api/issues/" + issue.id + "/subscribe", { method: "POST" });
  const commenter = await addMember(scenario);
  const comment = await commenter.requestJSON<Comment>("/api/issues/" + issue.id + "/comments", {
    method: "POST",
    body: { content, type: "comment" },
  });
  const findNotification = async () => {
    const items = await scenario.api.requestJSON<InboxNotification[]>("/api/inbox");
    return items.find((item) => item.type === "new_comment" && item.details?.comment_id === comment.id);
  };
  await expect.poll(findNotification, { timeout: 15_000 }).toMatchObject({
    issue_id: issue.id,
    details: { comment_id: comment.id },
  });
  const notification = await findNotification();
  if (!notification) throw new Error("The real comment notification was not created");
  return { issue, comment, notification };
}

async function placeCaretAtEnd(paragraph: Locator) {
  await paragraph.click();
  // Set the native DOM caret only. The actual keyboard event below must pass
  // through the production ProseMirror keymaps; no editor commands are called.
  await paragraph.evaluate((node) => {
    const text = node.firstChild;
    if (!text || text.nodeType !== Node.TEXT_NODE) throw new Error("Expected a plain text list item");
    const range = document.createRange();
    range.setStart(text, text.textContent?.length ?? 0);
    range.collapse(true);
    const selection = window.getSelection();
    if (!selection) throw new Error("Browser selection is unavailable");
    selection.removeAllRanges();
    selection.addRange(range);
  });
}

test.use({ screenshot: "only-on-failure", trace: "retain-on-failure" });

test.describe("selectively merged upstream regressions", () => {
  test.setTimeout(120_000);

  test("keeps the full Chinese and emoji comment behind active and archived Inbox previews", async ({ page }) => {
    const scenario = await createScenario("comment preview");
    try {
      const tail = "完整评论结尾-保留原文-" + scenario.suffix;
      const fullComment = "中文评论😀完整内容。".repeat(60) + "\n\n" + tail;
      const { issue, comment, notification } = await createCommentNotification(scenario, fullComment);
      const preview = Array.from(fullComment).slice(0, 199).join("") + "…";
      expect(notification.body).toBe(preview);
      expect(Array.from(notification.body ?? "")).toHaveLength(200);

      await authenticateBrowser(page, scenario.api);
      await page.setViewportSize({ width: 1100, height: 900 });
      const listResponse = page.waitForResponse((response) => isApiResponse(response, "/api/inbox"));
      await page.goto("/" + scenario.workspace.slug + "/inbox", { waitUntil: "domcontentloaded" });
      const listed: InboxNotification[] = await (await listResponse).json();
      expect(listed.find((item) => item.id === notification.id)).toMatchObject({
        body: preview,
        details: { comment_id: comment.id },
      });
      const row = notificationRow(page, issue.title);
      await expect(row).toContainText(preview, { timeout: 30_000 });
      await expect(row).not.toContainText(tail);

      const readResponse = page.waitForResponse((response) =>
        isApiResponse(response, "/api/inbox/" + notification.id + "/read", "POST"));
      await row.click();
      const read: InboxNotification = await (await readResponse).json();
      expect(read).toMatchObject({
        body: fullComment,
        details: { comment_id: comment.id },
      });
      const commentBody = page.locator("#comment-body-" + comment.id);
      await expect(commentBody).toContainText(tail, { timeout: 30_000 });
      // Selection renders locally before Next finishes its URL transition.
      // Refresh only once the deep link has committed, so the browser reloads
      // the selected notification instead of the previous bare Inbox URL.
      await expect.poll(() => new URL(page.url()).searchParams.get("issue"), {
        timeout: 30_000,
      }).toBe(issue.id);
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(commentBody).toContainText(tail, { timeout: 30_000 });

      const archived = await scenario.api.requestJSON<InboxNotification>(
        "/api/inbox/" + notification.id + "/archive",
        { method: "POST" },
      );
      expect(archived).toMatchObject({
        archived: true,
        body: fullComment,
        details: { comment_id: comment.id },
      });
      const archiveResponse = page.waitForResponse((response) =>
        isApiResponse(response, "/api/inbox/archived"));
      await page.goto("/" + scenario.workspace.slug + "/inbox?view=archived", {
        waitUntil: "domcontentloaded",
      });
      const archiveList: InboxNotification[] = await (await archiveResponse).json();
      expect(archiveList.find((item) => item.id === notification.id)).toMatchObject({
        body: preview,
        details: { comment_id: comment.id },
      });
      await expect(row).toContainText(preview, { timeout: 30_000 });
      await row.click();
      await expect(commentBody).toContainText(tail, { timeout: 30_000 });
    } finally {
      await scenario.api.deleteFeatureWorkspace(scenario.workspace.id);
    }
  });

  test("keeps one usable Inbox sidebar toggle in split view and a way back at compact widths", async ({ page }, info) => {
    const scenario = await createScenario("Inbox navigation");
    try {
      const { issue, comment } = await createCommentNotification(scenario, "Navigation remains available.");
      await authenticateBrowser(page, scenario.api);
      // Between lg and xl, both panes and the header nav triggers are visible.
      // A desktop-only 1440px check would miss the original duplicate.
      await page.setViewportSize({ width: 1100, height: 900 });
      await page.goto("/" + scenario.workspace.slug + "/inbox", { waitUntil: "domcontentloaded" });
      const row = notificationRow(page, issue.title);
      await expect(row).toBeVisible({ timeout: 30_000 });
      await row.click();
      await expect(page.locator("#comment-body-" + comment.id)).toContainText("Navigation remains available.");
      await expect(row).toBeVisible();
      const triggers = page.locator('[data-sidebar="trigger"]:visible');
      await expect(triggers).toHaveCount(1);
      const sidebar = page.locator('[data-slot="sidebar"][data-side="left"][data-state]');
      const before = await sidebar.getAttribute("data-state");
      expect(["expanded", "collapsed"]).toContain(before);
      if (before !== "expanded" && before !== "collapsed") throw new Error("Sidebar state is unavailable");
      await triggers.click();
      await expect(sidebar).toHaveAttribute("data-state", before === "collapsed" ? "expanded" : "collapsed");
      await triggers.click();
      await expect(sidebar).toHaveAttribute("data-state", before);
      await page.screenshot({ path: info.outputPath("inbox-split-1100.png"), animations: "disabled" });

      for (const width of [851, 390]) {
        await page.setViewportSize({ width, height: 900 });
        if (width === 390) await row.click();
        await expect(row).toBeHidden();
        const back = page.getByRole("button", { name: "Inbox", exact: true });
        await expect(back).toBeVisible();
        await back.click();
        await expect(row).toBeVisible();
        await expect(triggers).toHaveCount(1);
        await triggers.click();
        await expect(page.getByRole("dialog", { name: "Sidebar", exact: true })).toBeVisible();
        await page.keyboard.press("Escape");
        await expect(page.getByRole("dialog", { name: "Sidebar", exact: true })).toBeHidden();
      }
      await page.screenshot({ path: info.outputPath("inbox-compact-390.png"), animations: "disabled" });
    } finally {
      await scenario.api.deleteFeatureWorkspace(scenario.workspace.id);
    }
  });

  test("edits a bullet inside a checkbox and persists the selected list level after reopening", async ({ page }, info) => {
    const scenario = await createScenario("nested list");
    try {
      const outer = "Outer task " + scenario.suffix;
      const first = "First child";
      const second = "Second child";
      const third = "Added child";
      const issue: Issue = await scenario.api.createIssue("Nested list regression " + scenario.suffix, {
        description: ["- [ ] " + outer, "   - " + first, "   - " + second].join("\n"),
      });
      await authenticateBrowser(page, scenario.api);
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto("/" + scenario.workspace.slug + "/issues/" + issue.id, {
        waitUntil: "domcontentloaded",
      });
      const editor = page.locator('.ProseMirror[contenteditable="true"]').filter({ hasText: outer });
      await expect(editor).toBeVisible({ timeout: 30_000 });
      const secondParagraph = editor.locator("p").filter({ hasText: second });
      await expect(secondParagraph.locator("xpath=ancestor::li")).toHaveCount(2);
      await placeCaretAtEnd(secondParagraph);
      await page.keyboard.press("Tab");
      await expect(secondParagraph.locator("xpath=ancestor::li")).toHaveCount(3);
      await page.keyboard.press("Shift+Tab");
      await expect(secondParagraph.locator("xpath=ancestor::li")).toHaveCount(2);
      await page.keyboard.press("Enter");
      await page.keyboard.insertText(third);
      await expect(editor.getByRole("checkbox")).toHaveCount(1);
      await expect(editor.getByRole("checkbox")).not.toBeChecked();
      await expect(editor.locator("p").filter({ hasText: third }).locator("xpath=ancestor::li")).toHaveCount(2);

      await expect.poll(async () => {
        const saved = await scenario.api.requestJSON<Issue>("/api/issues/" + issue.id);
        return saved.description;
      }, { timeout: 15_000 }).toBe([
        "- [ ] " + outer, "   - " + first, "   - " + second, "   - " + third,
      ].join("\n"));
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(editor).toBeVisible({ timeout: 30_000 });
      await expect(editor.getByRole("checkbox")).toHaveCount(1);
      await expect(editor.getByRole("checkbox")).not.toBeChecked();
      for (const text of [first, second, third]) {
        await expect(editor.locator("p").filter({ hasText: text }).locator("xpath=ancestor::li")).toHaveCount(2);
      }
      await page.screenshot({ path: info.outputPath("nested-list-persisted.png"), animations: "disabled" });
    } finally {
      await scenario.api.deleteFeatureWorkspace(scenario.workspace.id);
    }
  });

  for (const version of ["0.4.40", "0.0.1"]) {
    test("defers hidden private runtime version " + version + " to the server's quick-create gate", async ({ page }) => {
      const scenario = await createScenario("hidden runtime");
      try {
        const runtime = await scenario.api.seedProjectRuntime();
        await withDatabase((client) => client.query(
          "UPDATE agent_runtime SET metadata = metadata || jsonb_build_object('cli_version', $2::text) WHERE id = $1",
          [runtime.id, version],
        ));
        const agent = await scenario.api.requestJSON<{ id: string; name: string }>("/api/agents", {
          method: "POST",
          body: {
            name: "Shared fixture agent " + scenario.suffix,
            runtime_id: runtime.id,
            visibility: "workspace",
          },
        });
        const member = await addMember(scenario);
        const runtimes = await member.requestJSON<Array<{ id: string }>>("/api/runtimes");
        expect(runtimes.some((item) => item.id === runtime.id)).toBe(false);

        await authenticateBrowser(page, member, "agent");
        await page.goto("/" + scenario.workspace.slug + "/issues", { waitUntil: "domcontentloaded" });
        await page.getByRole("button", { name: "New Issue", exact: true }).click();
        const dialog = page.getByRole("dialog", { name: "Quick create issue", exact: true });
        await expect(dialog).toBeVisible({ timeout: 30_000 });
        await dialog.getByRole("button", { name: /Created by/ }).click();
        await page.locator("button[data-picker-item]").filter({ hasText: agent.name }).click();
        await expect(dialog.getByText(/doesn't report a CLI version/)).toHaveCount(0);
        await dialog.locator('.ProseMirror[contenteditable="true"]').fill("Create a fixture issue without running a real CLI.");
        const create = dialog.getByRole("button", { name: "Create", exact: true });
        await expect(create).toBeEnabled();
        const submitted = page.waitForResponse((response) =>
          isApiResponse(response, "/api/issues/quick-create", "POST"));
        await create.click();
        const response = await submitted;
        if (version === "0.4.40") {
          expect(response.status()).toBe(202);
          const accepted: { task_id: string } = await response.json();
          expect(accepted.task_id).toBeTruthy();
          await expect(dialog).toBeHidden();
          const tasks = await withDatabase((client) => client.query<{ id: string }>(
            "SELECT id FROM agent_task_queue WHERE agent_id = $1",
            [agent.id],
          ));
          expect(tasks.rows.map((task) => task.id)).toEqual([accepted.task_id]);
        } else {
          expect(response.status()).toBe(422);
          expect(await response.json()).toMatchObject({
            code: "daemon_version_unsupported",
            current_version: version,
            runtime_id: runtime.id,
          });
          await expect(dialog.getByText(/below the required/)).toBeVisible();
          const tasks = await withDatabase((client) => client.query<{ count: string }>(
            "SELECT count(*) FROM agent_task_queue WHERE agent_id = $1",
            [agent.id],
          ));
          expect(Number(tasks.rows[0]?.count)).toBe(0);
        }
      } finally {
        await scenario.api.deleteFeatureWorkspace(scenario.workspace.id);
      }
    });
  }
});
