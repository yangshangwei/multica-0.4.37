package migrations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

var configInvalidationMigrations = []string{
	"704_collaboration_config_invalidation",
	"705_collaboration_config_invalidation_id",
	"706_collaboration_config_invalidation_resource",
	"707_collaboration_config_invalidation_poll",
}

func configInvalidationMigrationSQL(t *testing.T, name, direction string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name+"."+direction+".sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func newConfigInvalidationMigrationFixture(t *testing.T) *testutil.Fixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	base, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close(ctx) })
	schema := "collaboration_config_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE") })
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, uuid.NewString(), uuid.NewString())
	for _, table := range []string{"agent", "agent_runtime", "squad", "agent_invocation_target", "squad_member", "agent_skill", "collaboration_target", "collaboration_grant", "collaboration_execution_group", "collaboration_run"} {
		fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE public."+pgx.Identifier{table}.Sanitize()+" INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES)")
	}
	// Rehearse from the actual pre-outbox epoch function and triggers. LIKE
	// does not copy triggers, and applying all of 660 would duplicate columns.
	old := configInvalidationMigrationSQL(t, "660_collaboration_resource_consent", "up")
	start, end := strings.Index(old, "CREATE FUNCTION collaboration_advance_resource_config()"), strings.Index(old, "-- Existing foundation grants")
	if start < 0 || end <= start {
		t.Fatal("missing previous resource epoch contract")
	}
	fx.Exec(t, old[start:end])
	return fx
}

func TestCollaborationConfigurationMigrationPreservesAndResumesInvalidation(t *testing.T) {
	fx := newConfigInvalidationMigrationFixture(t)
	ctx := t.Context()
	runtimeID := fx.Runtime(t, "Configuration migration runtime")
	agentID := fx.Agent(t, "Configuration migration agent", runtimeID)
	squadID := fx.Squad(t, "Configuration migration squad", agentID)
	roster, err := json.Marshal([]map[string]string{{"role": "leader", "agent_id": agentID, "runtime_id": runtimeID, "owner_user_id": fx.UserID, "execution_workspace_id": fx.WorkspaceID}})
	if err != nil {
		t.Fatal(err)
	}
	targetID := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": uuid.NewString(), "kind": "squad", "resource_id": squadID, "roster": roster})
	fx.Exec(t, `UPDATE agent SET instructions='changed before upgrade' WHERE id=$1`, agentID)
	var before, after string
	fx.QueryRow(t, `SELECT to_jsonb(t)::text FROM collaboration_target t WHERE id=$1`, targetID).Scan(&before)
	fx.Exec(t, configInvalidationMigrationSQL(t, configInvalidationMigrations[0], "up"))
	fx.QueryRow(t, `SELECT to_jsonb(t)::text FROM collaboration_target t WHERE id=$1`, targetID).Scan(&after)
	if before != after || fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation`) != 3 ||
		fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_kind='agent' AND resource_id=$1 AND config_epoch=2 AND reconciled_at IS NULL`, agentID) != 1 {
		t.Fatal("upgrade changed source intent or omitted the pre-upgrade resource scan")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='collaboration_config_invalidation'::regclass`) != 0 {
		t.Fatal("configuration schema created an inline primary/unique index")
	}
	for _, name := range configInvalidationMigrations[1:] {
		fx.Exec(t, configInvalidationMigrationSQL(t, name, "up"))
	}
	if fx.Count(t, `SELECT count(*) FROM pg_index WHERE indrelid='collaboration_config_invalidation'::regclass AND indisvalid`) != 3 ||
		fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid='collaboration_config_invalidation'::regclass AND contype='f'`) != 0 {
		t.Fatal("configuration outbox indexes are invalid or added foreign keys")
	}
	fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(i) ORDER BY id)::text FROM collaboration_config_invalidation i`).Scan(&before)
	for _, name := range configInvalidationMigrations {
		fx.Exec(t, configInvalidationMigrationSQL(t, name, "up"))
	}
	fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(i) ORDER BY id)::text FROM collaboration_config_invalidation i`).Scan(&after)
	if before != after {
		t.Fatal("migration replay replaced original durable invalidation identities")
	}
	if _, err = fx.Pool.Exec(ctx, `INSERT INTO collaboration_config_invalidation SELECT * FROM collaboration_config_invalidation LIMIT 1`); err == nil {
		t.Fatal("concurrent unique index did not protect durable intent identity")
	}

	fx.Exec(t, `UPDATE agent SET name='display name',description='display description',status='offline',max_concurrent_tasks=2 WHERE id=$1`, agentID)
	fx.Exec(t, `UPDATE agent_runtime SET custom_name='display name',status='offline',last_seen_at=now() WHERE id=$1`, runtimeID)
	fx.Exec(t, `UPDATE squad SET name='display name',description='display description' WHERE id=$1`, squadID)
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation`) != 3 {
		t.Fatal("display, availability or capacity changes invalidated execution authority")
	}
	fx.Exec(t, `UPDATE agent SET instructions='new execution configuration' WHERE id=$1`, agentID)
	fx.Exec(t, `UPDATE agent_runtime SET visibility='public' WHERE id=$1`, runtimeID)
	fx.Exec(t, `UPDATE squad SET instructions='new routing policy' WHERE id=$1`, squadID)
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation`) != 6 {
		t.Fatal("resource configuration writes did not record their epoch changes")
	}
	fx.Exec(t, `UPDATE collaboration_config_invalidation SET reconciled_at=now()`)
	for _, name := range configInvalidationMigrations {
		if _, err = fx.Pool.Exec(ctx, configInvalidationMigrationSQL(t, name, "down")); err == nil || !strings.Contains(err.Error(), "retained collaboration configuration invalidation") {
			t.Fatalf("%s discarded a processed durable intent: %v", name, err)
		}
	}
	fx.Exec(t, `DELETE FROM collaboration_config_invalidation`)
	holder, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err = holder.Exec(ctx, `UPDATE agent SET instructions='uncommitted writer' WHERE id=$1`, agentID); err != nil {
		t.Fatal(err)
	}
	if _, err = fx.Pool.Exec(ctx, configInvalidationMigrationSQL(t, configInvalidationMigrations[0], "down")); err == nil || !strings.Contains(err.Error(), "55P03") {
		t.Fatalf("rollback waited past an uncommitted resource invalidation: %v", err)
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for i := len(configInvalidationMigrations) - 1; i >= 0; i-- {
		fx.Exec(t, configInvalidationMigrationSQL(t, configInvalidationMigrations[i], "down"))
	}
	if fx.Count(t, `SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace AND relname='collaboration_config_invalidation'`) != 0 {
		t.Fatal("empty maintenance rollback left the outbox behind")
	}
}

