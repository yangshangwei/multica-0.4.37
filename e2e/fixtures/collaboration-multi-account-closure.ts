import { randomUUID } from "node:crypto";
import { expect, type Page } from "@playwright/test";
import type { CollaborationIssue, CollaborationOperation, CollaborationResourceIdentityView, CollaborationResourceOptions, CollaborationResources, CollaborationStatusOptions } from "@multica/core/collaboration";
import { createHostActor, enrollHostRuntime, hostAuthority, hostMode, issueRoot, startHostDaemon, versioned, waitForHostRuntime, type HostActor, type HostIssue } from "./collaboration-host-runtime";
import { selectCollaborationRecipient } from "./collaboration-offline-handoff";
import { revealCollaborationDisclosure } from "./collaboration-disclosures";
import { refreshDirectedIssue } from "./collaboration-handoff-chain";

export type ClosureActor = HostActor & { name: string };
export type ClosureResource = Awaited<ReturnType<typeof createClosureResource>>;
export const readClosureIssue = (actor: HostActor, id: string) => actor.client.requestJSON<CollaborationIssue>(versioned(issueRoot(id)));
export const workflowAuthority = (issue: CollaborationIssue) => {
  const authority = hostAuthority(issue);
  return { expected_lifecycle_epoch: authority.expected_lifecycle_epoch, expected_participant_epoch: authority.expected_participant_epoch,
    expected_account_epoch: authority.expected_account_epoch, expected_collaboration_revision: issue.collaboration_revision };
};

export async function closureActor(apiUrl: string, label: string, suffix: string): Promise<ClosureActor> {
  return { ...await createHostActor(apiUrl, label, suffix), name: `Host collaborator ${label}` };
}

export async function createClosureResource(actor: ClosureActor, apiUrl: string, deploymentId: string, slug: string) {
  const workspace = await actor.client.ensureWorkspace(`${actor.name} private workspace`, slug);
  const runtime = await enrollHostRuntime(actor.client, actor.id, workspace.id, apiUrl, deploymentId);
  const daemon = await startHostDaemon(runtime, apiUrl);
  try {
    await waitForHostRuntime(actor.client, runtime.runtimeId);
    const agent = await actor.client.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: {
      name: `${actor.name} agent`, runtime_id: runtime.runtimeId, permission_mode: "private",
    } });
    return { actor, workspace, runtime, daemon, agent };
  } catch (error) { await daemon.close(); throw error; }
}

export async function rawClosureRequest(apiUrl: string, actor: HostActor, path: string, method = "GET", body?: unknown, key: string = randomUUID()) {
  const response = await fetch(`${apiUrl}${path}`, { method, headers: { Authorization: `Bearer ${actor.client.getToken()!}`,
    "Content-Type": "application/json", ...(method === "GET" ? {} : { "Idempotency-Key": key }) },
    body: body === undefined ? undefined : JSON.stringify(body) });
  return { status: response.status, body: await response.json(), request_id: key };
}

