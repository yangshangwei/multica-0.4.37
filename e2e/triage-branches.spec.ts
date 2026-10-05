import { randomUUID } from "node:crypto";
import { expect, test as base } from "@playwright/test";
import { TestApiClient } from "./fixtures";

interface Settings {
  enabled: boolean;
  acceptance_status: string;
  require_priority: boolean;
  responsibility_mode: "none" | "notify" | "assign";
  responsibility_member_id: string | null;
  revision: number;
}

interface Item {
  issue: {
    id: string;
    identifier: string;
    title: string;
    admission_status: string;
    revision: number;
    status: string;
    priority: string;
    assignee_id: string | null;
    project_id: string | null;
  };
  reviewer_id: string | null;
  reviewer_valid: boolean;
  round: number;
  snoozed_until: string | null;
}

interface Action {
  id: string;
  action: string;
  round: number;
  reason: string | null;
}

interface ActionResult {
  item: Item;
  action: Action;
}

interface Fixture {
  api: TestApiClient;
  workspace: { id: string; slug: string };
  owner: { id: string; name: string };
}

const test = base.extend<{ triage: Fixture }>({
  triage: async ({ page }, use) => {
    const api = new TestApiClient();
    const identity = `triage-branches-${randomUUID().slice(0, 12)}`;
    await api.login(`${identity}@example.invalid`, "Triage branch reviewer");
    const workspace = await api.ensureWorkspace("Triage branch coverage", identity);
    try {
      await api.markUserOnboarded();
      const owner = await api.requestJSON<Fixture["owner"]>("/api/me", {
        method: "PATCH",
        body: { language: "en" },
      });
      const token = api.getToken();
      if (!token) throw new Error("Triage fixture has no authentication token");
      await page.addInitScript((value) => {
        localStorage.setItem("multica_token", value);
        localStorage.setItem("multica-locale", "en");
        localStorage.setItem("multica:chat:isOpen", "false");
      }, token);
      await use({ api, workspace, owner });
    } finally {
      await api.deleteFeatureWorkspace(workspace.id);
    }
  },
});

async function configure(api: TestApiClient, patch: Partial<Settings> = {}) {
  const current = await api.requestJSON<Settings>("/api/triage/settings");
  return api.requestJSON<Settings>("/api/triage/settings", {
    method: "PUT",
    body: settingsInput({ ...current, enabled: true, ...patch }),
  });
}

function settingsInput(settings: Settings) {
  return {
    enabled: settings.enabled,
    acceptance_status: settings.acceptance_status,
    require_priority: settings.require_priority,
    responsibility_mode: settings.responsibility_mode,
    responsibility_member_id: settings.responsibility_member_id,
    expected_revision: settings.revision,
  };
}

async function pending(api: TestApiClient, title: string) {
  return api.requestJSON<Item>("/api/triage/items", {
    method: "POST",
    body: { request_id: randomUUID(), title },
  });
}

async function detail(api: TestApiClient, item: Item) {
  return api.requestJSON<Item>(`/api/triage/items/${item.issue.id}`);
}

async function history(api: TestApiClient, item: Item) {
  return (await api.requestJSON<{ events: Action[] }>(`/api/triage/items/${item.issue.id}/history`)).events;
}

async function decide(api: TestApiClient, item: Item, action: string, fields: Record<string, unknown> = {}) {
  return api.requestJSON<ActionResult>(`/api/triage/items/${item.issue.id}/actions`, {
    method: "POST",
    body: { request_id: randomUUID(), expected_revision: item.issue.revision, action, ...fields },
  });
}

// Raw responses are needed only for rejected requests; fixture writes use TestApiClient.
async function request(fixture: Fixture, path: string, method: string, body: unknown) {
  const origin = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
  return fetch(`${origin}${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${fixture.api.getToken()}`,
      "X-Workspace-ID": fixture.workspace.id,
    },
    body: JSON.stringify(body),
  });
}

