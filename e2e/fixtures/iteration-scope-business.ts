import { randomUUID } from "node:crypto";
import { writeFile } from "node:fs/promises";
import { expect, type CDPSession, type Locator, type Page, type TestInfo } from "@playwright/test";
import type { TestApiClient } from "../fixtures";
import { iterationDateRange } from "./iterations-i1";
import { p1NoOverflow, p1Project, type p1Session } from "./project-p1";

export type ScopeLocale = "en" | "zh-Hans";
type Session = Awaited<ReturnType<typeof p1Session>>;
type Period = { id: string; name: string; status: string; revision: number; scope_revision: number; started_at: string | null };
type ScopeIssue = { id: string; identifier: string; title: string; revision: number; current_iteration_id: string | null; status: string };
type RecordedIssue = { issue_id: string; identifier: string; title: string; status_category: string };
type ScopeEvent = { id: string; issue_id: string | null; kind: string; sequence: number; operation_id: string; reason: string | null };
type Statistics = {
  original: number; current: number; initial_effective: number; effective: number;
  cancelled: number; added_unique: number; removed_events: number; reentry_events: number;
  cancel_events: number; reopen_events: number; net_effective_change: number;
};
type Detail = { iteration: Period; statistics: Statistics; snapshot: { statistics: Statistics; scope: RecordedIssue[]; original: RecordedIssue[]; events: ScopeEvent[] } | null };
type FlowOptions = { locale: ScopeLocale; native?: boolean };
type ScopeViewport = { width: number; height: number; coarse: boolean; touchPoints: number };

const WIDE_VIEWPORT: ScopeViewport = { width: 1280, height: 1000, coarse: false, touchPoints: 0 };
const NARROW_TOUCH_VIEWPORT: ScopeViewport = { width: 680, height: 900, coarse: true, touchPoints: 5 };
const NARROW_VIEWPORT: ScopeViewport = { width: 680, height: 900, coarse: false, touchPoints: 0 };

const copy = {
  en: {
    iterations: "Iterations", past: "Past iterations", tasks: /^Tasks/,
    planned: "Planned tasks", planning: "Planning adjustments", scope: "Scope changes",
    summary: "Effective scope summary", start: "Scope at start", current: "Current effective scope",
    final: "Effective scope at closure", net: "Net effective change", added: "Tasks added after start",
    cancellations: "Cancellations", removals: "Removed during iteration", reentries: "Re-entries", reopens: "Reopens",
    allCounts: "View all scope counts", detail: "Scope metric details", back: "Back to activity",
    search: "Search task records", taskSearch: "Search tasks", category: "Change category", removed: "Removed",
    clear: "Clear filters", allChanges: "All changes", allActivity: "All activity", noMatches: "No matching activity.",
    noTasks: "No matching tasks.", audit: "Technical audit", project: "Project", assignee: "Assignee",
    filterTasks: "Filter and group tasks", priority: "Priority", high: "High",
    compare: "Compare with current task", historical: "Historical value", latest: "Current value", openCurrent: "Open current task",
    cancelledPlan: "This plan was cancelled before a commitment was recorded. Its planning history remains available.",
  },
  "zh-Hans": {
    iterations: "迭代", past: "历史迭代", tasks: /^任务/,
    planned: "计划任务", planning: "计划调整", scope: "范围变化",
    summary: "有效范围摘要", start: "开始时有效范围", current: "当前有效范围",
    final: "结束时有效范围", net: "有效范围净变化", added: "开始后新增任务",
    cancellations: "取消次数", removals: "期间移出", reentries: "重新加入次数", reopens: "重开次数",
    allCounts: "查看完整范围计数", detail: "范围指标明细", back: "返回活动",
    search: "搜索任务记录", taskSearch: "搜索任务", category: "变化类别", removed: "移出",
    clear: "清除筛选", allChanges: "全部变化", allActivity: "全部活动", noMatches: "没有匹配的活动。",
    noTasks: "没有匹配的任务。", audit: "技术审计", project: "项目", assignee: "负责人",
    filterTasks: "筛选与分组任务", priority: "优先级", high: "高",
    compare: "对照当前任务", historical: "历史值", latest: "当前值", openCurrent: "打开当前任务",
    cancelledPlan: "此计划在确定承诺前已取消，计划调整记录仍可查看。",
  },
} as const;

