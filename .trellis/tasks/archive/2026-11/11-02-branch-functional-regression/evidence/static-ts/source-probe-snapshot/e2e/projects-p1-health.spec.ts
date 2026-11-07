import { expect, test } from "@playwright/test";
import { p1Capture, p1Failure, p1Overview, p1Project, p1Session, type P1Project } from "./fixtures/project-p1";
import { p1FormalGolden, p1Metric, p1PendingCandidate } from "./fixtures/project-p1-health";

// H01/H04/H05–H11 split the former statistics/five-view/acceptance journey.
// H02 adds the candidate-only empty set; H03 independently proves all-done
// scope closure. Neither terminal case may manufacture human acceptance.
test("P1-H01 six done plus two cancelled plus two open display distinct totals and legacy closure", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const project = await p1Project(api);
    await p1FormalGolden(api, project);
    await p1PendingCandidate(api, project.id);
    const overview = await p1Overview(api, project.id);
    expect(overview.statistics.counts).toMatchObject({ total: 10, completed: 6, cancelled: 2, open: 2 });
    expect(overview.statistics.closure_ratio).toBe(.8);
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).done_count).toBe(8);
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    for (const [label, value] of [["Formal issues", "10"], ["Completed", "6"], ["Cancelled", "2"], ["Open", "2"], ["Scope closed", "80%"]] as const) {
      await expect(p1Metric(page, label)).toHaveText(value);
    }
    await expect(page.getByText(/acceptance: No acceptance recorded/)).toHaveCount(2);
    await p1Capture(page, info, "formal-six-two-two");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-H02 a candidate-only project has no percentage while its own overdue date remains visible", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const project = await p1Project(api, { due_date: "2020-01-01" });
    await p1PendingCandidate(api, project.id, "Pending candidate is not an empty-scope completion");
    const overview = await p1Overview(api, project.id);
    expect(overview.statistics.counts).toMatchObject({ total: 0, completed: 0, cancelled: 0, open: 0 });
    expect(overview.statistics.closure_ratio).toBeNull();
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    await expect(page.getByText("No formal issues yet", { exact: true })).toBeVisible();
    for (const label of ["Formal issues", "Completed", "Cancelled", "Open"]) await expect(p1Metric(page, label)).toHaveText("0");
    await expect(p1Metric(page, "Scope closed")).toHaveText("Not available");
    await expect(page.getByText("The project is past its due date.", { exact: true })).toBeVisible();
    await expect(page.getByText(/acceptance: No acceptance recorded/)).toHaveCount(2);
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe("planned");
    await p1Capture(page, info, "candidate-only-empty-formal-scope");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

for (const scenario of [
  { id: "P1-H03", status: "done", completed: 2, cancelled: 0, title: "all completed issues close scope without accepting the goal or completing the project" },
  { id: "P1-H04", status: "cancelled", completed: 0, cancelled: 2, title: "all cancelled issues close scope without claiming delivered work or acceptance" },
] as const) {
  test(`${scenario.id} ${scenario.title}`, async ({ page }, info) => {
    const { api, workspace } = await p1Session(page);
    try {
      const project = await p1Project(api);
      for (let i = 0; i < 2; i++) await api.createIssue(`${scenario.status} scope item ${i}`, { project_id: project.id, status: scenario.status });
      const overview = await p1Overview(api, project.id);
      expect(overview.statistics.counts).toMatchObject({ total: 2, completed: scenario.completed, cancelled: scenario.cancelled, open: 0 });
      expect(overview.statistics.closure_ratio).toBe(1);
      expect(overview.current_description_acceptance).toBeNull();
      expect(overview.latest_acceptance).toBeNull();
      await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
      for (const [label, value] of [["Formal issues", "2"], ["Completed", String(scenario.completed)], ["Cancelled", String(scenario.cancelled)], ["Open", "0"], ["Scope closed", "100%"]] as const) {
        await expect(p1Metric(page, label)).toHaveText(value);
      }
      await expect(page.getByText(/acceptance: No acceptance recorded/)).toHaveCount(2);
      expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe("planned");
      await p1Capture(page, info, `${scenario.status}-closure-not-acceptance`);
    } catch (error) { await p1Failure(page, info); throw error; }
    finally { await api.deleteFeatureWorkspace(workspace.id); }
  });
}

