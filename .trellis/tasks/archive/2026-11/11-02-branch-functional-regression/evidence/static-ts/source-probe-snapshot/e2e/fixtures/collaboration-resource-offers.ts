import { randomBytes, randomUUID } from "node:crypto";
import pg from "pg";
import { expect, type Locator, type Page } from "@playwright/test";
import { TestApiClient } from "../fixtures";
import type { CollaborationResourceSnapshot } from "./collaboration-resources";

interface OfferIssue extends CollaborationResourceSnapshot {
  session: { queue_state: string; control_version: string };
  execution: CollaborationResourceSnapshot["execution"] & {
    commands: { id: string; state: string; target_id: string }[];
    groups: unknown[];
    channels: { id: string; queue_state: string; control_version: string }[];
  };
}
interface Offer {
  issue_id: string; offer_id: string; version: string; state: string; resource_id: string; name: string;
  offered_by: { user_id: string }; target_id: string | null; can_adopt: boolean; can_withdraw: boolean;
  resources: { agent_id: string; runtime_id: string; execution_workspace_id: string; owner_user_id: string }[];
}
interface OffersPage { issue_id: string; items: Offer[]; next_cursor: string | null }
interface Resources {
  issue_id: string;
  targets: { target_id: string; resource_id: string; authorized: boolean; resources: {
    agent_id: string; owner_user_id: string; consented: boolean; grant_id: string | null; grant_epoch: string | null;
  }[] }[];
}
interface ResourceReceipt {
  issue_id: string; request_id: string; action: string; status: number; code: string;
  offer_id?: string; offer_version?: string; target_id: string | null; grant_id: string | null;
}
interface MentionTargets { targets: { target_id: string; resource_id: string }[] }
interface CommentReceipt {
  issue_id: string; comment_id: string; event_id: string; event_seq: string;
  targets: { target_id: string; resource_id: string; state: string; command_id: string | null }[];
}
interface Notifications {
  items: { id: string; issue_id: string; source_kind: string; source_id: string | null; is_mention: boolean; event_seq: string }[];
}

function offerEnvironment() {
  const { DATABASE_URL: databaseUrl, COLLABORATION_E2E_API_URL: apiUrl, MULTICA_DEPLOYMENT_ID: deploymentId, NEXT_PUBLIC_API_URL: browserApi } = process.env;
  if (!databaseUrl || !apiUrl || !deploymentId || !browserApi) throw new Error("Requires the task-owned collaboration API/DB and matching production browser API");
  for (const value of [databaseUrl, apiUrl, browserApi]) {
    if (!["localhost", "127.0.0.1"].includes(new URL(value).hostname)) throw new Error("Resource offer fixtures require loopback test services");
  }
  if (["", "multica", "postgres", "template0", "template1"].includes(new URL(databaseUrl).pathname.slice(1))) throw new Error("Resource offer fixtures refuse a default database");
  if (new URL(apiUrl).origin !== new URL(browserApi).origin) throw new Error("Browser and fixture API origins differ");
  return { databaseUrl, apiUrl, deploymentId };
}

/** Creates only synthetic accounts and resources in the caller's isolated DB.
 * Creation and participation use real APIs. The runtime/enrollment use the
 * existing resource fixture pattern: no daemon, executable or provider account.
 * Rows remain available for the coordinator's inspection and scoped teardown. */
