import { randomUUID } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

interface Item {
  issue: { id: string; title: string; identifier: string; admission_status: string; revision: number };
  snoozed_until: string | null;
}
interface Queue {
  items: Item[];
  counts: { pending: number; ready: number; snoozed: number };
}

async function fixture(page: Page) {
  const api = new TestApiClient();
  const name = `triage-${randomUUID().slice(0, 12)}`;
  await api.login(`${name}@example.invalid`, "Triage E2E");
  const workspace = await api.ensureWorkspace("Triage E2E", name);
  await api.markUserOnboarded();
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
  const token = api.getToken();
  if (!token) throw new Error("Triage fixture did not receive authentication");
  await page.addInitScript((token) => {
    localStorage.setItem("multica_token", token);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
  return { api, workspace };
}

async function enable(api: TestApiClient) {
  const settings = await api.requestJSON<{ revision: number }>("/api/triage/settings");
  await api.requestJSON("/api/triage/settings", {
    method: "PUT",
    body: { enabled: true, acceptance_status: "todo", require_priority: false, responsibility_mode: "none", responsibility_member_id: null, expected_revision: settings.revision },
  });
}

async function pending(api: TestApiClient, title: string) {
  return api.requestJSON<Item>("/api/triage/items", { method: "POST", body: { request_id: randomUUID(), title } });
}

test("triage settings, manual intake and ordinary acceptance preserve the execution boundary", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await fixture(page);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    await page.goto(`/${workspace.slug}/settings?tab=triage`);
    await expect(page.getByRole("switch", { name: "Enable triage", exact: true })).not.toBeChecked();
    await page.getByRole("switch", { name: "Enable triage", exact: true }).click();
    await page.getByRole("button", { name: "Save settings", exact: true }).click();
    await expect(page.getByText("Settings saved", { exact: true })).toBeVisible();
    await page.goto(`/${workspace.slug}/triage`);
    await expect(page.getByText("Queue is clear", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Create pending task", exact: true }).click();
    const create = page.getByRole("dialog");
    const title = `Customer request ${randomUUID().slice(0, 8)}`;
    await create.getByRole("textbox", { name: "Title", exact: true }).fill(title);
    await create.getByRole("button", { name: "Create pending task", exact: true }).click();
    await expect(create).toHaveCount(0);
    const queue = await api.requestJSON<Queue>("/api/triage/items?view=all");
    const item = queue.items.find((row) => row.issue.title === title);
    expect(item).toBeDefined();
    if (!item) throw new Error("Created pending task missing from queue");
    expect(item.issue.admission_status).toBe("pending");
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
    await page.getByRole("textbox", { name: "Search title or identifier", exact: true }).fill("jk123");
    await page.keyboard.press("1");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await page.getByRole("textbox", { name: "Search title or identifier", exact: true }).fill("");
    await expect(page.getByRole("textbox", { name: "Search title or identifier", exact: true })).toHaveValue("");
    await expect(page).not.toHaveURL(/[?&]q=/);
    await page.screenshot({ path: info.outputPath("triage-pending-desktop.png"), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.screenshot({ path: info.outputPath("triage-pending-compact.png"), fullPage: true });
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.getByRole("button", { name: "Accept", exact: true }).click();
    const accept = page.getByRole("dialog");
    await expect(accept.getByText("These fields are saved together with acceptance. Accept does not start an agent.")).toBeVisible();
    await accept.getByRole("button", { name: "Accept", exact: true }).click();
    await expect(accept).toHaveCount(0);
    const accepted = await api.requestJSON<Item>(`/api/triage/items/${item.issue.id}`);
    expect(accepted.issue.admission_status).toBe("accepted");
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
    await expect.poll(async () => (await api.requestJSON<Queue>("/api/triage/items")).counts.pending).toBe(0);
    await expect(page.getByRole("heading", { name: "Queue is clear", exact: true })).toBeFocused();
    await page.goto(`/${workspace.slug}/triage?view=history`);
    await expect(page.getByRole("button", { name: new RegExp(title) }).first()).toBeVisible();
    await page.setViewportSize({ width: 390, height: 844 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.screenshot({ path: info.outputPath("triage-history-compact.png"), fullPage: true });
    expect(errors).toEqual([]);
  } catch (error) {
    await info.attach("triage-failure-state", { body: `${page.url()}\n${await page.locator("body").innerText()}`, contentType: "text/plain" });
    await page.screenshot({ path: info.outputPath("triage-before-cleanup.png"), fullPage: true });
    throw error;
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("snooze, rejection and reopen retain old review rounds", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await fixture(page);
  try {
    await enable(api);
    const item = await pending(api, "Needs clarification before accepting");
    await page.goto(`/${workspace.slug}/triage?issue=${item.issue.id}`);
    await page.getByRole("button", { name: "Snooze", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: /In one hour/ }).click();
    await expect(dialog.getByText(/Time zone:/)).toBeVisible();
    await dialog.getByRole("button", { name: "Snooze", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect.poll(async () => (await api.requestJSON<Queue>("/api/triage/items")).counts.snoozed).toBe(1);
    await page.goto(`/${workspace.slug}/triage?view=snoozed&issue=${item.issue.id}`);
    await page.getByRole("button", { name: "Reject", exact: true }).click();
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("Outside the team's current scope");
    await dialog.getByRole("button", { name: "Reject", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await page.goto(`/${workspace.slug}/triage?view=history&issue=${item.issue.id}`);
    await page.getByRole("button", { name: "Reopen review", exact: true }).click();
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("New information makes a review worthwhile");
    await dialog.getByRole("button", { name: "Reopen review", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const history = await api.requestJSON<{ events: { action: string; reason: string | null }[] }>(`/api/triage/items/${item.issue.id}/history`);
    expect(history.events).toEqual(expect.arrayContaining([
      expect.objectContaining({ action: "reject", reason: "Outside the team's current scope" }),
      expect.objectContaining({ action: "reopen", reason: "New information makes a review worthwhile" }),
    ]));
    expect((await api.requestJSON<Item>(`/api/triage/items/${item.issue.id}`)).issue.admission_status).toBe("pending");
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
    await page.screenshot({ path: info.outputPath("triage-reopened.png"), fullPage: true });
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("CSV preview, mapping and partial import create only selected pending rows", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await fixture(page);
  try {
    await enable(api);
    await page.goto(`/${workspace.slug}/triage`);
    await page.getByRole("button", { name: "Import CSV", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.locator('input[type="file"]').setInputFiles({ name: "backlog.csv", mimeType: "text/csv", buffer: Buffer.from("title,description,status,external_id\nValid input,\"@agent quoted, description\",in_progress,EXT-1\n,,,EXT-2\nDuplicate ID,Context,todo,EXT-1\n", "utf8") });
    await expect(dialog.getByText("Map columns", { exact: true })).toBeVisible();
    expect((await api.requestJSON<Queue>("/api/triage/items")).counts.pending).toBe(0);
    await dialog.getByRole("button", { name: "Select valid nonduplicate rows", exact: true }).click();
    await dialog.getByRole("button", { name: "Import selected rows", exact: true }).click();
    await expect(dialog.getByText(/1 created/)).toBeVisible();
    const queue = await api.requestJSON<Queue>("/api/triage/items?view=all");
    expect(queue.items.map((item) => item.issue.title)).toEqual(["Valid input"]);
    expect(queue.items[0].issue.admission_status).toBe("pending");
    expect(await api.countIssueDispatches(queue.items[0].issue.id)).toBe(0);
    await page.screenshot({ path: info.outputPath("triage-csv-results.png"), fullPage: true });
    const history = await api.requestJSON<{ entries: { kind: string; filename: string | null }[] }>("/api/triage/history");
    expect(history.entries).toEqual(expect.arrayContaining([expect.objectContaining({ kind: "import", filename: "backlog.csv" })]));
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("real comment listeners notify the creator without changing a snoozed review", async ({ page }) => {
  test.setTimeout(120_000);
  const { api, workspace } = await fixture(page);
  const colleague = new TestApiClient();
  try {
    const identity = `triage-colleague-${randomUUID().slice(0, 12)}`;
    await colleague.login(`${identity}@example.invalid`, "Triage colleague");
    const user = await colleague.requestJSON<{ id: string }>("/api/me");
    // Seed membership, then exercise real HTTP handlers, event listeners and inbox.
    const db = new pg.Client(process.env.DATABASE_URL);
    await db.connect();
    try {
      await db.query("INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'member')", [workspace.id, user.id]);
    } finally {
      await db.end();
    }
    colleague.setWorkspaceId(workspace.id);
    colleague.setWorkspaceSlug(workspace.slug);
    await enable(api);
    const item = await pending(api, "Collect customer details tomorrow");
    const until = new Date(Date.now() + 3_600_000).toISOString();
    await api.requestJSON(`/api/triage/items/${item.issue.id}/actions`, {
      method: "POST",
      body: { request_id: randomUUID(), expected_revision: item.issue.revision, action: "snooze", snoozed_until: until },
    });
    await colleague.requestJSON(`/api/issues/${item.issue.id}/comments`, {
      method: "POST", body: { content: "Customer supplied the missing reproduction steps." },
    });
    type Notice = { id: string; type: string; issue_id: string | null };
    await expect.poll(async () => (await api.requestJSON<Notice[]>("/api/inbox")).filter((notice) => notice.type === "new_comment" && notice.issue_id === item.issue.id).length).toBe(1);
    const notice = (await api.requestJSON<Notice[]>("/api/inbox")).find((row) => row.type === "new_comment" && row.issue_id === item.issue.id);
    if (!notice) throw new Error("Creator notification vanished");
    for (const action of ["read", "archive", "unarchive", "unread"]) {
      await api.requestJSON(`/api/inbox/${notice.id}/${action}`, { method: "POST" });
      const current = await api.requestJSON<Item>(`/api/triage/items/${item.issue.id}`);
      expect(current.issue.admission_status).toBe("pending");
      expect(new Date(current.snoozed_until!).toISOString()).toBe(until);
    }
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/issues/${item.issue.id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${api.getToken()}`, "X-Workspace-ID": workspace.id },
      body: JSON.stringify({ status: "in_progress" }),
    });
    expect(response.status).toBe(409);
    expect(JSON.stringify(await response.json())).toMatch(/triage|review|admi/i);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("explicit execution shows the real executor and resource context before queuing once", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await fixture(page);
  try {
    await enable(api);
    const runtime = await api.seedProjectRuntime();
    const agent = await api.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: "Triage execution fixture", runtime_id: runtime.id, permission_mode: "private" } });
    const project = await api.requestJSON<{ id: string }>("/api/projects", { method: "POST", body: { title: "Customer feedback delivery" } });
    await api.requestJSON(`/api/projects/${project.id}/resources`, { method: "POST", body: { resource_type: "github_repo", resource_ref: { url: "https://example.invalid/triage/demo.git" }, label: "Review repository" } });
    const item = await api.requestJSON<Item>("/api/triage/items", { method: "POST", body: { request_id: randomUUID(), title: "Explicit execution context", candidate_project_id: project.id, candidate_assignee_type: "agent", candidate_assignee_id: agent.id } });
    await page.goto(`/${workspace.slug}/triage?issue=${item.issue.id}`);
    await page.getByRole("button", { name: "Accept and execute", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("Triage execution fixture", { exact: true }).last()).toBeVisible();
    await expect(dialog.getByText("Customer feedback delivery", { exact: true }).last()).toBeVisible();
    await expect(dialog.getByText(/example.invalid\/triage\/demo.git/)).toBeVisible();
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
    await page.screenshot({ path: info.outputPath("triage-execution-confirmation.png"), fullPage: true });
    await dialog.getByRole("button", { name: "Accept and execute", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect.poll(() => api.countIssueDispatches(item.issue.id)).toBe(1);
    const history = await api.requestJSON<{ events: { id: string; execution_status: string }[] }>(`/api/triage/items/${item.issue.id}/history`);
    const action = history.events.find((entry) => entry.execution_status === "queued");
    if (!action) throw new Error("Explicit execution has no durable queued action");
    await api.requestJSON(`/api/triage/actions/${action.id}/retry-execution`, { method: "POST", body: {} });
    expect(await api.countIssueDispatches(item.issue.id)).toBe(1);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});

test("Chinese compact triage keeps validation input and visible review controls", async ({ page }, info) => {
  test.setTimeout(120_000);
  const { api, workspace } = await fixture(page);
  try {
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    await page.addInitScript(() => localStorage.setItem("multica-locale", "zh-Hans"));
    await enable(api);
    const item = await pending(api, "客户反馈：登录后的页面没有刷新");
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/${workspace.slug}/triage?issue=${item.issue.id}`);
    await page.getByRole("button", { name: "拒绝", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "原因", exact: true }).fill("   ");
    await dialog.getByRole("button", { name: "拒绝", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("请填写原因");
    expect((await api.requestJSON<Item>(`/api/triage/items/${item.issue.id}`)).issue.admission_status).toBe("pending");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.screenshot({ path: info.outputPath("triage-chinese-compact-validation.png"), fullPage: true });
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
