package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The real I1 files and registered runner hooks must recover a failed concurrent
// build without treating the ledger alone as proof of a usable schema.
func TestIterationMigrationsRecoverPartialInvalidIndex(t *testing.T) {
	ctx, pool, schema, versions := iterationRecoveryFixture(t)
	opts := runOptions{
		Direction: "up", Files: realMigrationFiles(t, versions[:3], "up"),
		SchemaMigrationsTable: schema + ".schema_migrations",
		AdvisoryLockKey:       int64(rand.Uint64()&0x7fffffffffffffff) | 1,
		Hooks:                 hooksForDirection("up"),
	}
	if err := runMigrations(ctx, pool, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO iteration
		(workspace_id,name,timezone,start_date,end_date,status,created_by)
		SELECT 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', 'Repair fixture ' || n,
		'UTC','2026-10-07','2026-10-20','active',gen_random_uuid()
		FROM generate_series(1,2) n`); err != nil {
		t.Fatal(err)
	}
	opts.Files = realMigrationFiles(t, versions, "up")
	err := runMigrations(ctx, pool, opts)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("conflicting active rows must fail the real 553 build: %v", err)
	}
	assertIndexValidity(t, pool, schema, "iteration_single_active", false)
	assertIndexExists(t, pool, schema, "iteration_workspace_status_date", false)
	iterationRecoveryLedgerCount(t, ctx, pool, 3)
	t.Log("partial up stopped at 553: 3 ledger rows; real single-active index INVALID; 554 absent")

	// Preserve the fixture's rows while resolving its conflict. This is an
	// isolated rehearsal, not advice to rewrite live iteration history.
	if _, err := pool.Exec(ctx, `UPDATE iteration SET status='planned' WHERE name='Repair fixture 2'`); err != nil {
		t.Fatal(err)
	}
	withoutCleanup := opts
	withoutCleanup.Hooks = nil
	err = runMigrations(ctx, pool, withoutCleanup)
	if !errors.As(err, &pgErr) || pgErr.Code != "42P07" {
		t.Fatalf("negative control must expose the invalid leftover without hooks: %v", err)
	}
	iterationRecoveryLedgerCount(t, ctx, pool, 3)
	if err := runMigrations(ctx, pool, opts); err != nil {
		t.Fatalf("real registered hooks failed to recover I1: %v", err)
	}
	iterationRecoveryLedgerCount(t, ctx, pool, 17)
	var total, usable, active, planned int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE i.indisvalid AND i.indisready AND i.indislive)
		FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid
		WHERE c.relnamespace=$1::regnamespace AND c.relname LIKE 'iteration_%'`, schema).Scan(&total, &usable); err != nil || total != 16 || usable != 16 {
		t.Fatalf("recovered I1 indexes: total=%d usable=%d err=%v", total, usable, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='active'), count(*) FILTER (WHERE status='planned') FROM iteration`).Scan(&active, &planned); err != nil || active != 1 || planned != 1 {
		t.Fatalf("recovery changed fixture rows: active=%d planned=%d err=%v", active, planned, err)
	}
	before := iterationRecoveryCatalog(t, ctx, pool, schema)
	if err := runMigrations(ctx, pool, opts); err != nil {
		t.Fatalf("recovered full prefix was not idempotent: %v", err)
	}
	if after := iterationRecoveryCatalog(t, ctx, pool, schema); after != before {
		t.Fatal("already-applied replay changed migration times, index identities, definitions or validity")
	}
	t.Log("recovery and replay: 17 ledger rows; all 16 I1 indexes valid/ready/live; catalog and applied_at unchanged")

	// Real retained I1 data refuses the first down step before any index or
	// ledger row can be removed; it does not need an operation tombstone too.
	reverse := slices.Clone(versions)
	slices.Reverse(reverse)
	down := opts
	down.Direction, down.Files, down.Hooks = "down", realMigrationFiles(t, reverse, "down"), hooksForDirection("down")
	err = runMigrations(ctx, pool, down)
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
		t.Fatalf("used I1 recovery must still refuse downgrade: %v", err)
	}
	if iterationRecoveryCatalog(t, ctx, pool, schema) != before {
		t.Fatal("refused downgrade changed catalog or ledger")
	}
	t.Log("used recovery rollback refused with P0001; all index identities and ledger rows retained")
}

func TestIterationMigrationsPartialEmptyRollbackPreservesSharedTimezone(t *testing.T) {
	for _, count := range []int{1, 3, 4, 11} {
		t.Run(fmt.Sprintf("after_%d_files", count), func(t *testing.T) {
			ctx, pool, schema, versions := iterationRecoveryFixture(t)
			opts := runOptions{
				Direction: "up", Files: realMigrationFiles(t, versions[:count], "up"),
				SchemaMigrationsTable: schema + ".schema_migrations",
				AdvisoryLockKey:       int64(rand.Uint64()&0x7fffffffffffffff) | 1,
				Hooks:                 hooksForDirection("up"),
			}
			if err := runMigrations(ctx, pool, opts); err != nil {
				t.Fatal(err)
			}
			iterationRecoveryLedgerCount(t, ctx, pool, count)
			reverse := slices.Clone(versions)
			slices.Reverse(reverse)
			opts.Direction, opts.Files, opts.Hooks = "down", realMigrationFiles(t, reverse, "down"), hooksForDirection("down")
			if err := runMigrations(ctx, pool, opts); err != nil {
				t.Fatalf("partial empty rollback: %v", err)
			}
			iterationRecoveryLedgerCount(t, ctx, pool, 0)
			var intact bool
			if err := pool.QueryRow(ctx, `SELECT
				(SELECT planning_timezone='Asia/Shanghai' FROM workspace)
				AND (SELECT title='Existing ordinary issue' FROM issue)
				AND to_regclass('iteration') IS NULL
				AND to_regclass('workspace_iteration_settings') IS NULL
				AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=$1
				AND table_name='issue' AND column_name IN ('current_iteration_id','iteration_rollover_count'))`, schema).Scan(&intact); err != nil || !intact {
				t.Fatalf("partial rollback changed shared/legacy state or left I1 schema: intact=%v err=%v", intact, err)
			}
			// Applying the complete sequence after partial down must be safe too.
			opts.Direction, opts.Files, opts.Hooks = "up", realMigrationFiles(t, versions, "up"), hooksForDirection("up")
			if err := runMigrations(ctx, pool, opts); err != nil {
				t.Fatalf("full upgrade after partial rollback: %v", err)
			}
			iterationRecoveryLedgerCount(t, ctx, pool, 17)
			t.Logf("%d-file empty prefix rolled back completely; shared timezone and issue preserved; full re-upgrade recorded 17 files", count)
		})
	}
}

func iterationRecoveryFixture(t *testing.T) (context.Context, *pgxpool.Pool, string, []string) {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires explicit isolated DATABASE_URL")
	}
	admin := openTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	schema := fmt.Sprintf("migrate_i1_recovery_%d_%d", time.Now().UnixNano(), rand.Uint32())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("remove owned recovery fixture: %v", err)
		}
	})
	pool := openTestPoolWithSearchPath(t, schema)
	if _, err := pool.Exec(ctx, `CREATE TABLE workspace(id uuid, planning_timezone text);
		CREATE TABLE issue(id uuid, workspace_id uuid, title text);
		INSERT INTO workspace VALUES(gen_random_uuid(),'Asia/Shanghai');
		INSERT INTO issue VALUES(gen_random_uuid(),gen_random_uuid(),'Existing ordinary issue')`); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*_iteration_*.up.sql"))
	if err != nil || len(paths) != 17 {
		t.Fatalf("I1 recovery sequence: files=%d err=%v", len(paths), err)
	}
	versions := make([]string, 0, len(paths))
	for _, path := range paths {
		versions = append(versions, strings.TrimSuffix(filepath.Base(path), ".up.sql"))
	}
	return ctx, pool, schema, versions
}

func iterationRecoveryLedgerCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&got); err != nil || got != want {
		t.Fatalf("I1 migration ledger: got=%d want=%d err=%v", got, want, err)
	}
}

func iterationRecoveryCatalog(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) string {
	t.Helper()
	var state string
	if err := pool.QueryRow(ctx, `SELECT jsonb_build_object(
		'ledger',(SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM schema_migrations m),
		'indexes',(SELECT jsonb_agg(jsonb_build_array(c.oid,c.relname,pg_get_indexdef(c.oid),
			i.indisvalid,i.indisready,i.indislive) ORDER BY c.relname)
			FROM pg_class c JOIN pg_index i ON i.indexrelid=c.oid WHERE c.relnamespace=$1::regnamespace))::text`, schema).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}
