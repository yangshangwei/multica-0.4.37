import { randomUUID } from "node:crypto";
import { expect, type Browser, type Page } from "@playwright/test";
import type {
  CollaborationCommandHistory, CollaborationCommentReceipt, CollaborationIssue, CollaborationMentionTargets,
  CollaborationOperation, CollaborationResources, CollaborationTriggerPreview,
} from "@multica/core/collaboration";
import type { TestApiClient } from "../fixtures";
import { consentControlTarget, type CollaborationControlJourney, type ControlTarget } from "./collaboration-control-journey";

/** Gap journeys reuse the public control seed. Every write below is a real
 * client UI action or a deliberate direct refusal probe; no SQL is written. */
export type GapRole = "creator" | "handler" | "owner";
export interface GapSession { role: GapRole; page: Page; errors: string[]; close(): Promise<void> }
const gapIssuePath = (issueId: string) => `/api/collaboration/issues/${issueId}`;
export const gapAccountPath = (issueId: string) => `/account?view=participating&issue=${issueId}`;

export function gapClient(fixture: CollaborationControlJourney, role: GapRole): TestApiClient {
  return role === "creator" ? fixture.creator : role === "handler" ? fixture.handler : fixture.owner;
}
export function gapUserId(fixture: CollaborationControlJourney, role: GapRole): string {
  return role === "creator" ? fixture.creatorId : role === "handler" ? fixture.handlerId : fixture.ownerId;
}

/** One isolated browser context per account, so cookies never cross actors. */
export async function openGapSession(browser: Browser, fixture: CollaborationControlJourney, role: GapRole, webUrl: string): Promise<GapSession> {
  expect(new URL(webUrl).hostname).toBe(new URL(fixture.apiUrl).hostname);
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await (role === "creator" ? fixture.authenticateWeb : role === "handler" ? fixture.authenticateHandlerWeb : fixture.authenticateOwnerWeb)(page);
  await context.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
  await page.goto(new URL(gapAccountPath(fixture.issueId), webUrl).href);
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  return { role, page, errors, close: () => context.close() };
}

export async function readGapIssue(fixture: CollaborationControlJourney, role: GapRole = "creator") {
  const issue = await gapClient(fixture, role).requestJSON<CollaborationIssue>(gapIssuePath(fixture.issueId));
  if (!issue.execution) throw new Error("The gap journey requires the execution projection");
  expect(issue.execution.channels).toHaveLength(3);
  expect(issue.execution.targets).toHaveLength(3);
  return issue as CollaborationIssue & { execution: NonNullable<CollaborationIssue["execution"]> };
}

