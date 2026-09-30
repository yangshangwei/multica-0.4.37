import { test, expect, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import { TestApiClient } from "./fixtures";

async function measureLoad(page: Page) {
  return page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
    const navigation = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming;
    const resources = performance.getEntriesByType("resource") as PerformanceResourceTiming[];
    const metrics = (window as unknown as { squadMetrics: { lcp: number; cls: number; longTasks: number; events: number[] } }).squadMetrics;
    return {
      ttfbMs: navigation.responseStart - navigation.requestStart,
      fcpMs: performance.getEntriesByName("first-contentful-paint")[0]?.startTime ?? null,
      lcpMs: metrics.lcp,
      cls: metrics.cls,
      longTasksMs: metrics.longTasks,
      domInteractiveMs: navigation.domInteractive,
      loadMs: navigation.loadEventEnd,
      readyMs: performance.now(),
      requests: resources.length,
      transferredBytes: resources.reduce((sum, entry) => sum + entry.transferSize, 0),
      scriptBytes: resources.filter((entry) => entry.initiatorType === "script").reduce((sum, entry) => sum + entry.encodedBodySize, 0),
      slowest: [...resources].sort((a, b) => b.duration - a.duration).slice(0, 5).map((entry) => ({ path: new URL(entry.name).pathname, durationMs: entry.duration, bytes: entry.encodedBodySize })),
    };
  });
}

