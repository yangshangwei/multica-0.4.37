package migrations

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const ownerReconsentMigration = "746_collaboration_owner_reconsent"

// Only the columns the epoch writer reads. The trigger is the one migration 660
// installs on the real table.
const ownerReconsentSetup = `
CREATE TABLE agent (id int PRIMARY KEY, workspace_id uuid, owner_id uuid, runtime_id uuid, kind text, archived_at timestamptz,
    permission_mode text, runtime_mode text, runtime_config jsonb, instructions text, custom_env jsonb, custom_args jsonb,
    mcp_config jsonb, model text, thinking_level text, service_tier text, composio_toolkit_allowlist jsonb,
    disabled_runtime_skills jsonb, autonomy_level text, collaboration_config_epoch bigint NOT NULL DEFAULT 1);
CREATE TABLE collaboration_target (id int PRIMARY KEY, configuration_fingerprint text);
INSERT INTO agent (id, instructions, model, thinking_level, permission_mode) VALUES (1, 'original', 'model-a', 'low', 'private');
INSERT INTO collaboration_target (id) VALUES (1);
`

const ownerReconsentTrigger = `CREATE TRIGGER collaboration_agent_config BEFORE UPDATE ON agent FOR EACH ROW EXECUTE FUNCTION collaboration_advance_resource_config()`

func ownerReconsentEpochs(t *testing.T, ctx context.Context, conn *pgx.Conn) (config int64, boundary *int64) {
	t.Helper()
	if err := conn.QueryRow(ctx, `SELECT collaboration_config_epoch, collaboration_boundary_epoch FROM agent WHERE id=1`).Scan(&config, &boundary); err != nil {
		t.Fatal(err)
	}
	return config, boundary
}

func TestCollaborationOwnerReconsentMigrationSeparatesBoundaryEpoch(t *testing.T) {
	ctx, conn, _ := adminRollbackConnections(t, adminRollbackCase{setup: ownerReconsentSetup})
	applyMigrationFile(t, ctx, conn, ownerReconsentMigration+".up.sql")
	if _, err := conn.Exec(ctx, ownerReconsentTrigger); err != nil {
		t.Fatal(err)
	}
	var boundaryColumn bool
	if err := conn.QueryRow(ctx, `SELECT configuration_boundary_fingerprint IS NULL FROM collaboration_target WHERE id=1`).Scan(&boundaryColumn); err != nil || !boundaryColumn {
		t.Fatalf("existing target did not keep a NULL boundary: %v", err)
	}

	if _, err := conn.Exec(ctx, `UPDATE agent SET instructions='edited', model='model-b', thinking_level='high' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if config, boundary := ownerReconsentEpochs(t, ctx, conn); config != 2 || boundary != nil {
		t.Fatalf("an in-boundary edit: config=%d boundary=%v", config, boundary)
	}
	if _, err := conn.Exec(ctx, `UPDATE agent SET permission_mode='public_to' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if config, boundary := ownerReconsentEpochs(t, ctx, conn); config != 3 || boundary == nil || *boundary != 1 {
		t.Fatalf("a widening edit: config=%d boundary=%v", config, boundary)
	}
	if _, err := conn.Exec(ctx, `UPDATE agent SET collaboration_config_epoch=collaboration_config_epoch+1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if config, boundary := ownerReconsentEpochs(t, ctx, conn); config != 4 || boundary == nil || *boundary != 2 {
		t.Fatalf("an explicit child advance: config=%d boundary=%v", config, boundary)
	}
	_, err := conn.Exec(ctx, `UPDATE agent SET collaboration_boundary_epoch=collaboration_boundary_epoch+1 WHERE id=1`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("direct boundary assignment: %v", err)
	}
	if _, err := conn.Exec(ctx, `UPDATE agent SET instructions=instructions WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if config, boundary := ownerReconsentEpochs(t, ctx, conn); config != 4 || *boundary != 2 {
		t.Fatalf("an unchanged write advanced an epoch: config=%d boundary=%v", config, boundary)
	}

	// Rollback restores the 704 writer, where every agent column advances only
	// the configuration epoch, then reapply starts boundaries from NULL again.
	applyMigrationFile(t, ctx, conn, ownerReconsentMigration+".down.sql")
	var columns int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema()
		AND column_name IN ('collaboration_boundary_epoch','configuration_boundary_fingerprint')`).Scan(&columns); err != nil || columns != 0 {
		t.Fatalf("rollback left %d boundary columns: %v", columns, err)
	}
	if _, err := conn.Exec(ctx, `UPDATE agent SET instructions='after rollback', permission_mode='private' WHERE id=1`); err != nil {
		t.Fatalf("restored 704 writer rejected an edit: %v", err)
	}
	var config int64
	if err := conn.QueryRow(ctx, `SELECT collaboration_config_epoch FROM agent WHERE id=1`).Scan(&config); err != nil || config != 5 {
		t.Fatalf("restored 704 writer: config=%d %v", config, err)
	}
	applyMigrationFile(t, ctx, conn, ownerReconsentMigration+".up.sql")
	applyMigrationFile(t, ctx, conn, ownerReconsentMigration+".up.sql")
	if _, err := conn.Exec(ctx, `UPDATE agent SET model='model-c' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if config, boundary := ownerReconsentEpochs(t, ctx, conn); config != 6 || boundary != nil {
		t.Fatalf("reapplied writer: config=%d boundary=%v", config, boundary)
	}
}
