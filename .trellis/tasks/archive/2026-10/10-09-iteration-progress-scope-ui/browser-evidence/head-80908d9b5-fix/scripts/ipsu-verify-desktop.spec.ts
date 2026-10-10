import { randomUUID } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { expect, type Page } from "@playwright/test";
import { test } from "./fixtures/project-p1-desktop";
import { iterationDateRange } from "./fixtures/iterations-i1";

// Task-only live check for 10-09-iteration-progress-scope-ui (AC-10). It runs
// the production Desktop renderer against an isolated API and keeps every
// screenshot outside Playwright's temporary output directory.
test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires an isolated local iteration API");
const evidence = process.env.IPSU_EVIDENCE_DIR!;

type Stats = Record<string, number | null>;
type Detail = { iteration: { revision: number; scope_revision: number; status: string }; snapshot: null | { statistics: Stats; events: { id: string; kind: string }[]; destinations: unknown[] } };

const labels = {
  en: { nav: "Iterations", history: "Past iterations", tasks: /^Task snapshot/, progress: "Progress", scope: "Scope changes", scopeFilter: "Scope changes", all: "All activity", filter: "Activity filter", records: (n: number) => `${n} records` },
  "zh-Hans": { nav: "迭代", history: "历史迭代", tasks: /^任务快照/, progress: "进展", scope: "范围变化", scopeFilter: "范围变化", all: "全部活动", filter: "筛选活动", records: (n: number) => `${n} 条记录` },
} as const;

