// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { managementResponseMAC, MANAGEMENT_RESPONSE } from "./managed-control";
import { prepareManagedInstallationSession, type ManagedInstallationSession } from "./managed-session";

const deploymentId = "11111111-1111-4111-8111-111111111111";
const installationId = "22222222-2222-4222-8222-222222222222";
const userId = "33333333-3333-4333-8333-333333333333";
const organizationId = "44444444-4444-4444-8444-444444444444";
const workspaceId = "88888888-8888-4888-8888-888888888888";
const userToken = `header.${Buffer.from(JSON.stringify({ sub: userId, auth_version: 1 })).toString("base64url")}.signature`;
let home: string;
let session: ManagedInstallationSession | null;
let mismatch: boolean;
let supported: boolean;
let busy: boolean;
let heartbeatProof: unknown;
let bindingHints: unknown[] | undefined;
let bindingHintsTruncated: boolean;
let lookupBinding: unknown;
let serverAuthVersion: string;
let localHandoffs: string[];
let fetchMock: ReturnType<typeof vi.fn>;

function issueChallenge(input: Record<string, unknown>) {
  const enroll = input.purpose === "enroll";
  const body = enroll ? { public_key: input.public_key, desktop_version: input.desktop_version, os: input.os }
    : { installation_id: input.installation_id, workspace_id: input.workspace_id, daemon_id: input.daemon_id, public_key: input.public_key, expected_binding_epoch: input.expected_binding_epoch };
  const bytes = Buffer.from(JSON.stringify(body));
  const payload = { protocol_version: "1", purpose: input.purpose, challenge_id: "55555555-5555-4555-8555-555555555555", deployment_id: deploymentId, organization_id: organizationId, user_id: userId, auth_version: serverAuthVersion,
    installation_id: enroll ? null : installationId, workspace_id: enroll ? null : workspaceId, daemon_id: enroll ? null : input.daemon_id,
    public_key_fingerprint: createHash("sha256").update(Buffer.from(String(input.public_key), "base64url")).digest("hex"), expected_binding_epoch: enroll ? null : input.expected_binding_epoch,
    nonce: Buffer.alloc(32, 1).toString("base64url"), expires_at: String(Math.floor(Date.now() / 1000) + 120), method: "POST", path: enroll ? "/api/installations/enroll" : "/api/daemon/installation-bindings", body_sha256: createHash("sha256").update(bytes).digest("hex") };
  return { challenge_id: payload.challenge_id, nonce: payload.nonce, deployment_id: deploymentId, expires_at: payload.expires_at, signature_payload: Buffer.from(JSON.stringify(payload)).toString("base64url"), body_payload: bytes.toString("base64url") };
}

