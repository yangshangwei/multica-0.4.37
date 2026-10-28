package migrations

import "testing"

const creationDefaultMigration = "744_collaboration_creation_default"

const creationDefaultSetup = `
CREATE TABLE collaboration_execution_gate (LIKE public.collaboration_execution_gate INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
ALTER TABLE collaboration_execution_gate ALTER COLUMN creation_enabled SET DEFAULT false;
CREATE TABLE admin_operation (kind text, target_kind text, result_code text, state text);
INSERT INTO collaboration_execution_gate (id) VALUES (1);
`

func TestCollaborationCreationDefaultPreservesAdministratorChoices(t *testing.T) {
	for _, tc := range []struct {
		name, kind, target, result, state string
		creation, want                    bool
	}{
		{name: "old default", want: true},
		{name: "already open", creation: true, want: true},
		{name: "explicit close", kind: "collaboration.gate", target: "collaboration_gate", result: "collaboration_creation_closed", state: "succeeded"},
		{name: "explicit open", kind: "collaboration.gate", target: "collaboration_gate", result: "collaboration_creation_opened", state: "succeeded", creation: true, want: true},
		{name: "current value after setting", kind: "collaboration.gate", target: "collaboration_gate", result: "collaboration_creation_opened", state: "succeeded"},
		{name: "failed close", kind: "collaboration.gate", target: "collaboration_gate", result: "collaboration_creation_closed", state: "failed", want: true},
		{name: "execution only", kind: "collaboration.gate", target: "collaboration_gate", result: "collaboration_execution_opened", state: "succeeded", want: true},
		{name: "unrelated operation", kind: "other", target: "collaboration_gate", result: "collaboration_creation_closed", state: "succeeded", want: true},
		{name: "unrelated target", kind: "collaboration.gate", target: "other", result: "collaboration_creation_closed", state: "succeeded", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, conn, _ := adminRollbackConnections(t, adminRollbackCase{setup: creationDefaultSetup})
			if _, err := conn.Exec(ctx, `UPDATE collaboration_execution_gate SET creation_enabled=$1, enabled=true, host_execution_enabled=true,
				epoch=42, freeze_operation_id=gen_random_uuid(), freeze_progress=gen_random_uuid() WHERE id=1`, tc.creation); err != nil {
				t.Fatal(err)
			}
			if tc.kind != "" {
				if _, err := conn.Exec(ctx, `INSERT INTO admin_operation(kind,target_kind,result_code,state) VALUES($1,$2,$3,$4)`, tc.kind, tc.target, tc.result, tc.state); err != nil {
					t.Fatal(err)
				}
			}
			var before, after string
			if err := conn.QueryRow(ctx, `SELECT (to_jsonb(g)-'creation_enabled')::text FROM collaboration_execution_gate g`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			applyMigrationFile(t, ctx, conn, creationDefaultMigration+".up.sql")
			var enabled bool
			if err := conn.QueryRow(ctx, `SELECT creation_enabled, (to_jsonb(g)-'creation_enabled')::text FROM collaboration_execution_gate g`).Scan(&enabled, &after); err != nil {
				t.Fatal(err)
			}
			if enabled != tc.want {
				t.Fatalf("creation_enabled=%t, want %t", enabled, tc.want)
			}
			if before != after {
				t.Fatalf("migration changed execution state: %s -> %s", before, after)
			}
		})
	}
}

func TestCollaborationCreationDefaultInsertRollbackAndReapply(t *testing.T) {
	ctx, conn, _ := adminRollbackConnections(t, adminRollbackCase{setup: creationDefaultSetup})
	applyMigrationFile(t, ctx, conn, creationDefaultMigration+".up.sql")
	if _, err := conn.Exec(ctx, `DELETE FROM collaboration_execution_gate; INSERT INTO collaboration_execution_gate(id) VALUES(1)`); err != nil {
		t.Fatal(err)
	}
	var creation, execution, host bool
	if err := conn.QueryRow(ctx, `SELECT creation_enabled,enabled,host_execution_enabled FROM collaboration_execution_gate`).Scan(&creation, &execution, &host); err != nil {
		t.Fatal(err)
	}
	if !creation || execution || host {
		t.Fatalf("new row defaults: creation=%t execution=%t host=%t", creation, execution, host)
	}
	applyMigrationFile(t, ctx, conn, creationDefaultMigration+".down.sql")
	if err := conn.QueryRow(ctx, `SELECT creation_enabled FROM collaboration_execution_gate`).Scan(&creation); err != nil || !creation {
		t.Fatalf("rollback changed the stored creation choice: %v", err)
	}
	var columnDefault string
	if err := conn.QueryRow(ctx, `SELECT column_default FROM information_schema.columns WHERE table_schema=current_schema()
		AND table_name='collaboration_execution_gate' AND column_name='creation_enabled'`).Scan(&columnDefault); err != nil || columnDefault != "false" {
		t.Fatalf("rollback default=%q, err=%v", columnDefault, err)
	}
	if _, err := conn.Exec(ctx, `UPDATE collaboration_execution_gate SET creation_enabled=false;
		INSERT INTO admin_operation VALUES('collaboration.gate','collaboration_gate','collaboration_creation_closed','succeeded')`); err != nil {
		t.Fatal(err)
	}
	applyMigrationFile(t, ctx, conn, creationDefaultMigration+".up.sql")
	applyMigrationFile(t, ctx, conn, creationDefaultMigration+".up.sql")
	if err := conn.QueryRow(ctx, `SELECT creation_enabled FROM collaboration_execution_gate`).Scan(&creation); err != nil || creation {
		t.Fatalf("reapplying migration reopened the administrator's close: %v", err)
	}
}