test.describe("Triage browser + real API branch coverage", () => {
  test("disabled triage hides intake and rejects a direct submission without creating work", async ({ page, triage }) => {
    const { api, workspace } = triage;
    await page.goto(`/${workspace.slug}/triage`);
    await expect(page.getByText("Triage is disabled. History remains available.", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Create pending task", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Import CSV", exact: true })).toHaveCount(0);
    const response = await request(triage, "/api/triage/items", "POST", {
      request_id: randomUUID(), title: "Disabled intake must not persist",
    });
    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({ error: expect.stringMatching(/enable|disabled/i) });
    expect(await api.requestJSON("/api/triage/items")).toMatchObject({ counts: { pending: 0 } });
    expect(await api.requestJSON("/api/triage/history")).toMatchObject({ entries: [] });
  });

  test("a snoozed pending item prevents disabling triage in both settings and the API", async ({ page, triage }) => {
    const { api, workspace } = triage;
    const settings = await configure(api);
    const item = await pending(api, "Snoozed work still requires review");
    const snoozed = await decide(api, item, "snooze", { snoozed_until: new Date(Date.now() + 3_600_000).toISOString() });
    await page.goto(`/${workspace.slug}/settings?tab=triage`);
    await expect(page.getByText("1 unresolved tasks remain, including snoozed tasks. Resolve them before disabling triage.", { exact: false })).toBeVisible();
    await expect(page.getByRole("switch", { name: "Enable triage", exact: true })).toBeDisabled();
    const response = await request(triage, "/api/triage/settings", "PUT", settingsInput({ ...settings, enabled: false }));
    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({ error: expect.stringContaining("including snoozed items") });
    expect(await api.requestJSON<Settings>("/api/triage/settings")).toMatchObject({ enabled: true, revision: settings.revision });
    expect(await detail(api, item)).toEqual(snoozed.item);
    expect((await history(api, item)).map((event) => event.action)).toEqual(["snooze"]);
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
  });

  test("required priority preserves the acceptance draft until a valid priority is selected", async ({ page, triage }) => {
    const { api, workspace } = triage;
    await configure(api, { require_priority: true });
    const item = await pending(api, "Acceptance needs a priority");
    await page.goto(`/${workspace.slug}/triage?issue=${item.issue.id}`);
    await page.getByRole("button", { name: "Accept", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("Approved after selecting a priority");
    await dialog.getByRole("button", { name: "Accept", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("Choose a priority before accepting.");
    expect(await detail(api, item)).toEqual(item);
    expect(await history(api, item)).toEqual([]);
    await dialog.locator("#triage-priority-field").getByRole("button").click();
    await page.getByRole("button", { name: "High", exact: true }).click();
    await expect(dialog.getByRole("textbox", { name: "Reason", exact: true })).toHaveValue("Approved after selecting a priority");
    await dialog.getByRole("button", { name: "Accept", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    expect((await detail(api, item)).issue).toMatchObject({ admission_status: "accepted", priority: "high", status: "todo", revision: item.issue.revision + 1 });
    expect(await history(api, item)).toEqual([expect.objectContaining({ action: "accept", reason: "Approved after selecting a priority" })]);
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
  });

  test("responsibility requires a reviewer and applies only to new intake without assigning execution", async ({ page, triage }) => {
    const { api, workspace, owner } = triage;
    await configure(api);
    const before = await pending(api, "Intake before responsibility configuration");
    await page.goto(`/${workspace.slug}/settings?tab=triage`);
    await page.getByRole("combobox", { name: "Triage responsibility", exact: true }).click();
    await page.getByRole("option", { name: "Assign a reviewer", exact: true }).click();
    await expect(page.getByRole("button", { name: "Save settings", exact: true })).toBeDisabled();
    await page.getByRole("combobox", { name: "Reviewer", exact: true }).click();
    await page.getByRole("option", { name: owner.name, exact: true }).click();
    await page.getByRole("button", { name: "Save settings", exact: true }).click();
    await expect(page.getByText("Settings saved", { exact: true })).toBeVisible();
    expect(await api.requestJSON<Settings>("/api/triage/settings")).toMatchObject({ responsibility_mode: "assign", responsibility_member_id: owner.id });
    const after = await pending(api, "Intake after responsibility configuration");
    expect(after).toMatchObject({ reviewer_id: owner.id, reviewer_valid: true, issue: { assignee_id: null, admission_status: "pending" } });
    expect((await detail(api, before)).reviewer_id).toBeNull();
    expect(await api.countIssueDispatches(after.issue.id)).toBe(0);
  });

  test("a stale review draft retains its reason, refreshes the revision and commits exactly once", async ({ page, triage }) => {
    const { api, workspace } = triage;
    await configure(api);
    const item = await pending(api, "Two readers must not overwrite each other");
    await page.goto(`/${workspace.slug}/triage?issue=${item.issue.id}`);
    await page.getByRole("button", { name: "Reject", exact: true }).click();
    const dialog = page.getByRole("dialog");
    const reason = "Rejection entered before the other reader snoozed";
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill(reason);
    const otherReader = await detail(api, item);
    const snoozed = await decide(api, otherReader, "snooze", { snoozed_until: new Date(Date.now() + 3_600_000).toISOString() });
    await dialog.getByRole("button", { name: "Reject", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("triage item changed");
    await expect(dialog.getByRole("textbox", { name: "Reason", exact: true })).toHaveValue(reason);
    expect(await detail(api, item)).toEqual(snoozed.item);
    expect((await history(api, item)).map((event) => event.action)).toEqual(["snooze"]);
    await dialog.getByRole("button", { name: "Refresh current task", exact: true }).click();
    await expect(dialog.getByRole("alert")).toHaveCount(0);
    await dialog.getByRole("button", { name: "Reject", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    expect((await detail(api, item)).issue).toMatchObject({ admission_status: "rejected", revision: item.issue.revision + 2 });
    expect(await history(api, item)).toEqual([
      expect.objectContaining({ action: "snooze" }),
      expect.objectContaining({ action: "reject", reason }),
    ]);
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
  });

  test("a past snooze time keeps the item ready until a valid future time is confirmed", async ({ page, triage }) => {
    const { api, workspace } = triage;
    await configure(api);
    const item = await pending(api, "Correct an expired snooze date");
    await page.goto(`/${workspace.slug}/triage?issue=${item.issue.id}`);
    await page.getByRole("button", { name: "Snooze", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Custom time", { exact: true }).fill("2000-01-01T12:00");
    await dialog.getByRole("button", { name: "Snooze", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("Choose a future time within 90 days.");
    expect(await detail(api, item)).toEqual(item);
    expect(await history(api, item)).toEqual([]);
    await dialog.getByRole("button", { name: /In one hour/ }).click();
    await dialog.getByRole("button", { name: "Snooze", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const current = await detail(api, item);
    expect(current.issue.admission_status).toBe("pending");
    expect(Date.parse(current.snoozed_until!)).toBeGreaterThan(Date.now());
    expect(await api.requestJSON("/api/triage/items")).toMatchObject({ counts: { pending: 1, ready: 0, snoozed: 1 } });
    expect((await history(api, item)).map((event) => event.action)).toEqual(["snooze"]);
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
  });

  test("a rejected item cannot reopen while disabled and uses current responsibility in the next round", async ({ page, triage }) => {
    const { api, workspace, owner } = triage;
    await configure(api);
    const item = await pending(api, "Reopen under the current responsibility policy");
    const invalid = await request(triage, `/api/triage/items/${item.issue.id}/actions`, "POST", {
      request_id: randomUUID(), expected_revision: item.issue.revision, action: "reject", reason: "   ",
    });
    expect(invalid.status).toBe(400);
    expect(await invalid.json()).toMatchObject({ error: "reason is required" });
    expect(await detail(api, item)).toEqual(item);
    expect(await history(api, item)).toEqual([]);
    const rejected = await decide(api, item, "reject", { reason: "Needs a new review request" });
    await configure(api, { enabled: false });
    await page.goto(`/${workspace.slug}/triage?view=history&issue=${item.issue.id}`);
    await expect(page.getByRole("button", { name: "Reopen review", exact: true })).toBeDisabled();
    const blocked = await request(triage, `/api/triage/items/${item.issue.id}/actions`, "POST", {
      request_id: randomUUID(), expected_revision: rejected.item.issue.revision, action: "reopen", reason: "Additional customer context",
    });
    expect(blocked.status).toBe(409);
    expect(await blocked.json()).toMatchObject({ error: "enable triage before reopening" });
    expect(await detail(api, item)).toEqual(rejected.item);
    expect((await history(api, item)).map((event) => event.action)).toEqual(["reject"]);
    await configure(api, { responsibility_mode: "assign", responsibility_member_id: owner.id });
    await page.reload();
    await page.getByRole("button", { name: "Reopen review", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("Additional customer context");
    await dialog.getByRole("button", { name: "Reopen review", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    expect(await detail(api, item)).toMatchObject({ round: 2, reviewer_id: owner.id, snoozed_until: null, issue: { admission_status: "pending", status: "backlog", assignee_id: null, project_id: null } });
    expect(await history(api, item)).toEqual([
      expect.objectContaining({ action: "reject", round: 1, reason: "Needs a new review request" }),
      expect.objectContaining({ action: "reopen", round: 2, reason: "Additional customer context" }),
    ]);
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
  });
});

// The exhaustive validators and authorization matrix remain in Go's triage_test.go
// and triage_boundary_test.go. These cases exercise durable contracts over HTTP.
test.describe("Triage API integration branch coverage", () => {
  test("intake replay returns one identity and a changed payload cannot reuse its request ID", async ({ triage }) => {
    const { api } = triage;
    await configure(api);
    const input = { request_id: randomUUID(), title: "Same intake intention" };
    const first = await api.requestJSON<Item>("/api/triage/items", { method: "POST", body: input });
    const replay = await api.requestJSON<Item>("/api/triage/items", { method: "POST", body: input });
    expect(replay).toEqual(first);
    const conflict = await request(triage, "/api/triage/items", "POST", { ...input, title: "Different intake intention" });
    expect(conflict.status).toBe(409);
    expect(await conflict.json()).toMatchObject({ error: "request_id was already used for different input" });
    const queue = await api.requestJSON<{ items: Item[] }>("/api/triage/items?view=all");
    expect(queue.items.map((item) => item.issue.id)).toEqual([first.issue.id]);
    expect(await detail(api, first)).toEqual(first);
    expect(await history(api, first)).toEqual([]);
    expect(await api.countIssueDispatches(first.issue.id)).toBe(0);
  });

  test("decision replay preserves the original revision and history while rejecting a reused key with a new reason", async ({ triage }) => {
    const { api } = triage;
    await configure(api);
    const item = await pending(api, "Idempotent review decision");
    const input = { request_id: randomUUID(), expected_revision: item.issue.revision, action: "reject", reason: "Original decision" };
    const path = `/api/triage/items/${item.issue.id}/actions`;
    const first = await api.requestJSON<ActionResult>(path, { method: "POST", body: input });
    const replay = await api.requestJSON<ActionResult>(path, { method: "POST", body: input });
    expect(replay).toEqual(first);
    const conflict = await request(triage, path, "POST", { ...input, reason: "Changed intention needs another key" });
    expect(conflict.status).toBe(409);
    expect(await conflict.json()).toMatchObject({ error: "request_id already used for a different action" });
    expect(await detail(api, item)).toEqual(first.item);
    expect(first.item.issue.revision).toBe(item.issue.revision + 1);
    expect(await history(api, item)).toEqual([expect.objectContaining({ id: first.action.id, action: "reject", reason: "Original decision" })]);
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
  });

  test("pending items reject rerun and ordinary assignment while retaining editable content", async ({ triage }) => {
    const { api, owner } = triage;
    await configure(api);
    const item = await pending(api, "Review must precede execution");
    for (const [path, method, body] of [
      [`/api/issues/${item.issue.id}/rerun`, "POST", {}],
      [`/api/issues/${item.issue.id}`, "PUT", { assignee_type: "member", assignee_id: owner.id }],
    ] as const) {
      const response = await request(triage, path, method, body);
      expect(response.status, path).toBe(409);
      expect(await response.json()).toMatchObject({ code: "triage_review_required", issue_id: item.issue.id });
    }
    expect(await detail(api, item)).toEqual(item);
    await api.requestJSON(`/api/issues/${item.issue.id}`, { method: "PUT", body: { title: "Review content may still be clarified" } });
    expect((await detail(api, item)).issue).toMatchObject({ title: "Review content may still be clarified", admission_status: "pending", status: "backlog", assignee_id: null });
    expect(await history(api, item)).toEqual([]);
    expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
  });
});
