import { randomUUID } from "node:crypto";
import { expect, type Locator, type Page } from "@playwright/test";
import type { CollaborationCommentReceipt, CollaborationIssue, CollaborationResources, CollaborationTriggerPreview } from "@multica/core/collaboration";
import {
  adoptHostTarget, createHostActor, enrollHostRuntime, hostEnvironment, hostMode, issueRoot, openHostAcceptanceGate, startHostDaemon, versioned, waitForHostRuntime,
  type HostActor,
} from "./collaboration-host-runtime";

/** One ordinary agent A that also leads two squads on the same capacity-one host runtime. */
export interface HostSquadTargets {
  agent: { targetId: string; resourceId: string; name: string };
  build: { targetId: string; resourceId: string; name: string };
  review: { targetId: string; resourceId: string; name: string };
}

/**
 * Real offer/adopt/consent APIs for an agent A, a build squad S (A leads, B is
 * the member) and a review squad R (A leads alone). All three share A's lane,
 * so @A+@S must merge and R must wait for the next batch. The native host
 * daemon and fake provider are the same production poller as the host spec.
 */
export async function seedHostSquad() {
  const env = hostEnvironment();
  const restoreGate = await openHostAcceptanceGate();
  let daemon: Awaited<ReturnType<typeof startHostDaemon>> | undefined;
  try {
    const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
    const owner = await createHostActor(env.apiUrl, "Squad", suffix), handler = await createHostActor(env.apiUrl, "Handler", suffix);
    const sourceWorkspace = await owner.client.ensureWorkspace("Host squad source", `host-squad-source-${suffix}`);
    const workspace = await owner.client.requestJSON<{ id: string; slug: string }>("/api/workspaces", { method: "POST", body: {
      name: "Private host squad resources", slug: `host-squad-resource-${suffix}`,
    } });
    owner.client.setWorkspaceId(workspace.id);
    owner.client.setWorkspaceSlug(workspace.slug);
    const runtime = await enrollHostRuntime(owner.client, owner.id, workspace.id, env.apiUrl, env.deploymentId);
    daemon = await startHostDaemon(runtime, env.apiUrl);
    await waitForHostRuntime(owner.client, runtime.runtimeId);
    const createAgent = (name: string) => owner.client.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: {
      name, runtime_id: runtime.runtimeId, permission_mode: "private",
    } });
    const leader = await createAgent(`Squad leader A ${suffix}`), member = await createAgent(`Squad member B ${suffix}`);
    const build = await owner.client.requestJSON<{ id: string; name: string }>("/api/squads", { method: "POST", body: { name: `Build squad S ${suffix}`, leader_id: leader.id } });
    await owner.client.requestJSON(`/api/squads/${build.id}/members`, { method: "POST", body: { member_type: "agent", member_id: member.id, role: "member" } });
    const review = await owner.client.requestJSON<{ id: string; name: string }>("/api/squads", { method: "POST", body: { name: `Review squad R ${suffix}`, leader_id: leader.id } });
    const issue = await owner.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: sourceWorkspace.id, handler_user_id: handler.id, title: `Host squad execution ${suffix}`,
      description: `Test-owned squad context ${suffix}. Ordinary fake Claude on one capacity-one host runtime.`, attachment_ids: [],
    } });
    const root = issueRoot(issue.id);
    const adopt = (kind: "agent" | "squad", resourceId: string, roster: [string, string][]) => adoptHostTarget(owner, owner.id, issue.id, workspace.id,
      kind, resourceId, roster.map(([role, agentId]) => ({ role, agentId, runtime })));
    const targets: HostSquadTargets = {
      agent: await adopt("agent", leader.id, [["agent", leader.id]]),
      build: await adopt("squad", build.id, [["leader", leader.id], ["member", member.id]]),
      review: await adopt("squad", review.id, [["leader", leader.id]]),
    };
    const authorized = await owner.client.requestJSON<CollaborationResources>(versioned(`${root}/resources`));
    for (const target of Object.values(targets)) expect(authorized.targets.find((item) => item.target_id === target.targetId)).toMatchObject({ authorized: true });
    const rawClose = daemon.close;
    let closing: ReturnType<typeof rawClose> | undefined;
    const close = () => (closing ??= (async () => { try { return await rawClose(); } finally { await restoreGate(); } })());
    return { ...env, owner, handler, sourceWorkspace, workspace, runtime, daemon: { ...daemon, close }, leader, member, build, review,
      issue: { id: issue.id, title: issue.title, description: issue.description }, targets };
  } catch (error) {
    try { await daemon?.close(); } finally { await restoreGate(); }
    throw error;
  }
}
export type HostSquadFixture = Awaited<ReturnType<typeof seedHostSquad>>;

