import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import pg from "pg";
import { expect, test, type Locator, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { nativeScreenshot, openDesktop } from "./fixtures/collaboration-native";
import { iterationOptOut } from "./fixtures/iterations-i1";
import { openSidebarNavigation } from "./fixtures/sidebar-navigation";

const apiUrl = process.env.COLLABORATION_E2E_API_URL;
const databaseUrl = process.env.DATABASE_URL;
type Workspace = { id: string; slug: string; name: string };
type Issue = { id: string; title: string };
type Native = Awaited<ReturnType<typeof openDesktop>>;
type Locale = "en" | "zh-Hans";
type IconGeometry = { name: string; targetWidth: number; targetHeight: number; iconWidth: number; iconHeight: number; dx: number; dy: number };

const copy = {
  en: { navigation: "Application navigation", create: "New Issue", chat: "AI Chat", notifications: "Collaboration notifications",
    myIssues: "My Issues", issues: "Workspace issues", projects: "Projects", agents: "Agents", more: "More navigation",
    settings: "Settings", help: "Help", switchWorkspace: "Switch workspace", startWork: "Start work", collaboration: "Start collaboration",
    current: "Current handling", resources: "Resources & connections", work: "Work", iterations: "Iterations", triage: "Triage" },
  "zh-Hans": { navigation: "应用导航", create: "新建任务", chat: "AI 对话", notifications: "协作通知",
    myIssues: "我的任务", issues: "任务", projects: "项目", agents: "智能体", more: "更多入口",
    settings: "设置", help: "帮助", switchWorkspace: "切换工作空间", startWork: "开始工作", collaboration: "发起协作",
    current: "当前处理", resources: "资源与连接", work: "工作", iterations: "迭代", triage: "分拣台" },
} as const;

function navigation(page: Page, locale: Locale = "en") {
  return page.getByRole("navigation", { name: copy[locale].navigation, exact: true });
}

function rail(page: Page) {
  return page.locator('[data-slot="sidebar"][data-collapsible="icon"]');
}

function toggle(page: Page) {
  return page.locator('[data-slot="sidebar-trigger"]:visible');
}

async function seed() {
  const api = new TestApiClient();
  const key = randomUUID().slice(0, 8);
  const passwordUsername = process.env.E2E_PASSWORD_AUTH === "1" ? `rail${key}` : null;
  if (passwordUsername) {
    await api.registerPassword(passwordUsername, `Rail-${key}-Navigation!`, "Rail viewer");
  } else {
    await api.login(`rail-${key}@example.invalid`, "Rail viewer");
  }
  const first = await api.ensureWorkspace("First rail workspace for product delivery and shared planning", `rail-first-${key}`);
  const second = await api.requestJSON<Workspace>("/api/workspaces", { method: "POST",
    body: { name: "Second rail workspace with a long name for captions and menu recovery", slug: `rail-second-${key}` } });
  if (passwordUsername) {
    await api.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
    await api.requestJSON("/api/me/onboarding/complete", { method: "POST", body: {} });
  } else {
    await api.markUserOnboarded();
  }
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
  api.setWorkspaceId(second.id);
  api.setWorkspaceSlug(second.slug);
  const viewer = await api.requestJSON<{ id: string }>("/api/me");
  const source: Issue = await api.createIssue(`Rail source ${key}`, { assignee_type: "member", assignee_id: viewer.id });
  const notice: Issue = await api.createIssue(`Rail notification ${key}`, { assignee_type: "member", assignee_id: viewer.id });
  await iterationOptOut(api, second);
  expect(await api.requestJSON(`/api/workspaces/${second.id}/iteration-capabilities`))
    .toMatchObject({ workspace_id: second.id, supported: true, manual: true, enabled: false });
  const triage = await api.requestJSON<{ revision: number }>("/api/triage/settings");
  await api.requestJSON("/api/triage/settings", { method: "PUT", body: {
    enabled: false, acceptance_status: "todo", require_priority: false,
    responsibility_mode: "none", responsibility_member_id: null, expected_revision: triage.revision,
  } });
  expect(await api.requestJSON("/api/triage/settings")).toMatchObject({ enabled: false });

  // Retained notification facts exercise the real protected unread endpoint.
  // No grant, runtime, daemon or execution command is created by this fixture.
  const notifications = [randomUUID(), randomUUID(), randomUUID()];
  const db = new pg.Client(databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    await db.query("INSERT INTO collaboration_account_fence (user_id) VALUES ($1) ON CONFLICT DO NOTHING", [viewer.id]);
    await db.query("INSERT INTO issue_collaboration (issue_id) VALUES ($1)", [notice.id]);
    await db.query("INSERT INTO issue_participant (issue_id,user_id) VALUES ($1,$2)", [notice.id, viewer.id]);
    await db.query("INSERT INTO collaboration_session (issue_id,event_seq) VALUES ($1,3)", [notice.id]);
    for (const [index, id] of notifications.entries()) {
      await db.query(`INSERT INTO collaboration_notification
        (id,recipient_user_id,issue_id,event_seq,kind,source_kind)
        VALUES ($1,$2,$3,$4,'information_requested','issue')`, [id, viewer.id, notice.id, index + 1]);
    }
    await db.query("COMMIT");
  } catch (error) {
    await db.query("ROLLBACK");
    throw error;
  } finally { await db.end(); }
  return { api, key, first, second, source, notice, notifications, passwordUsername };
}

async function removeRetainedFacts(fixture: Awaited<ReturnType<typeof seed>>) {
  const db = new pg.Client(databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    await db.query("DELETE FROM collaboration_notification WHERE id=ANY($1::uuid[])", [fixture.notifications]);
    await db.query("DELETE FROM issue_participant WHERE issue_id=$1", [fixture.notice.id]);
    await db.query("DELETE FROM collaboration_session WHERE issue_id=$1", [fixture.notice.id]);
    await db.query("DELETE FROM issue_collaboration WHERE issue_id=$1", [fixture.notice.id]);
    await db.query("COMMIT");
  } catch (error) {
    await db.query("ROLLBACK");
    throw error;
  } finally { await db.end(); }
}

async function resize(page: Page, native: Native | undefined, width: number, height: number, zoom = 1) {
  if (native) {
    const window = await native.app.browserWindow(page);
    try {
      await window.evaluate((target, size) => {
        target.webContents.setZoomFactor(size.zoom);
        target.setContentSize(size.width, size.height);
      }, { width, height, zoom });
    } finally { await window.dispose(); }
  } else {
    await page.setViewportSize({ width, height });
  }
  await expect.poll(() => page.evaluate(() => innerWidth)).toBe(width / zoom);
}

async function capture(page: Page, native: Native | undefined, info: TestInfo, name: string) {
  await expect.poll(() => page.evaluate(() => document.getAnimations().filter((animation) =>
    (animation.playState === "running" || animation.pending)
    && animation.effect?.getComputedTiming().endTime !== Infinity).length)).toBe(0);
  await page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
  });
  const path = info.outputPath(`${name}.png`);
  if (native) await nativeScreenshot(native.app, page, path);
  else await page.screenshot({ path, fullPage: false });
  await info.attach(name, { path, contentType: "image/png" });
}

