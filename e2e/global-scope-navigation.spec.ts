import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import pg from "pg";
import { expect, test, type Locator, type Page, type Request, type Route } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { deepLink, nativeScreenshot, openDesktop } from "./fixtures/collaboration-native";
import { openSidebarNavigation } from "./fixtures/sidebar-navigation";

const apiUrl = process.env.COLLABORATION_E2E_API_URL;
const databaseUrl = process.env.DATABASE_URL;
type Workspace = { id: string; slug: string; name: string };
type Issue = { id: string; identifier: string; title: string };

async function seed() {
  const api = new TestApiClient();
  const key = randomUUID().slice(0, 8);
  const passwordAuth = process.env.E2E_PASSWORD_AUTH === "1";
  const passwordUsername = `scope${key}`;
  if (passwordAuth) {
    await api.registerPassword(passwordUsername, `Scope-${key}-Navigation!`, "Navigation viewer");
  } else {
    await api.login(`scope-${key}@example.invalid`, "Navigation viewer");
  }
  const first = await api.ensureWorkspace("First navigation space for research and shared delivery planning", `scope-first-${key}`);
  const second = await api.requestJSON<Workspace>("/api/workspaces", { method: "POST", body: { name: "Second navigation space for product delivery and release coordination", slug: `scope-second-${key}` } });
  if (passwordAuth) {
    await api.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
    await api.requestJSON("/api/me/onboarding/complete", { method: "POST", body: {} });
  } else {
    await api.markUserOnboarded();
  }
  api.setWorkspaceId(second.id);
  api.setWorkspaceSlug(second.slug);
  const viewer = await api.requestJSON<{ id: string }>("/api/me");
  const source: Issue = await api.createIssue(`Workspace task ${key}`, { assignee_type: "member", assignee_id: viewer.id });
  const issues: Issue[] = [];
  for (let i = 0; i < 24; i++) {
    issues.push(await api.createIssue(`Resume collaboration ${String(i).padStart(2, "0")} ${key}`, {
      assignee_type: "member", assignee_id: viewer.id,
      description: Array.from({ length: 65 }, (_, paragraph) => `Navigation fixture paragraph ${paragraph + 1}. This content makes the saved detail position observable.`).join("\n\n"),
    }));
  }
  const db = new pg.Client(databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    await db.query("INSERT INTO collaboration_account_fence (user_id) VALUES ($1) ON CONFLICT DO NOTHING", [viewer.id]);
    for (const issue of issues) {
      await db.query("INSERT INTO issue_collaboration (issue_id) VALUES ($1) ON CONFLICT DO NOTHING", [issue.id]);
      await db.query("INSERT INTO collaboration_session (issue_id) VALUES ($1) ON CONFLICT DO NOTHING", [issue.id]);
      await db.query("INSERT INTO issue_participant (issue_id,user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING", [issue.id, viewer.id]);
    }
    await db.query("COMMIT");
  } catch (error) {
    await db.query("ROLLBACK");
    throw error;
  } finally { await db.end(); }
  return { api, first, second, source, issues, key, passwordUsername: passwordAuth ? passwordUsername : null };
}

async function removeCollaborationFixtureRows(issueIds: string[]) {
  const db = new pg.Client(databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    await db.query("DELETE FROM issue_participant WHERE issue_id = ANY($1::uuid[])", [issueIds]);
    await db.query("DELETE FROM collaboration_session WHERE issue_id = ANY($1::uuid[])", [issueIds]);
    await db.query("DELETE FROM issue_collaboration WHERE issue_id = ANY($1::uuid[])", [issueIds]);
    await db.query("COMMIT");
  } catch (error) {
    await db.query("ROLLBACK");
    throw error;
  } finally { await db.end(); }
}

const globalNav = (page: Page) => page.getByRole("navigation", { name: "Application navigation", exact: true });

async function openNavigation(page: Page, locale: "en" | "zh-Hans" = "en") {
  const navigation = await openSidebarNavigation(page);
  await expect(navigation).toHaveAccessibleName(locale === "en" ? "Application navigation" : "应用导航");
  return navigation;
}

async function resumeWorkspace(page: Page, workspace: Workspace) {
  const previousUrl = page.url();
  const picker = globalNav(page).getByRole("button", { name: "Switch workspace", exact: true });
  await picker.getByText(workspace.name, { exact: true }).click();
  await expect(page.getByRole("menu")).toBeVisible();
  await expect(page).toHaveURL(previousUrl);
  await page.getByRole("menuitem").filter({ hasText: workspace.name }).click();
}

