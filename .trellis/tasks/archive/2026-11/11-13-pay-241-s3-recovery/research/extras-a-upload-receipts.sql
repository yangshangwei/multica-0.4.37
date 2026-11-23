SELECT attachment.id AS attachment_id,
       attachment.uploader_user_id, attachment.draft_id, attachment.upload_request_id,
       attachment.storage_key, attachment.content_sha256, attachment.size_bytes,
       attachment.delivered_at IS NOT NULL AND attachment.discarded_at IS NULL AS delivered,
       operation.request_id AS comment_request_id, operation.result AS receipt,
       original_comment.id AS comment_id, original_event.id AS event_id,
       (SELECT jsonb_agg(jsonb_build_array(ref.reference_kind, ref.reference_id, ref.reference_revision)
                         ORDER BY ref.reference_kind, ref.reference_id, ref.reference_revision)
          FROM collaboration_attachment_reference AS ref
         WHERE ref.attachment_id = attachment.id AND ref.removed_at IS NULL) AS active_references
  FROM collaboration_attachment AS attachment
  LEFT JOIN collaboration_operation AS operation
    ON operation.scope_kind = 'issue' AND operation.scope_id = attachment.issue_id
   AND operation.actor_user_id = attachment.uploader_user_id AND operation.kind = 'create_comment'
   AND operation.completed_at IS NOT NULL AND operation.result->'attachment_ids' ? attachment.id::text
  LEFT JOIN comment AS original_comment
    ON original_comment.id::text = operation.result->>'comment_id' AND original_comment.issue_id = attachment.issue_id
   AND original_comment.author_id = operation.actor_user_id AND original_comment.revision::text = operation.result->>'comment_revision'
  LEFT JOIN collaboration_event AS original_event
    ON original_event.id::text = operation.result->>'event_id' AND original_event.issue_id = attachment.issue_id
   AND original_event.actor_user_id = operation.actor_user_id AND original_event.event_seq::text = operation.result->>'event_seq'
 WHERE attachment.intended_issue_id = $1::uuid
 ORDER BY attachment.id, operation.request_id;
