import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("empty squads explain collaboration and keep creation and docs reachable", async ({ page }, info) => {
  const api = new TestApiClient();
  const run = Date.now().toString(36);
  await api.login(`e2e-squad-welcome-${run}@multica.ai`, "Squad welcome tester");
  const workspace = await api.ensureWorkspace("Squad welcome", `squad-welcome-${run}`);
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));

  try {
    await api.markUserOnboarded();
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
    }, token);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${workspace.slug}/squads`);
    const hero = page.getByRole("heading", { name: "Let your agents work as a team." });
    await expect(hero).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    await page.screenshot({ path: info.outputPath("desktop-en.png"), animations: "disabled" });

    const carousel = page.getByRole("region", { name: "How squads work" });
    await expect(carousel.getByRole("button", { name: "Previous example" })).toBeDisabled();
    const next = carousel.getByRole("button", { name: "Next example" });
    await next.focus();
    await page.keyboard.press("Enter");
    await expect(carousel.getByRole("heading", { name: "A role for every member" })).toBeVisible();
    await page.screenshot({ path: info.outputPath("roles-en.png"), animations: "disabled" });
    await next.click();
    await expect(carousel.getByRole("heading", { name: "Every result moves work forward" })).toBeVisible();
    await expect(next).toBeDisabled();
    await page.screenshot({ path: info.outputPath("handoff-en.png"), animations: "disabled" });
    await carousel.getByRole("button", { name: /^Show example 1:/ }).click();
    await expect(carousel.getByRole("heading", { name: "One entry point. The right next step." })).toBeVisible();

    await page.getByRole("link", { name: "New Squad", exact: true }).click();
    await expect(page).toHaveURL(`/${workspace.slug}/squads/new`);
    await expect(page.getByRole("link", { name: /Create from template/ })).toBeVisible();
    await expect(page.getByRole("link", { name: /Create custom squad/ })).toBeVisible();
    await page.goBack();
    const docs = page.getByRole("link", { name: "Read the docs", exact: true });
    await docs.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(`/${workspace.slug}/docs/squads`);
    await expect(page.getByRole("heading", { name: "AI小队", exact: true })).toBeVisible();
    await page.goBack();
    await page.getByRole("tab", { name: "Templates", exact: true }).click();
    await expect(page).toHaveURL(`/${workspace.slug}/squads?view=templates`);
    await expect(page.getByRole("button", { name: "Apply to project", exact: true }).first()).toBeVisible();
    await page.getByRole("tab", { name: "Workspace", exact: true }).click();
    await expect(hero).toBeVisible();

    for (const locale of ["en", "zh-Hans"] as const) {
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: new URL(page.url()).origin }]);
      await page.goto(`/${workspace.slug}/squads`);
      const headline = page.getByRole("heading", {
        name: locale === "en" ? "Let your agents work as a team." : /让智能体组队/,
      });
      await expect(headline).toBeVisible();
      for (const width of [1440, 1024, 390]) {
        await page.setViewportSize({ width, height: 900 });
        await page.evaluate(() => document.fonts.ready);
        await expect(page.getByRole("link", { name: locale === "en" ? "New Squad" : "新建AI小队", exact: true })).toBeInViewport();
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        await page.screenshot({ path: info.outputPath(`${locale}-${width}.png`), animations: "disabled" });
      }
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.evaluate(() => {
      localStorage.setItem("theme", "dark");
      document.documentElement.classList.remove("light");
      document.documentElement.classList.add("dark");
      document.documentElement.style.colorScheme = "dark";
    });
    await page.screenshot({ path: info.outputPath("desktop-dark.png"), animations: "disabled" });
    await page.getByRole("button", { name: "下一个示例", exact: true }).click();
    await expect(page.getByRole("button", { name: "上一个示例", exact: true })).toBeEnabled();
    expect(pageErrors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
