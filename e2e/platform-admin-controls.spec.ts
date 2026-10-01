import { generateKeyPairSync, randomUUID, sign } from "node:crypto";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import pg from "pg";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test.describe("platform execution controls", () => {
  test.skip(process.env.E2E_PLATFORM_ADMIN !== "1", "Requires the isolated password-mode production deployment");
  test("admission receipts and original-key recovery survive a lost cancellation response and reload", async ({ page }) => {
    test.setTimeout(120_000);
    const base = process.env.NEXT_PUBLIC_API_URL;
    const web = process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN;
    const database = process.env.DATABASE_URL;
    const adminName = process.env.E2E_PLATFORM_ADMIN_USERNAME;
    const adminPassword = process.env.E2E_PLATFORM_ADMIN_PASSWORD;
    expect(base && web && database && adminName && adminPassword, "Explicit task-owned configuration is required").toBeTruthy();
    for (const value of [base!, web!, database!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
    const admin = new TestApiClient();
    await admin.loginPassword(adminName!, adminPassword!);
    const owner = new TestApiClient();
    const ownerName = `s04${randomUUID().replaceAll("-", "").slice(0, 20)}`;
    const ownerUser = await owner.registerPassword(ownerName, `fixture-${randomUUID()}`, "Control acceptance owner");
    const db = new pg.Client(database);
    await db.connect();
    let workspace: { id: string; slug: string } | undefined;
    let installationId: string | undefined;
    const queued = randomUUID(), running = randomUUID();
    const reportDir = resolve(".omx/reports/platform-admin-controls");
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    try {
      workspace = await owner.ensureWorkspace("Control acceptance", `s04-${randomUUID()}`);
      const keys = generateKeyPairSync("ed25519");
      const publicKey = keys.publicKey.export({ format: "der", type: "spki" }).subarray(-32).toString("base64url");
      const challenge = await owner.requestJSON<{ challenge_id: string; signature_payload: string; body_payload: string }>("/api/installations/challenges", { method: "POST", body: { purpose: "enroll", public_key: publicKey, desktop_version: "0.4.40", os: "macos" } });
      const enrolled = await owner.requestJSON<{ installation_id: string }>("/api/installations/enroll", { method: "POST", body: {
        challenge_id: challenge.challenge_id, signature_payload: challenge.signature_payload, body_payload: challenge.body_payload,
        signature: sign(null, Buffer.from(challenge.signature_payload, "base64url"), keys.privateKey).toString("base64url"),
      } });
      installationId = enrolled.installation_id;
      const runtime = await owner.seedProjectRuntime();
      // No daemon process exists behind these rows. The offline runtime cannot
      // invoke installed agents or consume provider quota.
      await db.query("UPDATE agent_runtime SET status='offline' WHERE id=$1", [runtime.id]);
      const agent = randomUUID();
      await db.query("INSERT INTO agent (id,workspace_id,name,runtime_mode,runtime_config,visibility,permission_mode,owner_id,runtime_id) VALUES ($1,$2,'Private fake control agent','local','{}','private','private',$3,$4)", [agent, workspace.id, ownerUser.id, runtime.id]);
      await db.query("INSERT INTO agent_task_queue (id,agent_id,runtime_id,status,context) VALUES ($1,$2,$3,'queued','{\"prompt\":\"PRIVATE CONTROL PROMPT\"}')", [queued, agent, runtime.id]);
      await db.query("INSERT INTO agent_task_queue (id,agent_id,runtime_id,status,dispatched_at,started_at,execution_installation_id,context) VALUES ($1,$2,$3,'running',now(),now(),$4,'{\"prompt\":\"PRIVATE CONTROL PROMPT\"}')", [running, agent, runtime.id, installationId]);

      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`${web}/admin/installations/${installationId}`);
      await page.locator("#password-username").fill(adminName!);
      await page.locator("#password-value").fill(adminPassword!);
      await page.locator('button[type="submit"]').click();
      await page.getByRole("button", { name: "Stop new claims", exact: true }).click();
      await expect(page.getByText(/Work already claimed continues/)).toBeVisible();
      await page.getByLabel("Reason", { exact: true }).fill("Scoped admission acceptance");
      await mkdir(reportDir, { recursive: true });
      await page.screenshot({ path: resolve(reportDir, "admission-confirmation.png"), fullPage: true });
      await page.getByRole("button", { name: "Confirm operation", exact: true }).click();
      await expect(page.getByText("The server has stopped new claims. In-flight work continues.", { exact: true })).toBeVisible();
      expect((await db.query("SELECT status FROM agent_task_queue WHERE id=$1", [running])).rows[0]?.status).toBe("running");
      await page.getByRole("button", { name: "Close", exact: true }).click();
      await page.getByRole("button", { name: "Resume new claims", exact: true }).click();
      await page.getByLabel("Reason", { exact: true }).fill("Scoped resume acceptance");
      await page.getByRole("button", { name: "Confirm operation", exact: true }).click();
      await expect(page.getByText("The server permits new claims again.", { exact: true })).toBeVisible();

      let writes = 0;
      let originalKey: string | undefined;
      let hiddenLookup = false;
      await page.route(`**/api/admin/tasks/${queued}/cancel`, async route => {
        writes += 1;
        originalKey = route.request().headers()["idempotency-key"];
        const accepted = await route.fetch();
        expect(accepted.status()).toBe(202);
        await route.abort("failed");
      });
      await page.route("**/api/admin/operations?idempotency_key=*", async route => {
        if (hiddenLookup) { await route.continue(); return; }
        hiddenLookup = true;
        const response = await route.fetch();
        const body = await response.json();
        await route.fulfill({ response, json: { ...body, items: [] } });
      });
      await page.goto(`${web}/admin/tasks/${queued}`);
      await page.getByRole("button", { name: "Cancel this execution", exact: true }).click();
      await page.getByLabel("Reason", { exact: true }).fill("Original cancellation acceptance reason");
      await page.getByRole("button", { name: "Confirm operation", exact: true }).click();
      await expect(page.getByText(/The outcome is uncertain/)).toBeVisible();
      expect(originalKey).toMatch(/^[0-9a-f-]{36}$/);
      await page.screenshot({ path: resolve(reportDir, "cancellation-uncertain.png"), fullPage: true });
      await page.reload();
      await expect(page.getByText("Cancelled before dispatch. No daemon acknowledgement was needed.", { exact: true })).toBeVisible();
      expect(writes).toBe(1);
      const receipts = await admin.requestJSON<{ items: Array<{ id: string; state: string; confirmation: string; result_code: string }> }>(`/api/admin/operations?idempotency_key=${originalKey}`);
      expect(receipts.items).toHaveLength(1);
      expect(receipts.items[0]).toMatchObject({ state: "succeeded", confirmation: "not_required", result_code: "cancelled_before_dispatch" });
      expect(JSON.stringify(receipts)).not.toContain("PRIVATE CONTROL");
      await page.getByRole("link", { name: "Open receipt", exact: true }).click();
      await expect(page).toHaveURL(`${web}/admin/operations/${receipts.items[0]!.id}`);
      await expect(page.getByText("No process confirmation required", { exact: true })).toBeVisible();
      await page.screenshot({ path: resolve(reportDir, "cancellation-receipt-desktop.png"), fullPage: true });
      await page.setViewportSize({ width: 390, height: 844 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await page.screenshot({ path: resolve(reportDir, "cancellation-receipt-mobile.png"), fullPage: true });

      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`${web}/admin/tasks/${running}`);
      await page.getByRole("button", { name: "Cancel this execution", exact: true }).click();
      await page.getByLabel("Reason", { exact: true }).fill("Legacy confirmation acceptance");
      await page.getByRole("button", { name: "Confirm operation", exact: true }).click();
      await expect(page.getByText("Applied on server", { exact: true })).toBeVisible();
      await expect(page.getByText("Process confirmation unavailable", { exact: true })).toBeVisible();
      await expect(page.getByText("Operation completed", { exact: true })).toHaveCount(0);
      await page.screenshot({ path: resolve(reportDir, "cancellation-unavailable.png"), fullPage: true });
      expect(errors).toEqual([]);
    } finally {
      try {
        await db.query("DELETE FROM admin_audit_event WHERE target_id=ANY($1::uuid[])", [[queued, running, ...(installationId ? [installationId] : [])]]);
        await db.query("DELETE FROM admin_operation WHERE target_id=ANY($1::uuid[])", [[queued, running, ...(installationId ? [installationId] : [])]]);
        if (workspace) await owner.deleteFeatureWorkspace(workspace.id);
        if (installationId) {
          await db.query("DELETE FROM installation_user WHERE installation_id=$1", [installationId]);
          await db.query("DELETE FROM managed_installation WHERE id=$1 AND responsible_user_id=$2", [installationId, ownerUser.id]);
        }
        await db.query("DELETE FROM installation_challenge WHERE user_id=$1", [ownerUser.id]);
        await owner.deletePasswordAccount(ownerName);
      } finally { await db.end(); }
    }
  });
});
