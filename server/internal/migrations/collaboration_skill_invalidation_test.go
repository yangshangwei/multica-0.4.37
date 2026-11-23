package migrations

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
)

const skillContentMigration = "709_collaboration_skill_content_invalidation"

func TestCollaborationSharedSkillMigrationUpgradeAndSemanticWrites(t *testing.T) {
	fx := newConfigInvalidationMigrationFixture(t)
	for _, table := range []string{"skill", "skill_file", "plugin_installation"} {
		fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE public."+pgx.Identifier{table}.Sanitize()+" INCLUDING ALL)")
	}
	for _, name := range configInvalidationMigrations {
		fx.Exec(t, configInvalidationMigrationSQL(t, name, "up"))
	}
	runtime := fx.Runtime(t, "Skill migration runtime")
	agent := fx.Agent(t, "Skill migration agent", runtime)
	disabled := fx.Agent(t, "Disabled skill binding", runtime)
	skill := fx.Insert(t, "skill", testutil.Cols{"workspace_id": fx.WorkspaceID, "name": "shared", "content": "old content"})
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agent, "skill_id": skill}, "agent_id=$1 AND skill_id=$2", agent, skill)
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": disabled, "skill_id": skill, "enabled": false}, "agent_id=$1 AND skill_id=$2", disabled, skill)
	fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": uuid.NewString(), "kind": "agent", "resource_id": agent, "roster": `[{"agent_id":"` + agent + `"}]`})
	file := fx.Insert(t, "skill_file", testutil.Cols{"skill_id": skill, "path": "guide.md", "content": "old file"})
	fx.Exec(t, `UPDATE skill SET content='untracked pre-upgrade content' WHERE id=$1`, skill)
	count := fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, agent)
	fx.Exec(t, configInvalidationMigrationSQL(t, skillContentMigration, "up"))
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, agent) != count+1 {
		t.Fatal("upgrade trusted untracked pre-upgrade content instead of retiring old consent")
	}
	count++
	fx.Exec(t, configInvalidationMigrationSQL(t, skillContentMigration, "up"))
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, agent) != count {
		t.Fatal("upgrade replay advanced the resource again")
	}
	if _, err := fx.Pool.Exec(t.Context(), configInvalidationMigrationSQL(t, skillContentMigration, "down")); err == nil || !strings.Contains(err.Error(), "cannot be downgraded") {
		t.Fatalf("migration allowed old execution consent semantics: %v", err)
	}
	disabledCount := fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, disabled)
	for _, statement := range []string{
		`UPDATE skill SET content=content,updated_at=now(),config='{"presentation":{"icon":"code"}}' WHERE id=$1`,
		`UPDATE skill_file SET content=content,updated_at=now() WHERE skill_id=$1`,
		`INSERT INTO skill_file(skill_id,path,content) VALUES($1,'guide.md','old file') ON CONFLICT(skill_id,path) DO UPDATE SET content=EXCLUDED.content,updated_at=now()`,
		`UPDATE agent_skill SET created_at=now() WHERE skill_id=$1`,
		`DELETE FROM agent_skill WHERE skill_id=$1 AND enabled=false`,
	} {
		fx.Exec(t, statement, skill)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, agent) != count || fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, disabled) != disabledCount {
		t.Fatal("display-only/no-op writes or deleting a disabled binding invalidated consent")
	}
	for _, statement := range []string{
		`UPDATE skill SET name='runtime bundle name' WHERE id=$1`,
		`UPDATE skill SET description='runtime bundle description' WHERE id=$1`,
		`UPDATE skill SET content='new content' WHERE id=$1`,
		`UPDATE skill SET config='{"origin":{"version":"2"}}' WHERE id=$1`,
		`UPDATE skill_file SET content='new file' WHERE skill_id=$1`,
		`UPDATE skill_file SET path='renamed.md' WHERE skill_id=$1`,
		`DELETE FROM skill_file WHERE skill_id=$1`,
	} {
		fx.Exec(t, statement, skill)
		count++
		if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, agent) != count {
			t.Fatalf("semantic write did not atomically record exactly one dependent epoch: %s", statement)
		}
	}
	// Rollback removes both the new content and its durable invalidation.
	tx, err := fx.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(t.Context(), `UPDATE skill SET content='rolled back' WHERE id=$1`, skill); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, agent) != count || fx.Count(t, `SELECT count(*) FROM skill WHERE id=$1 AND content='new content'`, skill) != 1 {
		t.Fatal("rolled-back skill writer left content or invalidation behind")
	}
	fx.Exec(t, `DELETE FROM skill WHERE id=$1`, skill)
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, agent) != count+1 || fx.Count(t, `SELECT count(*) FROM skill_file WHERE id=$1`, file) != 0 {
		t.Fatal("skill deletion lost its last enabled-binding invalidation")
	}
}