for (const [index, mode] of ["board", "list", "table", "swimlane", "gantt"].entries()) {
  test(`P1-H${String(index + 5).padStart(2, "0")} ${mode} independently retains formal issues and excludes candidate-only project links`, async ({ page }, info) => {
    const { api, workspace } = await p1Session(page);
    try {
      const project = await p1Project(api);
      await p1FormalGolden(api, project);
      const candidate = await p1PendingCandidate(api, project.id);
      await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
      await page.getByRole("button", { name: "Issues", exact: true }).click();
      await expect(page).toHaveURL(/section=issues/);
      await page.evaluate(({ slug, id, mode }) => localStorage.setItem(`multica_issue_surface_views:${slug}`, JSON.stringify({ state: { surfaces: { [`project:${id}`]: { state: { viewMode: mode }, updatedAt: new Date().toISOString() } } }, version: 0 })), { slug: workspace.slug, id: project.id, mode });
      await page.reload();
      await expect(page.getByRole("button", { name: mode[0]!.toUpperCase() + mode.slice(1), exact: true })).toBeVisible();
      await expect(page.getByText("Formal item 9", { exact: true }).first()).toBeVisible();
      await expect(page.getByText(candidate.title, { exact: true })).toHaveCount(0);
      expect((await p1Overview(api, project.id)).statistics.counts.total).toBe(10);
      await p1Capture(page, info, `formal-${mode}`);
    } catch (error) { await p1Failure(page, info); throw error; }
    finally { await api.deleteFeatureWorkspace(workspace.id); }
  });
}

test("P1-H10 ordinary triage acceptance adds one formal issue without starting the default squad", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const runtime = await api.seedProjectRuntime();
    const squad = await api.requestJSON<{ squad: { id: string } }>("/api/squads/from-template", { method: "POST", body: { template_key: "feature-delivery", runtime_id: runtime.id, name: "P1 safe fixture squad" } });
    const project = await p1Project(api, { execution_squads: [{ squad_id: squad.squad.id }] });
    const issues = await p1FormalGolden(api, project);
    const pending = await p1PendingCandidate(api, project.id);
    expect((await p1Overview(api, project.id)).statistics.counts.total).toBe(10);
    await page.goto(`/${workspace.slug}/triage?issue=${pending.id}`);
    await page.getByRole("button", { name: "Accept", exact: true }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Accept", exact: true }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect.poll(async () => (await p1Overview(api, project.id)).statistics.counts.total).toBe(11);
    expect(await api.requestJSON(`/api/issues/${pending.id}`)).toMatchObject({ project_id: project.id, admission_status: "accepted", status: "todo", assignee_type: null, assignee_id: null });
    expect(await api.countIssueDispatches(pending.id)).toBe(0);
    for (const issue of issues) expect(await api.countIssueDispatches(issue.id)).toBe(0);
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    await expect(p1Metric(page, "Formal issues")).toHaveText("11");
    await page.reload();
    await expect(p1Metric(page, "Formal issues")).toHaveText("11");
    expect(await api.countIssueDispatches(pending.id)).toBe(0);
    await p1Capture(page, info, "accepted-once-without-execution");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-H11 finishing the open remainder closes a mixed done and cancelled scope without human acceptance", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const project = await p1Project(api);
    const issues = await p1FormalGolden(api, project);
    expect((await p1Overview(api, project.id)).statistics.closure_ratio).toBe(.8);
    for (const issue of issues.slice(8)) await api.requestJSON(`/api/issues/${issue.id}`, { method: "PUT", body: { status: "done" } });
    const closed = await p1Overview(api, project.id);
    expect(closed.statistics.counts).toMatchObject({ total: 10, completed: 8, cancelled: 2, open: 0 });
    expect(closed.statistics.closure_ratio).toBe(1);
    expect(closed.current_description_acceptance).toBeNull();
    expect(closed.latest_acceptance).toBeNull();
    await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
    await expect(p1Metric(page, "Completed")).toHaveText("8");
    await expect(p1Metric(page, "Cancelled")).toHaveText("2");
    await expect(p1Metric(page, "Scope closed")).toHaveText("100%");
    await expect(page.getByText(/acceptance: No acceptance recorded/)).toHaveCount(2);
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe("planned");
    await p1Capture(page, info, "mixed-terminal-closure-not-acceptance");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});
