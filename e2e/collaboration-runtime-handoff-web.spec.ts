import { expect, test, type Request, type Response } from "@playwright/test";
import { assertRuntimeHandoff, readCommentPage, recoverLongComment, removeRuntimeRecipient, seedRuntimeHandoff } from "./fixtures/collaboration-runtime-handoff";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "only-on-failure" });

test("Web recipient recalls retained handoff sources and omitted comments, then loses private reads on removal", async ({ page }, info) => {
  test.setTimeout(180_000);
  const fixture = await seedRuntimeHandoff({ olderComments: 54 });
  const errors: string[] = [];
  const runtimeRequests: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  page.on("request", request => { if (new URL(request.url()).pathname.startsWith("/api/collaboration/runtime/")) runtimeRequests.push(new URL(request.url()).pathname); });
  await fixture.recipient.authenticateWeb(page);
  await page.goto(fixture.path);
  const { handoff } = await assertRuntimeHandoff(page, fixture);
  await info.attach("web-runtime-handoff", { body: await handoff.screenshot({ animations: "disabled" }), contentType: "image/png" });

  const first = await readCommentPage(fixture);
  expect(first).toMatchObject({ total: 55, returned: 1, truncated: true });
  expect(first.items[0]).toMatchObject({ id: fixture.longCommentId, content: "", content_state: "omitted", trust: "data_only" });
  const { comments } = await recoverLongComment(page, fixture);
  await comments.getByRole("button", { name: "Load more", exact: true }).click();
  await expect(comments.getByText("Loaded 51 of 55 comments", { exact: true })).toBeVisible();
  await comments.getByRole("button", { name: "Load more", exact: true }).click();
  await expect(comments.getByText("Loaded 55 of 55 comments", { exact: true })).toBeVisible();
  await expect(comments.getByRole("button", { name: "Load more", exact: true })).toHaveCount(0);
  for (const id of [fixture.olderCommentIds[0]!, fixture.olderCommentIds.at(-1)!, fixture.longCommentId]) {
    await expect(page.locator(`[id="comment-${id}"]`)).toHaveCount(1);
  }
  await expect(page.locator(`[id="comment-${fixture.longCommentId}"]`).getByText(fixture.longContent, { exact: true })).toBeVisible();

  const sourceLink = handoff.getByRole("link", { name: "Open source run", exact: true });
  const sourceHref = await sourceLink.getAttribute("href");
  expect(sourceHref).toBe(`${fixture.accountPath}#run-${fixture.priorRunId}`);
  const sourceRecordsPath = `${fixture.root}/runs/${fixture.priorRunId}/records`;
  const recordsRequests: string[] = [];
  const recordsResponses: Promise<{ path: string; status: number; body: string; truncated: boolean }>[] = [];
  const observeRequest = (request: Request) => {
    const url = new URL(request.url());
    if (request.method() === "GET" && url.pathname === sourceRecordsPath) recordsRequests.push(url.pathname + url.search);
  };
  const observeResponse = (response: Response) => {
    const url = new URL(response.url());
    if (response.request().method() !== "GET" || url.pathname !== sourceRecordsPath) return;
    recordsResponses.push(response.text().then(body => ({ path: url.pathname + url.search, status: response.status(),
      body: body.replace(/\b(?:mcb|mct|mdt|mat)_[A-Za-z0-9._-]+/g, "[redacted]")
        .replace(/\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g, "[redacted]").slice(0, 16_384),
      truncated: body.length > 16_384 }), () => ({ path: url.pathname + url.search, status: response.status(), body: "[body unavailable]", truncated: false })));
  };
  page.on("request", observeRequest);
  page.on("response", observeResponse);
  await sourceLink.click();
  const targetRun = page.locator(`[id="run-${fixture.priorRunId}"]`);
  const prior = targetRun.getByRole("region", { name: "Run records", exact: true });
  await expect.soft(page).toHaveURL(new URL(`${fixture.accountPath}#run-${fixture.priorRunId}`, page.url()).href);
  await expect.soft(targetRun.getByRole("button", { name: `Hide records for ${fixture.priorRunId}`, exact: true })).toHaveAttribute("aria-expanded", "true");
  await expect.soft(prior.getByText("Recording range was not recorded.", { exact: true })).toBeVisible();
  page.off("request", observeRequest);
  page.off("response", observeResponse);
  const loadedRecordResponses = await Promise.all(recordsResponses);
  await info.attach("source-run-navigation", { contentType: "application/json", body: JSON.stringify({
    requested_href: sourceHref, observed_url: page.url(), issue_id: fixture.issueId, source_run_id: fixture.priorRunId,
    target_run_outer_html: await targetRun.evaluateAll(elements => elements.map(element => element.outerHTML.slice(0, 32_768))),
    open_record_regions: await page.locator('section[id^="records-"]').evaluateAll(elements => elements.map(element => element.id)),
    records_requests: recordsRequests, records_responses: loadedRecordResponses,
  }) });
  await info.attach("source-run-navigation-screen", { body: await page.screenshot({ animations: "disabled" }), contentType: "image/png" });
  expect(loadedRecordResponses.length).toBeGreaterThan(0);
  const sourceRecords = loadedRecordResponses.at(-1)!;
  expect(sourceRecords).toMatchObject({ status: 200, truncated: false });
  // This server omits legacy retention metadata; its explicit recording status
  // and the seeded source history establish missing process coverage.
  expect(JSON.parse(sourceRecords.body)).toMatchObject({ issue_id: fixture.issueId, run: { id: fixture.priorRunId },
    recording: { status: "not_recorded", inputs_recorded: true } });
  // Removal is an independent acceptance fact. A failed source-link assertion
  // remains a test failure while this explicit navigation permits its coverage.
  await page.goto(fixture.path);
  await assertRuntimeHandoff(page, fixture);
  await removeRuntimeRecipient(fixture);
  await expect(page.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "Handoff context", exact: true })).toHaveCount(0);
  await expect(page.getByText(fixture.priorResult, { exact: true })).toHaveCount(0);
  const denied = await fetch(`${fixture.apiUrl}${fixture.handoffPath}`, { headers: { Authorization: `Bearer ${fixture.recipient.client.getToken()!}` } });
  expect(denied.status).toBe(404);
  const retained = await fixture.creator.client.requestJSON<{ run_id: string }>(fixture.handoffPath);
  expect(retained.run_id).toBe(fixture.runId);
  expect(runtimeRequests).toEqual([]);
  expect(errors).toEqual([]);
  await info.attach("runtime-handoff-web-evidence", { contentType: "application/json", body: JSON.stringify({
    issue_id: fixture.issueId, run_id: fixture.runId, prior_run_id: fixture.priorRunId, handoff: fixture.handoff,
    recording: fixture.records.recording, long_comment_id: fixture.longCommentId, long_comment_bytes: Buffer.byteLength(fixture.longContent),
    loaded_comments: 55, removed_viewer_http_status: denied.status, source_still_readable: retained.run_id,
    runtime_requests: runtimeRequests, page_errors: errors, history_origin: "seeded retained events; ingestion verified separately in Go",
  }, null, 2) });
});
