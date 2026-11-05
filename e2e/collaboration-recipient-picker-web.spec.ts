import { randomUUID } from "node:crypto";
import { test, expect, type Page, type Request, type Route } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const apiUrl = process.env.COLLABORATION_E2E_API_URL;

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
  const sourceId = (await source.requestJSON<{ id: string }>("/api/me")).id;

  const recent = new TestApiClient();
  const recentName = `Recent collaborator ${suffix}`;
  await recent.login(`recipient-picker-recent-${suffix}@example.invalid`, recentName);
  await recent.markUserOnboarded();
  const recentId = (await recent.requestJSON<{ id: string }>("/api/me")).id;
  expect(recentId).not.toBe(sourceId);

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
  const title = `Picker assigned issue ${suffix}`;
  await dialog.getByRole("textbox", { name: "Title", exact: true }).fill(title);
  await expect(dialog.getByText("Assigned to you", { exact: true })).toBeVisible();
  await expect(trigger).toContainText(`Account ID: ${sourceId}`);
  const createRequests: Request[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/api/collaboration/issues" && request.method() === "POST") createRequests.push(request);
  });

  // The creation picker exposes the localized self shortcut before any
  // network-backed result is chosen.
  const emptySearch = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === "/api/collaboration/recipients/search" && response.request().method() === "POST";
  });
  await trigger.click();
  // PopoverContent portals outside the trigger's container.
  const list = page.getByRole("listbox", { name: "Recipient", exact: true });
  const popup = page.locator('[data-slot="popover-content"]').filter({ has: list });
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

  const search = popup.getByRole("textbox", { name: "Filter options", exact: true });
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
  await expect(popup.getByRole("button", { name: "Find by full username, email, or account ID", exact: true })).toBeVisible();

  // Escape closes the picker while keeping the creation dialog mounted.
  await search.press("Escape");
  await expect(list).toBeHidden();
  await expect(dialog).toBeVisible();

  // A narrow viewport keeps the portalled picker within the screen and leaves
  // the submit action reachable.
  await page.setViewportSize({ width: 375, height: 812 });
  await trigger.click();
  await expect(list).toBeVisible();
  const box = await popup.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(375);
  expect(box!.width).toBeLessThanOrEqual(375);
  await expect(dialog.getByRole("button", { name: "Create collaboration issue", exact: true })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("recipient-picker-narrow.png"), animations: "disabled" });

  // Start with an active self row, then hold an uncached name search. The
  // real API response is delivered unchanged only after Enter is exercised.
  await expect(search).toHaveValue("");
  await expect(list).toHaveAttribute("aria-busy", "false");
  const selfOption = list.getByRole("option", { name: `Assign to me, Account ID: ${sourceId}`, exact: true });
  await expect(selfOption).toHaveAttribute("aria-selected", "true");
  if (await search.getAttribute("aria-activedescendant") === null) await search.press("ArrowDown");
  await expect(search).toHaveAttribute("aria-activedescendant", (await selfOption.getAttribute("id"))!);

  const currentQuery = `collaborator ${suffix}`;
  const searchPattern = (url: URL) => url.pathname === "/api/collaboration/recipients/search";
  let releaseSearch!: () => void;
  const released = new Promise<void>((resolve) => { releaseSearch = resolve; });
  let responseHeld = false;
  let delivery: Promise<void> | undefined;
  const holdSearch = (route: Route) => {
    const request = route.request();
    if (request.method() !== "POST" || request.postDataJSON().q !== currentQuery || delivery) return route.continue();
    delivery = (async () => {
      const response = await route.fetch();
      responseHeld = true;
      await released;
      await route.fulfill({ response });
    })();
    return delivery;
  };
  await page.route(searchPattern, holdSearch);
  try {
    await search.fill(currentQuery);
    await expect.poll(() => responseHeld).toBe(true);
    await expect(list).toHaveAttribute("aria-busy", "true");
    await expect(popup.getByRole("status")).toHaveText("Loading accounts...");
    await search.press("Enter");
    await expect(popup).toBeVisible();
    await expect(search).toBeVisible();
    await expect(list).toBeAttached();
    await expect(list).toHaveAttribute("aria-busy", "true");
    await expect(dialog).toBeVisible();
    await expect(search).not.toHaveAttribute("aria-activedescendant");
    await expect(list.getByRole("option", { disabled: false })).toHaveCount(0);
    await expect(trigger).toContainText(`Account ID: ${sourceId}`);
    await expect(dialog.getByText("Assigned to you", { exact: true })).toBeVisible();
    expect(createRequests).toHaveLength(0);

    const currentSearch = page.waitForResponse((response) => searchPattern(new URL(response.url()))
      && response.request().method() === "POST" && response.request().postDataJSON().q === currentQuery);
    releaseSearch();
    const currentResponse = await currentSearch;
    expect(currentResponse.status()).toBe(200);
    const currentBody = await currentResponse.json() as typeof firstBody;
    expect(currentBody.recipients).toEqual([expect.objectContaining({ user_id: recentId, name: recentName, external: true })]);
    await expect(list).toHaveAttribute("aria-busy", "false");
    const recipientOption = list.getByRole("option", { name: `${recentName}, Account ID: ${recentId}, External`, exact: true });
    await expect(recipientOption).toHaveCount(1);
    await expect(recipientOption).toBeEnabled();
    await recipientOption.click();
    await expect(list).toBeHidden();
    await expect(trigger).toContainText(`Account ID: ${recentId}`);
    await expect(trigger).not.toContainText(`Account ID: ${sourceId}`);
    await expect(dialog.getByText(`Assigned to ${recentName}`, { exact: true })).toBeVisible();
  } finally {
    releaseSearch();
    try { await delivery; } finally { await page.unroute(searchPattern, holdSearch); }
  }

  // Choosing a recipient is not submission. Only the explicit create action
  // may write, and both accounts must read the same persisted handler identity.
  expect(createRequests).toHaveLength(0);
  const [createdResponse] = await Promise.all([
    page.waitForResponse((response) => new URL(response.url()).pathname === "/api/collaboration/issues" && response.request().method() === "POST"),
    dialog.getByRole("button", { name: "Create collaboration issue", exact: true }).click(),
  ]);
  expect(createdResponse.status()).toBe(201);
  expect(createdResponse.request().postDataJSON()).toMatchObject({ source_workspace_id: workspace.id, handler_user_id: recentId, title });
  const issue = await createdResponse.json() as { id: string; handler: { user_id: string } };
  const expectedIssue = { title, source_workspace: { id: workspace.id }, creator: { user_id: sourceId }, handler: { user_id: recentId } };
  expect(issue).toMatchObject(expectedIssue);
  expect(issue.handler.user_id).not.toBe(sourceId);
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL((url) => url.pathname === "/account" && url.searchParams.get("view") === "participating"
    && url.searchParams.get("issue") === issue.id.toLowerCase());
  await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
  for (const account of [source, recent]) {
    expect(await account.requestJSON(`/api/collaboration/issues/${issue.id}`)).toMatchObject({ id: issue.id, ...expectedIssue });
  }
  expect(createRequests).toHaveLength(1);
});
