import { generateKeyPairSync, randomUUID, sign } from "node:crypto";
import { expect, type APIResponse, type Locator, type Page, type Request, type Route } from "@playwright/test";
import type {
  CollaborationCommentInput, CollaborationCommentReceipt, CollaborationControlInput, CollaborationControlReceipt, CollaborationIssue, CollaborationMentionTargets,
  CollaborationParticipants, CollaborationResourceIdentityView, CollaborationResourceOptions, CollaborationResources,
} from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";

interface Challenge { challenge_id: string; signature_payload: string; body_payload: string }
interface ResourceReceipt { status: number; code: string; offer_id: string | null; offer_version: string | null; target_id: string | null }
interface ControlTarget { kind: "agent" | "squad"; targetId: string; resourceId: string; channelId: string }
interface RuntimeIdentity {
  deployment_id: string; installation_id: string; issue_id: string; session_id: string;
  target_id: string; channel_id: string; batch_id: string; group_id: string; run_id: string;
  fence: string; claim_generation: string; launch_nonce: string;
}
interface PendingControlRun { commandId: string; batchId: string; groupId: string; runId: string }
export interface ControlJourneyAdapter {
  navigate(page: Page, path: string): Promise<void>;
  capture?(stage: string, page: Page, focus: Locator): Promise<void>;
}
export const controlEnvironmentReady = () => process.env.E2E_PASSWORD_AUTH === "1" && Boolean(process.env.DATABASE_URL
  && process.env.COLLABORATION_E2E_API_URL && process.env.NEXT_PUBLIC_API_URL && process.env.MULTICA_DEPLOYMENT_ID);
const issuePath = (fixture: { issueId: string }) => `/api/collaboration/issues/${fixture.issueId}`;
export const controlAccountPath = (fixture: { issueId: string }) => `/account?view=participating&issue=${fixture.issueId}`;
function authority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Control fixture requires a current human participant");
  return { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch,
    expected_account_epoch: issue.write_context.account_epoch };
}

/** A synthetic remote Linux resource, never the macOS Electron client's identity.
 * Enrollment/binding sign the server's original bytes. The optional protocol
 * simulation obtains a real permit but starts no process and supplies no proof. */
async function enrollOfflineControlRuntime(owner: TestApiClient, workspaceId: string, ownerId: string, apiUrl: string, deploymentId: string) {
  const keys = generateKeyPairSync("ed25519");
  const publicKey = keys.publicKey.export({ format: "der", type: "spki" }).subarray(-32).toString("base64url");
  const proof = (challenge: Challenge) => ({ challenge_id: challenge.challenge_id, signature_payload: challenge.signature_payload, body_payload: challenge.body_payload,
    signature: sign(null, Buffer.from(challenge.signature_payload, "base64url"), keys.privateKey).toString("base64url") });
  const challenge = await owner.requestJSON<Challenge>("/api/installations/challenges", { method: "POST", body: {
    purpose: "enroll", public_key: publicKey, desktop_version: "0.4.40", os: "linux",
  } });
  const envelope: { user_id: string; deployment_id: string } = JSON.parse(Buffer.from(challenge.signature_payload, "base64url").toString("utf8"));
  expect(envelope).toMatchObject({ user_id: ownerId, deployment_id: deploymentId });
  const installation = await owner.requestJSON<{ installation_id: string }>("/api/installations/enroll", { method: "POST", body: proof(challenge) });
  const pat = await owner.requestJSON<{ token: string }>("/api/tokens", { method: "POST", body: { name: "Synthetic offline control registration", expires_in_days: 1 } });
  const daemonId = randomUUID();
  const bindingChallenge = await owner.requestJSON<Challenge>("/api/installations/challenges", { method: "POST", body: {
    purpose: "bind", installation_id: installation.installation_id, workspace_id: workspaceId, daemon_id: daemonId,
    public_key: publicKey, expected_binding_epoch: null,
  } });
  const daemonJSON = async <T>(path: string, token: string, body?: unknown): Promise<T> => {
    const response = await fetch(`${apiUrl}${path}`, { method: body === undefined ? "GET" : "POST", headers: {
      Authorization: `Bearer ${token}`, "Content-Type": "application/json", "X-Client-Capabilities": "managed_installation_v1",
    }, body: body === undefined ? undefined : JSON.stringify(body) });
    if (!response.ok) throw new Error(`Synthetic managed registration ${path} failed: ${response.status}`);
    return response.json();
  };
  const binding = await daemonJSON<{ binding_id: string; binding_epoch: string; daemon_token: string }>("/api/daemon/installation-bindings", pat.token, proof(bindingChallenge));
  if (!binding.daemon_token.startsWith("mdt_")) throw new Error("The test binding did not issue a scoped daemon credential");
  const register = () => daemonJSON<{ runtimes: { id: string }[] }>("/api/daemon/register", binding.daemon_token, {
    workspace_id: workspaceId, daemon_id: daemonId, device_name: "Synthetic offline Linux control resource", cli_version: "0.4.40", launched_by: "desktop",
    runtimes: [{ name: "Synthetic control runtime; no executable", type: "codex", version: "fixture", status: "online" }],
  });
  const registration = await register();
  expect(registration.runtimes).toHaveLength(1);
  const runtimeId = registration.runtimes[0]!.id;
  const disconnect = () => daemonJSON("/api/daemon/deregister", binding.daemon_token, { runtime_ids: [runtimeId] });
  await disconnect();
  const runtimes = await owner.requestJSON<{ id: string; status: string }[]>("/api/runtimes?owner=me");
  expect(runtimes.find((runtime) => runtime.id === runtimeId)?.status).toBe("offline");
  return { runtimeId, installationId: installation.installation_id, bindingId: binding.binding_id,
    simulatedDaemon: {
      disconnect,
      async permitPendingInput(issueId: string, target: ControlTarget, commandId: string, batchId: string): Promise<PendingControlRun> {
        const online = await register();
        expect(online.runtimes.map((runtime) => runtime.id)).toEqual([runtimeId]);
        const list = await daemonJSON<{ run_ids: string[] }>(`/api/collaboration/runtime/runs?runtime_id=${runtimeId}`, binding.daemon_token);
        expect(list.run_ids).toHaveLength(1);
        const runId = list.run_ids[0]!;
        const path = `/api/collaboration/runtime/runs/${runId}`;
        const claim = await daemonJSON<{ identity: RuntimeIdentity; inputs: { command_id: string; author_user_id: string }[] }>(`${path}/claim`, binding.daemon_token, {
          protocol_version: 2, deployment_id: deploymentId, installation_id: installation.installation_id,
          runtime_id: runtimeId, binding_id: binding.binding_id, binding_epoch: binding.binding_epoch, containment: "linux_pid_namespace_v1",
        });
        expect({ deployment_id: claim.identity.deployment_id, installation_id: claim.identity.installation_id,
          issue_id: claim.identity.issue_id, target_id: claim.identity.target_id, channel_id: claim.identity.channel_id,
          batch_id: claim.identity.batch_id, run_id: claim.identity.run_id }).toEqual({ deployment_id: deploymentId, installation_id: installation.installation_id,
          issue_id: issueId, target_id: target.targetId, channel_id: target.channelId, batch_id: batchId, run_id: runId });
        expect(claim.inputs.map((input) => input.command_id)).toEqual([commandId]);
        const permit = await daemonJSON<{ identity: RuntimeIdentity; attempt_seq: string; attempt_nonce: string }>(`${path}/permit`, binding.daemon_token, {
          identity: claim.identity, attempt_seq: "1", attempt_nonce: randomUUID(), containment: "linux_pid_namespace_v1",
        });
        expect(Object.entries(claim.identity).every(([key, value]) => permit.identity[key as keyof RuntimeIdentity] === value)).toBe(true);
        expect(permit.attempt_seq).toBe("1");
        // Do not return either capability, launch/attempt nonce, or pretend a
        // process ran. The server's genuine issued permit stays unproven.
        return { commandId, batchId, groupId: claim.identity.group_id, runId };
      },
    },
  };
}

