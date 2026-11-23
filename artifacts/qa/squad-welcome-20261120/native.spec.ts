import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "../../../e2e/fixtures";
import { openDesktop, nativeScreenshot, type NativeHarness } from "../../../e2e/fixtures/collaboration-native";

async function settlePaint(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
  });
  let previous: string | undefined;
  await expect.poll(async () => {
    await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    const current = JSON.stringify(await geometry(page));
    const stable = previous === current;
    previous = current;
    return stable;
  }, { message: "Capture requires two stable painted layout observations" }).toBe(true);
}

async function geometry(page: Page) {
  return page.getByRole("region", { name: /How squads work|AI小队如何协作/ }).evaluate((element) => {
    const card = element.querySelector<HTMLElement>('[data-active="true"]');
    if (!card) throw new Error("Active collaboration example is required");
    const style = getComputedStyle(card);
    const rect = card.getBoundingClientRect();
    return {
      viewport: { width: innerWidth, height: innerHeight },
      pageWidth: document.documentElement.scrollWidth,
      card: { x: rect.x, y: rect.y, width: rect.width, height: rect.height, radius: style.borderTopLeftRadius },
      groups: Array.from(element.querySelectorAll('[role="group"]')).map((slide) => ({
        active: slide.getAttribute("data-active"),
        hidden: slide.getAttribute("aria-hidden"),
        inert: slide.hasAttribute("inert"),
      })),
    };
  });
}