async function expectStartWork(page: Page, workspace: Workspace, locale: "en" | "zh-Hans" = "en") {
  const chinese = locale === "zh-Hans";
  const navigation = page.getByRole("navigation", { name: chinese ? "应用导航" : "Application navigation", exact: true });
  const group = navigation.getByRole("group", { name: chinese ? "开始工作" : "Start work", exact: true });
  const newIssue = group.getByRole("button", { name: chinese ? "新建任务" : "New Issue", exact: true });
  const collaboration = navigation.getByRole("button", { name: chinese ? "发起协作" : "Start collaboration", exact: true });
  const chat = group.getByRole("link", { name: chinese ? "AI 对话" : "AI Chat", exact: true });
  await expect(group).toBeVisible();
  await expect(newIssue).toBeVisible();
  await expect(newIssue).toBeEnabled();
  await expect(collaboration).toBeVisible();
  await expect(collaboration).toBeEnabled();
  await expect(collaboration).toHaveAccessibleDescription(/Beta/);
  await expect(navigation.locator('[data-sidebar-scope="collaboration"]').getByText("Beta", { exact: true })).toHaveCount(1);
  const picker = navigation.getByRole("button", { name: chinese ? "切换工作空间" : "Switch workspace", exact: true });
  await expect(picker).toHaveAccessibleDescription(workspace.name);
  await expect(picker.getByText(workspace.name, { exact: true })).toBeVisible();
  for (const control of [newIssue, chat]) {
    await expect(control.getByText(workspace.name, { exact: true })).toHaveCount(0);
    await expect(control).toHaveAccessibleDescription(new RegExp(workspace.name));
  }
  await expect(navigation.getByRole("link", { name: chinese ? "AI 对话" : "AI Chat", exact: true })).toHaveCount(1);
  await expect.poll(async () => {
    const href = await chat.getAttribute("href");
    return href === null ? null : new URL(href, page.url()).pathname;
  }).toBe(`/${workspace.slug}/chat`);
  await expect(navigation.getByRole("button", { name: /^(New|新建)$/ })).toHaveCount(0);
  const bounds = await Promise.all([newIssue, collaboration, chat].map((control) => control.boundingBox()));
  expect(bounds.every((box) => box !== null)).toBe(true);
  expect(bounds[1]!.y).toBeLessThan(bounds[0]!.y);
  expect(bounds[0]!.y + bounds[0]!.height).toBeLessThanOrEqual(bounds[2]!.y);
  expect(Math.abs(bounds[0]!.x - bounds[2]!.x)).toBeLessThanOrEqual(1);
  expect(Math.abs(bounds[0]!.width - bounds[2]!.width)).toBeLessThanOrEqual(1);
  if (await page.evaluate(() => matchMedia("(pointer: coarse)").matches)) {
    expect(bounds[1]!.width).toBeGreaterThanOrEqual(44);
    expect(bounds[1]!.height).toBeGreaterThanOrEqual(44);
  }
  return { group, newIssue, collaboration, chat };
}

async function setSidebarTheme(page: Page, theme: "light" | "dark" | "system") {
  const label = theme === "system" ? "Use System Theme" : `Switch to ${theme === "dark" ? "Dark" : "Light"} Theme`;
  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "Search", exact: true });
  await expect(palette).toBeVisible();
  await palette.getByRole("combobox").fill(label);
  await palette.getByRole("option", { name: new RegExp(`^${label}`) }).click();
  await expect(palette).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => localStorage.getItem("theme"))).toBe(theme);
  await expect.poll(() => page.evaluate((value) => {
    const resolved = value === "system" ? matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light" : value;
    return document.documentElement.classList.contains(resolved);
  }, theme)).toBe(true);
}

