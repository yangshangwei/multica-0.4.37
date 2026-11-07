import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type Page, type Route, type TestInfo } from "@playwright/test";
import type { CollaborationIssue, CollaborationKnowledgeDetail, CollaborationKnowledgeProposal } from "@multica/core/collaboration";
import { setCollaborationGate, legacyLaneMissing } from "./fixtures/lane-guards";
import { deepLink, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import { seedCollaborationRecordJourney } from "./fixtures/collaboration-records";
import { memoryAuthority } from "./fixtures/collaboration-memory";
import { revealCollaborationDisclosure } from "./fixtures/collaboration-disclosures";
import { apiApprove, apiProposal, approveKnowledge, captureKnowledge, chooseCandidate, fillKnowledge, openKnowledge,
  observeKnowledgeHints, openTarget, reviewPanel, seedKnowledge, selectKnowledgeOption, sourcePanel, submitKnowledge, targetItems, targetPanel,
  type KnowledgeFixture } from "./fixtures/collaboration-knowledge";

test.use({ actionTimeout: 15_000, trace: "on", screenshot: "only-on-failure" });
let restoreGate: (() => Promise<void>) | undefined;
test.beforeAll(async () => {
  const missing = legacyLaneMissing();
  if (missing) throw new Error(missing);
  // Knowledge execution stays disabled. Only human creation is enabled in the
  // dedicated database, with original gate values restored after the suite.
  restoreGate = await setCollaborationGate(() => ({ enabled: false, creation_enabled: true, host_execution_enabled: false }));
});
test.afterAll(async () => { await restoreGate?.(); });

const conclusion = "Use the original request identity when recovering an uncertain write.\n\n"
  + "Keep immutable versions and review the exact source before publication. ".repeat(18)
  + "\n\nLong identifier: " + "original_request_identity_".repeat(18);

async function externalProposal(page: Page, f: KnowledgeFixture) {
  const forbidden: string[] = [];
  page.on("request", r => {
    const path = new URL(r.url()).pathname;
    if (/^\/api\/(projects|members)(\/|$)/.test(path)) forbidden.push(path);
  });
  await sourcePanel(page).getByRole("button", { name: "Propose knowledge", exact: true }).click();
  const form = await fillKnowledge(page, "Recover writes with the original request", conclusion, "Comment", /^Source project:/);
  await form.getByLabel("Evidence note (optional)", { exact: true }).fill("Observed one candidate after replaying the original request in an isolated fixture.");
  await selectKnowledgeOption(page, form, "Verification claim", "Observed");
  const created = await submitKnowledge(page);
  expect(created.sent.target).toEqual({ kind: "project", workspace_id: f.workspace.id, project_id: f.project.id });
  expect(created.sent.sources).toEqual([expect.objectContaining({ kind: "comment", comment_id: f.commentId, version: "1" })]);
  expect(created.sent.evidence[0]).toMatchObject({ verification: "observed" });
  expect(forbidden).toEqual([]);
  await expect(reviewPanel(page).getByRole("button", { name: "Approve exact version", exact: true })).toHaveCount(0);
  expect((await targetItems(f.owner, f.project.id, "projects")).items).toEqual([]);
  return created;
}

async function ownerVersions(page: Page, f: KnowledgeFixture, first: Awaited<ReturnType<typeof submitKnowledge>>, info: TestInfo,
  native?: Awaited<ReturnType<typeof openDesktop>>) {
  await chooseCandidate(page, first.sent.summary);
  await captureKnowledge(page, sourcePanel(page), info, "full-source-review", native?.app, first.sent.summary);
  await approveKnowledge(page, f.owner, first.receipt.candidate_id);
  const old = (await f.owner.requestJSON<CollaborationKnowledgeDetail>(`/api/collaboration/knowledge-candidates/${first.receipt.candidate_id}`)).candidate;
  expect(old.state).toBe("approved");
  await reviewPanel(page).getByRole("button", { name: "Propose correction", exact: true }).click();
  await expect(page.getByLabel("Conclusion (Markdown)", { exact: true })).toHaveValue("");
  await fillKnowledge(page, "Corrected recovery decision", "A confirmed original receipt closes recovery. A new intent requires a new request identity.", "Shared context");
  const corrected = await submitKnowledge(page);
  expect(corrected.sent.supersedes_candidate_id).toBe(first.receipt.candidate_id);
  expect(corrected.receipt.version).toBe("2");
  expect((await targetItems(f.owner, f.project.id, "projects")).items.map(c => c.candidate_id)).toContain(first.receipt.candidate_id);
  await approveKnowledge(page, f.owner, corrected.receipt.candidate_id);
  expect((await f.owner.requestJSON<CollaborationKnowledgeDetail>(`/api/collaboration/knowledge-candidates/${first.receipt.candidate_id}`)).candidate.state).toBe("superseded");
  await reviewPanel(page).getByRole("button", { name: "Propose workspace version", exact: true }).click();
  await fillKnowledge(page, "Workspace recovery guidance", "Check the original result before attempting a new write.", "Shared context");
  const derived = await submitKnowledge(page);
  expect(derived.sent.derived_from_candidate_id).toBe(corrected.receipt.candidate_id);
  expect(derived.sent.target).toEqual({ kind: "workspace", workspace_id: f.workspace.id });
  await approveKnowledge(page, f.owner, derived.receipt.candidate_id);

  await openTarget(page, f, "project", native?.app);
  await chooseCandidate(page, corrected.sent.summary, targetPanel(page));
  await captureKnowledge(page, targetPanel(page), info, "project-managed", native?.app, corrected.sent.summary);
  expect(await f.owner.requestJSON(`/api/projects/${f.project.id}`)).toMatchObject({ description: f.base });
  await openTarget(page, f, "workspace", native?.app);
  await chooseCandidate(page, derived.sent.summary, targetPanel(page));
  await captureKnowledge(page, targetPanel(page), info, "workspace-managed", native?.app, derived.sent.summary);
  await expect(page.getByRole("textbox", { name: "Context", exact: true })).toHaveValue(f.base);
  await info.attach("version-journey", { contentType: "application/json", body: JSON.stringify({
    issue_id: f.issue.id, external_handler: f.handlerId, source_workspace: f.workspace.id, project: f.project.id,
    first: first.receipt, correction: corrected.receipt, derived: derived.receipt, private_catalog_requests: 0,
    base_context_unchanged: true, provider_account_access: false, knowledge_execution_enabled: false,
  }, null, 2) });
}

test("Web external comment proposal, exact human review, correction and workspace derivation", async ({ page, browser }, info) => {
  test.setTimeout(240_000);
  const f = await seedKnowledge();
  const errors: string[] = [];
  const context = await browser.newContext();
  try {
    const external = await context.newPage();
    external.on("pageerror", e => errors.push(e.message));
    await openKnowledge(external, f.handler, f.path);
    const first = await externalProposal(external, f);
    await openKnowledge(page, f.owner, f.path);
    page.on("pageerror", e => errors.push(e.message));
    await ownerVersions(page, f, first, info);
    expect(errors).toEqual([]);
  } finally { await context.close(); }
});

test("Web required memory purge clears open source and managed target material", async ({ page, browser }, info) => {
  test.setTimeout(120_000);
  const f = await seedKnowledge();
  await openKnowledge(page, f.handler, f.path);
  await sourcePanel(page).getByRole("button", { name: "Propose knowledge", exact: true }).click();
  await fillKnowledge(page, "Private memory-derived conclusion", "PURGE_SENTINEL_must_disappear_from_every_projection", "Shared context");
  const submitted = await submitKnowledge(page);
  const created = { receipt: submitted.receipt, body: submitted.sent };
  expect(created.body.target).toEqual({ kind: "workspace", workspace_id: f.workspace.id });
  await apiApprove(f.owner, created.receipt.candidate_id);
  await expect(reviewPanel(page).getByText(created.body.content, { exact: true })).toBeVisible();
  const context = await browser.newContext();
  try {
    const target = await context.newPage();
    const hints = observeKnowledgeHints(target);
    await openKnowledge(target, f.owner, f.path);
    await openTarget(target, f, "workspace");
    await chooseCandidate(target, created.body.summary, targetPanel(target));
    const before = (await targetItems(f.owner, f.workspace.id)).knowledge_revision;
    // The purged version is historical; the current memory remains lawful.
    await f.owner.requestJSON(`${f.root}/memory`, { method: "PUT", headers: { "Idempotency-Key": randomUUID() }, body: { expected_version: "1", content: "Replacement source without the old conclusion." } });
    await f.owner.requestJSON(`${f.root}/memory/versions/1/purge`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { reason: "Dispose of test-owned historical source material." } });
    await expect(page.getByText(created.body.content, { exact: true })).toHaveCount(0, { timeout: 35_000 });
    await expect(target.getByText(created.body.content, { exact: true })).toHaveCount(0, { timeout: 35_000 });
    await expect(reviewPanel(target).getByText("Source material is unavailable. Its text and references are hidden.", { exact: true })).toBeVisible();
    await hints.expectAdvance(f.workspace.id, before);
    expect((await f.owner.requestJSON<CollaborationKnowledgeDetail>(`/api/collaboration/knowledge-candidates/${created.receipt.candidate_id}`)).candidate).toMatchObject({ availability: "source_unavailable" });
    await captureKnowledge(target, targetPanel(target), info, "purged-target");
    await info.attach("purge-evidence", { contentType: "application/json", body: JSON.stringify({ candidate_id: created.receipt.candidate_id, source_and_target_cleared: true, hints: hints.frames }, null, 2) });
  } finally { await context.close(); }
});

