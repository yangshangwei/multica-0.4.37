import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Generator matrices remain in core/workspace/workspace-names.test.ts.
// This suite verifies the form against the real API, including saved fields.
test.use({ viewport: { width: 1440, height: 1000 }, contextOptions: { reducedMotion: "reduce" }, screenshot: "only-on-failure" });

for (const locale of ["en", "zh-Hans"] as const) {
  test(`${locale} naming preference survives reload and generated workspace preserves manual URL and prefix`, async ({ page, baseURL }, info) => {
    const api = new TestApiClient();
    const suffix = randomUUID().slice(0, 8);
    const copy = JSON.parse(readFileSync(`packages/views/locales/${locale}/workspace.json`, "utf8")).name_picker;
    const onboarding = JSON.parse(readFileSync(`packages/views/locales/${locale}/onboarding.json`, "utf8"));
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await api.login(`naming-${suffix}@example.invalid`, "Naming acceptance");
    const initial = await api.ensureWorkspace(`Naming ${suffix}`, `naming-${suffix}`);
    let created: { id: string; slug: string; name: string; issue_prefix: string } | undefined;
    try {
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: baseURL! }]);
      await page.addInitScript(({ token, locale }) => {
        localStorage.setItem("multica_token", token!);
        localStorage.setItem("multica-locale", locale);
      }, { token: api.getToken(), locale });
      const enter = async () => {
        await page.goto("/workspaces/new", { waitUntil: "domcontentloaded" });
        await page.getByRole("button", { name: onboarding.welcome.continue_on_web, exact: true }).click();
        await page.getByRole("radio", { name: onboarding.questions.role.engineer, exact: true }).click();
        await page.getByRole("checkbox", { name: onboarding.questions.use_case.ship_code, exact: true }).check();
        await page.getByRole("button", { name: onboarding.common.continue, exact: true }).click();
        await expect(page.locator("#ws-name")).toBeVisible();
      };
      await enter();
      const name = page.locator("#ws-name");
      const slug = page.locator("#ws-slug");
      const prefix = page.locator("#ws-issue-prefix");
      const random = page.getByRole("button", { name: copy.random, exact: true });
      const choose = page.getByRole("button", { name: copy.choose_series, exact: true });
      await random.click();
      const before = [await name.inputValue(), await slug.inputValue(), await prefix.inputValue()];
      await choose.click();
      await expect(page.getByRole("menuitemradio")).toHaveCount(7);
      await page.getByRole("menuitemradio").filter({ hasText: copy.series.nature }).click();
      expect([await name.inputValue(), await slug.inputValue(), await prefix.inputValue()]).toEqual(before);
      await expect(choose).toBeFocused();
      await page.reload();
      await enter();
      await expect(page.getByText(copy.current_series.replace("{{series}}", copy.series.nature), { exact: true })).toBeVisible();
      await random.click();
      const generated = await slug.inputValue();
      await name.fill("Edited workspace");
      await expect(slug).toHaveValue(generated);
      const manualSlug = `named-${suffix}`;
      await slug.fill(manualSlug);
      await prefix.fill("NAME");
      await random.click();
      await expect(slug).toHaveValue(manualSlug);
      await expect(prefix).toHaveValue("NAME");
      const finalName = await name.inputValue();
      expect(finalName).not.toBe("Edited workspace");
      expect(finalName.trim()).not.toBe("");
      await page.setViewportSize({ width: 390, height: 844 });
      await choose.click();
      const menu = page.getByRole("menu");
      await expect(menu).toBeVisible();
      await page.screenshot({ path: info.outputPath(`${locale}-naming-menu.png`), animations: "disabled" });
      const bounds = await menu.boundingBox();
      expect(bounds).not.toBeNull();
      expect(bounds!.x).toBeGreaterThanOrEqual(0);
      expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.keyboard.press("Escape");
      const response = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/workspaces" && response.request().method() === "POST");
      await page.getByRole("button", { name: onboarding.step_workspace.cta_create_named.replace("{{name}}", finalName), exact: true }).click();
      const result = await response;
      expect(result.status()).toBe(201);
      created = await result.json();
      expect(created).toMatchObject({ name: finalName, slug: manualSlug, issue_prefix: "NAME" });
      expect(await api.getWorkspaces()).toEqual(expect.arrayContaining([expect.objectContaining({ id: created!.id, name: finalName, slug: manualSlug })]));
      api.setWorkspaceId(created!.id);
      api.setWorkspaceSlug(created!.slug);
      const issue = await api.createIssue(`Prefix acceptance ${suffix}`, { status: "todo" });
      expect(issue.identifier).toMatch(/^NAME-\d+$/);
      expect(errors).toEqual([]);
    } finally {
      try {
        if (created) {
          api.setWorkspaceId(created.id);
          api.setWorkspaceSlug(created.slug);
          await api.deleteFeatureWorkspace(created.id);
        }
      } finally {
        api.setWorkspaceId(initial.id);
        api.setWorkspaceSlug(initial.slug);
        await api.deleteFeatureWorkspace(initial.id);
      }
    }
  });
}
