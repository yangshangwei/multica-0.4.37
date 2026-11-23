package migrations

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

const legacyExecutionFenceUp = "650_collaboration_legacy_execution_fence.up.sql"
const legacyExecutionFenceDown = "650_collaboration_legacy_execution_fence.down.sql"

func legacyExecutionRollbackFixture() adminRollbackCase {
	return adminRollbackCase{
		setup: `CREATE TABLE agent_task_queue(id uuid);
			CREATE TABLE issue(id uuid, admission_status text);
			CREATE TABLE comment(id uuid, issue_id uuid, dispatch_eligible boolean);
			CREATE TABLE issue_collaboration(issue_id uuid);
			CREATE TABLE issue_source_context(id uuid, source_issue_id uuid);`,
		up: legacyExecutionFenceUp,
	}
}

func TestLegacyExecutionFenceRollbackRetainsCollaboration(t *testing.T) {
	ctx, conn, _ := adminRollbackConnections(t, legacyExecutionRollbackFixture())
	if _, err := conn.Exec(ctx, `
		INSERT INTO issue VALUES('00000000-0000-0000-0000-000000000001', 'not_required');
		INSERT INTO issue_collaboration VALUES('00000000-0000-0000-0000-000000000001');
	`); err != nil {
		t.Fatal(err)
	}
	_, err := conn.Exec(ctx, readMigrationFile(t, legacyExecutionFenceDown))
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
		t.Fatalf("retained collaboration must refuse weakening the execution fence: %v", err)
	}
	var allowed bool
	if err := conn.QueryRow(ctx, `SELECT lock_issue_execution('00000000-0000-0000-0000-000000000001')`).Scan(&allowed); err != nil || allowed {
		t.Fatalf("refused rollback lost the collaboration fence: allowed=%v err=%v", allowed, err)
	}
}

func TestLegacyExecutionFenceEmptyRollbackKeepsTriage(t *testing.T) {
	ctx, conn, _ := adminRollbackConnections(t, legacyExecutionRollbackFixture())
	applyMigrationFile(t, ctx, conn, legacyExecutionFenceDown)
	if _, err := conn.Exec(ctx, `
		INSERT INTO issue VALUES
			('00000000-0000-0000-0000-000000000001', 'not_required'),
			('00000000-0000-0000-0000-000000000002', 'pending');
		INSERT INTO comment VALUES('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', false);
		INSERT INTO issue_collaboration VALUES('00000000-0000-0000-0000-000000000001');
	`); err != nil {
		t.Fatal(err)
	}
	// The synthetic marker is inserted only after the maintenance rollback:
	// this proves the old function was restored, without touching app data.
	var ordinary, pending, pendingComment bool
	if err := conn.QueryRow(ctx, `SELECT
		lock_issue_execution('00000000-0000-0000-0000-000000000001'),
		lock_issue_execution('00000000-0000-0000-0000-000000000002'),
		lock_issue_execution('00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000003')
	`).Scan(&ordinary, &pending, &pendingComment); err != nil || !ordinary || pending || pendingComment {
		t.Fatalf("rollback changed triage behavior: ordinary=%v pending=%v comment=%v err=%v", ordinary, pending, pendingComment, err)
	}
}

func TestLegacyExecutionFenceRollbackRefusesConcurrentWriters(t *testing.T) {
	for name, write := range map[string]string{
		"atomic creation before collaboration marker": `INSERT INTO issue VALUES('00000000-0000-0000-0000-000000000001', 'not_required')`,
		"first collaboration marker":                  `INSERT INTO issue_collaboration VALUES('00000000-0000-0000-0000-000000000001')`,
		"ordinary queue writer":                       `INSERT INTO agent_task_queue VALUES('00000000-0000-0000-0000-000000000001')`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx, guard, writer := adminRollbackConnections(t, legacyExecutionRollbackFixture())
			tx, err := writer.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(ctx, write); err != nil {
				t.Fatal(err)
			}
			_, err = guard.Exec(ctx, readMigrationFile(t, legacyExecutionFenceDown))
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
				t.Fatalf("rollback must refuse an uncommitted writer before inspecting rows: %v", err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatalf("refused rollback disrupted the writer: %v", err)
			}
		})
	}
}

func TestLegacyExecutionFenceAtomicCreationNeverAdmitsOrdinaryWork(t *testing.T) {
	ctx, reader, writer := adminRollbackConnections(t, legacyExecutionRollbackFixture())
	tx, err := writer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `INSERT INTO issue VALUES('00000000-0000-0000-0000-000000000001','not_required')`); err != nil {
		t.Fatal(err)
	}
	var allowed bool
	if err := reader.QueryRow(ctx, `SELECT lock_issue_execution('00000000-0000-0000-0000-000000000001')`).Scan(&allowed); err != nil || allowed {
		t.Fatalf("uncommitted issue admitted ordinary work: allowed=%v err=%v", allowed, err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO issue_collaboration VALUES('00000000-0000-0000-0000-000000000001')`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := reader.QueryRow(ctx, `SELECT lock_issue_execution('00000000-0000-0000-0000-000000000001')`).Scan(&allowed); err != nil || allowed {
		t.Fatalf("committed collaboration admitted ordinary work: allowed=%v err=%v", allowed, err)
	}
}

func TestLegacyExecutionFenceMissingAuthoritySchemaFailsClosed(t *testing.T) {
	ctx, conn, _ := adminRollbackConnections(t, legacyExecutionRollbackFixture())
	if _, err := conn.Exec(ctx, `
		INSERT INTO issue VALUES('00000000-0000-0000-0000-000000000001','not_required');
		DROP TABLE issue_collaboration;
	`); err != nil {
		t.Fatal(err)
	}
	var allowed bool
	err := conn.QueryRow(ctx, `SELECT lock_issue_execution('00000000-0000-0000-0000-000000000001')`).Scan(&allowed)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42P01" || allowed {
		t.Fatalf("missing collaboration authority became ordinary admission: allowed=%v err=%v", allowed, err)
	}
}
