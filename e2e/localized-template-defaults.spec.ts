import { randomUUID } from "node:crypto";
import { test as base, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import enSkills from "../packages/views/locales/en/skills.json" with { type: "json" };
import zhSkills from "../packages/views/locales/zh-Hans/skills.json" with { type: "json" };

interface LocalizedFixture {
  api: TestApiClient;
  slug: string;
  origin: string;
  runtime: { id: string; daemon_id: string };
}

interface Agent {
  id: string;
  name: string;
  template_key: string;
  instructions: string;
  runtime_id: string;
  skills?: Array<{ name: string }>;
}

interface StaffedSquad {
  squad: { id: string; name: string; leader_id: string; template_key: string; member_count: number };
  created_agent_ids: string[];
  reused_agent_ids: string[];
}

const test = base.extend<{ localized: LocalizedFixture }>({
  localized: async ({ page, baseURL }, runFixture, testInfo) => {
    const slug = `e2e-localized-${randomUUID().slice(0, 12)}`;
    const api = new TestApiClient();
    await api.login(`${slug}@multica.ai`, "Localized template regression owner");
    const workspace = await api.ensureWorkspace("Localized template regression", slug);
    expect(workspace.slug).toBe(slug);

    try {
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
      const token = api.getToken();
      if (!token || !baseURL) throw new Error("Localized template fixture authentication is incomplete");
      await page.context().addCookies([
        { name: "multica-locale", value: "zh-Hans", url: baseURL },
        { name: "multica_logged_in", value: "1", url: baseURL },
      ]);
      await page.addInitScript((value) => {
        localStorage.setItem("multica_token", value);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, token);
      const runtime = await api.seedProjectRuntime();
      await runFixture({ api, slug, origin: baseURL, runtime });
    } finally {
      if (testInfo.status !== testInfo.expectedStatus) {
        await testInfo.attach("localized-template-failure", {
          body: await page.screenshot({ fullPage: true }), contentType: "image/png",
        });
      }
      await api.deleteFeatureWorkspace(workspace.id);
      expect((await api.getWorkspaces()).some((item) => item.id === workspace.id)).toBe(false);
    }
  },
});

test.use({ viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure" });

async function setLocale(page: Page, fixture: LocalizedFixture, locale: "en" | "zh-Hans") {
  await fixture.api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
  await page.context().addCookies([{ name: "multica-locale", value: locale, url: fixture.origin }]);
}

async function openDiscoveryStaffing(page: Page, fixture: LocalizedFixture, locale: "en" | "zh-Hans") {
  await setLocale(page, fixture, locale);
  const language = locale === "zh-Hans" ? "zh" : "en";
  const catalog = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === "/api/squads/templates" && url.searchParams.get("language") === language;
  });
  await page.goto(`/${fixture.slug}/squads`);
  await page.locator("header").getByRole("button", { name: locale === "zh-Hans" ? "使用模板" : "Use a template", exact: true }).click();
  expect((await catalog).status()).toBe(200);
  const dialog = page.getByRole("dialog");
  const name = locale === "zh-Hans" ? "需求预研小队" : "Discovery Squad";
  await dialog.getByRole("button", { name: new RegExp(name) }).click();
  await expect(dialog.locator("#staff-squad-name")).toHaveValue(name);
  // Empty means use the server's localized default, not an explicit copy of
  // the picker label. This exercises the actual browser request boundary.
  await dialog.locator("#staff-squad-name").fill("");
  return dialog.getByRole("button", { name: locale === "zh-Hans" ? "创建AI小队" : "Create squad", exact: true });
}

async function createDiscoverySquad(page: Page, fixture: LocalizedFixture, locale: "en" | "zh-Hans") {
  const create = await openDiscoveryStaffing(page, fixture, locale);
  const pending = page.waitForResponse((response) =>
    new URL(response.url()).pathname === "/api/squads/from-template" && response.request().method() === "POST",
  );
  await create.click();
  const response = await pending;
  expect(response.status()).toBe(201);
  expect(response.request().postDataJSON()).toMatchObject({
    template_key: "discovery", runtime_id: fixture.runtime.id,
    language: locale === "zh-Hans" ? "zh" : "en",
  });
  expect(response.request().postDataJSON()).not.toHaveProperty("name");
  const staffed: StaffedSquad = await response.json();
  await expect(page).toHaveURL(`/${fixture.slug}/squads/${staffed.squad.id}`);
  await expect(page.getByText(staffed.squad.name, { exact: true }).first()).toBeVisible();
  return staffed;
}

test("role picker defaults and omitted-name API creation follow Chinese and English locales", async ({ page, localized }) => {
  const { api, runtime, slug } = localized;
  for (const [locale, language, name] of [["zh-Hans", "zh", "产品分析师"], ["en", "en", "Product Analyst"]] as const) {
    await setLocale(page, localized, locale);
    const catalog = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return url.pathname === "/api/agents/templates" && url.searchParams.get("language") === language;
    });
    await page.goto(`/${slug}/agents/new/template?template=product-analyst`);
    expect((await catalog).status()).toBe(200);
    await expect(page.locator("#agent-create-name")).toHaveValue(name);

    const agent = await api.requestJSON<Agent>("/api/agents/from-template", {
      method: "POST",
      body: { template_key: "product-analyst", runtime_id: runtime.id, language, permission_mode: "private" },
    });
    expect(agent).toMatchObject({ name, template_key: "product-analyst", runtime_id: runtime.id });
    await page.goto(`/${slug}/agents/${agent.id}`);
    await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
    expect(await api.requestJSON(`/api/agents/${agent.id}`)).toMatchObject({ name });
  }
  const agents = await api.requestJSON<Agent[]>("/api/agents");
  expect(agents.map((agent) => agent.name).sort()).toEqual(["Product Analyst", "产品分析师"].sort());
});

