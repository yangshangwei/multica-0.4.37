import { test, expect, type BrowserContext, type Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { TestApiClient } from "./fixtures";

const root = resolve(import.meta.dirname, "..");
const fixtureApi = process.env.CHANGELOG_E2E_API_URL;
const fixtureFeed = process.env.CHANGELOG_E2E_FEED;
const artifactDir = resolve(root, ".omx/state/desktop-changelog");

// This relay sends requests to the actual isolated Go server. It changes no
// response data and lets acceptance reuse the running web renderer without
// restarting another task's development environment.
async function useLocalApi(context: BrowserContext) {
  if (!fixtureApi) return;
  const target = new URL(fixtureApi);
  if (!["localhost", "127.0.0.1"].includes(target.hostname)) {
    throw new Error("Changelog acceptance requires a local API");
  }
  await context.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (!["localhost", "127.0.0.1"].includes(url.hostname)) {
      await route.abort();
    } else if (url.pathname.startsWith("/api/") || url.pathname.startsWith("/auth/")) {
      const response = await route.fetch({ url: `${target.origin}${url.pathname}${url.search}` });
      await route.fulfill({ response });
    } else {
      await route.continue();
    }
  });
  // These checks exercise the HTTP release feed. Do not send test identity to
  // the renderer's separately configured development WebSocket server.
  await context.routeWebSocket("**/ws**", (socket) => socket.close());
}

