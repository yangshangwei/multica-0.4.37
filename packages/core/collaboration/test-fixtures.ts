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
