import { randomUUID } from "node:crypto";
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

async function openMarketSession(page: Page, info: TestInfo) {
  const api = new TestApiClient();
  const slug = `market-welcome-${randomUUID().slice(0, 12)}`;
  await api.login(`${slug}@example.invalid`, "Market welcome reviewer");
  const workspace = await api.ensureWorkspace("Market welcome review", slug);
  await api.markUserOnboarded();
  const token = api.getToken();
  if (!token) throw new Error("Market reviewer is not authenticated");
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
    document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
  }, token);
  const origin = info.project.use.baseURL;
  if (typeof origin !== "string") throw new Error("Playwright base URL is required");

  return {
    slug,
    async useLocale(locale: "en" | "zh-Hans") {
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: origin }]);
    },
    cleanup: () => api.deleteFeatureWorkspace(workspace.id),
  };
}

async function capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = info.outputPath(`${name}.png`);
  await page.screenshot({ path, animations: "disabled" });
  await info.attach(name, { path, contentType: "image/png" });
}

test("the market case follows category selection, browser history and category docs", async ({ page }, info) => {
  const session = await openMarketSession(page, info);
  const path = `/${session.slug}/marketplace`;
  const writes: string[] = [];
  page.on("request", (request) => {
    const pathname = new URL(request.url()).pathname;
    if (request.method() !== "GET" && /^\/api\/(?:workspaces\/[^/]+\/)?(?:mcp-servers|skills|agents|squads|projects)(?:\/|$)/.test(pathname)) {
      writes.push(`${request.method()} ${pathname}`);
    }
  });
  try {
    await session.useLocale("en");
    await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(path);
    await expect(page.locator("html")).toHaveClass(/\bdark\b/);
    await expect(page.getByRole("heading", { name: "Equip your team for the next task" })).toBeVisible();
    const example = page.getByRole("figure", { name: "Usage example" });
    const task = example.getByText("Check sign-up before release", { exact: true });
    const docs = example.getByRole("link", { name: "Read the docs", exact: true });
    const types = page.getByRole("tablist", { name: "Capability type" });
    const mcp = types.getByRole("tab", { name: "MCP", exact: true });
    const skill = types.getByRole("tab", { name: "Skills", exact: true });
    const selectedColor = await mcp.evaluate((element) => getComputedStyle(element).color);
    await mcp.hover();
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    expect(await mcp.evaluate((element) => getComputedStyle(element).color)).toBe(selectedColor);
    await expect(task).toBeVisible();
    await expect(example.getByText("Playwright", { exact: true })).toBeVisible();
    await expect(docs).toHaveAttribute("href", `/${session.slug}/docs/mcp`);
    await expect(page.getByTestId("mcp-market")).toBeVisible();
    await mcp.focus();
    await page.keyboard.press("ArrowRight");
    await expect(skill).toBeFocused();
    await expect(mcp).toHaveAttribute("aria-selected", "true");
    await expect(example.getByText("Playwright", { exact: true })).toBeVisible();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(`${path}?type=skills`);
    await expect(example.getByText("Sign-up checklist", { exact: true })).toBeVisible();
    await expect.poll(() => example.locator('[class*="marketplace-welcome_caseDetail"]').evaluate((element) => getComputedStyle(element).animationName)).toBe("none");
    await expect(docs).toHaveAttribute("href", `/${session.slug}/docs/skills`);
    await expect(page.getByRole("textbox", { name: "Search templates", exact: true })).toBeInViewport();
    await capture(page, info, "market-en-dark-wide");

    await page.goBack();
    await expect(example.getByText("Playwright", { exact: true })).toBeVisible();
    await page.goForward();
    await expect(example.getByText("Sign-up checklist", { exact: true })).toBeVisible();
    await types.getByRole("tab", { name: "Squad templates", exact: true }).click();
    await expect(example.getByText("Release Squad", { exact: true })).toBeVisible();
    await expect(docs).toHaveAttribute("href", `/${session.slug}/docs/squads`);
    await types.getByRole("tab", { name: "Agent templates", exact: true }).click();
    await expect(example.getByText("QA Engineer", { exact: true })).toBeVisible();
    await expect(docs).toHaveAttribute("href", `/${session.slug}/docs/agents`);

    await session.useLocale("zh-Hans");
    await page.emulateMedia({ colorScheme: "light" });
    await page.goto(path);
    const chineseExample = page.getByRole("figure", { name: "使用示例" });
    await expect(chineseExample.getByText("发布前检查注册流程", { exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "为下一项任务配好能力" })).toBeVisible();
    await expect(page.getByRole("searchbox", { name: "搜索 MCP 模板" })).toBeInViewport();
    await capture(page, info, "market-zh-light-wide");
    expect(writes).toEqual([]);
  } finally {
    await session.cleanup();
  }
});