async function consentControlTarget(owner: TestApiClient, root: string, targetId: string, runtime: { installationId: string; bindingId: string }) {
  const registered = await owner.requestJSON<CollaborationResources>(`${root}/resources`);
  const target = registered.targets.find((item) => item.target_id === targetId);
  if (!target) throw new Error("The adopted target is missing from the owner's resources");
  for (const resource of target.resources) {
    const identity = await owner.requestJSON<CollaborationResourceIdentityView>(`${root}/resources/identity?target_id=${targetId}&agent_id=${resource.agent_id}`);
    expect(identity.identity).toMatchObject({ kind: "managed", installation_id: runtime.installationId, binding_id: runtime.bindingId });
    const consent = await owner.requestJSON<ResourceReceipt>(`${root}/targets/${targetId}/consents`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...authority(await owner.requestJSON<CollaborationIssue>(root)), agent_id: resource.agent_id,
      expected_target_epoch: target.target_epoch, expected_config_version: target.config_version, identity: identity.identity,
    } });
    expect(consent).toMatchObject({ status: 201, code: "resource_consented", target_id: targetId });
  }
  expect((await owner.requestJSON<CollaborationResources>(`${root}/resources`)).targets.find((item) => item.target_id === targetId)?.authorized).toBe(true);
}

/** All accounts, issue participation, resource grants and controls use public
 * APIs in the coordinator's password deployment. No database rows are seeded. */
