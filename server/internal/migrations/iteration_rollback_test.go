package migrations

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func iterationRollbackFixture() adminRollbackCase {
	return adminRollbackCase{
		// P1 owns the shared timezone column before the I1 migrations run.
		setup: `CREATE TABLE workspace(id uuid, planning_timezone text); CREATE TABLE issue(id uuid, workspace_id uuid, title text);`,
		up:    "550_iteration_tables.up.sql",
	}
}

func TestIterationSingleActiveAndLegacyFieldPreservation(t *testing.T) {
	ctx, conn, writer := adminRollbackConnections(t, iterationRollbackFixture())
	applyMigrationFile(t, ctx, conn, "552_iteration_id.up.sql")
	applyMigrationFile(t, ctx, conn, "553_iteration_single_active.up.sql")
	const workspaceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const iterationID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	const issueID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	if _, err := conn.Exec(ctx, `INSERT INTO issue(id,workspace_id,title) VALUES($1,$2,'before')`, issueID, workspaceID); err != nil {
		t.Fatal(err)
	}
	var pointer *string
	var count int
	if err := conn.QueryRow(ctx, `SELECT current_iteration_id::text,iteration_rollover_count FROM issue WHERE id=$1`, issueID).Scan(&pointer, &count); err != nil || pointer != nil || count != 0 {
		t.Fatalf("legacy defaults: pointer=%v count=%d err=%v", pointer, count, err)
	}
	if _, err := conn.Exec(ctx, `UPDATE issue SET current_iteration_id=$2,iteration_rollover_count=3 WHERE id=$1`, issueID, iterationID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `UPDATE issue SET title='legacy edit' WHERE id=$1`, issueID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT current_iteration_id::text,iteration_rollover_count FROM issue WHERE id=$1`, issueID).Scan(&pointer, &count); err != nil || pointer == nil || *pointer != iterationID || count != 3 {
		t.Fatalf("legacy edit cleared iteration fields: pointer=%v count=%d err=%v", pointer, count, err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	insert := `INSERT INTO iteration(id,workspace_id,name,timezone,start_date,end_date,status,created_by) VALUES(gen_random_uuid(),$1,'same space','UTC','2026-10-06','2026-10-19','active',gen_random_uuid())`
	if _, err = tx.Exec(ctx, insert, workspaceID); err != nil {
		t.Fatal(err)
	}
	second := make(chan error, 1)
	go func() { _, err := writer.Exec(ctx, insert, workspaceID); second <- err }()
	// The winner remains uncommitted while the second connection attempts the
	// unique index write; it cannot independently create a second active row.
	select {
	case err := <-second:
		t.Fatalf("competing active write finished before winner committed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-second:
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			t.Fatalf("second active must conflict: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestIterationSchemaIsAdditive(t *testing.T) {
	sql := readMigrationFile(t, "550_iteration_tables.up.sql")
	for _, required := range []string{"current_iteration_id", "iteration_rollover_count", "iteration_participation", "iteration_snapshot", "iteration_operation", "iteration_notification"} {
		if !strings.Contains(sql, required) {
			t.Errorf("missing additive schema contract %s", required)
		}
	}
	for _, forbidden := range []string{"REFERENCES ", "FOREIGN KEY", "PRIMARY KEY", " UNIQUE ", "CREATE INDEX "} {
		if strings.Contains(strings.ToUpper(sql), forbidden) {
			t.Errorf("table migration contains forbidden implicit index or relationship: %s", forbidden)
		}
	}
}

func TestIterationEveryRollbackStepRetainsEvidence(t *testing.T) {
	writes := map[string]string{
		"current pointer":     `INSERT INTO issue(id,workspace_id,current_iteration_id) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid())`,
		"rollover count":      `INSERT INTO issue(id,workspace_id,iteration_rollover_count) VALUES(gen_random_uuid(),gen_random_uuid(),1)`,
		"settings":            `INSERT INTO workspace_iteration_settings(workspace_id) VALUES(gen_random_uuid())`,
		"operation tombstone": `INSERT INTO iteration_operation(id,workspace_id,actor_user_id,request_id,operation,payload_hash,result) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'delete','hash','{}')`,
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			ctx, conn, _ := adminRollbackConnections(t, iterationRollbackFixture())
			if _, err := conn.Exec(ctx, write); err != nil {
				t.Fatal(err)
			}
			for _, file := range migrationFilesForLint(t, "*.down.sql") {
				if !strings.Contains(filepath.Base(file), "_iteration_") {
					continue
				}
				_, err := conn.Exec(ctx, readMigrationFile(t, filepath.Base(file)))
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
					t.Fatalf("%s must preserve evidence: %v", filepath.Base(file), err)
				}
			}
		})
	}
}

func TestIterationEmptyRollbackAndConcurrentWriter(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx, conn, _ := adminRollbackConnections(t, iterationRollbackFixture())
		applyMigrationFile(t, ctx, conn, "550_iteration_tables.down.sql")
		var gone bool
		if err := conn.QueryRow(ctx, `SELECT to_regclass('iteration') IS NULL`).Scan(&gone); err != nil || !gone {
			t.Fatalf("empty schema rollback: gone=%v err=%v", gone, err)
		}
	})
	t.Run("uncommitted writer", func(t *testing.T) {
		ctx, guard, writer := adminRollbackConnections(t, iterationRollbackFixture())
		tx, err := writer.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `INSERT INTO workspace_iteration_settings(workspace_id) VALUES(gen_random_uuid())`); err != nil {
			t.Fatal(err)
		}
		_, err = guard.Exec(ctx, readMigrationFile(t, "550_iteration_tables.down.sql"))
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
			t.Fatalf("rollback must refuse a concurrent writer: %v", err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	})
}