test("Web original-key recovery stays locked after ambiguity and recovers by GET after reload", async ({ page }, info) => {
  test.setTimeout(120_000);
  const f = await seedKnowledge();
  await openKnowledge(page, f.handler, f.path);
  await sourcePanel(page).getByRole("button", { name: "Propose knowledge", exact: true }).click();
  await fillKnowledge(page, "Recover exactly one pending candidate", "A lost response must preserve the original client-selected identity.");
  const calls: CollaborationKnowledgeProposal[] = [];
  const url = `**${f.root}/knowledge-candidates`;
  const intercept = async (route: Route) => {
    if (route.request().method() !== "POST") return route.continue();
    calls.push(route.request().postDataJSON());
    if (calls.length === 1) {
      const response = await route.fetch();
      expect(response.status()).toBe(201);
      await route.abort("failed");
    } else await route.fulfill({ status: 403, contentType: "application/json", body: JSON.stringify({ code: "knowledge_permission_denied", error: "Injected unsealed refusal after uncertain delivery" }) });
  };
  await page.route(url, intercept);
  try {
    await page.getByRole("button", { name: "Submit candidate", exact: true }).click();
    await expect(page.getByRole("button", { name: "Check original result", exact: true })).toBeEnabled();
    await expect(page.getByLabel("Summary", { exact: true })).toBeDisabled();
    await page.getByRole("button", { name: "Retry same request", exact: true }).click();
    await expect.poll(() => calls.length).toBe(2);
    expect(calls[1]).toEqual(calls[0]);
    await expect(page.getByLabel("Summary", { exact: true })).toBeDisabled();
    await page.unroute(url, intercept);
    const postsAfterReload: string[] = [];
    page.on("request", request => {
      if (request.method() === "POST" && new URL(request.url()).pathname === `${f.root}/knowledge-candidates`) postsAfterReload.push(request.url());
    });
    await page.reload();
    const attention = page.getByRole("status").filter({ hasText: "Needs attention:" });
    await expect(attention).toBeVisible();
    await attention.getByRole("button", { name: "Knowledge candidates", exact: true }).click();
    await expect(page.getByRole("button", { name: "Check original result", exact: true })).toBeEnabled();
    await expect(page.getByRole("button", { name: "Retry same request", exact: true })).toHaveCount(0);
    const recovered = page.waitForResponse(r => new URL(r.url()).pathname.endsWith(`/operations/${calls[0]!.request_id}`));
    await page.getByRole("button", { name: "Check original result", exact: true }).click();
    expect((await recovered).request().method()).toBe("GET");
    await expect(page.getByText("Candidate proposed. It is not active until approved.", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Check original result", exact: true })).toHaveCount(0);
    const list = await f.handler.requestJSON<{ items: unknown[] }>(`${f.root}/knowledge-candidates`);
    expect(list.items).toHaveLength(1);
    expect(postsAfterReload).toEqual([]);
    await info.attach("recovery-evidence", { contentType: "application/json", body: JSON.stringify({ candidate_id: calls[0]!.candidate_id, request_id: calls[0]!.request_id,
      attempts: calls.length, exact_replay: true, recovery_method: "GET", created_candidates: list.items.length }, null, 2) });
  } finally { await page.unroute(url, intercept); }
});

test("Web definitive unapplied refusal unlocks the draft for a new request", async ({ page }, info) => {
  test.setTimeout(120_000);
  const f = await seedKnowledge();
  await openKnowledge(page, f.handler, f.path);
  await sourcePanel(page).getByRole("button", { name: "Propose knowledge", exact: true }).click();
  await fillKnowledge(page, "Editable after definitive refusal", "The corrected human intent gets a fresh request identity.");
  let rejected: CollaborationKnowledgeProposal | undefined;
  const url = `**${f.root}/knowledge-candidates`;
  const reject = async (route: Route) => {
    if (route.request().method() !== "POST") return route.continue();
    rejected = route.request().postDataJSON();
    await route.fulfill({ status: 400, contentType: "application/json", body: JSON.stringify({ code: "invalid_knowledge_request", error: "Injected definitive pre-application rejection" }) });
  };
  await page.route(url, reject);
  await page.getByRole("button", { name: "Submit candidate", exact: true }).click();
  await expect(page.getByText("The request was rejected before it was applied. Edit this draft and submit a new request.", { exact: true })).toBeVisible();
  await expect(page.getByLabel("Summary", { exact: true })).toBeEnabled();
  await expect(page.getByRole("button", { name: "Check original result", exact: true })).toHaveCount(0);
  await page.unroute(url, reject);
  await page.getByLabel("Summary", { exact: true }).fill("Corrected human intent after refusal");
  const created = await submitKnowledge(page);
  expect(created.sent.request_id).not.toBe(rejected?.request_id);
  expect(created.sent.candidate_id).not.toBe(rejected?.candidate_id);
  await info.attach("definitive-refusal", { contentType: "application/json", body: JSON.stringify({ old_request_id: rejected?.request_id, new_request_id: created.sent.request_id, pre_application_fault: true }) });
});

test("Web recorded-run source is selected through the authorized reader", async ({ page }, info) => {
  test.setTimeout(120_000);
  const f = await seedCollaborationRecordJourney();
  await openKnowledge(page, f.viewer, `/account?view=participating&issue=${f.issue.id}`);
  await sourcePanel(page).getByRole("button", { name: "Propose knowledge", exact: true }).click();
  const form = page.getByRole("form", { name: "Propose knowledge", exact: true });
  await expect(form.getByLabel("Summary", { exact: true })).toBeFocused();
  await form.getByLabel("Summary", { exact: true }).fill("Retained run finding");
  await form.getByLabel("Conclusion (Markdown)", { exact: true }).fill("The recorded run identifies a review step; publication does not verify the claim.");
  await selectKnowledgeOption(page, form, "Sources", "Run record");
  await selectKnowledgeOption(page, form, "Choose a recorded run", new RegExp(f.runId.slice(-8)));
  await form.getByRole("checkbox", { name: /^Run record ·/ }).first().check();
  const proposed = await submitKnowledge(page);
  expect(proposed.sent.sources[0]).toMatchObject({ kind: "run_record", run_id: f.runId, issue_id: f.issue.id });
  await info.attach("recorded-source-evidence", { contentType: "application/json", body: JSON.stringify({ sources: proposed.sent.sources,
    origin: "TestApiClient human/resource setup with existing retained-history SQL fixture", runtime_ingestion_tested: false, provider_account_access: false }, null, 2) });
});

test("Web target-only administrator loses source material but can withdraw", async ({ page }, info) => {
  test.setTimeout(120_000);
  const f = await seedKnowledge();
  const target = await f.handler.ensureWorkspace("Independent knowledge destination", `knowledge-target-${randomUUID().slice(0, 8)}`);
  const created = await apiProposal(f, { target: { kind: "workspace", workspace_id: target.id }, summary: "Restricted destination control",
    content: "TARGET_ONLY_PRIVATE_BODY_must_disappear" });
  await apiApprove(f.handler, created.receipt.candidate_id);
  const hints = observeKnowledgeHints(page);
  await openKnowledge(page, f.handler, f.path);
  await page.goto(`/${target.slug}/settings?tab=workspace`);
  await chooseCandidate(page, created.body.summary, targetPanel(page));
  const previous = (await targetItems(f.handler, target.id)).knowledge_revision;
  const issue = await f.owner.requestJSON<CollaborationIssue>(f.root);
  await f.owner.requestJSON(`${f.root}/handoffs`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...memoryAuthority(issue), target_user_id: f.ownerId, purpose: "transfer", note: "Return source responsibility before removing the external participant.",
  } });
  const current = await f.owner.requestJSON<CollaborationIssue>(f.root);
  await f.owner.requestJSON(`${f.root}/participants/${f.handlerId}/remove`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: memoryAuthority(current) });
  await expect(page.getByText(created.body.content, { exact: true })).toHaveCount(0, { timeout: 35_000 });
  await expect(reviewPanel(page).getByText("Restricted contribution", { exact: true })).toBeVisible();
  await hints.expectAdvance(target.id, previous);
  await expect(reviewPanel(page).getByRole("link", { name: "Open source collaboration", exact: true })).toHaveCount(0);
  await expect(reviewPanel(page).getByRole("button", { name: "Reject candidate", exact: true })).toHaveCount(0);
  await captureKnowledge(page, targetPanel(page), info, "restricted-target");
  await reviewPanel(page).getByLabel("Decision reason", { exact: true }).fill("Remove this contribution from the target without reading its source.");
  await reviewPanel(page).getByRole("button", { name: "Withdraw contribution", exact: true }).click();
  await expect(page.getByText("Decision recorded.", { exact: true })).toBeVisible();
  const withdrawn = (await targetItems(f.handler, target.id)).items.find(c => c.candidate_id === created.receipt.candidate_id);
  expect(withdrawn).toMatchObject({ state: "withdrawn", availability: "restricted", capabilities: { can_withdraw: false } });
  await expect(reviewPanel(page).getByRole("button", { name: "Withdraw contribution", exact: true })).toHaveCount(0);
  await info.attach("target-only-hints", { contentType: "application/json", body: JSON.stringify(hints.frames) });
});

