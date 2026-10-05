import { expect, test } from "@playwright/test";
import { p1Capture, p1DB, p1Failure, p1Overview, p1Project, p1Session } from "./fixtures/project-p1";
import { p1NextRisk, p1OpenRisk, p1PagedRisks, p1PendingCandidate, p1ReenterOnContinuation, p1RiskLabels, p1RiskResponse, type P1RiskPage, type P1RiskSignal } from "./fixtures/project-p1-health";

// R01–R04 add independent real-API/UI membership checks for each signal.
// R05–R08 split the former long ADR-05 journey. R07 additionally preserves a
// nonempty page across failed restart, rather than checking only an empty one.
for (const [index, signal] of (["blocked", "overdue", "unassigned", "in_review"] as const).entries()) {
  test(`P1-R${String(index + 1).padStart(2, "0")} ${signal} drilldown shows exactly its formal risk IDs despite personal filters`, async ({ page }, info) => {
    const { api, workspace, owner } = await p1Session(page);
    try {
      const project = await p1Project(api);
      const issues = await api.seedTableIssues([
        { title: "Blocked and overdue unassigned delivery", status: "blocked" },
        { title: "Assigned parent outside this risk set", status: "todo" },
        { title: "Assigned delivery awaiting review", status: "in_review" },
        { title: "Undated unassigned child", status: "todo" },
        { title: "Completed delivery is not an open risk", status: "done" },
      ]);
      await p1DB("UPDATE issue SET project_id=$1,due_date='2030-01-01' WHERE workspace_id=$2 AND id=ANY($3::uuid[])", [project.id, workspace.id, issues.map((issue) => issue.id)]);
      await p1DB("UPDATE issue SET due_date='2020-01-01' WHERE workspace_id=$1 AND id=ANY($2::uuid[])", [workspace.id, [issues[0]!.id, issues[4]!.id]]);
      await p1DB("UPDATE issue SET assignee_type='member',assignee_id=$1 WHERE workspace_id=$2 AND id=ANY($3::uuid[])", [owner.id, workspace.id, [issues[1]!.id, issues[2]!.id]]);
      await p1DB("UPDATE issue SET parent_issue_id=$1,due_date=NULL WHERE workspace_id=$2 AND id=$3", [issues[1]!.id, workspace.id, issues[3]!.id]);
      const candidate = await p1PendingCandidate(api, project.id, "Candidate risk must remain outside the formal scope");
      const membership: Record<P1RiskSignal, number[]> = { blocked: [0], overdue: [0], unassigned: [0, 3], in_review: [2] };
      const expectedIssues = membership[signal].map((position) => issues[position]!);
      const preference = { viewMode: "list", showSubIssues: false, assigneeFilters: [{ type: "member", id: owner.id }], statusFilters: ["done"] };
      await page.addInitScript(({ slug, id, preference }) => localStorage.setItem(`multica_issue_surface_views:${slug}`, JSON.stringify({ state: { surfaces: { [`project:${id}`]: { state: preference, updatedAt: new Date().toISOString() } } }, version: 0 })), { slug: workspace.slug, id: project.id, preference });
      const { first, risk } = await p1OpenRisk(page, workspace.slug, project.id, signal, expectedIssues.length);
      expect(first.items.map((issue) => issue.id)).toEqual(expectedIssues.map((issue) => issue.id).sort());
      expect(first.total).toBe(expectedIssues.length);
      expect(first.overview.statistics.counts[signal]).toBe(expectedIssues.length);
      await expect(risk.getByRole("link")).toHaveCount(expectedIssues.length);
      for (const issue of expectedIssues) await expect(risk.getByRole("link", { name: new RegExp(issue.title) })).toBeVisible();
      await expect(risk.getByText(candidate.title, { exact: true })).toHaveCount(0);
      await expect(risk).toContainText("All matching formal issues, including sub-issues. Personal and saved-view filters do not apply.");
      const reference = (await p1Overview(api, project.id)).statistics;
      await expect(risk).toContainText(reference.reference_date);
      await expect(risk).toContainText(reference.timezone);
      if (signal === "unassigned") {
        await risk.getByRole("button", { name: "Table", exact: true }).click();
        await expect(risk.getByRole("row").filter({ hasText: "Undated unassigned child" })).toContainText("No due date");
        await expect(risk.getByRole("link")).toHaveCount(2);
      }
      await p1Capture(page, info, `${signal}-exact-formal-scope`);
      await risk.getByRole("button", { name: "Overview", exact: true }).click();
      const stored = await page.evaluate((slug) => JSON.parse(localStorage.getItem(`multica_issue_surface_views:${slug}`)!), workspace.slug);
      expect(stored.state.surfaces[`project:${project.id}`].state).toMatchObject(preference);
    } catch (error) { await p1Failure(page, info); throw error; }
    finally { await api.deleteFeatureWorkspace(workspace.id); }
  });
}

