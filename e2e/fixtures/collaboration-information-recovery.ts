import { createHash, randomUUID } from "node:crypto";
import { expect, type Browser, type Frame, type Locator, type Request, type Response, type TestInfo } from "@playwright/test";
import { z } from "zod";
import { CollaborationCreationInputSchema, CollaborationCreationOptionsSchema, CollaborationCreationResultSchema,
  CollaborationInformationAffectedSchema, CollaborationInformationDetailSchema, CollaborationInformationHistorySchema,
  CollaborationIssueSchema, CollaborationOperationSchema, CollaborationStageConfirmationSchema, CollaborationStagePolicySchema,
  CollaborationStatusOptionsSchema, collaborationInformationHistoryHasDetails, compareCollaborationInformationTuple,
  type CollaborationInformationHistoryItem, type CollaborationInformationRepairBasis, type CollaborationIssue,
  type CollaborationOperation, type CollaborationStatusOptions } from "@multica/core/collaboration";
import { closureActor } from "./collaboration-multi-account-closure";
import { seedOfflineHandoff, selectHandoffRecipient } from "./collaboration-offline-handoff";
import { revealCollaborationDisclosure } from "./collaboration-disclosures";
import { HeldRequest, withSessions, type RecoveryActor, type RecoveryEvidence, type RecoveryPlatform,
  type RecoverySession, type RecoverySurface } from "./collaboration-recovery";

export const informationRecoveryCases = ["A", "B", "C", "E", "F", "G"] as const;
export type InformationRecoveryCase = typeof informationRecoveryCases[number];
type JSONRecord = Record<string, unknown>;
const object = z.record(z.string(), z.unknown());
const issuePath = (id: string) => `/api/collaboration/issues/${id}`;
const digest = (value: unknown) => createHash("sha256").update(JSON.stringify(value)).digest("hex");
const now = () => new Date().toISOString();

interface HTTPFact {
  at: string; actor_id: string; path: string; method: string; request_id: string | null;
  input: JSONRecord | null; input_sha256: string | null; status: number; body: unknown;
  response_bytes: number; response_sha256: string; cache_control: string | null;
}
interface OriginalOperation { actor: RecoveryActor; key: string; receipt: CollaborationOperation; input: JSONRecord }
interface InformationFixture extends RecoverySurface {
  creator: RecoveryActor; handler: RecoveryActor; qa: RecoveryActor;
  workspace: { id: string; name: string; slug: string }; projectId: string; setupIssueId: string;
  evidence: RecoveryEvidence; http: HTTPFact[]; operations: OriginalOperation[];
  requestIds: string[]; returnIds: string[]; repairIds: string[];
}
interface InformationForm {
  session: RecoverySession; baseline: CollaborationIssue; form: Locator;
  action: "request_information" | "return_information" | "repair_information" | "submit_acceptance";
  options: CollaborationStatusOptions;
}
interface SentOperation extends OriginalOperation { before: CollaborationIssue; after: CollaborationIssue }

export function informationRecoveryEnvironmentReady(): boolean {
  return process.env.PAY241_S3_INFORMATION_RECOVERY_ACCEPTANCE === "1" && process.env.E2E_PASSWORD_AUTH === "1" &&
    [process.env.COLLABORATION_E2E_API_URL, process.env.NEXT_PUBLIC_API_URL, process.env.DATABASE_URL,
      process.env.TASK_DATABASE_NAME, process.env.COLLABORATION_ELECTRON_RENDERER_URL,
      process.env.COLLABORATION_ELECTRON_SOURCE_ROOT].every(Boolean);
}