beforeEach(async () => {
  home = await mkdtemp(join(tmpdir(), "managed-session-")); session = null; mismatch = false; supported = true; busy = false; localHandoffs = []; heartbeatProof = null; bindingHints = undefined; bindingHintsTruncated = false; lookupBinding = undefined; serverAuthVersion = "1";
  fetchMock = vi.fn(async (raw: string, init: RequestInit) => {
    const url = new URL(raw); const headers = new Headers(init.headers); const input = typeof init.body === "string" ? JSON.parse(init.body) : null;
    let result: unknown;
    if (url.pathname === "/api/config") result = { auth_mode: "password", deployment_id: deploymentId, managed_installation_supported: supported };
    else if (url.pathname === "/api/me") result = { id: mismatch && headers.get("Authorization") === "Bearer mul_daemon" ? organizationId : userId };
    else if (url.pathname === "/api/installations/challenges") result = issueChallenge(input);
    else if (url.pathname === "/api/installations/enroll") result = { installation_id: installationId, key_version: "1", metadata_proof: "mip_enroll-test", ...(bindingHints ? { bindings: bindingHints, bindings_truncated: bindingHintsTruncated } : {}) };
    else if (url.pathname === `/api/installations/${installationId}/binding`) {
      expect(init.method).toBe("GET");
      expect(headers.get("Authorization")).toBe(`Bearer ${userToken}`);
      expect(headers.get("X-Installation-Proof")).toBe("mip_enroll-test");
      expect(url.searchParams.get("workspace_id")).toBe(workspaceId);
      expect(url.searchParams.get("daemon_id")).toBe(session!.managedDaemonId);
      if (lookupBinding === undefined) return new Response("{}", { status: 503 });
      result = { binding: lookupBinding };
    }
    else if (url.pathname === "/api/workspaces") result = [{ id: workspaceId }];
    else if (url.pathname.endsWith("/heartbeat")) { heartbeatProof = input; result = { accepted: true, server_time: new Date().toISOString(), next_report_after: 60, metadata_proof: "mip_heartbeat-test" }; }
    else if (url.pathname === "/management/session") {
      expect(headers.has("X-Multica-Management-Token")).toBe(false);
      result = { capability_version: "1", ready: true, deployment_id: deploymentId, organization_id: organizationId, user_id: userId, auth_version: "1", installation_id: installationId, managed_daemon_id: session!.managedDaemonId, profile: "desktop-test", active_task_count: busy ? 1 : 0, workspace_ids: [] };
    } else if (url.pathname === "/management/handoff") { localHandoffs.push(String(init.body)); result = { accepted: true }; }
    else throw new Error(`Unexpected test endpoint ${url.pathname}`);
    const body = Buffer.from(JSON.stringify(result));
    const responseHeaders = new Headers({ "Content-Type": "application/json" });
    if (url.hostname === "127.0.0.1") responseHeaders.set(MANAGEMENT_RESPONSE, managementResponseMAC("A".repeat(43), init.method ?? "GET", url.pathname as "/management/session" | "/management/handoff", headers, 200, body));
    return new Response(body, { headers: responseHeaders });
  });
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(async () => { session?.dispose(); vi.unstubAllGlobals(); await rm(home, { recursive: true, force: true }); });

function options() { return { homeDirectory: home, profile: "desktop-test", apiBaseUrl: "https://test.example", userToken, daemonToken: "mul_daemon", userId, desktopVersion: "test", os: "linux" }; }

async function localControl() {
  const dir = join(home, ".multica/profiles/desktop-test"); await mkdir(dir, { recursive: true, mode: 0o700 });
  await writeFile(join(dir, "management-control.json"), JSON.stringify({ version: "1", token: "A".repeat(43), pid: 123 }), { mode: 0o600 });
}

describe("managed Desktop session", () => {
  it("creates only scoped proofs for the anonymous handoff and sends an explicit desktop heartbeat", async () => {
    session = await prepareManagedInstallationSession(options());
    expect(session).not.toBeNull();
    const raw = await session!.createHandoff(); const handoff = JSON.parse(raw);
    expect(handoff).toMatchObject({ user_id: userId, auth_version: "1", installation_id: installationId });
    expect(handoff.bindings).toHaveLength(1);
    expect(raw).not.toContain(userToken); expect(raw).not.toContain("mul_daemon"); expect(raw).not.toContain("private_key_seed");
    expect(JSON.stringify(session)).toBe("{}");
    await session!.reportHeartbeat();
    const proof = heartbeatProof as { signature_payload: string };
    const payload = JSON.parse(Buffer.from(proof.signature_payload, "base64url").toString("utf8"));
    expect(payload).toMatchObject({ purpose: "heartbeat", installation_id: installationId, user_id: userId, auth_version: "1", sequence: "1", method: "POST", path: `/api/installations/${installationId}/heartbeat` });
    expect(session!.metadataProof?.proof).toBe("mip_heartbeat-test");
  });

  it("rejects a daemon credential owned by another user before enrollment", async () => {
    mismatch = true;
    await expect(prepareManagedInstallationSession(options())).rejects.toThrow("same complete password identity");
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes("/enroll"))).toBe(false);
  });

  it("leaves unbound old servers in legacy mode but never downgrades a managed profile", async () => {
    supported = false;
    expect(await prepareManagedInstallationSession(options())).toBeNull();
    await expect(prepareManagedInstallationSession({ ...options(), previousDeploymentId: deploymentId })).rejects.toThrow("cannot downgrade");
  });

  it("uses server epoch hints for another profile and after credential rotation", async () => {
    const first = await prepareManagedInstallationSession(options());
    bindingHints = [{ binding_id: "99999999-9999-4999-8999-999999999999", workspace_id: workspaceId, daemon_id: first!.managedDaemonId, binding_epoch: "7", auth_version: "1" }];
    first!.dispose();
    serverAuthVersion = "2";
    const currentToken = `header.${Buffer.from(JSON.stringify({ sub: userId, auth_version: 2 })).toString("base64url")}.signature`;
    session = await prepareManagedInstallationSession({ ...options(), profile: "desktop-second", userToken: currentToken });
    const handoff = JSON.parse(await session!.createHandoff());
    const payload = JSON.parse(Buffer.from(handoff.bindings[0].proof.signature_payload, "base64url").toString("utf8"));
    expect(payload).toMatchObject({ expected_binding_epoch: "7", auth_version: "2", user_id: userId });
    expect(handoff.managed_daemon_id).toBe(bindingHints[0] && (bindingHints[0] as { daemon_id: string }).daemon_id);
    expect(handoff).not.toHaveProperty("daemon_token");
  });

  it("requires private authenticated IPC and refuses a busy daemon without sending handoff", async () => {
    session = await prepareManagedInstallationSession(options());
    expect(await session!.handoffToRunningDaemon(19001)).toBe(false);
    await localControl(); busy = true;
    expect(await session!.handoffToRunningDaemon(19001)).toBe(false);
    expect(localHandoffs).toEqual([]);
    busy = false;
    expect(await session!.handoffToRunningDaemon(19001)).toBe(true);
    expect(localHandoffs).toHaveLength(1);
  });
});