function escapePattern(text: string) { return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"); }
function taskCount(locale: ScopeLocale, count: number) { return locale === "en" ? `${count} task${count === 1 ? "" : "s"}` : `${count} 个任务`; }
function eventCount(locale: ScopeLocale, count: number) { return locale === "en" ? `${count} event${count === 1 ? "" : "s"}` : `${count} 次`; }
function matchingTasks(locale: ScopeLocale, count: number) { return locale === "en" ? `${count} matching task${count === 1 ? "" : "s"}` : `匹配 ${count} 个任务`; }
function matchingRecords(locale: ScopeLocale, count: number) { return locale === "en" ? `${count} matching record${count === 1 ? "" : "s"}` : `匹配 ${count} 条记录`; }
function recordCount(locale: ScopeLocale, count: number) { return locale === "en" ? `${count} record${count === 1 ? "" : "s"}` : `${count} 条记录`; }
function disclosure(container: Locator, name: string) { return container.locator("summary").filter({ hasText: new RegExp(`^${escapePattern(name)}$`) }); }

function metric(summary: Locator, locale: ScopeLocale, name: string, count: number, events = false) {
  const value = events ? eventCount(locale, count) : taskCount(locale, count);
  return summary.getByRole("button", { name: locale === "en" ? `View ${name}: ${value}` : `查看${name}：${value}`, exact: true });
}

async function selectOption(page: Page, control: Locator, option: string) {
  await control.click();
  await page.getByRole("option", { name: option, exact: true }).click();
}

async function scopeAPI(api: TestApiClient, workspace: Session["workspace"]) {
  const base = `/api/workspaces/${workspace.id}`;
  const settings = await api.requestJSON<{ revision: number; effective_timezone: string }>(`${base}/iteration-settings`);
  await api.requestJSON(`${base}/iteration-settings/enable`, {
    method: "POST", body: { request_id: randomUUID(), expected_revision: settings.revision, confirmed_timezone: settings.effective_timezone },
  });
  const detail = (id: string) => api.requestJSON<Detail>(`${base}/iterations/${id}`);
  const issue = (id: string) => api.requestJSON<ScopeIssue>(`/api/issues/${id}`);
  const scope = async (id: string) => {
    const result = await api.requestJSON<{ items: RecordedIssue[]; total: number; next_cursor: string | null }>(`${base}/iterations/${id}/issues?limit=100`);
    expect(result.next_cursor, "The small fixture must fit a complete API page").toBeNull();
    expect(result.items).toHaveLength(result.total);
    return result.items;
  };
  async function apply(draft: Record<string, unknown>) {
    const current = await api.requestJSON<{ revision: number }>(`${base}/iteration-settings`);
    const preview = await api.requestJSON<{ draft: unknown; preview_hash: string; invalid_items: unknown[] }>(`${base}/iteration-previews`, {
      method: "POST", body: { iteration_id: null, expected_iteration_revision: null, expected_scope_revision: null, expected_settings_revision: current.revision, start: null, ...draft },
    });
    expect(preview.invalid_items).toEqual([]);
    await api.requestJSON(`${base}/iteration-operations`, { method: "POST", body: { request_id: randomUUID(), draft: preview.draft, preview_hash: preview.preview_hash } });
  }
  async function moves(ids: string[], target: string | null) {
    return Promise.all(ids.map(async id => {
      const current = await issue(id);
      return { issue_id: id, expected_issue_revision: current.revision, expected_source_id: current.current_iteration_id, target_id: target, allow_completed: false };
    }));
  }
  return {
    detail, scope,
    async create(name: string) {
      const dates = iterationDateRange(settings.effective_timezone);
      const result = await api.requestJSON<{ iteration_ids: string[] }>(`${base}/iterations`, {
        method: "POST", body: { request_id: randomUUID(), name, start_date: dates.startDate, end_date: dates.endDate, confirmed_timezone: settings.effective_timezone },
      });
      expect(result.iteration_ids).toHaveLength(1);
      return (await detail(result.iteration_ids[0]!)).iteration;
    },
    async move(ids: string[], target: string | null, reason: string) {
      await apply({ operation: "move", reason, moves: await moves(ids, target) });
    },
    async lifecycle(id: string, operation: "start" | "end" | "cancel") {
      const current = await detail(id);
      const remaining = operation === "start" ? [] : (await scope(id)).filter(item => !["done", "cancelled"].includes(item.status_category));
      await apply({
        operation, iteration_id: id, expected_iteration_revision: current.iteration.revision, expected_scope_revision: current.iteration.scope_revision,
        reason: `Scope business verification: ${operation}`, moves: await moves(remaining.map(item => item.issue_id), null),
        start: operation === "start" ? { target_id: id, mode: "scheduled", terminal_choices: [] } : null,
      });
      return detail(id);
    },
    async events(id: string) {
      const result = await api.requestJSON<{ items: ScopeEvent[]; next_cursor: string | null }>(`${base}/iterations/${id}/events?limit=100`);
      expect(result.next_cursor, "The fixture must not silently assert against partial history").toBeNull();
      return result.items;
    },
  };
}

async function openPeriod(page: Page, session: Session, period: Period, options: FlowOptions) {
  if (options.native) {
    const labels = copy[options.locale];
    await page.getByRole("link", { name: labels.iterations, exact: true }).first().click();
    if (["completed", "cancelled"].includes(period.status)) await page.getByRole("tab", { name: labels.past, exact: true }).click();
    await page.getByRole("link", { name: period.name, exact: true }).click();
  } else await page.goto(`/${session.workspace.slug}/iterations/${period.id}`);
  await expect(page.getByRole("heading", { name: period.name, exact: true })).toBeVisible();
}

async function assertCurrentTask(page: Page, session: Session, task: ScopeIssue, native = false) {
  // IssueDetailRoute replaces UUID links with the known human-readable task
  // identifier. Wait for that stable identity instead of racing the initial URL.
  const path = `/${session.workspace.slug}/issues/${task.identifier}`;
  if (native) {
    // The renderer's URL never changes: Desktop owns session paths in its memory router.
    await expect.poll(() => page.evaluate(() => {
      const raw = localStorage.getItem("multica_tabs");
      if (!raw) return null;
      const { state }: { state: { activeWorkspaceSlug: string; byWorkspace: Record<string, { activeTabId: string; tabs: { id: string; url: string }[] }> } } = JSON.parse(raw);
      const group = state.byWorkspace[state.activeWorkspaceSlug];
      return group?.tabs.find(tab => tab.id === group.activeTabId)?.url ?? null;
    })).toBe(path);
  } else await expect(page).toHaveURL(new RegExp(`${escapePattern(path)}(?:[?#]|$)`));
  await expect(page.getByRole("button", { name: task.title, exact: true })).toBeVisible();
}

function readViewport(page: Page) {
  return page.evaluate(() => ({ width: innerWidth, height: innerHeight, coarse: matchMedia("(pointer: coarse)").matches, touchPoints: navigator.maxTouchPoints }));
}

async function setScopeViewport(page: Page, cdp: CDPSession, viewport: ScopeViewport) {
  // Keep Playwright's viewport state aligned with Chromium. CDP-only device
  // metrics can be overwritten when Playwright captures or restores a viewport.
  await page.setViewportSize({ width: viewport.width, height: viewport.height });
  await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: viewport.coarse, ...(viewport.coarse ? { maxTouchPoints: viewport.touchPoints } : {}) });
  await expect.poll(() => readViewport(page)).toEqual(viewport);
}

