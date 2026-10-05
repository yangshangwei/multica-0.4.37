import { randomUUID } from "node:crypto";
import { expect, test as base, type Browser, type Locator, type Page, type TestInfo } from "@playwright/test";
import type { TestApiClient } from "../fixtures";
import { p1Authenticate, p1DB, p1Failure, p1Project, p1Session, type P1Project } from "./project-p1";

export interface ProgressEvidenceInput { kind: "issue" | "execution" | "url"; id: string | null; url: string | null }
export interface ProgressAcceptanceInput { conclusion: "passed" | "partial" | "failed"; scope: string; explanation: string | null }
export interface ProgressDraft {
  operation: "create" | "correct"; update_id: string | null; expected_revision: number | null;
  kind: "progress" | "risk" | "acceptance"; body: string; health_judgment: "on_track" | "attention" | "risk" | null;
  evidence: ProgressEvidenceInput[]; acceptance: ProgressAcceptanceInput | null;
  expected_description_revision: number | null; include_statistics: boolean; correction_reason: string | null;
}
export interface ProgressPreview {
  workspace_id: string; project_id: string; draft: ProgressDraft; preview_hash: string; evidence_versions: unknown[];
  recipients: { id: string; name: string | null }[];
  statistics_snapshot: { calculated_at: string; counts: Record<string, number | null> } | null;
}
export interface ProgressRevision {
  revision: number; body: string; kind: ProgressDraft["kind"]; created_at: string;
  editor: { id: string; name: string | null }; correction_reason: string | null;
  health_judgment: ProgressDraft["health_judgment"];
  evidence: { input: ProgressEvidenceInput; availability: string; href: string | null; label: string | null }[];
  acceptance: (ProgressAcceptanceInput & { description_revision: number; description_snapshot: string }) | null;
  statistics_snapshot: ProgressPreview["statistics_snapshot"];
}
export interface ProgressUpdate {
  id: string; current_revision: number; published_at: string; author: { id: string; name: string | null }; current: ProgressRevision;
}
export interface ProgressWrite { update_id: string; result_revision: number; replayed: boolean; result: ProgressRevision }
export interface ProgressNotification { id: string; type: string; details: Record<string, string> | null }

export type ProgressCase = Awaited<ReturnType<typeof p1Session>> & { project: P1Project };

/** Every case receives its own account, workspace and project, including cases
 * whose publication/history preconditions are prepared through the API. */
export const test = base.extend<{ progress: ProgressCase }>({
  progress: async ({ page }, use, info) => {
    const session = await p1Session(page);
    try {
      const project = await p1Project(session.api, { title: `Progress ${info.title.split(" ")[0]}` });
      await use({ ...session, project });
    } finally {
      try { if (info.status !== info.expectedStatus) await p1Failure(page, info); }
      finally { await session.api.deleteFeatureWorkspace(session.workspace.id); }
    }
  },
});

export const progressComposer = (page: Page) => page.locator('[aria-label="Write progress"]');
export const progressEditor = (page: Page) => progressComposer(page).locator('[contenteditable="true"]');
export async function openProgress(page: Page, workspace: { slug: string }, project: P1Project, acceptance = false) {
  await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
  await page.getByRole("button", { name: acceptance ? "Record acceptance" : "Write progress", exact: true }).click();
  return progressComposer(page);
}
export async function addProgressMention(page: Page, name: string, agent = false) {
  const editor = progressEditor(page);
  await editor.pressSequentially(` @${name}`);
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await page.getByRole("button", { name: new RegExp(`${escaped}${agent ? ".*Agent" : ""}$`) }).click();
  // Mention insertion explicitly restores focus on the following frame.
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  await expect(editor).toBeFocused();
}
export async function addProgressEvidence(page: Page, kind: ProgressEvidenceInput["kind"], value: string) {
  const composer = progressComposer(page);
  await composer.getByRole("combobox", { name: "Evidence", exact: true }).selectOption(kind);
  const input = composer.getByRole("textbox", { name: "One HTTP/HTTPS URL, issue UUID, or execution UUID per line.", exact: true });
  await input.click(); await expect(input).toBeFocused(); await input.fill(value); await expect(input).toHaveValue(value);
  await composer.getByRole("button", { name: "Add evidence", exact: true }).click();
}
export async function previewProgress(page: Page, projectId: string) {
  const response = page.waitForResponse((item) => new URL(item.url()).pathname === `/api/projects/${projectId}/updates/preview` && item.request().method() === "POST");
  await progressComposer(page).getByRole("button", { name: "Preview publication", exact: true }).click();
  return response;
}
export async function publishProgress(page: Page, projectId: string, updateId?: string) {
  const path = `/api/projects/${projectId}/updates${updateId ? `/${updateId}` : ""}`;
  const response = page.waitForResponse((item) => new URL(item.url()).pathname === path && item.request().method() === (updateId ? "PUT" : "POST"));
  await progressComposer(page).getByRole("button", { name: "Publish", exact: true }).click();
  return response;
}
/** A real elapsed debounce window, without a synthetic clock or timing-only
 * synchronization of a UI action. The caller asserts the preview afterward. */
