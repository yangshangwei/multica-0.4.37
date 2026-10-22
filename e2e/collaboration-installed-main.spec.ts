import { randomUUID } from "node:crypto";
import { test, expect, type Frame, type Page, type Request } from "@playwright/test";
import type { CollaborationCommentInput, CollaborationCommentReceipt, CollaborationCommandHistory,
  CollaborationControlInput, CollaborationControlReceipt, CollaborationIssue, CollaborationOperation, CollaborationStatusOptions, CollaborationWorkbench } from "@multica/core/collaboration";
import { TestApiClient } from "./fixtures";
import { authenticateControlWeb, controlAccountPath, seedCollaborationControlJourney } from "./fixtures/collaboration-control-journey";
import { dockerTaskURL, InstalledMainClient, type InstalledMainMetadata, type InstalledMainWrite } from "./fixtures/collaboration-installed-main";

// The local runner handles secrets outside Playwright tracing. This actual
// installed Main evidence supplements the canonical service/renderer journeys;
// the remote pending permit below starts no provider and produces no proof.
test.use({ actionTimeout: 20_000, trace: "off", screenshot: "off", video: "off" });

function sharedFacts(issue: CollaborationIssue) {
  return { id: issue.id, title: issue.title, description: issue.description, creator: issue.creator,
    handler: issue.handler, source_workspace: issue.source_workspace, collaboration_revision: issue.collaboration_revision,
    lifecycle_epoch: issue.lifecycle_epoch, status: issue.status, session: issue.session, execution: issue.execution,
    current_information_request: issue.current_information_request };
}

async function refreshWebIssue(page: Page, issueId: string) {
  let navigated = false;
  const requests = new WeakSet();
  const navigation = (frame: Frame) => { if (frame === page.mainFrame()) navigated = true; };
  const observe = (value: Request) => {
    if (navigated && value.method() === "GET" && new URL(value.url()).pathname === `/api/collaboration/issues/${issueId}`) requests.add(value);
  };
  page.on("framenavigated", navigation);
  page.on("request", observe);
  try {
    const pending = page.waitForResponse((response) => requests.has(response.request()) && response.status() === 200);
    await page.reload();
    const response = await pending;
    expect(response.status()).toBe(200);
    const current = await page.request.get(response.url());
    expect(current.status()).toBe(200);
    const issue: CollaborationIssue = await current.json();
    expect(issue.id).toBe(issueId);
    const heading = page.getByRole("heading", { name: issue.title, exact: true });
    await expect(heading).toBeVisible();
    await expect(heading.locator("..").locator("..").locator("dl > div")
      .filter({ has: page.getByText("Handler", { exact: true }) }).locator("dd")).toContainText(issue.handler.name);
    await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    return issue;
  } finally {
    page.off("framenavigated", navigation);
    page.off("request", observe);
  }
}

async function webOperation(page: Page, issueId: string, action: "invite" | "transfer", userId: string, name: string, note = "") {
  const baseline = await refreshWebIssue(page, issueId);
  const label = action === "invite" ? "Add participant" : "Transfer";
  await page.getByRole("button", { name: label, exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: label, exact: true }) });
  await form.getByLabel("Exact recipient", { exact: true }).fill(userId);
  await form.getByRole("button", { name: "Find account", exact: true }).click();
  await expect(form.getByText(`Selected account: ${name}`, { exact: true })).toBeVisible();
  if (action === "transfer") await form.getByLabel("Note", { exact: true }).fill(note);
  const endpoint = `/api/collaboration/issues/${issueId}/${action === "invite" ? "participants" : "handoffs"}`;
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === endpoint && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  const requestId = response.request().headers()["idempotency-key"];
  expect(requestId).toMatch(/^[0-9a-f-]{36}$/i);
  const input = response.request().postDataJSON();
  expect(input.expected_collaboration_revision).toBe(baseline.collaboration_revision);
  expect(input.expected_lifecycle_epoch).toBe(baseline.lifecycle_epoch);
  await expect(form).toHaveCount(0);
  return { receipt, requestId, input: response.request().postDataJSON() as unknown };
}