export async function seedCollaborationResourceOfferJourney() {
  const { databaseUrl, apiUrl, deploymentId } = offerEnvironment();
  const suffix = randomUUID().slice(0, 10);
  const creator = new TestApiClient();
  await creator.login(`offer-creator-${suffix}@example.invalid`, "Offer creator P");
  const sourceWorkspace = await creator.ensureWorkspace("Offer source workspace", `offer-source-${suffix}`);
  await creator.markUserOnboarded();
  const creatorAccount = await creator.requestJSON<{ id: string }>("/api/me");
  const handler = new TestApiClient();
  await handler.login(`offer-handler-${suffix}@example.invalid`, "Offer handler H");
  await handler.markUserOnboarded();
  const handlerAccount = await handler.requestJSON<{ id: string }>("/api/me");
  const owner = new TestApiClient();
  await owner.login(`offer-owner-${suffix}@example.invalid`, "Private resource owner D");
  const executionWorkspace = await owner.ensureWorkspace("Owner's private resource workspace", `offer-private-${suffix}`);
  await owner.markUserOnboarded();
  const ownerAccount = await owner.requestJSON<{ id: string }>("/api/me");
  expect(executionWorkspace.id).not.toBe(sourceWorkspace.id);
  expect((await owner.getWorkspaces()).map((workspace) => workspace.id)).not.toContain(sourceWorkspace.id);
  for (const actor of [creator, handler]) expect((await actor.getWorkspaces()).map((workspace) => workspace.id)).not.toContain(executionWorkspace.id);

  const options = await creator.requestJSON<{ creation_enabled: boolean }>("/api/collaboration/creation-options");
  if (!options.creation_enabled) throw new Error("Enable creation only in this task-owned browser database before the offer journey");
  const creationRequestId = randomUUID();
  const issue = await creator.requestJSON<OfferIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": creationRequestId }, body: {
    source_workspace_id: sourceWorkspace.id, handler_user_id: handlerAccount.id, title: `Private resource collaboration ${suffix}`,
    description: "A participant offers one private resource. Its owner authorizes it separately. No process is started.", attachment_ids: [],
  } });
  const invitationRequestId = randomUUID();
  await creator.requestJSON(`/api/collaboration/issues/${issue.id}/participants`, { method: "POST", headers: { "Idempotency-Key": invitationRequestId }, body: {
    target_user_id: ownerAccount.id, expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch,
  } });
  const runtime = await owner.seedProjectRuntime();
  const agentName = `Explicitly offered private agent ${suffix}`;
  const undisclosedAgentName = `Undisclosed private agent ${suffix}`;
  const agent = await owner.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: agentName, runtime_id: runtime.id, permission_mode: "private" } });
  const undisclosed = await owner.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: undisclosedAgentName, runtime_id: runtime.id, permission_mode: "private" } });
  const db = new pg.Client(databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    const updated = await db.query("UPDATE agent_runtime SET status='offline', custom_name='Private owner machine for the selected resource' WHERE id=$1 AND owner_id=$2 AND workspace_id=$3", [runtime.id, ownerAccount.id, executionWorkspace.id]);
    expect(updated.rowCount).toBe(1);
    await db.query("INSERT INTO collaboration_runtime_enrollment (deployment_id,owner_user_id,execution_workspace_id,runtime_id,installation_id,public_key) VALUES ($1,$2,$3,$4,$5,$6)",
      [deploymentId, ownerAccount.id, executionWorkspace.id, runtime.id, randomUUID(), randomBytes(32)]);
    await db.query("COMMIT");
  } catch (error) { await db.query("ROLLBACK"); throw error; }
  finally { await db.end(); }
  return { creator, handler, owner, creatorId: creatorAccount.id, handlerId: handlerAccount.id, ownerId: ownerAccount.id,
    sourceWorkspace, executionWorkspace, issueId: issue.id, title: issue.title, runtimeId: runtime.id, agentId: agent.id, agentName,
    undisclosedAgentId: undisclosed.id, undisclosedAgentName, creationRequestId, invitationRequestId, apiUrl };
}
export type CollaborationResourceOfferJourney = Awaited<ReturnType<typeof seedCollaborationResourceOfferJourney>>;
export type ResourceOfferCaptureStage = "supplier" | "adoption" | "authorized" | "withdrawn";
export interface ResourceOfferJourneyAdapter {
  navigate(page: Page, path: string): Promise<void>;
  capture?(stage: ResourceOfferCaptureStage, page: Page, focus: Locator): Promise<void>;
}

export const resourceOfferAccountPath = (fixture: CollaborationResourceOfferJourney) => `/account?view=participating&issue=${fixture.issueId}`;
const apiIssuePath = (fixture: CollaborationResourceOfferJourney) => `/api/collaboration/issues/${fixture.issueId}`;

async function assertPrivateCatalogClosed(fixture: CollaborationResourceOfferJourney) {
  for (const actor of [fixture.creator, fixture.handler]) {
    const response = await fetch(`${fixture.apiUrl}${apiIssuePath(fixture)}/resources/options?execution_workspace_id=${fixture.executionWorkspace.id}`, {
      headers: { Authorization: `Bearer ${actor.getToken()}` },
    });
    expect(response.status).toBe(404);
    expect(await response.json()).toMatchObject({ code: "resource_workspace_unavailable" });
    expect(response.headers.get("cache-control")).toContain("no-store");
  }
}

