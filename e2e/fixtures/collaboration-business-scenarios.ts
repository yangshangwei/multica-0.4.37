import { createHash, randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";
import { expect, type Browser, type BrowserContext, type Page, type TestInfo } from "@playwright/test";
import type {
  CollaborationControlReceipt, CollaborationIssue, CollaborationOperation, CollaborationResourceIdentityView, CollaborationResourceOptions, CollaborationResources,
} from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";
import {
  enrollHostRuntime, hostAuthority, hostMode, hostPanel, issueRoot, openHostConsent, startHostDaemon, versioned, waitForHostRuntime, type HostIssue,
} from "./collaboration-host-runtime";
import { revealCollaborationDisclosure } from "./collaboration-disclosures";
import { refreshDirectedIssue } from "./collaboration-handoff-chain";

// Task-owned helpers for the business-scenario closure journeys. Setup uses the
// same public APIs as the existing host fixtures; business actions stay in the UI.

export const sha256 = (value: string | Buffer) => createHash("sha256").update(value).digest("hex");
export const readIssue = (actor: BizActor, id: string) => actor.client.requestJSON<CollaborationIssue>(versioned(issueRoot(id)));

export interface BizActor {
  client: TestApiClient; id: string; name: string; username: string;
  authenticateInstalled: (login: (credentials: { username: string; password: string; userId: string }) => Promise<unknown>) => Promise<unknown>;
  authenticateWeb(page: Page): Promise<void>;
}

/** A synthetic, onboarded password account with a business display name. */
export async function bizActor(apiUrl: string, key: string, name: string, suffix: string): Promise<BizActor> {
  const client = new TestApiClient();
  const username = `biz${key}${suffix}`, password = `Biz-${randomUUID()}!`;
  const user = await client.registerPassword(username, password, name);
  const onboarded = await client.requestJSON<{ onboarded_at: string | null }>("/api/me/onboarding/complete", { method: "POST", body: {} });
  expect(onboarded.onboarded_at).not.toBeNull();
  return { client, id: user.id, name, username,
    authenticateInstalled: (login) => login({ username, password, userId: user.id }),
    async authenticateWeb(page: Page) {
      const response = await page.request.post(`${apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200);
      const cookies = await page.context().cookies(apiUrl);
      expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
    } };
}

export type BizDaemon = Awaited<ReturnType<typeof startHostDaemon>>;
export interface BizRuntimeOwner {
  actor: BizActor; workspace: { id: string; slug: string; name?: string };
  runtime: Awaited<ReturnType<typeof enrollHostRuntime>>; daemon: BizDaemon;
}

/** A private workspace with one managed host runtime and its real native poller. */
export async function bizRuntimeOwner(actor: BizActor, env: { apiUrl: string; deploymentId: string }, workspaceName: string, slug: string): Promise<BizRuntimeOwner> {
  const workspace = await actor.client.ensureWorkspace(workspaceName, slug);
  if (!workspace || workspace.slug !== slug) throw new Error("The fixture must own a fresh workspace");
  const runtime = await enrollHostRuntime(actor.client, actor.id, workspace.id, env.apiUrl, env.deploymentId);
  const daemon = await startHostDaemon(runtime, env.apiUrl);
  try {
    await waitForHostRuntime(actor.client, runtime.runtimeId);
  } catch (error) { await daemon.close(); throw error; }
  return { actor, workspace, runtime, daemon };
}

export async function bizAgent(owner: BizRuntimeOwner, name: string) {
  return owner.actor.client.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: {
    name, runtime_id: owner.runtime.runtimeId, permission_mode: "private" } });
}

export async function bizSquad(owner: BizRuntimeOwner, name: string, leaderId: string, memberIds: string[]) {
  const squad = await owner.actor.client.requestJSON<{ id: string; name: string }>("/api/squads", { method: "POST", body: { name, leader_id: leaderId } });
  for (const id of memberIds) {
    await owner.actor.client.requestJSON(`/api/squads/${squad.id}/members`, { method: "POST", body: { member_type: "agent", member_id: id, role: "member" } });
  }
  return squad;
}

export interface BizTarget { targetId: string; channelId: string; kind: "agent" | "squad"; resourceId: string; name: string; offerId: string }

/** The resource owner offers; a separate issue manager adopts. Consent stays with the owner. */
export async function offerAndAdopt(owner: BizRuntimeOwner, manager: BizActor, issueId: string, kind: "agent" | "squad", resourceId: string,
  roster: { role: string; agentId: string }[]): Promise<BizTarget> {
  const root = issueRoot(issueId);
  const options = await owner.actor.client.requestJSON<CollaborationResourceOptions>(versioned(`${root}/resources/offers/options?execution_workspace_id=${owner.workspace.id}`));
  const option = options.items.find((item) => item.kind === kind && item.resource_id === resourceId);
  if (!option?.available) throw new Error(`The owner's ${kind} is not offerable (${option?.reason ?? "missing"})`);
  const byAgent = (a: { agent_id: string }, b: { agent_id: string }) => a.agent_id.localeCompare(b.agent_id);
  expect(option.resources.map(({ role, agent_id, runtime_id }) => ({ role, agent_id, runtime_id })).sort(byAgent))
    .toEqual(roster.map(({ role, agentId }) => ({ role, agent_id: agentId, runtime_id: owner.runtime.runtimeId })).sort(byAgent));
  for (const item of option.resources) expect(item).toMatchObject({ owner_user_id: owner.actor.id, execution_workspace_id: owner.workspace.id, execution_mode: hostMode });
  const offer = await owner.actor.client.requestJSON<{ code: string; offer_id: string; offer_version: string }>(`${root}/resources/offers`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...hostAuthority(await readIssue(owner.actor, issueId)),
      kind, resource_id: resourceId, execution_workspace_id: owner.workspace.id,
      roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })) },
  });
  expect(offer.code).toBe("resource_offered");
  const before = await manager.client.requestJSON<CollaborationResources>(versioned(`${root}/resources`));
  const adopted = await manager.client.requestJSON<{ code: string; target_id: string }>(`${root}/resources/offers/${offer.offer_id}/adopt`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...hostAuthority(await readIssue(manager, issueId)),
      expected_offer_version: offer.offer_version, expected_target_set_version: before.target_set_version },
  });
  expect(adopted.code).toBe("resource_offer_adopted");
  const target = (await readIssue(owner.actor, issueId)).execution?.targets.find((item) => item.id === adopted.target_id);
  if (!target?.channel_id) throw new Error("Adoption did not create a target channel");
  for (const { agentId } of roster) {
    const identity = await owner.actor.client.requestJSON<CollaborationResourceIdentityView>(versioned(`${root}/resources/identity?target_id=${target.id}&agent_id=${agentId}`));
    expect(identity.identity?.installation_id).toBe(owner.runtime.installationId);
  }
  return { targetId: target.id, channelId: target.channel_id, kind, resourceId, name: option.name, offerId: offer.offer_id };
}

