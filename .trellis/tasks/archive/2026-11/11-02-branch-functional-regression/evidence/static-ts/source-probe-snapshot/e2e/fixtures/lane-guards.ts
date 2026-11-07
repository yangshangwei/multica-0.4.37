import { randomUUID } from "node:crypto";
import pg from "pg";
import type { Page } from "@playwright/test";
import { TestApiClient } from "../fixtures";

// Shared lane guards and gate helpers for these specs. Some of them change the
// deployment-wide collaboration gate, so every database helper refuses
// non-loopback services and the default databases before it connects.
const LOOPBACK = new Set(["localhost", "127.0.0.1", "[::1]"]);
const DEFAULT_DATABASES = new Set(["", "multica", "postgres", "template0", "template1"]);

/** Why the isolated services are unusable here, or null when they are configured. */
export function isolatedServicesMissing(): string | null {
  const { DATABASE_URL: database, NEXT_PUBLIC_API_URL: api } = process.env;
  if (!database || !api) return "Requires an explicit isolated DATABASE_URL and NEXT_PUBLIC_API_URL";
  try {
    // Never echo either URL: the database one can carry credentials.
    if (!LOOPBACK.has(new URL(api).hostname) || !LOOPBACK.has(new URL(database).hostname)) return "Requires loopback test services";
    if (DEFAULT_DATABASES.has(new URL(database).pathname.slice(1))) return "Refuses a default database";
  } catch {
    return "DATABASE_URL and NEXT_PUBLIC_API_URL must be valid URLs";
  }
  return null;
}

/** Email-code login only exists in legacy mode. */
export function legacyLaneMissing(): string | null {
  if (process.env.MULTICA_AUTH_MODE === "password" || process.env.E2E_PASSWORD_AUTH === "1") {
    return "Requires the legacy email-code lane";
  }
  return isolatedServicesMissing();
}

export function adminLaneMissing(): string | null {
  if (process.env.E2E_PLATFORM_ADMIN !== "1" || process.env.E2E_PASSWORD_AUTH !== "1"
    || !process.env.E2E_PLATFORM_ADMIN_USERNAME || !process.env.E2E_PLATFORM_ADMIN_PASSWORD) {
    return "Requires the password/admin lane with an injected synthetic super administrator";
  }
  return isolatedServicesMissing();
}

export async function sql<T extends pg.QueryResultRow>(query: string, params: unknown[] = []): Promise<T[]> {
  const client = new pg.Client(process.env.DATABASE_URL);
  await client.connect();
  try {
    return (await client.query<T>(query, params)).rows;
  } finally {
    await client.end();
  }
}

export interface RawResponse { status: number; body: Record<string, unknown> | null; text: string }

/** An authenticated call whose refusal is the assertion, so it never throws on status. */
export async function rawRequest(actor: TestApiClient, path: string,
  init: { method?: string; body?: unknown; headers?: Record<string, string>; workspaceId?: string } = {}): Promise<RawResponse> {
  const token = actor.getToken();
  if (!token) throw new Error("Raw request actor is not authenticated");
  const headers: Record<string, string> = { "Content-Type": "application/json", Authorization: `Bearer ${token}`, ...init.headers };
  if (init.workspaceId) headers["X-Workspace-ID"] = init.workspaceId;
  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}${path}`, {
    method: init.method ?? "GET", headers, body: init.body === undefined ? undefined : JSON.stringify(init.body),
  });
  const text = await response.text();
  let body: Record<string, unknown> | null = null;
  try {
    const parsed: unknown = text ? JSON.parse(text) : null;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) body = parsed as Record<string, unknown>;
  } catch { /* non-JSON bodies are reported through text */ }
  return { status: response.status, body, text };
}

/** A legacy-lane owner with a fresh, onboarded workspace named after the run. */
export async function loginLegacyOwner(prefix: string, name: string) {
  const slug = `${prefix}-${randomUUID().slice(0, 8)}`;
  const api = new TestApiClient();
  const session = await api.login(`${slug}@example.test`, name);
  const workspace = await api.ensureWorkspace(name, slug);
  if (workspace.slug !== slug) throw new Error("The fixture must own its workspace");
  await api.markUserOnboarded();
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
  const token = api.getToken();
  const userId: unknown = session?.user?.id;
  if (!token || typeof userId !== "string") throw new Error("Incomplete legacy session");
  return { api, workspace, slug, token, userId };
}

/** Bearer-token Web session in a fixed locale; Web reads the cookie, Desktop the storage key. */
export async function openLegacySession(page: Page, baseURL: string, token: string, createMode: "agent" | "manual" | null = null) {
  await page.context().addCookies([
    { name: "multica-locale", value: "en", url: baseURL },
    { name: "multica_logged_in", value: "1", url: baseURL },
  ]);
  await page.addInitScript(({ authToken, mode }) => {
    localStorage.setItem("multica_token", authToken);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
    if (mode) localStorage.setItem("multica_create_mode", JSON.stringify({ state: { lastMode: mode }, version: 0 }));
  }, { authToken: token, mode: createMode });
}

export interface GateSwitches { enabled: boolean; creation_enabled: boolean; host_execution_enabled: boolean }

export async function readCollaborationGate(): Promise<GateSwitches & { epoch: string }> {
  const [gate] = await sql<GateSwitches & { epoch: string }>(
    "SELECT enabled, creation_enabled, host_execution_enabled, epoch::text AS epoch FROM collaboration_execution_gate WHERE id = 1");
  if (!gate) throw new Error("The isolated database has no collaboration execution gate");
  return gate;
}

/**
 * Changes the deployment-wide gate under its row lock and returns a restore
 * for the original booleans. Like the console, only an execution or host
 * admission change raises the epoch; creation is independent of it.
 */
export async function setCollaborationGate(next: (current: GateSwitches) => GateSwitches): Promise<() => Promise<void>> {
  const original = await writeGate(next);
  let restored = false;
  return async () => {
    if (restored) return;
    restored = true;
    await writeGate(() => original);
  };
}

async function writeGate(next: (current: GateSwitches) => GateSwitches): Promise<GateSwitches> {
  const client = new pg.Client(process.env.DATABASE_URL);
  await client.connect();
  try {
    await client.query("BEGIN");
    const [current] = (await client.query<GateSwitches>(
      "SELECT enabled, creation_enabled, host_execution_enabled FROM collaboration_execution_gate WHERE id = 1 FOR UPDATE")).rows;
    if (!current) throw new Error("The isolated database has no collaboration execution gate");
    const target = next(current);
    const executionChanged = target.enabled !== current.enabled || target.host_execution_enabled !== current.host_execution_enabled;
    if (executionChanged || target.creation_enabled !== current.creation_enabled) {
      await client.query(
        "UPDATE collaboration_execution_gate SET enabled = $1, creation_enabled = $2, host_execution_enabled = $3, epoch = epoch + $4 WHERE id = 1",
        [target.enabled, target.creation_enabled, target.host_execution_enabled, executionChanged ? 1 : 0]);
    }
    await client.query("COMMIT");
    return { enabled: current.enabled, creation_enabled: current.creation_enabled, host_execution_enabled: current.host_execution_enabled };
  } catch (error) {
    await client.query("ROLLBACK").catch(() => undefined);
    throw error;
  } finally {
    await client.end();
  }
}
