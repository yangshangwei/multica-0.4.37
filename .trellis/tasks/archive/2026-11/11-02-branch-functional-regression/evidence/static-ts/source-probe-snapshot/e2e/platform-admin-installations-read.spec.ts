import { createHash, generateKeyPairSync, randomUUID, sign } from "node:crypto";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import pg from "pg";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

interface Challenge { challenge_id: string; signature_payload: string; body_payload: string }
interface InstallationPage { items: Array<{ id: string; runtime_count: number; client_activity: { state: string }; daemon_reachability: { state: string }; execution_readiness: { state: string } }> }

test.describe("managed installation read model", () => {
  test.skip(process.env.E2E_PLATFORM_ADMIN !== "1", "Requires the isolated password-mode production deployment");

  test("real enrollment and binding keep client, daemon and engine status separate", async ({ page }) => {
    test.setTimeout(120_000);
    const base = process.env.NEXT_PUBLIC_API_URL;
    const web = process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN;
    const database = process.env.DATABASE_URL;
    const adminName = process.env.E2E_PLATFORM_ADMIN_USERNAME;
    const adminPassword = process.env.E2E_PLATFORM_ADMIN_PASSWORD;
    expect(base && web && database && adminName && adminPassword, "Explicit task-owned fixture configuration is required").toBeTruthy();
    for (const value of [base!, web!, database!]) expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
    const admin = new TestApiClient();
    await admin.loginPassword(adminName!, adminPassword!);
    const owner = new TestApiClient();
    const username = `s02${randomUUID().replaceAll("-", "").slice(0, 20)}`;
    const ownerUser = await owner.registerPassword(username, `fixture-${randomUUID()}`, "Installation acceptance owner");
    const db = new pg.Client(database);
    await db.connect();
    const keys = generateKeyPairSync("ed25519");
    const publicBytes = keys.publicKey.export({ format: "der", type: "spki" }).subarray(-32);
    const publicKey = publicBytes.toString("base64url");
    const fingerprint = createHash("sha256").update(publicBytes).digest("hex");
    const reportDir = resolve(".omx/reports/platform-admin-installations-read");
    let workspace: { id: string; slug: string } | undefined;
    let installationID: string | undefined;
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    async function daemonJSON<T>(path: string, token: string, body: unknown): Promise<T> {
      const result = await fetch(`${base}${path}`, { method: "POST", headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json", "X-Client-Capabilities": "managed_installation_v1" }, body: JSON.stringify(body) });
      if (!result.ok) throw new Error(`Fake protocol ${path} returned ${result.status}`);
      return result.json() as Promise<T>;
    }
    const proof = (challenge: Challenge) => ({
      challenge_id: challenge.challenge_id, signature_payload: challenge.signature_payload,
      body_payload: challenge.body_payload,
      signature: sign(null, Buffer.from(challenge.signature_payload, "base64url"), keys.privateKey).toString("base64url"),
    });
    try {
      const config = await (await fetch(`${base}/api/config`)).json();
      expect(config.auth_mode).toBe("password");
      expect(config.managed_installation_supported).toBe(true);
      const deploymentID = String(config.deployment_id);
      workspace = await owner.ensureWorkspace("Installation acceptance", `s02-${randomUUID()}`);
      const challenge = await owner.requestJSON<Challenge>("/api/installations/challenges", { method: "POST", body: { purpose: "enroll", public_key: publicKey, desktop_version: "0.4.40", os: "macos" } });
      const envelope = JSON.parse(Buffer.from(challenge.signature_payload, "base64url").toString("utf8"));
      expect(envelope.user_id).toBe(ownerUser.id);
      expect(envelope.deployment_id).toBe(deploymentID);
      const enrolled = await owner.requestJSON<{ installation_id: string; metadata_proof: string }>("/api/installations/enroll", { method: "POST", body: proof(challenge) });
      installationID = enrolled.installation_id;
      expect(installationID).toMatch(/^[0-9a-f-]{36}$/);
      // An attribution proof is not a login credential.
      const denied = await fetch(`${base}/api/admin/me`, { headers: { Authorization: `Bearer ${enrolled.metadata_proof}` } });
      expect(denied.status).toBe(401);
      const pat = await owner.requestJSON<{ token: string }>("/api/tokens", { method: "POST", body: { name: "Isolated installation protocol", expires_in_days: 1 } });
      const daemonID = randomUUID();
      const bindingChallenge = await owner.requestJSON<Challenge>("/api/installations/challenges", { method: "POST", body: { purpose: "bind", installation_id: installationID, workspace_id: workspace.id, daemon_id: daemonID, public_key: publicKey, expected_binding_epoch: null } });
      const bound = await daemonJSON<{ binding_id: string; daemon_token: string }>("/api/daemon/installation-bindings", pat.token, proof(bindingChallenge));
      expect(bound.binding_id).toMatch(/^[0-9a-f-]{36}$/);
      if (typeof bound.daemon_token !== "string" || !bound.daemon_token.startsWith("mdt_")) throw new Error("Scoped daemon credential was not issued");
      const registered = await daemonJSON<{ runtimes: Array<{ id: string }> }>("/api/daemon/register", bound.daemon_token, {
        workspace_id: workspace.id, daemon_id: daemonID, device_name: "PRIVATE DEVICE /fixture/private", cli_version: "0.4.40", launched_by: "desktop",
        runtimes: [{ name: "PRIVATE codex", type: "codex", version: "fixture", status: "online" }, { name: "PRIVATE claude", type: "claude", version: "fixture", status: "online" }],
      });
      expect(registered.runtimes).toHaveLength(2);
      for (const runtime of registered.runtimes) await daemonJSON("/api/daemon/heartbeat", bound.daemon_token, { runtime_id: runtime.id });
      const boot = randomUUID();
      const heartbeat = async (sequence: number) => {
        const bytes = Buffer.from(JSON.stringify({ protocol_version: "1", purpose: "heartbeat", deployment_id: deploymentID, installation_id: installationID, user_id: ownerUser.id, auth_version: envelope.auth_version, boot_id: boot, sequence: String(sequence), reported_at: String(Math.floor(Date.now() / 1000)), desktop_version: "0.4.40", os: "macos", method: "POST", path: `/api/installations/${installationID}/heartbeat` }));
        const result = await owner.requestJSON<{ accepted: boolean }>(`/api/installations/${installationID}/heartbeat`, { method: "POST", body: { signature_payload: bytes.toString("base64url"), signature: sign(null, bytes, keys.privateKey).toString("base64url") } });
        expect(result.accepted).toBe(true);
      };
      await heartbeat(1);
      const legacy = await owner.seedProjectRuntime();
      const listPath = `/api/admin/installations?q=${installationID}`;
      let list = await admin.requestJSON<InstallationPage>(listPath);
      expect(list.items).toHaveLength(1);
      expect(list.items[0]?.runtime_count).toBe(2);
      expect(list.items[0]?.execution_readiness.state).toBe("ready");
      expect(JSON.stringify(list)).not.toMatch(/PRIVATE|public_key|token_hash|\/fixture\/private/);
      // Simulate a closed window without waiting three minutes in the test.
      await db.query("UPDATE managed_installation SET client_seen_at=now()-interval '10 minutes' WHERE id=$1 AND responsible_user_id=$2", [installationID, ownerUser.id]);
      list = await admin.requestJSON<InstallationPage>(listPath);
      expect(list.items[0]?.client_activity.state).toBe("inactive");
      expect(list.items[0]?.daemon_reachability.state).toBe("reachable");
      expect(list.items[0]?.execution_readiness.state).toBe("ready");

      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`${web}/admin/installations?q=${installationID}`);
      await page.locator("#password-username").fill(adminName!);
      await page.locator("#password-value").fill(adminPassword!);
      await page.locator('button[type="submit"]').click();
      const table = page.getByRole("table");
      await expect(table.getByText("Inactive", { exact: true })).toBeVisible();
      await expect(table.getByText("Reachable", { exact: true })).toBeVisible();
      await expect(table.getByText("Ready to accept work", { exact: true })).toBeVisible();
      await mkdir(reportDir, { recursive: true });
      await page.screenshot({ path: resolve(reportDir, "installations-desktop.png"), fullPage: true });
      await page.setViewportSize({ width: 390, height: 844 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await page.screenshot({ path: resolve(reportDir, "installations-mobile.png"), fullPage: true });
      await table.scrollIntoViewIfNeeded();
      expect(await table.locator("tbody td").first().evaluate(cell => {
        const link = cell.querySelector("a");
        if (!link) return false;
        const range = document.createRange();
        range.selectNodeContents(link);
        return range.getBoundingClientRect().right <= cell.getBoundingClientRect().right;
      }), "Installation identifier must not overlap the next status cell").toBe(true);
      await page.screenshot({ path: resolve(reportDir, "installations-mobile-table.png"), fullPage: true });

      await heartbeat(2);
      await daemonJSON("/api/daemon/deregister", bound.daemon_token, { runtime_ids: registered.runtimes.map(runtime => runtime.id), offline_reasons: Object.fromEntries(registered.runtimes.map(runtime => [runtime.id, { code: "not_executable", detail: "PRIVATE engine fault /fixture/private" }])) });
      list = await admin.requestJSON<InstallationPage>(listPath);
      expect(list.items[0]?.client_activity.state).toBe("active");
      expect(list.items[0]?.daemon_reachability.state).toBe("reachable");
      expect(list.items[0]?.execution_readiness.state).toBe("environment_unavailable");
      const detail = await admin.requestJSON<Record<string, unknown>>(`/api/admin/installations/${installationID}`);
      expect(JSON.stringify(detail)).not.toMatch(/PRIVATE|public_key|token_hash|\/fixture\/private/);
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(`${web}/admin/installations/${installationID}`);
      await expect(page.getByText("Environment unavailable", { exact: true })).toBeVisible();
      await expect(page.getByRole("link", { name: "View executions" })).toHaveAttribute("href", `/admin/tasks?installation_id=${installationID}`);
      await page.screenshot({ path: resolve(reportDir, "installation-detail.png"), fullPage: true });
      await page.goto(`${web}/admin/installations/unassociated?workspace_id=${workspace.id}`);
      await expect(page.getByRole("table").getByText(legacy.id, { exact: true })).toBeVisible();
      for (const runtime of registered.runtimes) await expect(page.getByRole("table").getByText(runtime.id, { exact: true })).toHaveCount(0);
      await page.screenshot({ path: resolve(reportDir, "unassociated-runtimes.png"), fullPage: true });
      expect(errors).toEqual([]);
    } finally {
      try {
        if (workspace) await owner.deleteFeatureWorkspace(workspace.id);
        await db.query("BEGIN");
        const owned = await db.query<{ id: string }>("SELECT id FROM managed_installation WHERE responsible_user_id=$1 AND key_fingerprint=$2", [ownerUser.id, fingerprint]);
        for (const item of owned.rows) {
          await db.query("DELETE FROM daemon_token WHERE installation_binding_id IN (SELECT id FROM installation_daemon_binding WHERE installation_id=$1)", [item.id]);
          await db.query("DELETE FROM installation_report_cursor WHERE installation_id=$1", [item.id]);
          await db.query("DELETE FROM installation_user WHERE installation_id=$1", [item.id]);
          await db.query("DELETE FROM installation_daemon_binding WHERE installation_id=$1", [item.id]);
          await db.query("DELETE FROM admin_audit_event WHERE target_kind='installation' AND target_id=$1 AND actor_user_id=$2", [item.id, ownerUser.id]);
          await db.query("DELETE FROM managed_installation WHERE id=$1", [item.id]);
        }
        await db.query("DELETE FROM installation_challenge WHERE user_id=$1", [ownerUser.id]);
        await db.query("COMMIT");
        await owner.deletePasswordAccount(username);
      } catch (error) { await db.query("ROLLBACK"); throw error; }
      finally { await db.end(); }
    }
  });
});
