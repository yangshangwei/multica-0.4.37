import { expect, type Page, type TestInfo } from "@playwright/test";
import type { TestApiClient } from "../fixtures";
import { p1Capture, p1NoOverflow } from "./project-p1";

type Period = { id: string; name: string; status: string; revision: number };
type Detail = { iteration: Period; statistics: { original: number; completed: number }; snapshot: unknown };

export function iterationDateRange(timezone: string) {
  const parts = new Intl.DateTimeFormat("en-CA", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date());
  const part = (name: Intl.DateTimeFormatPartTypes) => parts.find(item => item.type === name)!.value;
  const startDate = `${part("year")}-${part("month")}-${part("day")}`;
  const endDate = new Date(Date.parse(`${startDate}T00:00:00Z`) + 13 * 86400000).toISOString().slice(0, 10);
  return { startDate, endDate };
}

// Shared browser actions run against both the real Next page and Electron renderer.
export async function iterationClosureFlow(page: Page, api: TestApiClient, workspace: { id: string; slug: string }, info: TestInfo, { recover = false, handoff = false }: { recover?: boolean; handoff?: boolean } = {}) {
  const base = `/api/workspaces/${workspace.id}`;
  const capability = await api.requestJSON<{ supported: boolean }>(`${base}/iteration-capabilities`);
  expect(capability.supported, "I1 E2E requires its isolated test API's feature fixture").toBe(true);
  await page.getByRole("link", { name: "Settings", exact: true }).first().click();
  await page.getByRole("tab", { name: "Iterations", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Iteration settings", exact: true })).toBeVisible();
  const toggle = page.getByRole("switch", { name: "Enable iterations", exact: true });
  await expect(toggle).toBeEnabled();
  await toggle.click();
  await expect(toggle).toBeChecked();
  await page.getByRole("link", { name: "Manage iterations", exact: true }).click();
  await expect(page.getByRole("button", { name: "Create iteration", exact: true })).toBeVisible();

  const firstName = ("I1 customer delivery — " + "long readable planning name ".repeat(6)).trim();
  const nextName = "I1 next delivery";
  const { effective_timezone: timezone } = await api.requestJSON<{ effective_timezone: string }>(`${base}/iteration-settings`);
  const { startDate: today, endDate: end } = iterationDateRange(timezone);
  async function create(name: string) {
    await page.getByRole("button", { name: "Create iteration", exact: true }).click();
    const form = page.locator("form").filter({ has: page.getByRole("button", { name: "Create iteration", exact: true }) });
    await form.getByLabel("Name", { exact: true }).fill(name);
    await form.getByLabel("Start date", { exact: true }).fill(today);
    await form.getByLabel("End date", { exact: true }).fill(end);
    await form.getByRole("button", { name: "Create iteration", exact: true }).click();
    await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
    const list = await api.requestJSON<{ items: Period[] }>(`${base}/iterations`);
    const period = list.items.find(item => item.name === name);
    expect(period).toBeDefined();
    return period!;
  }
  const first = await create(firstName);
  await page.getByRole("link", { name: "Back to iterations", exact: true }).click();
  const next = await create(nextName);
  if (handoff) await api.createIssue("Existing next-period commitment", { status: "todo", current_iteration_id: next.id, expected_iteration_revision: next.revision });
  await page.getByRole("link", { name: "Back to iterations", exact: true }).click();
  await page.getByRole("link", { name: firstName, exact: true }).click();
  const finished = await api.createIssue("I1 completed delivery task", { status: "todo" });
  const remaining = await api.createIssue("I1 remaining delivery task", { status: "todo" });
  await page.getByRole("button", { name: "Add existing tasks", exact: true }).click();
  const assignment = page.getByRole("dialog");
  // Unplanned open work is offered before typing; planned work is not.
  const unplanned = assignment.getByRole("region", { name: "Open tasks without an iteration", exact: true });
  await expect(unplanned.getByRole("checkbox", { name: /I1 completed delivery task/ })).toBeVisible();
  await expect(unplanned.getByRole("checkbox", { name: /I1 remaining delivery task/ })).toBeVisible();
  if (handoff) await expect(unplanned.getByText("Existing next-period commitment", { exact: false })).toHaveCount(0);
  // A pasted ID list resolves exactly; unassigned work joining a plan needs no reason.
  await assignment.getByRole("textbox", { name: "Search tasks", exact: true }).fill(`${finished.id}, ${remaining.id}`);
  await assignment.getByRole("button", { name: "Select all 2 available tasks", exact: true }).click();
  await expect(assignment.getByLabel("Reason", { exact: true })).toHaveCount(0);
  await assignment.getByRole("button", { name: "Add 2 tasks", exact: true }).click();
  await expect(assignment).toBeHidden();
  await expect.poll(async () => (await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).statistics.original).toBe(0);

  await page.getByRole("button", { name: "Start iteration", exact: true }).focus();
  await page.keyboard.press("Enter");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Preview changes", exact: true }).click();
  await dialog.getByRole("button", { name: "Start iteration", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect.poll(async () => (await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).statistics.original).toBe(2);

  // Simulate the independent task writer; both clients must observe its real event.
  await api.requestJSON(`/api/issues/${finished.id}`, { method: "PUT", body: { status: "done" } });
  await expect.poll(async () => (await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).statistics.completed).toBe(1);
  await expect(page.locator("dl > div").filter({ has: page.getByText("Completed", { exact: true }) }).locator("dd")).toHaveText("1");
  if (handoff) {
    await page.getByRole("button", { name: "More actions", exact: true }).click();
    await page.getByRole("menuitem", { name: "End and start next iteration", exact: true }).click();
  } else await page.getByRole("button", { name: "End iteration", exact: true }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByLabel("Reason", { exact: true }).fill("Close accepted delivery; carry remaining work");
  await dialog.getByRole("combobox", { name: "Move remaining work to", exact: true }).click();
  await page.getByRole("option", { name: nextName, exact: true }).click();
  await dialog.getByRole("button", { name: "Preview changes", exact: true }).click();
  await expect(dialog.getByText("I1 remaining delivery task", { exact: false }).first()).toBeVisible();
  const previewTime = dialog.locator("time");
  await expect(previewTime).toHaveAttribute("datetime", /Z$/);
  await expect(previewTime).not.toHaveText(/T\d{2}:\d{2}/);
  await expect(previewTime.locator("..")).toContainText(timezone);
  await previewTime.scrollIntoViewIfNeeded();
  await p1Capture(page, info, "i1-end-confirmation-preview");
  let lostRequestId: string | undefined;
  if (recover) {
    await api.requestJSON(`/api/issues/${remaining.id}`, { method: "PUT", body: { title: "Changed after end preview" } });
    const rejected = page.waitForResponse(response => response.url().endsWith("/iteration-operations") && response.status() === 409);
    await dialog.getByRole("button", { name: "End iteration", exact: true }).click();
    await rejected;
    await expect(dialog.getByLabel("Reason", { exact: true })).toHaveValue("Close accepted delivery; carry remaining work");
    await expect(dialog.getByRole("combobox", { name: "Move remaining work to", exact: true }).locator('[data-slot="select-value"]')).toHaveText(nextName);
    await dialog.getByRole("button", { name: "Preview changes", exact: true }).click();
    await expect(dialog.getByText("Changed after end preview", { exact: false }).first()).toBeVisible();
    await page.route("**/iteration-operations", async route => {
      if (lostRequestId) { await route.continue(); return; }
      const request = route.request().postDataJSON();
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      lostRequestId = request.request_id;
      await route.abort("connectionreset");
    });
  }
  await dialog.getByRole("button", { name: handoff ? "End and start next iteration" : "End iteration", exact: true }).focus();
  await page.keyboard.press("Enter");
  if (recover) {
    await expect(dialog.getByRole("button", { name: "Check original request", exact: true })).toBeVisible();
    await dialog.getByRole("button", { name: "Check original request", exact: true }).click();
    expect(lostRequestId).toBeTruthy();
    expect(await api.requestJSON(`${base}/iteration-operations/${lostRequestId}`)).toMatchObject({ request_id: lostRequestId, operation: "end" });
    await page.unroute("**/iteration-operations");
  }
  await expect(dialog).toBeHidden();
  const frozenHistory = page.locator("header").getByText("This history is frozen at closure.", { exact: false });
  await expect(frozenHistory).toBeVisible();
  await page.getByRole("tab", { name: "Progress", exact: true }).click();
  const progress = page.getByRole("tabpanel", { name: "Progress", exact: true });
  const chartData = progress.getByRole("table", { name: "Chart data", exact: true });
  await expect(chartData).toBeHidden();
  await progress.locator("summary").filter({ hasText: "View chart data" }).focus();
  await page.keyboard.press("Enter");
  await expect(chartData).toBeVisible();
  const closed = await api.requestJSON<Detail>(`${base}/iterations/${first.id}`);
  expect(closed.iteration.status).toBe("completed");
  expect(closed.statistics).toMatchObject({ original: 2, completed: 1 });
  expect(await api.requestJSON(`/api/issues/${remaining.id}`)).toMatchObject({ current_iteration_id: next.id, iteration_rollover_count: 1 });
  expect(await api.requestJSON(`/api/issues/${finished.id}`)).toMatchObject({ current_iteration_id: null, iteration_rollover_count: 0 });
  if (handoff) {
    const started = await api.requestJSON<Detail>(`${base}/iterations/${next.id}`);
    expect(started.iteration.status).toBe("active");
    expect(started.statistics.original).toBe(2);
  }
  await api.requestJSON(`/api/issues/${finished.id}`, { method: "PUT", body: { status: "todo", title: "Changed after closure" } });
  await page.reload();
  await expect(frozenHistory).toBeVisible();
  expect((await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).snapshot).toEqual(closed.snapshot);
  expect(await api.countIssueDispatches(finished.id)).toBe(0);
  expect(await api.countIssueDispatches(remaining.id)).toBe(0);
  await p1NoOverflow(page);
  await page.getByRole("tab", { name: "Progress", exact: true }).click();
  await p1Capture(page, info, "i1-frozen-history");
  await page.getByRole("tab", { name: "Scope changes", exact: true }).click();
  const events = page.getByRole("tabpanel", { name: "Scope changes", exact: true });
  await events.getByRole("button", { name: "All activity", exact: true }).click();
  const startOperation = events.locator("li").filter({ hasText: "Iteration started" }).first();
  await startOperation.locator("summary").filter({ hasText: /^View \d+ records?$/ }).focus();
  await page.keyboard.press("Enter");
  const frozenTaskEvent = startOperation.getByText(/I1 completed delivery task$/).first();
  await expect(frozenTaskEvent).toBeVisible();
  await expect(events).not.toContainText("Changed after closure");
  await frozenTaskEvent.scrollIntoViewIfNeeded();
  await p1Capture(page, info, "i1-frozen-task-events");
  if (handoff) {
    await page.getByRole("link", { name: "Settings", exact: true }).first().click();
    await page.getByRole("tab", { name: "Iterations", exact: true }).click();
    await page.getByRole("switch", { name: "Enable iterations", exact: true }).click();
    const disable = page.getByRole("dialog");
    await disable.getByLabel("Reason", { exact: true }).fill("Suspend manual planning");
    await disable.getByRole("button", { name: "Preview changes", exact: true }).click();
    await disable.getByRole("button", { name: "Disable all iterations", exact: true }).click();
    await expect(disable).toBeHidden();
    expect(await api.requestJSON(`${base}/iteration-settings`)).toMatchObject({ enabled: false });
    expect((await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).snapshot).toEqual(closed.snapshot);
    expect((await api.requestJSON<Detail>(`${base}/iterations/${next.id}`)).iteration.status).toBe("completed");
  }
}

// This deliberately stays on one mounted settings page: reloading clears the old enable mutation.
export async function iterationSettingsFeedbackFlow(page: Page, api: TestApiClient, workspace: { id: string; slug: string }, info: TestInfo) {
  const base = `/api/workspaces/${workspace.id}`;
  await page.getByRole("link", { name: "Settings", exact: true }).first().click();
  await page.getByRole("tab", { name: "Iterations", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Iteration settings", exact: true })).toBeVisible();
  const settingsURL = page.url();
  const toggle = page.getByRole("switch", { name: "Enable iterations", exact: true });
  const enabledStatus = page.getByRole("status").filter({ hasText: /^Iterations are enabled\.$/ });
  const disabledStatus = page.getByRole("status").filter({ hasText: /^Iterations are disabled\. Saved history remains available\.$/ });
  await expect(toggle).not.toBeChecked();
  await expect(disabledStatus).toBeVisible();
  await expect(toggle).toBeEnabled();
  await toggle.focus();
  await page.keyboard.press("Space");
  await expect(toggle).toBeChecked();
  await expect(toggle).toBeEnabled();
  await expect(enabledStatus).toBeVisible();
  expect(await api.requestJSON(`${base}/iteration-settings`)).toMatchObject({ enabled: true });
  expect((await api.requestJSON<{ items: unknown[] }>(`${base}/iterations`)).items).toEqual([]);

  await toggle.focus();
  await page.keyboard.press("Space");
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(page.getByRole("switch", { name: "Enable iterations", exact: true, includeHidden: true })).toBeChecked();
  await dialog.getByLabel("Reason", { exact: true }).fill("Pause planning after the same-page enable");
  await dialog.getByRole("button", { name: "Preview changes", exact: true }).click();
  await dialog.getByRole("button", { name: "Disable all iterations", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(toggle).not.toBeChecked();
  await expect(toggle).toBeEnabled();
  await expect(disabledStatus).toBeVisible();
  await expect(enabledStatus).toHaveCount(0);
  await expect(page.getByText("Iterations enabled.", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "View history", exact: true })).toBeVisible();
  expect(await api.requestJSON(`${base}/iteration-settings`)).toMatchObject({ enabled: false });
  await expect(page).toHaveURL(settingsURL);
  await p1Capture(page, info, "iteration-settings-normal-disabled");

  await toggle.click();
  await expect(toggle).toBeChecked();
  await expect(toggle).toBeEnabled();
  await expect(enabledStatus).toBeVisible();
  await expect(disabledStatus).toHaveCount(0);
  expect(await api.requestJSON(`${base}/iteration-settings`)).toMatchObject({ enabled: true });
  await expect(page).toHaveURL(settingsURL);
}