async function storeCredential(token: string, epoch: string) {
  const directory = join(home, ".multica/management", deploymentId, "credentials", userId);
  await mkdir(directory, { recursive: true, mode: 0o700 });
  const profile = "desktop-test";
  const path = join(directory, `${createHash("sha256").update(profile).digest("hex")}.json`);
  await writeFile(path, JSON.stringify({ version: "1", server_url: options().apiBaseUrl, deployment_id: deploymentId, user_id: userId, profile, installation_id: installationId, managed_daemon_id: session!.managedDaemonId, auth_version: "1", bindings: [{ workspace_id: workspaceId, credential: { binding_epoch: epoch, daemon_token: token, auth_version: "1", principal_user_id: userId, expires_at: new Date(Date.now() + 3_600_000).toISOString() } }] }), { mode: 0o600 });
}

it("replays pending proof bytes until a newly persisted matching credential acknowledges them", async () => {
  const first = await prepareManagedInstallationSession(options());
  bindingHints = [{ binding_id: "99999999-9999-4999-8999-999999999999", workspace_id: workspaceId, daemon_id: first!.managedDaemonId, binding_epoch: "2", auth_version: "1" }];
  first!.dispose(); session = await prepareManagedInstallationSession(options());
  await storeCredential("mdt_before", "1");
  const firstProof = await session!.createHandoff();
  const challengeCount = () => fetchMock.mock.calls.filter(([url, init]) => String(url).endsWith("/challenges") && String(init.body).includes('"purpose":"bind"')).length;
  expect(challengeCount()).toBe(1);
  await session!.acknowledgeHandoff(firstProof);
  expect(await session!.createHandoff()).toBe(firstProof);
  expect(challengeCount()).toBe(1);
  await storeCredential("mdt_after", "2");
  await session!.acknowledgeHandoff(firstProof);
  expect(JSON.parse(await session!.createHandoff(new Set([workspaceId]))).bindings).toEqual([]);
  expect(challengeCount()).toBe(1);
});

it("replaces an expired pending challenge without persisting its proof", async () => {
  session = await prepareManagedInstallationSession(options());
  const first = await session!.createHandoff();
  const now = Date.now();
  const clock = vi.spyOn(Date, "now").mockReturnValue(now + 121_000);
  try {
    const next = await session!.createHandoff();
    expect(next).not.toBe(first);
    expect(JSON.parse(next).bindings).toHaveLength(1);
  } finally { clock.mockRestore(); }
});

it("refuses to infer an omitted workspace epoch from truncated enrollment hints", async () => {
  bindingHints = []; bindingHintsTruncated = true;
  session = await prepareManagedInstallationSession(options());
  await storeCredential("mdt_old_scope", "7");
  await expect(session!.createHandoff()).rejects.toThrow("Management request failed (503)");
  expect(fetchMock.mock.calls.some(([url, init]) => String(url).endsWith("/challenges") && String(init.body).includes('"purpose":"bind"'))).toBe(false);
});

it("can still use an authoritative included workspace hint when other hints are truncated", async () => {
  const first = await prepareManagedInstallationSession(options());
  bindingHints = [{ binding_id: "99999999-9999-4999-8999-999999999999", workspace_id: workspaceId, daemon_id: first!.managedDaemonId, binding_epoch: "7", auth_version: "1" }];
  bindingHintsTruncated = true; first!.dispose();
  session = await prepareManagedInstallationSession(options());
  const handoff = JSON.parse(await session!.createHandoff());
  const payload = JSON.parse(Buffer.from(handoff.bindings[0].proof.signature_payload, "base64url").toString("utf8"));
  expect(payload.expected_binding_epoch).toBe("7");
});

it("recovers an omitted epoch through the authenticated targeted lookup", async () => {
  bindingHints = []; bindingHintsTruncated = true;
  session = await prepareManagedInstallationSession(options());
  lookupBinding = { binding_id: "99999999-9999-4999-8999-999999999999", workspace_id: workspaceId, daemon_id: session!.managedDaemonId, binding_epoch: "107", auth_version: "1" };
  const handoff = JSON.parse(await session!.createHandoff());
  expect(JSON.parse(Buffer.from(handoff.bindings[0].proof.signature_payload, "base64url").toString("utf8")).expected_binding_epoch).toBe("107");
});

it("uses epoch null only when targeted lookup proves the workspace has no prior binding", async () => {
  bindingHints = []; bindingHintsTruncated = true; lookupBinding = null;
  session = await prepareManagedInstallationSession(options());
  const handoff = JSON.parse(await session!.createHandoff());
  expect(JSON.parse(Buffer.from(handoff.bindings[0].proof.signature_payload, "base64url").toString("utf8")).expected_binding_epoch).toBeNull();
});

it.each(["workspace", "daemon", "epoch"])("rejects a targeted lookup with mismatched %s", async (field) => {
  bindingHints = []; bindingHintsTruncated = true;
  session = await prepareManagedInstallationSession(options());
  lookupBinding = { binding_id: "99999999-9999-4999-8999-999999999999", workspace_id: field === "workspace" ? organizationId : workspaceId, daemon_id: field === "daemon" ? organizationId : session!.managedDaemonId, binding_epoch: field === "epoch" ? "-1" : "107", auth_version: "1" };
  await expect(session!.createHandoff()).rejects.toThrow(/another (workspace|scope)/);
  expect(fetchMock.mock.calls.some(([url, init]) => String(url).endsWith("/challenges") && String(init.body).includes('"purpose":"bind"'))).toBe(false);
});
