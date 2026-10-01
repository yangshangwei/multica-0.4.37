import { createHash, createPrivateKey, createPublicKey, randomBytes, randomUUID, sign } from "node:crypto";
import { constants, type Stats } from "node:fs";
import { lstat, mkdir, open, rename, unlink } from "node:fs/promises";
import { dirname, join } from "node:path";
import { withManagementFileLock } from "./managed-lock";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const DECIMAL = /^[1-9][0-9]*$/;
const PKCS8_PREFIX = Buffer.from("302e020100300506032b657004220420", "hex");
const FILE_KEYS = ["version", "deployment_id", "daemon_namespace_id", "installation_id", "key_version", "public_key", "private_key_seed"];
const PAYLOAD_KEYS = ["protocol_version", "purpose", "challenge_id", "deployment_id", "organization_id", "user_id", "auth_version", "installation_id", "workspace_id", "daemon_id", "public_key_fingerprint", "expected_binding_epoch", "nonce", "expires_at", "method", "path", "body_sha256"];

interface StoredIdentity {
  version: 1;
  deployment_id: string;
  daemon_namespace_id: string;
  installation_id: string | null;
  key_version: string | null;
  public_key: string;
  private_key_seed: string;
}

export interface ChallengeScope {
  purpose: "enroll" | "bind" | "renew";
  deploymentId: string;
  organizationId?: string;
  userId: string;
  authVersion: string;
  installationId: string | null;
  workspaceId: string | null;
  daemonId: string | null;
  expectedBindingEpoch: string | null;
  method: "POST";
  path: "/api/installations/enroll" | "/api/daemon/installation-bindings" | "/api/daemon/installation-bindings/renew";
  bodyPayload: string;
}

export interface InstallationChallenge {
  challenge_id: string;
  nonce: string;
  deployment_id: string;
  expires_at: string;
  signature_payload: string;
  body_payload: string;
}

export interface InstallationProof {
  challenge_id: string;
  signature_payload: string;
  body_payload: string;
  signature: string;
}

function decimal(value: unknown): value is string {
  return typeof value === "string" && DECIMAL.test(value) && BigInt(value) <= 9_223_372_036_854_775_807n;
}

function uuid(value: unknown): value is string {
  return typeof value === "string" && UUID.test(value);
}

function decode(value: unknown, limit: number): Buffer {
  if (typeof value !== "string" || value.length > limit * 2 || !/^[A-Za-z0-9_-]+$/.test(value)) throw new Error("Invalid management base64url value");
  const bytes = Buffer.from(value, "base64url");
  if (bytes.length > limit || bytes.toString("base64url") !== value) throw new Error("Noncanonical management base64url value");
  return bytes;
}

function canonicalObject(bytes: Buffer, keys: readonly string[]): Record<string, unknown> {
  const text = bytes.toString("utf8");
  let value: unknown;
  try { value = JSON.parse(text); } catch { throw new Error("Invalid management JSON envelope"); }
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).join(",") !== keys.join(",") || JSON.stringify(value) !== text) {
    throw new Error("Invalid management JSON envelope");
  }
  return value as Record<string, unknown>;
}

function privateKey(seed: string) {
  const bytes = decode(seed, 32);
  if (bytes.length !== 32) throw new Error("Invalid installation private key");
  return createPrivateKey({ key: Buffer.concat([PKCS8_PREFIX, bytes]), format: "der", type: "pkcs8" });
}

function publicKey(seed: string): string {
  return createPublicKey(privateKey(seed)).export({ format: "der", type: "spki" }).subarray(-32).toString("base64url");
}

function parseStored(bytes: Buffer, deploymentId: string): StoredIdentity {
  const value = canonicalObject(Buffer.from(bytes.toString("utf8").trim()), FILE_KEYS);
  if (value.version !== 1 || value.deployment_id !== deploymentId || !uuid(value.deployment_id) || !uuid(value.daemon_namespace_id) ||
    !(value.installation_id === null || uuid(value.installation_id)) ||
    !(value.key_version === null || decimal(value.key_version)) ||
    (value.installation_id === null) !== (value.key_version === null) ||
    typeof value.public_key !== "string" || typeof value.private_key_seed !== "string" ||
    decode(value.public_key, 32).length !== 32 || publicKey(value.private_key_seed) !== value.public_key) {
    throw new Error("Invalid persisted installation identity");
  }
  return value as unknown as StoredIdentity;
}

function checkOwnerAndMode(info: Stats, privateMode: boolean) {
  if (process.platform !== "win32" && privateMode && (info.mode & 0o077) !== 0) throw new Error("Management identity permissions must be private");
  if (process.getuid && info.uid !== process.getuid()) throw new Error("Management identity belongs to another OS user");
}

async function ensureDirectory(path: string, privateMode: boolean) {
  await mkdir(path, { mode: 0o700 }).catch((error: NodeJS.ErrnoException) => { if (error.code !== "EEXIST") throw error; });
  const info = await lstat(path);
  if (!info.isDirectory() || info.isSymbolicLink()) throw new Error("Management identity path must be a real directory");
  checkOwnerAndMode(info, privateMode);
}