async function expectWidth(page: Page, width: number) {
  await expect.poll(() => page.locator('[data-slot="sidebar-container"]').evaluate((element) =>
    element.getBoundingClientRect().width)).toBe(width);
  await expect.poll(() => page.locator('[data-slot="sidebar-gap"]').evaluate((element) =>
    element.getBoundingClientRect().width)).toBe(width);
}

async function collapseAfterWebScopeChange(page: Page) {
  // Web owns the open state in each route shell. Scope changes may mount a
  // new provider; only the expanded width is a durable shared preference.
  await openSidebarNavigation(page);
  await expectWidth(page, 300);
  await toggle(page).click();
  await expectWidth(page, 64);
}

async function expectPaintedControl(control: Locator) {
  await expect(control).toBeVisible();
  const geometry = await control.evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    const hit = document.elementFromPoint(bounds.left + bounds.width / 2, bounds.top + bounds.height / 2);
    return { x: bounds.x, y: bounds.y, right: bounds.right, bottom: bounds.bottom, width: bounds.width, height: bounds.height,
      hit: hit !== null && element.contains(hit), disabled: element instanceof HTMLButtonElement && element.disabled,
      coarse: matchMedia("(any-pointer: coarse)").matches,
      viewport: { width: innerWidth, height: innerHeight } };
  });
  expect(geometry.width).toBeGreaterThan(0);
  expect(geometry.height).toBeGreaterThan(0);
  expect(geometry.x).toBeGreaterThanOrEqual(0);
  expect(geometry.y).toBeGreaterThanOrEqual(0);
  expect(geometry.right).toBeLessThanOrEqual(geometry.viewport.width);
  expect(geometry.bottom).toBeLessThanOrEqual(geometry.viewport.height);
  if (!geometry.disabled) expect(geometry.hit).toBe(true);
  if (geometry.coarse) expect(geometry.height).toBeGreaterThanOrEqual(44);
  return geometry;
}

