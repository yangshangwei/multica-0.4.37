import { expect, test, type Locator, type Page, type TestInfo } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { TestApiClient } from "./fixtures";

const englishTitle = "Connect your agents to your tools";
const chineseTitle = "让智能体连接你的工具";

async function capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = info.outputPath(`${name}.png`);
  await page.screenshot({ path, animations: "disabled" });
  await info.attach(name, { path, contentType: "image/png" });
}

async function layoutMetrics(welcome: Locator) {
  return welcome.evaluate((element) => {
    const active = element.querySelector<HTMLElement>('[role="group"][data-active="true"]');
    if (!active) throw new Error("Active MCP example is missing");
    return {
      documentWidth: document.documentElement.scrollWidth,
      viewportWidth: innerWidth,
      contentWidth: element.scrollWidth,
      clientWidth: element.clientWidth,
      visibleScenes: element.querySelectorAll('[role="group"]:not([inert])').length,
      cardRadius: getComputedStyle(active).borderTopLeftRadius,
    };
  });
}

test("empty MCP explains configuration and assignment before the first saved server", async ({ page }, info) => {
  test.setTimeout(180_000);
  const api = new TestApiClient();
  const run = `${Date.now().toString(36)}-${info.parallelIndex}`;
  await api.login(`mcp-welcome-${run}@example.invalid`, "MCP welcome acceptance");
  const workspace = await api.ensureWorkspace("MCP welcome acceptance", `mcp-welcome-${run}`);
  expect(workspace.slug).toBe(`mcp-welcome-${run}`);
  const directory = `/${workspace.slug}/mcp`;
  const inventoryPath = `/api/workspaces/${workspace.id}/mcp-servers`;
  const pageErrors: string[] = [];
  const resourceWrites: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (/^(POST|PUT|PATCH|DELETE)$/.test(request.method()) &&
      (path.startsWith(inventoryPath) || /^\/api\/agents(?:\/|$)/.test(path))) {
      resourceWrites.push(`${request.method()} ${path}`);
    }
  });

  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const token = api.getToken();
    if (!token) throw new Error("Fixture authentication failed");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, token);
    const origin = info.project.use.baseURL;
    if (typeof origin !== "string") throw new Error("Playwright baseURL is required");
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: origin }]);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.setViewportSize({ width: 1440, height: 900 });

    // Use genuinely empty directories as the visual authority, before creating agents or skills.
    for (const reference of [
      { route: "skills", title: "Turn your team’s know-how into a skill" },
      { route: "agents", title: "Meet your first AI teammate" },
    ]) {
      await page.goto(`/${workspace.slug}/${reference.route}`);
      await expect(page.getByRole("region", { name: reference.title, exact: true })).toBeVisible();
      await capture(page, info, `reference-${reference.route}-empty`);
    }

    await page.goto(directory);
    const welcome = page.getByRole("region", { name: englishTitle, exact: true });
    const examples = page.getByRole("region", { name: "How MCP works", exact: true });
    const create = welcome.getByRole("button", { name: "Add custom server", exact: true });
    const market = welcome.getByRole("button", { name: "Browse capability market", exact: true });
    await expect(welcome).toBeVisible();
    await expect(page).toHaveURL(directory);
    await expect(page.getByRole("heading", { level: 1, name: "MCP", exact: true })).toHaveCount(1);
    await expect(page.getByRole("button", { name: "Add custom server", exact: true })).toHaveCount(1);
    await expect(page.getByRole("button", { name: "Browse capability market", exact: true })).toHaveCount(1);

    await create.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("heading", { name: "Add MCP Server", exact: true })).toBeVisible();
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(create).toBeFocused();

    const previous = examples.getByRole("button", { name: "Previous example", exact: true });
    const next = examples.getByRole("button", { name: "Next example", exact: true });
    await expect(previous).toBeDisabled();
    await next.focus();
    await page.keyboard.press("Enter");
    const configuration = examples.getByRole("group", { name: "Configure the connection", exact: true });
    await expect(configuration).toBeVisible();
    expect(await configuration.evaluate((element) => getComputedStyle(element.parentElement!).transitionDuration)).toBe("0s");
    await capture(page, info, "configuration-en-desktop");
    await next.click();
    await expect(examples.getByRole("heading", { name: "Assign it to your agents", exact: true })).toBeVisible();
    await expect(next).toBeDisabled();
    await capture(page, info, "assignment-en-desktop");
    await examples.getByRole("button", { name: /^Show example 1:/ }).click();
    await expect(examples.getByRole("group", { name: "Choose a service", exact: true })).not.toHaveAttribute("inert");
    await expect(examples.locator('[role="group"][inert][aria-hidden="true"]')).toHaveCount(2);
    expect(resourceWrites, "Opening and cancelling creation or exploring examples must not save or assign resources").toEqual([]);

    const layouts = [];
    for (const locale of ["en", "zh-Hans"] as const) {
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: origin }]);
      await page.goto(directory);
      const region = page.getByRole("region", { name: locale === "en" ? englishTitle : chineseTitle, exact: true });
      await expect(region).toBeVisible();
      for (const theme of ["light", "dark"] as const) {
        await page.evaluate((value) => {
          document.documentElement.classList.remove("light", "dark");
          document.documentElement.classList.add(value);
          document.documentElement.style.colorScheme = value;
        }, theme);
        for (const viewport of [
          { width: 1440, height: 900 },
          { width: locale === "en" ? 390 : 320, height: locale === "en" ? 844 : 568 },
        ]) {
          await page.setViewportSize(viewport);
          await region.evaluate((element) => { element.scrollTop = 0; });
          const browse = region.getByRole("button", { name: locale === "en" ? "Browse capability market" : "浏览能力市场", exact: true });
          await expect(browse).toBeInViewport();
          const metrics = await layoutMetrics(region);
          layouts.push({ locale, theme, ...viewport, ...metrics });
          expect(metrics.documentWidth).toBeLessThanOrEqual(metrics.viewportWidth);
          expect(metrics.contentWidth).toBeLessThanOrEqual(metrics.clientWidth + 1);
          expect(metrics.visibleScenes).toBe(1);
          expect(parseFloat(metrics.cardRadius)).toBeGreaterThan(0);
          await capture(page, info, `${locale}-${viewport.width}-${theme}`);
          if (viewport.width < 400) {
            const custom = region.getByRole("button", { name: locale === "en" ? "Add custom server" : "自定义添加", exact: true });
            await custom.scrollIntoViewIfNeeded();
            await expect(custom).toBeInViewport();
            const control = region.getByRole("button", { name: locale === "en" ? "Next example" : "下一个示例", exact: true });
            await control.scrollIntoViewIfNeeded();
            await expect(control).toBeInViewport();
            await capture(page, info, `${locale}-${viewport.width}-${theme}-examples`);
          }
        }
      }
    }
    await writeFile(info.outputPath("layout-metrics.json"), JSON.stringify(layouts, null, 2));

    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: origin }]);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(directory);
    await market.click();
    await expect(page).toHaveURL(`/${workspace.slug}/marketplace?type=mcp`);
    await page.goBack();
    await expect(welcome).toBeVisible();
    expect(resourceWrites).toEqual([]);

    await create.click();
    await dialog.getByRole("textbox", { name: "Server name", exact: true }).fill("welcome-server");
    await dialog.getByRole("button", { name: /^Streamable HTTP/ }).click();
    await dialog.getByRole("textbox", { name: "Server URL", exact: true }).fill("https://welcome.example.invalid/mcp");
    const created = page.waitForResponse((response) =>
      response.request().method() === "POST" && new URL(response.url()).pathname === inventoryPath);
    await dialog.getByRole("button", { name: "Add", exact: true }).click();
    const creation = await created;
    expect(creation.status()).toBe(201);
    const saved = await creation.json() as { id: string };
    expect(saved.id).toBeTruthy();
    await expect(dialog).toBeHidden();
    await expect(welcome).toBeHidden();
    await expect(page.getByText("welcome-server", { exact: true })).toBeVisible();
    expect(resourceWrites).toEqual([`POST ${inventoryPath}`]);
    expect(await api.requestJSON<unknown[]>("/api/agents")).toEqual([]);

    const deletion = await page.request.delete(`${origin}${inventoryPath}/${saved.id}`, {
      headers: { Authorization: `Bearer ${token}`, "X-Workspace-ID": workspace.id, "X-Workspace-Slug": workspace.slug },
    });
    expect(deletion.status()).toBe(204);
    // An external deletion verifies the persisted empty response after re-entry;
    // live Query transitions are canonical in mcp-tab.test.tsx.
    await page.reload();
    await expect(welcome).toBeVisible();
    await expect(page.getByText("welcome-server", { exact: true })).toHaveCount(0);
    expect(pageErrors).toEqual([]);
  } finally {
    await info.attach("page-errors", { body: JSON.stringify(pageErrors, null, 2), contentType: "application/json" });
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
