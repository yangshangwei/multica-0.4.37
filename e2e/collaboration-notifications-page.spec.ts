import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type Page } from "@playwright/test";
import { CollaborationNotificationReceiptSchema, CollaborationNotificationsSchema } from "../packages/core/collaboration/content-schema";
import { authenticateCollaborationEditor } from "./fixtures/collaboration-editing";
import { nativeScreenshot, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import {
  notificationList, notificationPage, notificationPath, notificationRow, notificationViews,
  openCollaborationNotifications, seedCollaborationNotifications,
} from "./fixtures/collaboration-notifications";
import { openSidebarNavigation } from "./fixtures/sidebar-navigation";

test.use({ actionTimeout: 15_000 });

async function painted(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done())));
  });
}

for (const platform of ["Web", "Desktop"] as const) {
  test(`${platform} collaboration notifications: complete filters, exact targets, processing and restored list/detail`, async ({ page: webPage }, info) => {
    test.setTimeout(240_000);
    test.skip(!process.env.COLLABORATION_E2E_API_URL || !process.env.DATABASE_URL || !process.env.TASK_DATABASE_NAME
      || !process.env.NEXT_PUBLIC_API_URL || platform === "Desktop" && !process.env.COLLABORATION_ELECTRON_RENDERER_URL,
    "Requires the verified task API database, production Web and built native renderer/preload");
    for (const value of [process.env.COLLABORATION_E2E_API_URL!, process.env.NEXT_PUBLIC_API_URL!]) {
      expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
    }
    const fixture = await seedCollaborationNotifications();
    const profile = platform === "Desktop" ? mkdtempSync(join(tmpdir(), "multica-notifications-page-")) : null;
    let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
    let page = webPage;
    const errors: string[] = [];
    const writes: { path: string; method: string }[] = [];
    const accountRequests: { path: string; workspaceHeader: boolean }[] = [];
    const pagination: Record<string, unknown>[] = [];
    const restoration: Record<string, unknown>[] = [];
    const captures: Record<string, unknown>[] = [];
    const currentPath = () => native ? page.evaluate(() => {
      const state = JSON.parse(localStorage.getItem("multica_tabs") ?? "null")?.state;
      return state?.activeScope === "account" ? state.accountGroup?.tabs[0]?.url : null;
    }) : Promise.resolve(new URL(page.url()).pathname + new URL(page.url()).search + new URL(page.url()).hash);
    const expectPath = async (path: string) => expect.poll(currentPath).toBe(path);
    const resize = async (compact: boolean) => {
      const width = compact ? 390 : 1440;
      if (native) {
        const window = await native.app.browserWindow(page);
        await window.evaluate((target, value) => {
          target.webContents.setZoomFactor(value.compact ? 2 : 1);
          target.setContentSize(value.compact ? 780 : 1440, 960);
        }, { compact });
        await window.dispose();
      } else await page.setViewportSize({ width, height: compact ? 844 : 960 });
      await expect.poll(() => page.evaluate(() => innerWidth)).toBe(width);
      await painted(page);
    };
    const capture = async (name: string) => {
      await expect.poll(() => page.evaluate(() => document.getAnimations().filter((animation) =>
        (animation.playState === "running" || animation.pending) && animation.effect?.getComputedTiming().endTime !== Infinity).length)).toBe(0);
      await painted(page);
      const geometry = await page.evaluate(() => {
        const main = document.querySelector("main")!.getBoundingClientRect();
        const list = document.querySelector<HTMLElement>('[data-tab-scroll-root="account-notifications-list"]')!;
        const detail = document.querySelector<HTMLElement>('[data-tab-scroll-root="account-detail"]')!;
        return { viewport: { width: innerWidth, height: innerHeight, devicePixelRatio },
          main: { left: main.left, right: main.right, top: main.top, bottom: main.bottom },
          list: { top: list.scrollTop, scrollWidth: list.scrollWidth, clientWidth: list.clientWidth, visible: getComputedStyle(list).visibility },
          detail: { top: detail.scrollTop, scrollWidth: detail.scrollWidth, clientWidth: detail.clientWidth, visible: getComputedStyle(detail).visibility },
          documentWidth: document.documentElement.scrollWidth };
      });
      expect(geometry.documentWidth).toBeLessThanOrEqual(geometry.viewport.width + 1);
      expect(geometry.main.left).toBeGreaterThanOrEqual(0);
      expect(geometry.main.right).toBeLessThanOrEqual(geometry.viewport.width + 1);
      expect(geometry.list.scrollWidth).toBeLessThanOrEqual(geometry.list.clientWidth + 1);
      expect(geometry.detail.scrollWidth).toBeLessThanOrEqual(geometry.detail.clientWidth + 1);
      const file = `${platform.toLowerCase()}-${name}.png`;
      if (native) await nativeScreenshot(native.app, page, info.outputPath(file));
      else await page.screenshot({ path: info.outputPath(file), fullPage: false });
      captures.push({ file, ...geometry });
    };
    const count = async (value: number) => {
      await expect(page.locator("main > header").getByText(`${value} unread`, { exact: true })).toBeVisible();
      const sidebar = await openSidebarNavigation(page);
      // NumberFlow renders all roller digits; its accessible label carries the actual count.
      await expect(sidebar.getByRole("link", { name: "Collaboration notifications", exact: true }).getByLabel(String(value), { exact: true })).toBeVisible();
    };
    const history = async (direction: "back" | "forward") => {
      if (native) await page.locator('[data-slot="window-toolbar"]').getByRole("button", { name: direction === "back" ? "Go back" : "Go forward", exact: true }).click();
      else if (direction === "back") await page.goBack();
      else await page.goForward();
    };
    try {
      if (profile) {
        native = await openDesktop(profile, { token: fixture.records.viewer.getToken()! });
        page = native.page;
      } else await authenticateCollaborationEditor(page, fixture.records.viewer.getToken()!);
      page.on("pageerror", (error) => errors.push(error.message));
      page.on("request", (request) => {
        const path = new URL(request.url()).pathname;
        if (/^\/api\/(collaboration\/|me\/collaboration-notifications)/.test(path)) {
          const headers = request.headers();
          accountRequests.push({ path, workspaceHeader: Boolean(headers["x-workspace-id"] || headers["x-workspace-slug"]) });
        }
        if (path.startsWith("/api/me/collaboration-notifications/") && request.method() === "POST") {
          writes.push({ path, method: request.method() });
        }
      });
      const workspacePath = `/${fixture.workspace.slug}/issues/${fixture.ordinary.id}`;
      if (native) {
        await native.start();
        await expect(page.getByRole("navigation", { name: "Application navigation", exact: true })).toBeVisible();
        await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), workspacePath);
      } else await page.goto(workspacePath);
      await resize(false);
      await expect(page.getByRole("button", { name: fixture.ordinary.title, exact: true })).toBeVisible();
      const unreadBefore = fixture.rows.filter((row) => row.read_at === null && row.archived_at === null).length;
      await test.step("The sidebar opens the default Unread page with authoritative feedback", async () => {
        const sidebar = await openSidebarNavigation(page);
        const directory = sidebar.locator('[data-sidebar-scope="collaboration"]');
        await expect(directory.getByRole("button", { name: "Expand collaboration navigation", exact: true })).toHaveAttribute("aria-expanded", "false");
        await expect(directory.locator("output")).toHaveAttribute("aria-label", `Collaboration notifications ${unreadBefore} unread`);
        await openCollaborationNotifications(page, "unread");
        await expectPath(notificationPath("unread"));
        const link = sidebar.getByRole("link", { name: "Collaboration notifications", exact: true });
        await expect(link).toHaveAttribute("aria-current", "page");
        await expect(link).toHaveAttribute("data-active");
        await expect(page.locator('[data-slot="popover-content"]')).toHaveCount(0);
        await count(unreadBefore);
      });

      await test.step("Unread, All and Archived each load their complete second server page", async () => {
        for (const view of ["unread", "all", "archived"] as const) {
          await notificationViews(page).getByRole("link", { name: view === "unread" ? "Unread" : view === "all" ? "All" : "Archived", exact: true }).click();
          await expectPath(notificationPath(view));
          const expected = fixture.rows.filter((row) => view === "archived" ? row.archived_at !== null : row.archived_at === null && (view === "all" || row.read_at === null));
          expect(expected.length).toBeGreaterThan(50);
          const first = await notificationPage(fixture, view);
          expect(first.view).toBe(view);
          expect(first.items.map((item) => item.id)).toEqual(expected.slice(0, 50).map((item) => item.id));
          expect(first.unread_count).toBe(unreadBefore);
          expect(first.next_cursor).toBeTruthy();
          const entries = notificationList(page).locator("li[data-notification-id]");
          await expect(entries).toHaveCount(50);
          expect(await entries.evaluateAll((elements) => elements.map((element) => element.getAttribute("data-notification-id")))).toEqual(first.items.map((item) => item.id));
          const pending = page.waitForResponse((response) => {
            const url = new URL(response.url());
            return response.request().method() === "GET" && url.pathname === "/api/me/collaboration-notifications"
              && url.searchParams.get("view") === view && url.searchParams.get("cursor") === first.next_cursor;
          });
          await notificationList(page).getByRole("button", { name: "Load more", exact: true }).click();
          const response = await pending;
          expect(response.status()).toBe(200);
          const second = CollaborationNotificationsSchema.parse(await response.json());
          expect(second).toMatchObject({ view, unread_count: unreadBefore, next_cursor: null });
          expect(second.items.map((item) => item.id)).toEqual(expected.slice(50).map((item) => item.id));
          await expect(entries).toHaveCount(expected.length);
          expect(await entries.evaluateAll((elements) => elements.map((element) => element.getAttribute("data-notification-id")))).toEqual(expected.map((item) => item.id));
          await expect(notificationList(page).getByRole("button", { name: "Load more", exact: true })).toHaveCount(0);
          await count(unreadBefore);
          pagination.push({ view, first_ids: first.items.map((item) => item.id), requested_cursor: first.next_cursor,
            second_ids: second.items.map((item) => item.id), next_cursor: second.next_cursor, unread_count: second.unread_count });
        }
      });

      await notificationViews(page).getByRole("link", { name: "All", exact: true }).click();
      await expectPath(notificationPath("all"));
      await expect(notificationViews(page).getByRole("link", { name: "All", exact: true })).toHaveAttribute("aria-current", "page");
      const list = notificationList(page);
      const detail = page.locator('[data-tab-scroll-root="account-detail"]');
      const retainedList = await list.elementHandle();
      if (!retainedList) throw new Error("The actual notification scroll root is missing");
      const sameList = async () => expect(await list.evaluate((element, original) => element === original, retainedList)).toBe(true);
      await test.step("Distinct notification IDs preserve exact run, comment, body and issue targets", async () => {
        const runLink = notificationRow(page, fixture.runNotice.id).getByRole("link", { name: fixture.records.issue.title, exact: true });
        const runPath = notificationPath("all", fixture.runNotice.id, fixture.records.issue.id, `#run-${fixture.records.runId}`);
        await expect(runLink).toHaveAttribute("href", new RegExp(`${runPath.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`));
        await runLink.click();
        await expectPath(runPath);
        const run = page.locator(`[id="run-${fixture.records.runId}"]`);
        await expect(run).toBeFocused();
        await expect(run.getByText(fixture.records.input, { exact: true })).toBeVisible();
        await sameList();
        const commentLink = notificationRow(page, fixture.commentNotice.id).getByRole("link", { name: fixture.title, exact: true });
        await commentLink.click();
        await expectPath(notificationPath("all", fixture.commentNotice.id, fixture.issue.id, `#comment-${fixture.commentId}`));
        await expect(page.locator(`[id="comment-${fixture.commentId}"]`)).toBeFocused();
        await expect(page.locator(`[id="comment-${fixture.commentId}"]`)).toContainText(fixture.commentBody);
        await sameList();
        const unavailable = notificationRow(page, fixture.unavailableId);
        await expect(unavailable.getByRole("link")).toHaveAttribute("href", new RegExp(`${notificationPath("all", fixture.unavailableId).replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`));
        await unavailable.getByRole("link", { name: "Issue unavailable", exact: true }).click();
        await expectPath(notificationPath("all", fixture.unavailableId));
        await expect(detail.getByText("You were removed from this collaboration issue.", { exact: true })).toBeVisible();
        expect(accountRequests.some((request) => request.path.includes(fixture.unavailableIssueId))).toBe(false);

        // Both same-issue rows fit together; pointer activation must preserve
        // the existing list offset, as well as the original mounted root.
        await list.evaluate((element, ids) => {
          const rows = ids.map((id) => element.querySelector<HTMLElement>(`[data-notification-id="${id}"]`)!.getBoundingClientRect().top);
          element.scrollTop += Math.min(...rows) - element.getBoundingClientRect().top - 12;
        }, [fixture.bodyId, fixture.issueNoticeId]);
        await painted(page);
        const savedList = await list.evaluate((element) => element.scrollTop);
        expect(savedList).toBeGreaterThan(0);
        await notificationRow(page, fixture.bodyId).getByRole("link", { name: fixture.title, exact: true }).click();
        await expectPath(notificationPath("all", fixture.bodyId, fixture.issue.id, "#description"));
        await expect(page.locator("#description")).toBeFocused();
        await sameList();
        expect(await list.evaluate((element) => element.scrollTop)).toBe(savedList);
        await notificationRow(page, fixture.issueNoticeId).getByRole("link", { name: fixture.title, exact: true }).click();
        await expectPath(notificationPath("all", fixture.issueNoticeId, fixture.issue.id));
        await sameList();
        expect(await list.evaluate((element) => element.scrollTop)).toBe(savedList);
        await notificationRow(page, fixture.bodyId).getByRole("link", { name: fixture.title, exact: true }).click();
        await expect(page.locator("#description")).toBeFocused();
        await expect(notificationRow(page, fixture.bodyId)).toHaveAttribute("data-selected", "true");
        expect(writes).toEqual([]);
        expect((await notificationPage(fixture, "all")).items.find((item) => item.id === fixture.commentNotice.id)?.read_at).toBeNull();
        await detail.evaluate((element) => { element.scrollTop = 0; });
        await capture("notifications-wide");
      });

      await test.step("Same-issue Back/Forward restores distinct notification offsets without remounting the draft", async () => {
        const pathA = notificationPath("all", fixture.bodyId, fixture.issue.id, "#description");
        const pathB = notificationPath("all", fixture.issueNoticeId, fixture.issue.id);
        const bodyLink = notificationRow(page, fixture.bodyId).getByRole("link", { name: fixture.title, exact: true });
        const issueLink = notificationRow(page, fixture.issueNoticeId).getByRole("link", { name: fixture.title, exact: true });
        const description = detail.locator("#description");
        const composer = detail.getByRole("form", { name: "Comments", exact: true });
        const draft = composer.getByLabel("Add comment", { exact: true });
        const draftText = `Unsent same-issue history draft ${fixture.bodyId}`;
        await expectPath(pathA);
        await draft.fill(draftText);
        const retainedDetail = await detail.elementHandle();
        const retainedComposer = await composer.elementHandle();
        const retainedDraft = await draft.elementHandle();
        if (!retainedDetail || !retainedComposer || !retainedDraft) throw new Error("Same-issue history requires the mounted detail and draft subtree");
        const offsets = async () => ({ list: await list.evaluate((element) => element.scrollTop), detail: await detail.evaluate((element) => element.scrollTop) });
        const sameNodes = async () => {
          await sameList();
          expect(await detail.evaluate((element, original) => element === original, retainedDetail)).toBe(true);
          expect(await composer.evaluate((element, original) => element === original, retainedComposer)).toBe(true);
          expect(await draft.evaluate((element, original) => element === original, retainedDraft)).toBe(true);
          await expect(draft).toHaveValue(draftText);
        };
        // Keyboard activation avoids scrolling an offscreen link into view
        // before navigation and overwriting the departing route's memento.
        const activate = async (link: typeof bodyLink) => {
          await link.evaluate((element) => element.focus({ preventScroll: true }));
          await link.press("Enter");
        };
        try {
          await expect.poll(() => list.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(400);
          await expect.poll(() => detail.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(700);
          await list.evaluate((element) => { element.scrollTop = 0; });
          await detail.evaluate((element) => { element.scrollTop = 0; });
          await painted(page);
          const savedA = await offsets();
          expect(savedA).toEqual({ list: 0, detail: 0 });
          await activate(issueLink);
          await expectPath(pathB);
          await sameNodes();
          await list.evaluate((element) => { element.scrollTop = 360; });
          await detail.evaluate((element) => { element.scrollTop = 620; });
          await painted(page);
          const savedB = await offsets();
          expect(savedB).toEqual({ list: 360, detail: 620 });
          for (const [direction, path, id, saved] of [["back", pathA, fixture.bodyId, savedA], ["forward", pathB, fixture.issueNoticeId, savedB]] as const) {
            await history(direction);
            await expectPath(path);
            await expect(notificationRow(page, id)).toHaveAttribute("data-selected", "true");
            await expect.poll(offsets).toEqual(saved);
            await painted(page);
            const actual = await offsets();
            expect(actual).toEqual(saved);
            await sameNodes();
            if (direction === "back") await expect(description).not.toBeFocused();
            restoration.push({ via: `same-issue ${direction}`, path, notification_id: id, expected: saved, actual,
              list_node_retained: true, detail_node_retained: true, composer_node_retained: true, draft_node_retained: true, draft_preserved: true });
          }
          await activate(bodyLink);
          await expectPath(pathA);
          await expect(description).toBeFocused();
          expect(await list.evaluate((element) => element.scrollTop)).toBe(savedB.list);
          await detail.evaluate((element) => { element.scrollTop = 620; });
          await painted(page);
          await activate(bodyLink);
          await expect(description).toBeFocused();
          await expect.poll(() => description.evaluate((element) => {
            const root = element.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]')!;
            return Math.abs(element.getBoundingClientRect().top - root.getBoundingClientRect().top);
          })).toBeLessThanOrEqual(1);
          await painted(page);
          await sameNodes();
          expect(await list.evaluate((element) => element.scrollTop)).toBe(savedB.list);
          restoration.push({ via: "explicit repeated body link after same-issue history", path: pathA,
            actual: await offsets(), source_anchor_focused: true, list_node_retained: true, detail_node_retained: true, draft_preserved: true });
          await draft.fill("");
          await detail.evaluate((element) => { element.scrollTop = 0; });
          await painted(page);
        } finally {
          await Promise.all([retainedDetail.dispose(), retainedComposer.dispose(), retainedDraft.dispose()]);
        }
      });

      await test.step("Account Back/Forward and workspace scope resume restore selection and painted offsets", async () => {
        await expect.poll(() => detail.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(700);
        await detail.evaluate((element) => { element.scrollTop = 620; });
        await painted(page);
        const saved = { list: await list.evaluate((element) => element.scrollTop), detail: await detail.evaluate((element) => element.scrollTop) };
        const selectedPath = notificationPath("all", fixture.bodyId, fixture.issue.id, "#description");
        const restored = async (via: string) => {
          await expectPath(selectedPath);
          await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
          await expect(notificationViews(page).getByRole("link", { name: "All", exact: true })).toHaveAttribute("aria-current", "page");
          await expect(notificationRow(page, fixture.bodyId)).toHaveAttribute("data-selected", "true");
          await expect.poll(() => list.evaluate((element) => element.scrollTop)).toBe(saved.list);
          await expect.poll(() => detail.evaluate((element) => element.scrollTop)).toBe(saved.detail);
          await painted(page);
          expect(await list.evaluate((element) => element.scrollTop)).toBe(saved.list);
          expect(await detail.evaluate((element) => element.scrollTop)).toBe(saved.detail);
          restoration.push({ via, path: selectedPath, ...saved });
        };
        const sidebar = await openSidebarNavigation(page);
        await sidebar.getByRole("link", { name: "Participating", exact: true }).click();
        await expectPath("/account?view=participating");
        await history("back");
        await restored("account Back");
        await history("forward");
        await expectPath("/account?view=participating");
        await history("back");
        await restored("account Forward then Back");
        await sidebar.getByRole("button", { name: fixture.workspace.name, exact: true }).click();
        await expect(page.getByRole("button", { name: fixture.ordinary.title, exact: true })).toBeVisible();
        if (native) {
          const expand = sidebar.getByRole("button", { name: "Expand collaboration navigation", exact: true });
          if (await expand.count()) await expand.click();
          await sidebar.getByRole("link", { name: "Collaboration notifications", exact: true }).click();
        }
        await history("back");
        await restored("workspace to notification history return");
        await sidebar.getByRole("button", { name: "Collapse collaboration navigation", exact: true }).click();
        expect(await detail.evaluate((element) => element.scrollTop)).toBe(saved.detail);
      });

      await test.step("Compact detail keeps the hidden list mounted and returns through a real back link", async () => {
        const compactList = await list.elementHandle();
        if (!compactList) throw new Error("Compact navigation requires the mounted list scroll root");
        await resize(true);
        await expect(list).toHaveCount(1);
        await expect(list).toBeHidden();
        await expect(list).toHaveAttribute("aria-hidden", "true");
        await expect(list).toHaveAttribute("inert", "");
        await expect.poll(() => list.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(1000);
        await detail.evaluate((element) => { element.scrollTop = 0; });
        const back = page.getByRole("link", { name: "Back to notifications", exact: true });
        await expect(back).toBeVisible();
        await expect(back).toHaveAttribute("href", new RegExp("/account\\?view=notifications&filter=all$"));
        await capture("notifications-compact-detail");
        await back.click();
        await expectPath(notificationPath("all"));
        await expect(list).toBeVisible();
        await expect(detail).toBeHidden();
        const bodyLink = notificationRow(page, fixture.bodyId).getByRole("link", { name: fixture.title, exact: true });
        await bodyLink.scrollIntoViewIfNeeded();
        await capture("notifications-compact-list");
        await bodyLink.click();
        await expectPath(notificationPath("all", fixture.bodyId, fixture.issue.id, "#description"));
        await expect(list).toBeHidden();
        await expect(page.locator("#description")).toBeFocused();
        expect(await list.evaluate((element, original) => element === original, compactList)).toBe(true);
        await compactList.dispose();
        await resize(false);
      });

      await test.step("Confirmed read and archive update membership and the badge without closing authorized detail", async () => {
        await openCollaborationNotifications(page, "unread");
        const comment = notificationRow(page, fixture.commentNotice.id);
        await comment.getByRole("link", { name: fixture.title, exact: true }).click();
        const readPending = page.waitForResponse((response) => response.request().method() === "POST"
          && new URL(response.url()).pathname === `/api/me/collaboration-notifications/${fixture.commentNotice.id}/read`);
        await comment.getByRole("button", { name: "Mark read", exact: true }).click();
        const readResponse = await readPending;
        expect(readResponse.status()).toBe(200);
        const read = CollaborationNotificationReceiptSchema.parse(await readResponse.json());
        expect(read).toMatchObject({ id: fixture.commentNotice.id, read_at: expect.any(String), archived_at: null });
        await expect(comment).toHaveCount(0);
        await expectPath(notificationPath("unread", fixture.commentNotice.id, fixture.issue.id, `#comment-${fixture.commentId}`));
        await expect(detail.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
        await count(unreadBefore - 1);
        const body = notificationRow(page, fixture.bodyId);
        await body.getByRole("link", { name: fixture.title, exact: true }).click();
        await expectPath(notificationPath("unread", fixture.bodyId, fixture.issue.id, "#description"));
        await expect(page.locator("#description")).toBeFocused();
        const archiveButton = body.getByRole("button", { name: "Archive", exact: true });
        await archiveButton.evaluate((element) => element.focus({ preventScroll: true }));
        await expect(archiveButton).toBeFocused();
        const archivePending = page.waitForResponse((response) => response.request().method() === "POST"
          && new URL(response.url()).pathname === `/api/me/collaboration-notifications/${fixture.bodyId}/archive`);
        await archiveButton.press("Enter");
        const archiveResponse = await archivePending;
        expect(archiveResponse.status()).toBe(200);
        const archived = CollaborationNotificationReceiptSchema.parse(await archiveResponse.json());
        expect(archived).toMatchObject({ id: fixture.bodyId, read_at: null, archived_at: expect.any(String) });
        await expect(body).toHaveCount(0);
        await expectPath(notificationPath("unread", fixture.bodyId, fixture.issue.id, "#description"));
        await expect(detail.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
        await info.attach("notification-archive-focus", { contentType: "application/json", body: JSON.stringify(await page.evaluate(() => {
          const active = document.activeElement;
          return { tag: active?.tagName ?? null, id: active?.id ?? null, ariaLabel: active?.getAttribute("aria-label") ?? null,
            scrollRoot: active?.closest("[data-tab-scroll-root]")?.getAttribute("data-tab-scroll-root") ?? null,
            connected: active?.isConnected ?? false, inert: Boolean(active?.closest("[inert]")),
            hidden: Boolean(active && (active.closest('[hidden], [aria-hidden="true"]') || getComputedStyle(active).visibility !== "visible" || active.getClientRects().length === 0)) };
        }), null, 2) });
        await expect(notificationList(page)).toBeFocused();
        await count(unreadBefore - 2);
        await capture("notifications-processed-wide");
        await notificationViews(page).getByRole("link", { name: "All", exact: true }).click();
        await expect(notificationRow(page, fixture.commentNotice.id)).toHaveCount(1);
        await expect(notificationRow(page, fixture.commentNotice.id).getByRole("button", { name: "Mark read", exact: true })).toHaveCount(0);
        await expect(notificationRow(page, fixture.bodyId)).toHaveCount(0);
        await notificationViews(page).getByRole("link", { name: "Archived", exact: true }).click();
        await expect(notificationRow(page, fixture.bodyId)).toHaveAttribute("data-unread", "true");
        await expect(notificationRow(page, fixture.bodyId).getByRole("button", { name: "Archive", exact: true })).toHaveCount(0);
        await count(unreadBefore - 2);
        expect(writes).toEqual([{ path: `/api/me/collaboration-notifications/${fixture.commentNotice.id}/read`, method: "POST" },
          { path: `/api/me/collaboration-notifications/${fixture.bodyId}/archive`, method: "POST" }]);
      });
      await retainedList.dispose();
      expect(await fixture.records.source.countIssueDispatches(fixture.records.issue.id)).toBe(0);
      expect(await fixture.records.source.countIssueDispatches(fixture.issue.id)).toBe(0);
      expect(accountRequests.every((request) => !request.workspaceHeader)).toBe(true);
      expect(errors).toEqual([]);
      const nativeEvidence = native ? await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence) : null;
      if (nativeEvidence) expect(nativeEvidence.daemonStarts).toBe(0);
      await info.attach("collaboration-notifications-page-evidence", { contentType: "application/json", body: JSON.stringify({
        platform, database: process.env.TASK_DATABASE_NAME, source_issue_id: fixture.issue.id,
        exact_targets: { run: fixture.records.runId, comment: fixture.commentId, body_notification: fixture.bodyId, issue_notification: fixture.issueNoticeId },
        pagination, restoration, captures, notification_writes: writes, account_requests: accountRequests,
        history_source: "public human setup plus synthetic retained notifications and run records; no runtime ingestion claim",
        native: nativeEvidence, errors,
      }, null, 2) });
    } finally {
      await native?.app.close();
      if (profile) rmSync(profile, { recursive: true, force: true });
    }
  });
}
