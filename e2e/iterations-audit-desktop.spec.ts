import { randomUUID } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { expect, type Locator } from "@playwright/test";
import { test } from "./fixtures/project-p1-desktop";
import { iterationDateRange } from "./fixtures/iterations-i1";
import { p1DB } from "./fixtures/project-p1";

test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires an isolated local iteration API");

test("iteration audit: actual contrast, focus, controls, touch, history and responsive rendering", async ({ native }, info) => {
  test.setTimeout(180_000);
  const { page, api, workspace } = native;
  const evidence = info.outputPath("audit-evidence");
  await mkdir(evidence, { recursive: true });
  const metrics: Record<string, unknown> = {};
  const base = `/api/workspaces/${workspace.id}`;
  const settings = await api.requestJSON<{ revision: number; effective_timezone: string }>(`${base}/iteration-settings`);
  const timezone = settings.effective_timezone;
  await api.requestJSON(`${base}/iteration-settings/enable`, { method: "POST", body: { request_id: randomUUID(), expected_revision: settings.revision, confirmed_timezone: timezone } });
  const dates = iterationDateRange(timezone);
  const create = async (name: string) => {
    const receipt = await api.requestJSON<{ iteration_ids: string[] }>(`${base}/iterations`, { method: "POST", body: { request_id: randomUUID(), name, start_date: dates.startDate, end_date: dates.endDate, confirmed_timezone: timezone } });
    return receipt.iteration_ids[0]!;
  };
  const period = await create("Audit delivery");
  const next = await create("Audit next delivery");
  await create("Audit cancel plan");
  await create("Audit delete plan");
  const first = await api.createIssue(`Audit task ${"UnbrokenHistoricalTitle".repeat(12)}`, { status: "todo" });
  const second = await api.createIssue("Audit remaining task", { status: "todo" });
  const detail = () => api.requestJSON<{ iteration: { revision: number; scope_revision: number } }>(`${base}/iterations/${period}`);
  const apply = async (operation: string, moves: unknown[], start: unknown = null) => {
    const current = await detail();
    const input = { operation, iteration_id: operation === "move" ? null : period, expected_settings_revision: settings.revision + 1, expected_iteration_revision: operation === "move" ? null : current.iteration.revision, expected_scope_revision: operation === "move" ? null : current.iteration.scope_revision, reason: "Isolated audit fixture", moves, start };
    const preview = await api.requestJSON<{ draft: unknown; preview_hash: string; invalid_items: unknown[] }>(`${base}/iteration-previews`, { method: "POST", body: input });
    expect(preview.invalid_items).toEqual([]);
    await api.requestJSON(`${base}/iteration-operations`, { method: "POST", body: { request_id: randomUUID(), draft: preview.draft, preview_hash: preview.preview_hash } });
  };
  const issueRevisions = await Promise.all([first, second].map(issue => api.requestJSON<{ revision: number }>(`/api/issues/${issue.id}`)));
  await apply("move", [first, second].map((issue, index) => ({ issue_id: issue.id, expected_issue_revision: issueRevisions[index]!.revision, expected_source_id: null, target_id: period, allow_completed: false })));
  await apply("start", [], { target_id: period, mode: "scheduled", terminal_choices: [] });
  // Backdate only this task-owned fixture to exercise an already-running overdue
  // iteration without waiting for its planned end date to pass in real time.
  const yesterday = new Date(Date.parse(`${dates.startDate}T00:00:00Z`) - 86400000).toISOString().slice(0, 10);
  const priorWeek = new Date(Date.parse(`${dates.startDate}T00:00:00Z`) - 7 * 86400000).toISOString().slice(0, 10);
  await p1DB("UPDATE iteration SET start_date=$2,end_date=$3 WHERE id=$1 AND workspace_id=$4", [period, priorWeek, yesterday, workspace.id]);

  const sidebarIterations = () => page.getByRole("link", { name: "Iterations", exact: true }).and(page.locator('[data-sidebar="menu-button"]'));
  const openPeriod = async () => {
    await sidebarIterations().click();
    await page.getByRole("link", { name: "Audit delivery", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Audit delivery", exact: true })).toBeVisible();
  };
  const capture = async (name: string) => {
    await page.evaluate(() => document.fonts.ready);
    const path = `${evidence}/${name}.png`;
    await page.screenshot({ path, animations: "disabled" });
    await info.attach(name, { path, contentType: "image/png" });
  };
  const setTheme = async (theme: "light" | "dark") => {
    // Exercise the real ThemeProvider's cross-window preference listener so
    // its class, native color-scheme and React theme state change together.
    await page.evaluate(value => {
      localStorage.setItem("theme", value);
      window.dispatchEvent(new StorageEvent("storage", { key: "theme", newValue: value, storageArea: localStorage }));
    }, theme);
    await expect.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)).toBe(theme);
  };
  const contrast = async (element: Locator) => element.evaluate(node => {
    const canvas = document.createElement("canvas"); canvas.width = canvas.height = 1;
    const ctx = canvas.getContext("2d")!;
    ctx.fillStyle = "white"; ctx.fillRect(0, 0, 1, 1);
    const ancestors: Element[] = [];
    for (let item: Element | null = node; item; item = item.parentElement) ancestors.unshift(item);
    for (const item of ancestors) { ctx.fillStyle = getComputedStyle(item).backgroundColor; ctx.fillRect(0, 0, 1, 1); }
    const background = Array.from(ctx.getImageData(0, 0, 1, 1).data).slice(0, 3);
    const color = getComputedStyle(node).color; ctx.fillStyle = color; ctx.fillRect(0, 0, 1, 1);
    const foreground = Array.from(ctx.getImageData(0, 0, 1, 1).data).slice(0, 3);
    const luminance = (rgb: number[]) => rgb.map(value => { const n = value / 255; return n <= 0.04045 ? n / 12.92 : ((n + 0.055) / 1.055) ** 2.4; }).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index]!, 0);
    const a = luminance(foreground), b = luminance(background);
    return { color, foreground, background, ratio: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05) };
  });

  await openPeriod();
  const main = page.getByRole("main");
  await expect(main).toHaveCount(1);
  await expect(main.getByRole("textbox", { name: "Search tasks", exact: true })).toBeVisible();
  await expect(main.getByText("Past planned end date — this iteration remains active.", { exact: true })).toBeVisible();
  await expect(main.locator("select")).toHaveCount(0);
  for (const theme of ["light", "dark"] as const) {
    await setTheme(theme);
    await main.locator('[data-slot="badge"]').evaluate(async node => { getComputedStyle(node).color; await Promise.all(node.getAnimations().map(animation => animation.finished.catch(() => {}))); });
    const warning = await contrast(main.getByText("Past planned end date — this iteration remains active.", { exact: true }));
    const badge = await contrast(main.locator('[data-slot="badge"]'));
    metrics[`${theme}Contrast`] = { warning, badge };
    expect(warning.ratio).toBeGreaterThanOrEqual(4.5);
    expect(badge.ratio).toBeGreaterThanOrEqual(4.5);
    await capture(`tasks-${theme}-wide`);
  }
  await setTheme("light");
  await main.getByRole("tab", { name: /^Tasks/ }).focus();
  await page.keyboard.press("Tab");
  const panel = main.getByRole("tabpanel", { name: /^Tasks/ });
  expect(await panel.evaluate(node => node.scrollWidth <= node.clientWidth + 1)).toBe(true);
  await expect(panel).toBeFocused();
  metrics.panelFocus = await panel.evaluate(node => { const style = getComputedStyle(node); return { outlineWidth: style.outlineWidth, outlineStyle: style.outlineStyle, outlineOffset: style.outlineOffset }; });
  expect(metrics.panelFocus).toMatchObject({ outlineWidth: "2px", outlineStyle: "solid" });
  await capture("keyboard-panel-focus");

  const taskRequests: string[] = [];
  const observe = (request: { url(): string }) => { if (request.url().includes(`/iterations/${period}/issues?`)) taskRequests.push(request.url()); };
  page.on("request", observe);
  await main.locator("summary").filter({ hasText: "Filter and group tasks" }).click();
  await main.getByRole("combobox", { name: "Status", exact: true }).click();
  await expect(page.getByRole("option", { name: "Todo", exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  page.off("request", observe);
  metrics.filterOpeningTaskRequests = taskRequests;
  expect(taskRequests).toHaveLength(0);

  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Emulation.setDeviceMetricsOverride", { width: 680, height: 900, deviceScaleFactor: 1, mobile: true });
  await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 });
  metrics.coarsePointer = await page.evaluate(() => ({ coarse: matchMedia("(pointer: coarse)").matches, width: innerWidth, touchPoints: navigator.maxTouchPoints }));
  expect(metrics.coarsePointer).toMatchObject({ coarse: true, touchPoints: 5 });
  metrics.headerTargets = await main.getByRole("button").evaluateAll(nodes => nodes.filter(node => node.getBoundingClientRect().height > 0).map(node => ({ name: node.textContent?.trim() || node.getAttribute("aria-label"), height: node.getBoundingClientRect().height, width: node.getBoundingClientRect().width })));
  for (const target of metrics.headerTargets as { height: number; width: number }[]) expect(target.height).toBeGreaterThanOrEqual(44);
  for (const control of await main.getByRole("combobox").all()) expect((await control.boundingBox())!.height).toBeGreaterThanOrEqual(44);
  expect(await panel.evaluate(node => node.scrollWidth <= node.clientWidth + 1)).toBe(true);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await capture("tasks-light-coarse-narrow");
  await main.getByRole("tab", { name: "Progress", exact: true }).click();
  await capture("progress-light-narrow");
  await main.getByRole("tab", { name: "Scope changes", exact: true }).click();
  await capture("scope-light-narrow");
  await cdp.send("Emulation.clearDeviceMetricsOverride");
  await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
  await main.getByRole("button", { name: "End iteration", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "Preview changes", exact: true })).toBeDisabled();
  await expect(dialog.getByText("Enter a reason before previewing this change.", { exact: true })).toBeVisible();
  await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("Close isolated audit delivery");
  await dialog.getByRole("combobox", { name: "Move remaining work to", exact: true }).click();
  await page.getByRole("option", { name: "Audit next delivery", exact: true }).click();
  await dialog.getByRole("button", { name: "Preview changes", exact: true }).click();
  await expect(dialog.getByRole("region", { name: "Change summary", exact: true })).toBeVisible();
  await expect(dialog.getByRole("region", { name: "Task changes", exact: true })).toBeVisible();
  await capture("end-preview-light-wide");
  await dialog.getByRole("button", { name: "End iteration", exact: true }).click();
  await expect(dialog).toBeHidden();
  await main.getByRole("tab", { name: /^Task snapshot/ }).click();
  await main.getByRole("button", { name: new RegExp(first.title) }).click();
  await expect(main.getByRole("heading", { name: "Historical value", exact: true })).toBeVisible();
  await cdp.send("Emulation.setDeviceMetricsOverride", { width: 680, height: 900, deviceScaleFactor: 1, mobile: false });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  metrics.comparison = await main.getByRole("button", { name: new RegExp(first.title) }).evaluate(node => ({ height: node.getBoundingClientRect().height, whiteSpace: getComputedStyle(node).whiteSpace }));
  expect(metrics.comparison).toMatchObject({ whiteSpace: "normal" });
  await capture("history-comparison-light-narrow");
  await cdp.send("Emulation.clearDeviceMetricsOverride");
  await sidebarIterations().click();
  await capture("overview-light-wide");
  await page.getByRole("button", { name: "Create iteration", exact: true }).click();
  await expect(page.getByRole("dialog").locator('[data-slot="textarea"]')).toBeVisible();
  await expect(page.getByRole("dialog").getByRole("combobox", { name: "Coordinator", exact: true })).toBeVisible();
  await capture("create-form-light-wide");
  await page.keyboard.press("Escape");
  for (const [plan, action] of [["Audit cancel plan", "Cancel iteration"], ["Audit delete plan", "Delete unused iteration"]]) {
    await page.getByRole("link", { name: plan, exact: true }).click();
    await page.getByRole("main").getByRole("button", { name: "More actions", exact: true }).click();
    await page.getByRole("menuitem", { name: action, exact: true }).click();
    const confirmation = page.getByRole("dialog");
    await confirmation.getByRole("textbox", { name: "Reason", exact: true }).fill("Review unused audit plan");
    await confirmation.getByRole("button", { name: "Preview changes", exact: true }).click();
    const confirm = confirmation.getByRole("button", { name: action, exact: true });
    await expect(confirm).toBeEnabled();
    const colors: Record<string, unknown> = {};
    for (const theme of ["light", "dark"] as const) {
      await setTheme(theme);
      await confirm.evaluate(async node => { getComputedStyle(node).color; await Promise.all(node.getAnimations().map(animation => animation.finished.catch(() => {}))); });
      const measured = await contrast(confirm);
      colors[theme] = measured;
      expect(measured.ratio).toBeGreaterThanOrEqual(4.5);
    }
    metrics[`${action}Contrast`] = colors;
    await setTheme("light");
    await capture(action === "Cancel iteration" ? "cancel-preview-light-wide" : "delete-preview-light-wide");
    await page.keyboard.press("Escape");
    await sidebarIterations().click();
  }
  await page.getByRole("link", { name: "Settings", exact: true }).and(page.locator('[data-sidebar="menu-button"]')).click();
  await page.getByRole("tab", { name: "Iterations", exact: true }).click();
  await capture("settings-light-wide");
  await page.getByRole("switch", { name: "Enable iterations", exact: true }).click();
  const disableDialog = page.getByRole("dialog");
  await disableDialog.getByRole("textbox", { name: "Reason", exact: true }).fill("Review workspace-wide iteration disable");
  await disableDialog.getByRole("button", { name: "Preview changes", exact: true }).click();
  await expect(disableDialog.getByRole("region", { name: "Change summary", exact: true })).toBeVisible();
  const disableConfirm = disableDialog.getByRole("button", { name: "Disable all iterations", exact: true });
  await expect(disableConfirm).toBeEnabled();
  for (const size of ["wide", "coarse-narrow"] as const) {
    if (size === "coarse-narrow") {
      await cdp.send("Emulation.setDeviceMetricsOverride", { width: 680, height: 900, deviceScaleFactor: 1, mobile: true });
      await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 });
    }
    for (const theme of ["light", "dark"] as const) {
      await setTheme(theme);
      await disableConfirm.scrollIntoViewIfNeeded();
      await disableDialog.evaluate(async node => {
        getComputedStyle(node).color;
        await Promise.all(node.getAnimations({ subtree: true }).filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(() => {})));
      });
      const measured = await contrast(disableConfirm);
      const destructiveForeground = await disableConfirm.evaluate(node => {
        const probe = document.createElement("span");
        probe.style.color = "var(--destructive-foreground)";
        probe.style.position = "absolute";
        probe.style.visibility = "hidden";
        node.append(probe);
        try {
          return { token: getComputedStyle(node).getPropertyValue("--destructive-foreground").trim(), color: getComputedStyle(probe).color };
        } finally {
          probe.remove();
        }
      });
      expect(destructiveForeground.token).not.toBe("");
      expect(measured.color).toBe(destructiveForeground.color);
      expect(measured.ratio).toBeGreaterThanOrEqual(4.5);
      const target = await disableConfirm.boundingBox();
      expect(target).not.toBeNull();
      const viewport = await page.evaluate(() => ({ width: innerWidth, height: innerHeight, coarse: matchMedia("(pointer: coarse)").matches, touchPoints: navigator.maxTouchPoints }));
      if (size === "coarse-narrow") {
        expect(viewport).toMatchObject({ width: 680, coarse: true, touchPoints: 5 });
        expect(target!.height).toBeGreaterThanOrEqual(44);
        expect(target!.width).toBeGreaterThanOrEqual(44);
      } else {
        expect(viewport.coarse).toBe(false);
      }
      const overflow = await disableDialog.evaluate(node => ({ dialogScrollWidth: node.scrollWidth, dialogClientWidth: node.clientWidth, documentScrollWidth: document.documentElement.scrollWidth }));
      expect(overflow.dialogScrollWidth).toBeLessThanOrEqual(overflow.dialogClientWidth + 1);
      expect(overflow.documentScrollWidth).toBeLessThanOrEqual(viewport.width + 1);
      metrics[`disablePreview-${theme}-${size}`] = { contrast: measured, destructiveForeground, target, viewport, overflow };
      await capture(`disable-preview-${theme}-${size}`);
    }
  }
  await page.keyboard.press("Escape");
  await expect(disableDialog).toBeHidden();
  await cdp.send("Emulation.clearDeviceMetricsOverride");
  await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
  await setTheme("light");
  await expect(page.getByRole("switch", { name: "Enable iterations", exact: true })).toBeChecked();
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
  await page.evaluate(() => localStorage.setItem("multica-locale", "zh-Hans"));
  await page.reload();
  await page.getByRole("link", { name: "迭代", exact: true }).and(page.locator('[data-sidebar="menu-button"]')).click();
  await expect(page.getByRole("heading", { name: "迭代", exact: true })).toBeVisible();
  await cdp.send("Emulation.setDeviceMetricsOverride", { width: 680, height: 900, deviceScaleFactor: 1, mobile: false });
  await setTheme("dark");
  await capture("overview-zh-dark-narrow");
  await page.getByRole("button", { name: "创建迭代", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("textbox", { name: "名称", exact: true })).toBeVisible();
  metrics.dateColorScheme = await page.getByRole("dialog").locator('input[type="date"]').evaluateAll(nodes => nodes.map(node => getComputedStyle(node).colorScheme));
  expect(metrics.dateColorScheme).toEqual(["dark", "dark"]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await capture("create-form-zh-dark-narrow");
  metrics.locales = ["en", "zh-Hans"];
  metrics.nextIteration = next;
  metrics.fixture = { workspace: workspace.id, period, timezone };
  await writeFile(`${evidence}/rendered-metrics.json`, JSON.stringify(metrics, null, 2));
  await cdp.detach();
});