test("Web expiry clears active target material when refresh fails", async ({ page }, info) => {
  test.setTimeout(120_000);
  const f = await seedKnowledge();
  const created = await apiProposal(f, { summary: "Short-lived guidance", content: "EXPIRY_PRIVATE_BODY_must_not_outlive_lease", expires_at: new Date(Date.now() + 12_000).toISOString() });
  await apiApprove(f.owner, created.receipt.candidate_id);
  await openKnowledge(page, f.owner, f.path);
  await openTarget(page, f, "workspace");
  await chooseCandidate(page, created.body.summary, targetPanel(page));
  await expect(page.getByText(created.body.content, { exact: true })).toBeVisible();
  await page.waitForLoadState("networkidle");
  const expiry = Date.parse(created.body.expires_at!);
  expect(expiry - Date.now()).toBeGreaterThan(3000);
  const failRefresh = async (route: Route) => route.request().method() === "GET" ? route.abort("failed") : route.continue();
  const knowledgeReads = /\/api\/.*\/knowledge/;
  await page.route(knowledgeReads, failRefresh);
  try {
    await expect(page.getByText(created.body.content, { exact: true })).toHaveCount(0, { timeout: 15_000 });
    const hidden = Date.now();
    expect(hidden).toBeGreaterThanOrEqual(expiry - 1500);
    expect(hidden).toBeLessThanOrEqual(expiry + 5000);
    await expect(targetPanel(page).getByRole("button", { name: /^Short-lived guidance/ })).toHaveCount(0);
    await info.attach("expiry-evidence", { contentType: "application/json", body: JSON.stringify({ expires_at: created.body.expires_at, hidden_at: new Date(hidden).toISOString(), refresh_failed: true, online_state_unchanged: true }) });
  } finally { await page.unroute(knowledgeReads, failRefresh); }
});

