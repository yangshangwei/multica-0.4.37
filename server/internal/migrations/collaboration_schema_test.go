package migrations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationSchemaRetainsIdentityAndExecutionBarriers(t *testing.T) {
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
	schema := "collaboration_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE") })
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	fx.Exec(t, "CREATE TABLE comment (id uuid, revision bigint NOT NULL DEFAULT 1)")
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	var applied []string
	for _, path := range files {
		name := filepath.Base(path)
		if name < "571_" || name >= "613_" {
			continue
		}
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		applied = append(applied, path)
	}
	if len(applied) < 3 {
		t.Fatal("collaboration schema and constraints were not applied")
	}
	if n := fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype='f'`); n != 0 {
		t.Fatalf("added %d database foreign keys", n)
	}
	if n := fx.Count(t, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND NOT i.indisvalid`); n != 0 {
		t.Fatalf("invalid indexes: %d", n)
	}
	if n := fx.Count(t, `SELECT count(*) FROM collaboration_execution_gate WHERE enabled OR creation_enabled`); n != 0 {
		t.Fatal("new deployment enabled incomplete collaboration")
	}
	// A retained row need not be committed yet: rollback must refuse the busy
	// table before its emptiness check can miss a concurrent first writer.
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `INSERT INTO collaboration_session (issue_id) VALUES ($1)`, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	downSQL, err := os.ReadFile(strings.TrimSuffix(applied[len(applied)-1], ".up.sql") + ".down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(downSQL)); err == nil || !strings.Contains(err.Error(), "55P03") {
		t.Fatalf("rollback did not refuse concurrent writer: %v", err)
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	issue := uuid.NewString()
	session := fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	if _, err := pool.Exec(ctx, `INSERT INTO collaboration_session (issue_id) VALUES ($1)`, issue); err == nil {
		t.Fatal("one issue acquired two permanent sessions")
	}
	command := uuid.NewString()
	cols := testutil.Cols{
		"session_id": session, "command_id": command, "grant_id": uuid.NewString(),
		"grant_epoch": 1, "resource_epoch": 1, "execution_workspace_id": uuid.NewString(),
		"runtime_id": uuid.NewString(), "installation_id": uuid.NewString(), "binding_id": uuid.NewString(), "binding_epoch": 1,
	}
	run := fx.Insert(t, "collaboration_run", cols)
	if _, err := pool.Exec(ctx, `UPDATE collaboration_run SET settled_at=now() WHERE id=$1`, run); err == nil {
		t.Fatal("unknown process state cleared the execution barrier")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO collaboration_run (session_id,command_id,grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch) SELECT session_id,gen_random_uuid(),grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch FROM collaboration_run WHERE id=$1`, run); err == nil {
		t.Fatal("session acquired two unsettled runs")
	}
	fx.Exec(t, `UPDATE collaboration_run SET proof_state='exited_proven', settled_at=now() WHERE id=$1`, run)
	cols["command_id"] = uuid.NewString()
	fx.Insert(t, "collaboration_run", cols)
	for _, path := range applied {
		down := strings.TrimSuffix(path, ".up.sql") + ".down.sql"
		sql, err := os.ReadFile(down)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err == nil || !strings.Contains(err.Error(), "retained collaboration state") {
			t.Fatalf("%s failed to preserve retained data: %v", filepath.Base(down), err)
		}
	}
	if n := fx.Count(t, `SELECT count(*) FROM collaboration_run`); n != 2 {
		t.Fatalf("rollback lost execution history: %d", n)
	}
	fx.Exec(t, `DELETE FROM collaboration_run WHERE session_id=$1`, session)
	fx.Exec(t, `DELETE FROM collaboration_session WHERE id=$1`, session)
	for i := len(applied) - 1; i >= 0; i-- {
		down := strings.TrimSuffix(applied[i], ".up.sql") + ".down.sql"
		sql, err := os.ReadFile(down)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("empty maintenance rollback %s: %v", filepath.Base(down), err)
		}
	}
	if n := fx.Count(t, `SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace AND relname LIKE 'collaboration_%'`); n != 0 {
		t.Fatalf("empty rollback left %d collaboration objects", n)
	}
}
