import { randomUUID } from "node:crypto";
import { test as base, expect, type Locator, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Real browser + real API lifecycle coverage. Only named failure cases replace
// their target mutation response; all fixture setup, reads and retries are real.
// The runtime fixture has no daemon process, credentials or model attached.
// Pure schedule/validation matrices remain canonical in the adjacent component
// and schedule-editor unit suites; these cases cover persistence and UI wiring.

interface Autopilot {
  id: string;
  title: string;
  description: string | null;
  assignee_id: string;
  execution_mode: string;
  status: string;
  template_key: string;
}

interface AutopilotDetail {
  autopilot: Autopilot;
  triggers: {
    id: string;
    kind: string;
    enabled: boolean;
    cron_expression: string | null;
    timezone: string | null;
    next_run_at: string | null;
  }[];
}

interface LifecycleFixture {
  api: TestApiClient;
  slug: string;
  agentId: string;
}

const AGENT_NAME = "Lifecycle fixture agent";
const MANUAL_EMPTY = "No schedule — this autopilot only runs when triggered manually.";
const NO_TRIGGERS = "No triggers configured. Add a schedule to run automatically.";

const test = base.extend<{ lifecycle: LifecycleFixture }>({
  lifecycle: async ({ page, baseURL }, use) => {
    const api = new TestApiClient();
    const slug = `e2e-ap-life-${randomUUID().slice(0, 12)}`;
    let workspaceId: string | undefined;
    try {
      await api.login(`${slug}@multica.ai`, "Autopilot lifecycle user");
      const workspace = await api.ensureWorkspace("Autopilot lifecycle workspace", slug);
      expect(workspace.slug).toBe(slug);
      workspaceId = workspace.id;
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
      const token = api.getToken();
      if (!token || !baseURL) throw new Error("Authenticated E2E setup is incomplete");
      await page.context().addCookies([
        { name: "multica-locale", value: "en", url: baseURL },
        { name: "multica_logged_in", value: "1", url: baseURL },
      ]);
      await page.addInitScript((value) => {
        localStorage.setItem("multica_token", value);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, token);
      const runtime = await api.seedProjectRuntime();
      const agent = await api.requestJSON<{ id: string }>("/api/agents", {
        method: "POST",
        body: { name: AGENT_NAME, runtime_id: runtime.id, permission_mode: "private" },
      });
      await use({ api, slug, agentId: agent.id });
    } finally {
      if (workspaceId) await api.deleteFeatureWorkspace(workspaceId);
    }
  },
});

test.use({ locale: "en-US", timezoneId: "UTC", viewport: { width: 1440, height: 960 } });

function mutationResponse(page: Page, path: string, method: string) {
  return page.waitForResponse((response) =>
    new URL(response.url()).pathname === path && response.request().method() === method,
  );
}

function detail(api: TestApiClient, id: string) {
  return api.requestJSON<AutopilotDetail>(`/api/autopilots/${id}`);
}

function list(api: TestApiClient) {
  return api.requestJSON<{ autopilots: Autopilot[] }>("/api/autopilots");
}

async function seedManual({ api, agentId }: LifecycleFixture, title: string) {
  // The browser's blank-create form always adds a schedule. Manual autopilots
  // are a supported API-created state which the browser must preserve on edit.
  const autopilot = await api.requestJSON<Autopilot>("/api/autopilots", {
    method: "POST",
    body: { title, assignee_type: "agent", assignee_id: agentId, execution_mode: "run_only" },
  });
  expect((await detail(api, autopilot.id)).triggers).toEqual([]);
  return autopilot;
}

async function openCreate(page: Page, slug: string) {
  await page.goto(`/${slug}/autopilots`);
  await page.getByRole("button", { name: "Start from scratch", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New Autopilot", exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function selectAgent(page: Page, dialog: Locator) {
  await dialog.getByRole("button", { name: "Select agent or squad", exact: true }).click();
  await page.getByRole("button", { name: new RegExp(AGENT_NAME) }).click();
}

async function openEdit(page: Page, slug: string, id: string) {
  await page.goto(`/${slug}/autopilots/${id}`);
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Edit Autopilot", exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function saveEdit(page: Page, dialog: Locator, id: string) {
  const response = mutationResponse(page, `/api/autopilots/${id}`, "PATCH");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(dialog).toBeHidden();
}

test.describe("autopilot lifecycle branches", () => {
  test("focuses missing required fields and creates nothing until title and executor are supplied", async ({ page, lifecycle }) => {
    const { api, slug } = lifecycle;
    let createRequests = 0;
    page.on("request", (request) => {
      if (request.method() === "POST" && new URL(request.url()).pathname === "/api/autopilots") createRequests += 1;
    });
    const dialog = await openCreate(page, slug);
    const title = dialog.getByRole("textbox", { name: "Autopilot name", exact: true });
    const submit = dialog.getByRole("button", { name: "Create autopilot", exact: true });
    await expect(submit).toBeEnabled();
    await submit.click();
    await expect(dialog.getByText("Enter a name for this autopilot.", { exact: true })).toBeVisible();
    await expect(title).toBeFocused();
    await title.fill("Required fields recovery");
    await submit.click();
    const picker = dialog.getByRole("button", { name: "Select agent or squad", exact: true });
    await expect(picker).toBeFocused();
    await expect(picker).toHaveAttribute("aria-invalid", "true");
    await expect(dialog.getByText("Enter a name for this autopilot.", { exact: true })).toBeHidden();
    expect((await list(api)).autopilots).toEqual([]);
    expect(createRequests).toBe(0);
    await selectAgent(page, dialog);
    await expect(dialog.getByText("Choose the agent or squad that will run this autopilot.", { exact: true })).toBeHidden();
    const response = mutationResponse(page, "/api/autopilots", "POST");
    await submit.click();
    expect((await response).status()).toBe(201);
    await expect(dialog).toBeHidden();
    expect((await list(api)).autopilots).toHaveLength(1);
    expect(createRequests).toBe(1);
  });

  test("creates a non-template run-only autopilot with its default schedule and persists it on reload", async ({ page, lifecycle }) => {
    const { api, slug, agentId } = lifecycle;
    const dialog = await openCreate(page, slug);
    await dialog.getByRole("textbox", { name: "Autopilot name", exact: true }).fill("Custom recurring audit");
    await selectAgent(page, dialog);
    await dialog.getByRole("button", { name: /Run only Runs directly/ }).click();
    const create = mutationResponse(page, "/api/autopilots", "POST");
    await dialog.getByRole("button", { name: "Create autopilot", exact: true }).click();
    const response = await create;
    expect(response.status()).toBe(201);
    const created: Autopilot = await response.json();
    await expect(dialog).toBeHidden();
    const saved = await detail(api, created.id);
    expect(saved.autopilot).toMatchObject({ title: "Custom recurring audit", assignee_id: agentId, execution_mode: "run_only", status: "active", template_key: "" });
    expect(saved.triggers).toHaveLength(1);
    // The editor's wire contract embeds the timezone as well as storing it in
    // the timezone field; the display strips that prefix for readability.
    expect(saved.triggers[0]).toMatchObject({ kind: "schedule", enabled: true, cron_expression: "TZ=UTC 0 9 * * *", timezone: "UTC" });
    expect(Date.parse(saved.triggers[0].next_run_at ?? "")).not.toBeNaN();
    await page.goto(`/${slug}/autopilots/${created.id}`);
    await page.reload();
    await expect(page.getByRole("heading", { name: created.title, exact: true })).toBeVisible();
    expect((await detail(api, created.id)).triggers).toEqual(saved.triggers);
  });

  test("renames a manual autopilot without silently creating a schedule", async ({ page, lifecycle }) => {
    const { api, slug } = lifecycle;
    const autopilot = await seedManual(lifecycle, "Manual before rename");
    const dialog = await openEdit(page, slug, autopilot.id);
    await expect(dialog.getByText(MANUAL_EMPTY, { exact: true })).toBeVisible();
    let triggerWrites = 0;
    page.on("request", (request) => {
      if (["POST", "PATCH"].includes(request.method()) && new URL(request.url()).pathname.startsWith(`/api/autopilots/${autopilot.id}/triggers`)) triggerWrites += 1;
    });
    await dialog.getByRole("textbox", { name: "Autopilot name", exact: true }).fill("Manual after rename");
    await saveEdit(page, dialog, autopilot.id);
    await page.reload();
    await expect(page.getByRole("heading", { name: "Manual after rename", exact: true })).toBeVisible();
    await expect(page.getByText(NO_TRIGGERS, { exact: true })).toBeVisible();
    expect((await detail(api, autopilot.id)).triggers).toEqual([]);
    expect(triggerWrites).toBe(0);
  });

  test("adds an explicitly requested default schedule to a manual autopilot exactly once", async ({ page, lifecycle }) => {
    const { api, slug } = lifecycle;
    const autopilot = await seedManual(lifecycle, "Explicit schedule audit");
    const dialog = await openEdit(page, slug, autopilot.id);
    await dialog.getByRole("button", { name: "Add schedule", exact: true }).click();
    await expect(dialog.getByText(MANUAL_EMPTY, { exact: true })).toBeHidden();
    const triggerResponse = mutationResponse(page, `/api/autopilots/${autopilot.id}/triggers`, "POST");
    await saveEdit(page, dialog, autopilot.id);
    expect((await triggerResponse).status()).toBe(201);
    const saved = await detail(api, autopilot.id);
    expect(saved.triggers).toHaveLength(1);
    expect(saved.triggers[0]).toMatchObject({ kind: "schedule", enabled: true, cron_expression: "TZ=UTC 0 9 * * *", timezone: "UTC" });
    // A second title-only save must patch the autopilot, not add a duplicate.
    const reopened = await openEdit(page, slug, autopilot.id);
    await reopened.getByRole("textbox", { name: "Autopilot name", exact: true }).fill("Explicit schedule renamed");
    await saveEdit(page, reopened, autopilot.id);
    await page.reload();
    expect((await detail(api, autopilot.id)).triggers).toEqual(saved.triggers);
    await expect(page.getByText(NO_TRIGGERS, { exact: true })).toBeHidden();
  });

  test("persists pause and resume across reload and gates manual runs while paused", async ({ page, lifecycle }) => {
    const { api, slug } = lifecycle;
    const autopilot = await seedManual(lifecycle, "Pause resume audit");
    await page.goto(`/${slug}/autopilots/${autopilot.id}`);
    const pause = mutationResponse(page, `/api/autopilots/${autopilot.id}`, "PATCH");
    await page.getByRole("switch", { name: "Pause autopilot", exact: true }).click();
    expect((await pause).status()).toBe(200);
    await page.reload();
    await expect(page.getByRole("switch", { name: "Activate autopilot", exact: true })).not.toBeChecked();
    await expect(page.getByRole("button", { name: "Run now", exact: true })).toBeDisabled();
    expect((await detail(api, autopilot.id)).autopilot.status).toBe("paused");
    const resume = mutationResponse(page, `/api/autopilots/${autopilot.id}`, "PATCH");
    await page.getByRole("switch", { name: "Activate autopilot", exact: true }).click();
    expect((await resume).status()).toBe(200);
    await page.reload();
    await expect(page.getByRole("switch", { name: "Pause autopilot", exact: true })).toBeChecked();
    await expect(page.getByRole("button", { name: "Run now", exact: true })).toBeEnabled();
    expect((await detail(api, autopilot.id)).autopilot.status).toBe("active");
    expect((await api.requestJSON<{ runs: unknown[] }>(`/api/autopilots/${autopilot.id}/runs`)).runs).toEqual([]);
  });

  test("rolls back a rejected pause and can pause successfully after retry", async ({ page, lifecycle }) => {
    const { api, slug } = lifecycle;
    const autopilot = await seedManual(lifecycle, "Rejected pause audit");
    const path = `/api/autopilots/${autopilot.id}`;
    await page.goto(`/${slug}/autopilots/${autopilot.id}`);
    const pattern = `**${path}`;
    await page.route(pattern, (route) => route.request().method() === "PATCH"
      ? route.fulfill({ status: 503, json: { error: "Injected pause failure" } })
      : route.continue());
    const rejected = mutationResponse(page, path, "PATCH");
    await page.getByRole("switch", { name: "Pause autopilot", exact: true }).click();
    expect((await rejected).status()).toBe(503);
    await expect(page.getByRole("switch", { name: "Pause autopilot", exact: true })).toBeChecked();
    await expect(page.getByRole("button", { name: "Run now", exact: true })).toBeEnabled();
    expect((await detail(api, autopilot.id)).autopilot.status).toBe("active");
    await page.unroute(pattern);
    const retried = mutationResponse(page, path, "PATCH");
    await page.getByRole("switch", { name: "Pause autopilot", exact: true }).click();
    expect((await retried).status()).toBe(200);
    await page.reload();
    await expect(page.getByRole("button", { name: "Run now", exact: true })).toBeDisabled();
    expect((await detail(api, autopilot.id)).autopilot.status).toBe("paused");
  });

  test("cancels detail deletion without issuing a delete or losing the manual autopilot", async ({ page, lifecycle }) => {
    const { api, slug } = lifecycle;
    const autopilot = await seedManual(lifecycle, "Keep cancelled deletion");
    let deletes = 0;
    page.on("request", (request) => {
      if (request.method() === "DELETE" && new URL(request.url()).pathname === `/api/autopilots/${autopilot.id}`) deletes += 1;
    });
    await page.goto(`/${slug}/autopilots/${autopilot.id}`);
    await page.getByRole("button", { name: "Delete autopilot", exact: true }).click();
    const dialog = page.getByRole("alertdialog", { name: "Delete autopilot", exact: true });
    await expect(dialog).toContainText(autopilot.title);
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).toBeHidden();
    await page.reload();
    await expect(page.getByRole("heading", { name: autopilot.title, exact: true })).toBeVisible();
    expect((await detail(api, autopilot.id)).triggers).toEqual([]);
    expect(deletes).toBe(0);
  });

  test("restores the list row after failed deletion and removes it after a real retry", async ({ page, lifecycle }) => {
    const { api, slug } = lifecycle;
    const autopilot = await seedManual(lifecycle, "Retry list deletion");
    const path = `/api/autopilots/${autopilot.id}`;
    await page.goto(`/${slug}/autopilots`);
    await page.getByRole("button", { name: "Automation actions", exact: true }).click();
    await page.getByRole("menuitem", { name: "Delete", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Delete automation?", exact: true });
    const pattern = `**${path}`;
    await page.route(pattern, (route) => route.request().method() === "DELETE"
      ? route.fulfill({ status: 503, json: { error: "Injected delete failure" } })
      : route.continue());
    const rejected = mutationResponse(page, path, "DELETE");
    await dialog.getByRole("button", { name: "Delete permanently", exact: true }).click();
    expect((await rejected).status()).toBe(503);
    await expect(page.getByText(autopilot.title, { exact: true })).toBeVisible();
    expect((await detail(api, autopilot.id)).autopilot.title).toBe(autopilot.title);
    await page.unroute(pattern);
    // Re-enter through a fresh server read, independent of the failed dialog's
    // mount lifetime while the list mutation rolls back its optimistic row.
    await page.reload();
    await page.getByRole("button", { name: "Automation actions", exact: true }).click();
    await page.getByRole("menuitem", { name: "Delete", exact: true }).click();
    const retried = mutationResponse(page, path, "DELETE");
    await dialog.getByRole("button", { name: "Delete permanently", exact: true }).click();
    expect((await retried).status()).toBe(204);
    await expect(page.getByText(autopilot.title, { exact: true })).toBeHidden();
    expect((await list(api)).autopilots).toEqual([]);
    await page.reload();
    await expect(page.getByRole("button", { name: "Start from scratch", exact: true })).toBeVisible();
  });

  test("preserves a created autopilot when its schedule fails and repairs it without a duplicate", async ({ page, lifecycle }) => {
    const { api, slug } = lifecycle;
    const dialog = await openCreate(page, slug);
    await dialog.getByRole("textbox", { name: "Autopilot name", exact: true }).fill("Recover partial creation");
    await selectAgent(page, dialog);
    const pattern = /\/api\/autopilots\/[^/]+\/triggers(?:\?|$)/;
    await page.route(pattern, (route) => route.request().method() === "POST"
      ? route.fulfill({ status: 409, json: { error: "Injected schedule conflict" } })
      : route.continue());
    const create = mutationResponse(page, "/api/autopilots", "POST");
    await dialog.getByRole("button", { name: "Create autopilot", exact: true }).click();
    const response = await create;
    expect(response.status()).toBe(201);
    const created: Autopilot = await response.json();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("Autopilot created, but schedule failed to save: Injected schedule conflict", { exact: true })).toBeVisible();
    expect((await list(api)).autopilots.map((item) => item.id)).toEqual([created.id]);
    expect((await detail(api, created.id)).triggers).toEqual([]);
    await page.unroute(pattern);
    const edit = await openEdit(page, slug, created.id);
    await expect(edit.getByText(MANUAL_EMPTY, { exact: true })).toBeVisible();
    await edit.getByRole("button", { name: "Add schedule", exact: true }).click();
    const trigger = mutationResponse(page, `/api/autopilots/${created.id}/triggers`, "POST");
    await saveEdit(page, edit, created.id);
    expect((await trigger).status()).toBe(201);
    await page.reload();
    expect((await list(api)).autopilots.map((item) => item.id)).toEqual([created.id]);
    expect((await detail(api, created.id)).triggers).toHaveLength(1);
    await expect(page.getByText(NO_TRIGGERS, { exact: true })).toBeHidden();
  });
});
