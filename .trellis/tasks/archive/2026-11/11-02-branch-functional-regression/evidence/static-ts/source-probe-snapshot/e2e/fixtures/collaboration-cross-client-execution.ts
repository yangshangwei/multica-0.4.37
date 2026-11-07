import { spawn, execFile, type ChildProcessWithoutNullStreams } from "node:child_process";
import { generateKeyPairSync, randomUUID, sign } from "node:crypto";
import { readFileSync, statSync } from "node:fs";
import { createServer, isIPv4, type Socket } from "node:net";
import { isAbsolute } from "node:path";
import { promisify } from "node:util";
import { expect, type Page, type Request } from "@playwright/test";
import type {
  CollaborationCommandHistory, CollaborationCommentInput, CollaborationCommentReceipt, CollaborationComments,
  CollaborationControlReceipt, CollaborationIssue, CollaborationMentionTargets, CollaborationResourceIdentityView,
  CollaborationResourceOptions, CollaborationResources, CollaborationRunRecords, CollaborationTriggerPreview,
} from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";

interface Challenge { challenge_id: string; signature_payload: string; body_payload: string }
interface ManagedCredential {
  binding_id: string; binding_epoch: string; capability_version: string; daemon_token: string;
  expires_at: string; principal_user_id: string; auth_version: string;
}
interface ResourceReceipt { status: number; code: string; offer_id: string | null; offer_version: string | null; target_id: string | null }
interface ProviderIdentity {
  deployment_id: string; installation_id: string; issue_id: string; session_id: string; target_id: string;
  channel_id: string; batch_id: string; group_id: string; run_id: string; fence: string; claim_generation: string;
}
interface ProviderInput {
  identity: ProviderIdentity;
  inputs: { command_id: string; author_user_id: string; body: string; attachment_ids: string[] }[];
}
interface ProviderVisit {
  lane: "A" | "B"; namespace_id: string; pid: number; input: ProviderInput;
  started_ms: number; released_ms: number | null; closed_ms: number | null; socket: Socket;
}
interface Runtime {
  runtimeId: string; workspaceId: string; daemonId: string; deploymentId: string; installationId: string;
  credential: ManagedCredential;
}
interface ExecutionTarget {
  lane: "A" | "B"; name: string; resourceId: string; targetId: string; channelId: string; ownerId: string; runtime: Runtime;
}
const execute = promisify(execFile);
const rootFor = (issueId: string) => `/api/collaboration/issues/${issueId}`;
export const crossClientAccountPath = (issueId: string) => `/account?view=participating&issue=${issueId}`;
export const crossClientEnvironmentReady = () => process.env.E2E_PASSWORD_AUTH === "1" && Boolean(process.env.DATABASE_URL
  && process.env.COLLABORATION_E2E_API_URL && process.env.NEXT_PUBLIC_API_URL && process.env.MULTICA_DEPLOYMENT_ID
  && process.env.COLLABORATION_ELECTRON_RENDERER_URL && process.env.COLLABORATION_CROSS_CLIENT_DAEMON
  && process.env.COLLABORATION_CROSS_CLIENT_PROVIDER && process.env.COLLABORATION_CROSS_CLIENT_HOST_IPV4);

function authority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("The cross-client fixture requires a current participant");
  return { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch,
    expected_account_epoch: issue.write_context.account_epoch };
}
function redact(value: string) {
  return value.replace(/\b(?:mdt_|mat_|mcn_|mul_)[A-Za-z0-9_.-]+/g, "[redacted capability]");
}

/** The server issues every credential and resource identity. No direct DB seed,
 * simulated claim, browser runtime response or synthetic proof is used. */
