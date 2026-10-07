import { randomUUID } from "node:crypto";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import pg from "pg";
import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

async function login(page: Page, username: string, password: string) {
  await page.locator("#password-username").fill(username);
  await page.locator("#password-value").fill(password);
  await page.locator('button[type="submit"]').click();
}
test.describe("platform observability", () => {
  test.skip(process.env.E2E_PLATFORM_ADMIN !== "1", "Requires the isolated password-mode production snapshot");
  test("windowed metrics, active alerts, original-key recovery and observer boundaries", async ({ page, browser }) => {
    test.setTimeout(150_000);
    const api = process.env.NEXT_PUBLIC_API_URL;
    const web = process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN;
    const database = process.env.DATABASE_URL;
    const username = process.env.E2E_PLATFORM_ADMIN_USERNAME;
    const password = process.env.E2E_PLATFORM_ADMIN_PASSWORD;
    expect(api && web && database && username && password).toBeTruthy();
    for (const value of [api!, web!, database!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
    const administrator = new TestApiClient();
    await administrator.loginPassword(username!, password!);
    const identity = await administrator.requestJSON<{ organization_id: string }>("/api/admin/me");
    const owner = new TestApiClient();
    const ownerName = `s06${randomUUID().replaceAll("-", "").slice(0, 20)}`;
    const ownerUser = await owner.registerPassword(ownerName, `fixture-${randomUUID()}`, "Observability fixture owner");
    const observer = new TestApiClient();
    const observerName = `s06o${randomUUID().replaceAll("-", "").slice(0, 19)}`;
    const observerPassword = `fixture-${randomUUID()}`;
    const observerUser = await observer.registerPassword(observerName, observerPassword, "Observability fixture observer");
    const db = new pg.Client(database);
    await db.connect();
    const finished = randomUUID(), queued = randomUUID(), failureAlert = randomUUID(), oldAlert = randomUUID();
    let workspace: { id: string; slug: string } | undefined;
    const reportDir = resolve(".omx/reports/platform-admin-observability");
    const observerContext = await browser.newContext({ locale: "en-US" });
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    try {
      workspace = await owner.ensureWorkspace("Observability acceptance", `s06-${randomUUID()}`);
      const runtime = await owner.seedProjectRuntime();
      await db.query("UPDATE agent_runtime SET status='offline' WHERE id=$1", [runtime.id]);
      const agent = randomUUID();
      // These rows have no attached daemon or provider process.
      await db.query("INSERT INTO agent (id,workspace_id,name,runtime_mode,runtime_config,visibility,permission_mode,owner_id,runtime_id) VALUES ($1,$2,'Private observation fixture','local','{}','private','private',$3,$4)", [agent, workspace.id, ownerUser.id, runtime.id]);
      await db.query("INSERT INTO agent_task_queue (id,agent_id,runtime_id,status,created_at,completed_at,context) VALUES ($1,$2,$3,'failed',now()-interval '2 days',now()-interval '5 minutes','{\"prompt\":\"PRIVATE OBSERVATION PROMPT\"}')", [finished, agent, runtime.id]);
      await db.query("INSERT INTO agent_task_queue (id,agent_id,runtime_id,status,created_at,context) VALUES ($1,$2,$3,'queued',now()-interval '45 days','{\"prompt\":\"PRIVATE OBSERVATION PROMPT\"}')", [queued, agent, runtime.id]);
      await db.query("INSERT INTO admin_alert (id,organization_id,rule,subject_kind,subject_id,fingerprint,severity,first_seen_at) VALUES ($1,$2,'execution_failed','task',$3,$4,'warning',now()),($5,$2,'queue_timeout','task',$6,$7,'warning',now()-interval '40 days') ON CONFLICT DO NOTHING", [failureAlert, identity.organization_id, finished, `v1:execution_failed:${finished}`, oldAlert, queued, `v1:queue_timeout:${queued}`]);
      // A detector may have observed a fixture between its insertion and the
      // explicit alert fixture, so use its canonical persisted episode ID.
      const alerts = await db.query<{ id: string; subject_id: string }>("SELECT id,subject_id FROM admin_alert WHERE subject_id=ANY($1::uuid[]) AND status<>'closed'", [[finished, queued]]);
      const failureId = alerts.rows.find(row => row.subject_id === finished)!.id;
      const queueId = alerts.rows.find(row => row.subject_id === queued)!.id;
      await db.query("UPDATE admin_alert SET first_seen_at=now()-interval '40 days' WHERE id=$1", [queueId]);
      const detail = await administrator.requestJSON<{ user: { auth_version: number } }>(`/api/admin/users/${observerUser.id}`);
      await administrator.requestJSON(`/api/admin/users/${observerUser.id}/role`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { role: "platform_observer", expected_role: null, expected_auth_version: detail.user.auth_version, password, reason: "Scoped observability browser fixture" } });
      await observer.loginPassword(observerName, observerPassword);

      const timeTo = new Date(Date.now() + 60_000).toISOString();
      const timeFrom = new Date(Date.now() - 3_600_000).toISOString();
      const window = new URLSearchParams({ time_from: timeFrom, time_to: timeTo, timezone: "Asia/Shanghai" });
      const overview = await administrator.requestJSON<{ executions: { failed: number }; usage: { billing: string; missing_tasks: number } }>(`/api/admin/overview?${window}`);
      expect(overview.executions.failed).toBeGreaterThanOrEqual(1);
      expect(overview.usage.missing_tasks).toBeGreaterThanOrEqual(1);
      expect(overview.usage.billing).toBe("tokens_only");
      const finishParams = new URLSearchParams(window); finishParams.set("time_basis", "finished"); finishParams.set("workspace_id", workspace.id); finishParams.set("status", "failed");
      const finishedList = await administrator.requestJSON<{ items: { id: string }[] }>(`/api/admin/tasks?${finishParams}`);
      expect(finishedList.items.some(item => item.id === finished)).toBe(true);
      finishParams.set("time_basis", "created");
      expect((await administrator.requestJSON<{ items: { id: string }[] }>(`/api/admin/tasks?${finishParams}`)).items.some(item => item.id === finished)).toBe(false);
      const current = await administrator.requestJSON<{ items: { id: string }[] }>(`/api/admin/tasks?state_scope=current&status=queued&time_basis=created&workspace_id=${workspace.id}`);
      expect(current.items.some(item => item.id === queued)).toBe(true);

      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`${web}/admin?${window}`);
      await login(page, username!, password!);
      await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
      await expect(page.getByText(/not a bill/)).toBeVisible();
      const failedLink = page.getByRole("link", { name: "Failed", exact: true });
      const href = await failedLink.getAttribute("href");
      expect(new URL(href!, web).searchParams.get("time_basis")).toBe("finished");
      expect(new URL(href!, web).searchParams.get("timezone")).toBe("Asia/Shanghai");
      await mkdir(reportDir, { recursive: true });
      await page.screenshot({ path: resolve(reportDir, "overview-desktop.png"), fullPage: true });
      await page.goto(`${web}/admin/alerts?subject_id=${queued}`);
      await expect(page.getByRole("link", { name: "Execution queued for over five minutes", exact: true })).toBeVisible();

      let writes = 0;
      let originalKey: string | undefined;
      let hiddenLookup = false;
      await page.route(`**/api/admin/alerts/${failureId}/acknowledge`, async route => {
        writes++; originalKey = route.request().headers()["idempotency-key"];
        const response = await route.fetch(); expect(response.ok()).toBe(true);
        await route.abort("failed");
      });
      await page.route("**/api/admin/operations?idempotency_key=*", async route => {
        if (hiddenLookup) { await route.continue(); return; }
        hiddenLookup = true; const response = await route.fetch(); const body = await response.json();
        await route.fulfill({ response, json: { ...body, items: [] } });
      });
      await page.goto(`${web}/admin/alerts/${failureId}`);
      await page.getByRole("button", { name: "Acknowledge alert", exact: true }).click();
      await page.getByLabel("Reason", { exact: true }).fill("Investigating the synthetic failed execution");
      await page.getByRole("button", { name: "Confirm operation", exact: true }).click();
      await expect(page.getByText(/The outcome is uncertain/)).toBeVisible();
      await page.reload();
      await expect(page.getByText("The alert was acknowledged. Recovery remains a separate observation.", { exact: true })).toBeVisible();
      expect(writes).toBe(1);
      const operation = await administrator.requestJSON<{ items: { id: string; kind: string; state: string }[] }>(`/api/admin/operations?idempotency_key=${originalKey}`);
      expect(operation.items).toHaveLength(1);
      expect(operation.items[0]).toMatchObject({ kind: "alert.acknowledge", state: "succeeded" });
      await page.getByRole("button", { name: "Close", exact: true }).click();
      await page.getByRole("button", { name: "Close alert", exact: true }).click();
      await page.getByRole("combobox", { name: "Disposition", exact: true }).selectOption("handled");
      await page.getByLabel("Reason", { exact: true }).fill("Synthetic failure investigated and handled");
      await page.getByRole("button", { name: "Confirm operation", exact: true }).click();
      await expect(page.getByText("The alert was closed with its recorded conclusion.", { exact: true })).toBeVisible();

      const observerPage = await observerContext.newPage();
      await observerPage.goto(`${web}/admin/alerts/${queueId}`);
      await login(observerPage, observerName, observerPassword);
      await expect(observerPage.getByRole("heading", { name: "Alert details", exact: true })).toBeVisible();
      for (const name of ["Acknowledge alert", "Assign administrator", "Close alert"]) await expect(observerPage.getByRole("button", { name, exact: true })).toHaveCount(0);
      const denied = await fetch(`${api}/api/admin/alerts/${queueId}/acknowledge`, { method: "POST", headers: { Authorization: `Bearer ${observer.getToken()}`, "Content-Type": "application/json", "Idempotency-Key": randomUUID() }, body: JSON.stringify({ expected_version: "1", reason: "Observer must not write" }) });
      expect(denied.status).toBe(403);
      for (const [path, title] of [["health", "Service health"], ["settings", "Deployment settings"], ["workspaces", "Workspaces"], ["audit", "Audit trail"]]) {
        await page.goto(`${web}/admin/${path}`);
        await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
        await expect(page.getByText("Data could not be loaded", { exact: true })).toHaveCount(0);
      }
      await page.goto(`${web}/admin/audit?target_id=${failureId}`);
      await expect(page.locator("summary").filter({ hasText: "Event details" }).first()).toBeVisible();
      for (const summary of await page.locator("summary").filter({ hasText: "Event details" }).all()) {
        await summary.click();
      }
      await expect(page.getByRole("link", { name: "Open own operation receipt", exact: true }).first()).toBeVisible();
      const receipt = page.getByRole("link", { name: "Open own operation receipt", exact: true })
        .and(page.locator(`a[href^="/admin/operations/${operation.items[0]!.id}?"]`)).first();
      await expect(receipt).toBeVisible();
      const receiptURL = new URL((await receipt.getAttribute("href"))!, web);
      expect(receiptURL.pathname).toBe(`/admin/operations/${operation.items[0]!.id}`);
      const returnTo = new URL(receiptURL.searchParams.get("return_to")!, web);
      expect(returnTo.pathname).toBe("/admin/audit");
      expect(returnTo.searchParams.get("target_id")).toBe(failureId);
      const audit = await administrator.requestJSON(`/api/admin/audit?target_id=${failureId}`);
      expect(JSON.stringify(audit)).not.toContain("PRIVATE OBSERVATION");
      await page.screenshot({ path: resolve(reportDir, "audit-desktop.png"), fullPage: true });
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(`${web}/admin/alerts/${queueId}`);
      await expect(page.getByRole("heading", { name: "Alert details", exact: true })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.screenshot({ path: resolve(reportDir, "alert-mobile.png"), fullPage: true });
      expect(errors).toEqual([]);
    } finally {
      await observerContext.close();
      try {
        if (workspace) await owner.deleteFeatureWorkspace(workspace.id);
        const alertRows = await db.query<{ id: string }>("SELECT id FROM admin_alert WHERE subject_id=ANY($1::uuid[])", [[finished, queued]]);
        const targets = [observerUser.id, ...alertRows.rows.map(row => row.id)];
        await db.query("DELETE FROM admin_audit_event WHERE target_id=ANY($1::uuid[])", [targets]);
        await db.query("DELETE FROM admin_operation WHERE target_id=ANY($1::uuid[])", [targets]);
        await db.query("DELETE FROM admin_alert WHERE subject_id=ANY($1::uuid[])", [[finished, queued]]);
        await db.query("DELETE FROM platform_role_binding WHERE user_id=$1", [observerUser.id]);
        await observer.deletePasswordAccount(observerName);
        await owner.deletePasswordAccount(ownerName);
      } finally { await db.end(); }
    }
  });
});
