import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import pg from "pg";
import { expect, test, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { deepLink, nativeScreenshot, openDesktop } from "./fixtures/collaboration-native";

const apiUrl = process.env.COLLABORATION_E2E_API_URL;
const databaseUrl = process.env.DATABASE_URL;
type Workspace = { id: string; slug: string; name: string };
type Issue = { id: string; identifier: string; title: string };

async function seed() {
  const api = new TestApiClient();
  const key = randomUUID().slice(0, 8);
  await api.login(`scope-${key}@example.invalid`, "Navigation viewer");
  const first = await api.ensureWorkspace("First navigation space", `scope-first-${key}`);
  const second = await api.requestJSON<Workspace>("/api/workspaces", { method: "POST", body: { name: "Second navigation space", slug: `scope-second-${key}` } });
  await api.markUserOnboarded();
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
  return { api, first, second, source, issues, key };
}

const globalNav = (page: Page) => page.getByRole("navigation", { name: "Application navigation", exact: true });

for (const platform of ["Web", "Desktop"] as const) {
  test(`${platform} global switching resumes the task, collaboration filters and both scroll positions`, async ({ page: webPage }, testInfo) => {
    test.setTimeout(180_000);
    test.skip(!apiUrl || !databaseUrl || (platform === "Desktop" && !process.env.COLLABORATION_ELECTRON_RENDERER_URL), "Requires the task-owned API/database and matching production clients");
    for (const value of [apiUrl!, databaseUrl!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
    const fixture = await seed();
    const profile = platform === "Desktop" ? mkdtempSync(join(tmpdir(), "multica-scope-navigation-")) : null;
    let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
    let page = webPage;
    try {
      const sourcePath = `/${fixture.second.slug}/issues/${fixture.source.id}`;
      if (profile) {
        native = await openDesktop(profile, { token: fixture.api.getToken()! });
        page = native.page;
        await native.start();
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
      const originalBounds = await globalNav(page).boundingBox();
      if (native) await nativeScreenshot(native.app, page, testInfo.outputPath("desktop-scope-workspace.png"));
      else await page.screenshot({ path: testInfo.outputPath("web-scope-workspace.png"), fullPage: false });
      await globalNav(page).getByRole("button", { name: "Collaboration", exact: true }).click();
      await page.getByRole("link", { name: "Participating", exact: true }).click();
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
      await globalNav(page).getByRole("button", { name: "Workspaces", exact: true }).click();
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await globalNav(page).getByRole("button", { name: "Collaboration", exact: true }).click();
      await expect(page.getByRole("heading", { name: selected.title, exact: true })).toBeVisible();
      await expect(search).toHaveValue("Resume collaboration");
      await expect(page.getByRole("combobox", { name: "Sort" })).toHaveValue("title");
      await expect.poll(() => list.evaluate((element) => element.scrollTop)).toBe(saved.list);
      await expect.poll(() => detail.evaluate((element) => element.scrollTop)).toBe(saved.detail);
      await globalNav(page).getByRole("button", { name: "Collaboration", exact: true }).click();
      expect(await detail.evaluate((element) => element.scrollTop)).toBe(saved.detail);
      const resumedBounds = await globalNav(page).boundingBox();
      expect(resumedBounds?.x).toBe(originalBounds?.x);
      expect(resumedBounds?.y).toBe(originalBounds?.y);
      const capture = async (name: string) => native ? nativeScreenshot(native.app, page, testInfo.outputPath(name)) : page.screenshot({ path: testInfo.outputPath(name), fullPage: false });
      await capture(`${platform.toLowerCase()}-scope-resumed.png`);

      const other = fixture.issues[1]!;
      const explicit = `/account?view=participating&issue=${other.id}#description`;
      if (native) await deepLink(native.app, explicit);
      else await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), explicit);
      await expect(page.getByRole("heading", { name: other.title, exact: true })).toBeVisible();
      await expect(page.locator("#description")).toBeFocused();
      await detail.evaluate((element) => { element.scrollTop = 800; });
      await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      await globalNav(page).getByRole("button", { name: "Workspaces", exact: true }).click();
      await expect(page.getByText(fixture.source.title, { exact: true }).last()).toBeVisible();
      await globalNav(page).getByRole("button", { name: "Collaboration", exact: true }).click();
      await expect.poll(() => detail.evaluate((element) => element.scrollTop)).toBe(800);

      if (native) {
        const window = await native.app.browserWindow(page);
        await window.evaluate((target) => { target.setContentSize(900, 1000); target.webContents.setZoomFactor(2); });
        await window.dispose();
      } else await page.setViewportSize({ width: 390, height: 844 });
      await expect(globalNav(page).getByRole("button", { name: "Workspaces", exact: true })).toBeVisible();
      await expect(globalNav(page).getByRole("button", { name: "Collaboration", exact: true })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await capture(`${platform.toLowerCase()}-scope-compact.png`);
      await fixture.api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
      await page.addInitScript(() => {
        localStorage.setItem("multica-locale", "zh-Hans");
        document.cookie = "multica-locale=zh-Hans; path=/; SameSite=Lax";
      });
      await page.reload();
      const chineseNavigation = page.getByRole("navigation", { name: "应用导航", exact: true });
      await expect(chineseNavigation.getByRole("button", { name: "工作空间", exact: true })).toBeVisible();
      await expect(chineseNavigation.getByRole("button", { name: "协作", exact: true })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await capture(`${platform.toLowerCase()}-scope-compact-zh.png`);
      await testInfo.attach("scope-restoration.json", { body: JSON.stringify({ platform, source: fixture.source.id, selected: selected.id, saved, originalBounds, resumedBounds }), contentType: "application/json" });
      if (native) expect(native.errors).toEqual([]);
    } finally {
      await native?.app.close();
      if (profile) rmSync(profile, { recursive: true, force: true });
      // These read-only protocol rows were seeded directly for this fixture.
      // Remove only those exact rows before normal workspace cleanup; the API
      // intentionally refuses deleting any workspace with retained collaboration.
      const db = new pg.Client(databaseUrl);
      await db.connect();
      try {
        await db.query("BEGIN");
        const ids = fixture.issues.map((issue) => issue.id);
        await db.query("DELETE FROM issue_participant WHERE issue_id = ANY($1::uuid[])", [ids]);
        await db.query("DELETE FROM collaboration_session WHERE issue_id = ANY($1::uuid[])", [ids]);
        await db.query("DELETE FROM issue_collaboration WHERE issue_id = ANY($1::uuid[])", [ids]);
        await db.query("COMMIT");
      } catch (error) {
        await db.query("ROLLBACK");
        throw error;
      } finally { await db.end(); }
      for (const workspace of [fixture.second, fixture.first]) {
        fixture.api.setWorkspaceId(workspace.id);
        fixture.api.setWorkspaceSlug(workspace.slug);
        await fixture.api.deleteFeatureWorkspace(workspace.id);
      }
    }
  });
}
