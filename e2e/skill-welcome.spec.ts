import { expect, test, type Locator, type Page, type TestInfo } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { TestApiClient } from "./fixtures";

const englishTitle = "Turn your team’s know-how into a skill";
const chineseTitle = "把团队的做事方法写成 skill";

async function capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = info.outputPath(`${name}.png`);
  await page.screenshot({ path, animations: "disabled" });
  await info.attach(name, { path, contentType: "image/png" });
}

async function layoutMetrics(welcome: Locator) {
  return welcome.evaluate((element) => {
    const active = element.querySelector<HTMLElement>('[role="group"][data-active="true"]');
    if (!active) throw new Error("Active skill example is missing");
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

test("empty skills explain reuse and retain real creation, docs and directory recovery", async ({ page }, info) => {
  test.setTimeout(180_000);
  const api = new TestApiClient();
  const run = `${Date.now().toString(36)}-${info.parallelIndex}`;
  await api.login(`skill-welcome-${run}@example.invalid`, "Skill welcome acceptance");
  const workspace = await api.ensureWorkspace("Skill welcome acceptance", `skill-welcome-${run}`);
  expect(workspace.slug).toBe(`skill-welcome-${run}`);
  const directory = `/${workspace.slug}/skills`;
  const errors: string[] = [];
  const writes: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    if (/^(POST|PUT|PATCH|DELETE)$/.test(request.method()) && /^\/api\/(skills|agents)(\/|$)/.test(new URL(request.url()).pathname)) {
      writes.push(`${request.method()} ${new URL(request.url()).pathname}`);
    }
  });

  try {
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
      document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    }, token);
    const origin = info.project.use.baseURL;
    if (typeof origin !== "string") throw new Error("Playwright baseURL is required");
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: origin }]);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(directory);
    const welcome = page.getByRole("region", { name: englishTitle, exact: true });
    const examples = page.getByRole("region", { name: "How skills work", exact: true });
    await expect(welcome).toBeVisible();
    await expect(page).toHaveURL(directory);
    const create = welcome.getByRole("button", { name: "New skill", exact: true });
    await expect(page.getByRole("button", { name: "New skill", exact: true })).toHaveCount(1);

    await create.focus();
    await page.keyboard.press("Enter");
    const chooser = page.getByRole("dialog");
    for (const method of ["Create manually", "Modify from template", "Import from local", "Import from URL", "Copy from runtime"]) {
      await expect(chooser.getByText(method, { exact: true })).toBeVisible();
    }
    await chooser.getByRole("button", { name: "Close", exact: true }).click();
    await expect(chooser).toBeHidden();
    await expect(create).toBeFocused();

    await expect(examples.getByRole("button", { name: "Previous example", exact: true })).toBeDisabled();
    const next = examples.getByRole("button", { name: "Next example", exact: true });
    await next.focus();
    await page.keyboard.press("Enter");
    await expect(examples.getByRole("heading", { name: "Assign it to your agents", exact: true })).toBeVisible();
    await capture(page, info, "assignment-en-desktop");
    await next.click();
    await expect(examples.getByRole("heading", { name: "Keep one source for future work", exact: true })).toBeVisible();
    await expect(next).toBeDisabled();
    await capture(page, info, "reuse-en-desktop");
    await examples.getByRole("button", { name: /^Show example 1:/ }).click();
    await expect(examples.getByRole("group", { name: "Five ways to add a skill", exact: true })).not.toHaveAttribute("inert");
    await expect(examples.locator('[role="group"][inert]')).toHaveCount(2);
    expect(writes, "Illustrative scenes and choosing/cancelling creation must not write skill or agent data").toEqual([]);

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
        for (const viewport of [{ width: 1440, height: 900 }, { width: locale === "en" ? 390 : 320, height: locale === "en" ? 844 : 568 }]) {
          await page.setViewportSize(viewport);
          await region.evaluate((element) => { element.scrollTop = 0; });
          await page.evaluate(() => document.fonts.ready);
          const primary = region.getByRole("button", { name: locale === "en" ? "New skill" : "新建 skill", exact: true });
          await expect(primary).toBeInViewport();
          const metrics = await layoutMetrics(region);
          layouts.push({ locale, theme, ...viewport, ...metrics });
          expect(metrics.documentWidth).toBeLessThanOrEqual(metrics.viewportWidth);
          expect(metrics.contentWidth).toBeLessThanOrEqual(metrics.clientWidth + 1);
          expect(metrics.visibleScenes).toBe(1);
          expect(parseFloat(metrics.cardRadius)).toBeGreaterThan(0);
          await capture(page, info, `${locale}-${viewport.width}-${theme}`);
          if (viewport.width < 400) {
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
    await welcome.getByRole("link", { name: "Read the docs", exact: true }).click();
    await expect(page).toHaveURL(`/${workspace.slug}/docs/skills`);
    await page.goBack();
    await expect(welcome).toBeVisible();
    await page.getByRole("link", { name: "From template", exact: true }).click();
    await expect(page).toHaveURL(`/${workspace.slug}/marketplace?type=skills`);
    await page.goBack();
    await expect(welcome).toBeVisible();

    await page.emulateMedia({ reducedMotion: "reduce" });
    await next.click();
    const active = examples.getByRole("group", { name: "Assign it to your agents", exact: true });
    await expect(active).toBeVisible();
    expect(await active.evaluate((element) => getComputedStyle(element.parentElement!).transitionDuration)).toBe("0s");

    const saved = await api.requestJSON<{ id: string }>("/api/skills", {
      method: "POST", body: { name: "welcome-review", description: "Reusable review steps", content: "# Review\n\nRun checks before reviewing." },
    });
    await expect(welcome).toBeHidden();
    await expect(page.getByRole("link", { name: "welcome-review", exact: true })).toBeVisible();
    const search = page.getByRole("textbox", { name: "Search workspace skills...", exact: true });
    await search.fill("no-matching-skill");
    await expect(page.getByRole("heading", { name: "No matches", exact: true })).toBeVisible();
    await expect(welcome).toBeHidden();
    await page.getByRole("button", { name: "Clear search", exact: true }).click();
    await expect(search).toBeFocused();
    const deletion = await page.request.delete(`${origin}/api/skills/${saved.id}`, {
      headers: { Authorization: `Bearer ${token}`, "X-Workspace-ID": workspace.id, "X-Workspace-Slug": workspace.slug },
    });
    expect(deletion.status()).toBe(204);
    await expect(welcome).toBeVisible();

    await create.click();
    await chooser.getByRole("button", { name: /^Create manually/ }).click();
    await chooser.getByRole("textbox", { name: "Name", exact: true }).fill("first-team-method");
    await chooser.getByRole("textbox", { name: "Description", exact: true }).fill("Check the team conventions before reviewing a change.");
    await chooser.getByRole("button", { name: "Create skill", exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/skills/[^/]+$`));
    await page.goto(directory);
    await expect(welcome).toBeHidden();
    await expect(page.getByRole("link", { name: "first-team-method", exact: true })).toBeVisible();
    expect(errors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