async function enrollRuntime(owner: TestApiClient, workspaceId: string, ownerId: string, apiUrl: string, deploymentId: string): Promise<Runtime> {
  const keys = generateKeyPairSync("ed25519");
  const publicKey = keys.publicKey.export({ format: "der", type: "spki" }).subarray(-32).toString("base64url");
  const proof = (challenge: Challenge) => ({ challenge_id: challenge.challenge_id,
    signature_payload: challenge.signature_payload, body_payload: challenge.body_payload,
    signature: sign(null, Buffer.from(challenge.signature_payload, "base64url"), keys.privateKey).toString("base64url") });
  const challenge = await owner.requestJSON<Challenge>("/api/installations/challenges", { method: "POST", body: {
    purpose: "enroll", public_key: publicKey, desktop_version: "0.4.40", os: "linux",
  } });
  expect(JSON.parse(Buffer.from(challenge.signature_payload, "base64url").toString("utf8")))
    .toMatchObject({ user_id: ownerId, deployment_id: deploymentId });
  const installation = await owner.requestJSON<{ installation_id: string }>("/api/installations/enroll", { method: "POST", body: proof(challenge) });
  const pat = await owner.requestJSON<{ token: string }>("/api/tokens", { method: "POST", body: { name: "AC33 synthetic runtime enrollment", expires_in_days: 1 } });
  const daemonId = randomUUID();
  const bindingChallenge = await owner.requestJSON<Challenge>("/api/installations/challenges", { method: "POST", body: {
    purpose: "bind", installation_id: installation.installation_id, workspace_id: workspaceId, daemon_id: daemonId,
    public_key: publicKey, expected_binding_epoch: null,
  } });
  const daemonJSON = async <T>(path: string, token: string, body: unknown): Promise<T> => {
    const response = await fetch(`${apiUrl}${path}`, { method: "POST", headers: {
      Authorization: `Bearer ${token}`, "Content-Type": "application/json", "X-Client-Capabilities": "managed_installation_v1",
    }, body: JSON.stringify(body) });
    if (!response.ok) throw new Error(`Public managed fixture ${path} failed: ${response.status}`);
    return response.json();
  };
  const credential = await daemonJSON<ManagedCredential>("/api/daemon/installation-bindings", pat.token, proof(bindingChallenge));
  expect(credential.principal_user_id).toBe(ownerId);
  if (!credential.daemon_token.startsWith("mdt_")) throw new Error("A scoped managed daemon credential was not issued");
  const registration = await daemonJSON<{ runtimes: { id: string }[] }>("/api/daemon/register", credential.daemon_token, {
    workspace_id: workspaceId, daemon_id: daemonId, device_name: "AC33 test-owned Linux daemon", cli_version: "0.4.40", launched_by: "desktop",
    runtimes: [{ name: "Test-owned Linux provider", type: "codex", status: "online" }],
  });
  expect(registration.runtimes).toHaveLength(1);
  const runtimeId = registration.runtimes[0]!.id;
  // The real helper registers this exact runtime again when it starts.
  await daemonJSON("/api/daemon/deregister", credential.daemon_token, { runtime_ids: [runtimeId] });
  return { runtimeId, workspaceId, daemonId, deploymentId, installationId: installation.installation_id, credential };
}

