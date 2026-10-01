import { createHash, randomUUID } from "node:crypto";
import { join } from "node:path";
import { ManagedControlConnection, ManagementControlError, UntrustedManagementPeerError } from "./managed-control";
import { managementAuthVersion, type InstallationMetadataProof } from "../shared/managed-installation";
import {
  loadManagedInstallation, readManagementPrivateFile, type InstallationChallenge,
  type ManagedInstallation, type InstallationProof,
} from "./managed-installation";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const MAX_HANDOFF_BYTES = 256 * 1024;

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("Invalid management response");
  return value as Record<string, unknown>;
}

function challenge(value: unknown): InstallationChallenge {
  const record = object(value);
  const keys = ["challenge_id", "nonce", "deployment_id", "expires_at", "signature_payload", "body_payload"] as const;
  for (const key of keys) if (typeof record[key] !== "string") throw new Error("Invalid installation challenge");
  return record as unknown as InstallationChallenge;
}

async function boundedJSON(response: Response): Promise<unknown> {
  const reader = response.body?.getReader();
  if (!reader) throw new Error("Management response has no body");
  const parts: Buffer[] = []; let length = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    length += value.byteLength;
    if (length > MAX_HANDOFF_BYTES) { await reader.cancel(); throw new Error("Management response exceeds its size limit"); }
    parts.push(Buffer.from(value));
  }
  try { return JSON.parse(Buffer.concat(parts).toString("utf8")); } catch { throw new Error("Invalid management response JSON"); }
}

