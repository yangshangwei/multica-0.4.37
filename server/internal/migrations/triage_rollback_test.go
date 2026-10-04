package migrations

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func triageRollbackFixture() adminRollbackCase {
	return adminRollbackCase{
		setup: `CREATE TABLE issue(id uuid,workspace_id uuid); CREATE TABLE comment(id uuid); CREATE TABLE issue_source_context(id uuid,source_issue_id uuid);`,
		up:    "513_triage_tables.up.sql",
	}
}

func TestTriageEveryRollbackStepPreservesAdmittedWork(t *testing.T) {
	for _, file := range migrationFilesForLint(t, "*.down.sql") {
		name := filepath.Base(file)
		n, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			t.Fatal(err)
		}
		if n < 513 || n > 535 {
			continue
		}
		t.Run(name, func(t *testing.T) {
			ctx, conn, _ := adminRollbackConnections(t, triageRollbackFixture())
			if n != 513 {
				applyMigrationFile(t, ctx, conn, strings.Replace(name, ".down.sql", ".up.sql", 1))
			}
			if _, err := conn.Exec(ctx, `INSERT INTO issue(id,workspace_id,admission_status) VALUES(gen_random_uuid(),gen_random_uuid(),'accepted')`); err != nil {
				t.Fatal(err)
			}
			_, err := conn.Exec(ctx, readMigrationFile(t, name))
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
				t.Fatalf("downgrade must refuse before deleting any data or execution/index fence, got %v", err)
			}
		})
	}
}

func TestTriageRollbackRetainsIsolatedRetryAndNotificationEvidence(t *testing.T) {
	writes := map[string]string{
		"import row tombstone": `INSERT INTO triage_import_row(batch_id,workspace_id,row_number,status,issue_id) VALUES(gen_random_uuid(),gen_random_uuid(),1,'created',gen_random_uuid())`,
		"notification":         `INSERT INTO triage_notification(id,workspace_id,recipient_id,event_key,title) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'batch:retained','summary')`,
		"configured workspace": `INSERT INTO workspace_triage_settings(workspace_id,enabled) VALUES(gen_random_uuid(),true)`,
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			ctx, conn, _ := adminRollbackConnections(t, triageRollbackFixture())
			if _, err := conn.Exec(ctx, write); err != nil {
				t.Fatal(err)
			}
			_, err := conn.Exec(ctx, readMigrationFile(t, "513_triage_tables.down.sql"))
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
				t.Fatalf("retained evidence must block destructive rollback: %v", err)
			}
		})
	}
}

func TestTriageEmptyMaintenanceRollbackAndConcurrentWriter(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx, conn, _ := adminRollbackConnections(t, triageRollbackFixture())
		applyMigrationFile(t, ctx, conn, "535_triage_execution_fence.up.sql")
		applyMigrationFile(t, ctx, conn, "535_triage_execution_fence.down.sql")
		applyMigrationFile(t, ctx, conn, "513_triage_tables.down.sql")
		var removed bool
		if err := conn.QueryRow(ctx, `SELECT to_regclass('triage_action') IS NULL`).Scan(&removed); err != nil || !removed {
			t.Fatalf("empty rollback failed: %v", err)
		}
	})
	t.Run("first uncommitted intake", func(t *testing.T) {
		ctx, conn, writer := adminRollbackConnections(t, triageRollbackFixture())
		tx, err := writer.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `INSERT INTO issue(id,workspace_id,admission_status) VALUES(gen_random_uuid(),gen_random_uuid(),'pending')`); err != nil {
			t.Fatal(err)
		}
		_, err = conn.Exec(ctx, readMigrationFile(t, "513_triage_tables.down.sql"))
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
			t.Fatalf("live writer must fail rollback immediately: %v", err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var count int
		if err = conn.QueryRow(ctx, `SELECT count(*) FROM issue WHERE admission_status='pending'`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("writer lost: count=%d err=%v", count, err)
		}
	})
}