async function expectCompactStartWorkBounds(group: Locator, locale: "en" | "zh-Hans" = "en") {
  const labels = locale === "zh-Hans" ? ["新建任务", "AI 对话", "收件箱", "我的任务"] : ["New Issue", "AI Chat", "Inbox", "My Issues"];
  const geometry = await group.locator("button, a").evaluateAll((controls, labelNames) => {
    const bounds = (rect: DOMRect) => ({ left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom,
      width: rect.width, height: rect.height });
    return controls.map((control, index) => {
      const name = labelNames[index];
      const labelRects: DOMRect[] = [];
      let labelClipped = false;
      const walker = document.createTreeWalker(control, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        if (node.textContent?.trim() !== name) continue;
        const range = document.createRange();
        range.selectNodeContents(node);
        const rects = Array.from(range.getClientRects()).filter((rect) => rect.width > 1 && rect.height > 1);
        labelRects.push(...rects);
        // Include the label's own truncate spans, not just the button/anchor.
        for (let ancestor: Element | null = node.parentElement; ancestor; ancestor = ancestor.parentElement) {
          const style = getComputedStyle(ancestor);
          const clip = ancestor.getBoundingClientRect();
          if (rects.some((rect) =>
            style.overflowX !== "visible" && (rect.left < clip.left - 1 || rect.right > clip.right + 1)
            || style.overflowY !== "visible" && (rect.top < clip.top - 1 || rect.bottom > clip.bottom + 1))) {
            labelClipped = true;
          }
        }
      }
      return { name, control: bounds(control.getBoundingClientRect()), labels: labelRects.map(bounds),
        labelClipped,
        viewport: { width: innerWidth, height: innerHeight }, coarse: matchMedia("(pointer: coarse)").matches };
    });
  }, labels);
  expect(geometry).toHaveLength(labels.length);
  for (const row of geometry) {
    expect(row.control.width, `${row.name} must have a painted target`).toBeGreaterThan(0);
    expect(row.control.left).toBeGreaterThanOrEqual(-1);
    expect(row.control.right).toBeLessThanOrEqual(row.viewport.width + 1);
    expect(row.control.top).toBeGreaterThanOrEqual(-1);
    expect(row.control.bottom).toBeLessThanOrEqual(row.viewport.height + 1);
    expect(row.labels.length, `${row.name} must retain its visible action text`).toBeGreaterThan(0);
    expect(row.labelClipped, `${row.name} must remain readable inside its clipping ancestors`).toBe(false);
    if (row.coarse) expect(row.control.height, `${row.name} needs a 44px coarse target`).toBeGreaterThanOrEqual(44);
  }
  return geometry;
}