func TestCollaborationConfigurationMigrationChildrenAndDeletion(t *testing.T) {
	fx := newConfigInvalidationMigrationFixture(t)
	for _, name := range configInvalidationMigrations {
		fx.Exec(t, configInvalidationMigrationSQL(t, name, "up"))
	}
	runtimeID := fx.Runtime(t, "Child configuration runtime")
	agentID := fx.Agent(t, "Child configuration agent", runtimeID)
	squadID := fx.Squad(t, "Child configuration squad", agentID)
	invoke := fx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": agentID, "target_type": "member", "target_id": fx.UserID})
	member := fx.SquadMember(t, squadID, "agent", agentID, testutil.Cols{"role": "leader"})
	skill := uuid.NewString()
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agentID, "skill_id": skill}, "agent_id=$1 AND skill_id=$2", agentID, skill)
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation`) != 3 {
		t.Fatal("child creation did not advance each owning resource")
	}
	fx.Exec(t, `UPDATE agent_invocation_target SET target_type='member',target_id=$2 WHERE id=$1`, invoke, fx.UserID)
	fx.Exec(t, `UPDATE squad_member SET created_at=now(),role='leader' WHERE id=$1`, member)
	fx.Exec(t, `UPDATE agent_skill SET enabled=true,created_at=now() WHERE agent_id=$1 AND skill_id=$2`, agentID, skill)
	fx.SquadMember(t, squadID, "member", fx.UserID)
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation`) != 3 {
		t.Fatal("incidental child metadata or human membership invalidated execution")
	}
	// The frozen invocation fingerprint includes its original provenance.
	// Changing it must schedule convergence just like changing its recipient.
	fx.Exec(t, `UPDATE agent_invocation_target SET created_by=$2,created_at=now() WHERE id=$1`, invoke, uuid.NewString())
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation`) != 4 {
		t.Fatal("changed invocation provenance invalidated the fingerprint without durable convergence")
	}
	// The squad fingerprint retains agent-member row identity, too.
	replacementMember := uuid.NewString()
	fx.Exec(t, `UPDATE squad_member SET id=$2 WHERE id=$1`, member, replacementMember)
	member = replacementMember
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation`) != 5 {
		t.Fatal("changed squad membership identity lacked durable convergence")
	}
	fx.Exec(t, `UPDATE squad_member SET role='changed execution role' WHERE id=$1`, member)
	fx.Exec(t, `UPDATE agent_skill SET enabled=false WHERE agent_id=$1 AND skill_id=$2`, agentID, skill)
	fx.Exec(t, `DELETE FROM agent_invocation_target WHERE id=$1`, invoke)
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation`) != 8 {
		t.Fatal("changed execution role, skill state or invoke membership lacked durable work")
	}
	fx.Exec(t, `DELETE FROM agent WHERE id=$1`, agentID)
	fx.Exec(t, `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
	fx.Exec(t, `DELETE FROM squad WHERE id=$1`, squadID)
	if fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE reason='resource_deleted'`) != 3 {
		t.Fatal("resource deletion erased the last invalidation identity")
	}
}
