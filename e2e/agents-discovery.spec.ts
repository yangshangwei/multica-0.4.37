import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Real auth/workspace, read-only directory fixtures. No daemon or agent CLI is
// involved; the browser must never write agent or squad business entities.
const ROLE_DEFINITIONS = [
  ["product-analyst", "产品分析师"], ["architect", "架构师"],
  ["implementer", "实现工程师"], ["qa-engineer", "测试工程师"],
  ["diagnostician", "诊断工程师"], ["code-reviewer", "代码审查员"],
  ["security-reviewer", "安全审查员"], ["release-engineer", "发布工程师"],
  ["technical-writer", "技术文档工程师"], ["reliability-engineer", "可靠性工程师"],
  ["migration-reviewer", "迁移审查员"], ["ux-verifier", "体验验证工程师"],
  ["performance-engineer", "性能工程师"], ["data-analyst", "数据分析师"],
] as const;
const LEAD_DEFINITIONS = [
  ["feature-delivery", "delivery-lead", "特性交付负责人"],
  ["bug-fix", "bug-fix-lead", "缺陷修复负责人"],
  ["review-gate", "review-gate-lead", "合并门禁负责人"],
  ["discovery", "discovery-lead", "需求预研负责人"],
  ["docs", "docs-lead", "文档协同负责人"],
  ["maintenance", "maintenance-lead", "例行维护负责人"],
] as const;
const RUNTIME_ID = "71000000-0000-4000-8000-000000000001";
const OTHER_OWNER_ID = "71000000-0000-4000-8000-000000000002";
const SQUAD_IDS = [
  "72000000-0000-4000-8000-000000000001",
  "72000000-0000-4000-8000-000000000002",
  "72000000-0000-4000-8000-000000000003",
];

function directoryFixtures(workspaceId: string, userId: string) {
  const templates = ROLE_DEFINITIONS.map(([key, title]) => ({
    key, title, name: title, version: 1, description: `${title}的模板职责。`,
    instructions: `Follow the ${key} role contract.`, autonomy_level: "contributor",
    avatar_emoji: "", max_concurrent_tasks: 1, skill_names: [],
  }));
  const squadTemplates = LEAD_DEFINITIONS.map(([key, leaderKey, title]) => ({
    key, title: `${title}小队`, name: `${title}小队`, version: 1,
    description: `${title}协调工作。`, instructions: "Route work to the appropriate member.", avatar_emoji: "",
    leader: { template_key: leaderKey, title, name: title, role: "协调工作", autonomy_level: "coordinator", avatar_emoji: "" },
    members: [],
  }));
  const definitions = [
    ...ROLE_DEFINITIONS.map(([key, name]) => ({ key, name })),
    ...LEAD_DEFINITIONS.map(([, key, name]) => ({ key, name })),
    { key: undefined, name: "自定义文案助手" },
    { key: "new-server-role", name: "新版本智能体" },
  ];
  const agents = definitions.map(({ key, name }, index) => ({
    id: `73000000-0000-4000-8000-${String(index + 1).padStart(12, "0")}`,
    workspace_id: workspaceId, runtime_id: RUNTIME_ID, runtime_bound: true, runtime_availability: "online",
    name: index === 2 ? "支付实现" : index === 13 ? "协作空间专家" : name,
    description: index === 2 ? "用户保存的说明：处理支付结算代码，保持原有约束。" : `用户保存的${name}说明。`,
    instructions: "User-owned instructions must not change.", avatar_url: null,
    runtime_mode: "local", runtime_config: {}, custom_args: [],
    visibility: "workspace", permission_mode: "public_to",
    invocation_targets: [{ target_type: "workspace", target_id: workspaceId }],
    status: "idle", max_concurrent_tasks: 1, model: "", owner_id: index === 13 ? OTHER_OWNER_ID : userId,
    skills: [], template_key: key, template_version: key ? 1 : undefined,
    created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z", archived_at: null, archived_by: null,
  }));
  const shared = agents[2];
  // The shared specialist is fourth, beyond the three-person preview.
  const rosterIds = [
    [agents[14].id, agents[0].id, agents[1].id, shared.id, agents[13].id],
    [agents[15].id, agents[4].id, shared.id],
    [agents[16].id, agents[0].id, agents[3].id, shared.id],
  ];
  const squads = ["支付交付小队", "客户缺陷小队", "旧服务审查小队"].map((name, index) => ({
    id: SQUAD_IDS[index], workspace_id: workspaceId, name, description: `${name}的现有说明。`,
    instructions: "Existing squad routing remains unchanged.", avatar_url: null,
    leader_id: rosterIds[index][0], creator_id: userId,
    created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z", archived_at: null, archived_by: null,
    member_count: rosterIds[index].length,
    member_preview: rosterIds[index].slice(0, 3).map((memberId) => ({ member_id: memberId, member_type: "agent", role: "Existing role" })),
    ...(index < 2 ? { agent_member_ids: rosterIds[index] } : {}),
  }));
  const legacyMembers = rosterIds[2].map((memberId, index) => ({
    id: `74000000-0000-4000-8000-${String(index + 1).padStart(12, "0")}`,
    squad_id: SQUAD_IDS[2], member_id: memberId, member_type: "agent", role: "Existing member role",
    created_at: "2026-09-01T00:00:00Z",
  }));
  return { agents, templates, squadTemplates, squads, legacyMembers, shared };
}

