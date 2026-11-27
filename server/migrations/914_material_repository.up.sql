CREATE TABLE project_material_repository (
 id uuid NOT NULL, workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 version bigint NOT NULL DEFAULT 1, generation bigint NOT NULL DEFAULT 1,
 config_version bigint NOT NULL, binding_digest text NOT NULL, payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE material_repository_revision (
 id uuid NOT NULL, repository_id uuid NOT NULL, workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 generation bigint NOT NULL, payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE material_repository_decision (
 id uuid NOT NULL, repository_id uuid NOT NULL, revision_id uuid NOT NULL, workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 actor_id uuid NOT NULL, member_id uuid NOT NULL, credential jsonb NOT NULL, payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE git_material_job ALTER COLUMN material_id DROP NOT NULL, ALTER COLUMN revision_id DROP NOT NULL;
ALTER TABLE git_material_job ADD COLUMN target_kind text NOT NULL DEFAULT 'material_revision', ADD COLUMN repository_id uuid, ADD COLUMN repository_revision_id uuid;
ALTER TABLE git_material_operation ALTER COLUMN material_id DROP NOT NULL, ALTER COLUMN revision_id DROP NOT NULL;
ALTER TABLE git_material_operation ADD COLUMN target_kind text NOT NULL DEFAULT 'material_revision', ADD COLUMN repository_id uuid, ADD COLUMN repository_revision_id uuid;
