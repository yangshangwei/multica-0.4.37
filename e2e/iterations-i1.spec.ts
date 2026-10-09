import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { p1Session, p1Failure, p1Capture, p1NoOverflow } from "./fixtures/project-p1";
import { iterationClosureFlow, iterationDateRange } from "./fixtures/iterations-i1";

test("I1 Web manual planning, atomic carryover and immutable history", async ({ page }, info) => {
  test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires isolated I1 test API fixture");
  test.setTimeout(180_000);
  const { api, workspace } = await p1Session(page);
  try {
    await page.goto(`/${workspace.slug}/issues`);
    await iterationClosureFlow(page, api, workspace, info);
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.addInitScript(() => localStorage.setItem("multica-locale", "zh-Hans"));
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    await expect(page.locator("header").getByText("此历史已在关闭时冻结。", { exact: false })).toBeVisible();
    await page.getByRole("tab", { name: "进展", exact: true }).click();
    await page.getByRole("tabpanel", { name: "进展", exact: true }).locator("summary").filter({ hasText: "查看图表数据" }).focus();
    await page.keyboard.press("Enter");
    await page.getByRole("table", { name: "图表数据", exact: true }).scrollIntoViewIfNeeded();
    await p1NoOverflow(page);
    await p1Capture(page, info, "i1-web-chinese-390");
  } catch (error) {
    await info.attach("original-error", { body: String(error), contentType: "text/plain" });
    await p1Failure(page, info).catch(() => {});
    throw error;
  }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("I1 Web starts the chosen next period atomically and disables the complete workspace", async ({ page }, info) => {
  test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires isolated I1 test API fixture");
  test.setTimeout(180_000);
  const { api, workspace } = await p1Session(page);
  try {
    await page.goto(`/${workspace.slug}/issues`);
    await iterationClosureFlow(page, api, workspace, info, { handoff: true });
  } catch (error) {
    await info.attach("original-error", { body: String(error), contentType: "text/plain" });
    await p1Failure(page, info).catch(() => {});
    throw error;
  } finally { await api.deleteFeatureWorkspace(workspace.id); }
});

// Choice/error/statistics matrices live in the core and views suites; this proves their browser wiring.
test("I1 Web retries detail reads, creates work in the searched iteration and opens chart data by keyboard", async ({ page }, info) => {
  test.skip(process.env.MULTICA_RUN_I1_E2E !== "1", "Requires isolated I1 test API fixture");
  const { api, workspace } = await p1Session(page);
  const base = `/api/workspaces/${workspace.id}`;
  try {
    const settings = await api.requestJSON<{ revision: number; effective_timezone: string }>(`${base}/iteration-settings`);
    await api.requestJSON(`${base}/iteration-settings/enable`, {
      method: "POST",
      body: { request_id: randomUUID(), expected_revision: settings.revision, confirmed_timezone: settings.effective_timezone },
    });
    const { startDate, endDate } = iterationDateRange(settings.effective_timezone);
    async function createPeriod(name: string) {
      const created = await api.requestJSON<{ iteration_ids: string[] }>(`${base}/iterations`, {
        method: "POST",
        body: { request_id: randomUUID(), name, start_date: startDate, end_date: endDate, confirmed_timezone: settings.effective_timezone },
      });
      const detail = await api.requestJSON<{ iteration: { id: string; name: string; revision: number } }>(`${base}/iterations/${created.iteration_ids[0]}`);
      return detail.iteration;
    }
    const source = await createPeriod("I1 closeout source");
    const target = await createPeriod("I1 closeout destination");

    const detailPath = `${base}/iterations/${source.id}`;
    // Fail only the first GET. Retry must use the real service, without a page reload.
    await page.route(url => url.pathname === detailPath, route => route.fulfill({
      status: 503, contentType: "application/json", body: JSON.stringify({ error: "Temporary iteration read failure" }),
    }), { times: 1 });
    const firstRead = page.waitForResponse(response => new URL(response.url()).pathname === detailPath && response.request().method() === "GET");
    await page.goto(`/${workspace.slug}/iterations/${source.id}`);
    expect((await firstRead).status()).toBe(503);
    const readError = page.locator("main").getByRole("alert").filter({ hasText: /^Could not load iteration data\. Try again\.$/ });
    await expect(readError).toBeVisible();
    await expect(page.getByRole("heading", { name: source.name, exact: true })).toHaveCount(0);
    const retryRead = page.waitForResponse(response => new URL(response.url()).pathname === detailPath && response.request().method() === "GET");
    await page.getByRole("button", { name: "Retry", exact: true }).click();
    expect((await retryRead).status()).toBe(200);
    await expect(page.getByRole("heading", { name: source.name, exact: true })).toBeVisible();
    await expect(readError).toHaveCount(0);
    await expect(page).toHaveURL(`/${workspace.slug}/iterations/${source.id}`);
    await expect(page.locator("header").filter({ hasText: settings.effective_timezone })).toBeVisible();

    await page.getByRole("button", { name: "New task", exact: true }).click();
    const createDialog = page.getByRole("dialog", { name: "New Issue", exact: true });
    await expect(createDialog).toBeVisible();
    await createDialog.getByRole("button", { name: `Iterations: ${source.name}`, exact: true }).click();
    const search = page.getByPlaceholder("Search iterations", { exact: true });
    await search.fill("closeout destination");
    await expect(page.getByRole("button", { name: target.name, exact: true })).toBeVisible();
    await search.press("Enter");
    const targetChoice = createDialog.getByRole("button", { name: `Iterations: ${target.name}`, exact: true });
    await expect(targetChoice).toBeVisible();

    await targetChoice.click();
    await page.getByRole("button", { name: "No iteration", exact: true }).click();
    const emptyChoice = createDialog.getByRole("button", { name: "Iterations: No iteration", exact: true });
    await expect(emptyChoice).toBeVisible();
    await emptyChoice.click();
    await expect(search).toHaveValue("");
    await search.fill("closeout destination");
    await expect(page.getByRole("button", { name: target.name, exact: true })).toBeVisible();
    await search.press("Enter");
    await expect(targetChoice).toBeVisible();

    const title = "Manual work uses the searched iteration";
    await createDialog.getByRole("textbox", { name: "Issue title", exact: true }).fill(title);
    const createdResponse = page.waitForResponse(response => new URL(response.url()).pathname === "/api/issues" && response.request().method() === "POST");
    await createDialog.getByRole("button", { name: "Create Issue", exact: true }).click();
    const response = await createdResponse;
    expect(response.status()).toBe(201);
    expect(response.request().postDataJSON()).toMatchObject({ current_iteration_id: target.id, expected_iteration_revision: target.revision });
    const created: { id: string } = await response.json();
    await expect(createDialog).toBeHidden();
    expect(await api.requestJSON(`/api/issues/${created.id}`)).toMatchObject({ title, current_iteration_id: target.id, iteration_rollover_count: 0 });
    const targetScope = await api.requestJSON<{ items: { issue_id: string }[] }>(`${base}/iterations/${target.id}/issues`);
    expect(targetScope.items.map(item => item.issue_id)).toEqual([created.id]);
    expect((await api.requestJSON<{ items: unknown[] }>(`${base}/iterations/${source.id}/issues`)).items).toEqual([]);
    expect(await api.countIssueDispatches(created.id)).toBe(0);

    await page.getByRole("link", { name: "Back to iterations", exact: true }).click();
    await page.getByRole("link", { name: target.name, exact: true }).click();
    await page.getByRole("button", { name: "Start iteration", exact: true }).click();
    const startDialog = page.getByRole("dialog");
    await startDialog.getByRole("button", { name: "Preview changes", exact: true }).click();
    await startDialog.getByRole("button", { name: "Start iteration", exact: true }).click();
    await expect(startDialog).toBeHidden();
    await expect(page.getByRole("button", { name: "End iteration", exact: true })).toBeVisible();
    const active = await api.requestJSON<{
      iteration: { status: string };
      statistics: { original: number; completed: number; chart: { date: string; effective: number; completed: number; original: number }[] };
    }>(`${base}/iterations/${target.id}`);
    expect(active).toMatchObject({ iteration: { status: "active" }, statistics: { original: 1, completed: 0 } });
    const point = active.statistics.chart.at(-1);
    expect(point).toBeDefined();

    await page.getByRole("link", { name: "Back to iterations", exact: true }).click();
    const current = page.locator("article").filter({ has: page.getByRole("link", { name: target.name, exact: true }) });
    const summary = current.locator("summary").filter({ hasText: "View chart data" });
    const table = current.getByRole("table", { name: "Chart data", exact: true });
    await expect(table).toBeHidden();
    await summary.focus();
    await expect(summary).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(table).toBeVisible();
    await expect(table.getByRole("columnheader")).toHaveText(["Date", "Effective scope", "Completed", "Original commitment"]);
    const row = table.getByRole("row").filter({ has: page.getByRole("rowheader", { name: point!.date, exact: true }) });
    await expect(row.getByRole("cell")).toHaveText([String(point!.effective), String(point!.completed), String(point!.original)]);
    await p1Capture(page, info, "iteration-closeout-chart-data");
    await summary.focus();
    await page.keyboard.press("Space");
    await expect(table).toBeHidden();
    await expect(summary).toBeFocused();
  } catch (error) {
    await info.attach("original-error", { body: String(error), contentType: "text/plain" });
    await p1Failure(page, info).catch(() => {});
    throw error;
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
