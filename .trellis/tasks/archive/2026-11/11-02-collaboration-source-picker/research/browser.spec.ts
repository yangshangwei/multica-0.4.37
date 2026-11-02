import { test, expect, type Page, type Locator } from "@playwright/test";
import { randomUUID, createHash } from "node:crypto";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import pg from "pg";
import { TestApiClient } from "../../../../e2e/fixtures";

const allocationPath = "/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-source-picker-1qnib9_x/allocation.json";
const evidence = fileURLToPath(new URL("./browser-evidence/", import.meta.url));
const phase = process.env.SOURCE_PICKER_BROWSER_PHASE ?? "candidate";
const ownedDatabase = "collab_source_browser_65139858fd";
const apiOrigin = "http://localhost:51953";
const webOrigin = "http://localhost:51954";
type Locale = "en" | "zh-Hans";
type SeedIssue = { id: string; identifier: string; title: string; number: number };

function copy(locale: Locale) {
  return JSON.parse(readFileSync(resolve(process.cwd(), `packages/views/locales/${locale}/collaboration.json`), "utf8")) as {
    creation: { entry: string; title: string; titleLabel: string; description: string; source: string; submit: string };
    fields: { initial: string; parent: string; findParent: string; noParent: string; chooseParent: string; parentHint: string;
      parentsLoading: string; parentsUnavailable: string; retryParents: string; noMatchingParents: string };
  };
}

async function sql<T extends pg.QueryResultRow>(query: string, values: unknown[] = []): Promise<T[]> {
  const client = new pg.Client({ connectionString: process.env.DATABASE_URL, connectionTimeoutMillis: 5000 });
  await client.connect();
  try { return (await client.query<T>(query, values)).rows; } finally { await client.end(); }
}

async function verifyServices() {
  const allocation = JSON.parse(readFileSync(allocationPath, "utf8"));
  expect(process.env.TASK_DATABASE_NAME).toBe(ownedDatabase);
  expect(process.env.NEXT_PUBLIC_API_URL).toBe(apiOrigin);
  expect(process.env.PLAYWRIGHT_BASE_URL).toBe(webOrigin);
  expect((await sql<{ name: string }>("SELECT current_database() AS name"))[0]?.name).toBe(ownedDatabase);
  const healthResponse = await fetch(`${apiOrigin}/health`);
  expect(healthResponse.status).toBe(200);
  const health = await healthResponse.json() as { status: string; pid: number; commit: string };
  expect(health.status).toBe("ok");
  expect(health.pid).toBe(allocation.api_pid);
  expect((await fetch(`${webOrigin}/login`)).status).toBe(200);
  const binary = phase === "candidate" ? allocation.candidate_api_binary : allocation.api_binary;
  const binarySha256 = createHash("sha256").update(readFileSync(binary)).digest("hex");
  if (phase === "candidate") {
    expect(binarySha256).toBe(allocation.candidate_api_sha256);
    expect(health.commit).toBe(allocation.base_commit);
    expect(Object.keys(allocation.candidate_source_sha256)).toHaveLength(13);
    expect(readFileSync(resolve(allocation.snapshot, "apps/web/.next/BUILD_ID"), "utf8").trim()).toBe(allocation.web_build_id);
  }
  return { health, base_commit: allocation.base_commit,
    api_binary: binary, api_binary_sha256: binarySha256,
    web_pid: allocation.web_pid, web_build_id: allocation.web_build_id ?? null,
    source_fingerprints: phase === "candidate" ? allocation.candidate_source_sha256 : null };
}

async function signInPage(page: Page, token: string, locale: Locale) {
  await page.context().addCookies([
    { name: "multica-locale", value: locale, url: webOrigin },
    { name: "multica_logged_in", value: "1", url: webOrigin },
  ]);
  await page.addInitScript(({ token: authToken, locale: language }) => {
    localStorage.setItem("multica_token", authToken);
    localStorage.setItem("multica-locale", language);
    localStorage.setItem("multica:chat:isOpen", "false");
  }, { token, locale });
}

async function capture(page: Page, target: Locator, locale: Locale, width: "wide" | "narrow") {
  const viewport = width === "wide" ? { width: 1280, height: 950 } : { width: 390, height: 844 };
  await page.setViewportSize(viewport);
  await target.scrollIntoViewIfNeeded();
  await expect(target).toBeVisible();
  const dimensions = await page.evaluate(() => ({ width: innerWidth, height: innerHeight, scrollWidth: document.documentElement.scrollWidth }));
  expect(dimensions).toMatchObject(viewport);
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(viewport.width);
  const filename = `${phase}-${locale}-${width}.png`;
  const png = await page.screenshot({ path: resolve(evidence, filename), fullPage: false, scale: "css" });
  expect(png.readUInt32BE(16)).toBe(viewport.width);
  expect(png.readUInt32BE(20)).toBe(viewport.height);
  return { filename, ...dimensions };
}

