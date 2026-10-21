import { randomUUID } from "node:crypto";
import { expect, type Locator, type Page, type Request } from "@playwright/test";
import type { CollaborationCommentReceipt, CollaborationIssue } from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";

interface OrdinaryIssue {
  id: string;
  identifier: string;
  title: string;
  parent_issue_id: string | null;
  collaboration?: unknown;
}

export function selectionBoundaryEnvironment() {
  if (process.env.E2E_PASSWORD_AUTH !== "1") throw new Error("AC87 requires the task-owned password acceptance environment");
  const apiUrl = process.env.COLLABORATION_E2E_API_URL;
  const rendererUrl = process.env.COLLABORATION_ELECTRON_RENDERER_URL;
  for (const value of [apiUrl, rendererUrl, process.env.NEXT_PUBLIC_API_URL, process.env.DATABASE_URL]) {
    if (!value || !["localhost", "127.0.0.1"].includes(new URL(value).hostname)) throw new Error("AC87 requires explicit loopback test services");
  }
  expect(new URL(apiUrl!).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL!).origin);
  expect(["", "multica", "postgres", "template0", "template1"]).not.toContain(new URL(process.env.DATABASE_URL!).pathname.slice(1));
  return { apiUrl: apiUrl!, rendererUrl: rendererUrl! };
}

/** Public APIs create one synthetic account, one collaboration and an ordinary
 * issue attached only after creation. No runtime, provider or fixture DB writer. */
