import { createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";

export type ManagementControlPath = "/management/session" | "/management/handoff" | "/shutdown";
export const MANAGEMENT_NONCE = "X-Multica-Management-Nonce";
export const MANAGEMENT_TIME = "X-Multica-Management-Time";
export const MANAGEMENT_AUTH = "X-Multica-Management-Auth";
export const MANAGEMENT_RESPONSE = "X-Multica-Management-Response";

function mac(secret: string, domain: string, method: string, path: string, headers: Headers, status: number | null, body: Buffer): string {
  const key = Buffer.from(secret, "base64url");
  if (key.length !== 32 || key.toString("base64url") !== secret) throw new Error("Invalid local management key");
  const fields = [domain, method, path, headers.get(MANAGEMENT_NONCE), headers.get(MANAGEMENT_TIME)];
  if (status !== null) fields.push(String(status));
  fields.push(createHash("sha256").update(body).digest("hex"));
  return createHmac("sha256", key).update(fields.join("\n")).digest("hex");
}

export function managementRequestHeaders(secret: string, method: string, path: ManagementControlPath, body: string): Headers {
  const headers = new Headers({ "Content-Type": "application/json", [MANAGEMENT_NONCE]: randomBytes(32).toString("base64url"), [MANAGEMENT_TIME]: String(Math.floor(Date.now() / 1000)) });
  headers.set(MANAGEMENT_AUTH, mac(secret, "multica-management-request-v1", method, path, headers, null, Buffer.from(body)));
  return headers;
}

export function managementResponseMAC(secret: string, method: string, path: ManagementControlPath, requestHeaders: Headers, status: number, body: Buffer): string {
  return mac(secret, "multica-management-response-v1", method, path, requestHeaders, status, body);
}

export class ManagementControlError extends Error {
  constructor(readonly status: number) { super(status === 409 ? "Managed daemon is busy or its scope changed" : `Authenticated management request failed (${status})`); }
}

export class UntrustedManagementPeerError extends Error {
  constructor() { super("Local daemon did not prove management ownership"); }
}

/** The per-boot key never traverses HTTP; both directions authenticate exact bytes. */
export class ManagedControlConnection {
  #secret: string;
  #port: number;
  #signal: AbortSignal;
  constructor(port: number, secret: string, signal: AbortSignal) {
    if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error("Invalid local management port");
    this.#port = port; this.#secret = secret; this.#signal = signal;
  }

  async request(method: "GET" | "POST", path: ManagementControlPath, body = ""): Promise<unknown> {
    const headers = managementRequestHeaders(this.#secret, method, path, body);
    const response = await fetch(`http://127.0.0.1:${this.#port}${path}`, { method, headers, body: method === "GET" ? undefined : body, credentials: "omit", redirect: "error", signal: AbortSignal.any([this.#signal, AbortSignal.timeout(method === "GET" ? 2_000 : 30_000)]) });
    const reader = response.body?.getReader(); if (!reader) throw new UntrustedManagementPeerError();
    const parts: Buffer[] = []; let length = 0;
    for (;;) {
      const { done, value } = await reader.read(); if (done) break;
      length += value.byteLength; if (length > 256 * 1024) { await reader.cancel(); throw new UntrustedManagementPeerError(); }
      parts.push(Buffer.from(value));
    }
    const raw = Buffer.concat(parts);
    const expected = Buffer.from(managementResponseMAC(this.#secret, method, path, headers, response.status, raw), "ascii");
    const actual = Buffer.from(response.headers.get(MANAGEMENT_RESPONSE) ?? "", "ascii");
    if (actual.length !== expected.length || !timingSafeEqual(actual, expected)) throw new UntrustedManagementPeerError();
    this.#signal.throwIfAborted();
    if (!response.ok) throw new ManagementControlError(response.status);
    try { return JSON.parse(raw.toString("utf8")); } catch { throw new Error("Invalid authenticated management response"); }
  }
}
