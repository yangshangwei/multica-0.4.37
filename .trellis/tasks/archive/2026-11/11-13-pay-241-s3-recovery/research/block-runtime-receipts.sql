SELECT h.issue_id, h.id AS handoff_id, h.purpose, h.actor_user_id,
       h.from_user_id, h.to_user_id, h.previous_status, h.resolution, h.note,
       h.parent_request_id, h.requester_user_id, h.superseded_by_repair_handoff_id,
       to_char(h.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS created_at,
       h.unblock_condition,
       to_char(h.estimated_resolution_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS estimated_resolution_at,
       e.id AS event_id, e.event_seq::text AS event_sequence, e.kind AS event_kind,
       e.actor_user_id AS event_actor, e.payload
  FROM issue_handoff AS h
  LEFT JOIN collaboration_event AS e ON e.issue_id = h.issue_id
   AND e.payload->>'handoff_id' = h.id::text
   AND e.kind IN ('information_requested', 'information_returned', 'information_repaired')
 WHERE h.issue_id = ANY($1::uuid[])
 ORDER BY h.issue_id, h.created_at, h.id;