export async function readManagementPrivateFile(path: string, maximumBytes: number): Promise<Buffer | null> {
  let info;
  try { info = await lstat(path); }
  catch (error) { if ((error as NodeJS.ErrnoException).code === "ENOENT") return null; throw error; }
  if (!info.isFile() || info.isSymbolicLink()) throw new Error("Management identity must be a regular file");
  const file = await open(path, constants.O_RDONLY | (constants.O_NOFOLLOW ?? 0));
  try {
    const opened = await file.stat();
    checkOwnerAndMode(opened, true);
    if (!opened.isFile() || opened.size > maximumBytes || opened.ino !== info.ino) throw new Error("Management identity file changed while reading");
    const bytes = await file.readFile();
    if (bytes.length > maximumBytes) throw new Error("Management file exceeds its size limit");
    return bytes;
  } finally { await file.close(); }
}

async function readStored(path: string, deploymentId: string): Promise<StoredIdentity | null> {
  const bytes = await readManagementPrivateFile(path, 4096);
  return bytes === null ? null : parseStored(bytes, deploymentId);
}

async function writeStored(path: string, value: StoredIdentity) {
  const temporary = `${path}.${randomBytes(12).toString("hex")}.tmp`;
  const file = await open(temporary, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL, 0o600);
  try {
    await file.writeFile(`${JSON.stringify(value)}\n`);
    await file.sync();
  } catch (error) {
    await file.close();
    await unlink(temporary);
    throw error;
  }
  await file.close();
  try { await rename(temporary, path); }
  catch (error) { await unlink(temporary); throw error; }
  if (process.platform !== "win32") {
    const directory = await open(dirname(path), constants.O_RDONLY);
    try { await directory.sync(); } finally { await directory.close(); }
  }
}

async function withIdentityLock<T>(directory: string, operation: () => Promise<T>): Promise<T> {
  return withManagementFileLock(directory, operation);
}

