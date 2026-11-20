import { randomUUID } from "node:crypto";
import { expect } from "@playwright/test";
import { authenticate, test, type Issue } from "./fixtures/core-journeys";

test.use({ locale: "en-US", viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure", screenshot: "only-on-failure" });

test("canonical issue URLs preserve a status picker opened before normalization completes", async ({ page, owner }, testInfo) => {
  const issue = await owner.api.createIssue(`Canonical picker ${randomUUID().slice(0, 6)}`, { status: "todo" }) as Issue;
  const other = await owner.api.createIssue(`Different issue ${randomUUID().slice(0, 6)}`, { status: "todo" }) as Issue;
  const canonicalPath = `/${owner.workspace.slug}/issues/${issue.identifier}`;
  const queryAndHash = "?from=inbox#description";
  let releaseNormalization!: () => void;
  const normalization = new Promise<void>((resolve) => { releaseNormalization = resolve; });
  let heldNavigations = 0;
  await page.route((url) => url.pathname === canonicalPath, async (route) => {
    heldNavigations += 1;
    await normalization;
    await route.continue();
  });
  await authenticate(page, owner);
  try {
    await page.goto(`/${owner.workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "Board", exact: true })).toBeVisible();
    await page.goto(`/${owner.workspace.slug}/issues/${issue.id}${queryAndHash}`, { waitUntil: "domcontentloaded" });
    const status = page.getByRole("button", { name: "Todo", exact: true });
    await expect(status).toBeVisible();
    const originalTrigger = await status.elementHandle();
    await status.click();
    const choice = page.locator("button[data-picker-item]").filter({ hasText: /^In Progress$/ });
    await expect(choice).toBeVisible();
    await testInfo.attach("normalization-before-release", { body: JSON.stringify({ url: page.url(), heldNavigations }), contentType: "application/json" });
    releaseNormalization();
    await expect.poll(() => new URL(page.url()).pathname).toBe(canonicalPath);
    expect(new URL(page.url()).search + new URL(page.url()).hash).toBe(queryAndHash);
    expect(await originalTrigger!.evaluate((element) => element.isConnected)).toBe(true);
    await expect(choice).toBeVisible();
    let writes = 0;
    page.on("request", (request) => {
      if (request.method() === "PUT" && new URL(request.url()).pathname === `/api/issues/${issue.id}`) writes += 1;
    });
    const updated = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/issues/${issue.id}` && response.request().method() === "PUT");
    await choice.click();
    expect((await updated).status()).toBe(200);
    await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
    expect(await owner.api.requestJSON<Issue>(`/api/issues/${issue.id}`)).toMatchObject({ id: issue.id, status: "in_progress" });
    expect(writes).toBe(1);
    // Exercise the retained UUID tree's history before any document reload.
    await page.goBack({ waitUntil: "domcontentloaded" });
    await expect.poll(() => new URL(page.url()).pathname).toBe(`/${owner.workspace.slug}/issues`);
    await page.goForward({ waitUntil: "domcontentloaded" });
    await expect.poll(() => new URL(page.url()).pathname).toBe(canonicalPath);
    await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();

    // Ordinary AppLink navigation still changes resource identity after alias
    // normalization, and returning through history restores the original issue.
    await page.getByRole("link", { name: "Workspace issues", exact: true }).click();
    await expect.poll(() => new URL(page.url()).pathname).toBe(`/${owner.workspace.slug}/issues`);
    await page.getByRole("link").filter({ hasText: other.title }).first().click();
    await expect(page.getByRole("button", { name: other.title, exact: true })).toBeVisible();
    await expect.poll(() => new URL(page.url()).pathname).toBe(`/${owner.workspace.slug}/issues/${other.identifier}`);
    await page.goBack({ waitUntil: "domcontentloaded" });
    await expect.poll(() => new URL(page.url()).pathname).toBe(`/${owner.workspace.slug}/issues`);
    await page.goBack({ waitUntil: "domcontentloaded" });
    await expect.poll(() => new URL(page.url()).pathname).toBe(canonicalPath);
    await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
    expect(await owner.api.requestJSON<Issue>(`/api/issues/${other.id}`)).toMatchObject({ id: other.id, status: "todo" });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
    expect(writes).toBe(1);
  } finally {
    releaseNormalization();
    await page.unrouteAll({ behavior: "wait" });
  }
});