async function expectCommonEntries(page: Page, workspace: Workspace, locale: Locale = "en") {
  const text = copy[locale];
  const nav = navigation(page, locale);
  await expect(rail(page)).toBeVisible();
  await expectWidth(page, 64);
  await expect(toggle(page)).toHaveCount(1);
  const controls = [nav.getByRole("button", { name: text.create, exact: true }),
    nav.getByRole("link", { name: text.chat, exact: true }),
    nav.getByRole("link", { name: text.notifications, exact: true }),
    ...[text.myIssues, text.issues, text.projects, text.agents].map((name) => nav.getByRole("link", { name, exact: true })),
    nav.getByRole("button", { name: text.more, exact: true }),
    nav.getByRole("link", { name: text.settings, exact: true }),
    nav.getByRole("button", { name: text.help, exact: true }),
    nav.getByRole("button", { name: "Rail viewer", exact: true })];
  const dimensions = [];
  for (const control of controls) dimensions.push(await expectPaintedControl(control));
  for (const [label, section] of [[text.chat, "chat"], [text.myIssues, "my-issues"], [text.issues, "issues"], [text.projects, "projects"], [text.agents, "agents"], [text.settings, "settings"]]) {
    await expect(nav.getByRole("link", { name: label, exact: true })).toHaveAttribute("href", new RegExp(`/${workspace.slug}/${section}$`));
  }
  await expect(nav.getByRole("link", { name: text.chat, exact: true })).toHaveAccessibleDescription(new RegExp(workspace.name));
  await expect(nav.getByRole("button", { name: text.create, exact: true })).toHaveAccessibleDescription(new RegExp(workspace.name));
  await expect(nav.getByRole("link", { name: text.notifications, exact: true })).toHaveAttribute("href", /\/account\?view=notifications&filter=unread$/);
  return dimensions;
}

async function coarseRailGeometry(page: Page) {
  const nav = navigation(page);
  const controls = [nav.getByRole("button", { name: copy.en.create, exact: true }),
    ...[copy.en.chat, copy.en.notifications, copy.en.myIssues, copy.en.issues, copy.en.projects, copy.en.agents, copy.en.settings]
      .map((name) => nav.getByRole("link", { name, exact: true })),
    nav.getByRole("button", { name: copy.en.more, exact: true }),
    nav.getByRole("button", { name: copy.en.help, exact: true }),
    nav.getByRole("button", { name: "Rail viewer", exact: true })];
  const targets = [];
  const icons: IconGeometry[] = [];
  for (const control of controls) {
    await control.scrollIntoViewIfNeeded();
    targets.push(await expectPaintedControl(control));
    if (await control.locator("svg").count() !== 1) continue;
    icons.push(await control.evaluate((element) => {
      const target = element.getBoundingClientRect();
      const icon = element.querySelector("svg")!.getBoundingClientRect();
      return { name: element.getAttribute("aria-label") ?? element.textContent ?? "",
        targetWidth: target.width, targetHeight: target.height, iconWidth: icon.width, iconHeight: icon.height,
        dx: icon.left + icon.width / 2 - target.left - target.width / 2,
        dy: icon.top + icon.height / 2 - target.top - target.height / 2 };
    }));
  }
  const scroll = await page.locator('[data-slot="sidebar-content"]').evaluate((element) => ({
    top: element.scrollTop, scrollHeight: element.scrollHeight, clientHeight: element.clientHeight,
    overflowY: getComputedStyle(element).overflowY,
  }));
  return { targets, icons, scroll };
}