export async function seedCrossClientExecution() {
  if (!crossClientEnvironmentReady()) throw new Error("AC33 requires the task-owned password deployment and explicit Linux fixture binaries");
  const apiUrl = process.env.COLLABORATION_E2E_API_URL!;
  for (const value of [apiUrl, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) {
    expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
  }
  expect(new URL(apiUrl).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL!).origin);
  expect(["", "multica", "postgres", "template0", "template1"]).not.toContain(new URL(process.env.DATABASE_URL!).pathname.slice(1));
  const configuration = await fetch(`${apiUrl}/api/config`);
  expect(configuration.status).toBe(200);
  expect(await configuration.json()).toMatchObject({ auth_mode: "password", managed_installation_supported: true,
    deployment_id: process.env.MULTICA_DEPLOYMENT_ID });
  const suffix = randomUUID().replaceAll("-", "").slice(0, 14);
  const actor = async (lane: "A" | "B") => {
    const client = new TestApiClient();
    const username = `s6cross${lane.toLowerCase()}${suffix}`;
    const password = `Cross-${randomUUID()}!`;
    const account = await client.registerPassword(username, password, `Cross-client collaborator ${lane}`);
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
  const webActor = await actor("A");
  const desktopActor = await actor("B");
  expect(webActor.id).not.toBe(desktopActor.id);
  const sourceWorkspace = await webActor.client.ensureWorkspace("Cross-client source", `cross-source-${suffix}`);
  const workspaceA = await webActor.client.ensureWorkspace("Private resource A", `cross-a-${suffix}`);
  const workspaceB = await desktopActor.client.ensureWorkspace("Private resource B", `cross-b-${suffix}`);
  const issue = await webActor.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    source_workspace_id: sourceWorkspace.id, handler_user_id: desktopActor.id, title: `AC33 actual cross-client execution ${suffix}`,
    description: "Two independent test-owned Linux namespaces and two actual human collaborators. Synthetic stdlib provider only.", attachment_ids: [],
  } });
  const root = rootFor(issue.id);
  const targets: ExecutionTarget[] = [];
  for (const [lane, owner, workspace] of [["A", webActor, workspaceA], ["B", desktopActor, workspaceB]] as const) {
    const runtime = await enrollRuntime(owner.client, workspace.id, owner.id, apiUrl, process.env.MULTICA_DEPLOYMENT_ID!);
    const agent = await owner.client.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: {
      name: `Independent execution agent ${lane}`, runtime_id: runtime.runtimeId, permission_mode: "private",
    } });
    const options = await owner.client.requestJSON<CollaborationResourceOptions>(`${root}/resources/offers/options?execution_workspace_id=${workspace.id}`);
    const option = options.items.find((item) => item.kind === "agent" && item.resource_id === agent.id);
    if (!option?.available) throw new Error(`The explicitly enrolled ${lane} resource is unavailable`);
    expect(option.resources).toHaveLength(1);
    expect(option.resources[0]).toMatchObject({ agent_id: agent.id, runtime_id: runtime.runtimeId, owner_user_id: owner.id, execution_workspace_id: workspace.id });
    const offer = await owner.client.requestJSON<ResourceReceipt>(`${root}/resources/offers`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...authority(await owner.client.requestJSON<CollaborationIssue>(root)), kind: "agent", resource_id: agent.id, execution_workspace_id: workspace.id,
      roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })),
    } });
    expect(offer).toMatchObject({ status: 201, code: "resource_offered", target_id: null });
    if (!offer.offer_id || !offer.offer_version) throw new Error("The offered resource has no durable identity");
    const resources = await webActor.client.requestJSON<CollaborationResources>(`${root}/resources`);
    const adoption = await webActor.client.requestJSON<ResourceReceipt>(`${root}/resources/offers/${offer.offer_id}/adopt`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...authority(await webActor.client.requestJSON<CollaborationIssue>(root)), expected_offer_version: offer.offer_version,
      expected_target_set_version: resources.target_set_version,
    } });
    const adopted = (await owner.client.requestJSON<CollaborationResources>(`${root}/resources`)).targets.find((item) => item.target_id === adoption.target_id);
    if (!adopted) throw new Error("The public adoption has no target");
    expect(adopted.authorized).toBe(false);
    const identity = await owner.client.requestJSON<CollaborationResourceIdentityView>(`${root}/resources/identity?target_id=${adopted.target_id}&agent_id=${agent.id}`);
    expect(identity.identity).toMatchObject({ kind: "managed", installation_id: runtime.installationId, binding_id: runtime.credential.binding_id });
    const consent = await owner.client.requestJSON<ResourceReceipt>(`${root}/targets/${adopted.target_id}/consents`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...authority(await owner.client.requestJSON<CollaborationIssue>(root)), agent_id: agent.id,
      expected_target_epoch: adopted.target_epoch, expected_config_version: adopted.config_version, identity: identity.identity,
    } });
    expect(consent).toMatchObject({ status: 201, code: "resource_consented", target_id: adopted.target_id });
    const current = await webActor.client.requestJSON<CollaborationIssue>(root);
    const channelId = current.execution?.targets.find((item) => item.id === adopted.target_id)?.channel_id;
    if (!channelId) throw new Error("The real target has no channel");
    targets.push({ lane, name: adopted.name, resourceId: agent.id, targetId: adopted.target_id, channelId, ownerId: owner.id, runtime });
  }
  const [targetA, targetB] = targets;
  if (!targetA || !targetB) throw new Error("Both independent resources are required");
  for (const key of ["targetId", "channelId", "resourceId"] as const) expect(targetA[key]).not.toBe(targetB[key]);
  for (const key of ["runtimeId", "daemonId", "installationId", "workspaceId"] as const) expect(targetA.runtime[key]).not.toBe(targetB.runtime[key]);
  for (const person of [webActor, desktopActor]) {
    const mentions = await person.client.requestJSON<CollaborationMentionTargets>(`${root}/mention-targets`);
    expect(mentions.targets.map((item) => item.target_id).sort()).toEqual(targets.map((item) => item.targetId).sort());
    expect(mentions.participants.map((item) => item.user_id).sort()).toEqual([webActor.id, desktopActor.id].sort());
  }
  return { apiUrl, issueId: issue.id, title: issue.title, webActor, desktopActor, sourceWorkspace, targets, targetA, targetB };
}
export type CrossClientFixture = Awaited<ReturnType<typeof seedCrossClientExecution>>;

