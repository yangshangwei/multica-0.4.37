import { collaborationExecutionIssueFixture } from "./test-fixtures";

export const resourceIssue = { ...collaborationExecutionIssueFixture, write_context: { participant_epoch: "1", account_epoch: "1" } };
const target = resourceIssue.execution.targets[0]!;
export const targetResource = {
  role: "agent", agent_id: target.resource_id, runtime_id: "00000000-0000-4000-8000-000000000099",
  execution_workspace_id: resourceIssue.source_workspace.id, owner_user_id: resourceIssue.creator.user_id,
  agent_name: "Implementation agent", runtime: { id: "00000000-0000-4000-8000-000000000099", name: "Runtime", custom_name: "Development", provider: "codex" },
  owner: resourceIssue.creator, consented: false, grant_id: null, grant_epoch: null, can_consent: true, can_revoke: false,
};
export const resourcesFixture = {
  protocol_version: 1, issue_id: resourceIssue.id, lifecycle_epoch: resourceIssue.lifecycle_epoch,
  target_set_version: resourceIssue.execution.target_set_version, can_organize: true,
  targets: [{ target_id: target.id, kind: target.kind, resource_id: target.resource_id, name: "Implementation agent",
    target_epoch: target.target_epoch, config_version: target.config_version, authorized: false, reason: "grant_required", resources: [targetResource] }],
};
export const resourceOptionsFixture = { protocol_version: 1, issue_id: resourceIssue.id, execution_workspace_id: resourceIssue.source_workspace.id,
  items: [{ kind: "agent", resource_id: target.resource_id, name: "Implementation agent", available: true, reason: "", resources: [targetResource] }] };
export const resourceIdentityFixture = { issue_id: resourceIssue.id, target_id: target.id, agent_id: target.resource_id, proposal_id: null,
  identity: { kind: "enrolled", installation_id: "00000000-0000-4000-8000-000000000070", binding_id: "00000000-0000-4000-8000-000000000071", binding_epoch: "9007199254740993" }, reason: "" };
export const resourceReceiptFixture = { protocol_version: 2, issue_id: resourceIssue.id, request_id: "00000000-0000-4000-8000-000000000080",
  action: "consent_resource", status: 201, code: "resource_consented", scope: "target", target_id: target.id,
  grant_id: "00000000-0000-4000-8000-000000000081" };
