import { randomUUID } from "node:crypto";
import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// The isolated check environment already pins NEXT_PUBLIC_API_URL to the
// started API. Keep the dedicated override for ad-hoc runs, but don't make a
// correctly configured production check skip the acceptance silently.
const apiUrl = process.env.COLLABORATION_E2E_API_URL ?? process.env.NEXT_PUBLIC_API_URL;

async function authenticate(page: Page, token: string) {
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
}

function picker(page: Page) {
  return page.locator('[data-recipient-picker]').first();
}

test("Web recipient picker exposes self/recent/search results and preserves the outer dialog on Escape", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  test.skip(!apiUrl || !process.env.DATABASE_URL || !process.env.NEXT_PUBLIC_API_URL,
    "Requires the isolated collaboration API/database and production Web build with creation enabled");
  for (const value of [apiUrl!, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) {
    expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  }
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);

  const suffix = randomUUID().slice(0, 10);
  const source = new TestApiClient();
  const sourceName = `Picker owner ${suffix}`;
  await source.login(`recipient-picker-source-${suffix}@example.invalid`, sourceName);
  const workspace = await source.ensureWorkspace(`Recipient picker ${suffix}`, `recipient-picker-${suffix}`);
  await source.markUserOnboarded();

  const recent = new TestApiClient();
  const recentName = `Recent collaborator ${suffix}`;
  await recent.login(`recipient-picker-recent-${suffix}@example.invalid`, recentName);
  await recent.markUserOnboarded();
  const recentId = (await recent.requestJSON<{ id: string }>("/api/me")).id;

  // Seed a real collaboration so the empty query's recent section is backed by
  // persisted issue_participant history rather than a mocked response.
  await source.requestJSON<{ id: string }>("/api/collaboration/issues", {
    method: "POST",
    headers: { "Idempotency-Key": randomUUID() },
    body: {
      source_workspace_id: workspace.id,
      handler_user_id: recentId,
      title: `Picker history ${suffix}`,
      description: "Recipient picker recent account seed",
      attachment_ids: [],
    },
  });

  await authenticate(page, source.getToken()!);
  await page.goto("/account");
  await page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
  const control = picker(page);
  await expect(control).toHaveCount(1);
  const trigger = control.locator('button[aria-label="Recipient"]');
  await expect(trigger).toBeVisible();

  // The creation picker must expose the caller's own account before any
  // network-backed result is chosen.
  const emptySearch = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === "/api/collaboration/recipients/search" && response.request().method() === "POST";
  });
  await trigger.click();
  // PropertyPicker portals its content to the dialog layer, outside the
  // trigger wrapper. Locate the live popup at page level, then scope rows to
  // that popup instead of assuming it remains under data-recipient-picker.
  const list = page.getByRole("listbox", { name: "Recipient" });
  await expect(list).toBeVisible();
  await expect(list.locator("[data-recipient-row]").filter({ hasText: "Assign to me" })).toBeVisible();

  const firstResponse = await emptySearch;
  expect(firstResponse.status()).toBe(200);
  const firstBody = await firstResponse.json() as {
    recent: Array<Record<string, unknown>>;
    recipients: Array<Record<string, unknown>>;
    next_cursor: string | null;
  };
  expect(firstBody.recent.some((account) => account.name === recentName)).toBe(true);
  for (const account of [...firstBody.recent, ...firstBody.recipients]) {
    expect(Object.keys(account).sort()).toEqual(["avatar_url", "external", "name", "user_id"]);
    expect(JSON.stringify(account)).not.toContain("@example.invalid");
  }
  await expect(list.locator("[data-recipient-row]").filter({ hasText: recentName })).toBeVisible();

  const search = page.getByRole("textbox", { name: "Filter options" });
  await expect(search).toBeVisible();
  const searched = page.waitForResponse((response) => {
    const request = response.request();
    if (new URL(response.url()).pathname !== "/api/collaboration/recipients/search" || request.method() !== "POST") return false;
    const body = request.postDataJSON() as { q?: string };
    return body.q === recentName;
  });
  await search.fill(recentName);
  const searchedResponse = await searched;
  expect(searchedResponse.status()).toBe(200);
  await expect(list.locator("[data-recipient-row]").filter({ hasText: recentName })).toHaveCount(1);
  await expect(page.getByRole("button", { name: /exact lookup|full username|email|account id/i })).toBeVisible();

  // Escape closes the picker while keeping the creation dialog mounted.
  await search.press("Escape");
  await expect(list).toBeHidden();
  await expect(dialog).toBeVisible();

  // A coarse-pointer viewport keeps the picker inside the dialog and leaves
  // the submit action reachable on the narrow layout.
  await page.setViewportSize({ width: 375, height: 812 });
  await trigger.click();
  await expect(list).toBeVisible();
  const box = await control.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.width).toBeLessThanOrEqual(375);
  await expect(dialog.getByRole("button", { name: "Create collaboration issue", exact: true })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("recipient-picker-narrow.png"), animations: "disabled" });
});
