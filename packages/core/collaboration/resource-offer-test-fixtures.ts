import { resourceIssue, targetResource } from "./resource-test-fixtures";

export const resourceOfferFixture = {
  protocol_version: 1, issue_id: resourceIssue.id, offer_id: "00000000-0000-4000-8000-000000000091", version: "1", state: "offered",
  offered_by: resourceIssue.creator, kind: "agent", resource_id: targetResource.agent_id, execution_workspace_id: targetResource.execution_workspace_id,
  name: "Offered agent", resources: [{ role: targetResource.role, agent_id: targetResource.agent_id, runtime_id: targetResource.runtime_id,
    execution_workspace_id: targetResource.execution_workspace_id, owner_user_id: targetResource.owner_user_id,
    agent_name: targetResource.agent_name, runtime: targetResource.runtime, owner: targetResource.owner }],
  created_at: "2026-10-21T01:00:00Z", withdrawn_at: null, adopted_at: null, adopted_by: null, target_id: null,
  available: true, reason: "", can_adopt: true, can_withdraw: true,
};
export const resourceOffersPageFixture = {
  protocol_version: 1, issue_id: resourceIssue.id, lifecycle_epoch: resourceIssue.lifecycle_epoch,
  target_set_version: resourceIssue.execution.target_set_version, can_offer: true, items: [resourceOfferFixture], next_cursor: null,
};