export async function readSquadIssue(fixture: HostSquadFixture, actor: HostActor = fixture.owner) {
  const current = await actor.client.requestJSON<CollaborationIssue>(versioned(issueRoot(fixture.issue.id)));
  expect(current.execution?.protocol_version).toBe(6);
  expect(current.execution?.targets).toHaveLength(3);
  return current.execution!;
}

export type SquadPick = { kind: "agent" | "squad"; resourceId: string; name: string };

/**
 * Sends one comment through the real Web composer with the picked AI targets,
 * checks the grouping preview before sending, and returns the typed receipt.
 */
export async function sendSquadComment(page: Page, fixture: { issue: { id: string } }, marker: string, picks: SquadPick[],
  inspectPreview: (grouping: Locator, preview: CollaborationTriggerPreview) => Promise<void>) {
  const root = issueRoot(fixture.issue.id);
  const composer = page.getByRole("form", { name: "Comments", exact: true });
  await composer.getByLabel("Add comment", { exact: true }).fill(marker);
  const choices = composer.getByRole("group", { name: "AI targets", exact: true });
  for (const pick of picks) await choices.getByRole("button", { name: pick.name, exact: true }).click();
  const content = await composer.getByLabel("Add comment", { exact: true }).inputValue();
  for (const pick of picks) expect(content).toContain(`mention://${pick.kind}/${pick.resourceId}`);
  const previewed = page.waitForResponse((response) => new URL(response.url()).pathname === `${root}/comments/trigger-preview` && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Preview delivery", exact: true }).click();
  const previewResponse = await previewed;
  expect(previewResponse.status()).toBe(200);
  expect(previewResponse.request().postDataJSON()).toMatchObject({ execution_view_version: 6 });
  await inspectPreview(page.getByRole("region", { name: "AI grouping preview", exact: true }), await previewResponse.json());
  const sent = page.waitForResponse((response) => new URL(response.url()).pathname === `${root}/comments` && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Send comment", exact: true }).click();
  const response = await sent;
  expect(response.status()).toBe(201);
  const receipt: CollaborationCommentReceipt = await response.json();
  expect(receipt).toMatchObject({ status: 201, issue_id: fixture.issue.id, code: "comment_created" });
  expect(receipt.targets).toHaveLength(picks.length);
  await expect(composer.getByLabel("Add comment", { exact: true })).toHaveValue("");
  const byTarget = (targetId: string) => {
    const target = receipt.targets.find((item) => item.target_id === targetId);
    if (!target?.command_id) throw new Error(`Comment ${marker} did not queue a command for ${targetId}`);
    return target;
  };
  return { content, marker, receipt, commentId: receipt.comment_id, byTarget };
}
export type SquadComment = Awaited<ReturnType<typeof sendSquadComment>>;

export async function waitSquadRun(fixture: HostSquadFixture, runId: string) {
  await expect.poll(async () => (await readSquadIssue(fixture)).runs.find((run) => run.id === runId), { timeout: 30_000 })
    .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", execution_mode: hostMode, launch_count: "1",
      business_outcome: "success", forced_disposition: null });
  return (await readSquadIssue(fixture)).runs.find((run) => run.id === runId)!;
}

/** Resume or stop the whole issue through the real control button. */
export async function squadControl(page: Page, fixture: HostSquadFixture, action: "resume" | "stop") {
  const root = issueRoot(fixture.issue.id);
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${root}/control` && response.request().method() === "POST");
  const panel = page.getByRole("region", { name: "AI execution", exact: true });
  await panel.getByRole("button", { name: action === "resume" ? "Resume all" : "Stop all", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  expect(response.request().postDataJSON()).toMatchObject({ action, scope: "issue", execution_view_version: 6 });
  return response.json();
}