for (const platform of ["Web", "Desktop"] as const) {
  test(`${platform} sidebar disclosure and navigation history preserve collaboration filters and scroll positions`, async ({ page: webPage }, testInfo) => {
    test.setTimeout(180_000);
    test.skip(!apiUrl || !databaseUrl || (platform === "Desktop" && !process.env.COLLABORATION_ELECTRON_RENDERER_URL), "Requires the task-owned API/database and matching production clients");
    for (const value of [apiUrl!, databaseUrl!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
    const fixture = await seed();
    const profile = platform === "Desktop" ? mkdtempSync(join(tmpdir(), "multica-scope-navigation-")) : null;
    let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
    let page = webPage;
    const returnToAccount = async () => {
      if (native) {
        const navigation = globalNav(page);
        const expand = navigation.getByRole("button", { name: "Expand collaboration navigation", exact: true });
        if (await expand.count()) await expand.click();
        await navigation.getByRole("link", { name: "Participating", exact: true }).click();
        await page.locator('[data-slot="window-toolbar"]').getByRole("button", { name: "Go back", exact: true }).click();
      } else await page.goBack();
    };
    const capture = async (name: string) => {
      // Native capturePage does not apply Playwright screenshot's animation
      // handling. Wait for actual finite transitions in both clients instead.
      await expect.poll(() => page.evaluate(() => document.getAnimations().filter((animation) =>
        (animation.playState === "running" || animation.pending)
        && animation.effect?.getComputedTiming().endTime !== Infinity).length),
      { message: "Screenshots require settled application and popup transitions" }).toBe(0);
      await page.evaluate(async () => {
        await document.fonts.ready;
        await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
      });
      return native ? nativeScreenshot(native.app, page, testInfo.outputPath(name))
        : page.screenshot({ path: testInfo.outputPath(name), fullPage: false });
    };
    try {
      await webPage.setViewportSize({ width: 1440, height: 960 });
      const sourcePath = `/${fixture.second.slug}/issues/${fixture.source.id}`;
      if (profile) {
        native = await openDesktop(profile, { token: fixture.api.getToken()! });
        page = native.page;
        await native.start();
        const nativeWindow = await native.app.browserWindow(page);
        await nativeWindow.evaluate((target) => target.setContentSize(1440, 960));
        await nativeWindow.dispose();
        await expect(globalNav(page)).toBeVisible();
        await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), sourcePath);
      } else {
        await page.addInitScript((token) => {
          localStorage.setItem("multica_token", token);
          localStorage.setItem("multica-locale", "en");
          localStorage.setItem("multica:chat:isOpen", "false");
        }, fixture.api.getToken()!);
        await page.goto(sourcePath);
      }
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await expect(globalNav(page)).toHaveCount(1);
      await expect(page.locator('[data-slot="global-scope-bar"]')).toHaveCount(0);
      const collaborationDirectory = globalNav(page).locator('[data-sidebar-scope="collaboration"]');
      const betaDescription = "For cross-workspace collaboration. This feature is in Beta, and some functionality is still being developed.";
      const collaborationHeading = collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true });
      await expect(collaborationHeading).toHaveAttribute("aria-expanded", "false");
      await expect.poll(() => collaborationHeading.evaluate((element) =>
        Math.abs(element.getBoundingClientRect().width - element.parentElement!.getBoundingClientRect().width)),
      { message: "The collaboration heading must cover the complete menu row" }).toBeLessThan(1);
      await expect(collaborationDirectory.getByRole("link", { name: "Participating", exact: true })).toHaveCount(0);
      await expect(collaborationDirectory.getByText("Beta", { exact: true })).toBeVisible();

      // Disclosure leaves the current page in place. The explicit notification
      // entry opens its account page, which survives a directory collapse.
      await collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true }).focus();
      await page.keyboard.press("Enter");
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await expect(collaborationDirectory.getByRole("link", { name: "Current handling", exact: true })).toBeVisible();
      const notificationsLink = collaborationDirectory.getByRole("link", { name: "Collaboration notifications", exact: true });
      await expect(notificationsLink).toHaveAttribute("href", /\/account\?view=notifications&filter=unread$/);
      await notificationsLink.click();
      const notificationsHeading = page.getByRole("heading", { name: "Collaboration notifications", exact: true });
      await expect(notificationsHeading).toBeVisible();
      await expect(notificationsLink).toHaveAttribute("aria-current", "page");
      await expect(notificationsLink).toHaveAttribute("data-active");
      await expect(page.locator('[data-slot="popover-content"]')).toHaveCount(0);
      await capture(`${platform.toLowerCase()}-scope-notifications.png`);
      await collaborationDirectory.getByRole("button", { name: "Collapse collaboration navigation", exact: true }).click();
      await expect(notificationsHeading).toBeVisible();
      await expect(notificationsLink).toHaveCount(0);
      await expect(collaborationDirectory.getByRole("link", { name: "Current handling", exact: true })).toHaveCount(0);
      await resumeWorkspace(page, fixture.second);
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true }).focus();
      await page.keyboard.press("Shift+Tab");
      await page.keyboard.press("Tab");
      await expect(collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true })).toBeFocused();
      const betaTooltip = page.locator('[data-slot="tooltip-content"]').filter({ hasText: betaDescription });
      await expect(betaTooltip).toBeVisible();
      await expect(betaTooltip).toHaveText(betaDescription);
      await expect(collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true })).toHaveAccessibleDescription(betaDescription);
      await (await expectStartWork(page, fixture.second)).newIssue.focus();
      const originalBounds = await globalNav(page).boundingBox();
      await capture(`${platform.toLowerCase()}-scope-workspace.png`);
      const originalTheme = await page.evaluate(() => {
        const preference = localStorage.getItem("theme");
        return preference === "light" || preference === "dark" ? preference : "system";
      });
      try {
        for (const theme of ["light", "dark"] as const) {
          await setSidebarTheme(page, theme);
          await (await expectStartWork(page, fixture.second)).newIssue.focus();
          await capture(`${platform.toLowerCase()}-scope-start-work-${theme}.png`);
        }
      } finally {
        await setSidebarTheme(page, originalTheme);
      }
      await collaborationDirectory.getByText("Collaboration", { exact: true }).click();
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await globalNav(page).getByRole("link", { name: "Participating", exact: true }).click();
      await expect(page.locator("main > header").getByText(betaDescription, { exact: true })).toBeVisible();
      await collaborationDirectory.getByRole("button", { name: "Collapse collaboration navigation", exact: true }).click();
      await expect(collaborationDirectory.getByRole("link", { name: "Participating", exact: true })).toHaveCount(0);
      await expect(page.getByRole("heading", { name: "Participating", exact: true })).toBeVisible();
      await collaborationDirectory.getByText("Collaboration", { exact: true }).click();
      await expect(collaborationDirectory.getByRole("button", { name: "Collapse collaboration navigation", exact: true })).toHaveAttribute("aria-expanded", "true");
      await expect(page.getByRole("heading", { name: "Participating", exact: true })).toBeVisible();
      await collaborationDirectory.getByRole("button", { name: "Collapse collaboration navigation", exact: true }).click();
      await resumeWorkspace(page, fixture.second);
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await collaborationDirectory.getByText("Collaboration", { exact: true }).click();
      await collaborationDirectory.getByRole("link", { name: "Participating", exact: true }).click();
      await expect(collaborationDirectory.getByRole("link", { name: "Participating", exact: true })).toHaveAttribute("aria-current", "page");
      await collaborationDirectory.getByRole("button", { name: "Collapse collaboration navigation", exact: true }).click();
      const startWork = await expectStartWork(page, fixture.second);
      await startWork.collaboration.click();
      const creation = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
      await expect(creation).toBeVisible();
      const creationWorkspace = creation.getByRole("combobox", { name: "Source workspace", exact: true });
      await expect(creationWorkspace).toHaveValue(fixture.second.id);
      await expect(creation.getByRole("textbox", { name: "Title", exact: true })).toBeVisible();
      await creationWorkspace.selectOption(fixture.first.id);
      await expect(creationWorkspace).toHaveValue(fixture.first.id);
      await expect(creation.getByRole("textbox", { name: "Title", exact: true })).toBeVisible();
      await creation.getByRole("button", { name: "Close", exact: true }).click();
      await expect(creation).toBeHidden();
      await expect(startWork.collaboration).toBeFocused();

      // One account-scope click must enter the displayed workspace before its
      // existing creation host opens. Modal semantics exclude background links.
      const workspaceIssues = `/${fixture.second.slug}/issues`;
      const ordinaryCreation = page.getByRole("dialog", { name: /^(New Issue|Quick create issue)$/ });
      await expect(ordinaryCreation).toHaveCount(0);
      const dialogObservation = await page.evaluateHandle(() => {
        const previous = new Set(document.querySelectorAll('[data-slot="dialog-content"]'));
        const opened = new Set<Element>();
        const observer = new MutationObserver(() => {
          for (const popup of document.querySelectorAll('[data-slot="dialog-content"]')) {
            if (!previous.has(popup)) opened.add(popup);
          }
        });
        observer.observe(document.body, { childList: true, subtree: true });
        return { opened, observer };
      });
      let ordinaryModalOpenings = 0;
      let ordinaryWorkspaceHeaders: Record<string, string> = {};
      try {
        await startWork.newIssue.click();
        await expect(ordinaryCreation).toHaveCount(1);
        await expect(ordinaryCreation).toBeVisible();
        if (!native) await expect(page).toHaveURL(workspaceIssues);
        await expect.poll(() => dialogObservation.evaluate(({ opened }) => opened.size)).toBe(1);
        await capture(`${platform.toLowerCase()}-scope-start-work-creation.png`);
        await page.keyboard.press("Escape");
        await expect(ordinaryCreation).toHaveCount(0);
        ordinaryModalOpenings = await dialogObservation.evaluate(({ opened }) => opened.size);
        expect(ordinaryModalOpenings).toBe(1);
        await expect(globalNav(page).getByRole("link", { name: "Workspace issues", exact: true })).toHaveAttribute("aria-current", "page");
      } finally {
        await dialogObservation.evaluate(({ observer }) => observer.disconnect());
        await dialogObservation.dispose();
      }

      // The detail already loaded the legacy GET catalog with Infinity
      // staleTime. Reload the admitted list for its current read-only Table
      // query channel and verify that the transient form cannot replay.
      const ordinaryRead = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/issues/table/rows"
        && response.request().method() === "POST" && response.status() === 200);
      await page.reload();
      const workspaceResponse = await ordinaryRead;
      expect(await workspaceResponse.finished()).toBeNull();
      ordinaryWorkspaceHeaders = await workspaceResponse.request().allHeaders();
      expect(ordinaryWorkspaceHeaders["x-workspace-slug"]).toBe(fixture.second.slug);
      await expect(globalNav(page).getByRole("link", { name: "Workspace issues", exact: true })).toHaveAttribute("aria-current", "page");
      await expect(ordinaryCreation).toHaveCount(0);

      const restoreParticipating = async () => {
        await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), sourcePath);
        await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
        const expand = collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true });
        if (await expand.count()) await expand.click();
        await collaborationDirectory.getByRole("link", { name: "Participating", exact: true }).click();
        await expect(page.getByRole("heading", { name: "Participating", exact: true })).toBeVisible();
      };
      await restoreParticipating();
      await expect(ordinaryCreation).toHaveCount(0);
      const accountChat = (await expectStartWork(page, fixture.second)).chat;
      await accountChat.click();
      await expect(page.getByRole("heading", { name: "AI Chat", exact: true })).toBeVisible();
      await expect(accountChat).toHaveAttribute("aria-current", "page");
      if (!native) await expect(page).toHaveURL(`/${fixture.second.slug}/chat`);
      await expect(ordinaryCreation).toHaveCount(0);
      await capture(`${platform.toLowerCase()}-scope-start-work-chat.png`);
      await restoreParticipating();
      await expect(ordinaryCreation).toHaveCount(0);
      await globalNav(page).getByRole("button", { name: "Help", exact: true }).click();
      await page.getByRole("menuitem", { name: "Feedback", exact: true }).click();
      const feedback = page.getByRole("dialog", { name: "Feedback", exact: true });
      await expect(feedback).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(feedback).toBeHidden();
      const selected = fixture.issues[0]!;
      await page.getByRole("region", { name: "Collaboration issues" }).getByRole("link", { name: new RegExp(selected.title) }).click();
      await expect(page.getByRole("heading", { name: selected.title, exact: true })).toBeVisible();
      const search = page.getByRole("textbox", { name: "Search loaded issues" });
      await search.fill("Resume collaboration");
      await page.getByRole("combobox", { name: "Sort" }).selectOption("title");
      const list = page.locator('[data-tab-scroll-root="account-list"]');
      const detail = page.locator('[data-tab-scroll-root="account-detail"]');
      await expect.poll(() => list.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(200);
      await expect.poll(() => detail.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(700);
      await list.evaluate((element) => { element.scrollTop = 180; });
      await detail.evaluate((element) => { element.scrollTop = 620; });
      // Let the real capture listener observe a committed frame before switching.
      await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      const saved = { list: await list.evaluate((element) => element.scrollTop), detail: await detail.evaluate((element) => element.scrollTop) };
      await resumeWorkspace(page, fixture.second);
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await returnToAccount();
      await expect(page.getByRole("heading", { name: selected.title, exact: true })).toBeVisible();
      await expect(search).toHaveValue("Resume collaboration");
      await expect(page.getByRole("combobox", { name: "Sort" })).toHaveValue("title");
      await expect.poll(() => list.evaluate((element) => element.scrollTop)).toBe(saved.list);
      await expect.poll(() => detail.evaluate((element) => element.scrollTop)).toBe(saved.detail);
      await collaborationDirectory.getByText("Collaboration", { exact: true }).click();
      expect(await detail.evaluate((element) => element.scrollTop)).toBe(saved.detail);
      const resumedBounds = await globalNav(page).boundingBox();
      expect(resumedBounds?.x).toBe(originalBounds?.x);
      expect(resumedBounds?.y).toBe(originalBounds?.y);
      await capture(`${platform.toLowerCase()}-scope-resumed.png`);

      // The app owns this root's saved offset. A retained read error can still
      // be present on resume and disappear only after an explicit refresh completes.
      const issueUrl = (url: URL) => url.pathname === `/api/collaboration/issues/${selected.id}`;
      let recovering = false;
      let releaseRecovery!: () => void;
      let recoveryRequested!: () => void;
      const recoveryReady = new Promise<void>((resolve) => { recoveryRequested = resolve; });
      const recoveryRequests = new Set<Request>();
      const recoveryRelease = new Promise<void>((resolve) => { releaseRecovery = resolve; });
      const banner = detail.locator(":scope > div > [role=alert]");
      const activeRecoveries = new Set<Promise<void>>();
      const recover = async (route: Route) => {
        if (!recovering) {
          await route.abort("failed");
          return;
        }
        // Refresh and realtime invalidation can replace an in-flight read,
        // including after release. Track the selected issue's actual transport.
        recoveryRequests.add(route.request());
        recoveryRequested();
        await recoveryRelease;
        // Continue the original renderer request so recovery uses its actual
        // transport response in both Chromium and Electron.
        await route.continue();
      };
      const intercept = (route: Route) => {
        const operation = recover(route);
        activeRecoveries.add(operation);
        return operation.finally(() => activeRecoveries.delete(operation));
      };
      await page.route(issueUrl, intercept);
      try {
        await resumeWorkspace(page, fixture.second);
        await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
        await returnToAccount();
        await expect(banner).toBeVisible();
        await detail.evaluate((element) => { element.scrollTop = 620; });
        await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
        expect(await detail.evaluate((element) => element.scrollTop)).toBe(620);
        await resumeWorkspace(page, fixture.second);
        await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
        await returnToAccount();
        await expect(banner).toBeVisible();
        await expect.poll(() => detail.evaluate((element) => element.scrollTop)).toBe(620);
        // Scope switching can cancel earlier reads. Finish those injected
        // failures before starting one refresh on the currently rendered detail.
        await expect.poll(() => activeRecoveries.size).toBe(0);
        const refresh = banner.getByRole("button", { name: "Refresh", exact: true });
        // Keyboard activation keeps the saved offset while exercising the real
        // refresh control; a pointer click would scroll its offscreen button in.
        await refresh.evaluate((element) => element.focus({ preventScroll: true }));
        recovering = true;
        // Bound the entire explicit recovery window, including replacements.
        // The barrier prevents a success before the saved offset is checked.
        const recovered = page.waitForResponse((response) =>
          response.status() === 200 && recoveryRequests.has(response.request()), { timeout: 10_000 });
        await refresh.press("Enter");
        await recoveryReady;
        // A second real refresh cancels/replaces an in-flight query. Keep both
        // held so the assertion cannot attach only to the superseded request.
        const heldBeforeReplacement = recoveryRequests.size;
        await refresh.press("Enter");
        await expect.poll(() => recoveryRequests.size).toBeGreaterThan(heldBeforeReplacement);
        expect(await detail.evaluate((element) => element.scrollTop)).toBe(620);
        releaseRecovery();
        const response = await recovered;
        expect(response.status()).toBe(200);
        expect(await response.finished()).toBeNull();
        await expect(banner).toHaveCount(0);
        await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
        expect(await detail.evaluate((element) => element.scrollTop)).toBe(620);
      } finally {
        releaseRecovery();
        await Promise.allSettled([...activeRecoveries]);
        await page.unroute(issueUrl, intercept);
      }

      const other = fixture.issues[1]!;
      const explicit = `/account?view=participating&issue=${other.id}#description`;
      if (native) await deepLink(native.app, explicit);
      else await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), explicit);
      await expect(page.getByRole("heading", { name: other.title, exact: true })).toBeVisible();
      await expect(page.locator("#description")).toBeFocused();
      await detail.evaluate((element) => { element.scrollTop = 800; });
      await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      expect(await detail.evaluate((element) => element.scrollTop)).toBe(800);
      await resumeWorkspace(page, fixture.second);
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await returnToAccount();
      await expect.poll(() => detail.evaluate((element) => element.scrollTop)).toBe(800);

      if (native) {
        const window = await native.app.browserWindow(page);
        await window.evaluate((target) => { target.setContentSize(900, 1000); target.webContents.setZoomFactor(2); });
        await window.dispose();
      } else await page.setViewportSize({ width: 390, height: 844 });
      await expect.poll(() => page.evaluate(() => innerWidth)).toBe(native ? 450 : 390);
      const compactNavigation = await openNavigation(page);
      const compactStartWork = await expectStartWork(page, fixture.second);
      await compactStartWork.newIssue.focus();
      await page.keyboard.press("Tab");
      await expect(compactStartWork.chat).toBeFocused();
      const compactStartWorkBounds = await expectCompactStartWorkBounds(compactStartWork.group);
      await testInfo.attach("start-work-compact-en-geometry.json", { body: JSON.stringify(compactStartWorkBounds), contentType: "application/json" });
      await expect(compactNavigation.getByRole("button", { name: "Switch workspace", exact: true })).toHaveAccessibleDescription(fixture.second.name);
      await expect(compactNavigation.getByRole("button", { name: /^(Expand|Collapse) collaboration navigation$/ })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await capture(`${platform.toLowerCase()}-scope-compact.png`);
      // The query-only view change must dismiss the compact navigation sheet.
      await compactNavigation.getByRole("link", { name: "Current handling", exact: true }).click();
      await expect(page.locator('[data-slot="sidebar"][data-mobile="true"]')).toBeHidden();
      await expect(page.getByRole("heading", { name: "Current handling", exact: true })).toBeVisible();
      await openNavigation(page);
      await globalNav(page).getByRole("link", { name: "Participating", exact: true }).click();
      await expect(page.locator('[data-slot="sidebar"][data-mobile="true"]')).toBeHidden();
      await expect(page.getByRole("heading", { name: "Participating", exact: true })).toBeVisible();
      await capture(`${platform.toLowerCase()}-scope-compact-dismissed.png`);
      await fixture.api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
      await page.addInitScript(() => {
        localStorage.setItem("multica-locale", "zh-Hans");
        document.cookie = "multica-locale=zh-Hans; path=/; SameSite=Lax";
      });
      await page.reload();
      const chineseNavigation = await openNavigation(page, "zh-Hans");
      // Web scope history lives for this document session; a full account
      // reload previews the first authoritative member. Desktop persists MRU.
      const reloadedWorkspace = native ? fixture.second : fixture.first;
      const chineseStartWork = await expectStartWork(page, reloadedWorkspace, "zh-Hans");
      const chineseStartWorkBounds = await expectCompactStartWorkBounds(chineseStartWork.group, "zh-Hans");
      await testInfo.attach("start-work-compact-zh-geometry.json", { body: JSON.stringify(chineseStartWorkBounds), contentType: "application/json" });
      await expect(chineseNavigation.getByRole("button", { name: "切换工作空间", exact: true })).toHaveAccessibleDescription(reloadedWorkspace.name);
      await expect(chineseNavigation.getByRole("button", { name: /^(展开|收起)协作空间导航$/ })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await capture(`${platform.toLowerCase()}-scope-compact-zh.png`);
      await chineseNavigation.getByRole("link", { name: "当前处理", exact: true }).click();
      await expect(page.locator('[data-slot="sidebar"][data-mobile="true"]')).toBeHidden();
      if (native) {
        const window = await native.app.browserWindow(page);
        await window.evaluate((target) => { target.webContents.setZoomFactor(1); target.setContentSize(1440, 960); });
        await window.dispose();
      } else await page.setViewportSize({ width: 1440, height: 960 });
      await expect.poll(() => page.evaluate(() => innerWidth)).toBe(1440);
      await openNavigation(page, "zh-Hans");
      await expect(chineseNavigation.getByRole("link", { name: "当前处理", exact: true })).toHaveAttribute("aria-current", "page");
      await expect(chineseNavigation.getByRole("link", { name: "设置", exact: true })).toBeVisible();
      await capture(`${platform.toLowerCase()}-scope-wide-zh.png`);
      await testInfo.attach("scope-restoration.json", { body: JSON.stringify({ platform, source: fixture.source.id, selected: selected.id, saved, originalBounds, resumedBounds,
        ordinaryCreation: { workspaceId: fixture.second.id, modalOpenings: ordinaryModalOpenings,
          workspaceHeader: ordinaryWorkspaceHeaders["x-workspace-slug"], reloadedWithoutReplay: true } }), contentType: "application/json" });
      if (native) expect(native.errors).toEqual([]);
    } finally {
      await native?.app.close();
      if (profile) rmSync(profile, { recursive: true, force: true });
      // These read-only protocol rows were seeded directly for this fixture.
      // Remove only those exact rows before normal workspace cleanup; the API
      // intentionally refuses deleting any workspace with retained collaboration.
      await removeCollaborationFixtureRows(fixture.issues.map((issue) => issue.id));
      for (const workspace of [fixture.second, fixture.first]) {
        fixture.api.setWorkspaceId(workspace.id);
        fixture.api.setWorkspaceSlug(workspace.slug);
        await fixture.api.deleteFeatureWorkspace(workspace.id);
      }
      if (fixture.passwordUsername) await fixture.api.deletePasswordAccount(fixture.passwordUsername);
    }
  });
}
