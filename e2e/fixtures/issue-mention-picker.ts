import { createServer } from "node:http";
import { randomUUID } from "node:crypto";
import { expect, type Page, type Route } from "@playwright/test";
import { TestApiClient } from "../fixtures";

const MOCK_HEADERS = { "access-control-allow-origin": "*", "access-control-allow-headers": "*" };
const RUNTIME_ID = "91000000-0000-4000-8000-000000000001";
export const actorId = (type: "agent" | "squad", index: number) =>
  `${type === "agent" ? "92000000" : "93000000"}-0000-4000-8000-${String(index).padStart(12, "0")}`;
export const picker = (page: Page) => page.locator('[data-mention-picker="categorized"]');
export const mentionRow = (page: Page, type: string, id: string) =>
  picker(page).locator(`[data-mention-key="${type}:${id}"]`);

export async function seedMentionWorkspace(locale = "en") {
  const api = new TestApiClient();
  const slug = `e2e-mentions-${randomUUID().slice(0, 8)}`;
  const session = await api.login(`${slug}@example.invalid`, "Mention verifier");
  const workspace = await api.ensureWorkspace("Mention verification", slug);
  expect(workspace.slug).toBe(slug);
  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
    const issue = await api.createIssue("Mention picker draft host");
    const target = await api.createIssue("Mention target issue");
    const parent = await api.requestJSON<{ id: string }>(`/api/issues/${issue.id}/comments`, {
      method: "POST", body: { content: "Seeded parent comment" },
    });
    const reply = await api.requestJSON<{ id: string }>(`/api/issues/${issue.id}/comments`, {
      method: "POST", body: { content: "Seeded reply comment", parent_id: parent.id },
    });
    const token = api.getToken();
    if (!token || !session.user?.id) throw new Error("Incomplete mention fixture session");
    return { api, workspace, issue, target, parent, reply, token, userId: session.user.id as string };
  } catch (error) {
    await api.deleteFeatureWorkspace(workspace.id);
    throw error;
  }
}
export type MentionFixture = Awaited<ReturnType<typeof seedMentionWorkspace>>;

export async function prepareMentionPage(page: Page, fixture: MentionFixture, origin: string, locale = "en", theme = "light", native = false) {
  const mock = await mockTransport(native);
  await page.context().addCookies([
    { name: "multica-locale", value: locale, url: origin },
    { name: "multica_logged_in", value: "1", url: origin },
  ]);
  await page.context().addInitScript(({ token, locale, theme }) => {
    localStorage.setItem("multica_token", token);
    localStorage.setItem("multica-locale", locale);
    localStorage.setItem("multica:chat:isOpen", "false");
    localStorage.setItem("theme", theme);
  }, { token: fixture.token, locale, theme });
  const writes: string[] = [];
  const errors: string[] = [];
  const failures: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => { if (message.type() === "error") errors.push(message.text()); });
  page.on("response", (response) => {
    if (response.status() >= 400 && new URL(response.url()).pathname.startsWith("/api/"))
      failures.push(`${response.status()} ${new URL(response.url()).pathname}`);
  });
  await page.route("**/api/**", (route) => {
    const request = route.request();
    if (["/api/issues/query", "/api/issues/table/rows", "/api/issues/table/facets"].includes(new URL(request.url()).pathname))
      return route.continue();
    if (new URL(request.url()).pathname === "/api/client-usage")
      return mock.fulfill(route, { headers: MOCK_HEADERS, status: 204 });
    if (new URL(request.url()).pathname.endsWith("/comments/trigger-preview"))
      return mock.fulfill(route, { status: 200, headers: MOCK_HEADERS, json: { agents: [] } });
    if (["GET", "HEAD", "OPTIONS"].includes(request.method())) return route.continue();
    writes.push(`${request.method()} ${new URL(request.url()).pathname}`);
    return mock.fulfill(route, { headers: MOCK_HEADERS, status: 409, json: { error: "Read-only mention acceptance fixture" } });
  });
  await mockMentionDirectory(page, fixture.workspace.id, fixture.userId, mock.fulfill);
  return { writes, errors, failures, close: mock.close };
}

export async function creationPreferences(page: Page, slug: string) {
  return page.evaluate((slug) => localStorage.getItem(`multica_quick_create:${slug}`), slug);
}

export async function mockMentionDirectory(page: Page, workspaceId: string, userId: string, fulfill: (route: Route, options: NonNullable<Parameters<Route["fulfill"]>[0]>) => Promise<void>, agentCount = 500, squadCount = 50) {
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
  await page.route("**/api/agents?**", (route) => fulfill(route, { status: 200, headers: MOCK_HEADERS, json: agents }));
  await page.route("**/api/squads", (route) => fulfill(route, { status: 200, headers: MOCK_HEADERS, json: squads }));
  await page.route("**/api/runtimes?**", (route) => fulfill(route, { status: 200, headers: MOCK_HEADERS, json: [{
    id: RUNTIME_ID, workspace_id: workspaceId, name: "No-process fixture", provider: "codex",
    runtime_mode: "local", owner_id: userId, status: "online", visibility: "private", daemon_id: null,
    metadata: { cli_version: "0.4.40", capabilities: ["rpc-v1"] }, launch_header: "", device_info: "Fixture only",
    last_seen_at: new Date().toISOString(), created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
  }] }));
  agents[499]!.name = "测试工程师";
  return { agents, squads };
}

async function mockTransport(native: boolean) {
  if (!native) return {
    fulfill: (route: Route, options: NonNullable<Parameters<Route["fulfill"]>[0]>) => {
      const { headers: _headers, ...response } = options;
      return route.fulfill(response);
    },
    close: async () => {},
  };
  const responses = new Map<string, NonNullable<Parameters<Route["fulfill"]>[0]>>();
  const server = createServer((request, response) => {
    const options = responses.get(request.url || "");
    if (!options) { response.writeHead(404); response.end(); return; }
    response.writeHead(options.status ?? 200, { ...MOCK_HEADERS, "content-type": "application/json" });
    response.end(JSON.stringify(options.json));
    responses.delete(request.url!);
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Missing mock HTTP address");
  return {
    fulfill: async (route: Route, options: NonNullable<Parameters<Route["fulfill"]>[0]>) => {
      const path = `/${randomUUID()}`;
      responses.set(path, options);
      await route.continue({ url: `http://127.0.0.1:${address.port}${path}` });
    },
    close: () => new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve())),
  };
}