/** Main-process-only owner. Private fields are not serialized into IPC/log data. */
export class ManagedInstallation {
  #record: StoredIdentity;
  #directory: string;
  constructor(record: StoredIdentity, directory: string) { this.#record = record; this.#directory = directory; }

  get publicInfo() {
    return {
      deploymentId: this.#record.deployment_id,
      daemonNamespaceId: this.#record.daemon_namespace_id,
      installationId: this.#record.installation_id,
      keyVersion: this.#record.key_version,
      publicKey: this.#record.public_key,
      fingerprint: createHash("sha256").update(decode(this.#record.public_key, 32)).digest("hex"),
    };
  }

  managedDaemonId(userId: string): string {
    if (!uuid(userId)) throw new Error("Invalid managed daemon user");
    const bytes = createHash("sha256").update(`multica-managed-daemon-v1\0${this.#record.daemon_namespace_id}\0${userId}`).digest().subarray(0, 16);
    bytes[6] = (bytes[6]! & 0x0f) | 0x80;
    bytes[8] = (bytes[8]! & 0x3f) | 0x80;
    const hex = bytes.toString("hex");
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }

  async saveEnrollment(installationId: string, keyVersion: string): Promise<void> {
    if (!uuid(installationId) || !decimal(keyVersion)) throw new Error("Invalid enrollment result");
    await withIdentityLock(this.#directory, async () => {
      const path = join(this.#directory, "installation.json");
      const current = await readStored(path, this.#record.deployment_id);
      if (!current || current.public_key !== this.#record.public_key ||
        (current.installation_id !== null && current.installation_id !== installationId)) {
        throw new Error("Installation identity changed before enrollment was saved");
      }
      if (current.key_version !== null && BigInt(keyVersion) < BigInt(current.key_version)) throw new Error("Installation key version regressed");
      const next = { ...current, installation_id: installationId, key_version: keyVersion };
      await writeStored(path, next);
      this.#record = next;
    });
  }

  signHeartbeat(input: { userId: string; authVersion: string; bootId: string; sequence: string; reportedAt: string; desktopVersion: string; os: string }): { signature_payload: string; signature: string } {
    if (!this.#record.installation_id || !uuid(input.userId) || !uuid(input.bootId) || !decimal(input.authVersion) || !decimal(input.sequence) || !decimal(input.reportedAt) ||
      input.desktopVersion.length > 128 || !["macos", "windows", "linux", "unknown"].includes(input.os)) throw new Error("Invalid desktop heartbeat scope");
    const path = `/api/installations/${this.#record.installation_id}/heartbeat`;
    const bytes = Buffer.from(JSON.stringify({ protocol_version: "1", purpose: "heartbeat", deployment_id: this.#record.deployment_id, installation_id: this.#record.installation_id,
      user_id: input.userId, auth_version: input.authVersion, boot_id: input.bootId, sequence: input.sequence, reported_at: input.reportedAt,
      desktop_version: input.desktopVersion, os: input.os, method: "POST", path }));
    return { signature_payload: bytes.toString("base64url"), signature: sign(null, bytes, privateKey(this.#record.private_key_seed)).toString("base64url") };
  }

  signChallenge(response: InstallationChallenge, expected: ChallengeScope, nowSeconds = Math.floor(Date.now() / 1000)): InstallationProof {
    const original = decode(response.signature_payload, 32_768);
    const body = decode(response.body_payload, 65_536);
    const payload = canonicalObject(original, PAYLOAD_KEYS);
    const bodyObject = canonicalObject(body, expected.purpose === "enroll"
      ? ["public_key", "desktop_version", "os"]
      : ["installation_id", "workspace_id", "daemon_id", "public_key", "expected_binding_epoch"]);
    if (bodyObject.public_key !== this.#record.public_key) throw new Error("Challenge body uses another installation key");
    if (expected.purpose === "enroll") {
      if (typeof bodyObject.desktop_version !== "string" || bodyObject.desktop_version.length > 128 ||
        !["macos", "windows", "linux", "unknown"].includes(String(bodyObject.os))) throw new Error("Invalid enrollment body");
    } else if (bodyObject.installation_id !== expected.installationId || bodyObject.workspace_id !== expected.workspaceId ||
      bodyObject.daemon_id !== expected.daemonId || bodyObject.expected_binding_epoch !== expected.expectedBindingEpoch) {
      throw new Error("Challenge body disagrees with binding scope");
    }
    const same = (actual: unknown, expectedValue: unknown) => actual === expectedValue;
    const purposePath = expected.purpose === "enroll" ? "/api/installations/enroll" : expected.purpose === "renew" ? "/api/daemon/installation-bindings/renew" : "/api/daemon/installation-bindings";
    if (payload.protocol_version !== "1" || !uuid(payload.challenge_id) || !uuid(payload.organization_id) || !uuid(payload.user_id) ||
      !decimal(payload.auth_version) || !decimal(payload.expires_at) ||
      decode(payload.nonce, 32).length !== 32 ||
      !same(payload.purpose, expected.purpose) || !same(payload.deployment_id, this.#record.deployment_id) || !same(payload.deployment_id, expected.deploymentId) ||
      (expected.organizationId !== undefined && !same(payload.organization_id, expected.organizationId)) ||
      !same(payload.user_id, expected.userId) || !same(payload.auth_version, expected.authVersion) ||
      !same(payload.installation_id, expected.installationId) || !same(payload.workspace_id, expected.workspaceId) || !same(payload.daemon_id, expected.daemonId) ||
      !same(payload.expected_binding_epoch, expected.expectedBindingEpoch) || !same(payload.public_key_fingerprint, this.publicInfo.fingerprint) ||
      payload.method !== "POST" || expected.method !== "POST" || payload.path !== purposePath || expected.path !== purposePath ||
      payload.body_sha256 !== createHash("sha256").update(body).digest("hex") || response.body_payload !== expected.bodyPayload ||
      payload.challenge_id !== response.challenge_id || payload.nonce !== response.nonce || payload.deployment_id !== response.deployment_id || payload.expires_at !== response.expires_at ||
      BigInt(payload.expires_at) <= BigInt(nowSeconds) || BigInt(payload.expires_at) > BigInt(nowSeconds + 300)) {
      throw new Error("Installation challenge does not match the current scope");
    }
    if (expected.purpose === "enroll") {
      if (expected.installationId !== null || expected.workspaceId !== null || expected.daemonId !== null || expected.expectedBindingEpoch !== null) throw new Error("Invalid enrollment scope");
    } else if (!uuid(expected.installationId) || expected.installationId !== this.#record.installation_id || !uuid(expected.workspaceId) || !uuid(expected.daemonId) || expected.daemonId !== this.managedDaemonId(expected.userId) ||
      !(expected.expectedBindingEpoch === null || decimal(expected.expectedBindingEpoch)) || (expected.purpose === "renew" && expected.expectedBindingEpoch === null)) throw new Error("Invalid daemon binding scope");
    return {
      challenge_id: response.challenge_id, signature_payload: response.signature_payload,
      body_payload: response.body_payload, signature: sign(null, original, privateKey(this.#record.private_key_seed)).toString("base64url"),
    };
  }
}

export async function loadManagedInstallation(homeDirectory: string, deploymentId: string): Promise<ManagedInstallation> {
  if (!uuid(deploymentId)) throw new Error("Invalid management deployment ID");
  const root = join(homeDirectory, ".multica");
  await ensureDirectory(root, false);
  await ensureDirectory(join(root, "management"), true);
  const directory = join(root, "management", deploymentId);
  await ensureDirectory(directory, true);
  return withIdentityLock(directory, async () => {
    const path = join(directory, "installation.json");
    let record = await readStored(path, deploymentId);
    if (!record) {
      const seed = randomBytes(32).toString("base64url");
      record = { version: 1, deployment_id: deploymentId, daemon_namespace_id: randomUUID(), installation_id: null, key_version: null, public_key: publicKey(seed), private_key_seed: seed };
      await writeStored(path, record);
    }
    return new ManagedInstallation(record, directory);
  });
}