async function nativeToolbarGeometry(page: Page) {
  const toolbar = page.locator('[data-slot="window-toolbar"]');
  const controls = toolbar.getByRole("button");
  for (const control of await controls.all()) {
    await expectPaintedControl(control);
    await expect.poll(() => control.evaluate((element) => getComputedStyle(element).getPropertyValue("-webkit-app-region"))).toBe("no-drag");
  }
  const geometry = await page.locator('[data-slot="sidebar-wrapper"] > div:last-child').evaluate((element) => {
    const header = element.querySelector<HTMLElement>(":scope > header");
    if (!header) throw new Error("Desktop main header is missing");
    const bar = document.querySelector<HTMLElement>('[data-slot="window-toolbar"]');
    if (!bar) throw new Error("Desktop window toolbar is missing");
    const tab = header.querySelector<HTMLElement>('button[aria-current="page"]');
    if (!tab) throw new Error("The active native tab is missing");
    return { headerPadding: Number.parseFloat(getComputedStyle(header).paddingLeft), toolbarRight: bar.getBoundingClientRect().right,
      activeTabLeft: tab.getBoundingClientRect().left, mainLeft: element.getBoundingClientRect().left };
  });
  expect(geometry.headerPadding).toBe(136);
  expect(geometry.mainLeft).toBe(64);
  expect(geometry.activeTabLeft).toBeGreaterThanOrEqual(geometry.toolbarRight);
  return geometry;
}

async function setTheme(page: Page, theme: "light" | "dark") {
  const label = `Switch to ${theme === "dark" ? "Dark" : "Light"} Theme`;
  await page.keyboard.press("ControlOrMeta+k");
  const search = page.getByRole("dialog", { name: "Search", exact: true });
  await expect(search).toBeVisible();
  await search.getByRole("combobox").fill(label);
  await search.getByRole("option", { name: new RegExp(`^${label}`) }).click();
  await expect(search).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => localStorage.getItem("theme"))).toBe(theme);
  await expect(page.locator("html")).toHaveClass(new RegExp(theme));
}