async function captureLayout(page: Page, info: TestInfo, name: string, expectedViewport: ScopeViewport, panel: Locator, detail?: Locator, controls: Locator[] = []) {
  await page.evaluate(() => document.fonts.ready);
  const viewport = await readViewport(page);
  const expectedSize = { width: expectedViewport.width, height: expectedViewport.height };
  expect(page.viewportSize(), `${name}: Playwright viewport before capture`).toEqual(expectedSize);
  expect(viewport, `${name}: rendered geometry and pointer before capture`).toEqual(expectedViewport);
  await p1NoOverflow(page);
  const containers = [panel, ...(detail ? [detail, ...await detail.getByRole("list").all()] : [])];
  const overflow = [];
  for (const container of containers) {
    if (!await container.isVisible()) continue;
    const bounds = await container.evaluate(node => ({ scrollWidth: node.scrollWidth, clientWidth: node.clientWidth }));
    expect(bounds.scrollWidth, `${name}: panel/detail/list content must not be clipped`).toBeLessThanOrEqual(bounds.clientWidth + 1);
    overflow.push(bounds);
  }
  const targets = [];
  for (const control of controls) {
    await expect(control).toBeVisible();
    const bounds = await control.boundingBox();
    expect(bounds).not.toBeNull();
    if (expectedViewport.coarse) expect(bounds!.height, `${name}: coarse control height`).toBeGreaterThanOrEqual(44);
    targets.push({ name: await control.getAttribute("aria-label") ?? await control.textContent(), ...bounds });
  }
  // The app scrolls inside its panels. Capture the actual viewport without the
  // full-page resize/restore cycle that can undo touch and device emulation.
  const path = info.outputPath(`${name}.png`);
  const png = await page.screenshot({ path, fullPage: false, animations: "disabled", scale: "css" });
  const viewportAfterCapture = await readViewport(page);
  expect(page.viewportSize(), `${name}: Playwright viewport after capture`).toEqual(expectedSize);
  expect(viewportAfterCapture, `${name}: capture preserves rendered geometry and pointer`).toEqual(expectedViewport);
  const screenshotSize = { width: png.readUInt32BE(16), height: png.readUInt32BE(20) };
  expect(screenshotSize, `${name}: PNG dimensions match the measured viewport`).toEqual(expectedSize);
  await info.attach(name, { path, contentType: "image/png" });
  return { name, expectedViewport, viewport, viewportAfterCapture, screenshotSize, overflow, targets };
}

