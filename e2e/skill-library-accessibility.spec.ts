import { test, expect as baseExpect, type Page, type TestInfo } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { TestApiClient } from "./fixtures";

const expect = baseExpect.configure({ timeout: 20_000 });

async function capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = info.outputPath(`${name}.png`);
  await page.screenshot({ path, animations: "disabled" });
  await info.attach(name, { path, contentType: "image/png" });
}

test("skill library supports keyboard selection, filter recovery and compact layouts", async ({ page }, info) => {
  test.setTimeout(240_000);
  page.setDefaultTimeout(20_000);
  page.setDefaultNavigationTimeout(120_000);
  const api = new TestApiClient();
  const slug = `skill-library-audit-${Date.now().toString(36)}`;
  await api.login(`${slug}@multica.ai`, "技能库验证");
  const workspace = await api.ensureWorkspace("技能库验证", slug);
  expect(workspace.slug).toBe(slug);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    const catalog = JSON.parse(await readFile(new URL("../packages/views/locales/zh-Hans/skills.json", import.meta.url), "utf8"));
    const definitions = [
      ["multica-code-review", "quality"],
      ["multica-security-review", "quality"],
      ["multica-rollout-and-canary-verification", "operations"],
      ["multica-architecture-decision-record", "design"],
      ["multica-requirement-clarification", "research"],
      ["multica-documentation-change", "writing"],
    ];
    const skills: { id: string; name: string; description: string }[] = [];
    for (const [name, category] of definitions) {
      const copy = catalog.builtin_role_skills[name!];
      skills.push(await api.requestJSON("/api/skills", { method: "POST", body: {
        name, description: copy.description, content: `# ${name}\n\n${copy.description}\n`,
        config: { presentation: { category }, origin: { type: "builtin_role_skill", name, version: 1 } },
      } }));
    }
    for (const name of ["上线验证", "回滚方案", "关键路径"]) {
      const label = await api.requestJSON<{ id: string }>("/api/labels", { method: "POST", body: {
        name, resource_type: "skill", color: "#64748b",
      } });
      await api.requestJSON(`/api/skills/${skills[2]!.id}/labels`, { method: "POST", body: { label_id: label.id } });
    }
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "system");
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, token);
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: String(info.project.use.baseURL) }]);
    await page.emulateMedia({ colorScheme: "light" });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${slug}/skills`);
    const cards = page.getByTestId("skill-card");
    await expect(cards).toHaveCount(6);
    const codeTitle = page.getByRole("link", { name: "代码审查", exact: true });
    const codeCard = cards.filter({ has: codeTitle });
    await expect(codeCard.getByText("手动创建", { exact: true })).toBeVisible();
    await expect(codeCard.getByText(catalog.builtin_role_skills["multica-code-review"].summary, { exact: true })).toBeVisible();

    const select = codeCard.getByRole("checkbox", { name: "选择 代码审查", exact: true });
    await select.focus();
    await page.keyboard.press("Space");
    await expect(select).toBeChecked();
    await expect(page).toHaveURL(new RegExp(`/${slug}/skills$`));
    await page.keyboard.press("Space");
    await expect(select).not.toBeChecked();

    await codeTitle.focus();
    await expect(codeTitle).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/skills/${skills[0]!.id}`));
    await page.goBack();
    await expect(cards).toHaveCount(6);

    const display = page.getByRole("button", { name: /^排序:/ });
    await display.click();
    await expect(page.getByText("显示列", { exact: true })).toHaveCount(0);
    await page.keyboard.press("Escape");

    await page.getByRole("button", { name: "列表视图", exact: true }).click();
    const selectAll = page.getByRole("checkbox", { name: "全选 skill", exact: true });
    await selectAll.focus();
    await page.keyboard.press("Space");
    await expect(selectAll).toBeChecked();
    await expect(page.getByRole("checkbox", { name: "选择 代码审查", exact: true })).toBeChecked();
    await page.keyboard.press("Space");
    await expect(selectAll).not.toBeChecked();
    await page.getByRole("button", { name: "卡片视图", exact: true }).click();

    // Exercise multiple categories through the real nested filter menu.
    const filter = page.getByRole("button", { name: /^筛选/ });
    for (const category of ["测试与质量", "发布与运维"]) {
      await filter.click();
      const categoryMenu = page.getByRole("menuitem", { name: /^分类/ });
      await categoryMenu.focus();
      await page.keyboard.press("ArrowRight");
      const categoryOption = page.getByRole("menuitemcheckbox", { name: new RegExp(category) });
      await categoryOption.click();
      await categoryOption.press("Escape");
      await expect(categoryOption).toBeHidden();
      if (await filter.getAttribute("aria-expanded") === "true") {
        await categoryMenu.press("Escape");
      }
      await expect(filter).toHaveAttribute("aria-expanded", "false");
    }
    const categories = page.getByRole("navigation", { name: "分类" });
    await expect(categories.getByRole("button", { name: /^全部/ })).toHaveAttribute("aria-pressed", "false");
    await expect(categories.getByRole("button", { name: /^测试与质量/ })).toHaveAttribute("aria-pressed", "true");
    await expect(categories.getByRole("button", { name: /^发布与运维/ })).toHaveAttribute("aria-pressed", "true");
    await expect(cards).toHaveCount(3);
    await page.mouse.move(0, 0);
    await filter.focus();
    await expect(filter).toBeFocused();
    await filter.press("Enter");
    const clearFilters = page.getByRole("menuitem", { name: "清除筛选", exact: true });
    await expect(clearFilters).toBeVisible();
    await expect(page.getByRole("menuitem").first()).toBeFocused();
    await page.keyboard.press("End");
    await expect(clearFilters).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(cards).toHaveCount(6);

    const search = page.getByRole("textbox", { name: "搜索工作空间 skill..." });
    await search.fill("不存在的内容");
    await expect(page.getByText(/没有 skill 匹配/)).toBeVisible();
    await page.getByRole("button", { name: "清空搜索", exact: true }).click();
    await expect(search).toBeFocused();
    await expect(cards).toHaveCount(6);
    await capture(page, info, "skills-desktop-light");
    await page.emulateMedia({ colorScheme: "dark" });
    await expect(page.locator("html")).toHaveClass(/dark/);
    await capture(page, info, "skills-desktop-dark");
    await page.emulateMedia({ colorScheme: "light" });
    await expect(page.locator("html")).not.toHaveClass(/dark/);

    const cdp = await page.context().newCDPSession(page);
    await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 1 });
    expect(await page.evaluate(() => matchMedia("(pointer: coarse)").matches)).toBe(true);
    for (const width of [375, 320]) {
      await page.setViewportSize({ width, height: 900 });
      await expect(filter).toBeVisible();
      await expect(filter).toHaveAccessibleName("筛选");
      await search.fill("不存在的内容");
      await page.getByRole("button", { name: "清空搜索", exact: true }).click();
      await expect(search).toBeFocused();
      expect(await page.evaluate(() => Math.max(document.body.scrollWidth, document.documentElement.scrollWidth) <= innerWidth)).toBe(true);
      const longCard = cards.filter({ has: page.getByRole("link", { name: "发布后与 Canary 验证", exact: true }) });
      await longCard.scrollIntoViewIfNeeded();
      expect(await longCard.evaluate((element) => element.scrollHeight <= element.clientHeight)).toBe(true);
      await cards.first().scrollIntoViewIfNeeded();
      const actions = cards.first().getByRole("button", { name: "操作", exact: true });
      await expect(actions).toBeVisible();
      const actionBounds = await actions.boundingBox();
      expect(actionBounds!.height).toBeGreaterThanOrEqual(44);
      await expect(actions).toHaveCSS("opacity", "1");
      const rectangles = await cards.evaluateAll((elements) => elements.map((element) => {
        const rect = element.getBoundingClientRect();
        return { top: rect.top, bottom: rect.bottom };
      }));
      for (let i = 1; i < rectangles.length; i++) {
        expect(rectangles[i]!.top).toBeGreaterThanOrEqual(rectangles[i - 1]!.bottom);
      }
      await capture(page, info, `skills-compact-${width}`);
    }

    const stored = await api.requestJSON<{ description: string }>(`/api/skills/${skills[0]!.id}`);
    expect(stored.description).toBe(skills[0]!.description);
    expect(errors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