/** A direct request for refusal probes. Status and body are returned unparsed. */
export async function rawGapRequest(fixture: CollaborationControlJourney, role: GapRole, method: "GET" | "POST", path: string, body?: unknown) {
  const requestId = method === "POST" ? randomUUID() : null;
  const response = await fetch(`${fixture.apiUrl}${path}`, { method, headers: { Authorization: `Bearer ${gapClient(fixture, role).getToken()!}`,
    "Content-Type": "application/json", ...(requestId ? { "Idempotency-Key": requestId } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await response.text();
  return { status: response.status, body: (text ? JSON.parse(text) : null) as Record<string, unknown> | null, requestId };
}

/** The four observations a workflow write needs, read as the writing actor. */
export function gapWorkflowObservations(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("A workflow write requires the actor's current participant observations");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}

/** The same Stop/Resume body the shared control client prepares
 * (packages/core/collaboration/control.ts:15-38), built from a fresh read. */
export function gapControlBody(issue: CollaborationIssue, action: "stop" | "resume", scope: "issue" | "target", targetId?: string) {
  if (!issue.execution || !issue.write_context) throw new Error("A control probe requires the current execution observations");
  const execution = issue.execution;
  const selected = scope === "issue" ? execution.targets : execution.targets.filter((target) => target.id === targetId);
  expect(selected.length).toBeGreaterThan(0);
  return { action, scope, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch,
    ...(scope === "issue" ? { expected_control_version: execution.control.control_version, expected_target_set_version: execution.target_set_version } : {}),
    ...(action === "resume" ? { expected_execution_epoch: execution.execution_gate.epoch } : {}),
    targets: selected.map((target) => ({ target_id: target.id, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version,
      expected_control_version: target.control.control_version, observed_group_id: target.active_group_id,
      observed_group_generation: target.group_generation, ...(action === "resume" ? { expected_hold_version: target.hold_version } : {}) })) };
}
export async function reloadGapPage(page: Page, issueId: string, title: string) {
  const fetched = page.waitForResponse((response) => new URL(response.url()).pathname === gapIssuePath(issueId) && response.request().method() === "GET");
  await page.reload();
  expect((await fetched).status()).toBe(200);
  await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
}

export interface GapComment {
  marker: string; content: string; requestId: string; receipt: CollaborationCommentReceipt; preview: CollaborationTriggerPreview;
}
/** Composer steps a person takes: type, pick AI targets by name, preview, send.
 * The composer is a plain textarea, so no editor readiness wait is needed. */
export async function commentThroughUI(page: Page, issueId: string, marker: string, targetNames: string[] = [], prefix = ""): Promise<GapComment> {
  const form = page.getByRole("form", { name: "Comments", exact: true });
  const field = form.getByLabel("Add comment", { exact: true });
  await field.fill(`${prefix}${marker}`);
  await expect(field).toHaveValue(`${prefix}${marker}`);
  for (const name of targetNames) await form.getByRole("group", { name: "AI targets", exact: true }).getByRole("button", { name, exact: true }).click();
  const content = await field.inputValue();
  const previewPending = page.waitForResponse((response) => new URL(response.url()).pathname === `${gapIssuePath(issueId)}/comments/trigger-preview`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Preview delivery", exact: true }).click();
  const previewResponse = await previewPending;
  expect(previewResponse.status()).toBe(200);
  const preview: CollaborationTriggerPreview = await previewResponse.json();
  const sendPending = page.waitForResponse((response) => new URL(response.url()).pathname === `${gapIssuePath(issueId)}/comments`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Send comment", exact: true }).click();
  const response = await sendPending;
  expect(response.status()).toBe(201);
  const receipt: CollaborationCommentReceipt = await response.json();
  const requestId = response.request().headers()["idempotency-key"];
  if (!requestId) throw new Error("The composer omitted its original request key");
  expect(response.request().postDataJSON()).toMatchObject({ content });
  expect(receipt).toMatchObject({ issue_id: issueId, status: 201, code: "comment_created" });
  await expect(form.getByRole("status").filter({ hasText: "Comment confirmed." })).toBeVisible();
  return { marker, content, requestId, receipt, preview };
}

/** One immutable input the UI comment queued for an exact target. */
export function queuedInput(comment: GapComment, target: ControlTarget) {
  const result = comment.receipt.targets.find((item) => item.target_id === target.targetId);
  if (!result?.command_id || !result.batch_id || !result.command_sequence) throw new Error("The UI comment queued no input for the selected target");
  expect(result).toMatchObject({ state: "waiting", channel_id: target.channelId });
  return { commandId: result.command_id, batchId: result.batch_id, sequence: result.command_sequence, marker: comment.marker, content: comment.content };
}

export async function mentionNames(client: TestApiClient, issueId: string) {
  const catalogue = await client.requestJSON<CollaborationMentionTargets>(`${gapIssuePath(issueId)}/mention-targets`);
  return { catalogue, nameOf(target: ControlTarget) {
    const found = catalogue.targets.find((item) => item.target_id === target.targetId);
    if (!found) throw new Error("The target is not in this actor's authorized @ catalogue");
    return found.name;
  } };
}

/** Complete, cancel or reopen through the shared workflow form. */
export async function lifecycleThroughUI(page: Page, issueId: string, action: "complete" | "cancel" | "reopen") {
  const label = { complete: "Complete issue", cancel: "Cancel issue", reopen: "Reopen issue" }[action];
  const optionsPending = page.waitForResponse((response) => new URL(response.url()).pathname === `${gapIssuePath(issueId)}/status-options`
    && new URL(response.url()).searchParams.get("action") === action && response.request().method() === "GET");
  await page.getByRole("button", { name: label, exact: true }).click();
  const optionsResponse = await optionsPending;
  expect(optionsResponse.status()).toBe(200);
  const options: { default_status: string; status_options_version: string; options: { key: string; category: string }[] } = await optionsResponse.json();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: label, exact: true }) });
  await form.getByLabel("Status", { exact: true }).selectOption(options.default_status);
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${gapIssuePath(issueId)}/lifecycle` && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe({ complete: "completed", cancel: "cancelled", reopen: "reopened" }[action]);
  const requestId = response.request().headers()["idempotency-key"];
  if (!requestId) throw new Error("The lifecycle form omitted its original request key");
  const input: Record<string, unknown> = response.request().postDataJSON();
  expect(input).toMatchObject({ action, status: options.default_status, status_options_version: options.status_options_version });
  await expect(form).toHaveCount(0);
  return { receipt, requestId, input, options };
}
export async function leaveThroughUI(page: Page, issueId: string) {
  await page.getByRole("button", { name: "Leave issue", exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Leave issue", exact: true }) });
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${gapIssuePath(issueId)}/leave` && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  const requestId = response.request().headers()["idempotency-key"];
  if (!requestId) throw new Error("Leave omitted its original request key");
  return { receipt, requestId, input: response.request().postDataJSON() as Record<string, unknown> };
}

export async function inviteThroughUI(page: Page, issueId: string, target: { id: string; name: string }) {
  await page.getByRole("button", { name: "Add participant", exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Add participant", exact: true }) });
  await form.getByLabel("Exact recipient", { exact: true }).fill(target.id);
  await form.getByRole("button", { name: "Find account", exact: true }).click();
  await expect(form.getByText(`Selected account: ${target.name}`, { exact: true })).toBeVisible();
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${gapIssuePath(issueId)}/participants` && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe("participant_added");
  await expect(form).toHaveCount(0);
  return { receipt, requestId: response.request().headers()["idempotency-key"] };
}

const panel = (page: Page) => page.getByRole("region", { name: "AI execution", exact: true });
async function openGapResources(page: Page) {
  const toggle = panel(page).getByRole("button", { name: "Manage AI resources", exact: true });
  if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  await expect(panel(page).getByRole("button", { name: "Refresh resources", exact: true })).toBeVisible();
}

export interface GapResourceReceipt {
  status: number; code: string; action?: string; issue_id: string; queue_state: string;
  channels: { channel_id: string; code: string; queue_state: string; group_id: string | null; group_settled: boolean; stopped_runs: number; unsettled_runs: number }[];
  target_holds: { target_id: string; code: string; hold_reason: string | null; hold_version: string }[];
  targets?: { target_id: string; channel_id: string; code: string; unsettled_runs: number }[];
}
/** The resource owner withdraws one of their own consents in Manage AI resources. */
export async function withdrawThroughUI(page: Page, fixture: CollaborationControlJourney, target: ControlTarget) {
  await reloadGapPage(page, fixture.issueId, fixture.title);
  await openGapResources(page);
  const resources = await fixture.owner.requestJSON<CollaborationResources>(`${gapIssuePath(fixture.issueId)}/resources`);
  const view = resources.targets.find((item) => item.target_id === target.targetId);
  const resource = view?.resources[0];
  if (!view || view.resources.length !== 1 || !resource?.can_revoke || !resource.grant_id) throw new Error("The owner has no single revocable grant for this target");
  await panel(page).getByRole("button", { name: `Withdraw authorization for ${resource.agent_name}`, exact: true }).click();
  const form = page.getByRole("form", { name: "Withdraw this authorization?", exact: true });
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${gapIssuePath(fixture.issueId)}/grants/${resource.grant_id}/revoke`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Withdraw authorization", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: GapResourceReceipt = await response.json();
  const requestId = response.request().headers()["idempotency-key"];
  if (!requestId) throw new Error("Withdrawal omitted its original request key");
  return { receipt, requestId, grantId: resource.grant_id, grantEpoch: resource.grant_epoch, input: response.request().postDataJSON() as Record<string, unknown> };
}

/** The resource owner re-authorizes a single-resource target in the shared form. */
export async function consentThroughUI(page: Page, fixture: CollaborationControlJourney, target: ControlTarget) {
  await reloadGapPage(page, fixture.issueId, fixture.title);
  await openGapResources(page);
  const resources = await fixture.owner.requestJSON<CollaborationResources>(`${gapIssuePath(fixture.issueId)}/resources`);
  const view = resources.targets.find((item) => item.target_id === target.targetId);
  const resource = view?.resources[0];
  if (!view || view.resources.length !== 1 || !resource?.can_consent) throw new Error("The owner cannot consent this single-resource target");
  await panel(page).getByRole("button", { name: `Authorize ${resource.agent_name}`, exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: `Authorize ${resource.agent_name}`, exact: true }) });
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${gapIssuePath(fixture.issueId)}/targets/${target.targetId}/consents`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm authorization", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(201);
  const receipt: GapResourceReceipt = await response.json();
  expect(receipt.code).toBe("resource_consented");
  return { receipt, requestId: response.request().headers()["idempotency-key"] };
}
export async function readGapHistory(client: TestApiClient, issueId: string) {
  return client.requestJSON<CollaborationCommandHistory>(`${gapIssuePath(issueId)}/commands?limit=100`);
}

/** Visible command history for one input; the panel fetches its own page. */
export async function showGapHistory(page: Page, commandId: string, marker: string, state: string, reason?: string) {
  const toggle = panel(page).getByRole("button", { name: "Command history", exact: true });
  if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  const entry = panel(page).getByRole("region", { name: "Command history", exact: true }).locator(`[data-command-id="${commandId}"]`);
  await expect(entry).toBeVisible();
  await expect(entry).toContainText(marker);
  await expect(entry).toContainText(state);
  if (reason) await expect(entry).toContainText(reason);
  return entry;
}

/** An unrelated issue on which the same owner authorizes the same agent A, so
 * a departure from the journey issue can be checked against another grant. */
export async function seedPeerGrant(fixture: CollaborationControlJourney) {
  const peer = await fixture.creator.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    source_workspace_id: fixture.sourceWorkspace.id, handler_user_id: fixture.creatorId, title: `Peer grant ${randomUUID().slice(0, 8)}`,
    description: "Independent issue whose owner grant must survive a departure elsewhere.", attachment_ids: [],
  } });
  const root = gapIssuePath(peer.id);
  await fixture.creator.requestJSON(`${root}/participants`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...gapWorkflowObservations(peer), target_user_id: fixture.ownerId,
  } });
  const authority = (issue: CollaborationIssue) => ({ expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context!.participant_epoch, expected_account_epoch: issue.write_context!.account_epoch });
  const options = await fixture.owner.requestJSON<{ items: { kind: string; resource_id: string; available: boolean; resources: Record<string, string>[] }[] }>(
    `${root}/resources/offers/options?execution_workspace_id=${fixture.executionWorkspace.id}`);
  const option = options.items.find((item) => item.kind === "agent" && item.resource_id === fixture.agentA.resourceId);
  if (!option?.available) throw new Error("Agent A cannot be offered on the peer issue");
  const offer = await fixture.owner.requestJSON<{ offer_id: string; offer_version: string }>(`${root}/resources/offers`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...authority(await fixture.owner.requestJSON<CollaborationIssue>(root)), kind: "agent", resource_id: fixture.agentA.resourceId, execution_workspace_id: fixture.executionWorkspace.id,
    roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })),
  } });
  const set = await fixture.creator.requestJSON<CollaborationResources>(`${root}/resources`);
  const adopted = await fixture.creator.requestJSON<{ target_id: string | null }>(`${root}/resources/offers/${offer.offer_id}/adopt`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...authority(await fixture.creator.requestJSON<CollaborationIssue>(root)), expected_offer_version: offer.offer_version, expected_target_set_version: set.target_set_version,
  } });
  if (!adopted.target_id) throw new Error("The peer offer has no adopted target");
  await consentControlTarget(fixture.owner, root, adopted.target_id, fixture);
  const resources = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);
  const grant = resources.targets.find((item) => item.target_id === adopted.target_id)!;
  expect(grant.authorized).toBe(true);
  return { issueId: peer.id, title: peer.title, targetId: adopted.target_id, grant };
}
