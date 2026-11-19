-- These fences are only for artifact-bound read projections. Real writers
-- retain the original lifecycle, capability, approval and execution locks.

-- name: LockDeliveryEvidenceIssueRead :one
SELECT * FROM issue WHERE id = $1 AND workspace_id = $2
FOR SHARE NOWAIT;

-- name: LockDeliveryArtifactPrincipalsRead :one
SELECT t.id FROM agent_task_queue t
JOIN agent a ON a.id=t.agent_id
JOIN agent_runtime r ON r.id=t.runtime_id
JOIN issue i ON i.id=t.issue_id
JOIN member origin_member ON origin_member.user_id=t.originator_user_id AND origin_member.workspace_id=i.workspace_id
JOIN member owner_member ON owner_member.user_id=r.owner_id AND owner_member.workspace_id=i.workspace_id
JOIN "user" origin_user ON origin_user.id=origin_member.user_id
JOIN "user" owner_user ON owner_user.id=owner_member.user_id
WHERE t.id=$1
FOR SHARE OF i,t,a,r,origin_member,owner_member,origin_user,owner_user NOWAIT;

-- name: LockDeliveryArtifactApprovalRead :one
SELECT * FROM agent_approval_request WHERE id = $1 FOR SHARE NOWAIT;

-- name: LockDeliveryArtifactExecutionRead :one
SELECT * FROM agent_artifact_execution WHERE approval_id = $1 FOR SHARE NOWAIT;
