import { randomUUID } from "node:crypto";
import pg from "pg";
import type { CollaborationIssue, CollaborationResourceIdentityView, CollaborationResourceOptions, CollaborationResources, CollaborationTriggerPreview } from "@multica/core/collaboration";
import { seedCollaborationResourceJourney } from "./collaboration-resources";

/** Real target/consent/comment APIs with an explicitly seeded paused queue.
 * No daemon or executable is started, and no runtime capability is claimed. */
export async function seedCollaborationPreviewJourney() {
  const fixture = await seedCollaborationResourceJourney();
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const suffix = randomUUID().slice(0, 8);
  const squads = [];
  for (const name of [`Build squad ${suffix}`, `Review squad ${suffix}`]) {
    squads.push(await fixture.creator.requestJSON<{ id: string; name: string }>("/api/squads", {
      method: "POST", body: { name, leader_id: fixture.agentId },
    }));
  }
  const options = await fixture.creator.requestJSON<CollaborationResourceOptions>(`${root}/resources/options?execution_workspace_id=${fixture.workspace.id}`);
  const selections = [{ kind: "agent", resourceId: fixture.agentId }, ...squads.map((squad) => ({ kind: "squad", resourceId: squad.id }))];
  const targetIds: string[] = [];
  for (const selection of selections) {
    const candidate = options.items.find((item) => item.kind === selection.kind && item.resource_id === selection.resourceId);
    if (!candidate?.available) throw new Error("Preview fixture target is unavailable");
    const issue = await fixture.creator.requestJSON<CollaborationIssue>(root);
    const resources = await fixture.creator.requestJSON<CollaborationResources>(`${root}/resources`);
    if (!issue.write_context) throw new Error("Preview fixture lacks write context");
    const authority = { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch,
      expected_account_epoch: issue.write_context.account_epoch };
    const registered = await fixture.creator.requestJSON<{ target_id: string }>(`${root}/targets`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...authority, kind: candidate.kind, resource_id: candidate.resource_id,
        expected_target_set_version: resources.target_set_version,
        roster: candidate.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })) },
    });
    targetIds.push(registered.target_id);
    const registeredResources = await fixture.creator.requestJSON<CollaborationResources>(`${root}/resources`);
    const target = registeredResources.targets.find((item) => item.target_id === registered.target_id);
    if (!target) throw new Error("Preview fixture target registration was not visible");
    for (const resource of target.resources) {
      const identity = await fixture.creator.requestJSON<CollaborationResourceIdentityView>(`${root}/resources/identity?target_id=${target.target_id}&agent_id=${resource.agent_id}`);
      if (!identity.identity) throw new Error("Preview fixture runtime enrollment is unavailable");
      await fixture.creator.requestJSON(`${root}/targets/${target.target_id}/consents`, { method: "POST", headers: { "Idempotency-Key": randomUUID() },
        body: { ...authority, agent_id: resource.agent_id, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version, identity: identity.identity } });
    }
  }
  const issue = await fixture.creator.requestJSON<CollaborationIssue>(root);
  const db = new pg.Client(process.env.DATABASE_URL);
  await db.connect();
  try {
    // Seed only this fixture's target holds. The issue stays paused, so the
    // real queue can accept waiting input without any scheduling or launch.
    await db.query("UPDATE collaboration_target SET hold_reason=NULL,hold_version=hold_version+1 WHERE session_id=$1 AND id=ANY($2::uuid[])", [issue.session.id, targetIds]);
  } finally { await db.end(); }
  const earlier = { content: `[@${fixture.agentName}](mention://agent/${fixture.agentId}) Earlier waiting input`, parent_id: null, suppressed_target_ids: [] };
  const preview = await fixture.creator.requestJSON<CollaborationTriggerPreview>(`${root}/comments/trigger-preview`, { method: "POST", body: earlier });
  if (!preview.write_context) throw new Error("Preview fixture input lacks participant authority");
  await fixture.creator.requestJSON(`${root}/comments`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...earlier, expected_lifecycle_epoch: preview.lifecycle_epoch, expected_participant_epoch: preview.write_context.participant_epoch,
    expected_account_epoch: preview.write_context.account_epoch, target_versions: preview.targets.map((target) => ({ target_id: target.target_id,
      expected_target_epoch: target.target_epoch, expected_config_version: target.config_version })),
  } });
  return { ...fixture, squads, targetIds };
}
