-- These readers grant no source authority. Runtime owners already hold the
-- ordered source locks before treating a positive read as a disclosure proof.

-- name: GetKnowledgeSourceState :one
SELECT * FROM issue_collaboration WHERE issue_id=$1;

-- Metadata-only pre-lock discovery never returns accepted result/basis prose.
-- name: KnowledgeDeliverySourcePotentiallyAvailable :one
SELECT EXISTS (
    SELECT 1 FROM issue_handoff h
    JOIN collaboration_event d ON d.issue_id=h.issue_id AND d.kind='acceptance_submitted'
      AND d.event_seq=sqlc.arg(delivery_event_seq) AND d.payload->>'handoff_id'=h.id::text
    JOIN collaboration_event a ON a.issue_id=h.issue_id AND a.kind='completed'
      AND a.event_seq=sqlc.arg(accepted_event_seq) AND a.payload->'delivery_decision'->>'submission_id'=h.id::text
      AND a.payload->'delivery_decision'->>'kind'='accepted'
    WHERE h.issue_id=sqlc.arg(issue_id) AND h.id=sqlc.arg(submission_id) AND h.purpose='acceptance'
)::boolean;

-- name: LockKnowledgeDeliverySources :many
SELECT id FROM issue_handoff WHERE issue_id=$1 AND id=ANY(sqlc.arg(ids)::uuid[])
ORDER BY id FOR SHARE NOWAIT;

-- name: LockKnowledgeDeliveryEvents :many
SELECT event_seq FROM collaboration_event WHERE issue_id=$1 AND event_seq=ANY(sqlc.arg(event_seqs)::bigint[])
ORDER BY event_seq FOR SHARE NOWAIT;

-- Readers share the exact validity head; revalidation writers take the same
-- target/source/candidate order before changing it.
-- name: LockKnowledgeValiditiesForRead :many
SELECT * FROM collaboration_knowledge_validity WHERE candidate_id=ANY(sqlc.arg(candidate_ids)::uuid[])
ORDER BY candidate_id FOR SHARE NOWAIT;
