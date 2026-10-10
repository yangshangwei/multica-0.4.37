import { test, expect, type Locator, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

type SavedAgent = { id: string; category: string; template_key?: string };

function agentRow(page: Page, name: string) {
  return page.getByRole("table").getByRole("row").filter({ has: page.getByRole("link", { name, exact: true }) });
}

async function groupLabel(row: Locator) {
  return row.evaluate((element) => {
    let sibling = element.previousElementSibling;
    while (sibling) {
      if (sibling.classList.contains("col-span-full") && sibling.getAttribute("role") !== "row") {
        return sibling.querySelector("span[title]")?.getAttribute("title");
      }
      sibling = sibling.previousElementSibling;
    }
    return null;
  });
}

test("saved categories drive the default directory, counts and filters after legacy preference migration", async ({ page }, info) => {
  test.setTimeout(180_000);
  const api = new TestApiClient();
  const slug = `category-grouping-${Date.now().toString(36)}`;
  await api.login(`${slug}@multica.ai`, "分类回归验证");
  const workspace = await api.ensureWorkspace("分类回归验证", slug);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    await page.addInitScript(({ token, slug }) => {
      localStorage.setItem("multica_token", token);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
      const key = `multica_agents_view:${slug}`;
      if (!localStorage.getItem(key)) {
        localStorage.setItem(key, JSON.stringify({ version: 0, state: {
          groupBy: "role", scope: "all", sortField: "name", sortDirection: "asc",
          filters: { roles: [], categories: [], squads: [] },
        } }));
      }
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, { token, slug });
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: String(info.project.use.baseURL) }]);
    const runtime = await api.seedProjectRuntime();
    const openCode = await api.requestJSON<SavedAgent>("/api/agents", { method: "POST", body: { name: "OpenCode", runtime_id: runtime.id } });
    await api.requestJSON("/api/agents", { method: "POST", body: { name: "通用助手", runtime_id: runtime.id } });
    const reviewer = await api.requestJSON<SavedAgent>("/api/agents/from-template", {
      method: "POST", body: { template_key: "security-reviewer", name: "安全审查员", runtime_id: runtime.id, language: "zh" },
    });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${slug}/agents/${openCode.id}?view=general`);
    await page.getByRole("button", { name: "展开分类列表", exact: true }).click();
    await page.getByRole("option", { name: "专业角色", exact: true }).click();
    await expect.poll(async () => (await api.requestJSON<SavedAgent>(`/api/agents/${openCode.id}`)).category).toBe("专业角色");

    // Return without changing Display: this is the previously broken path.
    await page.goto(`/${slug}/agents`);
    const row = agentRow(page, "OpenCode");
    await expect.poll(() => groupLabel(row)).toBe("专业角色");
    await expect(page.getByRole("button", { name: /^专业角色\s*2$/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /^通用智能体\s*1$/ })).toBeVisible();
    await expect.poll(() => page.evaluate((slug) => JSON.parse(localStorage.getItem(`multica_agents_view:${slug}`) || "{}").version, slug)).toBe(1);
    await page.screenshot({ path: info.outputPath("default-category-grouping-fixed-1440.png"), animations: "disabled" });
    await page.getByRole("button", { name: /^专业角色\s*2$/ }).click();
    await expect(row).toBeVisible();
    await expect(agentRow(page, "安全审查员")).toBeVisible();
    await expect(agentRow(page, "通用助手")).toHaveCount(0);
    await page.reload();
    await expect(row).toBeVisible();

    await page.goto(`/${slug}/agents/${openCode.id}?view=general`);
    const categoryInput = page.locator('input[name="agent-category"]');
    await categoryInput.fill("研发");
    await categoryInput.blur();
    await expect.poll(async () => (await api.requestJSON<SavedAgent>(`/api/agents/${openCode.id}`)).category).toBe("研发");
    await page.goto(`/${slug}/agents`);
    await expect(page.getByRole("button", { name: /^专业角色\s*1$/ })).toBeVisible();
    await expect(row).toHaveCount(0);
    await page.getByRole("button", { name: "全部分类", exact: true }).click();
    await expect.poll(() => groupLabel(row)).toBe("研发");
    await page.getByRole("button", { name: "分类", exact: true }).click();
    await page.getByRole("menuitemcheckbox", { name: /^研发\s*1$/ }).click();
    await page.keyboard.press("Escape");
    // Wait for the desktop-positioned menu to finish its exit before resizing.
    await expect(page.getByRole("menu", { includeHidden: true })).toHaveCount(0);
    await expect(row).toBeVisible();
    await expect(agentRow(page, "安全审查员")).toHaveCount(0);
    await page.getByRole("button", { name: "全部分类", exact: true }).click();
    await page.setViewportSize({ width: 390, height: 1000 });
    expect(await page.evaluate(() => Math.max(document.body.scrollWidth, document.documentElement.scrollWidth) <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath("custom-category-grouping-fixed-390.png"), animations: "disabled" });

    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${slug}/agents/${openCode.id}?view=general`);
    await page.getByRole("button", { name: "展开分类列表", exact: true }).click();
    await page.getByRole("option", { name: "使用默认分类", exact: true }).click();
    await expect.poll(async () => (await api.requestJSON<SavedAgent>(`/api/agents/${openCode.id}`)).category).toBe("");
    await page.goto(`/${slug}/agents`);
    await expect.poll(() => groupLabel(row)).toBe("通用智能体");
    await expect(page.getByRole("button", { name: /^通用智能体\s*2$/ })).toBeVisible();
    expect((await api.requestJSON<SavedAgent>(`/api/agents/${reviewer.id}`)).template_key).toBe("security-reviewer");
    expect((await api.requestJSON<SavedAgent>(`/api/agents/${openCode.id}`)).template_key).toBeFalsy();
    expect(errors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
