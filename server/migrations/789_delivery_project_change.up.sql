CREATE TABLE delivery_project_change (
 workspace_id uuid NOT NULL, operation_id uuid NOT NULL, issue_id uuid NOT NULL,
 issue_revision bigint NOT NULL CHECK(issue_revision>=0),
 actor_kind text NOT NULL CHECK(actor_kind IN ('human','agent')),
 actor_id uuid NOT NULL, actor_user_id uuid NOT NULL,
 from_project_id uuid, to_project_id uuid,
 from_project_revision bigint NOT NULL CHECK(from_project_revision>=0),
 to_project_revision bigint NOT NULL CHECK(to_project_revision>=0),
 from_policy_revision bigint NOT NULL CHECK(from_policy_revision>=0),
 to_policy_revision bigint NOT NULL CHECK(to_policy_revision>=0),
 reason text NOT NULL CHECK(char_length(reason)<=1000),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK(from_project_id IS DISTINCT FROM to_project_id)
);
