package migrations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationRuntimePreparationMigrationPreservesAndFencesEvidence(t *testing.T) {
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
	schema := "collaboration_preparation_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
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
	read := func(direction string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", "737_collaboration_runtime_preparation."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	// Minimal old tables exercise the actual additive DDL and retention guards.
	// Full epoch/outbox co-existence is covered by the service registration test.
	fx.Exec(t, `CREATE TABLE agent_runtime (id uuid NOT NULL DEFAULT gen_random_uuid(), collaboration_config_epoch bigint NOT NULL DEFAULT 1)`)
	fx.Exec(t, `CREATE TABLE collaboration_run (id uuid NOT NULL DEFAULT gen_random_uuid(), prompt_snapshot jsonb,
		authorized_attempt_seq bigint NOT NULL DEFAULT 0, settled_at timestamptz, launch_nonce uuid NOT NULL DEFAULT gen_random_uuid(), proof_state text NOT NULL DEFAULT 'unknown')`)
	fx.Exec(t, `CREATE TABLE collaboration_run_event (id uuid NOT NULL DEFAULT gen_random_uuid(), kind text NOT NULL)`)
	runtimeID := fx.Insert(t, "agent_runtime", testutil.Cols{"id": uuid.NewString()})
	oldRun := fx.Insert(t, "collaboration_run", testutil.Cols{"prompt_snapshot": `[]`, "authorized_attempt_seq": 1, "proof_state": "exited_proven", "settled_at": testutil.Raw("now()")})
	var before, after string
	fx.QueryRow(t, `SELECT to_jsonb(r)::text FROM collaboration_run r WHERE id=$1`, oldRun).Scan(&before)
	fx.Exec(t, read("up"))
	fx.QueryRow(t, `SELECT (to_jsonb(r)-'preparation_snapshot')::text FROM collaboration_run r WHERE id=$1`, oldRun).Scan(&after)
	if before != after {
		t.Fatal("upgrade rewrote an old run's prompt, nonce or proof")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype='f'`) != 0 {
		t.Fatal("preparation introduced a foreign key")
	}
	if _, err = pool.Exec(ctx, `UPDATE collaboration_run SET preparation_snapshot='{}'::jsonb WHERE id=$1`, oldRun); err == nil {
		t.Fatal("preparation was injected into a previously claimed run")
	}
	newRun := fx.Insert(t, "collaboration_run", testutil.Cols{"id": uuid.NewString()})
	fx.Exec(t, `UPDATE collaboration_run SET prompt_snapshot='[]'::jsonb,preparation_snapshot='{"version":1}'::jsonb WHERE id=$1`, newRun)
	for _, change := range []string{`'{}'::jsonb`, `NULL`} {
		if _, err = pool.Exec(ctx, `UPDATE collaboration_run SET preparation_snapshot=`+change+` WHERE id=$1`, newRun); err == nil {
			t.Fatal("claimed preparation could be replaced or erased")
		}
	}
	fx.Exec(t, `UPDATE agent_runtime SET collaboration_capabilities='{"runtime_config_epoch":"2"}'::jsonb WHERE id=$1`, runtimeID)
	fx.Exec(t, `UPDATE agent_runtime SET collaboration_capabilities='{"runtime_config_epoch":"2"}'::jsonb WHERE id=$1`, runtimeID)
	if fx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id=$1 AND collaboration_config_epoch=2`, runtimeID) != 1 {
		t.Fatal("capability change/no-op did not use exactly one resource epoch")
	}
	if _, err = pool.Exec(ctx, `UPDATE agent_runtime SET collaboration_capabilities='{"runtime_config_epoch":"2","changed":true}'::jsonb WHERE id=$1`, runtimeID); err == nil {
		t.Fatal("changed capabilities kept an old recorded epoch")
	}
	if _, err = pool.Exec(ctx, read("down")); err == nil || !strings.Contains(err.Error(), "retained collaboration preparation") {
		t.Fatalf("rollback erased retained preparation evidence: %v", err)
	}
	fx.Exec(t, `DELETE FROM agent_runtime WHERE id=$1`, runtimeID)
	fx.Exec(t, `DELETE FROM collaboration_run WHERE id=$1`, newRun)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, `INSERT INTO agent_runtime (collaboration_capabilities) VALUES ('{"runtime_config_epoch":"1"}'::jsonb)`); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = pool.Exec(bounded, read("down"))
	cancel()
	var busy *pgconn.PgError
	if !errors.As(err, &busy) || busy.Code != "55P03" {
		t.Fatalf("rollback failed to fence an uncommitted first capability: %v", err)
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, read("down"))
	if fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema()
		AND column_name IN ('preparation_snapshot','collaboration_capabilities')`) != 0 {
		t.Fatal("empty maintenance rollback left preparation columns")
	}
	fx.QueryRow(t, `SELECT to_jsonb(r)::text FROM collaboration_run r WHERE id=$1`, oldRun).Scan(&after)
	if before != after {
		t.Fatal("empty rollback changed unrelated retained proof")
	}
}