async function saveEvidence(info: TestInfo, evidence: Record<string, unknown>) {
  const path = info.outputPath("rendered-metrics.json");
  await writeFile(path, JSON.stringify(evidence, null, 2));
  await info.attach("rendered-metrics", { path, contentType: "application/json" });
}

// The full transition/malformed-fact matrix belongs to core/iterations/scope.test.ts.
// These real-API flows own phase wiring, exact visible details and navigation on both platforms.
export async function iterationScopeBusinessFlow(page: Page, session: Session, info: TestInfo, options: FlowOptions) {
  const { api, workspace, owner } = session;
  const { locale, native = false } = options;
  const labels = copy[locale];
  const harness = await scopeAPI(api, workspace);
  const period = await harness.create(locale === "en" ? "Scope business acceptance" : "范围变化业务验证");
  const project = await p1Project(api, { title: locale === "en" ? "Scope evidence project" : "范围证据项目" });
  const createTask = (title: string, fields: Record<string, unknown> = {}) => api.requestJSON<ScopeIssue>("/api/issues", { method: "POST", body: { title, status: "todo", ...fields } });
  const alpha = await createTask(`${locale === "en" ? "Scope baseline A" : "原始任务 A"} — ${"UnbrokenTitle".repeat(12)}`, { priority: "high", project_id: project.id, assignee_type: "member", assignee_id: owner.id });
  const beta = await createTask(locale === "en" ? "Scope baseline B" : "原始任务 B");
  const added = await createTask(locale === "en" ? "Scope inserted C" : "临时加入任务 C");
  await harness.move([alpha.id, beta.id], period.id, "Prepare the original plan");
  if (native) {
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
    await page.evaluate(value => localStorage.setItem("multica-locale", value), locale);
    await page.reload();
  }
  const cdp = await page.context().newCDPSession(page);
  const layouts: Awaited<ReturnType<typeof captureLayout>>[] = [];
  const errors: string[] = [];
  const onError = (error: Error) => errors.push(error.message);
  page.on("pageerror", onError);
  try {
    await setScopeViewport(page, cdp, WIDE_VIEWPORT);
    await openPeriod(page, session, period, options);
    await page.getByRole("tab", { name: labels.planning, exact: true }).click();
    let panel = page.getByRole("tabpanel", { name: labels.planning, exact: true });
    let summary = panel.getByRole("region", { name: labels.summary, exact: true });
    const planned = await harness.detail(period.id);
    expect(planned.statistics).toMatchObject({ current: 2, initial_effective: 0, added_unique: 0, net_effective_change: 2 });
    await expect(metric(summary, locale, labels.planned, planned.statistics.current)).toBeVisible();
    for (const name of [labels.start, labels.net, labels.added]) await expect(summary.getByText(name, { exact: true })).toHaveCount(0);
    layouts.push(await captureLayout(page, info, `planned-${locale}-wide`, WIDE_VIEWPORT, panel));

    await metric(summary, locale, labels.planned, planned.statistics.current).focus();
    await page.keyboard.press("Enter");
    let detail = panel.getByRole("region", { name: labels.detail, exact: true });
    await expect(detail.getByRole("status")).toHaveText(matchingTasks(locale, 2));
    const plannedSummary = await summary.innerText();
    await selectOption(page, detail.getByRole("combobox", { name: labels.project, exact: true }), project.title);
    await selectOption(page, detail.getByRole("combobox", { name: labels.assignee, exact: true }), "P1 project owner");
    await expect(detail.getByRole("status")).toHaveText(matchingTasks(locale, 1));
    await expect(detail.getByText(alpha.title, { exact: true })).toBeVisible();
    expect(await summary.innerText()).toBe(plannedSummary);
    await detail.getByRole("textbox", { name: labels.search, exact: true }).fill("no matching planned task");
    await expect(detail.getByText(labels.noTasks, { exact: true })).toBeVisible();
    await detail.getByRole("button", { name: labels.clear, exact: true }).click();
    await expect(detail.getByRole("status")).toHaveText(matchingTasks(locale, 2));
    await detail.getByRole("button", { name: labels.back, exact: true }).click();

    await page.getByRole("tab", { name: labels.tasks }).click();
    const tasks = page.getByRole("tabpanel", { name: labels.tasks });
    await tasks.getByRole("textbox", { name: labels.taskSearch, exact: true }).fill(alpha.title);
    await disclosure(tasks, labels.filterTasks).click();
    await selectOption(page, tasks.getByRole("combobox", { name: labels.priority, exact: true }), labels.high);
    await expect(tasks.getByRole("link", { name: new RegExp(escapePattern(alpha.title)) })).toBeVisible();
    await page.getByRole("tab", { name: labels.planning, exact: true }).click();
    await harness.lifecycle(period.id, "start");
    // No reload: both Next and native must observe the actual lifecycle WebSocket update.
    await expect(page.getByRole("tab", { name: labels.scope, exact: true })).toHaveAttribute("aria-selected", "true");
    panel = page.getByRole("tabpanel", { name: labels.scope, exact: true });
    summary = panel.getByRole("region", { name: labels.summary, exact: true });
    await expect(metric(summary, locale, labels.start, 2)).toBeVisible();

    await harness.move([added.id], period.id, "Accept an urgent request after start");
    for (let occurrence = 1; occurrence <= 2; occurrence++) {
      await harness.move([added.id], null, `Reconsider the request ${occurrence}`);
      await harness.move([added.id], period.id, `Re-enter after review ${occurrence}`);
    }
    await api.updateIssue(added.id, { status: "cancelled" });
    await api.updateIssue(added.id, { status: "todo" });
    await api.updateIssue(beta.id, { status: "cancelled" });
    await api.updateIssue(beta.id, { status: "done" });
    await api.updateIssue(beta.id, { status: "todo" });
    await harness.move([added.id], null, "Return the inserted request to future planning");
    const active = await harness.detail(period.id);
    expect(active.statistics).toMatchObject({ original: 2, current: 2, initial_effective: 2, effective: 2, cancelled: 0, added_unique: 1, removed_events: 3, reentry_events: 2, cancel_events: 2, reopen_events: 2, net_effective_change: 0 });
    const events = await harness.events(period.id);
    const removals = events.filter(event => event.issue_id === added.id && event.kind === "leave").sort((a, b) => b.sequence - a.sequence);
    expect(removals).toHaveLength(active.statistics.removed_events);
    expect(new Set(removals.map(event => event.issue_id)).size).toBe(1);
    await expect(metric(summary, locale, labels.current, active.statistics.effective)).toBeVisible();
    await expect(metric(summary, locale, labels.net, active.statistics.net_effective_change)).toBeVisible();
    await expect(metric(summary, locale, labels.added, active.statistics.added_unique)).toBeVisible();
    await expect(metric(summary, locale, labels.cancellations, active.statistics.cancel_events, true)).toBeVisible();
    await disclosure(summary, labels.allCounts).click();
    for (const [label, count] of [[labels.removals, active.statistics.removed_events], [labels.reentries, active.statistics.reentry_events], [labels.reopens, active.statistics.reopen_events]] as const) {
      await expect(metric(summary, locale, label, count, true)).toBeVisible();
    }
    const activeSummary = await summary.innerText();
    await metric(summary, locale, labels.added, active.statistics.added_unique).click();
    detail = panel.getByRole("region", { name: labels.detail, exact: true });
    await expect(detail.getByRole("status")).toHaveText(matchingTasks(locale, active.statistics.added_unique));
    await expect(detail.getByRole("link", { name: new RegExp(`^${escapePattern(labels.openCurrent)}`) })).toHaveCount(1);
    await expect(detail.getByText(added.title, { exact: true })).toBeVisible();
    expect((await harness.scope(period.id)).map(task => task.issue_id)).not.toContain(added.id);
    layouts.push(await captureLayout(page, info, `scope-added-${locale}-wide`, WIDE_VIEWPORT, panel, detail));

    await metric(summary, locale, labels.removals, active.statistics.removed_events, true).click();
    await expect(detail.getByRole("status")).toHaveText(matchingRecords(locale, active.statistics.removed_events));
    const audits = disclosure(detail, labels.audit);
    await expect(audits).toHaveCount(removals.length);
    for (const event of removals) {
      await expect(detail.getByText(event.id, { exact: true })).toHaveCount(1);
      await expect(detail.getByText(event.id, { exact: true })).toBeHidden();
    }
    for (const event of events.filter(event => event.kind !== "leave")) await expect(detail.getByText(event.id, { exact: true })).toHaveCount(0);
    await audits.first().focus();
    await page.keyboard.press("Enter");
    await expect(detail.getByText(removals[0]!.id, { exact: true })).toBeVisible();
    await expect(detail.getByText("leave", { exact: true }).first()).toBeVisible();
    layouts.push(await captureLayout(page, info, `technical-audit-${locale}-wide`, WIDE_VIEWPORT, panel, detail));
    await audits.first().focus();
    await page.keyboard.press("Space");
    await expect(detail.getByText(removals[0]!.id, { exact: true })).toBeHidden();

    await setScopeViewport(page, cdp, NARROW_TOUCH_VIEWPORT);
    layouts.push(await captureLayout(page, info, `scope-metric-${locale}-narrow`, NARROW_TOUCH_VIEWPORT, panel, detail, [metric(summary, locale, labels.added, active.statistics.added_unique), audits.first()]));

    await detail.getByRole("button", { name: labels.back, exact: true }).click();
    await panel.getByRole("button", { name: labels.allActivity, exact: true }).click();
    const category = panel.getByRole("combobox", { name: labels.category, exact: true });
    await category.focus();
    await page.keyboard.press("Enter");
    await page.getByRole("option", { name: labels.removed, exact: true }).press("Enter");
    await expect(category.locator('[data-slot="select-value"]')).toHaveText(labels.removed);
    const search = panel.getByRole("textbox", { name: labels.search, exact: true });
    await search.fill(added.title);
    await expect(panel.getByRole("status")).toHaveText(matchingRecords(locale, removals.length));
    await search.fill("no matching scope task");
    await expect(panel.getByText(labels.noMatches, { exact: true })).toBeVisible();
    await expect(panel.getByRole("status")).toHaveText(matchingRecords(locale, 0));
    expect(await summary.innerText()).toBe(activeSummary);
    layouts.push(await captureLayout(page, info, `scope-empty-${locale}-narrow`, NARROW_TOUCH_VIEWPORT, panel, undefined, [search, category, panel.getByRole("button", { name: labels.clear, exact: true })]));
    await panel.getByRole("button", { name: labels.clear, exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect(search).toHaveValue("");
    await expect(category.locator('[data-slot="select-value"]')).toHaveText(labels.allChanges);
    await expect(panel.getByRole("status")).toHaveText(recordCount(locale, events.length));
    expect(await summary.innerText()).toBe(activeSummary);

    await metric(summary, locale, labels.net, active.statistics.net_effective_change).click();
    await expect(detail.getByRole("status")).toHaveText(locale === "en" ? /^[1-9]\d* matching records?$/ : /^匹配 [1-9]\d* 条记录$/);
    await expect(detail.getByText(added.title, { exact: true }).first()).toBeVisible();
    await page.getByRole("tab", { name: labels.tasks }).click();
    await expect(tasks.getByRole("textbox", { name: labels.taskSearch, exact: true })).toHaveValue(alpha.title);
    await expect(tasks.getByRole("combobox", { name: labels.priority, exact: true }).locator('[data-slot="select-value"]')).toHaveText(labels.high);
    await expect(tasks.getByRole("link", { name: new RegExp(escapePattern(alpha.title)) })).toBeVisible();
    await expect(tasks.getByRole("link", { name: new RegExp(escapePattern(beta.title)) })).toHaveCount(0);

    await setScopeViewport(page, cdp, WIDE_VIEWPORT);
    await page.getByRole("tab", { name: labels.scope, exact: true }).click();
    await metric(summary, locale, labels.added, active.statistics.added_unique).click();
    const currentLink = detail.getByRole("link", { name: new RegExp(`^${escapePattern(labels.openCurrent)}`) });
    // AppLink exposes an absolute public URL on Desktop. Both fixtures use the
    // document origin as the app URL; the DOM property normalizes Web's relative href.
    await expect(currentLink).toHaveJSProperty("href", new URL(`/${workspace.slug}/issues/${added.id}`, page.url()).href);
    await currentLink.click();
    await assertCurrentTask(page, session, added, native);

    const closed = await harness.lifecycle(period.id, "end");
    expect(closed.iteration.status).toBe("completed");
    expect(closed.snapshot).not.toBeNull();
    const changed = await api.requestJSON<ScopeIssue>(`/api/issues/${alpha.id}`, { method: "PUT", body: { title: locale === "en" ? "Current title after closure" : "结束后修改的当前标题", status: "done" } });
    expect((await harness.detail(period.id)).snapshot).toEqual(closed.snapshot);
    // Web's goto below starts a fresh document. Give the memory-router native
    // path the same frozen-read boundary, without a stale active detail cache.
    if (native) await page.reload();
    const liveHistoryReads: string[] = [];
    const currentTaskReads: string[] = [];
    const observe = (request: { url(): string }) => {
      const path = new URL(request.url()).pathname;
      if (path.endsWith(`/iterations/${period.id}/events`)) liveHistoryReads.push(path);
      if (path === `/api/issues/${alpha.id}`) currentTaskReads.push(path);
    };
    page.on("request", observe);
    try {
      await openPeriod(page, session, closed.iteration, options);
      await page.getByRole("tab", { name: labels.scope, exact: true }).click();
      panel = page.getByRole("tabpanel", { name: labels.scope, exact: true });
      summary = panel.getByRole("region", { name: labels.summary, exact: true });
      await expect(metric(summary, locale, labels.final, closed.snapshot!.statistics.effective)).toBeVisible();
      await expect(panel.getByText(changed.title, { exact: true })).toHaveCount(0);
      await metric(summary, locale, labels.final, closed.snapshot!.statistics.effective).click();
      detail = panel.getByRole("region", { name: labels.detail, exact: true });
      await expect(detail.getByRole("status")).toHaveText(matchingTasks(locale, closed.snapshot!.statistics.effective));
      expect(liveHistoryReads).toEqual([]);
      expect(currentTaskReads).toEqual([]);
      const frozenTask = detail.getByRole("listitem").filter({ has: page.getByText(alpha.title, { exact: true }) });
      await expect(frozenTask.getByText(alpha.title, { exact: true })).toBeVisible();
      await frozenTask.getByRole("button", { name: labels.compare, exact: true }).click();
      const historical = frozenTask.getByRole("heading", { name: labels.historical, exact: true }).locator("..");
      const latest = frozenTask.getByRole("heading", { name: labels.latest, exact: true }).locator("..");
      await expect(historical.getByText(alpha.title, { exact: true })).toBeVisible();
      await expect(historical.getByText(changed.title, { exact: true })).toHaveCount(0);
      await expect(latest.getByText(changed.title, { exact: true })).toBeVisible();
      expect(currentTaskReads.length).toBeGreaterThan(0);
      await expect(metric(summary, locale, labels.final, closed.snapshot!.statistics.effective)).toBeVisible();
      await historical.scrollIntoViewIfNeeded();
      layouts.push(await captureLayout(page, info, `frozen-current-separation-${locale}-wide`, WIDE_VIEWPORT, panel, detail));
      await setScopeViewport(page, cdp, NARROW_VIEWPORT);
      await historical.scrollIntoViewIfNeeded();
      layouts.push(await captureLayout(page, info, `frozen-historical-${locale}-narrow`, NARROW_VIEWPORT, panel, detail));
      await latest.scrollIntoViewIfNeeded();
      layouts.push(await captureLayout(page, info, `frozen-current-${locale}-narrow`, NARROW_VIEWPORT, panel, detail));
      await frozenTask.getByRole("link", { name: labels.openCurrent, exact: true }).click();
      await assertCurrentTask(page, session, changed, native);
    } finally { page.off("request", observe); }

    expect(errors).toEqual([]);
    await saveEvidence(info, { platform: native ? "electron" : "web", locale, workspaceId: workspace.id, iterationId: period.id, taskIds: { alpha: alpha.id, beta: beta.id, added: added.id }, planned: planned.statistics, active: active.statistics, frozen: closed.snapshot!.statistics, removalEventIds: removals.map(event => event.id), uniqueRemovedTasks: new Set(removals.map(event => event.issue_id)).size, layouts, pageErrors: errors, mockedBranches: [], realtime: "Observed start and post-start writes without reload; frozen navigation intentionally remounts." });
  } finally {
    page.off("pageerror", onError);
    await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
    await cdp.detach();
  }
}

export async function iterationEmptyPlanFlow(page: Page, session: Session, info: TestInfo) {
  const labels = copy.en;
  const layouts: Awaited<ReturnType<typeof captureLayout>>[] = [];
  await page.setViewportSize({ width: WIDE_VIEWPORT.width, height: WIDE_VIEWPORT.height });
  const harness = await scopeAPI(session.api, session.workspace);
  const period = await harness.create("Emptied planning history");
  const task = await session.api.requestJSON<ScopeIssue>("/api/issues", { method: "POST", body: { title: "Removed before the commitment", status: "todo" } });
  await harness.move([task.id], period.id, "Prepare this plan");
  await harness.move([task.id], null, "Keep the adjustment after emptying the plan");
  await openPeriod(page, session, period, { locale: "en" });
  await page.getByRole("tab", { name: labels.planning, exact: true }).click();
  const planning = page.getByRole("tabpanel", { name: labels.planning, exact: true });
  const summary = planning.getByRole("region", { name: labels.summary, exact: true });
  await expect(metric(summary, "en", labels.planned, 0)).toBeVisible();
  await planning.getByRole("textbox", { name: labels.search, exact: true }).fill(task.title);
  await expect(planning.getByRole("status")).toHaveText("2 matching records");
  await expect(planning.getByText("Keep the adjustment after emptying the plan", { exact: true })).toBeVisible();
  layouts.push(await captureLayout(page, info, "empty-plan-adjustments", WIDE_VIEWPORT, planning));

  const cancelled = await harness.lifecycle(period.id, "cancel");
  expect(cancelled.iteration.started_at).toBeNull();
  expect(cancelled.snapshot).toBeNull();
  await expect(planning).toBeVisible();
  await expect(summary.getByText(labels.cancelledPlan, { exact: true })).toBeVisible();
  for (const name of [labels.start, labels.final, labels.net, labels.added]) await expect(summary.getByText(name, { exact: true })).toHaveCount(0);
  await expect(planning.getByText("Keep the adjustment after emptying the plan", { exact: true })).toBeVisible();
  layouts.push(await captureLayout(page, info, "cancelled-before-start-history", WIDE_VIEWPORT, planning));

  const empty = await harness.create("A real empty commitment");
  const started = await harness.lifecycle(empty.id, "start");
  expect(started.iteration.started_at).not.toBeNull();
  expect(started.statistics).toMatchObject({ initial_effective: 0, effective: 0, added_unique: 0 });
  await openPeriod(page, session, started.iteration, { locale: "en" });
  await page.getByRole("tab", { name: labels.scope, exact: true }).click();
  const scope = page.getByRole("tabpanel", { name: labels.scope, exact: true });
  const startedSummary = scope.getByRole("region", { name: labels.summary, exact: true });
  await expect(metric(startedSummary, "en", labels.start, started.statistics.initial_effective)).toBeVisible();
  await expect(metric(startedSummary, "en", labels.current, started.statistics.effective)).toBeVisible();
  await expect(startedSummary.getByText(labels.planned, { exact: true })).toHaveCount(0);
  const cancelledAfterStart = await harness.lifecycle(empty.id, "cancel");
  expect(cancelledAfterStart.snapshot).not.toBeNull();
  await expect(metric(startedSummary, "en", labels.final, cancelledAfterStart.snapshot!.statistics.effective)).toBeVisible();
  await expect(startedSummary.getByText(labels.cancelledPlan, { exact: true })).toHaveCount(0);
  layouts.push(await captureLayout(page, info, "empty-started-then-cancelled", WIDE_VIEWPORT, scope));
  await saveEvidence(info, { platform: "web", workspaceId: session.workspace.id, emptiedPlan: cancelled, emptyStartedPeriod: cancelledAfterStart, layouts, mockedBranches: [] });
}