function agentRow(page: Page, name: string) {
  return page.getByRole("table").getByRole("row").filter({ has: page.getByRole("link", { name, exact: true }) });
}

async function selectSquad(page: Page, name: string) {
  await page.getByRole("button", { name: "AI小队", exact: true }).click();
  if (name === "全部小队") await page.getByRole("menuitem", { name, exact: true }).click();
  else await page.getByRole("menuitemcheckbox", { name, exact: true }).click();
  await page.keyboard.press("Escape");
}

test("agent discovery keeps role and squad identification accurate without changing members", async ({ page }, info) => {
  const api = new TestApiClient();
  let workspaceId: string | undefined;
  let releaseLegacy = () => {};
  const pageErrors: string[] = [];
  const mutationRequests: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  try {
    const run = `${Date.now().toString(36)}-${info.parallelIndex}`;
    const session = await api.login(`e2e-agent-discovery-${run}@multica.ai`, "智能体目录测试员");
    const workspace = await api.ensureWorkspace("智能体目录验证", `agents-discovery-${run}`);
    workspaceId = workspace.id;
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    const token = api.getToken();
    if (!token || !session.user?.id) throw new Error("Fixture login did not return a user and token");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, token);
    const baseURL = info.project.use.baseURL;
    if (typeof baseURL !== "string") throw new Error("Playwright baseURL is required");
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: baseURL }]);
    const fixtures = directoryFixtures(workspace.id, session.user.id);
    const { shared } = fixtures;
    await page.route("**/api/agents?**", (route) => route.fulfill({ json: fixtures.agents }));
    await page.route("**/api/agents/templates**", (route) => route.fulfill({ json: { templates: fixtures.templates } }));
    await page.route("**/api/squads/templates**", (route) => route.fulfill({ json: { templates: fixtures.squadTemplates } }));
    await page.route("**/api/squads", (route) => route.fulfill({ json: fixtures.squads }));
    await page.route("**/api/runtimes?**", (route) => route.fulfill({ json: [{
      id: RUNTIME_ID, workspace_id: workspace.id, name: "E2E runtime", provider: "codex", runtime_mode: "local",
      owner_id: session.user.id, status: "online", visibility: "public", daemon_id: null,
      metadata: {}, launch_header: "", device_info: "No daemon process", last_seen_at: new Date().toISOString(),
      created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
    }] }));
    await page.route("**/api/agent-task-snapshot", (route) => route.fulfill({ json: [] }));
    await page.route("**/api/agent-activity-30d", (route) => route.fulfill({ json: [] }));
    await page.route("**/api/agent-run-counts", (route) => route.fulfill({ json: [] }));
    let legacyRequests = 0;
    let failLegacy = true;
    const legacyGate = new Promise<void>((resolve) => { releaseLegacy = resolve; });
    await page.route(`**/api/squads/${SQUAD_IDS[2]}/members`, async (route) => {
      legacyRequests++;
      await legacyGate;
      if (failLegacy) await route.fulfill({ status: 503, json: { error: "Fixture membership temporarily unavailable" } });
      else await route.fulfill({ json: fixtures.legacyMembers });
    });
    page.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (["POST", "PUT", "PATCH", "DELETE"].includes(request.method()) && /^\/api\/(agents|squads)(\/|$)/.test(path)) {
        mutationRequests.push(`${request.method()} ${path}`);
      }
    });

    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/agents`);
    const search = page.getByRole("textbox", { name: "搜索智能体...", exact: true });
    await expect(search).toBeVisible();
    await expect(page.getByRole("button", { name: /^我的\s*21$/ })).toBeVisible();
    await expect(page.getByRole("region", { name: /内置智能体/ })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^统筹角色\s*6$/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /^专业角色\s*13$/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /^其他智能体\s*2$/ })).toBeVisible();
    await page.getByRole("button", { name: /^全部\s*22$/ }).click();
    await expect(page.getByRole("button", { name: /^专业角色\s*14$/ })).toBeVisible();
    await expect(page.getByRole("table").getByText(/^统筹角色\s*6$/)).toHaveCount(1);
    await page.screenshot({ path: info.outputPath("agents-22-grouped-1440-zh.png"), animations: "disabled" });

    // Search reaches saved names, immutable role provenance and real squad names.
    await search.fill("支付实现");
    const sharedRow = agentRow(page, shared.name);
    await expect(sharedRow).toBeVisible();
    await expect(sharedRow.getByText("角色模板：实现工程师", { exact: true })).toBeVisible();
    await expect(sharedRow.getByText(shared.description, { exact: true })).toBeVisible();
    await expect(sharedRow.getByRole("link", { name: "支付交付小队", exact: true })).toHaveAttribute("href", `/${workspace.slug}/squads/${SQUAD_IDS[0]}`);
    await expect(sharedRow.getByRole("link", { name: "客户缺陷小队", exact: true })).toHaveAttribute("href", `/${workspace.slug}/squads/${SQUAD_IDS[1]}`);
    await search.fill("实现工程师");
    await expect(sharedRow).toBeVisible();
    await expect(page.getByRole("table").getByRole("row")).toHaveCount(2);
    await search.fill("客户缺陷小队");
    await expect(sharedRow).toBeVisible();
    await expect(page.getByRole("table").getByRole("row")).toHaveCount(4);
    await search.fill("自定义文案助手");
    await expect(agentRow(page, "自定义文案助手")).toBeVisible();
    await expect(agentRow(page, "自定义文案助手").getByText(/角色模板：/)).toHaveCount(0);
    await search.fill("");

    // Filtering must intersect Mine; a shared specialist beyond the preview
    // remains included, while the same squad's other-owner agent stays out.
    await page.getByRole("button", { name: /^我的\s*21$/ }).click();
    await selectSquad(page, "支付交付小队");
    await expect(sharedRow).toBeVisible();
    await expect(page.getByRole("table").getByRole("row")).toHaveCount(5);
    await expect(agentRow(page, "协作空间专家")).toHaveCount(0);
    await page.getByRole("button", { name: /^全部\s*22$/ }).click();
    await expect(agentRow(page, "协作空间专家")).toBeVisible();
    await expect(page.getByRole("table").getByRole("row")).toHaveCount(6);
    await page.getByRole("button", { name: /^我的\s*21$/ }).click();
    await expect(agentRow(page, "协作空间专家")).toHaveCount(0);
    await expect(sharedRow).toBeVisible();
    await selectSquad(page, "全部小队");
    await page.getByRole("button", { name: /^其他智能体\s*2$/ }).click();
    await expect(agentRow(page, "自定义文案助手")).toBeVisible();
    await expect(agentRow(page, "新版本智能体")).toBeVisible();
    await page.getByRole("button", { name: "全部角色", exact: true }).click();

    // Fresh defaults are concise. Explicit column and grouping preferences
    // survive a reload without requiring a new directory or changing data.
    await page.getByRole("button", { name: "显示", exact: true }).click();
    await expect(page.getByRole("switch", { name: "owner", exact: true })).not.toBeChecked();
    await expect(page.getByRole("switch", { name: "运行时", exact: true })).not.toBeChecked();
    await expect(page.getByRole("switch", { name: "近 30 天运行次数", exact: true })).not.toBeChecked();
    await page.getByRole("switch", { name: "owner", exact: true }).click();
    await page.getByRole("switch", { name: "按角色分组", exact: true }).click();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("table").getByText(/^统筹角色\s*6$/)).toHaveCount(0);
    await page.reload();
    await expect(search).toBeVisible();
    await page.getByRole("button", { name: "显示", exact: true }).click();
    await expect(page.getByRole("switch", { name: "owner", exact: true })).toBeChecked();
    await expect(page.getByRole("switch", { name: "按角色分组", exact: true })).not.toBeChecked();
    await page.getByRole("switch", { name: "owner", exact: true }).click();
    await page.getByRole("switch", { name: "按角色分组", exact: true }).click();
    await page.keyboard.press("Escape");

    // Legacy servers lack the complete member IDs. Opening the page and
    // choosing other squads must not cause eager per-squad member requests.
    expect(legacyRequests).toBe(0);
    await selectSquad(page, "旧服务审查小队");
    await expect.poll(() => legacyRequests).toBeGreaterThan(0);
    await expect(page.getByText("该筛选下没有匹配的智能体。", { exact: true })).toHaveCount(0);
    releaseLegacy();
    const membershipError = page.getByRole("alert").filter({ hasText: "无法加载小队成员。" });
    await expect(membershipError).toBeVisible({ timeout: 15_000 });
    await expect(sharedRow).toHaveCount(0);
    await expect(page.getByText("该筛选下没有匹配的智能体。", { exact: true })).toHaveCount(0);
    failLegacy = false;
    await membershipError.getByRole("button", { name: "重试", exact: true }).click();
    await expect(sharedRow).toBeVisible();
    await expect(page.getByRole("table").getByRole("row")).toHaveCount(5);
    await expect(sharedRow.getByRole("link", { name: "旧服务审查小队", exact: true })).toBeVisible();
    await selectSquad(page, "全部小队");

    for (const width of [1440, 768, 390]) {
      await page.setViewportSize({ width, height: 1000 });
      await search.fill("支付实现");
      await expect(sharedRow).toBeVisible();
      await page.screenshot({ path: info.outputPath(`agents-shared-${width}-zh.png`), animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await search.fill("");
      await page.screenshot({ path: info.outputPath(`agents-grouped-${width}-zh.png`), animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
    expect(pageErrors).toEqual([]);
    expect(mutationRequests).toEqual([]);
  } catch (error) {
    await page.screenshot({ path: info.outputPath("agents-discovery-failure.png"), animations: "disabled" });
    await info.attach("browser-evidence", { body: JSON.stringify({ pageErrors, mutationRequests }), contentType: "application/json" });
    throw error;
  } finally {
    releaseLegacy();
    if (workspaceId) await api.deleteFeatureWorkspace(workspaceId);
  }
});
