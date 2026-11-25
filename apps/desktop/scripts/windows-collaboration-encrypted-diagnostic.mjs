import { constants, createCipheriv, createHash, createPublicKey, publicEncrypt, randomBytes } from "node:crypto";
import { lstat, open, realpath } from "node:fs/promises";
import path from "node:path";
import { collaborationResults, parseCollaborationReport } from "./windows-collaboration-diagnostics.mjs";

const logLimit = 64 * 1024;
const errorLimit = 16 * 1024;
const phases = new Set(["install_candidate", "upgrade_candidate", "reinstall_candidate"]);
function bounded(value, limit) {
  const bytes = Buffer.from(typeof value === "string" ? value : "", "utf8");
  let start = Math.max(0, bytes.length - limit);
  while (start < bytes.length && (bytes[start] & 0xc0) === 0x80) start++;
  return bytes.subarray(start).toString("utf8");
}
const samePath = (left, right) => process.platform === "win32" ? left.toLowerCase() === right.toLowerCase() : left === right;

async function ownedLogTail({ phaseRoot, home, apiUrl }) {
  const root = path.resolve(phaseRoot);
  const ownedHome = path.resolve(home);
  const relative = path.relative(root, ownedHome);
  if (!relative || relative.startsWith("..") || path.isAbsolute(relative) || !samePath(await realpath(root), root)
    || !samePath(await realpath(ownedHome), ownedHome)) throw new Error("unowned_capture_path");
  const api = new URL(apiUrl);
  if (api.protocol !== "http:" || api.hostname !== "127.0.0.1" || !api.port || api.username || api.password) throw new Error("unowned_capture_endpoint");
  const file = path.join(ownedHome, ".multica", "profiles", `desktop-${api.host.replace(/:/g, "-").toLowerCase()}`, "daemon.err.log");
  const before = await lstat(file);
  if (!before.isFile() || before.isSymbolicLink() || !samePath(await realpath(file), file)) throw new Error("unowned_capture_file");
  const handle = await open(file, "r");
  try {
    const current = await handle.stat();
    if (!current.isFile() || current.ino !== before.ino || current.dev !== before.dev) throw new Error("changed_capture_file");
    const data = Buffer.alloc(Math.min(current.size, logLimit));
    const { bytesRead } = await handle.read(data, 0, data.length, Math.max(0, current.size - logLimit));
    return { status: "captured", truncated: current.size > logLimit, encoding: "base64", data: data.subarray(0, bytesRead).toString("base64") };
  } finally { await handle.close(); }
}

/** Failure-only envelope. Raw material stays inside authenticated ciphertext. */
export async function encryptedCollaborationDiagnostic(options) {
  if (!options.publicKey) return undefined;
  let publicKey;
  try {
    const encoded = options.publicKey.trim();
    if (encoded.length > 16 * 1024 || !/^[A-Za-z0-9+/]+={0,2}$/.test(encoded)) throw new Error();
    const der = Buffer.from(encoded, "base64");
    if (der.toString("base64") !== encoded) throw new Error();
    publicKey = createPublicKey({ key: der, format: "der", type: "spki" });
    if (publicKey.asymmetricKeyType !== "rsa" || publicKey.asymmetricKeyDetails.modulusLength < 3072) throw new Error();
  } catch { return { status: "invalid_key" }; }
  if (!/^[a-f0-9]{40}$/.test(options.sourceCommit) || !phases.has(options.phase)) return { status: "invalid_context" };
  let key;
  try {
    let daemonErrorLog;
    try { daemonErrorLog = await ownedLogTail(options); }
    catch (error) { daemonErrorLog = { status: error.code === "ENOENT" ? "unavailable" : "rejected" }; }
    const errors = collaborationResults(parseCollaborationReport(options.stdout)).flatMap(result => {
      const error = result.error;
      return [error?.message, error?.stack].filter(value => typeof value === "string");
    });
    const plaintext = Buffer.from(JSON.stringify({ schema_version: 1, daemon_error_log: daemonErrorLog,
      controller_failure: bounded(errors.join("\n"), errorLimit), subprocess_stderr: bounded(options.stderr, errorLimit) }));
    const aad = { schema_version: 1, source_commit: options.sourceCommit, phase: options.phase };
    key = randomBytes(32);
    const iv = randomBytes(12);
    const cipher = createCipheriv("aes-256-gcm", key, iv);
    cipher.setAAD(Buffer.from(JSON.stringify(aad)));
    const ciphertext = Buffer.concat([cipher.update(plaintext), cipher.final()]);
    plaintext.fill(0);
    const wrappedKey = publicEncrypt({ key: publicKey, oaepHash: "sha256", padding: constants.RSA_PKCS1_OAEP_PADDING }, key);
    return { status: "sealed", algorithm: "RSA-OAEP-SHA256+AES-256-GCM", aad,
      recipient_sha256: createHash("sha256").update(publicKey.export({ format: "der", type: "spki" })).digest("hex"),
      wrapped_key: wrappedKey.toString("base64"), iv: iv.toString("base64"), tag: cipher.getAuthTag().toString("base64"), ciphertext: ciphertext.toString("base64") };
  } catch { return { status: "encryption_failed" }; }
  finally { key?.fill(0); }
}