export async function seedCollaborationControlJourney() {
  if (!controlEnvironmentReady()) throw new Error("Control acceptance requires the isolated password/managed-installation deployment");
  const apiUrl = process.env.COLLABORATION_E2E_API_URL!;
  for (const url of [apiUrl, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) {
    expect(["localhost", "127.0.0.1"]).toContain(new URL(url).hostname);
  }
  expect(["", "multica", "postgres", "template0", "template1"]).not.toContain(new URL(process.env.DATABASE_URL!).pathname.slice(1));
  expect(new URL(apiUrl).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL!).origin);
  const configuration = await fetch(`${apiUrl}/api/config`);
  expect(configuration.status).toBe(200);
  expect(await configuration.json()).toMatchObject({ auth_mode: "password", managed_installation_supported: true,
    deployment_id: process.env.MULTICA_DEPLOYMENT_ID });
  const suffix = randomUUID().replaceAll("-", "").slice(0, 14);
  const createActor = async (role: string) => {
    const client = new TestApiClient();
    const username = `s6control${role}${suffix}`;
    const password = `Control-${randomUUID()}!`;
    const account = await client.registerPassword(username, password, `Control ${role}`);
    const onboarded = await client.requestJSON<{ onboarded_at: string | null }>("/api/me/onboarding/complete", { method: "POST", body: {} });
    expect(onboarded.onboarded_at).not.toBeNull();
    return { client, id: account.id, async authenticateWeb(page: Page) {
      const response = await page.request.post(`${apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200);
      const cookies = await page.context().cookies(apiUrl);
      expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
      expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
    } };
  };
  const creator = await createActor("creator");
  const handler = await createActor("handler");
  const owner = await createActor("owner");
  const sourceWorkspace = await creator.client.ensureWorkspace("Control source", `control-source-${suffix}`);
  const executionWorkspace = await owner.client.ensureWorkspace("Private control resources", `control-resources-${suffix}`);
  expect((await creator.client.getWorkspaces()).map((workspace) => workspace.id)).not.toContain(executionWorkspace.id);
  const issue = await creator.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    source_workspace_id: sourceWorkspace.id, handler_user_id: handler.id, title: `Independent control lanes ${suffix}`,
    description: "Synthetic remote Linux resources for human control checks. No provider or supervisor process executes AI.", attachment_ids: [],
  } });
  const root = issuePath({ issueId: issue.id });
  await creator.client.requestJSON(`${root}/participants`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...authority(issue), expected_collaboration_revision: issue.collaboration_revision, target_user_id: owner.id,
  } });
  const runtime = await enrollOfflineControlRuntime(owner.client, executionWorkspace.id, owner.id, apiUrl, process.env.MULTICA_DEPLOYMENT_ID!);
  const createAgent = (name: string) => owner.client.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: {
    name, runtime_id: runtime.runtimeId, permission_mode: "private",
  } });
  const agentAResource = await createAgent("Independent control agent A");
  const agentBResource = await createAgent("Independent control agent B");
  const leader = await createAgent("Control squad S leader");
  const member = await createAgent("Control squad S member");
  const squad = await owner.client.requestJSON<{ id: string }>("/api/squads", { method: "POST", body: { name: "Independent control squad S", leader_id: leader.id } });
  await owner.client.requestJSON(`/api/squads/${squad.id}/members`, { method: "POST", body: { member_type: "agent", member_id: member.id, role: "member" } });
  const targets: ControlTarget[] = [];
  for (const [kind, resourceId, agentIds] of [["agent", agentAResource.id, [agentAResource.id]], ["agent", agentBResource.id, [agentBResource.id]],
    ["squad", squad.id, [leader.id, member.id]]] as const) {
    const options = await owner.client.requestJSON<CollaborationResourceOptions>(`${root}/resources/offers/options?execution_workspace_id=${executionWorkspace.id}`);
    const option = options.items.find((item) => item.kind === kind && item.resource_id === resourceId);
    if (!option?.available) throw new Error("The managed control resource is unavailable for an owner offer");
    expect(option.resources.map((resource) => resource.agent_id).sort()).toEqual([...agentIds].sort());
    expect(option.resources.every((resource) => resource.owner_user_id === owner.id)).toBe(true);
    const offer = await owner.client.requestJSON<ResourceReceipt>(`${root}/resources/offers`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...authority(await owner.client.requestJSON<CollaborationIssue>(root)), kind, resource_id: resourceId, execution_workspace_id: executionWorkspace.id,
      roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })),
    } });
    expect(offer).toMatchObject({ status: 201, code: "resource_offered", target_id: null });
    if (!offer.offer_id || !offer.offer_version) throw new Error("The control resource offer has no durable identity");
    const resources = await creator.client.requestJSON<CollaborationResources>(`${root}/resources`);
    const adopted = await creator.client.requestJSON<ResourceReceipt>(`${root}/resources/offers/${offer.offer_id}/adopt`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...authority(await creator.client.requestJSON<CollaborationIssue>(root)), expected_offer_version: offer.offer_version,
      expected_target_set_version: resources.target_set_version,
    } });
    if (!adopted.target_id) throw new Error("The adopted control resource has no target");
    const registered = await owner.client.requestJSON<CollaborationResources>(`${root}/resources`);
    const target = registered.targets.find((item) => item.target_id === adopted.target_id)!;
    expect(target.authorized).toBe(false);
    await consentControlTarget(owner.client, root, target.target_id, runtime);
    const snapshot = await creator.client.requestJSON<CollaborationIssue>(root);
    const channelId = snapshot.execution?.targets.find((item) => item.id === target.target_id)?.channel_id;
    if (!channelId) throw new Error("The registered target has no actual control lane");
    targets.push({ kind, targetId: target.target_id, resourceId, channelId });
  }
  const [agentA, agentB, squadS] = targets;
  if (!agentA || !agentB || !squadS) throw new Error("Independent control targets A, B and squad S are required");
  expect(new Set(targets.map((target) => target.channelId)).size).toBe(3);
  const mentions = await creator.client.requestJSON<CollaborationMentionTargets>(`${root}/mention-targets`);
  expect(mentions.targets.map((target) => target.target_id).sort()).toEqual(targets.map((target) => target.targetId).sort());
  return { issueId: issue.id, title: issue.title, creator: creator.client, creatorId: creator.id, handler: handler.client, handlerId: handler.id,
    owner: owner.client, ownerId: owner.id, authenticateWeb: creator.authenticateWeb,
    authenticateOwnerWeb: owner.authenticateWeb, authenticateHandlerWeb: handler.authenticateWeb,
    sourceWorkspace, executionWorkspace, apiUrl, agentA, agentB, squadS, targets, ...runtime };
}
export type CollaborationControlJourney = Awaited<ReturnType<typeof seedCollaborationControlJourney>>;

export async function authenticateControlWeb(page: Page, fixture: CollaborationControlJourney, webUrl: string) {
  expect(new URL(webUrl).hostname).toBe(new URL(fixture.apiUrl).hostname);
  await fixture.authenticateWeb(page);
  await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
}

const panelFor = (page: Page) => page.getByRole("region", { name: "AI execution", exact: true });
function targetRow(page: Page, target: ControlTarget) {
  return panelFor(page).getByRole("listitem").filter({ has: page.getByText(target.resourceId, { exact: true }) })
    .filter({ has: page.getByRole("button", { name: "Stop target", exact: true }) });
}
async function readControlIssue(fixture: CollaborationControlJourney, pending?: PendingControlRun) {
  const issue = await fixture.creator.requestJSON<CollaborationIssue>(issuePath(fixture));
  if (!issue.execution) throw new Error("The control projection is missing");
  expect(issue.execution.execution_gate.enabled).toBe(true);
  expect(issue.execution.channels).toHaveLength(3);
  expect(issue.execution.targets).toHaveLength(3);
  if (pending) {
    expect(issue.execution.commands).toHaveLength(1);
    expect(issue.execution.groups).toHaveLength(1);
    expect(issue.execution.runs).toHaveLength(1);
    expect(issue.execution.commands[0]).toMatchObject({ id: pending.commandId, batch_id: pending.batchId,
      group_id: pending.groupId, target_id: fixture.agentA.targetId, channel_id: fixture.agentA.channelId, author_user_id: fixture.creatorId });
    expect(issue.execution.groups[0]).toMatchObject({ id: pending.groupId, batch_id: pending.batchId,
      target_id: fixture.agentA.targetId, channel_id: fixture.agentA.channelId });
    expect(issue.execution.runs[0]).toMatchObject({ id: pending.runId, group_id: pending.groupId, batch_id: pending.batchId,
      target_id: fixture.agentA.targetId, channel_id: fixture.agentA.channelId, proof_state: "unknown", business_outcome: null, settled_at: null });
  } else {
    expect(issue.execution.commands).toEqual([]);
    expect(issue.execution.groups).toEqual([]);
    expect(issue.execution.runs).toEqual([]);
  }
  expect(issue.creator.user_id).toBe(fixture.creatorId);
  expect(issue.handler.user_id).toBe(fixture.handlerId);
  return issue;
}
async function assertControlView(page: Page, fixture: CollaborationControlJourney, issue: CollaborationIssue) {
  await expect(panelFor(page).locator(":scope > p").filter({ hasText: new RegExp(`^${issue.session.queue_state === "active" ? "Active" : "Paused"}$`) })).toBeVisible();
  for (const target of fixture.targets) {
    const row = targetRow(page, target);
    const current = issue.execution!.targets.find((item) => item.id === target.targetId)!;
    const lane = row.locator("..").locator("..");
    await expect(lane.getByText(current.control.queue_state === "active" ? "Active" : "Paused", { exact: true })).toBeVisible();
    const label = current.hold_reason === null ? "Target released" : current.hold_reason === "grant_required" ? "Resource authorization required" : "Awaiting initial authorization";
    await expect(row.getByText(label, { exact: true })).toBeVisible();
  }
}

function assertUntouchedLanes(fixture: CollaborationControlJourney, before: CollaborationIssue, after: CollaborationIssue, selected: ControlTarget) {
  expect(after.session).toEqual(before.session);
  for (const target of fixture.targets.filter((target) => target.targetId !== selected.targetId)) {
    expect(after.execution!.channels.find((lane) => lane.id === target.channelId)).toEqual(before.execution!.channels.find((lane) => lane.id === target.channelId));
    expect(after.execution!.targets.find((item) => item.id === target.targetId)).toEqual(before.execution!.targets.find((item) => item.id === target.targetId));
  }
  expect(after.execution!.commands).toEqual(before.execution!.commands);
  expect(after.execution!.groups).toEqual(before.execution!.groups);
  expect(after.execution!.runs).toEqual(before.execution!.runs);
}

interface ObservedControl { receipt: CollaborationControlReceipt; requestId: string; input: CollaborationControlInput }
function observeControlRequests(page: Page, root: string) {
  const writes: { request_id: string; action: string; scope: string; target_ids: string[] }[] = [];
  const recoveryPaths: string[] = [];
  const record = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (path === `${root}/control` && request.method() === "POST") {
      const input: CollaborationControlInput = request.postDataJSON();
      writes.push({ request_id: request.headers()["idempotency-key"]!, action: input.action, scope: input.scope, target_ids: input.targets.map((target) => target.target_id) });
    }
    if (path.startsWith(`${root}/control/operations/`) && request.method() === "GET") recoveryPaths.push(path);
  };
  page.on("request", record);
  return { writes, recoveryPaths, dispose: () => page.off("request", record) };
}
async function clickControl(page: Page, fixture: CollaborationControlJourney, button: Locator, status: number, loseResponse = false): Promise<ObservedControl> {
  const path = `${issuePath(fixture)}/control`;
  const readResponse = async (response: Pick<APIResponse, "status" | "json">, request: Request): Promise<ObservedControl> => {
    expect(response.status()).toBe(status);
    const requestId = request.headers()["idempotency-key"];
    if (!requestId) throw new Error("A UI control request omitted its original key");
    const receipt: CollaborationControlReceipt = await response.json();
    const input: CollaborationControlInput = request.postDataJSON();
    expect(receipt).toMatchObject({ issue_id: fixture.issueId, request_id: requestId, status, action: input.action, scope: input.scope });
    return { receipt, requestId, input };
  };
  if (!loseResponse) {
    const responsePending = page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === "POST");
    await button.click();
    const response = await responsePending;
    const outcome = await readResponse(response, response.request());
    await expect(panelFor(page).getByRole("status", { name: "Control result", exact: true })).toBeVisible();
    return outcome;
  }
  let resolveCommit: (value: ObservedControl) => void = () => undefined;
  let rejectCommit: (reason: unknown) => void = () => undefined;
  const committed = new Promise<ObservedControl>((resolve, reject) => { resolveCommit = resolve; rejectCommit = reject; });
  const intercept = async (route: Route) => {
    if (route.request().method() !== "POST") return route.continue();
    try {
      const outcome = await readResponse(await route.fetch(), route.request());
      await route.abort("failed");
      resolveCommit(outcome);
    } catch (error) { await route.abort("failed"); rejectCommit(error); }
  };
  await page.route(`**${path}`, intercept);
  let original: ObservedControl;
  try {
    [original] = await Promise.all([committed, button.click()]);
    await expect(panelFor(page).getByRole("button", { name: "Check original result", exact: true })).toBeEnabled();
  } finally { await page.unroute(`**${path}`, intercept); }
  // This refresh is the intentional uncertain-response recovery case. The
  // original persisted draft must survive; no new control POST is permitted.
  await page.reload();
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  const recover = panelFor(page).getByRole("button", { name: "Check original result", exact: true });
  await expect(recover).toBeEnabled();
  const recoveredPending = page.waitForResponse((response) => new URL(response.url()).pathname === `${path}/operations/${original.requestId}`
    && response.request().method() === "GET");
  await recover.click();
  const recovered = await recoveredPending;
  expect(recovered.status()).toBe(200);
  expect(await recovered.json()).toEqual(original.receipt);
  await expect(recover).toHaveCount(0);
  return original;
}

/** Backend tests own process/proof matrices. This verifies real client controls
 * against independent empty lanes and a real partial authorization refusal. */
export async function exerciseCollaborationControlJourney(page: Page, fixture: CollaborationControlJourney, adapter: ControlJourneyAdapter) {
  const root = issuePath(fixture);
  const { writes, recoveryPaths, dispose } = observeControlRequests(page, root);
  try {
    await adapter.navigate(page, controlAccountPath(fixture));
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const initial = await readControlIssue(fixture);
    expect(initial.session.queue_state).toBe("paused");
    expect(initial.execution!.targets.every((target) => target.hold_reason !== null)).toBe(true);
    const participants = await fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`);
    await assertControlView(page, fixture, initial);
    const refused = await clickControl(page, fixture, targetRow(page, fixture.agentB).getByRole("button", { name: "Resume target", exact: true }), 409);
    expect(refused.receipt).toMatchObject({ action: "resume", scope: "target", target_id: fixture.agentB.targetId, code: "issue_paused", channels: [], target_holds: [] });
    await expect(panelFor(page).getByRole("status", { name: "Control result", exact: true })).toContainText("Issue paused; explicit resume required");
    expect((await readControlIssue(fixture)).execution).toEqual(initial.execution);

    const resources = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);
    const grant = resources.targets.find((target) => target.target_id === fixture.agentA.targetId)?.resources[0];
    if (!grant?.grant_id || !grant.grant_epoch) throw new Error("A has no real owner grant to revoke");
    const revoked = await fixture.owner.requestJSON<ResourceReceipt>(`${root}/grants/${grant.grant_id}/revoke`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...authority(await fixture.owner.requestJSON<CollaborationIssue>(root)), expected_grant_epoch: grant.grant_epoch,
    } });
    expect(revoked).toMatchObject({ status: 200, code: "grant_revoked" });
    const afterRevoke = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);
    expect(afterRevoke.targets.find((target) => target.target_id === fixture.agentA.targetId)?.authorized).toBe(false);
    expect(afterRevoke.targets.find((target) => target.target_id === fixture.agentB.targetId)).toEqual(resources.targets.find((target) => target.target_id === fixture.agentB.targetId));
    await expect(targetRow(page, fixture.agentA).getByText("Resource authorization required", { exact: true })).toBeVisible();
    const partial = await clickControl(page, fixture, panelFor(page).getByRole("button", { name: "Resume all", exact: true }), 200, true);
    expect(partial.receipt).toMatchObject({ action: "resume", scope: "issue", code: "partially_resumed", queue_state: "active" });
    expect(partial.receipt.channels.map((lane) => lane.channel_id).sort()).toEqual(fixture.targets.map((target) => target.channelId).sort());
    expect(partial.receipt.channels.every((lane) => lane.code === "resumed" && lane.queue_state === "active" && lane.unsettled_runs === 0)).toBe(true);
    expect(partial.receipt.target_holds).toHaveLength(3);
    expect(partial.receipt.target_holds.find((target) => target.target_id === fixture.agentA.targetId)).toMatchObject({ code: "grant_required", hold_reason: "grant_required" });
    expect(partial.receipt.target_holds.find((target) => target.target_id === fixture.agentB.targetId)).toMatchObject({ code: "hold_released", hold_reason: null });
    expect(partial.receipt.target_holds.find((target) => target.target_id === fixture.squadS.targetId)).toMatchObject({ code: "hold_released", hold_reason: null });
    const result = panelFor(page).getByRole("status", { name: "Control result", exact: true });
    await expect(result.getByText("3 lanes resumed; 0 remain paused.", { exact: true })).toBeVisible();
    await expect(result.getByText("1 targets remain held.", { exact: true })).toBeVisible();
    await expect(result.getByText(`Agent ${fixture.agentA.resourceId} · Resource authorization required`, { exact: true })).toBeVisible();
    await expect(result.getByText(`Agent ${fixture.agentB.resourceId} · Active`, { exact: true })).toBeVisible();
    await expect(result.getByText(`Squad ${fixture.squadS.resourceId} · Active`, { exact: true })).toBeVisible();
    await expect(result).not.toContainText("Termination unconfirmed");
    await expect(result).not.toContainText(/all (?:targets |lanes )?resumed/i);
    const resumed = await readControlIssue(fixture);
    expect(resumed.session.queue_state).toBe("active");
    for (const decision of partial.receipt.target_holds) {
      expect(resumed.execution!.targets.find((target) => target.id === decision.target_id)).toMatchObject({ hold_reason: decision.hold_reason, hold_version: decision.hold_version });
    }
    await assertControlView(page, fixture, resumed);
    await adapter.capture?.("control-partial-resume", page, result);

    const controls = [refused, partial];
    const isolated: { target: ControlTarget; stopped: CollaborationControlReceipt; resumed: CollaborationControlReceipt }[] = [];
    // B and the distinct squad have their own controls even while A is held.
    for (const target of [fixture.agentB, fixture.squadS]) {
      const before = await readControlIssue(fixture);
      const stopped = await clickControl(page, fixture, targetRow(page, target).getByRole("button", { name: "Stop target", exact: true }), 200);
      expect(stopped.receipt).toMatchObject({ action: "stop", scope: "target", target_id: target.targetId, channel_id: target.channelId, queue_state: "paused", code: "already_ended" });
      expect(stopped.receipt.channels.map((lane) => lane.channel_id)).toEqual([target.channelId]);
      await expect(panelFor(page).getByRole("status", { name: "Control result", exact: true }).getByText("Termination confirmed", { exact: true })).toBeVisible();
      const paused = await readControlIssue(fixture);
      assertUntouchedLanes(fixture, before, paused, target);
      expect(paused.execution!.channels.find((lane) => lane.id === target.channelId)?.queue_state).toBe("paused");
      await assertControlView(page, fixture, paused);
      const resumedTarget = await clickControl(page, fixture, targetRow(page, target).getByRole("button", { name: "Resume target", exact: true }), 200);
      expect(resumedTarget.receipt).toMatchObject({ action: "resume", scope: "target", target_id: target.targetId, channel_id: target.channelId, code: "resumed", queue_state: "active" });
      expect(resumedTarget.receipt.channels.map((lane) => lane.channel_id)).toEqual([target.channelId]);
      await expect(panelFor(page).getByRole("status", { name: "Control result", exact: true }).getByText("1 lanes resumed; 0 remain paused.", { exact: true })).toBeVisible();
      const restored = await readControlIssue(fixture);
      assertUntouchedLanes(fixture, paused, restored, target);
      await assertControlView(page, fixture, restored);
      controls.push(stopped, resumedTarget);
      isolated.push({ target, stopped: stopped.receipt, resumed: resumedTarget.receipt });
      if (target.kind === "squad") await adapter.capture?.("control-squad-resumed", page, panelFor(page).getByRole("status", { name: "Control result", exact: true }));
    }
    const beforeConsent = await readControlIssue(fixture);
    await consentControlTarget(fixture.owner, root, fixture.agentA.targetId, fixture);
    const reconsented = await readControlIssue(fixture);
    assertUntouchedLanes(fixture, beforeConsent, reconsented, fixture.agentA);
    expect(reconsented.execution!.targets.find((target) => target.id === fixture.agentA.targetId)?.hold_reason).toBe("grant_required");
    // The external owner consent advances hold_version without changing its
    // visible reason. Reload to capture the fresh authoritative observation.
    await page.reload();
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    await assertControlView(page, fixture, reconsented);
    const resumedA = await clickControl(page, fixture, targetRow(page, fixture.agentA).getByRole("button", { name: "Resume target", exact: true }), 200);
    expect(resumedA.receipt).toMatchObject({ code: "resumed", scope: "target", target_id: fixture.agentA.targetId });
    expect(resumedA.receipt.target_holds).toEqual([expect.objectContaining({ target_id: fixture.agentA.targetId, code: "hold_released", hold_reason: null })]);
    const allReleased = await readControlIssue(fixture);
    assertUntouchedLanes(fixture, reconsented, allReleased, fixture.agentA);
    expect(allReleased.execution!.targets.every((target) => target.hold_reason === null)).toBe(true);
    await assertControlView(page, fixture, allReleased);
    controls.push(resumedA);
    const stoppedAll = await clickControl(page, fixture, panelFor(page).getByRole("button", { name: "Stop all", exact: true }), 200);
    expect(stoppedAll.receipt).toMatchObject({ action: "stop", scope: "issue", queue_state: "paused", code: "already_ended" });
    await expect(panelFor(page).getByRole("status", { name: "Control result", exact: true }).getByText("Termination confirmed", { exact: true })).toBeVisible();
    expect(stoppedAll.receipt.channels).toHaveLength(3);
    expect(stoppedAll.receipt.channels.every((lane) => lane.queue_state === "paused" && lane.unsettled_runs === 0)).toBe(true);
    const final = await readControlIssue(fixture);
    expect(final.session.queue_state).toBe("paused");
    expect(final.status).toBe(initial.status);
    expect(final.status_category).toBe(initial.status_category);
    expect(final.execution!.channels.every((lane) => lane.queue_state === "paused")).toBe(true);
    await assertControlView(page, fixture, final);
    await adapter.capture?.("control-stopped-all", page, panelFor(page).getByRole("status", { name: "Control result", exact: true }));
    controls.push(stoppedAll);
    for (const original of controls) {
      expect(await fixture.creator.requestJSON(`${root}/control/operations/${original.requestId}`)).toEqual(original.receipt);
    }
    expect((await readControlIssue(fixture)).execution).toEqual(final.execution);
    expect((await fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`)).participants).toEqual(participants.participants);
    expect(writes.map(({ action, scope, target_ids }) => [action, scope, [...target_ids].sort()])).toEqual([
      ["resume", "target", [fixture.agentB.targetId]], ["resume", "issue", fixture.targets.map((target) => target.targetId).sort()],
      ["stop", "target", [fixture.agentB.targetId]], ["resume", "target", [fixture.agentB.targetId]],
      ["stop", "target", [fixture.squadS.targetId]], ["resume", "target", [fixture.squadS.targetId]],
      ["resume", "target", [fixture.agentA.targetId]], ["stop", "issue", fixture.targets.map((target) => target.targetId).sort()],
    ]);
    expect(new Set(writes.map((write) => write.request_id)).size).toBe(8);
    expect(recoveryPaths).toEqual([`${root}/control/operations/${partial.requestId}`]);
    expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    expect((await fixture.owner.requestJSON<{ id: string; status: string }[]>("/api/runtimes?owner=me")).find((runtime) => runtime.id === fixture.runtimeId)?.status).toBe("offline");
    return { issue_id: fixture.issueId, installation_id: fixture.installationId, binding_id: fixture.bindingId, runtime_id: fixture.runtimeId,
      target_a: fixture.agentA, target_b: fixture.agentB, squad_s: fixture.squadS, refused: refused.receipt, partial: partial.receipt,
      isolated_targets: isolated, reconsented_resume: resumedA.receipt, stopped_all: stoppedAll.receipt, control_writes: writes, recovery_paths: recoveryPaths,
      scope: "Real control UI and public managed registration for an offline synthetic remote Linux resource; no AI/process/proof execution" };
  } finally { dispose(); }
}

/** A deliberately simulated daemon obtains an actual server permit and then
 * withholds all outcome/proof writes. This tests the UI's real pending barrier,
 * never OS containment, provider execution, or proof acceptance/recovery. */
export async function exerciseCollaborationPendingProofJourney(page: Page, fixture: CollaborationControlJourney, adapter: ControlJourneyAdapter) {
  const root = issuePath(fixture);
  const { writes, recoveryPaths, dispose } = observeControlRequests(page, root);
  try {
    await adapter.navigate(page, controlAccountPath(fixture));
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const initial = await readControlIssue(fixture);
    const participants = await fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`);
    await assertControlView(page, fixture, initial);
    const resumed = await clickControl(page, fixture, panelFor(page).getByRole("button", { name: "Resume all", exact: true }), 200);
    expect(resumed.receipt).toMatchObject({ code: "resumed", queue_state: "active" });
    const active = await readControlIssue(fixture);
    expect(active.execution!.targets.every((target) => target.hold_reason === null)).toBe(true);
    await assertControlView(page, fixture, active);
    const target = active.execution!.targets.find((item) => item.id === fixture.agentA.targetId)!;
    const commentInput: CollaborationCommentInput = {
      ...authority(active), content: `[@A](mention://agent/${fixture.agentA.resourceId}) Test-only permit with no provider process or proof`,
      target_versions: [{ target_id: target.id, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version }],
      attachment_ids: [], parent_id: null, suppressed_target_ids: [],
    };
    const comment = await fixture.creator.requestJSON<CollaborationCommentReceipt>(`${root}/comments`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: commentInput });
    expect(comment).toMatchObject({ status: 201, code: "comment_created" });
    expect(comment.targets).toHaveLength(1);
    const input = comment.targets[0];
    if (!input?.command_id || !input.batch_id) throw new Error("The protocol fixture has no real immutable input");
    expect(input).toMatchObject({ target_id: fixture.agentA.targetId, state: "waiting" });
    const pending = await fixture.simulatedDaemon.permitPendingInput(fixture.issueId, fixture.agentA, input.command_id, input.batch_id);
    const permitted = await readControlIssue(fixture, pending);
    expect(permitted.execution!.runs[0]?.progress_state).toBe("launch_authorized");
    // Refresh only after the real public API has committed the group/run, so
    // the control request captures current group/control observations.
    await page.reload();
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    await assertControlView(page, fixture, permitted);
    const stoppedA = await clickControl(page, fixture, targetRow(page, fixture.agentA).getByRole("button", { name: "Stop target", exact: true }), 200);
    expect(stoppedA.receipt).toMatchObject({ code: "stop_requested", scope: "target", target_id: fixture.agentA.targetId, queue_state: "paused" });
    expect(stoppedA.receipt.channels).toEqual([expect.objectContaining({ channel_id: fixture.agentA.channelId,
      code: "stop_requested", queue_state: "paused", group_id: pending.groupId, unsettled_runs: 1, group_settled: false })]);
    await expect(panelFor(page).getByRole("status", { name: "Control result", exact: true }).getByText("Stop accepted; termination remains unconfirmed", { exact: true })).toBeVisible();
    const stopped = await readControlIssue(fixture, pending);
    // Stop may change A's group/run disposition, but neither unrelated lane,
    // target hold nor issue control may change.
    expect(stopped.session).toEqual(permitted.session);
    for (const other of [fixture.agentB, fixture.squadS]) {
      expect(stopped.execution!.channels.find((lane) => lane.id === other.channelId)).toEqual(permitted.execution!.channels.find((lane) => lane.id === other.channelId));
      expect(stopped.execution!.targets.find((item) => item.id === other.targetId)).toEqual(permitted.execution!.targets.find((item) => item.id === other.targetId));
    }
    await assertControlView(page, fixture, stopped);
    const refused = await clickControl(page, fixture, targetRow(page, fixture.agentA).getByRole("button", { name: "Resume target", exact: true }), 409);
    expect(refused.receipt).toMatchObject({ code: "stop_unconfirmed", scope: "target", target_id: fixture.agentA.targetId });
    await expect(panelFor(page).getByRole("status", { name: "Control result", exact: true }).getByText("Termination unconfirmed", { exact: true })).toBeVisible();
    expect((await readControlIssue(fixture, pending)).execution).toEqual(stopped.execution);
    const stoppedAll = await clickControl(page, fixture, panelFor(page).getByRole("button", { name: "Stop all", exact: true }), 200);
    expect(stoppedAll.receipt).toMatchObject({ code: "stop_requested", scope: "issue", queue_state: "paused" });
    expect(stoppedAll.receipt.channels).toHaveLength(3);
    const paused = await readControlIssue(fixture, pending);
    expect(paused.execution!.channels.every((lane) => lane.queue_state === "paused")).toBe(true);
    await assertControlView(page, fixture, paused);
    const partial = await clickControl(page, fixture, panelFor(page).getByRole("button", { name: "Resume all", exact: true }), 200, true);
    expect(partial.receipt).toMatchObject({ code: "partially_resumed", scope: "issue", queue_state: "active" });
    expect(partial.receipt.channels.find((lane) => lane.channel_id === fixture.agentA.channelId)).toMatchObject({ code: "stop_unconfirmed", queue_state: "paused", unsettled_runs: 1, group_id: pending.groupId });
    for (const other of [fixture.agentB, fixture.squadS]) {
      expect(partial.receipt.channels.find((lane) => lane.channel_id === other.channelId)).toMatchObject({ code: "resumed", queue_state: "active", unsettled_runs: 0 });
    }
    const partiallyActive = await readControlIssue(fixture, pending);
    expect(partiallyActive.execution!.channels.find((lane) => lane.id === fixture.agentA.channelId)).toEqual(paused.execution!.channels.find((lane) => lane.id === fixture.agentA.channelId));
    expect(partiallyActive.execution!.targets.find((item) => item.id === fixture.agentA.targetId)).toEqual(paused.execution!.targets.find((item) => item.id === fixture.agentA.targetId));
    await assertControlView(page, fixture, partiallyActive);
    const result = panelFor(page).getByRole("status", { name: "Control result", exact: true });
    await expect(result.getByText("2 lanes resumed; 1 remain paused.", { exact: true })).toBeVisible();
    const laneNumber = partiallyActive.execution!.channels.findIndex((lane) => lane.id === fixture.agentA.channelId) + 1;
    await expect(result.getByText(`Execution lane ${laneNumber} · Termination unconfirmed`, { exact: true })).toBeVisible();
    await expect(result).not.toContainText("Termination confirmed");
    await expect(result).not.toContainText(/all (?:targets |lanes )?resumed/i);
    await adapter.capture?.("control-proof-partial-resume", page, result);
    const stoppedSquad = await clickControl(page, fixture, targetRow(page, fixture.squadS).getByRole("button", { name: "Stop target", exact: true }), 200);
    expect(stoppedSquad.receipt).toMatchObject({ code: "already_ended", scope: "target", target_id: fixture.squadS.targetId });
    const squadPaused = await readControlIssue(fixture, pending);
    assertUntouchedLanes(fixture, partiallyActive, squadPaused, fixture.squadS);
    await assertControlView(page, fixture, squadPaused);
    const resumedSquad = await clickControl(page, fixture, targetRow(page, fixture.squadS).getByRole("button", { name: "Resume target", exact: true }), 200);
    expect(resumedSquad.receipt).toMatchObject({ code: "resumed", scope: "target", target_id: fixture.squadS.targetId });
    const squadActive = await readControlIssue(fixture, pending);
    assertUntouchedLanes(fixture, squadPaused, squadActive, fixture.squadS);
    await assertControlView(page, fixture, squadActive);
    const finalStop = await clickControl(page, fixture, panelFor(page).getByRole("button", { name: "Stop all", exact: true }), 200);
    expect(finalStop.receipt).toMatchObject({ code: "stop_requested", queue_state: "paused" });
    const final = await readControlIssue(fixture, pending);
    expect(final.session.queue_state).toBe("paused");
    expect(final.status).toBe(initial.status);
    expect(final.status_category).toBe(initial.status_category);
    expect(final.execution!.channels.every((lane) => lane.queue_state === "paused")).toBe(true);
    for (const original of [resumed, stoppedA, refused, stoppedAll, partial, stoppedSquad, resumedSquad, finalStop]) {
      expect(await fixture.creator.requestJSON(`${root}/control/operations/${original.requestId}`)).toEqual(original.receipt);
    }
    expect((await readControlIssue(fixture, pending)).execution).toEqual(final.execution);
    expect((await fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`)).participants).toEqual(participants.participants);
    expect(writes.map(({ action, scope }) => [action, scope])).toEqual([["resume", "issue"], ["stop", "target"], ["resume", "target"],
      ["stop", "issue"], ["resume", "issue"], ["stop", "target"], ["resume", "target"], ["stop", "issue"]]);
    expect(new Set(writes.map((write) => write.request_id)).size).toBe(8);
    expect(recoveryPaths).toEqual([`${root}/control/operations/${partial.requestId}`]);
    expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    return { issue_id: fixture.issueId, runtime_id: fixture.runtimeId, target_a: fixture.agentA, target_b: fixture.agentB, squad_s: fixture.squadS,
      input: pending, stopped_target: stoppedA.receipt, refused_target_resume: refused.receipt, stopped_all: stoppedAll.receipt,
      partial_resume: partial.receipt, stopped_squad: stoppedSquad.receipt, resumed_squad: resumedSquad.receipt,
      final_stop: finalStop.receipt, control_writes: writes, recovery_paths: recoveryPaths,
      scope: "Simulated managed daemon; real public comment/List/Claim/Permit and UI controls; no provider/supervisor process, outcome, proof submission, or kernel containment evidence" };
  } finally {
    dispose();
    // Keep the unproven run/barrier intact. Deregistration only marks this
    // test-owned runtime offline; it must never fabricate termination proof.
    await fixture.simulatedDaemon.disconnect();
  }
}