async function webControl(page: Page, issueId: string, action: "stop" | "resume", resourceId?: string) {
  await refreshWebIssue(page, issueId);
  const panel = page.getByRole("region", { name: "AI execution", exact: true });
  const scope = resourceId ? panel.getByRole("listitem").filter({ has: page.getByText(resourceId, { exact: true }) })
    .filter({ has: page.getByRole("button", { name: "Stop target", exact: true }) }) : panel;
  const button = scope.getByRole("button", { name: action === "stop" ? resourceId ? "Stop target" : "Stop all"
    : resourceId ? "Resume target" : "Resume all", exact: true });
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${issueId}/control`
    && response.request().method() === "POST");
  await button.click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationControlReceipt = await response.json();
  const requestId = response.request().headers()["idempotency-key"];
  expect(receipt.request_id).toBe(requestId);
  return { receipt, requestId, input: response.request().postDataJSON() as CollaborationControlInput };
}

type DirectedAction = "request_information" | "return_information" | "submit_acceptance";
interface DirectedWrite extends InstalledMainWrite<CollaborationOperation, Record<string, unknown>> {
  baseline: CollaborationIssue; selectedStatus: string;
}

async function webInformationForm(page: Page, issueId: string, action: DirectedAction, note: string,
  recipient?: { id: string; name: string }): Promise<DirectedWrite> {
  const baseline = await refreshWebIssue(page, issueId);
  expect(baseline.capabilities[action]).toBe(true);
  const label = { request_information: "Request information", return_information: "Return information", submit_acceptance: "Submit for acceptance" }[action];
  const optionsPending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${issueId}/status-options`
    && new URL(response.url()).searchParams.get("action") === action && response.request().method() === "GET");
  await page.getByRole("button", { name: label, exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: label, exact: true }) });
  const optionsResponse = await optionsPending;
  expect(optionsResponse.status()).toBe(200);
  const options: CollaborationStatusOptions = await optionsResponse.json();
  expect(options.action).toBe(action);
  expect(options.options).toHaveLength(1);
  if (action === "request_information") {
    if (!recipient) throw new Error("Request information needs the exact participant recipient");
    await form.getByLabel("Exact recipient", { exact: true }).fill(recipient.id);
    await form.getByRole("button", { name: "Find account", exact: true }).click();
    await expect(form.getByText(`Selected account: ${recipient.name}`, { exact: true })).toBeVisible();
  }
  const select = form.getByLabel("Status", { exact: true });
  await expect(select.locator("option")).toHaveCount(1);
  await select.selectOption(options.default_status);
  await form.getByLabel("Note", { exact: true }).fill(note);
  const endpoint = `/api/collaboration/issues/${issueId}/${action === "request_information" ? "handoffs" : action === "return_information" ? "information-returns" : "acceptance-submissions"}`;
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === endpoint && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  const input: Record<string, unknown> = response.request().postDataJSON();
  const requestId = response.request().headers()["idempotency-key"]!;
  expect(requestId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(input).toMatchObject({ expected_collaboration_revision: baseline.collaboration_revision, expected_lifecycle_epoch: baseline.lifecycle_epoch,
    expected_participant_epoch: baseline.write_context!.participant_epoch, expected_account_epoch: baseline.write_context!.account_epoch,
    expected_status: options.default_status, status_options_version: options.status_options_version, note });
  if (action === "return_information") expect(input.information_request_id).toBe(baseline.current_information_request!.id);
  await expect(form).toHaveCount(0);
  return { receipt, input, requestId, baseline, selectedStatus: options.default_status, resultText: "Action confirmed." };
}

