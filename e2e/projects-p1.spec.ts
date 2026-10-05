import { randomUUID } from "node:crypto";
import { writeFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";
import { p1Session, p1Authenticate, p1Project, p1Overview, p1EnableTriage, p1Failure, type P1Issue } from "./fixtures/project-p1";

// Business cases are independent specs by domain. Keep this 30-sample
// convergence measurement intact; samples are not separate test cases.
test("P1-C01 convergence performance: two real pages observe 30 due-date, assignee and admission changes within five seconds", async ({ page }, info) => {
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