// The same human flow runs in a real Electron compositor, with production
// native routing/IPC and test-owned daemon services from the existing harness.
// eslint-disable-next-line no-empty-pattern -- Electron owns its browser context.
test("Desktop native source review and independent project/workspace contributions", async ({}, info) => {
  test.setTimeout(240_000);
  if (!process.env.COLLABORATION_ELECTRON_RENDERER_URL || !process.env.COLLABORATION_ELECTRON_SOURCE_ROOT) throw new Error("Explicit built native fixture required");
  const f = await seedKnowledge();
  const profiles = [mkdtempSync(join(tmpdir(), "knowledge-native-handler-")), mkdtempSync(join(tmpdir(), "knowledge-native-owner-"))];
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  let traceStarted = false;
  try {
    native = await openDesktop(profiles[0]!, { token: f.handler.getToken()! });
    observeKnowledgeHints(native.page);
    const connected: string[] = [];
    native.page.on("websocket", socket => socket.on("framereceived", ({ payload }) => {
      try { const frame = JSON.parse(payload.toString()); if (frame.type === "auth_ack") connected.push(frame.type); } catch { /* Heartbeat. */ }
    }));
    await native.app.context().tracing.start({ screenshots: true, snapshots: true });
    traceStarted = true;
    await native.start();
    await deepLink(native.app, f.path);
    await native.page.bringToFront();
    await expect.poll(() => connected.length, { timeout: 10_000 }).toBeGreaterThan(0);
    await revealCollaborationDisclosure(native.page, "Knowledge candidates");
    await expect(sourcePanel(native.page)).toBeVisible();
    const first = await externalProposal(native.page, f);
    expect(native.errors).toEqual([]);
    await native.app.context().tracing.stop({ path: info.outputPath("desktop-handler-trace.zip") });
    traceStarted = false;
    await native.app.close();
    native = await openDesktop(profiles[1]!, { token: f.owner.getToken()! });
    const hints = observeKnowledgeHints(native.page);
    await native.app.context().tracing.start({ screenshots: true, snapshots: true });
    traceStarted = true;
    await native.start();
    await deepLink(native.app, f.path);
    await revealCollaborationDisclosure(native.page, "Knowledge candidates");
    await ownerVersions(native.page, f, first, info, native);
    const previous = (await targetItems(f.owner, f.workspace.id)).knowledge_revision;
    await f.owner.requestJSON(`${f.root}/memory`, { method: "PUT", headers: { "Idempotency-Key": randomUUID() }, body: { expected_version: "1", content: "Replacement source without the old conclusion." } });
    await f.owner.requestJSON(`${f.root}/memory/versions/1/purge`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { reason: "Dispose of native acceptance source material." } });
    await expect(reviewPanel(native.page).getByText("Check the original result before attempting a new write.", { exact: true })).toHaveCount(0, { timeout: 35_000 });
    await expect(reviewPanel(native.page).getByText("Source material is unavailable. Its text and references are hidden.", { exact: true })).toBeVisible();
    await hints.expectAdvance(f.workspace.id, previous);
    await captureKnowledge(native.page, targetPanel(native.page), info, "purged-target", native.app);
    expect(native.errors).toEqual([]);
    const evidence = await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence);
    expect(evidence.daemonStarts).toBe(0);
    await info.attach("native-execution-boundary", { contentType: "application/json", body: JSON.stringify({ platform: process.platform,
      renderer: process.env.COLLABORATION_ELECTRON_RENDERER_URL, source: process.env.COLLABORATION_ELECTRON_SOURCE_ROOT, ...evidence }) });
    await info.attach("native-target-hints", { contentType: "application/json", body: JSON.stringify(hints.frames) });
  } finally {
    if (native) {
      await info.attach("native-page-errors", { contentType: "application/json", body: JSON.stringify(native.errors) });
      await native.page.screenshot({ path: info.outputPath("desktop-final-state.png") }).catch(() => undefined);
      if (traceStarted) await native.app.context().tracing.stop({ path: info.outputPath("desktop-trace.zip") });
      await native.app.close().catch(() => undefined);
    }
    for (const profile of profiles) rmSync(profile, { recursive: true, force: true });
  }
});
