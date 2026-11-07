import { createHmac, randomUUID } from "node:crypto";
import { spawnSync } from "node:child_process";
import pg from "pg";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Server tests own the credential-revocation and data-preservation matrices.
// These tests cover real browser routing across restricted account states.
test.describe("intranet password migration", () => {
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires isolated password-mode deployment");

  for (const flow of ["legacy setup", "temporary recovery"] as const) {
    test(`${flow} returns to the original identity and workspace`, async ({ page }, info) => {
      test.setTimeout(120_000);
      const apiBase = process.env.NEXT_PUBLIC_API_URL!;
      expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
      expect(["localhost", "127.0.0.1"]).toContain(new URL(process.env.DATABASE_URL!).hostname);
      expect((await (await page.request.get(`${apiBase}/api/config`)).json()).auth_mode).toBe("password");
      const username = `migration${randomUUID().replaceAll("-", "").slice(0, 16)}`;
      const initialPassword = "initial acceptance password 2026";
      const finalPassword = "final acceptance password 2026";
      const temporaryPassword = "temporary acceptance password 2026";
      const api = new TestApiClient();
      const database = new pg.Client(process.env.DATABASE_URL);
      await database.connect();
      let userID: string | undefined;
      let workspace: { id: string; slug: string } | undefined;
      let savedCredential: { password_hash: string } | undefined;
      const events: { time: number; method?: string; path: string; status?: number }[] = [];
      page.on("response", response => {
        const path = new URL(response.url()).pathname;
        if (path.startsWith("/api/") || path.startsWith("/auth/")) events.push({ time: Date.now(), method: response.request().method(), path, status: response.status() });
      });
      page.on("framenavigated", frame => {
        if (frame === page.mainFrame()) events.push({ time: Date.now(), path: new URL(frame.url()).pathname });
      });
      const errors: string[] = [];
      page.on("pageerror", error => errors.push(error.message));
      try {
        const registration = await page.request.post(`${apiBase}/auth/register`, {
          data: { username, password: initialPassword, name: "Migration acceptance" },
        });
        expect(registration.status()).toBe(201);
        const original = await api.loginPassword(username, initialPassword);
        userID = original.id;
        workspace = await api.ensureWorkspace(`Migration ${username}`, username);
        await database.query('UPDATE "user" SET onboarded_at = now() WHERE id = $1', [userID]);

        if (flow === "legacy setup") {
          expect(process.env.JWT_SECRET).toBeTruthy();
          const cutoff = Date.parse(process.env.MULTICA_PASSWORD_MIGRATION_CUTOFF!);
          expect(Number.isFinite(cutoff)).toBe(true);
          const removed = await database.query<{ password_hash: string }>(
            "DELETE FROM user_password_credential WHERE user_id = $1 RETURNING password_hash", [userID],
          );
          savedCredential = removed.rows[0];
          const header = Buffer.from(JSON.stringify({ alg: "HS256", typ: "JWT" })).toString("base64url");
          const claims = Buffer.from(JSON.stringify({ sub: userID, iat: Math.floor(cutoff / 1000) - 60, exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64url");
          const payload = `${header}.${claims}`;
          const token = `${payload}.${createHmac("sha256", process.env.JWT_SECRET!).update(payload).digest("base64url")}`;
          await page.addInitScript(token => {
            // Seed once: reload must retain the newly issued session.
            if (!sessionStorage.getItem("legacy-seeded")) {
              localStorage.setItem("multica_token", token);
              sessionStorage.setItem("legacy-seeded", "1");
            }
          }, token);
          await page.goto("/login");
          await expect(page.locator("#password-name")).toBeVisible();
          await page.locator("#password-username").fill(username);
          await page.locator("#password-name").fill("Migrated acceptance");
        } else {
          expect(process.env.E2E_PASSWORD_SERVER_BINARY).toBeTruthy();
          const recovery = spawnSync(process.env.E2E_PASSWORD_SERVER_BINARY!, ["password-recover", "--user", userID, "--reason", "Isolated browser password recovery acceptance"], {
            input: `${temporaryPassword}\n`, encoding: "utf8", timeout: 30_000,
            env: { ...process.env, MULTICA_AUTH_MODE: "password", MULTICA_PASSWORD_LIMITER_MODE: "single" },
          });
          expect(recovery.status, recovery.stderr).toBe(0);
          await page.goto("/login");
          await page.locator("#password-username").fill(username);
          await page.locator("#password-value").fill(temporaryPassword);
          await page.locator('button[type="submit"]').click();
          await expect(page.locator("#password-current")).toBeVisible();
          await expect(page.locator("#password-username")).toHaveCount(0);
          await page.reload();
          await expect(page.locator("#password-current")).toBeVisible();
          await page.locator("#password-current").fill(temporaryPassword);
        }

        await page.locator("#password-value").fill(finalPassword);
        const endpoint = flow === "legacy setup" ? "setup" : "change";
        const completed = page.waitForResponse(response => response.url().endsWith(`/api/me/password/${endpoint}`) && response.request().method() === "POST");
        await page.locator('button[type="submit"]').click();
        const response = await completed;
        expect(response.status()).toBe(200);
        expect((await response.json()).user.id).toBe(userID);
        await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/`), { timeout: 30_000 });
        await page.reload();
        await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/`));
        await expect(page.locator("#password-value")).toHaveCount(0);
        expect((await api.loginPassword(username, finalPassword)).id).toBe(userID);
        expect((await api.getWorkspaces()).map(item => item.id)).toContain(workspace.id);
        expect(errors).toEqual([]);
      } finally {
        await info.attach("auth-request-order", { body: JSON.stringify(events, null, 2), contentType: "application/json" });
        if (userID) {
          // Restore only this test's credential if setup failed before binding.
          if (savedCredential) await database.query(
            "INSERT INTO user_password_credential (user_id, username, password_hash) VALUES ($1, $2, $3) ON CONFLICT (user_id) DO NOTHING",
            [userID, username, savedCredential.password_hash],
          );
          await database.query("UPDATE user_password_credential SET must_change_password = false WHERE user_id = $1", [userID]);
          // Recovery or binding may revoke the fixture client's earlier token.
          try { await api.loginPassword(username, finalPassword); }
          catch {
            try { await api.loginPassword(username, temporaryPassword); }
            catch { await api.loginPassword(username, initialPassword); }
          }
          if (workspace) await api.deleteFeatureWorkspace(workspace.id);
          await api.deletePasswordAccount(username);
        }
        await database.end();
      }
    });
  }
});