/** The callback only observes an actual contained executable and releases its
 * blocking socket. Outcome and sealed proof still travel through the Runner. */
export async function startCrossClientDaemons(fixture: CrossClientFixture) {
  const daemon = process.env.COLLABORATION_CROSS_CLIENT_DAEMON!;
  const provider = process.env.COLLABORATION_CROSS_CLIENT_PROVIDER!;
  const host = process.env.COLLABORATION_CROSS_CLIENT_HOST_IPV4!;
  if (!isIPv4(host)) throw new Error("The contained provider callback requires a numeric Docker host IPv4 address");
  for (const binary of [daemon, provider]) {
    if (!isAbsolute(binary) || !statSync(binary).isFile()) throw new Error("An explicit prebuilt test executable is required");
    expect(readFileSync(binary).subarray(0, 4).toString("hex")).toBe("7f454c46");
  }
  const started = process.hrtime.bigint();
  const now = () => Number(process.hrtime.bigint() - started) / 1_000_000;
  const visits: ProviderVisit[] = [];
  const callbackErrors: string[] = [];
  const sockets = new Set<Socket>();
  const server = createServer((socket) => {
    sockets.add(socket);
    socket.setEncoding("utf8");
    let body = "";
    let received = false;
    socket.on("error", (error) => callbackErrors.push(error.message));
    socket.on("close", () => {
      sockets.delete(socket);
      const visit = visits.find((item) => item.socket === socket);
      if (visit) visit.closed_ms = now();
    });
    socket.on("data", (chunk: string) => {
      if (received) return;
      body += chunk;
      if (body.length > 1_000_000) { callbackErrors.push("Provider callback exceeded its bounded fixture payload"); socket.destroy(); return; }
      const newline = body.indexOf("\n");
      if (newline < 0) return;
      received = true;
      try {
        const value: Omit<ProviderVisit, "started_ms" | "released_ms" | "closed_ms" | "socket"> = JSON.parse(body.slice(0, newline));
        expect(["A", "B"]).toContain(value.lane);
        expect(value.namespace_id).toMatch(/^pid:\[\d+\]$/);
        expect(value.pid).toBeGreaterThan(1);
        expect(value.input.identity.issue_id).toBe(fixture.issueId);
        if (visits.some((visit) => visit.input.identity.run_id === value.input.identity.run_id)) throw new Error("The provider executed the same actual run twice");
        visits.push({ ...value, started_ms: now(), released_ms: null, closed_ms: null, socket });
      } catch (error) { callbackErrors.push(redact(String(error))); socket.destroy(); }
    });
  });
  await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(0, "0.0.0.0", resolve); });
  const address = server.address();
  if (address === null || typeof address === "string") throw new Error("The fixture callback has no TCP port");
  const docker = async (args: string[]) => execute("docker", args, { timeout: 30_000, maxBuffer: 1 << 20 });
  const daemons: { name: string; lane: string; process: ChildProcessWithoutNullStreams; messages: Record<string, unknown>[]; stderr: string; exit: Promise<number | null> }[] = [];
  const owned: string[] = [];
  const release = (visit: ProviderVisit) => {
    if (visit.released_ms !== null || visit.socket.destroyed) return;
    visit.released_ms = now();
    visit.socket.write("r");
  };
  const close = async () => {
    visits.forEach(release);
    for (const child of daemons) child.process.stdin.end();
    const results = await Promise.all(daemons.map(async (child) => {
      const timeout = setTimeout(() => { void docker(["stop", "--time", "2", child.name]).catch(() => undefined); }, 15_000);
      try { return { lane: child.lane, exit_code: await child.exit, messages: child.messages, stderr: child.stderr }; }
      finally { clearTimeout(timeout); }
    }));
    for (const name of owned) await docker(["rm", "--force", name]).catch(() => undefined);
    for (const socket of sockets) socket.destroy();
    await new Promise<void>((resolve) => server.close(() => resolve()));
    return results;
  };
  try {
    for (const target of fixture.targets) {
      const name = `multica-gap-ac33-${target.lane.toLowerCase()}-${randomUUID().slice(0, 8)}`;
      await docker(["create", "--interactive", "--name", name, "--privileged", "--security-opt", "seccomp=unconfined", "--security-opt", "apparmor=unconfined",
        "--entrypoint", "/fixture-daemon", "redis:7-alpine", "__multica_collaboration_router_poller"]);
      owned.push(name);
      // This Docker daemon does not share macOS /tmp bind paths. Copy only
      // these two explicitly built test executables; never mount a user home.
      await docker(["cp", daemon, `${name}:/fixture-daemon`]);
      await docker(["cp", provider, `${name}:/fixture-provider`]);
      const child = spawn("docker", ["start", "--attach", "--interactive", name], { stdio: "pipe" });
      const output = { name, lane: target.lane, process: child, messages: [] as Record<string, unknown>[], stderr: "",
        exit: new Promise<number | null>((resolve, reject) => { child.once("error", reject); child.once("exit", resolve); }) };
      daemons.push(output);
      let pending = "";
      child.stdout.setEncoding("utf8");
      child.stdout.on("data", (value: string) => {
        pending += value;
        let newline: number;
        while ((newline = pending.indexOf("\n")) >= 0) {
          const line = pending.slice(0, newline); pending = pending.slice(newline + 1);
          if (line.trim()) { try { output.messages.push(JSON.parse(line)); } catch { output.stderr += `${redact(line)}\n`; } }
        }
      });
      child.stderr.setEncoding("utf8");
      child.stderr.on("data", (value: string) => { output.stderr = (output.stderr + redact(value)).slice(-64_000); });
      // Scoped capabilities travel only over stdin. Neither child arguments,
      // environment, Docker inspection nor evidence attachments contain them.
      const serverUrl = new URL(fixture.apiUrl); serverUrl.hostname = "host.docker.internal";
      child.stdin.write(`${JSON.stringify({ ServerURL: serverUrl.origin, WorkspaceID: target.runtime.workspaceId, RuntimeID: target.runtime.runtimeId,
        DaemonID: target.runtime.daemonId, DeploymentID: target.runtime.deploymentId, InstallationID: target.runtime.installationId,
        Directory: "/tmp/ac33-runtime", Executable: "/fixture-provider", Arguments: [target.lane, `${host}:${address.port}`], Credential: target.runtime.credential })}\n`);
      await expect.poll(() => output.messages.find((message) => message.ready === true) ?? { stderr: output.stderr }, { timeout: 30_000 })
        .toMatchObject({ ready: true, max_concurrent_tasks: 1 });
    }
  } catch (error) { await close(); throw error; }
  return { visits, callbackErrors, release, close,
    async waitForVisit(lane: "A" | "B", index: number) {
      await expect.poll(() => ({ errors: callbackErrors, count: visits.filter((visit) => visit.lane === lane).length }), { timeout: 30_000 })
        .toEqual({ errors: [], count: index + 1 });
      return visits.filter((visit) => visit.lane === lane)[index]!;
    },
    evidence: () => visits.map(({ lane, namespace_id, pid, input, started_ms, released_ms, closed_ms }) => ({ lane, namespace_id, pid,
      identity: Object.fromEntries(Object.entries(input.identity).filter(([key]) => key !== "launch_nonce")),
      inputs: input.inputs, started_ms, released_ms, closed_ms })),
  };
}
export type CrossClientDaemons = Awaited<ReturnType<typeof startCrossClientDaemons>>;