for (const platform of ["Web", "Desktop"] as const) {
  test(`${platform} collapsed sidebar keeps common icons, overflow navigation and restored width`, async ({ page: webPage }, info) => {
    test.setTimeout(180_000);
    test.skip(!apiUrl || !databaseUrl || platform === "Desktop" && !process.env.COLLABORATION_ELECTRON_RENDERER_URL,
      "Requires the task-owned loopback API/database and production clients");
    for (const value of [apiUrl!, databaseUrl!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
    const fixture = await seed();
    const profile = platform === "Desktop" ? mkdtempSync(join(tmpdir(), "multica-sidebar-rail-")) : null;
    let native: Native | undefined;
    let page = webPage;
    const errors: string[] = [];
    const geometry: Record<string, unknown> = {};
    let coarseIcons: IconGeometry[] = [];
    try {
      if (profile) {
        native = await openDesktop(profile, { token: fixture.api.getToken()! });
        page = native.page;
      }
      page.setDefaultTimeout(15_000);
      page.on("pageerror", (error) => errors.push(error.message));
      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.addInitScript(({ token, key }) => {
        localStorage.setItem("multica_token", token);
        localStorage.setItem("multica:chat:isOpen", "false");
        if (localStorage.getItem("sidebar-rail-test-seed") !== key) {
          localStorage.setItem("sidebar-rail-test-seed", key);
          localStorage.setItem("multica-locale", "en");
          localStorage.setItem("sidebar_width", "300");
          localStorage.setItem("theme", "light");
        }
      }, { token: fixture.api.getToken()!, key: fixture.key });
      const sourcePath = `/${fixture.second.slug}/issues/${fixture.source.id}`;
      if (native) {
        await native.start();
        await resize(page, native, 1440, 960);
        await expect(navigation(page)).toBeVisible();
        await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), sourcePath);
      } else {
        await resize(page, native, 1440, 960);
        await page.goto(sourcePath);
      }
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await expectWidth(page, 300);
      await expect(toggle(page)).toHaveCount(1);
      await toggle(page).click();
      geometry.common = await expectCommonEntries(page, fixture.second);
      if (native) geometry.native = await nativeToolbarGeometry(page);
      const nav = navigation(page);
      const more = nav.getByRole("button", { name: copy.en.more, exact: true });
      const notifications = nav.getByRole("link", { name: copy.en.notifications, exact: true });
      await expect(notifications.getByLabel("3", { exact: true })).toBeVisible();
      await expect(nav.getByRole("link", { name: copy.en.issues, exact: true })).toHaveAttribute("aria-current", "location");
      await capture(page, native, info, `${platform.toLowerCase()}-rail-light`);

      // Real Tab traversal establishes focus-visible before expecting Base UI's tooltip.
      const chat = nav.getByRole("link", { name: copy.en.chat, exact: true });
      await chat.focus();
      await page.keyboard.press("Shift+Tab");
      await page.keyboard.press("Tab");
      await expect(chat).toBeFocused();
      const chatTooltip = page.locator('[data-slot="tooltip-content"][data-open]')
        .filter({ hasText: `${copy.en.chat} · ${fixture.second.name}` });
      await expect(chatTooltip).toBeVisible();
      await expect(chatTooltip).toHaveText(`${copy.en.chat} · ${fixture.second.name}`);
      await chat.click();
      await expect(page.getByRole("heading", { name: "AI Chat", exact: true })).toBeVisible();
      await expect(chat).toHaveAttribute("aria-current", "page");
      await expectWidth(page, 64);

      // Common entries are reachable even though the full AI group starts closed.
      const agents = nav.getByRole("link", { name: copy.en.agents, exact: true });
      await agents.click();
      await expect(agents).toHaveAttribute("aria-current", "page");
      await expect(nav.getByRole("link", { name: copy.en.projects, exact: true })).toBeVisible();
      await more.focus();
      await page.keyboard.press("Enter");
      const resources = page.getByRole("menuitem", { name: copy.en.resources, exact: true });
      await expect(resources).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(more).toBeFocused();
      await more.click();
      await resources.hover();
      const mcp = page.getByRole("menuitem", { name: "MCP", exact: true });
      await expect(mcp).toBeVisible();
      await capture(page, native, info, `${platform.toLowerCase()}-rail-more`);
      await mcp.click();
      await expect(page.getByRole("menu")).toHaveCount(0);
      await expect(more).toHaveAttribute("aria-current", "location");
      await expectWidth(page, 64);
      if (!native) await expect(page).toHaveURL(`/${fixture.second.slug}/mcp`);

      // Disabled workspace features do not acquire a shortcut through More.
      expect(await fixture.api.requestJSON(`/api/workspaces/${fixture.second.id}/iteration-capabilities`))
        .toMatchObject({ supported: true, manual: true, enabled: false });
      expect(await fixture.api.requestJSON("/api/triage/settings")).toMatchObject({ enabled: false });
      await more.click();
      await expect(page.getByRole("menuitem", { name: copy.en.resources, exact: true })).toBeVisible();
      const work = page.getByRole("menuitem", { name: copy.en.work, exact: true });
      await expect(work).toHaveCount(0);
      await expect(page.getByRole("menuitem", { name: copy.en.iterations, exact: true })).toHaveCount(0);
      await expect(page.getByRole("menuitem", { name: copy.en.triage, exact: true })).toHaveCount(0);
      await page.keyboard.press("Escape");

      await setTheme(page, "dark");
      await expectCommonEntries(page, fixture.second);
      await capture(page, native, info, `${platform.toLowerCase()}-rail-dark`);
      await setTheme(page, "light");
      await notifications.click();
      await expect(page.getByRole("heading", { name: copy.en.notifications, exact: true })).toBeVisible();
      if (!native) await collapseAfterWebScopeChange(page);
      await expect(notifications).toHaveAttribute("aria-current", "page");
      await expect(notifications.getByLabel("3", { exact: true })).toBeVisible();
      await expectWidth(page, 64);

      // Creating from account scope keeps the original workspace handoff.
      const ordinary = page.getByRole("dialog", { name: /^(New Issue|Quick create issue)$/ });
      await nav.getByRole("button", { name: copy.en.create, exact: true }).click();
      await expect(ordinary).toHaveCount(1);
      await expect(ordinary).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(ordinary).toHaveCount(0);
      if (!native) await collapseAfterWebScopeChange(page);
      await expect(nav.getByRole("link", { name: copy.en.issues, exact: true })).toHaveAttribute("aria-current", "page");
      await expectWidth(page, 64);

      await more.click();
      await page.getByRole("menuitem", { name: copy.en.collaboration, exact: true }).click();
      const collaboration = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
      await expect(collaboration).toBeVisible();
      await expect(collaboration.getByRole("combobox", { name: "Source workspace", exact: true })).toHaveValue(fixture.second.id);
      const responsiveDraft = `Keep collaboration draft ${fixture.key}`;
      await collaboration.getByRole("textbox", { name: "Title", exact: true }).fill(responsiveDraft);
      await resize(page, native, native ? 900 : 390, native ? 1000 : 844, native ? 2 : 1);
      await expect(collaboration).toBeVisible();
      await expect(collaboration.getByRole("textbox", { name: "Title", exact: true })).toHaveValue(responsiveDraft);
      await expect(collaboration.getByRole("combobox", { name: "Source workspace", exact: true })).toHaveValue(fixture.second.id);
      await resize(page, native, 1440, 960);
      await expect(collaboration.getByRole("textbox", { name: "Title", exact: true })).toHaveValue(responsiveDraft);
      await collaboration.getByRole("button", { name: "Close", exact: true }).click();
      await expect(collaboration).toBeHidden();
      await expect(more).toBeFocused();

      // Selecting a workspace uses the same directory, without expanding the rail.
      await nav.getByRole("button", { name: copy.en.switchWorkspace, exact: true }).click();
      await page.getByRole("menuitem").filter({ hasText: fixture.first.name }).click();
      await expect(nav.getByRole("link", { name: copy.en.issues, exact: true })).toHaveAttribute("href", new RegExp(`/${fixture.first.slug}/issues$`));
      if (!native) await collapseAfterWebScopeChange(page);
      await expectWidth(page, 64);
      await nav.getByRole("button", { name: copy.en.switchWorkspace, exact: true }).click();
      await page.getByRole("menuitem").filter({ hasText: fixture.second.name }).click();
      await expect(nav.getByRole("link", { name: copy.en.issues, exact: true })).toHaveAttribute("href", new RegExp(`/${fixture.second.slug}/issues$`));
      if (!native) await collapseAfterWebScopeChange(page);
      await toggle(page).click();
      await expectWidth(page, 300);
      expect(await page.evaluate(() => localStorage.getItem("sidebar_width"))).toBe("300");

      // Auto-collapse preserves its parked expanded state and permits manual reopening.
      await resize(page, native, 1100, 960);
      await expect(rail(page)).toBeVisible();
      await expectWidth(page, 64);
      await toggle(page).click();
      await expectWidth(page, 300);
      await resize(page, native, 1440, 960);
      await expectWidth(page, 300);
      await toggle(page).click();
      await expectWidth(page, 64);

      // The original large-screen rail must stay usable with 44px touch
      // targets and a short window, including its scrolling body and footer.
      await resize(page, native, 1100, 650);
      const cdp = await page.context().newCDPSession(page);
      try {
        await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 });
        await expect.poll(() => page.evaluate(() => matchMedia("(any-pointer: coarse)").matches)).toBe(true);
        await expectWidth(page, 64);
        const coarse = await coarseRailGeometry(page);
        geometry.coarse = coarse;
        coarseIcons = coarse.icons;
        await info.attach("rail-coarse-geometry.json", { body: JSON.stringify({ platform, ...coarse }), contentType: "application/json" });
        await capture(page, native, info, `${platform.toLowerCase()}-rail-coarse-short`);
        const coarseMore = navigation(page).getByRole("button", { name: copy.en.more, exact: true });
        await coarseMore.scrollIntoViewIfNeeded();
        await coarseMore.click();
        await expect(page.getByRole("menuitem", { name: copy.en.resources, exact: true })).toBeVisible();
        await toggle(page).focus();
        await toggle(page).press("Enter");
        await expectWidth(page, 300);
        await expect(page.getByRole("menu")).toHaveCount(0);
        await expect(page.locator('[data-sidebar-more="navigation"]')).toHaveCount(0);
        await toggle(page).click();
        await expectWidth(page, 64);
      } finally {
        await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
        await cdp.detach();
      }
      await resize(page, native, 1440, 960);
      await expectWidth(page, 64);

      // A compact Sheet must show complete text/action navigation even with
      // the large-screen state parked collapsed. Native uses actual 200% zoom.
      await resize(page, native, native ? 900 : 390, native ? 1000 : 844, native ? 2 : 1);
      const compact = await openSidebarNavigation(page);
      await expect(compact.getByRole("group", { name: copy.en.startWork, exact: true })).toBeVisible();
      await expect(compact.getByRole("button", { name: copy.en.create, exact: true }).getByText(fixture.second.name, { exact: true })).toBeVisible();
      await expect(compact.getByRole("button", { name: copy.en.collaboration, exact: true })).toBeVisible();
      await expect(compact.getByRole("button", { name: copy.en.more, exact: true })).toHaveCount(0);
      const compactCollaboration = compact.getByRole("button", { name: copy.en.collaboration, exact: true });
      await compactCollaboration.click();
      await expect(collaboration).toBeVisible();
      await expect(page.locator('[data-slot="sidebar"][data-mobile="true"]')).toBeHidden();
      await collaboration.getByRole("textbox", { name: "Title", exact: true }).fill(`Compact draft ${fixture.key}`);
      await page.keyboard.press("Escape");
      await expect(collaboration).toBeHidden();
      await expect(compact).toBeVisible();
      await expect(compactCollaboration).toBeFocused();
      const expand = compact.getByRole("button", { name: "Expand collaboration navigation", exact: true });
      await expand.click();
      await capture(page, native, info, `${platform.toLowerCase()}-rail-compact-full`);
      await compact.getByRole("link", { name: copy.en.current, exact: true }).click();
      await expect(page.locator('[data-slot="sidebar"][data-mobile="true"]')).toBeHidden();
      await expect(page.getByRole("heading", { name: copy.en.current, exact: true })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

      await fixture.api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
      await page.addInitScript(() => {
        localStorage.setItem("multica-locale", "zh-Hans");
        document.cookie = "multica-locale=zh-Hans; path=/; SameSite=Lax";
      });
      await resize(page, native, 1440, 960);
      if (native) {
        await page.reload();
        await expect(navigation(page, "zh-Hans")).toBeVisible();
        await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), sourcePath);
      } else await page.goto(sourcePath);
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await expectWidth(page, 300);
      await toggle(page).click();
      geometry.chinese = await expectCommonEntries(page, fixture.second, "zh-Hans");
      await capture(page, native, info, `${platform.toLowerCase()}-rail-zh`);
      await info.attach("rail-geometry.json", { body: JSON.stringify({ platform, ...geometry }), contentType: "application/json" });
      expect(coarseIcons.length).toBeGreaterThan(0);
      expect(coarseIcons.filter((icon) => Math.abs(icon.dx) > 1 || Math.abs(icon.dy) > 1
        || Math.abs(icon.iconWidth - 16) > 1 || Math.abs(icon.iconHeight - 16) > 1),
      "Coarse 44px targets must retain centered 16px navigation icons").toEqual([]);
      expect(errors).toEqual([]);
      if (native) expect(native.errors).toEqual([]);
    } catch (error) {
      // Retain the actual failed window before profile and fixture teardown.
      // Evidence failures must never replace the original product assertion.
      await info.attach("rail-original-error", { body: String(error), contentType: "text/plain" }).catch(() => undefined);
      await info.attach("rail-page-errors", { body: JSON.stringify({ errors, nativeErrors: native?.errors ?? [] }), contentType: "application/json" }).catch(() => undefined);
      await info.attach("rail-failure-state", {
        body: `${page.url()}\n${await page.locator("body").innerText().catch(() => "Window body unavailable")}`,
        contentType: "text/plain",
      }).catch(() => undefined);
      const failurePath = info.outputPath(`${platform.toLowerCase()}-rail-before-cleanup.png`);
      await (native ? nativeScreenshot(native.app, page, failurePath) : page.screenshot({ path: failurePath, fullPage: false }))
        .then(() => info.attach("rail-before-cleanup", { path: failurePath, contentType: "image/png" }))
        .catch(() => undefined);
      throw error;
    } finally {
      await native?.app.close();
      if (profile) rmSync(profile, { recursive: true, force: true });
      await removeRetainedFacts(fixture);
      for (const workspace of [fixture.second, fixture.first]) {
        fixture.api.setWorkspaceId(workspace.id);
        fixture.api.setWorkspaceSlug(workspace.slug);
        await fixture.api.deleteFeatureWorkspace(workspace.id);
      }
      if (fixture.passwordUsername) await fixture.api.deletePasswordAccount(fixture.passwordUsername);
    }
  });
}