test("production Web enlarged text retains reachable creation and manual examples", async ({ page }, info) => {
  const api = new TestApiClient();
  const run = Date.now().toString(36);
  await api.login(`e2e-squad-zoom-${run}@multica.ai`, "Squad zoom tester");
  const workspace = await api.ensureWorkspace("Squad enlarged text", `squad-zoom-${run}`);
  const evidence: Record<string, unknown>[] = [];
  try {
    await api.markUserOnboarded();
    await page.addInitScript((token) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
    }, api.getToken());
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(`/${workspace.slug}/squads`);
    await expect(page.getByRole("heading", { name: "Let your agents work as a team." })).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    const create = page.getByRole("link", { name: "New Squad", exact: true });
    await expect(create).toBeInViewport();
    const initial = await geometry(page);
    expect(parseFloat(initial.card.radius)).toBeGreaterThan(0);
    expect(initial.pageWidth).toBeLessThanOrEqual(initial.viewport.width);
    evidence.push({ state: "1024x768", ...initial });
    await page.screenshot({ path: info.outputPath("web-1024x768.png"), animations: "disabled" });

    await page.evaluate(() => { document.documentElement.style.zoom = "2"; });
    await create.scrollIntoViewIfNeeded();
    await expect(create).toBeVisible();
    await create.focus();
    await expect(create).toBeFocused();
    const next = page.getByRole("button", { name: "Next example", exact: true });
    await next.scrollIntoViewIfNeeded();
    await next.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "A role for every member", exact: true })).toBeVisible();
    await settlePaint(page);
    await expect(next).toBeInViewport();
    const zoomed = await geometry(page);
    expect(zoomed.pageWidth).toBeLessThanOrEqual(zoomed.viewport.width);
    expect(zoomed.card.x).toBeGreaterThanOrEqual(0);
    expect(zoomed.card.x + zoomed.card.width).toBeLessThanOrEqual(zoomed.viewport.width);
    expect(zoomed.groups.filter((group) => group.active === "true")).toHaveLength(1);
    expect(zoomed.groups.filter((group) => group.active !== "true").every((group) => group.hidden === "true" && group.inert)).toBe(true);
    evidence.push({ state: "200-percent", ...zoomed });
    await page.screenshot({ path: info.outputPath("web-200-percent.png"), animations: "disabled" });
  } finally {
    writeFileSync(info.outputPath("web-geometry.json"), JSON.stringify(evidence, null, 2));
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("built Electron renderer preserves the squad welcome, chooser and offline documentation", async ({}, info) => {
  const api = new TestApiClient();
  const run = Date.now().toString(36);
  await api.login(`e2e-squad-native-${run}@multica.ai`, "Squad desktop tester");
  const workspace = await api.ensureWorkspace("Squad desktop", `squad-native-${run}`);
  const profile = mkdtempSync(join(tmpdir(), "multica-squad-welcome-native-"));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  const evidence: Record<string, unknown>[] = [];
  try {
    await api.markUserOnboarded();
    native = await openDesktop(profile, { token: api.getToken()! });
    await native.app.context().addInitScript(() => { localStorage.setItem("theme", "light"); });
    await native.start();
    const { page, app } = native;
    await expect(page.getByRole("navigation", { name: "Application navigation", exact: true })).toBeVisible();
    await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), `/${workspace.slug}/squads`);
    await expect(page.getByRole("heading", { name: "Let your agents work as a team." })).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    const window = await app.browserWindow(page);
    await window.evaluate((target) => { target.setContentSize(1380, 900); });
    await settlePaint(page);
    const wide = await geometry(page);
    expect(parseFloat(wide.card.radius)).toBeGreaterThan(0);
    expect(wide.pageWidth).toBeLessThanOrEqual(wide.viewport.width);
    evidence.push({ state: "native-wide", ...wide });
    await nativeScreenshot(app, page, info.outputPath("electron-wide.png"));

    const next = page.getByRole("button", { name: "Next example", exact: true });
    await next.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "A role for every member", exact: true })).toBeVisible();
    await next.click();
    await expect(page.getByRole("heading", { name: "Every result moves work forward", exact: true })).toBeVisible();
    await expect(next).toBeDisabled();
    await page.getByRole("button", { name: /^Show example 1:/ }).click();

    await page.getByRole("link", { name: "New Squad", exact: true }).click();
    await expect(page.getByRole("link", { name: /Create from template/ })).toBeVisible();
    await expect(page.getByRole("link", { name: /Create custom squad/ })).toBeVisible();
    await page.getByRole("link", { name: "Squads", exact: true }).click();
    const docs = page.getByRole("link", { name: "Read the docs", exact: true });
    await expect(docs).toBeVisible();
    await docs.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "AI小队", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "新建AI小队", exact: true })).toHaveCount(0);
    await page.getByRole("link", { name: "Squads", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Let your agents work as a team." })).toBeVisible();

    await page.getByRole("tab", { name: "Templates", exact: true }).click();
    await expect(page.getByRole("button", { name: "Apply to project", exact: true }).first()).toBeVisible();
    await page.getByRole("tab", { name: "Workspace", exact: true }).click();
    await page.emulateMedia({ reducedMotion: "reduce" });
    await window.evaluate((target) => { target.webContents.setZoomFactor(2); target.setContentSize(1024, 900); });
    await expect.poll(() => page.evaluate(() => innerWidth)).toBe(512);
    const create = page.getByRole("link", { name: "New Squad", exact: true });
    await create.scrollIntoViewIfNeeded();
    await expect(create).toBeInViewport();
    await settlePaint(page);
    evidence.push({ state: "native-200-percent-cta", bounds: await create.boundingBox() });
    await nativeScreenshot(app, page, info.outputPath("electron-200-percent-cta.png"));
    await next.scrollIntoViewIfNeeded();
    await next.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "A role for every member", exact: true })).toBeVisible();
    await expect(next).toBeInViewport();
    await settlePaint(page);
    const zoomed = await geometry(page);
    expect(zoomed.pageWidth).toBeLessThanOrEqual(zoomed.viewport.width);
    expect(zoomed.card.x).toBeGreaterThanOrEqual(0);
    expect(zoomed.card.x + zoomed.card.width).toBeLessThanOrEqual(zoomed.viewport.width);
    expect(zoomed.groups.filter((group) => group.active !== "true").every((group) => group.inert && group.hidden === "true")).toBe(true);
    evidence.push({ state: "native-200-percent", ...zoomed });
    await nativeScreenshot(app, page, info.outputPath("electron-200-percent-carousel.png"));

    await window.evaluate((target) => { target.webContents.setZoomFactor(1); target.setContentSize(1380, 900); });
    await page.getByRole("button", { name: /^Show example 1:/ }).click();
    await page.evaluate(() => {
      document.documentElement.classList.remove("light");
      document.documentElement.classList.add("dark");
      document.documentElement.style.colorScheme = "dark";
    });
    await settlePaint(page);
    await nativeScreenshot(app, page, info.outputPath("electron-dark.png"));
    await window.dispose();
    expect(native.errors).toEqual([]);
    const daemonStarts = await app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts);
    expect(daemonStarts).toBe(0);
    evidence.push({ daemonStarts, rendererErrors: native.errors });
  } finally {
    writeFileSync(info.outputPath("native-geometry.json"), JSON.stringify(evidence, null, 2));
    if (native) await native.app.close();
    rmSync(profile, { recursive: true, force: true });
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
