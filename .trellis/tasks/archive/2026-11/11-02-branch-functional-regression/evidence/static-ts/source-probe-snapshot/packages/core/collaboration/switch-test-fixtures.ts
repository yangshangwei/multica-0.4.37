import type { CollaborationSwitchInput, CollaborationSwitchProposal, CollaborationSwitchReceipt } from "./switch-schema";
import { collaborationExecutionFixture } from "./test-fixtures";

export const switchIssueId = collaborationExecutionFixture.issue_id;
export const switchTargetId = collaborationExecutionFixture.targets[0]!.id;
export const switchProposalId = "00000000-0000-4000-8000-000000000071";
export const switchRequestId = "00000000-0000-4000-8000-000000000072";
export const switchOtherId = "00000000-0000-4000-8000-000000000073";
export const switchResource = {
  role: "agent" as const, agent_id: collaborationExecutionFixture.targets[0]!.resource_id,
  runtime_id: switchOtherId, execution_workspace_id: switchIssueId, owner_user_id: switchIssueId,
};
const authority = { expected_lifecycle_epoch: "1", expected_participant_epoch: "2", expected_account_epoch: "9007199254740993" };
export const switchProposeInput = {
  ...authority, action: "propose_target_switch", target_id: switchTargetId,
  expected_target_epoch: "9007199254740993", expected_config_version: "9007199254740993", roster: [switchResource],
} satisfies CollaborationSwitchInput;
export const switchConsentInput = {
  ...authority, action: "consent_target_switch", target_id: switchTargetId, proposal_id: switchProposalId,
  agent_id: switchResource.agent_id,
  identity: { kind: "managed", installation_id: switchIssueId, binding_id: switchOtherId, binding_epoch: "9007199254740999" },
} satisfies CollaborationSwitchInput;
export const switchRevokeInput = {
  ...authority, action: "revoke_target_switch_consent", target_id: switchTargetId, proposal_id: switchProposalId, agent_id: switchResource.agent_id,
} satisfies CollaborationSwitchInput;
export const switchCommitInput = {
  ...authority, action: "commit_target_switch", target_id: switchTargetId, proposal_id: switchProposalId,
  control: { target_id: switchTargetId, expected_target_epoch: "9007199254740993", expected_config_version: "9007199254740993",
    expected_hold_version: "9007199254740993", expected_control_version: "9007199254740995",
    observed_group_id: collaborationExecutionFixture.targets[0]!.active_group_id, observed_group_generation: "1" },
} satisfies CollaborationSwitchInput;
export const switchProposal: CollaborationSwitchProposal = {
  proposal_id: switchProposalId, target_id: switchTargetId, proposed_by: switchIssueId,
  lifecycle_epoch: "1", base_target_epoch: "9007199254740993", base_config_version: "9007199254740993",
  proposed_target_epoch: "9007199254740994", proposed_config_version: "9007199254740994",
  status: "consent_ready", reason: "", resources: [{ ...switchResource, consented: true, revoked: false, grant_id: null, grant_epoch: null }],
};
export const switchReceipt: CollaborationSwitchReceipt = {
  protocol_version: 2, issue_id: switchIssueId, request_id: switchRequestId, action: "commit_target_switch", status: 200, code: "target_switched",
  scope: "target", target_id: switchTargetId, proposal_id: switchProposalId, channel_id: null, queue_state: "paused",
  control_version: "9007199254740995", target_set_version: "9007199254740999",
  grant_id: null, batch_id: null, command_id: null, group_id: null, group_settled: false, run_id: null, run_settled: false, channels: [], target_holds: [],
};