async function readOffers(actor: TestApiClient, fixture: CollaborationResourceOfferJourney) {
  const page = await actor.requestJSON<OffersPage>(`${apiIssuePath(fixture)}/resources/offers?limit=50`);
  expect(page.issue_id).toBe(fixture.issueId);
  expect(page.next_cursor).toBeNull();
  expect(JSON.stringify(page)).not.toContain(fixture.undisclosedAgentId);
  expect(JSON.stringify(page)).not.toContain(fixture.undisclosedAgentName);
  for (const offer of page.items) {
    expect(offer.issue_id).toBe(fixture.issueId);
    expect(offer.offered_by.user_id).toBe(fixture.ownerId);
    expect(offer.resource_id).toBe(fixture.agentId);
    expect(offer.resources).toHaveLength(1);
    expect(offer.resources[0]).toMatchObject({ agent_id: fixture.agentId, runtime_id: fixture.runtimeId, owner_user_id: fixture.ownerId, execution_workspace_id: fixture.executionWorkspace.id });
    expect(offer).not.toHaveProperty("configuration_fingerprint");
    expect(offer).not.toHaveProperty("resource_authority");
    expect(offer.resources[0]).not.toHaveProperty("identity");
    expect(offer.resources[0]).not.toHaveProperty("grant_id");
  }
  return page.items;
}

async function assertNoExecution(fixture: CollaborationResourceOfferJourney, expectedCommands = 0) {
  const issue = await fixture.owner.requestJSON<OfferIssue>(apiIssuePath(fixture));
  expect(issue.handler.user_id).toBe(fixture.handlerId);
  expect(issue.session.queue_state).toBe("paused");
  expect(issue.execution.runs).toEqual([]);
  expect(issue.execution.groups).toEqual([]);
  expect(issue.execution.commands).toHaveLength(expectedCommands);
  expect(issue.execution.commands.every((command) => command.state === "waiting")).toBe(true);
  expect(issue.execution.channels.every((channel) => channel.queue_state === "paused")).toBe(true);
  expect(issue.execution.targets.every((target) => target.hold_reason !== null)).toBe(true);
  return issue;
}

async function submitResource(page: Page, actor: TestApiClient, fixture: CollaborationResourceOfferJourney, button: Locator, suffix: string, action: string, code: string, status: number) {
  const responsePending = page.waitForResponse((response) => new URL(response.url()).pathname === `${apiIssuePath(fixture)}${suffix}` && response.request().method() === "POST");
  await button.click();
  const response = await responsePending;
  expect(response.status()).toBe(status);
  const requestId = response.request().headers()["idempotency-key"];
  expect(requestId).toMatch(/^[0-9a-f-]{36}$/i);
  const receipt: ResourceReceipt = await response.json();
  expect(receipt).toMatchObject({ issue_id: fixture.issueId, request_id: requestId, action, code, status });
  const original = await actor.requestJSON<ResourceReceipt>(`${apiIssuePath(fixture)}/resources/operations/${requestId}`);
  expect(original).toEqual(receipt);
  return { receipt, body: response.request().postDataJSON(), requestId: requestId! };
}

async function offerSelectedResource(ownerPage: Page, fixture: CollaborationResourceOfferJourney, adapter: ResourceOfferJourneyAdapter, capture: boolean) {
  const offers = ownerPage.getByRole("region", { name: "Offered resources", exact: true });
  await offers.getByRole("button", { name: "Offer resource", exact: true }).click();
  const form = offers.getByRole("form", { name: "Offer resource", exact: true });
  await form.getByLabel("Resource workspace", { exact: true }).selectOption(fixture.executionWorkspace.id);
  const target = form.getByLabel("AI target", { exact: true });
  await expect(target.locator(`option[value="agent:${fixture.agentId}"]`)).toHaveCount(1);
  await expect(target.locator(`option[value="agent:${fixture.undisclosedAgentId}"]`)).toHaveCount(1);
  await target.selectOption(`agent:${fixture.agentId}`);
  await expect(form.getByRole("button", { name: "Offer resource", exact: true })).toBeEnabled();
  if (capture) await adapter.capture?.("supplier", ownerPage, form);
  const result = await submitResource(ownerPage, fixture.owner, fixture, form.getByRole("button", { name: "Offer resource", exact: true }), "/resources/offers", "offer_resource", "resource_offered", 201);
  expect(result.body).toMatchObject({ execution_workspace_id: fixture.executionWorkspace.id, kind: "agent", resource_id: fixture.agentId,
    roster: [{ role: "agent", agent_id: fixture.agentId, runtime_id: fixture.runtimeId, execution_workspace_id: fixture.executionWorkspace.id, owner_user_id: fixture.ownerId }] });
  expect(result.body).not.toHaveProperty("expected_target_set_version");
  expect(result.body).not.toHaveProperty("identity");
  expect(result.receipt).toMatchObject({ offer_version: "1", target_id: null, grant_id: null });
  if (!result.receipt.offer_id) throw new Error("Offering did not return an offer ID");
  await expect(ownerPage.getByText("Resource offered.", { exact: true })).toBeVisible();
  await expect(form).toHaveCount(0);
  return { ...result, offerId: result.receipt.offer_id };
}

