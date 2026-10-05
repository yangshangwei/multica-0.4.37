import { randomUUID } from "node:crypto";
import { writeFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";
import { p1Session, p1Authenticate, p1Member, p1Project, p1DB, p1Overview, p1EnableTriage, p1Capture, p1Failure, p1NoOverflow, p1Raw, type P1Project, type P1Issue } from "./fixtures/project-p1";

test("P1 goal template preserves content and a real member conflict keeps both descriptions reviewable", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await p1Session(page);
  let releaseWrite: (() => void) | undefined;
  try {
    const colleague = await p1Member(workspace);
    const project = await p1Project(api);
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/projects/${project.id}`);
    await page.getByRole("button", { name: "Goal template", exact: true }).click();
    await page.getByRole("checkbox", { name: "Goal", exact: true }).check();
    await page.getByRole("checkbox", { name: "Acceptance criteria", exact: true }).check();
    await page.getByRole("button", { name: "Preview selected sections", exact: true }).click();
    await expect(page.locator("pre")).toContainText("Acceptance criteria");
    await expect(page.locator("pre")).not.toContainText("## Goal");
    await page.getByRole("button", { name: "Append selected sections", exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect.poll(async () => (await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toContain("## Acceptance criteria");
    const appended = await api.requestJSON<P1Project>(`/api/projects/${project.id}`);
    expect(appended.description).toContain("Keep the original customer goal.");
    expect(appended.description?.match(/## Goal/g)).toHaveLength(1);
    await page.getByRole("button", { name: "Goal template", exact: true }).click();
    await page.getByRole("button", { name: "Preview selected sections", exact: true }).click();
    await expect(page.getByRole("button", { name: "Append selected sections", exact: true })).toBeDisabled();
    await page.getByRole("button", { name: "Goal template", exact: true }).click();
    const held = new Promise<void>((resolve) => { releaseWrite = resolve; });
    let writeSeen = false;
    await page.route(`**/api/projects/${project.id}`, async (route) => {
      if (route.request().method() !== "PUT") return route.continue();
      writeSeen = true; await held; await route.continue();
    });
    const editor = page.getByRole("button", { name: "Description", exact: true }).locator("..").locator('[contenteditable="true"]');
    await editor.fill("Local customer goal awaiting comparison");
    await expect.poll(() => writeSeen).toBe(true);
    await colleague.api.requestJSON(`/api/projects/${project.id}`, { method: "PUT", body: { description: "Colleague's reviewed scope", expected_description_revision: appended.description_revision } });
    releaseWrite?.();
    await expect(page.getByText("The server has a newer version. Compare before saving.", { exact: true })).toBeVisible();
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toBe("Colleague's reviewed scope");
    await expect(editor).toContainText("Local customer goal awaiting comparison");
    await expect(page.getByRole("button", { name: "Use server version", exact: true })).toBeInViewport({ ratio: 1 });
    await expect(page.getByRole("button", { name: "Save reviewed draft", exact: true })).toBeInViewport({ ratio: 1 });
    await p1Capture(page, info, "description-conflict");
    await page.unroute(`**/api/projects/${project.id}`);
    await page.getByRole("button", { name: "Save reviewed draft", exact: true }).click();
    await expect.poll(async () => (await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toContain("Local customer goal awaiting comparison");
    await page.reload();
    await expect(editor).toContainText("Local customer goal awaiting comparison");
    const beforeServerChoice = await api.requestJSON<P1Project>(`/api/projects/${project.id}`);
    let secondWriteSeen = false; let blockedDescriptionReads = 0;
    const secondHeld = new Promise<void>((resolve) => { releaseWrite = resolve; });
    await page.route(`**/api/projects/${project.id}`, async (route) => {
      if (route.request().method() === "GET") {
        blockedDescriptionReads++;
        return route.fulfill({ status: 503, json: { error: "Fixture delayed description refetch" } });
      }
      if (route.request().method() !== "PUT") return route.continue();
      secondWriteSeen = true; await secondHeld; await route.continue();
    });
    await editor.fill("Local draft discarded after explicit review");
    await expect.poll(() => secondWriteSeen).toBe(true);
    await colleague.api.requestJSON(`/api/projects/${project.id}`, { method: "PUT", body: { description: "Server version chosen explicitly", expected_description_revision: beforeServerChoice.description_revision } });
    releaseWrite?.();
    await expect.poll(() => blockedDescriptionReads).toBeGreaterThan(0);
    await page.getByRole("button", { name: "Use server version", exact: true }).click();
    await expect(editor).toContainText("Server version chosen explicitly");
    await expect(editor).not.toContainText("Local draft discarded after explicit review");
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toBe("Server version chosen explicitly");
    await p1Capture(page, info, "description-adopt-server-with-read-failure");
    await page.unroute(`**/api/projects/${project.id}`);
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { releaseWrite?.(); await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1 convergence performance: two real pages observe 30 due-date, assignee and admission changes within five seconds", async ({ page }, info) => {
  test.setTimeout(240_000);
  const { api, workspace, owner } = await p1Session(page);
  const second = await page.context().newPage();
  const pages = [page, second];
  const observed = new Map<Page, { version: string; counts: Record<string, number> }>();
  const pageErrors: string[] = [];
  for (const client of pages) client.on("pageerror", (error) => pageErrors.push(error.message));
  type Sample = { kind: string; index: number; started_at: string; ended_at: string; before_version: string; after_version: string; before_counts: Record<string, number>; after_counts: Record<string, number>; client_ms: number[]; slowest_ms: number };
  const samples: Sample[] = [];
  let activeChange: { kind: string; index: number; expected: Record<string, number>; before_version: string } | undefined;
  try {
    await p1Authenticate(second, api);
    const project = await p1Project(api, { title: "P1 convergence measurement" });
    const issue: P1Issue = await api.createIssue("Observe real project health changes", { project_id: project.id, status: "todo", due_date: "2030-01-01", assignee_type: "member", assignee_id: owner.id });
    await p1EnableTriage(api);
    const pending: P1Issue[] = [];
    for (let i = 0; i < 10; i++) pending.push((await api.requestJSON<{ issue: P1Issue }>("/api/triage/items", { method: "POST", body: { title: `Convergence pending ${i}`, request_id: randomUUID(), candidate_project_id: project.id } })).issue);
    for (const client of pages) client.on("response", async (response) => {
      if (!response.url().endsWith(`/api/projects/${project.id}/overview`) || !response.ok()) return;
      try { const data = await response.json(); observed.set(client, { version: data.statistics.snapshot_version, counts: data.statistics.counts }); } catch { /* A closed page can cancel an irrelevant trailing read. */ }
    });
    await Promise.all(pages.map((client) => client.goto(`/${workspace.slug}/projects/${project.id}?section=overview`)));
    await Promise.all(pages.map(async (client) => {
      await expect(client.locator("dl > div").filter({ hasText: "Formal issues" }).locator("dd")).toHaveText("1");
      await expect.poll(() => observed.has(client)).toBe(true);
    }));
    let previous = (await p1Overview(api, project.id)).statistics;
    const measure = async (kind: string, index: number, changes: Record<string, number>, mutate: () => Promise<unknown>) => {
      const expected = { ...previous.counts, ...changes };
      const beforeVersion = previous.snapshot_version;
      const beforeCounts = previous.counts;
      activeChange = { kind, index, expected, before_version: beforeVersion };
      const started = Date.now();
      const readiness = pages.map(async (client) => {
        await client.waitForFunction((counts) => {
          const labels: Record<string, string> = { total: "Formal issues", completed: "Completed", cancelled: "Cancelled", open: "Open" };
          return Object.entries(labels).every(([key, label]) => {
            const cell = Array.from(document.querySelectorAll("dl > div")).find((entry) => entry.querySelector("dt")?.textContent?.trim() === label);
            return cell?.querySelector("dd")?.textContent?.trim() === String(counts[key]);
          }) && ["overdue", "unassigned"].every((key) => Array.from(document.querySelectorAll("button")).some((button) => button.textContent?.replace(/\s+/g, " ").trim() === `${key === "overdue" ? "Overdue" : "Unassigned"} ${counts[key]}`));
        }, expected, { timeout: 15_000, polling: 20 });
        await expect.poll(() => {
          const value = observed.get(client);
          return value?.version !== beforeVersion && !!value && Object.entries(changes).every(([key, count]) => value.counts[key] === count);
        }, { timeout: 15_000, intervals: [20, 40, 80] }).toBe(true);
        return Date.now() - started;
      });
      await mutate();
      const clientMs = await Promise.all(readiness);
      const after = (await p1Overview(api, project.id)).statistics;
      expect(observed.get(page)?.version).toBe(after.snapshot_version);
      expect(observed.get(second)?.version).toBe(after.snapshot_version);
      samples.push({ kind, index, started_at: new Date(started).toISOString(), ended_at: new Date(started + Math.max(...clientMs)).toISOString(), before_version: beforeVersion, after_version: after.snapshot_version, before_counts: beforeCounts, after_counts: after.counts, client_ms: clientMs, slowest_ms: Math.max(...clientMs) });
      previous = after;
      activeChange = undefined;
    };
    for (let i = 0; i < 10; i++) await measure("due_date", i, { overdue: i % 2 === 0 ? 1 : 0 }, () => api.requestJSON(`/api/issues/${issue.id}`, { method: "PUT", body: { due_date: i % 2 === 0 ? "2020-01-01" : "2030-01-01" } }));
    for (let i = 0; i < 10; i++) await measure("assignee", i, { unassigned: i % 2 === 0 ? 1 : 0 }, () => api.requestJSON(`/api/issues/${issue.id}`, { method: "PUT", body: { assignee_type: i % 2 === 0 ? null : "member", assignee_id: i % 2 === 0 ? null : owner.id } }));
    for (let i = 0; i < 10; i++) await measure("admission", i, { total: i + 2, open: i + 2, unassigned: i + 1 }, () => api.requestJSON(`/api/triage/items/${pending[i]!.id}/actions`, { method: "POST", body: { request_id: randomUUID(), action: "accept", expected_revision: pending[i]!.revision } }));
    const percentile = (values: number[], p: number) => [...values].sort((a, b) => a - b)[Math.ceil(values.length * p) - 1]!;
    const summary = Object.fromEntries(["all", "due_date", "assignee", "admission"].map((kind) => {
      const selected = samples.filter((sample) => kind === "all" || sample.kind === kind).map((sample) => sample.slowest_ms);
      return [kind, { count: selected.length, p50_ms: percentile(selected, .5), p95_ms: percentile(selected, .95), max_ms: Math.max(...selected) }];
    }));
    const samplePath = info.outputPath("convergence-samples.json");
    await writeFile(samplePath, JSON.stringify({ measured: "HTTP submission start to both real pages showing changed DOM counts and the same new overview version", budget_p95_ms: 5000, summary, samples, page_errors: pageErrors }, null, 2));
    await info.attach("convergence-30-raw-and-percentiles", { path: samplePath, contentType: "application/json" });
    expect(samples).toHaveLength(30);
    for (const bucket of Object.values(summary)) expect(bucket.p95_ms).toBeLessThanOrEqual(5000);
    expect(pageErrors).toEqual([]);
    expect(await api.countIssueDispatches(issue.id)).toBe(0);
    for (const item of pending) expect(await api.countIssueDispatches(item.id)).toBe(0);
  } catch (error) {
    await info.attach("convergence-partial-samples", { body: JSON.stringify({ samples, active_change: activeChange, page_errors: pageErrors }, null, 2), contentType: "application/json" });
    await p1Failure(page, info); throw error;
  } finally { await second.close(); await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1 formal totals split six completed and two cancelled; candidate acceptance adds one without execution", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await p1Session(page);
  try {
    const runtime = await api.seedProjectRuntime();
    const squad = await api.requestJSON<{ squad: { id: string } }>("/api/squads/from-template", { method: "POST", body: { template_key: "feature-delivery", runtime_id: runtime.id, name: "P1 safe fixture squad" } });
    const project = await p1Project(api, { execution_squads: [{ squad_id: squad.squad.id }] });
    const issues: P1Issue[] = [];
    for (let i = 0; i < 10; i++) issues.push(await api.createIssue(`Formal item ${i + 1}`, { project_id: project.id, status: i < 6 ? "done" : i < 8 ? "cancelled" : "todo", assignee_type: null, assignee_id: null, due_date: "2026-10-15" }));
    await p1EnableTriage(api);
    const pending = await api.requestJSON<{ issue: P1Issue }>("/api/triage/items", { method: "POST", body: { title: "Candidate not counted until accepted", request_id: randomUUID(), candidate_project_id: project.id } });
    const overview = await p1Overview(api, project.id);
    expect(overview.statistics.counts).toMatchObject({ total: 10, completed: 6, cancelled: 2, open: 2 });
    expect(overview.statistics.closure_ratio).toBe(.8);
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).done_count).toBe(8);
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    for (const [label, value] of [["Formal issues", "10"], ["Completed", "6"], ["Cancelled", "2"], ["Open", "2"], ["Scope closed", "80%"]]) {
      await expect(page.locator("dl > div").filter({ has: page.locator("dt", { hasText: new RegExp(`^${label}$`) }) }).locator("dd")).toHaveText(value!);
    }
    await expect(page.getByText(/acceptance: No acceptance recorded/)).toHaveCount(2);
    await p1Capture(page, info, "formal-six-two-two");
    await page.getByRole("button", { name: "Issues", exact: true }).click();
    await expect(page).toHaveURL(/section=issues/);
    for (const mode of ["board", "list", "table", "swimlane", "gantt"]) {
      await page.evaluate(({ slug, id, mode }) => localStorage.setItem(`multica_issue_surface_views:${slug}`, JSON.stringify({ state: { surfaces: { [`project:${id}`]: { state: { viewMode: mode }, updatedAt: new Date().toISOString() } } }, version: 0 })), { slug: workspace.slug, id: project.id, mode });
      await page.reload();
      await expect(page.getByRole("button", { name: mode[0]!.toUpperCase() + mode.slice(1), exact: true })).toBeVisible();
      await expect(page.getByText("Formal item 9", { exact: true }).first()).toBeVisible();
      await expect(page.getByText("Candidate not counted until accepted", { exact: true })).toHaveCount(0);
      await p1Capture(page, info, `formal-${mode}`);
    }
    await page.goto(`/${workspace.slug}/triage?issue=${pending.issue.id}`);
    await page.getByRole("button", { name: "Accept", exact: true }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Accept", exact: true }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect.poll(async () => (await p1Overview(api, project.id)).statistics.counts.total).toBe(11);
    expect(await api.countIssueDispatches(pending.issue.id)).toBe(0);
    for (const issue of issues) expect(await api.countIssueDispatches(issue.id)).toBe(0);
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    await expect(page.locator("dl > div").filter({ hasText: "Formal issues" }).locator("dd")).toHaveText("11");
    for (const id of [...issues.slice(8).map((issue) => issue.id), pending.issue.id]) await api.requestJSON(`/api/issues/${id}`, { method: "PUT", body: { status: "done" } });
    await page.reload();
    await expect(page.locator("dl > div").filter({ hasText: "Scope closed" }).locator("dd")).toHaveText("100%");
    await expect(page.getByText(/acceptance: No acceptance recorded/)).toHaveCount(2);
    expect((await p1Overview(api, project.id)).current_description_acceptance).toBeNull();
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe("planned");
    await p1Capture(page, info, "terminal-closure-not-acceptance");
    const cancelled = await p1Project(api, { title: "Cancelled scope remains unaccepted" });
    for (let i = 0; i < 2; i++) await api.createIssue(`Cancelled scope item ${i}`, { project_id: cancelled.id, status: "cancelled" });
    await page.goto(`/${workspace.slug}/projects/${cancelled.id}?section=overview`);
    for (const [label, value] of [["Completed", "0"], ["Cancelled", "2"], ["Open", "0"], ["Scope closed", "100%"]]) await expect(page.locator("dl > div").filter({ has: page.locator("dt", { hasText: new RegExp(`^${label}$`) }) }).locator("dd")).toHaveText(value!);
    await expect(page.getByText(/acceptance: No acceptance recorded/)).toHaveCount(2);
    expect((await api.requestJSON<P1Project>(`/api/projects/${cancelled.id}`)).status).toBe("planned");
    await p1Capture(page, info, "cancelled-closure-not-acceptance");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1 risk drilldown uses ADR-05 live cursors, sticky disclosure and fresh restart while preserving personal filters", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace, owner } = await p1Session(page);
  try {
    const project = await p1Project(api);
    // Keep the A/reset implementation and its actual run evidence in
    // .omx/projects-p1-risk-a-test.txt and projects-p1-browser-run3.log.
    const issues = await api.seedTableIssues(Array.from({ length: 156 }, (_, i) => ({ title: `Overdue delivery ${String(i + 1).padStart(3, "0")}`, status: "todo" as const })));
    const ordered = [...issues].sort((a, b) => a.id.localeCompare(b.id));
    const reentering = ordered[0]!;
    await p1DB("UPDATE issue SET project_id=$1,due_date='2020-01-01' WHERE workspace_id=$2 AND id=ANY($3::uuid[])", [project.id, workspace.id, issues.map((issue) => issue.id)]);
    await p1DB("UPDATE issue SET status='done' WHERE id=$1", [reentering.id]);
    await p1DB("UPDATE issue SET parent_issue_id=$1 WHERE id=$2", [ordered[10]!.id, ordered[20]!.id]);
    const preference = { viewMode: "list", showSubIssues: false, assigneeFilters: [{ type: "member", id: owner.id }], statusFilters: ["done"] };
    await page.addInitScript(({ slug, id, preference }) => localStorage.setItem(`multica_issue_surface_views:${slug}`, JSON.stringify({ state: { surfaces: { [`project:${id}`]: { state: preference, updatedAt: new Date().toISOString() } } }, version: 0 })), { slug: workspace.slug, id: project.id, preference });
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    await page.getByRole("button", { name: "Overdue 155", exact: true }).focus(); await page.keyboard.press("Enter");
    await expect(page.getByText("All matching formal issues, including sub-issues. Personal and saved-view filters do not apply.", { exact: true })).toBeVisible();
    const risk = page.locator("section").filter({ has: page.getByRole("heading", { name: /^Overdue ·/ }) });
    await expect(risk.getByRole("link")).toHaveCount(50);
    const reference = (await p1Overview(api, project.id)).statistics;
    await expect(risk).toContainText(reference.reference_date);
    await expect(risk).toContainText(reference.timezone);
    type RiskPage = { items: P1Issue[]; next_cursor: string | null; refreshed: boolean; snapshot_version: string; total: number };
    const first = await api.requestJSON<RiskPage>(`/api/projects/${project.id}/health/issues?signal=overdue`);
    const all = [...first.items]; let cursor = first.next_cursor;
    while (cursor) { const next = await api.requestJSON<RiskPage>(`/api/projects/${project.id}/health/issues?signal=overdue&cursor=${encodeURIComponent(cursor)}`); all.push(...next.items); cursor = next.next_cursor; }
    expect(all).toHaveLength(155); expect(all.map((issue) => issue.id)).toContain(ordered[20]!.id);
    // Change the authoritative input after the browser captured its cursor;
    // suppressing a websocket race here makes the snapshot handoff deterministic.
    let changed = false; let emptySuffix = false; let failRestart = false;
    await page.route(`**/api/projects/${project.id}/health/issues?*`, async (route) => {
      if (failRestart && !new URL(route.request().url()).searchParams.has("cursor")) return route.fulfill({ status: 503, json: { error: "Fixture first-page refresh failure", code: "project_health_unavailable", retryable: true } });
      if (!changed && new URL(route.request().url()).searchParams.has("cursor")) {
        changed = true;
        await p1DB("UPDATE issue SET status='todo',revision=revision+1 WHERE id=$1", [reentering.id]);
      } else if (emptySuffix && new URL(route.request().url()).searchParams.has("cursor")) {
        emptySuffix = false;
        await p1DB("UPDATE issue SET status='done',revision=revision+1 WHERE id=ANY($1::uuid[])", [ordered.slice(151).map((issue) => issue.id)]);
      }
      await route.continue();
    });
    const nextResponse = () => page.waitForResponse((response) => response.url().includes(`/api/projects/${project.id}/health/issues?`) && new URL(response.url()).searchParams.has("cursor") && response.ok());
    const pageTwoResponse = nextResponse();
    await page.getByRole("button", { name: "Next page", exact: true }).click();
    await expect.poll(() => changed).toBe(true);
    const second: RiskPage = await (await pageTwoResponse).json();
    expect(second.refreshed).toBe(true); expect(second.total).toBe(156);
    expect(second.items[0]!.id > first.items.at(-1)!.id).toBe(true);
    await expect(risk.getByRole("link")).toHaveCount(50);
    await expect(risk.getByRole("link", { name: new RegExp(reentering.title) })).toHaveCount(0);
    const disclosure = risk.getByRole("status").filter({ hasText: /changed/i });
    await expect(disclosure).toBeVisible();
    await expect(disclosure).toContainText(/before|earlier/i);
    const pageThreeResponse = nextResponse();
    await page.getByRole("button", { name: "Next page", exact: true }).click();
    const third: RiskPage = await (await pageThreeResponse).json();
    expect(third.refreshed).toBe(false); expect(third.items[0]!.id > second.items.at(-1)!.id).toBe(true);
    await expect(risk.getByRole("link")).toHaveCount(50);
    await expect(disclosure).toBeVisible();
    await p1Capture(page, info, "risk-live-third-page-sticky-warning");
    emptySuffix = true;
    const pageFourResponse = nextResponse();
    await page.getByRole("button", { name: "Next page", exact: true }).click();
    const fourth: RiskPage = await (await pageFourResponse).json();
    expect(fourth.total).toBe(151); expect(fourth.items).toHaveLength(0);
    await expect(risk.getByRole("link")).toHaveCount(0);
    await expect(risk.getByText("No matches after this position. Earlier matches may still exist; refresh from the start to see them.", { exact: true })).toBeVisible();
    await expect(risk.getByText("No matching issues", { exact: true })).toHaveCount(0);
    await expect(disclosure).toBeVisible();
    failRestart = true;
    const failedRestart = page.waitForResponse((response) => response.url().includes(`/api/projects/${project.id}/health/issues?`) && response.status() === 503);
    await page.getByRole("button", { name: "Refresh from start", exact: true }).click();
    await failedRestart;
    await expect(risk.getByRole("alert")).toBeVisible();
    await expect(disclosure).toBeVisible();
    await expect(risk.getByRole("link")).toHaveCount(0);
    failRestart = false;
    const restartResponse = page.waitForResponse((response) => response.url().includes(`/api/projects/${project.id}/health/issues?`) && !new URL(response.url()).searchParams.has("cursor") && response.ok());
    await page.getByRole("button", { name: "Refresh from start", exact: true }).click();
    const restarted: RiskPage = await (await restartResponse).json();
    expect(restarted.items[0]!.id).toBe(reentering.id);
    await expect(risk.getByRole("link")).toHaveCount(50);
    await expect(risk.getByRole("link", { name: new RegExp(reentering.title) })).toBeVisible();
    await expect(disclosure).toHaveCount(0);
    await info.attach("adr05-live-page-responses", { body: JSON.stringify({ second, third, fourth, restarted }), contentType: "application/json" });
    await p1Capture(page, info, "risk-fresh-start-includes-reentered-low-id");
    await page.getByRole("button", { name: "Overview", exact: true }).last().click();
    const stored = await page.evaluate((slug) => JSON.parse(localStorage.getItem(`multica_issue_surface_views:${slug}`)!), workspace.slug);
    expect(stored.state.surfaces[`project:${project.id}`].state).toMatchObject(preference);
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1 publication retries a committed lost response once, preserves corrections, and versions acceptance", async ({ page, browser }, info) => {
  test.setTimeout(180_000);
  const { api, workspace } = await p1Session(page);
  try {
    const reviewer = await p1Member(workspace);
    const runtime = await api.seedProjectRuntime();
    const agent = await api.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: "P1 reference agent", runtime_id: runtime.id } });
    const evidenceAgent = await api.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: "P1 execution evidence fixture", runtime_id: runtime.id, permission_mode: "private" } });
    const execution = randomUUID();
    await p1DB("INSERT INTO agent_task_queue(id,agent_id,runtime_id,status,created_at,completed_at,result) VALUES($1,$2,$3,'completed',now()-interval '1 minute',now(),$4::jsonb)", [execution, evidenceAgent.id, runtime.id, JSON.stringify({ summary: "Synthetic customer verification result; no process executed" })]);
    const project = await p1Project(api);
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    await page.getByRole("button", { name: "Write progress", exact: true }).click();
    const composer = page.locator('[aria-label="Write progress"]');
    const updateEditor = composer.locator('[contenteditable="true"]');
    await updateEditor.fill("Delivery verified ");
    await updateEditor.pressSequentially("@P1 reviewer");
    await page.getByRole("button", { name: /P1 reviewer$/ }).click();
    await updateEditor.pressSequentially(" @P1 reference agent");
    await page.getByRole("button", { name: /P1 reference agent.*Agent$/ }).click();
    // Mention insertion restores editor focus on the next animation frame.
    // Let that user interaction settle before moving to another input.
    await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    await expect(updateEditor).toBeFocused();
    await composer.getByRole("combobox", { name: "Evidence", exact: true }).selectOption("execution");
    const evidenceInput = composer.getByRole("textbox", { name: "One HTTP/HTTPS URL, issue UUID, or execution UUID per line.", exact: true });
    await evidenceInput.click();
    await expect(evidenceInput).toBeFocused();
    await evidenceInput.fill(execution);
    await expect(evidenceInput).toHaveValue(execution);
    await composer.getByRole("button", { name: "Add evidence", exact: true }).click();
    await composer.getByRole("checkbox", { name: "Attach current system statistics", exact: true }).check();
    const publicationPreview = page.waitForResponse((response) => response.url().endsWith(`/api/projects/${project.id}/updates/preview`) && response.request().method() === "POST");
    await composer.getByRole("button", { name: "Preview publication", exact: true }).click();
    const previewResponse = await publicationPreview;
    const previewBody = await previewResponse.json();
    await info.attach("publication-preview-request-response", { body: JSON.stringify({ status: previewResponse.status(), request: previewResponse.request().postDataJSON(), response: previewBody, composer_text: await composer.innerText().catch(() => "Composer detached") }, null, 2), contentType: "application/json" });
    expect(previewResponse.status()).toBe(200);
    expect(previewBody.recipients.map((member: { id: string }) => member.id)).toEqual([reviewer.user.id]);
    await expect(composer.getByText("P1 reviewer", { exact: true })).toBeVisible();
    expect((await api.requestJSON<{ items: unknown[] }>(`/api/projects/${project.id}/updates`)).items).toHaveLength(0);
    const requests: unknown[] = [];
    let dropped = false;
    await page.route(`**/api/projects/${project.id}/updates`, async (route) => {
      if (route.request().method() !== "POST") return route.continue();
      requests.push(route.request().postDataJSON());
      if (dropped) return route.continue();
      dropped = true;
      const response = await route.fetch(); expect(response.status()).toBe(201); await route.abort("failed");
    });
    await composer.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(composer.getByRole("alert")).toContainText("Publication has not been confirmed");
    await composer.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(composer).toHaveCount(0);
    expect(requests).toHaveLength(2); expect(requests[1]).toEqual(requests[0]);
    await page.unroute(`**/api/projects/${project.id}/updates`);
    const list = await api.requestJSON<{ items: { id: string; current_revision: number; current: { body: string } }[] }>(`/api/projects/${project.id}/updates`);
    expect(list.items).toHaveLength(1);
    const executionRead = page.waitForResponse((response) => response.url().endsWith(`/updates/${list.items[0]!.id}/revisions/1/executions/${execution}`));
    await page.getByRole("button", { name: `Execution ${execution}`, exact: true }).click();
    const executionResponse = await executionRead;
    expect(executionResponse.status()).toBe(200);
    expect((await executionResponse.json()).task).toMatchObject({ id: execution, status: "completed" });
    await expect(page.getByRole("dialog")).toBeVisible();
    await p1Capture(page, info, "execution-evidence-transcript");
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect.poll(async () => (await reviewer.api.requestJSON<{ type: string }[]>("/api/inbox")).filter((item) => item.type === "project_update").length, { timeout: 15_000 }).toBe(1);
    expect(Number((await p1DB<{ count: string }>("SELECT count(*) FROM agent_task_queue WHERE agent_id=$1", [agent.id]))[0]!.count)).toBe(0);
    await page.getByRole("button", { name: "Correct", exact: true }).click();
    await composer.locator('[contenteditable="true"]').fill("Delivery verified with corrected release date");
    await composer.getByRole("textbox", { name: "Reason for correction", exact: true }).fill("The customer approved the revised date");
    await composer.getByRole("button", { name: "Preview publication", exact: true }).click();
    await composer.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(composer).toHaveCount(0);
    await page.getByRole("button", { name: "Revision history", exact: true }).click();
    await expect(page.getByText("The customer approved the revised date", { exact: false }).first()).toBeVisible();
    const revisions = await api.requestJSON<{ items: { revision: number; body: string }[] }>(`/api/projects/${project.id}/updates/${list.items[0]!.id}/revisions`);
    expect(revisions.items.map((item) => item.revision)).toEqual([2, 1]); expect(revisions.items[1]!.body).toContain("mention://member/");
    await page.getByRole("button", { name: "Record acceptance", exact: true }).click();
    await composer.locator('[contenteditable="true"]').fill("Customer goal accepted");
    await composer.getByRole("textbox", { name: "Acceptance scope", exact: true }).fill("The customer delivery goal");
    await composer.getByRole("button", { name: "Preview publication", exact: true }).click();
    await expect(composer.getByRole("alert")).toBeVisible();
    await expect(composer.locator('[contenteditable="true"]')).toContainText("Customer goal accepted");
    await composer.getByRole("textbox", { name: "Verifiable explanation", exact: true }).fill("The customer signed the delivery checklist on this date.");
    await composer.getByRole("button", { name: "Preview publication", exact: true }).click();
    await composer.getByRole("button", { name: "Publish", exact: true }).click(); await expect(composer).toHaveCount(0);
    expect((await p1Overview(api, project.id)).current_description_acceptance?.conclusion).toBe("passed");
    await api.requestJSON(`/api/projects/${project.id}`, { method: "PUT", body: { description: "Updated goal for phase two", expected_description_revision: project.description_revision } });
    await page.reload();
    await expect(page.getByText("For an older description; review again", { exact: true }).first()).toBeVisible();
    expect((await p1Overview(api, project.id)).current_description_acceptance).toBeNull();
    await page.getByRole("button", { name: "Record acceptance", exact: true }).click();
    await composer.locator('[contenteditable="true"]').fill("Phase two still requires review");
    await composer.getByRole("combobox", { name: "Acceptance", exact: true }).selectOption("failed");
    await composer.getByRole("textbox", { name: "Acceptance scope", exact: true }).fill("Phase two");
    await composer.getByRole("button", { name: "Preview publication", exact: true }).click();
    await composer.getByRole("button", { name: "Publish", exact: true }).click(); await expect(composer).toHaveCount(0);
    expect((await p1Overview(api, project.id)).current_description_acceptance).toMatchObject({ description_revision: 2, conclusion: "failed" });
    await p1Capture(page, info, "progress-acceptance-history");
    const notification = (await reviewer.api.requestJSON<{ id: string; type: string }[]>("/api/inbox")).find((item) => item.type === "project_update");
    expect(notification).toBeDefined();
    const reviewerContext = await browser.newContext();
    try {
      const reviewerPage = await reviewerContext.newPage();
      await p1Authenticate(reviewerPage, reviewer.api);
      await reviewerPage.goto(new URL(`/${workspace.slug}/inbox?issue=${notification!.id}`, page.url()).href);
      await reviewerPage.getByRole("button", { name: "Open project update", exact: true }).click();
      await expect(reviewerPage).toHaveURL(new RegExp(`update=${list.items[0]!.id}`));
      await expect(reviewerPage.getByText("Delivery verified with corrected release date", { exact: true }).first()).toBeVisible();
      await expect(reviewerPage.getByRole("button", { name: `Execution ${execution}`, exact: true })).toHaveCount(0);
      expect((await p1Raw(reviewer.api, workspace.id, `/api/projects/${project.id}/updates/${list.items[0]!.id}/revisions/1/executions/${execution}`, "GET")).status).toBe(403);
      await p1Capture(reviewerPage, info, "notification-project-deep-link");
    } finally { await reviewerContext.close(); }
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1 completion warning allows closure without changing issues; member deletion is refused and owner deletion preserves work", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await p1Session(page);
  try {
    const member = await p1Member(workspace);
    const project = await p1Project(api, { status: "in_progress" });
    const issue: P1Issue = await api.createIssue("Unfinished customer work survives project completion", { project_id: project.id, status: "todo" });
    const runtime = await api.seedProjectRuntime();
    const agent = await api.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: "P1 execution preservation fixture", runtime_id: runtime.id, permission_mode: "private" } });
    const execution = randomUUID();
    await p1DB("INSERT INTO agent_task_queue(id,agent_id,runtime_id,issue_id,status,dispatched_at,started_at) VALUES($1,$2,$3,$4,'running',now(),now())", [execution, agent.id, runtime.id, issue.id]);
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    await page.getByRole("button", { name: "In Progress", exact: true }).click();
    await page.getByRole("menuitem", { name: "Completed", exact: true }).click();
    const confirm = page.getByRole("alertdialog");
    await expect(confirm).toContainText("1 open issues");
    await expect(confirm).toContainText("No acceptance recorded");
    await p1Capture(page, info, "completion-warning");
    await confirm.getByRole("button", { name: "Complete project", exact: true }).click();
    await expect(confirm).toHaveCount(0);
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe("completed");
    expect((await api.requestJSON<P1Issue>(`/api/issues/${issue.id}`)).status).toBe("todo");
    for (const status of ["paused", "cancelled", "in_progress", "completed"]) {
      await api.requestJSON(`/api/projects/${project.id}`, { method: "PUT", body: { status } });
      expect((await p1DB<{ status: string; issue_id: string }>("SELECT status,issue_id FROM agent_task_queue WHERE id=$1", [execution]))[0]).toEqual({ status: "running", issue_id: issue.id });
      expect((await api.requestJSON<P1Issue>(`/api/issues/${issue.id}`)).status).toBe("todo");
    }
    expect((await p1Raw(member.api, workspace.id, `/api/projects/${project.id}`, "DELETE")).status).toBe(403);
    const impact = await api.requestJSON<{ preserves_issues: boolean; preserves_executions: boolean; formal_issue_count: number }>(`/api/projects/${project.id}/delete-impact`);
    expect(impact).toMatchObject({ preserves_issues: true, preserves_executions: true, formal_issue_count: 1 });
    expect((await p1Raw(api, workspace.id, `/api/projects/${project.id}`, "DELETE")).status).toBe(204);
    expect(await api.requestJSON<P1Issue>(`/api/issues/${issue.id}`)).toMatchObject({ project_id: null, status: "todo" });
    expect(await api.countIssueDispatches(issue.id)).toBe(1);
    expect((await p1DB<{ status: string; issue_id: string }>("SELECT status,issue_id FROM agent_task_queue WHERE id=$1", [execution]))[0]).toEqual({ status: "running", issue_id: issue.id });
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1 capability and overview failures recover; Chinese compact overview supports keyboard without overflow", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await p1Session(page);
  try {
    const project = await p1Project(api);
    await page.route(`**/api/projects/${project.id}/overview`, (route) => route.fulfill({ status: 503, json: { error: "Fixture transient overview failure", code: "project_health_unavailable", retryable: true } }));
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    await expect(page.getByRole("alert").filter({ hasText: "Could not load project management" })).toBeVisible();
    await page.unroute(`**/api/projects/${project.id}/overview`);
    await page.getByRole("button", { name: "Retry", exact: true }).click();
    await expect(page.getByText("No formal issues yet", { exact: true })).toBeVisible();
    await page.route(`**/api/workspaces/${workspace.id}/project-capabilities`, async (route) => {
      const response = await route.fetch(); const body = await response.json();
      await route.fulfill({ response, json: { ...body, overview: false, updates: false, description_cas: false, planning_timezone: false } });
    });
    await page.reload();
    await expect(page.getByRole("button", { name: "Goal template", exact: true })).toHaveCount(0);
    await expect(page.getByText("This server does not support safe description editing. Update the server to edit; your text is preserved.", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Write progress", exact: true })).toHaveCount(0);
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toBe(project.description);
    await page.unroute(`**/api/workspaces/${workspace.id}/project-capabilities`);
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: new URL(page.url()).origin }]);
    await page.setViewportSize({ width: 390, height: 844 }); await page.reload();
    await expect(page.getByRole("button", { name: "记录进展", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "记录进展", exact: true }).focus(); await page.keyboard.press("Enter");
    await expect(page.getByRole("button", { name: "预览发布", exact: true })).toBeVisible();
    const compactComposer = page.locator('[aria-label="记录进展"]');
    await compactComposer.locator('[contenteditable="true"]').fill("客户验收记录已复核，下一步完成交付。");
    await compactComposer.getByRole("button", { name: "预览发布", exact: true }).focus(); await page.keyboard.press("Enter");
    await compactComposer.getByRole("button", { name: "发布", exact: true }).scrollIntoViewIfNeeded();
    await p1NoOverflow(page); await p1Capture(page, info, "chinese-compact-keyboard-composer");
    const timezoneSave = page.getByRole("button", { name: "保存", exact: true });
    await timezoneSave.evaluate((element) => element.scrollIntoView({ block: "center" }));
    expect(await timezoneSave.evaluate((element) => {
      const box = element.getBoundingClientRect();
      return [0.25, 0.5, 0.75].every((x) => [0.25, 0.5, 0.75].every((y) => {
        const target = document.elementFromPoint(box.x + box.width * x, box.y + box.height * y);
        return target === element || element.contains(target);
      }));
    }), "The floating chat action must not cover the planning-timezone save button").toBe(true);
    await p1Capture(page, info, "chinese-compact-last-control");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});
