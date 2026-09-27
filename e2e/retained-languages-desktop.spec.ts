import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { _electron as electron, test, expect, type ElectronApplication } from "@playwright/test";
import { TestApiClient } from "./fixtures";

for (const retired of ["ja", "ko"]) {
  test(`Desktop keeps retained languages after upgrading ${retired} preferences`, async ({}, testInfo) => {
    test.skip(!process.env.CHANGELOG_ELECTRON_RENDERER_URL || !process.env.CHANGELOG_E2E_API_URL
      || !process.env.NEXT_PUBLIC_API_URL || !process.env.DATABASE_URL,
      "Requires built Desktop renderer/preload and explicit isolated API/database configuration");
    expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin)
      .toBe(new URL(process.env.CHANGELOG_E2E_API_URL!).origin);
    const apiUrl = new URL(process.env.CHANGELOG_E2E_API_URL!);
    const rendererUrl = new URL(process.env.CHANGELOG_ELECTRON_RENDERER_URL!);
    for (const url of [apiUrl, rendererUrl]) {
      expect(["localhost", "127.0.0.1"]).toContain(url.hostname);
    }
    const preflight = await fetch(new URL("/api/config", apiUrl), {
      method: "OPTIONS",
      headers: {
        Origin: rendererUrl.origin,
        "Access-Control-Request-Method": "GET",
        "Access-Control-Request-Headers": "authorization,content-type",
      },
    });
    expect(preflight.headers.get("access-control-allow-origin"),
      "The isolated API must allow the renderer origin before Desktop authentication can run")
      .toBe(rendererUrl.origin);
    test.setTimeout(90_000);
    const root = resolve(import.meta.dirname, "..");
    const profile = mkdtempSync(join(tmpdir(), "multica-retained-languages-"));
    const api = new TestApiClient();
    let workspaceId: string | undefined;
    let desktop: ElectronApplication | undefined;
    try {
      const slug = `e2e-desktop-language-${randomUUID().slice(0, 10)}`;
      await api.login(`${slug}@example.invalid`, "Desktop language regression");
      const workspace = await api.ensureWorkspace("Desktop languages", slug);
      workspaceId = workspace.id;
      await api.markUserOnboarded();
      expect(await api.requestJSON("/api/me", { method: "PATCH", body: { language: retired } }))
        .toMatchObject({ language: "en" });
      const token = api.getToken();
      if (!token) throw new Error("Desktop fixture is missing its token");
      const executablePath = join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin"
        ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron");
      const options = {
        executablePath,
        args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
        env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_ELECTRON_SYSTEM_LOCALE: retired },
      };
      desktop = await electron.launch(options);
      let page = await desktop.firstWindow();
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.locator("#root > *").first().waitFor({ state: "attached" });
      expect(await page.evaluate(() => (window as Window & {
        desktopAPI: { systemLocale: string };
      }).desktopAPI.systemLocale)).toBe(retired);
      await page.context().addInitScript(({ token, locale }) => {
        localStorage.setItem("multica_token", token);
        if (!localStorage.getItem("multica-locale")) localStorage.setItem("multica-locale", locale);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, { token, locale: retired });
      await page.reload();
      await expect(page.locator("html")).toHaveAttribute("lang", "en");
      await page.getByRole("button", { name: "Help", exact: true }).click();
      await page.getByRole("menuitem", { name: "Desktop version 0.4.40-test", exact: true }).click();
      await page.getByRole("tab", { name: "Preferences", exact: true }).click();
      await page.getByRole("combobox", { name: "Language", exact: true }).click();
      await expect(page.getByRole("option")).toHaveText(["English", "中文"]);
      await page.getByRole("option", { name: "中文", exact: true }).click();
      await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
      await expect(page.getByRole("combobox", { name: "语言", exact: true })).toContainText("中文");
      await page.screenshot({ path: testInfo.outputPath(`${retired}-chinese.png`), animations: "disabled" });
      expect(errors).toEqual([]);
      await desktop.close();
      desktop = await electron.launch(options);
      page = await desktop.firstWindow();
      page.on("pageerror", (error) => errors.push(error.message));
      await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
      await expect(page.getByRole("combobox", { name: "语言", exact: true })).toContainText("中文");
      await page.getByRole("combobox", { name: "语言", exact: true }).click();
      await page.getByRole("option", { name: "English", exact: true }).click();
      await expect(page.locator("html")).toHaveAttribute("lang", "en");
      await expect(page.getByRole("combobox", { name: "Language", exact: true })).toContainText("English");
      await page.screenshot({ path: testInfo.outputPath(`${retired}-english.png`), animations: "disabled" });
      expect(await desktop.evaluate(() => (globalThis as unknown as {
        changelogAcceptance: { daemonStarts: number };
      }).changelogAcceptance.daemonStarts)).toBe(0);
      expect(errors).toEqual([]);
    } catch (error) {
      const page = desktop?.windows()[0];
      if (page && !page.isClosed()) {
        await testInfo.attach("desktop-language-failure", {
          body: await page.screenshot(), contentType: "image/png",
        });
        await testInfo.attach("desktop-language-failure-text", {
          body: await page.locator("body").innerText(), contentType: "text/plain",
        });
      }
      throw error;
    } finally {
      try { await desktop?.close(); }
      finally {
        try { if (workspaceId) await api.deleteFeatureWorkspace(workspaceId); }
        finally { rmSync(profile, { recursive: true, force: true }); }
      }
    }
  });
}
