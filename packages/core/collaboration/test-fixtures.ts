const id = "00000000-0000-4000-8000-000000000001";
const capabilities = {
  reference: true, read: true, comment: true, control_ai: true,
  submit_command: true, invite: true, transfer: true, remove_participants: true,
  leave: false, edit: true, edit_source_structure: false, reorder: false,
  change_status: true, complete: true, cancel: true, reopen: false,
  resolve_thread: true, react: true,
};
export const collaborationIssueFixture = {
  id, title: "Collaboration", description: "Private content", status: "todo",
  status_category: "todo", priority: "none",
  source_workspace: { id, name: "Source", identifier: "SRC-1" },
  creator: { user_id: id, name: "Creator", avatar_url: "", external: true },
  handler: { user_id: id, name: "Handler", avatar_url: "", external: true },
  collaboration_revision: "9007199254740993", lifecycle_epoch: "1",
  session: { id, queue_state: "paused", control_version: "9007199254740995" },
  capabilities,
};

const executionId = (value: number) => `00000000-0000-4000-8000-${value.toString(16).padStart(12, "0")}`;
const targetVersion = "9007199254740993";
const agentTargetId = executionId(10);
const squadTargetId = executionId(20);
const agentCommandId = executionId(11);
const squadCommandId = executionId(21);
const agentGroupId = executionId(12);
const squadGroupId = executionId(22);
const leaderRunId = executionId(23);
const agentChannelId = executionId(60);
const squadChannelId = executionId(61);
const agentBatchId = executionId(62);
const waitingBatchId = executionId(63);
const squadBatchId = executionId(64);
const settledAt = "2026-10-17T00:00:00Z";

/** A complete read projection; runtime availability is separate from role rights. */
export const collaborationExecutionFixture = {
  protocol_version: 3,
  issue_id: id,
  session_id: id,
  availability: "unavailable",
  unavailable_reason: "shared_runtime_unavailable",
  target_set_version: "9007199254740999",
  control: { scope: "issue", queue_state: "paused", control_version: collaborationIssueFixture.session.control_version },
  targets: [
    { id: agentTargetId, channel_id: agentChannelId, kind: "agent", resource_id: executionId(30), target_epoch: targetVersion, config_version: targetVersion,
      control: { scope: "target", queue_state: "paused", control_version: "9007199254740995" },
      group_generation: "1", next_command_seq: "3", active_group_id: agentGroupId, pause_reason: "stop_requested", revoked_at: null },
    { id: squadTargetId, channel_id: squadChannelId, kind: "squad", resource_id: executionId(40), target_epoch: targetVersion, config_version: targetVersion,
      control: { scope: "target", queue_state: "paused", control_version: "2" },
      group_generation: "1", next_command_seq: "2", active_group_id: squadGroupId, pause_reason: "stop_requested", revoked_at: null },
  ],
  commands: [
    { id: agentCommandId, target_id: agentTargetId, channel_id: agentChannelId, batch_id: agentBatchId, input_event_id: executionId(50), sequence: "1", author_user_id: id, state: "in_flight" },
    { id: executionId(14), target_id: agentTargetId, channel_id: agentChannelId, batch_id: waitingBatchId, input_event_id: executionId(51), sequence: "2", author_user_id: id, state: "waiting" },
    { id: squadCommandId, target_id: squadTargetId, channel_id: squadChannelId, batch_id: squadBatchId, input_event_id: executionId(50), sequence: "1", author_user_id: id, state: "in_flight" },
  ],
  groups: [
    { id: agentGroupId, target_id: agentTargetId, command_id: agentCommandId, channel_id: agentChannelId, batch_id: agentBatchId, generation: "1", target_epoch: targetVersion, config_version: targetVersion,
      state: "ending", expansion_sealed_at: settledAt, forced_disposition: "stopped", settled_at: null },
    { id: squadGroupId, target_id: squadTargetId, command_id: squadCommandId, channel_id: squadChannelId, batch_id: squadBatchId, generation: "1", target_epoch: targetVersion, config_version: targetVersion,
      state: "ending", expansion_sealed_at: settledAt, forced_disposition: "stopped", settled_at: null },
  ],
  runs: [
    { id: executionId(13), command_id: agentCommandId, target_id: agentTargetId, channel_id: agentChannelId, batch_id: agentBatchId, group_id: agentGroupId, parent_run_id: null,
      step_key: "root", step_kind: "agent", agent_id: executionId(30), target_epoch: targetVersion, config_version: targetVersion,
      progress_state: "ending", proof_state: "unknown", business_outcome: null, forced_disposition: "stopped", recording_incomplete: false, settled_at: null },
    { id: leaderRunId, command_id: squadCommandId, target_id: squadTargetId, channel_id: squadChannelId, batch_id: squadBatchId, group_id: squadGroupId, parent_run_id: null,
      step_key: "root", step_kind: "leader", agent_id: executionId(32), target_epoch: targetVersion, config_version: targetVersion,
      progress_state: "settled", proof_state: "exited_proven", business_outcome: "success", forced_disposition: null, recording_incomplete: false, settled_at: settledAt },
    { id: executionId(24), command_id: squadCommandId, target_id: squadTargetId, channel_id: squadChannelId, batch_id: squadBatchId, group_id: squadGroupId, parent_run_id: leaderRunId,
      step_key: "member-1", step_kind: "member", agent_id: executionId(31), target_epoch: targetVersion, config_version: targetVersion,
      progress_state: "ending", proof_state: "unknown", business_outcome: null, forced_disposition: "stopped", recording_incomplete: true, settled_at: null },
  ],
};

export const collaborationExecutionIssueFixture = { ...collaborationIssueFixture, execution: collaborationExecutionFixture };

export const collaborationFieldOptionsFixture = {
  issue_id: id,
  source_workspace_id: id,
  collaboration_revision: collaborationIssueFixture.collaboration_revision,
  labels: [{ id: "00000000-0000-4000-8000-000000000002", name: "Bug", color: "#112233" }],
  properties: [{
    id: "00000000-0000-4000-8000-000000000003",
    name: "Impact", type: "select", icon: "", archived: false,
    config: { options: [{ id: "option-one", name: "First", color: "#445566" }] },
  }],
};