// The supplier and the manager deliberately remain separate identities.
export async function offerClosureResource(resource: ClosureResource, manager: ClosureActor, issue: CollaborationIssue) {
  const { actor, agent, workspace, runtime } = resource;
  const root = issueRoot(issue.id);
  const options = await actor.client.requestJSON<CollaborationResourceOptions>(versioned(`${root}/resources/offers/options?execution_workspace_id=${workspace.id}`));
  const option = options.items.find(item => item.kind === "agent" && item.resource_id === agent.id);
  expect(option?.available).toBe(true);
  if (!option) throw new Error("The owner's real host resource was not offerable");
  expect(option.resources).toEqual([expect.objectContaining({ agent_id: agent.id, runtime_id: runtime.runtimeId, owner_user_id: actor.id,
    execution_workspace_id: workspace.id, execution_mode: hostMode })]);
  const offer = await actor.client.requestJSON<{ code: string; offer_id: string; offer_version: string }>(`${root}/resources/offers`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...hostAuthority(await readClosureIssue(actor, issue.id)),
      kind: "agent", resource_id: agent.id, execution_workspace_id: workspace.id,
      roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })) },
  });
  expect(offer.code).toBe("resource_offered");
  const resources = await manager.client.requestJSON<CollaborationResources>(versioned(`${root}/resources`));
  const adopted = await manager.client.requestJSON<{ code: string; target_id: string }>(`${root}/resources/offers/${offer.offer_id}/adopt`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...hostAuthority(await readClosureIssue(manager, issue.id)),
      expected_offer_version: offer.offer_version, expected_target_set_version: resources.target_set_version },
  });
  expect(adopted.code).toBe("resource_offer_adopted");
  const current = await readClosureIssue(actor, issue.id);
  const target = current.execution?.targets.find(item => item.id === adopted.target_id);
  if (!target?.channel_id) throw new Error("Adoption did not create a channel");
  const identity = await actor.client.requestJSON<CollaborationResourceIdentityView>(versioned(`${root}/resources/identity?target_id=${target.id}&agent_id=${agent.id}`));
  expect(identity.identity?.installation_id).toBe(runtime.installationId);
  const consentBody = { ...hostAuthority(current), agent_id: agent.id, expected_target_epoch: target.target_epoch,
    expected_config_version: target.config_version, identity: identity.identity };
  const hostIssue: HostIssue = { id: issue.id, title: issue.title, description: issue.description, targetId: target.id,
    channelId: target.channel_id, agentId: agent.id, name: agent.name, attachment: null };
  return { offer, adopted, consentBody, hostIssue };
}

export async function simpleWorkflow(page: Page, issueId: string, action: "invite" | "transfer", target: ClosureActor, note = "") {
  await refreshDirectedIssue(page, issueId);
  const label = action === "invite" ? "Add participant" : "Transfer";
  const button = page.getByRole("button", { name: label, exact: true }).or(page.getByRole("menuitem", { name: label, exact: true }));
  if (!await button.isVisible()) await revealCollaborationDisclosure(page, "More actions");
  await button.click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: label, exact: true }) });
  await selectCollaborationRecipient(page, form, target);
  if (action === "transfer") await form.getByLabel("Note", { exact: true }).fill(note);
  const path = `${issueRoot(issueId)}/${action === "invite" ? "participants" : "handoffs"}`;
  const pending = page.waitForResponse(response => new URL(response.url()).pathname === path && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe(action === "invite" ? "participant_added" : "transferred");
  await expect(form).toHaveCount(0);
  return { receipt, input: response.request().postDataJSON(), request_id: response.request().headers()["idempotency-key"]! };
}

export async function decideClosureResult(page: Page, issueId: string, target?: ClosureActor, reason = "") {
  const baseline = await refreshDirectedIssue(page, issueId);
  const submission = baseline.delivery?.current;
  if (!submission) throw new Error("Expected a pending delivery");
  const action = target ? "return_acceptance" : "complete";
  const label = target ? "Return for revision" : "Accept and complete";
  const optionsPending = page.waitForResponse(response => new URL(response.url()).pathname === `${issueRoot(issueId)}/status-options`
    && new URL(response.url()).searchParams.get("action") === action && response.request().method() === "GET");
  await page.getByRole("button", { name: label, exact: true }).click();
  const optionsResponse = await optionsPending;
  expect(optionsResponse.status()).toBe(200);
  const options: CollaborationStatusOptions = await optionsResponse.json();
  const form = page.getByRole("form", { name: label, exact: true });
  await expect(form.locator(`[data-submission-id="${submission.id}"]`)).toHaveCount(1);
  if (target) {
    await selectCollaborationRecipient(page, form, target);
    await form.getByLabel("Revision reason", { exact: true }).fill(reason);
  }
  await form.getByLabel("Status", { exact: true }).selectOption(options.default_status);
  const path = `${issueRoot(issueId)}/${target ? "acceptance-returns" : "lifecycle"}`;
  const pending = page.waitForResponse(response => new URL(response.url()).pathname === path && response.request().method() === "POST");
  await form.getByRole("button", { name: label, exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe(target ? "acceptance_returned" : "completed");
  const input: Record<string, unknown> = response.request().postDataJSON();
  expect(input).toMatchObject({ submission_id: submission.id, status_options_version: options.status_options_version });
  await expect(form).toHaveCount(0);
  return { receipt, input, submission_id: submission.id, request_id: response.request().headers()["idempotency-key"]! };
}