test("diagnostician API creation preserves workspace skill and agent copies", async ({ page, localized }) => {
  const { api, runtime, slug } = localized;
  const customSkill = await api.requestJSON<{ id: string }>("/api/skills", {
    method: "POST",
    body: {
      name: "multica-debugging",
      description: "Workspace-owned diagnostic contract",
      content: "# Workspace diagnostic contract\n\nKeep this edited skill body.\n",
    },
  });

  const first = await api.requestJSON<Agent>("/api/agents/from-template", {
    method: "POST",
    body: {
      template_key: "diagnostician",
      runtime_id: runtime.id,
      language: "zh",
      name: "诊断工程师 · 工作区定制",
      permission_mode: "private",
    },
  });
  expect(first).toMatchObject({
    name: "诊断工程师 · 工作区定制",
    template_key: "diagnostician",
    template_version: 1,
    autonomy_level: "contributor",
    max_concurrent_tasks: 1,
  });
  expect(first.skills?.map((skill) => skill.name)).toEqual(["multica-debugging"]);

  const preservedSkill = await api.requestJSON<{ content: string }>(`/api/skills/${customSkill.id}`);
  expect(preservedSkill.content).toContain("Keep this edited skill body.");

  await api.requestJSON(`/api/agents/${first.id}`, {
    method: "PUT",
    body: { instructions: "Workspace-specific diagnostician instructions must survive." },
  });
  const second = await api.requestJSON<Agent>("/api/agents/from-template", {
    method: "POST",
    body: {
      template_key: "diagnostician",
      runtime_id: runtime.id,
      language: "zh",
      name: "诊断工程师 · 第二副本",
      permission_mode: "private",
    },
  });
  expect(second.skills?.map((skill) => skill.name)).toEqual(["multica-debugging"]);
  expect(await api.requestJSON<Agent>(`/api/agents/${first.id}`)).toMatchObject({
    name: "诊断工程师 · 工作区定制",
    instructions: "Workspace-specific diagnostician instructions must survive.",
  });

  await page.goto(`/${slug}/agents/${second.id}`);
  await expect(page.getByText("诊断工程师 · 第二副本", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("multica-debugging", { exact: true })).toBeVisible();
});

test("specialist skills localize, search and open their official copies without rewriting stored content", async ({ page, localized }) => {
  const { api, runtime, slug } = localized;
  const specialists = [
    { role: "experience-validation-engineer", skill: "multica-experience-validation" },
    { role: "migration-reviewer", skill: "multica-migration-review" },
  ] as const;
  for (const { role } of specialists) {
    await api.requestJSON("/api/agents/from-template", {
      method: "POST",
      body: { template_key: role, runtime_id: runtime.id, language: "zh", permission_mode: "private" },
    });
  }
  const skills = await api.requestJSON<{ id: string; name: string }[]>("/api/skills");
  const copies = await Promise.all(specialists.map(async ({ skill: name }) => {
    const skill = skills.find((item) => item.name === name);
    if (!skill) throw new Error(`Specialist did not materialize ${name}`);
    const snapshot = await api.requestJSON(`/api/skills/${skill.id}`);
    expect(snapshot).toMatchObject({ config: { origin: { type: "builtin_role_skill", name } } });
    return { ...skill, name, snapshot };
  }));
  const writes: string[] = [];
  page.on("request", (request) => {
    if (["PUT", "PATCH", "POST"].includes(request.method()) && new URL(request.url()).pathname.startsWith("/api/skills")) {
      writes.push(request.url());
    }
  });
  for (const [locale, copy] of [["en", enSkills], ["zh-Hans", zhSkills]] as const) {
    await setLocale(page, localized, locale);
    for (const skill of copies) {
      const displayed = copy.builtin_role_skills[skill.name];
      await page.goto(`/${slug}/skills`);
      const search = page.getByPlaceholder(copy.page.search_placeholder);
      for (const purpose of [enSkills.builtin_role_skills[skill.name].description, zhSkills.builtin_role_skills[skill.name].description]) {
        await search.fill(purpose);
        await expect(page.getByText(displayed.name, { exact: true }).first()).toBeVisible();
        await expect(page.getByText(displayed.description, { exact: true }).first()).toBeVisible();
      }
      const catalog = page.getByRole("region", { name: copy.catalog.title, exact: true });
      await catalog.getByRole("button", { name: new RegExp(`^${copy.catalog.title}`) }).click();
      const row = catalog.getByRole("listitem", { name: displayed.name, exact: true });
      await row.getByRole("button", { name: copy.catalog.open_instance, exact: true }).click();
      await expect(page).toHaveURL(`/${slug}/skills/${skill.id}`);
      await expect(page.getByRole("heading", { level: 1, name: displayed.name, exact: true })).toBeVisible();
      if (locale === "en") await expect(page.getByText(displayed.description, { exact: true })).toBeVisible();
      expect(await api.requestJSON(`/api/skills/${skill.id}`)).toEqual(skill.snapshot);
    }
  }
  expect(writes).toEqual([]);
});

test("Chinese squad staffing creates localized roles and English restaffing preserves customized agents", async ({ page, localized }, testInfo) => {
  const { api } = localized;
  const first = await createDiscoverySquad(page, localized, "zh-Hans");
  expect(first.squad).toMatchObject({ name: "需求预研小队", template_key: "discovery", member_count: 3 });
  expect(first.created_agent_ids).toHaveLength(3);
  expect(first.reused_agent_ids).toEqual([]);
  const agents = await api.requestJSON<Agent[]>("/api/agents");
  expect(agents.map((agent) => agent.name).sort()).toEqual(["需求预研负责人", "产品分析师", "架构师"].sort());
  const analyst = agents.find((agent) => agent.template_key === "product-analyst");
  if (!analyst) throw new Error("Staffed squad is missing its product analyst");
  const customName = "需求负责人 · 工作区定制";
  const customInstructions = "Workspace-specific discovery instructions must survive restaffing.";
  await api.requestJSON(`/api/agents/${analyst.id}`, {
    method: "PUT", body: { name: customName, instructions: customInstructions },
  });

  const second = await createDiscoverySquad(page, localized, "en");
  expect(second.squad).toMatchObject({ name: "Discovery Squad", template_key: "discovery", leader_id: first.squad.leader_id, member_count: 3 });
  expect(second.squad.id).not.toBe(first.squad.id);
  expect(second.created_agent_ids).toEqual([]);
  expect(second.reused_agent_ids.sort()).toEqual(first.created_agent_ids.sort());
  const after = await api.requestJSON<Agent[]>("/api/agents");
  expect(after.map((agent) => agent.id).sort()).toEqual(agents.map((agent) => agent.id).sort());
  expect(after.map((agent) => agent.name).sort()).toEqual(["需求预研负责人", customName, "架构师"].sort());
  expect(after.find((agent) => agent.id === analyst.id)).toMatchObject({ name: customName, instructions: customInstructions });
  const members = await api.requestJSON<{ member_id: string }[]>(`/api/squads/${second.squad.id}/members`);
  expect(members.map((member) => member.member_id).sort()).toEqual(first.created_agent_ids.sort());
  await testInfo.attach("localized-squad-reuse", {
    body: Buffer.from(JSON.stringify({ first, second, agents: after }, null, 2)), contentType: "application/json",
  });
});

test("a localized name collision leaves no partial squad and does not block English role names", async ({ page, localized }) => {
  const { api, runtime } = localized;
  const existing = await api.requestJSON<Agent>("/api/agents", {
    method: "POST", body: { name: "产品分析师", runtime_id: runtime.id, permission_mode: "private" },
  });
  const beforeSkills = await api.requestJSON<{ id: string }[]>("/api/skills");
  const create = await openDiscoveryStaffing(page, localized, "zh-Hans");
  const pending = page.waitForResponse((response) =>
    new URL(response.url()).pathname === "/api/squads/from-template" && response.request().method() === "POST",
  );
  await create.click();
  const response = await pending;
  expect(response.status()).toBe(409);
  await expect(page.getByRole("dialog").getByText(/an agent named "产品分析师" already exists/)).toBeVisible();
  expect(await api.requestJSON("/api/squads")).toEqual([]);
  expect((await api.requestJSON<Agent[]>("/api/agents")).map((agent) => agent.id)).toEqual([existing.id]);
  expect((await api.requestJSON<{ id: string }[]>("/api/skills")).map((skill) => skill.id).sort()).toEqual(beforeSkills.map((skill) => skill.id).sort());

  const english = await createDiscoverySquad(page, localized, "en");
  expect(english.squad.name).toBe("Discovery Squad");
  expect(english.created_agent_ids).toHaveLength(3);
  expect(english.reused_agent_ids).toEqual([]);
  const agents = await api.requestJSON<Agent[]>("/api/agents");
  expect(agents.filter((agent) => agent.id !== existing.id).map((agent) => agent.name).sort()).toEqual(["Discovery Lead", "Product Analyst", "Architect"].sort());
  expect(agents.find((agent) => agent.id === existing.id)?.name).toBe("产品分析师");
});
