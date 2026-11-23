import { test, expect } from "@playwright/test";
import { authenticateCollaborationResourcesPage, seedCollaborationResourceJourney, type CollaborationResourceSnapshot } from "./fixtures/collaboration-resources";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

test("resource owners register, authorize, withdraw and switch while participant management preserves authority", async ({ page, browser }, testInfo) => {
  test.setTimeout(180_000);
  test.skip(!process.env.COLLABORATION_E2E_API_URL || !process.env.DATABASE_URL || !process.env.MULTICA_DEPLOYMENT_ID || !process.env.NEXT_PUBLIC_API_URL,
    "Requires isolated API/DB, matching deployment ID and a production Web build");
  const fixture = await seedCollaborationResourceJourney();
  const errors: string[] = [];
  const headers: Record<string, string>[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => { if (request.url().includes("/api/collaboration/")) headers.push(request.headers()); });
  await authenticateCollaborationResourcesPage(page, fixture.creator);
  const path = `/account?view=participating&issue=${fixture.issueId}`;
  await page.goto(path);
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Manage AI resources", exact: true }).click();
  await page.getByRole("button", { name: "Add AI target", exact: true }).click();
  await page.getByLabel("Resource workspace", { exact: true }).selectOption(fixture.workspace.id);
  await page.getByLabel("AI target", { exact: true }).selectOption(`agent:${fixture.agentId}`);
  await page.getByRole("button", { name: "Register target", exact: true }).click();
  await expect(page.getByText("Target registered. Each owner must authorize their resources.")).toBeVisible();
  const registered = await fixture.creator.requestJSON<CollaborationResourceSnapshot>(`/api/collaboration/issues/${fixture.issueId}`);
  const target = registered.execution.targets.find((item) => item.resource_id === fixture.agentId);
  expect(target).toBeDefined();
  expect(registered.execution.runs).toEqual([]);

  const consentPath = `**/api/collaboration/issues/${fixture.issueId}/targets/${target!.id}/consents`;
  let consentKey = "";
  let consentPosts = 0;
  await page.route(consentPath, async (route) => {
    consentPosts += 1;
    consentKey = route.request().headers()["idempotency-key"] ?? "";
    const response = await route.fetch();
    expect(response.status()).toBe(201);
    await route.abort("failed");
  });
  await page.getByRole("button", { name: `Authorize ${fixture.agentName}`, exact: true }).click();
  await page.getByRole("button", { name: "Confirm authorization", exact: true }).click();
  await expect(page.getByText(/The result is not confirmed/)).toBeVisible();
  await page.unroute(consentPath);
  await page.reload();
  const recovery = page.waitForResponse((response) => response.url().endsWith(`/resources/operations/${consentKey}`) && response.request().method() === "GET");
  await page.getByRole("button", { name: "Check original result", exact: true }).click();
  expect((await recovery).status()).toBe(200);
  await expect(page.getByText("Resource authorized. Explicit resume is still required.")).toBeVisible();
  expect(consentPosts).toBe(1);
  await page.getByRole("button", { name: "Manage AI resources", exact: true }).click();
  await page.getByRole("button", { name: `Withdraw authorization for ${fixture.agentName}`, exact: true }).click();
  await page.getByRole("button", { name: "Withdraw authorization", exact: true }).click();
  await expect(page.getByText("Authorization withdrawn. This does not confirm that running work has ended.")).toBeVisible();

  const switchToggle = page.getByRole("button", { name: "Switch proposals", exact: true });
  await switchToggle.click();
  const switchId = await switchToggle.getAttribute("aria-controls");
  expect(switchId).not.toBeNull();
  const proposalPanel = page.locator(`[id="${switchId}"]`);
  await proposalPanel.getByRole("button", { name: "Propose configuration switch", exact: true }).click();
  await proposalPanel.getByLabel("Resource workspace", { exact: true }).selectOption(fixture.workspace.id);
  await proposalPanel.getByLabel("AI target", { exact: true }).selectOption(`agent:${fixture.agentId}`);
  await proposalPanel.getByRole("button", { name: "Create Switch proposal", exact: true }).click();
  await expect(page.getByText("Switch proposal created. Collect each owner's consent before applying it.")).toBeVisible();
  await proposalPanel.getByRole("button", { name: `Authorize ${fixture.agentName}`, exact: true }).click();
  await proposalPanel.getByRole("button", { name: "Confirm authorization", exact: true }).click();
  await expect(page.getByText("Proposal consent recorded. The live configuration has not changed.")).toBeVisible();
  await proposalPanel.getByRole("button", { name: "Commit Switch", exact: true }).click();
  await proposalPanel.getByRole("button", { name: "Apply configuration", exact: true }).click();
  await expect(page.getByText("Configuration switched. Explicit resume is still required.")).toBeVisible();
  const switched = await fixture.creator.requestJSON<CollaborationResourceSnapshot>(`/api/collaboration/issues/${fixture.issueId}`);
  expect(switched.execution.targets.find((item) => item.id === target!.id)?.hold_reason).toBe("switch_hold");
  expect(switched.execution.runs).toEqual([]);

  const peerPage = await browser.newPage();
  try {
    await authenticateCollaborationResourcesPage(peerPage, fixture.peer);
    await peerPage.goto(new URL(path, page.url()).href);
    await expect(peerPage.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    await peerPage.getByRole("button", { name: "Manage AI resources", exact: true }).click();
    await expect(peerPage.getByRole("heading", { name: fixture.agentName, exact: true })).toBeVisible();
    await expect(peerPage.getByRole("button", { name: "Add AI target", exact: true })).toHaveCount(0);
    await expect(peerPage.getByRole("button", { name: /^Authorize / })).toHaveCount(0);
    await peerPage.getByRole("button", { name: "Switch proposals", exact: true }).click();
    await expect(peerPage.getByRole("button", { name: "Propose configuration switch", exact: true })).toHaveCount(0);
  } finally { await peerPage.close(); }

  await page.getByRole("button", { name: "Participants", exact: true }).click();
  await expect(page.getByText("Account unavailable", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Remove Disabled resource handler", exact: true }).click();
  await expect(page.getByRole("alertdialog")).toContainText("Handling will return to the creator");
  const removal = page.waitForResponse((response) => response.request().method() === "POST" &&
    response.url().endsWith(`/api/collaboration/issues/${fixture.issueId}/participants/${fixture.disabledId}/remove`));
  await page.getByRole("button", { name: "Remove participant", exact: true }).click();
  expect((await removal).status()).toBe(200);
  await expect(page.getByRole("button", { name: "Remove Disabled resource handler", exact: true })).toHaveCount(0);
  const removed = await fixture.creator.requestJSON<CollaborationResourceSnapshot>(`/api/collaboration/issues/${fixture.issueId}`);
  expect(removed.handler.user_id).toBe(fixture.ownerId);
  expect(removed.execution.runs).toEqual([]);
  await page.locator('[data-tab-scroll-root="account-detail"]').evaluate((element) => { element.scrollTop = 0; });
  await page.screenshot({ path: testInfo.outputPath("resource-forms-wide.png"), animations: "disabled" });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("resource-forms-narrow.png"), animations: "disabled" });
  expect(headers.length).toBeGreaterThan(8);
  expect(headers.every((item) => !item["x-workspace-id"] && !item["x-workspace-slug"])).toBe(true);
  expect(errors).toEqual([]);
  await testInfo.attach("resource-journey-identities", { contentType: "application/json", body: JSON.stringify({ issue_id: fixture.issueId, runtime_id: fixture.runtimeId, agent_id: fixture.agentId, target_id: target!.id, consent_request_id: consentKey }) });
});
