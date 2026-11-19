import { createHash, randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
import { createServer, request as httpRequest, type IncomingHttpHeaders } from "node:http";
import type { Socket } from "node:net";
import { gunzipSync, inflateSync } from "node:zlib";
import { expect, type Browser, type TestInfo } from "@playwright/test";
import pg from "pg";
import { z } from "zod";
import {
  CollaborationAttachmentSchema, CollaborationCommentInputSchema, CollaborationCommentReceiptSchema,
  CollaborationCommandHistorySchema, CollaborationControlReceiptSchema, CollaborationIssueSchema,
  CollaborationNotificationsSchema, CollaborationOperationSchema, CollaborationParticipantsSchema,
  CollaborationResourceReceiptSchema, CollaborationResourcesSchema, CollaborationRunHandoffSchema,
  CollaborationRunRecordsSchema, type CollaborationIssue, type CollaborationNotification,
} from "@multica/core/collaboration";
import {
  adoptHostTarget, createHostActor, enrollHostRuntime, hostControl, hostEnvironment, hostMode,
  issueRoot, openHostAcceptanceGate, sendHostComment, startHostDaemon, versioned,
  waitForHostRuntime, type HostActor, type HostIssue, type HostVisit,
} from "./collaboration-host-runtime";
import {
  RecoveryEvidence, authenticatedSocket, disconnectObserver, recoveryEnvironmentReady, withSessions,
  type RecoveryActor, type RecoveryPlatform, type RecoverySession, type RecoverySurface,
} from "./collaboration-recovery";

const object = z.record(z.string(), z.unknown());
const sha = (bytes: string | Buffer) => createHash("sha256").update(bytes).digest("hex");
const digest = (value: unknown) => sha(JSON.stringify(value));
const now = () => new Date().toISOString();
const decimal = z.string().regex(/^[1-9][0-9]*$/);
const runRoot = (id: string) => `/api/collaboration/runtime/runs/${id}`;
const identitySchema = z.object({ deployment_id: z.uuid(), installation_id: z.uuid(), issue_id: z.uuid(), session_id: z.uuid(),
  target_id: z.uuid(), channel_id: z.uuid(), batch_id: z.uuid(), group_id: z.uuid(), run_id: z.uuid(),
  fence: decimal, claim_generation: decimal, launch_nonce: z.uuid() });
const permitSchema = z.object({ protocol_version: z.literal(3), identity: identitySchema,
  attempt_seq: decimal, attempt_nonce: z.uuid(), business_token: z.string().regex(/^mcb_[a-f0-9]{64}$/) });
const claimSchema = z.object({ protocol_version: z.literal(3), containment: z.literal("host_process_supervision_v1"),
  identity: identitySchema, termination_token: z.string().regex(/^mct_[a-f0-9]{64}$/), handoff_version: z.literal(1), recording_version: z.literal(1) });
type RuntimeIdentity = z.infer<typeof identitySchema>;
interface WireReply { status: number; headers: IncomingHttpHeaders; bytes: Buffer }
interface OriginalWire {
  path: string; method: string; authorization: string; bytes: Buffer; sha256: string;
  response?: WireReply;
}
interface RuntimeHTTPFact {
  at: string; run_id: string | null; operation: string; method: string; path: string;
  request_sha256: string; authorization_sha256: string | null; status: number | null;
  response_sha256: string | null; forwarded: boolean; proof_request_held: boolean; acknowledgement_lost: boolean;
}

export function recoveryRuntimeEnvironmentReady() {
  return recoveryEnvironmentReady() && process.env.PAY241_S3_RECOVERY_RUNTIME_ACCEPTANCE === "1" &&
    ["MULTICA_DEPLOYMENT_ID", "COLLABORATION_HOST_DAEMON", "COLLABORATION_HOST_PROVIDER", "COLLABORATION_HOST_CLI"].every((key) => Boolean(process.env[key]));
}

function safeError(error: unknown) {
  const value = error instanceof Error ? error : new Error(String(error));
  return { type: value.name, message: value.message.replace(/Bearer\s+[^\s"']+/gi, "Bearer [redacted]")
    .replace(/\b(?:mcb_|mct_|mdt_|mat_|mcn_|mul_|mrd_)[A-Za-z0-9_.-]+/g, "[redacted-capability]")
    .replace(/[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}/g, "[redacted-token]").slice(0, 4096) };
}

/** Allowlisted wire identity, never the raw private nonce or authorization. */
function wireIdentity(value: RuntimeIdentity) {
  const { launch_nonce, ...identity } = value;
  return { ...identity, launch_nonce_sha256: sha(launch_nonce) };
}

function wireJSON(reply: WireReply): unknown {
  const encoding = reply.headers["content-encoding"];
  const bytes = encoding === "gzip" ? gunzipSync(reply.bytes) : encoding === "deflate" ? inflateSync(reply.bytes) : reply.bytes;
  if (bytes.length > 4 * 1024 * 1024) throw new Error("Decoded runtime response exceeded its bound");
  return JSON.parse(bytes.toString("utf8")) as unknown;
}

function refusal(reply: WireReply) {
  const value = z.object({ code: z.string().min(1) }).parse(wireJSON(reply));
  return { status: reply.status, code: value.code, response_bytes: reply.bytes.length, response_sha256: sha(reply.bytes) };
}

/** This owner observes issued credentials in memory and forwards unmodified wire bytes. */
async function runtimeTransport(apiUrl: string) {
  const origin = new URL(apiUrl);
  if (origin.protocol !== "http:" || !["localhost", "127.0.0.1"].includes(origin.hostname)) throw new Error("Controlled runtime requires an owned loopback HTTP API");
  const sockets = new Set<Socket>(), errors: string[] = [], facts: RuntimeHTTPFact[] = [];
  const originals = new Map<string, OriginalWire>();
  const permits = new Map<string, z.infer<typeof permitSchema>>();
  const claims = new Map<string, z.infer<typeof claimSchema>>();
  const proofHolds = new Map<string, { wait: Promise<void>; release(): void }>();
  const lostACKs = new Set<string>();
  let closing = false;
  const key = (runId: string, operation: string) => `${runId}:${operation}`;
  const known = new Set(["claim", "permit", "proof", "outcome", "handoff/snapshots", "memory/base"]);

  const call = async (path: string, method: string, authorization: string, bytes: Buffer = Buffer.alloc(0), headers: IncomingHttpHeaders = {}): Promise<WireReply> =>
    new Promise((resolve, reject) => {
      const request = httpRequest(new URL(path, origin), { method, agent: false, headers: { ...headers, host: origin.host,
        authorization, "content-length": bytes.length, ...(bytes.length ? { "content-type": headers["content-type"] ?? "application/json" } : {}) }, timeout: 15_000 }, (response) => {
        const chunks: Buffer[] = []; let size = 0;
        response.on("data", (chunk: Buffer) => {
          size += chunk.length;
          if (size > 4 * 1024 * 1024) { response.destroy(new Error("Runtime response exceeded its bound")); return; }
          chunks.push(chunk);
        });
        response.once("end", () => resolve({ status: response.statusCode ?? 0, headers: response.headers, bytes: Buffer.concat(chunks) }));
        response.once("error", reject);
        response.once("aborted", () => reject(new Error("Runtime response aborted")));
      });
      request.once("error", reject);
      request.once("timeout", () => request.destroy(new Error("Controlled runtime request timed out")));
      request.end(bytes);
    });

  const server = createServer(async (request, response) => {
    let fact: RuntimeHTTPFact | undefined;
    try {
      const path = request.url ?? "/", url = new URL(path, origin), method = request.method ?? "GET";
      const match = /^\/api\/collaboration\/runtime\/runs\/([a-f0-9-]{36})\/(.+)$/.exec(url.pathname);
      const runId = match?.[1] ?? null, operation = match?.[2] ?? url.pathname;
      const chunks: Buffer[] = []; let size = 0;
      for await (const chunk of request) {
        const bytes = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
        size += bytes.length;
        if (size > 2 * 1024 * 1024) throw new Error("Runtime request exceeded its bound");
        chunks.push(bytes);
      }
      const bytes = Buffer.concat(chunks), authorization = request.headers.authorization ?? "";
      fact = { at: now(), run_id: runId, operation, method, path, request_sha256: sha(bytes),
        authorization_sha256: authorization ? sha(authorization) : null, status: null, response_sha256: null,
        forwarded: false, proof_request_held: false, acknowledgement_lost: false };
      facts.push(fact);
      let original: OriginalWire | undefined;
      if (runId && method === "POST" && known.has(operation)) {
        const identity = key(runId, operation), previous = originals.get(identity);
        if (previous && (previous.sha256 !== fact.request_sha256 || previous.authorization !== authorization || previous.path !== path)) {
          throw new Error("A natural runtime replay changed its original request");
        }
        original = previous ?? { path, method, bytes, authorization, sha256: fact.request_sha256 };
        originals.set(identity, original);
      }
      const hold = runId && operation === "proof" ? proofHolds.get(runId) : undefined;
      if (hold) { fact.proof_request_held = true; await hold.wait; }
      if (response.destroyed) return;
      fact.forwarded = true;
      const upstream = await call(path, method, authorization, bytes, request.headers);
      fact.status = upstream.status; fact.response_sha256 = sha(upstream.bytes);
      if (original && upstream.status === 200) original.response ??= upstream;
      if (runId && method === "POST" && upstream.status === 200 && operation === "permit") {
        const current = permitSchema.parse(wireJSON(upstream));
        expect(current.identity.run_id).toBe(runId); expect(current.attempt_seq).toBe("1");
        const previous = permits.get(runId);
        if (previous) expect(current).toEqual(previous);
        permits.set(runId, current);
      }
      if (runId && method === "POST" && upstream.status === 200 && operation === "claim") {
        const current = claimSchema.parse(wireJSON(upstream));
        expect(current.identity.run_id).toBe(runId); claims.set(runId, current);
      }
      if (runId && upstream.status >= 200 && upstream.status < 300 && lostACKs.has(key(runId, operation))) {
        fact.acknowledgement_lost = true;
        response.destroy();
        return;
      }
      response.writeHead(upstream.status, upstream.headers);
      response.end(upstream.bytes);
    } catch (error) {
      if (!closing && !response.destroyed) errors.push(`Controlled runtime forwarding failed: ${safeError(error).type}`);
      response.destroy();
    }
  });
  server.on("connection", (socket) => { sockets.add(socket); socket.once("close", () => sockets.delete(socket)); });
  await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(0, "127.0.0.1", resolve); });
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Controlled runtime transport did not bind");
  const originalFor = (runId: string, operation: string) => {
    const original = originals.get(key(runId, operation));
    if (!original) throw new Error(`The genuine original ${operation} request was not observed`);
    return original;
  };
  const issuedFor = (runId: string) => {
    const issued = permits.get(runId);
    if (!issued) throw new Error("The genuine original launch permit was not observed");
    return issued;
  };
  return {
    apiUrl: `http://127.0.0.1:${address.port}`,
    holdProof(runId: string) {
      if (proofHolds.has(runId)) throw new Error("The original proof is already held");
      let release!: () => void;
      const wait = new Promise<void>((resolve) => { release = resolve; });
      proofHolds.set(runId, { wait, release });
    },
    releaseProof(runId: string) { proofHolds.get(runId)?.release(); proofHolds.delete(runId); },
    loseReceiptACKs(runId: string) { for (const operation of ["outcome", "proof"]) lostACKs.add(key(runId, operation)); },
    allowReceiptACKs(runId: string) { for (const operation of ["outcome", "proof"]) lostACKs.delete(key(runId, operation)); },
    releaseAll() { for (const hold of proofHolds.values()) hold.release(); proofHolds.clear(); lostACKs.clear(); },
    async waitForHeldProof(runId: string) {
      await expect.poll(() => facts.some((fact) => fact.run_id === runId && fact.operation === "proof" && fact.proof_request_held), { timeout: 35_000 }).toBe(true);
      expect(errors).toEqual([]);
      expect(facts.filter((fact) => fact.run_id === runId && fact.operation === "proof").every((fact) => !fact.forwarded)).toBe(true);
    },
    async waitForCommittedProof(runId: string) {
      await expect.poll(() => facts.some((fact) => fact.run_id === runId && fact.operation === "proof" && fact.status === 200 && fact.acknowledgement_lost), { timeout: 35_000 }).toBe(true);
      expect(errors).toEqual([]);
    },
    async replay(runId: string, operation: "outcome" | "proof" | "handoff/snapshots" | "memory/base") {
      const original = originalFor(runId, operation);
      const response = await call(original.path, original.method, original.authorization, original.bytes);
      return { response, identity: { path: original.path, request_sha256: original.sha256,
        authorization_sha256: sha(original.authorization), status: response.status, response_sha256: sha(response.bytes) } };
    },
    async read(runId: string, suffix: string) {
      const issued = issuedFor(runId);
      return call(`${runRoot(runId)}/${suffix}`, "GET", `Bearer ${issued.business_token}`);
    },
    async lateOutcome(runId: string, changeOriginal: boolean) {
      const issued = issuedFor(runId), original = originals.get(key(runId, "outcome"));
      let input: Record<string, unknown>;
      if (changeOriginal) {
        if (!original?.response) throw new Error("Only an actually recorded original outcome can be changed for the refusal check");
        input = object.parse(JSON.parse(original.bytes.toString("utf8")));
        input = { ...input, body: `${z.string().parse(input.body)} Changed late result must be refused.` };
      } else {
        input = { protocol_version: 3, containment: hostMode, identity: issued.identity,
          attempt_seq: issued.attempt_seq, attempt_nonce: issued.attempt_nonce, event_id: randomUUID(),
          outcome: "failure", body: "Explicit late result must not write a new business effect." };
      }
      const bytes = Buffer.from(JSON.stringify(input));
      const reply = await call(`${runRoot(runId)}/outcome`, "POST", `Bearer ${issued.business_token}`, bytes);
      return { ...refusal(reply), request_sha256: sha(bytes), event_id: input.event_id, changed_original: changeOriginal };
    },
    recordedOutcome(runId: string) { return originals.get(key(runId, "outcome"))?.response?.status === 200; },
    credentialEvidence(runId: string) {
      const issued = issuedFor(runId), claim = claims.get(runId), original = originalFor(runId, "permit");
      if (!claim) throw new Error("The genuine original managed claim was not observed");
      const businessHash = sha(issued.business_token), terminationHash = sha(claim.termination_token);
      const daemonHash = sha(original.authorization.replace(/^Bearer /, ""));
      expect(new Set([businessHash, terminationHash, daemonHash]).size).toBe(3);
      return { identity: wireIdentity(issued.identity), attempt_seq: issued.attempt_seq, attempt_nonce_sha256: sha(issued.attempt_nonce),
        business_token_sha256: businessHash, termination_token_sha256: terminationHash, daemon_token_sha256: daemonHash,
        actual_permit_request_sha256: original.sha256, actual_permit_response_sha256: sha(original.response!.bytes) };
    },
    runFacts: (runId: string) => structuredClone(facts.filter((fact) => fact.run_id === runId)),
    evidence: () => ({ api_url: `http://127.0.0.1:${address.port}`, facts: structuredClone(facts), errors: [...errors],
      originals: [...originals.entries()].map(([identity, wire]) => ({ identity, path: wire.path, method: wire.method,
        request_sha256: wire.sha256, authorization_sha256: sha(wire.authorization),
        first_committed_status: wire.response?.status ?? null, first_committed_response_sha256: wire.response ? sha(wire.response.bytes) : null })) }),
    async close() {
      closing = true;
      for (const hold of proofHolds.values()) hold.release(); proofHolds.clear(); lostACKs.clear();
      for (const socket of sockets) socket.destroy();
      await new Promise<void>((resolve) => server.close(() => resolve()));
      originals.clear(); permits.clear(); claims.clear();
    },
  };
}

type ControlledDaemon = Awaited<ReturnType<typeof startHostDaemon>>;
interface RuntimeFixture {
  apiUrl: string; deploymentId: string; owner: HostActor; handler: HostActor; participant: HostActor; outsider: HostActor;
  actors: RecoveryActor[]; sourceWorkspace: { id: string; slug: string }; workspace: { id: string; slug: string };
  runtime: Awaited<ReturnType<typeof enrollHostRuntime>>; daemon: ControlledDaemon;
  transport: Awaited<ReturnType<typeof runtimeTransport>>; issues: HostIssue[];
}
type OriginalInput = Awaited<ReturnType<typeof sendHostComment>> & { input: z.infer<typeof CollaborationCommentInputSchema> };

function authority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Current human participant observations are required");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}
async function readIssue(fixture: RuntimeFixture, issue: HostIssue, actor = fixture.owner) {
  const current = CollaborationIssueSchema.parse(await actor.client.requestJSON(versioned(issueRoot(issue.id))));
  expect(current.execution?.protocol_version).toBe(6); expect(current.execution?.execution_gate.enabled).toBe(true);
  return current;
}
const surface = (fixture: RuntimeFixture, issue: HostIssue): RecoverySurface => ({ apiUrl: fixture.apiUrl, issueId: issue.id, title: issue.title,
  accountPath: `/account?view=participating&issue=${issue.id}` });
const actorFor = (fixture: RuntimeFixture, actor: HostActor) => {
  const current = fixture.actors.find((item) => item.id === actor.id);
  if (!current) throw new Error("The selected current runtime actor was not registered");
  return current;
};

async function withRuntime<T>(evidence: RecoveryEvidence, exercise: (fixture: RuntimeFixture) => Promise<T>): Promise<T> {
  if (!recoveryRuntimeEnvironmentReady()) throw new Error("Extras-B requires its explicitly authorized fresh controlled runtime allocation");
  expect(new URL(process.env.DATABASE_URL!).pathname.slice(1)).toBe(process.env.TASK_DATABASE_NAME);
  const env = hostEnvironment(), configuration = await fetch(`${env.apiUrl}/api/config`);
  expect(configuration.status).toBe(200);
  expect(await configuration.json()).toMatchObject({ auth_mode: "password", managed_installation_supported: true, deployment_id: env.deploymentId });
  const restoreGate = await openHostAcceptanceGate();
  let daemon: ControlledDaemon | undefined, transport: Awaited<ReturnType<typeof runtimeTransport>> | undefined;
  let result: { ok: true; value: T } | { ok: false; error: unknown };
  const cleanup: { owner: string; status: string; error?: ReturnType<typeof safeError> }[] = [];
  try {
    const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
    evidence.checkpoint("before_public_runtime_password_setup", { seed_suffix: suffix, database: process.env.TASK_DATABASE_NAME,
      api_url: env.apiUrl, deployment_id: env.deploymentId });
    const owner = await createHostActor(env.apiUrl, "Owner", suffix), handler = await createHostActor(env.apiUrl, "Handler", suffix);
    const participant = await createHostActor(env.apiUrl, "Viewer", suffix), outsider = await createHostActor(env.apiUrl, "Outside", suffix);
    const actors: RecoveryActor[] = [];
    for (const actor of [owner, handler, participant, outsider]) {
      const current = z.object({ id: z.uuid(), name: z.string() }).parse(await actor.client.requestJSON("/api/me"));
      expect(current.id).toBe(actor.id);
      actors.push({ client: actor.client, id: actor.id, name: current.name, async authenticateWeb(page) {
        await actor.authenticateWeb(page);
        await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
      } });
    }
    const workspaceSchema = z.object({ id: z.uuid(), slug: z.string() });
    const sourceWorkspace = workspaceSchema.parse(await owner.client.requestJSON("/api/workspaces", { method: "POST",
      body: { name: "S3 controlled source", slug: `s3-runtime-source-${suffix}` } }));
    const workspace = workspaceSchema.parse(await owner.client.requestJSON("/api/workspaces", { method: "POST",
      body: { name: "S3 separate controlled resource", slug: `s3-runtime-resource-${suffix}` } }));
    expect(sourceWorkspace.id).not.toBe(workspace.id);
    owner.client.setWorkspaceId(workspace.id); owner.client.setWorkspaceSlug(workspace.slug);
    const runtime = await enrollHostRuntime(owner.client, owner.id, workspace.id, env.apiUrl, env.deploymentId);
    evidence.checkpoint("issued_runtime_identity_before_daemon_start", { runtime_id: runtime.runtimeId, installation_id: runtime.installationId,
      binding_id: runtime.credential.binding_id, binding_epoch: runtime.credential.binding_epoch, runtime_config_epoch: runtime.runtimeConfigEpoch,
      source_workspace_id: sourceWorkspace.id, execution_workspace_id: workspace.id,
      actor_ids: { owner: owner.id, handler: handler.id, participant: participant.id, outsider: outsider.id } });
    transport = await runtimeTransport(env.apiUrl);
    daemon = await startHostDaemon(runtime, transport.apiUrl, false, { testReport: true });
    await waitForHostRuntime(owner.client, runtime.runtimeId);
    const fixture: RuntimeFixture = { ...env, owner, handler, participant, outsider, actors, sourceWorkspace, workspace, runtime,
      daemon, transport, issues: [] };
    evidence.set({ database: process.env.TASK_DATABASE_NAME, api_url: env.apiUrl, deployment_id: env.deploymentId,
      source_workspace_id: sourceWorkspace.id, execution_workspace_id: workspace.id,
      actor_ids: { creator_resource_owner: owner.id, handler: handler.id, external_participant: participant.id, outsider: outsider.id },
      runtime: { id: runtime.runtimeId, installation_id: runtime.installationId, binding_id: runtime.credential.binding_id,
        daemon_id: runtime.daemonId, readiness: daemon.ready },
      real_model_execution: false, installed_main_execution: false, execution_mode: hostMode,
      sources: ["e2e/collaboration-recovery-runtime.spec.ts", "e2e/fixtures/collaboration-recovery-runtime.ts", "e2e/fixtures/collaboration-host-runtime.ts",
        "e2e/fixtures/collaboration-host-provider.go", "e2e/fixtures/collaboration-recovery.ts", "e2e/fixtures/collaboration-native.ts",
        process.env.COLLABORATION_HOST_DAEMON!, process.env.COLLABORATION_HOST_PROVIDER!, process.env.COLLABORATION_HOST_CLI!]
        .map((path) => ({ path, sha256: sha(readFileSync(path)) })) });
    result = { ok: true, value: await exercise(fixture) };
  } catch (error) {
    result = { ok: false, error };
    evidence.checkpoint("runtime_primary_failure_before_owned_teardown", { original_error: safeError(error),
      provider_observations: daemon?.evidence() ?? [], original_ledger: daemon?.ledgerEvidence() ?? [], transport: transport?.evidence() ?? null });
  } finally {
    transport?.releaseAll();
    for (const [owner, action] of [["controlled_daemon", async () => {
      if (daemon) {
        const summary = await daemon.close();
        evidence.set({ daemon_final: summary, provider_observations: daemon.evidence() });
        expect(summary.exit_code).toBe(0); expect(summary.errors).toEqual([]); expect(summary.stderr).toBe("");
        const capacity = summary.messages.filter((item) => "claims" in item);
        expect(capacity.length).toBeGreaterThan(0);
        expect(capacity.every((item) => item.invalid_capacity === false && item.available_slots === 1 && item.active_tasks === 0)).toBe(true);
        expect(capacity.reduce((count, item) => count + item.claims, 0)).toBe(daemon.visits.length);
        expect(summary.ledger).toHaveLength(daemon.visits.length);
      }
    }], ["loopback_transport", async () => {
      if (transport) { evidence.set({ runtime_transport: transport.evidence() }); await transport.close(); }
    }], ["execution_gate", restoreGate]] as const) {
      try { await action(); cleanup.push({ owner, status: "fulfilled" }); }
      catch (error) { cleanup.push({ owner, status: "rejected", error: safeError(error) }); }
      evidence.set({ runtime_cleanup: cleanup });
    }
  }
  if (!result.ok) throw result.error;
  expect(cleanup.every((item) => item.status === "fulfilled")).toBe(true);
  return result.value;
}

async function createRuntimeIssue(fixture: RuntimeFixture, label: string, evidence: RecoveryEvidence): Promise<HostIssue> {
  const draftId = randomUUID(), uploadKey = randomUUID(), body = `S3 controlled ${label} input file. Explicit test-owned bytes.\n`;
  const upload = new FormData(); upload.set("draft_id", draftId); upload.set("source_workspace_id", fixture.sourceWorkspace.id);
  upload.set("file", new Blob([body], { type: "text/plain" }), `s3-${label}.txt`);
  const uploaded = await fetch(`${fixture.apiUrl}/api/collaboration/attachments/drafts`, { method: "POST", redirect: "error",
    headers: { Authorization: `Bearer ${fixture.owner.client.getToken()!}`, "Idempotency-Key": uploadKey }, body: upload });
  expect(uploaded.status).toBe(201);
  const attachment = CollaborationAttachmentSchema.parse(await uploaded.json());
  evidence.checkpoint("actual_controlled_input_upload_before_issue_creation", { upload_key: uploadKey, draft_id: draftId,
    attachment, plaintext_sha256: sha(body), uploader_user_id: fixture.owner.id });
  const requestId = randomUUID();
  let current = CollaborationIssueSchema.parse(await fixture.owner.client.requestJSON("/api/collaboration/issues", { method: "POST",
    headers: { "Idempotency-Key": requestId }, body: { source_workspace_id: fixture.sourceWorkspace.id,
      handler_user_id: fixture.owner.id, title: `S3 runtime ${label} ${requestId.slice(0, 8)}`,
      description: `S3 controlled ${label} private source and original runtime recovery.`, draft_id: draftId, attachment_ids: [attachment.id] } }));
  const root = issueRoot(current.id), transferKey = randomUUID(), handoffNote = `S3 original ${label} handoff source; keep the original result and input identities.`;
  const transfer = CollaborationOperationSchema.parse(await fixture.owner.client.requestJSON(`${root}/handoffs`, { method: "POST",
    headers: { "Idempotency-Key": transferKey }, body: { ...authority(current), purpose: "transfer", target_user_id: fixture.handler.id, note: handoffNote } }));
  expect(transfer.code).toBe("transferred");
  current = CollaborationIssueSchema.parse(await fixture.owner.client.requestJSON(versioned(root)));
  const invitationKey = randomUUID();
  const invitation = CollaborationOperationSchema.parse(await fixture.owner.client.requestJSON(`${root}/participants`, { method: "POST",
    headers: { "Idempotency-Key": invitationKey }, body: { ...authority(current), target_user_id: fixture.participant.id } }));
  expect(invitation.code).toBe("participant_added");
  const agent = z.object({ id: z.uuid(), name: z.string() }).parse(await fixture.owner.client.requestJSON("/api/agents", { method: "POST",
    body: { name: `S3 controlled ${label} Agent`, runtime_id: fixture.runtime.runtimeId, permission_mode: "private" } }));
  const adopted = await adoptHostTarget(fixture.owner, fixture.owner.id, current.id, fixture.workspace.id, "agent", agent.id,
    [{ role: "agent", agentId: agent.id, runtime: fixture.runtime }]);
  current = CollaborationIssueSchema.parse(await fixture.owner.client.requestJSON(versioned(root)));
  const target = current.execution?.targets.find((item) => item.id === adopted.targetId);
  if (!target?.channel_id) throw new Error("The real authorized runtime target has no channel");
  const issue: HostIssue = { id: current.id, title: current.title, description: current.description, targetId: target.id,
    channelId: target.channel_id, agentId: agent.id, name: agent.name, attachment: { id: attachment.id, name: attachment.name, sha256: sha(body) } };
  fixture.issues.push(issue);
  const participants = CollaborationParticipantsSchema.parse(await fixture.owner.client.requestJSON(`${root}/participants`));
  expect(participants.participants.map((item) => item.user_id).sort()).toEqual([fixture.owner.id, fixture.handler.id, fixture.participant.id].sort());
  expect(current.execution?.runs).toEqual([]); expect(current.execution?.commands).toEqual([]); expect(current.session.queue_state).toBe("paused");
  for (const actor of [fixture.handler, fixture.participant, fixture.outsider]) expect(await actor.client.getWorkspaces()).toEqual([]);
  evidence.checkpoint("public_password_issue_resource_and_original_source_ready", { issue, creation_key: requestId, upload_key: uploadKey,
    draft_id: draftId, upload: attachment, transfer_key: transferKey, transfer, invitation_key: invitationKey, invitation,
    actual_participants: participants, current });
  return issue;
}

async function submitInput(session: RecoverySession, issue: HostIssue, marker: string, evidence: RecoveryEvidence): Promise<OriginalInput> {
  const request = session.page.waitForRequest((request) => new URL(request.url()).pathname === `${issueRoot(issue.id)}/comments` && request.method() === "POST");
  const [captured, sent] = await Promise.allSettled([request, sendHostComment(session.page, issue, marker, session.actor.id)]);
  if (sent.status === "rejected") throw sent.reason;
  if (captured.status === "rejected") throw captured.reason;
  const input = CollaborationCommentInputSchema.parse(captured.value.postDataJSON());
  const requestId = z.uuid().parse((await captured.value.allHeaders())["idempotency-key"]);
  expect(requestId).toBe(sent.value.requestId);
  const receipt = CollaborationCommentReceiptSchema.parse(sent.value.receipt);
  expect(receipt.targets[0]).toMatchObject({ state: "waiting", command_id: sent.value.commandId, batch_id: sent.value.batchId });
  evidence.checkpoint("actual_platform_input_and_original_request", { platform: session.platform, actor_id: session.actor.id,
    request_id: requestId, input, input_sha256: digest(input), receipt });
  return { ...sent.value, receipt, input };
}

async function control(session: RecoverySession, issue: HostIssue, action: "stop" | "resume", evidence: RecoveryEvidence,
  scope: "target" | "issue" = "target") {
  const request = session.page.waitForRequest((request) => new URL(request.url()).pathname === `${issueRoot(issue.id)}/control` && request.method() === "POST");
  const [captured, acted] = await Promise.allSettled([request, hostControl(session.page, issue, action, scope)]);
  if (acted.status === "rejected") throw acted.reason;
  if (captured.status === "rejected") throw captured.reason;
  const requestId = z.uuid().parse((await captured.value.allHeaders())["idempotency-key"]), input = object.parse(captured.value.postDataJSON());
  const receipt = CollaborationControlReceiptSchema.parse(acted.value);
  evidence.checkpoint(scope === "issue" ? "actual_platform_issue_control" : "actual_platform_target_control", { request_id: requestId, input, receipt, actor_id: session.actor.id });
  return { requestId, input, receipt };
}

function assertVisit(fixture: RuntimeFixture, issue: HostIssue, visit: HostVisit, input: OriginalInput) {
  expect(visit).toMatchObject({ protocol_version: 3, execution_mode: hostMode, agent_id: issue.agentId,
    agent_owner_id: fixture.owner.id, resource_owner_id: fixture.owner.id, cli_hash: fixture.daemon.cliHash,
    identity: { issue_id: issue.id, target_id: issue.targetId, channel_id: issue.channelId, batch_id: input.batchId },
    cli_context: { issue_id: issue.id, title: issue.title, description: issue.description } });
  const original = { command_id: input.commandId, author_user_id: input.actorId, body: input.content, attachment_ids: [] };
  expect(visit.inputs).toEqual([original]);
  expect(visit.cli_context.inputs.map(({ command_id, author_user_id, body, attachment_ids }) => ({ command_id, author_user_id, body, attachment_ids }))).toEqual([original]);
  expect(visit.attachment_hashes).toEqual({ [issue.attachment!.id]: issue.attachment!.sha256 });
}

interface RuntimeSubcase { name: string; status: "NOT_RUN" | "RUNNING" | "PASSED" | "FAILED"; result: unknown }
function inventory(evidence: RecoveryEvidence, names: readonly string[]) {
  const cases: RuntimeSubcase[] = names.map((name) => ({ name, status: "NOT_RUN", result: null }));
  evidence.set({ inventory: cases, planned_subcases: names.length, retries: 0, business_case_count: null });
  return async <T>(name: string, action: () => Promise<T>) => {
    const item = cases.find((item) => item.name === name);
    if (!item || item.status !== "NOT_RUN") throw new Error("Each controlled runtime subcase must start exactly once");
    item.status = "RUNNING"; evidence.set({ inventory: cases, active_phase: name });
    try { const result = await action(); item.status = "PASSED"; item.result = result; return result; }
    catch (error) { item.status = "FAILED"; evidence.checkpoint("runtime_subcase_primary_failure", { subcase: name, original_error: safeError(error) }); throw error; }
    finally { evidence.set({ inventory: cases, actual_started_subcases: cases.filter((item) => item.status !== "NOT_RUN").length }); }
  };
}

/** Read every owned business row through the actual session-to-issue relation. */
async function audit(fixture: RuntimeFixture, evidence: RecoveryEvidence, stage: string) {
  const database = new pg.Client(process.env.DATABASE_URL!);
  await database.connect();
  try {
    await database.query("BEGIN READ ONLY");
    await database.query("SET LOCAL statement_timeout='5s'");
    const identity = await database.query<{ database: string }>("SELECT current_database() AS database");
    expect(identity.rows[0]?.database).toBe(process.env.TASK_DATABASE_NAME);
    const ids = fixture.issues.map((issue) => issue.id);
    const rows: Record<string, Record<string, unknown>[]> = {};
    const queries = {
      comments: `SELECT id,issue_id,author_type,author_id,revision::text,content,deleted_at::text
        FROM comment WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,created_at,id`,
      commands: `SELECT c.id,c.session_id,c.input_event_id,c.target_id,c.channel_id,c.batch_id,c.group_id,c.seq::text,
        c.author_user_id,c.participant_epoch::text,c.author_auth_epoch::text,c.lifecycle_epoch::text,c.state,c.reason,c.content
        FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id
        WHERE s.issue_id=ANY($1::uuid[]) ORDER BY s.issue_id,c.seq,c.id`,
      groups: `SELECT g.id,g.session_id,g.target_id,g.command_id,g.channel_id,g.batch_id,g.target_epoch::text,g.config_version::text,
        g.state,g.forced_disposition,g.settled_at::text FROM collaboration_execution_group g
        JOIN collaboration_session s ON s.id=g.session_id WHERE s.issue_id=ANY($1::uuid[]) ORDER BY s.issue_id,g.created_at,g.id`,
      runs: `SELECT r.id,r.session_id,r.target_id,r.command_id,r.group_id,r.agent_id,r.runtime_id,r.resource_owner_user_id,
        r.grant_id,r.grant_epoch::text,r.binding_id,r.binding_epoch::text,r.installation_id,r.target_epoch::text,r.config_version::text,
        r.fence::text,r.claim_generation::text,r.containment,r.authorized_attempt_seq::text,r.sealed_attempt_seq::text,
        r.progress_state,r.proof_state,r.business_outcome,r.forced_disposition,r.recording_incomplete,
        r.started_at::text,r.settled_at::text,r.prompt_snapshot::text AS prompt_json,r.preparation_snapshot::text AS preparation_json
        FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id
        WHERE s.issue_id=ANY($1::uuid[]) ORDER BY s.issue_id,r.created_at,r.id`,
      attempts: `SELECT a.run_id,a.attempt_seq::text,a.claim_generation::text,a.containment,a.authorized_at::text,
        a.attempt_nonce,a.launch_descriptor::text AS launch_json,a.terminal_observation::text AS terminal_json
        FROM collaboration_attempt a JOIN collaboration_run r ON r.id=a.run_id JOIN collaboration_session s ON s.id=r.session_id
        WHERE s.issue_id=ANY($1::uuid[]) ORDER BY a.run_id,a.attempt_seq`,
      capabilities: `SELECT b.run_id,b.attempt_seq::text,b.token_hash,b.created_at::text,b.revoked_at::text
        FROM collaboration_business_capability b JOIN collaboration_run r ON r.id=b.run_id JOIN collaboration_session s ON s.id=r.session_id
        WHERE s.issue_id=ANY($1::uuid[]) ORDER BY b.run_id,b.attempt_seq`,
      reservations: `SELECT rr.run_id,rr.execution_workspace_id,rr.agent_id,rr.runtime_id,rr.daemon_id,rr.installation_id,rr.binding_id,
        rr.created_at::text,rr.released_at::text FROM collaboration_runtime_reservation rr
        JOIN collaboration_run r ON r.id=rr.run_id JOIN collaboration_session s ON s.id=r.session_id
        WHERE s.issue_id=ANY($1::uuid[]) ORDER BY rr.run_id`,
      grants: `SELECT id,issue_id,target_id,owner_user_id,agent_id,runtime_id,installation_id,binding_id,binding_epoch::text,
        epoch::text,owner_auth_epoch::text,owner_participant_epoch::text,target_epoch::text,config_version::text,
        consented_at::text,revoked_at::text,revocation_reason FROM collaboration_grant WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,id`,
      participants: `SELECT issue_id,user_id,epoch::text,joined_at::text,revoked_at::text,revocation_reason
        FROM issue_participant WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,user_id`,
      source_memberships: `SELECT m.id,m.workspace_id,m.user_id,m.role FROM member m
        WHERE m.workspace_id IN (SELECT workspace_id FROM issue WHERE id=ANY($1::uuid[])) ORDER BY m.workspace_id,m.user_id,m.id`,
      inputs: `SELECT i.group_id,i.command_id,i.channel_id,i.batch_id,i.seq::text,i.consuming_run_id FROM collaboration_group_input i
        JOIN collaboration_execution_group g ON g.id=i.group_id JOIN collaboration_session s ON s.id=g.session_id
        WHERE s.issue_id=ANY($1::uuid[]) ORDER BY i.group_id,i.seq,i.command_id`,
      run_events: `SELECT id,issue_id,run_id,producer_event_id,seq::text,kind,payload::text AS payload_json
        FROM collaboration_run_event WHERE issue_id=ANY($1::uuid[]) ORDER BY run_id,seq,id`,
      events: `SELECT id,issue_id,event_seq::text,actor_user_id,kind,payload::text AS payload_json
        FROM collaboration_event WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,event_seq,id`,
      operations: `SELECT id,scope_id,actor_user_id,request_id,kind,payload_hash,result_code,result::text AS result_json,completed_at::text
        FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=ANY($1::uuid[]) ORDER BY scope_id,created_at,id`,
      notifications: `SELECT id,issue_id,recipient_user_id,event_seq::text,kind,is_mention,source_kind,source_id,run_id
        FROM collaboration_notification WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,event_seq,recipient_user_id,id`,
      uploads: `SELECT id,issue_id,draft_id,uploader_user_id,source_workspace_id,upload_request_id,version::text,
        content_sha256,size_bytes::text,delivered_at::text,discarded_at::text FROM collaboration_attachment
        WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,id`,
      references: `SELECT a.attachment_id,a.reference_kind,a.reference_id,a.reference_revision::text,a.removed_at::text
        FROM collaboration_attachment_reference a JOIN collaboration_attachment f ON f.id=a.attachment_id
        WHERE f.issue_id=ANY($1::uuid[]) ORDER BY a.attachment_id,a.reference_kind,a.reference_id,a.reference_revision`,
      ordinary_dispatches: `SELECT id,issue_id FROM agent_task_queue WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,id`,
      ordinary_attachments: `SELECT id,issue_id FROM attachment WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,id`,
      source_inbox: `SELECT id,issue_id FROM inbox_item WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,id`,
      source_subscribers: `SELECT issue_id,user_id FROM issue_subscriber WHERE issue_id=ANY($1::uuid[]) ORDER BY issue_id,user_id`,
    };
    for (const [name, query] of Object.entries(queries)) {
      const result = await database.query<Record<string, unknown>>(query, [ids]);
      rows[name] = result.rows.map((row) => {
        const safe: Record<string, unknown> = {};
        for (const [field, value] of Object.entries(row)) {
          if (field === "content" || field === "attempt_nonce" || field.endsWith("_json")) {
            safe[`${field.replace(/_json$/, "")}_sha256`] = value === null ? null : sha(z.string().parse(value));
          } else safe[field] = value;
        }
        return safe;
      });
    }
    await database.query("COMMIT");
    for (const name of ["ordinary_dispatches", "ordinary_attachments", "source_inbox", "source_subscribers"]) expect(rows[name]).toEqual([]);
    const snapshot = { database: process.env.TASK_DATABASE_NAME, issue_ids: ids, rows,
      counts: Object.fromEntries(Object.entries(rows).map(([name, rows]) => [name, rows.length])) };
    evidence.checkpoint(stage, snapshot);
    return snapshot;
  } catch (error) { await database.query("ROLLBACK").catch(() => undefined); throw error; }
  finally { await database.end(); }
}

async function notifications(fixture: RuntimeFixture, issue: HostIssue) {
  const result: Record<string, CollaborationNotification[]> = {};
  for (const actor of fixture.actors) {
    const items: CollaborationNotification[] = [], seen = new Set<string>(); let cursor: string | null = null;
    do {
      const query = new URLSearchParams({ include_archived: "true", limit: "100" }); if (cursor) query.set("cursor", cursor);
      const page = CollaborationNotificationsSchema.parse(await actor.client.requestJSON(`/api/me/collaboration-notifications?${query}`));
      for (const item of page.items) { expect(seen.has(item.id)).toBe(false); seen.add(item.id); items.push(item); }
      if (page.next_cursor) expect(page.next_cursor).not.toBe(cursor); cursor = page.next_cursor;
    } while (cursor);
    result[actor.id] = items.filter((item) => item.issue_id === issue.id);
  }
  expect(result[fixture.outsider.id]).toEqual([]);
  return result;
}

async function originalInputReceipt(issue: HostIssue, actor: RecoveryActor, input: OriginalInput, replayWrite: boolean) {
  if (replayWrite) {
    const replay = CollaborationCommentReceiptSchema.parse(await actor.client.requestJSON(`${issueRoot(issue.id)}/comments`, { method: "POST",
      headers: { "Idempotency-Key": input.requestId }, body: input.input }));
    expect(replay).toEqual(input.receipt);
  }
  const original = CollaborationCommentReceiptSchema.parse(await actor.client.requestJSON(`${issueRoot(issue.id)}/comments/operations/${input.requestId}`));
  expect(original).toMatchObject({ issue_id: input.receipt.issue_id, comment_id: input.receipt.comment_id,
    event_seq: input.receipt.event_seq, code: input.receipt.code, status: input.receipt.status });
  if (replayWrite) expect(original).toEqual(input.receipt);
  return original;
}

async function records(fixture: RuntimeFixture, issue: HostIssue, runId: string) {
  const page = CollaborationRunRecordsSchema.parse(await fixture.owner.client.requestJSON(versioned(`${issueRoot(issue.id)}/runs/${runId}/records?limit=100`)));
  expect(page.issue_id).toBe(issue.id); expect(page.run.id).toBe(runId); expect(page.next_cursor).toBeNull();
  expect(page.resource).toMatchObject({ agent_id: issue.agentId, runtime_id: fixture.runtime.runtimeId,
    execution_workspace_id: fixture.workspace.id, resource_owner_user_id: fixture.owner.id, execution_mode: hostMode });
  return page;
}

async function waitSettlement(fixture: RuntimeFixture, issue: HostIssue, visit: HostVisit) {
  const runId = z.uuid().parse(visit.identity.run_id);
  await expect.poll(async () => (await readIssue(fixture, issue)).execution!.runs.find((item) => item.id === runId), { timeout: 35_000 })
    .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", execution_mode: hostMode, launch_count: "1" });
  const run = (await readIssue(fixture, issue)).execution!.runs.find((item) => item.id === runId)!;
  expect(run.settled_at).not.toBeNull();
  expect(["process_group", "windows_job", "direct_process"]).toContain(run.descendant_cleanup?.scope);
  expect(["best_effort", "unverified", "tracked_scope_complete"]).toContain(run.descendant_cleanup?.state);
  await expect.poll(() => visit.closed_ms).not.toBeNull();
  return run;
}

async function recoverOriginal(fixture: RuntimeFixture, issue: HostIssue, visit: HostVisit, evidence: RecoveryEvidence) {
  const runId = z.uuid().parse(visit.identity.run_id), visitCount = fixture.daemon.visits.length;
  const launchCalls = fixture.transport.runFacts(runId).filter((fact) => fact.operation === "claim" || fact.operation === "permit");
  const stopped = await fixture.daemon.stopPreservingState();
  evidence.checkpoint("stopped_poller_retained_original_private_ledger", { run_id: runId, stopped, original_transport: fixture.transport.runFacts(runId) });
  expect(stopped.exit_code).toBe(0); expect(stopped.errors).toEqual([]); expect(stopped.stderr).toBe("");
  const pending = stopped.ledger.find((item) => item.run_id === runId);
  expect(pending).toMatchObject({ version: 2, sealed: true, proof_state: "host_exit_observed" });
  fixture.transport.allowReceiptACKs(runId);
  const restarted = await fixture.daemon.restart();
  await expect.poll(() => fixture.daemon.ledgerEvidence().find((item) => item.run_id === runId)?.phase, { timeout: 35_000 }).toBe("proof_acknowledged");
  const current = fixture.daemon.ledgerEvidence().find((item) => item.run_id === runId)!;
  expect(current.proof_receipt_id).toBe(pending!.proof_receipt_id); expect(current.proof_sha256).toBe(pending!.proof_sha256);
  expect(current.outcome_event_id).toBe(pending!.outcome_event_id); expect(current.outcome_sha256).toBe(pending!.outcome_sha256);
  expect(current.identity_sha256).toBe(pending!.identity_sha256); expect(current.launches).toEqual(pending!.launches);
  expect(current).toMatchObject({ launches: [{ attempt_seq: "1", kind: "provider", phase: "attempt_reaped", process_observed: true,
    authorized: true, terminal_accepted: true, terminal_kind: "managed_launch_ended" }] });
  expect(fixture.daemon.visits).toHaveLength(visitCount);
  expect(fixture.transport.runFacts(runId).filter((fact) => fact.operation === "claim" || fact.operation === "permit")).toEqual(launchCalls);
  const originalProof = await fixture.transport.replay(runId, "proof");
  expect(originalProof.response.status).toBe(200);
  const proof = z.object({ receipt_id: z.uuid(), accepted: z.literal(true) }).parse(wireJSON(originalProof.response));
  expect(proof.receipt_id).toBe(current.proof_receipt_id);
  const outcome = fixture.transport.recordedOutcome(runId) ? await fixture.transport.replay(runId, "outcome") : null;
  if (outcome) { expect(outcome.response.status).toBe(200); expect(object.parse(wireJSON(outcome.response))).toEqual({ code: "outcome_recorded" }); }
  evidence.checkpoint("new_poller_recovered_exact_original_requests_without_launch", { run_id: runId, restarted, original_ledger: pending,
    recovered_ledger: current, proof: originalProof.identity, original_outcome_ack: outcome?.identity ?? null,
    no_original_outcome_ack_claimed: outcome === null, preserved_launch_requests: launchCalls, visit_count: visitCount });
  return { restarted, ledger: current, proof: originalProof.identity, outcome: outcome?.identity ?? null, run: await waitSettlement(fixture, issue, visit) };
}

async function rejectLateResults(fixture: RuntimeFixture, issue: HostIssue, runId: string, evidence: RecoveryEvidence) {
  const before = await audit(fixture, evidence, "before_explicit_late_result_attempts");
  const changed = fixture.transport.recordedOutcome(runId) ? await fixture.transport.lateOutcome(runId, true) : null;
  if (changed) expect(changed).toMatchObject({ status: 409, code: "outcome_conflict" });
  const fresh = await fixture.transport.lateOutcome(runId, false);
  expect([403, 409]).toContain(fresh.status);
  const after = await audit(fixture, evidence, "late_result_refusals_leave_every_business_identity_unchanged");
  expect(after).toEqual(before);
  expect((await readIssue(fixture, issue)).execution!.runs.some((item) => item.id === runId)).toBe(true);
  return { changed_original: changed, fresh_late_event: fresh, before_counts: before.counts, after_counts: after.counts };
}

async function reconnect(session: RecoverySession, previousId: string, evidence: RecoveryEvidence, denied = false) {
  const at = now(); await session.context.setOffline(false);
  await expect.poll(() => session.page.evaluate(() => navigator.onLine)).toBe(true);
  if (!denied) {
    const current = await authenticatedSocket(session, at, previousId);
    evidence.checkpoint("actual_runtime_surface_reauthenticated_after_offline", { old_socket_id: previousId, current_socket_id: current.id,
      current_frames: current.frames, handshakes: session.handshakes, business_writes: session.writes });
    return current;
  }
  const detail = session.page.locator('[data-tab-scroll-root="account-detail"]');
  // Only the direct alert replaces the protected detail; nested panels can
  // report their own denials while the original issue is still being removed.
  const denial = detail.locator(':scope > [role="alert"]').filter({
    has: session.page.getByText("You no longer have access to this issue.", { exact: true }),
  });
  await expect(denial).toBeVisible({ timeout: 35_000 });
  await expect(session.page.getByRole("heading", { name: session.fixture.title, exact: true })).toHaveCount(0);
  await expect(detail.locator("#description, #discussion")).toHaveCount(0);
  await expect(session.page.getByRole("region", { name: "Issue actions", exact: true })).toHaveCount(0);
  await expect(session.page.getByRole("form", { name: "Comments", exact: true })).toHaveCount(0);
  await expect.poll(() => session.page.evaluate(({ userId, issueId }) => Object.keys(localStorage).filter((key) => {
    if (!key.includes("multica_collaboration_draft:") && !key.includes("multica_collaboration_delivery:")) return false;
    const record = JSON.parse(localStorage.getItem(key) ?? "null");
    return record?.scope?.userId === userId && (record.scope.issueId === issueId || record.scope.context?.issueId === issueId);
  }).length, { userId: session.actor.id, issueId: session.fixture.issueId })).toBe(0);
  const find = () => [...session.sockets].reverse().find((socket) => socket.created_at >= at && socket.id !== previousId && socket.closed_at === null &&
    session.handshakes.some((handshake) => handshake.request_id === socket.id && handshake.status === 101) &&
    (session.platform === "Web" || socket.frames.some((frame) => frame.direction === "received" && frame.type === "auth_ack")));
  await expect.poll(() => find() !== undefined, { timeout: 35_000 }).toBe(true);
  const current = find()!;
  if (session.platform === "Desktop") {
    expect(current.frames.findIndex((frame) => frame.direction === "sent" && frame.type === "auth")).toBe(0);
    expect(current.frames.findIndex((frame) => frame.direction === "received" && frame.type === "auth_ack")).toBeGreaterThan(0);
  } else expect(current.frames.some((frame) => frame.type === "auth")).toBe(false);
  expect(current.frames.some((frame) => frame.direction === "received" && frame.type === "subscribe_ack" && frame.scope === "issue" && frame.id === session.fixture.issueId)).toBe(false);
  evidence.checkpoint("reauthenticated_account_cannot_restore_removed_issue_subscription", { old_socket_id: previousId,
    current_socket_id: current.id, current_frames: current.frames, handshakes: session.handshakes, private_issue_removed: true });
  return current;
}

async function showResultNotification(session: RecoverySession, issue: HostIssue, runId: string, kind: "ai_completed" | "ai_failed" | "ai_stopped") {
  const labels = { ai_completed: "AI execution completed", ai_failed: "AI execution failed", ai_stopped: "AI stop confirmed" };
  await session.page.getByRole("button", { name: /^Notifications/ }).click();
  const popup = session.page.locator('[data-slot="popover-content"]').filter({ has: session.page.getByText("Notifications", { exact: true }) });
  await expect(popup).toBeVisible();
  const row = popup.getByRole("listitem").filter({ has: session.page.getByText(labels[kind], { exact: true }) });
  await expect(row).toHaveCount(1);
  const link = row.getByRole("link", { name: issue.title, exact: true });
  const href = await link.getAttribute("href"); if (!href) throw new Error("The proven result notification has no real destination");
  const destination = new URL(href, session.fixture.apiUrl);
  expect(destination.searchParams.get("issue")).toBe(issue.id); expect(destination.hash).toBe(`#run-${runId}`);
  await link.click(); await expect(popup).toBeHidden();
  const disclosure = session.page.locator(`[id="run-${runId}"]`);
  await expect(disclosure).toBeFocused();
  await expect(disclosure).toContainText(kind === "ai_stopped" ? "Managed execution stopped" : "Managed execution ended");
  const view = disclosure.getByRole("button", { name: new RegExp(`^(?:View|Hide) records for ${runId}$`) });
  if (await view.getAttribute("aria-expanded") !== "true") await view.click();
  const panel = disclosure.getByRole("region", { name: "Run records", exact: true });
  await expect(panel).toBeVisible(); await expect(panel).toContainText("Direct launches: 1"); await expect(panel).toContainText("Standard host execution");
  if (kind === "ai_completed") await expect(panel).toContainText(`Host fixture result for ${runId}`);
  if (kind === "ai_failed") await expect(panel).toContainText(`Synthetic controlled test execution failed for ${runId}`);
  return { kind, href, run_id: runId, actual_platform: session.platform };
}

export async function exerciseRecoveryRuntimeNotifications(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const run = inventory(evidence, ["proven_success_result", "proven_failed_or_incomplete_result", "proven_stopped_result"]);
  evidence.set({ planned_intentional_runs: 3, planned_physical_launches: 3, planned_original_inputs: 3 });
  return withRuntime(evidence, async (fixture) => {
    const issue = await createRuntimeIssue(fixture, "notifications", evidence);
    return withSessions(browser, platform, surface(fixture, issue), info, evidence, async (open) => {
      const actor = actorFor(fixture, fixture.handler), session = await open(actor);
      let socket = await authenticatedSocket(session, "");
      await control(session, issue, "resume", evidence, "issue");
      const allInputs: OriginalInput[] = [];
      const preserved = new Map<string, { records: Awaited<ReturnType<typeof records>>; notices: CollaborationNotification[] }>();
      for (const [name, mode, kind] of [
        ["proven_failed_or_incomplete_result", "failure", "ai_failed"],
        ["proven_success_result", "success", "ai_completed"],
        ["proven_stopped_result", "stopped", "ai_stopped"],
      ] as const) {
        await run(name, async () => {
          const input = await submitInput(session, issue, `S3 ${mode} original input`, evidence); allInputs.push(input);
          const visit = await fixture.daemon.waitForVisit(allInputs.length); assertVisit(fixture, issue, visit, input);
          const runId = z.uuid().parse(visit.identity.run_id);
          fixture.transport.holdProof(runId); fixture.transport.loseReceiptACKs(runId);
          const credentials = fixture.transport.credentialEvidence(runId), live = await records(fixture, issue, runId);
          expect(live.run).toMatchObject({ proof_state: "unknown", settled_at: null, launch_count: "1" });
          let stop: Awaited<ReturnType<typeof control>> | null = null;
          if (mode === "stopped") {
            await session.page.reload(); await expect(session.page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
            socket = await authenticatedSocket(session, "", socket.id);
            stop = await control(session, issue, "stop", evidence); expect(stop.receipt.code).toBe("stop_requested");
          } else fixture.daemon.release(visit, mode);
          await disconnectObserver(session, socket, evidence, `${name}_actual_observer_offline`);
          await fixture.transport.waitForHeldProof(runId);
          const preProof = await readIssue(fixture, issue), beforeNotices = await notifications(fixture, issue);
          expect(preProof.execution!.runs.find((item) => item.id === runId)).toMatchObject({ proof_state: "unknown", settled_at: null });
          for (const notices of Object.values(beforeNotices)) expect(notices.filter((notice) => notice.run_id === runId)).toEqual([]);
          evidence.checkpoint("natural_original_proof_held_before_group_result_notifications", { run_id: runId, current: preProof,
            actual_recipients_before_proof: beforeNotices, original_transport: fixture.transport.runFacts(runId), credentials });
          fixture.transport.releaseProof(runId);
          await fixture.transport.waitForCommittedProof(runId);
          const settled = await waitSettlement(fixture, issue, visit);
          expect(settled.business_outcome).toBe(mode === "stopped" ? null : mode);
          expect(settled.forced_disposition).toBe(mode === "stopped" ? "stopped" : null);
          const originalNotices = await notifications(fixture, issue), recipients = [fixture.owner.id, fixture.handler.id, fixture.participant.id];
          const notices = recipients.flatMap((id) => originalNotices[id]!.filter((notice) => notice.run_id === runId));
          expect(notices).toHaveLength(3); expect(new Set(notices.map((notice) => notice.id)).size).toBe(3);
          expect(new Set(notices.map((notice) => notice.event_seq)).size).toBe(1);
          for (const id of recipients) expect(originalNotices[id]!.filter((notice) => notice.run_id === runId)).toEqual([
            expect.objectContaining({ kind, issue_id: issue.id, source_kind: "issue", source_id: null, can_open: true, issue_title: issue.title, run_id: runId }),
          ]);
          const recovery = await recoverOriginal(fixture, issue, visit, evidence);
          socket = await reconnect(session, socket.id, evidence);
          const destination = await showResultNotification(session, issue, runId, kind);
          const beforeReplay = await audit(fixture, evidence, "before_original_input_and_receipt_replay");
          const receipt = await originalInputReceipt(issue, actor, input, true);
          if (stop) {
            const replay = CollaborationControlReceiptSchema.parse(await actor.client.requestJSON(`${issueRoot(issue.id)}/control`, {
              method: "POST", headers: { "Idempotency-Key": stop.requestId }, body: stop.input }));
            expect(replay).toEqual(stop.receipt);
          }
          const afterReplay = await audit(fixture, evidence, "original_platform_intents_replay_without_business_or_launch_effects");
          expect(afterReplay).toEqual(beforeReplay);
          const late = await rejectLateResults(fixture, issue, runId, evidence);
          const history = CollaborationCommandHistorySchema.parse(await fixture.owner.client.requestJSON(`${issueRoot(issue.id)}/commands?limit=100`));
          expect(history.next_cursor).toBeNull(); expect(history.items).toHaveLength(allInputs.length);
          expect(history.items.find((item) => item.id === input.commandId)).toMatchObject({
            state: mode === "failure" ? "failed" : mode === "stopped" ? "stopped" : "succeeded", author_user_id: actor.id,
            content: input.content, comment_id: input.receipt.comment_id, input_event_seq: input.receipt.event_seq });
          const currentRecords = await records(fixture, issue, runId), currentNotices = await notifications(fixture, issue);
          expect(currentNotices).toEqual(originalNotices);
          for (const [oldRun, old] of preserved) {
            expect(await records(fixture, issue, oldRun)).toEqual(old.records);
            expect(Object.values(currentNotices).flat().filter((notice) => notice.run_id === oldRun)).toEqual(old.notices);
          }
          preserved.set(runId, { records: currentRecords, notices: Object.values(currentNotices).flat().filter((notice) => notice.run_id === runId) });
          expect(fixture.daemon.visits).toHaveLength(allInputs.length);
          expect((await readIssue(fixture, issue)).execution!.runs).toHaveLength(allInputs.length);
          return { original_input: input, credentials, result_kind: kind, notification_ids: notices.map((notice) => notice.id),
            notification_event_seq: notices[0]!.event_seq, current_records: currentRecords, original_receipt: receipt,
            recovery, actual_destination: destination, late_results: late, original_stop: stop };
        });
      }
      const final = await audit(fixture, evidence, "complete_result_notification_runtime_denominator");
      expect(final.counts.runs).toBe(3); expect(final.counts.groups).toBe(3); expect(final.counts.attempts).toBe(3);
      expect(final.counts.commands).toBe(3); expect(final.counts.comments).toBe(3); expect(final.counts.uploads).toBe(1);
      expect(final.rows.notifications!.filter((row) => ["ai_completed", "ai_failed", "ai_stopped"].includes(z.string().parse(row.kind)))).toHaveLength(9);
      expect(fixture.daemon.visits).toHaveLength(3); expect(fixture.daemon.errors).toEqual([]);
      return { final, intentional_runs: 3, physical_launches: 3, recovery_launches: 0, inputs: allInputs };
    });
  });
}

interface ProtectedProbe { name: string; suffix?: string; replay?: "handoff/snapshots" | "memory/base" }
function protectedFact(probe: ProtectedProbe, reply: WireReply) {
  return { name: probe.name, path_suffix: probe.suffix ?? probe.replay, method: probe.replay ? "POST_ORIGINAL_REPLAY" : "GET",
    status: reply.status, response_bytes: reply.bytes.length, response_sha256: sha(reply.bytes) };
}

async function establishProtectedReads(fixture: RuntimeFixture, issue: HostIssue, visit: HostVisit, input: OriginalInput, evidence: RecoveryEvidence) {
  const runId = z.uuid().parse(visit.identity.run_id), facts: ReturnType<typeof protectedFact>[] = [];
  const probes: ProtectedProbe[] = [];
  const read = async (probe: ProtectedProbe) => {
    probes.push(probe);
    const reply = probe.replay ? (await fixture.transport.replay(runId, probe.replay)).response : await fixture.transport.read(runId, probe.suffix!);
    expect(reply.status).toBe(200); facts.push(protectedFact(probe, reply)); return reply;
  };
  const context = z.object({ issue_id: z.uuid(), title: z.string(), description: z.string(),
    inputs: z.array(z.object({ command_id: z.uuid(), author_user_id: z.uuid(), body: z.string() })) })
    .parse(wireJSON(await read({ name: "current_runtime_context", suffix: "context" })));
  expect(context).toMatchObject({ issue_id: issue.id, title: issue.title, description: issue.description });
  expect(context.inputs).toEqual([expect.objectContaining({ command_id: input.commandId, author_user_id: input.actorId, body: input.content })]);
  const preparation = z.object({ version: z.literal(2), identity: identitySchema,
    attachments: z.array(z.object({ id: z.uuid(), version: decimal })) })
    .parse(wireJSON(await read({ name: "original_preparation_manifest", suffix: "preparation" })));
  expect(preparation.identity.run_id).toBe(runId);
  const file = preparation.attachments.find((item) => item.id === issue.attachment!.id);
  if (!file) throw new Error("The genuine runtime preparation omitted the explicit controlled input file");
  const bytes = await read({ name: "input_file_content", suffix: `attachments/${file.id}/content?version=${file.version}` });
  expect(sha(bytes.bytes)).toBe(issue.attachment!.sha256);
  const comments = z.object({ protocol_version: z.literal(1), identity: identitySchema, issue_id: z.uuid(), next_cursor: z.string().nullable(),
    items: z.array(z.object({ id: z.uuid(), comment_revision: decimal, content: z.string(), issue_id: z.uuid() })) })
    .parse(wireJSON(await read({ name: "current_runtime_comment_page", suffix: "comments?limit=50&max_bytes=65536" })));
  expect(comments.issue_id).toBe(issue.id); expect(comments.identity.run_id).toBe(runId); expect(comments.next_cursor).toBeNull();
  const comment = comments.items.find((item) => item.id === input.receipt.comment_id);
  if (!comment) throw new Error("The genuine runtime comment page omitted the original UI comment");
  expect(comment.content).toBe(input.content);
  const chunk = z.object({ issue_id: z.uuid(), comment_id: z.uuid(), revision: decimal, content: z.string(), complete: z.boolean(), next_cursor: z.string().nullable() })
    .parse(wireJSON(await read({ name: "original_comment_content", suffix: `comments/${comment.id}/content?revision=${comment.comment_revision}&max_bytes=65536` })));
  expect(chunk).toMatchObject({ issue_id: issue.id, comment_id: comment.id, revision: comment.comment_revision, content: input.content, complete: true, next_cursor: null });
  const memory = z.object({ issue_id: z.uuid(), run_id: z.uuid(), version: z.string().regex(/^(0|[1-9][0-9]*)$/) })
    .parse(wireJSON(await read({ name: "current_runtime_memory", suffix: "memory" })));
  expect(memory).toMatchObject({ issue_id: issue.id, run_id: runId });
  const handoff = CollaborationRunHandoffSchema.parse(wireJSON(await read({ name: "live_runtime_handoff", suffix: "handoff" })));
  expect(handoff).toMatchObject({ issue_id: issue.id, run_id: runId, mode: "live", authority: { handler_id: fixture.handler.id } });
  const frozen = CollaborationRunHandoffSchema.parse(wireJSON(await read({ name: "original_frozen_handoff_rehydration", replay: "handoff/snapshots" })));
  expect(frozen).toMatchObject({ issue_id: issue.id, run_id: runId, mode: "snapshot", snapshot_id: expect.any(String) });
  const base = z.object({ issue_id: z.uuid(), run_id: z.uuid(), version: z.string() })
    .parse(wireJSON(await read({ name: "original_memory_base_replay", replay: "memory/base" })));
  expect(base).toMatchObject({ issue_id: issue.id, run_id: runId });
  expect(handoff.sources.length).toBeGreaterThan(0);
  for (const source of handoff.sources) {
    const query = new URLSearchParams({ kind: source.kind, sha256: source.sha256, offset: "0" });
    if (source.run_id) query.set("source_run_id", source.run_id); if (source.version) query.set("version", source.version);
    const readSource = z.object({ protocol_version: z.literal(1), issue_id: z.uuid(), run_id: z.uuid(), offset: z.literal(0),
      source: z.object({ id: z.uuid(), kind: z.string(), sha256: z.string(), content: z.string() }) })
      .parse(wireJSON(await read({ name: `genuine_${source.kind}_source_chunk`, suffix: `handoff/sources/${source.id}?${query}` })));
    expect(readSource).toMatchObject({ issue_id: issue.id, run_id: runId, source: { id: source.id, kind: source.kind, sha256: source.sha256, content: source.content } });
  }
  const credentials = fixture.transport.credentialEvidence(runId);
  evidence.checkpoint("original_business_capability_reads_actual_protected_content_before_withdrawal", { run_id: runId,
    credentials, protected_reads: facts, actual_frozen_snapshot_id: frozen.snapshot_id, actual_comment_id: comment.id,
    actual_input_attachment: file, actual_source_ids: handoff.sources.map((source) => ({ kind: source.kind, id: source.id, sha256: source.sha256 })) });
  return { probes, facts, credentials };
}

async function deniedProtectedReads(fixture: RuntimeFixture, issue: HostIssue, visit: HostVisit, protectedReads: Awaited<ReturnType<typeof establishProtectedReads>>, evidence: RecoveryEvidence) {
  const runId = z.uuid().parse(visit.identity.run_id), facts = [];
  for (const probe of protectedReads.probes) {
    const reply = probe.replay ? (await fixture.transport.replay(runId, probe.replay)).response : await fixture.transport.read(runId, probe.suffix!);
    expect([403, 404, 409]).toContain(reply.status);
    const denial = refusal(reply);
    expect(reply.bytes.toString("utf8")).not.toContain(issue.title); expect(reply.bytes.toString("utf8")).not.toContain(issue.description);
    facts.push({ ...protectedFact(probe, reply), code: denial.code });
  }
  expect(facts).toHaveLength(protectedReads.facts.length);
  expect(fixture.transport.credentialEvidence(runId)).toEqual(protectedReads.credentials);
  evidence.checkpoint("same_original_business_token_denied_every_previously_authorized_read", { run_id: runId,
    original_business_token_sha256: protectedReads.credentials.business_token_sha256,
    before: protectedReads.facts, after: facts, no_wrong_token_or_unsupported_route_substitution: true });
  return facts;
}

export async function exerciseRecoveryRuntimeWithdrawal(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const run = inventory(evidence, ["queued_input_author_withdrawal", "live_resource_grant_withdrawal"]);
  evidence.set({ planned_intentional_runs: 2, planned_physical_launches: 2, planned_original_inputs: 3 });
  return withRuntime(evidence, async (fixture) => {
    const retained: { issue: HostIssue; input: OriginalInput; waiting: OriginalInput | null; runId: string; snapshot: Awaited<ReturnType<typeof records>> }[] = [];
    for (const [index, name] of ["queued_input_author_withdrawal", "live_resource_grant_withdrawal"].entries()) {
      await run(name, async () => {
        const authorWithdrawal = name === "queued_input_author_withdrawal";
        const issue = await createRuntimeIssue(fixture, authorWithdrawal ? "author-withdrawal" : "grant-withdrawal", evidence);
        const actor = actorFor(fixture, authorWithdrawal ? fixture.participant : fixture.handler);
        // Await owned cleanup before the next scope can replace session evidence.
        try { return await withSessions(browser, platform, surface(fixture, issue), info, evidence, async (open) => {
          const session = await open(actor), socket = await authenticatedSocket(session, "");
          // The current handler owns controls even when another participant authors the input.
          if (authorWithdrawal) {
            const controller = await open(actorFor(fixture, fixture.handler)); await authenticatedSocket(controller, "");
            await control(controller, issue, "resume", evidence, "issue");
          } else await control(session, issue, "resume", evidence, "issue");
          const input = await submitInput(session, issue, `S3 ${name} actual live input`, evidence);
          const visit = await fixture.daemon.waitForVisit(index + 1); assertVisit(fixture, issue, visit, input);
          const runId = z.uuid().parse(visit.identity.run_id);
          fixture.transport.holdProof(runId); fixture.transport.loseReceiptACKs(runId);
          const waiting = authorWithdrawal ? await submitInput(session, issue, "S3 original queued input must never promote after author removal", evidence) : null;
          if (waiting) {
            expect(waiting.commandId).not.toBe(input.commandId); expect(waiting.batchId).not.toBe(input.batchId);
            expect((await readIssue(fixture, issue)).execution!.commands.find((item) => item.id === waiting.commandId))
              .toMatchObject({ state: "waiting", group_id: null, author_user_id: actor.id });
          }
          const before = await audit(fixture, evidence, "actual_live_run_original_reservation_and_waiting_input_before_withdrawal");
          expect(before.rows.runs!.find((item) => item.id === runId)).toMatchObject({ authorized_attempt_seq: "1", proof_state: "unknown", settled_at: null });
          expect(before.rows.reservations!.find((item) => item.run_id === runId)).toMatchObject({ runtime_id: fixture.runtime.runtimeId, released_at: null });
          const protectedReads = await establishProtectedReads(fixture, issue, visit, input, evidence);
          expect(before.rows.capabilities!.find((item) => item.run_id === runId)).toMatchObject({ attempt_seq: "1",
            token_hash: protectedReads.credentials.business_token_sha256, revoked_at: null });
          expect(await audit(fixture, evidence, "original_protected_reads_do_not_create_business_effects")).toEqual(before);
          expect(fixture.daemon.visits).toHaveLength(index + 1); expect(visit.closed_ms).toBeNull(); expect(visit.released_ms).toBeNull();
          fixture.daemon.freeze();
          let frozen = true;
          let primaryFailure: unknown;
          let thawFailure: unknown;
          let completed: Record<string, unknown> | undefined;
          try {
            await disconnectObserver(session, socket, evidence, `${name}_actual_actor_offline`);
            const observed = authority(await readIssue(fixture, issue)), requestId = randomUUID();
            let withdrawal: unknown;
            if (authorWithdrawal) {
              withdrawal = CollaborationOperationSchema.parse(await fixture.owner.client.requestJSON(`${issueRoot(issue.id)}/participants/${actor.id}/remove`, {
                method: "POST", headers: { "Idempotency-Key": requestId }, body: observed }));
              expect(withdrawal).toMatchObject({ code: "participant_removed" });
            } else {
              const resources = CollaborationResourcesSchema.parse(await fixture.owner.client.requestJSON(versioned(`${issueRoot(issue.id)}/resources`)));
              const resource = resources.targets.find((item) => item.target_id === issue.targetId)?.resources[0];
              if (!resource?.grant_id || !resource.grant_epoch) throw new Error("The actual live resource has no original current grant");
              const body = { execution_view_version: 6, expected_lifecycle_epoch: observed.expected_lifecycle_epoch,
                expected_participant_epoch: observed.expected_participant_epoch, expected_account_epoch: observed.expected_account_epoch, expected_grant_epoch: resource.grant_epoch };
              withdrawal = CollaborationResourceReceiptSchema.parse(await fixture.owner.client.requestJSON(`${issueRoot(issue.id)}/grants/${resource.grant_id}/revoke`, {
                method: "POST", headers: { "Idempotency-Key": requestId }, body }));
              expect(withdrawal).toMatchObject({ code: "grant_revoked", status: 200 });
              evidence.checkpoint("actual_live_owner_grant_request_and_original_observations", { request_id: requestId, grant_id: resource.grant_id, body, withdrawal });
            }
            evidence.checkpoint("real_authority_withdrawal_committed_while_original_provider_remains_live", { run_id: runId, actor_id: actor.id,
              request_id: requestId, withdrawal, provider_released_ms: visit.released_ms, provider_closed_ms: visit.closed_ms });
            const denied = await deniedProtectedReads(fixture, issue, visit, protectedReads, evidence);
            const late = await fixture.transport.lateOutcome(runId, false); expect([403, 409]).toContain(late.status);
            const beforeProof = await audit(fixture, evidence, "withdrawal_preserves_original_unknown_run_and_prevents_queued_promotion");
            expect(beforeProof.rows.runs!.find((item) => item.id === runId)).toMatchObject({ proof_state: "unknown", settled_at: null });
            expect(beforeProof.rows.capabilities!.find((item) => item.run_id === runId)?.revoked_at).not.toBeNull();
            expect(beforeProof.rows.inputs!.map((item) => item.command_id)).not.toContain(waiting?.commandId);
            if (waiting) expect(beforeProof.rows.commands!.find((item) => item.id === waiting.commandId)).toMatchObject({ state: "cancelled", group_id: null });
            fixture.daemon.thaw(); frozen = false;
            await fixture.transport.waitForHeldProof(runId);
            expect((await readIssue(fixture, issue)).execution!.runs.find((item) => item.id === runId)).toMatchObject({ proof_state: "unknown", settled_at: null });
            fixture.transport.releaseProof(runId); await fixture.transport.waitForCommittedProof(runId);
            const recovered = await recoverOriginal(fixture, issue, visit, evidence);
            expect(recovered.run.forced_disposition).toBe("stopped");
            await reconnect(session, socket.id, evidence, authorWithdrawal);
            const originalReceipt = await originalInputReceipt(issue, actor, input, !authorWithdrawal);
            const waitingReceipt = waiting ? await originalInputReceipt(issue, actor, waiting, false) : null;
            if (authorWithdrawal) {
              expect(originalReceipt.targets).toEqual([]); expect(waitingReceipt?.targets).toEqual([]);
              const deniedDetail = await fetch(`${fixture.apiUrl}${versioned(issueRoot(issue.id))}`, { redirect: "error", headers: { Authorization: `Bearer ${actor.client.getToken()!}` } });
              expect(deniedDetail.status).toBe(404);
            } else {
              const resources = CollaborationResourcesSchema.parse(await fixture.owner.client.requestJSON(versioned(`${issueRoot(issue.id)}/resources`)));
              expect(resources.targets.find((item) => item.target_id === issue.targetId)?.authorized).toBe(false);
              await expect(session.page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
            }
            await deniedProtectedReads(fixture, issue, visit, protectedReads, evidence);
            const snapshot = await records(fixture, issue, runId), final = await audit(fixture, evidence, "withdrawal_recovery_keeps_old_epochs_without_reenrollment_or_resume");
            expect(snapshot.items.filter((item) => item.kind === "input")).toEqual([
              expect.objectContaining({ command_id: input.commandId, author_user_id: actor.id, body: input.content }),
            ]);
            if (waiting) {
              const originalParticipant = before.rows.participants!.find((item) => item.issue_id === issue.id && item.user_id === actor.id)!;
              const departed = final.rows.participants!.find((item) => item.issue_id === issue.id && item.user_id === actor.id)!;
              expect(departed.epoch).toBe((BigInt(z.string().parse(originalParticipant.epoch)) + 1n).toString());
              expect(departed.revoked_at).not.toBeNull();
              expect(final.rows.commands!.find((item) => item.id === waiting.commandId)).toMatchObject({ state: "cancelled", group_id: null,
                author_user_id: actor.id, participant_epoch: before.rows.commands!.find((item) => item.id === waiting.commandId)!.participant_epoch,
                author_auth_epoch: before.rows.commands!.find((item) => item.id === waiting.commandId)!.author_auth_epoch,
                content_sha256: sha(waiting.content) });
              expect(final.rows.inputs!.some((item) => item.command_id === waiting.commandId)).toBe(false);
            } else {
              const originalGrant = before.rows.grants!.find((item) => item.issue_id === issue.id)!;
              const revokedGrant = final.rows.grants!.find((item) => item.id === originalGrant.id)!;
              expect(revokedGrant.epoch).toBe((BigInt(z.string().parse(originalGrant.epoch)) + 1n).toString());
              expect(revokedGrant.revoked_at).not.toBeNull();
            }
            expect(fixture.daemon.visits).toHaveLength(index + 1); expect(final.counts.runs).toBe(index + 1); expect(final.counts.attempts).toBe(index + 1);
            for (const prior of retained) {
              expect(await records(fixture, prior.issue, prior.runId)).toEqual(prior.snapshot);
              if (prior.waiting) expect(final.rows.commands!.find((item) => item.id === prior.waiting!.commandId)).toMatchObject({ state: "cancelled", group_id: null });
            }
            retained.push({ issue, input, waiting, runId, snapshot });
            completed = { issue_id: issue.id, run_id: runId, original_input: input, original_waiting_input: waiting,
              original_receipt: originalReceipt, waiting_receipt: waitingReceipt, authority_withdrawal: withdrawal,
              original_business_reads: protectedReads.facts, current_denials: denied, credentials: protectedReads.credentials,
              late_result: late, original_proof_recovery: recovered, final };
          } catch (error) {
            primaryFailure = error;
            evidence.checkpoint("withdrawal_primary_failure_before_thaw", { original_error: safeError(error), run_id: runId });
          }
          finally {
            if (frozen) try { fixture.daemon.thaw(); }
            catch (error) {
              thawFailure = error;
              evidence.checkpoint("runtime_thaw_cleanup_failed", { original_error: safeError(error), primary_failure_preserved: primaryFailure !== undefined });
            }
          }
          if (primaryFailure !== undefined) throw primaryFailure;
          if (thawFailure !== undefined) throw thawFailure;
          if (!completed) throw new Error("The complete withdrawal subcase result was not retained");
          return completed;
        }); } finally {
          const sessions = evidence.value.sessions ?? [];
          evidence.checkpoint(Array.isArray(sessions) && sessions.length > 0
            ? "runtime_withdrawal_client_scope_closed" : "runtime_withdrawal_client_scope_without_acquired_clients", {
            subcase: name, issue_id: issue.id, sessions,
            all_sessions_cleanup: evidence.value.all_sessions_cleanup ?? [],
          });
        }
      });
    }
    const final = await audit(fixture, evidence, "complete_author_and_live_grant_runtime_denominator");
    expect(final.counts.runs).toBe(2); expect(final.counts.groups).toBe(2); expect(final.counts.attempts).toBe(2);
    expect(final.counts.commands).toBe(3); expect(final.counts.comments).toBe(3); expect(final.counts.uploads).toBe(2);
    expect(fixture.daemon.visits).toHaveLength(2); expect(fixture.daemon.errors).toEqual([]);
    return { final, intentional_runs: 2, physical_launches: 2, recovery_launches: 0,
      old_cancelled_waiting_inputs: retained.filter((item) => item.waiting !== null).map((item) => item.waiting!.commandId) };
  });
}

// The output acceptance shares the original runtime/proof/ledger owners. Keep
// their accepted case bodies unchanged; its multipart observation is separate.
export {
  runtimeTransport, createRuntimeIssue, readIssue, surface, actorFor, submitInput, control, assertVisit, inventory,
  audit, records, waitSettlement, recoverOriginal, notifications, originalInputReceipt, wireJSON, refusal, safeError,
};
export type { RuntimeFixture, OriginalInput, WireReply };