test("progress and scope activity: live keyboard, locale, theme and width check", async ({ native }) => {
  test.setTimeout(300_000);
  if (!evidence) throw new Error("IPSU_EVIDENCE_DIR is required");
  await mkdir(evidence, { recursive: true });
  const { page, api, workspace, desktop } = native;
  const captures: string[] = [];
  const overflowMetrics: Record<string, unknown> = {};
  const viewportMetrics: Record<string, unknown> = {};
  const metrics: Record<string, unknown> = { captures, overflow: overflowMetrics, viewports: viewportMetrics };
  const save = () => writeFile(`${evidence}/live-metrics.json`, JSON.stringify(metrics, null, 2));
  const base = `/api/workspaces/${workspace.id}`;

  // Fixture mirrors the audited example: original A-D, E added during the
  // period, B cancelled, A/E completed, C carried over and D removed at closure.
  const settings = await api.requestJSON<{ revision: number; effective_timezone: string }>(`${base}/iteration-settings`);
  const timezone = settings.effective_timezone;
  await api.requestJSON(`${base}/iteration-settings/enable`, { method: "POST", body: { request_id: randomUUID(), expected_revision: settings.revision, confirmed_timezone: timezone } });
  const dates = iterationDateRange(timezone);
  const create = async (name: string) => (await api.requestJSON<{ iteration_ids: string[] }>(`${base}/iterations`, { method: "POST", body: { request_id: randomUUID(), name, start_date: dates.startDate, end_date: dates.endDate, confirmed_timezone: timezone } })).iteration_ids[0]!;
  const period = await create("Delivery check");
  const next = await create("Delivery check next");
  const [a, b, c, d] = await Promise.all(["Alpha checkout flow", "Beta export", "Gamma settings copy", "Delta onboarding"].map(title => api.createIssue(title, { status: "todo" })));
  const detail = () => api.requestJSON<Detail>(`${base}/iterations/${period}`);
  const revision = async (id: string) => (await api.requestJSON<{ revision: number }>(`/api/issues/${id}`)).revision;
  const apply = async (operation: string, moves: unknown[], start: unknown = null) => {
    const current = await detail();
    const input = { operation, iteration_id: operation === "move" ? null : period, expected_settings_revision: settings.revision + 1, expected_iteration_revision: operation === "move" ? null : current.iteration.revision, expected_scope_revision: operation === "move" ? null : current.iteration.scope_revision, reason: "Isolated progress and scope check", moves, start };
    const preview = await api.requestJSON<{ draft: unknown; preview_hash: string; invalid_items: unknown[] }>(`${base}/iteration-previews`, { method: "POST", body: input });
    expect(preview.invalid_items).toEqual([]);
    await api.requestJSON(`${base}/iteration-operations`, { method: "POST", body: { request_id: randomUUID(), draft: preview.draft, preview_hash: preview.preview_hash } });
  };
  const move = async (issue: { id: string }, source: string | null, target: string | null) => ({ issue_id: issue.id, expected_issue_revision: await revision(issue.id), expected_source_id: source, target_id: target, allow_completed: false });
  await apply("move", await Promise.all([a, b, c, d].map(issue => move(issue, null, period))));
  await apply("start", [], { target_id: period, mode: "scheduled", terminal_choices: [] });
  const e = await api.createIssue("Epsilon late request", { status: "todo" });
  await apply("move", [await move(e, null, period)]);
  await api.updateIssue(b.id, { status: "cancelled" });
  await api.updateIssue(a.id, { status: "done" });
  await api.updateIssue(e.id, { status: "done" });
  await api.updateIssue(a.id, { title: "Alpha checkout flow (renamed)" });
  await apply("end", [await move(c, period, next), await move(d, period, null)]);
  const closed = await detail();
  metrics.snapshotStatistics = closed.snapshot?.statistics;
  metrics.snapshotEventCount = closed.snapshot?.events.length;
  metrics.snapshotEventKinds = closed.snapshot?.events.map(event => event.kind);
  await save();
  expect(closed.iteration.status).toBe("completed");
  expect(closed.snapshot?.statistics).toMatchObject({ original: 4, current: 5, cancelled: 1, effective: 4, completed: 2, original_completed: 1, remaining: 2, initial_effective: 4, net_effective_change: 0, added_unique: 1, cancel_events: 1, removed_events: 0 });
  const eventIds = closed.snapshot!.events.map(event => event.id);

  const sizes = { wide: { width: 1360, height: 1040 }, narrow: { width: 900, height: 900 } } as const;
  const resize = async (size: keyof typeof sizes) => {
    await desktop.evaluate(({ BrowserWindow }, target) => { BrowserWindow.getAllWindows()[0]!.setContentSize(target.width, target.height); }, sizes[size]);
    await expect.poll(() => page.evaluate(() => innerWidth)).toBe(sizes[size].width);
    viewportMetrics[size] = await page.evaluate(() => ({ width: innerWidth, height: innerHeight }));
  };
  const setTheme = async (theme: "light" | "dark") => {
    await page.evaluate(value => {
      localStorage.setItem("theme", value);
      window.dispatchEvent(new StorageEvent("storage", { key: "theme", newValue: value, storageArea: localStorage }));
    }, theme);
    await expect.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)).toBe(theme);
  };
  const overflow = () => page.evaluate(() => {
    const panel = [...document.querySelectorAll<HTMLElement>('[role="tabpanel"]')].find(node => node.offsetParent !== null && node.closest("main"));
    return { document: document.documentElement.scrollWidth - innerWidth, panel: panel ? panel.scrollWidth - panel.clientWidth : null };
  });
  const capture = async (name: string) => {
    await page.evaluate(async () => { await document.fonts.ready; await Promise.all(document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(() => {}))); });
    const measured = await overflow();
    overflowMetrics[name] = measured;
    expect(measured.document, `${name} document overflow`).toBeLessThanOrEqual(1);
    if (measured.panel !== null) expect(measured.panel, `${name} panel overflow`).toBeLessThanOrEqual(1);
    await page.screenshot({ path: `${evidence}/${name}.png`, animations: "disabled" });
    captures.push(name);
  };
  const main = page.getByRole("main");
  const tab = (name: string | RegExp) => main.getByRole("tab", typeof name === "string" ? { name, exact: true } : { name });
  const panel = (name: string) => main.getByRole("tabpanel", { name, exact: true });
  const openPeriod = async (locale: keyof typeof labels) => {
    await page.getByRole("link", { name: labels[locale].nav, exact: true }).and(page.locator('[data-sidebar="menu-button"]')).click();
    await main.getByRole("tab", { name: labels[locale].history }).click();
    await main.getByRole("link", { name: "Delivery check", exact: true }).click();
    await expect(main.getByRole("heading", { name: "Delivery check", exact: true })).toBeVisible();
  };
  const matrix = async (locale: keyof typeof labels) => {
    const l = labels[locale];
    for (const theme of ["light", "dark"] as const) for (const size of ["wide", "narrow"] as const) {
      await setTheme(theme); await resize(size);
      await tab(l.progress).click(); await expect(panel(l.progress)).toBeVisible();
      await capture(`progress-${locale}-${theme}-${size}`);
      await tab(l.scope).click(); await expect(panel(l.scope).getByRole("group", { name: l.filter, exact: true })).toBeVisible();
      await capture(`scope-${locale}-${theme}-${size}`);
    }
    await setTheme("light"); await resize("wide");
  };
  const disclose = async (summary: ReturnType<Page["locator"]>, key: "Enter" | " ") => {
    await summary.focus(); await expect(summary).toBeFocused();
    await page.keyboard.press(key === " " ? "Space" : key);
    await expect.poll(() => summary.evaluate(node => (node.parentElement as HTMLDetailsElement).open)).toBe(true);
  };

  // English: keyboard tab activation, then the visual matrix, then semantics.
  await setTheme("light"); await resize("wide");
  await openPeriod("en");
  const en = labels.en;
  // P2 fix: the ending time sits inside the frozen sentence, not after it.
  const frozenNote = page.locator("header time").filter({ hasText: /^This history was frozen when the iteration ended on .+\.$/ });
  await expect(frozenNote).toHaveCount(1);
  metrics.frozenNoteEn = await frozenNote.innerText();
  await tab(en.tasks).focus();
  await page.keyboard.press("ArrowRight");
  await expect(tab(en.progress)).toBeFocused();
  metrics.arrowOnlySelectsProgress = await tab(en.progress).getAttribute("aria-selected");
  await page.keyboard.press("Enter");
  await expect(tab(en.progress)).toHaveAttribute("aria-selected", "true");
  await expect(panel(en.progress)).toBeVisible();
  await page.keyboard.press("ArrowRight");
  await expect(tab(en.scope)).toBeFocused();
  await page.keyboard.press("Space");
  await expect(tab(en.scope)).toHaveAttribute("aria-selected", "true");
  await expect(panel(en.scope)).toBeVisible();
  await capture("keyboard-scope-tab-en-light-wide");
  await matrix("en");

  await tab(en.progress).click();
  const progress = panel(en.progress);
  const delivery = progress.getByRole("region", { name: "Delivery summary", exact: true });
  const deliveryText = (await delivery.innerText()).replace(/\s+/g, " ");
  metrics.deliveryText = deliveryText.slice(0, 400);
  expect(deliveryText).toMatch(/Effective scope completion 2 \/ 4 50%/);
  expect(deliveryText).toMatch(/Original commitment completion 1 \/ 4 25%/);
  expect(deliveryText).toMatch(/Remaining 2/);
  const closure = progress.getByRole("region", { name: "Unfinished work at closure", exact: true });
  const closureList = (await closure.getByRole("list", { name: "Unfinished work at closure", exact: true }).innerText()).replace(/\s+/g, " ");
  metrics.closureList = closureList;
  expect(closureList).toMatch(/Gamma settings copy.*Carried to.*Delivery check next/);
  expect(closureList).toMatch(/Delta onboarding.*Removed from iteration at closure/);
  expect(closureList).not.toMatch(/Alpha|Beta|Epsilon/);
  expect((await closure.innerText()).replace(/\s+/g, " ")).toMatch(/Tasks carried over at closure 1 Tasks removed at closure 1/);
  await disclose(progress.locator("summary").filter({ hasText: "View count details" }), "Enter");
  await disclose(progress.locator("summary").filter({ hasText: "View closure details" }), " ");
  await expect(progress.getByText("Isolated progress and scope check", { exact: true })).toBeVisible();
  await disclose(progress.locator("summary").filter({ hasText: "View chart data" }), "Enter");
  await expect(progress.locator("table")).toBeVisible();

  await tab(en.scope).click();
  const scope = panel(en.scope);
  const summaryText = (await scope.getByRole("region", { name: "Effective scope summary", exact: true }).innerText()).replace(/\s+/g, " ");
  metrics.scopeSummary = summaryText.slice(0, 400);
  expect(summaryText).toMatch(/Initial effective scope 4 tasks/);
  expect(summaryText).toMatch(/Effective scope at closure 4 tasks/);
  expect(summaryText).toMatch(/Net effective change 0 tasks/);
  expect(summaryText).toMatch(/Distinct tasks added 1 task/);
  expect(summaryText).toMatch(/Cancellations 1 event/);
  const filter = scope.getByRole("group", { name: en.filter, exact: true });
  const scopeButton = filter.getByRole("button", { name: en.scopeFilter, exact: true });
  const allButton = filter.getByRole("button", { name: en.all, exact: true });
  const recordStatus = scope.getByRole("status").filter({ hasText: /records?$/ });
  await expect(scopeButton).toHaveAttribute("aria-pressed", "true");
  const scopeRecords = Number((await recordStatus.innerText()).match(/\d+/)![0]);
  await allButton.focus(); await page.keyboard.press("Space");
  await expect(allButton).toHaveAttribute("aria-pressed", "true");
  await expect(scopeButton).toHaveAttribute("aria-pressed", "false");
  await expect(recordStatus).toHaveText(en.records(eventIds.length));
  metrics.recordCounts = { scope: scopeRecords, all: eventIds.length };
  expect(scopeRecords).toBeLessThan(eventIds.length);
  await capture("scope-all-en-light-wide");
  while (await scope.getByRole("button", { name: "Show more activity", exact: true }).isVisible()) await scope.getByRole("button", { name: "Show more activity", exact: true }).click();
  await disclose(scope.locator("summary").filter({ hasText: /^View \d+ records?$/ }).first(), "Enter");
  await scope.locator("details").evaluateAll(nodes => { for (const node of nodes) (node as HTMLDetailsElement).open = true; });
  const expanded = await scope.innerText();
  // P2 fix: a recorded move from or to no iteration is described in the past tense.
  expect(expanded).toContain("Not in an iteration");
  expect(expanded).not.toContain("No current iteration");
  const missing = eventIds.filter(id => !expanded.includes(id));
  metrics.missingEventIds = missing;
  expect(missing).toEqual([]);
  await scopeButton.focus(); await page.keyboard.press("Enter");
  await expect(scopeButton).toHaveAttribute("aria-pressed", "true");
  await expect(recordStatus).toHaveText(en.records(scopeRecords));
  await save();

  // Chinese: same matrix plus the localized filter and frozen figures.
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
  await page.evaluate(() => localStorage.setItem("multica-locale", "zh-Hans"));
  await page.reload();
  await openPeriod("zh-Hans");
  const zh = labels["zh-Hans"];
  await matrix("zh-Hans");
  const zhFrozen = page.locator("header time").filter({ hasText: /^迭代已于 .+ 结束，此历史已冻结。$/ });
  await expect(zhFrozen).toHaveCount(1);
  metrics.frozenNoteZh = await zhFrozen.innerText();
  await tab(zh.progress).click();
  // The progress panel is hidden while another tab is active, so check it here.
  await expect(panel(zh.progress).getByRole("region", { name: "结束时未完成任务去向", exact: true })).toBeVisible();
  expect((await panel(zh.progress).innerText()).replace(/\s+/g, " ")).toMatch(/有效范围完成率 2 \/ 4 50%.*原始承诺完成率 1 \/ 4 25%/);
  await tab(zh.scope).click();
  const zhAll = panel(zh.scope).getByRole("group", { name: zh.filter, exact: true }).getByRole("button", { name: zh.all, exact: true });
  await zhAll.focus(); await page.keyboard.press("Enter");
  await expect(zhAll).toHaveAttribute("aria-pressed", "true");
  await expect(panel(zh.scope).getByRole("status").filter({ hasText: /条记录$/ })).toHaveText(zh.records(eventIds.length));
  const zhScopeText = await panel(zh.scope).innerText();
  expect(zhScopeText).toContain("不在迭代中");
  expect(zhScopeText).not.toContain("无当前迭代");
  await capture("scope-all-zh-Hans-light-wide");
  await setTheme("dark"); await resize("narrow");
  await capture("scope-all-zh-Hans-dark-narrow");
  metrics.locales = ["en", "zh-Hans"];
  await save();
});