export async function passProgressDebounceWindow(page: Page) {
  await page.evaluate(() => new Promise<void>((resolve) => {
    const until = performance.now() + 350;
    const frame = () => performance.now() >= until ? resolve() : requestAnimationFrame(frame);
    requestAnimationFrame(frame);
  }));
}
export function progressDraft(body: string, patch: Partial<ProgressDraft> = {}): ProgressDraft {
  return { operation: "create", update_id: null, expected_revision: null, kind: "progress", body,
    health_judgment: null, evidence: [], acceptance: null, expected_description_revision: null,
    include_statistics: false, correction_reason: null, ...patch };
}
/** API-only preconditions; every case exercises its named operation in UI. */
export async function seedProgress(api: TestApiClient, project: P1Project, draft: ProgressDraft): Promise<ProgressWrite> {
  const preview = await api.requestJSON<ProgressPreview>(`/api/projects/${project.id}/updates/preview`, { method: "POST", body: draft });
  return api.requestJSON<ProgressWrite>(`/api/projects/${project.id}/updates${draft.operation === "correct" ? `/${draft.update_id}` : ""}`, {
    method: draft.operation === "correct" ? "PUT" : "POST",
    body: { request_id: randomUUID(), draft: preview.draft, preview_hash: preview.preview_hash, evidence_versions: preview.evidence_versions },
  });
}
export async function progressUpdates(api: TestApiClient, projectId: string) {
  return (await api.requestJSON<{ items: ProgressUpdate[] }>(`/api/projects/${projectId}/updates`)).items;
}
export async function progressRevisions(api: TestApiClient, projectId: string, updateId: string) {
  return (await api.requestJSON<{ items: ProgressRevision[] }>(`/api/projects/${projectId}/updates/${updateId}/revisions`)).items;
}
export async function progressNotifications(api: TestApiClient, projectId: string) {
  return (await api.requestJSON<ProgressNotification[]>("/api/inbox")).filter((item) => item.type === "project_update" && item.details?.project_id === projectId);
}
export async function seedProgressAgent(api: TestApiClient, name = "P1 reference agent", sharedWith?: string) {
  const runtime = await api.seedProjectRuntime();
  const agent = await api.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: {
    name, runtime_id: runtime.id, permission_mode: sharedWith ? "public_to" : "private",
    invocation_targets: sharedWith ? [{ target_type: "member", target_id: sharedWith }] : [],
  } });
  return { agent, runtime };
}
export async function seedProgressExecution(api: TestApiClient, sharedWith?: string) {
  const { agent, runtime } = await seedProgressAgent(api, "P1 execution evidence fixture", sharedWith);
  const id = randomUUID();
  const summary = `Synthetic customer verification ${new Date().toISOString()}; no process executed`;
  await p1DB("INSERT INTO agent_task_queue(id,agent_id,runtime_id,status,created_at,completed_at,result) VALUES($1,$2,$3,'completed',now()-interval '1 minute',now(),$4::jsonb)", [id, agent.id, runtime.id, JSON.stringify({ summary })]);
  return { id, agent, runtime, summary };
}
export async function progressAgentTaskCount(agentId: string) {
  return Number((await p1DB<{ count: string }>("SELECT count(*) FROM agent_task_queue WHERE agent_id=$1", [agentId]))[0]!.count);
}
export async function progressOutboxCount(projectId: string, recipientId: string) {
  return Number((await p1DB<{ count: string }>("SELECT count(*) FROM project_update_notification WHERE project_id=$1 AND recipient_user_id=$2", [projectId, recipientId]))[0]!.count);
}
export async function withProgressMemberPage(browser: Browser, api: TestApiClient, info: TestInfo, run: (page: Page) => Promise<void>) {
  const context = await browser.newContext({ baseURL: process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN ?? "http://localhost:3000" });
  const page = await context.newPage();
  try { await p1Authenticate(page, api); await run(page); }
  catch (error) { await p1Failure(page, info); throw error; }
  finally { await context.close(); }
}
export async function correctProgressInUI(page: Page, body: string, reason?: string) {
  await page.getByRole("button", { name: "Correct", exact: true }).first().click();
  await progressEditor(page).fill(body);
  if (reason !== undefined) await progressComposer(page).getByRole("textbox", { name: "Reason for correction", exact: true }).fill(reason);
}
export async function fillAcceptance(composer: Locator, body: string, scope: string, explanation?: string) {
  await composer.locator('[contenteditable="true"]').fill(body);
  await composer.getByRole("textbox", { name: "Acceptance scope", exact: true }).fill(scope);
  if (explanation !== undefined) await composer.getByRole("textbox", { name: "Verifiable explanation", exact: true }).fill(explanation);
}