export async function seedSelectionBoundary() {
  const environment = selectionBoundaryEnvironment();
  const configuration = await fetch(`${environment.apiUrl}/api/config`);
  expect(configuration.status).toBe(200);
  expect(await configuration.json()).toMatchObject({ auth_mode: "password" });
  const client = new TestApiClient();
  const suffix = randomUUID().replaceAll("-", "").slice(0, 14);
  const username = `s6select${suffix}`;
  const password = `Selection-${randomUUID()}!`;
  const actor = await client.registerPassword(username, password, "AC87 selection owner");
  const onboarded = await client.requestJSON<{ onboarded_at: string | null }>("/api/me/onboarding/complete", { method: "POST", body: {} });
  expect(onboarded.onboarded_at).not.toBeNull();
  const workspace = await client.ensureWorkspace("AC87 selection source", `selection-${suffix}`);
  expect(await client.requestJSON("/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const body = "AC87 shared body selection";
  const comment = "AC87 shared comment selection";
  const issue = await client.requestJSON<CollaborationIssue>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: workspace.id, handler_user_id: actor.id,
      title: `AC87 shared selection ${suffix}`, description: body, attachment_ids: [],
    },
  });
  const receipt = await client.requestJSON<CollaborationCommentReceipt>(`/api/collaboration/issues/${issue.id}/comments`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      content: comment, expected_lifecycle_epoch: issue.lifecycle_epoch,
      expected_participant_epoch: issue.write_context!.participant_epoch,
      expected_account_epoch: issue.write_context!.account_epoch, target_versions: [],
    },
  });
  expect(receipt).toMatchObject({ code: "comment_created", status: 201 });
  expect(receipt.targets).toEqual([]);
  const ordinaryBody = "AC87 ordinary body selection";
  const ordinaryComment = "AC87 ordinary comment selection";
  const ordinary = await client.requestJSON<OrdinaryIssue>("/api/issues", {
    method: "POST", body: { title: `AC87 ordinary source ${suffix}`, description: ordinaryBody, status: "todo" },
  });
  expect(ordinary.parent_issue_id).toBeNull();
  const attached = await client.requestJSON<OrdinaryIssue>(`/api/issues/${ordinary.id}`, {
    method: "PUT", body: { parent_issue_id: issue.id },
  });
  expect(attached.parent_issue_id).toBe(issue.id);
  expect(attached.collaboration ?? null).toBeNull();
  const ordinaryAnchor = await client.requestJSON<{ id: string }>(`/api/issues/${ordinary.id}/comments`, {
    method: "POST", body: { content: ordinaryComment },
  });
  return { ...environment, client, actorId: actor.id, workspace, issue, receipt, body, comment,
    ordinary: attached, ordinaryBody, ordinaryComment, ordinaryCommentId: ordinaryAnchor.id,
    accountPath: `/account?view=participating&issue=${issue.id}`,
    ordinaryPath: `/${workspace.slug}/issues/${ordinary.id}`,
    async authenticateWeb(page: Page) {
      const response = await page.request.post(`${environment.apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200);
      expect((await page.context().cookies(environment.apiUrl)).some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
      await page.addInitScript(() => {
        localStorage.setItem("multica-locale", "en");
        localStorage.setItem("multica:chat:isOpen", "false");
      });
    },
  };
}
export type SelectionBoundaryFixture = Awaited<ReturnType<typeof seedSelectionBoundary>>;

/** Native pointer selection drives the actual DOM/editor selection machinery. */
async function selectText(page: Page, locator: Locator, text: string) {
  await expect(locator).toBeVisible();
  await locator.scrollIntoViewIfNeeded();
  const bounds = await locator.evaluate((element, marker) => {
    const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      const start = node.textContent?.indexOf(marker) ?? -1;
      if (start < 0) continue;
      const range = document.createRange();
      range.setStart(node, start);
      range.setEnd(node, start + marker.length);
      const rects = [...range.getClientRects()];
      if (rects.length !== 1) throw new Error("The short selection fixture must fit on one visible line");
      const rect = rects[0]!;
      return { left: rect.left, right: rect.right, y: rect.top + rect.height / 2 };
    }
    throw new Error("Selection marker is missing from its actual rendered text");
  }, text);
  await page.mouse.move(bounds.left + 0.5, bounds.y);
  await page.mouse.down();
  await page.mouse.move(bounds.right - 0.5, bounds.y, { steps: 16 });
  await page.mouse.up();
  await expect.poll(() => page.evaluate(() => getSelection()?.toString() ?? "")).toContain(text);
  return { marker: text, selected: await page.evaluate(() => getSelection()?.toString() ?? ""), bounds };
}

export function observeSelectionRequests(page: Page) {
  const requests: { method: string; path: string }[] = [];
  const listener = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (path.includes("sub-issue-preview") || path.endsWith("/sub-issues") || path.includes("source-context")) {
      requests.push({ method: request.method(), path });
    }
  };
  page.on("request", listener);
  return { requests, dispose: () => page.off("request", listener) };
}

export async function inspectSharedSelection(page: Page, fixture: SelectionBoundaryFixture) {
  await expect(page.getByRole("heading", { name: fixture.issue.title, exact: true })).toBeVisible();
  const body = await selectText(page, page.locator('#description [data-rich-content]').first(), fixture.body);
  await expect(page.getByRole("button", { name: "Create sub-issue from selection", exact: true })).toHaveCount(0);
  await expect(page.getByRole("menuitem", { name: "Create sub-issue from here", exact: true })).toHaveCount(0);
  await expect(page.getByText("Create sub-issue from here", { exact: true })).toHaveCount(0);
  await page.keyboard.press("Escape");
  const comment = await selectText(page, page.locator(`[id="comment-${fixture.receipt.comment_id}"] [data-rich-content]`).first(), fixture.comment);
  await expect(page.getByRole("button", { name: "Create sub-issue from selection", exact: true })).toHaveCount(0);
  await expect(page.getByText("Create sub-issue from here", { exact: true })).toHaveCount(0);
  await expect(page.locator('.bubble-menu:visible')).toHaveCount(0);
  return { body, comment, create_subtask_entries: 0 };
}

export async function inspectOrdinarySelection(page: Page, fixture: SelectionBoundaryFixture) {
  await expect(page.getByRole("button", { name: "Issue actions", exact: true })).toBeVisible();
  await expect(page.getByText(fixture.ordinary.title, { exact: true }).first()).toBeVisible();
  const body = await selectText(page, page.locator('.ProseMirror').filter({ hasText: fixture.ordinaryBody }).first(), fixture.ordinaryBody);
  await expect(page.getByRole("button", { name: "Create sub-issue from selection", exact: true })).toBeVisible();
  await expect(page.locator('.bubble-menu:visible')).toBeVisible();
  await page.keyboard.press("Escape");
  const commentBody = page.locator(`[id="comment-body-${fixture.ordinaryCommentId}"]`);
  const comment = await selectText(page, commentBody, fixture.ordinaryComment);
  await page.locator(`[id="comment-${fixture.ordinaryCommentId}"]`).getByRole("button", { name: "Comment actions", exact: true }).click();
  const action = page.getByRole("menuitem", { name: "Create sub-issue from here", exact: true });
  await expect(action).toBeVisible();
  const path = `/api/comments/${fixture.ordinaryCommentId}/sub-issue-preview`;
  const responsePending = page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === "GET");
  await action.click();
  const response = await responsePending;
  expect(response.status()).toBe(200);
  const preview = await response.json() as { source_issue: { id: string }; capture_token: string };
  expect(preview.source_issue.id).toBe(fixture.ordinary.id);
  expect(preview.capture_token).toBeTruthy();
  return { body, comment, create_subtask_entries: 2, preview: { source_issue_id: preview.source_issue.id, status: response.status() } };
}