test("squad audit: browser interactions, responsive layout and performance", async ({ page }, info) => {
  test.setTimeout(180_000);
  const api = new TestApiClient();
  const run = Date.now().toString(36);
  await api.login(`e2e-squad-audit-${run}@multica.ai`, "Squad audit tester");
  const workspace = await api.ensureWorkspace("Squad audit", `squad-audit-${run}`);
  const errors: string[] = [];
  const requests: string[] = [];
  page.on("requestfailed", (request) => requests.push(`failed ${new URL(request.url()).origin}${new URL(request.url()).pathname} ${request.failure()?.errorText}`));
  page.on("response", (response) => { if (response.status() >= 400) requests.push(`${response.status()} ${new URL(response.url()).pathname}`); });
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    await api.markUserOnboarded();
    const runtime = await api.seedProjectRuntime();
    for (const [key, name] of [
      ["release", "Release squad"], ["review-gate", "Review squad"],
      ["maintenance", "Maintenance squad"], ["bug-fix", "Bug fix squad"],
      ["incident", "Incident squad"], ["feature-delivery", "Feature squad"],
      ["docs", "Documentation squad"], ["discovery", "Discovery squad"],
    ]) {
      await api.requestJSON("/api/squads/from-template", { method: "POST", body: { template_key: key, runtime_id: runtime.id, name } });
    }
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      const metrics = { lcp: 0, cls: 0, longTasks: 0, events: [] as number[] };
      (window as unknown as { squadMetrics: typeof metrics }).squadMetrics = metrics;
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) metrics.lcp = entry.startTime;
      }).observe({ type: "largest-contentful-paint", buffered: true });
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          const shift = entry as PerformanceEntry & { hadRecentInput: boolean; value: number };
          if (!shift.hadRecentInput) metrics.cls += shift.value;
        }
      }).observe({ type: "layout-shift", buffered: true });
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) metrics.longTasks += entry.duration;
      }).observe({ type: "longtask", buffered: true });
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) metrics.events.push(entry.duration);
      }).observe({ type: "event", buffered: true, durationThreshold: 16 } as PerformanceObserverInit);
    }, token);
    await page.setViewportSize({ width: 1440, height: 1000 });
    const samples = [];
    for (let sample = 0; sample < 3; sample++) {
      await page.goto(`/${workspace.slug}/squads`, { waitUntil: "load" });
      await expect(page.getByRole("link", { name: "Release squad", exact: true })).toBeVisible({ timeout: 30_000 });
      samples.push(await measureLoad(page));
    }
    const phase = process.env.SQUAD_AUDIT_PHASE ?? "after";
    await mkdir(".omx/reports/squad-browser", { recursive: true });
    await writeFile(`.omx/reports/squad-browser/${phase}-performance.json`, JSON.stringify({ environment: `local Next.js ${phase.startsWith("production") ? "production build" : "development"}, Chromium, 1440x1000, unthrottled; first navigation then two warm navigations; cross-origin API byte sizes unavailable`, samples }, null, 2));
    await page.screenshot({ path: `.omx/reports/squad-browser/${phase}-desktop.png`, animations: "disabled" });
    await info.attach("performance", { body: JSON.stringify(samples), contentType: "application/json" });
    if (phase === "before") return;

    const releaseRow = page.getByRole("row").filter({ has: page.getByRole("link", { name: "Release squad", exact: true }) });
    const squadLink = releaseRow.getByRole("link", { name: "Release squad", exact: true });
    const leaderLink = releaseRow.locator('a[href*="/agents/"]').first();
    const leaderHref = await leaderLink.getAttribute("href");
    const leaderName = (await leaderLink.innerText()).trim();
    await squadLink.focus();
    await page.keyboard.press("Tab");
    await expect(leaderLink).toBeFocused();
    await leaderLink.press("Enter");
    await expect(page).toHaveURL(new RegExp(`${leaderHref}$`));
    await page.goBack();
    await expect(squadLink).toBeVisible();

    await page.getByRole("button", { name: "Display", exact: true }).click();
    await page.getByRole("switch", { name: "Created by", exact: true }).check();
    await page.getByRole("switch", { name: "Created", exact: true }).check();
    await page.keyboard.press("Escape");
    const creatorLink = releaseRow.getByRole("link", { name: "Squad audit tester", exact: true });
    const creatorHref = await creatorLink.getAttribute("href");
    await creatorLink.focus();
    await creatorLink.press("Enter");
    await expect(page).toHaveURL(new RegExp(`${creatorHref}$`));
    await page.goBack();
    await expect(squadLink).toBeVisible();

    // The actual PreviewCard primitive opens on keyboard focus. Its details
    // link must be visible even with the pointer outside the preview.
    await page.mouse.move(0, 0);
    await releaseRow.locator('[data-slot="hover-card-trigger"]').first().focus();
    const preview = page.locator('[data-slot="hover-card-content"]');
    await expect(preview).toBeVisible();
    const detail = preview.getByRole("link").first();
    await detail.focus();
    await expect(detail).toBeFocused();
    await expect(detail).toHaveCSS("opacity", "1");
    await expect(detail).not.toHaveCSS("box-shadow", "none");
    await page.keyboard.press("Escape");
    await squadLink.focus();

    const interactions: { action: string; ms: number }[] = [];
    const filterStart = await page.evaluate(() => performance.now());
    await page.getByRole("button", { name: "Filter", exact: true }).click();
    await page.getByRole("menuitem", { name: "Leader", exact: true }).click();
    await page.getByRole("menuitemcheckbox", { name: new RegExp(leaderName) }).click();
    await page.keyboard.press("Escape");
    await page.keyboard.press("Escape");
    await expect(page.getByRole("row").filter({ has: page.locator('a[href*="/squads/"]') })).toHaveCount(1);
    interactions.push({ action: "open and apply leader filter (Playwright wall time)", ms: await page.evaluate(() => performance.now()) - filterStart });
    await expect(page.getByRole("status").filter({ hasText: leaderName })).toBeVisible();

    const contrast: { theme: string; ratio: number }[] = [];
    for (const theme of ["light", "dark"]) {
      await page.evaluate((value) => {
        document.documentElement.classList.toggle("dark", value === "dark");
        document.documentElement.classList.toggle("light", value === "light");
      }, theme);
      const ratio = await page.getByRole("button", { name: "Filter", exact: true }).evaluate(async (element) => {
        // Changing theme may cancel an in-flight color transition.
        await Promise.allSettled(element.getAnimations().map((animation) => animation.finished));
        await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
        await Promise.allSettled(element.getAnimations().map((animation) => animation.finished));
        const style = getComputedStyle(element);
        const canvas = document.createElement("canvas");
        canvas.width = canvas.height = 1;
        const context = canvas.getContext("2d")!;
        const luminance = (color: string) => {
          context.clearRect(0, 0, 1, 1);
          context.fillStyle = color;
          context.fillRect(0, 0, 1, 1);
          const rgb = [...context.getImageData(0, 0, 1, 1).data].slice(0, 3).map((channel) => channel / 255).map((channel) => channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4);
          return rgb[0]! * 0.2126 + rgb[1]! * 0.7152 + rgb[2]! * 0.0722;
        };
        const fg = luminance(style.color), bg = luminance(style.backgroundColor);
        return (Math.max(fg, bg) + 0.05) / (Math.min(fg, bg) + 0.05);
      });
      expect(ratio).toBeGreaterThanOrEqual(4.5);
      contrast.push({ theme, ratio });
      await page.screenshot({ path: `.omx/reports/squad-browser/${phase}-${theme}-filter.png`, animations: "disabled" });
    }
    await page.getByRole("button", { name: "Clear filters", exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("row").filter({ has: page.locator('a[href*="/squads/"]') })).toHaveCount(8);
    await page.evaluate(() => { document.documentElement.classList.remove("dark"); document.documentElement.classList.add("light"); });

    // Sorting and row actions must keep navigation in place.
    const sortedHeader = page.getByRole("columnheader", { name: "Squad", exact: true });
    await sortedHeader.getByRole("button").click();
    await expect(sortedHeader).toHaveAttribute("aria-sort", "descending");
    const rowActions = releaseRow.getByRole("button", { name: "Squad actions", exact: true });
    await rowActions.focus();
    await rowActions.press("Enter");
    await expect(page.getByRole("menuitem", { name: "Archive", exact: true })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(rowActions).toBeFocused();
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/squads$`));
    await page.getByRole("button", { name: "New Squad", exact: true }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(page.getByRole("button", { name: /Create custom squad/ })).toBeVisible();
    await page.keyboard.press("Escape");

    const layouts = [];
    for (const width of [1440, 1024, 900, 768, 390, 320]) {
      await page.setViewportSize({ width, height: 1000 });
      const dimensions = await page.getByRole("table").evaluate((element) => {
        const scroller = element.parentElement!;
        return { windowWidth: innerWidth, containerWidth: scroller.clientWidth, scrollWidth: scroller.scrollWidth, pageWidth: document.documentElement.scrollWidth };
      });
      expect(dimensions.pageWidth).toBeLessThanOrEqual(width);
      expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.containerWidth + 1);
      layouts.push(dimensions);
      await page.screenshot({ path: `.omx/reports/squad-browser/${phase}-${width}.png`, animations: "disabled" });
    }
    const cdp = await page.context().newCDPSession(page);
    await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 1 });
    await page.setViewportSize({ width: 390, height: 844 });
    expect(await page.evaluate(() => matchMedia("(pointer: coarse)").matches)).toBe(true);
    const touchTargets = [];
    for (const name of ["Filter", "Display", "New Squad"]) {
      const bounds = await page.getByRole("button", { name, exact: true }).boundingBox();
      expect(bounds?.height).toBeGreaterThanOrEqual(44);
      expect(bounds?.width).toBeGreaterThanOrEqual(44);
      touchTargets.push({ name, bounds });
    }
    const actionBounds = await rowActions.boundingBox();
    expect(actionBounds?.height).toBeGreaterThanOrEqual(44);
    expect(actionBounds?.width).toBeGreaterThanOrEqual(44);
    await page.screenshot({ path: `.omx/reports/squad-browser/${phase}-touch.png`, animations: "disabled" });

    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: new URL(page.url()).origin }]);
    await page.reload();
    await expect(page.getByRole("tab", { name: "工作空间", exact: true })).toBeVisible();
    await page.getByRole("tab", { name: "模板", exact: true }).click();
    await expect(page).toHaveURL(/view=templates/);
    await page.getByRole("tab", { name: "工作空间", exact: true }).click();
    await expect(squadLink).toBeVisible();
    await page.screenshot({ path: `.omx/reports/squad-browser/${phase}-zh-touch.png`, animations: "disabled" });
    await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.screenshot({ path: `.omx/reports/squad-browser/${phase}-zh-desktop.png`, animations: "disabled" });

    // Collect a slower-CPU warm-cache lab sample separately from the
    // unthrottled measurements; it is not a production field INP metric.
    await cdp.send("Emulation.setCPUThrottlingRate", { rate: 4 });
    await page.reload({ waitUntil: "load" });
    await expect(squadLink).toBeVisible({ timeout: 30_000 });
    const cpu4x = await measureLoad(page);
    const filterButton = page.getByRole("button", { name: "筛选", exact: true });
    const openStart = await page.evaluate(() => performance.now());
    await filterButton.click();
    await expect(page.getByRole("menuitem", { name: "队长", exact: true })).toBeVisible();
    interactions.push({ action: "open filter at 4x CPU (Playwright wall time)", ms: await page.evaluate(() => performance.now()) - openStart });
    await page.keyboard.press("Escape");
    const eventDurations = await page.evaluate(() => (window as unknown as { squadMetrics: { events: number[] } }).squadMetrics.events);
    await cdp.send("Emulation.setCPUThrottlingRate", { rate: 1 });
    await writeFile(`.omx/reports/squad-browser/${phase}-interaction.json`, JSON.stringify({ contrast, interactions, layouts, touchTargets, cpu4x, eventDurationsMs: eventDurations, errors, requests }, null, 2));
    expect(errors).toEqual([]);
  } catch (error) {
    await mkdir(".omx/reports/squad-browser", { recursive: true });
    await page.screenshot({ path: ".omx/reports/squad-browser/failure.png" });
    await writeFile(".omx/reports/squad-browser/failure.txt", `${page.url()}\n${await page.locator("body").innerText()}\n${JSON.stringify({ errors, requests })}`);
    throw error;
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