function observations(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Information recovery requires the actual participant observations");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}

async function api(fixture: InformationFixture, actor: RecoveryActor, path: string, method = "GET", input?: JSONRecord,
  key?: string, workspaceId?: string): Promise<HTTPFact> {
  const token = actor.client.getToken();
  if (!token) throw new Error("The public password fixture has no authenticated session");
  fixture.evidence.checkpoint("public_http_request_started", { actor_id: actor.id, path, method, request_id: key ?? null,
    input: input ?? null, input_sha256: input === undefined ? null : digest(input) });
  const response = await fetch(`${fixture.apiUrl}${path}`, { method, headers: {
    Authorization: `Bearer ${token}`, "Content-Type": "application/json",
    ...(key ? { "Idempotency-Key": key } : {}), ...(workspaceId ? { "X-Workspace-ID": workspaceId } : {}),
  }, ...(input === undefined ? {} : { body: JSON.stringify(input) }) });
  const bytes = Buffer.from(await response.arrayBuffer());
  fixture.evidence.checkpoint("public_http_response_received", { actor_id: actor.id, path, method, request_id: key ?? null,
    status: response.status, response_bytes: bytes.length, response_sha256: createHash("sha256").update(bytes).digest("hex"),
    body_text: bytes.toString() });
  const fact: HTTPFact = { at: now(), actor_id: actor.id, path, method, request_id: key ?? null,
    input: input ?? null, input_sha256: input === undefined ? null : digest(input), status: response.status,
    body: JSON.parse(bytes.toString()) as unknown, response_bytes: bytes.length,
    response_sha256: createHash("sha256").update(bytes).digest("hex"), cache_control: response.headers.get("cache-control") };
  fixture.http.push(fact);
  fixture.evidence.checkpoint("actual_public_http_result", { fact });
  return fact;
}

async function current(fixture: InformationFixture, actor = fixture.creator) {
  const result = await api(fixture, actor, `${issuePath(fixture.issueId)}?execution_view_version=6`);
  expect(result.status).toBe(200);
  const issue = CollaborationIssueSchema.parse(result.body);
  expect(issue.id).toBe(fixture.issueId); expect(issue.source_workspace.id).toBe(fixture.workspace.id);
  return issue;
}

function executionFacts(issue: CollaborationIssue) {
  const session = issue.session === null ? null : Object.fromEntries(Object.entries(object.parse(issue.session)).filter(([key]) => key !== "event_seq"));
  return { session, execution: issue.execution, lifecycle_epoch: issue.lifecycle_epoch };
}
function unchangedExecution(before: CollaborationIssue, after: CollaborationIssue) {
  expect(executionFacts(after)).toEqual(executionFacts(before));
  expect(after.execution?.runs ?? []).toEqual([]); expect(after.execution?.commands ?? []).toEqual([]);
  expect(after.execution?.targets ?? []).toEqual([]);
}

async function options(fixture: InformationFixture, actor: RecoveryActor, action: InformationForm["action"]) {
  const result = await api(fixture, actor, `${issuePath(fixture.issueId)}/status-options?action=${action}`);
  expect(result.status).toBe(200);
  const value = CollaborationStatusOptionsSchema.parse(result.body); expect(value.action).toBe(action);
  return value;
}

async function policy(fixture: InformationFixture, enabled: boolean) {
  const path = `/api/projects/${fixture.projectId}/stage-policy`;
  const before = await api(fixture, fixture.creator, path, "GET", undefined, undefined, fixture.workspace.id);
  expect(before.status).toBe(200);
  const observed = CollaborationStagePolicySchema.parse(before.body);
  expect(observed.project_id).toBe(fixture.projectId);
  if (observed.enabled === enabled) return observed;
  if (enabled) expect(observed.activation_available).toBe(true);
  const key = randomUUID(), input = { expected_policy_revision: observed.policy_revision, enabled, reason: "Explicit information recovery policy transition" };
  const changed = await api(fixture, fixture.creator, path, "PUT", input, key, fixture.workspace.id);
  expect(changed.status).toBe(200);
  const confirmation = CollaborationStageConfirmationSchema.parse(object.parse(changed.body).stage_confirmation);
  expect(confirmation).toMatchObject({ action: "revise_policy", project_id: fixture.projectId, enabled });
  const recovered = await api(fixture, fixture.creator, `${path}/operations/${key}`, "GET", undefined, undefined, fixture.workspace.id);
  expect(recovered.status).toBe(200); expect(recovered.body).toEqual(changed.body);
  const result = await api(fixture, fixture.creator, path, "GET", undefined, undefined, fixture.workspace.id);
  const value = CollaborationStagePolicySchema.parse(result.body);
  expect(value.enabled).toBe(enabled); return value;
}

async function seedInformation(caseName: InformationRecoveryCase, evidence: RecoveryEvidence): Promise<InformationFixture> {
  expect(new URL(process.env.DATABASE_URL!).pathname.slice(1)).toBe(process.env.TASK_DATABASE_NAME);
  const seed = await seedOfflineHandoff();
  const qa = await closureActor(seed.apiUrl, "InformationQA", randomUUID().replaceAll("-", "").slice(0, 12));
  const fixture: InformationFixture = { ...seed, handler: seed.recipient, qa, projectId: "", setupIssueId: seed.issueId,
    evidence, http: [], operations: [], requestIds: [], returnIds: [], repairIds: [] };
  evidence.set({ case: caseName, database: process.env.TASK_DATABASE_NAME, api_url: seed.apiUrl,
    actors: { creator: seed.creator.id, handler: seed.recipient.id, qa: qa.id }, source_workspace_id: seed.workspace.id,
    setup_issue_id: seed.issueId, project_policy_setup: "public_owner_revision", installed_main_execution: false,
    provider_execution: false, G_scope: "genuine_existing_unmanaged_pending_submission_then_managed" });
  expect(await qa.client.getWorkspaces()).toEqual([]); expect(await seed.recipient.client.getWorkspaces()).toEqual([]);
  const project = await api(fixture, fixture.creator, "/api/projects", "POST", { title: `Information recovery ${caseName}` }, undefined, fixture.workspace.id);
  expect(project.status).toBe(201);
  fixture.projectId = z.object({ id: z.uuid(), title: z.string() }).parse(project.body).id;
  expect((await policy(fixture, false)).policy_revision).toBe("0");
  const read = await api(fixture, fixture.creator, `/api/collaboration/creation-options?source_workspace_id=${fixture.workspace.id}&project_id=${fixture.projectId}`);
  expect(read.status).toBe(200);
  const creationOptions = CollaborationCreationOptionsSchema.parse(read.body);
  expect(creationOptions.stage_policy_observation).toMatchObject({ state: "known", project_id: fixture.projectId, managed: false, policy_revision: "0" });
  const key = randomUUID(), input = CollaborationCreationInputSchema.parse({ source_workspace_id: fixture.workspace.id,
    project_id: fixture.projectId, handler_user_id: fixture.handler.id, title: `Information ${caseName} ${randomUUID().slice(0, 8)}`,
    description: `Private information recovery case ${caseName}.`, ai_policy: "manual", attachment_ids: [] });
  const created = await api(fixture, fixture.creator, "/api/collaboration/issues", "POST", input, key);
  expect(created.status).toBe(201);
  const issue = CollaborationIssueSchema.parse(created.body);
  fixture.issueId = issue.id; fixture.title = issue.title; fixture.accountPath = `/account?view=participating&issue=${issue.id}`;
  evidence.set({ issue_id: issue.id, title: issue.title, project_id: fixture.projectId, creation_key: key, initial_issue: issue });
  expect(issue.information_workflow?.managed).toBe(false); expect(issue.status_category).not.toBe("blocked");
  if (caseName !== "C" && caseName !== "G") await policy(fixture, true);
  return fixture;
}

async function actionResponse(session: RecoverySession, predicate: (response: Response) => boolean, action: () => Promise<unknown>) {
  const pending = session.page.waitForResponse(predicate, { timeout: 15_000 });
  const [acted, received] = await Promise.allSettled([Promise.resolve().then(action), pending]);
  if (acted.status === "rejected") throw acted.reason;
  if (received.status === "rejected") throw received.reason;
  return received.value;
}

async function refresh(session: RecoverySession, fixture: InformationFixture) {
  const requests = new WeakSet<Request>(); let navigated = false;
  const navigation = (frame: Frame) => { if (frame === session.page.mainFrame()) navigated = true; };
  const request = (value: Request) => {
    if (navigated && value.method() === "GET" && new URL(value.url()).pathname === issuePath(fixture.issueId)) requests.add(value);
  };
  session.page.on("framenavigated", navigation); session.page.on("request", request);
  try {
    const response = await actionResponse(session, (value) => requests.has(value.request()) && value.status() === 200, () => session.page.reload());
    const raw: unknown = await response.json();
    fixture.evidence.checkpoint("actual_renderer_issue_read", { actor_id: session.actor.id, raw });
    const issue = CollaborationIssueSchema.parse(raw);
    expect(issue.id).toBe(fixture.issueId);
    await expect(session.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    return issue;
  } finally { session.page.off("framenavigated", navigation); session.page.off("request", request); }
}

const actionNames = { request_information: "Request information", return_information: "Return information",
  repair_information: "Repair information block", submit_acceptance: "Submit result" } as const;

async function openForm(session: RecoverySession, fixture: InformationFixture, action: InformationForm["action"]): Promise<InformationForm> {
  const baseline = await refresh(session, fixture), name = actionNames[action];
  const response = await actionResponse(session, (value) => new URL(value.url()).pathname === `${issuePath(fixture.issueId)}/status-options`
    && new URL(value.url()).searchParams.get("action") === action && value.request().method() === "GET", async () => {
    const button = session.page.getByRole("button", { name, exact: true });
    if (await button.isVisible()) await button.click();
    else { await revealCollaborationDisclosure(session.page, "More actions"); await session.page.getByRole("menuitem", { name, exact: true }).click(); }
  });
  const raw: unknown = await response.json();
  fixture.evidence.checkpoint("actual_renderer_status_options", { actor_id: session.actor.id, action, status: response.status(), raw });
  expect(response.status()).toBe(200);
  const value = CollaborationStatusOptionsSchema.parse(raw); expect(value.action).toBe(action);
  const form = session.page.getByRole("form", { name, exact: true }); await expect(form).toBeVisible();
  if (action === "repair_information") await expect(form.getByLabel("Status", { exact: true })).toHaveValue("");
  else await expect(form.getByLabel("Status", { exact: true })).toHaveValue(value.default_status);
  return { session, baseline, form, action, options: value };
}

async function settleOriginalUI(session: RecoverySession, fixture: InformationFixture, original: OriginalOperation) {
  await expect(session.page.getByText("The result is not confirmed. Keep this request and check its original outcome.", { exact: true })).toBeVisible();
  await refresh(session, fixture);
  await Promise.allSettled([...session.pendingReads]); const writes = session.writes.length;
  const recover = session.page.getByRole("button", { name: "Check original result", exact: true }).filter({ visible: true });
  await expect(recover).toHaveCount(1); await expect(recover).toBeEnabled();
  const path = `${issuePath(fixture.issueId)}/operations/${original.key}`;
  const response = await actionResponse(session, (value) => new URL(value.url()).pathname === path && value.request().method() === "GET", () => recover.click());
  const raw: unknown = await response.json();
  fixture.evidence.checkpoint("original_ui_get_recovered", { actor_id: session.actor.id, request_id: original.key,
    status: response.status(), raw, input: original.input });
  expect(response.status()).toBe(200); expect(CollaborationOperationSchema.parse(raw)).toEqual(original.receipt);
  await expect(recover).toHaveCount(0); await Promise.allSettled([...session.pendingReads]);
  expect(session.writes.slice(writes)).toEqual([]);
}

async function submitForm(fixture: InformationFixture, opened: InformationForm, code: string, loseReply = false): Promise<SentOperation> {
  const path = `${issuePath(fixture.issueId)}/${opened.action === "return_information" ? "information-returns"
    : opened.action === "submit_acceptance" ? "acceptance-submissions" : "handoffs"}`;
  const held = new HeldRequest(opened.session, path, "POST", `INFORMATION_UI_${opened.action}`);
  await held.install();
  let original: OriginalOperation;
  try {
    const name = opened.action === "submit_acceptance" ? "Submit result" : "Confirm";
    const button = opened.form.getByRole("button", { name, exact: true }); await expect(button).toBeEnabled(); await button.click();
    const fact = await held.captured(); fixture.evidence.checkpoint("original_ui_request_built", { fact });
    expect(fact.request_id).toMatch(/^[a-f0-9-]{36}$/); expect(fact.body).toMatchObject(observations(opened.baseline));
    expect(fact.body.status_options_version).toBe(opened.options.status_options_version);
    const result = await held.forward(loseReply);
    fixture.evidence.checkpoint("original_ui_request_completed", { fact, result,
      transport: loseReply ? "actual_upstream_commit_then_aborted_ack" : "unchanged_continue_actual_browser_response" });
    expect(result.status).toBe(200);
    const receipt = CollaborationOperationSchema.parse(result.body); expect(receipt.code).toBe(code);
    original = { actor: opened.session.actor, key: fact.request_id!, input: fact.body, receipt };
    fixture.operations.push(original);
  } finally { await held.close(); }
  if (loseReply) await settleOriginalUI(opened.session, fixture, original);
  else await expect(opened.form).toHaveCount(0);
  const after = await current(fixture); unchangedExecution(opened.baseline, after);
  return { ...original, before: opened.baseline, after };
}

function estimate() {
  // Test data uses one explicit +08:00 instant with retained microseconds.
  const local = new Date(Date.now() + 86_400_000 + 28_800_000).toISOString().slice(0, 19);
  const input = `${local}.123456+08:00`;
  return { input, stored: `${new Date(input).toISOString().slice(0, 19)}.123456Z` };
}

async function requestInformation(fixture: InformationFixture, session: RecoverySession, target: RecoveryActor,
  label: string, settings: { legacy?: boolean; requiredFieldsControl?: boolean } = {}) {
  const opened = await openForm(session, fixture, "request_information"), time = estimate();
  const reason = `Information ${label}: verify the recorded recovery decision.`, condition = `Provide the signed answer for ${label}.`;
  const confirm = opened.form.getByRole("button", { name: "Confirm", exact: true });
  await Promise.allSettled([...session.pendingReads]); const priorWrites = session.writes.length;
  if (settings.requiredFieldsControl) await expect(confirm).toBeDisabled();
  await selectHandoffRecipient(session.page, opened.form, target);
  if (settings.requiredFieldsControl) await expect(confirm).toBeDisabled();
  await opened.form.getByLabel(settings.legacy ? "Note" : "Reason", { exact: true }).fill(reason);
  if (!settings.legacy) {
    if (settings.requiredFieldsControl) await expect(confirm).toBeDisabled();
    await opened.form.getByLabel("Unblock condition", { exact: true }).fill(condition);
    if (settings.requiredFieldsControl) {
      await expect(confirm).toBeDisabled();
      await opened.form.getByLabel("Estimated resolution time", { exact: true }).fill("2026-11-20T18:00:00");
      await expect(opened.form.getByLabel("Estimated resolution time", { exact: true })).toHaveAttribute("aria-invalid", "true");
      await expect(confirm).toBeDisabled(); await Promise.allSettled([...session.pendingReads]);
      const observedRequests = session.writes.slice(priorWrites);
      const unexpectedWrites = observedRequests.filter((fact) => fact.method !== "POST" || fact.request_id !== null ||
        (fact.path !== "/api/collaboration/recipients/search" && fact.path !== "/api/collaboration/recipients/resolve"));
      fixture.evidence.checkpoint("mandatory_four_fields_ui_negative", { reason, condition, invalid_estimate: "timezone_absent",
        observed_requests: observedRequests, no_business_write: unexpectedWrites.length === 0, actor_id: session.actor.id });
      expect(unexpectedWrites).toEqual([]);
    }
    await opened.form.getByLabel("Estimated resolution time", { exact: true }).fill(time.input);
  }
  const sent = await submitForm(fixture, opened, "information_requested");
  const request = sent.after.current_information_request;
  expect(request).not.toBeNull();
  if (!request) throw new Error("The successful request has no recorded current information identity");
  fixture.requestIds.push(request.id);
  expect(request).toMatchObject({ requester: { user_id: session.actor.id }, recipient: { user_id: target.id },
    note: reason });
  const recorded = (await historyPage(fixture, fixture.creator, null, 50)).items.find((item) => item.handoff_id === request.id);
  if (!recorded || !collaborationInformationHistoryHasDetails(recorded) || recorded.kind !== "information_request") {
    throw new Error("The current request has no exact public original history record");
  }
  expect(recorded).toMatchObject({ actor_user_id: session.actor.id, requester_user_id: session.actor.id,
    to_user_id: target.id, previous_status: sent.before.status, note: reason, block: request.block });
  expect(sent.after.status_category).toBe("blocked"); expect(sent.after.handler.user_id).toBe(target.id);
  if (settings.legacy) {
    expect(request.block).toEqual({ protocol_version: 1, state: "legacy" });
    expect(sent.input).not.toHaveProperty("unblock_condition"); expect(sent.input).not.toHaveProperty("estimated_resolution_at");
  } else {
    expect(sent.input).toMatchObject({ purpose: "information", target_user_id: target.id, note: reason,
      unblock_condition: condition, estimated_resolution_at: time.input });
    expect(request.block).toEqual({ protocol_version: 1, state: "complete", unblock_condition: condition, estimated_resolution_at: time.stored });
  }
  fixture.evidence.checkpoint("recorded_information_request", { label, request, recorded, source_status: sent.before.status,
    current_status: sent.after.status, current_handler: sent.after.handler.user_id, input_estimate: settings.legacy ? null : time.input });
  return { ...sent, request, recorded };
}

async function returnInformation(fixture: InformationFixture, session: RecoverySession, requestId: string, expectedStatus: string) {
  const opened = await openForm(session, fixture, "return_information");
  expect(opened.baseline.current_information_request?.id).toBe(requestId);
  expect(opened.options.default_status).toBe(expectedStatus);
  await opened.form.getByLabel("Note", { exact: true }).fill(`Answer to original request ${requestId}.`);
  const sent = await submitForm(fixture, opened, "information_returned");
  expect(sent.input.information_request_id).toBe(requestId); expect(sent.after.status).toBe(expectedStatus);
  const page = await historyPage(fixture, fixture.creator, null, 50);
  const returned = page.items.find((item) => collaborationInformationHistoryHasDetails(item) && item.kind === "information_return" && item.consumed_request_id === requestId);
  expect(returned).toBeDefined();
  if (!returned || !collaborationInformationHistoryHasDetails(returned) || returned.kind !== "information_return") throw new Error("The real return has no exact recorded handoff");
  expect(returned).toMatchObject({ actor_user_id: session.actor.id, restored_status: expectedStatus, return_status_fallback: false });
  fixture.returnIds.push(returned.handoff_id); fixture.evidence.checkpoint("recorded_information_return", { returned, original: sent.receipt });
  return sent;
}

async function refused(fixture: InformationFixture, actor: RecoveryActor, path: string, input: JSONRecord, code: string) {
  const key = randomUUID(), before = await current(fixture);
  const result = await api(fixture, actor, path, "POST", input, key);
  expect(result.status).toBe(409);
  const receipt = CollaborationOperationSchema.parse(result.body); expect(receipt.code).toBe(code);
  fixture.operations.push({ actor, key, input, receipt });
  expect(await current(fixture)).toEqual(before);
  return { key, receipt };
}

async function refuseBlockedB2(fixture: InformationFixture, actor: RecoveryActor, target: RecoveryActor) {
  const issue = await current(fixture, actor), offered = await options(fixture, actor, "request_information");
  const time = estimate();
  expect(issue.information_workflow?.actions.request).toEqual({ available: false, code: "information_repair_required" });
  return refused(fixture, actor, `${issuePath(fixture.issueId)}/handoffs`, { ...observations(issue), purpose: "information",
    target_user_id: target.id, note: "This parentless blocked B2 must retain its refusal.", unblock_condition: "Obtain the missing answer.",
    estimated_resolution_at: time.input, expected_status: offered.default_status, status_options_version: offered.status_options_version }, "information_repair_required");
}

async function remove(fixture: InformationFixture, target: RecoveryActor) {
  const issue = await current(fixture), input = observations(issue), key = randomUUID();
  const result = await api(fixture, fixture.creator, `${issuePath(fixture.issueId)}/participants/${target.id}/remove`, "POST", input, key);
  expect(result.status).toBe(200);
  const receipt = CollaborationOperationSchema.parse(result.body); expect(receipt.code).toBe("participant_removed");
  fixture.operations.push({ actor: fixture.creator, key, input, receipt });
  const after = await current(fixture); unchangedExecution(issue, after); expect(after.status_category).toBe("blocked");
  return after;
}

async function repair(fixture: InformationFixture, session: RecoverySession, basisKind: CollaborationInformationRepairBasis["kind"],
  target: RecoveryActor, expectedCount: string, loseReply = false) {
  const opened = await openForm(session, fixture, "repair_information");
  if (opened.options.action !== "repair_information") throw new Error("Repair did not return its closed status-options variant");
  const context = opened.options.information_repair_context;
  let basis: CollaborationInformationRepairBasis, selected: string;
  if (basisKind === "pending_submission") {
    if (!context.pending_submission) throw new Error("The genuine pending submission was not offered");
    expect(context.human_selection_allowed).toBe(false);
    await expect(opened.form.getByRole("radio", { name: "Choose a working status", exact: true })).toHaveCount(0);
    await opened.form.getByRole("radio", { name: /^Restore this pending submission/ }).check();
    await opened.form.getByRole("button", { name: "Use the creator", exact: true }).click();
    basis = { kind: basisKind, submission_id: context.pending_submission.submission_id, submission_event_id: context.pending_submission.submission_event_id };
    expect(context.pending_submission.creator_user_id).toBe(target.id);
    selected = opened.options.options.find((item) => item.category === "in_review")!.key;
  } else if (basisKind === "historical_information") {
    if (!context.historical_origin) throw new Error("The actual original historical request was not offered");
    await opened.form.getByRole("radio", { name: /^Restore the recorded status:/ }).check();
    basis = { kind: basisKind, origin_request_id: context.historical_origin.origin_request_id, origin_event_id: context.historical_origin.origin_event_id };
    selected = context.historical_origin.status;
    await selectHandoffRecipient(session.page, opened.form, target);
  } else {
    await opened.form.getByRole("radio", { name: "Choose a working status", exact: true }).check();
    basis = { kind: "human_selected" };
    selected = opened.options.options.find((item) => item.category === "todo")!.key;
    if (session.actor.id === target.id) await opened.form.getByRole("button", { name: "Assign the repaired issue to me", exact: true }).click();
    else await selectHandoffRecipient(session.page, opened.form, target);
  }
  await opened.form.getByLabel("Status", { exact: true }).selectOption(selected);
  await opened.form.getByLabel("Repair reason", { exact: true }).fill("Restore the actual information block explicitly before any later request.");
  const sent = await submitForm(fixture, opened, "information_repaired", loseReply);
  expect(sent.input).toMatchObject({ purpose: "information_repair", target_user_id: target.id, expected_status: selected,
    information_repair: { context_version: context.context_version, active_request_id: context.active_request_id, basis } });
  expect(sent.input).not.toHaveProperty("unblock_condition"); expect(sent.input).not.toHaveProperty("estimated_resolution_at");
  const confirmation = sent.receipt.information_repair_confirmation;
  expect(confirmation).toMatchObject({ prior_active_request_id: context.active_request_id, status: selected, handler_user_id: target.id, superseded_count: expectedCount });
  if (!confirmation) throw new Error("The completed repair has no exact confirmation");
  fixture.repairIds.push(confirmation.repair_handoff_id);
  expect(sent.after.current_information_request).toBeNull(); expect(sent.after.information_workflow?.active_request_id).toBeNull();
  expect(sent.after.status).toBe(selected); expect(sent.after.status_category).not.toBe("blocked"); expect(sent.after.handler.user_id).toBe(target.id);
  expect(sent.after.delivery).toEqual(sent.before.delivery);
  fixture.evidence.checkpoint("atomic_repair_before_separate_B2", { original_key: sent.key, confirmation, basis,
    original_context: context, after: sent.after, no_automatic_B2: true });
  return { ...sent, confirmation, basis };
}

async function historyPage(fixture: InformationFixture, actor: RecoveryActor, cursor: string | null, limit: number) {
  const query = new URLSearchParams({ limit: String(limit) }); if (cursor !== null) query.set("cursor", cursor);
  const result = await api(fixture, actor, `${issuePath(fixture.issueId)}/handoffs?${query}`);
  expect(result.status).toBe(200); expect(result.response_bytes).toBeLessThanOrEqual(65536); expect(result.cache_control).toBe("no-store");
  const page = CollaborationInformationHistorySchema.parse(result.body); expect(page.issue_id).toBe(fixture.issueId);
  return page;
}

async function completeHistory(fixture: InformationFixture) {
  const pages: ReturnType<typeof CollaborationInformationHistorySchema.parse>[] = [], items: CollaborationInformationHistoryItem[] = [];
  const seen = new Set<string>(); let cursor: string | null = null;
  const expectedIds = [...fixture.requestIds, ...fixture.returnIds, ...fixture.repairIds];
  do {
    const page = await historyPage(fixture, fixture.creator, cursor, 1); pages.push(page); items.push(...page.items);
    if (pages.length > expectedIds.length + 1) throw new Error("Information history exceeded its complete planned identity denominator");
    if (page.next_cursor !== null) { expect(seen.has(page.next_cursor)).toBe(false); seen.add(page.next_cursor); }
    cursor = page.next_cursor;
  } while (cursor !== null);
  expect(items.map((item) => item.handoff_id).sort()).toEqual(expectedIds.sort());
  expect(new Set(items.map((item) => item.handoff_id)).size).toBe(items.length);
  const first = pages[0]!;
  for (const page of pages) expect({ revision: page.collaboration_revision, lifecycle: page.lifecycle_epoch, upper: page.upper_handoff })
    .toEqual({ revision: first.collaboration_revision, lifecycle: first.lifecycle_epoch, upper: first.upper_handoff });
  for (let index = 1; index < items.length; index += 1) expect(compareCollaborationInformationTuple(items[index - 1]!, items[index]!)).toBeGreaterThan(0);
  fixture.evidence.checkpoint("complete_quiescent_history_limit_one", { pages, items, expected_ids: expectedIds });
  return items;
}

async function historyUI(fixture: InformationFixture, session: RecoverySession, items: CollaborationInformationHistoryItem[]) {
  await refresh(session, fixture);
  const drawer = await revealCollaborationDisclosure(session.page, "Activity");
  const summary = drawer.locator("details > summary").filter({ hasText: /^Information handoff history$/ });
  const response = await actionResponse(session, (value) => new URL(value.url()).pathname === `${issuePath(fixture.issueId)}/handoffs`
    && value.request().method() === "GET", () => summary.click());
  const raw: unknown = await response.json();
  fixture.evidence.checkpoint("actual_ui_information_history_page", { actor_id: session.actor.id, status: response.status(), raw });
  expect(response.status()).toBe(200); expect(CollaborationInformationHistorySchema.parse(raw).items).toEqual(items);
  const region = drawer.getByRole("region", { name: "Information handoff history", exact: true });
  const rows = region.locator("[data-information-handoff-id]");
  await expect(rows).toHaveCount(items.length);
  expect(await rows.evaluateAll((nodes) => nodes.map((node) => node.getAttribute("data-information-handoff-id")))).toEqual(items.map((item) => item.handoff_id));
  for (const item of items) {
    const row = region.locator(`[data-information-handoff-id="${item.handoff_id}"]`);
    await expect(row.locator("time").first()).toHaveAttribute("datetime", item.created_at);
    if (!collaborationInformationHistoryHasDetails(item)) throw new Error("This public fixture unexpectedly lost readable original history");
    await expect(row).toContainText(item.note); await expect(row).toContainText(item.actor_user_id);
    await row.locator("details > summary").filter({ hasText: /^Record identity$/ }).click();
    if (item.provenance.state === "linked") await expect(row).toContainText(item.provenance.event_id);
    if (item.kind === "information_request" && item.block.state === "complete") {
      await expect(row).toContainText(item.block.unblock_condition);
      await expect(row.locator(`time[datetime="${item.block.estimated_resolution_at}"]`)).toHaveCount(1);
    }
    if (item.kind !== "information_repair") continue;
    const path = `${issuePath(fixture.issueId)}/handoffs/${item.handoff_id}/superseded-requests`;
    const affectedResponse = await actionResponse(session, (value) => new URL(value.url()).pathname === path && value.request().method() === "GET",
      () => row.locator("details > summary").filter({ hasText: /^Affected requests$/ }).click());
    const affectedRaw: unknown = await affectedResponse.json();
    fixture.evidence.checkpoint("actual_ui_affected_requests", { repair_id: item.handoff_id, status: affectedResponse.status(), raw: affectedRaw });
    expect(affectedResponse.status()).toBe(200);
    const affected = CollaborationInformationAffectedSchema.parse(affectedRaw);
    expect(affected.superseded_request_count).toBe(item.superseded_request_count); expect(affected.next_cursor).toBeNull();
    const exactRequests = items.filter((value) => collaborationInformationHistoryHasDetails(value) && value.kind === "information_request" && value.superseded_by_repair_handoff_id === item.handoff_id);
    expect(affected.items.map((value) => value.request_id).sort()).toEqual(exactRequests.map((value) => value.handoff_id).sort());
    const affectedRegion = row.getByRole("region", { name: "Affected requests", exact: true });
    await expect(affectedRegion.getByRole("status").filter({ hasText: `All ${item.superseded_request_count} affected request identities are loaded.` })).toBeVisible();
    await expect(affectedRegion.locator("[data-information-request-id]")).toHaveCount(Number(item.superseded_request_count));
    if (item.prior_active_request_id !== null) {
      const originalPath = `${issuePath(fixture.issueId)}/handoffs/${item.prior_active_request_id}`;
      const detailResponse = await actionResponse(session, (value) => new URL(value.url()).pathname === originalPath && value.request().method() === "GET",
        () => row.getByText("Previous active request", { exact: true }).locator("..").getByRole("button", { name: "View the original record", exact: true }).click());
      const detailRaw: unknown = await detailResponse.json();
      fixture.evidence.checkpoint("actual_ui_original_record_link", { repair_id: item.handoff_id, status: detailResponse.status(), raw: detailRaw });
      expect(detailResponse.status()).toBe(200);
      expect(CollaborationInformationDetailSchema.parse(detailRaw).item).toEqual(items.find((value) => value.handoff_id === item.prior_active_request_id));
      await region.getByRole("button", { name: "Close the original record", exact: true }).click();
    }
  }
  await drawer.getByRole("button", { name: "Close resources", exact: true }).click();
}

async function originalsRemain(fixture: InformationFixture) {
  for (const original of fixture.operations) {
    const result = await api(fixture, original.actor, `${issuePath(fixture.issueId)}/operations/${original.key}`);
    expect(result.status).toBe(200); expect(CollaborationOperationSchema.parse(result.body)).toEqual(original.receipt);
    fixture.evidence.checkpoint("immutable_original_operation_after_later_intents", { actor_id: original.actor.id, request_id: original.key,
      original_input_sha256: digest(original.input), original_receipt: original.receipt, recovered: result.body });
  }
}

async function laterB2(fixture: InformationFixture, sender: RecoverySession, recipient: RecoveryActor,
  receivingSession: () => Promise<RecoverySession>) {
  const before = await current(fixture); expect(before.status_category).not.toBe("blocked");
  const requested = await requestInformation(fixture, sender, recipient, "independent later B2");
  expect(requested.recorded.parent_request_id).toBeNull(); expect(requested.recorded.previous_status).toBe(before.status);
  const receiver = await receivingSession(); expect(receiver.actor.id).toBe(recipient.id);
  await returnInformation(fixture, receiver, requested.request.id, before.status);
  const after = await current(fixture); expect(after.current_information_request).toBeNull();
  expect(after.handler.user_id).toBe(sender.actor.id); expect(after.delivery).toEqual(before.delivery);
  return requested;
}

async function privateHistoryDenied(fixture: InformationFixture, removed: RecoveryActor, originalId: string, repairId: string) {
  for (const path of [`${issuePath(fixture.issueId)}/handoffs?limit=1`, `${issuePath(fixture.issueId)}/handoffs/${originalId}`,
    `${issuePath(fixture.issueId)}/handoffs/${repairId}/superseded-requests?limit=1`]) {
    const result = await api(fixture, removed, path); expect(result.status).toBe(404); expect(result.cache_control).toBe("no-store");
    expect(JSON.stringify(result.body)).not.toContain("verify the recorded recovery decision");
  }
}

export async function exerciseInformationRecovery(browser: Browser, platform: RecoveryPlatform, info: TestInfo,
  evidence: RecoveryEvidence, caseName: InformationRecoveryCase) {
  const fixture = await seedInformation(caseName, evidence);
  const outcome = await withSessions(browser, platform, fixture, info, evidence, async (open) => {
    const sessions: RecoverySession[] = [];
    const openTracked = async (actor: RecoveryActor) => { const session = await open(actor); sessions.push(session); return session; };
    const creator = await openTracked(fixture.creator), handler = await openTracked(fixture.handler);
    const baseline = await current(fixture);
    evidence.set({ active_phase: `${caseName}_real_information_sequence` });
    if (caseName === "A") {
      const first = await requestInformation(fixture, handler, fixture.qa, "A H to QA", { requiredFieldsControl: true });
      expect(first.request.requester.user_id).toBe(fixture.handler.id);
      expect(first.request.recipient.user_id).not.toBe(fixture.creator.id);
      await refuseBlockedB2(fixture, fixture.creator, fixture.handler);
      const repaired = await repair(fixture, creator, "historical_information", fixture.creator, "1", true);
      expect(repaired.basis).toMatchObject({ origin_request_id: first.request.id });
      await laterB2(fixture, creator, fixture.handler, async () => handler);
    } else if (caseName === "B") {
      const first = await requestInformation(fixture, handler, fixture.qa, "B recipient removal");
      const after = await remove(fixture, fixture.qa);
      expect(after.current_information_request).toBeNull(); expect(after.information_workflow?.active_request_id).toBeNull();
      const old = (await historyPage(fixture, fixture.creator, null, 50)).items.find((item) => item.handoff_id === first.request.id);
      expect(old).toMatchObject({ resolution: "superseded", superseded_by_repair_handoff_id: null });
      await refuseBlockedB2(fixture, fixture.creator, fixture.handler);
      await repair(fixture, creator, "human_selected", fixture.creator, "0");
      await laterB2(fixture, creator, fixture.handler, async () => handler);
    } else if (caseName === "C") {
      const first = await requestInformation(fixture, handler, fixture.creator, "C old unmanaged request", { legacy: true });
      const oldReturn = await options(fixture, fixture.creator, "return_information");
      await policy(fixture, true);
      const managed = await current(fixture); expect(managed.information_workflow).toMatchObject({ managed: true, record_state: "legacy" });
      expect(managed.information_workflow?.actions.return).toEqual({ available: false, code: "information_repair_required" });
      await refused(fixture, fixture.creator, `${issuePath(fixture.issueId)}/information-returns`, { ...observations(managed),
        information_request_id: first.request.id, note: "The original legacy request cannot become a complete managed return.",
        expected_status: oldReturn.default_status, status_options_version: oldReturn.status_options_version }, "information_repair_required");
      await refuseBlockedB2(fixture, fixture.creator, fixture.handler);
      const creationKey = randomUUID(), blockedInput = { source_workspace_id: fixture.workspace.id, project_id: fixture.projectId,
        handler_user_id: fixture.handler.id, title: "Managed initial blocked must be refused", status: "blocked", attachment_ids: [] };
      const blocked = await api(fixture, fixture.creator, "/api/collaboration/issues", "POST", blockedInput, creationKey);
      expect(blocked.status).toBe(409); expect(object.parse(blocked.body).code).toBe("status_unavailable");
      const saved = await api(fixture, fixture.creator, `/api/collaboration/creations/${creationKey}/result?source_workspace_id=${fixture.workspace.id}`);
      expect(saved.status).toBe(200); expect(CollaborationCreationResultSchema.parse(saved.body)).toMatchObject({ outcome: "rejected", code: "status_unavailable", request_id: creationKey });
      await repair(fixture, creator, "human_selected", fixture.creator, "1");
      await laterB2(fixture, creator, fixture.handler, async () => handler);
      const old = (await historyPage(fixture, fixture.creator, null, 50)).items.find((item) => item.handoff_id === first.request.id);
      expect(old).toMatchObject({ previous_status: first.before.status, block: { protocol_version: 1, state: "legacy" } });
    } else if (caseName === "E") {
      const first = await requestInformation(fixture, handler, fixture.creator, "E unavailable requester");
      const oldOptions = await options(fixture, fixture.creator, "return_information");
      const after = await remove(fixture, fixture.handler); expect(after.handler.user_id).toBe(fixture.creator.id);
      await refused(fixture, fixture.creator, `${issuePath(fixture.issueId)}/information-returns`, { ...observations(after),
        information_request_id: first.request.id, note: "Do not reenroll the unavailable requester.", expected_status: oldOptions.default_status,
        status_options_version: oldOptions.status_options_version }, "return_target_unavailable");
      const repaired = await repair(fixture, creator, "human_selected", fixture.creator, "1");
      await laterB2(fixture, creator, fixture.qa, () => openTracked(fixture.qa));
      await privateHistoryDenied(fixture, fixture.handler, first.request.id, repaired.confirmation.repair_handoff_id);
      await handler.page.reload(); await expect(handler.page.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
      await expect(handler.page.getByRole("heading", { name: fixture.title, exact: true })).toHaveCount(0);
      const item = (await historyPage(fixture, fixture.creator, null, 50)).items.find((value) => value.handoff_id === repaired.confirmation.repair_handoff_id);
      expect(item).toMatchObject({ from_user_id: fixture.creator.id, to_user_id: fixture.creator.id, actor_user_id: fixture.creator.id });
    } else if (caseName === "F") {
      const first = await requestInformation(fixture, handler, fixture.creator, "F first complete layer");
      const second = await requestInformation(fixture, creator, fixture.qa, "F nested complete layer");
      const qa = await openTracked(fixture.qa);
      expect(second.recorded.parent_request_id).toBe(first.request.id); expect(second.recorded.previous_status).toBe(first.after.status);
      await returnInformation(fixture, qa, second.request.id, first.after.status);
      const reactivated = await current(fixture); expect(reactivated.current_information_request?.id).toBe(first.request.id);
      expect(reactivated.status_category).toBe("blocked"); expect(reactivated.handler.user_id).toBe(fixture.creator.id);
      await returnInformation(fixture, creator, first.request.id, first.before.status);
      expect(fixture.repairIds).toEqual([]);
    } else {
      const opened = await openForm(handler, fixture, "submit_acceptance");
      await opened.form.getByLabel("Result", { exact: true }).fill("G original genuine pending result, retained without acceptance.");
      await opened.form.getByLabel("Basis", { exact: true }).fill("Public unmanaged submission before explicit policy activation.");
      const submitted = await submitForm(fixture, opened, "acceptance_submitted"), pending = submitted.after.delivery?.current;
      expect(pending?.evidence_state).toBe("available"); expect(pending?.decision).toBeNull();
      if (!pending) throw new Error("G has no genuine public pending result");
      await policy(fixture, true);
      const first = await requestInformation(fixture, creator, fixture.handler, "G pending review information");
      expect(first.recorded.previous_status).toBe(submitted.after.status); expect(first.after.delivery?.current).toEqual(pending);
      const observed = await current(fixture), offered = await options(fixture, fixture.creator, "repair_information");
      if (offered.action !== "repair_information" || !offered.information_repair_context.pending_submission) throw new Error("G repair context lost the exact submission/event");
      const context = offered.information_repair_context, exact = context.pending_submission!;
      expect(exact.submission_id).toBe(pending.id);
      const selected = offered.options.find((item) => item.category === "in_review")!.key;
      const repairInput = { ...observations(observed), purpose: "information_repair", target_user_id: fixture.creator.id,
        note: "Preserve G's original pending submission and review authority.", expected_status: selected, status_options_version: offered.status_options_version,
        information_repair: { context_version: context.context_version, active_request_id: context.active_request_id,
          basis: { kind: "pending_submission", submission_id: exact.submission_id, submission_event_id: exact.submission_event_id } } };
      await refused(fixture, fixture.creator, `${issuePath(fixture.issueId)}/handoffs`, { ...repairInput, expected_status: "todo" }, "status_unavailable");
      await refused(fixture, fixture.creator, `${issuePath(fixture.issueId)}/handoffs`, { ...repairInput,
        information_repair: { ...repairInput.information_repair, basis: { kind: "human_selected" } } }, "information_repair_basis_unavailable");
      await refused(fixture, fixture.creator, `${issuePath(fixture.issueId)}/handoffs`, { ...repairInput, target_user_id: fixture.handler.id }, "information_repair_basis_unavailable");
      const repaired = await repair(fixture, creator, "pending_submission", fixture.creator, "1");
      expect(repaired.basis).toEqual(repairInput.information_repair.basis);
      expect(repaired.after.status_category).toBe("in_review"); expect(repaired.after.delivery?.current).toEqual(pending);
      await laterB2(fixture, creator, fixture.handler, async () => handler);
      expect((await current(fixture)).delivery?.current).toEqual(pending);
      evidence.checkpoint("G_exact_pending_result_preserved", { pending, original_submission_key: submitted.key, exact_submission_event: exact,
        policy_scope: "existing_unmanaged_pending_then_managed", no_result_decision: true });
    }
    const final = await current(fixture); unchangedExecution(baseline, final);
    expect(final.current_information_request).toBeNull(); expect(final.status_category).not.toBe("blocked");
    expect(new Set(fixture.operations.map((item) => item.key)).size).toBe(fixture.operations.length);
    const history = await completeHistory(fixture); await historyUI(fixture, creator, history); await originalsRemain(fixture);
    const forbiddenWrites = fixture.http.filter((fact) => fact.method !== "GET" && /\/(?:commands|inputs|resume|stage-advances|stage-decisions|completion-approvals)(?:\/|$)/.test(fact.path));
    expect(forbiddenWrites).toEqual([]);
    for (const session of sessions) {
      await Promise.allSettled([...session.pendingReads]);
      expect(session.writes.filter((fact) => /\/(?:commands|inputs|resume|stage-advances|stage-decisions|completion-approvals)(?:\/|$)/.test(fact.path))).toEqual([]);
    }
    const allPosts = [...fixture.http.filter((fact) => fact.method === "POST"), ...sessions.flatMap((session) => session.writes.filter((fact) => fact.method === "POST"))];
    for (const original of fixture.operations) expect(allPosts.filter((fact) => fact.request_id === original.key)).toHaveLength(1);
    evidence.checkpoint("complete_information_case", { final_issue: final, request_ids: fixture.requestIds,
      return_ids: fixture.returnIds, repair_ids: fixture.repairIds, original_operation_keys: fixture.operations.map((item) => item.key),
      exact_history: history, http: fixture.http, no_new_execution: true });
    return { case: caseName, issue_id: fixture.issueId, setup_issue_id: fixture.setupIssueId, project_id: fixture.projectId,
      request_ids: fixture.requestIds, return_ids: fixture.returnIds, repair_ids: fixture.repairIds,
      original_operation_keys: fixture.operations.map((item) => item.key), final_issue: final };
  });
  return outcome;
}