test("P1-R05 changed cursors advance by ID and keep the disclosure across a later unchanged page", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const { project, ordered, reentering } = await p1PagedRisks(api, workspace.id);
    const { first, risk } = await p1OpenRisk(page, workspace.slug, project.id, "overdue", 155);
    await expect(risk.getByRole("link")).toHaveCount(50);
    const all = [...first.items]; let cursor = first.next_cursor;
    while (cursor) {
      const next = await api.requestJSON<P1RiskPage>(`/api/projects/${project.id}/health/issues?signal=overdue&cursor=${encodeURIComponent(cursor)}`);
      all.push(...next.items); cursor = next.next_cursor;
    }
    expect(all.map((issue) => issue.id)).toEqual(ordered.slice(1).map((issue) => issue.id));
    expect(all).toHaveLength(155);
    expect(all.map((issue) => issue.id)).toContain(ordered[20]!.id);
    await p1ReenterOnContinuation(page, workspace.id, project.id, reentering.id);
    const second = await p1NextRisk(page, project.id);
    expect(second.refreshed).toBe(true); expect(second.total).toBe(156);
    expect(second.items[0]!.id > first.items.at(-1)!.id).toBe(true);
    await expect(risk.getByRole("link")).toHaveCount(50);
    await expect(risk.getByRole("link", { name: new RegExp(reentering.title) })).toHaveCount(0);
    const disclosure = risk.getByRole("status").filter({ hasText: /changed/i });
    await expect(disclosure).toBeVisible(); await expect(disclosure).toContainText(/before|earlier/i);
    const third = await p1NextRisk(page, project.id);
    expect(third.refreshed).toBe(false);
    expect(third.items[0]!.id > second.items.at(-1)!.id).toBe(true);
    await expect(risk.getByRole("link")).toHaveCount(50);
    await expect(disclosure).toBeVisible();
    await info.attach("live-continuation-responses", { body: JSON.stringify({ first, second, third }), contentType: "application/json" });
    await p1Capture(page, info, "risk-live-third-page-sticky-warning");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await page.unrouteAll({ behavior: "ignoreErrors" }); await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-R06 a successful fresh restart includes reentered low IDs and clears the changed notice", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const { project, reentering } = await p1PagedRisks(api, workspace.id);
    const { risk } = await p1OpenRisk(page, workspace.slug, project.id, "overdue", 155);
    await p1ReenterOnContinuation(page, workspace.id, project.id, reentering.id);
    const second = await p1NextRisk(page, project.id);
    expect(second.refreshed).toBe(true);
    await expect(risk.getByRole("status").filter({ hasText: /changed/i })).toBeVisible();
    await expect(risk.getByRole("link", { name: new RegExp(reentering.title) })).toHaveCount(0);
    const response = p1RiskResponse(page, project.id, false);
    await risk.getByRole("button", { name: "Refresh from start", exact: true }).click();
    const raw = await response;
    const restarted: P1RiskPage = await raw.json();
    expect(new URL(raw.url()).searchParams.has("snapshot_version")).toBe(false);
    expect(restarted.items[0]!.id).toBe(reentering.id);
    expect(restarted.total).toBe(156);
    await expect(risk.getByRole("link")).toHaveCount(50);
    await expect(risk.getByRole("link", { name: new RegExp(reentering.title) })).toBeVisible();
    await expect(risk.getByRole("status").filter({ hasText: /changed/i })).toHaveCount(0);
    await p1Capture(page, info, "risk-fresh-start-includes-reentered-low-id");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await page.unrouteAll({ behavior: "ignoreErrors" }); await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-R07 failed fresh restart preserves the current rows and sticky notice until recovery succeeds", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const { project, reentering } = await p1PagedRisks(api, workspace.id);
    const { risk } = await p1OpenRisk(page, workspace.slug, project.id, "overdue", 155);
    await p1ReenterOnContinuation(page, workspace.id, project.id, reentering.id);
    const second = await p1NextRisk(page, project.id);
    expect(second.refreshed).toBe(true);
    await expect(risk.getByRole("link")).toHaveCount(50);
    const before = await risk.getByRole("link").allTextContents();
    await page.unroute(`**/api/projects/${project.id}/health/issues?*`);
    await page.route(`**/api/projects/${project.id}/health/issues?*`, async (route) => {
      if (!new URL(route.request().url()).searchParams.has("cursor")) return route.fulfill({ status: 503, json: { error: "Fixture first-page refresh failure", code: "project_health_unavailable", retryable: true } });
      await route.continue();
    });
    const failed = p1RiskResponse(page, project.id, false, 503);
    await risk.getByRole("button", { name: "Refresh from start", exact: true }).click();
    await failed;
    await expect(risk.getByRole("alert")).toBeVisible();
    await expect(risk.getByRole("status").filter({ hasText: /changed/i })).toBeVisible();
    expect(await risk.getByRole("link").allTextContents()).toEqual(before);
    await p1Capture(page, info, "risk-failed-restart-retains-current-page");
    await page.unroute(`**/api/projects/${project.id}/health/issues?*`);
    const recovered = p1RiskResponse(page, project.id, false);
    await risk.getByRole("alert").getByRole("button", { name: "Retry", exact: true }).click();
    const restarted: P1RiskPage = await (await recovered).json();
    expect(restarted.items[0]!.id).toBe(reentering.id);
    await expect(risk.getByRole("alert")).toHaveCount(0);
    await expect(risk.getByRole("status").filter({ hasText: /changed/i })).toHaveCount(0);
    await expect(risk.getByRole("link", { name: new RegExp(reentering.title) })).toBeVisible();
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await page.unrouteAll({ behavior: "ignoreErrors" }); await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-R08 an emptied suffix with a positive total does not claim that the project has no risks", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const { project, ordered, reentering } = await p1PagedRisks(api, workspace.id);
    const { risk } = await p1OpenRisk(page, workspace.slug, project.id, "overdue", 155);
    await p1ReenterOnContinuation(page, workspace.id, project.id, reentering.id);
    await p1NextRisk(page, project.id);
    const third = await p1NextRisk(page, project.id);
    expect(third.next_cursor).not.toBeNull();
    await page.unroute(`**/api/projects/${project.id}/health/issues?*`);
    let emptied = false;
    await page.route(`**/api/projects/${project.id}/health/issues?*`, async (route) => {
      if (!emptied && new URL(route.request().url()).searchParams.has("cursor")) {
        emptied = true;
        await p1DB("UPDATE issue SET status='done',revision=revision+1 WHERE workspace_id=$1 AND id=ANY($2::uuid[])", [workspace.id, ordered.slice(151).map((issue) => issue.id)]);
      }
      await route.continue();
    });
    const fourth = await p1NextRisk(page, project.id);
    expect(fourth.total).toBe(151); expect(fourth.items).toHaveLength(0); expect(fourth.next_cursor).toBeNull();
    await expect(risk.getByRole("heading", { name: `${p1RiskLabels.overdue} · 151`, exact: true })).toBeVisible();
    await expect(risk.getByRole("link")).toHaveCount(0);
    await expect(risk.getByText("No matches after this position. Earlier matches may still exist; refresh from the start to see them.", { exact: true })).toBeVisible();
    await expect(risk.getByText("No matching issues", { exact: true })).toHaveCount(0);
    await expect(risk.getByRole("status").filter({ hasText: /changed/i })).toBeVisible();
    await expect(risk.getByRole("button", { name: "Next page", exact: true })).toHaveCount(0);
    await expect(risk.getByRole("button", { name: "Refresh from start", exact: true })).toBeEnabled();
    await p1Capture(page, info, "risk-empty-suffix-positive-total");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await page.unrouteAll({ behavior: "ignoreErrors" }); await api.deleteFeatureWorkspace(workspace.id); }
});