function row(page: Page, issue: SeedIssue) {
  return page.getByRole("button").and(page.locator("[data-picker-item]")).filter({ hasText: issue.identifier }).filter({ hasText: issue.title });
}

for (const locale of ["en", "zh-Hans"] as const) {
  test(`${phase}: source parent picker in ${locale}`, async ({ page }) => {
    mkdirSync(evidence, { recursive: true });
    const services = await verifyServices();
    const labels = copy(locale);
    const suffix = randomUUID().slice(0, 8);
    const actor = new TestApiClient();
    await actor.login(`source-picker-${phase}-${suffix}@example.invalid`, "Source picker verification");
    const workspace = await actor.ensureWorkspace("Source picker verification", `source-picker-${suffix}`);
    await actor.markUserOnboarded();
    const alpha = await actor.createIssue(`Alpha source planning ${suffix}`) as SeedIssue;
    const beta = await actor.createIssue(`Beta source task ${suffix} — 这是一项用于检查长标题换行的来源任务，包含较长的说明和可辨识的任务内容。`.repeat(2)) as SeedIssue;
    expect(alpha.id).toMatch(/^[a-f0-9-]{36}$/i);
    expect(alpha.identifier).toContain("-");
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    let creationPosts = 0;
    page.on("request", request => {
      if (request.method() === "POST" && new URL(request.url()).pathname === "/api/collaboration/issues") creationPosts++;
    });
    await signInPage(page, actor.getToken()!, locale);
    await page.goto(`/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: labels.creation.entry, exact: true }).click();
    const dialog = page.getByRole("dialog", { name: labels.creation.title, exact: true });
    const title = dialog.getByRole("textbox", { name: labels.creation.titleLabel, exact: true });
    const description = dialog.getByRole("textbox", { name: labels.creation.description, exact: true });
    await expect(title).toBeEnabled();
    const preservedTitle = `Created from an existing source ${suffix}`;
    const preservedBody = "This description survives source task searches and retry.";
    await title.fill(preservedTitle);
    await description.fill(preservedBody);
    await dialog.locator("summary").filter({ hasText: labels.fields.initial }).click();
    const screenshots: unknown[] = [];
    if (phase === "baseline") {
      const oldSearch = dialog.getByRole("textbox", { name: labels.fields.findParent, exact: true });
      const oldSelect = dialog.getByRole("combobox", { name: labels.fields.parent, exact: true });
      await expect(oldSearch).toBeVisible();
      await expect(oldSelect.locator("option").filter({ hasText: alpha.title })).toHaveCount(1);
      screenshots.push(await capture(page, oldSearch, locale, "wide"));
      screenshots.push(await capture(page, oldSearch, locale, "narrow"));
      const responseWait = page.waitForResponse(response => new URL(response.url()).searchParams.get("parent_search") === alpha.identifier);
      await oldSearch.fill(alpha.identifier);
      const response = await responseWait;
      expect(response.status()).toBe(200);
      const options = await response.json() as { fields: { source_options: { parents: unknown[] } } };
      expect(options.fields.source_options.parents).toEqual([]);
      await expect(oldSelect.locator("option").filter({ hasText: alpha.title })).toHaveCount(0);
      expect(creationPosts).toBe(0);
      expect(errors).toEqual([]);
      writeFileSync(resolve(evidence, `${phase}-${locale}-observations.json`), JSON.stringify({ phase, locale, services,
        workspace_id: workspace.id, fixtures: [alpha, beta], screenshots, full_identifier_matches: 0, creation_posts: creationPosts, page_errors: errors,
        interpretation: "Baseline observation only. The old disconnected controls and complete-identifier failure were reproduced; this is not repair acceptance." }, null, 2) + "\n");
      await actor.cleanup();
      return;
    }

    const trigger = dialog.getByRole("button", { name: labels.fields.parent, exact: true });
    await trigger.click();
    const search = page.getByPlaceholder(labels.fields.findParent, { exact: true });
    await expect(search).toBeVisible();
    await expect(search).toHaveValue("");
    await expect(row(page, alpha)).toBeVisible();
    await expect(row(page, beta)).toBeVisible();
    await expect(search).toHaveAttribute("maxlength", "200");
    screenshots.push(await capture(page, search, locale, "wide"));
    screenshots.push(await capture(page, search, locale, "narrow"));
    const popup = search.locator("xpath=ancestor::*[@data-slot='popover-content']");
    await expect(popup).toBeVisible();
    const popupBounds = await popup.boundingBox();
    expect(popupBounds).not.toBeNull();
    expect(popupBounds!.x).toBeGreaterThanOrEqual(0);
    expect(popupBounds!.x + popupBounds!.width).toBeLessThanOrEqual(390);
    await page.setViewportSize({ width: 1280, height: 950 });
    await search.fill("Alpha source planning");
    await expect(row(page, alpha)).toBeVisible();
    await expect(row(page, beta)).toHaveCount(0);
    await search.fill(String(alpha.number));
    await expect(row(page, alpha)).toBeVisible();
    await search.fill(alpha.identifier.toLowerCase());
    await expect(row(page, alpha)).toBeVisible();
    await expect(row(page, beta)).toHaveCount(0);
    await search.fill(`Missing task ${suffix}`);
    await expect(page.getByText(labels.fields.noMatchingParents, { exact: true })).toBeVisible();
    await search.fill("");
    await expect(row(page, alpha)).toBeVisible();
    await expect(row(page, beta)).toBeVisible();
    await search.press("Escape");
    await expect(search).toHaveCount(0);
    await expect(dialog).toBeVisible();
    await expect(trigger).toBeFocused();
    await trigger.press("Enter");
    await expect(search).toBeFocused();
    await expect(row(page, beta)).toBeVisible();
    await search.press("ArrowDown");
    await search.press("Enter");
    await expect(search).toHaveCount(0);
    await expect(trigger).toContainText(beta.title);
    await trigger.click();
    await page.getByRole("button", { name: labels.fields.noParent, exact: true }).click();
    await expect(trigger).toContainText(labels.fields.chooseParent);
    expect(creationPosts).toBe(0);

    await trigger.click();
    await expect(row(page, alpha)).toBeVisible();
    await row(page, alpha).click();
    await expect(trigger).toContainText(alpha.title);
    await trigger.click();
    let injectedFailure = false;
    const failedSearch = `Retry source search ${suffix}`;
    const handler = async (route: import("@playwright/test").Route) => {
      const url = new URL(route.request().url());
      if (!injectedFailure && url.searchParams.get("parent_search") === failedSearch) {
        injectedFailure = true;
        const response = await route.fetch();
        await route.fulfill({ response, status: 500, body: JSON.stringify({ code: "internal_error", message: "Task-only parent search fault" }) });
      } else await route.continue();
    };
    await page.route("**/api/collaboration/creation-options?**", handler);
    await search.fill(failedSearch);
    await expect(page.getByText(labels.fields.parentsUnavailable, { exact: true })).toBeVisible();
    await expect(title).toHaveValue(preservedTitle);
    await expect(description).toHaveValue(preservedBody);
    await expect(trigger).toContainText(alpha.title);
    await expect(search).toHaveValue(failedSearch);
    await expect(row(page, alpha)).toHaveCount(0);
    await page.getByRole("button", { name: labels.fields.retryParents, exact: true }).click();
    await expect(page.getByText(labels.fields.noMatchingParents, { exact: true })).toBeVisible();
    await expect(search).toBeFocused();
    await page.unroute("**/api/collaboration/creation-options?**", handler);
    await search.fill(alpha.identifier);
    await expect(row(page, alpha)).toBeVisible();
    await row(page, alpha).click();
    await expect(search).toHaveCount(0);
    await expect(trigger).toContainText(alpha.title);
    await trigger.click();
    await expect(search).toHaveValue("");
    await expect(row(page, beta)).toBeVisible();
    await search.press("Escape");
    expect(creationPosts).toBe(0);
    const creationResponse = page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/api/collaboration/issues");
    await dialog.getByRole("button", { name: labels.creation.submit, exact: true }).click();
    const response = await creationResponse;
    expect(response.status()).toBe(201);
    expect(response.request().postDataJSON()).toMatchObject({ title: preservedTitle, description: preservedBody, parent_issue_id: alpha.id });
    const created = await response.json() as { id: string };
    expect((await sql<{ parent_issue_id: string }>("SELECT parent_issue_id::text FROM issue WHERE id = $1", [created.id]))[0]?.parent_issue_id).toBe(alpha.id);
    await expect(page).toHaveURL(url => url.pathname === "/account" && url.searchParams.get("issue") === created.id.toLowerCase());
    expect(creationPosts).toBe(1);
    expect(errors).toEqual([]);
    writeFileSync(resolve(evidence, `${phase}-${locale}-observations.json`), JSON.stringify({ phase, locale, services,
      workspace_id: workspace.id, fixtures: [alpha, beta], created_issue_id: created.id, saved_parent_issue_id: alpha.id, screenshots,
      creation_posts: creationPosts, page_errors: errors, injected_transient_search_failure: injectedFailure,
      coverage: ["empty click browsing", "title search", "bare number search", "complete case-insensitive identifier", "no matches", "clear/reopen", "keyboard selection", "clear parent", "nested Escape/focus", "transient error/retry", "preserved draft", "explicit create and saved parent UUID"],
      limitations: ["Chromium Web only; no physical touch or native Electron session", "delayed-obsolete result matrix stays in canonical shared component tests", "fixture rows retained in the task-owned browser database for evidence"] }, null, 2) + "\n");
  });
}
