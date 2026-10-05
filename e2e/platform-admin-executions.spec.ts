import { randomUUID } from "node:crypto";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import pg from "pg";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test.describe("platform execution metadata", () => {
  test.skip(process.env.E2E_PLATFORM_ADMIN !== "1", "Requires the isolated password-mode production deployment");

  test("private metadata, filters, cursor, attempt detail and original content permissions", async ({ page }) => {
    test.setTimeout(120_000);
    const web = process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN;
    const apiBase = process.env.NEXT_PUBLIC_API_URL;
    const username = process.env.E2E_PLATFORM_ADMIN_USERNAME;
    const password = process.env.E2E_PLATFORM_ADMIN_PASSWORD;
    const database = process.env.DATABASE_URL;
    expect(web && apiBase && username && password && database, "Use the task-owned protected fixture environment").toBeTruthy();
    expect(["localhost", "127.0.0.1"]).toContain(new URL(web!).hostname);
    expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase!).hostname);
    expect(["localhost", "127.0.0.1"]).toContain(new URL(database!).hostname);
    const admin = new TestApiClient();
    const adminUser = await admin.loginPassword(username!, password!);
    const owner = new TestApiClient();
    const ownerName = `s03${randomUUID().replaceAll("-", "").slice(0, 20)}`;
    const ownerUser = await owner.registerPassword(ownerName, `fixture-${randomUUID()}`, "Execution acceptance owner");
    const db = new pg.Client(database);
    await db.connect();
    let workspace: { id: string; slug: string } | undefined;
    const reports = resolve(".omx/reports/platform-admin-executions");
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    try {
      workspace = await owner.ensureWorkspace("Execution acceptance", `s03-${randomUUID()}`);
      const privateTitle = `PRIVATE-S03-${randomUUID()}`;
      const issue = await owner.createIssue(privateTitle, { description: "Private body must remain in the original issue API" });
      const runtime = await owner.seedProjectRuntime();
      // There is no daemon or installed agent behind this fixture. Terminal
      // rows cannot dispatch; the offline runtime is metadata only.
      await db.query("UPDATE agent_runtime SET status='offline' WHERE id=$1", [runtime.id]);
      const agent = randomUUID(), failed = randomUUID(), retried = randomUUID(), quick = randomUUID();
      await db.query(`INSERT INTO agent (id,workspace_id,name,runtime_mode,runtime_config,visibility,permission_mode,owner_id,runtime_id)
        VALUES ($1,$2,'Private fake agent','local','{}','private','private',$3,$4)`, [agent, workspace.id, ownerUser.id, runtime.id]);
      await db.query(`INSERT INTO agent_task_queue
        (id,agent_id,runtime_id,issue_id,status,attempt,created_at,completed_at,error,failure_reason,work_dir,context)
        VALUES ($1,$2,$3,$4,'failed',1,now()-interval '1 hour',now(),'PRIVATE ERROR /fixture/private','timeout','/fixture/private','{"prompt":"PRIVATE PROMPT"}')`, [failed, agent, runtime.id, issue.id]);
      await db.query(`INSERT INTO agent_task_queue
        (id,agent_id,runtime_id,issue_id,status,attempt,retry_of_task_id,parent_task_id,created_at,completed_at)
        VALUES ($1,$2,$3,$4,'completed',2,$5,$5,now()-interval '2 hour',now())`, [retried, agent, runtime.id, issue.id, failed]);
      await db.query(`INSERT INTO task_usage (task_id,provider,model,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens)
        VALUES ($1,'openai','fixture-model',12,4,0,0)`, [retried]);
      await db.query(`INSERT INTO agent_task_queue (id,agent_id,runtime_id,status,created_at,completed_at,context)
        VALUES ($1,$2,$3,'failed',now()-interval '3 hour',now(),'{"type":"quick_create","prompt":"PRIVATE QUICK PROMPT"}')`, [quick, agent, runtime.id]);

      const filter = `workspace_id=${workspace.id}&source=issue&limit=1`;
      const response = await admin.requestJSON<{ items: Array<Record<string, unknown>>; next_cursor: string }>(`/api/admin/tasks?${filter}`);
      expect(response.items).toHaveLength(1);
      expect(response.items[0]).toMatchObject({ id: failed, title: null, content_access: false, usage: null, source: "issue", attempt: 1 });
      expect(JSON.stringify(response)).not.toMatch(/PRIVATE|\/fixture\/private/);
      const next = await admin.requestJSON<{ items: Array<Record<string, unknown>> }>(`/api/admin/tasks?${filter}&cursor=${encodeURIComponent(response.next_cursor)}`);
      expect(next.items[0]).toMatchObject({ id: retried, attempt: 2, retry_of_task_id: failed });
      const rejected = await fetch(`${apiBase}/api/issues/${issue.id}`, { headers: { Authorization: `Bearer ${admin.getToken()}`, "X-Workspace-ID": workspace.id } });
      expect(rejected.status).toBe(404);

      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`${web}/admin/tasks?${filter}`);
      await expect(page.locator("#password-username")).toBeVisible();
      await page.locator("#password-username").fill(username!);
      await page.locator("#password-value").fill(password!);
      await page.locator('button[type="submit"]').click();
      const executionRow = (id: string) => page.getByRole("row").filter({ has: page.getByText(id, { exact: true }) });
      const failedLink = executionRow(failed).getByRole("link", { name: "Restricted task", exact: true });
      await expect(failedLink).toBeVisible();
      expect(new URL((await failedLink.getAttribute("href"))!, web).pathname).toBe(`/admin/tasks/${failed}`);
      await expect(page.getByText(privateTitle)).toHaveCount(0);
      await expect(page.getByText("Restricted task", { exact: true })).toBeVisible();
      await mkdir(reports, { recursive: true });
      await page.screenshot({ path: resolve(reports, "executions-desktop.png"), fullPage: true });
      await page.setViewportSize({ width: 390, height: 844 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await page.screenshot({ path: resolve(reports, "executions-mobile.png"), fullPage: true });
      await page.getByRole("table").scrollIntoViewIfNeeded();
      await page.screenshot({ path: resolve(reports, "executions-mobile-table.png"), fullPage: true });
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.getByRole("button", { name: "Next page", exact: true }).click();
      const retriedLink = executionRow(retried).getByRole("link", { name: "Restricted task", exact: true });
      await expect(retriedLink).toBeVisible();
      expect(new URL((await retriedLink.getAttribute("href"))!, web).pathname).toBe(`/admin/tasks/${retried}`);
      await retriedLink.click();
      await expect(page.getByRole("heading", { name: "Execution details" })).toBeVisible();
      await expect(page.getByRole("link", { name: failed, exact: true })).toHaveCount(2);
      await page.locator("summary").filter({ hasText: "Technical details" }).click();
      await expect(page.getByText("fixture-model", { exact: true })).toBeVisible();
      await expect(page.getByRole("link", { name: "Open original content" })).toHaveCount(0);
      await page.goto(`${web}/admin/tasks/${failed}`);
      await expect(page.getByText("No usage has been reported for this execution.")).toBeVisible();
      await page.screenshot({ path: resolve(reports, "execution-detail.png"), fullPage: true });
      await page.goto(`${web}/admin/tasks?workspace_id=${workspace.id}`);
      await page.getByLabel("Source", { exact: true }).selectOption("quick_create");
      await page.getByRole("button", { name: "Apply filters", exact: true }).click();
      const quickLink = executionRow(quick).getByRole("link", { name: "Restricted task", exact: true });
      await expect(quickLink).toBeVisible();
      expect(new URL((await quickLink.getAttribute("href"))!, web).pathname).toBe(`/admin/tasks/${quick}`);
      await expect(executionRow(failed)).toHaveCount(0);

      // Grant only the original workspace membership, never a global content
      // bypass. Removing it during teardown restores the admin's prior state.
      await db.query("INSERT INTO member (workspace_id,user_id,role) VALUES ($1,$2,'member')", [workspace.id, adminUser.id]);
      const original = await fetch(`${apiBase}/api/issues/${issue.id}`, { headers: { Authorization: `Bearer ${admin.getToken()}`, "X-Workspace-ID": workspace.id } });
      expect(original.status).toBe(200);
      expect((await original.json()).title).toBe(privateTitle);
      await page.goto(`${web}/admin/issues?workspace_id=${workspace.id}`);
      await expect(page.getByRole("link", { name: privateTitle, exact: true })).toBeVisible();
      await expect(page.getByRole("link", { name: "2", exact: true })).toBeVisible();
      await page.goto(`${web}/admin/tasks/${failed}`);
      await expect(page.getByRole("link", { name: "Open original content" })).toHaveAttribute("href", `/${workspace.slug}/issues/${issue.id}`);
      expect(errors).toEqual([]);
    } finally {
      try {
        if (workspace) await owner.deleteFeatureWorkspace(workspace.id);
        await owner.deletePasswordAccount(ownerName);
      } finally { await db.end(); }
    }
  });
});
