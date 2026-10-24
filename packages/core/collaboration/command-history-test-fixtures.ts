export const collaborationCommandHistoryFixture = {
  protocol_version: 1,
  issue_id: "00000000-0000-4000-8000-000000000001",
  items: [{
    id: "00000000-0000-4000-8000-000000000011",
    session_id: "00000000-0000-4000-8000-000000000002",
    target_id: "00000000-0000-4000-8000-000000000003",
    target: { kind: "squad", resource_id: "00000000-0000-4000-8000-000000000004" },
    input_event_id: "00000000-0000-4000-8000-000000000005",
    input_event_seq: "9007199254740993",
    comment_id: "00000000-0000-4000-8000-000000000006",
    channel_id: "00000000-0000-4000-8000-000000000007",
    batch_id: "00000000-0000-4000-8000-000000000008",
    group_id: null,
    seq: "9007199254740993",
    author_user_id: "00000000-0000-4000-8000-000000000009",
    author: { user_id: "00000000-0000-4000-8000-000000000009", name: "Original author", avatar_url: "", external: true },
    content: "Original instruction before the comment was edited.",
    attachment_ids: ["00000000-0000-4000-8000-00000000000a"],
    state: "cancelled",
    reason: "cancelled_by_participant",
    created_at: "2026-10-21T08:00:00.123456Z",
    cancellation: {
      event_id: "00000000-0000-4000-8000-00000000000b",
      event_seq: "9007199254740994",
      actor_user_id: "00000000-0000-4000-8000-000000000001",
      actor: { user_id: "00000000-0000-4000-8000-000000000001", name: "Cancelling participant", avatar_url: "", external: false },
      created_at: "2026-10-21T08:05:00.654321Z",
      kind: "manual",
    },
  }],
  next_cursor: null,
};

export const collaborationUnknownCancellationFixture = {
  event_id: null, event_seq: null, actor_user_id: null, actor: null, created_at: null, kind: null,
};
