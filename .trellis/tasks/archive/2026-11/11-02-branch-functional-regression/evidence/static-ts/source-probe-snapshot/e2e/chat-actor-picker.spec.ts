import { randomUUID } from "node:crypto";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

test.use({ screenshot: "only-on-failure" });

// Component/model suites own ranking and preference boundaries. These tests
// exercise the real browser surfaces without sending a chat or starting a CLI.
const SEARCH = "Search names or responsibilities...";
const SEARCH_PATTERN = /^(?:Search names or responsibilities\.\.\.|搜索名称或职责\.\.\.)$/;
const RUNTIME_ID = "84000000-0000-4000-8000-000000000001";
const agentId = (index: number) => `85000000-0000-4000-8000-${String(index).padStart(12, "0")}`;
const actorRow = (page: Page, index: number) => page.locator(`[data-actor-key="agent:${agentId(index)}"]`);
const actorRows = (page: Page) => page.locator("[data-actor-key] button[data-picker-item]");
const popup = (page: Page) => page.locator('[data-slot="popover-content"]').filter({ has: page.getByPlaceholder(SEARCH_PATTERN) });

function evidencePath(info: TestInfo, name: string) {
  const directory = process.env.ACTOR_PICKER_EVIDENCE_DIR;
  if (!directory) return info.outputPath(name);
  mkdirSync(directory, { recursive: true });
  return join(directory, name);
}

async function setup(page: Page, info: TestInfo, locale: "en" | "zh-Hans" = "en") {
  const api = new TestApiClient();
  const slug = `e2e-chat-picker-${randomUUID().slice(0, 8)}`;
  const username = slug.replaceAll("-", "");
  let userId: string;
  if (process.env.E2E_PASSWORD_AUTH === "1") {
    const apiBase = process.env.NEXT_PUBLIC_API_URL;
    if (!apiBase) throw new Error("Password fixture requires NEXT_PUBLIC_API_URL");
    const password = "chat picker acceptance password 2026";
    const response = await page.request.post(`${apiBase}/auth/register`, {
      data: { username, password, name: "Chat picker verifier" },
    });
    expect(response.status()).toBe(201);
    userId = (await api.loginPassword(username, password)).id;
  } else {
    const session = await api.login(`${slug}@multica.ai`, "Chat picker verifier");
    if (!session.user?.id) throw new Error("Incomplete E2E user");
    userId = session.user.id as string;
  }
  const workspace = await api.ensureWorkspace("Chat picker verification", slug);
  const cleanup = async () => {
    await api.deleteFeatureWorkspace(workspace.id);
    if (process.env.E2E_PASSWORD_AUTH === "1") await api.deletePasswordAccount(username);
  };
  try {
    expect(workspace.slug).toBe(slug);
    if (process.env.E2E_PASSWORD_AUTH === "1") {
      const database = new pg.Client(process.env.DATABASE_URL);
      await database.connect();
      try {
        await database.query('UPDATE "user" SET onboarded_at = now() WHERE id = $1', [userId]);
      } finally {
        await database.end();
      }
    } else {
      await api.markUserOnboarded();
    }
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
    const token = api.getToken();
    const baseURL = info.project.use.baseURL;
    if (!token || typeof baseURL !== "string") throw new Error("Incomplete E2E session");
    await page.context().addCookies([
      { name: "multica-locale", value: locale, url: baseURL },
      { name: "multica_logged_in", value: "1", url: baseURL },
    ]);
    await page.addInitScript((authToken) => {
      localStorage.setItem("multica_token", authToken);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("multica:chat:floatingChatEnabled", "true");
      localStorage.setItem("theme", "light");
    }, token);
    return { workspace, userId, cleanup };
  } catch (error) {
    await cleanup();
    throw error;
  }
}

async function mockDirectory(page: Page, workspaceId: string, userId: string) {
  const agents = [
    { name: "Coordination lead", description: "Coordinate delivery and track the squad's progress." },
    { name: "小阿孚", description: "Help the workspace choose its next task.", system_key: "mika" },
    { name: "Test engineer", description: `${"Saved responsibility details. ".repeat(16)}browser-only-duty: Verify keyboard and narrow layouts.` },
    { name: "Unbound reviewer", description: "Review changes after a runtime is connected." },
  ].map((actor, index) => ({
    id: agentId(index), workspace_id: workspaceId, runtime_id: index === 3 ? "" : RUNTIME_ID,
    runtime_bound: index !== 3, runtime_availability: "online",
    ...actor, instructions: "private-instructions-not-searchable", avatar_url: null,
    runtime_mode: "local", runtime_config: {}, custom_args: [], visibility: "workspace",
    permission_mode: "private", status: "idle", max_concurrent_tasks: 1, model: "",
    owner_id: userId, skills: [], created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
    archived_at: null, archived_by: null,
  }));
  const squads = [{
    id: "86000000-0000-4000-8000-000000000001", workspace_id: workspaceId, name: "Delivery squad",
    description: "A squad is not a direct chat identity.", instructions: "", avatar_url: null,
    leader_id: agentId(0), creator_id: userId, created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
    archived_at: null, archived_by: null, member_count: 1, agent_member_ids: [agentId(0)], member_preview: [],
  }];
  await page.route("**/api/agents?**", (route) => route.fulfill({ json: agents }));
  await page.route("**/api/squads", (route) => route.fulfill({ json: squads }));
  await page.route("**/api/runtimes?**", (route) => route.fulfill({ json: [{
    id: RUNTIME_ID, workspace_id: workspaceId, name: "No-process fixture", provider: "codex",
    runtime_mode: "local", owner_id: userId, status: "online", visibility: "private", daemon_id: null,
    metadata: { cli_version: "0.4.40", capabilities: ["rpc-v1"] }, launch_header: "", device_info: "Fixture only",
    last_seen_at: new Date().toISOString(), created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
  }] }));
  let submissions = 0;
  await page.route("**/api/chat/**", (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() !== "GET") {
      submissions++;
      return route.fulfill({ status: 409, json: { error: "Read-only chat picker fixture" } });
    }
    if (path === "/api/chat/sessions" || path === "/api/chat/pinned-agents") return route.fulfill({ json: [] });
    if (path === "/api/chat/pending-tasks/has-any") return route.fulfill({ json: { has_pending: false } });
    if (path === "/api/chat/pending-tasks") return route.fulfill({ json: { tasks: [] } });
    return route.continue();
  });
  return { submissions: () => submissions };
}

