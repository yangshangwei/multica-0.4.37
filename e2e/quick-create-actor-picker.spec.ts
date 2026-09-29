import { randomUUID } from "node:crypto";
import { cpus, platform, release, tmpdir } from "node:os";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { _electron as electron, expect, test, type ElectronApplication, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Store/model/component suites own the boundary matrices. These browser tests
// cover real API acceptance plus browser-only focus, layout and render timing.
// The runtime fixture has no process: no installed agent CLI is ever invoked.
const SEARCH = "Search names or responsibilities...";
const RUNTIME_ID = "81000000-0000-4000-8000-000000000001";
const actorId = (type: "agent" | "squad", index: number) =>
  `${type === "agent" ? "82000000" : "83000000"}-0000-4000-8000-${String(index).padStart(12, "0")}`;
const actorRow = (page: Page, type: "agent" | "squad", id: string) =>
  page.locator(`[data-actor-key="${type}:${id}"]`);
const items = (page: Page) => page.locator("[data-actor-key] button[data-picker-item]");

function evidencePath(info: TestInfo, name: string) {
  const directory = process.env.ACTOR_PICKER_EVIDENCE_DIR;
  if (!directory) return info.outputPath(name);
  mkdirSync(directory, { recursive: true });
  return join(directory, name);
}

async function setup(page: Page, info: TestInfo) {
  const api = new TestApiClient();
  const slug = `e2e-actor-picker-${randomUUID().slice(0, 8)}`;
  const session = await api.login(`${slug}@multica.ai`, "Actor picker verifier");
  const workspace = await api.ensureWorkspace("Actor picker verification", slug);
  expect(workspace.slug).toBe(slug);
  await api.markUserOnboarded();
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
  const token = api.getToken();
  const baseURL = info.project.use.baseURL;
  if (!token || !session.user?.id || typeof baseURL !== "string") throw new Error("Incomplete E2E session");
  await page.context().addCookies([
    { name: "multica-locale", value: "en", url: baseURL },
    { name: "multica_logged_in", value: "1", url: baseURL },
  ]);
  await page.addInitScript((authToken) => {
    localStorage.setItem("multica_token", authToken);
    localStorage.setItem("multica:chat:isOpen", "false");
    localStorage.setItem("multica_create_mode", JSON.stringify({ state: { lastMode: "agent" }, version: 0 }));
    if (!localStorage.getItem("theme")) localStorage.setItem("theme", "light");
  }, token);
  return { api, workspace, userId: session.user.id as string };
}

async function openCreate(page: Page, slug: string) {
  await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "New Issue", exact: true }).first().click();
  await expect(page.getByRole("button", { name: /^Creation assistant/ })).toBeVisible();
}

async function openPicker(page: Page) {
  await page.getByRole("button", { name: /^Creation assistant/ }).click();
  await expect(page.getByPlaceholder(SEARCH, { exact: true })).toBeVisible();
}

async function mockDirectory(page: Page, workspaceId: string, userId: string, agentCount = 500, squadCount = 50) {
  const agents = Array.from({ length: agentCount }, (_, index) => ({
    id: actorId("agent", index), workspace_id: workspaceId, runtime_id: RUNTIME_ID,
    runtime_bound: true, runtime_availability: "online", name: `Agent ${String(index).padStart(3, "0")}`,
    description: `${"Saved responsibility details. ".repeat(16)}responsibility-${String(index).padStart(4, "0")}`,
    instructions: "private-system-instruction-not-searchable", avatar_url: null,
    runtime_mode: "local", runtime_config: {}, custom_args: [], visibility: "workspace",
    permission_mode: "private", status: "idle", max_concurrent_tasks: 1, model: "",
    owner_id: userId, skills: [], created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
    archived_at: null, archived_by: null,
  }));
  const squads = Array.from({ length: squadCount }, (_, index) => ({
    id: actorId("squad", index), workspace_id: workspaceId, name: `Squad ${String(index).padStart(3, "0")}`,
    description: `Saved squad responsibility squad-duty-${String(index).padStart(4, "0")}`,
    instructions: "private-squad-instruction", avatar_url: null, leader_id: agents[0]!.id,
    creator_id: userId, created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
    archived_at: null, archived_by: null, member_count: 1, agent_member_ids: [agents[0]!.id], member_preview: [],
  }));
  await page.route("**/api/agents?**", (route) => route.fulfill({ json: agents }));
  await page.route("**/api/squads", (route) => route.fulfill({ json: squads }));
  await page.route("**/api/runtimes?**", (route) => route.fulfill({ json: [{
    id: RUNTIME_ID, workspace_id: workspaceId, name: "No-process fixture", provider: "codex",
    runtime_mode: "local", owner_id: userId, status: "online", visibility: "private", daemon_id: null,
    metadata: { cli_version: "0.4.40", capabilities: ["rpc-v1"] }, launch_header: "", device_info: "Fixture only",
    last_seen_at: new Date().toISOString(), created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
  }] }));
  // Large fixtures only exercise the picker; fail loudly if they try to submit.
  await page.route("**/api/issues/quick-create", (route) => route.fulfill({ status: 409, json: { error: "Read-only directory fixture" } }));
  return { agents, squads };
}

