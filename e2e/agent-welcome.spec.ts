import { test, expect } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { TestApiClient } from "./fixtures";

test("empty agents introduce the workflow and become a list after the first creation", async ({ page }, info) => {
  test.setTimeout(180_000);
  const api = new TestApiClient();
  const run = `${Date.now().toString(36)}-${info.parallelIndex}`;
  await api.login(`e2e-agent-welcome-${run}@multica.ai`, "Agent welcome tester");
  const workspace = await api.ensureWorkspace("Agent welcome", `agent-welcome-${run}`);
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));

  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
    }, token);
    const origin = info.project.use.baseURL;
    if (typeof origin !== "string") throw new Error("Playwright baseURL is required");
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: origin }]);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${workspace.slug}/agents`);
    const hero = page.getByRole("heading", { name: "Meet your first AI teammate" });
    await expect(hero).toBeVisible();
    const welcome = page.getByRole("region", { name: "Meet your first AI teammate" });
    const carousel = page.getByRole("region", { name: "How agents work" });

    const layouts = [];
    for (const locale of ["en", "zh-Hans"] as const) {
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: origin }]);
      await page.goto(`/${workspace.slug}/agents`);
      const heading = locale === "en" ? "Meet your first AI teammate" : "招第一位 AI 队友";
      const region = page.getByRole("region", { name: heading, exact: true });
      await expect(region).toBeVisible();
      for (const theme of ["light", "dark"] as const) {
        await page.evaluate((value) => {
          document.documentElement.classList.remove("light", "dark");
          document.documentElement.classList.add(value);
          document.documentElement.style.colorScheme = value;
        }, theme);
        for (const viewport of [
          { width: 1440, height: 900 },
          { width: 1024, height: 768 },
          { width: 390, height: 844 },
          { width: 320, height: 568 },
        ]) {
          await page.setViewportSize(viewport);
          await region.evaluate((element) => { element.scrollTop = 0; });
          await page.evaluate(() => document.fonts.ready);
          const create = region.getByRole("link", {
            name: locale === "en" ? "New agent" : "新建智能体", exact: true,
          });
          await expect(create).toBeInViewport();
          const metrics = await region.evaluate((element) => {
            const card = element.querySelector('[role="group"][data-active="true"]');
            const checklist = element.querySelector("ol");
            if (!card || !checklist) throw new Error("Welcome illustration/checklist missing");
            return {
              documentWidth: document.documentElement.scrollWidth,
              viewportWidth: innerWidth,
              scrollWidth: element.scrollWidth,
              clientWidth: element.clientWidth,
              cardRadius: getComputedStyle(card).borderTopLeftRadius,
              checklistRadius: getComputedStyle(checklist).borderTopLeftRadius,
              visibleSlides: element.querySelectorAll('[role="group"]:not([inert])').length,
            };
          });
          layouts.push({ locale, theme, ...viewport, ...metrics });
          expect(metrics.documentWidth).toBeLessThanOrEqual(viewport.width);
          expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.clientWidth + 1);
          expect(metrics.visibleSlides).toBe(1);
          await page.screenshot({ path: info.outputPath(`${locale}-${viewport.width}-${theme}.png`), animations: "disabled" });
          if (viewport.width <= 390) {
            const examples = page.getByRole("region", {
              name: locale === "en" ? "How agents work" : "智能体如何工作", exact: true,
            });
            await examples.getByRole("button", { name: locale === "en" ? "Next example" : "下一个示例", exact: true }).scrollIntoViewIfNeeded();
            await expect(examples.getByRole("button", { name: locale === "en" ? "Next example" : "下一个示例", exact: true })).toBeInViewport();
            await page.screenshot({ path: info.outputPath(`${locale}-${viewport.width}-${theme}-examples.png`), animations: "disabled" });
          }
        }
      }
    }
    const layoutPath = info.outputPath("layout-metrics.json");
    await writeFile(layoutPath, JSON.stringify(layouts, null, 2));
    await info.attach("layout metrics", { path: layoutPath, contentType: "application/json" });
    for (const layout of layouts) {
      expect(parseFloat(layout.cardRadius), `${layout.locale}/${layout.theme}/${layout.width} card radius`).toBeGreaterThan(0);
      expect(parseFloat(layout.checklistRadius), `${layout.locale}/${layout.theme}/${layout.width} checklist radius`).toBeGreaterThan(0);
    }

    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: origin }]);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${workspace.slug}/agents`);
    await page.evaluate(() => {
      document.documentElement.classList.remove("dark");
      document.documentElement.classList.add("light");
      document.documentElement.style.colorScheme = "light";
    });
    await expect(hero).toBeVisible();
    await expect(carousel.getByRole("button", { name: "Previous example" })).toBeDisabled();
    const next = carousel.getByRole("button", { name: "Next example" });
    await next.focus();
    await page.keyboard.press("Enter");
    await expect(carousel.getByRole("heading", { name: "Assign work like a teammate" })).toBeVisible();
    await page.screenshot({ path: info.outputPath("assignment-en.png"), animations: "disabled" });
    await next.click();
    await expect(carousel.getByRole("heading", { name: "Find the result in the comments" })).toBeVisible();
    await expect(next).toBeDisabled();
    await page.screenshot({ path: info.outputPath("delivery-en.png"), animations: "disabled" });
    await carousel.getByRole("button", { name: /^Show example 1:/ }).click();
    await expect(carousel.getByRole("group", { name: "Describe what it should do" })).not.toHaveAttribute("inert");
    await expect(carousel.getByRole("group", { name: "Assign work like a teammate", includeHidden: true })).toHaveAttribute("inert");
    await expect(welcome.getByRole("link", { name: "New agent", exact: true })).toHaveCount(1);
    await expect(page.getByRole("button", { name: "Approvals", exact: true })).toBeVisible();

    await welcome.getByRole("link", { name: "New agent", exact: true }).click();
    await expect(page).toHaveURL(`/${workspace.slug}/agents/new`);
    await expect(page.getByRole("link", { name: /template/i })).toBeVisible();
    await page.goBack();
    await welcome.getByRole("link", { name: "Read the docs", exact: true }).click();
    await expect(page).toHaveURL(`/${workspace.slug}/docs/agents`);
    await page.goBack();
    await welcome.getByRole("link", { name: "Connect runtime", exact: true }).click();
    await expect(page).toHaveURL(`/${workspace.slug}/runtimes`);
    await page.goBack();

    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: origin }]);
    await page.goto(`/${workspace.slug}/agents`);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.evaluate(() => {
      document.documentElement.classList.remove("light");
      document.documentElement.classList.add("dark");
      document.documentElement.style.colorScheme = "dark";
    });
    await page.screenshot({ path: info.outputPath("zh-Hans-1440-dark.png"), animations: "disabled" });
    await page.getByRole("button", { name: "下一个示例", exact: true }).click();
    await expect(page.getByRole("heading", { name: "像同事一样分配任务" })).toBeVisible();
    const reducedTransition = await page.getByRole("group", { name: "像同事一样分配任务" }).evaluate((element) => {
      if (!element.parentElement) throw new Error("Carousel track missing");
      return getComputedStyle(element.parentElement).transitionDuration;
    });
    expect(reducedTransition).toBe("0s");

    // A fixture runtime has no daemon or credentials, so creating an agent
    // exercises real persistence/realtime without invoking a provider.
    const runtime = await api.seedProjectRuntime();
    await page.reload();
    await expect(page.getByText("已接入运行时", { exact: true })).toBeVisible();
    const agent = await api.requestJSON<{ id: string }>("/api/agents", {
      method: "POST",
      body: { name: "Welcome acceptance agent", runtime_id: runtime.id },
    });
    await expect(page.getByRole("heading", { name: "招第一位 AI 队友" })).not.toBeVisible();
    const agentLink = page.getByRole("table").locator(`a[href="/${workspace.slug}/agents/${agent.id}"]`);
    await expect(agentLink).toBeVisible();
    await expect(agentLink).toHaveText("Welcome acceptance agent");
    await page.screenshot({ path: info.outputPath("populated-list.png"), animations: "disabled" });
    await api.requestJSON(`/api/agents/${agent.id}/archive`, { method: "POST" });
    await expect(agentLink).not.toBeVisible();
    await expect(page.getByRole("heading", { name: "招第一位 AI 队友" })).not.toBeVisible();
    await expect(page.getByRole("textbox", { name: "搜索智能体...", exact: true })).toBeVisible();
    expect(pageErrors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
