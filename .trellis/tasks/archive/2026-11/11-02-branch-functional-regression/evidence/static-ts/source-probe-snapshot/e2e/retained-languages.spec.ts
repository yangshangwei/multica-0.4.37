import { randomUUID } from "node:crypto";
import { test as base, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

interface LanguageFixture {
  api: TestApiClient;
  slug: string;
  origin: string;
}

const test = base.extend<{ languageFixture: LanguageFixture }>({
  languageFixture: async ({ page, baseURL }, use, testInfo) => {
    const slug = `e2e-languages-${randomUUID().slice(0, 12)}`;
    const api = new TestApiClient();
    await api.login(`${slug}@example.invalid`, "Language regression owner");
    const workspace = await api.ensureWorkspace("Language regression", slug);
    try {
      await api.markUserOnboarded();
      const token = api.getToken();
      if (!token || !baseURL) throw new Error("Language fixture authentication is incomplete");
      await page.context().addCookies([{ name: "multica_logged_in", value: "1", url: baseURL }]);
      await page.addInitScript((value) => {
        localStorage.setItem("multica_token", value);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, token);
      await use({ api, slug, origin: baseURL });
    } finally {
      if (testInfo.status !== testInfo.expectedStatus) {
        await testInfo.attach("language-failure", {
          body: await page.screenshot({ fullPage: true }), contentType: "image/png",
        });
      }
      await api.deleteFeatureWorkspace(workspace.id);
    }
  },
});

test.use({ locale: "zh-CN", viewport: { width: 1440, height: 1000 } });

for (const retired of ["ja", "ko"]) {
  test(`retired ${retired} preference falls back to English and both retained choices persist`, async ({ page, languageFixture }, testInfo) => {
    const { api, slug, origin } = languageFixture;
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const user = await api.requestJSON<{ language: string }>("/api/me", {
      method: "PATCH", body: { language: retired },
    });
    expect(user.language).toBe("en");
    await page.context().addCookies([{ name: "multica-locale", value: retired, url: origin }]);
    await page.goto(`/${slug}/settings?tab=preferences`);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await page.getByRole("combobox", { name: "Language", exact: true }).click();
    await expect(page.getByRole("option")).toHaveText(["English", "中文"]);
    await page.getByRole("option", { name: "中文", exact: true }).click();
    await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
    await expect(page.getByRole("combobox", { name: "语言", exact: true })).toContainText("中文");
    expect(await api.requestJSON("/api/me")).toMatchObject({ language: "zh-Hans" });
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
    await page.screenshot({ path: testInfo.outputPath(`${retired}-to-chinese.png`), animations: "disabled" });
    await page.getByRole("combobox", { name: "语言", exact: true }).click();
    await page.getByRole("option", { name: "English", exact: true }).click();
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.getByRole("combobox", { name: "Language", exact: true })).toContainText("English");
    expect(await api.requestJSON("/api/me")).toMatchObject({ language: "en" });
    await page.screenshot({ path: testInfo.outputPath(`${retired}-to-english.png`), animations: "disabled" });
    expect(errors).toEqual([]);
  });
}

test("a new client normalizes a retired preference returned by an older server", async ({ page, languageFixture }) => {
  const { api, slug, origin } = languageFixture;
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
  await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: origin }]);
  await page.route("**/api/me", async (route) => {
    const response = await route.fetch();
    const user = await response.json();
    await route.fulfill({ response, json: { ...user, language: "ja" } });
  });
  let documents = 0;
  page.on("request", (request) => {
    if (request.isNavigationRequest() && request.frame() === page.mainFrame()) documents++;
  });
  await page.goto(`/${slug}/settings?tab=preferences`);
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.getByRole("combobox", { name: "Language", exact: true })).toContainText("English");
  expect((await page.context().cookies()).find((cookie) => cookie.name === "multica-locale")?.value).toBe("en");
  expect(documents).toBeLessThanOrEqual(2);
  await page.reload();
  await expect(page.getByRole("combobox", { name: "Language", exact: true })).toContainText("English");
  expect(documents).toBeLessThanOrEqual(3);
});