test("real API actors keep favorites and accepted recents local to their workspace", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await setup(page, info);
  let other: { id: string; slug: string } | undefined;
  try {
    const runtime = await api.seedProjectRuntime();
    const agents: { id: string; name: string }[] = [];
    for (const name of ["Amber analyst", "Birch builder", "Cedar reviewer", "Delta researcher"]) {
      agents.push(await api.requestJSON("/api/agents", {
        method: "POST", body: { name, description: `${name}: saved responsibility.`, runtime_id: runtime.id, permission_mode: "private" },
      }));
    }
    const squad = await api.requestJSON<{ id: string; name: string }>("/api/squads", {
      method: "POST", body: { name: "Delivery squad", description: "Coordinate delivery, not the leader's individual role.", leader_id: agents[0]!.id },
    });
    await page.setViewportSize({ width: 1280, height: 900 });
    await openCreate(page, workspace.slug);
    const editor = page.getByRole("dialog", { name: "Quick create issue", exact: true }).locator('[contenteditable="true"]').first();
    await editor.fill("Preserve this draft while pinning roles.");
    const originalActor = await page.getByRole("button", { name: /^Creation assistant/ }).textContent();
    await openPicker(page);
    await expect(items(page)).toHaveCount(4);
    const coordination = page.getByRole("button", { name: "Planning and coordination (1)" });
    await expect(coordination).toHaveAttribute("aria-expanded", "false");
    await coordination.click();
    await expect(items(page)).toHaveCount(5);
    for (const name of [...agents.slice(0, 3).map((agent) => agent.name), squad.name]) {
      await page.getByPlaceholder(SEARCH).fill(name);
      await page.getByRole("button", { name: `Pin ${name} to favorites`, exact: true }).click();
    }
    await expect(page.getByPlaceholder(SEARCH)).toBeVisible();
    await expect(editor).toHaveText("Preserve this draft while pinning roles.");
    await expect(page.getByRole("button", { name: /^Creation assistant/ })).toHaveText(originalActor!);
    await page.getByRole("button", { name: /^Creation assistant/ }).click();
    await openPicker(page);
    await expect(items(page)).toHaveCount(3);
    await page.getByRole("button", { name: "View all favorites (4)", exact: true }).click();
    await expect(items(page)).toHaveCount(3);
    await page.getByRole("button", { name: "Planning and coordination (1)" }).click();
    await expect(items(page)).toHaveCount(4);
    await expect(actorRow(page, "squad", squad.id)).toContainText("Coordinate delivery");
    // Searching from favorites must reach the unpinned actor too.
    await page.getByPlaceholder(SEARCH).fill(agents[3]!.name);
    await actorRow(page, "agent", agents[3]!.id).locator("button[data-picker-item]").click();
    const accepted = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/issues/quick-create" && response.request().method() === "POST");
    await page.getByRole("button", { name: /^Create(?:\s|$)/ }).last().click();
    const response = await accepted;
    expect(response.status()).toBe(202);
    expect(response.request().postDataJSON()).toMatchObject({ agent_id: agents[3]!.id, prompt: "Preserve this draft while pinning roles." });
    expect(await response.json()).toMatchObject({ task_id: expect.any(String) });
    await expect(page.getByRole("button", { name: /^Creation assistant/ })).toHaveCount(0);
    await openCreate(page, workspace.slug);
    await openPicker(page);
    await expect(page.getByText("Recently used", { exact: true })).toBeVisible();
    await expect(items(page)).toHaveCount(4);
    await expect(actorRow(page, "agent", agents[3]!.id)).toBeVisible();
    await actorRow(page, "agent", agents[3]!.id).locator("button[data-picker-item]").hover();
    await page.screenshot({ path: evidencePath(info, "browser-actor-picker-1280.png"), animations: "disabled" });

    other = await api.requestJSON("/api/workspaces", { method: "POST", body: { name: "Empty preferences workspace", slug: `${workspace.slug}-empty` } });
    await openCreate(page, other.slug);
    await openPicker(page);
    await expect(page.getByText("Favorites", { exact: true })).toHaveCount(0);
    await expect(page.getByText("Recently used", { exact: true })).toHaveCount(0);
    await expect(items(page)).toHaveCount(0);
    await openCreate(page, workspace.slug);
    await openPicker(page);
    await expect(items(page)).toHaveCount(4);
    await expect(page.getByRole("button", { name: "View all favorites (4)", exact: true })).toBeVisible();
    await page.evaluate(() => localStorage.setItem("theme", "dark"));
    await openCreate(page, workspace.slug);
    await openPicker(page);
    await expect(page.locator("html")).toHaveClass(/dark/);
    await actorRow(page, "agent", agents[3]!.id).locator("button[data-picker-item]").hover();
    await page.screenshot({ path: evidencePath(info, "browser-actor-picker-1280-dark.png"), animations: "disabled" });
  } finally {
    if (other) { api.setWorkspaceId(other.id); api.setWorkspaceSlug(other.slug); await api.deleteFeatureWorkspace(other.id); }
    api.setWorkspaceId(workspace.id); api.setWorkspaceSlug(workspace.slug);
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("550 route-mocked actors remain searchable and pageable with browser render samples", async ({ page, browser }, info) => {
  test.setTimeout(120_000);
  const { api, workspace, userId } = await setup(page, info);
  try {
    await mockDirectory(page, workspace.id, userId);
    await page.setViewportSize({ width: 1280, height: 900 });
    await openCreate(page, workspace.slug);
    await openPicker(page);
    await expect(items(page)).toHaveCount(50);
    for (let count = 100; count <= 550; count += 50) {
      await page.getByRole("button", { name: /^Show more \(/ }).click();
      await expect(items(page)).toHaveCount(Math.min(count, 549));
    }
    await page.getByRole("button", { name: "Planning and coordination (1)" }).click();
    await expect(items(page)).toHaveCount(550);
    await expect(actorRow(page, "squad", actorId("squad", 49))).toBeVisible();
    const search = page.getByPlaceholder(SEARCH);
    await search.fill("responsibility-0499");
    await expect(items(page)).toHaveCount(1);
    await expect(actorRow(page, "agent", actorId("agent", 499))).toBeVisible();
    await search.fill("");
    await page.getByRole("button", { name: "Planning and coordination (1)" }).click();
    await expect(items(page)).toHaveCount(50);
    await search.fill("squad-duty-0049");
    await page.getByRole("button", { name: "Agents", exact: true }).click();
    await expect(items(page)).toHaveCount(0);
    await page.getByRole("button", { name: "AI squads", exact: true }).click();
    await expect(actorRow(page, "squad", actorId("squad", 49))).toBeVisible();
    await page.getByRole("button", { name: "All", exact: true }).click();

    const samples: { query: string; durationMs: number }[] = [];
    const queries = Array.from({ length: 20 }, (_, index) => {
      const number = index * 23;
      return { query: index % 2 ? `responsibility-${String(number).padStart(4, "0")}` : `Agent ${String(number).padStart(3, "0")}`, key: `agent:${actorId("agent", number)}` };
    });
    for (const sample of queries) {
      await search.evaluate((input, expectedKey) => {
        delete input.dataset.renderDuration;
        input.addEventListener("input", () => {
          const start = performance.now();
          const check = () => {
            const rows = document.querySelectorAll("[data-actor-key] button[data-picker-item]");
            if (rows.length === 1 && rows[0]?.closest("[data-actor-key]")?.getAttribute("data-actor-key") === expectedKey) {
              requestAnimationFrame(() => { input.dataset.renderDuration = String(performance.now() - start); });
            } else if (performance.now() - start < 5000) requestAnimationFrame(check);
          };
          requestAnimationFrame(check);
        }, { once: true, capture: true });
      }, sample.key);
      await search.fill(sample.query);
      await expect(search).toHaveAttribute("data-render-duration", /\d/);
      samples.push({ query: sample.query, durationMs: Number(await search.getAttribute("data-render-duration")) });
    }
    const ordered = samples.map((sample) => sample.durationMs).sort((a, b) => a - b);
    const report = {
      dataset: "500 agents + 50 squads, API directory route mocks; real authenticated app",
      mode: process.env.ACTOR_PICKER_BUILD_MODE ?? "unspecified (set ACTOR_PICKER_BUILD_MODE when collecting acceptance evidence)",
      browser: `Chromium ${browser.version()}`, os: `${platform()} ${release()}`, cpu: cpus()[0]?.model,
      viewport: page.viewportSize(), node: process.version, samples,
      method: "Browser performance.now from captured native input event to first animation frame after the DOM contains the expected sole actor. Full saved-description and exact-name queries alternate; data is already loaded. Includes frame scheduling, excludes Playwright transport and initial network/compilation.",
      p95Ms: ordered[Math.ceil(ordered.length * 0.95) - 1], targetMs: 100,
    };
    const reportPath = evidencePath(info, "browser-actor-picker-performance.json");
    writeFileSync(reportPath, JSON.stringify(report, null, 2));
    await info.attach("input-to-render performance", { path: reportPath, contentType: "application/json" });
    // Report the machine-dependent target; do not make shared CI flaky.
    expect(samples).toHaveLength(20);
    expect(samples.every((sample) => Number.isFinite(sample.durationMs) && sample.durationMs > 0)).toBe(true);
  } finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("keyboard actions preserve draft and reset stale choices in a narrow viewport", async ({ page }, info) => {
  const { api, workspace, userId } = await setup(page, info);
  try {
    const directory = await mockDirectory(page, workspace.id, userId, 8, 2);
    directory.agents[6]!.name = "跨工作区审查与长期职责说明智能体 / Long responsibility reviewer";
    await page.setViewportSize({ width: 1280, height: 900 });
    await openCreate(page, workspace.slug);
    await page.setViewportSize({ width: 375, height: 812 });
    const editor = page.getByRole("dialog", { name: "Quick create issue", exact: true }).locator('[contenteditable="true"]').first();
    await editor.fill("Keep my keyboard draft.");
    await openPicker(page);
    const search = page.getByPlaceholder(SEARCH);
    await search.fill("Agent 007");
    await search.dispatchEvent("keydown", { key: "Enter", code: "Enter", isComposing: true });
    await expect(search).toBeVisible();
    await search.press("Enter");
    await expect(search).toHaveCount(0);
    const trigger = page.getByRole("button", { name: /^Creation assistant/ });
    await expect(trigger).toContainText("Agent 007");
    await trigger.press("Enter");
    await expect(search).toHaveValue("");
    await expect(page.getByRole("button", { name: "All", exact: true })).toHaveAttribute("aria-pressed", "true");
    await search.fill("Squad");
    await page.getByRole("button", { name: "Agents", exact: true }).click();
    await search.focus();
    await search.press("Enter");
    await expect(search).toBeVisible();
    await expect(trigger).toContainText("Agent 007");
    await page.getByRole("button", { name: "All", exact: true }).click();
    await search.fill("");
    await search.fill("Agent 000");
    const pin = page.getByRole("button", { name: "Pin Agent 000 to favorites", exact: true });
    await search.focus();
    for (let tab = 0; tab < 16 && !(await pin.evaluate((button) => button === document.activeElement)); tab++) await page.keyboard.press("Tab");
    await expect(pin).toBeFocused();
    await page.keyboard.press("Space");
    const unpin = page.getByRole("button", { name: "Unpin Agent 000 from favorites", exact: true });
    await expect(unpin).toBeFocused();
    await expect(unpin).toHaveAttribute("aria-pressed", "true");
    await expect(trigger).toContainText("Agent 007");
    await expect(editor).toHaveText("Keep my keyboard draft.");
    await search.fill("");
    const browseAll = page.getByRole("button", { name: "Browse all (10)", exact: true });
    await browseAll.click();
    await expect(search).toBeFocused();
    await actorRow(page, "agent", actorId("agent", 6)).scrollIntoViewIfNeeded();
    await actorRow(page, "agent", actorId("agent", 7)).locator("button[data-picker-item]").hover();
    await page.screenshot({ path: evidencePath(info, "browser-actor-picker-375.png"), animations: "disabled" });
    const bounds = await search.boundingBox();
    expect(bounds).not.toBeNull();
    expect(bounds!.x).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(375);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(search).toHaveCount(0);
    await expect(trigger).toBeFocused();
  } finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("desktop renderer supports picker search and pinning without starting a daemon", async ({}, info) => {
  test.skip(!process.env.CHANGELOG_ELECTRON_RENDERER_URL || !process.env.CHANGELOG_E2E_API_URL,
    "Requires the built desktop renderer/preload and isolated local API fixture");
  test.setTimeout(90_000);
  const root = resolve(import.meta.dirname, "..");
  const profile = mkdtempSync(join(tmpdir(), "multica-actor-picker-"));
  const api = new TestApiClient();
  let workspaceId: string | undefined;
  let desktop: ElectronApplication | undefined;
  let desktopPage: Page | undefined;
  const desktopErrors: string[] = [];
  try {
    const slug = `e2e-desktop-picker-${randomUUID().slice(0, 8)}`;
    await api.login(`${slug}@example.invalid`, "Desktop actor picker verifier");
    const workspace = await api.ensureWorkspace("Desktop actor picker", slug);
    workspaceId = workspace.id;
    expect(workspace.slug).toBe(slug);
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const runtime = await api.seedProjectRuntime();
    const name = "Desktop saved-role reviewer";
    const actor = await api.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: {
      name, description: "Review desktop interactions and saved responsibilities.", runtime_id: runtime.id, permission_mode: "private",
    } });
    const token = api.getToken();
    if (!token) throw new Error("Desktop fixture has no token");
    const executablePath = join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin"
      ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron");
    desktop = await electron.launch({
      executablePath, args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
      env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en" },
    });
    const page = await desktop.firstWindow();
    desktopPage = page;
    const pageErrors: string[] = [];
    page.on("pageerror", (error) => pageErrors.push(error.message));
    page.on("console", (message) => { if (message.type() === "error") desktopErrors.push(message.text()); });
    await page.waitForURL(new URL(process.env.CHANGELOG_ELECTRON_RENDERER_URL!).href);
    await page.locator("#root > *").first().waitFor({ state: "attached" });
    await page.context().addInitScript((authToken) => {
      localStorage.setItem("multica_token", authToken);
      localStorage.setItem("multica-locale", "en");
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("multica_create_mode", JSON.stringify({ state: { lastMode: "agent" }, version: 0 }));
      localStorage.setItem("theme", "light");
    }, token);
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "New Issue", exact: true }).first().click({ timeout: 45_000 });
    await openPicker(page);
    const search = page.getByPlaceholder(SEARCH);
    await search.fill("saved responsibilities");
    await expect(actorRow(page, "agent", actor.id)).toContainText("Review desktop interactions");
    const pin = page.getByRole("button", { name: `Pin ${name} to favorites`, exact: true });
    await pin.focus();
    await pin.press("Space");
    await expect(page.getByRole("button", { name: `Unpin ${name} from favorites`, exact: true })).toBeFocused();
    await page.screenshot({ path: evidencePath(info, "browser-actor-picker-desktop.png"), animations: "disabled" });
    await page.keyboard.press("Escape");
    await expect(search).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^Creation assistant/ })).toBeFocused();
    expect(await desktop.evaluate(() => (globalThis as unknown as {
      changelogAcceptance: { daemonStarts: number; externalLinks: string[]; installCalls: number };
    }).changelogAcceptance)).toEqual({ daemonStarts: 0, externalLinks: [], installCalls: 0 });
    expect(pageErrors).toEqual([]);
  } catch (error) {
    if (desktopPage && !desktopPage.isClosed()) {
      await info.attach("desktop picker failure", { body: await desktopPage.screenshot(), contentType: "image/png" });
      await info.attach("desktop picker failure details", {
        body: `${await desktopPage.locator("body").innerText()}\n\n${desktopErrors.join("\n")}`, contentType: "text/plain",
      });
    }
    throw error;
  } finally {
    try { await desktop?.close(); }
    finally {
      try { if (workspaceId) await api.deleteFeatureWorkspace(workspaceId); }
      finally { rmSync(profile, { recursive: true, force: true }); }
    }
  }
});