/** Both pages must already be authenticated as owner D and handler H.
 * Navigation/capture are injected so the same journey can use real Electron
 * deep links and native capturePage without a mock browser/native adapter. */
export async function exerciseCollaborationResourceOffers(ownerPage: Page, organizerPage: Page, fixture: CollaborationResourceOfferJourney, adapter: ResourceOfferJourneyAdapter) {
  const root = apiIssuePath(fixture);
  const requestEvidence: { actor: string; method: string; path: string; workspaceIdHeader: boolean; workspaceSlugHeader: boolean }[] = [];
  const errors: string[] = [];
  for (const [page, actor] of [[ownerPage, "owner"], [organizerPage, "handler"]] as const) {
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (!path.startsWith("/api/collaboration/")) return;
      const headers = request.headers();
      requestEvidence.push({ actor, path, method: request.method(), workspaceIdHeader: Boolean(headers["x-workspace-id"]), workspaceSlugHeader: Boolean(headers["x-workspace-slug"]) });
    });
    await adapter.navigate(page, resourceOfferAccountPath(fixture));
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Manage AI resources", exact: true }).click();
    await expect(page.getByText("No resources have been offered.", { exact: true })).toBeVisible();
  }
  await expect(ownerPage.getByRole("button", { name: "Add AI target", exact: true })).toHaveCount(0);
  await assertPrivateCatalogClosed(fixture);
  expect((await assertNoExecution(fixture)).execution.targets).toEqual([]);

  const supplied = await offerSelectedResource(ownerPage, fixture, adapter, true);
  expect((await assertNoExecution(fixture)).execution.targets).toEqual([]);
  for (const actor of [fixture.creator, fixture.handler]) {
    const offers = await readOffers(actor, fixture);
    expect(offers.map((offer) => offer.offer_id)).toEqual([supplied.offerId]);
    expect(offers[0]).toMatchObject({ state: "offered", can_adopt: true, can_withdraw: false, target_id: null });
    expect((await actor.requestJSON<MentionTargets>(`${root}/mention-targets`)).targets).toEqual([]);
  }
  const organizerOffers = organizerPage.getByRole("region", { name: "Offered resources", exact: true });
  await organizerOffers.getByRole("button", { name: "Refresh", exact: true }).click();
  await organizerOffers.getByRole("button", { name: "Adopt resource", exact: true }).click();
  const adoption = organizerOffers.getByRole("form", { name: "Adopt resource", exact: true });
  await expect(adoption.getByRole("button", { name: "Adopt resource", exact: true })).toBeEnabled();
  await expect(adoption).toContainText(fixture.agentName);
  await expect(adoption).toContainText("each resource owner must authorize");
  await expect(organizerPage.getByText(fixture.undisclosedAgentName, { exact: true })).toHaveCount(0);
  await adapter.capture?.("adoption", organizerPage, adoption);
  const adopted = await submitResource(organizerPage, fixture.handler, fixture, adoption.getByRole("button", { name: "Adopt resource", exact: true }),
    `/resources/offers/${supplied.offerId}/adopt`, "adopt_resource_offer", "resource_offer_adopted", 201);
  expect(adopted.body).toMatchObject({ expected_offer_version: "1" });
  expect(adopted.body).not.toHaveProperty("roster");
  expect(adopted.receipt).toMatchObject({ offer_id: supplied.offerId, offer_version: "2", grant_id: null });
  const targetId = adopted.receipt.target_id;
  if (!targetId) throw new Error("Adoption did not return a target ID");
  await expect(organizerPage.getByText("Resource adopted. Each owner can now authorize their resource.", { exact: true })).toBeVisible();
  const unconsented = await fixture.owner.requestJSON<Resources>(`${root}/resources`);
  expect(unconsented.targets).toHaveLength(1);
  expect(unconsented.targets[0]).toMatchObject({ target_id: targetId, resource_id: fixture.agentId, authorized: false,
    resources: [{ agent_id: fixture.agentId, owner_user_id: fixture.ownerId, consented: false, grant_id: null }] });
  expect((await fixture.handler.requestJSON<MentionTargets>(`${root}/mention-targets`)).targets).toEqual([]);
  await assertNoExecution(fixture);

  await ownerPage.getByRole("button", { name: "Refresh resources", exact: true }).click();
  await ownerPage.getByRole("button", { name: `Authorize ${fixture.agentName}`, exact: true }).click();
  const consent = await submitResource(ownerPage, fixture.owner, fixture, ownerPage.getByRole("button", { name: "Confirm authorization", exact: true }),
    `/targets/${targetId}/consents`, "consent_resource", "resource_consented", 201);
  expect(consent.body).toMatchObject({ agent_id: fixture.agentId });
  expect(consent.body.identity).toMatchObject({ kind: "enrolled" });
  expect(consent.receipt.target_id).toBe(targetId);
  expect(consent.receipt.grant_id).toMatch(/^[0-9a-f-]{36}$/i);
  await expect(ownerPage.getByText("Resource authorized. Explicit resume is still required.", { exact: true })).toBeVisible();
  const ownResources = await fixture.owner.requestJSON<Resources>(`${root}/resources`);
  expect(ownResources.targets[0]).toMatchObject({ target_id: targetId, authorized: true, resources: [{ consented: true, grant_id: consent.receipt.grant_id }] });
  for (const actor of [fixture.creator, fixture.handler, fixture.owner]) {
    const mentions = await actor.requestJSON<MentionTargets>(`${root}/mention-targets`);
    expect(mentions.targets.map((target) => [target.target_id, target.resource_id])).toEqual([[targetId, fixture.agentId]]);
  }
  const withdrawalControl = ownerPage.getByRole("button", { name: `Withdraw authorization for ${fixture.agentName}`, exact: true });
  await expect(withdrawalControl).toBeVisible();
  const authorizedResource = ownerPage.getByRole("listitem").filter({ has: withdrawalControl }).first();
  await adapter.capture?.("authorized", ownerPage, authorizedResource);
  const beforeWithdrawal = await assertNoExecution(fixture);

  // A second offer of this same resource remains unadopted. Withdrawing it
  // must not revoke the already adopted target's independent authorization.
  const second = await offerSelectedResource(ownerPage, fixture, adapter, false);
  expect(second.offerId).not.toBe(supplied.offerId);
  const ownerOffers = ownerPage.getByRole("region", { name: "Offered resources", exact: true });
  await ownerOffers.getByRole("button", { name: "Withdraw offer", exact: true }).click();
  const withdrawal = ownerOffers.getByRole("form", { name: "Withdraw offer", exact: true });
  await expect(withdrawal.getByRole("button", { name: "Withdraw offer", exact: true })).toBeEnabled();
  const withdrawn = await submitResource(ownerPage, fixture.owner, fixture, withdrawal.getByRole("button", { name: "Withdraw offer", exact: true }),
    `/resources/offers/${second.offerId}/withdraw`, "withdraw_resource_offer", "resource_offer_withdrawn", 200);
  expect(withdrawn.body).toMatchObject({ expected_offer_version: "1" });
  expect(withdrawn.receipt).toMatchObject({ offer_id: second.offerId, offer_version: "2", target_id: null, grant_id: null });
  await expect(ownerPage.getByText("Resource offer withdrawn.", { exact: true })).toBeVisible();
  expect((await fixture.owner.requestJSON<Resources>(`${root}/resources`)).targets).toEqual(ownResources.targets);
  const afterWithdrawal = await assertNoExecution(fixture);
  expect(afterWithdrawal.session).toEqual(beforeWithdrawal.session);
  expect(afterWithdrawal.execution.targets).toEqual(beforeWithdrawal.execution.targets);
  expect(afterWithdrawal.execution.channels).toEqual(beforeWithdrawal.execution.channels);
  for (const actor of [fixture.creator, fixture.handler]) {
    const offers = await readOffers(actor, fixture);
    expect(offers.map((offer) => [offer.offer_id, offer.state])).toEqual([[second.offerId, "withdrawn"], [supplied.offerId, "adopted"]]);
  }
  await expect(ownerOffers.getByText("Withdrawn", { exact: true })).toBeVisible();
  await adapter.capture?.("withdrawn", ownerPage, ownerOffers);
  await assertPrivateCatalogClosed(fixture);

  // This cross-slice happy path uses the real preview/comment/notification APIs.
  // The explicit AI input stays waiting under the existing issue/target holds.
  const composer = organizerPage.getByRole("form", { name: "Comments", exact: true });
  await composer.getByRole("button", { name: "@all · All participants", exact: true }).click();
  await composer.getByRole("button", { name: fixture.agentName, exact: true }).click();
  await expect(composer.getByLabel("Add comment", { exact: true })).toHaveValue(new RegExp(`mention://all/all.*mention://agent/${fixture.agentId}`));
  await composer.getByRole("button", { name: "Preview delivery", exact: true }).click();
  await expect(composer.getByRole("button", { name: "Send comment", exact: true })).toBeEnabled();
  const commentResponse = organizerPage.waitForResponse((response) => new URL(response.url()).pathname === `${root}/comments` && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Send comment", exact: true }).click();
  const sent = await commentResponse;
  expect(sent.status()).toBe(201);
  const comment: CommentReceipt = await sent.json();
  expect(comment.issue_id).toBe(fixture.issueId);
  expect(comment.targets).toHaveLength(1);
  expect(comment.targets[0]).toMatchObject({ target_id: targetId, resource_id: fixture.agentId, state: "waiting" });
  await expect(composer.getByText("Comment confirmed.", { exact: true })).toBeVisible();
  const notificationIds: Record<string, string[]> = {};
  for (const [actor, actorId, expectedCount] of [[fixture.creator, fixture.creatorId, 1], [fixture.owner, fixture.ownerId, 1], [fixture.handler, fixture.handlerId, 0]] as const) {
    const notifications = await actor.requestJSON<Notifications>("/api/me/collaboration-notifications?limit=50");
    const exact = notifications.items.filter((item) => item.issue_id === fixture.issueId && item.source_kind === "comment" && item.source_id === comment.comment_id);
    expect(exact).toHaveLength(expectedCount);
    expect(exact.every((item) => item.is_mention && item.event_seq === comment.event_seq)).toBe(true);
    notificationIds[actorId] = exact.map((item) => item.id);
  }
  await assertNoExecution(fixture, 1);
  expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
  expect(requestEvidence.length).toBeGreaterThan(10);
  expect(requestEvidence.every((item) => !item.workspaceIdHeader && !item.workspaceSlugHeader)).toBe(true);
  expect(requestEvidence.filter((item) => item.method === "POST" && /\/control$|\/revoke$|\/participants$|\/targets$/.test(item.path))).toEqual([]);
  expect(errors).toEqual([]);
  return { issue_id: fixture.issueId, source_workspace_id: fixture.sourceWorkspace.id, execution_workspace_id: fixture.executionWorkspace.id,
    creator_id: fixture.creatorId, handler_id: fixture.handlerId, owner_id: fixture.ownerId, runtime_id: fixture.runtimeId,
    agent_id: fixture.agentId, undisclosed_agent_id: fixture.undisclosedAgentId, target_id: targetId, grant_id: consent.receipt.grant_id,
    creation_request_id: fixture.creationRequestId, invitation_request_id: fixture.invitationRequestId,
    offer_id: supplied.offerId, offer_request_id: supplied.requestId, adopt_request_id: adopted.requestId, consent_request_id: consent.requestId,
    withdrawn_offer_id: second.offerId, second_offer_request_id: second.requestId, withdraw_request_id: withdrawn.requestId,
    comment_id: comment.comment_id, comment_event_id: comment.event_id, command_id: comment.targets[0]!.command_id,
    notification_ids: notificationIds, request_scopes: requestEvidence };
}
