import { test, expect } from "@playwright/test";
import type { CollaborationCommentReceipt, CollaborationComments, CollaborationIssue } from "@multica/core/collaboration";
import { authenticateCollaborationResourcesPage } from "./fixtures/collaboration-resources";
import { seedCollaborationPreviewJourney } from "./fixtures/collaboration-preview";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

test("trigger preview explains shared and later batches before a suppressed target is sent", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  test.skip(!process.env.COLLABORATION_E2E_API_URL || !process.env.DATABASE_URL || !process.env.MULTICA_DEPLOYMENT_ID || !process.env.NEXT_PUBLIC_API_URL,
    "Requires an isolated API/DB and matching production Web build");
  const fixture = await seedCollaborationPreviewJourney();
  const buildSquad = fixture.squads[0]!;
  const reviewSquad = fixture.squads[1]!;
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await authenticateCollaborationResourcesPage(page, fixture.creator);
  await page.goto(`/account?view=participating&issue=${fixture.issueId}`);
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  const composer = page.getByRole("form", { name: "Comments", exact: true });
  for (const name of [fixture.agentName, buildSquad.name, reviewSquad.name]) {
    await composer.getByRole("button", { name, exact: true }).click();
  }
  const original = await composer.getByLabel("Add comment", { exact: true }).inputValue();
  await composer.getByRole("button", { name: "Preview delivery", exact: true }).click();
  const grouping = page.getByRole("region", { name: "AI grouping preview", exact: true });
  await expect(grouping.getByText(`${fixture.agentName}, ${buildSquad.name}`, { exact: true })).toBeVisible();
  await expect(grouping.getByText("Expected to merge into one squad execution.", { exact: true })).toBeVisible();
  await expect(grouping.getByText("Includes earlier waiting input.", { exact: true })).toBeVisible();
  await expect(grouping.getByText("Follows an earlier batch in the same agent's queue.", { exact: true })).toBeVisible();
  await expect(grouping.getByText("Preview only. Sending rechecks permissions and queue state.", { exact: true })).toBeVisible();
  const commentsBefore = await fixture.creator.requestJSON<CollaborationComments>(`${root}/comments`);
  expect(commentsBefore.items).toHaveLength(1);
  expect((await fixture.creator.requestJSON<CollaborationIssue>(root)).execution?.runs).toEqual([]);
  await grouping.scrollIntoViewIfNeeded();
  await page.screenshot({ path: testInfo.outputPath("preview-en-wide.png"), animations: "disabled" });
  await page.setViewportSize({ width: 390, height: 844 });
  await grouping.scrollIntoViewIfNeeded();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("preview-en-narrow.png"), animations: "disabled" });

  // Web resolves the initial locale from its cookie, then syncs the account
  // preference on login. localStorage alone only configures Desktop's locale.
  await fixture.creator.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
  await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: new URL(page.url()).origin }]);
  const chinesePage = await page.context().newPage();
  chinesePage.on("pageerror", (error) => errors.push(error.message));
  await chinesePage.setViewportSize({ width: 390, height: 844 });
  await authenticateCollaborationResourcesPage(chinesePage, fixture.creator, "zh-Hans");
  await chinesePage.goto(page.url());
  await expect(chinesePage.locator("html")).toHaveAttribute("lang", "zh-CN");
  const chineseComposer = chinesePage.getByRole("form", { name: "评论", exact: true });
  await expect(chineseComposer.getByLabel("添加评论", { exact: true })).toHaveValue(original);
  await chineseComposer.getByRole("button", { name: "预览发送", exact: true }).click();
  const chineseGrouping = chinesePage.getByRole("region", { name: "AI 分组预览", exact: true });
  await expect(chineseGrouping.getByText("预计合并为同一次小队执行。", { exact: true })).toBeVisible();
  await chineseGrouping.scrollIntoViewIfNeeded();
  await chinesePage.screenshot({ path: testInfo.outputPath("preview-zh-narrow.png"), animations: "disabled" });
  const suppressed = chineseComposer.getByRole("checkbox", { name: new RegExp(buildSquad.name) });
  await suppressed.focus();
  await suppressed.press("Space");
  await expect(chineseGrouping).toBeHidden();
  await expect(chineseComposer.getByRole("button", { name: "发送评论", exact: true })).toBeDisabled();
  await expect(chineseComposer.getByLabel("添加评论", { exact: true })).toHaveValue(original);
  await chineseComposer.getByRole("button", { name: "预览发送", exact: true }).click();
  await expect(chineseGrouping.getByText(buildSquad.name, { exact: false })).toHaveCount(0);
  await expect(chineseGrouping).toBeVisible();
  const response = chinesePage.waitForResponse((item) => item.url().endsWith(`${root}/comments`) && item.request().method() === "POST");
  await chineseComposer.getByRole("button", { name: "发送评论", exact: true }).click();
  const sent = await response;
  expect(sent.status()).toBe(201);
  const receipt: CollaborationCommentReceipt = await sent.json();
  expect(receipt.targets.find((item) => item.resource_id === buildSquad.id)?.state).toBe("suppressed");
  expect(receipt.targets.filter((item) => item.state === "waiting")).toHaveLength(2);
  await expect(chineseComposer.getByText("评论已确认。", { exact: true })).toBeVisible();
  expect((await fixture.creator.requestJSON<CollaborationIssue>(root)).execution?.runs).toEqual([]);
  expect(errors).toEqual([]);
  await testInfo.attach("preview-identities", { contentType: "application/json", body: JSON.stringify({ issue_id: fixture.issueId, target_ids: fixture.targetIds }) });
});
