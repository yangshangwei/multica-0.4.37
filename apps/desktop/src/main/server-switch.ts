import { randomUUID } from "crypto";

/** A frozen session stays blocked until the replacement renderer boots. */
export class ServerSwitchCoordinator {
  private running = false;
  blocked = false;
  rendererReady(): void { if (!this.running) this.blocked = false; }
  async run(actions: { probe: () => Promise<void>; freeze: () => void; cleanup: () => Promise<void>; save: () => Promise<void>; reload: () => void }): Promise<void> {
    if (this.running) throw new Error("Server switch already in progress");
    this.running = true;
    try {
      await actions.probe();
      this.blocked = true;
      actions.freeze();
      await actions.cleanup();
      await actions.save();
      actions.reload();
    } finally { this.running = false; }
  }
}

export async function probeServer(apiUrl: string, request: typeof fetch = fetch): Promise<void> {
  const signal = AbortSignal.timeout(5000);
  const options: RequestInit = { signal, credentials: "omit", redirect: "error", headers: { accept: "application/json" } };
  const health = await request(`${apiUrl}/health`, options);
  if (!health.ok) throw new Error(`Server responded with HTTP ${health.status}`);
  const response = await request(`${apiUrl}/api/config`, options);
  if (!response.ok) throw new Error(`Server capabilities returned HTTP ${response.status}`);
  const config: unknown = await response.json();
  if (!config || typeof config !== "object" || Array.isArray(config)) throw new Error("Invalid server capabilities");
  const fields = config as Record<string, unknown>;
  // Missing auth_mode is the explicit installed legacy-server protocol.
  if (fields.auth_mode !== undefined && fields.auth_mode !== "legacy" && fields.auth_mode !== "password") throw new Error("Unsupported server authentication mode");
  if (fields.auth_mode === "password" && fields.device_auth_available === true) throw new Error("Conflicting server authentication capabilities");
}

interface Cookie { name: string; domain?: string; path?: string; secure?: boolean }
export async function clearServerCookies(cookies: { get: (filter: object) => Promise<Cookie[]>; remove: (url: string, name: string) => Promise<void> }, urls: string[]): Promise<void> {
  const hosts = urls.map(url => new URL(url).hostname);
  // Enumerate instead of URL-filtering: URL filters omit alternate Path cookies.
  for (const cookie of await cookies.get({})) {
    if (cookie.name !== "multica_auth" && cookie.name !== "multica_csrf") continue;
    const domain = cookie.domain?.replace(/^\./, "");
    if (!domain || !hosts.some(host => host === domain || host.endsWith(`.${domain}`))) continue;
    await cookies.remove(`${cookie.secure ? "https" : "http"}://${domain}${cookie.path || "/"}`, cookie.name);
  }
}

/** Bind browser callbacks to a single main-process login attempt. */
export class BrowserLoginAttempt {
  private nonce: string | null = null;
  begin(): string { this.nonce = randomUUID(); return this.nonce; }
  clear(): void { this.nonce = null; }
  consume(value: string | null): boolean {
    if (!value || !this.nonce || value !== this.nonce) return false;
    this.clear();
    return true;
  }
}
