import { test, expect, _electron as electron } from "@playwright/test";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { TestApiClient } from "./fixtures";

test("native desktop keeps market and direct role creation entrances with source-aware Back", async ({}, info) => {
  const rendererUrl = process.env.COLLABORATION_ELECTRON_RENDERER_URL;
  const apiUrl = process.env.COLLABORATION_E2E_API_URL;
  test.skip(!rendererUrl || !apiUrl, "Requires a task-owned production Desktop renderer and local API");
  test.setTimeout(150_000);

  const root = resolve(import.meta.dirname, "..");
  const profile = mkdtempSync(join(tmpdir(), "multica-agent-template-market-"));
  const run = `${Date.now().toString(36)}-${info.parallelIndex}`;
  const api = new TestApiClient();
  let workspaceId: string | undefined;
  let desktop: Awaited<ReturnType<typeof electron.launch>> | undefined;
  try {
    await api.login(`agent-template-desktop-${run}@multica.ai`, "Agent template desktop acceptance");
    const workspace = await api.ensureWorkspace("Agent template desktop acceptance", `agent-template-desktop-${run}`);
    workspaceId = workspace.id;
    await api.markUserOnboarded();
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    await api.seedProjectRuntime();

    const env = { ...process.env };
    delete env.ELECTRON_RUN_AS_NODE;
    desktop = await electron.launch({
      executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin"
        ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"),
      args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
      env: {
        ...env,
        CHANGELOG_E2E_API_URL: apiUrl!,
        CHANGELOG_ELECTRON_RENDERER_URL: rendererUrl!,
        CHANGELOG_ELECTRON_PROFILE: profile,
        CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en-US",
      },
    });
    const page = await desktop.firstWindow();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await page.waitForURL(new URL(rendererUrl!).href);
      await page.evaluate((token) => {
        localStorage.setItem("multica_token", token!);
        localStorage.setItem("multica-locale", "en");
        localStorage.setItem("multica:chat:isOpen", "false");
        localStorage.setItem("theme", "light");
      }, api.getToken());
      await page.reload({ waitUntil: "domcontentloaded" });
      const marketLink = page.getByRole("link", { name: "Capability market", exact: true });
      await expect(marketLink).toBeVisible({ timeout: 45_000 });
      await marketLink.click();
      const agentType = page.getByRole("tab", { name: "Agent templates", exact: true });
      await agentType.click();
      await expect(agentType).toHaveAttribute("aria-selected", "true");
      const useTemplate = page.getByRole("link", { name: "Use template", exact: true }).first();
      await expect(useTemplate).toBeVisible();
      const href = await useTemplate.getAttribute("href");
      expect(href).toContain(`/${workspace.slug}/agents/new/template?`);
      expect(new URL(href!, "http://localhost").searchParams.get("source")).toBe("marketplace");

      await useTemplate.click();
      await expect(page.getByText("Role instructions", { exact: true })).toBeVisible();
      const configurationBack = page.getByRole("banner")
        .filter({ has: page.getByRole("heading", { name: "Create an agent", exact: true }) })
        .getByRole("button", { name: "Go back", exact: true });
      await configurationBack.click();
      await expect(agentType).toHaveAttribute("aria-selected", "true");
      await page.screenshot({ path: info.outputPath("native-agent-template-market.png"), animations: "disabled" });

      // A modifier click creates a cold tab with no prior market entry to go back to.
      const tabCount = await page.locator("[data-tab-id]").count();
      await useTemplate.click({ modifiers: [process.platform === "darwin" ? "Meta" : "Control"] });
      await expect(page.locator("[data-tab-id]")).toHaveCount(tabCount + 1);
      await page.locator("[data-tab-id]").last().click();
      await expect(page.getByText("Role instructions", { exact: true })).toBeVisible();
      await configurationBack.click();
      await expect(agentType).toHaveAttribute("aria-selected", "true");
      await expect(useTemplate).toBeVisible();

      await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
      await page.evaluate(() => localStorage.setItem("multica-locale", "zh-Hans"));
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(page.getByRole("tab", { name: "智能体模板", exact: true })).toHaveAttribute("aria-selected", "true");
      await expect(page.getByRole("link", { name: "使用模板", exact: true }).first()).toBeVisible();
      await page.screenshot({ path: info.outputPath("native-agent-template-market-zh.png"), animations: "disabled" });
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
      await page.evaluate(() => localStorage.setItem("multica-locale", "en"));
      await page.reload({ waitUntil: "domcontentloaded" });

      await page.getByRole("link", { name: "Agents", exact: true }).click();
      await page.getByRole("link", { name: "New agent", exact: true }).click();
      await page.getByText("Use a template", { exact: true }).click();
      await expect(page.getByRole("heading", { name: "Start from a role template", exact: true })).toBeVisible();
      await page.getByRole("link", { name: "Create agent", exact: true }).first().click();
      await expect(page.getByText("Role instructions", { exact: true })).toBeVisible();
      await configurationBack.click();
      await expect(page.getByRole("heading", { name: "Start from a role template", exact: true })).toBeVisible();

      await marketLink.click();
      await agentType.click();
      await useTemplate.click();
      const createdResponse = page.waitForResponse((response) =>
        new URL(response.url()).pathname === "/api/agents/from-template" && response.request().method() === "POST");
      await page.getByRole("button", { name: /Create & open agent/ }).click();
      const response = await createdResponse;
      expect(response.status()).toBe(201);
      expect(response.request().postDataJSON()).not.toHaveProperty("source");
      const created = await response.json();
      expect(created.id).toBeTruthy();
      await expect(page.getByRole("heading", { name: created.name, exact: true })).toBeVisible();
      const services = await desktop.evaluate(() => (globalThis as unknown as {
        changelogAcceptance: { daemonStarts: number; installCalls: number };
      }).changelogAcceptance);
      expect(services.daemonStarts).toBe(0);
      expect(services.installCalls).toBe(0);
      expect(errors).toEqual([]);
    } catch (error) {
      await page.screenshot({ path: info.outputPath("native-agent-template-error.png"), animations: "disabled" });
      writeFileSync(info.outputPath("native-agent-template-error.txt"), `${await page.locator("body").innerText()}\n\n${errors.join("\n")}`);
      throw error;
    }
  } finally {
    try {
      await desktop?.close();
    } finally {
      try {
        if (workspaceId) await api.deleteFeatureWorkspace(workspaceId);
      } finally {
        rmSync(profile, { recursive: true, force: true });
      }
    }
  }
});
