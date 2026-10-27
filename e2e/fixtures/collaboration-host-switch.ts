import { createHash, randomUUID } from "node:crypto";
import { expect } from "@playwright/test";
import type { CollaborationIssue } from "@multica/core/collaboration";
import {
  adoptHostTarget, createHostActor, enrollHostRuntime, hostEnvironment, issueRoot, openHostAcceptanceGate, startHostDaemon, versioned, waitForHostRuntime,
  type HostActor, type HostIssue,
} from "./collaboration-host-runtime";

type HostDaemon = Awaited<ReturnType<typeof startHostDaemon>>;

/**
 * One owner with two separately enrolled native runtimes (separate daemon
 * processes, HOME and workdir roots) and a squad S whose leader is agent L1 on
 * the first runtime. The spec later moves S's leader to agent L2 on the second
 * runtime and must Switch the live target before anything runs there.
 */
export async function seedHostSwitch() {
  const env = hostEnvironment();
  const restoreGate = await openHostAcceptanceGate();
  const daemons: HostDaemon[] = [];
  const closeAll = async () => {
    const results = [];
    try { for (const daemon of daemons.splice(0)) results.push(await daemon.close()); } finally { await restoreGate(); }
    return results;
  };
  try {
    const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
    const owner = await createHostActor(env.apiUrl, "Switch", suffix), handler = await createHostActor(env.apiUrl, "Peer", suffix);
    const sourceWorkspace = await owner.client.ensureWorkspace("Host switch source", `host-switch-source-${suffix}`);
    const workspace = await owner.client.requestJSON<{ id: string; slug: string }>("/api/workspaces", { method: "POST", body: {
      name: "Private host switch resources", slug: `host-switch-resource-${suffix}`,
    } });
    owner.client.setWorkspaceId(workspace.id);
    owner.client.setWorkspaceSlug(workspace.slug);
    const first = await enrollHostRuntime(owner.client, owner.id, workspace.id, env.apiUrl, env.deploymentId);
    daemons.push(await startHostDaemon(first, env.apiUrl));
    await waitForHostRuntime(owner.client, first.runtimeId);
    const second = await enrollHostRuntime(owner.client, owner.id, workspace.id, env.apiUrl, env.deploymentId);
    daemons.push(await startHostDaemon(second, env.apiUrl));
    await waitForHostRuntime(owner.client, second.runtimeId);
    expect(second.installationId).not.toBe(first.installationId);
    const createAgent = (name: string, runtimeId: string) => owner.client.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: {
      name, runtime_id: runtimeId, permission_mode: "private",
    } });
    const oldLeader = await createAgent(`Switch leader L1 ${suffix}`, first.runtimeId);
    const newLeader = await createAgent(`Switch leader L2 ${suffix}`, second.runtimeId);
    const squad = await owner.client.requestJSON<{ id: string; name: string }>("/api/squads", { method: "POST", body: { name: `Switch squad S ${suffix}`, leader_id: oldLeader.id } });
    // The only file the issue explicitly delivers. Anything else a run leaves in
    // its workdir is local state that a Switch must not carry to a new runtime.
    const draftId = randomUUID();
    const body = "Explicitly delivered switch attachment. No private files or account data.\n";
    const upload = new FormData();
    upload.set("draft_id", draftId);
    upload.set("source_workspace_id", sourceWorkspace.id);
    upload.set("file", new Blob([body], { type: "text/plain" }), "switch-input.txt");
    const uploaded = await fetch(`${env.apiUrl}/api/collaboration/attachments/drafts`, { method: "POST", headers: {
      Authorization: `Bearer ${owner.client.getToken()!}`, "Idempotency-Key": randomUUID(),
    }, body: upload });
    expect(uploaded.status).toBe(201);
    const file: { id: string; name: string } = await uploaded.json();
    const attachment = { id: file.id, name: file.name, sha256: createHash("sha256").update(body).digest("hex") };
    const issue = await owner.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: sourceWorkspace.id, handler_user_id: handler.id, title: `Host switch execution ${suffix}`,
      description: `Test-owned switch context ${suffix}. Ordinary fake Claude on two native host runtimes.`, draft_id: draftId, attachment_ids: [attachment.id],
    } });
    const target = await adoptHostTarget(owner, owner.id, issue.id, workspace.id, "squad", squad.id, [{ role: "leader", agentId: oldLeader.id, runtime: first }]);
    const view = (await owner.client.requestJSON<CollaborationIssue>(versioned(issueRoot(issue.id)))).execution!.targets.find((item) => item.id === target.targetId)!;
    // HostIssue keeps the shared control/comment helpers usable; agentId is the leader that answers @S.
    const hostIssue: HostIssue = { id: issue.id, title: issue.title, description: issue.description, targetId: target.targetId,
      channelId: view.channel_id!, agentId: oldLeader.id, name: target.name, attachment };
    return { ...env, owner, handler, sourceWorkspace, workspace, squad, oldLeader, newLeader, issue: hostIssue,
      runtimes: { first, second }, daemons: { first: daemons[0]!, second: daemons[1]! }, close: closeAll };
  } catch (error) {
    await closeAll();
    throw error;
  }
}
export type HostSwitchFixture = Awaited<ReturnType<typeof seedHostSwitch>>;

/** A raw authenticated write that returns the server's status and receipt, so refusals can be asserted. */
export async function postCollaboration(apiUrl: string, actor: HostActor, path: string, body: unknown) {
  const response = await fetch(`${apiUrl}${path}`, { method: "POST", redirect: "error", headers: {
    Authorization: `Bearer ${actor.client.getToken()!}`, "Content-Type": "application/json", "Idempotency-Key": randomUUID(),
  }, body: JSON.stringify(body) });
  return { status: response.status, body: await response.json() as { code: string; [key: string]: unknown } };
}

/** The daemon's own run list, read with its issued managed credential exactly as its poller does. */
export async function listRuntimeRuns(fixture: HostSwitchFixture, runtime: "first" | "second") {
  const current = fixture.runtimes[runtime];
  const response = await fetch(`${fixture.apiUrl}/api/collaboration/runtime/runs?runtime_id=${current.runtimeId}`, { redirect: "error", headers: {
    Authorization: `Bearer ${current.credential.daemon_token}`, "X-Client-Capabilities": "managed_installation_v1",
  } });
  const body = await response.text();
  expect(response.status, `runtime ${runtime} run list: ${body}`).toBe(200);
  return (JSON.parse(body) as { run_ids: string[] }).run_ids;
}
