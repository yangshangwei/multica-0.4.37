import { randomUUID } from "node:crypto";
import { expect, type Page, type TestInfo } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "../fixtures";

const p1PageErrors = new WeakMap<Page, string[]>();

export interface P1Project { id: string; workspace_id: string; title: string; description: string | null; revision: number; description_revision: number; status: string; issue_count: number; done_count: number }
export interface P1Issue { id: string; title: string; status: string; project_id: string | null; admission_status: string; revision: number }
export interface P1Overview { statistics: { counts: Record<string, number>; closure_ratio: number | null; snapshot_version: string; reference_date: string; timezone: string }; current_description_acceptance: { description_revision: number; conclusion: string } | null; latest_acceptance: { description_revision: number; conclusion: string } | null }

export async function p1DB<T extends pg.QueryResultRow = pg.QueryResultRow>(sql: string, values: unknown[] = []) {
  if (!process.env.DATABASE_URL || !process.env.NEXT_PUBLIC_API_URL) throw new Error("Source the task-owned browser env before P1 E2E");
  const url = new URL(process.env.NEXT_PUBLIC_API_URL);
  if (!["localhost", "127.0.0.1"].includes(url.hostname)) throw new Error("P1 fixtures require a local API");
  const db = new pg.Client(process.env.DATABASE_URL);
  await db.connect();
  try { return (await db.query<T>(sql, values)).rows; } finally { await db.end(); }
}

export async function p1Session(page?: Page, locale = "en") {
  const api = new TestApiClient();
  const slug = `p1-${randomUUID().slice(0, 12)}`;
  await api.login(`${slug}@example.invalid`, "P1 project owner");
  const workspace = await api.ensureWorkspace("P1 browser acceptance", slug);
  expect(workspace.slug).toBe(slug);
  await api.markUserOnboarded();
  await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
  const owner = await api.requestJSON<{ id: string }>("/api/me");
  if (page) await p1Authenticate(page, api, locale);
  return { api, workspace, owner };
}

export async function p1Authenticate(page: Page, api: TestApiClient, locale = "en") {
  page.setDefaultTimeout(15_000);
  const pageErrors: string[] = [];
  p1PageErrors.set(page, pageErrors);
  page.on("pageerror", (error) => pageErrors.push(error.stack ?? error.message));
  const token = api.getToken();
  if (!token) throw new Error("P1 fixture authentication missing");
  await page.addInitScript(({ token, locale }) => {
    localStorage.setItem("multica_token", token);
    localStorage.setItem("multica-locale", locale);
    localStorage.setItem("multica:chat:isOpen", "false");
    localStorage.setItem("theme", "light");
    document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
  }, { token, locale });
}

export async function p1Member(workspace: { id: string; slug: string }) {
  const api = new TestApiClient();
  await api.login(`p1-member-${randomUUID()}@example.invalid`, "P1 reviewer");
  const user = await api.requestJSON<{ id: string }>("/api/me");
  await p1DB("INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'member')", [workspace.id, user.id]);
  api.setWorkspaceId(workspace.id); api.setWorkspaceSlug(workspace.slug);
  await api.markUserOnboarded();
  return { api, user };
}

export async function p1Project(api: TestApiClient, body: Record<string, unknown> = {}) {
  return api.requestJSON<P1Project>("/api/projects", { method: "POST", body: { title: "Customer delivery P1", description: "## Goal\n\nKeep the original customer goal.", ...body } });
}

export async function p1EnableTriage(api: TestApiClient) {
  const settings = await api.requestJSON<{ revision: number }>("/api/triage/settings");
  await api.requestJSON("/api/triage/settings", { method: "PUT", body: { enabled: true, acceptance_status: "todo", require_priority: false, responsibility_mode: "none", responsibility_member_id: null, expected_revision: settings.revision } });
}

export async function p1Overview(api: TestApiClient, id: string) { return api.requestJSON<P1Overview>(`/api/projects/${id}/overview`); }
export async function p1Capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = info.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: true, animations: "disabled" });
  await info.attach(name, { path, contentType: "image/png" });
}
export async function p1NoOverflow(page: Page) { expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true); }
export async function p1Failure(page: Page, info: TestInfo) {
  await info.attach("page-errors", { body: JSON.stringify(p1PageErrors.get(page) ?? [], null, 2), contentType: "application/json" });
  await info.attach("failure-state", { body: `${page.url()}\n${await page.locator("body").innerText()}`, contentType: "text/plain" });
  await p1Capture(page, info, "failure");
}
export async function p1Raw(api: TestApiClient, workspaceId: string, path: string, method: string, body?: unknown) {
  return fetch(`${process.env.NEXT_PUBLIC_API_URL}${path}`, { method, headers: { Authorization: `Bearer ${api.getToken()}`, "X-Workspace-ID": workspaceId, "Content-Type": "application/json" }, body: body === undefined ? undefined : JSON.stringify(body) });
}
