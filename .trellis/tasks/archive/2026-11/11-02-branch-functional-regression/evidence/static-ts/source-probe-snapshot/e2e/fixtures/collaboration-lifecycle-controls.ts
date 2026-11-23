import { createHash, randomUUID } from "node:crypto";
import { createServer, request as httpRequest, type IncomingHttpHeaders } from "node:http";
import { isIPv4, type Socket } from "node:net";
import { expect, type Locator, type Page } from "@playwright/test";
import type {
  CollaborationCommandHistory, CollaborationCommentInput, CollaborationCommentReceipt, CollaborationControlInput,
  CollaborationControlReceipt, CollaborationIssue, CollaborationMentionTargets, CollaborationOperation,
  CollaborationParticipants, CollaborationResourceIdentityView, CollaborationResourceOptions, CollaborationResourceReceipt,
  CollaborationResources,
} from "@multica/core/collaboration";
import { assertProviderInput, crossClientAccountPath, seedCrossClientExecution, startCrossClientDaemons,
  type CrossClientDaemons, type CrossClientFixture, type SentCrossClientComment } from "./collaboration-cross-client-execution";

export type LifecycleFixture = CrossClientFixture;
type Actor = LifecycleFixture["webActor"];
type Target = LifecycleFixture["targetA"];
type Visit = CrossClientDaemons["visits"][number];
export const lifecycleIssuePath = (issueId: string) => `/api/collaboration/issues/${issueId}`;

function authority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Lifecycle controls require current participant authority");
  return { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch,
    expected_account_epoch: issue.write_context.account_epoch };
}

export async function readLifecycleIssue(fixture: LifecycleFixture, actor = fixture.webActor) {
  const issue = await actor.client.requestJSON<CollaborationIssue>(lifecycleIssuePath(fixture.issueId));
  expect(issue.id).toBe(fixture.issueId);
  expect(issue.creator.user_id).toBe(fixture.webActor.id);
  if (!issue.execution) throw new Error("The lifecycle journey requires current execution evidence");
  return issue;
}

/** Only transport is interrupted. Proof request bytes/capabilities never leave
 * this closure or enter diagnostics, and every retry must keep its first hash. */
export async function createLifecycleRuntimeTransport(apiUrl: string) {
  const upstream = new URL(apiUrl);
  if (upstream.protocol !== "http:" || !["127.0.0.1", "localhost"].includes(upstream.hostname)) {
    throw new Error("Runtime forwarding requires an explicit task-owned loopback API");
  }
  const held = new Set<string>();
  const fingerprints = new Map<string, string>();
  const sockets = new Set<Socket>();
  const attempts: { method: string; path: string; status: number; body_sha256: string | null }[] = [];
  const errors: string[] = [];
  let disconnected = false;
  const forward = (method: string, path: string, headers: IncomingHttpHeaders, body: Buffer) =>
    new Promise<{ status: number; headers: IncomingHttpHeaders; bytes: Buffer }>((resolve, reject) => {
      const request = httpRequest(new URL(path, upstream), { method, headers: { ...headers, host: upstream.host }, timeout: 15_000 }, (response) => {
        const chunks: Buffer[] = [];
        response.on("data", (chunk: Buffer) => chunks.push(chunk));
        response.once("error", reject);
        response.once("end", () => resolve({ status: response.statusCode ?? 502, headers: response.headers, bytes: Buffer.concat(chunks) }));
      });
      request.once("error", reject);
      request.once("timeout", () => request.destroy(new Error("task runtime forwarding timeout")));
      request.end(body);
    });
  const server = createServer(async (request, response) => {
    const path = new URL(request.url ?? "/", "http://localhost").pathname;
    const method = request.method ?? "GET";
    const chunks: Buffer[] = [];
    let bytes = 0;
    try {
      for await (const value of request) {
        const chunk = Buffer.isBuffer(value) ? value : Buffer.from(value);
        bytes += chunk.length;
        if (bytes > 2_000_000) throw new Error("runtime fixture body limit exceeded");
        chunks.push(chunk);
      }
      const body = Buffer.concat(chunks);
      const proof = method === "POST" && /^\/api\/collaboration\/runtime\/runs\/[0-9a-f-]+\/proof$/.test(path);
      const digest = proof ? createHash("sha256").update(body).digest("hex") : null;
      if (proof) {
        const original = fingerprints.get(path);
        if (original !== undefined && original !== digest) errors.push("Original sealed proof request changed during transport retry");
        if (original === undefined) fingerprints.set(path, digest!);
      }
      if (disconnected) {
        attempts.push({ method, path, status: 0, body_sha256: digest });
        response.destroy();
        return;
      }
      if (proof && held.has(path)) {
        attempts.push({ method, path, status: 503, body_sha256: digest });
        response.writeHead(503, { "Content-Type": "application/json", "Cache-Control": "no-store", "Retry-After": "1" });
        response.end('{"code":"fixture_transport_unavailable"}');
        return;
      }
      const result = await forward(method, request.url ?? "/", request.headers, body);
      attempts.push({ method, path, status: result.status, body_sha256: digest });
      response.writeHead(result.status, result.headers);
      response.end(result.bytes);
    } catch {
      errors.push("Task runtime forwarding failed");
      if (!response.headersSent) response.writeHead(502, { "Content-Type": "application/json", "Cache-Control": "no-store" });
      response.end('{"code":"fixture_transport_unavailable"}');
    }
  });
  server.on("connection", (socket) => { sockets.add(socket); socket.once("close", () => sockets.delete(socket)); });
  await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(0, "0.0.0.0", resolve); });
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("The runtime fixture proxy has no listening port");
  return {
    apiUrl: `http://127.0.0.1:${address.port}`,
    holdProof(runId: string) { held.add(`/api/collaboration/runtime/runs/${runId}/proof`); },
    releaseProof(runId: string) { held.delete(`/api/collaboration/runtime/runs/${runId}/proof`); },
    async waitForHeldProof(runId: string) {
      const path = `/api/collaboration/runtime/runs/${runId}/proof`;
      await expect.poll(() => attempts.find((attempt) => attempt.path === path && attempt.status === 503), { timeout: 30_000 }).toBeDefined();
      expect(errors).toEqual([]);
    },
    async reconnectRuntime() {
      disconnected = true;
      for (const socket of sockets) socket.destroy();
      await expect.poll(() => attempts.filter((attempt) => attempt.status === 0 && attempt.path.startsWith("/api/collaboration/runtime/")).length,
        { timeout: 30_000 }).toBeGreaterThan(0);
      const before = attempts.length;
      disconnected = false;
      await expect.poll(() => attempts.slice(before).some((attempt) => attempt.path.startsWith("/api/collaboration/runtime/") && attempt.status === 200),
        { timeout: 30_000 }).toBe(true);
    },
    async waitForAcceptedProof(runId: string) {
      const path = `/api/collaboration/runtime/runs/${runId}/proof`;
      await expect.poll(() => attempts.find((attempt) => attempt.path === path && attempt.status === 200), { timeout: 30_000 }).toBeDefined();
      expect(errors).toEqual([]);
    },
    evidence: () => ({ attempts: [...attempts], errors: [...errors], original_proof_hashes: Object.fromEntries(fingerprints) }),
    async close() {
      held.clear(); disconnected = false;
      for (const socket of sockets) socket.destroy();
      await new Promise<void>((resolve) => server.close(() => resolve()));
    },
  };
}