export function observeCrossClientWrites(page: Page, issueId: string, client: "web" | "electron", actorId: string) {
  const writes: { client: string; actor_id: string; path: string; method: string; request_id: string | null }[] = [];
  const listener = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (path.startsWith(rootFor(issueId)) && ["POST", "PUT", "PATCH", "DELETE"].includes(request.method())) {
      writes.push({ client, actor_id: actorId, path, method: request.method(), request_id: request.headers()["idempotency-key"] ?? null });
    }
  };
  page.on("request", listener);
  return { writes, dispose: () => page.off("request", listener) };
}
export async function readCrossClientIssue(fixture: CrossClientFixture) {
  const issue = await fixture.webActor.client.requestJSON<CollaborationIssue>(rootFor(fixture.issueId));
  if (!issue.execution) throw new Error("The exact execution projection is missing");
  expect(issue.execution.execution_gate.enabled).toBe(true);
  expect(issue.creator.user_id).toBe(fixture.webActor.id);
  expect(issue.handler.user_id).toBe(fixture.desktopActor.id);
  expect(issue.execution.targets).toHaveLength(2);
  expect(issue.execution.channels).toHaveLength(2);
  return issue;
}
export async function sendCrossClientComment(page: Page, fixture: CrossClientFixture, target: ExecutionTarget, marker: string, actorId: string, client: "web" | "electron") {
  const composer = page.getByRole("form", { name: "Comments", exact: true });
  await composer.getByLabel("Add comment", { exact: true }).fill(marker);
  await composer.getByRole("group", { name: "AI targets", exact: true }).getByRole("button", { name: target.name, exact: true }).click();
  const content = await composer.getByLabel("Add comment", { exact: true }).inputValue();
  expect(content).toContain(`mention://agent/${target.resourceId}`);
  const previewResponse = page.waitForResponse((response) => new URL(response.url()).pathname === `${rootFor(fixture.issueId)}/comments/trigger-preview`
    && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Preview delivery", exact: true }).click();
  const response = await previewResponse;
  expect(response.status()).toBe(200);
  const preview: CollaborationTriggerPreview = await response.json();
  expect(preview.targets).toEqual([expect.objectContaining({ target_id: target.targetId, resource_id: target.resourceId, state: "waiting", hold_reason: null })]);
  const sent = page.waitForResponse((response) => new URL(response.url()).pathname === `${rootFor(fixture.issueId)}/comments` && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Send comment", exact: true }).click();
  const result = await sent;
  expect(result.status()).toBe(201);
  const receipt: CollaborationCommentReceipt = await result.json();
  const input: CollaborationCommentInput = result.request().postDataJSON();
  const requestId = result.request().headers()["idempotency-key"];
  if (!requestId) throw new Error("The actual client omitted its immutable request key");
  expect(input.content).toBe(content);
  expect(input.target_versions.map((item) => item.target_id)).toEqual([target.targetId]);
  expect(receipt).toMatchObject({ issue_id: fixture.issueId, code: "comment_created", status: 201 });
  expect(receipt.targets).toHaveLength(1);
  const command = receipt.targets[0];
  if (!command?.command_id || !command.batch_id || !command.command_sequence) throw new Error("The actual UI send did not retain a command");
  expect(command).toMatchObject({ target_id: target.targetId, channel_id: target.channelId, state: "waiting" });
  await expect(composer.getByLabel("Add comment", { exact: true })).toHaveValue("");
  return { client, actorId, targetId: target.targetId, content, marker, requestId, receipt, commandId: command.command_id,
    batchId: command.batch_id, sequence: command.command_sequence, preview };
}
export type SentCrossClientComment = Awaited<ReturnType<typeof sendCrossClientComment>>;

export function assertProviderInput(visit: ProviderVisit, fixture: CrossClientFixture, target: ExecutionTarget, comments: SentCrossClientComment[]) {
  expect(visit.lane).toBe(target.lane);
  expect(visit.input.identity).toMatchObject({ deployment_id: target.runtime.deploymentId, installation_id: target.runtime.installationId,
    issue_id: fixture.issueId, target_id: target.targetId, channel_id: target.channelId, batch_id: comments[0]!.batchId });
  expect(visit.input.inputs.map(({ command_id, author_user_id, body, attachment_ids }) => ({ command_id, author_user_id, body, attachment_ids })))
    .toEqual(comments.map((comment) => ({ command_id: comment.commandId, author_user_id: comment.actorId, body: comment.content, attachment_ids: [] })));
}
export async function waitForCrossClientSettlement(fixture: CrossClientFixture, runId: string) {
  await expect.poll(async () => (await readCrossClientIssue(fixture)).execution!.runs.find((run) => run.id === runId), { timeout: 30_000 })
    .toMatchObject({ progress_state: "settled", proof_state: "exited_proven", business_outcome: "success", forced_disposition: null });
}

export async function resumeCrossClientIssue(page: Page, fixture: CrossClientFixture) {
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${rootFor(fixture.issueId)}/control` && response.request().method() === "POST");
  await page.getByRole("region", { name: "AI execution", exact: true }).getByRole("button", { name: "Resume all", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationControlReceipt = await response.json();
  expect(receipt).toMatchObject({ code: "resumed", action: "resume", scope: "issue", queue_state: "active" });
  expect(receipt.target_holds).toHaveLength(2);
  expect(receipt.target_holds.every((target) => target.hold_reason === null)).toBe(true);
  return receipt;
}

export async function inspectCrossClientRecords(page: Page, runs: ProviderVisit[], comments: SentCrossClientComment[], cancelled: SentCrossClientComment) {
  const panel = page.getByRole("region", { name: "AI execution", exact: true });
  const historyButton = panel.getByRole("button", { name: "Command history", exact: true });
  if ((await historyButton.getAttribute("aria-expanded")) !== "true") await historyButton.click();
  const history = panel.getByRole("region", { name: "Command history", exact: true });
  for (const comment of comments) {
    const entry = history.locator(`[data-command-id="${comment.commandId}"]`);
    await expect(entry).toBeVisible();
    await expect(entry).toContainText(comment.marker);
    await expect(entry).toContainText(comment.commandId === cancelled.commandId ? "Cancelled" : "Succeeded");
    await entry.locator("details > summary").click();
    await expect(entry).toContainText(comment.actorId);
    await expect(entry).toContainText(comment.targetId);
  }
  await expect(history.locator(`[data-command-id="${cancelled.commandId}"]`)).toContainText("Cancelled by a participant");
  for (const visit of runs) {
    const id = visit.input.identity.run_id;
    const disclosure = panel.locator(`[id="run-${id}"]`);
    await expect(disclosure).toBeVisible();
    await expect(disclosure).toContainText("Termination confirmed");
    await disclosure.getByRole("button", { name: `View records for ${id}`, exact: true }).click();
    const records = disclosure.getByRole("region", { name: "Run records", exact: true });
    await expect(records).toContainText(`AC33 synthetic ${visit.lane} result for ${id}`);
    await expect(records.getByText("Succeeded", { exact: true }).first()).toBeVisible();
    for (const input of visit.input.inputs) {
      await expect(records).toContainText(input.command_id);
    }
    await records.locator("details > summary").filter({ hasText: /^Recorded resource$/ }).click();
    await expect(records).toContainText(id);
  }
  return panel;
}

export async function verifyCrossClientFinal(fixture: CrossClientFixture, visits: ProviderVisit[], comments: SentCrossClientComment[], cancelled: SentCrossClientComment) {
  const root = rootFor(fixture.issueId);
  const issue = await readCrossClientIssue(fixture);
  const history = await fixture.webActor.client.requestJSON<CollaborationCommandHistory>(`${root}/commands?limit=100`);
  const listedComments = await fixture.webActor.client.requestJSON<CollaborationComments>(`${root}/comments`);
  expect(issue.execution!.commands).toHaveLength(5);
  expect(issue.execution!.groups).toHaveLength(3);
  expect(issue.execution!.runs).toHaveLength(3);
  expect(issue.execution!.runs.every((run) => run.proof_state === "exited_proven" && run.business_outcome === "success" && run.progress_state === "settled")).toBe(true);
  expect(issue.session.queue_state).toBe("active");
  expect(issue.execution!.channels.every((channel) => channel.queue_state === "active" && channel.active_group_id === null)).toBe(true);
  expect(issue.execution!.targets.every((target) => target.hold_reason === null)).toBe(true);
  expect(history.items).toHaveLength(5);
  expect(history.next_cursor).toBeNull();
  expect(listedComments.items.filter((comment) => comment.author_type === "member")).toHaveLength(5);
  const recordEvidence = [];
  for (const comment of comments) {
    const command = issue.execution!.commands.find((item) => item.id === comment.commandId)!;
    const historic = history.items.find((item) => item.id === comment.commandId)!;
    expect(command).toMatchObject({ author_user_id: comment.actorId, target_id: comment.targetId, batch_id: comment.batchId,
      state: comment.commandId === cancelled.commandId ? "cancelled" : "succeeded" });
    expect(historic).toMatchObject({ author_user_id: comment.actorId, content: comment.content, comment_id: comment.receipt.comment_id,
      target_id: comment.targetId, batch_id: comment.batchId, state: command.state });
  }
  const cancelledEntry = history.items.find((item) => item.id === cancelled.commandId)!;
  expect(cancelledEntry).toMatchObject({ group_id: null, reason: "cancelled_by_participant",
    cancellation: { actor_user_id: fixture.desktopActor.id, kind: "manual" } });
  for (const visit of visits) {
    const target = fixture.targets.find((item) => item.targetId === visit.input.identity.target_id)!;
    const group = issue.execution!.groups.find((item) => item.id === visit.input.identity.group_id)!;
    const originals = visit.input.inputs.map((input) => comments.find((comment) => comment.commandId === input.command_id)!);
    expect(group).toMatchObject({ target_id: target.targetId, channel_id: target.channelId, batch_id: visit.input.identity.batch_id,
      command_id: originals[0]!.commandId, state: "succeeded" });
    expect(group.inputs).toEqual(originals.map((comment) => ({ command_id: comment.commandId, channel_id: target.channelId,
      batch_id: comment.batchId, sequence: comment.sequence, consuming_run_id: visit.input.identity.run_id, reader_run_ids: [visit.input.identity.run_id] })));
    for (const comment of originals) expect(issue.execution!.commands.find((command) => command.id === comment.commandId)?.group_id).toBe(group.id);
    const records = await fixture.webActor.client.requestJSON<CollaborationRunRecords>(`${root}/runs/${visit.input.identity.run_id}/records?limit=100`);
    expect(records.issue_id).toBe(fixture.issueId);
    expect(records.run).toMatchObject({ id: visit.input.identity.run_id, group_id: visit.input.identity.group_id,
      command_id: originals[0]!.commandId, target_id: target.targetId, channel_id: target.channelId, batch_id: visit.input.identity.batch_id, agent_id: target.resourceId,
      progress_state: "settled", proof_state: "exited_proven", business_outcome: "success" });
    expect(records.resource).toMatchObject({ agent_id: target.resourceId, runtime_id: target.runtime.runtimeId,
      resource_owner_user_id: target.ownerId, execution_workspace_id: target.runtime.workspaceId });
    expect(records.items.filter((item) => item.kind === "input").map((item) => ({ command_id: item.command_id, author_user_id: item.author_user_id, body: item.body })))
      .toEqual(visit.input.inputs.map((item) => ({ command_id: item.command_id, author_user_id: item.author_user_id, body: item.body })));
    expect(records.items.filter((item) => item.kind === "result")).toEqual([expect.objectContaining({ outcome: "success", body: `AC33 synthetic ${visit.lane} result for ${visit.input.identity.run_id}` })]);
    expect(records.items.some((item) => item.kind === "input" && item.command_id === cancelled.commandId)).toBe(false);
    recordEvidence.push(records);
  }
  expect(new Set(visits.map((visit) => visit.input.identity.run_id)).size).toBe(3);
  expect(new Set(comments.map((comment) => comment.requestId)).size).toBe(5);
  expect((await fixture.desktopActor.client.requestJSON<CollaborationIssue>(root)).execution).toEqual(issue.execution);
  return { issue, history, records: recordEvidence, comments: listedComments };
}
