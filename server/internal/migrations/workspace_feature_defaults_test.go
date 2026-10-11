package migrations

import "testing"

func TestWorkspaceFeatureDefaultsPreserveSavedChoices(t *testing.T) {
	ctx, conn, _ := adminRollbackConnections(t, adminRollbackCase{
		setup: `CREATE TABLE workspace(id uuid, planning_timezone text); CREATE TABLE issue(id uuid, workspace_id uuid, title text);`,
		up:    "513_triage_tables.up.sql",
	})
	applyMigrationFile(t, ctx, conn, "550_iteration_tables.up.sql")
	tables := []string{"workspace_triage_settings", "workspace_iteration_settings"}
	for _, table := range tables {
		if _, err := conn.Exec(ctx, "INSERT INTO "+table+"(workspace_id,enabled,revision) VALUES(gen_random_uuid(),false,7),(gen_random_uuid(),true,7)"); err != nil {
			t.Fatal(err)
		}
	}
	for _, step := range []struct {
		direction string
		enabled   bool
	}{{"up", true}, {"down", false}} {
		applyMigrationFile(t, ctx, conn, "567_workspace_feature_defaults."+step.direction+".sql")
		for _, table := range tables {
			var enabled bool
			if err := conn.QueryRow(ctx, "INSERT INTO "+table+"(workspace_id) VALUES(gen_random_uuid()) RETURNING enabled").Scan(&enabled); err != nil {
				t.Fatal(err)
			}
			if enabled != step.enabled {
				t.Errorf("%s %s default enabled=%v, want %v", step.direction, table, enabled, step.enabled)
			}
			var savedDisabled, savedEnabled int
			if err := conn.QueryRow(ctx, "SELECT count(*) FILTER (WHERE NOT enabled),count(*) FILTER (WHERE enabled) FROM "+table+" WHERE revision=7").Scan(&savedDisabled, &savedEnabled); err != nil {
				t.Fatal(err)
			}
			if savedDisabled != 1 || savedEnabled != 1 {
				t.Errorf("%s %s rewrote existing choices: disabled=%d enabled=%d", step.direction, table, savedDisabled, savedEnabled)
			}
		}
	}
}
