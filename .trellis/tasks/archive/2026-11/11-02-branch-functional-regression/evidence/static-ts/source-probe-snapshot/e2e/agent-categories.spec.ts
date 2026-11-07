import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

type SavedAgent = { id: string; name: string; description: string; category: string; template_key?: string };

function agentRow(page: Page, name: string) {
  return page.getByRole("table").getByRole("row").filter({ has: page.getByRole("link", { name, exact: true }) });
}

test("custom categories survive creation, editing and clearing and compose with directory grouping", async ({ page }, info) => {
  test.setTimeout(180_000);
  const api = new TestApiClient();
  const slug = `agent-categories-${Date.now().toString(36)}`;
  await api.login(`${slug}@multica.ai`, "分类验证员");
  const workspace = await api.ensureWorkspace("智能体分类验证", slug);
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));

  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    const token = api.getToken();
    if (!token) throw new Error("Fixture login did not return a token");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, token);
    const baseURL = info.project.use.baseURL;
    if (typeof baseURL !== "string") throw new Error("Missing browser base URL");
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: baseURL }]);
    const runtime = await api.seedProjectRuntime();
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/agents/new/manual`);
    await page.getByRole("button", { name: "展开分类列表", exact: true }).click();
    await expect(page.getByText(/还没有自定义分类。可以选择默认分类/)).toBeVisible();
    for (const name of ["通用智能体", "专业角色", "统筹角色"]) {
      await expect(page.getByRole("option", { name, exact: true })).toBeVisible();
    }
    await page.screenshot({ path: info.outputPath("category-dropdown-empty-1440-zh.png"), animations: "disabled" });
    await page.keyboard.press("Escape");
    const reviewer = await api.requestJSON<SavedAgent>("/api/agents/from-template", {
      method: "POST", body: { template_key: "code-reviewer", runtime_id: runtime.id, name: "研发审查员", category: "研发", language: "zh" },
    });
    expect(reviewer.category).toBe("研发");
    const writer = await api.requestJSON<SavedAgent>("/api/agents", {
      method: "POST", body: { runtime_id: runtime.id, name: "内容助手", category: "内容" },
    });

    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/agents/new/manual`);
    const categoryInput = page.locator('input[name="agent-category"]');
    await expect(categoryInput).toBeVisible();
    await page.getByRole("button", { name: "展开分类列表", exact: true }).click();
    await expect(page.getByRole("option", { name: "研发", exact: true })).toBeVisible();
    await page.screenshot({ path: info.outputPath("category-dropdown-existing-1440-zh.png"), animations: "disabled" });
    await page.getByRole("option", { name: "研发", exact: true }).click();
    await expect(categoryInput).toHaveValue("研发");
    await page.getByRole("textbox", { name: "名称", exact: true }).fill("运营助手");
    await categoryInput.fill("  运营  ");
    await expect(page.getByRole("option", { name: '使用新分类 "运营"', exact: true })).toBeVisible();
    await expect(page.getByText(/创建智能体时保存/)).toBeVisible();
    await page.screenshot({ path: info.outputPath("category-dropdown-new-1440-zh.png"), animations: "disabled" });
    await page.getByRole("option", { name: '使用新分类 "运营"', exact: true }).click();
    const createResponse = page.waitForResponse((response) => response.request().method() === "POST" && new URL(response.url()).pathname === "/api/agents");
    await page.getByRole("button", { name: "创建并打开", exact: true }).click();
    const createdResponse = await createResponse;
    expect(createdResponse.status()).toBe(201);
    const created: SavedAgent = await createdResponse.json();
    expect(created.category).toBe("运营");
    await expect(page).toHaveURL(new RegExp(`/agents/${created.id}`));

    await page.goto(`/${workspace.slug}/agents/${created.id}?view=general`);
    await expect(categoryInput).toHaveValue("运营");
    await expect(page.getByText(/修改后自动保存/)).toBeVisible();
    await page.getByRole("button", { name: "展开分类列表", exact: true }).click();
    await page.getByRole("option", { name: "专业角色", exact: true }).click();
    await expect.poll(async () => (await api.requestJSON<SavedAgent>(`/api/agents/${created.id}`)).category).toBe("专业角色");
    expect((await api.requestJSON<SavedAgent>(`/api/agents/${created.id}`)).template_key).toBeFalsy();
    await page.reload();
    await expect(categoryInput).toHaveValue("专业角色");
    await page.getByRole("button", { name: "展开分类列表", exact: true }).click();
    await expect(page.getByRole("option", { name: "专业角色", exact: true })).toHaveCount(1);
    await page.getByRole("option", { name: "研发", exact: true }).click();
    await expect.poll(async () => (await api.requestJSON<SavedAgent>(`/api/agents/${created.id}`)).category).toBe("研发");
    await page.reload();
    await expect(categoryInput).toHaveValue("研发");

    // A remote category edit must remain authoritative when this tab only
    // changes another profile field. WebSocket invalidation uses the real API.
    await api.requestJSON(`/api/agents/${created.id}`, { method: "PUT", body: { category: "协作" } });
    await expect(categoryInput).toHaveValue("协作");
    const description = page.getByRole("textbox", { name: "描述", exact: true });
    await description.fill("支持跨窗口协作。");
    await description.blur();
    await expect.poll(async () => (await api.requestJSON<SavedAgent>(`/api/agents/${created.id}`)).description).toBe("支持跨窗口协作。");
    expect((await api.requestJSON<SavedAgent>(`/api/agents/${created.id}`)).category).toBe("协作");
    await categoryInput.fill("研发");
    await categoryInput.blur();
    await expect.poll(async () => (await api.requestJSON<SavedAgent>(`/api/agents/${created.id}`)).category).toBe("研发");
    await page.screenshot({ path: info.outputPath("category-settings-1440-zh.png"), animations: "disabled" });

    await page.goto(`/${workspace.slug}/agents`);
    await expect(agentRow(page, created.name)).toContainText("研发");
    await page.getByRole("button", { name: "分类", exact: true }).click();
    await page.getByRole("menuitemcheckbox", { name: /^研发/ }).click();
    await page.keyboard.press("Escape");
    await expect(agentRow(page, created.name)).toBeVisible();
    await expect(agentRow(page, reviewer.name)).toBeVisible();
    await expect(agentRow(page, writer.name)).toHaveCount(0);
    await page.getByRole("button", { name: "全部分类", exact: true }).click();
    await expect(agentRow(page, writer.name)).toBeVisible();
    await page.getByRole("button", { name: "分类", exact: true }).click();
    await page.getByRole("menuitem", { name: "全部分类", exact: true }).click();
    await page.getByRole("button", { name: "显示", exact: true }).click();
    await page.getByRole("group", { name: "分组方式", exact: true }).getByRole("button", { name: "分类", exact: true }).click();
    await page.keyboard.press("Escape");
    await page.reload();
    await expect(agentRow(page, writer.name)).toBeVisible();
    await page.getByRole("button", { name: "显示", exact: true }).click();
    await expect(page.getByRole("group", { name: "分组方式", exact: true }).getByRole("button", { name: "分类", exact: true })).toHaveAttribute("aria-pressed", "true");
    await page.keyboard.press("Escape");

    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 1000 });
      await page.evaluate(() => document.fonts.ready);
      await expect.poll(() => page.evaluate(() => Math.max(document.body.scrollWidth, document.documentElement.scrollWidth) <= innerWidth)).toBe(true);
      await page.screenshot({ path: info.outputPath(`categories-grouped-${width}-zh.png`), animations: "disabled" });
    }

    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/agents/${created.id}?view=general`);
    await categoryInput.fill("");
    await categoryInput.blur();
    await expect.poll(async () => (await api.requestJSON<SavedAgent>(`/api/agents/${created.id}`)).category).toBe("");
    await page.reload();
    await expect(categoryInput).toHaveValue("");
    await page.setViewportSize({ width: 390, height: 1000 });
    await categoryInput.scrollIntoViewIfNeeded();
    await page.getByRole("button", { name: "展开分类列表", exact: true }).click();
    await expect(page.getByRole("option", { name: "研发", exact: true })).toBeVisible();
    expect(await page.evaluate(() => Math.max(document.body.scrollWidth, document.documentElement.scrollWidth) <= innerWidth)).toBe(true);
    await expect(page.getByRole("option", { name: "使用默认分类", exact: true })).toHaveAttribute("aria-selected", "true");
    await page.screenshot({ path: info.outputPath("category-dropdown-390-zh.png"), animations: "disabled" });
    await page.keyboard.press("Escape");
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/agents`);
    await page.getByRole("button", { name: "分类", exact: true }).click();
    await page.getByRole("menuitemcheckbox", { name: /^通用智能体/ }).click();
    await page.keyboard.press("Escape");
    await expect(agentRow(page, created.name)).toBeVisible();
    await expect(agentRow(page, reviewer.name)).toHaveCount(0);
    expect((await api.requestJSON<SavedAgent>(`/api/agents/${reviewer.id}`)).template_key).toBe("code-reviewer");
    expect(pageErrors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