async function requestJSON(baseUrl: string, path: string, token: string, signal: AbortSignal, body?: unknown, metadataProof?: string): Promise<unknown> {
  const response = await fetch(`${baseUrl}${path}`, {
    method: body === undefined ? "GET" : "POST", credentials: "omit", redirect: "error",
    signal: AbortSignal.any([signal, AbortSignal.timeout(15_000)]),
    headers: { "Content-Type": "application/json", "X-Client-Platform": "desktop", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(metadataProof ? { "X-Installation-Proof": metadataProof } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) throw Object.assign(new Error(`Management request failed (${response.status})`), { status: response.status });
  return boundedJSON(response);
}

export interface ManagedSessionOptions {
  homeDirectory: string;
  profile: string;
  apiBaseUrl: string;
  userToken: string;
  daemonToken: string;
  userId: string;
  desktopVersion: string;
  os: string;
  previousDeploymentId?: string;
  onMetadataProof?: (value: InstallationMetadataProof | null) => void;
  onHeartbeat?: () => void;
}

export interface ManagedDaemonScope {
  deployment_id: string; organization_id: string; user_id: string; auth_version: string;
  installation_id: string; managed_daemon_id: string; profile: string; active_task_count: number; workspace_ids: string[]; drain_intent_id?: string | null;
}
export interface OwnedManagedDaemon { scope: ManagedDaemonScope; connection: ManagedControlConnection }

interface BindingEpochHint { epoch: string; authVersion: string }
function parseBindingHint(value: unknown, daemonId: string): { workspaceId: string; hint: BindingEpochHint } {
  const binding = object(value);
  if (typeof binding.binding_id !== "string" || !UUID.test(binding.binding_id) || typeof binding.workspace_id !== "string" || !UUID.test(binding.workspace_id) ||
    binding.daemon_id !== daemonId || typeof binding.binding_epoch !== "string" || !/^[1-9][0-9]*$/.test(binding.binding_epoch) || BigInt(binding.binding_epoch) > 9_223_372_036_854_775_807n ||
    typeof binding.auth_version !== "string" || !/^[1-9][0-9]*$/.test(binding.auth_version) || BigInt(binding.auth_version) > 9_223_372_036_854_775_807n) throw new Error("Enrollment binding hint belongs to another scope");
  return { workspaceId: binding.workspace_id, hint: { epoch: binding.binding_epoch, authVersion: binding.auth_version } };
}

interface StoredBindingMetadata { epoch: string; reusable: boolean; tokenFingerprint: string }

interface BindingHandoff { workspace_id: string; challenge: InstallationChallenge; proof: InstallationProof }
interface StartupHandoff {
  version: "1"; server_url: string; deployment_id: string; organization_id: string; user_id: string; auth_version: string;
  installation_id: string; managed_daemon_id: string; bindings: BindingHandoff[];
}

export class ManagedInstallationSession {
  #options: ManagedSessionOptions;
  #identity: ManagedInstallation;
  #organizationId: string;
  #authVersion: string;
  #controller: AbortController;
  #timer: ReturnType<typeof setTimeout> | null = null;
  #bootId = randomUUID();
  #sequence = 0n;
  #heartbeatStarted = false;
  #metadataProof: string | null;
  #bindingEpochHints: Map<string, BindingEpochHint> | null;
  #bindingHintsTruncated: boolean;
  #pendingHandoff: { value: string; expiresAt: number; fingerprints: Map<string, string | null> } | null = null;

  constructor(options: ManagedSessionOptions, identity: ManagedInstallation, organizationId: string, authVersion: string, controller: AbortController, metadataProof: string | null = null, bindingEpochHints: Map<string, BindingEpochHint> | null = null, bindingHintsTruncated = false) {
    this.#options = options; this.#identity = identity; this.#organizationId = organizationId; this.#authVersion = authVersion; this.#controller = controller; this.#metadataProof = metadataProof; this.#bindingEpochHints = bindingEpochHints; this.#bindingHintsTruncated = bindingHintsTruncated;
  }

  get metadataProof(): InstallationMetadataProof | null {
    if (!this.#metadataProof || this.#controller.signal.aborted) return null;
    return { serverUrl: this.#options.apiBaseUrl, userId: this.userId, authVersion: this.authVersion, proof: this.#metadataProof };
  }
  get profile() { return this.#options.profile; }
  get serverUrl() { return this.#options.apiBaseUrl; }
  get deploymentId() { return this.#identity.publicInfo.deploymentId; }
  get userId() { return this.#options.userId; }
  get authVersion() { return this.#authVersion; }
  get installationId() { return this.#identity.publicInfo.installationId!; }
  get managedDaemonId() { return this.#identity.managedDaemonId(this.userId); }

  dispose() {
    this.#controller.abort();
    if (this.#timer !== null) clearTimeout(this.#timer);
    this.#timer = null;
    this.#metadataProof = null;
    this.#pendingHandoff = null;
    this.#options.onMetadataProof?.(null);
    this.#options.userToken = "";
    this.#options.daemonToken = "";
  }

  async #storedBindings(): Promise<Map<string, StoredBindingMetadata>> {
    const profileHash = createHash("sha256").update(this.#options.profile).digest("hex");
    const path = join(this.#options.homeDirectory, ".multica/management", this.deploymentId, "credentials", this.userId, `${profileHash}.json`);
    const raw = await readManagementPrivateFile(path, MAX_HANDOFF_BYTES);
    if (raw === null) return new Map();
    let stored: Record<string, unknown>;
    try { stored = object(JSON.parse(raw.toString("utf8"))); } catch { throw new Error("Invalid managed credential metadata"); }
    if (stored.version !== "1" || stored.server_url !== this.#options.apiBaseUrl || stored.deployment_id !== this.deploymentId || stored.user_id !== this.userId ||
      stored.profile !== this.#options.profile || stored.installation_id !== this.installationId || stored.managed_daemon_id !== this.managedDaemonId || !Array.isArray(stored.bindings)) throw new Error("Managed credential metadata belongs to another scope");
    const result = new Map<string, StoredBindingMetadata>();
    for (const value of stored.bindings) {
      const binding = object(value); const credential = object(binding.credential);
      if (typeof binding.workspace_id !== "string" || !UUID.test(binding.workspace_id) || typeof credential.binding_epoch !== "string" || !/^[1-9][0-9]*$/.test(credential.binding_epoch) || result.has(binding.workspace_id)) throw new Error("Invalid managed workspace binding metadata");
      if (typeof credential.daemon_token !== "string" || !credential.daemon_token.startsWith("mdt_")) throw new Error("Invalid protected managed credential");
      result.set(binding.workspace_id, { epoch: credential.binding_epoch, tokenFingerprint: createHash("sha256").update(credential.daemon_token).digest("hex"), reusable: stored.auth_version === this.authVersion && credential.auth_version === this.authVersion && credential.principal_user_id === this.userId && typeof credential.expires_at === "string" && Date.parse(credential.expires_at) > Date.now() + 60_000 });
    }
    return result;
  }

  async acknowledgeHandoff(value: string): Promise<void> {
    const pending = this.#pendingHandoff;
    if (!pending || pending.value !== value) return;
    const stored = await this.#storedBindings();
    let complete = true;
    for (const [workspace, priorFingerprint] of pending.fingerprints) {
      const current = stored.get(workspace);
      if (!current?.reusable || current.tokenFingerprint === priorFingerprint) { complete = false; continue; }
      this.#bindingEpochHints?.set(workspace, { epoch: current.epoch, authVersion: this.authVersion });
    }
    if (complete) this.#pendingHandoff = null;
  }

  async createHandoff(knownWorkspaceIds?: ReadonlySet<string>): Promise<string> {
    if (this.#pendingHandoff) {
      await this.acknowledgeHandoff(this.#pendingHandoff.value);
      if (this.#pendingHandoff && this.#pendingHandoff.expiresAt > Date.now() + 1_000) return this.#pendingHandoff.value;
      this.#pendingHandoff = null;
    }
    const options = this.#options;
    const workspaces = await requestJSON(options.apiBaseUrl, "/api/workspaces", options.userToken, this.#controller.signal);
    if (!Array.isArray(workspaces) || workspaces.length > 100) throw new Error("Invalid managed workspace list");
    const stored = await this.#storedBindings();
    const bindings: BindingHandoff[] = [];
    const fingerprints = new Map<string, string | null>();
    const seen = new Set<string>();
    for (const value of workspaces) {
      const workspace = object(value);
      if (typeof workspace.id !== "string" || !UUID.test(workspace.id) || seen.has(workspace.id)) throw new Error("Invalid managed workspace identity");
      seen.add(workspace.id);
      const local = stored.get(workspace.id);
      let hint = this.#bindingEpochHints?.get(workspace.id);
      if (!hint && this.#bindingHintsTruncated) {
        if (!this.#metadataProof) throw new Error("Incomplete enrollment binding hints require an installation metadata proof");
        const query = new URLSearchParams({ workspace_id: workspace.id, daemon_id: this.managedDaemonId });
        const result = object(await requestJSON(options.apiBaseUrl, `/api/installations/${this.installationId}/binding?${query}`, options.userToken, this.#controller.signal, undefined, this.#metadataProof));
        if (!("binding" in result)) throw new Error("Invalid targeted installation binding response");
        if (result.binding !== null) {
          const resolved = parseBindingHint(result.binding, this.managedDaemonId);
          if (resolved.workspaceId !== workspace.id) throw new Error("Targeted installation binding belongs to another workspace");
          hint = resolved.hint;
          this.#bindingEpochHints?.set(workspace.id, hint);
        }
      }
      const matchesHint = this.#bindingEpochHints === null || (hint?.epoch === local?.epoch && hint?.authVersion === this.authVersion);
      if (local?.reusable && matchesHint && (!knownWorkspaceIds || knownWorkspaceIds.has(workspace.id))) continue;
      const epoch = this.#bindingEpochHints === null ? local?.epoch ?? null : hint?.epoch ?? null;
      const body = { installation_id: this.installationId, workspace_id: workspace.id, daemon_id: this.managedDaemonId, public_key: this.#identity.publicInfo.publicKey, expected_binding_epoch: epoch };
      const response = challenge(await requestJSON(options.apiBaseUrl, "/api/installations/challenges", options.userToken, this.#controller.signal, { purpose: "bind", ...body }));
      this.#controller.signal.throwIfAborted();
      const proof = this.#identity.signChallenge(response, { purpose: "bind", deploymentId: this.deploymentId, organizationId: this.#organizationId, userId: this.userId, authVersion: this.authVersion,
        installationId: this.installationId, workspaceId: workspace.id, daemonId: this.managedDaemonId, expectedBindingEpoch: epoch, method: "POST", path: "/api/daemon/installation-bindings", bodyPayload: Buffer.from(JSON.stringify(body)).toString("base64url") });
      bindings.push({ workspace_id: workspace.id, challenge: response, proof });
      fingerprints.set(workspace.id, local?.tokenFingerprint ?? null);
    }
    const handoff: StartupHandoff = { version: "1", server_url: options.apiBaseUrl, deployment_id: this.deploymentId, organization_id: this.#organizationId, user_id: this.userId,
      auth_version: this.authVersion, installation_id: this.installationId, managed_daemon_id: this.managedDaemonId, bindings };
    const serialized = JSON.stringify(handoff);
    if (Buffer.byteLength(serialized) > MAX_HANDOFF_BYTES) throw new Error("Managed handoff exceeds its size limit");
    if (bindings.length > 0) this.#pendingHandoff = { value: serialized, fingerprints, expiresAt: Math.min(...bindings.map((binding) => Number(binding.challenge.expires_at) * 1000)) };
    return serialized;
  }

  async connectRunningDaemon(port: number): Promise<OwnedManagedDaemon | null> {
    const path = join(this.#options.homeDirectory, ".multica/profiles", this.#options.profile, "management-control.json");
    const raw = await readManagementPrivateFile(path, 4096);
    if (raw === null) return null;
    let control: Record<string, unknown>;
    try { control = object(JSON.parse(raw.toString("utf8"))); } catch { throw new Error("Invalid local management credential"); }
    if (control.version !== "1" || typeof control.token !== "string" || !/^[A-Za-z0-9_-]{43}$/.test(control.token) || !Number.isInteger(control.pid)) throw new Error("Invalid local management credential");
    const connection = new ManagedControlConnection(port, control.token, this.#controller.signal);
    let scope: Record<string, unknown>;
    try { scope = object(await connection.request("GET", "/management/session")); }
    catch (error) { if (error instanceof UntrustedManagementPeerError) return null; throw error; }
    if (scope.capability_version !== "1" || scope.ready !== true || scope.deployment_id !== this.deploymentId || scope.organization_id !== this.#organizationId ||
      typeof scope.user_id !== "string" || !UUID.test(scope.user_id) || typeof scope.auth_version !== "string" || !/^[1-9][0-9]*$/.test(scope.auth_version) ||
      scope.installation_id !== this.installationId || scope.managed_daemon_id !== this.#identity.managedDaemonId(scope.user_id) || scope.profile !== this.#options.profile ||
      typeof scope.active_task_count !== "number" || !Number.isSafeInteger(scope.active_task_count) || scope.active_task_count < 0 ||
      (scope.drain_intent_id !== undefined && scope.drain_intent_id !== null && (typeof scope.drain_intent_id !== "string" || !UUID.test(scope.drain_intent_id))) ||
      !Array.isArray(scope.workspace_ids) || scope.workspace_ids.some((id) => typeof id !== "string" || !UUID.test(id))) throw new Error("Running daemon does not match the private installation scope");
    return { connection, scope: scope as unknown as ManagedDaemonScope };
  }

  matchesDaemonScope(scope: ManagedDaemonScope): boolean {
    return scope.user_id === this.userId && scope.auth_version === this.authVersion && scope.managed_daemon_id === this.managedDaemonId;
  }

  async handoffToRunningDaemon(port: number): Promise<boolean> {
    const owned = await this.connectRunningDaemon(port);
    if (!owned || !this.matchesDaemonScope(owned.scope)) return false;
    const handoff = await this.createHandoff(new Set(owned.scope.workspace_ids));
    if ((JSON.parse(handoff) as StartupHandoff).bindings.length === 0) return true;
    if (owned.scope.active_task_count !== 0) return false;
    try { return object(await owned.connection.request("POST", "/management/handoff", handoff)).accepted === true; }
    catch (error) { if (error instanceof ManagementControlError && error.status === 409) return false; throw error; }
  }

  async reportHeartbeat(): Promise<void> {
    this.#controller.signal.throwIfAborted();
    this.#sequence += 1n;
    const proof = this.#identity.signHeartbeat({ userId: this.userId, authVersion: this.authVersion, bootId: this.#bootId, sequence: String(this.#sequence), reportedAt: String(Math.floor(Date.now() / 1000)), desktopVersion: this.#options.desktopVersion, os: this.#options.os });
    const result = object(await requestJSON(this.#options.apiBaseUrl, `/api/installations/${this.installationId}/heartbeat`, this.#options.userToken, this.#controller.signal, proof));
    this.#controller.signal.throwIfAborted();
    if (typeof result.accepted !== "boolean" || typeof result.server_time !== "string" || !Number.isFinite(Date.parse(result.server_time)) || typeof result.next_report_after !== "number" || !Number.isFinite(result.next_report_after) || result.next_report_after <= 0) throw new Error("Invalid desktop heartbeat response");
    if (result.accepted === true && typeof result.metadata_proof === "string" && result.metadata_proof.startsWith("mip_") && result.metadata_proof.length <= 16_384) {
      this.#metadataProof = result.metadata_proof; this.#options.onMetadataProof?.(this.metadataProof);
    }
    this.#options.onHeartbeat?.();
  }

  startHeartbeat(onUnavailable: () => void = () => undefined): void {
    if (this.#heartbeatStarted || this.#controller.signal.aborted) return;
    this.#heartbeatStarted = true;
    const tick = async () => {
      if (this.#controller.signal.aborted) return;
      try { await this.reportHeartbeat(); } catch { if (!this.#controller.signal.aborted) onUnavailable(); }
      if (!this.#controller.signal.aborted) this.#timer = setTimeout(tick, 60_000 * (0.8 + Math.random() * 0.4));
    };
    void tick();
  }
}

export async function prepareManagedInstallationSession(options: ManagedSessionOptions): Promise<ManagedInstallationSession | null> {
  const base = new URL(options.apiBaseUrl);
  if (!["http:", "https:"].includes(base.protocol) || base.username || base.password || base.search || base.hash || !options.profile || /[/\\]/.test(options.profile)) throw new Error("Invalid managed Desktop configuration");
  const normalized = { ...options, apiBaseUrl: base.toString().replace(/\/+$/, "") };
  const controller = new AbortController();
  try {
    const config = object(await requestJSON(normalized.apiBaseUrl, "/api/config", "", controller.signal));
    if (config.auth_mode !== "password" || typeof config.deployment_id !== "string" || !UUID.test(config.deployment_id) || config.managed_installation_supported !== true) {
      if (options.previousDeploymentId) throw new Error("Managed installation is unavailable; profile cannot downgrade to legacy");
      return null;
    }
    const authVersion = managementAuthVersion(options.userToken, options.userId);
    if (!authVersion) {
      if (options.previousDeploymentId) throw new Error("Managed installation requires a complete password JWT");
      return null;
    }
    const [human, daemon] = await Promise.all([
      requestJSON(normalized.apiBaseUrl, "/api/me", options.userToken, controller.signal),
      requestJSON(normalized.apiBaseUrl, "/api/me", options.daemonToken, controller.signal),
    ]);
    if (object(human).id !== options.userId || object(daemon).id !== options.userId || object(human).requires_account_setup === true || object(human).requires_password_change === true) throw new Error("Desktop and daemon require the same complete password identity");
    const identity = await loadManagedInstallation(options.homeDirectory, config.deployment_id);
    const body = { public_key: identity.publicInfo.publicKey, desktop_version: options.desktopVersion, os: options.os };
    const response = challenge(await requestJSON(normalized.apiBaseUrl, "/api/installations/challenges", options.userToken, controller.signal, { purpose: "enroll", ...body, expected_binding_epoch: null }));
    const proof = identity.signChallenge(response, { purpose: "enroll", deploymentId: config.deployment_id, userId: options.userId, authVersion, installationId: null, workspaceId: null, daemonId: null, expectedBindingEpoch: null, method: "POST", path: "/api/installations/enroll", bodyPayload: Buffer.from(JSON.stringify(body)).toString("base64url") });
    const payload = object(JSON.parse(Buffer.from(response.signature_payload, "base64url").toString("utf8")));
    const enrolled = object(await requestJSON(normalized.apiBaseUrl, "/api/installations/enroll", options.userToken, controller.signal, proof));
    if (typeof enrolled.installation_id !== "string" || typeof enrolled.key_version !== "string") throw new Error("Invalid managed enrollment response");
    await identity.saveEnrollment(enrolled.installation_id, enrolled.key_version);
    if (enrolled.bindings_truncated !== undefined && typeof enrolled.bindings_truncated !== "boolean") throw new Error("Invalid enrollment binding truncation flag");
    if (enrolled.bindings_truncated === true && !Array.isArray(enrolled.bindings)) throw new Error("Truncated enrollment omitted its binding list");
    let hints: Map<string, BindingEpochHint> | null = null;
    if (enrolled.bindings !== undefined) {
      if (!Array.isArray(enrolled.bindings) || enrolled.bindings.length > 100) throw new Error("Invalid enrollment binding hints");
      hints = new Map();
      for (const value of enrolled.bindings) {
        const binding = parseBindingHint(value, identity.managedDaemonId(options.userId));
        if (hints.has(binding.workspaceId)) throw new Error("Duplicate enrollment binding hint");
        hints.set(binding.workspaceId, binding.hint);
      }
    }
    const metadataProof = typeof enrolled.metadata_proof === "string" && enrolled.metadata_proof.startsWith("mip_") && enrolled.metadata_proof.length <= 16_384 ? enrolled.metadata_proof : null;
    return new ManagedInstallationSession(normalized, identity, String(payload.organization_id), authVersion, controller, metadataProof, hints, enrolled.bindings_truncated === true);
  } catch (error) { controller.abort(); throw error; }
}