export async function openActor(browser: Browser, actor: BizActor, contexts: BrowserContext[], errors: string[]): Promise<Page> {
  const context = await browser.newContext();
  contexts.push(context);
  await context.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
  const page = await context.newPage();
  page.on("pageerror", (error) => errors.push(`${actor.name}: ${error.message}`));
  await actor.authenticateWeb(page);
  return page;
}

export async function showIssue(page: Page, baseURL: string, issue: { id: string; title: string }) {
  await page.goto(new URL(`/account?view=participating&issue=${issue.id}`, baseURL).href);
  await expect(page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
}

/** Select the exact resource's control when several targets share the execution panel. */
export async function controlTargetInUI(page: Page, issueId: string, target: BizTarget, action: "stop" | "resume") {
  const label = `${action === "stop" ? "Stop" : "Resume"} target`;
  const row = hostPanel(page).getByRole("listitem")
    .filter({ has: page.getByText(target.resourceId, { exact: true }) })
    .filter({ has: page.getByRole("button", { name: label, exact: true }) });
  await expect(row).toHaveCount(1);
  const [response] = await Promise.all([
    page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issueId)}/control`
      && response.request().method() === "POST"),
    row.getByRole("button", { name: label, exact: true }).click(),
  ]);
  expect(response.status()).toBe(200);
  expect(response.request().postDataJSON()).toMatchObject({ action, scope: "target", execution_view_version: 6,
    targets: [{ target_id: target.targetId }] });
  const receipt: CollaborationControlReceipt = await response.json();
  expect(receipt).toMatchObject({ action, scope: "target", target_id: target.targetId,
    queue_state: action === "resume" ? "active" : "paused" });
  return receipt;
}

/** The resource owner authorizes one roster resource through the shared consent panel. */
export async function consentInUI(page: Page, issue: { id: string; title: string }, target: BizTarget, agent: { id: string; name: string }) {
  const hostIssue: HostIssue = { id: issue.id, title: issue.title, description: "", targetId: target.targetId, channelId: target.channelId,
    agentId: agent.id, name: agent.name, attachment: null };
  await openHostConsent(page, hostIssue);
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issue.id)}/targets/${target.targetId}/consents`
    && response.request().method() === "POST");
  await hostPanel(page).getByRole("button", { name: "Authorize for this issue", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(201);
  expect(response.request().postDataJSON()).toMatchObject({ agent_id: agent.id, execution_view_version: 6 });
  const receipt: { code: string; target_id: string } = await response.json();
  expect(receipt).toMatchObject({ code: "resource_consented", target_id: target.targetId });
  return receipt;
}

/** The owner withdraws one of their grants in Manage AI resources. */
export async function withdrawInUI(page: Page, issueId: string, agentName: string) {
  const manage = hostPanel(page).getByRole("button", { name: "Manage AI resources", exact: true });
  if (await manage.getAttribute("aria-expanded") !== "true") await manage.click();
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname.startsWith(`${issueRoot(issueId)}/grants/`)
    && response.url().endsWith("/revoke") && response.request().method() === "POST");
  await page.getByRole("button", { name: `Withdraw authorization for ${agentName}`, exact: true }).click();
  await page.getByRole("button", { name: "Withdraw authorization", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  return response.json() as Promise<{ code: string; channels: { channel_id: string; code: string; stopped_runs: number; unsettled_runs: number }[];
    target_holds: { target_id: string; hold_reason: string | null }[] }>;
}

async function workflowButton(page: Page, label: string) {
  const button = page.getByRole("button", { name: label, exact: true }).or(page.getByRole("menuitem", { name: label, exact: true }));
  if (!await button.first().isVisible()) await revealCollaborationDisclosure(page, "More actions");
  await button.first().click();
}

/** Initiator-only AI participation change through the shared workflow form. */
export async function changePolicyInUI(page: Page, issueId: string, policy: "manual" | "assisted") {
  await refreshDirectedIssue(page, issueId);
  await workflowButton(page, "Change AI policy");
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Change AI policy", exact: true }) });
  await form.getByLabel("AI participation", { exact: true }).selectOption(policy);
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issueId)}/ai-policy` && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe("ai_policy_changed");
  await expect(form).toHaveCount(0);
  return { receipt, input: response.request().postDataJSON() as Record<string, unknown> };
}

/** Initiator removes a participant through the Participants list and its confirmation. */
export async function removeParticipantInUI(page: Page, issueId: string, target: { id: string; name: string }) {
  await refreshDirectedIssue(page, issueId);
  const toggle = page.getByRole("button", { name: "Participants", exact: true }).first();
  if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  await page.getByRole("button", { name: `Remove ${target.name}`, exact: true }).click();
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toContainText(`Remove ${target.name}?`);
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issueId)}/participants/${target.id}/remove`
    && response.request().method() === "POST");
  await dialog.getByRole("button", { name: "Remove participant", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe("participant_removed");
  return receipt;
}

export async function rawCall(apiUrl: string, actor: BizActor, path: string, method = "GET", body?: unknown) {
  const response = await fetch(`${apiUrl}${path}`, { method, headers: { Authorization: `Bearer ${actor.client.getToken()!}`, "Content-Type": "application/json",
    ...(method === "GET" ? {} : { "Idempotency-Key": randomUUID() }) }, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await response.text();
  let parsed: unknown = null;
  try { parsed = text ? JSON.parse(text) : null; } catch { parsed = null; }
  return { status: response.status, body: parsed as Record<string, unknown> | null, text };
}

export interface WorkbenchItem { id: string; title?: string }
export async function workbench(actor: BizActor, view: "current" | "participating") {
  const page = await actor.client.requestJSON<{ items: WorkbenchItem[]; total: number }>(`/api/me/workbench?view=${view}&limit=100`);
  return page.items.map((item) => item.id);
}

export interface BizNotification { id: string; issue_id: string; kind: string; can_open: boolean; issue_title: string | null }
export async function notifications(actor: BizActor, issueId: string) {
  const page = await actor.client.requestJSON<{ items: BizNotification[] }>("/api/me/collaboration-notifications?limit=100&include_archived=true");
  return page.items.filter((item) => item.issue_id === issueId);
}

/** Uploads one draft file for a later comment, creation or delivery reference. */
export async function uploadDraft(apiUrl: string, actor: BizActor, sourceWorkspaceId: string, intendedIssueId: string | null, name: string, body: string) {
  const draftId = randomUUID();
  const form = new FormData();
  form.set("draft_id", draftId);
  form.set("source_workspace_id", sourceWorkspaceId);
  if (intendedIssueId) form.set("issue_id", intendedIssueId);
  form.set("file", new Blob([body], { type: "text/plain" }), name);
  const response = await fetch(`${apiUrl}/api/collaboration/attachments/drafts`, { method: "POST", headers: {
    Authorization: `Bearer ${actor.client.getToken()!}`, "Idempotency-Key": randomUUID() }, body: form });
  expect(response.status).toBe(201);
  const file: { id: string; name: string } = await response.json();
  return { draftId, id: file.id, name: file.name, sha256: sha256(body) };
}

/** Poll an exact run to its settled host fact. */
export async function waitRun(actor: BizActor, issueId: string, runId: string, expected: Record<string, unknown>, timeout = 40_000) {
  await expect.poll(async () => (await readIssue(actor, issueId)).execution?.runs.find((run) => run.id === runId), { timeout }).toMatchObject(expected);
  return (await readIssue(actor, issueId)).execution!.runs.find((run) => run.id === runId)!;
}

export function runCount(issue: CollaborationIssue) {
  return issue.execution?.runs.length ?? 0;
}

/** Ordered business-step evidence written with owner-only permissions. */
export class Evidence {
  readonly steps: { step: string; at: string; facts: unknown }[] = [];
  readonly data: Record<string, unknown> = {};
  constructor(readonly scenario: string, readonly info: TestInfo) {}
  record(step: string, facts: unknown = {}) { this.steps.push({ step, at: new Date().toISOString(), facts }); }
  async shot(page: Page, name: string) {
    try { await page.screenshot({ path: this.info.outputPath(`${name}.png`), fullPage: true, timeout: 10_000 }); }
    catch { this.record(`screenshot unavailable: ${name}`); }
  }
  async write(result: "passed" | "failed", extra: Record<string, unknown>) {
    const encoded = JSON.stringify({ scenario: this.scenario, result, ...this.data, ...extra, steps: this.steps }, null, 2);
    expect(encoded, "Evidence must not contain credential-shaped values").not.toMatch(/\b(?:mcb_|mct_|mdt_|mat_|mul_)[A-Za-z0-9_.-]+/);
    const path = this.info.outputPath(`${this.scenario}-evidence.json`);
    writeFileSync(path, encoded, { mode: 0o600 });
    await this.info.attach(`${this.scenario}-evidence`, { path, contentType: "application/json" });
  }
}

/** Close every owned daemon, check its exit ledger, then the browser contexts. */
export async function closeOwned(owners: BizRuntimeOwner[], contexts: BrowserContext[], strict: boolean) {
  const cleanup: unknown[] = [];
  const failures: string[] = [];
  for (const owner of [...owners].reverse()) {
    try {
      const summary = await owner.daemon.close();
      cleanup.push({ owner_id: owner.actor.id, runtime_id: owner.runtime.runtimeId, exit_code: summary.exit_code, errors: summary.errors, ledger: summary.ledger });
      if (strict) {
        if (summary.exit_code !== 0) failures.push(`${owner.actor.name} daemon exit ${summary.exit_code}`);
        if (summary.errors.length) failures.push(`${owner.actor.name} daemon errors: ${summary.errors.join("; ")}`);
        for (const visit of owner.daemon.visits) {
          const entry = summary.ledger.find((item) => item.run_id === visit.identity.run_id);
          if (!entry || entry.phase !== "proof_acknowledged" || entry.sealed !== true || entry.proof_state !== "host_exit_observed") {
            failures.push(`${owner.actor.name} run ${visit.identity.run_id} lacks an acknowledged host exit ledger`);
          }
        }
      }
    } catch (error) {
      failures.push(error instanceof Error ? error.message : "daemon cleanup failed");
    }
  }
  for (const context of contexts) {
    try { await context.close(); } catch (error) { failures.push(error instanceof Error ? error.message : "browser cleanup failed"); }
  }
  return { cleanup, failures };
}
