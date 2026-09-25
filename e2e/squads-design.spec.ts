import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("squad discovery keeps keyboard navigation, saved choices and project setup usable", async ({ page }, info) => {
  const api = new TestApiClient();
  const run = Date.now().toString(36);
  await api.login(`e2e-squads-design-${run}@multica.ai`, "Squad design tester");
  const workspace = await api.ensureWorkspace("Squad design", `squads-design-${run}`);
  const pageErrors: string[] = [];
  const failedRequests: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("requestfailed", (request) => failedRequests.push(`${new URL(request.url()).pathname}: ${request.failure()?.errorText}`));
  try {
    await api.markUserOnboarded();
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, token);
    const runtime = await api.seedProjectRuntime();
    const { squad } = await api.requestJSON<{ squad: { id: string; name: string } }>("/api/squads/from-template", {
      method: "POST",
      body: { template_key: "feature-delivery", runtime_id: runtime.id, name: "Payments delivery" },
    });
    const description = "Our own workflow: payment reconciliation, release checks, and customer support.";
    await api.requestJSON(`/api/squads/${squad.id}`, { method: "PUT", body: { description } });
    for (const [key, name] of [["bug-fix", "Customer issue response"], ["review-gate", "Release review"], ["docs", "Documentation upkeep"]]) {
      await api.requestJSON("/api/squads/from-template", {
        method: "POST", body: { template_key: key, runtime_id: runtime.id, name },
      });
    }

    await page.setViewportSize({ width: 1920, height: 1080 });
    await page.goto(`/${workspace.slug}/squads`);
    const workspaceTab = page.getByRole("tab", { name: "Workspace squads" });
    const templateTab = page.getByRole("tab", { name: "Squad templates" });
    await expect(workspaceTab).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("link", { name: squad.name, exact: true })).toBeVisible();
    await expect(page.getByText(description, { exact: true })).toBeVisible();
    await page.screenshot({ path: info.outputPath("workspace-wide.png"), animations: "disabled" });

    await page.getByRole("button", { name: "Display", exact: true }).click();
    const creatorColumn = page.getByRole("switch", { name: "Created by", exact: true });
    await expect(creatorColumn).not.toBeChecked();
    await expect(page.getByRole("switch", { name: "Created", exact: true })).not.toBeChecked();
    await creatorColumn.click();
    await page.keyboard.press("Escape");
    await page.reload();
    await page.getByRole("button", { name: "Display", exact: true }).click();
    await expect(page.getByRole("switch", { name: "Created by", exact: true })).toBeChecked();
    await page.keyboard.press("Escape");

    const squadLink = page.getByRole("link", { name: squad.name, exact: true });
    await squadLink.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/squads/${squad.id}$`));
    await page.goBack();
    await expect(workspaceTab).toHaveAttribute("aria-selected", "true");
    await page.getByRole("button", { name: "Squad actions" }).first().click();
    await expect(page.getByRole("menuitem", { name: "Archive", exact: true })).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/squads$`));
    await page.keyboard.press("Escape");

    await workspaceTab.focus();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("Enter");
    await expect(templateTab).toHaveAttribute("aria-selected", "true");
    await expect(page).toHaveURL(/\?view=templates$/);
    await page.reload();
    await expect(templateTab).toHaveAttribute("aria-selected", "true");
    const template = page.getByRole("listitem", { name: "Feature Delivery Squad", exact: true });
    await expect(template.getByRole("link", { name: squad.name })).toBeVisible();
    await page.screenshot({ path: info.outputPath("templates-wide.png"), animations: "disabled" });
    await template.getByRole("button", { name: "Apply to project" }).click();
    await expect(page.getByRole("dialog", { name: "Use Feature Delivery Squad for a project" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Create a project", exact: true })).toBeEnabled();
    await page.keyboard.press("Escape");

    await template.getByRole("link", { name: squad.name }).click();
    await expect(page).toHaveURL(new RegExp(`/squads/${squad.id}$`));
    await page.goBack();
    await expect(templateTab).toHaveAttribute("aria-selected", "true");

    for (const width of [768, 390]) {
      await page.setViewportSize({ width, height: 900 });
      await page.screenshot({ path: info.outputPath(`templates-${width}.png`), animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await workspaceTab.click();
      await expect(workspaceTab).toHaveAttribute("aria-selected", "true");
      await expect(page.getByRole("link", { name: squad.name, exact: true })).toBeVisible();
      await page.screenshot({ path: info.outputPath(`workspace-${width}.png`), animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await templateTab.click();
      await expect(templateTab).toHaveAttribute("aria-selected", "true");
    }
    await page.getByRole("button", { name: "New Squad", exact: true }).click();
    await expect(page.getByRole("menuitem", { name: "Create from template", exact: true })).toBeVisible();
    await expect(page.getByRole("menuitem", { name: "Create custom squad", exact: true })).toBeVisible();
    await page.keyboard.press("Escape");
    for (const [locale, label] of [["zh-Hans", "AI小队模板"], ["ja", "テンプレート"]]) {
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: new URL(page.url()).origin }]);
      await page.goto(`/${workspace.slug}/squads?view=templates`);
      await expect(page.getByRole("tab", { name: label })).toHaveAttribute("aria-selected", "true");
      const tabBounds = await page.getByRole("tab", { name: label }).boundingBox();
      expect(tabBounds).not.toBeNull();
      expect(tabBounds!.x + tabBounds!.width).toBeLessThanOrEqual(390);
      await page.screenshot({ path: info.outputPath(`templates-${locale}-390.png`), animations: "disabled" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.screenshot({ path: info.outputPath(`templates-${locale}-1440.png`), animations: "disabled" });
      await page.setViewportSize({ width: 390, height: 900 });
    }
    expect(pageErrors).toEqual([]);
  } catch (error) {
    await page.screenshot({ path: info.outputPath("failure.png"), animations: "disabled" });
    await info.attach("browser-errors", { body: JSON.stringify({ pageErrors, failedRequests }), contentType: "application/json" });
    throw error;
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
