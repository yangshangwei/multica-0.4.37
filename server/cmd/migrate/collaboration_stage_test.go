package main

import (
	"slices"
	"testing"
)

var collaborationStageVersions = []string{
	"805_collaboration_stage", "806_project_stage_policy_revision", "807_project_stage_policy_request", "808_collaboration_stage_task_issue",
	"809_collaboration_stage_role_revision", "810_collaboration_stage_candidate_id", "811_collaboration_stage_candidate_version",
	"812_collaboration_stage_decision_id", "813_collaboration_stage_decision_issue", "814_collaboration_stage_decision_candidate",
	"815_collaboration_stage_decision_accept", "816_collaboration_stage_child_issue", "817_collaboration_stage_child_root",
	"818_collaboration_stage_child_materialization", "819_collaboration_stage_ready", "820_collaboration_stage_child_code",
}

// The actual runner applies the actual stage files: upgrade, ledger replay,
// retained-history refusal that keeps the ledger and every index, empty
// rollback, and re-upgrade. internal/migrations owns the constraint matrix.
func TestCollaborationStageMigrationsRunnerUpgradeRefusalRollbackAndReupgrade(t *testing.T) {
	ctx, fx, schema, opts := switchMigrationFixture(t)
	opts.Files = realMigrationFiles(t, collaborationStageVersions, "up")
	down := opts
	reverse := slices.Clone(collaborationStageVersions)
	slices.Reverse(reverse)
	down.Direction, down.Hooks, down.Files = "down", hooksForDirection("down"), realMigrationFiles(t, reverse, "down")
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != len(collaborationStageVersions) ||
		fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_class r ON r.oid=i.indrelid
 WHERE c.relnamespace=current_schema()::regnamespace AND r.relname LIKE '%stage%' AND i.indisvalid AND i.indisready AND i.indislive`) != 13 {
		t.Fatal("upgrade did not record every version or build every stage index")
	}
	before := iterationRecoveryCatalog(t, ctx, fx.Pool, schema)
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if after := iterationRecoveryCatalog(t, ctx, fx.Pool, schema); after != before {
		t.Fatal("upgrade replay changed the catalog or ledger")
	}
	fx.Exec(t, `INSERT INTO project_stage_policy (workspace_id,project_id,revision,contract,enabled,reason,created_by,request_id,request_digest)
 VALUES (gen_random_uuid(),gen_random_uuid(),1,'collaboration-stage-v1',true,'retained decision',gen_random_uuid(),'key',repeat('a',64))`)
	artifactPGCode(t, runMigrations(ctx, fx.Pool, down), "P0001")
	if after := iterationRecoveryCatalog(t, ctx, fx.Pool, schema); after != before || fx.Count(t, `SELECT count(*) FROM project_stage_policy`) != 1 {
		t.Fatal("a refused rollback removed a ledger row, an index or retained history")
	}
	fx.Exec(t, `DELETE FROM project_stage_policy`)
	if err := runMigrations(ctx, fx.Pool, down); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != 0 || fx.Count(t, `SELECT count(*) FROM pg_class WHERE oid=to_regclass('collaboration_stage_child')`) != 0 ||
		fx.Count(t, `SELECT count(*) FROM pg_proc WHERE pronamespace=current_schema()::regnamespace AND proname='collaboration_stage_immutable'`) != 0 {
		t.Fatal("empty rollback retained stage objects or ledger rows")
	}
	if err := runMigrations(ctx, fx.Pool, opts); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM schema_migrations`) != len(collaborationStageVersions) {
		t.Fatal("re-upgrade did not record every version")
	}
}