export async function seedLifecycleControls() {
  const fixture = await seedCrossClientExecution();
  const api = fixture.webActor.client;
  const peer = await api.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    source_workspace_id: fixture.sourceWorkspace.id, handler_user_id: fixture.webActor.id, title: `Unrelated lifecycle peer ${randomUUID().slice(0, 8)}`,
    description: "Independent private issue state must survive another issue's removal.", attachment_ids: [],
  } });
  async function register(issueId: string, kind: "agent" | "squad", resourceId: string) {
    const root = lifecycleIssuePath(issueId);
    const options = await api.requestJSON<CollaborationResourceOptions>(`${root}/resources/options?execution_workspace_id=${fixture.targetA.runtime.workspaceId}`);
    const candidate = options.items.find((item) => item.kind === kind && item.resource_id === resourceId);
    if (!candidate?.available) throw new Error("The creator's explicit same-runtime resource cannot be registered");
    const before = await api.requestJSON<CollaborationIssue>(root);
    const resources = await api.requestJSON<CollaborationResources>(`${root}/resources`);
    const registered = await api.requestJSON<CollaborationResourceReceipt>(`${root}/targets`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...authority(before), kind, resource_id: resourceId, expected_target_set_version: resources.target_set_version,
      roster: candidate.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })),
    } });
    if ("redacted" in registered || !registered.target_id) throw new Error("Registered lifecycle resource has no target identity");
    const targetId = registered.target_id;
    const current = await api.requestJSON<CollaborationResources>(`${root}/resources`);
    const target = current.targets.find((item) => item.target_id === targetId)!;
    for (const resource of target.resources) {
      const identity = await api.requestJSON<CollaborationResourceIdentityView>(`${root}/resources/identity?target_id=${target.target_id}&agent_id=${resource.agent_id}`);
      await api.requestJSON(`${root}/targets/${target.target_id}/consents`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
        ...authority(await api.requestJSON<CollaborationIssue>(root)), agent_id: resource.agent_id,
        expected_target_epoch: target.target_epoch, expected_config_version: target.config_version, identity: identity.identity,
      } });
    }
    const after = await api.requestJSON<CollaborationIssue>(root);
    const channel = after.execution!.targets.find((item) => item.id === target.target_id)?.channel_id;
    if (!channel) throw new Error("The registered independent target has no actual channel");
    return { lane: "A" as const, name: target.name, resourceId, targetId: target.target_id, channelId: channel,
      ownerId: fixture.webActor.id, runtime: fixture.targetA.runtime };
  }
  const independentAgent = await api.requestJSON<{ id: string }>("/api/agents", { method: "POST", headers: { "X-Workspace-ID": fixture.targetA.runtime.workspaceId,
    "X-Workspace-Slug": "" }, body: { name: `Independent creator target C ${randomUUID().slice(0, 8)}`, runtime_id: fixture.targetA.runtime.runtimeId, permission_mode: "private" } });
  const independent = await register(fixture.issueId, "agent", independentAgent.id);
  const peerTarget = await register(peer.id, "agent", fixture.targetA.resourceId);
  const peerIssue = await api.requestJSON<CollaborationIssue>(lifecycleIssuePath(peer.id));
  const peerInput: CollaborationCommentInput = { ...authority(peerIssue), content: `[@peer](mention://agent/${fixture.targetA.resourceId}) unrelated paused peer input`,
    target_versions: [{ target_id: peerTarget.targetId, expected_target_epoch: "1", expected_config_version: "1" }], attachment_ids: [], parent_id: null, suppressed_target_ids: [] };
  await api.requestJSON(`${lifecycleIssuePath(peer.id)}/comments`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: peerInput });
  return { fixture, independent, peerId: peer.id, peerBefore: await api.requestJSON<CollaborationIssue>(lifecycleIssuePath(peer.id)) };
}

