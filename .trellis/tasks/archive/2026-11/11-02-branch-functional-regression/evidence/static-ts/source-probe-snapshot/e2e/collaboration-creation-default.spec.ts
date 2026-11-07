import { randomUUID } from "node:crypto";
import { test, expect, type Page } from "@playwright/test";
import { legacyLaneMissing, loginLegacyOwner, openLegacySession, rawRequest, readCollaborationGate,
  setCollaborationGate, sql } from "./fixtures/lane-guards";

// Migration 744 makes a fresh installation open collaboration creation while
// AI execution and host admission stay closed. The entry point is the
// "New collaboration issue" button in the Issues page header
// (packages/views/collaboration/creation-form.tsx). Both tests change the
// deployment-wide gate and restore its original booleans in `finally`.
// Copy: packages/views/locales/en/collaboration.json creation.*.
const DISABLED = "Creating collaboration issues is not enabled on this server.";

async function openCreation(page: Page, slug: string) {
  await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
  await expect(dialog.getByRole("combobox", { name: "Source workspace", exact: true })).toBeVisible();
  return dialog;
}

test.describe("collaboration creation default", () => {
  test.skip(legacyLaneMissing() !== null, legacyLaneMissing() ?? "");
  test.use({ viewport: { width: 1280, height: 900 }, actionTimeout: 15_000 });

  test("at the fresh-install defaults an ordinary member creates a collaboration issue while execution stays closed", async ({ page, baseURL }) => {
    test.setTimeout(120_000);
    // The defaults come from the migrated schema, not from this spec.
    const defaults = await sql<{ column_name: string; column_default: string }>(
      `SELECT column_name, column_default FROM information_schema.columns
       WHERE table_name = 'collaboration_execution_gate' AND column_name IN ('creation_enabled', 'enabled', 'host_execution_enabled')`);
    expect(Object.fromEntries(defaults.map((item) => [item.column_name, item.column_default]))).toEqual({
      creation_enabled: "true", enabled: "false", host_execution_enabled: "false",
    });
    const restore = await setCollaborationGate(() => ({ creation_enabled: true, enabled: false, host_execution_enabled: false }));
    try {
      const member = await loginLegacyOwner("e2e-supp-cc", "Collaboration default member");
      await openLegacySession(page, baseURL!, member.token);
      const dialog = await openCreation(page, member.slug);
      await expect(dialog.getByRole("combobox", { name: "Source workspace", exact: true })).toHaveValue(member.workspace.id);
      const title = dialog.getByRole("textbox", { name: "Title", exact: true });
      await expect(title).toBeEnabled();
      await expect(dialog.getByText(DISABLED, { exact: true })).toHaveCount(0);
      await expect(dialog.getByText("Assigned to you", { exact: true })).toBeVisible();

      await title.fill(`Default-open collaboration ${randomUUID().slice(0, 8)}`);
      const created = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/collaboration/issues" && response.request().method() === "POST");
      await dialog.getByRole("button", { name: "Create collaboration issue", exact: true }).click();
      const response = await created;
      expect(response.status(), await response.text()).toBe(201);
      const issue = await response.json() as { id: string };
      await expect(page).toHaveURL((url) => url.pathname === "/account" && url.searchParams.get("view") === "participating"
        && url.searchParams.get("issue") === issue.id.toLowerCase());
      // Creation opened alone: the switches this spec set are untouched.
      expect(await readCollaborationGate()).toMatchObject({ creation_enabled: true, enabled: false, host_execution_enabled: false });
    } finally {
      await restore();
    }
  });

  test("with new collaboration issues closed the member sees the reason and the server refuses the create", async ({ page, baseURL }) => {
    test.setTimeout(120_000);
    const restore = await setCollaborationGate((gate) => ({ ...gate, creation_enabled: false }));
    try {
      const member = await loginLegacyOwner("e2e-supp-cc", "Collaboration closed member");
      await openLegacySession(page, baseURL!, member.token);
      const dialog = await openCreation(page, member.slug);
      await expect(dialog.getByText(DISABLED, { exact: true })).toBeVisible();
      await expect(dialog.getByRole("textbox", { name: "Title", exact: true })).toBeDisabled();
      await expect(dialog.getByRole("button", { name: "Create collaboration issue", exact: true })).toBeDisabled();

      const refused = await rawRequest(member.api, "/api/collaboration/issues", {
        method: "POST", headers: { "Idempotency-Key": randomUUID() }, workspaceId: member.workspace.id,
        body: { source_workspace_id: member.workspace.id, title: "Refused while closed", description: "", attachment_ids: [] },
      });
      expect(refused.status, refused.text).toBe(503);
      expect(refused.body).toMatchObject({ code: "collaboration_unavailable" });
      const [count] = await sql<{ count: string }>(
        "SELECT count(*)::text AS count FROM issue_collaboration c JOIN issue i ON i.id = c.issue_id WHERE i.workspace_id = $1", [member.workspace.id]);
      expect(count?.count).toBe("0");
    } finally {
      await restore();
    }
  });
});