test.describe("compact capability market", () => {
  test.use({ hasTouch: true });

  test("all entries fit and the optional case scrolls with the role catalog", async ({ page }, info) => {
    const session = await openMarketSession(page, info);
    const path = `/${session.slug}/marketplace?type=agent-templates`;
    try {
      for (const layout of [
        { locale: "en", width: 390, height: 844, title: "Agent templates", task: "Check sign-up before release", example: "QA Engineer", useTemplate: "Use template", colorScheme: "dark" },
        { locale: "zh-Hans", width: 320, height: 700, title: "智能体模板", task: "发布前检查注册流程", example: "测试工程师", useTemplate: "使用模板", colorScheme: "light" },
      ] as const) {
        await session.useLocale(layout.locale);
        await page.emulateMedia({ colorScheme: layout.colorScheme, reducedMotion: "reduce" });
        await page.setViewportSize({ width: layout.width, height: layout.height });
        await page.goto(path);
        const tabs = page.getByRole("tablist", { name: layout.locale === "en" ? "Capability type" : "能力类型" });
        const tab = tabs.getByRole("tab", { name: layout.title, exact: true });
        const panel = page.getByRole("tabpanel");
        const disclosure = panel.locator("details");
        const summary = disclosure.locator("summary");
        await expect(tab).toHaveAttribute("aria-selected", "true");
        await expect(summary).toBeInViewport();
        await expect(disclosure).toHaveJSProperty("open", false);
        const entries = await tabs.getByRole("tab").evaluateAll((elements) => elements.map((element) => {
          const box = element.getBoundingClientRect();
          return { left: box.left, right: box.right, top: box.top, height: box.height, scroll: element.scrollWidth, client: element.clientWidth };
        }));
        expect(entries).toHaveLength(4);
        expect(entries[0]!.top).toBe(entries[1]!.top);
        expect(entries[2]!.top).toBe(entries[3]!.top);
        expect(entries[2]!.top).toBeGreaterThan(entries[0]!.top);
        for (const entry of entries) {
          expect(entry.left).toBeGreaterThanOrEqual(0);
          expect(entry.right).toBeLessThanOrEqual(layout.width);
          expect(entry.height).toBeGreaterThanOrEqual(44);
          expect(entry.scroll).toBeLessThanOrEqual(entry.client + 1);
        }
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        await capture(page, info, `market-${layout.locale}-${layout.width}-closed`);

        await summary.focus();
        await page.keyboard.press("Enter");
        await expect(disclosure).toHaveJSProperty("open", true);
        await expect(disclosure.getByText(layout.task, { exact: true })).toBeVisible();
        await expect(disclosure.getByText(layout.example, { exact: true })).toBeVisible();
        await capture(page, info, `market-${layout.locale}-${layout.width}-example`);
        const roleCatalog = panel.getByRole("region", { name: layout.title, exact: true });
        const lastTemplate = roleCatalog.getByRole("link", { name: layout.useTemplate, exact: true }).last();
        await lastTemplate.scrollIntoViewIfNeeded();
        await expect(lastTemplate).toBeInViewport();
        expect(await roleCatalog.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
        await expect(tab).toHaveAccessibleName(layout.title);
      }
    } finally {
      await session.cleanup();
    }
  });
});