async function expectChatCategories(page: Page, zh = false) {
  for (const name of zh ? ["全部", "小阿孚", "智能体", "规划与协调"] : ["All", "小阿孚", "Agents", "Planning and coordination"]) {
    await expect(popup(page).getByRole("button", { name, exact: true })).toBeVisible();
  }
  await expect(popup(page).getByRole("button", { name: zh ? "AI小队" : "AI squads", exact: true })).toHaveCount(0);
  await expect(popup(page).getByRole("button", { name: zh ? "将当前助手设为默认" : "Set current as default", exact: true })).toHaveCount(0);
}

for (const locale of ["en", "zh-Hans"] as const) {
test(`new chat reuses responsibility discovery, categories and keyboard selection at wide and narrow widths (${locale})`, async ({ page }, info) => {
  test.setTimeout(90_000);
  const { workspace, userId, cleanup } = await setup(page, info, locale);
  const zh = locale === "zh-Hans";
  try {
    const directory = await mockDirectory(page, workspace.id, userId);
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto(`/${workspace.slug}/chat`, { waitUntil: "domcontentloaded" });
    const newChat = page.getByRole("button", { name: zh ? "新对话" : "New chat", exact: true });
    await newChat.click();
    const search = page.getByPlaceholder(SEARCH_PATTERN);
    await expect(search).toBeVisible();
    await expectChatCategories(page, zh);
    await expect(actorRows(page)).toHaveCount(4);
    await expect(actorRow(page, 3).locator("button[data-picker-item]")).toBeDisabled();

    await popup(page).getByRole("button", { name: zh ? "规划与协调" : "Planning and coordination", exact: true }).click();
    await expect(actorRows(page)).toHaveCount(1);
    await expect(actorRow(page, 0)).toContainText("Delivery squad");
    await popup(page).getByRole("button", { name: "小阿孚", exact: true }).click();
    await expect(actorRows(page)).toHaveCount(1);
    await expect(actorRow(page, 1)).toBeVisible();
    await popup(page).getByRole("button", { name: zh ? "智能体" : "Agents", exact: true }).click();
    await expect(actorRow(page, 0)).toHaveCount(0);
    await popup(page).getByRole("button", { name: zh ? "全部" : "All", exact: true }).click();

    for (const width of [1280, 390]) {
      // Compact chat replaces the split-pane header, remounting its trigger.
      await search.press("Escape");
      await expect(search).toHaveCount(0);
      await page.setViewportSize({ width, height: 900 });
      await expect(newChat).toBeInViewport();
      await newChat.click();
      await expect(search).toBeVisible();
      await expect.poll(async () => {
        const box = await popup(page).boundingBox();
        return box ? box.x + box.width : Infinity;
      }).toBeLessThanOrEqual(width);
      const bounds = await popup(page).boundingBox();
      expect(bounds).not.toBeNull();
      expect(bounds!.x).toBeGreaterThanOrEqual(0);
      expect(bounds!.y).toBeGreaterThanOrEqual(0);
      expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(900);
      await page.screenshot({ path: evidencePath(info, `chat-actor-picker-${locale}-${width}.png`), animations: "disabled" });
    }

    await search.fill("Unbound reviewer");
    await search.press("Enter");
    await expect(search).toBeVisible();
    await search.fill("browser-only-duty");
    await expect(actorRows(page)).toHaveCount(1);
    await expect(actorRow(page, 2)).toBeVisible();
    await search.press("Enter");
    await expect(search).toHaveCount(0);
    await expect(page.getByRole("heading", { name: /Test engineer/ }).last()).toBeVisible();
    expect(directory.submissions()).toBe(0);
  } finally {
    await cleanup();
  }
});
}

test("floating chat avatar uses the same agent discovery and restores focus on Escape", async ({ page }, info) => {
  const { workspace, userId, cleanup } = await setup(page, info);
  try {
    const directory = await mockDirectory(page, workspace.id, userId);
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto(`/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "Ask Multica", exact: true }).click();
    const trigger = page.getByRole("button", { name: /^Coordination lead/ });
    await trigger.click();
    const search = page.getByPlaceholder(SEARCH, { exact: true });
    await expect(search).toBeVisible();
    await expectChatCategories(page);
    await expect(actorRow(page, 3).locator("button[data-picker-item]")).toBeDisabled();
    await expect(actorRow(page, 0)).toContainText("Selected");
    await search.press("Escape");
    await expect(search).toHaveCount(0);
    await expect(trigger).toBeFocused();
    await trigger.press("Enter");
    await search.fill("browser-only-duty");
    await search.press("Enter");
    await expect(search).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^Test engineer/ })).toBeVisible();
    expect(directory.submissions()).toBe(0);
  } finally {
    await cleanup();
  }
});
