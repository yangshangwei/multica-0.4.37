import { randomUUID } from "node:crypto";
import { test, expect, type Locator, type Request } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { loginAdministrationWeb } from "./fixtures/collaboration-administration";
import { adminLaneMissing, rawRequest, readCollaborationGate, setCollaborationGate, sql } from "./fixtures/lane-guards";

// Admin Settings > Collaboration (packages/views/admin/settings/settings-page.tsx)
// writes POST /api/admin/collaboration/gate with a reason and an Idempotency-Key
// (server/internal/service/admin_collaboration_gate.go). The spec starts from
// creation open, closes it and reopens it through the real form, checks the
// receipt, the audit trail and the gate row, and proves an ordinary member's
// create path follows. The original booleans are restored in `finally`.
// Copy: packages/views/locales/en/admin.json settings.gate and operations.results.

// Authentication request bodies must never enter browser trace artifacts.
test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off", viewport: { width: 1380, height: 1000 } });

const row = (region: Locator, label: string) =>
  region.locator(`xpath=.//dt[normalize-space(.)="${label}"]/following-sibling::dd[1]`);
const isGateWrite = (request: Request) =>
  request.method() === "POST" && new URL(request.url()).pathname === "/api/admin/collaboration/gate";

test("a super administrator closes and reopens new collaboration issues from Settings and members follow", async ({ page, baseURL }) => {
  test.skip(adminLaneMissing() !== null, adminLaneMissing() ?? "");
  test.setTimeout(150_000);
  const admin = new TestApiClient();
  const adminAccount = await admin.loginPassword(process.env.E2E_PLATFORM_ADMIN_USERNAME!, process.env.E2E_PLATFORM_ADMIN_PASSWORD!);
  const identity = await admin.requestJSON<{ user_id: string; organization_id: string; role: string }>("/api/admin/me");
  expect(identity).toMatchObject({ user_id: adminAccount.id, role: "super_admin" });
  const suffix = randomUUID().replaceAll("-", "").slice(0, 16);
  const member = new TestApiClient();
  await member.registerPassword(`sgate${suffix}`, `Gate-${randomUUID()}!`, "Gate switch member");
  const workspace = await member.ensureWorkspace("Gate switch member workspace", `sgate-${suffix}`);
  const memberCreate = (title: string) => rawRequest(member, "/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, workspaceId: workspace.id,
    body: { source_workspace_id: workspace.id, title, description: "Synthetic gate-switch acceptance", attachment_ids: [] },
  });
  const creationOpen = async () => (await member.requestJSON<{ creation_enabled: boolean }>("/api/collaboration/creation-options")).creation_enabled;

  // A known start: creation open, execution and host admission as they were.
  const restore = await setCollaborationGate((gate) => ({ ...gate, creation_enabled: true }));
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    const start = await readCollaborationGate();
    expect(await creationOpen()).toBe(true);
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: baseURL! }]);
    await page.addInitScript(() => {
      localStorage.setItem("multica-locale", "en");
      localStorage.setItem("multica:chat:isOpen", "false");
    });
    await loginAdministrationWeb(page, new URL("/admin/settings", baseURL!).href);

    const region = page.getByRole("region", { name: "Collaboration", exact: true });
    await expect(row(region, "New collaboration issues")).toHaveText("Open");
    await expect(row(region, "AI execution")).toHaveText(start.enabled ? "Open" : "Closed");
    await expect(row(region, "Execution mode")).toHaveText(start.host_execution_enabled ? "Standard host execution" : "Isolated Linux execution");
    if (!start.enabled) {
      await expect(region.getByText("Members can create collaboration issues, but agents will not run on them until AI execution is open.", { exact: true })).toBeVisible();
    }
    // The page offers exactly what the server lists; closing execution is never offered.
    const view = await admin.requestJSON<{ allowed_actions: string[] }>("/api/admin/collaboration/gate");
    const labels: Record<string, string> = { close_creation: "Close new collaboration issues", open_execution: "Open AI execution" };
    expect(view.allowed_actions).toContain("close_creation");
    await expect(region.getByRole("button")).toHaveText(view.allowed_actions.map((action) => labels[action] ?? action));

    await region.getByRole("button", { name: "Close new collaboration issues", exact: true }).click();
    await expect(region.getByRole("heading", { name: "Close new collaboration issues", exact: true })).toBeVisible();
    await expect(region.getByText("Stops new collaboration issues. Existing issues, comments and execution are unchanged.", { exact: true })).toBeVisible();
    await expect(region.getByText("Collaboration switches for this server", { exact: true })).toBeVisible();
    const closeReason = `Supplemental acceptance close ${suffix}`;
    await region.getByLabel("Reason", { exact: true }).fill(closeReason);
    const closeWrite = page.waitForRequest(isGateWrite);
    const closed = page.waitForResponse((response) => isGateWrite(response.request()));
    await region.getByRole("button", { name: "Confirm operation", exact: true }).click();
    const closeRequest = await closeWrite;
    expect((await closed).status()).toBe(200);
    expect(closeRequest.postDataJSON()).toEqual({ action: "close_creation", reason: closeReason });
    const closeKey = closeRequest.headers()["idempotency-key"];
    expect(closeKey).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
    await expect(region.getByText("This request closed new collaboration issues on this server. Existing issues were unchanged.", { exact: true })).toBeVisible();

    // Creation does not move the execution epoch or any other switch.
    expect(await readCollaborationGate()).toEqual({ ...start, creation_enabled: false });
    const [receipt] = await sql<{ id: string; actor_id: string; organization_id: string; target_kind: string; kind: string; state: string; result_code: string; reason: string }>(
      "SELECT id, actor_id, organization_id, target_kind, kind, state, result_code, reason FROM admin_operation WHERE idempotency_key = $1", [closeKey]);
    expect(receipt).toMatchObject({ actor_id: adminAccount.id, organization_id: identity.organization_id, target_kind: "collaboration_gate",
      kind: "collaboration.gate", state: "succeeded", result_code: "collaboration_creation_closed", reason: closeReason });
    const audit = await sql<{ phase: string; before_state: Record<string, unknown>; after_state: Record<string, unknown> }>(
      "SELECT phase, before_state, after_state FROM admin_audit_event WHERE operation_id = $1 ORDER BY phase", [receipt!.id]);
    expect(audit.map((event) => event.phase)).toEqual(["applied", "request", "succeeded"]);
    for (const event of audit) {
      expect(event.before_state).toMatchObject({ creation_enabled: true, execution_epoch: start.epoch });
      expect(event.after_state).toMatchObject({ creation_enabled: false, execution_epoch: start.epoch });
    }

    // The ordinary member's create path follows the switch.
    expect(await creationOpen()).toBe(false);
    const refused = await memberCreate(`Refused while closed ${suffix}`);
    expect(refused.status, refused.text).toBe(503);
    expect(refused.body).toMatchObject({ code: "collaboration_unavailable" });

    await region.getByRole("button", { name: "Close", exact: true }).click();
    await expect(row(region, "New collaboration issues")).toHaveText("Closed", { timeout: 15_000 });
    await region.getByRole("button", { name: "Open new collaboration issues", exact: true }).click();
    await expect(region.getByText("Members can create collaboration issues from their workspaces and share them with confirmed accounts.", { exact: true })).toBeVisible();
    const openReason = `Supplemental acceptance reopen ${suffix}`;
    await region.getByLabel("Reason", { exact: true }).fill(openReason);
    const openWrite = page.waitForRequest(isGateWrite);
    const opened = page.waitForResponse((response) => isGateWrite(response.request()));
    await region.getByRole("button", { name: "Confirm operation", exact: true }).click();
    const openRequest = await openWrite;
    expect((await opened).status()).toBe(200);
    expect(openRequest.postDataJSON()).toEqual({ action: "open_creation", reason: openReason });
    expect(openRequest.headers()["idempotency-key"]).not.toBe(closeKey);
    await expect(region.getByText("This request opened new collaboration issues on this server.", { exact: true })).toBeVisible();
    expect(await readCollaborationGate()).toEqual(start);
    const [reopened] = await sql<{ state: string; result_code: string }>(
      "SELECT state, result_code FROM admin_operation WHERE idempotency_key = $1", [openRequest.headers()["idempotency-key"]]);
    expect(reopened).toEqual({ state: "succeeded", result_code: "collaboration_creation_opened" });

    expect(await creationOpen()).toBe(true);
    const accepted = await memberCreate(`Accepted after reopen ${suffix}`);
    expect(accepted.status, accepted.text).toBe(201);
    expect(accepted.body?.id).toEqual(expect.any(String));
    await region.getByRole("button", { name: "Close", exact: true }).click();
    await expect(row(region, "New collaboration issues")).toHaveText("Open", { timeout: 15_000 });
    expect(errors).toEqual([]);
  } finally {
    await restore();
  }
});
