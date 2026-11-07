import { collaborationExecutionFixture } from "./test-fixtures";

export const collaborationRunRecordsFixture = {
  protocol_version: 1,
  issue_id: collaborationExecutionFixture.issue_id,
  run: collaborationExecutionFixture.runs[1]!,
  resource: {
    agent_id: collaborationExecutionFixture.runs[1]!.agent_id,
    runtime_id: "00000000-0000-4000-8000-000000000090",
    resource_owner_user_id: "00000000-0000-4000-8000-000000000091",
    execution_workspace_id: "00000000-0000-4000-8000-000000000092",
    grant_id: "00000000-0000-4000-8000-000000000093",
    provider: "recorded-provider", model: "recorded-model", account_source: "runtime_configuration",
  },
  recording: { status: "complete", inputs_recorded: true },
  usage: {
    input_tokens: "9007199254740993", output_tokens: "0", cache_read_tokens: null,
    cache_creation_tokens: null, total_tokens: "9223372036854775807", cost_usd: "1234567890123456789.1234567890",
  },
  items: [
    { id: "input-1", kind: "input", command_id: collaborationExecutionFixture.runs[1]!.command_id,
      author_user_id: collaborationExecutionFixture.issue_id, body: "Frozen input actually read", attachment_ids: [] },
    { id: "event-1:message-0", kind: "message", role: "assistant", body: "Recorded progress" },
    { id: "event-1:message-1", kind: "tool_call", call_id: "tool-1", name: "read", body: "Recorded tool call" },
    { id: "event-1:message-2", kind: "tool_result", call_id: "tool-1", body: "Recorded tool output", is_error: false },
    { id: "event-2", kind: "comment", comment_id: "00000000-0000-4000-8000-000000000094", body: "Original retained comment" },
    { id: "event-3", kind: "squad_activity", outcome: "action", body: "Recorded handoff" },
    { id: "event-4:result", kind: "result", outcome: "success", body: "Recorded final result" },
  ],
  next_cursor: null,
};