async function openAuthenticated(page: Page, api: TestApiClient, slug: string) {
  const token = api.getToken();
  if (!token) throw new Error("Missing test session");
  await page.context().addCookies([
    { name: "multica-locale", value: "zh-Hans", url: process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:3000" },
  ]);
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
  await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("button", { name: "帮助", exact: true })).toBeVisible({ timeout: 45_000 });
  await page.getByRole("button", { name: "帮助", exact: true }).click();
  await page.getByRole("menuitem", { name: "变更说明", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/${slug}/changelog$`), { timeout: 30_000 });
  await expect(page.getByRole("heading", { name: "变更说明", exact: true })).toBeVisible();
}

test("Help opens this deployment's attributed changelog", async ({ page }) => {
  await useLocalApi(page.context());
  const api = new TestApiClient();
  await api.login(`changelog-read-${Date.now()}@example.invalid`, "Changelog acceptance");
  const workspace = await api.ensureWorkspace("Changelog acceptance", `changelog-read-${Date.now()}`);
  await api.markUserOnboarded();
  try {
    await openAuthenticated(page, api, workspace.slug);
    await expect(page.getByRole("navigation", { name: "浏览版本" })).toBeVisible();
    await expect(page.getByText("当前分支尚未发布版本", { exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "内网协作、团队模板与中文体验" })).toBeVisible();
    await expect(page.getByText("以下变更已提交到代码库，尚未正式发布。", { exact: true })).toBeVisible();
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.mouse.move(800, 50);
    await page.screenshot({ path: join(artifactDir, "reader-desktop.png") });
    await page.emulateMedia({ colorScheme: "dark" });
    await expect(page.locator("html")).toHaveClass(/dark/);
    await page.screenshot({ path: join(artifactDir, "reader-dark.png") });
    await page.emulateMedia({ colorScheme: "light" });
    await expect(page.locator("html")).not.toHaveClass(/dark/);

    await page.getByRole("navigation", { name: "浏览版本" }).getByRole("link", { name: /v0\.4\.37/ }).click();
    const historical = page.getByRole("heading", { name: "任务列表更快，长时间执行更稳定" });
    await expect(historical).toBeFocused();
    await expect(page.getByText("摘自 Multica 官方变更说明，不代表当前分支已发布此版本。", { exact: true })).toBeVisible();
    await page.goto(`/${workspace.slug}/changelog?version=v999.0.0`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText("当前部署的历史中还没有此版本。", { exact: true })).toBeVisible({ timeout: 30_000 });
    await page.getByRole("link", { name: "全部版本", exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/changelog$`), { timeout: 30_000 });

    await page.setViewportSize({ width: 760, height: 980 });
    await page.screenshot({ path: join(artifactDir, "reader-narrow.png") });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await expect(page.getByRole("button", { name: "刷新", exact: true })).toBeVisible();
  } catch (error) {
    await page.screenshot({ path: join(artifactDir, "browser-error.png") });
    writeFileSync(join(artifactDir, "browser-error.txt"), `${page.url()}\n${await page.locator("body").innerText()}`);
    throw error;
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("generated publication reaches an open reader and a fresh client without restart", async ({ page, browser }, testInfo) => {
  test.skip(!fixtureApi || !fixtureFeed, "Requires a task-owned CHANGELOG_E2E_API_URL and CHANGELOG_E2E_FEED");
  test.setTimeout(150_000);
  if (!resolve(fixtureFeed!).startsWith(`${artifactDir}/live/`)) {
    throw new Error("Refusing to replace a feed outside the task-owned acceptance directory");
  }
  await useLocalApi(page.context());
  const previous = readFileSync(fixtureFeed!);
  const directory = mkdtempSync(join(tmpdir(), "multica-changelog-e2e-"));
  const repository = join(directory, "repository");
  const history = join(directory, "history.json");
  const first = join(directory, "first.json");
  const second = join(directory, "second.json");
  const initialNote = "初始验收记录：支持在应用内阅读版本更新。";
  const updateNote = "热更新验收记录：无需重新打开页面即可看到新发布。";
  const git = (...args: string[]) => execFileSync("git", args, { cwd: repository, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim();
  execFileSync("git", ["init", "--initial-branch=main", repository], { stdio: "ignore" });
  git("config", "user.name", "Changelog fixture");
  git("config", "user.email", "fixture@example.invalid");
  git("commit", "--allow-empty", "-m", "chore: fixture baseline");
  const base = git("rev-parse", "HEAD");
  writeFileSync(history, JSON.stringify({ schema_version: 1, generated_at: "2026-09-01T00:00:00Z", releases: [] }));
  const generate = (version: string, input: string, output: string, baseRef?: string) => {
    execFileSync(process.execPath, [
      join(root, "scripts/generate-changelog.mjs"), "--ref", version,
      ...(baseRef ? ["--base", baseRef] : []), "--history", input,
      "--repository", "fixture/multica", "--version", version, "--status", "published",
      "--published-at", version === "v0.0.1" ? "2026-09-01T01:00:00Z" : "2026-09-02T01:00:00Z",
      "--output", output, "--markdown", `${output}.md`,
    ], { cwd: repository, stdio: "pipe" });
  };
  const publish = (input: string) => execFileSync(process.execPath, [
    join(root, "scripts/publish-changelog.mjs"), "--input", input, "--destination", fixtureFeed!,
  ], { cwd: root, stdio: "pipe" });
  git("commit", "--allow-empty", "-m", "feat: initial changelog", "-m", `Release-note: ${initialNote}`);
  git("tag", "v0.0.1");
  generate("v0.0.1", history, first, base);
  publish(first);
  const api = new TestApiClient();
  await api.login(`changelog-live-${Date.now()}@example.invalid`, "Changelog live acceptance");
  const workspace = await api.ensureWorkspace("Changelog live acceptance", `changelog-live-${Date.now()}`);
  await api.markUserOnboarded();
  try {
    const initialHealth = await (await fetch(`${fixtureApi}/health`)).json();
    await openAuthenticated(page, api, workspace.slug);
    await expect(page.getByText(initialNote, { exact: true })).toBeVisible();
    await page.bringToFront();
    const timeOrigin = await page.evaluate(() => performance.timeOrigin);
    git("commit", "--allow-empty", "-m", "fix: publish while reader stays open", "-m", `Release-note: ${updateNote}`);
    git("tag", "v0.0.2");
    generate("v0.0.2", first, second);
    const publishedAt = Date.now();
    publish(second);
    await expect(page.getByText(updateNote, { exact: true })).toBeVisible({ timeout: 65_000 });
    const observedAfterMs = Date.now() - publishedAt;
    expect(await page.evaluate(() => performance.timeOrigin)).toBe(timeOrigin);
    const finalHealth = await (await fetch(`${fixtureApi}/health`)).json();
    expect(finalHealth.pid).toBe(initialHealth.pid);
    expect(finalHealth.started_at).toBe(initialHealth.started_at);
    await expect(page.getByText(initialNote, { exact: true })).toBeVisible();

    const fresh = await browser.newContext({ baseURL: process.env.PLAYWRIGHT_BASE_URL });
    try {
      await useLocalApi(fresh);
      const freshPage = await fresh.newPage();
      await openAuthenticated(freshPage, api, workspace.slug);
      await expect(freshPage.getByText(updateNote, { exact: true })).toBeVisible();
    } finally {
      await fresh.close();
    }

    writeFileSync(fixtureFeed!, "{invalid-json");
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    await expect(page.getByText("服务端暂时无法更新变更说明，当前显示最近可用的内容。", { exact: true })).toBeVisible();
    await expect(page.getByText(updateNote, { exact: true })).toBeVisible();
    publish(second);
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    await expect(page.getByText("服务端暂时无法更新变更说明，当前显示最近可用的内容。", { exact: true })).toHaveCount(0);

    await page.route("**/api/changelog", (route) => route.abort());
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    await expect(page.getByText("暂时无法刷新，仍显示此前加载的变更内容。", { exact: true })).toBeVisible();
    await expect(page.getByText(updateNote, { exact: true })).toBeVisible();
    await page.unroute("**/api/changelog");
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    await expect(page.getByText("暂时无法刷新，仍显示此前加载的变更内容。", { exact: true })).toHaveCount(0);
    const evidence = JSON.stringify({ observedAfterMs, serverPid: initialHealth.pid, unchangedTimeOrigin: timeOrigin, generatedVersions: ["v0.0.1", "v0.0.2"], externalNetworkBlocked: true }, null, 2);
    writeFileSync(join(artifactDir, "live-publication-evidence.json"), `${evidence}\n`);
    await testInfo.attach("live-publication-evidence", {
      body: evidence,
      contentType: "application/json",
    });
  } finally {
    const restore = join(directory, "restore.json");
    writeFileSync(restore, previous);
    publish(restore);
    await api.deleteFeatureWorkspace(workspace.id);
    rmSync(directory, { recursive: true, force: true });
  }
});