test("installed Linux full Main and production Web hand off the same issue, preserve authority/history and reject unconfirmed Resume", async ({ page }, testInfo) => {
  test.setTimeout(600_000);
  const required = ["COLLABORATION_INSTALLED_IMAGE", "COLLABORATION_E2E_API_URL", "NEXT_PUBLIC_API_URL", "MULTICA_DEPLOYMENT_ID", "DATABASE_URL"];
  for (const key of required) expect(process.env[key], `Explicit task-owned ${key}`).toBeTruthy();
  expect(process.env.COLLABORATION_INSTALLED_APP_DIR || process.env.COLLABORATION_INSTALLED_APP_VOLUME).toBeTruthy();
  expect(process.env.E2E_PASSWORD_AUTH).toBe("1");
  const webURL = String(testInfo.project.use.baseURL);
  const fixture = await seedCollaborationControlJourney();
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const readWeb = () => fixture.creator.requestJSON<CollaborationIssue>(root);
  const actor = new TestApiClient();
  const username = `s6installed${randomUUID().replaceAll("-", "").slice(0, 12)}`;
  const password = `Installed-${randomUUID()}!`;
  const name = `Installed full Main handler ${randomUUID().slice(0, 6)}`;
  const user = await actor.registerPassword(username, password, name);
  expect((await actor.requestJSON<{ onboarded_at: string | null }>("/api/me/onboarding/complete", { method: "POST", body: {} })).onboarded_at).not.toBeNull();
  expect(await actor.getWorkspaces()).toEqual([]);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await authenticateControlWeb(page, fixture, webURL);
  await page.goto(new URL(controlAccountPath(fixture), webURL).href);
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  const initial = await readWeb();
  const added = await webOperation(page, fixture.issueId, "invite", user.id, name);
  expect(added.receipt.code).toBe("participant_added");
  const transferToInstalled = await webOperation(page, fixture.issueId, "transfer", user.id, name, "Web hands the same issue to the installed full Main");
  expect(transferToInstalled.receipt.code).toBe("transferred");
  const transferred = await readWeb();
  expect(transferred.handler.user_id).toBe(user.id);
  expect(transferred.id).toBe(initial.id);
  expect(transferred.creator).toEqual(initial.creator);
  expect(transferred.session).toEqual(initial.session);
  expect(transferred.execution).toEqual(initial.execution);
  expect((await actor.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=current")).items.map((item) => item.id)).toEqual([fixture.issueId]);

  const native = new InstalledMainClient();
  const evidence: Record<string, unknown> = { issueId: fixture.issueId, creatorId: fixture.creatorId,
    installedHandlerId: user.id, sourceWorkspaceId: fixture.sourceWorkspace.id, webTransfer: transferToInstalled,
    nativeExecutionScope: "Full packaged production Main/preload/renderer and bundled CLI version bootstrap; remote permit is a public protocol simulation with no executable, outcome or proof" };
  let started = false;
  try {
    const boot = await native.command<InstalledMainMetadata>("init", { apiUrl: dockerTaskURL(fixture.apiUrl), appUrl: dockerTaskURL(webURL) }, 180_000);
    started = true;
    evidence.boot = boot;
    expect(boot.main.packaged).toBe(true);
    expect(boot.main.electron).toBe("39.8.7");
    expect(boot.bundledCLI?.version).toBeTruthy();
    evidence.login = await native.command("login", { username, password, userId: user.id });
    const opened = await native.command<InstalledMainMetadata>("navigate", { path: `/account?view=current&issue=${fixture.issueId}`,
      issueId: fixture.issueId, title: fixture.title });
    expect(opened.tabs?.activeScope).toBe("account");
    expect(opened.tabs?.activeWorkspaceSlug).toBeNull();
    expect(opened.tabs?.accountTabs).toHaveLength(1);
    const readNative = () => native.command<CollaborationIssue>("read", { path: root });
    expect(sharedFacts(await readNative())).toEqual(sharedFacts(transferred));
    expect((await readNative()).capabilities.control_ai).toBe(true);
    expect(transferred.capabilities.control_ai).toBe(true);
    expect(await native.command("read", { path: "/api/workspaces" })).toEqual([]);

    const creatorProfile = await fixture.creator.requestJSON<{ name: string }>("/api/me");
    const workflowWrites: { client: "web" | "electron"; action: DirectedAction; outcome: DirectedWrite; after: CollaborationIssue }[] = [];
    async function checkDirected(client: "web" | "electron", action: DirectedAction, outcome: DirectedWrite, handlerId: string) {
      const after = await readWeb();
      expect(outcome.receipt.code).toBe(action === "request_information" ? "information_requested" : action === "return_information" ? "information_returned" : "acceptance_submitted");
      expect(after.id).toBe(fixture.issueId);
      expect(after.handler.user_id).toBe(handlerId);
      expect(after.creator).toEqual(initial.creator);
      expect(after.session).toEqual(outcome.baseline.session);
      expect(after.execution).toEqual(outcome.baseline.execution);
      expect(after.status).toBe(outcome.selectedStatus);
      expect(outcome.input.expected_collaboration_revision).toBe(outcome.baseline.collaboration_revision);
      expect((await native.command<CollaborationIssue>("read", { path: root })).handler.user_id).toBe(handlerId);
      if (action === "request_information") expect(after.current_information_request?.recipient.user_id).toBe(handlerId);
      else expect(after.current_information_request).toBeNull();
      workflowWrites.push({ client, action, outcome, after });
    }
    const requestedByNative = await native.command<DirectedWrite>("workflow", { issueId: fixture.issueId, action: "request_information",
      recipientId: fixture.creatorId, recipientName: creatorProfile.name, note: "Installed handler requests information from the Web creator" });
    await checkDirected("electron", "request_information", requestedByNative, fixture.creatorId);
    const returnedByWeb = await webInformationForm(page, fixture.issueId, "return_information", "Web creator returns the requested information");
    await checkDirected("web", "return_information", returnedByWeb, user.id);
    const acceptanceByNative = await native.command<DirectedWrite>("workflow", { issueId: fixture.issueId, action: "submit_acceptance",
      note: "Installed handler submits for creator acceptance" });
    await checkDirected("electron", "submit_acceptance", acceptanceByNative, fixture.creatorId);
    const requestedByWeb = await webInformationForm(page, fixture.issueId, "request_information", "Web creator requests information from the installed participant", { id: user.id, name });
    await checkDirected("web", "request_information", requestedByWeb, user.id);
    const returnedByNative = await native.command<DirectedWrite>("workflow", { issueId: fixture.issueId, action: "return_information",
      note: "Installed participant returns information to its requester" });
    await checkDirected("electron", "return_information", returnedByNative, fixture.creatorId);
    const handoffForWebAcceptance = await webOperation(page, fixture.issueId, "transfer", user.id, name, "Same handler switches to Web for acceptance submission");
    expect(handoffForWebAcceptance.receipt.code).toBe("transferred");
    const handlerContext = await page.context().browser()!.newContext({ baseURL: webURL });
    try {
      const login = await handlerContext.request.post(`${fixture.apiUrl}/auth/login`, { data: { username, password } });
      expect(login.status()).toBe(200);
      expect((await login.json()).user.id).toBe(user.id);
      const cookies = await handlerContext.cookies(fixture.apiUrl);
      expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
      expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
      await handlerContext.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
      const handlerWeb = await handlerContext.newPage();
      await handlerWeb.goto(new URL(controlAccountPath(fixture), webURL).href);
      const acceptanceByWeb = await webInformationForm(handlerWeb, fixture.issueId, "submit_acceptance", "Same installed handler submits acceptance from Web");
      await checkDirected("web", "submit_acceptance", acceptanceByWeb, fixture.creatorId);
    } finally { await handlerContext.close(); }
    const workflowFinished = await readWeb();
    const activity = await fixture.creator.requestJSON<{ items: { kind: string }[] }>(`${root}/activity`);
    for (const kind of ["information_requested", "information_returned", "acceptance_submitted"]) {
      expect(activity.items.filter((item) => item.kind === kind)).toHaveLength(2);
    }
    expect(workflowWrites).toHaveLength(6);
    expect(new Set(workflowWrites.map((entry) => entry.outcome.requestId)).size).toBe(6);
    evidence.directedWorkflows = { writes: workflowWrites, sameUserWebLogin: user.id, handoffForWebAcceptance,
      finalStatus: workflowFinished.status, activityKinds: activity.items.map((item) => item.kind) };
    const handoffAfterWorkflows = await webOperation(page, fixture.issueId, "transfer", user.id, name, "Web resumes original installed-handler control journey");
    expect(handoffAfterWorkflows.receipt.code).toBe("transferred");

    const resume = await webControl(page, fixture.issueId, "resume");
    expect(resume.receipt).toMatchObject({ code: "resumed", queue_state: "active" });
    await native.command("reload", { issueId: fixture.issueId, title: fixture.title });
    const stoppedB = await native.command<InstalledMainWrite<CollaborationControlReceipt, CollaborationControlInput>>("control", {
      issueId: fixture.issueId, action: "stop", resourceId: fixture.agentB.resourceId, status: 200 });
    expect(stoppedB.receipt).toMatchObject({ target_id: fixture.agentB.targetId, scope: "target", code: "already_ended" });
    const resumedB = await native.command<InstalledMainWrite<CollaborationControlReceipt, CollaborationControlInput>>("control", {
      issueId: fixture.issueId, action: "resume", resourceId: fixture.agentB.resourceId, status: 200 });
    expect(resumedB.receipt).toMatchObject({ target_id: fixture.agentB.targetId, code: "resumed" });

    const targets = await fixture.creator.requestJSON<{ targets: { name: string; target_id: string }[] }>(`${root}/mention-targets`);
    const targetName = (targetId: string) => {
      const target = targets.targets.find((item) => item.target_id === targetId);
      if (!target) throw new Error("The installed Main target is absent from the actual authorized catalogue");
      return target.name;
    };
    const markerA = `Installed Main original A input ${randomUUID().slice(0, 8)}`;
    const markerB = `Installed Main waiting B input ${randomUUID().slice(0, 8)}`;
    const inputA = await native.command<InstalledMainWrite<CollaborationCommentReceipt, CollaborationCommentInput>>("comment", {
      issueId: fixture.issueId, marker: markerA, resourceId: fixture.agentA.resourceId, targetName: targetName(fixture.agentA.targetId) });
    const inputB = await native.command<InstalledMainWrite<CollaborationCommentReceipt, CollaborationCommentInput>>("comment", {
      issueId: fixture.issueId, marker: markerB, resourceId: fixture.agentB.resourceId, targetName: targetName(fixture.agentB.targetId) });
    expect(inputA.receipt.targets).toHaveLength(1);
    expect(inputB.receipt.targets).toHaveLength(1);
    const commandA = inputA.receipt.targets[0]!;
    const commandB = inputB.receipt.targets[0]!;
    expect(commandA).toMatchObject({ state: "waiting", target_id: fixture.agentA.targetId });
    expect(commandB).toMatchObject({ state: "waiting", target_id: fixture.agentB.targetId });
    expect(commandA.command_id && commandA.batch_id).toBeTruthy();
    expect(commandB.command_id && commandB.command_sequence).toBeTruthy();
    const cancelled = await native.command<InstalledMainWrite<CollaborationControlReceipt>>("cancel", {
      issueId: fixture.issueId, commandId: commandB.command_id, sequence: commandB.command_sequence, resourceId: fixture.agentB.resourceId });
    expect(cancelled.receipt).toMatchObject({ code: "command_cancelled", command_id: commandB.command_id });
    await native.command("history", { commandId: commandB.command_id, marker: markerB, state: "Cancelled", reason: "Cancelled by a participant" });
    const history = await fixture.creator.requestJSON<CollaborationCommandHistory>(`${root}/commands?limit=100`);
    const cancelledEntry = history.items.find((item) => item.id === commandB.command_id);
    expect(cancelledEntry).toMatchObject({ id: commandB.command_id, state: "cancelled", content: inputB.input.content,
      author_user_id: user.id, cancellation: { actor_user_id: user.id, kind: "manual" } });
    await page.reload();
    await page.getByRole("button", { name: "Command history", exact: true }).click();
    await expect(page.locator(`[data-command-id="${commandB.command_id}"]`)).toContainText(markerB);
    await expect(page.locator(`[data-command-id="${commandB.command_id}"]`)).toContainText("Cancelled");
    const pending = await fixture.simulatedDaemon.permitPendingInput(fixture.issueId, fixture.agentA, commandA.command_id!, commandA.batch_id!);
    evidence.pendingPermit = { ...pending, simulation: true, providerStarted: false, terminationProofSupplied: false };
    const permitted = await readWeb();
    expect(permitted.execution?.runs.find((run) => run.id === pending.runId)?.progress_state).toBe("launch_authorized");
    await page.reload();
    const stoppedA = await webControl(page, fixture.issueId, "stop", fixture.agentA.resourceId);
    expect(stoppedA.receipt).toMatchObject({ code: "stop_requested", target_id: fixture.agentA.targetId });
    const beforeRefusal = await readWeb();
    await native.command("reload", { issueId: fixture.issueId, title: fixture.title });
    const refused = await native.command<InstalledMainWrite<CollaborationControlReceipt, CollaborationControlInput>>("control", {
      issueId: fixture.issueId, action: "resume", resourceId: fixture.agentA.resourceId, status: 409 });
    expect(refused.receipt).toMatchObject({ code: "stop_unconfirmed", target_id: fixture.agentA.targetId });
    expect(refused.resultText).toContain("Termination unconfirmed");
    expect(sharedFacts(await readNative())).toEqual(sharedFacts(beforeRefusal));
    const stoppedAll = await native.command<InstalledMainWrite<CollaborationControlReceipt, CollaborationControlInput>>("control", {
      issueId: fixture.issueId, action: "stop", status: 200 });
    expect(stoppedAll.receipt).toMatchObject({ code: "stop_requested", queue_state: "paused", scope: "issue" });
    const partial = await native.command<InstalledMainWrite<CollaborationControlReceipt, CollaborationControlInput>>("control", {
      issueId: fixture.issueId, action: "resume", status: 200 });
    expect(partial.receipt).toMatchObject({ code: "partially_resumed", queue_state: "active" });
    expect(partial.receipt.channels.find((lane) => lane.channel_id === fixture.agentA.channelId)).toMatchObject({ code: "stop_unconfirmed", queue_state: "paused", unsettled_runs: 1 });
    expect(partial.resultText).toContain("2 lanes resumed; 1 remain paused.");
    expect(partial.resultText).toContain("Termination unconfirmed");
    for (const target of [fixture.agentB, fixture.squadS]) {
      expect(partial.receipt.channels.find((lane) => lane.channel_id === target.channelId)).toMatchObject({ code: "resumed", queue_state: "active" });
    }
    await page.reload();
    const stoppedSquad = await webControl(page, fixture.issueId, "stop", fixture.squadS.resourceId);
    expect(stoppedSquad.receipt).toMatchObject({ target_id: fixture.squadS.targetId, code: "already_ended" });
    await native.command("reload", { issueId: fixture.issueId, title: fixture.title });
    const resumedSquad = await native.command<InstalledMainWrite<CollaborationControlReceipt, CollaborationControlInput>>("control", {
      issueId: fixture.issueId, action: "resume", resourceId: fixture.squadS.resourceId, status: 200 });
    expect(resumedSquad.receipt).toMatchObject({ target_id: fixture.squadS.targetId, code: "resumed" });
    const finalStop = await native.command<InstalledMainWrite<CollaborationControlReceipt, CollaborationControlInput>>("control", {
      issueId: fixture.issueId, action: "stop", status: 200 });
    const beforeTransferBack = await readWeb();
    expect(beforeTransferBack.session.queue_state).toBe("paused");
    expect(beforeTransferBack.execution?.channels.every((lane) => lane.queue_state === "paused")).toBe(true);
    const transferBack = await native.command<InstalledMainWrite<CollaborationOperation>>("handoff", {
      issueId: fixture.issueId, recipientId: fixture.creatorId, recipientName: creatorProfile.name,
      note: "Installed full Main hands the original issue back to Web" });
    expect(transferBack.receipt.code).toBe("transferred");
    const afterTransferBack = await readWeb();
    expect(afterTransferBack.handler.user_id).toBe(fixture.creatorId);
    expect(afterTransferBack.creator).toEqual(initial.creator);
    expect(afterTransferBack.id).toBe(initial.id);
    expect(afterTransferBack.session).toEqual(beforeTransferBack.session);
    expect(afterTransferBack.execution).toEqual(beforeTransferBack.execution);
    expect((await actor.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=current")).items).toEqual([]);
    expect((await actor.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=participating")).items.map((item) => item.id)).toEqual([fixture.issueId]);
    await native.command("navigate", { path: `/account?view=participating&issue=${fixture.issueId}`, issueId: fixture.issueId, title: fixture.title });
    const nativeFormer = await readNative();
    // Effective participants retain Stop/Resume; only organizer/handler actions
    // such as transfer disappear after handing responsibility back.
    expect(nativeFormer.capabilities.control_ai).toBe(true);
    expect(nativeFormer.capabilities.transfer).toBe(false);
    expect(afterTransferBack.capabilities.control_ai).toBe(true);
    expect(sharedFacts(nativeFormer)).toEqual(sharedFacts(afterTransferBack));
    const priorHistory = await native.command<CollaborationCommandHistory>("read", { path: `${root}/commands?limit=100` });
    const restarted = await native.command<InstalledMainMetadata>("restart", { issueId: fixture.issueId, title: fixture.title });
    expect(restarted.main.packaged).toBe(true);
    expect(restarted.artifactSha256).toBe(boot.artifactSha256);
    expect(restarted.main.userData).toBe(boot.main.userData);
    expect(restarted.native.daemon.state).toBe("stopped");
    expect(restarted.tabs?.accountTabs).toHaveLength(1);
    expect(sharedFacts(await readNative())).toEqual(sharedFacts(afterTransferBack));
    expect(await native.command("read", { path: `${root}/commands?limit=100` })).toEqual(priorHistory);
    await native.command("history", { commandId: commandB.command_id, marker: markerB, state: "Cancelled", reason: "Cancelled by a participant" });
    await page.reload();
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Stop all", exact: true })).toBeEnabled();
    const current = await readWeb();
    expect((await fixture.creator.requestJSON<CollaborationCommandHistory>(`${root}/commands?limit=100`)).items.find((item) => item.id === commandB.command_id)).toEqual(cancelledEntry);
    expect(current.execution?.runs.map((run) => run.id)).toEqual([pending.runId]);
    expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    const status = await native.command<InstalledMainMetadata>("status");
    expect(status.errors).toEqual([]);
    expect(status.accountRequests.length).toBeGreaterThan(0);
    expect(status.accountRequests.every((request) => !request.workspaceId && !request.workspaceSlug)).toBe(true);
    expect(status.accountRequests.filter((request) => request.method === "POST").every((request) => request.clientOS === "linux"
      && request.clientVersion === boot.main.version)).toBe(true);
    expect(errors).toEqual([]);
    const capture = await native.command<{ png: string; noHorizontalOverflow: boolean }>("capture");
    expect(capture.noHorizontalOverflow).toBe(true);
    await testInfo.attach("installed-full-main.png", { contentType: "image/png", body: Buffer.from(capture.png, "base64") });
    evidence.controls = { resume, stoppedB, resumedB, stoppedA, refused, stoppedAll, partial, stoppedSquad, resumedSquad, finalStop };
    evidence.inputs = { inputA, inputB, cancelled, cancelledEntry };
    evidence.transferBack = transferBack;
    evidence.restored = restarted;
    evidence.final = { sharedFacts: sharedFacts(current), status, transport: native.transportEvidence };
    await testInfo.attach("installed-full-main-journey", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
  } catch (error) {
    if (started) {
      try {
        evidence.failureState = await native.command("status", {}, 20_000);
        await testInfo.attach("installed-full-main-failure", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
      } catch { /* Preserve the original failure if the native process exited. */ }
    }
    throw error;
  } finally {
    await fixture.simulatedDaemon.disconnect();
    await native.close();
  }
});
