import { randomBytes, randomUUID } from "node:crypto";
import pg from "pg";
import type { Page } from "@playwright/test";
import { TestApiClient } from "../fixtures";

export interface CollaborationResourceSnapshot {
  id: string;
  title: string;
  collaboration_revision: string;
  lifecycle_epoch: string;
  write_context: { participant_epoch: string; account_epoch: string };
  handler: { user_id: string };
  execution: { targets: { id: string; resource_id: string; hold_reason: string | null }[]; runs: unknown[] };
}

export async function authenticateCollaborationResourcesPage(page: Page, actor: TestApiClient) {
  const token = actor.getToken();
  if (!token) throw new Error("Fixture actor has no login session");
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
}

/** Retained only in the caller's explicitly isolated DB. No daemon process,
 * provider credential or executable is installed or invoked by this fixture. */
export async function seedCollaborationResourceJourney() {
  const databaseUrl = process.env.DATABASE_URL;
  const apiUrl = process.env.COLLABORATION_E2E_API_URL;
  const deploymentId = process.env.MULTICA_DEPLOYMENT_ID;
  if (!databaseUrl || !apiUrl || !deploymentId || !process.env.NEXT_PUBLIC_API_URL) throw new Error("Requires isolated collaboration resource E2E environment");
  for (const value of [databaseUrl, apiUrl, process.env.NEXT_PUBLIC_API_URL]) {
    if (!["localhost", "127.0.0.1"].includes(new URL(value).hostname)) throw new Error("Resource fixture requires loopback test services");
  }
  if (new URL(apiUrl).origin !== new URL(process.env.NEXT_PUBLIC_API_URL).origin) throw new Error("Browser and fixture API origins differ");
  const suffix = randomUUID().slice(0, 10);
  const creator = new TestApiClient();
  await creator.login(`resource-creator-${suffix}@example.invalid`, "Resource creator");
  const workspace = await creator.ensureWorkspace("Resource journey workspace", `resource-journey-${suffix}`);
  await creator.markUserOnboarded();
  const owner = await creator.requestJSON<{ id: string }>("/api/me");
  const peer = new TestApiClient();
  await peer.login(`resource-peer-${suffix}@example.invalid`, "Ordinary resource participant");
  await peer.markUserOnboarded();
  const peerAccount = await peer.requestJSON<{ id: string }>("/api/me");
  const disabled = new TestApiClient();
  await disabled.login(`resource-disabled-${suffix}@example.invalid`, "Disabled resource handler");
  await disabled.markUserOnboarded();
  const disabledAccount = await disabled.requestJSON<{ id: string }>("/api/me");
  const options = await creator.requestJSON<{ creation_enabled: boolean }>("/api/collaboration/creation-options");
  if (options.creation_enabled !== true) throw new Error("Enable creation only in the task-owned browser database before running this journey");
  const issue = await creator.requestJSON<CollaborationResourceSnapshot>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() },
    body: { source_workspace_id: workspace.id, handler_user_id: disabledAccount.id, title: `Resource collaboration ${suffix}`,
      description: "Disposable resource authorization journey. No AI execution is requested.", attachment_ids: [] },
  });
  await creator.requestJSON(`/api/collaboration/issues/${issue.id}/participants`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    target_user_id: peerAccount.id, expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch,
  } });
  const runtime = await creator.seedProjectRuntime();
  const agentName = `Resource journey agent ${suffix}`;
  const agent = await creator.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: agentName, runtime_id: runtime.id } });
  const db = new pg.Client(databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    await db.query("UPDATE agent_runtime SET status='offline', custom_name='Resource journey machine' WHERE id=$1 AND owner_id=$2", [runtime.id, owner.id]);
    await db.query("INSERT INTO collaboration_runtime_enrollment (deployment_id,owner_user_id,execution_workspace_id,runtime_id,installation_id,public_key) VALUES ($1,$2,$3,$4,$5,$6)",
      [deploymentId, owner.id, workspace.id, runtime.id, randomUUID(), randomBytes(32)]);
    await db.query('UPDATE "user" SET disabled_at=now() WHERE id=$1', [disabledAccount.id]);
    await db.query("COMMIT");
  } catch (error) { await db.query("ROLLBACK"); throw error; }
  finally { await db.end(); }
  return { creator, peer, ownerId: owner.id, peerId: peerAccount.id, disabledId: disabledAccount.id, issueId: issue.id, title: issue.title,
    workspace, runtimeId: runtime.id, agentId: agent.id, agentName };
}
