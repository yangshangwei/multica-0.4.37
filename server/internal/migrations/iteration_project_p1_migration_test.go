package migrations

import (
	"path/filepath"
	"strings"
	"testing"
)

func iterationMigrationFiles(t *testing.T, direction string) []string {
	t.Helper()
	var files []string
	for _, path := range migrationFilesForLint(t, "*."+direction+".sql") {
		if strings.Contains(filepath.Base(path), "_iteration_") {
			files = append(files, filepath.Base(path))
		}
	}
	if len(files) != 17 {
		t.Fatalf("I1 migration coverage changed: got %d, want 17", len(files))
	}
	return files
}

func TestIterationUpgradeAndEmptyRollbackPreservesProjectP1(t *testing.T) {
	f := newProjectP1Fixture(t)
	projectP1Up(t, f)
	for _, scenario := range projectP1ProtectedWrites {
		projectP1Exec(t, f, scenario.write)
	}
	projectP1Exec(t, f, `INSERT INTO issue(id,workspace_id,status,admission_status) VALUES(gen_random_uuid(),gen_random_uuid(),'in_progress','accepted')`)
	before := map[string]string{}
	for _, table := range append([]string{"workspace", "project", "issue", "project_resource"}, projectP1Tables...) {
		before[table] = projectP1Snapshot(t, f, table)
	}
	for _, file := range iterationMigrationFiles(t, "up") {
		applyMigrationFile(t, f.ctx, f.conn, file)
	}
	for table, expected := range before {
		var exclude []string
		if table == "issue" {
			exclude = []string{"current_iteration_id", "iteration_rollover_count"}
		}
		if got := projectP1Snapshot(t, f, table, exclude...); got != expected {
			t.Errorf("I1 upgrade changed existing %s: before=%s after=%s", table, expected, got)
		}
	}
	files := iterationMigrationFiles(t, "down")
	for i := len(files) - 1; i >= 0; i-- {
		applyMigrationFile(t, f.ctx, f.conn, files[i])
	}
	for table, expected := range before {
		if got := projectP1Snapshot(t, f, table); got != expected {
			t.Errorf("empty I1 rollback changed existing %s: before=%s after=%s", table, expected, got)
		}
	}
	var iterationGone, projectIndexesIntact bool
	if err := f.conn.QueryRow(f.ctx, `SELECT to_regclass('iteration') IS NULL
		AND to_regclass('workspace_iteration_settings') IS NULL
		AND NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
		AND table_name='issue' AND column_name IN ('current_iteration_id','iteration_rollover_count'))`).Scan(&iterationGone); err != nil || !iterationGone {
		t.Fatalf("empty I1 rollback did not remove owned schema: %v", err)
	}
	if err := f.conn.QueryRow(f.ctx, `SELECT count(*)=13 AND bool_and(i.indisvalid AND i.indisready AND i.indislive)
		FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid WHERE c.relnamespace=$1::regnamespace`, f.schema).Scan(&projectIndexesIntact); err != nil || !projectIndexesIntact {
		t.Fatalf("empty I1 rollback changed P1 indexes: %v", err)
	}
}

func TestIterationRollbackAfterProjectP1RetainsUsedData(t *testing.T) {
	f := newProjectP1Fixture(t)
	projectP1Up(t, f)
	projectP1Exec(t, f, `INSERT INTO workspace(id,planning_timezone) VALUES(gen_random_uuid(),'Asia/Shanghai')`)
	for _, file := range iterationMigrationFiles(t, "up") {
		applyMigrationFile(t, f.ctx, f.conn, file)
	}
	projectP1Exec(t, f, `INSERT INTO iteration_operation(workspace_id,actor_user_id,request_id,operation,payload_hash,result)
		VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'delete','retained','{}')`)
	before := projectP1Snapshot(t, f, "iteration_operation")
	for _, file := range iterationMigrationFiles(t, "down") {
		_, err := f.conn.Exec(f.ctx, readMigrationFile(t, file))
		projectP1WantState(t, err, "P0001")
		if got := projectP1Snapshot(t, f, "iteration_operation"); got != before {
			t.Fatalf("%s changed retained operation evidence", file)
		}
	}
	var intact bool
	if err := f.conn.QueryRow(f.ctx, `SELECT count(*)=29 AND bool_and(i.indisvalid AND i.indisready AND i.indislive)
		FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid WHERE c.relnamespace=$1::regnamespace`, f.schema).Scan(&intact); err != nil || !intact {
		t.Fatalf("refused I1 rollback changed P1 or I1 indexes: %v", err)
	}
}