export const lifecycleTargetRow = (page: Page, resourceId: string) => page.getByRole("region", { name: "AI execution", exact: true })
  .locator("li").filter({ has: page.locator("span").filter({ hasText: new RegExp(`^${resourceId}$`) }) }).first();

export async function lifecycleControl(page: Page, fixture: LifecycleFixture, action: "stop" | "resume", target?: Target, status = 200) {
  const responsePromise = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/control`
    && response.request().method() === "POST");
  const button = target ? lifecycleTargetRow(page, target.resourceId).getByRole("button", { name: action === "stop" ? "Stop target" : "Resume target", exact: true })
    : page.getByRole("region", { name: "AI execution", exact: true }).getByRole("button", { name: action === "stop" ? "Stop all" : "Resume all", exact: true });
  await button.click();
  const response = await responsePromise;
  expect(response.status()).toBe(status);
  const receipt: CollaborationControlReceipt = await response.json();
  const requestId = response.request().headers()["idempotency-key"];
  if (!requestId) throw new Error("Actual lifecycle UI control omitted its immutable request ID");
  const input: CollaborationControlInput = response.request().postDataJSON();
  return { receipt, requestId, input };
}

/** Admissible targets preview as waiting even while idle and active. Paused
 * targets remain valid inputs; their explicit hold/waiting reason is retained. */
export async function lifecycleComment(page: Page, fixture: LifecycleFixture, target: Target, actor: Actor, marker: string, client: "web" | "electron"): Promise<SentCrossClientComment> {
  const form = page.getByRole("form", { name: "Comments", exact: true });
  await form.getByLabel("Add comment", { exact: true }).fill(marker);
  await form.getByRole("group", { name: "AI targets", exact: true }).getByRole("button", { name: target.name, exact: true }).click();
  const content = await form.getByLabel("Add comment", { exact: true }).inputValue();
  expect(content).toContain(`mention://agent/${target.resourceId}`);
  const previewPromise = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/comments/trigger-preview`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Preview delivery", exact: true }).click();
  const previewResponse = await previewPromise;
  expect(previewResponse.status()).toBe(200);
  const preview: SentCrossClientComment["preview"] = await previewResponse.json();
  const catalogue = await actor.client.requestJSON<CollaborationMentionTargets>(`${lifecycleIssuePath(fixture.issueId)}/mention-targets`);
  const currentTarget = catalogue.targets.find((item) => item.target_id === target.targetId);
  if (!currentTarget) throw new Error("Lifecycle preview has no current authorized target");
  expect(preview.targets).toEqual([expect.objectContaining({ target_id: target.targetId, kind: "agent", resource_id: target.resourceId,
    state: "waiting", target_epoch: currentTarget.target_epoch, config_version: currentTarget.config_version,
    hold_reason: currentTarget.hold_reason, hold_version: currentTarget.hold_version, waiting_reason: currentTarget.waiting_reason,
    command_id: null, group_id: null })]);
  const sent = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/comments` && response.request().method() === "POST");
  await form.getByRole("button", { name: "Send comment", exact: true }).click();
  const response = await sent;
  expect(response.status()).toBe(201);
  const receipt: CollaborationCommentReceipt = await response.json();
  const input: CollaborationCommentInput = response.request().postDataJSON();
  const requestId = response.request().headers()["idempotency-key"];
  const result = receipt.targets[0];
  if (!requestId || !result?.command_id || !result.batch_id || !result.command_sequence) throw new Error("Lifecycle UI comment has no exact command/batch/event key");
  expect(input.content).toBe(content);
  expect(input.target_versions).toEqual([{ target_id: target.targetId, expected_target_epoch: currentTarget.target_epoch,
    expected_config_version: currentTarget.config_version }]);
  expect(receipt).toMatchObject({ issue_id: fixture.issueId, status: 201, code: "comment_created" });
  expect(result).toMatchObject({ target_id: target.targetId, channel_id: target.channelId, state: "waiting" });
  return { client, actorId: actor.id, targetId: target.targetId, content, marker, requestId, receipt, commandId: result.command_id,
    batchId: result.batch_id, sequence: result.command_sequence, preview };
}

