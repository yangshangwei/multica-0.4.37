CREATE TABLE project_material_settings (
    project_id uuid NOT NULL, workspace_id uuid NOT NULL, version bigint NOT NULL,
    settings jsonb NOT NULL, authorized_by uuid NOT NULL, member_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE git_material (
    id uuid NOT NULL, workspace_id uuid NOT NULL, project_id uuid NOT NULL,
    title text NOT NULL, path text NOT NULL, version bigint NOT NULL DEFAULT 1,
    status text NOT NULL DEFAULT 'draft', blocker text,
    current_revision_id uuid, baseline jsonb,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE git_material_revision (
    id uuid NOT NULL, material_id uuid NOT NULL, workspace_id uuid NOT NULL, project_id uuid NOT NULL,
    kind text NOT NULL, payload jsonb NOT NULL, settings jsonb NOT NULL,
    config_version bigint NOT NULL, created_by uuid NOT NULL,
    publication_status text NOT NULL, review jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE git_material_decision (
    id uuid NOT NULL, material_id uuid NOT NULL, revision_id uuid NOT NULL,
    workspace_id uuid NOT NULL, project_id uuid NOT NULL, actor_id uuid NOT NULL,
    member_id uuid NOT NULL, credential jsonb NOT NULL,
    decision text NOT NULL, reason text NOT NULL, head_sha text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE git_material_comment (
    id uuid NOT NULL, material_id uuid NOT NULL, revision_id uuid NOT NULL,
    workspace_id uuid NOT NULL, project_id uuid NOT NULL, actor_id uuid NOT NULL,
    body text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE git_material_request (
    workspace_id uuid NOT NULL, project_id uuid NOT NULL, actor_id uuid NOT NULL,
    request_id text NOT NULL, body_hash text NOT NULL, response jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE git_material_job (
    id uuid NOT NULL, workspace_id uuid NOT NULL, project_id uuid NOT NULL,
    material_id uuid NOT NULL, revision_id uuid NOT NULL, kind text NOT NULL,
    state text NOT NULL DEFAULT 'pending', attempts integer NOT NULL DEFAULT 0,
    lease_token uuid, leased_until timestamptz, next_at timestamptz NOT NULL DEFAULT now(),
    blocker text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE git_material_plan (
    material_id uuid NOT NULL, workspace_id uuid NOT NULL, project_id uuid NOT NULL,
    baseline_id uuid NOT NULL, issue_id uuid NOT NULL, status text NOT NULL,
    requested boolean NOT NULL, authorized_by uuid NOT NULL, member_id uuid NOT NULL,
    agent_id uuid, task_id uuid, revision_id uuid, blocker text,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
