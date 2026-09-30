import { randomUUID } from "node:crypto";
import { test as base, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import enSkills from "../packages/views/locales/en/skills.json" with { type: "json" };
import zhSkills from "../packages/views/locales/zh-Hans/skills.json" with { type: "json" };

type SkillLocale = "en" | "zh-Hans";
type SkillCopy = typeof enSkills | typeof zhSkills;

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

async function setLocale(page: Page, fixture: LocalizedFixture, locale: SkillLocale) {
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
  await page.locator("header").getByRole("button", { name: locale === "zh-Hans" ? "新建AI小队" : "New Squad", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: locale === "zh-Hans" ? /^从模板创建/ : /^Create from template/ }).click();
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

test("specialist skills localize, search and open their official copies without rewriting stored content", async ({ page, localized }, testInfo) => {
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
    if (["PUT", "PATCH", "POST", "DELETE"].includes(request.method()) && new URL(request.url()).pathname.startsWith("/api/skills")) {
      writes.push(request.url());
    }
  });
  const catalog = await api.requestJSON<{ templates: { name: string }[] }>("/api/skills/templates");
  const visualStates: Record<string, unknown>[] = [];
  const capture = async (state: string, locale: SkillLocale, copy: SkillCopy, theme: "light" | "dark") => {
    await page.evaluate(() => document.fonts.ready);
    const viewport = page.viewportSize()!;
    await expect(page.locator("html")).toHaveClass(new RegExp(`\\b${theme}\\b`));
    await expect.poll(() => page.evaluate(() =>
      Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) <= window.innerWidth,
    )).toBe(true);
    const entry = page.getByRole("tablist", { name: copy.market.views_label, exact: true, includeHidden: true });
    const entryBounds = await entry.boundingBox();
    expect(entryBounds).not.toBeNull();
    expect(entryBounds!.height).toBeLessThanOrEqual(viewport.width >= 768 ? 64 : 112);
    const openDialog = page.getByRole("dialog");
    if (await openDialog.count()) {
      await expect.poll(async () => {
        const bounds = await openDialog.boundingBox();
        return bounds !== null && bounds.x >= 0 && bounds.y >= 0
          && bounds.x + bounds.width <= viewport.width
          && bounds.y + bounds.height <= viewport.height;
      }).toBe(true);
      await expect.poll(() => openDialog.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
      await expect(openDialog.getByRole("button", { name: copy.create.template.use_template, exact: true })).toBeInViewport({ ratio: 1 });
    }
    const name = `skill-templates-${locale}-${theme}-${state}-${viewport.width}x${viewport.height}`;
    const path = testInfo.outputPath(`${name}.png`);
    await page.screenshot({ path, animations: "disabled" });
    await testInfo.attach(name, { path, contentType: "image/png" });
    visualStates.push({ name, locale, theme, viewport, entryHeight: entryBounds!.height,
      templateCount: catalog.templates.length, workspaceSkillCount: skills.length });
  };
  for (const [locale, copy] of [["en", enSkills], ["zh-Hans", zhSkills]] as const) {
    await setLocale(page, localized, locale);
    const theme = locale === "en" ? "light" : "dark";
    await page.goto(`/${slug}/skills`);
    await page.evaluate((value) => localStorage.setItem("theme", value), theme);
    await page.reload();
    await expect(page.locator("html")).toHaveClass(new RegExp(`\\b${theme}\\b`));
    for (const [index, skill] of copies.entries()) {
      const displayed = copy.builtin_role_skills[skill.name];
      await page.setViewportSize({ width: 1280, height: 720 });
      await page.goto(`/${slug}/skills`);
      await page.getByRole("tab", { name: copy.market.workspace, exact: true }).click();
      const entry = page.getByRole("tablist", { name: copy.market.views_label, exact: true, includeHidden: true });
      await expect(entry.getByRole("tab", { name: copy.market.title, exact: true, includeHidden: true })).toContainText(String(catalog.templates.length));
      await expect(page.locator("header").getByText(String(skills.length), { exact: true })).toHaveCount(0);
      await expect(entry.getByRole("tab", { name: copy.market.workspace, exact: true })).toContainText(String(skills.length));
      const fromTemplate = page.getByRole("button", { name: copy.market.from_template, exact: true });
      if (index === 0) {
        await capture("workspace", locale, copy, theme);
        await page.setViewportSize({ width: 375, height: 667 });
        await expect(page.getByRole("textbox", { name: copy.market.workspace_search, exact: true })).toBeVisible();
        await expect(fromTemplate).toBeInViewport({ ratio: 1 });
        await capture("workspace", locale, copy, theme);
        await page.setViewportSize({ width: 360, height: 800 });
        await expect(fromTemplate).toBeInViewport({ ratio: 1 });
        await capture("workspace", locale, copy, theme);
        await page.setViewportSize({ width: 1280, height: 720 });
      }
      const search = page.getByRole("textbox", { name: copy.market.workspace_search, exact: true });
      for (const purpose of [enSkills.builtin_role_skills[skill.name].description, zhSkills.builtin_role_skills[skill.name].description]) {
        await search.fill(purpose);
        await expect(page.getByText(displayed.name, { exact: true }).first()).toBeVisible();
        await expect(page.getByText(displayed.summary, { exact: true }).first()).toBeVisible();
        await expect(page.getByTitle(displayed.description, { exact: true }).first()).toBeVisible();
      }
      await expect(entry.getByRole("tab", { name: copy.market.title, exact: true, includeHidden: true })).toContainText(String(catalog.templates.length));
      await fromTemplate.click();
      await expect(entry.getByRole("tab", { name: copy.market.title, exact: true })).toBeFocused();
      await page.getByRole("button", { name: copy.market.preview_label.replace("{{name}}", displayed.name), exact: true }).click();
      const preview = page.getByRole("dialog");
      await expect(preview).toHaveAccessibleName(copy.create.template.preview_label);
      await expect(preview.getByRole("textbox", { name: copy.create.template.search_placeholder })).toBeFocused();
      await preview.getByRole("textbox", { name: copy.create.template.search_placeholder }).fill(skill.name);
      const templateRow = preview.getByRole("button", { name: new RegExp(`^${displayed.name}`) });
      await expect(templateRow).toContainText(displayed.summary);
      if (index === 0) {
        await capture("list", locale, copy, theme);
        await page.setViewportSize({ width: 375, height: 667 });
        await preview.getByRole("button", { name: copy.create.template.back_to_templates, exact: true }).click();
        await capture("list", locale, copy, theme);
      }
      await templateRow.focus();
      await page.keyboard.press("Enter");
      await expect(preview.getByText(displayed.description, { exact: true })).toBeVisible();
      const relatedCount = "related_count_one" in copy.create.template
        ? copy.create.template.related_count_one
        : copy.create.template.related_count_other;
      await expect(preview.getByText(relatedCount.replace("{{count}}", "1"), { exact: true })).toBeVisible();
      const relatedLink = preview.getByRole("link", { name: displayed.name, exact: true });
      await expect(relatedLink).toHaveAttribute("href", `/${slug}/skills/${skill.id}`);
      if (index === 0) {
        await expect(preview.getByRole("button", { name: copy.create.template.use_template, exact: true })).toBeFocused();
        await capture("preview", locale, copy, theme);
        await page.setViewportSize({ width: 1280, height: 720 });
        await capture("preview", locale, copy, theme);
      }
      await relatedLink.focus();
      await page.keyboard.press("Enter");
      await expect(page).toHaveURL(`/${slug}/skills/${skill.id}`);
      await expect(preview).toHaveCount(0);
      await expect(page.getByRole("heading", { level: 1, name: displayed.name, exact: true })).toBeVisible();
      if (locale === "en") await expect(page.getByText(displayed.description, { exact: true })).toBeVisible();
      expect(await api.requestJSON(`/api/skills/${skill.id}`)).toEqual(skill.snapshot);
    }
  }
  // These locales share the same creation semantics. Keep only the narrow
  // label/keyboard smoke here; the full flow remains canonical above.
  for (const [locale, copy, theme] of [["en", enSkills, "light"], ["zh-Hans", zhSkills, "dark"]] as const) {
    await setLocale(page, localized, locale);
    await page.setViewportSize({ width: 360, height: 800 });
    await page.goto(`/${slug}/skills`);
    await page.evaluate((value) => localStorage.setItem("theme", value), theme);
    await page.reload();
    const entry = page.getByRole("tablist", { name: copy.market.views_label, exact: true });
    await expect(entry.getByRole("tab", { name: copy.market.title, exact: true, includeHidden: true })).toContainText(String(catalog.templates.length));
    await entry.getByRole("tab", { name: copy.market.title, exact: true }).click();
    const displayed = copy.builtin_role_skills["multica-experience-validation"];
    const browse = page.getByRole("button", { name: copy.market.preview_label.replace("{{name}}", displayed.name), exact: true });
    await browse.scrollIntoViewIfNeeded();
    await expect(browse).toBeInViewport({ ratio: 1 });
    await capture("catalog", locale, copy, theme);
    await browse.focus();
    await page.keyboard.press("Enter");
    const picker = page.getByRole("dialog", { name: copy.create.template.preview_label, exact: true });
    // A named market card opens details on narrow screens; Back exposes the
    // original picker so its source tabs and keyboard behavior stay covered.
    await picker.getByRole("button", { name: copy.create.template.back_to_templates, exact: true }).click();
    const search = picker.getByRole("textbox", { name: copy.create.template.search_placeholder });
    await expect(search).toBeFocused();
    await capture("list", locale, copy, theme);
    const title = picker.getByRole("heading", { name: copy.create.template.preview_label, exact: true });
    await expect.poll(() => title.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
    await expect(picker.getByRole("tab")).toHaveCount(2);
    for (const tab of await picker.getByRole("tab").all()) {
      await expect(tab).toBeInViewport({ ratio: 1 });
      await expect.poll(() => tab.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
      // The transparent ::after indicator extends scrollHeight below the tab.
      // DOM range rectangles measure label/count fit and expose clipped text.
      await expect.poll(() => tab.evaluate((element) => {
        const bounds = element.getBoundingClientRect();
        const range = document.createRange();
        range.selectNodeContents(element);
        const textBounds = Array.from(range.getClientRects()).filter((rect) => rect.width > 0 && rect.height > 0);
        return textBounds.length > 0 && textBounds.every((rect) => rect.left >= bounds.left && rect.right <= bounds.right
          && rect.top >= bounds.top && rect.bottom <= bounds.bottom);
      })).toBe(true);
    }
    await search.fill("multica-experience-validation");
    const row = picker.getByRole("button", { name: new RegExp(`^${displayed.name}`) });
    await expect(row).toContainText(displayed.summary);
    await row.focus();
    await page.keyboard.press("Enter");
    await expect(picker.getByText(displayed.description, { exact: true })).toBeVisible();
    await capture("preview", locale, copy, theme);
    await picker.getByRole("button", { name: copy.create.template.back_to_templates, exact: true }).click();
    await expect(row).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(picker).toHaveCount(0);
    await expect(browse).toBeFocused();
  }
  expect(writes).toEqual([]);
  await testInfo.attach("localized-skill-template-visual-evidence", {
    body: Buffer.from(JSON.stringify({ visualStates, browserSkillWrites: writes }, null, 2)), contentType: "application/json",
  });
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
