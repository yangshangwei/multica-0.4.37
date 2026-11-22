import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import pg from "pg";
import { expect, test, type Page, type Request, type Route } from "@playwright/test";
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
  const first = await api.ensureWorkspace("First navigation space", `scope-first-${key}`);
  const second = await api.requestJSON<Workspace>("/api/workspaces", { method: "POST", body: { name: "Second navigation space", slug: `scope-second-${key}` } });
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
      await globalNav(page).getByRole("button", { name: fixture.second.name, exact: true }).click();
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true }).focus();
      await page.keyboard.press("Shift+Tab");
      await page.keyboard.press("Tab");
      await expect(collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true })).toBeFocused();
      const betaTooltip = page.locator('[data-slot="tooltip-content"]').filter({ hasText: betaDescription });
      await expect(betaTooltip).toBeVisible();
      await expect(betaTooltip).toHaveText(betaDescription);
      await expect(collaborationDirectory.getByRole("button", { name: "Expand collaboration navigation", exact: true })).toHaveAccessibleDescription(betaDescription);
      await globalNav(page).getByRole("button", { name: "New", exact: true }).click();
      await expect(page.getByRole("menuitem", { name: /^New Issue/ })).toBeEnabled();
      await expect(page.getByRole("menuitem", { name: "New collaboration issue", exact: true })).toBeEnabled();
      await expect(page.getByRole("menuitem", { name: "New collaboration issue", exact: true }).getByText("Beta", { exact: true })).toBeVisible();
      await page.keyboard.press("Escape");
      const originalBounds = await globalNav(page).boundingBox();
      await capture(`${platform.toLowerCase()}-scope-workspace.png`);
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
      await globalNav(page).getByRole("button", { name: fixture.second.name, exact: true }).click();
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await collaborationDirectory.getByText("Collaboration", { exact: true }).click();
      await collaborationDirectory.getByRole("link", { name: "Participating", exact: true }).click();
      await expect(collaborationDirectory.getByRole("link", { name: "Participating", exact: true })).toHaveAttribute("aria-current", "page");
      await globalNav(page).getByRole("button", { name: "New", exact: true }).click();
      await expect(page.getByRole("menuitem", { name: /^New Issue/ })).toBeDisabled();
      await page.getByRole("menuitem", { name: "New collaboration issue", exact: true }).click();
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
      await expect(globalNav(page).getByRole("button", { name: "New", exact: true })).toBeFocused();
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
      await globalNav(page).getByRole("button", { name: fixture.second.name, exact: true }).click();
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
        await globalNav(page).getByRole("button", { name: fixture.second.name, exact: true }).click();
        await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
        await returnToAccount();
        await expect(banner).toBeVisible();
        await detail.evaluate((element) => { element.scrollTop = 620; });
        await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
        expect(await detail.evaluate((element) => element.scrollTop)).toBe(620);
        await globalNav(page).getByRole("button", { name: fixture.second.name, exact: true }).click();
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
      await globalNav(page).getByRole("button", { name: fixture.second.name, exact: true }).click();
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
      await expect(compactNavigation.getByRole("button", { name: fixture.second.name, exact: true })).toBeVisible();
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
      await expect(chineseNavigation.getByRole("button", { name: reloadedWorkspace.name, exact: true })).toBeVisible();
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
      await expect(chineseNavigation.getByRole("link", { name: "工作空间设置", exact: true })).toBeVisible();
      await capture(`${platform.toLowerCase()}-scope-wide-zh.png`);
      await testInfo.attach("scope-restoration.json", { body: JSON.stringify({ platform, source: fixture.source.id, selected: selected.id, saved, originalBounds, resumedBounds }), contentType: "application/json" });
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
