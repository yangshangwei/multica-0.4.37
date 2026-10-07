import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { p1Session, p1Failure, p1Capture, p1NoOverflow, p1Raw } from "./fixtures/project-p1";

test("I1 filters and groups the complete scope and preserves historical priority, labels and participation", async ({ page }, info) => {
  test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires isolated I1 test API fixture");
  test.setTimeout(180_000);
  const { api, workspace } = await p1Session(page);
  const base = `/api/workspaces/${workspace.id}`;
  try {
    await api.requestJSON(`${base}/iteration-settings/enable`, { method: "POST", body: { request_id: randomUUID(), expected_revision: 1, confirmed_timezone: "UTC" } });
    const created = await api.requestJSON<{ iteration_ids: string[] }>(`${base}/iterations`, { method: "POST", body: { request_id: randomUUID(), name: "Metadata cycle", start_date: new Date().toISOString().slice(0, 10), end_date: new Date(Date.now() + 13 * 86400000).toISOString().slice(0, 10), confirmed_timezone: "UTC" } });
    const id = created.iteration_ids[0]!;
    const issues = await api.seedTableIssues(Array.from({ length: 61 }, (_, n) => ({ title: `Metadata item ${String(n).padStart(2, "0")}`, status: "todo" as const, priority: n % 2 === 0 ? "high" as const : "low" as const })));
    const special = issues[0]!;
    const label = await api.requestJSON<{ id: string }>("/api/labels", { method: "POST", body: { name: "Frozen review", color: "#123456", resource_type: "issue" } });
    await api.requestJSON(`/api/issues/${special.id}/labels`, { method: "POST", body: { label_id: label.id } });
    const specialCurrent = await api.requestJSON<{ revision: number }>(`/api/issues/${special.id}`);
    async function apply(draft: Record<string, unknown>) {
      const preview = await api.requestJSON<{ draft: unknown; preview_hash: string; invalid_items: unknown[] }>(`${base}/iteration-previews`, { method: "POST", body: draft });
      expect(preview.invalid_items).toEqual([]);
      await api.requestJSON(`${base}/iteration-operations`, { method: "POST", body: { request_id: randomUUID(), draft: preview.draft, preview_hash: preview.preview_hash } });
    }
    const common = { expected_settings_revision: 2, reason: "Verify complete historical scope", start: null };
    await apply({ ...common, operation: "move", iteration_id: null, expected_iteration_revision: null, expected_scope_revision: null, moves: issues.map(issue => ({ issue_id: issue.id, expected_issue_revision: issue.id === special.id ? specialCurrent.revision : 1, expected_source_id: null, target_id: id, allow_completed: false })) });
    const getPeriod = () => api.requestJSON<{ iteration: { revision: number; scope_revision: number }; snapshot: unknown }>(`${base}/iterations/${id}`);
    let detail = await getPeriod();
    await apply({ ...common, operation: "start", iteration_id: id, expected_iteration_revision: detail.iteration.revision, expected_scope_revision: detail.iteration.scope_revision, moves: [], start: { target_id: id, mode: "scheduled", terminal_choices: [] } });

    await page.goto(`/${workspace.slug}/iterations/${id}`);
    await page.locator("summary").filter({ hasText: "Filter and group tasks" }).click();
    await page.getByRole("combobox", { name: "Group tasks by", exact: true }).selectOption("priority");
    await expect(page.getByRole("heading", { name: "High (31)", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Low (30)", exact: true })).toBeVisible();
    await page.getByRole("combobox", { name: "Priority", exact: true }).selectOption("high");
    await page.getByRole("combobox", { name: "Label", exact: true }).selectOption(label.id);
    await expect(page.getByRole("heading", { name: "High (1)", exact: true })).toBeVisible();
    await expect(page.locator("dl > div").filter({ has: page.getByText("Original commitment", { exact: true }) }).locator("dd")).toHaveText("61");

    const scope = await api.requestJSON<{ items: { issue_id: string }[] }>(`${base}/iterations/${id}/issues?limit=100`);
    const moves = await Promise.all(scope.items.map(async item => {
      const current = await api.requestJSON<{ revision: number }>(`/api/issues/${item.issue_id}`);
      return { issue_id: item.issue_id, expected_issue_revision: current.revision, expected_source_id: id, target_id: null, allow_completed: false };
    }));
    detail = await getPeriod();
    await apply({ ...common, operation: "end", iteration_id: id, expected_iteration_revision: detail.iteration.revision, expected_scope_revision: detail.iteration.scope_revision, moves });
    const frozen = (await getPeriod()).snapshot;
    await api.requestJSON(`/api/issues/${special.id}`, { method: "PUT", body: { title: "Changed live metadata", priority: "low" } });
    await api.requestJSON(`/api/labels/${label.id}`, { method: "PUT", body: { name: "Renamed current label" } });
    expect((await p1Raw(api, workspace.id, `/api/labels/${label.id}`, "DELETE")).status).toBe(204);
    expect((await getPeriod()).snapshot).toEqual(frozen);

    await page.goto(`/${workspace.slug}/issues/${special.id}`);
    await page.locator("summary").filter({ hasText: "Iteration participation" }).click();
    await expect(page.getByRole("link", { name: "Metadata cycle", exact: true })).toBeVisible();
    await page.getByRole("link", { name: "Metadata cycle", exact: true }).click();
    await page.locator("summary").filter({ hasText: "Filter and group tasks" }).click();
    await page.getByRole("combobox", { name: "Priority", exact: true }).selectOption("high");
    await page.getByRole("combobox", { name: "Label", exact: true }).selectOption(label.id);
    await expect(page.getByRole("button", { name: new RegExp(special.title) })).toBeVisible();
    await page.getByRole("button", { name: new RegExp(special.title) }).click();
    await expect(page.getByText("Changed live metadata", { exact: false }).first()).toBeVisible();
    expect(await api.countIssueDispatches(special.id)).toBe(0);
    await api.deleteIssue(special.id);
    await expect(page.getByText("Task deleted or no longer accessible. Historical values are preserved.", { exact: true })).toBeVisible();
    expect((await getPeriod()).snapshot).toEqual(frozen);
    await p1NoOverflow(page);
    await p1Capture(page, info, "i1-frozen-metadata-and-deleted-task");
  } catch (error) {
    await info.attach("original-error", { body: String(error), contentType: "text/plain" });
    await p1Failure(page, info).catch(() => {});
    throw error;
  } finally { await api.deleteFeatureWorkspace(workspace.id); }
});