export async function lifecycleRemove(page: Page, fixture: LifecycleFixture) {
  const participants = page.getByRole("button", { name: "Participants", exact: true });
  if (await participants.getAttribute("aria-expanded") !== "true") await participants.click();
  const accounts = await fixture.webActor.client.requestJSON<CollaborationParticipants>(`${lifecycleIssuePath(fixture.issueId)}/participants`);
  const removed = accounts.participants.find((person) => person.user_id === fixture.desktopActor.id)!;
  await page.getByRole("button", { name: `Remove ${removed.name}`, exact: true }).click();
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toBeVisible();
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/participants/${fixture.desktopActor.id}/remove`
    && response.request().method() === "POST");
  await dialog.getByRole("button", { name: "Remove participant", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe("participant_removed");
  return { receipt, requestId: response.request().headers()["idempotency-key"] };
}

export async function showLifecycleHistory(page: Page, command: SentCrossClientComment, state: "Cancelled" | "Stopped" | "Succeeded" | "Waiting", reason?: string) {
  const panel = page.getByRole("region", { name: "AI execution", exact: true });
  const toggle = panel.getByRole("button", { name: "Command history", exact: true });
  if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  const entry = panel.getByRole("region", { name: "Command history", exact: true }).locator(`[data-command-id="${command.commandId}"]`);
  await expect(entry).toBeVisible();
  await expect(entry).toContainText(command.marker);
  await expect(entry).toContainText(state);
  if (reason) await expect(entry).toContainText(reason);
  return entry;
}

export async function assertLifecyclePaused(page: Page, fixture: LifecycleFixture, originalRun: string, survivor: SentCrossClientComment, cancelled: SentCrossClientComment) {
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  const disclosure = page.locator(`[id="run-${originalRun}"]`);
  await expect(disclosure).toContainText("Termination unconfirmed");
  await expect(disclosure).toContainText("Waiting for termination");
  await expect(disclosure).not.toContainText("Termination confirmed");
  await showLifecycleHistory(page, cancelled, "Cancelled", "The author was removed");
  await showLifecycleHistory(page, survivor, "Waiting");
}

export async function refreshLifecyclePage(page: Page, fixture: LifecycleFixture) {
  const response = page.waitForResponse((value) => new URL(value.url()).pathname === lifecycleIssuePath(fixture.issueId) && value.request().method() === "GET");
  await page.reload();
  expect((await response).status()).toBe(200);
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
}

export async function exerciseLifecycleRemoval(fixture: LifecycleFixture, page: Page, handlerPage: Page, client: "web" | "electron",
  controls: { proxy: Awaited<ReturnType<typeof createLifecycleRuntimeTransport>>; daemons: CrossClientDaemons; independent: Target; peerId: string; peerBefore: CollaborationIssue;
    reconnectClients(): Promise<void>; capture(stage: string, page: Page, focus: Locator): Promise<void> }) {
  const root = lifecycleIssuePath(fixture.issueId);
  const read = () => readLifecycleIssue(fixture);
  const independentBefore = await read();
  const independentTarget = independentBefore.execution!.targets.find((target) => target.id === controls.independent.targetId)!;
  const independentChannel = independentBefore.execution!.channels.find((channel) => channel.id === controls.independent.channelId)!;
  const assertUntouched = async () => {
    const current = await read();
    expect(current.execution!.targets.find((target) => target.id === controls.independent.targetId)).toEqual(independentTarget);
    expect(current.execution!.channels.find((channel) => channel.id === controls.independent.channelId)).toEqual(independentChannel);
    expect(await fixture.webActor.client.requestJSON(lifecycleIssuePath(controls.peerId))).toEqual(controls.peerBefore);
  };
  const original = await lifecycleComment(handlerPage, fixture, fixture.targetA, fixture.desktopActor, "Lifecycle original handler input", client === "web" ? "electron" : "web");
  const visit = await controls.daemons.waitForVisit("A", 0);
  assertProviderInput(visit, fixture, fixture.targetA, [original]);
  controls.proxy.holdProof(visit.input.identity.run_id);
  const authorWaiting = await lifecycleComment(handlerPage, fixture, fixture.targetA, fixture.desktopActor, "Lifecycle cancelled author's queued input", client === "web" ? "electron" : "web");
  const survivor = await lifecycleComment(page, fixture, fixture.targetA, fixture.webActor, "Lifecycle remaining creator queued input", client);
  const pausedTargetStop = await lifecycleControl(page, fixture, "stop", fixture.targetB);
  expect(pausedTargetStop.receipt).toMatchObject({ scope: "target", target_id: fixture.targetB.targetId, queue_state: "paused", code: "already_ended" });
  await refreshLifecyclePage(handlerPage, fixture);
  const pausedAuthorWaiting = await lifecycleComment(handlerPage, fixture, fixture.targetB, fixture.desktopActor,
    "Lifecycle author input submitted to an already paused target", client === "web" ? "electron" : "web");
  expect(pausedAuthorWaiting.preview.targets[0]?.waiting_reason).toBe("target_paused");
  expect((await read()).execution!.commands.find((input) => input.id === pausedAuthorWaiting.commandId)?.state).toBe("waiting");
  expect(controls.daemons.visits.filter((input) => input.lane === "B")).toHaveLength(0);
  const activeBeforeRemoval = await read();
  expect(activeBeforeRemoval.execution!.runs.find((run) => run.id === visit.input.identity.run_id)).toMatchObject({
    proof_state: "unknown", forced_disposition: null, settled_at: null,
  });
  expect(activeBeforeRemoval.execution!.commands.find((input) => input.id === original.commandId)?.state).toBe("in_flight");
  expect(activeBeforeRemoval.execution!.channels.find((lane) => lane.id === fixture.targetA.channelId)?.queue_state).toBe("active");
  const removal = await lifecycleRemove(page, fixture);
  await controls.proxy.waitForHeldProof(visit.input.identity.run_id);
  await expect.poll(async () => (await read()).handler.user_id).toBe(fixture.webActor.id);
  await refreshLifecyclePage(page, fixture);
  const addedWhilePaused = await lifecycleComment(page, fixture, fixture.targetA, fixture.webActor, "Lifecycle added during target pause", client);
  expect(addedWhilePaused.preview.targets[0]?.waiting_reason).toBe("target_paused");
  const paused = await read();
  expect(paused.execution!.channels.find((lane) => lane.id === fixture.targetA.channelId)?.queue_state).toBe("paused");
  expect(paused.execution!.commands.find((input) => input.id === authorWaiting.commandId)).toMatchObject({ state: "cancelled" });
  expect(paused.execution!.commands.find((input) => input.id === pausedAuthorWaiting.commandId)).toMatchObject({ state: "cancelled" });
  expect(paused.execution!.commands.find((input) => input.id === survivor.commandId)?.state).toBe("waiting");
  expect(paused.execution!.runs.find((run) => run.id === visit.input.identity.run_id)).toMatchObject({ proof_state: "unknown", forced_disposition: "stopped", settled_at: null });
  expect(controls.daemons.visits.filter((input) => input.lane === "A")).toHaveLength(1);
  const resources = await fixture.webActor.client.requestJSON<CollaborationResources>(`${root}/resources`);
  expect(resources.targets.find((target) => target.target_id === fixture.targetA.targetId)?.authorized).toBe(true);
  const commandHistory = () => fixture.webActor.client.requestJSON<CollaborationCommandHistory>(`${root}/commands?limit=100`);
  const cancelledBeforeReconnect = (await commandHistory()).items.find((input) => input.id === authorWaiting.commandId);
  if (!cancelledBeforeReconnect?.cancellation?.event_id || !cancelledBeforeReconnect.cancellation.event_seq) {
    throw new Error("Removal must retain the exact automatic cancellation event");
  }
  expect(cancelledBeforeReconnect).toMatchObject({ state: "cancelled", reason: "participant_removed", group_id: null,
    author_user_id: fixture.desktopActor.id, content: authorWaiting.content, target_id: fixture.targetA.targetId,
    channel_id: fixture.targetA.channelId, batch_id: authorWaiting.batchId, seq: authorWaiting.sequence,
    cancellation: { kind: "automatic", actor_user_id: fixture.webActor.id } });
  expect(cancelledBeforeReconnect.cancellation.event_id).not.toBe(cancelledBeforeReconnect.input_event_id);
  expect(BigInt(cancelledBeforeReconnect.cancellation.event_seq)).toBeGreaterThan(BigInt(cancelledBeforeReconnect.input_event_seq!));
  const cancelledPausedBeforeReconnect = (await commandHistory()).items.find((input) => input.id === pausedAuthorWaiting.commandId);
  expect(cancelledPausedBeforeReconnect).toMatchObject({ state: "cancelled", reason: "participant_removed", group_id: null,
    author_user_id: fixture.desktopActor.id, content: pausedAuthorWaiting.content, target_id: fixture.targetB.targetId,
    channel_id: fixture.targetB.channelId, batch_id: pausedAuthorWaiting.batchId, seq: pausedAuthorWaiting.sequence,
    cancellation: { kind: "automatic", actor_user_id: fixture.webActor.id, event_id: expect.any(String) } });
  await refreshLifecyclePage(page, fixture);
  await assertLifecyclePaused(page, fixture, visit.input.identity.run_id, survivor, authorWaiting);
  const disclosure = page.locator(`[id="run-${visit.input.identity.run_id}"]`);
  await disclosure.getByRole("button", { name: `View records for ${visit.input.identity.run_id}`, exact: true }).click();
  await expect(disclosure.getByRole("region", { name: "Run records", exact: true })).toContainText("Stop accepted; termination remains unconfirmed");
  await controls.capture("removed-stop-unconfirmed", page, page.getByRole("region", { name: "AI execution", exact: true }));
  await controls.reconnectClients();
  await refreshLifecyclePage(page, fixture);
  await assertLifecyclePaused(page, fixture, visit.input.identity.run_id, survivor, authorWaiting);
  const beforeRuntimeReconnect = await read();
  await controls.proxy.reconnectRuntime();
  const afterRuntimeReconnect = await read();
  expect(afterRuntimeReconnect.execution).toEqual(beforeRuntimeReconnect.execution);
  expect(afterRuntimeReconnect.session).toEqual(beforeRuntimeReconnect.session);
  expect(controls.daemons.visits.filter((input) => input.lane === "A")).toHaveLength(1);
  expect(visit.namespace_id).toMatch(/^pid:\[\d+\]$/);
  expect(visit.closed_ms).not.toBeNull();
  const originalProofPath = `/api/collaboration/runtime/runs/${visit.input.identity.run_id}/proof`;
  const originalProofHash = controls.proxy.evidence().original_proof_hashes[originalProofPath];
  expect(originalProofHash).toMatch(/^[a-f0-9]{64}$/);
  expect((await commandHistory()).items.find((input) => input.id === authorWaiting.commandId)).toEqual(cancelledBeforeReconnect);
  expect((await commandHistory()).items.find((input) => input.id === pausedAuthorWaiting.commandId)).toEqual(cancelledPausedBeforeReconnect);
  await assertUntouched();

  const refused = await lifecycleControl(page, fixture, "resume", fixture.targetA, 409);
  expect(refused.receipt.code).toBe("stop_unconfirmed");
  expect((await read()).execution).toEqual(afterRuntimeReconnect.execution);
  const staleRemovedControl = await fetch(`${fixture.apiUrl}${root}/control`, { method: "POST", headers: { Authorization: `Bearer ${fixture.desktopActor.client.getToken()!}`,
    "Content-Type": "application/json", "Idempotency-Key": randomUUID() }, body: JSON.stringify(refused.input) });
  expect(staleRemovedControl.status).toBe(404);
  expect((await read()).execution).toEqual(afterRuntimeReconnect.execution);
  await handlerPage.reload();
  await expect(handlerPage.getByRole("button", { name: "Resume target", exact: true })).toHaveCount(0);
  await expect(handlerPage.getByRole("heading", { name: fixture.title, exact: true })).toHaveCount(0);

  controls.proxy.releaseProof(visit.input.identity.run_id);
  await controls.proxy.waitForAcceptedProof(visit.input.identity.run_id);
  await expect.poll(async () => (await read()).execution!.runs.find((run) => run.id === visit.input.identity.run_id), { timeout: 30_000 })
    .toMatchObject({ proof_state: "exited_proven", forced_disposition: "stopped", progress_state: "settled" });
  const proofConfirmed = await read();
  expect(proofConfirmed.execution!.channels.find((lane) => lane.id === fixture.targetA.channelId)?.queue_state).toBe("paused");
  expect(proofConfirmed.execution!.commands.find((input) => input.id === authorWaiting.commandId)?.state).toBe("cancelled");
  expect(await fixture.webActor.client.requestJSON(`${root}/control/operations/${refused.requestId}`)).toEqual(refused.receipt);
  expect(controls.proxy.evidence().original_proof_hashes[originalProofPath]).toBe(originalProofHash);
  expect((await commandHistory()).items.find((input) => input.id === authorWaiting.commandId)).toEqual(cancelledBeforeReconnect);
  expect((await commandHistory()).items.find((input) => input.id === pausedAuthorWaiting.commandId)).toEqual(cancelledPausedBeforeReconnect);
  await refreshLifecyclePage(page, fixture);
  await expect(page.locator(`[id="run-${visit.input.identity.run_id}"]`)).toContainText("Termination confirmed");
  await controls.capture("natural-proof-confirmed-still-paused", page, page.getByRole("region", { name: "AI execution", exact: true }));

  const invitation = await fixture.webActor.client.requestJSON<CollaborationOperation>(`${root}/participants`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...authority(proofConfirmed), expected_collaboration_revision: proofConfirmed.collaboration_revision, target_user_id: fixture.desktopActor.id,
  } });
  expect(invitation.code).toBe("participant_added");
  const rejoined = await read();
  expect(rejoined.execution!.commands.find((input) => input.id === authorWaiting.commandId)?.state).toBe("cancelled");
  expect(rejoined.execution!.commands.find((input) => input.id === original.commandId)?.state).toBe("stopped");
  expect(rejoined.execution!.channels.find((lane) => lane.id === fixture.targetA.channelId)?.queue_state).toBe("paused");
  expect((await commandHistory()).items.find((input) => input.id === authorWaiting.commandId)).toEqual(cancelledBeforeReconnect);
  expect((await commandHistory()).items.find((input) => input.id === pausedAuthorWaiting.commandId)).toEqual(cancelledPausedBeforeReconnect);
  await refreshLifecyclePage(handlerPage, fixture);
  await showLifecycleHistory(handlerPage, authorWaiting, "Cancelled", "The author was removed");
  const resubmitted = await lifecycleComment(handlerPage, fixture, fixture.targetA, fixture.desktopActor, authorWaiting.marker, client === "web" ? "electron" : "web");
  expect(resubmitted.commandId).not.toBe(authorWaiting.commandId);
  expect(resubmitted.requestId).not.toBe(authorWaiting.requestId);
  expect(resubmitted.receipt.comment_id).not.toBe(authorWaiting.receipt.comment_id);
  expect(resubmitted.receipt.event_id).not.toBe(authorWaiting.receipt.event_id);
  expect(BigInt(resubmitted.sequence)).toBeGreaterThan(BigInt(authorWaiting.sequence));
  expect(resubmitted.content).toBe(authorWaiting.content);
  const ready = await read();
  expect(ready.execution!.commands.find((input) => input.id === resubmitted.commandId)?.state).toBe("waiting");
  expect(ready.execution!.commands.find((input) => input.id === authorWaiting.commandId)?.state).toBe("cancelled");
  await controls.reconnectClients();
  await refreshLifecyclePage(page, fixture);
  await refreshLifecyclePage(handlerPage, fixture);
  expect((await read()).execution).toEqual(ready.execution);
  await controls.proxy.reconnectRuntime();
  expect((await read()).execution).toEqual(ready.execution);
  expect(controls.daemons.visits.filter((input) => input.lane === "A")).toHaveLength(1);
  expect(controls.proxy.evidence().original_proof_hashes[originalProofPath]).toBe(originalProofHash);
  expect((await commandHistory()).items.find((input) => input.id === authorWaiting.commandId)).toEqual(cancelledBeforeReconnect);
  expect((await commandHistory()).items.find((input) => input.id === pausedAuthorWaiting.commandId)).toEqual(cancelledPausedBeforeReconnect);
  await showLifecycleHistory(page, authorWaiting, "Cancelled", "The author was removed");
  await showLifecycleHistory(page, pausedAuthorWaiting, "Cancelled", "The author was removed");
  // Rejoining restores participation only. Explicit owner consent plus Resume
  // cannot revive the input that was already waiting on the paused B lane.
  const currentResources = await fixture.desktopActor.client.requestJSON<CollaborationResources>(`${root}/resources`);
  const targetB = currentResources.targets.find((target) => target.target_id === fixture.targetB.targetId)!;
  expect(targetB.authorized).toBe(false);
  const identityB = await fixture.desktopActor.client.requestJSON<CollaborationResourceIdentityView>(
    `${root}/resources/identity?target_id=${targetB.target_id}&agent_id=${fixture.targetB.resourceId}`);
  const reconsentB = await fixture.desktopActor.client.requestJSON<CollaborationResourceReceipt>(`${root}/targets/${targetB.target_id}/consents`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...authority(await readLifecycleIssue(fixture, fixture.desktopActor)),
      agent_id: fixture.targetB.resourceId, expected_target_epoch: targetB.target_epoch, expected_config_version: targetB.config_version, identity: identityB.identity },
  });
  expect(reconsentB).toMatchObject({ code: "resource_consented", target_id: fixture.targetB.targetId });
  expect((await read()).execution!.channels.find((lane) => lane.id === fixture.targetB.channelId)?.queue_state).toBe("paused");
  await refreshLifecyclePage(page, fixture);
  const resumeCancelledTarget = await lifecycleControl(page, fixture, "resume", fixture.targetB);
  expect(resumeCancelledTarget.receipt).toMatchObject({ code: "resumed", target_id: fixture.targetB.targetId, queue_state: "active" });
  expect((await commandHistory()).items.find((input) => input.id === pausedAuthorWaiting.commandId)).toEqual(cancelledPausedBeforeReconnect);
  expect((await read()).execution!.channels.find((lane) => lane.id === fixture.targetB.channelId)?.active_group_id).toBeNull();
  expect(controls.daemons.visits.filter((input) => input.lane === "B")).toHaveLength(0);
  const resumed = await lifecycleControl(page, fixture, "resume", fixture.targetA);
  expect(resumed.receipt).toMatchObject({ code: "resumed", scope: "target", target_id: fixture.targetA.targetId });
  const followup = await controls.daemons.waitForVisit("A", 1);
  assertProviderInput(followup, fixture, fixture.targetA, [survivor, addedWhilePaused, resubmitted]);
  controls.daemons.release(followup);
  await expect.poll(async () => (await read()).execution!.runs.find((run) => run.id === followup.input.identity.run_id), { timeout: 30_000 })
    .toMatchObject({ proof_state: "exited_proven", business_outcome: "success", progress_state: "settled" });
  const final = await read();
  expect(final.execution!.commands.find((input) => input.id === original.commandId)?.state).toBe("stopped");
  expect(final.execution!.commands.find((input) => input.id === authorWaiting.commandId)?.state).toBe("cancelled");
  expect(final.execution!.runs).toHaveLength(2);
  await refreshLifecyclePage(page, fixture);
  await refreshLifecyclePage(handlerPage, fixture);
  for (const currentPage of [page, handlerPage]) {
    await showLifecycleHistory(currentPage, original, "Stopped");
    await showLifecycleHistory(currentPage, authorWaiting, "Cancelled", "The author was removed");
    await showLifecycleHistory(currentPage, pausedAuthorWaiting, "Cancelled", "The author was removed");
    for (const command of [survivor, addedWhilePaused, resubmitted]) await showLifecycleHistory(currentPage, command, "Succeeded");
  }
  const history = await commandHistory();
  expect(history.items.find((input) => input.id === authorWaiting.commandId)).toEqual(cancelledBeforeReconnect);
  expect(history.items.find((input) => input.id === pausedAuthorWaiting.commandId)).toEqual(cancelledPausedBeforeReconnect);
  expect(controls.daemons.visits.filter((input) => input.lane === "B")).toHaveLength(0);
  const originalProofAttempts = controls.proxy.evidence().attempts.filter((attempt) => attempt.path === originalProofPath);
  expect(originalProofAttempts.some((attempt) => attempt.status === 503)).toBe(true);
  expect(originalProofAttempts.some((attempt) => attempt.status === 200)).toBe(true);
  expect(new Set(originalProofAttempts.map((attempt) => attempt.body_sha256))).toEqual(new Set([originalProofHash]));
  expect(controls.proxy.evidence().errors).toEqual([]);
  await assertUntouched();
  expect(await fixture.webActor.client.countIssueDispatches(fixture.issueId)).toBe(0);
  return { original, survivor, authorWaiting, pausedAuthorWaiting, addedWhilePaused, resubmitted, removal, refused, resumed,
    paused_target_stop: pausedTargetStop, paused_cancelled_target_reconsent: reconsentB, paused_cancelled_target_resume: resumeCancelledTarget,
    independent_target: controls.independent.targetId, peer_issue: controls.peerId, paused_execution: paused.execution,
    proof_confirmed_execution: proofConfirmed.execution, final_execution: final.execution, provider_observations: controls.daemons.evidence(),
    runtime_transport: controls.proxy.evidence(), original_proof_sha256: originalProofHash,
    cancelled_history_before_reconnect: cancelledBeforeReconnect, cancelled_history_after_resume: history.items.find((input) => input.id === authorWaiting.commandId),
    paused_cancelled_history_before_reconnect: cancelledPausedBeforeReconnect,
    paused_cancelled_history_after_resume: history.items.find((input) => input.id === pausedAuthorWaiting.commandId),
    scope: "Actual Web/Electron public UI, actual contained Linux synthetic provider and original natural sealed proof; no real provider account" };
}

export async function startLifecycleDaemons(fixture: LifecycleFixture, proxy: Awaited<ReturnType<typeof createLifecycleRuntimeTransport>>) {
  const host = process.env.COLLABORATION_CROSS_CLIENT_HOST_IPV4;
  if (!host || !isIPv4(host)) throw new Error("Lifecycle native execution needs the explicit Docker host IPv4");
  return startCrossClientDaemons({ ...fixture, apiUrl: proxy.apiUrl });
}

export const lifecyclePath = (fixture: LifecycleFixture) => crossClientAccountPath(fixture.issueId);
export type LifecycleVisit = Visit;
