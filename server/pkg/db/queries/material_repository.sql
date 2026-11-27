-- name: GetMaterialRepository :one
SELECT * FROM project_material_repository WHERE project_id=$1 AND workspace_id=$2;
-- name: GetMaterialRepositoryByID :one
SELECT * FROM project_material_repository WHERE id=$1;
-- name: CreateMaterialRepository :one
INSERT INTO project_material_repository (id,workspace_id,project_id,config_version,binding_digest,payload) VALUES ($1,$2,$3,$4,$5,$6) RETURNING *;
-- name: SaveMaterialRepository :one
UPDATE project_material_repository SET version=version+1,generation=$2,config_version=$3,binding_digest=$4,payload=$5,updated_at=now() WHERE id=$1 RETURNING *;
-- name: CreateMaterialRepositoryRevision :one
INSERT INTO material_repository_revision (id,repository_id,workspace_id,project_id,generation,payload) VALUES ($1,$2,$3,$4,$5,$6) RETURNING *;
-- name: GetMaterialRepositoryRevision :one
SELECT * FROM material_repository_revision WHERE id=$1;
-- name: SaveMaterialRepositoryRevision :exec
UPDATE material_repository_revision SET payload=$2 WHERE id=$1;
-- name: ListMaterialRepositoryDecisions :many
SELECT * FROM material_repository_decision WHERE repository_id=$1 ORDER BY created_at,id;
-- name: CreateMaterialRepositoryDecision :one
INSERT INTO material_repository_decision (id,repository_id,revision_id,workspace_id,project_id,actor_id,member_id,credential,payload) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;
-- name: LatestMaterialRepositoryDecision :one
SELECT * FROM material_repository_decision WHERE revision_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1;
-- name: EnsureMaterialRepositoryJob :exec
INSERT INTO git_material_job (id,workspace_id,project_id,repository_id,repository_revision_id,target_kind,kind) VALUES ($1,$2,$3,$4,$5,'repository_initialization',$6)
ON CONFLICT (repository_revision_id,kind) DO UPDATE SET state='pending',next_at=now(),blocker=NULL
WHERE git_material_job.state IN ('done','blocked','dispatched');
-- name: CurrentMaterialRepositoryOperation :one
SELECT o.*,t.status AS task_status FROM git_material_operation o JOIN agent_task_queue t ON t.id=o.task_id WHERE o.repository_id=$1 ORDER BY o.created_at DESC LIMIT 1;
-- name: RecoverMaterialRepositoryOperation :execrows
UPDATE git_material_operation o SET state='recovery_requested' FROM agent_task_queue t WHERE o.task_id=t.id AND o.repository_id=$1 AND o.id=(SELECT id FROM git_material_operation WHERE repository_id=$1 ORDER BY created_at DESC LIMIT 1) AND o.state NOT IN ('succeeded','no_effect') AND t.status IN ('completed','failed','cancelled');
