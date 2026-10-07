import { expect, type Page, type TestInfo } from "@playwright/test";
import type { TestApiClient } from "../fixtures";
import { p1Capture, p1NoOverflow } from "./project-p1";

type Period = { id: string; name: string; status: string; revision: number };
type Detail = { iteration: Period; statistics: { original: number; completed: number }; snapshot: unknown };

// Shared browser actions run against both the real Next page and Electron renderer.
export async function iterationClosureFlow(page: Page, api: TestApiClient, workspace: { id: string; slug: string }, info: TestInfo, { recover = false, handoff = false }: { recover?: boolean; handoff?: boolean } = {}) {
  const base = `/api/workspaces/${workspace.id}`;
  const capability = await api.requestJSON<{ supported: boolean }>(`${base}/iteration-capabilities`);
  expect(capability.supported, "I1 E2E requires its isolated test API's feature fixture").toBe(true);
  await page.getByRole("link", { name: "Iterations", exact: true }).first().click();
  await page.locator("summary").filter({ hasText: "Iteration settings" }).click();
  await expect(page.getByText("Manual planning: iterations never start or end automatically. Planning actions never start or stop task executions.", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Enable iterations", exact: true }).click();
  await expect(page.locator("summary").filter({ hasText: "Create iteration" })).toBeVisible();

  const firstName = ("I1 customer delivery — " + "long readable planning name ".repeat(6)).trim();
  const nextName = "I1 next delivery";
  const today = new Date().toISOString().slice(0, 10);
  const end = new Date(Date.now() + 13 * 86400000).toISOString().slice(0, 10);
  async function create(name: string) {
    await page.locator("summary").filter({ hasText: "Create iteration" }).click();
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
  await page.locator("summary").filter({ hasText: "Assign to iteration" }).click();
  const assignment = page.locator("details").filter({ has: page.locator("summary").filter({ hasText: "Assign to iteration" }) });
  await assignment.getByLabel("Task ID", { exact: true }).fill(`${finished.id}, ${remaining.id}`);
  await assignment.getByLabel("Reason", { exact: true }).fill("Customer delivery commitment");
  await assignment.getByRole("button", { name: "Preview changes", exact: true }).click();
  await assignment.getByRole("button", { name: "Confirm changes", exact: true }).click();
  await expect.poll(async () => (await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).statistics.original).toBe(0);

  await page.getByRole("button", { name: "Start iteration", exact: true }).focus();
  await page.keyboard.press("Enter");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Preview changes", exact: true }).click();
  await dialog.getByRole("button", { name: "Confirm changes", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect.poll(async () => (await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).statistics.original).toBe(2);

  // Simulate the independent task writer; both clients must observe its real event.
  await api.requestJSON(`/api/issues/${finished.id}`, { method: "PUT", body: { status: "done" } });
  await expect.poll(async () => (await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).statistics.completed).toBe(1);
  await expect(page.locator("dl > div").filter({ has: page.getByText("Completed", { exact: true }) }).locator("dd")).toHaveText("1");
  await page.getByRole("button", { name: handoff ? "End and start next iteration" : "End iteration", exact: true }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByLabel("Reason", { exact: true }).fill("Close accepted delivery; carry remaining work");
  await dialog.getByRole("combobox", { name: "Move remaining work to", exact: true }).selectOption(next.id);
  await dialog.getByRole("button", { name: "Preview changes", exact: true }).click();
  await expect(dialog.getByText("I1 remaining delivery task", { exact: false }).first()).toBeVisible();
  const previewTime = dialog.locator("time");
  await expect(previewTime).toHaveAttribute("datetime", /Z$/);
  await expect(previewTime).not.toHaveText(/T\d{2}:\d{2}/);
  await expect(previewTime.locator("..")).toContainText("UTC");
  await previewTime.scrollIntoViewIfNeeded();
  await p1Capture(page, info, "i1-end-confirmation-preview");
  let lostRequestId: string | undefined;
  if (recover) {
    await api.requestJSON(`/api/issues/${remaining.id}`, { method: "PUT", body: { title: "Changed after end preview" } });
    const rejected = page.waitForResponse(response => response.url().endsWith("/iteration-operations") && response.status() === 409);
    await dialog.getByRole("button", { name: "Confirm changes", exact: true }).click();
    await rejected;
    await expect(dialog.getByLabel("Reason", { exact: true })).toHaveValue("Close accepted delivery; carry remaining work");
    await expect(dialog.getByRole("combobox", { name: "Move remaining work to", exact: true })).toHaveValue(next.id);
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
  await dialog.getByRole("button", { name: "Confirm changes", exact: true }).focus();
  await page.keyboard.press("Enter");
  if (recover) {
    await expect(dialog.getByRole("button", { name: "Check original request", exact: true })).toBeVisible();
    await dialog.getByRole("button", { name: "Check original request", exact: true }).click();
    expect(lostRequestId).toBeTruthy();
    expect(await api.requestJSON(`${base}/iteration-operations/${lostRequestId}`)).toMatchObject({ request_id: lostRequestId, operation: "end" });
    await page.unroute("**/iteration-operations");
  }
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("heading", { name: "Frozen history", exact: true })).toBeVisible();
  await expect(page.getByRole("table", { name: "Chart data", exact: true })).toBeVisible();
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
  await expect(page.getByRole("heading", { name: "Frozen history", exact: true })).toBeVisible();
  expect((await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).snapshot).toEqual(closed.snapshot);
  expect(await api.countIssueDispatches(finished.id)).toBe(0);
  expect(await api.countIssueDispatches(remaining.id)).toBe(0);
  await p1NoOverflow(page);
  await p1Capture(page, info, "i1-frozen-history");
  const events = page.locator("details").filter({ has: page.locator("summary").filter({ hasText: /^Events$/ }) });
  await events.locator("summary").click();
  const frozenTaskEvent = events.getByText(/Task: .*I1 completed delivery task/).first();
  await expect(frozenTaskEvent).toBeVisible();
  await expect(events).not.toContainText("Changed after closure");
  await frozenTaskEvent.scrollIntoViewIfNeeded();
  await p1Capture(page, info, "i1-frozen-task-events");
  if (handoff) {
    await page.getByRole("link", { name: "Back to iterations", exact: true }).click();
    await page.locator("summary").filter({ hasText: "Iteration settings" }).click();
    await page.getByRole("button", { name: "Disable all iterations", exact: true }).click();
    const disable = page.getByRole("dialog");
    await disable.getByLabel("Reason", { exact: true }).fill("Suspend manual planning");
    await disable.getByRole("button", { name: "Preview changes", exact: true }).click();
    await disable.getByRole("button", { name: "Confirm changes", exact: true }).click();
    await expect(disable).toBeHidden();
    expect(await api.requestJSON(`${base}/iteration-settings`)).toMatchObject({ enabled: false });
    expect((await api.requestJSON<Detail>(`${base}/iterations/${first.id}`)).snapshot).toEqual(closed.snapshot);
    expect((await api.requestJSON<Detail>(`${base}/iterations/${next.id}`)).iteration.status).toBe("completed");
  }
}
